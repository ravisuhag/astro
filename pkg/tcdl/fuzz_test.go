package tcdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

func FuzzDecodeTCTransferFrame(f *testing.F) {
	// Seed with a valid encoded frame so the fuzzer starts from real structure.
	if frame, err := tcdl.NewTCTransferFrame(42, 5, []byte("seed-payload")); err == nil {
		if encoded, err := frame.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 7))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = tcdl.DecodeTCTransferFrame(data)
		_, _ = tcdl.DecodeTCTransferFrameWithSegmentHeader(data)
	})
}
