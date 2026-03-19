package tcdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

func TestPhysicalChannel_MCMultiplexing(t *testing.T) {
	pc := tcdl.NewPhysicalChannel("TC-uplink")
	mc1 := tcdl.NewMasterChannel(100)
	mc2 := tcdl.NewMasterChannel(200)

	vc1 := tcdl.NewVirtualChannel(1, 10)
	vc2 := tcdl.NewVirtualChannel(1, 10)
	mc1.AddVirtualChannel(vc1, 1)
	mc2.AddVirtualChannel(vc2, 1)

	pc.AddMasterChannel(mc1, 1)
	pc.AddMasterChannel(mc2, 1)

	f1, _ := tcdl.NewTCTransferFrame(100, 1, []byte("sc100"))
	f2, _ := tcdl.NewTCTransferFrame(200, 1, []byte("sc200"))
	vc1.AddFrame(f1)
	vc2.AddFrame(f2)

	got1, _ := pc.GetNextFrame()
	if got1.Header.SpacecraftID != 100 {
		t.Errorf("first frame SCID = %d, want 100", got1.Header.SpacecraftID)
	}
	got2, _ := pc.GetNextFrame()
	if got2.Header.SpacecraftID != 200 {
		t.Errorf("second frame SCID = %d, want 200", got2.Header.SpacecraftID)
	}
}

func TestPhysicalChannel_NoMasterChannels(t *testing.T) {
	pc := tcdl.NewPhysicalChannel("empty")
	_, err := pc.GetNextFrame()
	if !errors.Is(err, tcdl.ErrNoMasterChannels) {
		t.Errorf("expected ErrNoMasterChannels, got %v", err)
	}
}
