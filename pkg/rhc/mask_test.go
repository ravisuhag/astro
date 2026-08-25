package rhc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// TestMaskTracksWithoutLoss is the invariant everything else rests on: with no
// loss, the decompressor's mask equals the compressor's after every cycle.
//
// It is worth checking separately from the round trip because a mask can drift
// for a long time before it changes any reconstructed byte — the drift only
// shows when a position it got wrong finally changes. This catches it at the
// cycle it happens.
func TestMaskTracksWithoutLoss(t *testing.T) {
	configs := []rhc.Config{
		{VectorLength: 128, Robustness: 0, SendMaskInterval: 16, UncompressedInterval: 16, NewMaskInterval: 24},
		{VectorLength: 128, Robustness: 3, NewMaskInterval: 8},
		{VectorLength: 64, Robustness: 7, NewMaskInterval: 3, SendMaskInterval: 5},
		{VectorLength: 33, Robustness: 2},
	}

	for _, config := range configs {
		packets := housekeeping(200, config.VectorLength, 2, 5)

		compressor, err := rhc.NewCompressor(config)
		if err != nil {
			t.Fatal(err)
		}
		decompressor, err := rhc.NewDecompressor(config)
		if err != nil {
			t.Fatal(err)
		}

		for i, packet := range packets {
			data, bitLen, err := compressor.Compress(packet)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decompressor.Decompress(data, bitLen); err != nil {
				t.Fatalf("packet %d: %v", i, err)
			}
			if got, want := decompressor.Mask().String(), compressor.Mask().String(); got != want {
				t.Fatalf("F=%d R=%d: mask diverged at packet %d\n comp %s\n dec  %s",
					config.VectorLength, config.Robustness, i, want, got)
			}
		}
	}
}

// TestMaskUpdateEquations walks the equations of §4.2 by hand on a tiny
// stream, so a failure names the equation rather than the stream.
//
// F = 8, M_0 = 0, no new mask flag. Inputs chosen so each step changes a
// different bit:
//
//	t=0  I = 00000000   M_0 = 00000000 (given)          D_0 = 0 (eq 8)
//	t=1  I = 10000000   I XOR I_prev = 10000000
//	                    M_1 = 10000000 (eq 7)           D_1 = 10000000
//	t=2  I = 10000001   I XOR I_prev = 00000001
//	                    M_2 = 10000001 (eq 7, cumulative)
//	t=3  I = 10000001   I XOR I_prev = 00000000
//	                    M_3 = 10000001 (unchanged)      D_3 = 0
func TestMaskUpdateEquations(t *testing.T) {
	config := rhc.Config{VectorLength: 8, Robustness: 0}
	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		input    byte
		wantMask string
	}{
		{0x00, "00000000"},
		{0x80, "10000000"},
		{0x81, "10000001"},
		{0x81, "10000001"},
	}

	for i, step := range steps {
		if _, _, err := compressor.Compress([]byte{step.input}); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if got := compressor.Mask().String(); got != step.wantMask {
			t.Errorf("after input %d (%08b): mask = %s, want %s",
				i, step.input, got, step.wantMask)
		}
	}
}

// TestNewMaskFlagReturnsPositionsToPredictable pins equations 6 and 7: the
// mask only ever grows until the new mask flag fires, and then it is replaced
// by the build — which holds only what changed since the last time it fired.
func TestNewMaskFlagReturnsPositionsToPredictable(t *testing.T) {
	config := rhc.Config{VectorLength: 8, Robustness: 0}
	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatal(err)
	}

	// Change bit 0, then bit 7, so both are unpredictable.
	for _, input := range []byte{0x00, 0x80, 0x81} {
		if _, _, err := compressor.Compress([]byte{input}); err != nil {
			t.Fatal(err)
		}
	}
	if got := compressor.Mask().String(); got != "10000001" {
		t.Fatalf("mask = %s, want 10000001", got)
	}

	// One new mask flag is not enough. Equation 7 replaces the mask with the
	// build, and the build has been accumulating the same changes all along
	// (equation 6), so the first flag swaps in a copy of what was already
	// there. What the first flag really does is reset the build.
	compressor.ForceNewMask()
	if _, _, err := compressor.Compress([]byte{0x81}); err != nil {
		t.Fatal(err)
	}
	if got := compressor.Mask().String(); got != "10000001" {
		t.Errorf("after one new mask flag, mask = %s; the build held the same bits", got)
	}

	// Now the build is empty. Hold steady so it stays empty, then ask again:
	// this time the mask really does clear. §2.1 describes the two-step
	// nature of it — positions move to predictable "only on the cycle when
	// the new mask is requested", and only if they have been quiet since the
	// previous request.
	for range 2 {
		if _, _, err := compressor.Compress([]byte{0x81}); err != nil {
			t.Fatal(err)
		}
	}
	compressor.ForceNewMask()
	if _, _, err := compressor.Compress([]byte{0x81}); err != nil {
		t.Fatal(err)
	}

	if got := compressor.Mask().String(); got != "00000000" {
		t.Errorf("after the second new mask flag with no changes, mask = %s, want all predictable", got)
	}
}

// TestVectorOperations pins the notation of §1.6.1, whose examples are given
// in the text: for a = '10111', a<< = '01110', ~a = '01000', <a> = '11101'.
func TestVectorOperations(t *testing.T) {
	a := rhc.VectorFromString("10111")

	if got := a.ShiftLeft().String(); got != "01110" {
		t.Errorf("a<< = %s, want 01110", got)
	}
	if got := a.Not().String(); got != "01000" {
		t.Errorf("~a = %s, want 01000", got)
	}
	if got := a.Reverse().String(); got != "11101" {
		t.Errorf("<a> = %s, want 11101", got)
	}
	if got := a.Weight(); got != 4 {
		t.Errorf("H(a) = %d, want 4", got)
	}
}

// TestBitExtraction pins §5.2.4: BE(a, b) is the bits of a at the positions
// where b has a one, emitted last selected position first. Equation 11 writes
// BE(a, b) = ȧ_{g(H-1)} || ... || ȧ_{g0} with g_0 the first '1' of b from the
// MSB, and equation 1's a« example fixes that the first term of such a
// concatenation is the first transmitted bit — so the forward scan comes out
// reversed.
//
// Here b selects a's first two positions, holding '1' then '0'; BE emits the
// later position first, so the extraction is '0' then '1'.
func TestBitExtraction(t *testing.T) {
	a := rhc.VectorFromString("10110")
	b := rhc.VectorFromString("11000")

	got := a.Extract(b)
	want := []bool{false, true}
	if len(got) != len(want) {
		t.Fatalf("BE gave %d bits, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BE bit %d = %v, want %v", i, got[i], want[i])
		}
	}
}
