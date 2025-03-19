package tmdl_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestPrimaryHeader_EncodeDecode(t *testing.T) {
	original := &tmdl.PrimaryHeader{
		VersionNumber:    1,
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		OCFFlag:          true,
		MCFrameCount:     0xFF,
		VCFrameCount:     0xFF,
		FSHFlag:          true,
		SyncFlag:         false,
		PacketOrderFlag:  true,
		SegmentLengthID:  0x03,
		FirstHeaderPtr:   0x07FF,
	}

	encoded := original.Encode()

	// Verify header is exactly 8 bytes (6 bytes + 2 bytes FirstHeaderPtr)
	if len(encoded) != 6 {
		t.Fatalf("Expected header length of 6 bytes, got %d", len(encoded))
	}

	decoded, err := original.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if *original != *decoded {
		t.Errorf("Expected %v, got %v", original, decoded)
	}
}

func TestPrimaryHeader_Validate(t *testing.T) {
	validHeader := &tmdl.PrimaryHeader{
		VersionNumber:    1,
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		SegmentLengthID:  0x03,
	}

	if err := validHeader.Validate(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	invalidHeader := &tmdl.PrimaryHeader{
		VersionNumber:    4, // Invalid VersionNumber
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		SegmentLengthID:  0x03,
	}

	if err := invalidHeader.Validate(); err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestPrimaryHeader_EncodeDecode_ValidHeader(t *testing.T) {
	// Create a fully populated Header
	original := &tmdl.PrimaryHeader{
		VersionNumber:    1,     // 01
		SpacecraftID:     0x3A5, // 933 (11 1010 0101)
		VirtualChannelID: 5,     // 101
		OCFFlag:          true,  // 1
		MCFrameCount:     20,    // 0001 0100
		VCFrameCount:     40,    // 0010 1000
		FSHFlag:          true,  // 1
		SyncFlag:         false, // 0
		PacketOrderFlag:  true,  // 1
		SegmentLengthID:  2,     // 10
		FirstHeaderPtr:   500,   // 001 1111 0100
	}
	// Encode the header
	encoded := original.Encode()

	// Convert to hex for readability
	hexEncoded := hex.EncodeToString(encoded)
	expectedHex := "7a5b1428b1f4"
	if hexEncoded != expectedHex {
		t.Errorf("Expected encoded header to be %s, got %s", expectedHex, hexEncoded)
	}

	// Decode the header
	decoded, err := original.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	// Compare the decoded header with the original
	if *original != *decoded {
		t.Errorf("Expected %v, got %v", original, decoded)
	}
}

func TestSecondaryHeader_EncodeDecode(t *testing.T) {
	original := &tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  0x3F,
		DataField:     []byte{0x01, 0x02, 0x03},
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded := &tmdl.SecondaryHeader{}
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(original.DataField, decoded.DataField) {
		t.Errorf("Expected %v, got %v", original.DataField, decoded.DataField)
	}
}

func TestSecondaryHeader_Validate(t *testing.T) {
	validHeader := &tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  0x3F,
	}

	if err := validHeader.Validate(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	invalidHeader := &tmdl.SecondaryHeader{
		VersionNumber: 1, // Invalid VersionNumber
		HeaderLength:  0x3F,
	}

	if err := invalidHeader.Validate(); err == nil {
		t.Errorf("Expected error, got nil")
	}
}
