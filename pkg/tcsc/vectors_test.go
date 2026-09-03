package tcsc_test

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/tcsc"
)

// The wire vectors for this package live in vectors/tcsc/. The BCH parity
// is the complement of the LFSR remainder, so the all-zero information
// field is the first case worth pinning: an implementation that dropped
// the complement would write 0x00 there and be wrong on every codeblock.

func TestBCHVectors(t *testing.T) {
	vectors.RunFile(t, "tcsc/bch.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			// The CLTU start and tail sequences are constants, not codeblocks.
			if which, err := f.Str("sequence"); err == nil {
				switch which {
				case "start":
					return tcsc.DefaultStartSequence(), nil
				case "tail":
					return tcsc.DefaultTailSequence(), nil
				default:
					return nil, fmt.Errorf("unknown sequence %q", which)
				}
			}
			info, err := f.Hex("info")
			if err != nil {
				return nil, err
			}
			cb, err := tcsc.BCHEncode(info)
			if err != nil {
				return nil, err
			}
			return cb[:], nil
		},

		ConstructFn: func(f, _ vectors.Fields) error {
			info, err := f.Hex("info")
			if err != nil {
				return err
			}
			_, err = tcsc.BCHEncode(info)
			return err
		},
	})
}
