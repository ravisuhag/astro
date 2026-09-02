package rhc

import (
	"fmt"
	"math/bits"
)

// The counter and run-length encoding functions of CCSDS 124.0-B-1 clause 5.2.

// MaxCount is the largest integer COUNT accepts, per clause 5.2.2.
const MaxCount = 1<<16 - 1

// AppendCount writes COUNT(a), the counter encoding function of clause 5.2.2:
//
//	A = 1          '0'
//	2 <= A <= 33   '110' || BIT5(A-2)
//	A >= 34        '111' || BITE(A-2)
//
// with E from equation 9. The three-way split is a prefix code, which is what
// lets the decoder tell the codewords apart without knowing how many there
// are: a leading '0' ends immediately, and a leading '11' promises more.
func AppendCount(w *BitWriter, a int) error {
	if a < 1 || a > MaxCount {
		return fmt.Errorf("%w: %d is outside 1 to %d", ErrInvalidCount, a, MaxCount)
	}

	switch {
	case a == 1:
		w.WriteString("0")
	case a <= 33:
		w.WriteString("110")
		w.WriteBits(uint64(a-2), 5)
	default:
		w.WriteString("111")
		w.WriteBits(uint64(a-2), countWidth(a))
	}
	return nil
}

// countWidth returns E, the width of the long form's payload, per equation 9:
//
//	E = 2*floor(log2(A-2) + 1) - 6
//
// The note under the equation explains what that buys: E is wider than the
// minimum needed for A-2, and the extra width appears as leading zeros. Since
// E = 2m - 6 where m is the minimal width, the number of leading zeros is
// m - 6, which grows one for one with the width. So the decoder counts
// leading zeros to learn the width, with no length field.
func countWidth(a int) int {
	value := a - 2
	minimal := bits.Len(uint(value))
	return 2*minimal - 6
}

// ReadCount reads one COUNT codeword.
//
// It also reports whether the codeword was the '10' that clause 5.2.3 uses to
// terminate a run-length encoding. That marker shares its first bit with the
// long form, so the two can only be told apart here, one bit deeper.
func ReadCount(r *BitReader) (a int, terminator bool, err error) {
	first, err := r.ReadBit()
	if err != nil {
		return 0, false, err
	}
	if !first {
		return 1, false, nil
	}

	second, err := r.ReadBit()
	if err != nil {
		return 0, false, err
	}
	if !second {
		// '10' is the run-length terminator of clause 5.2.3, not a counter value.
		return 0, true, nil
	}

	third, err := r.ReadBit()
	if err != nil {
		return 0, false, err
	}
	if !third {
		// '110' || BIT5(A-2)
		v, err := r.ReadBits(5)
		if err != nil {
			return 0, false, err
		}
		return int(v) + 2, false, nil
	}

	// '111' || BITE(A-2). Count the leading zeros to learn E: each one widens
	// the payload by one bit past the six-bit minimum, so z zeros mean a
	// minimal width of z+6 and E = 2z+6.
	zeros := 0
	for {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, false, err
		}
		if bit {
			break
		}
		zeros++
		// A-2 is at most 65533, whose minimal width is 16, so at most ten
		// leading zeros can be legal.
		if zeros > 10 {
			return 0, false, fmt.Errorf("%w: %d leading zeros in the long form", ErrInvalidCount, zeros)
		}
	}

	// The one just read is the top bit of the minimal encoding; the rest
	// follow.
	minimal := zeros + 6
	rest, err := r.ReadBits(minimal - 1)
	if err != nil {
		return 0, false, err
	}

	value := int(uint64(1)<<uint(minimal-1) | rest)
	a = value + 2
	if a > MaxCount {
		return 0, false, fmt.Errorf("%w: decoded %d", ErrInvalidCount, a)
	}
	return a, false, nil
}

// AppendRLE writes RLE(v), the run-length encoding of clause 5.2.3:
//
//	RLE(a) = COUNT(C0) || ... || COUNT(C_{H(a)-1}) || '10'
//
// where C_i is one more than the number of zeros before the ith one bit,
// counting from the first transmitted bit.
//
// Trailing zeros are not encoded. Note 1 of clause 5.2.3 says why they need not be:
// the decoder knows the vector's length, so whatever is left after the last
// one bit must be zeros.
func AppendRLE(w *BitWriter, v Vector) error {
	previous := -1
	for i := range v.Len() {
		if !v.Get(i) {
			continue
		}
		if err := AppendCount(w, i-previous); err != nil {
			return err
		}
		previous = i
	}
	// Clause 5.2.3 note 2: a vector with no one bits encodes as just the terminator.
	w.WriteString("10")
	return nil
}

// ReadRLE reads a run-length encoding back into a vector of the given length.
func ReadRLE(r *BitReader, length int) (Vector, error) {
	v := NewVector(length)

	position := -1
	for {
		count, terminator, err := ReadCount(r)
		if err != nil {
			return Vector{}, err
		}
		if terminator {
			return v, nil
		}

		position += count
		if position >= length {
			return Vector{}, fmt.Errorf("%w: position %d in a %d-bit vector",
				ErrInvalidRunLength, position, length)
		}
		v.Set(position, true)
	}
}
