package tmsc

import (
	"errors"
	"testing"
)

// TestForneyRejectsZeroSigmaDerivative pins the fix for the silent-skip bug:
// when the formal derivative σ'(X⁻¹) evaluates to zero at a claimed error
// position, the magnitude is undefined and the decode must fail loudly
// rather than skip the position and report success.
func TestForneyRejectsZeroSigmaDerivative(t *testing.T) {
	rs := NewRS255_223()

	// σ(x) = 1 + x². In characteristic 2 the formal derivative keeps only
	// the odd-degree coefficients, all of which are zero here, so σ'
	// evaluates to zero at every point — including any claimed position.
	sigma := []byte{1, 0, 1}
	syndromes := make([]byte, rs.nroots)
	codeword := make([]byte, rsNN)

	err := rs.forney(codeword, syndromes, sigma, []int{0})
	if !errors.Is(err, ErrUncorrectable) {
		t.Fatalf("forney with σ'(X⁻¹) = 0 returned %v, want ErrUncorrectable", err)
	}
}

// TestSyndromesDetectCorruption covers the post-correction recheck helper:
// a clean codeword has all-zero syndromes, and any single corrupted symbol
// makes at least one syndrome nonzero.
func TestSyndromesDetectCorruption(t *testing.T) {
	rs := NewRS255_239()

	data := make([]byte, rs.DataLen())
	for i := range data {
		data[i] = byte(i * 11)
	}
	cw, err := rs.Encode(data)
	if err != nil {
		t.Fatal(err)
	}

	work := make([]byte, rsNN)
	copy(work, cw)
	toConventional(work)
	if _, allZero := rs.syndromes(work); !allZero {
		t.Fatal("valid codeword must have all-zero syndromes")
	}

	work[42] ^= 0x01
	if _, allZero := rs.syndromes(work); allZero {
		t.Fatal("corrupted codeword must have a nonzero syndrome")
	}
}
