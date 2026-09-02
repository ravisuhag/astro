// Package pn generates the CCSDS pseudo-randomizer sequences used by the TM
// and TC synchronization and channel coding layers.
//
// TM and TC do NOT share a randomizer. Both are 8-bit linear feedback shift
// registers preset to all ones, and both repeat after 255 bits, but the
// polynomials differ:
//
//	CCSDS 131.0-B-5 clause 10.4.2 (TM)  h(x) = x^8 + x^7 + x^5 + x^3 + 1
//	CCSDS 231.0-B-4 clause 6.2   (TC)   h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1
//
// The two sequences diverge at the second octet (TM opens FF 48 0E C0 9A and
// TC opens FF 39 9E 5A 68) so equipment fed the wrong one recovers noise. The
// generators are therefore named for the standard they belong to, and neither
// name is available unqualified: TMSequence and TMApply are for pkg/tmsc,
// TCSequence and TCApply are for pkg/tcsc.
//
// Nothing about a randomizer can be checked by round-tripping it. XOR is its
// own inverse, so any sequence at all (right taps, wrong taps, or a constant)
// randomizes and derandomizes back to the input. Correctness here rests
// entirely on the vectors CCSDS publishes; see the tests.
//
// pkg/ocsc deliberately does not use this. The optical randomizer of
// CCSDS 142.0-B works on blocks that are not a whole number of octets, so it
// needs a bit-addressable implementation of its own.
package pn

import (
	"math/bits"
	"sync"
)

// Period is the length in octets after which either sequence repeats.
//
// Each register is 8 bits with a maximal-length polynomial, so the bit sequence
// has period 2^8-1 = 255. Since 255 and 8 share no factor, the octet sequence
// only realigns after 255 octets.
const Period = 255

// Feedback tap masks, in the Fibonacci form the generator below uses: the
// output bit is the register's most significant bit, the feedback bit is the
// parity of the tapped cells, and the register then shifts left with the
// feedback entering at the bottom.
//
// With that convention the register cell at bit 7 holds the oldest output bit
// and bit 0 the newest, so a tap at bit k contributes output bit b(n+7-k) to
// the recurrence b(n+8) = sum of taps. Reading the polynomial exponents
// straight off as bit indices gives a different maximal-length sequence in both
// cases. Only the published vectors tell the two apart, which is what
// TestTMSequenceMatchesTheCCSDSVector and TestTCSequenceMatchesTheCCSDSVector
// exist to check.
const (
	// tmTaps: bits 7, 4, 2, 0 give b(n+8) = b(n+7)+b(n+5)+b(n+3)+b(n),
	// which is CCSDS 131.0-B-5 clause 10.4.2's h(x) = x^8 + x^7 + x^5 + x^3 + 1.
	tmTaps = 0b10010101

	// tcTaps: bits 7, 6, 5, 4, 3, 1 give
	// b(n+8) = b(n+6)+b(n+4)+b(n+3)+b(n+2)+b(n+1)+b(n), which is
	// CCSDS 231.0-B-4 clause 6.2's h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1.
	tcTaps = 0b11111010
)

// generator holds one randomizer's tap set and its cached period.
type generator struct {
	taps   uint8
	once   sync.Once
	period [Period]byte
}

var (
	tm = generator{taps: tmTaps}
	tc = generator{taps: tcTaps}
)

// build runs the register once over a full period.
func (g *generator) build() {
	reg := uint8(0xFF)
	for i := range g.period {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			b |= ((reg >> 7) & 1) << uint(bit)

			feedback := uint8(bits.OnesCount8(reg&g.taps) & 1)
			reg = (reg << 1) | feedback
		}
		g.period[i] = b
	}
}

// sequence returns the first length octets, tiled from the cached period.
func (g *generator) sequence(length int) []byte {
	if length <= 0 {
		return nil
	}
	g.once.Do(g.build)

	out := make([]byte, length)
	for i := 0; i < length; i += Period {
		copy(out[i:], g.period[:])
	}
	return out
}

// apply XORs data with the sequence, returning a new slice.
func (g *generator) apply(data []byte) []byte {
	g.once.Do(g.build)

	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ g.period[i%Period]
	}
	return out
}

// TMSequence returns the first length octets of the TM randomizer sequence of
// CCSDS 131.0-B-5 clause 10.4.2. It opens FF 48 0E C0 9A.
//
// The period is computed once and tiled, so a caller randomizing every frame
// on a channel is not re-running the register each time.
func TMSequence(length int) []byte { return tm.sequence(length) }

// TMApply XORs data with the TM sequence and returns a new slice, leaving the
// input untouched. The operation is its own inverse, so it both randomizes and
// derandomizes.
func TMApply(data []byte) []byte { return tm.apply(data) }

// TCSequence returns the first length octets of the TC randomizer sequence of
// CCSDS 231.0-B-4 clause 6.2. It opens FF 39 9E 5A 68, which is a different sequence
// from TMSequence, see the package comment.
//
// It is cached and tiled exactly like the TM sequence.
func TCSequence(length int) []byte { return tc.sequence(length) }

// TCApply XORs data with the TC sequence and returns a new slice, leaving the
// input untouched. Like TMApply it is its own inverse, and like TMApply that
// property proves nothing about the taps being right.
func TCApply(data []byte) []byte { return tc.apply(data) }

// OIDSequence generates the Pseudo Noise sequence that fills the data field of
// Only Idle Data transfer frames. CCSDS 132.0-B-3 clause 4.1.4.6.2 (TM, annex D) and
// CCSDS 732.1-B-3 clause 4.1.4.1.10 (USLP, annex H) mandate the same generator: a
// 32-cell Fibonacci-form linear feedback shift register realising
//
//	D^0 + D^1 + D^2 + D^22 + D^32
//
// seeded to all ones at device start-up and never restarted between frames.
// Both standards publish the same opening octets, FF FF FF FF 6D B6 D8 61 45
// 1F, which is the only way to tell correct taps from a plausible-looking
// permutation of them, the same trap the 8-bit randomizer above fell into.
//
// This is neither of the randomizers above: those are 8-bit registers applied
// by the channel coding layer to every frame, while this one is a 32-bit
// register whose output *is* the payload of an idle frame.
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
