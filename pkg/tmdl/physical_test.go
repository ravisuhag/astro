package tmdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// --- ChannelConfig Tests ---

func TestChannelConfig_DataFieldCapacity(t *testing.T) {
	tests := []struct {
		name               string
		config             tmdl.ChannelConfig
		secondaryHeaderLen int
		want               int
	}{
		{"minimal", tmdl.ChannelConfig{FrameLength: 100}, 0, 94},
		{"with OCF", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true}, 0, 90},
		{"with FEC", tmdl.ChannelConfig{FrameLength: 100, HasFEC: true}, 0, 92},
		{"OCF+FEC", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true}, 0, 88},
		{"secondary header", tmdl.ChannelConfig{FrameLength: 100, HasOCF: true, HasFEC: true}, 3, 84},
		{"CCSDS 1115-byte", tmdl.ChannelConfig{FrameLength: 1115, HasOCF: true, HasFEC: true}, 0, 1103},
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
	pc := tmdl.NewPhysicalChannel("X-band", config)
	mc := tmdl.NewMasterChannel(933, config)
	pc.AddMasterChannel(mc, 1)

	if pc.Len() != 1 {
		t.Errorf("Len = %d, want 1", pc.Len())
	}
	if pc.Name != "X-band" {
		t.Errorf("Name = %q, want 'X-band'", pc.Name)
	}
}

func TestPhysicalChannel_MCMultiplexing(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("downlink", config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	_ = svc1.Send(makeTestPacket([]byte{0x01}))
	_ = svc1.Flush()
	_ = svc2.Send(makeTestPacket([]byte{0x02}))
	_ = svc2.Flush()

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
	pc := tmdl.NewPhysicalChannel("downlink", config)

	mc1 := tmdl.NewMasterChannel(100, config)
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, config)
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 2)
	pc.AddMasterChannel(mc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(100, 1, vc1, config, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(200, 1, vc2, config, nil)

	for range 3 {
		_ = svc1.Send(makeTestPacket([]byte{0xAA}))
		_ = svc1.Flush()
		_ = svc2.Send(makeTestPacket([]byte{0xBB}))
		_ = svc2.Flush()
	}

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
	pc := tmdl.NewPhysicalChannel("uplink", tmdl.ChannelConfig{})

	mc1 := tmdl.NewMasterChannel(100, tmdl.ChannelConfig{})
	vc1 := tmdl.NewVirtualChannel(1, 100)
	mc1.AddVirtualChannel(vc1, 1)

	mc2 := tmdl.NewMasterChannel(200, tmdl.ChannelConfig{})
	vc2 := tmdl.NewVirtualChannel(1, 100)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	f1, _ := tmdl.NewTMTransferFrame(100, 1, []byte("for-sc100"), nil, nil)
	f2, _ := tmdl.NewTMTransferFrame(200, 1, []byte("for-sc200"), nil, nil)
	_ = pc.AddFrame(f1)
	_ = pc.AddFrame(f2)

	got1, _ := vc1.Next()
	if string(got1.DataField) != "for-sc100" {
		t.Errorf("MC1 got %q", got1.DataField)
	}
	got2, _ := vc2.Next()
	if string(got2.DataField) != "for-sc200" {
		t.Errorf("MC2 got %q", got2.DataField)
	}
}

func TestPhysicalChannel_DemuxUnknownSCID(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	frame, _ := tmdl.NewTMTransferFrame(999, 1, []byte("data"), nil, nil)
	if !errors.Is(pc.AddFrame(frame), tmdl.ErrMasterChannelNotFound) {
		t.Error("Expected ErrMasterChannelNotFound")
	}
}

func TestPhysicalChannel_GetNextFrameOrIdle(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("test", config)
	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	frame, _ := pc.GetNextFrameOrIdle()
	if !tmdl.IsIdleFrame(frame) {
		t.Error("Expected idle when empty")
	}

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	_ = svc.Send(makeTestPacket([]byte{0x01}))
	_ = svc.Flush()

	frame, _ = pc.GetNextFrameOrIdle()
	if tmdl.IsIdleFrame(frame) {
		t.Error("Expected data frame")
	}
}

func TestPhysicalChannel_NoMasterChannels(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("empty", tmdl.ChannelConfig{})
	_, err := pc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoMasterChannels) {
		t.Errorf("Expected ErrNoMasterChannels, got %v", err)
	}
}

func TestPhysicalChannel_HasPendingFrames(t *testing.T) {
	pc := tmdl.NewPhysicalChannel("test", tmdl.ChannelConfig{})
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	if pc.HasPendingFrames() {
		t.Error("Expected false")
	}
	_ = vc.Add(&tmdl.TMTransferFrame{})
	if !pc.HasPendingFrames() {
		t.Error("Expected true")
	}
}

