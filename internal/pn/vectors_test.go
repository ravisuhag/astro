package pn_test

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/pn"
	"github.com/ravisuhag/astro/internal/vectors"
)

// The published sequence digits for this package live in vectors/pn/.
// They are the whole defence: the randomizer is XOR, so it is its own
// inverse and any sequence at all round-trips perfectly. Only these
// published octets distinguish correct taps from a wrong set.

func TestSequenceVectors(t *testing.T) {
	vectors.RunFile(t, "pn/sequences.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			which, err := f.Str("sequence")
			if err != nil {
				return nil, err
			}
			n, err := f.Uint("length")
			if err != nil {
				return nil, err
			}
			switch which {
			case "tm":
				return pn.TMSequence(int(n)), nil
			case "tc":
				return pn.TCSequence(int(n)), nil
			case "oid":
				out := make([]byte, n)
				pn.NewOIDSequence().Fill(out)
				return out, nil
			default:
				return nil, fmt.Errorf("unknown sequence %q", which)
			}
		},
	})
}
