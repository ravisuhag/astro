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
	if err := mc.AddFrame(frame); err != usdl.ErrSCIDMismatch {
		t.Errorf("expected ErrSCIDMismatch, got %v", err)
	}
}

func TestMasterChannel_VCNotFound(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := usdl.NewMasterChannel(100, config)
	vc := usdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := usdl.NewTransferFrame(100, 5, 0, []byte{0x01})
	if err := mc.AddFrame(frame); err != usdl.ErrVirtualChannelNotFound {
		t.Errorf("expected ErrVirtualChannelNotFound, got %v", err)
	}
}

func TestMasterChannel_GetNextFrameOrIdle(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true, VCFCountLen: 2}
	mc := usdl.NewMasterChannel(100, config)

	// No VCs registered, should get idle frames with their own VC 63 count.
	first, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	if !usdl.IsIdleFrame(first) {
		t.Error("expected idle frame")
	}
	second, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	if first.Header.VCFCount != 0 || second.Header.VCFCount != 1 {
		t.Errorf("OID VCF counts = %d, %d; want 0, 1",
			first.Header.VCFCount, second.Header.VCFCount)
	}
}

func TestFrameGapDetector(t *testing.T) {
	det := usdl.NewFrameGapDetector(2)

	mk := func(count uint64) *usdl.TransferFrame {
		f, err := usdl.NewTransferFrame(100, 1, 0, []byte{0x01},
			usdl.WithVCFCount(2, count))
		if err != nil {
			t.Fatalf("NewTransferFrame() error = %v", err)
		}
		return f
	}

	if gap := det.Track(mk(0)); gap != 0 {
		t.Errorf("first frame gap = %d, want 0", gap)
	}
	if gap := det.Track(mk(1)); gap != 0 {
		t.Errorf("sequential frame gap = %d, want 0", gap)
	}
	if gap := det.Track(mk(4)); gap != 2 {
		t.Errorf("gap = %d, want 2", gap)
	}
}

// Clause 4.1.2.12.4-12.5: sequence-controlled and expedited frames keep
// independent counts per VC, so mixed traffic must not read as a gap.
func TestFrameGapDetector_PerQoSCounts(t *testing.T) {
	det := usdl.NewFrameGapDetector(2)

	mk := func(count uint64, expedited bool) *usdl.TransferFrame {
		opts := []usdl.FrameOption{usdl.WithVCFCount(2, count)}
		if expedited {
			opts = append(opts, usdl.WithBypassSeqCtrl())
		}
		f, err := usdl.NewTransferFrame(100, 1, 0, []byte{0x01}, opts...)
		if err != nil {
			t.Fatalf("NewTransferFrame() error = %v", err)
		}
		return f
	}

	if gap := det.Track(mk(0, false)); gap != 0 {
		t.Errorf("seq #0 gap = %d, want 0", gap)
	}
	if gap := det.Track(mk(0, true)); gap != 0 {
		t.Errorf("expedited #0 gap = %d, want 0 (independent counter)", gap)
	}
	if gap := det.Track(mk(1, false)); gap != 0 {
		t.Errorf("seq #1 gap = %d, want 0 (expedited frame must not disturb it)", gap)
	}
	if gap := det.Track(mk(3, true)); gap != 2 {
		t.Errorf("expedited #3 gap = %d, want 2", gap)
	}
}

func TestFrameGapDetector_NoCount(t *testing.T) {
	det := usdl.NewFrameGapDetector(0)
	f, err := usdl.NewTransferFrame(100, 1, 0, []byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	if gap := det.Track(f); gap != 0 {
		t.Errorf("gap = %d, want 0 when no VCF count is carried", gap)
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
