package tmsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// The wire vectors for this package live in vectors/tmsc/. The sync marker
// is fixed by the standard and the randomizer is XOR against a published
// sequence, so both are exactly checkable — and in both a symmetric error
// is invisible from the inside, which is what these octets pin.

func TestCADUVectors(t *testing.T) {
	vectors.RunFile(t, "tmsc/cadu.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			if f.Has("marker") {
				return tmsc.DefaultASM(), nil
			}
			frame, err := f.Hex("frame")
			if err != nil {
				return nil, err
			}
			randomize, err := config.BoolOr("randomize", false)
			if err != nil {
				return nil, err
			}
			return tmsc.WrapCADU(frame, nil, randomize), nil
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			randomize, err := config.BoolOr("randomize", false)
			if err != nil {
				return nil, err
			}
			frame, err := tmsc.UnwrapCADU(input, nil, randomize)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{"frame": frame}, nil
		},
	})
}
