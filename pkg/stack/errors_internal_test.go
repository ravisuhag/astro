package stack

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// TestReceiverNextPropagatesServiceErrors reproduces B7 on the downlink
// side: Next used to treat any error from the packet service's Receive as
// "nothing ready yet", which made a real reassembly failure indistinguishable
// from an empty buffer. Reaching past SetPacketSizer to force the service
// into that state is only possible from inside the package (a caller using
// stack.NewReceiver always gets one wired up), which is why this lives in an
// internal test rather than alongside the black-box ones.
func TestReceiverNextPropagatesServiceErrors(t *testing.T) {
	config := Downlink{
		SpacecraftID: 42,
		FrameLength:  64,
		Channels:     []VC{{ID: 0}},
	}
	receiver, err := NewReceiver(config)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	// Undo the packet sizer NewReceiver wires up, so Receive() reports
	// tmdl.ErrNoPacketSizer instead of an empty buffer.
	receiver.services[0].SetPacketSizer(nil)

	_, ok, err := receiver.Next(0)
	if ok {
		t.Fatal("Next reported a packet ready from a service with no packet sizer")
	}
	if !errors.Is(err, tmdl.ErrNoPacketSizer) {
		t.Errorf("Next error = %v, want ErrNoPacketSizer surfaced rather than swallowed", err)
	}

	// Packets' error arm was dead code while Next only ever returned a nil
	// error: check it actually yields the failure and stops rather than
	// looping or being skipped.
	seen := 0
	for _, yieldErr := range receiver.Packets(0) {
		seen++
		if !errors.Is(yieldErr, tmdl.ErrNoPacketSizer) {
			t.Errorf("Packets yielded error %v, want ErrNoPacketSizer", yieldErr)
		}
	}
	if seen != 1 {
		t.Errorf("Packets yielded %d times, want exactly 1 (the error, then stop)", seen)
	}
}

// TestOnboardNextPropagatesServiceErrors is TestReceiverNextPropagatesServiceErrors
// for the uplink's spacecraft side.
func TestOnboardNextPropagatesServiceErrors(t *testing.T) {
	config := Uplink{
		SpacecraftID: 42,
		Channels:     []UplinkVC{{ID: 0}},
	}
	onboard, err := NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	onboard.services[0].SetPacketSizer(nil)

	_, ok, err := onboard.Next(0)
	if ok {
		t.Fatal("Next reported a packet ready from a service with no packet sizer")
	}
	if !errors.Is(err, tcdl.ErrNoPacketSizer) {
		t.Errorf("Next error = %v, want ErrNoPacketSizer surfaced rather than swallowed", err)
	}
}
