package cdm_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cdm"
)

// The CDM's risk is that it has no delimiters. What separates its three
// sections is the OBJECT keyword, so a keyword's meaning depends entirely on
// how many OBJECT lines came before it.
func FuzzDecode(f *testing.F) {
	f.Add(clause362)
	f.Add("")
	f.Add("CCSDS_CDM_VERS = 1.0\n")

	// The section boundary abused in each direction.
	f.Add("CCSDS_CDM_VERS = 1.0\nCREATION_DATE = 2010-03-12T22:31:12\nORIGINATOR = X\nMESSAGE_ID = 1\nOBJECT = OBJECT1\n")
	f.Add("CCSDS_CDM_VERS = 1.0\nCREATION_DATE = 2010-03-12T22:31:12\nORIGINATOR = X\nMESSAGE_ID = 1\nX = 1.0\n")
	f.Add("CCSDS_CDM_VERS = 1.0\nCREATION_DATE = 2010-03-12T22:31:12\nORIGINATOR = X\nMESSAGE_ID = 1\nOBJECT = OBJECT3\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := cdm.Decode([]byte(data))
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

		again, err := cdm.Decode(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if len(again.Relative.Fields) != len(m.Relative.Fields) {
			t.Fatal("the relative section lost a keyword on round trip")
		}
		for i := range m.Objects {
			if len(again.Objects[i].Fields) != len(m.Objects[i].Fields) {
				t.Fatalf("object %d lost a keyword on round trip", i+1)
			}
			// The covariance order is not recoverable from the numbers, so it
			// has to survive the round trip on its own.
			if again.Objects[i].CovarianceOrder() != m.Objects[i].CovarianceOrder() {
				t.Fatalf("object %d covariance order changed on round trip", i+1)
			}
		}
	})
}
