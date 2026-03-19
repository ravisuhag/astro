package tcsc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcsc"
)

func TestDefaultStartSequence(t *testing.T) {
	ss := tcsc.DefaultStartSequence()
	want := []byte{0xEB, 0x90}
	if !bytes.Equal(ss, want) {
		t.Errorf("DefaultStartSequence() = %x, want %x", ss, want)
	}

	// Verify fresh copy each call.
	ss[0] = 0x00
	ss2 := tcsc.DefaultStartSequence()
	if ss2[0] != 0xEB {
		t.Error("DefaultStartSequence must return a fresh copy")
	}
}

func TestDefaultTailSequence(t *testing.T) {
	ts := tcsc.DefaultTailSequence()
	want := []byte{0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0x79}
	if !bytes.Equal(ts, want) {
		t.Errorf("DefaultTailSequence() = %x, want %x", ts, want)
	}

	// Verify fresh copy each call.
	ts[0] = 0x00
	ts2 := tcsc.DefaultTailSequence()
	if ts2[0] != 0xC5 {
		t.Error("DefaultTailSequence must return a fresh copy")
	}
}

func TestGeneratePNSequence(t *testing.T) {
	seq := tcsc.GeneratePNSequence(5)
	if len(seq) != 5 {
		t.Fatalf("len = %d, want 5", len(seq))
	}
	// First byte of CCSDS PN sequence from all-ones register is 0xFF.
	if seq[0] != 0xFF {
		t.Errorf("first byte = 0x%02X, want 0xFF", seq[0])
	}

	// Deterministic: same call yields same output.
	seq2 := tcsc.GeneratePNSequence(5)
	if !bytes.Equal(seq, seq2) {
		t.Error("PN sequence must be deterministic")
	}
}

func TestRandomize_SelfInverse(t *testing.T) {
	original := []byte("hello spacecraft")
	randomized := tcsc.Randomize(original)

	// Should differ from original.
	if bytes.Equal(randomized, original) {
		t.Error("randomized data should differ from original")
	}

	// Applying again should recover original (XOR is self-inverse).
	recovered := tcsc.Randomize(randomized)
	if !bytes.Equal(recovered, original) {
		t.Errorf("Randomize is not self-inverse: got %x, want %x", recovered, original)
	}
}

func TestRandomize_DoesNotMutateInput(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03}
	saved := make([]byte, len(original))
	copy(saved, original)

	tcsc.Randomize(original)
	if !bytes.Equal(original, saved) {
		t.Error("Randomize must not modify the input slice")
	}
}

func TestWrapUnwrapCLTU_RoundTrip(t *testing.T) {
	// Data that is exactly 7 bytes (1 codeblock, no padding needed).
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Expected length: 2 (start) + 8 (1 codeblock) + 8 (tail) = 18
	if len(cltu) != 18 {
		t.Fatalf("CLTU length = %d, want 18", len(cltu))
	}

	got, corr, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if corr != 0 {
		t.Errorf("corrections = %d, want 0", corr)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("round trip: got %x, want %x", got, frameData)
	}
}

func TestWrapUnwrapCLTU_WithPadding(t *testing.T) {
	// Data that requires padding (5 bytes, needs 2 bytes of padding).
	frameData := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	// Result includes padding — first 5 bytes should match.
	if !bytes.Equal(got[:len(frameData)], frameData) {
		t.Errorf("data prefix: got %x, want %x", got[:len(frameData)], frameData)
	}
	// Remaining bytes should be the fill pattern.
	for i := len(frameData); i < len(got); i++ {
		if got[i] != 0x55 {
			t.Errorf("padding byte %d = 0x%02X, want 0x55", i, got[i])
		}
	}
}

func TestWrapUnwrapCLTU_MultiBlock(t *testing.T) {
	// Data that spans multiple codeblocks (21 bytes = 3 blocks).
	frameData := make([]byte, 21)
	for i := range frameData {
		frameData[i] = byte(i)
	}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Expected: 2 + 3*8 + 8 = 34
	if len(cltu) != 34 {
		t.Fatalf("CLTU length = %d, want 34", len(cltu))
	}

	got, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("round trip: got %x, want %x", got, frameData)
	}
}

func TestWrapUnwrapCLTU_WithRandomization(t *testing.T) {
	frameData := []byte("command data for spacecraft")

	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	// With padding, only check the original data portion.
	if !bytes.Equal(got[:len(frameData)], frameData) {
		t.Errorf("round trip with randomization: got %x, want %x",
			got[:len(frameData)], frameData)
	}
}

func TestWrapCLTU_CustomSequences(t *testing.T) {
	customStart := []byte{0xDE, 0xAD}
	customTail := []byte{0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF, 0xBE, 0xEF}
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

	cltu, err := tcsc.WrapCLTU(frameData, customStart, customTail, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(cltu[:2], customStart) {
		t.Errorf("start = %x, want %x", cltu[:2], customStart)
	}
	if !bytes.Equal(cltu[len(cltu)-8:], customTail) {
		t.Errorf("tail = %x, want %x", cltu[len(cltu)-8:], customTail)
	}

	got, _, err := tcsc.UnwrapCLTU(cltu, customStart, customTail, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("round trip: got %x, want %x", got, frameData)
	}
}

func TestWrapCLTU_EmptyData(t *testing.T) {
	_, err := tcsc.WrapCLTU(nil, nil, nil, false)
	if !errors.Is(err, tcsc.ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}

	_, err = tcsc.WrapCLTU([]byte{}, nil, nil, false)
	if !errors.Is(err, tcsc.ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestUnwrapCLTU_TooShort(t *testing.T) {
	_, _, err := tcsc.UnwrapCLTU([]byte{0xEB, 0x90}, nil, nil, false)
	if !errors.Is(err, tcsc.ErrDataTooShort) {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestUnwrapCLTU_BadStartSequence(t *testing.T) {
	// Build a valid-length CLTU with wrong start sequence.
	cltu := make([]byte, 18)
	cltu[0], cltu[1] = 0x00, 0x00
	_, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if !errors.Is(err, tcsc.ErrStartSequenceMismatch) {
		t.Errorf("expected ErrStartSequenceMismatch, got %v", err)
	}
}

func TestUnwrapCLTU_BadTailSequence(t *testing.T) {
	// Build a CLTU with valid start but wrong tail.
	cltu := make([]byte, 18)
	cltu[0], cltu[1] = 0xEB, 0x90
	_, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if !errors.Is(err, tcsc.ErrTailSequenceMismatch) {
		t.Errorf("expected ErrTailSequenceMismatch, got %v", err)
	}
}

func TestUnwrapCLTU_InvalidBodyLength(t *testing.T) {
	// CLTU with body that's not a multiple of 8.
	start := tcsc.DefaultStartSequence()
	tail := tcsc.DefaultTailSequence()
	// 2 (start) + 12 (body, not multiple of 8) + 8 (tail) = 22
	cltu := make([]byte, 22)
	copy(cltu, start)
	copy(cltu[len(cltu)-len(tail):], tail)

	_, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if !errors.Is(err, tcsc.ErrInvalidCLTULength) {
		t.Errorf("expected ErrInvalidCLTULength, got %v", err)
	}
}

func TestUnwrapCLTU_BitErrorCorrection(t *testing.T) {
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt 1 bit in the first codeblock (byte index 2, which is the
	// first byte of codeblock data after the 2-byte start sequence).
	cltu[2] ^= 0x40

	got, corr, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr != 1 {
		t.Errorf("corrections = %d, want 1", corr)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("corrected data: got %x, want %x", got, frameData)
	}
}

func TestWrapUnwrapCLTU_Deterministic(t *testing.T) {
	frameData := []byte("deterministic test command")
	cltu1, _ := tcsc.WrapCLTU(frameData, nil, nil, true)
	cltu2, _ := tcsc.WrapCLTU(frameData, nil, nil, true)
	if !bytes.Equal(cltu1, cltu2) {
		t.Error("same input should produce same CLTU")
	}
}

func TestWrapCLTU_StartSequencePresent(t *testing.T) {
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	start := tcsc.DefaultStartSequence()
	if !bytes.Equal(cltu[:len(start)], start) {
		t.Errorf("CLTU start = %x, want %x", cltu[:len(start)], start)
	}

	tail := tcsc.DefaultTailSequence()
	if !bytes.Equal(cltu[len(cltu)-len(tail):], tail) {
		t.Errorf("CLTU tail = %x, want %x", cltu[len(cltu)-len(tail):], tail)
	}
}
