package spp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
)

func FuzzDecode(f *testing.F) {
	// Seed with a valid encoded packet so the fuzzer starts from real structure.
	if pkt, err := spp.NewTMPacket(0x2AB, []byte("seed-payload")); err == nil {
		if encoded, err := pkt.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	if pkt, err := spp.NewTCPacket(0x2AB, []byte("seed"), spp.WithErrorControl()); err == nil {
		if encoded, err := pkt.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 6))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = spp.Decode(data)
		_, _ = spp.Decode(data, spp.WithDecodeErrorControl())
	})
}
