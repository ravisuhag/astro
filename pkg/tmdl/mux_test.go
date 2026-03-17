package tmdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestNewMultiplexer(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	if mux.Len() != 0 {
		t.Errorf("Expected Len 0, got %v", mux.Len())
	}

	if mux.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be false")
	}
}

func TestMultiplexerAddVirtualChannel(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vc := tmdl.NewVirtualChannel(0x01, 10)
	mux.AddVirtualChannel(vc, 1)

	if mux.Len() != 1 {
		t.Errorf("Expected Len 1, got %v", mux.Len())
	}
}

func TestMultiplexerGetNextFrame(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	vc1 := tmdl.NewVirtualChannel(0x01, 10)
	vc2 := tmdl.NewVirtualChannel(0x02, 10)
	mux.AddVirtualChannel(vc1, 1)
	mux.AddVirtualChannel(vc2, 1)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}
	if err := vc1.AddFrame(frame1); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if err := vc2.AddFrame(frame2); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// With sorted VCIDs and priority 1, should get VC1 first, then VC2
	retrieved1, err := mux.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved1 != frame1 {
		t.Errorf("Expected frame1 first (lower VCID has priority)")
	}

	retrieved2, err := mux.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved2 != frame2 {
		t.Errorf("Expected frame2 second")
	}

	_, err = mux.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Fatalf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestMultiplexerPriorityWeighting(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	vc1 := tmdl.NewVirtualChannel(0x01, 10)
	vc2 := tmdl.NewVirtualChannel(0x02, 10)
	mux.AddVirtualChannel(vc1, 2) // Priority 2: gets 2 turns
	mux.AddVirtualChannel(vc2, 1) // Priority 1: gets 1 turn

	// Add 3 frames to vc1 and 3 to vc2
	for i := range 3 {
		if err := vc1.AddFrame(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: uint8(i)}}); err != nil {
			t.Fatalf("Failed to add frame to vc1: %v", err)
		}
		if err := vc2.AddFrame(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: uint8(10 + i)}}); err != nil {
			t.Fatalf("Failed to add frame to vc2: %v", err)
		}
	}

	// Expected order: vc1, vc1 (weight 2), vc2 (weight 1), vc1, vc1 (weight 2), vc2 (weight 1)
	expected := []uint8{0, 1, 10, 2, 11, 12}
	for i, exp := range expected {
		frame, err := mux.GetNextFrame()
		if err != nil {
			t.Fatalf("Frame %d: unexpected error: %v", i, err)
		}
		if frame.Header.MCFrameCount != exp {
			t.Errorf("Frame %d: expected MCFrameCount %d, got %d", i, exp, frame.Header.MCFrameCount)
		}
	}
}

func TestMultiplexerHasPendingFrames(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vc := tmdl.NewVirtualChannel(0x01, 10)
	mux.AddVirtualChannel(vc, 1)

	if mux.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be false")
	}

	if err := vc.AddFrame(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatalf("Failed to add frame: %v", err)
	}

	if !mux.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be true")
	}

	if _, err := mux.GetNextFrame(); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if mux.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be false")
	}
}

func TestMultiplexerNoChannels(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	_, err := mux.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoVirtualChannels) {
		t.Errorf("Expected ErrNoVirtualChannels, got %v", err)
	}
}
