// Package crc provides CRC checksums used across CCSDS standards.
//
// The normative authority for these algorithms is the CCSDS Blue Books that
// specify each Frame/Packet Error Control Field:
//
//   - CCSDS 133.0-B-2 §4.1.4 (Space Packet Protocol, Packet Error Control)
//   - CCSDS 132.0-B-3 §4.1.6 (TM Space Data Link, Frame Error Control Field)
//   - CCSDS 232.0-B-4 §4.1.4 (TC Space Data Link, Frame Error Control Field)
//   - CCSDS 732.0-B-4 §4.1.6 (AOS Space Data Link, Frame Error Control Field)
//   - CCSDS 732.1-B-3 §4.1.6 (USLP, 16-bit Frame Error Control Field)
//   - CCSDS 211.2-B-3 annex C (Proximity-1 coding, 32-bit CRC)
//
// The Green Book CCSDS 130.0-G-4 (Overview of Space Communications
// Protocols) is informative background only.
//
// CRC-16-CCITT: generator polynomial x^16 + x^12 + x^5 + 1 (0x1021),
// initial value 0xFFFF, no reflection, no final XOR. Check value:
// CRC("123456789") = 0x29B1.
//
// CRC-32: generator polynomial x^32 + x^23 + x^21 + x^11 + x^2 + 1
// (0x00A00805, MSB-first), the Proximity-1 CRC-32 of CCSDS 211.2-B-3
// annex C: the register is preset to zero — unlike the CRC-16, which
// presets to all ones — with no reflection and no final XOR. The retired
// first issue of USLP (CCSDS 732.1-B-1, 2018) borrowed it for an optional
// 32-bit FECF; that option was removed in 732.1-B-2, and current USLP
// (732.1-B-3 §4.1.6.2.2) carries only the 16-bit FECF. This is NOT the
// IEEE CRC-32 (0x04C11DB7) and NOT CRC-32C/Castagnoli (0x1EDC6F41).
// Check value: CRC("123456789") = 0x51693C0C.
package crc

// The generator polynomials, in the MSB-first form both standards write them.
const (
	// poly16 is x^16 + x^12 + x^5 + 1, CCSDS 133.0-B-2 §4.1.4.
	poly16 = uint16(0x1021)
	// poly32 is x^32 + x^23 + x^21 + x^11 + x^2 + 1, CCSDS 211.2-B-3 annex C.
	poly32 = uint32(0x00A00805)
)

// The lookup tables, one entry per byte value.
//
// Every frame and every packet runs a checksum over its whole length on both
// the send and receive paths, which makes this the hottest loop in the
// library. Processing a byte at a time instead of a bit at a time is worth
// about eight times the throughput and changes nothing about the result: the
// table holds, for each byte, what the bit loop would have done to it.
//
// The tables are computed from the polynomials above rather than written out.
// A transcribed table is 256 opportunities to typo a constant that no
// round-trip test would catch, which is exactly the failure CONTRIBUTING
// warns about; a derived one can only be wrong if the polynomial is, and the
// polynomial is one cited number. TestTableMatchesBitwise checks the two
// agree over every length up to a frame, and the published check values pin
// both.
var (
	table16 = makeTable16()
	table32 = makeTable32()
)

func makeTable16() [256]uint16 {
	var table [256]uint16
	for i := range table {
		crc := uint16(i) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ poly16
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}

func makeTable32() [256]uint32 {
	var table [256]uint32
	for i := range table {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ poly32
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
	return table
}

// ComputeCRC16 computes the CRC-16-CCITT checksum used by the Space Packet
// (CCSDS 133.0-B-2 §4.1.4), TM (132.0-B-3 §4.1.6), TC (232.0-B-4 §4.1.4),
// AOS (732.0-B-4 §4.1.6) and USLP (732.1-B-3 §4.1.6) error control fields.
// Polynomial 0x1021, initial value 0xFFFF, no reflection, no final XOR.
func ComputeCRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		// The top byte of the register leaves it as this byte enters, so the
		// two are combined to pick the table entry.
		crc = crc<<8 ^ table16[byte(crc>>8)^b]
	}
	return crc
}

// ComputeCRC32 computes the Proximity-1 CRC-32 (CCSDS 211.2-B-3 annex C).
// The retired USLP issue 1 (CCSDS 732.1-B-1) once offered it as an
// optional 32-bit FECF, but that option was removed in 732.1-B-2; current
// USLP frames carry only the 16-bit FECF. Polynomial 0x00A00805 MSB-first,
// initial value 0, no reflection, no final XOR.
func ComputeCRC32(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = crc<<8 ^ table32[byte(crc>>24)^b]
	}
	return crc
}
