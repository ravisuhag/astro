// Package crc provides CRC checksums used across CCSDS standards.
//
// CRC-16-CCITT uses polynomial 0x1021 with initial value 0xFFFF,
// as specified in the CCSDS overview of space link protocols
// (CCSDS 130.0-G-3). It is used by:
//   - Space Packet Protocol error control (CCSDS 133.0-B-2)
//   - TM Transfer Frame error control (CCSDS 132.0-B-3)
//   - TC Transfer Frame error control (CCSDS 232.0-B-4)
//
// CRC-32 uses the Castagnoli polynomial (CRC-32C, 0x1EDC6F41)
// as specified in CCSDS 732.1-B-2 for USLP Frame Error Control.
package crc

// ComputeCRC16 computes the CRC-16-CCITT checksum per CCSDS specification.
// Uses polynomial 0x1021 with initial value 0xFFFF.
func ComputeCRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ComputeCRC32 computes the CRC-32C (Castagnoli) checksum per CCSDS 732.1-B-2.
// Uses polynomial 0x1EDC6F41 with initial value 0xFFFFFFFF and final XOR 0xFFFFFFFF.
func ComputeCRC32(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x82F63B78 // reflected polynomial
			} else {
				crc >>= 1
			}
		}
	}
	return crc ^ 0xFFFFFFFF
}
