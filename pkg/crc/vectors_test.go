package crc_test

import (
	"encoding/binary"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/crc"
)

// The check values for this package live in vectors/crc/. A CRC result is
// a scalar, so it is written as hex at its natural width, big-endian —
// which is also the order the field takes on the wire.

func TestCRC16Vectors(t *testing.T) {
	vectors.RunFile(t, "crc/crc16.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			out := make([]byte, 2)
			binary.BigEndian.PutUint16(out, crc.ComputeCRC16(data))
			return out, nil
		},
	})
}

func TestCRC32Vectors(t *testing.T) {
	vectors.RunFile(t, "crc/crc32.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			out := make([]byte, 4)
			binary.BigEndian.PutUint32(out, crc.ComputeCRC32(data))
			return out, nil
		},
	})
}

// TestCRC16InteropVectors runs check values computed by Yamcs, a mission
// control system written from the standards by other authors.
//
// The published 0x29b1 is pinned elsewhere in this corpus. What these add is
// agreement on the cases no standard prints — an empty input, all zeros, a
// short arbitrary sequence — which is where an initial value or a final XOR
// that is wrong by a constant hides. The check value alone can be reached by
// more than one wrong combination.
func TestCRC16InteropVectors(t *testing.T) {
	vectors.RunFile(t, "crc/interop.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			data, err := f.Hex("data")
			if err != nil {
				return nil, err
			}
			out := make([]byte, 2)
			binary.BigEndian.PutUint16(out, crc.ComputeCRC16(data))
			return out, nil
		},
	})
}
