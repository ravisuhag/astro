package ocsc

// Pseudo-randomizer, per CCSDS 142.0-B-1 §3.5.
//
// Each information block is XORed with a pseudo-random sequence, so a long run
// of identical data does not become a long run of identical optical pulses.
// A receiver needs transitions to keep symbol timing.
//
// The generating polynomial is g(D) = D^8 + D^7 + D^5 + D^3 + 1 (§3.5.2.1),
// the register starts at all ones (§3.5.3.2), and the sequence repeats every
// 255 digits (§3.5.3.1).
//
// This is the same sequence pkg/tmsc uses for TM randomization — §3.5.2.1's
// note says so outright. It is reimplemented here rather than shared because
// the optical chain addresses individual bits: an information block is 5006
// bits at code rate 1/3, so a byte-oriented generator cannot serve it.

// PNPeriod is how many digits pass before the sequence repeats (§3.5.3.1).
const PNPeriod = 255

// pnSequence is one period of the pseudo-random sequence, one bit per entry.
var pnSequence = generatePN()

// generatePN runs the LFSR of §3.5.2.1 for one full period.
//
// The register is initialized to all ones (§3.5.3.2), the output is its top
// bit, and the feedback taps sit at register bits 7, 4, 2 and 0.
//
// Those tap positions are the ones that reproduce the sequence the standard
// publishes. §3.5.2.1's note gives the first 40 digits, and
// TestPNSequenceMatchesTheSpecVector checks them: a polynomial this short has
// several plausible register layouts, and only one of them is right.
func generatePN() [PNPeriod]uint8 {
	var out [PNPeriod]uint8
	reg := uint8(0xFF)

	for i := 0; i < PNPeriod; i++ {
		// The output is the top bit of the register.
		out[i] = reg >> 7 & 1

		feedback := (reg>>7 ^ reg>>4 ^ reg>>2 ^ reg) & 1
		reg = reg<<1 | feedback
	}
	return out
}

// PNBit returns digit i of the pseudo-random sequence, wrapping at the period.
func PNBit(i int) uint8 {
	if i < 0 {
		return 0
	}
	return pnSequence[i%PNPeriod]
}

// PNSequence returns the first n digits of the pseudo-random sequence, one bit
// per entry.
func PNSequence(n int) []uint8 {
	if n <= 0 {
		return nil
	}
	out := make([]uint8, n)
	for i := range out {
		out[i] = PNBit(i)
	}
	return out
}

// Randomize XORs a block with the pseudo-random sequence, per §3.5.1.1.
//
// §3.5.3.1: the sequence begins at the first digit of each block, so every
// block is randomized independently. The operation is its own inverse.
func Randomize(block *BitString) *BitString {
	if block == nil {
		return NewBitString(0)
	}
	out := NewBitString(block.Len())
	for i := 0; i < block.Len(); i++ {
		out.SetBit(i, block.Bit(i)^PNBit(i))
	}
	return out
}

// Derandomize reverses Randomize. It is the same operation, named for what the
// receiver is doing.
func Derandomize(block *BitString) *BitString { return Randomize(block) }
