package pn_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/internal/pn"
)

// TestTMAndTCSequencesDiffer states the fact the package exists to keep
// straight. Both sequences start FF, because both registers are preset to all
// ones; they part company at the second octet and never realign within a
// period. If this ever passes trivially, the two generators have been wired to
// the same taps again.
func TestTMAndTCSequencesDiffer(t *testing.T) {
	tm := pn.TMSequence(pn.Period)
	tc := pn.TCSequence(pn.Period)

	if bytes.Equal(tm, tc) {
		t.Fatal("the TM and TC sequences are identical; one generator has the other's taps")
	}
	if tm[1] == tc[1] {
		t.Errorf("the sequences agree at octet 1: TM %02X, TC %02X", tm[1], tc[1])
	}

	// Neither may be a rotation of the other either, which is what a
	// misapplied preset would look like.
	for shift := 1; shift < pn.Period; shift++ {
		rotated := append(append([]byte{}, tm[shift:]...), tm[:shift]...)
		if bytes.Equal(rotated, tc) {
			t.Fatalf("the TC sequence is the TM sequence rotated by %d octets", shift)
		}
	}
}

// TestPeriodIs255Octets checks the constant the tiling relies on, for both
// generators. Each register is 8 bits with a maximal-length polynomial, so the
// bit sequence repeats after 255 bits; 255 and 8 are coprime, so the octet
// sequence realigns only after 255 octets.
func TestPeriodIs255Octets(t *testing.T) {
	for _, tc := range []struct {
		name string
		seq  func(int) []byte
	}{
		{"TM", pn.TMSequence},
		{"TC", pn.TCSequence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seq := tc.seq(pn.Period * 3)

			for i := range pn.Period * 2 {
				if seq[i] != seq[i+pn.Period] {
					t.Fatalf("octet %d differs from octet %d: %02X vs %02X",
						i, i+pn.Period, seq[i], seq[i+pn.Period])
				}
			}
			// And it must not repeat sooner, or the period constant is wrong.
			for shorter := 1; shorter < pn.Period; shorter++ {
				same := true
				for i := range pn.Period {
					if seq[i] != seq[i+shorter] {
						same = false
						break
					}
				}
				if same {
					t.Fatalf("the sequence repeats every %d octets, not %d", shorter, pn.Period)
				}
			}
		})
	}
}

// lfsr regenerates a sequence the long way, straight from the polynomial
// recurrence b(n+8) = sum of the tapped earlier bits, with no cache and no
// tiling. taps names the register bit positions of a Fibonacci-form register
// whose output is the top bit.
func lfsr(taps []uint, length int) []byte {
	reg := uint8(0xFF)
	out := make([]byte, length)
	for i := range out {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			b |= ((reg >> 7) & 1) << uint(bit)

			var feedback uint8
			for _, tap := range taps {
				feedback ^= (reg >> tap) & 1
			}
			reg = (reg << 1) | (feedback & 1)
		}
		out[i] = b
	}
	return out
}

// TestTilingMatchesDirectGeneration guards the optimisation for both
// generators: a long sequence built by tiling the cached period must equal one
// generated straight through.
func TestTilingMatchesDirectGeneration(t *testing.T) {
	const length = pn.Period*4 + 37

	for _, tc := range []struct {
		name string
		seq  func(int) []byte
		taps []uint // h(x) = x^8 + x^7 + x^5 + x^3 + 1 / + x^6 + x^4 + x^3 + x^2 + x + 1
	}{
		{"TM", pn.TMSequence, []uint{7, 4, 2, 0}},
		{"TC", pn.TCSequence, []uint{7, 6, 5, 4, 3, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tiled := tc.seq(length)
			direct := lfsr(tc.taps, length)

			for i := range direct {
				if tiled[i] != direct[i] {
					t.Fatalf("first difference at octet %d: tiled %02X, direct %02X",
						i, tiled[i], direct[i])
				}
			}
		})
	}
}

func TestApplyIsItsOwnInverse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func([]byte) []byte
	}{
		{"TMApply", pn.TMApply},
		{"TCApply", pn.TCApply},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("housekeeping telemetry, mostly unchanging")
			once := tc.apply(data)
			twice := tc.apply(once)

			if !bytes.Equal(twice, data) {
				t.Errorf("applied twice gave %q, want %q", twice, data)
			}
			if bytes.Equal(once, data) {
				t.Error("apply left the data unchanged")
			}
		})
	}
}

// TestApplyUsesItsOwnSequence checks that the two Apply functions are wired to
// the generators their names promise. Randomizing with one and derandomizing
// with the other must not recover the input, which is exactly the failure a
// conformant peer sees when the wrong randomizer ships.
func TestApplyUsesItsOwnSequence(t *testing.T) {
	data := []byte("telecommand, uplinked once")

	if got := pn.TMApply(pn.TCApply(data)); bytes.Equal(got, data) {
		t.Error("TMApply and TCApply cancel each other; they share a sequence")
	}

	if got := pn.TMApply(data); !bytes.Equal(got, xor(data, pn.TMSequence(len(data)))) {
		t.Error("TMApply does not apply TMSequence")
	}
	if got := pn.TCApply(data); !bytes.Equal(got, xor(data, pn.TCSequence(len(data)))) {
		t.Error("TCApply does not apply TCSequence")
	}
}

func xor(a, b []byte) []byte {
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func TestApplyDoesNotAliasInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func([]byte) []byte
	}{
		{"TMApply", pn.TMApply},
		{"TCApply", pn.TCApply},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{1, 2, 3, 4}
			original := append([]byte(nil), data...)

			out := tc.apply(data)
			out[0] ^= 0xFF

			if !bytes.Equal(data, original) {
				t.Error("apply wrote through to its input")
			}
		})
	}
}

func TestSequenceEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		seq  func(int) []byte
	}{
		{"TMSequence", pn.TMSequence},
		{"TCSequence", pn.TCSequence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.seq(0); got != nil {
				t.Errorf("%s(0) = %v, want nil", tc.name, got)
			}
			if got := tc.seq(-1); got != nil {
				t.Errorf("%s(-1) = %v, want nil", tc.name, got)
			}
			// Both registers are preset to all ones, so both open FF.
			if got := tc.seq(1); len(got) != 1 || got[0] != 0xFF {
				t.Errorf("%s(1) = % X, want FF", tc.name, got)
			}
		})
	}
}

// TestOIDSequenceIsContinuousAcrossFills checks that the generator streams:
// filling two buffers must give the same octets as filling one of the combined
// length. Idle frames are filled one at a time and the sequence may not
// restart between them (132.0-B-3 clause 4.1.4.6.2.1).
func TestOIDSequenceIsContinuousAcrossFills(t *testing.T) {
	whole := make([]byte, 32)
	pn.NewOIDSequence().Fill(whole)

	split := make([]byte, 32)
	s := pn.NewOIDSequence()
	s.Fill(split[:7])
	s.Fill(split[7:20])
	s.Fill(split[20:])

	if !bytes.Equal(whole, split) {
		t.Errorf("split fills = % X, want % X", split, whole)
	}
}
