package epp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func TestNewIdlePacket(t *testing.T) {
	pkt, err := epp.NewIdlePacket()
	if err != nil {
		t.Fatalf("NewIdlePacket failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected IsIdle()=true")
	}
	if pkt.Header.ProtocolID != epp.ProtocolIDIdle {
		t.Errorf("ProtocolID = %d, want %d", pkt.Header.ProtocolID, epp.ProtocolIDIdle)
	}
	if len(pkt.Data) != 0 {
		t.Errorf("Expected empty data, got %d bytes", len(pkt.Data))
	}
}

func TestNewIdlePacketEncodeDecode(t *testing.T) {
	pkt, err := epp.NewIdlePacket()
	if err != nil {
		t.Fatalf("NewIdlePacket failed: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encoded) != 1 {
		t.Fatalf("Expected 1 byte, got %d", len(encoded))
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !decoded.IsIdle() {
		t.Error("Decoded packet should be idle")
	}
}

func TestIdlePacketWithDataFails(t *testing.T) {
	_, err := epp.NewPacket(epp.ProtocolIDIdle, []byte{0x01})
	if !errors.Is(err, epp.ErrIdleWithData) {
		t.Errorf("Expected ErrIdleWithData, got %v", err)
	}
}

func TestNewIPEPacketFormat2(t *testing.T) {
	data := []byte{0x45, 0x00, 0x00, 0x14} // IPv4 header start
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	if pkt.Header.ProtocolID != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", pkt.Header.ProtocolID, epp.ProtocolIDIPE)
	}
	if pkt.Header.Format() != 2 {
		t.Errorf("Format = %d, want 2", pkt.Header.Format())
	}
	if pkt.Header.PacketLength != uint32(2+len(data)) {
		t.Errorf("PacketLength = %d, want %d", pkt.Header.PacketLength, 2+len(data))
	}
}

func TestFormat2EncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Header.ProtocolID != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", decoded.Header.ProtocolID, epp.ProtocolIDIPE)
	}
	if !bytes.Equal(decoded.Data, data) {
		t.Errorf("Data mismatch. Got %v, want %v", decoded.Data, data)
	}
}

func TestFormat3EncodeDecode(t *testing.T) {
	data := make([]byte, 300) // exceeds 8-bit length → need Format 3
	data[0] = 0xAA
	data[299] = 0xBB

	pkt, err := epp.NewIPEPacket(data, epp.WithLongLength())
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	if pkt.Header.Format() != 3 {
		t.Errorf("Format = %d, want 3", pkt.Header.Format())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.Data, data) {
		t.Error("Data mismatch after round-trip")
	}
}

func TestFormat3WithUserDefined(t *testing.T) {
	data := []byte{0x01, 0x02}
	pkt, err := epp.NewUserDefinedPacket(data, epp.WithUserDefined(0xFE))
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if pkt.Header.Format() != 3 {
		t.Errorf("Format = %d, want 3", pkt.Header.Format())
	}
	if pkt.Header.UserDefined != 0xFE {
		t.Errorf("UserDefined = 0x%02X, want 0xFE", pkt.Header.UserDefined)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.UserDefined != 0xFE {
		t.Errorf("Decoded UserDefined = 0x%02X, want 0xFE", decoded.Header.UserDefined)
	}
}

func TestFormat4EncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithExtendedProtocolID(42))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Format() != 4 {
		t.Errorf("Format = %d, want 4", pkt.Header.Format())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.ExtendedProtocolID != 42 {
		t.Errorf("ExtendedProtocolID = %d, want 42", decoded.Header.ExtendedProtocolID)
	}
	if !bytes.Equal(decoded.Data, data) {
		t.Errorf("Data mismatch")
	}
}

func TestFormat5EncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithCCSDSDefined(55, 0x9876))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Format() != 5 {
		t.Errorf("Format = %d, want 5", pkt.Header.Format())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.ExtendedProtocolID != 55 {
		t.Errorf("ExtendedProtocolID = %d, want 55", decoded.Header.ExtendedProtocolID)
	}
	if decoded.Header.CCSDSDefined != 0x9876 {
		t.Errorf("CCSDSDefined = 0x%04X, want 0x9876", decoded.Header.CCSDSDefined)
	}
	if !bytes.Equal(decoded.Data, data) {
		t.Error("Data mismatch")
	}
}

func TestFormat2MaxData(t *testing.T) {
	// Format 2: 2-byte header + data, total ≤ 255
	data := make([]byte, 253) // 2 + 253 = 255
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("Max format 2 packet failed: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 255 {
		t.Errorf("Encoded length = %d, want 255", len(encoded))
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 253 {
		t.Errorf("Decoded data length = %d, want 253", len(decoded.Data))
	}
}

func TestFormat2ExceedsMax(t *testing.T) {
	// 2-byte header + 254 bytes data = 256 > 255
	data := make([]byte, 254)
	_, err := epp.NewIPEPacket(data)
	if !errors.Is(err, epp.ErrPacketTooLarge) {
		t.Errorf("Expected ErrPacketTooLarge, got %v", err)
	}
}

func TestNonIdleEmptyDataFails(t *testing.T) {
	_, err := epp.NewPacket(epp.ProtocolIDIPE, nil)
	if !errors.Is(err, epp.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}

	_, err = epp.NewPacket(epp.ProtocolIDIPE, []byte{})
	if !errors.Is(err, epp.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestInvalidProtocolID(t *testing.T) {
	_, err := epp.NewPacket(8, []byte{0x01})
	if !errors.Is(err, epp.ErrInvalidProtocolID) {
		t.Errorf("Expected ErrInvalidProtocolID, got %v", err)
	}
}

func TestDecodeDataTooShort(t *testing.T) {
	_, err := epp.Decode(nil)
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}

	_, err = epp.Decode([]byte{})
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}
}

func TestDecodeTruncatedPacket(t *testing.T) {
	// Create a valid format 2 packet, then truncate
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02, 0x03})
	encoded, _ := pkt.Encode()

	// Truncate: only header, no data
	_, err := epp.Decode(encoded[:2])
	if !errors.Is(err, epp.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort for truncated packet, got %v", err)
	}
}

func TestIsIdle(t *testing.T) {
	idle, _ := epp.NewIdlePacket()
	if !idle.IsIdle() {
		t.Error("Expected IsIdle()=true for idle packet")
	}

	nonIdle, _ := epp.NewIPEPacket([]byte{0x01})
	if nonIdle.IsIdle() {
		t.Error("Expected IsIdle()=false for IPE packet")
	}
}

func TestPacketSizer(t *testing.T) {
	// Idle packet
	idle, _ := epp.NewIdlePacket()
	idleBytes, _ := idle.Encode()
	if got := epp.PacketSizer(idleBytes); got != 1 {
		t.Errorf("PacketSizer(idle) = %d, want 1", got)
	}

	// Format 2 packet
	f2, _ := epp.NewIPEPacket([]byte{0x01, 0x02, 0x03})
	f2Bytes, _ := f2.Encode()
	if got := epp.PacketSizer(f2Bytes); got != len(f2Bytes) {
		t.Errorf("PacketSizer(format2) = %d, want %d", got, len(f2Bytes))
	}

	// Format 5 packet
	f5, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01}, epp.WithCCSDSDefined(10, 0))
	f5Bytes, _ := f5.Encode()
	if got := epp.PacketSizer(f5Bytes); got != len(f5Bytes) {
		t.Errorf("PacketSizer(format5) = %d, want %d", got, len(f5Bytes))
	}

	// Too short
	if got := epp.PacketSizer(nil); got != -1 {
		t.Errorf("PacketSizer(nil) = %d, want -1", got)
	}
	if got := epp.PacketSizer([]byte{}); got != -1 {
		t.Errorf("PacketSizer(empty) = %d, want -1", got)
	}
}

func TestHumanize(t *testing.T) {
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02})
	s := pkt.Humanize()
	if s == "" {
		t.Error("Humanize returned empty string")
	}
}

func TestAllProtocolIDsEncodeDecode(t *testing.T) {
	// Test non-reserved, non-idle protocol IDs
	pids := []struct {
		pid  uint8
		name string
	}{
		{epp.ProtocolIDIPE, "IPE"},
		{epp.ProtocolIDUserDef, "UserDef"},
	}

	for _, p := range pids {
		t.Run(p.name, func(t *testing.T) {
			data := []byte{0x01, 0x02}
			pkt, err := epp.NewPacket(p.pid, data)
			if err != nil {
				t.Fatalf("NewPacket failed: %v", err)
			}
			encoded, err := pkt.Encode()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := epp.Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Header.ProtocolID != p.pid {
				t.Errorf("ProtocolID = %d, want %d", decoded.Header.ProtocolID, p.pid)
			}
			if !bytes.Equal(decoded.Data, data) {
				t.Error("Data mismatch")
			}
		})
	}
}

func TestNewUserDefinedPacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	pkt, err := epp.NewUserDefinedPacket(data)
	if err != nil {
		t.Fatalf("NewUserDefinedPacket failed: %v", err)
	}
	if pkt.Header.ProtocolID != epp.ProtocolIDUserDef {
		t.Errorf("ProtocolID = %d, want %d", pkt.Header.ProtocolID, epp.ProtocolIDUserDef)
	}
}

func TestFormat5LargePacket(t *testing.T) {
	// Create a packet that requires 32-bit length (> 65535 bytes)
	data := make([]byte, 70000)
	data[0] = 0xAA
	data[69999] = 0xBB

	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithCCSDSDefined(1, 0))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if pkt.Header.Format() != 5 {
		t.Errorf("Format = %d, want 5", pkt.Header.Format())
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := epp.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Data[0] != 0xAA || decoded.Data[69999] != 0xBB {
		t.Error("Large packet data corrupted")
	}
}

func TestValidateMismatchedLength(t *testing.T) {
	pkt := &epp.EncapsulationPacket{
		Header: epp.Header{
			PVN:            epp.PVN,
			ProtocolID:     epp.ProtocolIDIPE,
			LengthOfLength: 0,
			PacketLength:   100, // wrong — doesn't match data
		},
		Data: []byte{0x01, 0x02},
	}
	if err := pkt.Validate(); err == nil {
		t.Error("Expected validation error for mismatched length")
	}
}

func TestDecodeExtraTrailingData(t *testing.T) {
	// Valid packet followed by extra bytes — Decode should only consume the packet
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02})
	encoded, _ := pkt.Encode()

	withExtra := append(encoded, 0xFF, 0xFF, 0xFF)
	decoded, err := epp.Decode(withExtra)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(decoded.Data, []byte{0x01, 0x02}) {
		t.Error("Decoded data should match original, ignoring trailing bytes")
	}
}

// --- Edge cases found during code review ---

func TestDecodeMalformedPacketLengthLessThanHeader(t *testing.T) {
	// Format 2 header: PVN=7, PID=2, LoL=0, PacketLength=1 (less than header size of 2)
	// This previously caused a panic via invalid slice indices.
	data := []byte{0x74, 0x01}
	_, err := epp.Decode(data)
	if !errors.Is(err, epp.ErrPacketLengthMismatch) {
		t.Errorf("Expected ErrPacketLengthMismatch for PacketLength < headerSize, got %v", err)
	}
}

func TestPacketSizerMalformedLength(t *testing.T) {
	// Format 2 with PacketLength=0 (less than header size 2) should return -1
	data := []byte{0x74, 0x00}
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(malformed) = %d, want -1", got)
	}

	// Format 2 with PacketLength=1 (less than header size 2) should return -1
	data = []byte{0x74, 0x01}
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(malformed) = %d, want -1", got)
	}
}

func TestPacketSizerFormat3(t *testing.T) {
	pkt, _ := epp.NewIPEPacket([]byte{0x01, 0x02}, epp.WithLongLength())
	encoded, _ := pkt.Encode()
	if got := epp.PacketSizer(encoded); got != len(encoded) {
		t.Errorf("PacketSizer(format3) = %d, want %d", got, len(encoded))
	}
}

func TestPacketSizerFormat4(t *testing.T) {
	pkt, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01}, epp.WithExtendedProtocolID(10))
	encoded, _ := pkt.Encode()
	if got := epp.PacketSizer(encoded); got != len(encoded) {
		t.Errorf("PacketSizer(format4) = %d, want %d", got, len(encoded))
	}
}

func TestPacketSizerTruncatedFormat4(t *testing.T) {
	// Format 4 needs 4 bytes, provide only 1
	data := []byte{0x7E} // PVN=7, PID=7, LoL=0 → Format 4
	if got := epp.PacketSizer(data); got != -1 {
		t.Errorf("PacketSizer(truncated format4) = %d, want -1", got)
	}
}

func TestReservedProtocolIDs(t *testing.T) {
	// Reserved PIDs (1, 3, 4, 5) should still encode/decode without error
	reserved := []uint8{1, 3, 4, 5}
	for _, pid := range reserved {
		pkt, err := epp.NewPacket(pid, []byte{0x01})
		if err != nil {
			t.Fatalf("NewPacket(PID=%d) failed: %v", pid, err)
		}
		encoded, err := pkt.Encode()
		if err != nil {
			t.Fatalf("Encode(PID=%d) failed: %v", pid, err)
		}
		decoded, err := epp.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(PID=%d) failed: %v", pid, err)
		}
		if decoded.Header.ProtocolID != pid {
			t.Errorf("PID = %d, want %d", decoded.Header.ProtocolID, pid)
		}
	}
}

func TestHeaderValidateInvalidLengthOfLength(t *testing.T) {
	h := epp.Header{PVN: epp.PVN, ProtocolID: 2, LengthOfLength: 2}
	if err := h.Validate(); err != epp.ErrInvalidLengthOfLength {
		t.Errorf("Expected ErrInvalidLengthOfLength, got %v", err)
	}
}

func TestWithLongLengthAndExtendedPID(t *testing.T) {
	// WithLongLength on an extended PID should produce Format 5
	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01},
		epp.WithExtendedProtocolID(42),
		epp.WithLongLength(),
	)
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}
	if pkt.Header.Format() != 5 {
		t.Errorf("Format = %d, want 5", pkt.Header.Format())
	}
}

func TestIdleWithLongLengthForcesFormat1(t *testing.T) {
	// PID=0 must always produce Format 1 idle, even if WithLongLength is used
	pkt, err := epp.NewPacket(epp.ProtocolIDIdle, nil, epp.WithLongLength())
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected IsIdle()=true")
	}
	if pkt.Header.LengthOfLength != 0 {
		t.Errorf("LengthOfLength = %d, want 0 (forced to Format 1)", pkt.Header.LengthOfLength)
	}
	if pkt.Header.Format() != 1 {
		t.Errorf("Format = %d, want 1", pkt.Header.Format())
	}
}

func TestIdleWithDataAndLongLengthFails(t *testing.T) {
	// PID=0 with data must fail regardless of LoL
	_, err := epp.NewPacket(epp.ProtocolIDIdle, []byte{0x01}, epp.WithLongLength())
	if !errors.Is(err, epp.ErrIdleWithData) {
		t.Errorf("Expected ErrIdleWithData, got %v", err)
	}
}

func TestHumanizeAllFormats(t *testing.T) {
	// Format 1 — idle
	idle, _ := epp.NewIdlePacket()
	if s := idle.Humanize(); s == "" {
		t.Error("Humanize(idle) returned empty")
	}

	// Format 3 — medium with user-defined
	f3, _ := epp.NewIPEPacket([]byte{0x01}, epp.WithUserDefined(0xAB))
	s := f3.Humanize()
	if s == "" {
		t.Error("Humanize(format3) returned empty")
	}

	// Format 4 — extended medium
	f4, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01}, epp.WithExtendedProtocolID(42))
	if s := f4.Humanize(); s == "" {
		t.Error("Humanize(format4) returned empty")
	}

	// Format 5 — extended long
	f5, _ := epp.NewPacket(epp.ProtocolIDExtended, []byte{0x01}, epp.WithCCSDSDefined(42, 0x1234))
	if s := f5.Humanize(); s == "" {
		t.Error("Humanize(format5) returned empty")
	}
}
