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

// TestSequenceInteropVectors runs the TM sequence as Yamcs produces it, a
// mission control system written from the standards by other authors.
//
// This is the smallest interop vector in the corpus and among the most
// valuable. Plan 026 found astro generating this sequence from the wrong
// feedback taps: every randomized frame was unreadable by a conforming
// receiver, and no round-trip test could tell, because XOR is self-inverse.
// The published digits caught it then. An independent implementation agreeing
// now means two sources rather than one reading of one table.
func TestSequenceInteropVectors(t *testing.T) {
	vectors.RunFile(t, "pn/interop.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			n, err := f.Uint("length")
			if err != nil {
				return nil, err
			}
			return pn.TMSequence(int(n)), nil
		},
	})
}
