package pxsc

// Convolutional encoding, per CCSDS 211.2-B-3 §3.4.3.
//
// Proximity-1 offers the rate 1/2, constraint-length 7 convolutional code that
// CCSDS 131.0-B defines. Each input bit produces two output symbols, so the
// stream doubles in length, and the redundancy lets a receiver correct errors
// the link introduced.
//
// Only the encoder is here. Decoding a convolutional code means a Viterbi
// trellis search over soft-decision symbols, and this library ships no
// soft-decision decoder anywhere — pkg/tmsc stops at Reed-Solomon, pkg/tcsc at
// BCH. Adding one here alone would be out of step, so §3.4.3.3's recommended
// three-bit soft decisions are left to the receiver hardware.

// Convolutional code parameters, per CCSDS 131.0-B as referenced by §3.4.3.1.
const (
	// ConstraintLength is the number of input bits each output symbol depends
	// on, including the current one.
	ConstraintLength = 7
	// CodeRateNumerator and CodeRateDenominator give the rate 1/2.
	CodeRateNumerator   = 1
	CodeRateDenominator = 2
)

// Connection vectors for the rate 1/2 code, in octal as CCSDS writes them:
// G1 = 171, G2 = 133.
const (
	// G1 is the first connection vector, 1111001 in binary.
	G1 uint8 = 0o171
	// G2 is the second connection vector, 1011011 in binary. Its output is
	// inverted, per §3.4.3.1 note 1.
	G2 uint8 = 0o133
)

// ConvolutionalEncoder holds the shift register between calls, so a stream can
// be encoded in pieces.
//
// The register is not reset between Encode calls. That is deliberate: §3.4.3.2
// encodes everything transmitted as one continuous stream, PLTUs and idle data
// alike, so the encoder state carries across unit boundaries.
type ConvolutionalEncoder struct {
	// state holds the previous ConstraintLength-1 input bits, most recent in
	// the low bit.
	state uint8
}

// NewConvolutionalEncoder returns an encoder with a cleared register.
func NewConvolutionalEncoder() *ConvolutionalEncoder {
	return &ConvolutionalEncoder{}
}

// Reset clears the shift register.
func (e *ConvolutionalEncoder) Reset() { e.state = 0 }

// parity returns the modulo-2 sum of the bits of v, which is what a
// connection vector tap computes.
func parity(v uint8) uint8 {
	v ^= v >> 4
	v ^= v >> 2
	v ^= v >> 1
	return v & 1
}

// EncodeBit encodes one input bit and returns the two output symbols.
//
// §3.4.3.1 note 1: the output on the G2 path is inverted.
func (e *ConvolutionalEncoder) EncodeBit(bit uint8) (c1, c2 uint8) {
	// Shift the new bit into the register, keeping ConstraintLength bits.
	reg := e.state<<1 | bit&1
	reg &= 1<<ConstraintLength - 1
	e.state = reg

	c1 = parity(reg & G1)
	c2 = parity(reg&G2) ^ 1 // inverted output path
	return c1, c2
}

// Encode convolutionally encodes data, returning twice as many octets.
//
// Bits are taken most significant first, matching the bit numbering
// convention of §1.6.2. Each input bit yields two symbols, packed the same way.
func (e *ConvolutionalEncoder) Encode(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	out := make([]byte, 0, len(data)*2)
	var acc uint8
	var accBits int

	for _, b := range data {
		for i := 7; i >= 0; i-- {
			c1, c2 := e.EncodeBit(b >> uint(i) & 1)

			acc = acc<<1 | c1
			accBits++
			if accBits == 8 {
				out = append(out, acc)
				acc, accBits = 0, 0
			}

			acc = acc<<1 | c2
			accBits++
			if accBits == 8 {
				out = append(out, acc)
				acc, accBits = 0, 0
			}
		}
	}
	return out
}

// ConvolutionalEncode encodes data with a fresh encoder. Use a
// ConvolutionalEncoder directly when the stream continues across calls.
func ConvolutionalEncode(data []byte) []byte {
	return NewConvolutionalEncoder().Encode(data)
}
