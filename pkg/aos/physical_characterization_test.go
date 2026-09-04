package aos_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

// These tests pin the behaviour of PhysicalChannel's master channel handling
// before that handling moved into pkg/sdl.
//
// They exist because the same five methods were copy-pasted across aos, usdl,
// tmdl and tcdl, and consolidating them behind one generic is only safe if
// each package's observable behaviour is written down first. The point is not
// that these assertions are surprising — it is that they were identical in
// four places and now come from one.
func TestPhysicalChannelRoutesFramesBySCID(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 128}
	pc := aos.NewPhysicalChannel("X-band", config)

	mc := aos.NewMasterChannel(42, config)
	vc := aos.NewVirtualChannel(0, 4)
	mc.AddVirtualChannel(vc, 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := aos.NewTransferFrame(42, 0, []byte("telemetry"))
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
	if got.Header.SCID != 42 {
		t.Errorf("frame SCID = %d, want 42", got.Header.SCID)
	}
	if pc.HasPendingFrames() {
		t.Error("a drained channel still reports pending frames")
	}
}

// A frame for a spacecraft this channel does not carry is refused rather than
// routed to whichever master channel happens to be registered.
func TestPhysicalChannelRefusesAnUnknownSCID(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 128}
	pc := aos.NewPhysicalChannel("X-band", config)

	mc := aos.NewMasterChannel(42, config)
	mc.AddVirtualChannel(aos.NewVirtualChannel(0, 4), 1)
	pc.AddMasterChannel(mc, 1)

	frame, err := aos.NewTransferFrame(7, 0, []byte("telemetry"))
	if err != nil {
		t.Fatalf("building a frame: %v", err)
	}
	if err := pc.AddFrame(frame); !errors.Is(err, aos.ErrMasterChannelNotFound) {
		t.Errorf("err = %v, want ErrMasterChannelNotFound", err)
	}
}
