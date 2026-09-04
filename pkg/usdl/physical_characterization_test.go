package usdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

// Pins PhysicalChannel's master channel handling before it moved into
// pkg/sdl. See the note in pkg/aos/physical_characterization_test.go.
func TestPhysicalChannelRoutesFramesBySCID(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 128, HasFECF: true}
	pc := usdl.NewPhysicalChannel("Ka-band", config)

	mc := usdl.NewMasterChannel(0xBEEF, config)
	mc.AddVirtualChannel(usdl.NewVirtualChannel(0, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := usdl.NewTransferFrame(0xBEEF, 0, 0, []byte("telemetry"))
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

	got, err := pc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame: %v", err)
	}
	if got.Header.SCID != 0xBEEF {
		t.Errorf("frame SCID = %#x, want 0xbeef", got.Header.SCID)
	}
}

func TestPhysicalChannelRefusesAnUnknownSCID(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 128, HasFECF: true}
	pc := usdl.NewPhysicalChannel("Ka-band", config)

	mc := usdl.NewMasterChannel(0xBEEF, config)
	mc.AddVirtualChannel(usdl.NewVirtualChannel(0, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := usdl.NewTransferFrame(7, 0, 0, []byte("telemetry"))
	if err != nil {
		t.Fatalf("building a frame: %v", err)
	}
	if err := pc.AddFrame(frame); !errors.Is(err, usdl.ErrMasterChannelNotFound) {
		t.Errorf("err = %v, want ErrMasterChannelNotFound", err)
	}
}
