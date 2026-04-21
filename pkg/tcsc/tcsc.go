// Package tcsc implements the TC Synchronization and Channel Coding sublayer
// per CCSDS 231.0-B-4 (TC Synchronization and Channel Coding).
//
// This sublayer sits between the TC Data Link Protocol (CCSDS 232.0-B-4)
// and the physical layer, providing:
//   - Command Link Transmission Unit (CLTU) wrapping and unwrapping
//   - BCH(63,56) forward error correction per codeblock
//   - CCSDS pseudo-randomization for bit transition density assurance
package tcsc

import "bytes"

// DefaultStartSequence returns the standard CCSDS CLTU start sequence
// (0xEB90) used to identify the beginning of each CLTU in the bitstream.
// A fresh copy is returned each call to prevent accidental mutation.
func DefaultStartSequence() []byte {
	return []byte{0xEB, 0x90}
}

// DefaultTailSequence returns the standard CCSDS CLTU tail sequence
// (0xC5C5C5C5C5C5C579) used to mark the end of a CLTU.
// A fresh copy is returned each call to prevent accidental mutation.
func DefaultTailSequence() []byte {
	return []byte{0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0xC5, 0x79}
}

// Randomize applies CCSDS pseudo-randomization by XOR-ing data with
// the standard PN (pseudo-noise) sequence. The same operation is used
// for both randomization and de-randomization since XOR is self-inverse.
// Returns a new slice; the input is not modified.
func Randomize(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	pn := GeneratePNSequence(len(data))
	for i := range out {
		out[i] ^= pn[i]
	}
	return out
}

// GeneratePNSequence produces the CCSDS pseudo-random sequence using an
// 8-bit LFSR with polynomial h(x) = x^8 + x^7 + x^5 + x^3 + 1,
// initialized to all 1s per CCSDS 231.0-B-4.
func GeneratePNSequence(length int) []byte {
	seq := make([]byte, length)
	reg := uint8(0xFF)
	for i := range length {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			output := (reg >> 7) & 1
			b |= output << uint(bit)
			// Taps: x^8(bit7), x^7(bit6), x^5(bit4), x^3(bit2)
			feedback := ((reg >> 7) ^ (reg >> 6) ^ (reg >> 4) ^ (reg >> 2)) & 1
			reg = ((reg << 1) | feedback) & 0xFF
		}
		seq[i] = b
	}
	return seq
}

// WrapCLTU produces a Command Link Transmission Unit from TC Transfer Frame
// data. It:
//  1. Optionally applies CCSDS pseudo-randomization to the frame data
//  2. Pads the data to a multiple of 7 bytes (InfoBytes per codeblock)
//  3. Encodes each 7-byte block with BCH(63,56) to produce 8-byte codeblocks
//  4. Prepends the start sequence and appends the tail sequence
//
// If startSeq or tailSeq is nil, the CCSDS defaults are used.
func WrapCLTU(frameData, startSeq, tailSeq []byte, randomize bool) ([]byte, error) {
	if len(frameData) == 0 {
		return nil, ErrEmptyData
	}
	if startSeq == nil {
		startSeq = DefaultStartSequence()
	}
	if tailSeq == nil {
		tailSeq = DefaultTailSequence()
	}

	data := frameData
	if randomize {
		data = Randomize(frameData)
	}

	// Pad to a multiple of InfoBytes with fill bytes (0x55).
	padded := data
	if rem := len(data) % InfoBytes; rem != 0 {
		padding := make([]byte, InfoBytes-rem)
		for i := range padding {
			padding[i] = 0x55
		}
		padded = make([]byte, len(data)+len(padding))
		copy(padded, data)
		copy(padded[len(data):], padding)
	}

	numBlocks := len(padded) / InfoBytes
	cltu := make([]byte, len(startSeq)+numBlocks*CodeblockBytes+len(tailSeq))

	// Prepend start sequence.
	copy(cltu, startSeq)
	offset := len(startSeq)

	// Encode each 7-byte info block into an 8-byte codeblock.
	for i := range numBlocks {
		info := padded[i*InfoBytes : (i+1)*InfoBytes]
		cb, err := BCHEncode(info)
		if err != nil {
			return nil, err
		}
		copy(cltu[offset:], cb[:])
		offset += CodeblockBytes
	}

	// Append tail sequence.
	copy(cltu[offset:], tailSeq)

	return cltu, nil
}

// UnwrapCLTU extracts and error-corrects TC Transfer Frame data from a CLTU.
// It:
//  1. Validates and strips the start and tail sequences
//  2. Decodes each 8-byte codeblock with BCH(63,56), correcting up to
//     1 bit error per codeblock
//  3. Concatenates the 7-byte info portions
//  4. Optionally de-randomizes the result
//
// Returns the recovered frame data, total number of corrected bit errors,
// and any error. If startSeq or tailSeq is nil, CCSDS defaults are used.
//
// Note: The caller must know the original data length to strip any padding
// added during WrapCLTU, as the padding is not self-describing.
func UnwrapCLTU(cltu, startSeq, tailSeq []byte, randomize bool) ([]byte, int, error) {
	if startSeq == nil {
		startSeq = DefaultStartSequence()
	}
	if tailSeq == nil {
		tailSeq = DefaultTailSequence()
	}

	minLen := len(startSeq) + CodeblockBytes + len(tailSeq)
	if len(cltu) < minLen {
		return nil, 0, ErrDataTooShort
	}

	// Validate start sequence.
	if !bytes.Equal(cltu[:len(startSeq)], startSeq) {
		return nil, 0, ErrStartSequenceMismatch
	}

	// Validate tail sequence.
	if !bytes.Equal(cltu[len(cltu)-len(tailSeq):], tailSeq) {
		return nil, 0, ErrTailSequenceMismatch
	}

	// Extract the codeblock body.
	body := cltu[len(startSeq) : len(cltu)-len(tailSeq)]
	if len(body)%CodeblockBytes != 0 {
		return nil, 0, ErrInvalidCLTULength
	}

	numBlocks := len(body) / CodeblockBytes
	result := make([]byte, 0, numBlocks*InfoBytes)
	totalCorr := 0

	for i := range numBlocks {
		var cb [CodeblockBytes]byte
		copy(cb[:], body[i*CodeblockBytes:(i+1)*CodeblockBytes])

		info, corr, err := BCHDecode(cb)
		if err != nil {
			return nil, 0, err
		}
		totalCorr += corr
		result = append(result, info...)
	}

	if randomize {
		result = Randomize(result)
	}

	return result, totalCorr, nil
}
