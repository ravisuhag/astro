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

// The XML form is a second front door, and its risk is the observation
// element: clause 3.4.3 pairs a timetag with exactly one observable, and an
// element carrying two or none has to be refused.
func FuzzDecodeXML(f *testing.F) {
	f.Add("")
	f.Add("<tdm/>")
	f.Add(`<tdm id="CCSDS_TDM_VERS" version="2.0"><header/><body><segment><metadata/><data/></segment></body></tdm>`)

	if m, err := tdm.Decode([]byte(figureE19)); err == nil {
		if encoded, err := m.EncodeXML(); err == nil {
			f.Add(string(encoded))
		}
	}

	f.Fuzz(func(t *testing.T, data string) {
		m, err := tdm.DecodeXML([]byte(data))
		if err != nil {
			return
		}

		encoded, err := m.EncodeXML()
		if err != nil {
			t.Fatalf("a message that decoded failed to encode: %v", err)
		}
		again, err := tdm.DecodeXML(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own XML failed: %v\n%s", err, encoded)
		}
		if again.Observations() != m.Observations() {
			t.Fatal("the observation count changed on an XML round trip")
		}

		kvn, err := m.Encode()
		if err != nil {
			t.Fatalf("a message that decoded from XML failed to encode as KVN: %v", err)
		}
		crossed, err := tdm.Decode(kvn)
		if err != nil {
			t.Fatalf("the key-value form written from XML does not read back: %v", err)
		}
		// The units live in the metadata, so they must cross intact.
		for i := range m.Segments {
			if crossed.Segments[i].Metadata.RangeUnits() != m.Segments[i].Metadata.RangeUnits() {
				t.Fatalf("segment %d range units changed crossing the forms", i)
			}
		}
	})
}
