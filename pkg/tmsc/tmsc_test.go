package tmsc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmsc"
)

func TestDefaultASM(t *testing.T) {
	asm := tmsc.DefaultASM()
	want := []byte{0x1A, 0xCF, 0xFC, 0x1D}
	if !bytes.Equal(asm, want) {
		t.Errorf("DefaultASM() = %x, want %x", asm, want)
	}

	// Verify fresh copy each call.
	asm[0] = 0x00
	asm2 := tmsc.DefaultASM()
	if asm2[0] != 0x1A {
		t.Error("DefaultASM must return a fresh copy")
	}
}

func TestGeneratePNSequence(t *testing.T) {
	seq := tmsc.GeneratePNSequence(5)
	if len(seq) != 5 {
		t.Fatalf("len = %d, want 5", len(seq))
	}
	// First byte of CCSDS PN sequence from all-ones register is 0xFF.
	if seq[0] != 0xFF {
		t.Errorf("first byte = 0x%02X, want 0xFF", seq[0])
	}

	// Deterministic: same call yields same output.
	seq2 := tmsc.GeneratePNSequence(5)
	if !bytes.Equal(seq, seq2) {
		t.Error("PN sequence must be deterministic")
	}
}

func TestRandomize_SelfInverse(t *testing.T) {
	original := []byte("hello spacecraft")
	randomized := tmsc.Randomize(original)

	// Should differ from original.
	if bytes.Equal(randomized, original) {
		t.Error("randomized data should differ from original")
	}

	// Applying again should recover original (XOR is self-inverse).
	recovered := tmsc.Randomize(randomized)
	if !bytes.Equal(recovered, original) {
		t.Errorf("Randomize is not self-inverse: got %x, want %x", recovered, original)
	}
}

func TestRandomize_DoesNotMutateInput(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03}
	saved := make([]byte, len(original))
	copy(saved, original)

	tmsc.Randomize(original)
	if !bytes.Equal(original, saved) {
		t.Error("Randomize must not modify the input slice")
	}
}

func TestWrapCADU_DefaultASM(t *testing.T) {
	frameData := []byte{0x01, 0x02, 0x03, 0x04}
	cadu := tmsc.WrapCADU(frameData, nil, false)

	asm := tmsc.DefaultASM()
	if !bytes.Equal(cadu[:4], asm) {
		t.Errorf("ASM = %x, want %x", cadu[:4], asm)
	}
	if !bytes.Equal(cadu[4:], frameData) {
		t.Errorf("frame data = %x, want %x", cadu[4:], frameData)
	}
}

func TestWrapCADU_CustomASM(t *testing.T) {
	customASM := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	frameData := []byte{0x01, 0x02}
	cadu := tmsc.WrapCADU(frameData, customASM, false)

	if !bytes.Equal(cadu[:4], customASM) {
		t.Errorf("ASM = %x, want %x", cadu[:4], customASM)
	}
}

func TestWrapUnwrapCADU_RoundTrip(t *testing.T) {
	frameData := []byte("round trip test data")
	cadu := tmsc.WrapCADU(frameData, nil, false)
	got, err := tmsc.UnwrapCADU(cadu, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("round trip: got %x, want %x", got, frameData)
	}
}

func TestWrapUnwrapCADU_WithRandomization(t *testing.T) {
	frameData := []byte("secret data for randomization")

	cadu := tmsc.WrapCADU(frameData, nil, true)
	// Randomized frame body should differ from original.
	if bytes.Equal(cadu[4:], frameData) {
		t.Error("randomized data should differ from plain")
	}

	got, err := tmsc.UnwrapCADU(cadu, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("round trip with randomization: got %x, want %x", got, frameData)
	}
}

func TestUnwrapCADU_TooShort(t *testing.T) {
	_, err := tmsc.UnwrapCADU([]byte{0x1A}, nil, false)
	if !errors.Is(err, tmsc.ErrDataTooShort) {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestUnwrapCADU_BadASM(t *testing.T) {
	_, err := tmsc.UnwrapCADU([]byte{0x00, 0x00, 0x00, 0x00, 0x01}, nil, false)
	if !errors.Is(err, tmsc.ErrSyncMarkerMismatch) {
		t.Errorf("expected ErrSyncMarkerMismatch, got %v", err)
	}
}

func TestUnwrapCADU_CustomASM_Mismatch(t *testing.T) {
	customASM := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	// CADU with default ASM should fail against custom ASM.
	cadu := tmsc.WrapCADU([]byte{0x01}, nil, false)
	_, err := tmsc.UnwrapCADU(cadu, customASM, false)
	if !errors.Is(err, tmsc.ErrSyncMarkerMismatch) {
		t.Errorf("expected ErrSyncMarkerMismatch, got %v", err)
	}
}

func TestWrapUnwrapCADU_Deterministic(t *testing.T) {
	frameData := []byte("deterministic test")
	cadu1 := tmsc.WrapCADU(frameData, nil, true)
	cadu2 := tmsc.WrapCADU(frameData, nil, true)
	if !bytes.Equal(cadu1, cadu2) {
		t.Error("same input should produce same CADU")
	}
}

func TestPNSequenceMatchesTheCCSDSVector(t *testing.T) {
	// CCSDS 131.0-B specifies h(x) = x^8 + x^7 + x^5 + x^3 + 1 with the
	// register preset to all ones. CCSDS 142.0-B-1 §3.5.2.1, which adopts the
	// same sequence by reference, publishes the first 40 digits:
	//
	//   1111 1111 0100 1000 0000 1110 1100 0000 1001 1010
	//
	// A polynomial this short admits several plausible register layouts that
	// produce entirely different sequences, which is why the standard prints
	// the digits. Checking against them is the only test that can catch a
	// wrong tap position: XOR is self-inverse, so a round trip passes with any
	// sequence at all.
	want := []byte{0xFF, 0x48, 0x0E, 0xC0, 0x9A}

	got := tmsc.GeneratePNSequence(len(want))
	if !bytes.Equal(got, want) {
		t.Errorf("PN sequence = % X, want % X", got, want)
	}
}

func TestPNSequencePeriodIs255(t *testing.T) {
	// The sequence repeats every 255 digits, so 255 octets' worth of output
	// brings the register back to where it started.
	const periodBytes = 255
	seq := tmsc.GeneratePNSequence(periodBytes * 2)

	if !bytes.Equal(seq[:periodBytes], seq[periodBytes:]) {
		t.Error("the sequence does not repeat after 255 octets; the taps are wrong")
	}
}
