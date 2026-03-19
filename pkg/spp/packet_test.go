package spp_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	spp2 "github.com/ravisuhag/astro/pkg/spp"
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
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithErrorControl())
	if err != nil {
		t.Fatalf("Failed to create new space packet with error control: %v", err)
	}

	if packet.ErrorControl == nil {
		t.Fatal("Expected ErrorControl to be set")
	}
}

func TestSpacePacketWithErrorControlEncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}

	// CRC is auto-computed during Encode
	packet, err := spp2.NewTMPacket(100, data, spp2.WithErrorControl())
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Verify CRC was written back to the packet
	if *packet.ErrorControl == 0 {
		t.Error("Expected CRC to be computed, got 0")
	}

	// Decode with error control validation
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeErrorControl())
	if err != nil {
		t.Fatalf("Failed to decode with CRC: %v", err)
	}
	if decoded.ErrorControl == nil {
		t.Fatal("Expected ErrorControl to be set")
	}
	if *decoded.ErrorControl != *packet.ErrorControl {
		t.Errorf("CRC mismatch. Got 0x%04X, want 0x%04X", *decoded.ErrorControl, *packet.ErrorControl)
	}
	if !bytes.Equal(decoded.UserData, data) {
		t.Errorf("User data mismatch. Got %v, want %v", decoded.UserData, data)
	}
}

func TestSpacePacketWithErrorControlCorrupted(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewTMPacket(100, data, spp2.WithErrorControl())
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Corrupt a data byte
	encoded[7] ^= 0xFF

	// Decoding with error control should fail
	_, err = spp2.Decode(encoded, spp2.WithDecodeErrorControl())
	if err == nil {
		t.Fatal("Expected CRC validation error")
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
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(decodedSH))
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
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(sh), spp2.WithErrorControl())
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
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(&testSecondaryHeader{}))
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

// --- Idle Packet Detection ---

func TestIsIdle(t *testing.T) {
	idlePkt, err := spp2.NewSpacePacket(0x7FF, spp2.PacketTypeTM, []byte{0xFF})
	if err != nil {
		t.Fatalf("Failed to create idle packet: %v", err)
	}
	if !idlePkt.IsIdle() {
		t.Error("Expected IsIdle()=true for APID 0x7FF")
	}

	normalPkt, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	if normalPkt.IsIdle() {
		t.Error("Expected IsIdle()=false for APID 100")
	}

	// APID 0 is valid but not idle
	zeroPkt, err := spp2.NewSpacePacket(0, spp2.PacketTypeTM, []byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	if zeroPkt.IsIdle() {
		t.Error("Expected IsIdle()=false for APID 0")
	}
}

// --- Secondary Header Size Boundaries ---

// minSecondaryHeader is a 1-byte secondary header (CCSDS minimum).
type minSecondaryHeader struct{ Value uint8 }

func (h *minSecondaryHeader) Encode() ([]byte, error) { return []byte{h.Value}, nil }
func (h *minSecondaryHeader) Decode(data []byte) error {
	if len(data) < 1 {
		return spp2.ErrDataTooShort
	}
	h.Value = data[0]
	return nil
}
func (h *minSecondaryHeader) Size() int { return 1 }

// maxSecondaryHeader is a 63-byte secondary header (CCSDS maximum).
type maxSecondaryHeader struct{ Data [63]byte }

func (h *maxSecondaryHeader) Encode() ([]byte, error) { return h.Data[:], nil }
func (h *maxSecondaryHeader) Decode(data []byte) error {
	if len(data) < 63 {
		return spp2.ErrDataTooShort
	}
	copy(h.Data[:], data[:63])
	return nil
}
func (h *maxSecondaryHeader) Size() int { return 63 }

// zeroSecondaryHeader has size 0 (below CCSDS minimum).
type zeroSecondaryHeader struct{}

func (h *zeroSecondaryHeader) Encode() ([]byte, error) { return nil, nil }
func (h *zeroSecondaryHeader) Decode([]byte) error      { return nil }
func (h *zeroSecondaryHeader) Size() int                 { return 0 }

// oversizedSecondaryHeader has size 64 (above CCSDS maximum).
type oversizedSecondaryHeader struct{ Data [64]byte }

func (h *oversizedSecondaryHeader) Encode() ([]byte, error) { return h.Data[:], nil }
func (h *oversizedSecondaryHeader) Decode(data []byte) error {
	copy(h.Data[:], data)
	return nil
}
func (h *oversizedSecondaryHeader) Size() int { return 64 }

func TestSecondaryHeaderMinSize(t *testing.T) {
	sh := &minSecondaryHeader{Value: 0x42}
	pkt, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01}, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("1-byte secondary header should be valid: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(&minSecondaryHeader{}))
	if err != nil {
		t.Fatal(err)
	}
	decodedSH := decoded.SecondaryHeader.(*minSecondaryHeader)
	if decodedSH.Value != 0x42 {
		t.Errorf("SecondaryHeader value = 0x%02X, want 0x42", decodedSH.Value)
	}
}

func TestSecondaryHeaderMaxSize(t *testing.T) {
	sh := &maxSecondaryHeader{}
	for i := range 63 {
		sh.Data[i] = byte(i)
	}

	pkt, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01}, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("63-byte secondary header should be valid: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decodeSH := &maxSecondaryHeader{}
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(decodeSH))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SecondaryHeader == nil {
		t.Fatal("Expected non-nil secondary header")
	}
	if decodeSH.Data[0] != 0 || decodeSH.Data[62] != 62 {
		t.Error("63-byte secondary header data corrupted during round-trip")
	}
}

func TestSecondaryHeaderTooSmall(t *testing.T) {
	sh := &zeroSecondaryHeader{}
	_, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01}, spp2.WithSecondaryHeader(sh))
	if !errors.Is(err, spp2.ErrSecondaryHeaderTooSmall) {
		t.Errorf("Expected ErrSecondaryHeaderTooSmall, got %v", err)
	}
}

func TestSecondaryHeaderTooLarge(t *testing.T) {
	sh := &oversizedSecondaryHeader{}
	_, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01}, spp2.WithSecondaryHeader(sh))
	if !errors.Is(err, spp2.ErrSecondaryHeaderTooLarge) {
		t.Errorf("Expected ErrSecondaryHeaderTooLarge, got %v", err)
	}
}

func TestSecondaryHeaderOnlyMinMax(t *testing.T) {
	// 1-byte secondary header, no user data
	sh1 := &minSecondaryHeader{Value: 0xAA}
	pkt, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, nil, spp2.WithSecondaryHeader(sh1))
	if err != nil {
		t.Fatalf("1-byte SH only: %v", err)
	}
	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(&minSecondaryHeader{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.UserData) != 0 {
		t.Errorf("Expected no user data, got %d bytes", len(decoded.UserData))
	}

	// 63-byte secondary header, no user data
	sh63 := &maxSecondaryHeader{}
	for i := range 63 {
		sh63.Data[i] = byte(i)
	}
	pkt, err = spp2.NewSpacePacket(100, spp2.PacketTypeTM, nil, spp2.WithSecondaryHeader(sh63))
	if err != nil {
		t.Fatalf("63-byte SH only: %v", err)
	}
	encoded, err = pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decodeSH := &maxSecondaryHeader{}
	decoded, err = spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(decodeSH))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.UserData) != 0 {
		t.Errorf("Expected no user data, got %d bytes", len(decoded.UserData))
	}
	if decodeSH.Data[62] != 62 {
		t.Error("63-byte secondary header corrupted")
	}
}

// --- Maximum Packet Size ---

func TestMaximumPacketSize(t *testing.T) {
	maxData := make([]byte, 65536)
	maxData[0] = 0xDE
	maxData[65535] = 0xAD

	pkt, err := spp2.NewTMPacket(100, maxData)
	if err != nil {
		t.Fatalf("Max-size packet should be valid: %v", err)
	}
	if pkt.PrimaryHeader.PacketLength != 65535 {
		t.Errorf("PacketLength = %d, want 65535", pkt.PrimaryHeader.PacketLength)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 65542 {
		t.Errorf("Encoded length = %d, want 65542", len(encoded))
	}

	decoded, err := spp2.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode max-size packet: %v", err)
	}
	if decoded.UserData[0] != 0xDE || decoded.UserData[65535] != 0xAD {
		t.Error("Max-size packet data corrupted during round-trip")
	}
}

func TestMaximumPacketSizeWithErrorControl(t *testing.T) {
	maxData := make([]byte, 65534)
	maxData[0] = 0xCA
	maxData[65533] = 0xFE

	pkt, err := spp2.NewTMPacket(100, maxData, spp2.WithErrorControl())
	if err != nil {
		t.Fatalf("Max-size packet with CRC should be valid: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 65542 {
		t.Errorf("Encoded length = %d, want 65542", len(encoded))
	}

	decoded, err := spp2.Decode(encoded, spp2.WithDecodeErrorControl())
	if err != nil {
		t.Fatal(err)
	}
	if decoded.UserData[0] != 0xCA || decoded.UserData[65533] != 0xFE {
		t.Error("Data corrupted during round-trip")
	}
}

func TestPacketExceedsMaximumSize(t *testing.T) {
	oversized := make([]byte, 65537)
	_, err := spp2.NewTMPacket(100, oversized)
	if !errors.Is(err, spp2.ErrPacketTooLarge) {
		t.Errorf("Expected ErrPacketTooLarge for 65537-byte data, got %v", err)
	}
}

// --- SecondaryHeaderFlag=1 with nil Header ---

func TestEncodeSecondaryHeaderFlagWithNilHeader(t *testing.T) {
	pkt := &spp2.SpacePacket{
		PrimaryHeader: spp2.PrimaryHeader{
			Version:             0,
			Type:                0,
			SecondaryHeaderFlag: 1,
			APID:                100,
			SequenceFlags:       3,
			SequenceCount:       0,
			PacketLength:        2,
		},
		UserData: []byte{0x01, 0x02, 0x03},
	}

	_, err := pkt.Encode()
	if !errors.Is(err, spp2.ErrSecondaryHeaderMissing) {
		t.Errorf("Expected ErrSecondaryHeaderMissing, got %v", err)
	}
}

// --- CRC Round-Trip Validation ---

func TestCRCRoundTripAllPacketCombinations(t *testing.T) {
	// TM with CRC
	tmPkt, _ := spp2.NewTMPacket(100, []byte{0x01, 0x02, 0x03, 0x04}, spp2.WithErrorControl())
	encoded, _ := tmPkt.Encode()
	decoded, err := spp2.Decode(encoded, spp2.WithDecodeErrorControl())
	if err != nil {
		t.Fatalf("TM+CRC round-trip failed: %v", err)
	}
	if !bytes.Equal(decoded.UserData, tmPkt.UserData) {
		t.Error("TM+CRC data mismatch")
	}

	// TC with CRC
	tcPkt, _ := spp2.NewTCPacket(200, []byte{0x05, 0x06}, spp2.WithErrorControl())
	encoded, _ = tcPkt.Encode()
	decoded, err = spp2.Decode(encoded, spp2.WithDecodeErrorControl())
	if err != nil {
		t.Fatalf("TC+CRC round-trip failed: %v", err)
	}
	if decoded.PrimaryHeader.Type != spp2.PacketTypeTC {
		t.Error("TC type not preserved")
	}

	// TM with secondary header + CRC
	sh := &testSecondaryHeader{Timestamp: 0xDEADBEEFCAFEBABE}
	pkt, _ := spp2.NewTMPacket(300, []byte{0x07}, spp2.WithSecondaryHeader(sh), spp2.WithErrorControl())
	encoded, _ = pkt.Encode()
	decodeSH := &testSecondaryHeader{}
	_, err = spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(decodeSH), spp2.WithDecodeErrorControl())
	if err != nil {
		t.Fatalf("TM+SH+CRC round-trip failed: %v", err)
	}
	if decodeSH.Timestamp != 0xDEADBEEFCAFEBABE {
		t.Errorf("Secondary header timestamp = 0x%X, want 0xDEADBEEFCAFEBABE", decodeSH.Timestamp)
	}
}

func TestCRCDetectsCorruptionAtVariousOffsets(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	pkt, _ := spp2.NewTMPacket(100, data, spp2.WithErrorControl())
	encoded, _ := pkt.Encode()

	for i := 0; i < len(encoded)-2; i++ {
		corrupted := make([]byte, len(encoded))
		copy(corrupted, encoded)
		corrupted[i] ^= 0x01

		_, err := spp2.Decode(corrupted, spp2.WithDecodeErrorControl())
		if err == nil {
			t.Errorf("CRC should detect corruption at byte %d", i)
		}
	}
}

// --- All Sequence Flag Values Encode/Decode ---

func TestAllSequenceFlagsEncodeDecode(t *testing.T) {
	flags := []struct {
		flag uint8
		name string
	}{
		{spp2.SeqFlagContinuation, "continuation"},
		{spp2.SeqFlagFirstSegment, "first"},
		{spp2.SeqFlagLastSegment, "last"},
		{spp2.SeqFlagUnsegmented, "unsegmented"},
	}

	for _, f := range flags {
		t.Run(f.name, func(t *testing.T) {
			pkt, err := spp2.NewTMPacket(100, []byte{0x01},
				spp2.WithSequenceFlags(f.flag),
				spp2.WithSequenceCount(1000),
			)
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := pkt.Encode()
			if err != nil {
				t.Fatal(err)
			}

			decoded, err := spp2.Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}

			if decoded.PrimaryHeader.SequenceFlags != f.flag {
				t.Errorf("flags = %d, want %d", decoded.PrimaryHeader.SequenceFlags, f.flag)
			}
			if decoded.PrimaryHeader.SequenceCount != 1000 {
				t.Errorf("seq count = %d, want 1000", decoded.PrimaryHeader.SequenceCount)
			}
		})
	}
}
