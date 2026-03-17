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
		HeaderLength:  3, // Must match len(DataField)
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
		VersionNumber: 1,
		HeaderLength:  0x3F,
	}

	if err := invalidHeader.Validate(); !errors.Is(err, tmdl.ErrInvalidSecondaryHeaderVersion) {
		t.Errorf("Expected ErrInvalidSecondaryHeaderVersion, got %v", err)
	}
}
