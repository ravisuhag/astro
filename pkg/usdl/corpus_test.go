package usdl_test

import (
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/usdl"
)

// The wire vectors for this package live in vectors/usdl/. The USLP
// primary header is variable length — truncated at 4 octets, otherwise 7
// plus a VCF count field whose width the header declares — so each form is
// pinned separately.

func fecSize(config vectors.Fields) (int, error) {
	bits, err := config.UintOr("fec_size", 0)
	if err != nil {
		return 0, err
	}
	if bits == 16 {
		return usdl.FECSize16, nil
	}
	return 0, nil
}

func buildUSLPFrame(f vectors.Fields) (*usdl.TransferFrame, error) {
	scid, err := f.Uint("scid")
	if err != nil {
		return nil, err
	}
	vcid, err := f.Uint("vcid")
	if err != nil {
		return nil, err
	}
	mapid, err := f.Uint("mapid")
	if err != nil {
		return nil, err
	}
	data, err := f.HexOr("data", nil)
	if err != nil {
		return nil, err
	}

	var opts []usdl.FrameOption
	if f.Has("source_or_dest") {
		sd, err := f.Uint("source_or_dest")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithSourceOrDest(uint8(sd)))
	}
	if f.Has("construction_rule") {
		r, err := f.Uint("construction_rule")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithConstructionRule(uint8(r)))
	}
	if f.Has("upid") {
		u, err := f.Uint("upid")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithUPID(uint8(u)))
	}
	if f.Has("pointer") {
		p, err := f.Uint("pointer")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithPointer(uint16(p)))
	}
	if f.Has("vcf_count_len") {
		n, err := f.Uint("vcf_count_len")
		if err != nil {
			return nil, err
		}
		c, err := f.Uint("vcf_count")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithVCFCount(uint8(n), c))
	}
	if f.Has("ocf") {
		ocf, err := f.Hex("ocf")
		if err != nil {
			return nil, err
		}
		opts = append(opts, usdl.WithOCF(ocf))
	}

	truncated, err := f.BoolOr("truncated", false)
	if err != nil {
		return nil, err
	}
	if truncated {
		return usdl.NewTruncatedFrame(uint16(scid), uint8(vcid), uint8(mapid), data, opts...)
	}
	return usdl.NewTransferFrame(uint16(scid), uint8(vcid), uint8(mapid), data, opts...)
}

func TestFrameVectorsFromCorpus(t *testing.T) {
	vectors.RunFile(t, "usdl/frame.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			// The OID fill generator is a sequence, not a frame.
			if which, err := f.Str("sequence"); err == nil && which == "oid" {
				n, err := f.Uint("length")
				if err != nil {
					return nil, err
				}
				out := make([]byte, n)
				usdl.NewOIDSequence().Fill(out)
				return out, nil
			}
			frame, err := buildUSLPFrame(f)
			if err != nil {
				return nil, err
			}
			return frame.Encode()
		},

		ConstructFn: func(f, _ vectors.Fields) error {
			frame, err := buildUSLPFrame(f)
			if err != nil {
				return err
			}
			_, err = frame.Encode()
			return err
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			fec, err := fecSize(config)
			if err != nil {
				return nil, err
			}
			frame, err := usdl.DecodeTransferFrame(input, fec, 0)
			if err != nil {
				return nil, err
			}
			h := frame.Header
			return vectors.Fields{
				"scid":           h.SCID,
				"source_or_dest": h.SourceOrDest,
				"vcid":           h.VCID,
				"mapid":          h.MAPID,
				"end_of_fph":     h.EndOfFPH,
				"frame_length":   h.FrameLength,
				"ocf_flag":       h.OCFFlag,
				"vcf_count_len":  h.VCFCountLen,
				"vcf_count":      h.VCFCount,
				"ocf":            frame.OCF,
				"data":           frame.DataField,
				// The TFDF header selects how the data zone is read, so a
				// decoder that recovers the frame but not these has not
				// recovered the frame.
				"construction_rule": frame.DataFieldHeader.ConstructionRule,
				"upid":              frame.DataFieldHeader.UPID,
				"pointer":           frame.DataFieldHeader.Pointer,
				"has_pointer":       frame.DataFieldHeader.HasPointer(),
			}, nil
		},
	})
}

// TestDecodeRejectsAnInvalidFECSize is deliberately not a vector. The FECF
// is either absent or 16 bits (clause 4.1.6.2.2), so any other managed
// size is a caller mistake rather than a wire condition — no octet string
// expresses it. It checks this API's parameter contract, which is a
// property of this package rather than of the standard.
func TestDecodeRejectsAnInvalidFECSize(t *testing.T) {
	frame, err := vectors.Load("usdl/frame.json")
	if err != nil {
		t.Fatalf("loading vectors: %v", err)
	}
	// Reuse a known-good frame so only the size argument is wrong.
	var input []byte
	for _, v := range frame.Decode {
		if v.Name == "non-truncated-inverse" {
			var err error
			input, err = hexDecode(v.Input)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if input == nil {
		t.Fatal("non-truncated-inverse vector not found")
	}
	if _, err := usdl.DecodeTransferFrame(input, 3, 0); err != usdl.ErrInvalidFECSize {
		t.Errorf("DecodeTransferFrame with fecSize 3: got %v, want ErrInvalidFECSize", err)
	}
}

func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
