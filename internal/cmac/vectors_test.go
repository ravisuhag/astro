package cmac_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/cmac"
	"github.com/ravisuhag/astro/internal/vectors"
)

// The RFC 4493 and NIST SP 800-38B example sets for this package live in
// vectors/cmac/. Test keys only, never real keys.

func TestCMACVectors(t *testing.T) {
	vectors.RunFile(t, "cmac/aes.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			key, err := f.Hex("key")
			if err != nil {
				return nil, err
			}
			message, err := f.Hex("message")
			if err != nil {
				return nil, err
			}
			c, err := cmac.New(key)
			if err != nil {
				return nil, err
			}
			return c.Sum(message), nil
		},
	})
}
