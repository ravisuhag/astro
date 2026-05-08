package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

func TestVirtualChannel_AddNext(t *testing.T) {
	vc := aos.NewVirtualChannel(1, 10)
	frame, err := aos.NewTransferFrame(100, 1, []byte{0x01})
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
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := aos.NewMasterChannel(100, config)

	vc1 := aos.NewVirtualChannel(1, 10)
	vc2 := aos.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	frame1, _ := aos.NewTransferFrame(100, 1, []byte{0x01})
	frame2, _ := aos.NewTransferFrame(100, 2, []byte{0x02})

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
	mc := aos.NewMasterChannel(100, aos.ChannelConfig{})
	vc := aos.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	frame, _ := aos.NewTransferFrame(99, 1, []byte{0x01})
	if err := mc.AddFrame(frame); err != aos.ErrSCIDMismatch {
		t.Errorf("expected ErrSCIDMismatch, got %v", err)
	}
}

func TestMasterChannel_VCNotFound(t *testing.T) {
	mc := aos.NewMasterChannel(100, aos.ChannelConfig{})
	vc := aos.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	frame, _ := aos.NewTransferFrame(100, 5, []byte{0x01})
	if err := mc.AddFrame(frame); err != aos.ErrVirtualChannelNotFound {
		t.Errorf("expected ErrVirtualChannelNotFound, got %v", err)
	}
}

func TestMasterChannel_GetNextFrameOrIdle(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	mc := aos.NewMasterChannel(100, config)
	frame, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle() error = %v", err)
	}
	if !aos.IsIdleFrame(frame) {
		t.Error("expected idle frame when no VCs registered")
	}
}

func TestFrameGapDetector(t *testing.T) {
	det := aos.NewFrameGapDetector()

	f1, _ := aos.NewTransferFrame(100, 1, []byte{0x01}, aos.WithVCFrameCount(0))
	if gap := det.Track(f1); gap != 0 {
		t.Errorf("first frame gap = %d, want 0", gap)
	}

	f2, _ := aos.NewTransferFrame(100, 1, []byte{0x02}, aos.WithVCFrameCount(1))
	if gap := det.Track(f2); gap != 0 {
		t.Errorf("sequential gap = %d, want 0", gap)
	}

	f3, _ := aos.NewTransferFrame(100, 1, []byte{0x03}, aos.WithVCFrameCount(4))
	if gap := det.Track(f3); gap != 2 {
		t.Errorf("gap = %d, want 2", gap)
	}
}

func TestFrameGapDetector_24BitWrap(t *testing.T) {
	det := aos.NewFrameGapDetector()
	f1, _ := aos.NewTransferFrame(1, 0, []byte{0x01}, aos.WithVCFrameCount(aos.MaxVCFrameCount))
	det.Track(f1)
	f2, _ := aos.NewTransferFrame(1, 0, []byte{0x02}, aos.WithVCFrameCount(0))
	if gap := det.Track(f2); gap != 0 {
		t.Errorf("wrap gap = %d, want 0", gap)
	}
}

func TestPhysicalChannel(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	pc := aos.NewPhysicalChannel("Ka-band", config)

	mc := aos.NewMasterChannel(50, config)
	vc := aos.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	frame, _ := aos.NewTransferFrame(50, 1, []byte{0x01})
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
	if got.Header.SCID != 50 {
		t.Errorf("SCID = %d, want 50", got.Header.SCID)
	}

	frame2, _ := aos.NewTransferFrame(50, 1, []byte{0x02})
	if err := pc.AddFrame(frame2); err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	got2, _ := vc.Next()
	if got2.DataField[0] != 0x02 {
		t.Errorf("DataField[0] = 0x%02X, want 0x02", got2.DataField[0])
	}
}

func TestChannelConfig_DataFieldCapacity(t *testing.T) {
	tests := []struct {
		name    string
		config  aos.ChannelConfig
		wantCap int
	}{
		{
			name:    "no options",
			config:  aos.ChannelConfig{FrameLength: 100},
			wantCap: 100 - aos.PrimaryHeaderSize,
		},
		{
			name:    "with FECF",
			config:  aos.ChannelConfig{FrameLength: 100, HasFECF: true},
			wantCap: 100 - aos.PrimaryHeaderSize - aos.FECFSize,
		},
		{
			name:    "with OCF and FECF",
			config:  aos.ChannelConfig{FrameLength: 100, HasOCF: true, HasFECF: true},
			wantCap: 100 - aos.PrimaryHeaderSize - aos.OCFSize - aos.FECFSize,
		},
		{
			name:    "with insert zone",
			config:  aos.ChannelConfig{FrameLength: 100, InsertZoneLen: 8, HasFECF: true},
			wantCap: 100 - aos.PrimaryHeaderSize - 8 - aos.FECFSize,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.DataFieldCapacity(); got != tt.wantCap {
				t.Errorf("DataFieldCapacity() = %d, want %d", got, tt.wantCap)
			}
		})
	}
}
