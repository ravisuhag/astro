package epp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func TestHeaderFormat(t *testing.T) {
	tests := []struct {
		name           string
		protocolID     uint8
		lengthOfLength uint8
		wantFormat     int
		wantSize       int
	}{
		{"idle", epp.ProtocolIDIdle, 0, 1, 1},
		{"short IPE", epp.ProtocolIDIPE, 0, 2, 2},
		{"short user-defined", epp.ProtocolIDUserDef, 0, 2, 2},
		{"medium IPE", epp.ProtocolIDIPE, 1, 3, 4},
		{"medium user-defined", epp.ProtocolIDUserDef, 1, 3, 4},
		{"extended medium", epp.ProtocolIDExtended, 0, 4, 4},
		{"extended long", epp.ProtocolIDExtended, 1, 5, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := epp.Header{
				PVN:            epp.PVN,
				ProtocolID:     tt.protocolID,
				LengthOfLength: tt.lengthOfLength,
			}
			if got := h.Format(); got != tt.wantFormat {
				t.Errorf("Format() = %d, want %d", got, tt.wantFormat)
			}
			if got := h.Size(); got != tt.wantSize {
				t.Errorf("Size() = %d, want %d", got, tt.wantSize)
			}
		})
	}
}

func TestHeaderEncodeDecodeFormat1(t *testing.T) {
	h := epp.Header{
		PVN:            epp.PVN,
		ProtocolID:     epp.ProtocolIDIdle,
		LengthOfLength: 0,
		PacketLength:   1,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 1 {
		t.Fatalf("Expected 1 byte, got %d", len(encoded))
	}
	// First nibble should be 0111 = 0x70, PID=000, LoL=0
	if encoded[0] != 0x70 {
		t.Errorf("Expected 0x70, got 0x%02X", encoded[0])
	}

	var decoded epp.Header
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.PVN != epp.PVN {
		t.Errorf("PVN = %d, want %d", decoded.PVN, epp.PVN)
	}
	if decoded.ProtocolID != epp.ProtocolIDIdle {
		t.Errorf("ProtocolID = %d, want %d", decoded.ProtocolID, epp.ProtocolIDIdle)
	}
}

func TestHeaderEncodeDecodeFormat2(t *testing.T) {
	h := epp.Header{
		PVN:            epp.PVN,
		ProtocolID:     epp.ProtocolIDIPE,
		LengthOfLength: 0,
		PacketLength:   200,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 2 {
		t.Fatalf("Expected 2 bytes, got %d", len(encoded))
	}

	var decoded epp.Header
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.ProtocolID != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", decoded.ProtocolID, epp.ProtocolIDIPE)
	}
	if decoded.PacketLength != 200 {
		t.Errorf("PacketLength = %d, want 200", decoded.PacketLength)
	}
}

func TestHeaderEncodeDecodeFormat3(t *testing.T) {
	h := epp.Header{
		PVN:            epp.PVN,
		ProtocolID:     epp.ProtocolIDIPE,
		LengthOfLength: 1,
		UserDefined:    0xAB,
		PacketLength:   5000,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 4 {
		t.Fatalf("Expected 4 bytes, got %d", len(encoded))
	}

	var decoded epp.Header
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.UserDefined != 0xAB {
		t.Errorf("UserDefined = 0x%02X, want 0xAB", decoded.UserDefined)
	}
	if decoded.PacketLength != 5000 {
		t.Errorf("PacketLength = %d, want 5000", decoded.PacketLength)
	}
}

func TestHeaderEncodeDecodeFormat4(t *testing.T) {
	h := epp.Header{
		PVN:                epp.PVN,
		ProtocolID:         epp.ProtocolIDExtended,
		LengthOfLength:     0,
		ExtendedProtocolID: 42,
		PacketLength:       1024,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 4 {
		t.Fatalf("Expected 4 bytes, got %d", len(encoded))
	}

	var decoded epp.Header
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.ExtendedProtocolID != 42 {
		t.Errorf("ExtendedProtocolID = %d, want 42", decoded.ExtendedProtocolID)
	}
	if decoded.PacketLength != 1024 {
		t.Errorf("PacketLength = %d, want 1024", decoded.PacketLength)
	}
}

func TestHeaderEncodeDecodeFormat5(t *testing.T) {
	h := epp.Header{
		PVN:                epp.PVN,
		ProtocolID:         epp.ProtocolIDExtended,
		LengthOfLength:     1,
		ExtendedProtocolID: 99,
		CCSDSDefined:       0x1234,
		PacketLength:       100000,
	}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 8 {
		t.Fatalf("Expected 8 bytes, got %d", len(encoded))
	}

	var decoded epp.Header
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.ExtendedProtocolID != 99 {
		t.Errorf("ExtendedProtocolID = %d, want 99", decoded.ExtendedProtocolID)
	}
	if decoded.CCSDSDefined != 0x1234 {
		t.Errorf("CCSDSDefined = 0x%04X, want 0x1234", decoded.CCSDSDefined)
	}
	if decoded.PacketLength != 100000 {
		t.Errorf("PacketLength = %d, want 100000", decoded.PacketLength)
	}
}

func TestHeaderDecodeInvalidPVN(t *testing.T) {
	// PVN=0 (SPP-like), not 7
	data := []byte{0x00, 0x00}
	var h epp.Header
	err := h.Decode(data)
	if err != epp.ErrInvalidPVN {
		t.Errorf("Expected ErrInvalidPVN, got %v", err)
	}
}

func TestHeaderDecodeTooShort(t *testing.T) {
	var h epp.Header
	err := h.Decode(nil)
	if err != epp.ErrDataTooShort {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}

	err = h.Decode([]byte{})
	if err != epp.ErrDataTooShort {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
}

func TestHeaderDecodeTooShortForFormat(t *testing.T) {
	// Format 2 header (2 bytes) with only 1 byte of data
	data := []byte{0x74} // PVN=7, PID=2, LoL=0 → Format 2 needs 2 bytes
	var h epp.Header
	err := h.Decode(data)
	if err != epp.ErrDataTooShort {
		t.Errorf("Expected ErrDataTooShort for format 2 with 1 byte, got %v", err)
	}
}

func TestHeaderValidateInvalidPVN(t *testing.T) {
	h := epp.Header{PVN: 0, ProtocolID: 0}
	if err := h.Validate(); err != epp.ErrInvalidPVN {
		t.Errorf("Expected ErrInvalidPVN, got %v", err)
	}
}

func TestHeaderSize(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"idle", []byte{0x70}, 1},
		{"short", []byte{0x74}, 2},
		{"medium", []byte{0x75}, 4},
		{"ext medium", []byte{0x7E}, 4},
		{"ext long", []byte{0x7F}, 8},
		{"empty", []byte{}, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := epp.HeaderSize(tt.data); got != tt.want {
				t.Errorf("HeaderSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHeaderHumanize(t *testing.T) {
	h := epp.Header{
		PVN:            epp.PVN,
		ProtocolID:     epp.ProtocolIDIPE,
		LengthOfLength: 0,
		PacketLength:   100,
	}
	s := h.Humanize()
	if s == "" {
		t.Error("Humanize returned empty string")
	}
}

func TestHeaderOctet0BitLayout(t *testing.T) {
	// Verify the bit layout: PVN(4) | PID(3) | LoL(1)
	tests := []struct {
		name       string
		protocolID uint8
		lol        uint8
		wantByte0  byte
	}{
		{"idle", 0, 0, 0x70},          // 0111_000_0
		{"IPE short", 2, 0, 0x74},     // 0111_010_0
		{"IPE medium", 2, 1, 0x75},    // 0111_010_1
		{"user short", 6, 0, 0x7C},    // 0111_110_0
		{"ext medium", 7, 0, 0x7E},    // 0111_111_0
		{"ext long", 7, 1, 0x7F},      // 0111_111_1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := epp.Header{
				PVN:            epp.PVN,
				ProtocolID:     tt.protocolID,
				LengthOfLength: tt.lol,
				PacketLength:   10, // needs non-zero for non-idle
			}
			if tt.protocolID == 0 && tt.lol == 0 {
				h.PacketLength = 1
			}
			encoded, err := h.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if encoded[0] != tt.wantByte0 {
				t.Errorf("Byte 0 = 0x%02X, want 0x%02X", encoded[0], tt.wantByte0)
			}
		})
	}
}
