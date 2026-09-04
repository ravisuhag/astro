package odm_test

import (
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
