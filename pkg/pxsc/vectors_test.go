package pxsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/pxsc"
)

// The wire vectors for this package live in vectors/pxsc/. They pin the
// convolutional code to the convention deployed receivers use. A
// mirror-image encoder decodes its own output and nobody else's, so a
// round trip proves nothing here — see the vector notes.

func TestConvolutionalVectors(t *testing.T) {
	vectors.RunFile(t, "pxsc/convolutional.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			return pxsc.ConvolutionalEncode(data), nil
		},
	})
}
