package tmsc

import (
	"errors"
	"testing"
)

// TestForneyRejectsZeroSigmaDerivative pins the fix for the silent-skip bug:
// when the formal derivative σ'(X^-1) evaluates to zero at a claimed error
// position, the magnitude is undefined and the decode must fail loudly
// rather than skip the position and report success.
func TestForneyRejectsZeroSigmaDerivative(t *testing.T) {
	rs := NewRS255_223()

	// σ(x) = 1 + x^2. In characteristic 2 the formal derivative keeps only
	// the odd-degree coefficients, all of which are zero here, so σ'
	// evaluates to zero at every point, including any claimed position.
	sigma := []byte{1, 0, 1}
	syndromes := make([]byte, rs.nroots)
	codeword := make([]byte, rsNN)

	err := rs.forney(codeword, syndromes, sigma, []int{0})
	if !errors.Is(err, ErrUncorrectable) {
		t.Fatalf("forney with σ'(X^-1) = 0 returned %v, want ErrUncorrectable", err)
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

// referenceSyndromes is the syndrome computation written the obvious way:
// one pass over the codeword per root, evaluating Horner's method with the
// general-purpose gfMul.
//
// It is slow and plainly correct, which is what makes it a useful reference.
// The real syndromes method interleaves the roots so the processor has
// several dependency chains to overlap, which is worth about 3.7x on a clean
// codeword, and is exactly the sort of rewrite that can be self-consistent
// and wrong. A wrong syndrome does not fail loudly: it makes a good codeword
// look corrupt, or worse, makes a corrupt one decode to the wrong data.
func (rs *RSCodec) referenceSyndromes(work []byte) ([]byte, bool) {
	syndromes := make([]byte, rs.nroots)
	allZero := true
	for i := range rs.nroots {
		s := byte(0)
		for j := range work {
			s = gfMul(s, gfPowB(rsFCR+i)) ^ work[j]
		}
		syndromes[i] = s
		if s != 0 {
			allZero = false
		}
	}
	return syndromes, allZero
}

// TestSyndromesMatchReference checks the interleaved computation against the
// straightforward one, on clean codewords and on every shape of corruption
// the decoder is meant to handle.
func TestSyndromesMatchReference(t *testing.T) {
	for _, rs := range []*RSCodec{NewRS255_223(), NewRS255_239()} {
		// A deterministic message, so a failure reproduces without a seed.
		message := make([]byte, rs.DataLen())
		for i := range message {
			message[i] = byte(i*31 + 7)
		}

		clean, err := rs.Encode(message)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}

		cases := map[string][]byte{"clean": clean}

		// One error, at the front, the middle and the end.
		for _, at := range []int{0, len(clean) / 2, len(clean) - 1} {
			corrupted := make([]byte, len(clean))
			copy(corrupted, clean)
			corrupted[at] ^= 0xFF
			cases["one error at "+itoa(at)] = corrupted
		}

		// Errors up to the correction limit, which is nroots/2.
		for _, count := range []int{2, rs.nroots / 2, rs.nroots} {
			corrupted := make([]byte, len(clean))
			copy(corrupted, clean)
			for e := 0; e < count; e++ {
				corrupted[e*7%len(corrupted)] ^= byte(0xA5 + e)
			}
			cases[itoa(count)+" errors"] = corrupted
		}

		// All zeros and all ones, the two patterns most likely to expose a
		// mishandled zero accumulator.
		cases["all zeros"] = make([]byte, rsNN)
		ones := make([]byte, rsNN)
		for i := range ones {
			ones[i] = 0xFF
		}
		cases["all ones"] = ones

		for name, codeword := range cases {
			gotSyndromes, gotZero := rs.syndromes(codeword)
			wantSyndromes, wantZero := rs.referenceSyndromes(codeword)

			if gotZero != wantZero {
				t.Errorf("RS(255,%d) %s: allZero = %v, the reference says %v",
					rs.DataLen(), name, gotZero, wantZero)
			}
			if len(gotSyndromes) != len(wantSyndromes) {
				t.Fatalf("RS(255,%d) %s: got %d syndromes, want %d",
					rs.DataLen(), name, len(gotSyndromes), len(wantSyndromes))
			}
			for i := range wantSyndromes {
				if gotSyndromes[i] != wantSyndromes[i] {
					t.Errorf("RS(255,%d) %s: syndrome %d = 0x%02X, the reference gives 0x%02X",
						rs.DataLen(), name, i, gotSyndromes[i], wantSyndromes[i])
				}
			}
		}
	}
}

// TestSyndromesMatchReferenceOverManyCodewords sweeps a large number of
// distinct messages and corruption patterns, because a syndrome bug could
// easily depend on the data rather than the shape of the error.
func TestSyndromesMatchReferenceOverManyCodewords(t *testing.T) {
	rs := NewRS255_223()

	// A simple deterministic generator: enough variety to be convincing,
	// reproducible without a seed.
	state := uint32(12345)
	next := func() byte {
		state = state*1664525 + 1013904223
		return byte(state >> 24)
	}

	for round := 0; round < 200; round++ {
		message := make([]byte, rs.DataLen())
		for i := range message {
			message[i] = next()
		}

		codeword, err := rs.Encode(message)
		if err != nil {
			t.Fatalf("round %d: Encode: %v", round, err)
		}

		// Corrupt a data-dependent number of positions, sometimes none.
		errorCount := int(next()) % (rs.nroots + 1)
		for e := 0; e < errorCount; e++ {
			position := int(next()) % len(codeword)
			codeword[position] ^= next()
		}

		gotSyndromes, gotZero := rs.syndromes(codeword)
		wantSyndromes, wantZero := rs.referenceSyndromes(codeword)

		if gotZero != wantZero {
			t.Fatalf("round %d: allZero = %v, the reference says %v", round, gotZero, wantZero)
		}
		for i := range wantSyndromes {
			if gotSyndromes[i] != wantSyndromes[i] {
				t.Fatalf("round %d: syndrome %d = 0x%02X, the reference gives 0x%02X",
					round, i, gotSyndromes[i], wantSyndromes[i])
			}
		}
	}
}

// itoa keeps the test names readable without pulling in strconv for one use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
