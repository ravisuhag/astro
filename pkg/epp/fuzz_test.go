package epp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func FuzzDecode(f *testing.F) {
	if pkt, err := epp.NewIPEPacket([]byte("seed-payload")); err == nil {
		if encoded, err := pkt.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	if pkt, err := epp.NewIdlePacket(); err == nil {
		if encoded, err := pkt.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 1))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = epp.Decode(data)
	})
}
