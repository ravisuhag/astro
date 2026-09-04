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

// TestCLTUInteropVectors runs CLTUs produced by Yamcs, a mission control
// system written from the standards by other authors.
//
// This is the first independent check this package has had. A BCH(63,56)
// parity octet, the complement applied to it, and the 0x55 fill that pads a
// short codeblock are three places where an implementation agrees with itself
// perfectly and with nobody else — and the start sequence 0xeb90 does not even
// survive text extraction from the standard, which yields a pattern reading
// FF00 from a page that plainly shows EB90.
func TestCLTUInteropVectors(t *testing.T) {
	vectors.RunFile(t, "tcsc/interop.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
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
			// The randomizer sequence, read out by randomizing zeros.
			if length, err := f.Uint("length"); err == nil {
				return tcsc.Randomize(make([]byte, length)), nil
			}
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			return tcsc.WrapCLTU(data, tcsc.DefaultStartSequence(),
				tcsc.DefaultTailSequence(), false)
		},
	})
}
