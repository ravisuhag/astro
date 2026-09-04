// Example: Store and forward for deep space
//
// Everything else in these examples assumes the link is up. On a deep space
// mission it usually is not: a relay is behind the planet, a station is
// scheduled for someone else, or the round trip is forty minutes and a
// handshake finishes tomorrow.
//
// Delay-Tolerant Networking is the answer, and it is two layers:
//
//	Bundle Protocol version 7 (RFC 9171, pkg/bp)
//	  The network layer. A bundle is addressed and given a lifetime, then
//	  stored at each hop rather than held in an end-to-end session.
//
//	Licklider Transmission Protocol (CCSDS 734.1, pkg/ltp)
//	  The convergence layer for one hop. It pushes a whole block, then asks
//	  what was missed. No handshake, because a handshake costs a round trip.
//
// This example builds a bundle, sends it over LTP across a link that loses a
// segment, recovers it, and reads the bundle out the other side. Then it
// fragments a bundle for a contact window too short to fit it.
package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/ltp"
)

const (
	// IPN endpoint identifiers: node, then service. The CCSDS profile uses
	// this scheme with Compressed Bundle Header Encoding, so the endpoints
	// travel as numbers rather than strings.
	roverNode   = 1
	earthNode   = 2
	scienceSvc  = 1
	segmentSize = 512
)

func main() {
	bundle := buildBundle()
	encoded := sendOverLTP(bundle)
	readBundle(encoded)
	lostCheckpoint(bundle)
	fragment(bundle)
}

// buildBundle makes one bundle: the network-layer unit of DTN. It is addressed
// end to end, and no node along the way needs to know the route beyond its own
// next hop.
func buildBundle() *bp.Bundle {
	fmt.Println("--- A bundle ---")
	fmt.Println()

	primary := &bp.PrimaryBlock{
		// Version 7 dropped the custody-transfer and priority machinery
		// version 6 carried. What is left is plainer: ask for a delivery
		// report, and ask for the time each status was asserted.
		Flags: bp.FlagReportDelivery | bp.FlagStatusTimeRequested,

		Destination: bp.IPN(earthNode, scienceSvc),
		Source:      bp.IPN(roverNode, scienceSvc),
		ReportTo:    bp.IPN(roverNode, scienceSvc),

		// A checksum on the primary block. Version 7 makes one mandatory
		// unless a security block covers the block instead.
		CRCType: bp.CRC32C,

		// Milliseconds since 2000, plus a sequence number for bundles created
		// in the same millisecond. Source plus timestamp identifies a bundle,
		// and reassembly and status reports both key on that pair.
		Timestamp: bp.CreationTimestamp{Time: 828_000_000_000, Sequence: 1},

		// How long the bundle stays worth carrying, in milliseconds. A node
		// holding an expired bundle deletes it rather than forwarding it into
		// a window that has already closed.
		Lifetime: 86_400_000, // one day
	}

	payload := []byte("SPECTRUM 4096 CHANNELS SOL 1247 SITE MERIDIANI")

	// A hop count block is the cheap insurance against a routing loop: a
	// bundle ping-ponging between two nodes is deleted once it passes the
	// limit, instead of circulating until its lifetime runs out.
	hops, err := bp.NewHopCountBlock(2, 32, 0)
	if err != nil {
		log.Fatalf("building the hop count block: %v", err)
	}

	bundle, err := bp.NewBundle(primary, payload, hops)
	if err != nil {
		log.Fatalf("building the bundle: %v", err)
	}

	encoded, err := bundle.Encode()
	if err != nil {
		log.Fatalf("encoding the bundle: %v", err)
	}

	fmt.Printf("  from ............ %s\n", primary.Source)
	fmt.Printf("  to .............. %s\n", primary.Destination)
	fmt.Printf("  lifetime ........ %d ms\n", primary.Lifetime)
	fmt.Printf("  payload ......... %d octets\n", len(payload))
	fmt.Printf("  encoded bundle .. %d octets\n", len(encoded))
	fmt.Printf("  blocks .......... %d (hop count, then payload)\n", len(bundle.Blocks))
	fmt.Println()

	return bundle
}

// sendOverLTP moves the encoded bundle across one hop. LTP calls it a block,
// and knows nothing about what is inside it.
func sendOverLTP(bundle *bp.Bundle) []byte {
	fmt.Println("--- One hop, over LTP ---")
	fmt.Println()

	block, err := bundle.Encode()
	if err != nil {
		log.Fatalf("encoding the bundle: %v", err)
	}

	// The whole block is red, so every octet is retransmitted until confirmed.
	// A telemetry stream would make it green and never chase a loss; a mixed
	// block puts the header in the red part and the samples in the green.
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             ltp.SessionID{EngineID: roverNode, SessionNumber: 42},
		ClientServiceID:       scienceSvc,
		SegmentSize:           24, // small, so the loss is visible
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 0x5A5A, // must be random and never zero
	})
	if err != nil {
		log.Fatalf("opening the LTP session: %v", err)
	}

	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID:         ltp.SessionID{EngineID: roverNode, SessionNumber: 42},
		FirstReportSerial: 0xA5A5,
		MaxBlockSize:      1 << 20, // cap it at what the mission actually sends
	})
	if err != nil {
		log.Fatalf("opening the receiving session: %v", err)
	}

	// Drop a data segment from the middle of the block.
	const dropAt = 1
	sent, dropped := 0, 0

	for {
		segment, ok, err := sender.NextSegment()
		if err != nil {
			log.Fatalf("building a segment: %v", err)
		}
		if !ok {
			break
		}

		if sent == dropAt {
			fmt.Printf("  down  %-52s LOST IN TRANSIT\n", describe(segment))
			sent++
			dropped++
			continue
		}
		sent++

		fmt.Printf("  down  %s\n", describe(segment))
		if err := receiver.HandleSegment(cross(segment)); err != nil {
			log.Fatalf("handling a segment: %v", err)
		}
	}

	fmt.Println()
	fmt.Printf("  red part complete ... %t\n", receiver.RedPartComplete())
	for _, gap := range receiver.MissingRanges() {
		fmt.Printf("  missing ............. offset %d, %d octets\n",
			gap.Offset, gap.Length)
	}
	fmt.Println()

	// The recovery exchange. The receiver reports what it has, the sender
	// works out what that leaves and resends it.
	fmt.Println("  Recovery:")
	pump(sender, receiver)
	fmt.Println()

	fmt.Printf("  segments sent ....... %d, lost %d\n", sent, dropped)
	fmt.Printf("  red part complete ... %t\n", receiver.RedPartComplete())
	fmt.Printf("  block identical ..... %t\n", bytes.Equal(receiver.Block(), block))
	fmt.Println()

	return receiver.Block()
}

// pump runs both LTP machines to a standstill. Nothing here owns a clock:
// LTP's three timers, checkpoint, report and cancel retransmission, are the
// caller's to run, because on a light-minutes link only the mission knows what
// a sensible timeout is.
func pump(sender *ltp.Sender, receiver *ltp.Receiver) {
	for round := 0; round < 50; round++ {
		progressed := false

		for {
			segment, ok, err := receiver.NextSegment()
			if err != nil {
				log.Fatalf("building a return segment: %v", err)
			}
			if !ok {
				break
			}
			progressed = true
			fmt.Printf("    up    %s\n", describe(segment))
			if err := sender.HandleSegment(cross(segment)); err != nil {
				log.Fatalf("handling a return segment: %v", err)
			}
		}

		for {
			segment, ok, err := sender.NextSegment()
			if err != nil {
				log.Fatalf("building a segment: %v", err)
			}
			if !ok {
				break
			}
			progressed = true
			fmt.Printf("    down  %s\n", describe(segment))
			if err := receiver.HandleSegment(cross(segment)); err != nil {
				log.Fatalf("handling a segment: %v", err)
			}
		}

		if !progressed {
			return
		}
	}
	log.Fatal("the session did not settle")
}

// readBundle is the far end: the block LTP delivered is a bundle again.
func readBundle(block []byte) {
	fmt.Println("--- The far end ---")
	fmt.Println()

	bundle, err := bp.Decode(block)
	if err != nil {
		log.Fatalf("decoding the bundle: %v", err)
	}
	payload := bundle.Payload()

	fmt.Printf("  from ......... %s\n", bundle.Primary.Source)
	fmt.Printf("  to ........... %s\n", bundle.Primary.Destination)
	fmt.Printf("  created ...... %s\n", bundle.Primary.Timestamp.Humanize())
	for _, blk := range bundle.Blocks {
		if limit, count, err := blk.HopCount(); err == nil {
			fmt.Printf("  hops ......... %d of %d\n", count, limit)
		}
	}
	fmt.Printf("  payload ...... %q\n", payload)
	fmt.Println()
}

// lostCheckpoint is the failure LTP's timers exist for. A lost data segment
// recovers on its own, because the checkpoint that follows it prompts a report.
// A lost checkpoint prompts nothing, and both ends sit there.
func lostCheckpoint(bundle *bp.Bundle) {
	fmt.Println("--- When the checkpoint is the segment that is lost ---")
	fmt.Println()

	block, err := bundle.Encode()
	if err != nil {
		log.Fatalf("encoding the bundle: %v", err)
	}

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             ltp.SessionID{EngineID: roverNode, SessionNumber: 43},
		ClientServiceID:       scienceSvc,
		SegmentSize:           24,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 0x5A5B,
	})
	if err != nil {
		log.Fatalf("opening the LTP session: %v", err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID:         ltp.SessionID{EngineID: roverNode, SessionNumber: 43},
		FirstReportSerial: 0xA5A6,
		MaxBlockSize:      1 << 20,
	})
	if err != nil {
		log.Fatalf("opening the receiving session: %v", err)
	}

	// Send everything except the checkpoint.
	for {
		segment, ok, err := sender.NextSegment()
		if err != nil {
			log.Fatalf("building a segment: %v", err)
		}
		if !ok {
			break
		}
		if segment.Data != nil && segment.Header.Type.IsCheckpoint() {
			continue // lost in transit
		}
		if err := receiver.HandleSegment(cross(segment)); err != nil {
			log.Fatalf("handling a segment: %v", err)
		}
	}

	_, senderHasMore, _ := sender.NextSegment()
	_, receiverHasMore, _ := receiver.NextSegment()

	fmt.Printf("  checkpoint lost\n")
	fmt.Printf("  sender has something to send ..... %t\n", senderHasMore)
	fmt.Printf("  receiver has something to send ... %t\n", receiverHasMore)
	fmt.Printf("  red part complete ................ %t\n", receiver.RedPartComplete())
	fmt.Println()
	fmt.Println("  Neither end will move. The receiver is waiting for a")
	fmt.Println("  checkpoint that will not arrive, and the sender is waiting")
	fmt.Println("  for a report that nothing will prompt.")
	fmt.Println()

	// Only the caller's timer breaks the deadlock.
	sender.ResendCheckpoint()
	fmt.Println("  After the caller's timer fires and calls ResendCheckpoint:")
	pump(sender, receiver)

	fmt.Println()
	fmt.Printf("  red part complete ... %t\n", receiver.RedPartComplete())
	fmt.Printf("  block identical ..... %t\n", bytes.Equal(receiver.Block(), block))
	fmt.Println()
}

// fragment shows what happens when a contact window is too short for the whole
// bundle. Bundle Protocol splits it, each fragment travels on its own, and the
// destination reassembles.
func fragment(bundle *bp.Bundle) {
	fmt.Println("--- A window too short for the bundle ---")
	fmt.Println()

	fragments, err := bundle.Fragment(16)
	if err != nil {
		log.Fatalf("fragmenting: %v", err)
	}

	fmt.Printf("  fragments ....... %d\n", len(fragments))
	for _, part := range fragments {
		encoded, err := part.Encode()
		if err != nil {
			log.Fatalf("encoding a fragment: %v", err)
		}
		fmt.Printf("    offset %2d of %2d, %2d octets on the wire\n",
			part.Primary.FragmentOffset, part.Primary.TotalADULength, len(encoded))
	}

	// Fragments can arrive by different routes, in any order, hours apart.
	// Reassembly is the destination's job and needs them all.
	rejoined, err := bp.Reassemble(fragments)
	if err != nil {
		log.Fatalf("reassembling: %v", err)
	}

	original := bundle.Payload()
	recovered := rejoined.Payload()

	fmt.Printf("  reassembled ..... %d octets, identical %t\n",
		len(recovered), bytes.Equal(recovered, original))
	fmt.Println()
	fmt.Println("  A bundle with FlagMustNotFragment cannot be split, so it waits")
	fmt.Println("  for a window big enough or expires trying.")
}

// cross puts a segment through encode and decode, which is the whole physical
// layer as far as this example is concerned. A real hop would put it in a
// Space Packet and then a frame.
func cross(segment *ltp.Segment) *ltp.Segment {
	encoded, err := segment.Encode()
	if err != nil {
		log.Fatalf("encoding a segment: %v", err)
	}
	decoded, err := ltp.DecodeSegment(encoded)
	if err != nil {
		log.Fatalf("decoding a segment: %v", err)
	}
	return decoded
}

// describe names a segment the way RFC 5326 does, with the offset that makes a
// data segment identifiable.
func describe(segment *ltp.Segment) string {
	switch {
	case segment.Data != nil:
		return fmt.Sprintf("data @ %-4d %s", segment.Data.Offset, segment.Header.Type)
	case segment.Report != nil:
		return fmt.Sprintf("report     %d claim(s)", len(segment.Report.Claims))
	default:
		return segment.Header.Type.String()
	}
}
