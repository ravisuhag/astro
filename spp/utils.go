package spp

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"strings"
)

// CalculatePacketSize computes the total size of a space packet from a raw header dump.
// It uses the packet length field in the primary header to determine the full packet size.
func CalculatePacketSize(header []byte) (int, error) {

	if len(header) < PrimaryHeaderSize {
		return 0, errors.New("header size is too small to calculate packet size")
	}

	// Extract the packet length field (bytes 4 and 5 of the header)
	packetLength := binary.BigEndian.Uint16(header[4:6])

	// Calculate the total packet size (primary header + payload length + 1 byte for packet length offset)
	totalPacketSize := PrimaryHeaderSize + int(packetLength) + 1

	return totalPacketSize, nil
}

// ComputeCRC computes the CRC checksum for the given data.
func ComputeCRC(data []byte) uint16 {
	checksum := crc32.ChecksumIEEE(data)
	return uint16(checksum & 0xFFFF) // Return lower 16 bits of the checksum
}

// mapToString converts a map to a string representation.
func mapToString(m map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range m {
		sb.WriteString(k + ": " + string(v.(string)) + "\n")
	}
	return sb.String()
}
