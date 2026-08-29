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

// OIDSequence generates the Pseudo Noise sequence that fills the data field of
// Only Idle Data transfer frames. CCSDS 132.0-B-3 §4.1.4.6.2 (TM, annex D) and
// CCSDS 732.1-B-3 §4.1.4.1.10 (USLP, annex H) mandate the same generator: a
// 32-cell Fibonacci-form linear feedback shift register realising
//
//	D^0 + D^1 + D^2 + D^22 + D^32
//
// seeded to all ones at device start-up and never restarted between frames.
// Both standards publish the same opening octets, FF FF FF FF 6D B6 D8 61 45
// 1F, which is the only way to tell correct taps from a plausible-looking
// permutation of them — the same trap the 8-bit randomizer above fell into.
//
// This is not the randomizer of Sequence and Apply: that one is an 8-bit
// register applied by the channel coding layer to every frame, while this one
// is a 32-bit register whose output *is* the payload of an idle frame.
//
// Unlike the randomizer this sequence is not tiled: its period is 2^32-1
// octets, so it is generated as a stream. A value is safe for concurrent use.
type OIDSequence struct {
	mu  sync.Mutex
	reg uint32 // bit i holds LFSR cell D(32-i): bit 0 is D32, bit 31 is D1
}

// NewOIDSequence returns a generator in the 'all ones' start-up state. Keep
// one per channel for the life of the device; restarting it makes every idle
// frame carry the same octets, which is the insufficient randomization the
// standards warn against.
func NewOIDSequence() *OIDSequence {
	return &OIDSequence{reg: 0xFFFFFFFF}
}

// Fill writes the next len(buf) octets of the sequence into buf, most
// significant bit first, advancing the generator.
func (s *OIDSequence) Fill(buf []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range buf {
		var b byte
		for range 8 {
			out := byte(s.reg & 1)                                // cell D32
			fb := (s.reg>>31 ^ s.reg>>30 ^ s.reg>>10 ^ s.reg) & 1 // D1+D2+D22+D32
			b = b<<1 | out
			s.reg = s.reg>>1 | fb<<31
		}
		buf[i] = b
	}
}
