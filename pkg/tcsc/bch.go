package tcsc

// BCH(63,56) codec for CCSDS TC Synchronization and Channel Coding
// per CCSDS 231.0-B-4 section 3.
//
// Each codeblock consists of:
//   - 56 information bits (7 octets)
//   - 7 parity bits: the COMPLEMENT of the LFSR remainder
//     (CCSDS 231.0-B-4 3.3: the parity symbols are inverted on the wire)
//   - 1 filler bit, always '0' (CCSDS 231.0-B-4 3.3.2)
//
// Total: 64 bits (8 octets) per codeblock.
//
// Known-answer vector: all-zeros information octets encode to a
// codeblock whose last octet is 0xFE (remainder 0, complemented to
// 1111111, filler 0).
//
// Two decoding modes are defined by the standard:
//   - SEC (Single Error Correction): corrects 1 bit error per codeblock.
//   - TED (Triple Error Detection): corrects nothing, detects up to
//     3 bit errors per codeblock.
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

// DecodeMode selects the BCH decoding mode per CCSDS 231.0-B-4 section 3.
type DecodeMode int

const (
	// ModeSEC is Single Error Correction: corrects up to 1 bit error per
	// codeblock. A 3-bit error pattern can silently miscorrect in this mode.
	ModeSEC DecodeMode = iota

	// ModeTED is Triple Error Detection: no correction is attempted, and
	// any detectable error pattern (up to 3 bit errors guaranteed) is
	// reported as ErrUncorrectable.
	ModeTED
)

// BCHEncode computes the 7-bit BCH parity for 7 information bytes (56 bits)
// and returns an 8-byte codeblock. Per CCSDS 231.0-B-4 3.3, the transmitted
// parity bits are the COMPLEMENT of the LFSR remainder; they occupy the high
// 7 bits of the 8th byte. The filler bit (LSB) is always '0' per 3.3.2.
// Returns ErrInvalidInfoLength if info is not exactly 7 bytes.
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

	// sr now contains the LFSR remainder. The transmitted parity bits are
	// its complement (CCSDS 231.0-B-4 3.3). Pack into the 8th byte:
	// parity in bits [7:1], filler bit '0' in bit [0] (3.3.2).
	parity := ^sr & 0x7F
	cb[InfoBytes] = parity << 1

	return cb, nil
}

// BCHDecode extracts 7 information bytes from an 8-byte codeblock in SEC
// mode, correcting up to 1 bit error. Returns the corrected information
// bytes, the number of corrected bit errors, and any error.
// Returns ErrUncorrectable if the codeblock has more than 1 bit error.
func BCHDecode(cb [CodeblockBytes]byte) ([]byte, int, error) {
	return BCHDecodeWithMode(cb, ModeSEC)
}

// BCHDecodeWithMode extracts 7 information bytes from an 8-byte codeblock
// using the given decoding mode.
//
// In ModeSEC, up to 1 bit error is corrected; more errors return
// ErrUncorrectable. In ModeTED, no correction is attempted and any
// non-zero syndrome returns ErrUncorrectable (guaranteed detection of
// up to 3 bit errors).
func BCHDecodeWithMode(cb [CodeblockBytes]byte, mode DecodeMode) ([]byte, int, error) {
	// Compute syndrome: feed all 63 code bits (56 info + 7 parity)
	// through the LFSR. Ignore the filler bit. The received parity bits
	// are complemented on the wire (CCSDS 231.0-B-4 3.3), so complement
	// them back before the syndrome pass.
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

	// Process the 7 parity bits (high 7 bits of byte 7), complementing
	// each received bit to undo the on-the-wire inversion.
	parityByte := cb[InfoBytes]
	for bit := 7; bit >= 1; bit-- {
		inBit := ((parityByte >> uint(bit)) & 1) ^ 1
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

	if mode == ModeTED {
		// Triple Error Detection: never correct; report the error.
		return nil, 0, ErrUncorrectable
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
		// Error is in the parity bits, doesn't affect info.
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
