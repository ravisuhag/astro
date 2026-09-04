package keywrap_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/internal/keywrap"
	"github.com/ravisuhag/astro/internal/vectors"
)

// The RFC 3394 example set for this package lives in vectors/keywrap/.
// Test keys only, never real keys.

func TestKeyWrapVectors(t *testing.T) {
	vectors.RunFile(t, "keywrap/keywrap.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			kek, err := f.Hex("kek")
			if err != nil {
				return nil, err
			}
			keyData, err := f.Hex("key_data")
			if err != nil {
				return nil, err
			}

			wrapped, err := keywrap.Wrap(kek, keyData)
			if err != nil {
				return nil, err
			}

			// Every vector is run in both directions. The corpus pins the
			// wrap; the unwrap has to return what went in, which is the
			// property RFC 3394 clause 2.2.3 builds the integrity check for.
			back, err := keywrap.Unwrap(kek, wrapped)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(back, keyData) {
				t.Errorf("Unwrap(Wrap(%x)) = %x, want the key data back", keyData, back)
			}
			return wrapped, nil
		},
	})
}
