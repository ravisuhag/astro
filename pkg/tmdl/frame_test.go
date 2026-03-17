package tmdl_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestHeaderEncoding(t *testing.T) {
	header := tmdl.PrimaryHeader{
		VersionNumber:    0b00,
		SpacecraftID:     933,
		VirtualChannelID: 2,
		OCFFlag:          true,
		FSHFlag:          false,
		MCFrameCount:     15,
		VCFrameCount:     8,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  0b11,
		FirstHeaderPtr:   1024,
	}

	// MCID = TFVN (2 bits) << 10 | SCID (10 bits)
	expectedMCID := uint16(header.VersionNumber)<<10 | header.SpacecraftID
	// GVCID = MCID (12 bits) << 3 | VCID (3 bits)
	expectedGVCID := expectedMCID<<3 | uint16(header.VirtualChannelID)

	if header.MCID() != expectedMCID {
		t.Errorf("Expected MCID: %d, Got: %d", expectedMCID, header.MCID())
	}

	if header.GVCID() != expectedGVCID {
		t.Errorf("Expected GVCID: %d, Got: %d", expectedGVCID, header.GVCID())
	}
}

func TestNewTMTransferFrame(t *testing.T) {
	data := make([]byte, 65535)
	secondaryHeader := []byte{0xAA, 0xBB}
	ocf := []byte{0x00, 0x00, 0x00, 0xFF}

	frame, err := tmdl.NewTMTransferFrame(933, 2, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Failed to create TM Transfer Frame: %v", err)
	}

	if frame.Header.SpacecraftID != 933 {
		t.Errorf("Expected SCID 933, Got: %d", frame.Header.SpacecraftID)
	}
}

func TestNewTMTransferFrame_SecondaryHeaderRoundTrip(t *testing.T) {
	data := []byte("payload")
	secondaryHeaderData := []byte{0xAA, 0xBB, 0xCC}

	frame, err := tmdl.NewTMTransferFrame(933, 1, data, secondaryHeaderData, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.SecondaryHeader.DataField, secondaryHeaderData) {
		t.Errorf("Secondary header data mismatch: expected %x, got %x",
			secondaryHeaderData, decoded.SecondaryHeader.DataField)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("Data field mismatch: expected %x, got %x", data, decoded.DataField)
	}
}

func TestFrameEncoding(t *testing.T) {
	data := []byte("Telemetry Data")
	frame, _ := tmdl.NewTMTransferFrame(1285, 3, data, nil, nil)
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expectedLength := 6 + len(data) + 2 // PrimaryHeader + Data + CRC
	if len(encodedFrame) != expectedLength {
		t.Errorf("Encoded frame length mismatch: expected %d, got %d", expectedLength, len(encodedFrame))
	}
}

func TestFrameDecoding(t *testing.T) {
	encodedFrame := []byte{
		0x3A, 0x5A, 0x00, 0x00, 0x18, 0x00, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
	}

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

func TestCRCValidation(t *testing.T) {
	data := []byte("Test Data")
	expectedCRC := tmdl.ComputeCRC(data)

	modifiedData := append(data, 0x01)
	modifiedCRC := tmdl.ComputeCRC(modifiedData)

	if expectedCRC == modifiedCRC {
		t.Errorf("CRC did not detect error; expected change but got identical CRC")
	}
}

func TestFrameRoundTrip(t *testing.T) {
	data := []byte("Round Trip Test  ")
	frame, _ := tmdl.NewTMTransferFrame(933, 5, data, nil, nil)
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decodedFrame, err := tmdl.DecodeTMTransferFrame(encodedFrame)
	if err != nil {
		t.Fatalf("Failed to decode frame in round-trip test: %v", err)
	}

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

func TestMalformedFrame(t *testing.T) {
	corruptFrame := []byte{0x68, 0x05}
	_, err := tmdl.DecodeTMTransferFrame(corruptFrame)
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}

	validFrame := []byte{
		0x68, 0x05, 0x03, 0x00, 0x16, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
		0xA1, 0xB2, // Mocked CRC
	}
	validFrame[len(validFrame)-1] ^= 0xFF

	_, err = tmdl.DecodeTMTransferFrame(validFrame)
	if err == nil {
		t.Error("Expected CRC error but decoding succeeded")
	}
}

func TestSecondaryHeaderLength_CCSDS(t *testing.T) {
	shData := []byte{0xAA, 0xBB, 0xCC}
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), shData, nil)
	if err != nil {
		t.Fatal(err)
	}

	if frame.SecondaryHeader.HeaderLength != 2 {
		t.Errorf("HeaderLength = %d, want 2 (len(DataField)-1 per CCSDS)", frame.SecondaryHeader.HeaderLength)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if wireLen := encoded[6] & 0x3F; wireLen != 2 {
		t.Errorf("Wire HeaderLength = %d, want 2", wireLen)
	}
}

func TestMinimumFrameSize(t *testing.T) {
	// 7 bytes: too short (need 6 header + 2 CRC minimum)
	_, err := tmdl.DecodeTMTransferFrame([]byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00})
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("7-byte input: got %v, want ErrDataTooShort", err)
	}

	// 8 bytes with valid CRC: smallest valid frame
	header := []byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x00}
	crc := tmdl.ComputeCRC(header)
	frame := make([]byte, 8)
	copy(frame, header)
	binary.BigEndian.PutUint16(frame[6:], crc)
	if _, err := tmdl.DecodeTMTransferFrame(frame); err != nil {
		t.Errorf("8-byte frame: got %v, want nil", err)
	}
}

func TestSecondaryHeaderDecodeSelfDescribing(t *testing.T) {
	data := []byte("payload")
	shData := []byte{0x01, 0x02}

	frame, err := tmdl.NewTMTransferFrame(933, 1, data, shData, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded.SecondaryHeader.DataField, shData) {
		t.Errorf("SecondaryHeader.DataField = %x, want %x", decoded.SecondaryHeader.DataField, shData)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %x, want %x", decoded.DataField, data)
	}
}

func TestOCFInsufficientData(t *testing.T) {
	header := tmdl.PrimaryHeader{
		SpacecraftID:     100,
		VirtualChannelID: 1,
		OCFFlag:          true,
		SegmentLengthID:  0b11,
	}
	hBytes, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// 2 bytes of data field — not enough for the 4-byte OCF the flag promises
	withoutCRC := append(hBytes, 0x01, 0x02)
	crc := tmdl.ComputeCRC(withoutCRC)
	frame := binary.BigEndian.AppendUint16(withoutCRC, crc)

	_, err = tmdl.DecodeTMTransferFrame(frame)
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("got %v, want ErrDataTooShort", err)
	}
}

func TestSecondaryHeaderValidateConsistency(t *testing.T) {
	sh := &tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  10,
		DataField:     []byte{0x01, 0x02, 0x03},
	}
	if err := sh.Validate(); err == nil {
		t.Error("Validate() = nil, want error for HeaderLength/DataField mismatch")
	}
}

func TestUninitializedFrame(t *testing.T) {
	// Zero-value frame should fail validation since SegmentLengthID must be 0b11 when SyncFlag is 0
	frame := &tmdl.TMTransferFrame{}
	_, err := frame.Encode()
	if !errors.Is(err, tmdl.ErrInvalidSegmentLengthID) {
		t.Errorf("Expected ErrInvalidSegmentLengthID for zero-value frame, got %v", err)
	}

	// Frame with valid defaults should encode successfully
	frame.Header.SegmentLengthID = 0b11
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encodedFrame) == 0 {
		t.Error("Encoded frame should not be zero-length")
	}
}
