package stack_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/stack"
)

// testConfig is a two-channel downlink at a frame length short enough that a
// packet has to span frames, which is the case worth testing.
func testConfig() stack.Downlink {
	return stack.Downlink{
		SpacecraftID: 42,
		FrameLength:  64,
		FECF:         true,
		Channels: []stack.VC{
			{ID: 0, Priority: 3},
			{ID: 1, Priority: 1},
		},
	}
}

// packet builds an encoded Space Packet of the given payload size.
func packet(t *testing.T, apid uint16, sequence uint16, size int) []byte {
	t.Helper()

	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(apid) ^ byte(i)
	}

	built, err := spp.NewTMPacket(apid, payload, spp.WithSequenceCount(sequence))
	if err != nil {
		t.Fatalf("building packet: %v", err)
	}
	encoded, err := built.Encode()
	if err != nil {
		t.Fatalf("encoding packet: %v", err)
	}
	return encoded
}

// transfer sends packets on a channel and returns what the receiver gets
// back, with both ends built from the same configuration.
func transfer(t *testing.T, config stack.Downlink, vcid uint8, packets [][]byte) [][]byte {
	t.Helper()

	got, _ := transferWithOCF(t, config, vcid, packets, nil)
	return got
}

// transferWithOCF is transfer for a channel carrying an operational control
// field. It returns the packets recovered and the fields the receiver read.
func transferWithOCF(t *testing.T, config stack.Downlink, vcid uint8,
	packets [][]byte, ocf func() []byte) (recovered [][]byte, fields [][]byte) {
	t.Helper()

	var options []stack.SenderOption
	if ocf != nil {
		options = append(options, stack.WithOCF(ocf))
	}

	sender, err := stack.NewSender(config, options...)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	for i, p := range packets {
		if err := sender.Send(vcid, p); err != nil {
			t.Fatalf("Send packet %d: %v", i, err)
		}
	}
	if err := sender.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	receiver, err := stack.NewReceiver(config)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	cadus := 0
	for cadu, err := range sender.CADUs() {
		if err != nil {
			t.Fatalf("CADUs: %v", err)
		}
		if err := receiver.Accept(cadu); err != nil {
			t.Fatalf("Accept CADU %d: %v", cadus, err)
		}
		cadus++
	}
	if cadus == 0 {
		t.Fatal("the sender produced no CADUs")
	}

	for field := range receiver.OCFs() {
		fields = append(fields, field)
	}

	var got [][]byte
	for p, err := range receiver.Packets(vcid) {
		if err != nil {
			t.Fatalf("Packets: %v", err)
		}
		got = append(got, p)
	}
	return got, fields
}

// The property the package exists for: one configuration builds both ends,
// and what goes in comes out.
func TestDownlinkRoundTrip(t *testing.T) {
	sent := [][]byte{
		packet(t, 100, 0, 10),
		packet(t, 100, 1, 10),
		packet(t, 100, 2, 10),
	}

	got := transfer(t, testConfig(), 0, sent)

	if len(got) != len(sent) {
		t.Fatalf("got %d packets back, sent %d", len(got), len(sent))
	}
	for i := range sent {
		if !bytes.Equal(got[i], sent[i]) {
			t.Errorf("packet %d came back changed:\n got %x\nwant %x", i, got[i], sent[i])
		}
	}
}

// A packet longer than a frame has to be split and put back together, which
// is the whole reason the packet service sits between packets and frames.
func TestDownlinkSpansFrames(t *testing.T) {
	config := testConfig()

	// Several frames' worth in one packet.
	big := packet(t, 200, 0, config.FrameLength*3)
	got := transfer(t, config, 0, [][]byte{big})

	if len(got) != 1 {
		t.Fatalf("got %d packets back, sent 1 spanning packet", len(got))
	}
	if !bytes.Equal(got[0], big) {
		t.Errorf("the spanning packet came back changed: got %d octets, want %d", len(got[0]), len(big))
	}
}

// Several packets that fit in one frame must all come back, which is the
// other half of the packing.
func TestDownlinkPacksSeveralPerFrame(t *testing.T) {
	var sent [][]byte
	for i := 0; i < 6; i++ {
		sent = append(sent, packet(t, 100, uint16(i), 4))
	}

	got := transfer(t, testConfig(), 0, sent)

	if len(got) != len(sent) {
		t.Fatalf("got %d packets back, sent %d", len(got), len(sent))
	}
	for i := range sent {
		if !bytes.Equal(got[i], sent[i]) {
			t.Errorf("packet %d came back changed", i)
		}
	}
}

// Traffic on two channels stays on its own channel.
func TestDownlinkKeepsChannelsApart(t *testing.T) {
	config := testConfig()

	sender, err := stack.NewSender(config)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}

	onZero := packet(t, 100, 0, 8)
	onOne := packet(t, 200, 0, 8)

	if err := sender.Send(0, onZero); err != nil {
		t.Fatalf("Send on VC0: %v", err)
	}
	if err := sender.Send(1, onOne); err != nil {
		t.Fatalf("Send on VC1: %v", err)
	}
	if err := sender.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	receiver, err := stack.NewReceiver(config)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	for cadu, err := range sender.CADUs() {
		if err != nil {
			t.Fatalf("CADUs: %v", err)
		}
		if err := receiver.Accept(cadu); err != nil {
			t.Fatalf("Accept: %v", err)
		}
	}

	gotZero, ok, err := receiver.Next(0)
	if err != nil || !ok {
		t.Fatalf("VC0 had no packet: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(gotZero, onZero) {
		t.Error("VC0 returned the wrong packet")
	}

	gotOne, ok, err := receiver.Next(1)
	if err != nil || !ok {
		t.Fatalf("VC1 had no packet: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(gotOne, onOne) {
		t.Error("VC1 returned the wrong packet")
	}
}

// Randomization has to be applied on the way out and undone on the way in.
// Both ends read the same flag, so it cannot be set on one side only.
func TestDownlinkRandomized(t *testing.T) {
	config := testConfig()
	config.Randomize = true

	sent := [][]byte{packet(t, 100, 0, 12)}
	got := transfer(t, config, 0, sent)

	if len(got) != 1 || !bytes.Equal(got[0], sent[0]) {
		t.Error("a randomized downlink did not round-trip")
	}
}

// The operational control field changes the frame layout, so both ends have
// to allow for it. They read one flag.
func TestDownlinkWithOCF(t *testing.T) {
	config := testConfig()
	config.OCF = true

	field := []byte{0x01, 0x00, 0x00, 0x07}
	sent := [][]byte{packet(t, 100, 0, 12)}
	got, _ := transferWithOCF(t, config, 0, sent, func() []byte { return field })

	if len(got) != 1 || !bytes.Equal(got[0], sent[0]) {
		t.Error("a downlink carrying an OCF did not round-trip")
	}
}

// A configuration carrying an OCF has to say what goes in it. Four zero
// octets decode as a valid CLCW reporting V(R)=0, so inventing them would
// have the ground believe a spacecraft was acknowledging nothing.
func TestSenderRefusesOCFWithoutSupplier(t *testing.T) {
	config := testConfig()
	config.OCF = true

	if _, err := stack.NewSender(config); !errors.Is(err, stack.ErrMissingOCF) {
		t.Errorf("NewSender with an OCF and no supplier = %v, want ErrMissingOCF", err)
	}
}

// The opposite mistake: a supplier that would never be called.
func TestSenderRefusesSupplierWithoutOCF(t *testing.T) {
	config := testConfig() // OCF is false

	_, err := stack.NewSender(config, stack.WithOCF(func() []byte {
		return make([]byte, 4)
	}))
	if !errors.Is(err, stack.ErrInvalidConfig) {
		t.Errorf("NewSender with a supplier and no OCF = %v, want ErrInvalidConfig", err)
	}
}

// What the supplier returns is what arrives, on every frame of the channel
// whichever virtual channel released it.
func TestOCFReachesTheReceiver(t *testing.T) {
	config := testConfig()
	config.OCF = true

	// A CLCW reporting V(R)=9 on virtual channel 0: control word type 0,
	// version 0, COP-1 in effect.
	field := []byte{0x01, 0x00, 0x00, 0x09}

	// Two packets long enough to span, so several frames go down and each
	// one has to carry the field.
	sent := [][]byte{packet(t, 100, 0, 80), packet(t, 100, 1, 80)}
	got, fields := transferWithOCF(t, config, 0, sent, func() []byte { return field })

	if len(got) != len(sent) {
		t.Fatalf("recovered %d packets, want %d", len(got), len(sent))
	}
	if len(fields) < 2 {
		t.Fatalf("the receiver read %d operational control fields, want one per frame", len(fields))
	}
	for i, f := range fields {
		if !bytes.Equal(f, field) {
			t.Errorf("field %d = % x, want % x", i, f, field)
		}
	}
}

// A changing supplier is the real case: FARM-1 reports a different V(R) as
// frames are accepted, and each frame carries the value at the time it was
// built.
func TestOCFIsSampledPerFrame(t *testing.T) {
	config := testConfig()
	config.OCF = true

	report := byte(0)
	supplier := func() []byte {
		report++
		return []byte{0x01, 0x00, 0x00, report}
	}

	sent := [][]byte{packet(t, 100, 0, 200)} // spans several frames
	_, fields := transferWithOCF(t, config, 0, sent, supplier)

	if len(fields) < 2 {
		t.Fatalf("read %d fields, want several", len(fields))
	}
	for i, f := range fields {
		if want := byte(i + 1); f[3] != want {
			t.Errorf("field %d reports %d, want %d: the supplier is not being "+
				"sampled once per frame", i, f[3], want)
		}
	}
}

// A supplier returning the wrong length fails the frame rather than being
// padded or truncated, because a receiver reads the field by position.
func TestOCFWrongLengthFailsTheFrame(t *testing.T) {
	config := testConfig()
	config.OCF = true

	sender, err := stack.NewSender(config, stack.WithOCF(func() []byte {
		return []byte{0x01, 0x00} // two octets, not four
	}))
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}

	if err := sender.Send(0, packet(t, 100, 0, 12)); err != nil {
		// The failure may surface on Send, when the frame fills, or on Flush.
		return
	}
	if err := sender.Flush(); err == nil {
		t.Error("a two-octet operational control field was accepted")
	}
}

// The receiver holds a bounded queue, so a caller that never reads it does
// not grow one frame at a time forever.
func TestOCFQueueIsBounded(t *testing.T) {
	config := testConfig()
	config.OCF = true

	report := byte(0)
	sender, err := stack.NewSender(config, stack.WithOCF(func() []byte {
		report++
		return []byte{0x01, 0x00, 0x00, report}
	}))
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	receiver, err := stack.NewReceiver(config)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	// Enough traffic to produce more frames than the queue holds. The
	// packets are drained each round so the frame buffer does not fill; the
	// operational control fields deliberately are not.
	for i := range 200 {
		if err := sender.Send(0, packet(t, 100, uint16(i), 40)); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
		for cadu, err := range sender.CADUs() {
			if err != nil {
				t.Fatalf("CADUs: %v", err)
			}
			if err := receiver.Accept(cadu); err != nil {
				t.Fatalf("Accept: %v", err)
			}
		}
		for _, err := range receiver.Packets(0) {
			if err != nil {
				t.Fatalf("Packets: %v", err)
			}
		}
	}

	held := 0
	for range receiver.OCFs() {
		held++
	}
	if held == 0 {
		t.Fatal("the receiver kept no operational control fields")
	}
	if held > stack.DefaultBuffer {
		t.Errorf("the receiver held %d fields, want at most %d",
			held, stack.DefaultBuffer)
	}
	if int(report) <= held {
		t.Skip("not enough frames were produced to overflow the queue")
	}
}

func TestDownlinkWithoutFECF(t *testing.T) {
	config := testConfig()
	config.FECF = false

	sent := [][]byte{packet(t, 100, 0, 12)}
	got := transfer(t, config, 0, sent)

	if len(got) != 1 || !bytes.Equal(got[0], sent[0]) {
		t.Error("a downlink without an error control field did not round-trip")
	}
}

// This is the failure the shared configuration prevents, demonstrated by
// forcing it: a receiver built from a different frame length cannot read the
// sender's frames. Nothing in the package lets this happen by accident,
// which is the point.
func TestDownlinkMismatchedConfigFails(t *testing.T) {
	senderConfig := testConfig()
	sender, err := stack.NewSender(senderConfig)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(0, packet(t, 100, 0, 12)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := sender.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	receiverConfig := testConfig()
	receiverConfig.FrameLength = senderConfig.FrameLength + 8
	receiver, err := stack.NewReceiver(receiverConfig)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	// Either accepting the frame fails, or it is accepted and yields no
	// packet. Both are the misconfiguration showing up; what must not happen
	// is a packet coming out that looks right.
	recovered := 0
	for cadu, err := range sender.CADUs() {
		if err != nil {
			t.Fatalf("CADUs: %v", err)
		}
		if err := receiver.Accept(cadu); err != nil {
			continue
		}
		if _, ok, _ := receiver.Next(0); ok {
			recovered++
		}
	}
	if recovered > 0 {
		t.Errorf("a receiver with the wrong frame length recovered %d packets", recovered)
	}
}

func TestSendToUnknownChannel(t *testing.T) {
	sender, err := stack.NewSender(testConfig())
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}

	err = sender.Send(5, packet(t, 100, 0, 4))
	if !errors.Is(err, stack.ErrUnknownChannel) {
		t.Errorf("Send to an unconfigured channel = %v, want ErrUnknownChannel", err)
	}
}

func TestReceiveFromUnknownChannel(t *testing.T) {
	receiver, err := stack.NewReceiver(testConfig())
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	if _, _, err := receiver.Next(5); !errors.Is(err, stack.ErrUnknownChannel) {
		t.Errorf("Next on an unconfigured channel = %v, want ErrUnknownChannel", err)
	}
}

// A bad configuration is caught when the endpoint is built, not when the
// first frame fails to decode.
func TestConfigValidation(t *testing.T) {
	for name, config := range map[string]stack.Downlink{
		"no frame length": {
			SpacecraftID: 42,
			Channels:     []stack.VC{{ID: 0}},
		},
		"negative frame length": {
			SpacecraftID: 42,
			FrameLength:  -1,
			Channels:     []stack.VC{{ID: 0}},
		},
		"no channels": {
			SpacecraftID: 42,
			FrameLength:  64,
		},
		"channel above the 3-bit field": {
			SpacecraftID: 42,
			FrameLength:  64,
			Channels:     []stack.VC{{ID: 8}},
		},
		"duplicate channel": {
			SpacecraftID: 42,
			FrameLength:  64,
			Channels:     []stack.VC{{ID: 1}, {ID: 1}},
		},
		"negative buffer": {
			SpacecraftID: 42,
			FrameLength:  64,
			Channels:     []stack.VC{{ID: 0, Buffer: -1}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := config.Validate(); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("Validate() = %v, want ErrInvalidConfig", err)
			}
			if _, err := stack.NewSender(config); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("NewSender() = %v, want ErrInvalidConfig", err)
			}
			if _, err := stack.NewReceiver(config); !errors.Is(err, stack.ErrInvalidConfig) {
				t.Errorf("NewReceiver() = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

// A CADU with no sync marker is reported rather than mistaken for a frame.
func TestAcceptRejectsRubbish(t *testing.T) {
	receiver, err := stack.NewReceiver(testConfig())
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	if err := receiver.Accept(bytes.Repeat([]byte{0xAA}, 64)); err == nil {
		t.Error("Accept took a CADU with no sync marker")
	}
	if err := receiver.Accept(nil); err == nil {
		t.Error("Accept took an empty CADU")
	}
}

// The typed helpers are the common case, so they get a round trip of their
// own rather than being left to the octet-level API.
func TestSendPacketAndNextPacket(t *testing.T) {
	config := testConfig()

	sender, err := stack.NewSender(config)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}

	built, err := spp.NewTMPacket(321, []byte("hello"), spp.WithSequenceCount(9))
	if err != nil {
		t.Fatalf("NewTMPacket: %v", err)
	}
	if err := sender.SendPacket(0, built); err != nil {
		t.Fatalf("SendPacket: %v", err)
	}
	if err := sender.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	receiver, err := stack.NewReceiver(config)
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	for cadu, err := range sender.CADUs() {
		if err != nil {
			t.Fatalf("CADUs: %v", err)
		}
		if err := receiver.Accept(cadu); err != nil {
			t.Fatalf("Accept: %v", err)
		}
	}

	got, ok, err := receiver.NextPacket(0)
	if err != nil || !ok {
		t.Fatalf("NextPacket: ok=%v err=%v", ok, err)
	}
	if got.PrimaryHeader.APID != 321 {
		t.Errorf("APID = %d, want 321", got.PrimaryHeader.APID)
	}
	if got.PrimaryHeader.SequenceCount != 9 {
		t.Errorf("SequenceCount = %d, want 9", got.PrimaryHeader.SequenceCount)
	}
	if string(got.UserData) != "hello" {
		t.Errorf("UserData = %q, want %q", got.UserData, "hello")
	}
}

// HasPending is what tells a caller there is work to drain.
func TestHasPending(t *testing.T) {
	sender, err := stack.NewSender(testConfig())
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if sender.HasPending() {
		t.Error("HasPending() on a fresh sender = true, want false")
	}

	if err := sender.Send(0, packet(t, 100, 0, 12)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := sender.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !sender.HasPending() {
		t.Error("HasPending() = false after a flush, want a frame waiting")
	}

	for range sender.CADUs() {
	}
	if sender.HasPending() {
		t.Error("HasPending() = true after draining, want false")
	}
}

// The composer is meant to replace about forty lines of setup with a handful.
// This is that claim as runnable code.
func Example() {
	config := stack.Downlink{
		SpacecraftID: 42,
		FrameLength:  64,
		FECF:         true,
		Channels:     []stack.VC{{ID: 0}},
	}

	sender, err := stack.NewSender(config)
	if err != nil {
		panic(err)
	}

	telemetry, err := spp.NewTMPacket(100, []byte("battery ok"))
	if err != nil {
		panic(err)
	}
	if err := sender.SendPacket(0, telemetry); err != nil {
		panic(err)
	}
	if err := sender.Flush(); err != nil {
		panic(err)
	}

	// The ground station is built from the same configuration, so the two
	// ends cannot disagree about the frame layout.
	receiver, err := stack.NewReceiver(config)
	if err != nil {
		panic(err)
	}
	for cadu, err := range sender.CADUs() {
		if err != nil {
			panic(err)
		}
		if err := receiver.Accept(cadu); err != nil {
			panic(err)
		}
	}

	received, ok, err := receiver.NextPacket(0)
	if err != nil || !ok {
		panic(fmt.Sprintf("no packet: %v", err))
	}
	fmt.Printf("APID %d: %s\n", received.PrimaryHeader.APID, received.UserData)

	// Output: APID 100: battery ok
}
