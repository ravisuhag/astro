package sdnv_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/sdnv"
)

// The RFC 5050 worked examples for this package live in vectors/sdnv/.

func TestSDNVVectors(t *testing.T) {
	vectors.RunFile(t, "sdnv/sdnv.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			v, err := f.Uint("value")
			if err != nil {
				return nil, err
			}
			return sdnv.Encode(v), nil
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			value, consumed, err := sdnv.Decode(input)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{"value": value, "consumed": consumed}, nil
		},
	})
}

// TestEncodedSizeAgreesWithEncode ties the sizing helper to the encoder
// across every vector, which a fixture cannot express: it is a property
// between two functions, not a pinned byte string.
func TestEncodedSizeAgreesWithEncode(t *testing.T) {
	f, err := vectors.Load("sdnv/sdnv.json")
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	for _, v := range f.Encode {
		value, err := v.Fields.Uint("value")
		if err != nil {
			t.Fatalf("%s: %v", v.Name, err)
		}
		if got, want := sdnv.EncodedSize(value), len(sdnv.Encode(value)); got != want {
			t.Errorf("%s: EncodedSize(%d) = %d, but Encode produced %d octets",
				v.Name, value, got, want)
		}
	}
}
