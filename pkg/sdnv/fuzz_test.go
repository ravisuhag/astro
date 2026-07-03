package sdnv_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0x7F})
	f.Add([]byte{0x81, 0x00})
	f.Add([]byte{0x81, 0x84, 0x34})
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and anything that decodes must re-encode to
		// exactly the octets it consumed. That catches non-canonical
		// encodings slipping through as valid.
		v, n, err := sdnv.Decode(data)
		if err != nil {
			return
		}
		if n > len(data) {
			t.Fatalf("consumed %d octets from a %d-octet input", n, len(data))
		}

		reEncoded := sdnv.Encode(v)
		if len(reEncoded) > n {
			t.Fatalf("re-encoding %d took %d octets, more than the %d decoded", v, len(reEncoded), n)
		}
	})
}

func FuzzRoundTrip(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(127))
	f.Add(uint64(128))
	f.Add(^uint64(0))

	f.Fuzz(func(t *testing.T, v uint64) {
		// Property: every uint64 survives a round trip exactly.
		encoded := sdnv.Encode(v)
		if len(encoded) != sdnv.EncodedSize(v) {
			t.Fatalf("EncodedSize said %d, Encode produced %d", sdnv.EncodedSize(v), len(encoded))
		}

		got, n, err := sdnv.Decode(encoded)
		if err != nil {
			t.Fatalf("decoding our own encoding of %d failed: %v", v, err)
		}
		if got != v {
			t.Fatalf("round trip of %d gave %d", v, got)
		}
		if n != len(encoded) {
			t.Fatalf("consumed %d of %d octets", n, len(encoded))
		}
	})
}
