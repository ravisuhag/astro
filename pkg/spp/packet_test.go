package spp_test

import (
	"bytes"
	"encoding/binary"
	spp2 "github.com/ravisuhag/astro/pkg/spp"
	"testing"
)

// testSecondaryHeader is a simple mission-specific secondary header for testing.
type testSecondaryHeader struct {
	Timestamp uint64
}

func (h *testSecondaryHeader) Encode() ([]byte, error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, h.Timestamp)
	return buf, nil
}

func (h *testSecondaryHeader) Decode(data []byte) error {
	if len(data) < 8 {
		return spp2.ErrDataTooShort
	}
	h.Timestamp = binary.BigEndian.Uint64(data[:8])
	return nil
}

func (h *testSecondaryHeader) Size() int {
	return 8
}

func TestNewSpacePacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	if packet.PrimaryHeader.APID != 100 {
		t.Errorf("Expected APID 100, got %d", packet.PrimaryHeader.APID)
	}

	if !bytes.Equal(packet.UserData, data) {
		t.Errorf("User data does not match. Got %v, want %v", packet.UserData, data)
	}
}

func TestSpacePacketEncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode space packet: %v", err)
	}

	decoded, err := spp2.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode space packet: %v", err)
	}

	if packet.PrimaryHeader != decoded.PrimaryHeader {
		t.Errorf("Primary header does not match. Got %+v, want %+v", decoded.PrimaryHeader, packet.PrimaryHeader)
	}

	if !bytes.Equal(packet.UserData, decoded.UserData) {
		t.Errorf("User data does not match. Got %v, want %v", decoded.UserData, packet.UserData)
	}
}

func TestSpacePacketWithSecondaryHeader(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("Failed to create new space packet with secondary header: %v", err)
	}

	if packet.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Errorf("Expected SecondaryHeaderFlag 1, got %d", packet.PrimaryHeader.SecondaryHeaderFlag)
	}

	if packet.SecondaryHeader == nil {
		t.Fatal("Expected secondary header, got nil")
	}
}

func TestSpacePacketWithErrorControl(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	crc := uint16(0xABCD)
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithErrorControl(crc))
	if err != nil {
		t.Fatalf("Failed to create new space packet with error control: %v", err)
	}

	if packet.ErrorControl == nil || *packet.ErrorControl != crc {
		t.Errorf("Error control does not match. Got %v, want %v", packet.ErrorControl, crc)
	}
}

func TestSpacePacketWithSecondaryHeaderEncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	decodedSH := &testSecondaryHeader{}
	decoded, err := spp2.Decode(encoded, decodedSH)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if !bytes.Equal(packet.UserData, decoded.UserData) {
		t.Errorf("UserData mismatch. Got %v, want %v", decoded.UserData, packet.UserData)
	}
	if decoded.SecondaryHeader == nil {
		t.Fatal("Expected secondary header, got nil")
	}
	if decodedSH.Timestamp != sh.Timestamp {
		t.Errorf("Timestamp mismatch. Got %d, want %d", decodedSH.Timestamp, sh.Timestamp)
	}
}

func TestSpacePacketWithSecondaryHeaderDecodeWithoutDecoder(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode without providing a secondary header decoder
	decoded, err := spp2.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Secondary header bytes should be included in UserData
	if decoded.SecondaryHeader != nil {
		t.Error("Expected nil secondary header when no decoder provided")
	}

	// UserData should contain secondary header bytes + original user data
	expectedLen := 8 + len(data) // 8 bytes timestamp + 3 bytes data
	if len(decoded.UserData) != expectedLen {
		t.Errorf("Expected UserData length %d, got %d", expectedLen, len(decoded.UserData))
	}
}

func TestPacketLengthIncludesAllFields(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	crc := uint16(0xABCD)
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(sh), spp2.WithErrorControl(crc))
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Per CCSDS: total packet = PrimaryHeader(6) + PacketLength + 1
	expectedTotal := 6 + int(packet.PrimaryHeader.PacketLength) + 1
	if len(encoded) != expectedTotal {
		t.Errorf("Encoded size %d != expected %d (6 + PacketLength(%d) + 1)",
			len(encoded), expectedTotal, packet.PrimaryHeader.PacketLength)
	}
}

func TestSpacePacketValidate(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	if err := packet.Validate(); err != nil {
		t.Errorf("Expected packet to be valid, but got error: %v", err)
	}

	// Test case: Invalid APID
	packet.PrimaryHeader.APID = 3000
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for invalid APID, but got none")
	}
	packet.PrimaryHeader.APID = 100

	// Test case: Invalid user data length
	packet.UserData = []byte{0x01, 0x02}
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for mismatched user data length, but got none")
	}
	packet.UserData = data

	// Test case: Invalid packet length
	packet.PrimaryHeader.PacketLength = 65535
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for packet length exceeding maximum, but got none")
	}
	packet.PrimaryHeader.PacketLength = uint16(len(data)) - 1

	// Test case: Secondary header flag set but no secondary header struct.
	// This is valid — it represents a decoded packet where no decoder was provided,
	// so the secondary header bytes are included in UserData.
	packet.PrimaryHeader.SecondaryHeaderFlag = 1
	if err := packet.Validate(); err != nil {
		t.Errorf("Expected valid packet (secondary header bytes in UserData), but got error: %v", err)
	}
	packet.PrimaryHeader.SecondaryHeaderFlag = 0

	// Test case: Valid secondary header
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	packet.SecondaryHeader = sh
	packet.PrimaryHeader.SecondaryHeaderFlag = 1
	packet.PrimaryHeader.PacketLength = uint16(len(data)+8) - 1
	if err := packet.Validate(); err != nil {
		t.Errorf("Expected packet to be valid with secondary header, but got error: %v", err)
	}
}

func TestNewTMPacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewTMPacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create TM packet: %v", err)
	}
	if packet.PrimaryHeader.Type != spp2.PacketTypeTM {
		t.Errorf("Expected TM type %d, got %d", spp2.PacketTypeTM, packet.PrimaryHeader.Type)
	}
}

func TestNewTCPacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewTCPacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create TC packet: %v", err)
	}
	if packet.PrimaryHeader.Type != spp2.PacketTypeTC {
		t.Errorf("Expected TC type %d, got %d", spp2.PacketTypeTC, packet.PrimaryHeader.Type)
	}
}

func TestPacketConstants(t *testing.T) {
	if spp2.PacketTypeTM != 0 {
		t.Errorf("PacketTypeTM should be 0, got %d", spp2.PacketTypeTM)
	}
	if spp2.PacketTypeTC != 1 {
		t.Errorf("PacketTypeTC should be 1, got %d", spp2.PacketTypeTC)
	}
	if spp2.SeqFlagContinuation != 0 {
		t.Errorf("SeqFlagContinuation should be 0, got %d", spp2.SeqFlagContinuation)
	}
	if spp2.SeqFlagFirstSegment != 1 {
		t.Errorf("SeqFlagFirstSegment should be 1, got %d", spp2.SeqFlagFirstSegment)
	}
	if spp2.SeqFlagLastSegment != 2 {
		t.Errorf("SeqFlagLastSegment should be 2, got %d", spp2.SeqFlagLastSegment)
	}
	if spp2.SeqFlagUnsegmented != 3 {
		t.Errorf("SeqFlagUnsegmented should be 3, got %d", spp2.SeqFlagUnsegmented)
	}
}

func TestNewSpacePacketC1SecondaryHeaderOnly(t *testing.T) {
	// C1: A packet with a secondary header and no user data should be valid
	sh := &testSecondaryHeader{Timestamp: 1234567890}
	packet, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, nil, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("Expected valid packet with secondary header only, got error: %v", err)
	}

	if packet.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Error("Expected secondary header flag to be set")
	}
	if len(packet.UserData) != 0 {
		t.Errorf("Expected empty user data, got %d bytes", len(packet.UserData))
	}

	// Round-trip encode/decode
	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := spp2.Decode(encoded, &testSecondaryHeader{})
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.PrimaryHeader.APID != 100 {
		t.Errorf("APID mismatch after round-trip. Got %d, want 100", decoded.PrimaryHeader.APID)
	}
}

func TestWithSequenceCount(t *testing.T) {
	packet, err := spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSequenceCount(42))
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}
	if packet.PrimaryHeader.SequenceCount != 42 {
		t.Errorf("Sequence count = %d, want 42", packet.PrimaryHeader.SequenceCount)
	}

	// Invalid sequence count
	_, err = spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSequenceCount(16384))
	if err == nil {
		t.Error("Expected error for sequence count > 16383")
	}
}

func TestWithSequenceFlags(t *testing.T) {
	packet, err := spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSequenceFlags(spp2.SeqFlagFirstSegment))
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}
	if packet.PrimaryHeader.SequenceFlags != spp2.SeqFlagFirstSegment {
		t.Errorf("Sequence flags = %d, want %d", packet.PrimaryHeader.SequenceFlags, spp2.SeqFlagFirstSegment)
	}

	// Invalid sequence flags
	_, err = spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSequenceFlags(4))
	if err == nil {
		t.Error("Expected error for sequence flags > 3")
	}
}

func TestNewSpacePacketC2NoSecondaryHeaderNoData(t *testing.T) {
	// C2: A packet with no secondary header AND no user data must be rejected
	_, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, nil)
	if err == nil {
		t.Fatal("Expected error for packet with no secondary header and no user data")
	}

	_, err = spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{})
	if err == nil {
		t.Fatal("Expected error for packet with no secondary header and empty user data")
	}
}
