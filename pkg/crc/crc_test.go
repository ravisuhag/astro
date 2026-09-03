package crc_test

import (
	"encoding/binary"

	"testing"

	"github.com/ravisuhag/astro/pkg/crc"
)

// TestComputeCRC32Syndrome ties ComputeCRC32 to its CRC-appended-codeword
// use (CCSDS 211.2-B-3 annex C: the Proximity-1 CRC-32 attached after the
// message it covers). Current USLP has no 32-bit FECF, the option existed
// only in the retired 732.1-B-1.
func TestComputeCRC32Syndrome(t *testing.T) {
	// An arbitrary message; the point is the append-and-check procedure
	// over the message bytes.
	frame := []byte{
		0xC0, 0x00, 0x00, 0x01, 0x02, 0x00, 0x0D,
		0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x42,
	}
	got := crc.ComputeCRC32(frame)

	// Codeword procedure: append the CRC big-endian. For this MSB-first,
	// zero-preset CRC the syndrome of a correct codeword is zero (CCSDS
	// 211.2-B-3 annex C2.2), so running the CRC over message plus FECF
	// must give zero.
	fecf := make([]byte, 4)
	binary.BigEndian.PutUint32(fecf, got)
	codeword := append(append([]byte{}, frame...), fecf...)
	if syndrome := crc.ComputeCRC32(codeword); syndrome != 0 {
		t.Fatalf("syndrome over frame+FECF = 0x%08X, want 0", syndrome)
	}
}
