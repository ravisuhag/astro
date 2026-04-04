package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

func TestVirtualChannel_AddNext(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 10)

	frame, err := usdl.NewTransferFrame(100, 1, 0, []byte{0x01})
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	if err := vc.Add(frame); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, err := vc.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	if got.Header.SCID != 100 {
		t.Errorf("SCID = %d, want 100", got.Header.SCID)
	}
}

func TestMasterChannel_Routing(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := usdl.NewMasterChannel(100, config)

	vc1 := usdl.NewVirtualChannel(1, 10)
	vc2 := usdl.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	frame1, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x01})
	frame2, _ := usdl.NewTransferFrame(100, 2, 0, []byte{0x02})

	if err := mc.AddFrame(frame1); err != nil {
		t.Fatalf("AddFrame(vc1) error = %v", err)
	}
	if err := mc.AddFrame(frame2); err != nil {
		t.Fatalf("AddFrame(vc2) error = %v", err)
	}

	got1, _ := vc1.Next()
	got2, _ := vc2.Next()

	if got1.Header.VCID != 1 {
		t.Errorf("vc1 VCID = %d, want 1", got1.Header.VCID)
	}
	if got2.Header.VCID != 2 {
		t.Errorf("vc2 VCID = %d, want 2", got2.Header.VCID)
	}
}

func TestMasterChannel_SCIDMismatch(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := usdl.NewMasterChannel(100, config)
	vc := usdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := usdl.NewTransferFrame(999, 1, 0, []byte{0x01})
	err := mc.AddFrame(frame)
	if err != usdl.ErrSCIDMismatch {
		t.Errorf("expected ErrSCIDMismatch, got %v", err)
	}
}

func TestMasterChannel_VCNotFound(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := usdl.NewMasterChannel(100, config)
	vc := usdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := usdl.NewTransferFrame(100, 5, 0, []byte{0x01})
	err := mc.AddFrame(frame)
	if err != usdl.ErrVirtualChannelNotFound {
		t.Errorf("expected ErrVirtualChannelNotFound, got %v", err)
	}
}

func TestMasterChannel_GetNextFrameOrIdle(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := usdl.NewMasterChannel(100, config)

	// No VCs registered, should get idle frame
	frame, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	if !usdl.IsIdleFrame(frame) {
		t.Error("expected idle frame")
	}
}

func TestFrameGapDetector(t *testing.T) {
	det := usdl.NewFrameGapDetector()

	// First frame — no gap
	f1, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x01}, usdl.WithSequenceNumber(0))
	gap := det.Track(f1)
	if gap != 0 {
		t.Errorf("first frame gap = %d, want 0", gap)
	}

	// Sequential frame — no gap
	f2, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x02}, usdl.WithSequenceNumber(1))
	gap = det.Track(f2)
	if gap != 0 {
		t.Errorf("sequential frame gap = %d, want 0", gap)
	}

	// Gap of 2 frames
	f3, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x03}, usdl.WithSequenceNumber(4))
	gap = det.Track(f3)
	if gap != 2 {
		t.Errorf("gap = %d, want 2", gap)
	}
}

func TestPhysicalChannel(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	pc := usdl.NewPhysicalChannel("X-band", config)

	mc := usdl.NewMasterChannel(100, config)
	vc := usdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	// Send path: add frame to VC, get from PC
	frame, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x01})
	if err := vc.Add(frame); err != nil {
		t.Fatalf("vc.Add() error = %v", err)
	}
	if !pc.HasPendingFrames() {
		t.Error("expected pending frames")
	}

	got, err := pc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame() error = %v", err)
	}
	if got.Header.SCID != 100 {
		t.Errorf("SCID = %d, want 100", got.Header.SCID)
	}

	// Receive path: add frame to PC
	frame2, _ := usdl.NewTransferFrame(100, 1, 0, []byte{0x02})
	if err := pc.AddFrame(frame2); err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	got2, _ := vc.Next()
	if got2.DataField[0] != 0x02 {
		t.Errorf("DataField[0] = 0x%02X, want 0x02", got2.DataField[0])
	}
}
