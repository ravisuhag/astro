package tmdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func FuzzDecodeTMTransferFrame(f *testing.F) {
	// Seed with a valid encoded frame so the fuzzer starts from real structure.
	if frame, err := tmdl.NewTMTransferFrame(42, 5, []byte("seed-payload"), nil, nil); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	if frame, err := tmdl.NewTMTransferFrame(42, 5, []byte("seed"), nil, []byte{1, 2, 3, 4}); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 6))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = tmdl.DecodeTMTransferFrame(data)
	})
}
