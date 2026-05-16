package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

func FuzzDecodeTransferFrame(f *testing.F) {
	// Seed with a valid encoded frame so the fuzzer starts from real structure.
	if frame, err := aos.NewTransferFrame(42, 5, []byte("seed-payload")); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 6))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic across the decode parameter matrix.
		for _, izLen := range []int{0, 8} {
			for _, ocf := range []bool{false, true} {
				for _, fecf := range []bool{false, true} {
					_, _ = aos.DecodeTransferFrame(data, izLen, ocf, fecf)
				}
			}
		}
	})
}
