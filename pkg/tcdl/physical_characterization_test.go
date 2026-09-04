package tcdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

// Pins PhysicalChannel's master channel handling before it moved into
// pkg/sdl. See the note in pkg/aos/physical_characterization_test.go.
func TestPhysicalChannelRoutesFramesBySCID(t *testing.T) {
	pc := tcdl.NewPhysicalChannel("S-band")

	mc := tcdl.NewMasterChannel(0x0AB)
	mc.AddVirtualChannel(tcdl.NewVirtualChannel(1, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := tcdl.NewTCTransferFrame(0x0AB, 1, []byte("command"))
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
	if got.Header.SpacecraftID != 0x0AB {
		t.Errorf("frame SCID = %#x, want 0xab", got.Header.SpacecraftID)
	}
}

func TestPhysicalChannelRefusesAnUnknownSCID(t *testing.T) {
	pc := tcdl.NewPhysicalChannel("S-band")

	mc := tcdl.NewMasterChannel(0x0AB)
	mc.AddVirtualChannel(tcdl.NewVirtualChannel(1, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := tcdl.NewTCTransferFrame(7, 1, []byte("command"))
	if err != nil {
		t.Fatalf("building a frame: %v", err)
	}
	if err := pc.AddFrame(frame); !errors.Is(err, tcdl.ErrMasterChannelNotFound) {
		t.Errorf("err = %v, want ErrMasterChannelNotFound", err)
	}
}
