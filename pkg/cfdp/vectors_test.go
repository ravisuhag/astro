package cfdp_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/cfdp"
)

// The wire vectors for this package live in vectors/cfdp/. They cover the
// checksum and the LV/TLV encodings. The transaction machines need a
// sequence of calls, which no vector kind expresses.
//
// The annex F checksum is one of very few genuinely published CCSDS test
// vectors, so it leads the file.

func TestWireVectors(t *testing.T) {
	vectors.RunFile(t, "cfdp/wire.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			// A checksum vector carries a file rather than a kind.
			if f.Has("file") {
				file, err := f.Hex("file")
				if err != nil {
					return nil, err
				}
				c, err := cfdp.NewChecksum(cfdp.ChecksumModular)
				if err != nil {
					return nil, err
				}
				c.Update(0, file)
				out := make([]byte, 4)
				binary.BigEndian.PutUint32(out, c.Sum())
				return out, nil
			}

			kind, err := f.Str("kind")
			if err != nil {
				return nil, err
			}
			value, err := f.Hex("value")
			if err != nil {
				return nil, err
			}
			switch kind {
			case "lv":
				return cfdp.LV{Value: value}.Encode()
			case "tlv":
				typ, err := f.Uint("type")
				if err != nil {
					return nil, err
				}
				return cfdp.TLV{Type: cfdp.TLVType(typ), Value: value}.Encode()
			default:
				return nil, fmt.Errorf("unknown kind %q", kind)
			}
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			// Which encoding is present is prior agreement, not wire
			// content: both LV and TLV open with an octet that could be a
			// length or a type, so the octets alone cannot say. Guessing
			// from them reads a TLV as an LV, which is why kind lives in
			// config.
			kind, err := config.Str("kind")
			if err != nil {
				return nil, err
			}
			switch kind {
			case "lv":
				lv, n, err := cfdp.DecodeLV(input)
				if err != nil {
					return nil, err
				}
				return vectors.Fields{"value": lv.Value, "consumed": n}, nil
			case "tlv":
				tlv, n, err := cfdp.DecodeTLV(input)
				if err != nil {
					return nil, err
				}
				return vectors.Fields{
					"type": uint8(tlv.Type), "value": tlv.Value, "consumed": n,
				}, nil
			default:
				return nil, fmt.Errorf("unknown kind %q", kind)
			}
		},
	})
}
