package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

func FuzzDecodeTransferFrame(f *testing.F) {
	// Seed with valid encoded frames so the fuzzer starts from real structure.
	if frame, err := usdl.NewTransferFrame(42, 5, 1, []byte("seed-payload"),
		usdl.WithVCFCount(2, 7)); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	if frame, err := usdl.NewTruncatedFrame(42, 5, 1, []byte{0x01, 0x02}); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 7))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic across the decode parameter matrix.
		for _, fecSize := range []int{0, usdl.FECSize16, usdl.FECSize32} {
			for _, izLen := range []int{0, 8} {
				_, _ = usdl.DecodeTransferFrame(data, fecSize, izLen)
			}
		}
	})
}
