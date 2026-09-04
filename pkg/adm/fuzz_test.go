package adm_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/adm"
)

// The APM's risk is its six delimited blocks: a stop must match its start, a
// start must not nest, and each block allows a different keyword set.
func FuzzDecodeAPM(f *testing.F) {
	f.Add(figureG1)
	f.Add(figureG3)
	f.Add("")
	f.Add("CCSDS_APM_VERS = 2.0\n")

	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_START\n")
	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_STOP\n")
	f.Add("CCSDS_APM_VERS = 2.0\nCREATION_DATE = 2003-09-30T19:23:57\nORIGINATOR = X\nQUAT_START\nEULER_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeAPM([]byte(data))
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

		again, err := adm.DecodeAPM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if (again.Quaternion == nil) != (m.Quaternion == nil) ||
			(again.Euler == nil) != (m.Euler == nil) ||
			(again.Spin == nil) != (m.Spin == nil) ||
			(again.Inertia == nil) != (m.Inertia == nil) {
			t.Fatal("a block appeared or vanished on round trip")
		}
		if len(again.Maneuvers) != len(m.Maneuvers) {
			t.Fatal("the maneuver count changed on round trip")
		}
		if m.Quaternion != nil && again.Quaternion.Quaternion != m.Quaternion.Quaternion {
			t.Fatal("the quaternion changed on round trip")
		}
	})
}

// The AEM's risk is that a data line's width comes from the metadata. A file
// whose ATTITUDE_TYPE and line width disagree must be refused rather than
// read as something else.
func FuzzDecodeAEM(f *testing.F) {
	f.Add(figureG4)
	f.Add("")
	f.Add("CCSDS_AEM_VERS = 2.0\n")

	f.Add("CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\nMETA_START\n")
	f.Add("CCSDS_AEM_VERS = 2.0\nCREATION_DATE = 2002-11-04T17:22:31\nORIGINATOR = X\nDATA_START\nDATA_STOP\n")

	f.Fuzz(func(t *testing.T, data string) {
		m, err := adm.DecodeAEM([]byte(data))
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

		again, err := adm.DecodeAEM(encoded)
		if err != nil {
			t.Fatalf("re-decoding our own output failed: %v\n%s", err, encoded)
		}
		if again.Records() != m.Records() || len(again.Blocks) != len(m.Blocks) {
			t.Fatal("the shape changed on round trip")
		}
		for i := range m.Blocks {
			// The type decides how the values are read, so it must survive.
			if again.Blocks[i].Metadata.Type != m.Blocks[i].Metadata.Type {
				t.Fatalf("block %d attitude type changed on round trip", i)
			}
			for j := range m.Blocks[i].Lines {
				if len(again.Blocks[i].Lines[j].Values) != len(m.Blocks[i].Lines[j].Values) {
					t.Fatalf("block %d line %d width changed on round trip", i, j)
				}
			}
		}
	})
}
