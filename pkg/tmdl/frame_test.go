package tmdl_test

import (
	"github.com/ravisuhag/astro/pkg/tmdl"
	"testing"
)

func TestNewTMTransferFrame(t *testing.T) {
	scid := uint16(0x3FF)
	vcid := uint8(0x3F)
	data := []byte{0x01, 0x02, 0x03, 0x04}
	secondaryHeader := []byte{0x05, 0x06}
	ocf := []byte{0x07, 0x08, 0x09, 0x0A}

	frame, err := tmdl.NewTMTransferFrame(scid, vcid, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if frame.SpacecraftID != scid&0x03FF {
		t.Errorf("Expected SpacecraftID %v, got %v", scid&0x03FF, frame.SpacecraftID)
	}

	if frame.VirtualChannelID != vcid&0x3F {
		t.Errorf("Expected VirtualChannelID %v, got %v", vcid&0x3F, frame.VirtualChannelID)
	}

	expectedLength := uint16(5 + len(secondaryHeader) + len(data) + len(ocf) + 2)
	if frame.FrameLength != expectedLength {
		t.Errorf("Expected FrameLength %v, got %v", expectedLength, frame.FrameLength)
	}
}

func TestTMTransferFrame_Encode(t *testing.T) {
	scid := uint16(0x3FF)
	vcid := uint8(0x3F)
	data := []byte{0x01, 0x02, 0x03, 0x04}
	secondaryHeader := []byte{0x05, 0x06}
	ocf := []byte{0x07, 0x08, 0x09, 0x0A}

	frame, err := tmdl.NewTMTransferFrame(scid, vcid, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	bytes := frame.Encode()
	if len(bytes) != int(frame.FrameLength) {
		t.Errorf("Expected byte slice length %v, got %v", frame.FrameLength, len(bytes))
	}
}

func TestDecodeTMTransferFrame(t *testing.T) {
	scid := uint16(0x3FF)
	vcid := uint8(0x3F)
	data := []byte{0x01, 0x02, 0x03, 0x04}
	secondaryHeader := []byte{0x05, 0x06}
	ocf := []byte{0x07, 0x08, 0x09, 0x0A}

	frame, err := tmdl.NewTMTransferFrame(scid, vcid, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	bytes := frame.Encode()
	decodedFrame, err := tmdl.DecodeTMTransferFrame(bytes)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if decodedFrame.SpacecraftID != frame.SpacecraftID {
		t.Errorf("Expected SpacecraftID %v, got %v", frame.SpacecraftID, decodedFrame.SpacecraftID)
	}

	if decodedFrame.VirtualChannelID != frame.VirtualChannelID {
		t.Errorf("Expected VirtualChannelID %v, got %v", frame.VirtualChannelID, decodedFrame.VirtualChannelID)
	}

	if decodedFrame.FrameLength != frame.FrameLength {
		t.Errorf("Expected FrameLength %v, got %v", frame.FrameLength, decodedFrame.FrameLength)
	}

	if string(decodedFrame.DataField) != string(frame.DataField) {
		t.Errorf("Expected DataField %v, got %v", frame.DataField, decodedFrame.DataField)
	}

	if string(decodedFrame.FrameSecondaryHeader) != string(frame.FrameSecondaryHeader) {
		t.Errorf("Expected FrameSecondaryHeader %v, got %v", frame.FrameSecondaryHeader, decodedFrame.FrameSecondaryHeader)
	}

	if string(decodedFrame.OperationalControl) != string(frame.OperationalControl) {
		t.Errorf("Expected OperationalControl %v, got %v", frame.OperationalControl, decodedFrame.OperationalControl)
	}
}
