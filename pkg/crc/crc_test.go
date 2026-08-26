package crc_test

import (
	"encoding/binary"

	"testing"

	"github.com/ravisuhag/astro/pkg/crc"
)

func TestComputeCRC16(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{
			name: "standard ASCII 123456789",
			data: []byte("123456789"),
			want: 0x29B1,
		},
		{
			name: "empty input",
			data: []byte{},
			want: 0xFFFF,
		},
		{
			name: "single zero byte",
			data: []byte{0x00},
			want: 0xE1F0,
		},
		{
			name: "single 0xFF byte",
			data: []byte{0xFF},
			want: 0xFF00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crc.ComputeCRC16(tt.data)
			if got != tt.want {
				t.Errorf("ComputeCRC16(%x) = 0x%04X, want 0x%04X", tt.data, got, tt.want)
			}
		})
	}
}

func TestComputeCRC32(t *testing.T) {
	// Known-answer values for the Proximity-1 CRC-32 (CCSDS 211.2-B-3
	// annex C: poly 0x00A00805 MSB-first, zero preset, no reflection, no
	// final XOR), hand-computed with an independent bitwise implementation
	// and cross-checked against the implementation in pkg/pxsc.
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{
			name: "standard ASCII 123456789",
			data: []byte("123456789"),
			want: 0x51693C0C,
		},
		{
			name: "empty input",
			data: []byte{},
			want: 0x00000000,
		},
		{
			name: "single zero byte (zero preset keeps it zero)",
			data: []byte{0x00},
			want: 0x00000000,
		},
		{
			name: "four 0xFF bytes",
			data: []byte{0xFF, 0xFF, 0xFF, 0xFF},
			want: 0x71800472,
		},
		{
			name: "deadbeef",
			data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
			want: 0x4440D14C,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crc.ComputeCRC32(tt.data)
			if got != tt.want {
				t.Errorf("ComputeCRC32(%x) = 0x%08X, want 0x%08X", tt.data, got, tt.want)
			}
		})
	}
}

// TestComputeCRC32Syndrome ties ComputeCRC32 to its CRC-appended-codeword
// use (CCSDS 211.2-B-3 annex C: the Proximity-1 CRC-32 attached after the
// message it covers). Current USLP has no 32-bit FECF — the option existed
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
