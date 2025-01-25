package spp

import (
	"hash/crc32"
	"strings"
)

// mapToString converts a map to a string representation.
func mapToString(m map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range m {
		sb.WriteString(k + ": " + string(v.(string)) + "\n")
	}
	return sb.String()
}

// ComputeCRC computes the CRC checksum for the given data.
func ComputeCRC(data []byte) uint16 {
	checksum := crc32.ChecksumIEEE(data)
	return uint16(checksum & 0xFFFF) // Return lower 16 bits of the checksum
}
