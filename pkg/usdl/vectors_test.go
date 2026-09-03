package usdl_test

import (
	"testing"

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
	if _, err := usdl.DecodeTransferFrame(padded, usdl.FECSize16, 0); err != usdl.ErrFrameLengthMismatch {
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
	if _, err := usdl.DecodeTransferFrame(encoded, 0, 0); err != usdl.ErrInvalidHeaderSpare {
		t.Errorf("expected ErrInvalidHeaderSpare, got %v", err)
	}
}
