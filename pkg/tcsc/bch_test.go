package tcsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tcsc"
)

func TestBCHEncode_CodeblockSize(t *testing.T) {
	info := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cb := tcsc.BCHEncode(info)
	if len(cb) != tcsc.CodeblockBytes {
		t.Fatalf("codeblock length = %d, want %d", len(cb), tcsc.CodeblockBytes)
	}
	// Information bytes must be preserved.
	for i := range tcsc.InfoBytes {
		if cb[i] != info[i] {
			t.Errorf("info byte %d = 0x%02X, want 0x%02X", i, cb[i], info[i])
		}
	}
}

func TestBCHEncode_ParityNotZero(t *testing.T) {
	// Non-zero data should produce non-zero parity.
	info := []byte{0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF}
	cb := tcsc.BCHEncode(info)
	if cb[tcsc.InfoBytes] == 0 {
		t.Error("parity byte should not be zero for non-trivial data")
	}
}

func TestBCHEncode_AllZeros(t *testing.T) {
	info := make([]byte, tcsc.InfoBytes)
	cb := tcsc.BCHEncode(info)
	// For all-zero information, the parity should also be zero.
	// The filler bit (complement of LSB of parity) should be 1.
	if cb[tcsc.InfoBytes] != 0x01 {
		t.Errorf("all-zero parity byte = 0x%02X, want 0x01 (filler=1)", cb[tcsc.InfoBytes])
	}
}

func TestBCHDecode_NoErrors(t *testing.T) {
	info := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cb := tcsc.BCHEncode(info)

	decoded, corr, err := tcsc.BCHDecode(cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr != 0 {
		t.Errorf("corrections = %d, want 0", corr)
	}
	for i := range tcsc.InfoBytes {
		if decoded[i] != info[i] {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, decoded[i], info[i])
		}
	}
}

func TestBCHDecode_SingleBitError_InfoByte(t *testing.T) {
	info := []byte{0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56, 0x78}
	cb := tcsc.BCHEncode(info)

	// Flip bit 4 of byte 2 (an information byte).
	cb[2] ^= 0x10

	decoded, corr, err := tcsc.BCHDecode(cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr != 1 {
		t.Errorf("corrections = %d, want 1", corr)
	}
	for i := range tcsc.InfoBytes {
		if decoded[i] != info[i] {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, decoded[i], info[i])
		}
	}
}

func TestBCHDecode_SingleBitError_ParityByte(t *testing.T) {
	info := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
	cb := tcsc.BCHEncode(info)

	// Flip a bit in the parity byte (bit 3).
	cb[tcsc.InfoBytes] ^= 0x08

	decoded, corr, err := tcsc.BCHDecode(cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if corr != 1 {
		t.Errorf("corrections = %d, want 1", corr)
	}
	for i := range tcsc.InfoBytes {
		if decoded[i] != info[i] {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, decoded[i], info[i])
		}
	}
}

func TestBCHDecode_TwoBitErrors_Uncorrectable(t *testing.T) {
	info := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	cb := tcsc.BCHEncode(info)

	// Flip two bits.
	cb[0] ^= 0x80
	cb[1] ^= 0x01

	_, _, err := tcsc.BCHDecode(cb)
	if err == nil {
		t.Fatal("expected error for 2-bit error, got nil")
	}
}

func TestBCHEncodeDecode_AllInfoBytes(t *testing.T) {
	// Test with several different information patterns.
	patterns := [][]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA},
		{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE},
		{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
	}

	for _, info := range patterns {
		cb := tcsc.BCHEncode(info)
		decoded, corr, err := tcsc.BCHDecode(cb)
		if err != nil {
			t.Errorf("info %x: unexpected error: %v", info, err)
			continue
		}
		if corr != 0 {
			t.Errorf("info %x: corrections = %d, want 0", info, corr)
		}
		for i := range tcsc.InfoBytes {
			if decoded[i] != info[i] {
				t.Errorf("info %x: byte %d = 0x%02X, want 0x%02X", info, i, decoded[i], info[i])
			}
		}
	}
}

func TestBCHDecode_SingleBitError_EachPosition(t *testing.T) {
	// Test single-bit correction at every position in the 56 info bits.
	info := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0x42}
	cb := tcsc.BCHEncode(info)

	for byteIdx := range tcsc.InfoBytes {
		for bitIdx := range 8 {
			corrupted := cb
			corrupted[byteIdx] ^= 1 << uint(bitIdx)

			decoded, corr, err := tcsc.BCHDecode(corrupted)
			if err != nil {
				t.Errorf("pos byte=%d bit=%d: error: %v", byteIdx, bitIdx, err)
				continue
			}
			if corr != 1 {
				t.Errorf("pos byte=%d bit=%d: corrections = %d, want 1", byteIdx, bitIdx, corr)
			}
			for i := range tcsc.InfoBytes {
				if decoded[i] != info[i] {
					t.Errorf("pos byte=%d bit=%d: decoded[%d] = 0x%02X, want 0x%02X",
						byteIdx, bitIdx, i, decoded[i], info[i])
				}
			}
		}
	}
}
