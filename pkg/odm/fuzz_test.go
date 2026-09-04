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

// The XML form is a second front door. Its risks are different from the
// key-value form's: attributes rather than bracketed units, blocks rather than
// keyword order, and an XML parser between the input and the message.
func FuzzDecodeXMLOPM(f *testing.F) {
	f.Add(figureG5)
	f.Add("")
	f.Add("<opm/>")
	f.Add(`<opm id="CCSDS_OPM_VERS" version="3.0"><header/><body/></opm>`)
	f.Add(`<opm id="CCSDS_OPM_VERS" version="3.0"><header/><body><segment><metadata/><data/></segment></body></opm>`)

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeXMLOPM([]byte(data))
		if err != nil {
			return
		}

		encoded, err := m.EncodeXML()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}
		again, err := odm.DecodeXMLOPM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own XML failed: %v\n%s", err, encoded)
		}
		if again.Data.StateVector.X != m.Data.StateVector.X {
			t.Fatal("X changed on an XML round trip")
		}

		// And the two forms must still agree after a crossing.
		kvn, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded from XML failed to encode as KVN: %v", err)
		}
		crossed, err := odm.DecodeOPM(kvn)
		if err != nil {
			t.Fatalf("the key-value form written from XML does not read back: %v", err)
		}
		if crossed.Data.StateVector.X != m.Data.StateVector.X {
			t.Fatal("X changed crossing the forms")
		}
	})
}

// The OCM is the largest surface in the package and the one with the least
// typing behind it. Its risks are its own: eight kinds of delimited section
// that can nest, repeat or never close; time tags that may be a date or a
// signed number and must not mix within a block; and row widths that come from
// a keyword rather than from the format.
func FuzzDecodeOCM(f *testing.F) {
	f.Add(ocmSimple)
	f.Add(ocmCharacteristics)
	f.Add(ocmCovariances)
	f.Add(ocmManeuvers)
	f.Add("")
	f.Add("CCSDS_OCM_VERS = 3.0\n")

	// Delimiters out of place: a section that never closes, one closed by
	// another's delimiter, two of a section that may appear once, and a data
	// row with nothing to belong to.
	const header = "CCSDS_OCM_VERS = 3.0\nCREATION_DATE = 2022-11-06T09:23:57\nORIGINATOR = X\n"
	const meta = "META_START\nEPOCH_TZERO = 2022-12-18T14:28:15.1172\nMETA_STOP\n"
	f.Add(header + meta + "TRAJ_START\n")
	f.Add(header + meta + "TRAJ_START\nCOV_STOP\n")
	f.Add(header + meta + "PHYS_START\nPHYS_STOP\nPHYS_START\nPHYS_STOP\n")
	f.Add(header + meta + "0.0 1.0 2.0\n")

	// Time tags of both kinds in one block, and a relative one with no
	// EPOCH_TZERO to resolve it against.
	f.Add(header + meta + "TRAJ_START\n0.0 1.0\n2022-12-18T14:28:15Z 2.0\nTRAJ_STOP\n")
	f.Add(header + "META_START\nMETA_STOP\nTRAJ_START\n0.0 1.0\nTRAJ_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeOCM([]byte(data))
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

		again, err := odm.DecodeOCM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}

		// The section shape must survive, since it is what says how to read
		// every number in the file.
		if len(again.Trajectories) != len(m.Trajectories) ||
			len(again.Covariances) != len(m.Covariances) ||
			len(again.Maneuvers) != len(m.Maneuvers) {
			t.Fatal("the section counts changed on round trip")
		}
		if (again.Physical == nil) != (m.Physical == nil) ||
			(again.Perturbations == nil) != (m.Perturbations == nil) ||
			(again.OrbitDetermination == nil) != (m.OrbitDetermination == nil) {
			t.Fatal("a section appeared or vanished on round trip")
		}
		if len(again.UserDefined) != len(m.UserDefined) {
			t.Fatal("the user-defined parameter count changed on round trip")
		}

		for i := range m.Trajectories {
			for j, row := range m.Trajectories[i].Rows {
				if strings.Join(again.Trajectories[i].Rows[j].Fields, " ") !=
					strings.Join(row.Fields, " ") {
					t.Fatalf("trajectory %d row %d changed on round trip", i, j)
				}
			}
		}
	})
}

// The OCM's XML form has the same eight sections behind a different parser,
// and one thing the key-value form does not: a units attribute that is put
// back onto the value on the way out, so a crossing between the forms has to
// survive whatever an attacker puts in it.
func FuzzDecodeXMLOCM(f *testing.F) {
	f.Add(ocmXML)
	f.Add("")
	f.Add("<ocm/>")
	f.Add(`<ocm id="CCSDS_OCM_VERS" version="3.0"><header/><body/></ocm>`)
	f.Add(`<ocm id="CCSDS_OCM_VERS" version="3.0"><header/><body><segment><metadata/><data/></segment></body></ocm>`)

	// Blocks out of order, repeated, and holding a row element that belongs to
	// another section.
	f.Add(strings.Replace(ocmXML, "<traj>", "<pert/><traj>", 1))
	f.Add(strings.Replace(ocmXML, "</pert>", "</pert><pert/>", 1))
	f.Add(strings.Replace(ocmXML, "<trajLine>", "<covLine>", 1))

	f.Fuzz(func(t *testing.T, data string) {
		m, err := odm.DecodeXMLOCM([]byte(data))
		if err != nil {
			return
		}

		encoded, err := m.EncodeXML()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}
		again, err := odm.DecodeXMLOCM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own XML failed: %v\n%s", err, encoded)
		}
		if len(again.Trajectories) != len(m.Trajectories) {
			t.Fatal("the trajectory count changed on an XML round trip")
		}

		// And the two forms must still agree after a crossing.
		kvn, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded from XML failed to encode as KVN: %v", err)
		}
		crossed, err := odm.DecodeOCM(kvn)
		if err != nil {
			t.Fatalf("the key-value form written from XML does not read back: %v\n%s", err, kvn)
		}
		if crossed.TimeSystem() != m.TimeSystem() {
			t.Fatal("the time system changed crossing the forms")
		}
	})
}
