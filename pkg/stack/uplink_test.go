package stack_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/stack"
)

func uplinkConfig() stack.Uplink {
	return stack.Uplink{
		SpacecraftID: 42,
		Channels: []stack.UplinkVC{
			{ID: 0, Window: 10}, // critical
			{ID: 1, Window: 10}, // routine
		},
	}
}

// command builds an encoded telecommand Space Packet.
func command(t *testing.T, apid uint16, sequence uint16, payload string) []byte {
	t.Helper()

	built, err := spp.NewTCPacket(apid, []byte(payload), spp.WithSequenceCount(sequence))
	if err != nil {
		t.Fatalf("building command: %v", err)
	}
	encoded, err := built.Encode()
	if err != nil {
		t.Fatalf("encoding command: %v", err)
	}
	return encoded
}

// The property the package exists for, on the uplink side: one configuration
// builds both ends, and a command sent arrives.
func TestUplinkRoundTrip(t *testing.T) {
	config := uplinkConfig()

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	sent := command(t, 100, 0, "SET MODE 3")
	if err := commander.Send(0, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}

	accepted := 0
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		ok, err := onboard.Accept(cltu)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if ok {
			accepted++
		}
	}
	if accepted == 0 {
		t.Fatal("the spacecraft accepted no frames")
	}

	got, ok, err := onboard.Next(0)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !ok {
		t.Fatal("no command came out on VC0")
	}
	if !bytes.Equal(got, sent) {
		t.Errorf("the command came back changed:\n got %x\nwant %x", got, sent)
	}
}

// This is what makes an uplink different from a downlink. FOP-1 will not send
// past its sliding window until a CLCW says the spacecraft has room, so a
// commander that never receives one stops, and a test that never feeds one
// back has to see it stop.
func TestUplinkStopsAtTheSlidingWindow(t *testing.T) {
	config := stack.Uplink{
		SpacecraftID: 42,
		// A window of one, so the second command cannot go until the first
		// is acknowledged.
		Channels: []stack.UplinkVC{{ID: 0, Window: 1}},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}

	for i := 0; i < 4; i++ {
		if err := commander.Send(0, command(t, 100, uint16(i), "CMD")); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	released := 0
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		if len(cltu) == 0 {
			t.Fatal("an empty CLTU")
		}
		released++
	}

	if released != 1 {
		t.Errorf("a window of one released %d CLTUs, want 1 until a CLCW arrives", released)
	}
	if pending, err := commander.Pending(0); err != nil || pending == 0 {
		t.Errorf("Pending() = %d (err %v), want the rest still held", pending, err)
	}
}

// And feeding the CLCW back releases the next one, which is the loop closing.
func TestUplinkCLCWReleasesTheNextCommand(t *testing.T) {
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{{ID: 0, Window: 1}},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	first := command(t, 100, 0, "ONE")
	second := command(t, 100, 1, "TWO")

	if err := commander.Send(0, first); err != nil {
		t.Fatalf("Send first: %v", err)
	}
	if err := commander.Send(0, second); err != nil {
		t.Fatalf("Send second: %v", err)
	}

	// Round one: the window lets exactly one frame out.
	delivered := drainUplink(t, commander, onboard)
	if delivered != 1 {
		t.Fatalf("round one delivered %d frames, want 1", delivered)
	}

	// The spacecraft reports what it accepted, and the ground acts on it.
	clcw, err := onboard.CLCW(0)
	if err != nil {
		t.Fatalf("CLCW: %v", err)
	}
	if err := commander.AcceptCLCW(clcw); err != nil {
		t.Fatalf("AcceptCLCW: %v", err)
	}

	// Round two: the window has moved, so the next frame goes.
	if delivered := drainUplink(t, commander, onboard); delivered == 0 {
		t.Error("no frame was released after the CLCW, so the window did not move")
	}

	// Both commands should now be readable, in the order they were sent.
	var received [][]byte
	for packet, err := range onboard.Packets(0) {
		if err != nil {
			t.Fatalf("Packets: %v", err)
		}
		received = append(received, packet)
	}

	if len(received) != 2 {
		t.Fatalf("got %d commands, want 2", len(received))
	}
	if !bytes.Equal(received[0], first) {
		t.Error("the first command came back changed or out of order")
	}
	if !bytes.Equal(received[1], second) {
		t.Error("the second command came back changed or out of order")
	}
}

// drainUplink moves every ready CLTU across and returns how many the
// spacecraft accepted.
func drainUplink(t *testing.T, commander *stack.Commander, onboard *stack.Onboard) int {
	t.Helper()

	accepted := 0
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		ok, err := onboard.Accept(cltu)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if ok {
			accepted++
		}
	}
	return accepted
}

// Expedited delivery bypasses the sequence check, so it goes out whatever the
// window is doing. That is what it is for.
func TestUplinkExpeditedIgnoresTheWindow(t *testing.T) {
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{{ID: 0, Window: 1}},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	// Fill the sequence-controlled window first.
	if err := commander.Send(0, command(t, 100, 0, "SEQ")); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Then an expedited one, which does not wait.
	urgent := command(t, 100, 1, "ABORT")
	if err := commander.SendExpedited(0, urgent); err != nil {
		t.Fatalf("SendExpedited: %v", err)
	}

	if delivered := drainUplink(t, commander, onboard); delivered < 2 {
		t.Errorf("delivered %d frames, want the expedited one out alongside the sequenced one",
			delivered)
	}
}

// Traffic on two channels stays on its own channel, and each has its own
// COP-1 state.
func TestUplinkKeepsChannelsApart(t *testing.T) {
	config := uplinkConfig()

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	onZero := command(t, 100, 0, "CRITICAL")
	onOne := command(t, 200, 0, "ROUTINE")

	if err := commander.Send(0, onZero); err != nil {
		t.Fatalf("Send on VC0: %v", err)
	}
	if err := commander.Send(1, onOne); err != nil {
		t.Fatalf("Send on VC1: %v", err)
	}
	drainUplink(t, commander, onboard)

	gotZero, ok, err := onboard.Next(0)
	if err != nil || !ok {
		t.Fatalf("VC0 had no command: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(gotZero, onZero) {
		t.Error("VC0 returned the wrong command")
	}

	gotOne, ok, err := onboard.Next(1)
	if err != nil || !ok {
		t.Fatalf("VC1 had no command: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(gotOne, onOne) {
		t.Error("VC1 returned the wrong command")
	}
}

// Randomisation has to be applied on the way out and undone on the way in,
// and both ends read one flag.
func TestUplinkRandomized(t *testing.T) {
	config := uplinkConfig()
	config.Randomize = true

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	sent := command(t, 100, 0, "RANDOMIZED")
	if err := commander.Send(0, sent); err != nil {
		t.Fatalf("Send: %v", err)
	}
	drainUplink(t, commander, onboard)

	got, ok, err := onboard.Next(0)
	if err != nil || !ok {
		t.Fatalf("no command came back: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(got, sent) {
		t.Error("a randomised uplink did not round-trip")
	}
}

// A commander whose randomisation setting differs from the spacecraft's
// cannot be understood, which is the failure the shared configuration
// prevents, forced here to show it is real.
func TestUplinkMismatchedRandomizationFails(t *testing.T) {
	sending := uplinkConfig()
	sending.Randomize = true

	receiving := uplinkConfig()
	receiving.Randomize = false

	commander, err := stack.NewCommander(sending)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(receiving)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	if err := commander.Send(0, command(t, 100, 0, "CMD")); err != nil {
		t.Fatalf("Send: %v", err)
	}

	recovered := 0
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		if ok, err := onboard.Accept(cltu); err != nil || !ok {
			continue
		}
		if _, ok, _ := onboard.Next(0); ok {
			recovered++
		}
	}
	if recovered > 0 {
		t.Errorf("a spacecraft with the wrong randomisation setting recovered %d commands", recovered)
	}
}

// A frame FARM-1 rejects is not an error. A retransmission of something
// already accepted is exactly what the procedure filters, and reporting it as
// a failure would make ordinary operation look broken.
func TestUplinkDuplicateIsRejectedNotFailed(t *testing.T) {
	config := uplinkConfig()

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	if err := commander.Send(0, command(t, 100, 0, "ONCE")); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var cltus [][]byte
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		cltus = append(cltus, append([]byte{}, cltu...))
	}
	if len(cltus) == 0 {
		t.Fatal("no CLTUs to replay")
	}

	for _, cltu := range cltus {
		if ok, err := onboard.Accept(cltu); err != nil || !ok {
			t.Fatalf("the first delivery was not accepted: ok=%v err=%v", ok, err)
		}
	}

	// The same frames again. FARM-1 has moved past them.
	for _, cltu := range cltus {
		ok, err := onboard.Accept(cltu)
		if err != nil {
			t.Errorf("replaying a frame reported an error rather than a rejection: %v", err)
		}
		if ok {
			t.Error("a frame already accepted was accepted a second time")
		}
	}
}

// FARM-1 counts a frame as accepted the instant it arrives, before it knows
// whether the virtual channel had room to store it. Without buffer
// accounting, a full buffer means the store fails after the count already
// advanced: the next CLCW reports the frame delivered, the ground prunes it
// from FOP-1's sent queue, and the command is gone for good. This sends more
// commands than the channel buffer holds, without draining in between, and
// checks that every one of them still arrives once FARM-1's Wait state and a
// retransmission round have run their course.
func TestUplinkOverfillingTheBufferDoesNotDropCommands(t *testing.T) {
	// A window generous enough that FOP-1's sliding window is never what
	// holds a frame back (100 is comfortably under cop.MaxWindow, the
	// COP-1 ceiling: any wider and a lost CLCW could not be told apart
	// from a wrap of the 8-bit sequence number), paired with a small
	// explicit buffer so that buffer, not the window, is the only limit
	// this test means to hit.
	channel := stack.UplinkVC{ID: 0, Window: 100, Buffer: 8}
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{channel},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	total := channel.Buffer + 2
	var sent []string
	for i := 0; i < total; i++ {
		text := fmt.Sprintf("CMD%d", i)
		sent = append(sent, text)
		if err := commander.Send(0, command(t, 100, uint16(i), text)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	// Every produced CLTU, taken without draining Next in between: the
	// application has not caught up, which is exactly when a buffer fills.
	// A store failure here is the bug itself (FARM already counted the
	// frame), so it is not fatal to the test.
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		_, _ = onboard.Accept(cltu) // deliberately ignored: see comment above.
	}

	var got []string
	drain := func() {
		for packet, err := range onboard.Packets(0) {
			if err != nil {
				t.Fatalf("Packets: %v", err)
			}
			decoded, err := spp.Decode(packet)
			if err != nil {
				t.Fatalf("decoding a command: %v", err)
			}
			got = append(got, string(decoded.UserData))
		}
	}
	drain()

	// Route the CLCW back so the ground learns what happened, and offer
	// whatever that unblocks: with the buffer freed by the drain above, a
	// retransmission round should recover anything FARM-1 held back.
	clcw, err := onboard.CLCW(0)
	if err != nil {
		t.Fatalf("CLCW: %v", err)
	}
	if err := commander.AcceptCLCW(clcw); err != nil {
		t.Fatalf("AcceptCLCW: %v", err)
	}
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		_, _ = onboard.Accept(cltu) // deliberately ignored: a store failure here is the bug itself.
	}
	drain()

	if len(got) != len(sent) {
		t.Fatalf("delivered %d of %d commands sent", len(got), len(sent))
	}
	for i := range sent {
		if got[i] != sent[i] {
			t.Errorf("command %d = %q, want %q", i, got[i], sent[i])
		}
	}
}

// The Wait flag in the CLCW is how the ground learns a buffer, not the
// sliding window, is why commands stopped moving. It should appear while the
// buffer is exhausted and clear once the application drains enough to free
// one.
func TestUplinkCLCWReportsWaitWhileBufferIsFull(t *testing.T) {
	// Same reasoning as TestUplinkOverfillingTheBufferDoesNotDropCommands:
	// a window under cop.MaxWindow so FOP-1 never throttles, and a small
	// explicit buffer so the buffer is what fills.
	channel := stack.UplinkVC{ID: 0, Window: 100, Buffer: 8}
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{channel},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	// One more command than the buffer holds, so the last one has nowhere
	// to go.
	total := channel.Buffer + 1
	for i := 0; i < total; i++ {
		if err := commander.Send(0, command(t, 100, uint16(i), fmt.Sprintf("CMD%d", i))); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	for cltu, err := range commander.CLTUs() {
		if err != nil {
			t.Fatalf("CLTUs: %v", err)
		}
		_, _ = onboard.Accept(cltu) // deliberately ignored: a store failure here is the bug itself.
	}

	if clcw := decodeCLCW(t, onboard, 0); !clcw.WaitFlag {
		t.Error("CLCW Wait flag is not set while the buffer is exhausted")
	}

	// Draining one packet frees exactly one buffer, which should be enough
	// to clear the Wait flag.
	if _, ok, err := onboard.Next(0); err != nil || !ok {
		t.Fatalf("Next: ok=%v err=%v", ok, err)
	}

	if clcw := decodeCLCW(t, onboard, 0); clcw.WaitFlag {
		t.Error("CLCW Wait flag is still set after draining a buffer's worth")
	}
}

// decodeCLCW reads back the control word Onboard would send on the
// telemetry link, decoded so a test can inspect its flags.
func decodeCLCW(t *testing.T, onboard *stack.Onboard, vcid uint8) cop.CLCW {
	t.Helper()

	encoded, err := onboard.CLCW(vcid)
	if err != nil {
		t.Fatalf("CLCW: %v", err)
	}
	var clcw cop.CLCW
	if err := clcw.Decode(encoded); err != nil {
		t.Fatalf("decoding CLCW: %v", err)
	}
	return clcw
}

func TestUplinkUnknownChannel(t *testing.T) {
	commander, err := stack.NewCommander(uplinkConfig())
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(uplinkConfig())
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	if err := commander.Send(9, command(t, 100, 0, "X")); !errors.Is(err, stack.ErrUnknownChannel) {
		t.Errorf("Send to an unconfigured channel = %v, want ErrUnknownChannel", err)
	}
	if _, _, err := onboard.Next(9); !errors.Is(err, stack.ErrUnknownChannel) {
		t.Errorf("Next on an unconfigured channel = %v, want ErrUnknownChannel", err)
	}
	if _, err := onboard.CLCW(9); !errors.Is(err, stack.ErrUnknownChannel) {
		t.Errorf("CLCW for an unconfigured channel = %v, want ErrUnknownChannel", err)
	}
}

func TestUplinkConfigValidation(t *testing.T) {
	for name, config := range map[string]stack.Uplink{
		"no channels": {SpacecraftID: 42},
		"channel above the 6-bit field": {
			SpacecraftID: 42,
			Channels:     []stack.UplinkVC{{ID: 64}},
		},
		"duplicate channel": {
			SpacecraftID: 42,
			Channels:     []stack.UplinkVC{{ID: 1}, {ID: 1}},
		},
		"negative buffer": {
			SpacecraftID: 42,
			Channels:     []stack.UplinkVC{{ID: 0, Buffer: -1}},
		},
		"window above the COP-1 maximum": {
			SpacecraftID: 42,
			// One past cop.MaxWindow (127): the smallest value that must be
			// refused.
			Channels: []stack.UplinkVC{{ID: 0, Window: 128}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := config.Validate(); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("Validate() = %v, want ErrInvalidConfig", err)
			}
			if _, err := stack.NewCommander(config); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("NewCommander() = %v, want ErrInvalidConfig", err)
			}
			if _, err := stack.NewOnboard(config); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("NewOnboard() = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

// The window ceiling is exactly cop.MaxWindow: at that value a channel is
// still legal, and zero (meaning "unset") takes DefaultWindow rather than
// being rejected as if it were an oversized window.
func TestUplinkWindowAtTheCeilingIsValid(t *testing.T) {
	for name, window := range map[string]uint8{
		"at MaxWindow":           cop.MaxWindow,
		"zero takes the default": 0,
	} {
		t.Run(name, func(t *testing.T) {
			config := stack.Uplink{
				SpacecraftID: 42,
				Channels:     []stack.UplinkVC{{ID: 0, Window: window}},
			}
			if err := config.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if _, err := stack.NewCommander(config); err != nil {
				t.Errorf("NewCommander() = %v, want nil", err)
			}
			if _, err := stack.NewOnboard(config); err != nil {
				t.Errorf("NewOnboard() = %v, want nil", err)
			}
		})
	}
}

// The state is what to look at when commands stop going out, so it has to be
// reachable.
func TestUplinkReportsState(t *testing.T) {
	commander, err := stack.NewCommander(uplinkConfig())
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(uplinkConfig())
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	// A freshly initialised channel is active and open.
	if state, err := commander.State(0); err != nil || state != cop.FOPActive {
		t.Errorf("FOP state = %v (err %v), want active", state, err)
	}
	if state, err := onboard.State(0); err != nil || state != cop.FARMOpen {
		t.Errorf("FARM state = %v (err %v), want open", state, err)
	}
}

func TestAcceptRejectsRubbishCLTU(t *testing.T) {
	onboard, err := stack.NewOnboard(uplinkConfig())
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	if _, err := onboard.Accept(bytes.Repeat([]byte{0xAA}, 64)); err == nil {
		t.Error("Accept took a CLTU that is not one")
	}
	if _, err := onboard.Accept(nil); err == nil {
		t.Error("Accept took an empty CLTU")
	}
}

// A blocked AD frame must not hold up the BD frame queued behind it: type
// BD "arrive[s] whatever state FOP-1 is in" (see the package doc), so an
// expedited command sent while the AD window is full has to reach the wire
// without waiting for a CLCW. This fills the window, leaves a second AD
// command stuck behind it, and only then sends the expedited one.
func TestUplinkExpeditedSkipsAStalledADBacklog(t *testing.T) {
	config := stack.Uplink{
		SpacecraftID: 42,
		// A window of one, so the second AD command has nowhere to go and
		// sits in the backlog rather than in flight.
		Channels: []stack.UplinkVC{{ID: 0, Window: 1}},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		t.Fatalf("NewOnboard: %v", err)
	}

	// Fills the window: this one goes straight to FOP-1.
	if err := commander.Send(0, command(t, 100, 0, "FIRST")); err != nil {
		t.Fatalf("Send first: %v", err)
	}
	// The window is full, so this one sits in the backlog: the blocked AD
	// frame at the front of the queue.
	if err := commander.Send(0, command(t, 100, 1, "SECOND")); err != nil {
		t.Fatalf("Send second: %v", err)
	}
	// Queued behind it, an expedited command that should not have to wait.
	if err := commander.SendExpedited(0, command(t, 100, 2, "ABORT")); err != nil {
		t.Fatalf("SendExpedited: %v", err)
	}

	delivered := drainUplink(t, commander, onboard)
	if delivered != 2 {
		t.Errorf("delivered %d frames, want 2 (FIRST and ABORT) — the stalled "+
			"AD command must not hold up the expedited one behind it", delivered)
	}

	// SECOND is still stuck behind the window: nothing freed it.
	if pending, err := commander.Pending(0); err != nil || pending == 0 {
		t.Errorf("Pending(0) = %d (err %v), want SECOND still held", pending, err)
	}
}

// A FOP-1 Alert takes a channel out of the Active state (S1-S3): after it,
// TransmitFrame on that channel returns ErrFOPNotActive until the operator
// recovers it. That must be a fault on the one channel, not on the whole
// commander: NextCLTU still has to serve every other channel, and there has
// to be a way back for the one that alerted.
func TestUplinkAlertOnOneChannelDoesNotStarveTheOthers(t *testing.T) {
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels: []stack.UplinkVC{
			{ID: 0, Window: 1}, // this one will be driven into an alert
			{ID: 1, Window: 10},
		},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		t.Fatalf("NewCommander: %v", err)
	}

	// VC0: fill the window, then queue a second command that stays stuck
	// in the backlog behind it.
	if err := commander.Send(0, command(t, 100, 0, "FIRST")); err != nil {
		t.Fatalf("Send VC0 first: %v", err)
	}
	if err := commander.Send(0, command(t, 100, 1, "SECOND")); err != nil {
		t.Fatalf("Send VC0 second: %v", err)
	}

	// A CLCW with the Lockout flag set alerts FOP-1 on VC0 (E14): it leaves
	// Active for Initial, and every later TransmitFrame on it fails.
	lockout := cop.CLCW{VirtualChannelID: 0, LockoutFlag: true}
	encoded, err := lockout.Encode()
	if err != nil {
		t.Fatalf("encoding the lockout CLCW: %v", err)
	}
	if err := commander.AcceptCLCW(encoded); err == nil {
		t.Fatal("AcceptCLCW with Lockout set did not report the alert")
	}
	if state, err := commander.State(0); err != nil || state != cop.FOPInitial {
		t.Fatalf("VC0 state = %v (err %v), want Initial after the alert", state, err)
	}

	// VC1 has a command ready and no reason to care what happened on VC0.
	if err := commander.Send(1, command(t, 200, 0, "ROUTINE")); err != nil {
		t.Fatalf("Send VC1: %v", err)
	}

	sawVC1 := false
	sawWedgeError := false
	for i := 0; i < 5; i++ {
		cltu, ok, err := commander.NextCLTU()
		if err != nil {
			sawWedgeError = true
		}
		if ok && len(cltu) > 0 {
			sawVC1 = true
		}
		if !ok {
			break
		}
	}
	if !sawVC1 {
		t.Error("a wedged VC0 kept VC1's ready command from ever going out")
	}
	if !sawWedgeError {
		t.Error("NextCLTU never reported VC0's alert: the fault must not be swallowed")
	}

	// Recovery: an operator brings VC0 back without rebuilding the
	// Commander.
	if err := commander.Initialize(0, 0); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if state, err := commander.State(0); err != nil || state != cop.FOPActive {
		t.Fatalf("VC0 state after Initialize = %v (err %v), want active", state, err)
	}

	// And the command that was stuck behind the alert now goes out.
	cltu, ok, err := commander.NextCLTU()
	if err != nil {
		t.Fatalf("NextCLTU after recovery: %v", err)
	}
	if !ok || len(cltu) == 0 {
		t.Error("VC0 produced nothing after recovery, want SECOND to be offered again")
	}
}

// The uplink half of the "few lines" claim, as runnable code.
func ExampleCommander() {
	config := stack.Uplink{
		SpacecraftID: 42,
		Channels:     []stack.UplinkVC{{ID: 0}},
	}

	commander, err := stack.NewCommander(config)
	if err != nil {
		panic(err)
	}
	onboard, err := stack.NewOnboard(config)
	if err != nil {
		panic(err)
	}

	telecommand, err := spp.NewTCPacket(100, []byte("SET MODE 3"))
	if err != nil {
		panic(err)
	}
	if err := commander.SendPacket(0, telecommand); err != nil {
		panic(err)
	}

	for cltu, err := range commander.CLTUs() {
		if err != nil {
			panic(err)
		}
		if _, err := onboard.Accept(cltu); err != nil {
			panic(err)
		}
	}

	received, ok, err := onboard.Next(0)
	if err != nil || !ok {
		panic(fmt.Sprintf("no command: %v", err))
	}

	packet, err := spp.Decode(received)
	if err != nil {
		panic(err)
	}
	fmt.Printf("APID %d: %s\n", packet.PrimaryHeader.APID, packet.UserData)

	// Output: APID 100: SET MODE 3
}
