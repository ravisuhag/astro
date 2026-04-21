package tcsc

// BCH(63,56) codec for CCSDS TC Synchronization and Channel Coding
// per CCSDS 231.0-B-4.
//
// Each codeblock consists of:
//   - 56 information bits (7 octets)
//   - 7 parity bits computed by the BCH generator polynomial
//   - 1 filler bit (complement of the last parity bit)
//
// Total: 64 bits (8 octets) per codeblock.
//
// The code can detect up to 3 bit errors and correct 1 bit error
// per codeblock.
//
// Generator polynomial: g(x) = x^7 + x^6 + x^2 + 1

const (
	// InfoBytes is the number of information bytes per codeblock.
	InfoBytes = 7

	// CodeblockBytes is the total number of bytes per codeblock
	// (7 info + 1 parity/filler).
	CodeblockBytes = 8

	// bchPoly is the generator polynomial g(x) = x^7 + x^6 + x^2 + 1
	// represented as a bit mask over 8 bits: 1_1000_101 = 0xC5.
	// The leading x^7 term corresponds to bit 7.
	bchPoly = 0xC5
)

// BCHEncode computes the 7-bit BCH parity for 7 information bytes (56 bits)
// and returns an 8-byte codeblock. The parity is placed in the high 7 bits
// of the 8th byte, and the filler bit (complement of the last parity bit)
// occupies the LSB. Returns ErrInvalidInfoLength if info is not exactly 7 bytes.
func BCHEncode(info []byte) ([CodeblockBytes]byte, error) {
	var cb [CodeblockBytes]byte
	if len(info) != InfoBytes {
		return cb, ErrInvalidInfoLength
	}
	copy(cb[:InfoBytes], info)

	// Compute parity: systematic encoding via polynomial division.
	// Process each of the 56 information bits through the LFSR
	// defined by bchPoly.
	var sr byte // 7-bit shift register
	for i := range InfoBytes {
		b := info[i]
		for bit := 7; bit >= 0; bit-- {
			inBit := (b >> uint(bit)) & 1
			feedback := ((sr >> 6) ^ inBit) & 1
			sr <<= 1
			if feedback != 0 {
				sr ^= bchPoly
			}
			sr &= 0x7F // keep 7 bits
		}
	}

	// sr now contains the 7 parity bits.
	// Pack into the 8th byte: parity in bits [7:1], filler in bit [0].
	parity := sr & 0x7F
	filler := ^parity & 1 // complement of lowest parity bit
	cb[InfoBytes] = (parity << 1) | filler

	return cb, nil
}

// BCHDecode extracts 7 information bytes from an 8-byte codeblock,
// correcting up to 1 bit error. Returns the corrected information bytes,
// the number of corrected bit errors, and any error.
// Returns ErrUncorrectable if the codeblock has more than 1 bit error.
func BCHDecode(cb [CodeblockBytes]byte) ([]byte, int, error) {
	// Compute syndrome: feed all 63 code bits (56 info + 7 parity)
	// through the LFSR. Ignore the filler bit.
	var sr byte
	for i := range InfoBytes {
		b := cb[i]
		for bit := 7; bit >= 0; bit-- {
			inBit := (b >> uint(bit)) & 1
			feedback := ((sr >> 6) ^ inBit) & 1
			sr <<= 1
			if feedback != 0 {
				sr ^= bchPoly
			}
			sr &= 0x7F
		}
	}

	// Process the 7 parity bits (high 7 bits of byte 7).
	parityByte := cb[InfoBytes]
	for bit := 7; bit >= 1; bit-- {
		inBit := (parityByte >> uint(bit)) & 1
		feedback := ((sr >> 6) ^ inBit) & 1
		sr <<= 1
		if feedback != 0 {
			sr ^= bchPoly
		}
		sr &= 0x7F
	}

	// sr is now the syndrome.
	if sr == 0 {
		// No errors.
		info := make([]byte, InfoBytes)
		copy(info, cb[:InfoBytes])
		return info, 0, nil
	}

	// Single-bit error correction: the syndrome equals the column of
	// the parity check matrix corresponding to the error position.
	// We search for the matching position among the 63 code bits.
	errPos := findErrorPosition(sr)
	if errPos < 0 {
		return nil, 0, ErrUncorrectable
	}

	// Correct the error.
	corrected := cb
	if errPos < 56 {
		// Error is in the information bits.
		byteIdx := errPos / 8
		bitIdx := 7 - (errPos % 8)
		corrected[byteIdx] ^= 1 << uint(bitIdx)
	} else {
		// Error is in the parity bits — doesn't affect info.
		parityBitIdx := errPos - 56
		bitIdx := 7 - parityBitIdx
		corrected[InfoBytes] ^= 1 << uint(bitIdx)
	}

	info := make([]byte, InfoBytes)
	copy(info, corrected[:InfoBytes])
	return info, 1, nil
}

// findErrorPosition returns the bit position (0-62) whose syndrome
// matches sr, or -1 if no match (multi-bit error).
func findErrorPosition(syndrome byte) int {
	// Generate the syndrome for each single-bit error position.
	// Position 0 is the first information bit (MSB of byte 0).
	for pos := range 63 {
		if syndromeForPosition(pos) == syndrome {
			return pos
		}
	}
	return -1
}

// syndromeForPosition computes the syndrome that a single-bit error
// at the given position (0-62) would produce. This is the corresponding
// column of the parity check matrix.
func syndromeForPosition(pos int) byte {
	// Create a 63-bit codeword with only bit `pos` set and compute
	// its syndrome.
	var sr byte
	for i := range 63 {
		var inBit byte
		if i == pos {
			inBit = 1
		}
		feedback := ((sr >> 6) ^ inBit) & 1
		sr <<= 1
		if feedback != 0 {
			sr ^= bchPoly
		}
		sr &= 0x7F
	}
	return sr
}
