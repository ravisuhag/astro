package tmdl_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// TestHeaderEncoding ensures header values are encoded correctly.
func TestHeaderEncoding(t *testing.T) {
	header := tmdl.PrimaryHeader{
		VersionNumber:    0b01,
		SpacecraftID:     933,
		VirtualChannelID: 2,
		OCFFlag:          true,
		FSHFlag:          false,
		MCFrameCount:     15,
		VCFrameCount:     8,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  1,
		FirstHeaderPtr:   1024,
	}

	expectedMCID := uint16(header.VersionNumber)<<8 | header.SpacecraftID
	expectedGVCID := expectedMCID + uint16(header.VirtualChannelID)

	if header.GetMCID() != expectedMCID {
		t.Errorf("Expected MCID: %d, Got: %d", expectedMCID, header.GetMCID())
	}

	if header.GetGVCID() != expectedGVCID {
		t.Errorf("Expected GVCID: %d, Got: %d", expectedGVCID, header.GetGVCID())
	}
}

// TestNewTMTransferFrame validates frame creation and size constraints.
func TestNewTMTransferFrame(t *testing.T) {
	data := make([]byte, 65535) // Max-length payload
	secondaryHeader := []byte{0xAA, 0xBB}
	ocf := []byte{0x00, 0x00, 0x00, 0xFF}

	// Valid frame creation
	frame, err := tmdl.NewTMTransferFrame(933, 2, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Failed to create TM Transfer Frame: %v", err)
	}

	// Validate header fields
	if frame.Header.SpacecraftID != 933 {
		t.Errorf("Expected SCID 933, Got: %d", frame.Header.SpacecraftID)
	}
}

// TestFrameEncoding checks if encoding produces the correct length and byte sequence.
func TestFrameEncoding(t *testing.T) {
	data := []byte("Telemetry Data")
	frame, _ := tmdl.NewTMTransferFrame(1285, 3, data, nil, nil)
	encodedFrame := frame.Encode()

	expectedLength := 6 + len(data) + 2 // PrimaryHeader + SecondaryHeader + Data + CRC
	if len(encodedFrame) != expectedLength {
		t.Errorf("Encoded frame length mismatch: expected %d, got %d", expectedLength, len(encodedFrame))
	}
}

// TestFrameDecoding verifies if decoding reconstructs correct values.
func TestFrameDecoding(t *testing.T) {
	encodedFrame := []byte{
		0x7A, 0x5A, 0x00, 0x00, 0x00, 0x00, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
	}

	// Compute the correct CRC for the frame
	crc := tmdl.ComputeCRC(encodedFrame)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	encodedFrame = append(encodedFrame, crcBytes...)

	frame, err := tmdl.DecodeTMTransferFrame(encodedFrame)
	if err != nil {
		t.Fatalf("Failed to decode TM Transfer Frame: %v", err)
	}

	expectedData := []byte("Telemetry Data")
	if !bytes.Equal(frame.DataField, expectedData) {
		t.Errorf("Expected data: %s, got: %s", expectedData, frame.DataField)
	}
}

// TestCRCValidation ensures CRC detects errors correctly.
func TestCRCValidation(t *testing.T) {
	data := []byte("Test Data")
	expectedCRC := tmdl.ComputeCRC(data)

	modifiedData := append(data, 0x01) // Introduce a change
	modifiedCRC := tmdl.ComputeCRC(modifiedData)

	if expectedCRC == modifiedCRC {
		t.Errorf("CRC did not detect error; expected change but got identical CRC")
	}
}

// TestFrameRoundTrip verifies that encoding and decoding produce identical results.
func TestFrameRoundTrip(t *testing.T) {
	data := []byte("Round Trip Test  ")
	frame, _ := tmdl.NewTMTransferFrame(933, 5, data, nil, nil)
	encodedFrame := frame.Encode()
	// print encoded frame in hex format
	decodedFrame, err := tmdl.DecodeTMTransferFrame(encodedFrame)
	if err != nil {
		t.Fatalf("Failed to decode frame in round-trip test: %v", err)
	}

	// Compare all relevant fields
	if frame.Header != decodedFrame.Header {
		t.Errorf("Header mismatch: expected %+v, got %+v", frame.Header, decodedFrame.Header)
	}
	if frame.FrameErrorControl != decodedFrame.FrameErrorControl {
		t.Errorf("Frame error control mismatch: expected %x, got %x",
			frame.FrameErrorControl, decodedFrame.FrameErrorControl)
	}
	if !bytes.Equal(frame.DataField, decodedFrame.DataField) {
		t.Errorf("Data field mismatch: expected %x, got %x", frame.DataField, decodedFrame.DataField)
	}

}

// TestMalformedFrame verifies handling of corrupted frame structures.
func TestMalformedFrame(t *testing.T) {
	// Corrupt header (too short)
	corruptFrame := []byte{0x68, 0x05}
	_, err := tmdl.DecodeTMTransferFrame(corruptFrame)
	if err == nil {
		t.Error("Expected error for malformed frame but got none")
	}

	// Corrupt CRC
	validFrame := []byte{
		0x68, 0x05, 0x03, 0x00, 0x16, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
		0xA1, 0xB2, // Mocked CRC
	}
	validFrame[len(validFrame)-1] ^= 0xFF // Corrupt the last byte (CRC)

	_, err = tmdl.DecodeTMTransferFrame(validFrame)
	if err == nil {
		t.Error("Expected CRC error but decoding succeeded")
	}
}

// TestUninitializedFrame ensures defaults are handled correctly.
func TestUninitializedFrame(t *testing.T) {
	frame := &tmdl.TMTransferFrame{}

	encodedFrame := frame.Encode()
	if len(encodedFrame) == 0 {
		t.Error("Encoded empty frame should not be zero-length")
	}
}
