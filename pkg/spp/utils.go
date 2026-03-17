package spp

import "encoding/binary"

// CalculatePacketSize computes the total size of a space packet from a raw header dump.
// It uses the packet length field in the primary header to determine the full packet size.
func CalculatePacketSize(header []byte) (int, error) {
	if len(header) < PrimaryHeaderSize {
		return 0, ErrDataTooShort
	}

	// Extract the packet length field (bytes 4 and 5 of the header)
	packetLength := binary.BigEndian.Uint16(header[4:6])

	// Total packet size = primary header (6) + packet data field (packetLength + 1)
	totalPacketSize := PrimaryHeaderSize + int(packetLength) + 1

	return totalPacketSize, nil
}

// IsIdle reports whether the packet is an idle packet (APID 0x7FF).
func (sp *SpacePacket) IsIdle() bool {
	return sp.PrimaryHeader.APID == 0x7FF
}

// ComputeCRC computes the CRC-16-CCITT checksum per CCSDS specification.
// Uses polynomial 0x1021 with initial value 0xFFFF.
func ComputeCRC(data []byte) uint16 {
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
