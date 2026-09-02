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
	// Result includes padding, first 5 bytes should match.
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

func TestUnwrapCLTU_ToleratesTailBitErrors(t *testing.T) {
	// Per CCSDS 231.0-B-4, the receiver terminates on the first codeblock
	// that fails to decode. The tail sequence is built to fail BCH
	// decoding, so a tail with bit errors must still end the CLTU.
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt one bit of the tail sequence (last 8 bytes). The tail is
	// designed so that any single-bit error still fails BCH decoding.
	cltu[len(cltu)-8] ^= 0x01

	got, corr, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr != 0 {
		t.Errorf("corrections = %d, want 0", corr)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("data = %x, want %x", got, frameData)
	}
}

func TestUnwrapCLTU_ToleratesTrailingOctets(t *testing.T) {
	// Extra octets after the tail (for example idle sequence from the
	// physical channel) must not break unwrapping.
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	cltu = append(cltu, 0x55, 0x55, 0x55)

	got, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, frameData) {
		t.Errorf("data = %x, want %x", got, frameData)
	}
}

func TestWrapCLTU_RandomizesFillOctets(t *testing.T) {
	// TCSC randomization covers everything between start and tail
	// sequences: the 0x55 fill must be added FIRST, then randomized.
	frameData := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE} // 5 bytes -> 2 fill bytes
	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	// The single codeblock's info bytes sit right after the 2-byte start
	// sequence. Positions 5 and 6 hold the fill octets: on the wire they
	// must be 0x55 XOR the PN sequence, not raw 0x55.
	pn := tcsc.GeneratePNSequence(7)
	info := cltu[2 : 2+7]
	for _, i := range []int{5, 6} {
		want := 0x55 ^ pn[i]
		if info[i] != want {
			t.Errorf("fill octet %d on the wire = 0x%02X, want 0x%02X (randomized)", i, info[i], want)
		}
	}

	// And the round trip recovers the original data with 0x55 padding.
	got, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:len(frameData)], frameData) {
		t.Errorf("data = %x, want %x", got[:len(frameData)], frameData)
	}
	for i := len(frameData); i < len(got); i++ {
		if got[i] != 0x55 {
			t.Errorf("recovered fill octet %d = 0x%02X, want 0x55", i, got[i])
		}
	}
}

func TestUplinkSequence_PLOP1AndPLOP2(t *testing.T) {
	frame := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cltu, err := tcsc.WrapCLTU(frame, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	acq := tcsc.AcquisitionSequence(0)
	if len(acq) != tcsc.DefaultAcquisitionOctets {
		t.Fatalf("acquisition length = %d, want %d", len(acq), tcsc.DefaultAcquisitionOctets)
	}
	for _, b := range acq {
		if b != 0x55 {
			t.Fatalf("acquisition octet = 0x%02X, want 0x55", b)
		}
	}
	idle := tcsc.IdleSequence(0)
	if len(idle) != tcsc.DefaultIdleOctets {
		t.Fatalf("idle length = %d, want %d", len(idle), tcsc.DefaultIdleOctets)
	}

	// PLOP-1: acquisition before every CLTU.
	stream1, err := tcsc.UplinkSequence(tcsc.PLOP1, [][]byte{cltu, cltu}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want1 := 2 * (len(acq) + len(cltu))
	if len(stream1) != want1 {
		t.Errorf("PLOP-1 stream length = %d, want %d", len(stream1), want1)
	}
	if !bytes.Equal(stream1[:len(acq)], acq) {
		t.Error("PLOP-1 stream must begin with the acquisition sequence")
	}

	// PLOP-2: one acquisition, idle between CLTUs.
	stream2, err := tcsc.UplinkSequence(tcsc.PLOP2, [][]byte{cltu, cltu}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want2 := len(acq) + len(cltu) + len(idle) + len(cltu)
	if len(stream2) != want2 {
		t.Errorf("PLOP-2 stream length = %d, want %d", len(stream2), want2)
	}

	if _, err := tcsc.UplinkSequence(tcsc.PLOP2, nil, 0, 0); !errors.Is(err, tcsc.ErrEmptyData) {
		t.Errorf("expected ErrEmptyData for no CLTUs, got %v", err)
	}
	if _, err := tcsc.UplinkSequence(tcsc.PLOP(9), [][]byte{cltu}, 0, 0); !errors.Is(err, tcsc.ErrInvalidPLOP) {
		t.Errorf("expected ErrInvalidPLOP, got %v", err)
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

func TestPNSequenceMatchesTheCCSDSVector(t *testing.T) {
	// TC does not share the TM randomizer. CCSDS 231.0-B-4 clause 6.2 specifies
	// h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1 with the register preset to
	// all ones, and prints the first 40 digits of the sequence:
	//
	//   1111 1111 0011 1001 1001 1110 0101 1010 0110 1000
	//
	// The TM sequence of CCSDS 131.0-B-5 clause 10.4.2 opens FF 48 0E C0 9A
	// instead. This package shipped that one for a while, and nothing caught
	// it: a randomize-then-derandomize round trip cannot, because XOR is
	// self-inverse whatever the sequence. Only the published digits can.
	want := []byte{0xFF, 0x39, 0x9E, 0x5A, 0x68}

	got := tcsc.GeneratePNSequence(len(want))
	if !bytes.Equal(got, want) {
		t.Errorf("PN sequence = % X, want % X", got, want)
	}
}

// TestWrapCLTU_RandomizedOctetsOnTheWire pins what a conformant receiver
// actually sees, end to end. The frame is exactly one codeblock's worth of
// information octets, so nothing is padded and the randomized frame lands
// whole in the codeblock, right after the 2-octet start sequence EB 90.
//
// The expected octets are 01..07 XOR the first seven octets of the
// CCSDS 231.0-B-4 clause 6.2 sequence, FF 39 9E 5A 68 E9 06. Under the TM
// randomizer this test would read FE 4A 0D C4 9F 0B 77, self-consistent,
// round-trippable, and unreadable on the far end.
func TestWrapCLTU_RandomizedOctetsOnTheWire(t *testing.T) {
	frameData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

	cltu, err := tcsc.WrapCLTU(frameData, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0xFE, 0x3B, 0x9D, 0x5E, 0x6D, 0xEF, 0x01}
	got := cltu[2 : 2+tcsc.InfoBytes]
	if !bytes.Equal(got, want) {
		t.Errorf("information octets on the wire = % X, want % X", got, want)
	}
}
