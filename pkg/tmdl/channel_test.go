package tmdl_test

import (
	"github.com/ravisuhag/astro/pkg/tmdl"
	"testing"
)

func TestNewVirtualChannel(t *testing.T) {
	vcid := uint8(0x01)
	bufferSize := 10

	vc := tmdl.NewVirtualChannel(vcid, bufferSize)

	if vc.VCID != vcid {
		t.Errorf("Expected VCID %v, got %v", vcid, vc.VCID)
	}

	if len(vc.FrameBuffer) != 0 {
		t.Errorf("Expected FrameBuffer length 0, got %v", len(vc.FrameBuffer))
	}

	if cap(vc.FrameBuffer) != bufferSize {
		t.Errorf("Expected FrameBuffer capacity %v, got %v", bufferSize, cap(vc.FrameBuffer))
	}

	if vc.MaxBufferSize != bufferSize {
		t.Errorf("Expected MaxBufferSize %v, got %v", bufferSize, vc.MaxBufferSize)
	}
}

func TestAddFrame(t *testing.T) {
	vcid := uint8(0x01)
	bufferSize := 2
	vc := tmdl.NewVirtualChannel(vcid, bufferSize)

	frame := &tmdl.TMTransferFrame{}
	err := vc.AddFrame(frame)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(vc.FrameBuffer) != 1 {
		t.Errorf("Expected FrameBuffer length 1, got %v", len(vc.FrameBuffer))
	}

	err = vc.AddFrame(frame)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = vc.AddFrame(frame)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if len(vc.FrameBuffer) != bufferSize {
		t.Errorf("Expected FrameBuffer length %v, got %v", bufferSize, len(vc.FrameBuffer))
	}
}

func TestGetNextFrame(t *testing.T) {
	vcid := uint8(0x01)
	bufferSize := 2
	vc := tmdl.NewVirtualChannel(vcid, bufferSize)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}

	err := vc.AddFrame(frame1)
	if err != nil {
		t.Errorf("Failed to add frame1: %v", err)
	}

	err = vc.AddFrame(frame2)
	if err != nil {
		t.Errorf("Failed to add frame2: %v", err)
	}

	retrievedFrame, err := vc.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrievedFrame != frame1 {
		t.Errorf("Expected frame1, got %v", retrievedFrame)
	}

	retrievedFrame, err = vc.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrievedFrame != frame2 {
		t.Errorf("Expected frame2, got %v", retrievedFrame)
	}

	_, err = vc.GetNextFrame()
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
}

func TestHasFrames(t *testing.T) {
	vcid := uint8(0x01)
	bufferSize := 2
	vc := tmdl.NewVirtualChannel(vcid, bufferSize)

	if vc.HasFrames() {
		t.Errorf("Expected HasFrames to be false, got true")
	}

	frame := &tmdl.TMTransferFrame{}
	err := vc.AddFrame(frame)
	if err != nil {
		t.Errorf("Failed to add frame: %v", err)
	}

	if !vc.HasFrames() {
		t.Errorf("Expected HasFrames to be true, got false")
	}

	_, err = vc.GetNextFrame()
	if err != nil {
		t.Errorf("Failed to retrieve frame: %v", err)
	}

	if vc.HasFrames() {
		t.Errorf("Expected HasFrames to be false, got true")
	}
}
