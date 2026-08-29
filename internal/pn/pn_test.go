package pn_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/internal/pn"
)

// TestSequenceMatchesTheCCSDSVector is the only test here that can catch wrong
// taps. CCSDS 131.0-B specifies h(x) = x^8 + x^7 + x^5 + x^3 + 1 preset to all
// ones, and CCSDS 142.0-B-1 §3.5.2.1, which adopts the same sequence,
// publishes the first 40 digits:
//
//	1111 1111 0100 1000 0000 1110 1100 0000 1001 1010
//
// as octets: FF 48 0E C0 9A.
//
// A round trip cannot substitute for this. The randomizer is XOR, so it is its
// own inverse and any sequence at all round-trips perfectly.
func TestSequenceMatchesTheCCSDSVector(t *testing.T) {
	want := []byte{0xFF, 0x48, 0x0E, 0xC0, 0x9A}
	if got := pn.Sequence(len(want)); !bytes.Equal(got, want) {
		t.Errorf("sequence = % X, want % X", got, want)
	}
}

// TestPeriodIs255Octets checks the constant the tiling relies on. The register
// is 8 bits with a maximal-length polynomial, so the bit sequence repeats after
// 255 bits; 255 and 8 are coprime, so the octet sequence realigns only after
// 255 octets.
func TestPeriodIs255Octets(t *testing.T) {
	seq := pn.Sequence(pn.Period * 3)

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
}

// TestTilingMatchesDirectGeneration guards the optimisation: a long sequence
// built by tiling the period must equal one generated straight through.
func TestTilingMatchesDirectGeneration(t *testing.T) {
	const length = pn.Period*4 + 37
	tiled := pn.Sequence(length)

	// Regenerate the long way, without the cache.
	direct := make([]byte, length)
	reg := uint8(0xFF)
	for i := range direct {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			b |= ((reg >> 7) & 1) << uint(bit)
			feedback := ((reg >> 7) ^ (reg >> 4) ^ (reg >> 2) ^ reg) & 1
			reg = ((reg << 1) | feedback) & 0xFF
		}
		direct[i] = b
	}

	if !bytes.Equal(tiled, direct) {
		for i := range direct {
			if tiled[i] != direct[i] {
				t.Fatalf("first difference at octet %d: tiled %02X, direct %02X", i, tiled[i], direct[i])
			}
		}
	}
}

func TestApplyIsItsOwnInverse(t *testing.T) {
	data := []byte("housekeeping telemetry, mostly unchanging")
	once := pn.Apply(data)
	twice := pn.Apply(once)

	if !bytes.Equal(twice, data) {
		t.Errorf("Apply twice gave %q, want %q", twice, data)
	}
	if bytes.Equal(once, data) {
		t.Error("Apply left the data unchanged")
	}
}

func TestApplyDoesNotAliasInput(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	original := append([]byte(nil), data...)

	out := pn.Apply(data)
	out[0] ^= 0xFF

	if !bytes.Equal(data, original) {
		t.Error("Apply wrote through to its input")
	}
}

func TestSequenceEdgeCases(t *testing.T) {
	if got := pn.Sequence(0); got != nil {
		t.Errorf("Sequence(0) = %v, want nil", got)
	}
	if got := pn.Sequence(-1); got != nil {
		t.Errorf("Sequence(-1) = %v, want nil", got)
	}
	if got := pn.Sequence(1); len(got) != 1 || got[0] != 0xFF {
		t.Errorf("Sequence(1) = % X, want FF", got)
	}
}

// TestOIDSequenceMatchesTheCCSDSVector pins the 32-cell OID generator to the
// octets CCSDS publishes for it (132.0-B-3 §4.1.4.6.2.2 note, 732.1-B-3 annex
// H). As with the 8-bit randomizer above, a permuted set of taps still yields
// a plausible maximal-length sequence that no round-trip test can catch; only
// the published digits distinguish right from wrong.
func TestOIDSequenceMatchesTheCCSDSVector(t *testing.T) {
	want := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x6D, 0xB6, 0xD8, 0x61, 0x45, 0x1F}
	got := make([]byte, len(want))
	pn.NewOIDSequence().Fill(got)
	if !bytes.Equal(got, want) {
		t.Errorf("OID sequence = % X, want % X", got, want)
	}
}

// TestOIDSequenceIsContinuousAcrossFills checks that the generator streams:
// filling two buffers must give the same octets as filling one of the combined
// length. Idle frames are filled one at a time and the sequence may not
// restart between them (132.0-B-3 §4.1.4.6.2.1).
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
