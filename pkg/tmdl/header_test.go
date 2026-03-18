package tmdl_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestPrimaryHeader_EncodeDecode(t *testing.T) {
	original := &tmdl.PrimaryHeader{
		VersionNumber:    0,
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		OCFFlag:          true,
		MCFrameCount:     0xFF,
		VCFrameCount:     0xFF,
		FSHFlag:          true,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  0x03,
		FirstHeaderPtr:   0x07FF,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoded) != 6 {
		t.Fatalf("Expected header length of 6 bytes, got %d", len(encoded))
	}

	var decoded tmdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if *original != decoded {
		t.Errorf("Expected %v, got %v", original, decoded)
	}
}

func TestPrimaryHeader_Validate(t *testing.T) {
	validHeader := &tmdl.PrimaryHeader{
		VersionNumber:    0,
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		SegmentLengthID:  0x03,
	}

	if err := validHeader.Validate(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	invalidHeader := &tmdl.PrimaryHeader{
		VersionNumber:    4,
		SpacecraftID:     0x3FF,
		VirtualChannelID: 0x07,
		SegmentLengthID:  0x03,
	}

	if err := invalidHeader.Validate(); !errors.Is(err, tmdl.ErrInvalidVersion) {
		t.Errorf("Expected ErrInvalidVersion, got %v", err)
	}
}

func TestPrimaryHeader_EncodeDecode_ValidHeader(t *testing.T) {
	original := &tmdl.PrimaryHeader{
		VersionNumber:    0,
		SpacecraftID:     0x3A5,
		VirtualChannelID: 5,
		OCFFlag:          true,
		MCFrameCount:     20,
		VCFrameCount:     40,
		FSHFlag:          true,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  3,
		FirstHeaderPtr:   500,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	hexEncoded := hex.EncodeToString(encoded)
	expectedHex := "3a5b142899f4"
	if hexEncoded != expectedHex {
		t.Errorf("Expected encoded header to be %s, got %s", expectedHex, hexEncoded)
	}

	var decoded tmdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if *original != decoded {
		t.Errorf("Expected %v, got %v", original, decoded)
	}
}

func TestSecondaryHeader_EncodeDecode(t *testing.T) {
	original := &tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  2, // CCSDS: len(DataField) - 1
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

func TestSecondaryHeader_MaxLength63(t *testing.T) {
	shData := make([]byte, 63)
	for i := range shData {
		shData[i] = byte(i)
	}

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("payload"), shData, nil)
	if err != nil {
		t.Fatalf("63-byte secondary header data should be valid: %v", err)
	}

	if frame.SecondaryHeader.HeaderLength != 62 {
		t.Errorf("HeaderLength = %d, want 62", frame.SecondaryHeader.HeaderLength)
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
		t.Error("63-byte secondary header data corrupted during round-trip")
	}
	if !bytes.Equal(decoded.DataField, []byte("payload")) {
		t.Error("Data field corrupted with large secondary header")
	}
}

func TestSecondaryHeader_MinLength1(t *testing.T) {
	shData := []byte{0x42}
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), shData, nil)
	if err != nil {
		t.Fatal(err)
	}
	if frame.SecondaryHeader.HeaderLength != 0 {
		t.Errorf("HeaderLength = %d, want 0 (1-byte data → len-1=0)", frame.SecondaryHeader.HeaderLength)
	}

	encoded, _ := frame.Encode()
	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.SecondaryHeader.DataField, shData) {
		t.Error("1-byte secondary header data corrupted")
	}
}

func TestSCIDBoundaries(t *testing.T) {
	tests := []struct {
		name string
		scid uint16
	}{
		{"min SCID 0", 0},
		{"max SCID 1023", 1023},
		{"mid SCID 512", 512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := tmdl.NewTMTransferFrame(tt.scid, 0, []byte("data"), nil, nil)
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

			if decoded.Header.SpacecraftID != tt.scid {
				t.Errorf("SCID = %d, want %d", decoded.Header.SpacecraftID, tt.scid)
			}
		})
	}
}

func TestVCIDBoundaries(t *testing.T) {
	for vcid := uint8(0); vcid <= 7; vcid++ {
		frame, err := tmdl.NewTMTransferFrame(100, vcid, []byte("data"), nil, nil)
		if err != nil {
			t.Fatalf("VCID %d: %v", vcid, err)
		}

		encoded, _ := frame.Encode()
		decoded, err := tmdl.DecodeTMTransferFrame(encoded)
		if err != nil {
			t.Fatalf("VCID %d decode: %v", vcid, err)
		}

		if decoded.Header.VirtualChannelID != vcid {
			t.Errorf("VCID = %d, want %d", decoded.Header.VirtualChannelID, vcid)
		}
	}
}

func TestInvalidSCID(t *testing.T) {
	header := &tmdl.PrimaryHeader{
		SpacecraftID:    1024,
		SegmentLengthID: 0b11,
	}
	if err := header.Validate(); !errors.Is(err, tmdl.ErrInvalidSpacecraftID) {
		t.Errorf("Expected ErrInvalidSpacecraftID, got %v", err)
	}
}

func TestInvalidVCID(t *testing.T) {
	header := &tmdl.PrimaryHeader{
		VirtualChannelID: 8,
		SegmentLengthID:  0b11,
	}
	if err := header.Validate(); !errors.Is(err, tmdl.ErrInvalidVCID) {
		t.Errorf("Expected ErrInvalidVCID, got %v", err)
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
		VersionNumber: 1,
		HeaderLength:  0x3F,
	}

	if err := invalidHeader.Validate(); !errors.Is(err, tmdl.ErrInvalidSecondaryHeaderVersion) {
		t.Errorf("Expected ErrInvalidSecondaryHeaderVersion, got %v", err)
	}
}
