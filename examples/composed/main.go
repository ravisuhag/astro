// Example: Both ends of a downlink from one configuration
//
// examples/downlink wires the layers by hand, which is what you want when you
// are learning where the boundaries are. This is the short version: pkg/stack
// takes one stack.Downlink value and builds the spacecraft side and the ground
// side from it, so the two cannot disagree about frame length, the frame error
// control field, randomization, or which virtual channels exist.
//
//	Spacecraft side:  SendPacket -> buffered -> frame fills -> CADU
//	Ground side:      Accept -> de-randomize, decode, route -> Packets
//
// Two virtual channels, as in examples/downlink:
//   - VC0 (priority 3): housekeeping telemetry (APID 100)
//   - VC1 (priority 1): science payload data (APID 200)
package main

import (
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/stack"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// One configuration. Both ends read it, so a setting cannot be applied to
	// only one of them.
	cfg := stack.Downlink{
		SpacecraftID: 42,
		FrameLength:  256,
		FECF:         true,
		Channels: []stack.VC{
			{ID: 0, Priority: 3}, // housekeeping
			{ID: 1, Priority: 1}, // science
		},
	}

	sender, err := stack.NewSender(cfg)
	if err != nil {
		return fmt.Errorf("building the spacecraft side: %w", err)
	}

	// The ground side comes from the same value, not a second copy of it.
	receiver, err := stack.NewReceiver(cfg)
	if err != nil {
		return fmt.Errorf("building the ground side: %w", err)
	}

	// Housekeeping on VC0, science on VC1.
	sent := map[uint8][]string{
		0: {"battery ok", "thermal nominal", "attitude locked"},
		1: {"spectrum frame 1", "spectrum frame 2"},
	}
	apid := map[uint8]uint16{0: 100, 1: 200}

	for _, vcid := range []uint8{0, 1} {
		for _, payload := range sent[vcid] {
			packet, err := spp.NewTMPacket(apid[vcid], []byte(payload))
			if err != nil {
				return fmt.Errorf("building a packet for VC%d: %w", vcid, err)
			}
			if err := sender.SendPacket(vcid, packet); err != nil {
				return fmt.Errorf("sending on VC%d: %w", vcid, err)
			}
		}
	}

	// Nothing becomes a CADU until a frame fills or Flush is called. Without
	// this the last packets sit in a buffer waiting for traffic that is not
	// coming.
	if err := sender.Flush(); err != nil {
		return fmt.Errorf("flushing: %w", err)
	}

	// The link. Accept treats a CADU that will not decode as an error, because
	// only the caller can tell a corrupt frame from a misconfigured channel.
	cadus := 0
	for cadu, err := range sender.CADUs() {
		if err != nil {
			return fmt.Errorf("reading a CADU: %w", err)
		}
		if err := receiver.Accept(cadu); err != nil {
			return fmt.Errorf("accepting a CADU: %w", err)
		}
		cadus++
	}
	fmt.Printf("%d CADUs of %d octets crossed the link\n\n", cadus, cfg.FrameLength+4)

	// Whole packets come back out, per virtual channel.
	for _, vcid := range []uint8{0, 1} {
		fmt.Printf("VC%d (APID %d)\n", vcid, apid[vcid])
		got := 0
		for packet, err := range receiver.Packets(vcid) {
			if err != nil {
				return fmt.Errorf("reading a packet from VC%d: %w", vcid, err)
			}
			decoded, err := spp.Decode(packet)
			if err != nil {
				return fmt.Errorf("decoding a packet from VC%d: %w", vcid, err)
			}
			fmt.Printf("  %q\n", decoded.UserData)
			got++
		}
		if got != len(sent[vcid]) {
			return fmt.Errorf("VC%d: sent %d packets, recovered %d", vcid, len(sent[vcid]), got)
		}
	}

	fmt.Println("\nEvery packet came back byte for byte, from one configuration.")
	return nil
}
