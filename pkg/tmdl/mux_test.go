package tmdl_test

import (
	"github.com/ravisuhag/astro/pkg/tmdl"
	"testing"
)

func TestNewMultiplexer(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	if len(mux.VChannels) != 0 {
		t.Errorf("Expected VChannels length 0, got %v", len(mux.VChannels))
	}

	if len(mux.Priority) != 0 {
		t.Errorf("Expected Priority length 0, got %v", len(mux.Priority))
	}
}

func TestMultiplexerAddVirtualChannel(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vcid := uint8(0x01)
	bufferSize := 10
	priority := 1

	vc := tmdl.NewVirtualChannel(vcid, bufferSize)
	mux.AddVirtualChannel(vc, priority)

	if len(mux.VChannels) != 1 {
		t.Errorf("Expected VChannels length 1, got %v", len(mux.VChannels))
	}

	if mux.VChannels[vcid] != vc {
		t.Errorf("Expected VirtualChannel %v, got %v", vc, mux.VChannels[vcid])
	}

	if mux.Priority[vcid] != priority {
		t.Errorf("Expected Priority %v, got %v", priority, mux.Priority[vcid])
	}
}

func TestMultiplexerGetNextFrame(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vcid1 := uint8(0x01)
	vcid2 := uint8(0x02)
	bufferSize := 10
	priority := 1

	vc1 := tmdl.NewVirtualChannel(vcid1, bufferSize)
	vc2 := tmdl.NewVirtualChannel(vcid2, bufferSize)
	mux.AddVirtualChannel(vc1, priority)
	mux.AddVirtualChannel(vc2, priority)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}
	err := vc1.AddFrame(frame1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	err = vc2.AddFrame(frame2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	retrievedFrame, err := mux.GetNextFrame()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrievedFrame != frame1 && retrievedFrame != frame2 {
		t.Errorf("Expected frame1 or frame2, got %v", retrievedFrame)
	}

	retrievedFrame, err = mux.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrievedFrame != frame1 && retrievedFrame != frame2 {
		t.Errorf("Expected frame1 or frame2, got %v", retrievedFrame)
	}

	_, err = mux.GetNextFrame()
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
}

func TestMultiplexerHasPendingFrames(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vcid := uint8(0x01)
	bufferSize := 10
	priority := 1

	vc := tmdl.NewVirtualChannel(vcid, bufferSize)
	mux.AddVirtualChannel(vc, priority)

	if mux.HasPendingFrames() {
		t.Errorf("Expected HasPendingFrames to be false, got true")
	}

	frame := &tmdl.TMTransferFrame{}
	err := vc.AddFrame(frame)
	if err != nil {
		t.Errorf("Failed to add frame: %v", err)
	}

	if !mux.HasPendingFrames() {
		t.Errorf("Expected HasPendingFrames to be true, got false")
	}

	_, err = mux.GetNextFrame()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if mux.HasPendingFrames() {
		t.Errorf("Expected HasPendingFrames to be false, got true")
	}
}
