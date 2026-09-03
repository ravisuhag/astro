package bp_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/bp"
)

// The wire vectors for this package live in vectors/bp/, and they are
// deliberately narrow.
//
// Most of BP's encoding is SDNVs, pinned independently and more
// thoroughly in vectors/sdnv/sdnv.json; duplicating them here would add
// no coverage and a second place to drift. Endpoint identifiers are URI
// strings, not octet fields. What is left is the administrative record's
// type-and-flags octet, where the two share one octet and a decoder that
// reads the whole octet as a type gets it wrong.
//
// Reassembly, fragmentation and custody transfer need a sequence of
// calls, which no vector kind expresses.

func TestAdminRecordVectors(t *testing.T) {
	vectors.RunFile(t, "bp/admin-record.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			rt, err := f.Uint("record_type")
			if err != nil {
				return nil, err
			}
			flags, err := f.Uint("flags")
			if err != nil {
				return nil, err
			}
			// The type-and-flags octet is the whole of what this vector
			// pins; a full record needs a body the layout does not fix.
			return []byte{byte(rt)<<4 | byte(flags)&0x0F}, nil
		},

		DecodeFn: func(input []byte, _ vectors.Fields) (vectors.Fields, error) {
			// The real decoder, so reject vectors mean something. It needs a
			// complete record body, which is why this file carries no
			// decode vectors — only encodes and rejects.
			a, err := bp.DecodeAdminRecord(input)
			if err != nil {
				return nil, err
			}
			return vectors.Fields{
				"record_type": uint8(a.Type),
				"flags":       a.Flags,
			}, nil
		},
	})
}

// TestAdminRecordDecoderRejectsUndefinedTypes uses the real decoder for
// the reject cases, which the octet-splitting helper above cannot express.
func TestAdminRecordDecoderRejectsUndefinedTypes(t *testing.T) {
	f, err := vectors.Load("bp/admin-record.json")
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	for _, v := range f.Reject {
		if v.Input == nil {
			continue
		}
		t.Run(v.Name, func(t *testing.T) {
			input, err := hexBytes(*v.Input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bp.DecodeAdminRecord(input); err == nil {
				t.Errorf("DecodeAdminRecord(%x) was accepted; the standard requires %s",
					input, v.Error)
			}
		})
	}
}

func hexBytes(s string) ([]byte, error) {
	out := make([]byte, len(s)/2)
	for i := range out {
		var b byte
		for j := range 2 {
			c := s[i*2+j]
			switch {
			case c >= '0' && c <= '9':
				b = b<<4 | (c - '0')
			case c >= 'a' && c <= 'f':
				b = b<<4 | (c - 'a' + 10)
			default:
				return nil, bp.ErrDataTooShort
			}
		}
		out[i] = b
	}
	return out, nil
}
