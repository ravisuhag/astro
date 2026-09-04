package odm_test

import (
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/odm"
)

// The property is that arbitrary input never panics and never drives an
// allocation from a length field an attacker controls. A key-value message has
// no length fields, which moves the risk somewhere else: the scanner splits on
// four different line terminators, the value parsers accept several numeric
// forms, and a manoeuvre block repeats without any count saying how often.
func FuzzDecodeOPM(f *testing.F) {
	f.Add(figureG1)
	f.Add(figureG2)
	f.Add("")
	f.Add("CCSDS_OPM_VERS = 3.0\n")

	// Line terminators, including the LF/CR pair that is easy to split wrongly.
	f.Add("CCSDS_OPM_VERS = 3.0\rCREATION_DATE = 2022-11-06T09:23:57\n\rORIGINATOR = JAXA\r\n")

	// Shapes that should be refused rather than crash: a keyword with no
	// value, a value with no keyword, a repeated section, and a maneuver
	// parameter with no maneuver.
	f.Add("CCSDS_OPM_VERS =\n")
	f.Add("= 3.0\n")
	f.Add("CCSDS_OPM_VERS = 3.0\nMAN_DV_1 = 1.0\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeOPM([]byte(data))
		if err != nil {
			return
		}

		// Anything that decodes must survive being summarised, and must
		// re-encode: Encode calls Validate, and DecodeOPM has already run it,
		// so a message that decoded and then failed to encode would mean the
		// two disagree about what a valid message is.
		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		// And the second reading must agree with the first about the values.
		again, err := odm.DecodeOPM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if !again.Data.StateVector.Epoch.Equal(m.Data.StateVector.Epoch) {
			t.Fatalf("epoch changed on round trip: %v then %v",
				m.Data.StateVector.Epoch, again.Data.StateVector.Epoch)
		}
		if again.Data.StateVector.X != m.Data.StateVector.X {
			t.Fatalf("X changed on round trip: %v then %v",
				m.Data.StateVector.X, again.Data.StateVector.X)
		}
		if len(again.Data.Maneuvers) != len(m.Data.Maneuvers) {
			t.Fatalf("maneuver count changed on round trip: %d then %d",
				len(m.Data.Maneuvers), len(again.Data.Maneuvers))
		}
	})
}

// The OEM adds what the OPM does not have: delimited blocks that can nest
// wrongly or never close, positional data rows with two legal widths, and a
// covariance section whose 21 values are spread over however many lines the
// producer chose.
func FuzzDecodeOEM(f *testing.F) {
	f.Add(figureG11)
	f.Add(figureG12)
	f.Add(figureG13Covariance)
	f.Add("")
	f.Add("CCSDS_OEM_VERS = 3.0\n")

	// Delimiters out of place, which is the shape the block reader has to
	// survive: an unclosed group, a nested one, a stray stop, and a covariance
	// section with no epoch.
	f.Add("CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\nMETA_START\n")
	f.Add("CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\nMETA_START\nMETA_START\n")
	f.Add("CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\nMETA_STOP\n")
	f.Add("CCSDS_OEM_VERS = 3.0\nCREATION_DATE = 1996-11-04T17:22:31\nORIGINATOR = X\nCOVARIANCE_START\n1.0\nCOVARIANCE_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeOEM([]byte(data))
		if err != nil {
			return
		}

		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		again, err := odm.DecodeOEM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if again.Records() != m.Records() {
			t.Fatalf("record count changed on round trip: %d then %d", m.Records(), again.Records())
		}
		if len(again.Blocks) != len(m.Blocks) {
			t.Fatalf("block count changed on round trip: %d then %d", len(m.Blocks), len(again.Blocks))
		}
		for i := range m.Blocks {
			for j := range m.Blocks[i].Lines {
				if m.Blocks[i].Lines[j] != again.Blocks[i].Lines[j] {
					t.Fatalf("block %d line %d changed on round trip", i, j)
				}
			}
		}
	})
}

// The OMM's risk is the paired keywords: three slots in table 4-3 accept two
// names each, which name applies depends on MEAN_ELEMENT_THEORY, and giving
// both must be refused rather than silently resolved.
func FuzzDecodeOMM(f *testing.F) {
	f.Add(figureG7)
	f.Add("")
	f.Add("CCSDS_OMM_VERS = 3.0\n")

	// Both halves of each pair, and a TLE-based message breaking each of the
	// four conventions clause 4.2.4.6 fixes.
	f.Add(figureG7 + "SEMI_MAJOR_AXIS = 42165.0\n")
	f.Add(figureG7 + "BTERM = 0.02\n")
	f.Add(figureG7 + "AGOM = 0.01\n")
	f.Add(strings.Replace(figureG7, "REF_FRAME      = TEME", "REF_FRAME      = EME2000", 1))
	f.Add(strings.Replace(figureG7, "MEAN_ELEMENT_THEORY = SGP/SGP4", "MEAN_ELEMENT_THEORY = DSST", 1))

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeOMM([]byte(data))
		if err != nil {
			return
		}

		if m.Humanize() == "" {
			t.Fatal("Humanize returned nothing for a message that decoded")
		}
		encoded, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}

		again, err := odm.DecodeOMM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		// The choice between each pair of alternatives must survive, since it
		// changes what the numbers mean.
		if again.Data.Elements.UsesMeanMotion != m.Data.Elements.UsesMeanMotion {
			t.Fatal("the size keyword changed on round trip")
		}
		if (again.Data.TLE == nil) != (m.Data.TLE == nil) {
			t.Fatal("the TLE block appeared or vanished on round trip")
		}
		if m.Data.TLE != nil {
			if again.Data.TLE.UsesBTerm != m.Data.TLE.UsesBTerm {
				t.Fatal("the drag keyword changed on round trip")
			}
			if again.Data.TLE.UsesAgom != m.Data.TLE.UsesAgom {
				t.Fatal("the second-derivative keyword changed on round trip")
			}
		}
	})
}
