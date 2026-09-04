package tdm_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tdm"
)

// The property is that arbitrary input never panics. A TDM's risk is its
// nesting: two pairs of delimiters that must alternate, a data record whose
// value is two fields rather than one, and forty-odd metadata keywords of
// which several are indexed families that shadow one another by prefix.
func FuzzDecode(f *testing.F) {
	f.Add(figureE19)
	f.Add("")
	f.Add("CCSDS_TDM_VERS = 2.0\n")

	// Delimiters out of place and records of the wrong width.
	f.Add("CCSDS_TDM_VERS = 2.0\nCREATION_DATE = 2010-050T20:15:02\nORIGINATOR = X\nMETA_START\n")
	f.Add("CCSDS_TDM_VERS = 2.0\nCREATION_DATE = 2010-050T20:15:02\nORIGINATOR = X\nDATA_START\nDATA_STOP\n")
	f.Add("CCSDS_TDM_VERS = 2.0\nCREATION_DATE = 2010-050T20:15:02\nORIGINATOR = X\nMETA_START\nMETA_STOP\nDATA_START\nRANGE = 1\nDATA_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := tdm.Decode([]byte(data))
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

		again, err := tdm.Decode(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if again.Observations() != m.Observations() {
			t.Fatalf("observation count changed: %d then %d", m.Observations(), again.Observations())
		}
		if len(again.Segments) != len(m.Segments) {
			t.Fatalf("segment count changed: %d then %d", len(m.Segments), len(again.Segments))
		}
		for i := range m.Segments {
			// A dropped metadata keyword changes what the measurements mean,
			// so the section has to survive whole.
			if len(again.Segments[i].Metadata.Fields) != len(m.Segments[i].Metadata.Fields) {
				t.Fatalf("segment %d lost metadata on round trip", i)
			}
			for j := range m.Segments[i].Observations {
				if again.Segments[i].Observations[j] != m.Segments[i].Observations[j] {
					t.Fatalf("segment %d observation %d changed on round trip", i, j)
				}
			}
		}
	})
}
