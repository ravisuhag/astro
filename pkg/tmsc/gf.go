package tmsc

// Galois Field GF(2^8) arithmetic for CCSDS Reed-Solomon.
//
// Field polynomial: x^8 + x^7 + x^2 + x + 1 (0x187)
// per CCSDS 131.0-B-4.

const fieldPoly = 0x187

// Lookup tables for GF(2^8) arithmetic.
// gfExp is doubled to 512 entries to avoid modular reduction after log addition.
var (
	gfExp [512]byte
	gfLog [256]byte
)

func init() {
	// Build exp table: gfExp[i] = alpha^i
	x := 1
	for i := range 255 {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x >= 256 {
			x ^= fieldPoly
		}
	}
	// Duplicate for wraparound-free indexing
	for i := range 512 {
		if i >= 255 {
			gfExp[i] = gfExp[i-255]
		}
	}
}

// gfMul returns a * b in GF(2^8).
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// gfInv returns the multiplicative inverse of a in GF(2^8).
// Panics if a == 0.
func gfInv(a byte) byte {
	if a == 0 {
		panic("gfInv(0): division by zero in GF(2^8)")
	}
	return gfExp[255-int(gfLog[a])]
}

// gfPow returns alpha^n in GF(2^8).
func gfPow(n int) byte {
	n %= 255
	if n < 0 {
		n += 255
	}
	return gfExp[n]
}
