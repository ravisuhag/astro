package stack_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/stack"
)

// The two composers are one system, not two, and the operational control
// field is what joins them: FOP-1 on the ground will not send past its
// sliding window until a CLCW comes back saying what the spacecraft accepted,
// and that CLCW travels in the OCF of a telemetry frame.
//
// This is the property WithOCF and Receiver.NextOCF exist for. Without them a
// downlink built by this package carried four zero octets, which the ground
// reads as a spacecraft acknowledging nothing, and the window never moved.
func TestFullDuplexThroughTheComposer(t *testing.T) {
	// A window of one, so the second command cannot go until the first is
	// acknowledged. That makes the test fail if the CLCW does not arrive.
	uplink := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{{ID: 0, Window: 1}},
	}

	commander, err := stack.NewCommander(uplink)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(uplink)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	// The downlink carries the CLCW the spacecraft's FARM-1 generates. This
	// closure is the whole join between the two directions.
	downlink := stack.Downlink{
		SpacecraftID: 42,
		FrameLength:  64,
		FECF:         true,
		OCF:          true,
		Channels:     []stack.VC{{ID: 0, Priority: 1}},
	}

	sender, err := stack.NewSender(downlink, stack.WithOCF(func() []byte {
		field, err := onboard.CLCW(0)
		if err != nil {
			return nil // a nil field fails the frame rather than faking one
		}
		return field
	}))
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	receiver, err := stack.NewReceiver(downlink)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	commands := []string{"SET MODE 3", "POINT 12.5 -3.1", "START SCAN"}
	accepted := 0

	for i, text := range commands {
		if err := commander.Send(0, command(t, 100, uint16(i), text)); err != nil {
			t.Fatalf("Send %q: %v", text, err)
		}

		// Uplink whatever FOP-1 will release. With a window of one that is a
		// single frame until a CLCW moves it.
		for cltu, err := range commander.CLTUs() {
			if err != nil {
				t.Fatalf("CLTUs: %v", err)
			}
			took, err := onboard.Accept(cltu)
			if err != nil {
				t.Fatalf("Accept CLTU: %v", err)
			}
			if took {
				accepted++
			}
		}

		// One telemetry frame down, carrying the CLCW.
		if err := sender.Send(0, packet(t, 200, uint16(i), 8)); err != nil {
			t.Fatalf("Send telemetry: %v", err)
		}
		if err := sender.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
		for cadu, err := range sender.CADUs() {
			if err != nil {
				t.Fatalf("CADUs: %v", err)
			}
			if err := receiver.Accept(cadu); err != nil {
				t.Fatalf("Accept CADU: %v", err)
			}
		}

		// Feed every CLCW that came home back to FOP-1, which is what
		// advances the window.
		fields := 0
		for field := range receiver.OCFs() {
			if err := commander.AcceptCLCW(field); err != nil {
				t.Fatalf("AcceptCLCW: %v", err)
			}
			fields++
		}
		if fields == 0 {
			t.Fatalf("round %d: no operational control field came home", i)
		}
	}

	if accepted != len(commands) {
		t.Errorf("the spacecraft accepted %d of %d commands; with a window of "+
			"one, a CLCW that never arrives stops the uplink after the first",
			accepted, len(commands))
	}

	if pending, err := commander.Pending(0); err != nil {
		t.Fatalf("Pending: %v", err)
	} else if pending != 0 {
		t.Errorf("%d frames still outstanding, want 0: the window did not clear",
			pending)
	}

	// The commands themselves survived, which is the point of the uplink.
	var got []string
	for encoded, err := range onboard.Packets(0) {
		if err != nil {
			t.Fatalf("Packets: %v", err)
		}
		decoded, err := spp.Decode(encoded)
		if err != nil {
			t.Fatalf("decoding a command: %v", err)
		}
		got = append(got, string(decoded.UserData))
	}
	if len(got) != len(commands) {
		t.Fatalf("recovered %d commands, want %d: %q", len(got), len(commands), got)
	}
	for i := range commands {
		if got[i] != commands[i] {
			t.Errorf("command %d = %q, want %q", i, got[i], commands[i])
		}
	}
}

// The same link with the OCF supplier missing is exactly what the old
// behaviour was: the ground would have been handed four zero octets. It is
// now unconstructable.
func TestFullDuplexRefusesAFakeCLCW(t *testing.T) {
	downlink := stack.Downlink{
		SpacecraftID: 42,
		FrameLength:  64,
		FECF:         true,
		OCF:          true,
		Channels:     []stack.VC{{ID: 0, Priority: 1}},
	}
	if _, err := stack.NewSender(downlink); err == nil {
		t.Error("a downlink reserving the OCF was built with nothing to put in it")
	}
}
