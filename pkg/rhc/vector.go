package rhc

// Binary vectors, as CCSDS 124.0-B-1 §1.6.1 defines them.
//
// The standard works entirely in fixed-length binary vectors and a handful of
// operations on them: XOR, OR, AND, inversion, left shift, bit reversal,
// Hamming weight, and bit extraction. Every one is a few lines, and having
// them named the way the spec names them is what lets the encoder read like
// the equations it implements.
//
// Indexing needs care. §1.6.1 numbers the first transmitted bit N-1 and counts
// down to 0, so the spec's subscript is a position from the *end*. This type
// indexes from the front instead — index 0 is the first transmitted bit, which
// the spec calls bit N-1 — because every operation here walks the vector in
// transmission order. Where a spec equation refers to a bit by its own
// numbering, the comment says so.

// Vector is a fixed-length binary vector.
type Vector struct {
	// bits holds one bit per entry. A bitset would be denser, but these
	// vectors are at most 65535 bits and every operation here is a full sweep,
	// so the packing would buy nothing and cost clarity in the shift and
	// reversal.
	bits []bool
}

// NewVector returns an all-zero vector of length n.
func NewVector(n int) Vector {
	return Vector{bits: make([]bool, n)}
}

// VectorFromBytes reads the first n bits of data, MSB first, into a vector.
func VectorFromBytes(data []byte, n int) Vector {
	v := NewVector(n)
	for i := range n {
		if i/8 < len(data) {
			v.bits[i] = data[i/8]>>(7-uint(i%8))&1 == 1
		}
	}
	return v
}

// VectorFromString reads a literal such as "10110", which is how the spec
// writes example vectors.
func VectorFromString(bits string) Vector {
	v := NewVector(len(bits))
	for i, c := range bits {
		v.bits[i] = c == '1'
	}
	return v
}

// Len returns the vector's length in bits.
func (v Vector) Len() int { return len(v.bits) }

// Get returns the bit at index i, counting from the first transmitted bit.
func (v Vector) Get(i int) bool { return v.bits[i] }

// Set writes the bit at index i.
func (v Vector) Set(i int, bit bool) { v.bits[i] = bit }

// Clone returns an independent copy.
func (v Vector) Clone() Vector {
	out := NewVector(len(v.bits))
	copy(out.bits, v.bits)
	return out
}

// Bytes packs the vector MSB first, zero padding the last octet.
func (v Vector) Bytes() []byte {
	out := make([]byte, (len(v.bits)+7)/8)
	for i, bit := range v.bits {
		if bit {
			out[i/8] |= 1 << (7 - uint(i%8))
		}
	}
	return out
}

// String renders the vector as a bit string, which is how the spec prints
// them.
func (v Vector) String() string {
	out := make([]byte, len(v.bits))
	for i, bit := range v.bits {
		out[i] = '0'
		if bit {
			out[i] = '1'
		}
	}
	return string(out)
}

// XOR returns the exclusive or of two vectors of equal length, per §1.6.1
// equation 3.
func (v Vector) XOR(other Vector) Vector {
	out := NewVector(len(v.bits))
	for i := range v.bits {
		out.bits[i] = v.bits[i] != other.bits[i]
	}
	return out
}

// OR returns the logical disjunction, per equation 2.
func (v Vector) OR(other Vector) Vector {
	out := NewVector(len(v.bits))
	for i := range v.bits {
		out.bits[i] = v.bits[i] || other.bits[i]
	}
	return out
}

// AND returns the logical conjunction, per equation 4.
func (v Vector) AND(other Vector) Vector {
	out := NewVector(len(v.bits))
	for i := range v.bits {
		out.bits[i] = v.bits[i] && other.bits[i]
	}
	return out
}

// Not returns the bit-wise inverse, which §1.6.1 writes ~a.
func (v Vector) Not() Vector {
	out := NewVector(len(v.bits))
	for i := range v.bits {
		out.bits[i] = !v.bits[i]
	}
	return out
}

// ShiftLeft returns the left bit-shift, which §1.6.1 writes a<< and equation 1
// defines: every bit moves one place towards the first transmitted bit, and a
// zero enters at the end.
func (v Vector) ShiftLeft() Vector {
	out := NewVector(len(v.bits))
	for i := range len(v.bits) - 1 {
		out.bits[i] = v.bits[i+1]
	}
	return out
}

// Reverse returns the vector with its bits in the opposite order, which
// §1.6.1 writes <a>.
func (v Vector) Reverse() Vector {
	out := NewVector(len(v.bits))
	for i, bit := range v.bits {
		out.bits[len(v.bits)-1-i] = bit
	}
	return out
}

// Weight returns the Hamming weight, which §1.6.1 writes H(a): the number of
// one bits.
func (v Vector) Weight() int {
	count := 0
	for _, bit := range v.bits {
		if bit {
			count++
		}
	}
	return count
}

// IsZero reports whether every bit is zero, which the spec writes a = 0.
func (v Vector) IsZero() bool { return v.Weight() == 0 }

// Extract returns the bit extraction of v relative to selector, which §5.2.4
// writes BE(a, b): the bits of v at the positions where selector has a one,
// taken from the first transmitted bit onwards.
//
// Equation 11 writes the result as ȧ_{g(H-1)} || ... || ȧ_{g0}, which reads
// backwards only because the spec's subscripts count up from the last bit. In
// transmission order it is simply "the selected bits, in order".
func (v Vector) Extract(selector Vector) []bool {
	out := make([]bool, 0, selector.Weight())
	for i := range v.bits {
		if selector.bits[i] {
			out = append(out, v.bits[i])
		}
	}
	return out
}

// allZero reports whether every bit of a bit slice is false.
func allZero(bits []bool) bool {
	for _, bit := range bits {
		if bit {
			return false
		}
	}
	return true
}
