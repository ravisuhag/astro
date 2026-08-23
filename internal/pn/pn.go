// Package pn generates the CCSDS pseudo-randomizer sequence shared by the TM
// and TC synchronization and channel coding layers.
//
// CCSDS 131.0-B (TM) and CCSDS 231.0-B (TC) specify the same randomizer:
// an 8-bit linear feedback shift register realising
//
//	h(x) = x^8 + x^7 + x^5 + x^3 + 1
//
// preset to all ones. pkg/tmsc and pkg/tcsc both need it, and both used to
// carry their own byte-for-byte copy. One copy means one place for the taps to
// be wrong — and they were wrong in both until the sequence was pinned to the
// digits CCSDS publishes.
//
// pkg/ocsc deliberately does not use this. The optical randomizer of
// CCSDS 142.0-B works on blocks that are not a whole number of octets, so it
// needs a bit-addressable implementation of its own.
package pn

import "sync"

// Period is the length in octets after which the sequence repeats.
//
// The register is 8 bits with a maximal-length polynomial, so the bit sequence
// has period 2^8-1 = 255. Since 255 and 8 share no factor, the octet sequence
// only realigns after 255 octets.
const Period = 255

var (
	once   sync.Once
	period [Period]byte
)

// buildPeriod runs the register once over a full period.
func buildPeriod() {
	reg := uint8(0xFF)
	for i := range period {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			b |= ((reg >> 7) & 1) << uint(bit)

			// Feedback taps for h(x) = x^8 + x^7 + x^5 + x^3 + 1, at register
			// bits 7, 4, 2 and 0.
			//
			// Reading the polynomial exponents straight off as bit indices
			// gives 7, 6, 4, 2 — a different maximal-length sequence that no
			// round-trip test can tell apart, because XOR is its own inverse.
			// The only way to know it is right is the published sequence; see
			// TestSequenceMatchesTheCCSDSVector.
			feedback := ((reg >> 7) ^ (reg >> 4) ^ (reg >> 2) ^ reg) & 1
			reg = ((reg << 1) | feedback) & 0xFF
		}
		period[i] = b
	}
}

// Sequence returns the first length octets of the sequence.
//
// The period is computed once and tiled, so a caller randomizing every frame
// on a channel is not re-running the register each time.
func Sequence(length int) []byte {
	if length <= 0 {
		return nil
	}
	once.Do(buildPeriod)

	out := make([]byte, length)
	for i := 0; i < length; i += Period {
		copy(out[i:], period[:])
	}
	return out
}

// Apply XORs data with the sequence and returns a new slice, leaving the input
// untouched. The operation is its own inverse, so it both randomizes and
// derandomizes.
func Apply(data []byte) []byte {
	once.Do(buildPeriod)

	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ period[i%Period]
	}
	return out
}
