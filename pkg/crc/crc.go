// Package crc provides the CRC-16-CCITT checksum used across CCSDS standards.
//
// The CRC algorithm uses polynomial 0x1021 with initial value 0xFFFF,
// as specified in the CCSDS overview of space link protocols
// (CCSDS 130.0-G-3). It is used by:
//   - Space Packet Protocol error control (CCSDS 133.0-B-2)
//   - TM Transfer Frame error control (CCSDS 132.0-B-3)
//   - TC Transfer Frame error control (CCSDS 232.0-B-4)
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
