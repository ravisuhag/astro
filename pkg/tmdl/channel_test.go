package tmdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestNewVirtualChannel(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 10)

	if vc.VCID != 0x01 {
		t.Errorf("Expected VCID 0x01, got %v", vc.VCID)
	}

	if vc.Len() != 0 {
		t.Errorf("Expected Len 0, got %v", vc.Len())
	}

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}
}

func TestAddFrame(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	frame := &tmdl.TMTransferFrame{}
	if err := vc.AddFrame(frame); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if vc.Len() != 1 {
		t.Errorf("Expected Len 1, got %v", vc.Len())
	}

	if err := vc.AddFrame(frame); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err := vc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrBufferFull) {
		t.Fatalf("Expected ErrBufferFull, got %v", err)
	}

	if vc.Len() != 2 {
		t.Errorf("Expected Len 2, got %v", vc.Len())
	}
}

func TestGetNextFrame(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}

	if err := vc.AddFrame(frame1); err != nil {
		t.Fatalf("Failed to add frame1: %v", err)
	}
	if err := vc.AddFrame(frame2); err != nil {
		t.Fatalf("Failed to add frame2: %v", err)
	}

	retrieved, err := vc.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved != frame1 {
		t.Errorf("Expected frame1, got %v", retrieved)
	}

	retrieved, err = vc.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved != frame2 {
		t.Errorf("Expected frame2, got %v", retrieved)
	}

	_, err = vc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Fatalf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestHasFrames(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}

	if err := vc.AddFrame(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatalf("Failed to add frame: %v", err)
	}

	if !vc.HasFrames() {
		t.Error("Expected HasFrames to be true")
	}

	if _, err := vc.GetNextFrame(); err != nil {
		t.Fatalf("Failed to retrieve frame: %v", err)
	}

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}
}
