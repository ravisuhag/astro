package pxsc_test

import (
	"encoding/binary"
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

// TestCRC32InteropVectors runs values computed by Yamcs, a mission control
// system written from the standards by other authors.
//
// The Proximity-1 CRC-32 is not any of the common variants — different
// polynomial, different initial value — so an implementation reaching for a
// standard library CRC-32 gets a plausible wrong answer. An independent
// implementation agreeing is what rules that out.
func TestCRC32InteropVectors(t *testing.T) {
	vectors.RunFile(t, "pxsc/interop.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			out := make([]byte, 4)
			binary.BigEndian.PutUint32(out, pxsc.ComputeCRC32(data))
			return out, nil
		},
	})
}
