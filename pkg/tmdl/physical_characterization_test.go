package tmdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// Pins PhysicalChannel's master channel handling before it moved into
// pkg/sdl. See the note in pkg/aos/physical_characterization_test.go.
func TestPhysicalChannelRoutesFramesBySCID(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 128, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("X-band", config)

	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(tmdl.NewVirtualChannel(0, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := tmdl.NewTMTransferFrame(933, 0, []byte("telemetry"), nil, nil)
	if err != nil {
		t.Fatalf("building a frame: %v", err)
	}

	if pc.HasPendingFrames() {
		t.Error("a channel with nothing added reports pending frames")
	}
	if err := pc.AddFrame(frame); err != nil {
		t.Fatalf("AddFrame: %v", err)
	}
	if !pc.HasPendingFrames() {
		t.Error("a channel holding a frame reports none pending")
	}
	if got := pc.Len(); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}

	got, err := pc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame: %v", err)
	}
	if got.Header.SpacecraftID != 933 {
		t.Errorf("frame SCID = %d, want 933", got.Header.SpacecraftID)
	}
}

func TestPhysicalChannelRefusesAnUnknownSCID(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 128, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("X-band", config)

	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(tmdl.NewVirtualChannel(0, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := tmdl.NewTMTransferFrame(7, 0, []byte("telemetry"), nil, nil)
	if err != nil {
		t.Fatalf("building a frame: %v", err)
	}
	if err := pc.AddFrame(frame); !errors.Is(err, tmdl.ErrMasterChannelNotFound) {
		t.Errorf("err = %v, want ErrMasterChannelNotFound", err)
	}
}
