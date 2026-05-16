package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

func FuzzDecodeTransferFrame(f *testing.F) {
	// Seed with a valid encoded frame so the fuzzer starts from real structure.
	if frame, err := usdl.NewTransferFrame(42, 5, 1, []byte("seed-payload")); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 7))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic across the decode parameter matrix.
		for _, fecSize := range []int{usdl.FECSize16, usdl.FECSize32} {
			for _, izLen := range []int{0, 8} {
				_, _ = usdl.DecodeTransferFrame(data, fecSize, izLen)
				_, _ = usdl.DecodeTransferFrameWithOCF(data, fecSize, izLen)
			}
		}
	})
}
