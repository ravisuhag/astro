package pxsc

// CRC-32 for Proximity-1, per CCSDS 211.2-B-3 annex C.
//
// This is not any of the common CRC-32s. Annex C, C1.3 gives the
// generator as:
//
//	G(X) = X^32 + X^23 + X^21 + X^11 + X^2 + 1
//
// which is 0x00A00805 in the usual MSB-first representation. Compare:
//
//	IEEE CRC-32   0x04C11DB7   used by zip, Ethernet
//	CRC-32C       0x1EDC6F41   Castagnoli, used by iSCSI
//	Proximity-1   0x00A00805   this one
//
// pkg/crc.ComputeCRC32 implements the same algorithm (the retired USLP
// issue 732.1-B-1 once borrowed it for an optional 32-bit FECF; current
// USLP carries only the 16-bit FECF); this copy stays here so the
// Proximity-1 stack keeps its own known-answer tests.
//
// The register also starts at zero, not all-ones. The spec flags that
// difference itself: "This initialization differs from that performed for the
// 16-bit CRC described in other CCSDS books, for which the cells are
// initialized to all 'ones'." There is no final inversion.
//
// Getting any of those three details wrong produces a checksum that looks
// plausible and rejects every frame, so this lives here with its own tests
// rather than borrowing from pkg/crc.

// CRC32Polynomial is the generator of annex C, C1.3, in MSB-first form with
// the implicit X^32 term dropped.
const CRC32Polynomial uint32 = 0x00A00805

// CRC32Size is the width of the attached CRC in octets (§3.2.2 c).
const CRC32Size = 4

// crc32Table is the byte-at-a-time lookup table for the Proximity-1 generator.
var crc32Table = buildCRC32Table()

// buildCRC32Table precomputes the MSB-first table for CRC32Polynomial.
func buildCRC32Table() [256]uint32 {
	var table [256]uint32
	for i := range table {
		reg := uint32(i) << 24
		for bit := 0; bit < 8; bit++ {
			if reg&0x80000000 != 0 {
				reg = reg<<1 ^ CRC32Polynomial
			} else {
				reg <<= 1
			}
		}
		table[i] = reg
	}
	return table
}

// ComputeCRC32 returns the Proximity-1 CRC-32 over data.
//
// The register starts at zero and there is no final inversion, per annex C
// and its encoder note.
func ComputeCRC32(data []byte) uint32 {
	var reg uint32
	for _, b := range data {
		reg = reg<<8 ^ crc32Table[byte(reg>>24)^b]
	}
	return reg
}

// VerifyCRC32 reports whether a block ending in its own CRC-32 checks out.
//
// Annex C, C2.2: the syndrome of a correct codeword is zero, so running the
// CRC over the message and its appended check value must give zero.
func VerifyCRC32(codeword []byte) bool {
	if len(codeword) < CRC32Size {
		return false
	}
	return ComputeCRC32(codeword) == 0
}
