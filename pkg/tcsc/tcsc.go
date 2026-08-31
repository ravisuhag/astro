// Package tcsc implements the TC Synchronization and Channel Coding sublayer
// per CCSDS 231.0-B-4 (TC Synchronization and Channel Coding).
//
// This sublayer sits between the TC Data Link Protocol (CCSDS 232.0-B-4)
// and the physical layer, providing:
//   - Command Link Transmission Unit (CLTU) wrapping and unwrapping
//   - BCH(63,56) forward error correction per codeblock
//   - CCSDS pseudo-randomization for bit transition density assurance
package tcsc

import (
	"bytes"

	"github.com/ravisuhag/astro/internal/pn"
)

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

// Randomize applies CCSDS pseudo-randomization by XOR-ing data with the TC
// PN (pseudo-noise) sequence. The same operation is used for both
// randomization and de-randomization since XOR is self-inverse.
// Returns a new slice; the input is not modified.
//
// That self-inverse property is also why a wrap/unwrap round trip proves
// nothing here: every sequence round-trips, including a wrong one. What pins
// this to the standard is TestPNSequenceMatchesTheCCSDSVector.
func Randomize(data []byte) []byte {
	return pn.TCApply(data)
}

// GeneratePNSequence produces the CCSDS 231.0-B-4 §6.2 pseudo-random sequence
// using an 8-bit LFSR with polynomial
//
//	h(x) = x^8 + x^6 + x^4 + x^3 + x^2 + x + 1
//
// initialized to all 1s. Its first 40 digits are FF 39 9E 5A 68.
//
// The generator lives in internal/pn next to the TM one. The two standards do
// NOT specify the same randomizer: CCSDS 131.0-B-5 §10.4.2 uses a different
// polynomial and a sequence that opens FF 48 0E C0 9A. Calling the TM
// generator here would put unintelligible octets on the uplink, so the
// internal names are qualified by standard and this package uses only the TC
// pair.
func GeneratePNSequence(length int) []byte {
	return pn.TCSequence(length)
}

// WrapCLTU produces a Command Link Transmission Unit from TC Transfer Frame
// data. It:
//  1. Pads the data to a multiple of 7 bytes (InfoBytes per codeblock)
//     with 0x55 fill octets
//  2. Optionally applies CCSDS pseudo-randomization to the padded buffer
//     (fill octets included, per CCSDS 231.0-B-4: randomization covers
//     everything between the start and tail sequences)
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

	// Pad to a multiple of InfoBytes with fill bytes (0x55) FIRST, then
	// randomize the whole padded buffer, so the fill octets go out
	// randomized like the rest of the data.
	padded := frameData
	if rem := len(frameData) % InfoBytes; rem != 0 {
		padded = make([]byte, len(frameData)+InfoBytes-rem)
		copy(padded, frameData)
		for i := len(frameData); i < len(padded); i++ {
			padded[i] = 0x55
		}
	}
	if randomize {
		padded = Randomize(padded)
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

// UnwrapCLTU extracts and error-corrects TC Transfer Frame data from a CLTU
// using SEC decoding. See UnwrapCLTUWithMode.
func UnwrapCLTU(cltu, startSeq, tailSeq []byte, randomize bool) ([]byte, int, error) {
	return UnwrapCLTUWithMode(cltu, startSeq, tailSeq, randomize, ModeSEC)
}

// UnwrapCLTUWithMode extracts and error-corrects TC Transfer Frame data
// from a CLTU. It:
//  1. Validates and strips the start sequence
//  2. Decodes 8-byte codeblocks with BCH(63,56) in the given mode
//  3. Terminates on the tail sequence, or on the first codeblock that
//     fails to decode (per CCSDS 231.0-B-4 the receiver stops at the
//     first rejected codeblock, so bit errors in the tail are tolerated)
//  4. Concatenates the 7-byte info portions
//  5. Optionally de-randomizes the result (fill octets included)
//
// Returns the recovered frame data, total number of corrected bit errors,
// and any error. If startSeq or tailSeq is nil, CCSDS defaults are used.
//
// Note: The caller must know the original data length to strip any padding
// added during WrapCLTU, as the padding is not self-describing.
func UnwrapCLTUWithMode(cltu, startSeq, tailSeq []byte, randomize bool, mode DecodeMode) ([]byte, int, error) {
	if startSeq == nil {
		startSeq = DefaultStartSequence()
	}
	if tailSeq == nil {
		tailSeq = DefaultTailSequence()
	}

	if len(cltu) < len(startSeq)+CodeblockBytes {
		return nil, 0, ErrDataTooShort
	}

	// Validate start sequence.
	if !bytes.Equal(cltu[:len(startSeq)], startSeq) {
		return nil, 0, ErrStartSequenceMismatch
	}

	body := cltu[len(startSeq):]
	result := make([]byte, 0, len(body)/CodeblockBytes*InfoBytes)
	totalCorr := 0

	for len(body) >= CodeblockBytes {
		// An exact tail sequence at a codeblock boundary ends the CLTU.
		if len(tailSeq) > 0 && len(body) >= len(tailSeq) &&
			bytes.Equal(body[:len(tailSeq)], tailSeq) {
			break
		}

		var cb [CodeblockBytes]byte
		copy(cb[:], body[:CodeblockBytes])

		info, corr, err := BCHDecodeWithMode(cb, mode)
		if err != nil {
			// First failed codeblock terminates the CLTU. The CCSDS
			// tail sequence is built to fail BCH decoding, so a tail
			// with bit errors still ends the CLTU here. A failure on
			// the very first codeblock means no data was recovered.
			if len(result) == 0 {
				return nil, 0, err
			}
			break
		}
		totalCorr += corr
		result = append(result, info...)
		body = body[CodeblockBytes:]
	}

	if len(result) == 0 {
		return nil, 0, ErrDataTooShort
	}

	if randomize {
		result = Randomize(result)
	}

	return result, totalCorr, nil
}
