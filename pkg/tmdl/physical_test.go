package tmdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestChannelConfig_DataFieldCapacity(t *testing.T) {
	tests := []struct {
		name               string
		config             tmdl.ChannelConfig
		secondaryHeaderLen int
		want               int
	}{
		{
			name:               "minimal frame (header only)",
			config:             tmdl.ChannelConfig{FrameLength: 100},
			secondaryHeaderLen: 0,
			want:               94, // 100 - 6
		},
		{
			name:               "with OCF",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true},
			secondaryHeaderLen: 0,
			want:               90, // 100 - 6 - 4
		},
		{
			name:               "with FEC",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               92, // 100 - 6 - 2
		},
		{
			name:               "with OCF and FEC",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               88, // 100 - 6 - 4 - 2
		},
		{
			name:               "with secondary header",
			config:             tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 3,
			want:               84, // 100 - 6 - (1+3) - 4 - 2
		},
		{
			name:               "CCSDS typical 1115-byte frame",
			config:             tmdl.ChannelConfig{FrameLength: 1115, HasOCF: true, HasFEC: true},
			secondaryHeaderLen: 0,
			want:               1103, // 1115 - 6 - 4 - 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DataFieldCapacity(tt.secondaryHeaderLen)
			if got != tt.want {
				t.Errorf("DataFieldCapacity(%d) = %d, want %d", tt.secondaryHeaderLen, got, tt.want)
			}
		})
	}
}

// --- PhysicalChannel Tests ---

func TestPhysicalChannel_AddMasterChannel(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel(config)

	mc := tmdl.NewMasterChannel(933, config)
	pc.AddMasterChannel(mc, 1)

	if pc.Len() != 1 {
		t.Errorf("Len = %d, want 1", pc.Len())
	}
}

func TestPhysicalChannel_MCMultiplexing(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel(config)

	// Two spacecraft on the same physical channel
	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	// Send data through both spacecraft
	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	if err := svc1.Send([]byte("sc100")); err != nil {
		t.Fatal(err)
	}
	if err := svc2.Send([]byte("sc200")); err != nil {
		t.Fatal(err)
	}

	// MC mux should serve lower SCID first (100), then 200
	f1, err := pc.GetNextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if f1.Header.SpacecraftID != 100 {
		t.Errorf("Frame 1 SCID = %d, want 100", f1.Header.SpacecraftID)
	}

	f2, err := pc.GetNextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if f2.Header.SpacecraftID != 200 {
		t.Errorf("Frame 2 SCID = %d, want 200", f2.Header.SpacecraftID)
	}
}

func TestPhysicalChannel_MCPriorityWeighting(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel(config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 2) // priority 2
	pc.AddMasterChannel(mc2, 1) // priority 1

	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	// 3 frames from each SC
	for range 3 {
		if err := svc1.Send([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if err := svc2.Send([]byte("b")); err != nil {
			t.Fatal(err)
		}
	}

	// Expected: sc100, sc100 (weight 2), sc200 (weight 1), sc100, sc200, sc200
	expected := []uint16{100, 100, 200, 100, 200, 200}
	for i, want := range expected {
		frame, err := pc.GetNextFrame()
		if err != nil {
			t.Fatalf("Frame %d: %v", i, err)
		}
		if frame.Header.SpacecraftID != want {
			t.Errorf("Frame %d: SCID = %d, want %d", i, frame.Header.SpacecraftID, want)
		}
	}
}

func TestPhysicalChannel_Demultiplex(t *testing.T) {
	config := tmdl.ChannelConfig{}
	pc := tmdl.NewPhysicalChannel(config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	// Route frames to correct MC by SCID
	f1, _ := tmdl.NewTMTransferFrame(100, 1, []byte("for-sc100"), nil, nil)
	f2, _ := tmdl.NewTMTransferFrame(200, 1, []byte("for-sc200"), nil, nil)

	if err := pc.AddFrame(f1); err != nil {
		t.Fatal(err)
	}
	if err := pc.AddFrame(f2); err != nil {
		t.Fatal(err)
	}

	// Verify frames routed correctly
	got1, err := vc1.GetNextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if string(got1.DataField) != "for-sc100" {
		t.Errorf("MC1 got %q, want 'for-sc100'", got1.DataField)
	}

	got2, err := vc2.GetNextFrame()
	if err != nil {
		t.Fatal(err)
	}
	if string(got2.DataField) != "for-sc200" {
		t.Errorf("MC2 got %q, want 'for-sc200'", got2.DataField)
	}
}

func TestPhysicalChannel_DemuxUnknownSCID(t *testing.T) {
	pc := tmdl.NewPhysicalChannel(tmdl.ChannelConfig{})

	frame, _ := tmdl.NewTMTransferFrame(999, 1, []byte("data"), nil, nil)
	err := pc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrMasterChannelNotFound) {
		t.Errorf("Expected ErrMasterChannelNotFound, got %v", err)
	}
}

func TestPhysicalChannel_GetNextFrameOrIdle(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel(config)

	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	// No data: should return idle
	frame, err := pc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(frame) {
		t.Error("Expected idle frame when no data available")
	}

	// With data: should return real frame
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	if err := svc.Send([]byte("data")); err != nil {
		t.Fatal(err)
	}

	frame, err = pc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if tmdl.IsIdleFrame(frame) {
		t.Error("Expected data frame, got idle")
	}
}

func TestPhysicalChannel_NoMasterChannels(t *testing.T) {
	pc := tmdl.NewPhysicalChannel(tmdl.ChannelConfig{})
	_, err := pc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoMasterChannels) {
		t.Errorf("Expected ErrNoMasterChannels, got %v", err)
	}
}

func TestPhysicalChannel_HasPendingFrames(t *testing.T) {
	config := tmdl.ChannelConfig{}
	pc := tmdl.NewPhysicalChannel(config)

	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	if pc.HasPendingFrames() {
		t.Error("Expected false with no data")
	}

	if err := vc.AddFrame(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatal(err)
	}

	if !pc.HasPendingFrames() {
		t.Error("Expected true with pending data")
	}
}
