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

	// Test case: Secondary header flag set but no secondary header struct on a
	// packet the caller built. CCSDS 4.1.3.3.3.2 makes the flag the signal that
	// a header is present, so a hand-built packet that promises one and has
	// none is rejected. (A packet Decode produced in this state is legal and is
	// covered by TestDecodeEncodeRoundTripWithoutSecondaryHeaderDecoder.)
	packet.PrimaryHeader.SecondaryHeaderFlag = 1
	if err := packet.Validate(); !errors.Is(err, spp2.ErrSecondaryHeaderMissing) {
		t.Errorf("Validate(flag set, no header) = %v, want ErrSecondaryHeaderMissing", err)
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

// maxSecondaryHeader is a 63-byte secondary header.
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
func (h *zeroSecondaryHeader) Decode([]byte) error     { return nil }
func (h *zeroSecondaryHeader) Size() int               { return 0 }

// oversizedSecondaryHeader has size 64 (above the old, invented 63-octet cap).
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

func TestSecondaryHeaderOver63Accepted(t *testing.T) {
	// SPP-F2: CCSDS 133.0-B-2 sets no 63-octet cap on the secondary header
	// (that limit belongs to TM's FSH). A 64-byte secondary header is valid.
	sh := &oversizedSecondaryHeader{}
	pkt, err := spp2.NewSpacePacket(100, spp2.PacketTypeTM, []byte{0x01}, spp2.WithSecondaryHeader(sh))
	if err != nil {
		t.Fatalf("64-byte secondary header should be valid: %v", err)
	}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := spp2.Decode(encoded, spp2.WithDecodeSecondaryHeader(&oversizedSecondaryHeader{})); err != nil {
		t.Fatalf("Decode with 64-byte secondary header failed: %v", err)
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

func TestDecode_DoesNotAliasInput(t *testing.T) {
	pkt, err := spp2.NewTMPacket(123, []byte("original"))
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

	// Simulate the caller reusing its read buffer.
	for i := range encoded {
		encoded[i] = 0xFF
	}
	if string(decoded.UserData) != "original" {
		t.Fatalf("UserData aliases the input buffer: %q", decoded.UserData)
	}
}

// --- Audit fixes (SPP-F1, F2, F4, F5, F10) ---

// TestGoldenWireVectors pins the primary header bit layout to spec-derived
// wire bytes so a symmetric encode/decode bug cannot hide (SPP-F10).
func TestGoldenWireVectors(t *testing.T) {
	tests := []struct {
		name string
		pkt  func() (*spp2.SpacePacket, error)
		want []byte
	}{
		{
			name: "TM APID 100, unsegmented, seq 5",
			pkt: func() (*spp2.SpacePacket, error) {
				return spp2.NewTMPacket(100, []byte{0xDE, 0xAD, 0xBE, 0xEF}, spp2.WithSequenceCount(5))
			},
			// version 000, type 0, SH flag 0, APID 0x064, flags '11',
			// count 5, length 4-1=3
			want: []byte{0x00, 0x64, 0xC0, 0x05, 0x00, 0x03, 0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name: "TC APID 0x123, unsegmented, seq 0",
			pkt: func() (*spp2.SpacePacket, error) {
				return spp2.NewTCPacket(0x123, []byte{0x42})
			},
			want: []byte{0x11, 0x23, 0xC0, 0x00, 0x00, 0x00, 0x42},
		},
		{
			name: "idle APID 0x7FF with two fill octets",
			pkt: func() (*spp2.SpacePacket, error) {
				return spp2.NewIdlePacket([]byte{0xFF, 0xFF})
			},
			want: []byte{0x07, 0xFF, 0xC0, 0x00, 0x00, 0x01, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := tt.pkt()
			if err != nil {
				t.Fatalf("constructor failed: %v", err)
			}
			got, err := pkt.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("Encode() = % X, want % X", got, tt.want)
			}
			decoded, err := spp2.Decode(tt.want)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if decoded.PrimaryHeader != pkt.PrimaryHeader {
				t.Errorf("Decoded header = %+v, want %+v", decoded.PrimaryHeader, pkt.PrimaryHeader)
			}
		})
	}
}

func TestNewIdlePacket(t *testing.T) {
	pkt, err := spp2.NewIdlePacket([]byte{0xFF, 0xFF, 0xFF})
	if err != nil {
		t.Fatalf("NewIdlePacket failed: %v", err)
	}
	if !pkt.IsIdle() {
		t.Error("Expected IsIdle()=true")
	}
	if pkt.PrimaryHeader.APID != 0x7FF {
		t.Errorf("APID = 0x%03X, want 0x7FF", pkt.PrimaryHeader.APID)
	}
	if pkt.PrimaryHeader.SecondaryHeaderFlag != 0 {
		t.Error("Idle packet must not set the secondary header flag")
	}
}

func TestIdleWithSecondaryHeaderRejected(t *testing.T) {
	// CCSDS 4.1.3.3.3.4: the Secondary Header Flag is '0' for idle packets.
	sh := &testSecondaryHeader{Timestamp: 1}
	_, err := spp2.NewSpacePacket(0x7FF, spp2.PacketTypeTM, []byte{0xFF}, spp2.WithSecondaryHeader(sh))
	if !errors.Is(err, spp2.ErrIdleWithSecondaryHeader) {
		t.Errorf("NewSpacePacket(idle+SH) = %v, want ErrIdleWithSecondaryHeader", err)
	}

	_, err = spp2.NewIdlePacket([]byte{0xFF}, spp2.WithSecondaryHeader(sh))
	if !errors.Is(err, spp2.ErrIdleWithSecondaryHeader) {
		t.Errorf("NewIdlePacket(+SH) = %v, want ErrIdleWithSecondaryHeader", err)
	}

	// Decode path: raw idle packet with the secondary header flag set.
	raw := []byte{0x0F, 0xFF, 0xC0, 0x00, 0x00, 0x01, 0xFF, 0xFF}
	_, err = spp2.Decode(raw)
	if !errors.Is(err, spp2.ErrIdleWithSecondaryHeader) {
		t.Errorf("Decode(idle+SH flag) = %v, want ErrIdleWithSecondaryHeader", err)
	}
}

func TestEncodeRecomputesLength(t *testing.T) {
	// SPP-F4: mutating UserData after construction must not emit an
	// inconsistent length field.
	pkt, err := spp2.NewTMPacket(42, []byte{0x01, 0x02})
	if err != nil {
		t.Fatal(err)
	}
	pkt.UserData = []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if got := binary.BigEndian.Uint16(encoded[4:6]); got != 4 {
		t.Errorf("Packet length field = %d, want 4", got)
	}
	decoded, err := spp2.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if !bytes.Equal(decoded.UserData, pkt.UserData) {
		t.Error("UserData mismatch after mutation and re-encode")
	}

	// Emptying the data field must fail, not underflow the length field.
	pkt.UserData = nil
	if _, err := pkt.Encode(); !errors.Is(err, spp2.ErrEmptyPacket) {
		t.Errorf("Encode(empty) = %v, want ErrEmptyPacket", err)
	}
}

// lyingSecondaryHeader reports Size()=4 but encodes 2 bytes.
type lyingSecondaryHeader struct{}

func (h *lyingSecondaryHeader) Encode() ([]byte, error) { return []byte{0x01, 0x02}, nil }
func (h *lyingSecondaryHeader) Decode([]byte) error     { return nil }
func (h *lyingSecondaryHeader) Size() int               { return 4 }

func TestEncodeSecondaryHeaderSizeMismatch(t *testing.T) {
	// SPP-F5: Encode must verify SecondaryHeader.Encode() returns Size() bytes.
	pkt, err := spp2.NewSpacePacket(7, spp2.PacketTypeTM, []byte{0x01},
		spp2.WithSecondaryHeader(&lyingSecondaryHeader{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pkt.Encode(); !errors.Is(err, spp2.ErrSecondaryHeaderSizeMismatch) {
		t.Errorf("Encode = %v, want ErrSecondaryHeaderSizeMismatch", err)
	}
}

// --- Secondary Header Flag / field agreement (CCSDS 4.1.3.3.3.2, 4.1.3.5.3) ---

// TestEncodeRejectsSecondaryHeaderWithFlagClear pins the fix for the length
// asymmetry: Encode used to add the secondary header's size to the Packet Data
// Length whenever the field was set, but write those octets only when the flag
// was set. A packet with the field set and the flag clear therefore went on the
// wire declaring six octets of data field while carrying two, and the receiver
// read the shortfall out of the packet that followed.
func TestEncodeRejectsSecondaryHeaderWithFlagClear(t *testing.T) {
	pkt, err := spp2.NewTMPacket(100, []byte{0xAA, 0xBB},
		spp2.WithSecondaryHeader(&testSecondaryHeader{Timestamp: 0x1122334455667788}))
	if err != nil {
		t.Fatal(err)
	}

	// Clear the flag behind the constructor's back, exactly the state the old
	// code accepted.
	pkt.PrimaryHeader.SecondaryHeaderFlag = 0

	if err := pkt.Validate(); !errors.Is(err, spp2.ErrSecondaryHeaderFlagClear) {
		t.Errorf("Validate = %v, want ErrSecondaryHeaderFlagClear", err)
	}
	if _, err := pkt.Encode(); !errors.Is(err, spp2.ErrSecondaryHeaderFlagClear) {
		t.Errorf("Encode = %v, want ErrSecondaryHeaderFlagClear", err)
	}
}

// TestNoCrossPacketBleedWithMismatchedFlag streams two packets and checks the
// second one arrives whole. Before the fix the first packet's declared length
// overran into the second, so decoding the stream produced garbage from the
// second packet's octets.
func TestNoCrossPacketBleedWithMismatchedFlag(t *testing.T) {
	first, err := spp2.NewTMPacket(100, []byte{0xAA, 0xBB},
		spp2.WithSecondaryHeader(&testSecondaryHeader{Timestamp: 0x1122334455667788}))
	if err != nil {
		t.Fatal(err)
	}
	first.PrimaryHeader.SecondaryHeaderFlag = 0

	// The bad packet must not encode at all.
	if _, err := first.Encode(); err == nil {
		t.Fatal("Encode accepted a packet whose declared length exceeds its octets")
	}

	// With the flag set, the same packet encodes to a self-consistent stream:
	// the second packet starts exactly where the first one ends.
	first.PrimaryHeader.SecondaryHeaderFlag = 1
	firstBytes, err := first.Encode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := spp2.NewTMPacket(200, []byte{0xCC, 0xDD, 0xEE})
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := second.Encode()
	if err != nil {
		t.Fatal(err)
	}

	stream := append(append([]byte{}, firstBytes...), secondBytes...)

	n := spp2.PacketSizer(stream)
	if n != len(firstBytes) {
		t.Fatalf("first packet size = %d, want %d", n, len(firstBytes))
	}
	got, err := spp2.Decode(stream[n:])
	if err != nil {
		t.Fatalf("decoding the second packet: %v", err)
	}
	if got.PrimaryHeader.APID != 200 {
		t.Errorf("second packet APID = %d, want 200 (the first packet bled over)", got.PrimaryHeader.APID)
	}
	if !bytes.Equal(got.UserData, []byte{0xCC, 0xDD, 0xEE}) {
		t.Errorf("second packet data = %v, want CC DD EE", got.UserData)
	}
}

// TestDecodeEncodeRoundTripWithoutSecondaryHeaderDecoder checks that a packet
// received with the Secondary Header Flag set but no decoder configured can be
// forwarded unchanged. Its header octets sit at the front of UserData; Encode
// used to refuse the packet outright, which broke the Packet Transfer Function
// (4.2.3) for any relay.
func TestDecodeEncodeRoundTripWithoutSecondaryHeaderDecoder(t *testing.T) {
	original, err := spp2.NewTMPacket(42, []byte{0xDE, 0xAD, 0xBE, 0xEF},
		spp2.WithSecondaryHeader(&testSecondaryHeader{Timestamp: 0x0102030405060708}))
	if err != nil {
		t.Fatal(err)
	}
	wire, err := original.Encode()
	if err != nil {
		t.Fatal(err)
	}

	relayed, err := spp2.Decode(wire)
	if err != nil {
		t.Fatalf("Decode without a secondary header decoder: %v", err)
	}
	if relayed.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Error("the Secondary Header Flag was not preserved")
	}
	if relayed.SecondaryHeader != nil {
		t.Error("no decoder was configured, so no header should have been parsed")
	}
	if len(relayed.UserData) != 12 {
		t.Errorf("UserData = %d octets, want 12 (8 header + 4 user)", len(relayed.UserData))
	}

	again, err := relayed.Encode()
	if err != nil {
		t.Fatalf("re-encoding a received packet: %v", err)
	}
	if !bytes.Equal(again, wire) {
		t.Errorf("re-encoded packet\n got %x\nwant %x", again, wire)
	}
}

// TestDecodeSecondaryHeaderLargerThanDataField checks the error tells the truth:
// the buffer is complete, the configured decoder simply wants more octets than
// this packet's data field holds.
func TestDecodeSecondaryHeaderLargerThanDataField(t *testing.T) {
	// APID 1, flag set, data field of 4 octets, smaller than the 8-octet
	// decoder.
	raw := []byte{0x08, 0x01, 0xC0, 0x00, 0x00, 0x03, 0x01, 0x02, 0x03, 0x04}

	_, err := spp2.Decode(raw, spp2.WithDecodeSecondaryHeader(&testSecondaryHeader{}))
	if !errors.Is(err, spp2.ErrSecondaryHeaderExceedsDataField) {
		t.Errorf("Decode = %v, want ErrSecondaryHeaderExceedsDataField", err)
	}
	if errors.Is(err, spp2.ErrDataTooShort) {
		t.Error("a decoder mismatch was reported as a short buffer")
	}

	// The bound is the packet data field, not the buffer. With another packet
	// following in the same buffer there are plenty of octets left, and a
	// decoder that measured against the buffer would read the next packet's
	// header into this packet's secondary header.
	withTrailing := append(append([]byte(nil), raw...),
		0x00, 0x02, 0xC0, 0x00, 0x00, 0x00, 0x55)
	_, err = spp2.Decode(withTrailing, spp2.WithDecodeSecondaryHeader(&testSecondaryHeader{}))
	if !errors.Is(err, spp2.ErrSecondaryHeaderExceedsDataField) {
		t.Errorf("Decode with a following packet = %v, want ErrSecondaryHeaderExceedsDataField", err)
	}
}

// --- PacketSizer (M3) ---

func TestPacketSizerRefusesIncompletePacket(t *testing.T) {
	pkt, err := spp2.NewTMPacket(7, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if got := spp2.PacketSizer(wire); got != len(wire) {
		t.Errorf("PacketSizer(complete) = %d, want %d", got, len(wire))
	}
	// One octet short: the declared length reaches past the buffer, so slicing
	// data[:n] would panic or read the next packet's octets.
	if got := spp2.PacketSizer(wire[:len(wire)-1]); got != -1 {
		t.Errorf("PacketSizer(truncated) = %d, want -1", got)
	}
	if got := spp2.PacketSizer(wire[:3]); got != -1 {
		t.Errorf("PacketSizer(partial header) = %d, want -1", got)
	}
	// Extra octets after the packet are another packet, not this one.
	if got := spp2.PacketSizer(append(wire, 0xFF, 0xFF)); got != len(wire) {
		t.Errorf("PacketSizer(with trailing octets) = %d, want %d", got, len(wire))
	}

	// DeclaredPacketSize answers from the header alone, which is what a stream
	// reader needs before it has fetched the body.
	if got := spp2.DeclaredPacketSize(wire[:6]); got != len(wire) {
		t.Errorf("DeclaredPacketSize(header only) = %d, want %d", got, len(wire))
	}
	if got := spp2.DeclaredPacketSize(wire[:5]); got != -1 {
		t.Errorf("DeclaredPacketSize(partial header) = %d, want -1", got)
	}
}

func TestIsIdleBytes(t *testing.T) {
	idle, err := spp2.NewIdlePacket([]byte{0xFF, 0xFF})
	if err != nil {
		t.Fatal(err)
	}
	idleBytes, err := idle.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !spp2.IsIdleBytes(idleBytes) {
		t.Error("IsIdleBytes(idle packet) = false, want true")
	}

	normal, err := spp2.NewTMPacket(100, []byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	normalBytes, err := normal.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if spp2.IsIdleBytes(normalBytes) {
		t.Error("IsIdleBytes(APID 100) = true, want false")
	}
	if spp2.IsIdleBytes([]byte{0x07}) {
		t.Error("IsIdleBytes(one octet) = true, want false")
	}

	// Only the 11 APID bits decide. The bits above them (version and packet
	// type) must not be read as part of the APID, or a telecommand idle
	// packet would go unrecognized and reach an application as if it were
	// data. pkg/tmdl and pkg/aos both discard fill through this function.
	tcIdle, err := spp2.NewIdlePacket([]byte{0xFF}, spp2.WithPacketType(spp2.PacketTypeTC))
	if err != nil {
		t.Fatal(err)
	}
	tcIdleBytes, err := tcIdle.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !spp2.IsIdleBytes(tcIdleBytes) {
		t.Errorf("IsIdleBytes(TC idle packet %x) = false, want true", tcIdleBytes)
	}

	// Every APID whose low 11 bits are all ones is idle, whatever sits above.
	for _, high := range []byte{0x00, 0x08, 0x10, 0x18} {
		if !spp2.IsIdleBytes([]byte{high | 0x07, 0xFF}) {
			t.Errorf("IsIdleBytes(%#02x 0xFF) = false, want true", high|0x07)
		}
	}
}

// TestIdlePacketType checks an idle packet is telemetry by default and can be
// built as a telecommand: CCSDS 4.1.3.3.2.3 and 4.1.3.3.4.4 do not tie the idle
// APID to either type.
func TestIdlePacketType(t *testing.T) {
	tm, err := spp2.NewIdlePacket([]byte{0xFF})
	if err != nil {
		t.Fatal(err)
	}
	if tm.PrimaryHeader.Type != spp2.PacketTypeTM {
		t.Errorf("default idle packet type = %d, want TM", tm.PrimaryHeader.Type)
	}

	tc, err := spp2.NewIdlePacket([]byte{0xFF}, spp2.WithPacketType(spp2.PacketTypeTC))
	if err != nil {
		t.Fatalf("NewIdlePacket(WithPacketType(TC)) failed: %v", err)
	}
	if tc.PrimaryHeader.Type != spp2.PacketTypeTC {
		t.Errorf("idle packet type = %d, want TC", tc.PrimaryHeader.Type)
	}
	if _, err := tc.Encode(); err != nil {
		t.Fatalf("encoding a TC idle packet: %v", err)
	}

	if _, err := spp2.NewTMPacket(1, []byte{0x01}, spp2.WithPacketType(2)); !errors.Is(err, spp2.ErrInvalidType) {
		t.Errorf("WithPacketType(2) = %v, want ErrInvalidType", err)
	}
}

// --- The packet owns its data (no aliasing) ---

// TestNewSpacePacketCopiesUserData checks a built packet does not alias the
// slice it was handed. Decode already copies out of its input; the two have to
// agree, or whether a caller can change a finished packet by touching its own
// buffer would depend on how the packet was made.
func TestNewSpacePacketCopiesUserData(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	pkt, err := spp2.NewTMPacket(5, data)
	if err != nil {
		t.Fatal(err)
	}
	before, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}

	for i := range data {
		data[i] = 0xFF
	}

	after, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("mutating the caller's slice changed the packet:\nbefore %x\nafter  %x",
			before, after)
	}
}

// retainingSecondaryHeader keeps whatever slice Decode hands it, which is the
// worst case for Decode's promise not to retain its input.
type retainingSecondaryHeader struct{ kept []byte }

func (h *retainingSecondaryHeader) Size() int { return 4 }

func (h *retainingSecondaryHeader) Encode() ([]byte, error) {
	out := make([]byte, 4)
	copy(out, h.kept)
	return out, nil
}

func (h *retainingSecondaryHeader) Decode(data []byte) error {
	h.kept = data
	return nil
}

// TestDecodeDoesNotAliasInputViaSecondaryHeader checks the secondary header
// decoder is handed a copy too. Decode documents that the returned packet does
// not retain the input slice, and passing the implementation a subslice of it
// would leave that promise resting on how the implementation behaves.
func TestDecodeDoesNotAliasInputViaSecondaryHeader(t *testing.T) {
	// APID 1, flag set, 4-octet header followed by 2 octets of user data.
	raw := []byte{0x08, 0x01, 0xC0, 0x00, 0x00, 0x05, 0xAA, 0xBB, 0xCC, 0xDD, 0x01, 0x02}

	sh := &retainingSecondaryHeader{}
	pkt, err := spp2.Decode(raw, spp2.WithDecodeSecondaryHeader(sh))
	if err != nil {
		t.Fatal(err)
	}
	kept := append([]byte(nil), sh.kept...)
	userData := append([]byte(nil), pkt.UserData...)

	for i := range raw {
		raw[i] = 0xFF
	}

	if !bytes.Equal(sh.kept, kept) {
		t.Errorf("secondary header aliased the input: %x became %x", kept, sh.kept)
	}
	if !bytes.Equal(pkt.UserData, userData) {
		t.Errorf("user data aliased the input: %x became %x", userData, pkt.UserData)
	}
}

// --- A rejected Encode leaves the packet alone ---

// TestEncodeLeavesPacketUnchangedOnFailure checks a failed Encode does not
// leave a rewritten Packet Data Length behind. A caller that fixes whatever
// Encode complained about and tries again must not find the length field
// already changed under it.
func TestEncodeLeavesPacketUnchangedOnFailure(t *testing.T) {
	pkt, err := spp2.NewTMPacket(5, []byte{0x01, 0x02, 0x03, 0x04})
	if err != nil {
		t.Fatal(err)
	}
	lengthBefore := pkt.PrimaryHeader.PacketLength

	pkt.PrimaryHeader.Version = 3 // not CCSDS v1, so Validate refuses
	pkt.UserData = []byte{0x09}   // and the length would otherwise be rewritten

	if _, err := pkt.Encode(); !errors.Is(err, spp2.ErrInvalidVersion) {
		t.Fatalf("Encode = %v, want ErrInvalidVersion", err)
	}
	if pkt.PrimaryHeader.PacketLength != lengthBefore {
		t.Errorf("Packet Data Length = %d after a failed Encode, want %d unchanged",
			pkt.PrimaryHeader.PacketLength, lengthBefore)
	}
}

// --- Secondary Header Indicator on a hand-assembled data field ---

// TestWithSecondaryHeaderIndicator checks a caller holding a pre-formatted
// data field can set the Secondary Header Flag without supplying a
// SecondaryHeader implementation. This is the Secondary Header Indicator of
// 3.4.2.3, translated into the flag by 4.2.2.4, and it is what lets a relay
// assemble a packet it cannot itself interpret.
func TestWithSecondaryHeaderIndicator(t *testing.T) {
	octets := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01, 0x02, 0x03}
	pkt, err := spp2.NewTMPacket(100, octets, spp2.WithSecondaryHeaderIndicator(true))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Errorf("Secondary Header Flag = %d, want 1", pkt.PrimaryHeader.SecondaryHeaderFlag)
	}
	if pkt.SecondaryHeader != nil {
		t.Error("no parsed header was supplied, so SecondaryHeader should be nil")
	}

	wire, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The header octets are counted once, as part of the data field.
	declared := int(wire[4])<<8 | int(wire[5]) + 1
	if declared != len(octets) {
		t.Errorf("Packet Data Length declares %d octets, want %d", declared, len(octets))
	}
	if declared != len(wire)-spp2.PrimaryHeaderSize {
		t.Errorf("declared %d octets, wrote %d", declared, len(wire)-spp2.PrimaryHeaderSize)
	}

	// It survives a decode and re-encode, so a relay can keep forwarding it.
	back, err := spp2.Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	again, err := back.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, again) {
		t.Errorf("round trip changed the packet:\n got %x\nwant %x", again, wire)
	}

	// False is the default and leaves the flag clear.
	plain, err := spp2.NewTMPacket(100, octets, spp2.WithSecondaryHeaderIndicator(false))
	if err != nil {
		t.Fatal(err)
	}
	if plain.PrimaryHeader.SecondaryHeaderFlag != 0 {
		t.Error("WithSecondaryHeaderIndicator(false) left the flag set")
	}
}

// TestSecondaryHeaderSuppliedTwiceRejected checks the two ways of supplying a
// secondary header cannot be combined. Honoring both would count the header
// once as a parsed header and again as user data, declaring a data field
// longer than the packet carries (4.1.3.5.3).
func TestSecondaryHeaderSuppliedTwiceRejected(t *testing.T) {
	octets := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01}

	_, err := spp2.NewTMPacket(100, octets,
		spp2.WithSecondaryHeader(&testSecondaryHeader{}),
		spp2.WithSecondaryHeaderIndicator(true))
	if !errors.Is(err, spp2.ErrSecondaryHeaderTwice) {
		t.Errorf("header then indicator = %v, want ErrSecondaryHeaderTwice", err)
	}

	// Either order.
	_, err = spp2.NewTMPacket(100, octets,
		spp2.WithSecondaryHeaderIndicator(true),
		spp2.WithSecondaryHeader(&testSecondaryHeader{}))
	if !errors.Is(err, spp2.ErrSecondaryHeaderTwice) {
		t.Errorf("indicator then header = %v, want ErrSecondaryHeaderTwice", err)
	}
}

// TestSecondaryHeaderAssignedOverIndicatorRejected checks the two ways of
// supplying a secondary header are still refused when the parsed header is
// assigned straight to the exported field instead of going through
// WithSecondaryHeader. Encode would otherwise count the header once as a
// parsed header and again as user data (4.1.3.5.3) and write it twice. The
// result passes as a well-formed packet, since the length field matches the
// octets sent, so the receiver hands its application duplicated header octets
// as data.
func TestSecondaryHeaderAssignedOverIndicatorRejected(t *testing.T) {
	octets := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01}

	pkt, err := spp2.NewTMPacket(100, octets, spp2.WithSecondaryHeaderIndicator(true))
	if err != nil {
		t.Fatal(err)
	}

	// Straight past both option guards.
	pkt.SecondaryHeader = &testSecondaryHeader{Timestamp: 0x0102030405060708}

	if _, err := pkt.Encode(); !errors.Is(err, spp2.ErrSecondaryHeaderTwice) {
		t.Errorf("Encode = %v, want ErrSecondaryHeaderTwice", err)
	}
	if err := pkt.Validate(); !errors.Is(err, spp2.ErrSecondaryHeaderTwice) {
		t.Errorf("Validate = %v, want ErrSecondaryHeaderTwice", err)
	}

	// Each way on its own is still fine.
	parsed, err := spp2.NewTMPacket(100, octets,
		spp2.WithSecondaryHeader(&testSecondaryHeader{Timestamp: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsed.Encode(); err != nil {
		t.Errorf("parsed header alone: Encode = %v, want no error", err)
	}

	indicated, err := spp2.NewTMPacket(100, octets, spp2.WithSecondaryHeaderIndicator(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := indicated.Encode(); err != nil {
		t.Errorf("indicator alone: Encode = %v, want no error", err)
	}
}

// TestIdleRejectsSecondaryHeaderIndicator checks 4.1.3.3.3.4 still holds for a
// flag set through the indicator rather than through a parsed header.
func TestIdleRejectsSecondaryHeaderIndicator(t *testing.T) {
	_, err := spp2.NewIdlePacket([]byte{0xFF, 0xFF},
		spp2.WithSecondaryHeaderIndicator(true))
	if !errors.Is(err, spp2.ErrIdleWithSecondaryHeader) {
		t.Errorf("NewIdlePacket(indicator) = %v, want ErrIdleWithSecondaryHeader", err)
	}
}

// TestWithSecondaryHeaderRejectsNil checks a nil header is refused rather than
// silently setting the flag with nothing behind it.
func TestWithSecondaryHeaderRejectsNil(t *testing.T) {
	_, err := spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSecondaryHeader(nil))
	if !errors.Is(err, spp2.ErrSecondaryHeaderMissing) {
		t.Errorf("WithSecondaryHeader(nil) = %v, want ErrSecondaryHeaderMissing", err)
	}
}
