package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/usdl"
)

// The wire vectors for this package are in vectors/usdl/frame.json and
// corpus_test.go runs them. What is left here are the checks a fixture
// cannot express.

// The decoded frame length field must match the delivered buffer.
func TestDecode_FrameLengthCrossCheck(t *testing.T) {
	frame, err := usdl.NewTransferFrame(1, 1, 0, []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// Deliver with a trailing extra byte: length field no longer matches.
	padded := append(append([]byte{}, encoded...), 0x00)
	if _, err := usdl.DecodeTransferFrameWithConfig(padded, usdl.ChannelConfig{HasFECF: true}); err != usdl.ErrFrameLengthMismatch {
		t.Errorf("expected ErrFrameLengthMismatch, got %v", err)
	}
}

// Reserved spare bits (bits 50-51) must be zero on decode.
func TestDecode_RejectsHeaderSpares(t *testing.T) {
	frame, err := usdl.NewTransferFrame(1, 1, 0, []byte{0x01},
		usdl.WithoutFECF(),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[6] |= 0x10 // set a reserved spare bit
	if _, err := usdl.DecodeTransferFrameWithConfig(encoded, usdl.ChannelConfig{}); err != usdl.ErrInvalidHeaderSpare {
		t.Errorf("expected ErrInvalidHeaderSpare, got %v", err)
	}
}

// TestPrimaryHeaderInteropVectors runs headers captured from spacepackets, a
// Python implementation written from the standards by other authors.
//
// USLP's header is variable length: the virtual channel frame count runs from
// nought to seven octets and its width is declared by three bits at the end of
// the fixed part. A decoder that assumed a fixed header reads frame data as
// header from the first non-zero count onwards, and its own encoder would
// agree with it.
func TestPrimaryHeaderInteropVectors(t *testing.T) {
	vectors.RunFile(t, "usdl/interop.json", vectors.Impl{
		EncodeFn: func(f, _ vectors.Fields) ([]byte, error) {
			h, err := interopHeaderFrom(f)
			if err != nil {
				return nil, err
			}
			return h.Encode()
		},
	})
}

// interopHeaderFrom builds a primary header from a vector's fields.
func interopHeaderFrom(f vectors.Fields) (usdl.PrimaryHeader, error) {
	var h usdl.PrimaryHeader

	read := func(name string, def uint64) uint64 {
		v, err := f.UintOr(name, def)
		if err != nil {
			return def
		}
		return v
	}
	flag := func(name string) bool {
		v, err := f.BoolOr(name, false)
		if err != nil {
			return false
		}
		return v
	}

	tfvn, err := f.Uint("tfvn")
	if err != nil {
		return h, err
	}
	scid, err := f.Uint("scid")
	if err != nil {
		return h, err
	}
	frameLength, err := f.Uint("frame_length")
	if err != nil {
		return h, err
	}

	h.TFVN = uint8(tfvn)
	h.SCID = uint16(scid)
	h.FrameLength = uint16(frameLength)
	h.SourceOrDest = uint8(read("source_or_dest", 0))
	h.VCID = uint8(read("vcid", 0))
	h.MAPID = uint8(read("map_id", 0))
	h.VCFCountLen = uint8(read("vcf_count_len", 0))
	h.VCFCount = read("vcf_count", 0)
	h.OCFFlag = flag("ocf_flag")
	h.BypassSeqCtrl = flag("bypass_seq_ctrl")
	h.ProtCtrlCmd = flag("prot_ctrl_cmd")
	return h, nil
}
