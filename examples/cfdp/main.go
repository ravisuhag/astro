// Example: Downlinking a file with CFDP
//
// The other examples move packets. This one moves a file, which is a different
// problem: a file has a name, a length, a checksum, and it is not delivered
// until every byte of it has arrived.
//
//	Spacecraft Side:
//	  1. A science image sits in the on-board filestore
//	  2. A CFDP Sender turns it into Metadata, File Data and EOF PDUs
//	  3. Each PDU rides inside a Space Packet
//
//	Link:
//	  One File Data PDU is dropped, the way a real pass loses frames
//
//	Ground Side:
//	  1. A CFDP Receiver reassembles the file
//	  2. It notices the gap and asks for it by offset
//	  3. The sender fills it, the receiver checksums, and both sides finish
//
// Class 2 (acknowledged) mode is what makes the recovery possible. Class 1
// would have delivered a file with a hole in it and told nobody.
package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/cfdp"
	"github.com/ravisuhag/astro/pkg/spp"
)

const (
	spacecraftEntity = 42  // CFDP entity ID of the spacecraft
	groundEntity     = 7   // CFDP entity ID of the ground station
	transactionSeq   = 101 // this transfer, unique per source entity

	apidFileTransfer = 300 // the APID CFDP PDUs travel on
	segmentSize      = 512 // file data octets per PDU

	sourceFile      = "science/img_0042.raw"
	destinationFile = "downlink/img_0042.raw"

	dropAtIndex = 3 // which outbound PDU the link loses
)

func main() {
	// The on-board filestore. MemoryFilestore is what you want in a test;
	// NewOSFilestore(dir) writes to a real directory.
	onboard := cfdp.NewMemoryFilestore()
	image := scienceImage(3000)
	if err := onboard.WriteAt(sourceFile, 0, image); err != nil {
		log.Fatalf("staging the image: %v", err)
	}

	ground := cfdp.NewMemoryFilestore()

	sender, err := cfdp.NewSender(onboard, cfdp.SenderConfig{
		Source:              cfdp.NewEntityID(spacecraftEntity),
		Destination:         cfdp.NewEntityID(groundEntity),
		TransactionSeq:      cfdp.NewEntityID(transactionSeq),
		Acknowledged:        true, // Class 2: gaps get filled
		SegmentSize:         segmentSize,
		SourceFileName:      sourceFile,
		DestinationFileName: destinationFile,
		CRCFlag:             true,
	})
	if err != nil {
		log.Fatalf("opening the transaction: %v", err)
	}

	// The receiver's config has to agree on the three IDs. PDUs from any other
	// transaction are ignored rather than misapplied.
	receiver := cfdp.NewReceiver(ground, cfdp.ReceiverConfig{
		Source:         cfdp.NewEntityID(spacecraftEntity),
		Destination:    cfdp.NewEntityID(groundEntity),
		TransactionSeq: cfdp.NewEntityID(transactionSeq),
		Acknowledged:   true,
		CRCFlag:        true,
	})

	link := &lossyLink{dropAt: dropAtIndex}

	fmt.Printf("--- Spacecraft: sending %s (%d octets) ---\n", sourceFile, len(image))
	fmt.Println()

	// First pass: push everything the sender has. The link eats one PDU.
	for {
		pdu, ok, err := sender.NextPDU()
		if err != nil {
			log.Fatalf("building a PDU: %v", err)
		}
		if !ok {
			break // nothing pending; the sender is waiting on the receiver
		}

		carried, dropped := link.send(pdu)
		if dropped {
			fmt.Printf("  %-24s LOST IN TRANSIT\n", describe(pdu))
			continue
		}
		fmt.Printf("  %-24s %3d octets in a Space Packet\n", describe(pdu), carried)

		if err := receiver.HandlePDU(pdu); err != nil {
			log.Fatalf("handling a PDU on the ground: %v", err)
		}
	}
	fmt.Println()

	fmt.Println("--- Ground: what is missing ---")
	fmt.Println()
	fmt.Printf("  Complete .... %t\n", receiver.Complete())
	for _, gap := range receiver.MissingSegments() {
		fmt.Printf("  Gap ......... octets %d to %d\n", gap.StartOffset, gap.EndOffset)
	}
	fmt.Println()

	// Second pass: pump both machines until neither has anything to say. This
	// is the whole recovery exchange, NAK out, data back, Finished, ACK.
	fmt.Println("--- Recovery ---")
	fmt.Println()
	pump(sender, receiver, link)
	fmt.Println()

	fmt.Println("--- Results ---")
	fmt.Println()

	delivered, err := ground.Read(destinationFile)
	if err != nil {
		log.Fatalf("reading the delivered file: %v", err)
	}

	fmt.Printf("  Delivered as ..... %s\n", receiver.FileName())
	fmt.Printf("  Size ............. %d octets\n", len(delivered))
	fmt.Printf("  Identical ........ %t\n", bytes.Equal(delivered, image))
	fmt.Printf("  Sender state ..... %s\n", sender.State())
	fmt.Printf("  Receiver state ... %s\n", receiver.State())
	fmt.Printf("  PDUs sent ........ %d down, %d up\n", link.down, link.up)
	fmt.Printf("  PDUs lost ........ %d\n", link.lost)

	if finished := sender.Finished(); finished != nil {
		fmt.Printf("  Condition ........ %s\n", finished.ConditionCode)
	}
}

// pump runs both transaction machines to a standstill. Nothing here owns a
// clock: the machines produce PDUs when they have something to say, and the
// caller decides when to ask. A real ground system does this on its own
// scheduler, and drives the retransmission timers the same way.
func pump(sender *cfdp.Sender, receiver *cfdp.Receiver, link *lossyLink) {
	acknowledged := false

	for round := 0; round < 50; round++ {
		progressed := false

		// Ground to spacecraft: NAKs and the Finished PDU.
		for {
			pdu, ok, err := receiver.NextPDU()
			if err != nil {
				log.Fatalf("building a PDU on the ground: %v", err)
			}
			if !ok {
				break
			}
			progressed = true
			link.up++
			fmt.Printf("  up   %s\n", describe(pdu))
			if err := sender.HandlePDU(pdu); err != nil {
				log.Fatalf("handling a PDU on the spacecraft: %v", err)
			}
		}

		// Spacecraft to ground: the retransmitted segments.
		for {
			pdu, ok, err := sender.NextPDU()
			if err != nil {
				log.Fatalf("building a PDU: %v", err)
			}
			if !ok {
				break
			}
			progressed = true
			link.down++
			fmt.Printf("  down %s\n", describe(pdu))
			if err := receiver.HandlePDU(pdu); err != nil {
				log.Fatalf("handling a PDU on the ground: %v", err)
			}
		}

		// The ACK of the Finished PDU is asked for separately, because the
		// sender owes it only once the receiver has said it is done. It is
		// owed once, so ask once: AckFinished will keep handing one out.
		if !acknowledged {
			if ack, ok, err := sender.AckFinished(); err == nil && ok {
				acknowledged = true
				progressed = true
				link.down++
				fmt.Printf("  down %s\n", describe(ack))
				if err := receiver.HandlePDU(ack); err != nil {
					log.Fatalf("handling the ACK on the ground: %v", err)
				}
			}
		}

		if sender.Done() && receiver.Done() {
			return
		}
		if !progressed {
			return
		}
	}
	log.Fatal("the transaction did not settle")
}

// lossyLink carries PDUs inside Space Packets and loses one of them.
//
// Wrapping is the honest part: a CFDP PDU is ordinary payload, so it needs a
// packet to travel in, and from there it goes through frames and CADUs like
// anything else. See the downlink guide for that half.
type lossyLink struct {
	dropAt         int
	index          int
	down, up, lost int
}

// send wraps a PDU in a Space Packet and reports how many octets went on the
// wire, or that the link ate it.
func (l *lossyLink) send(pdu *cfdp.PDU) (octets int, dropped bool) {
	raw, err := pdu.Encode()
	if err != nil {
		log.Fatalf("encoding a PDU: %v", err)
	}

	packet, err := spp.NewTMPacket(apidFileTransfer, raw,
		spp.WithSequenceCount(uint16(l.index)))
	if err != nil {
		log.Fatalf("wrapping a PDU: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		log.Fatalf("encoding the packet: %v", err)
	}

	drop := l.index == l.dropAt
	l.index++
	if drop {
		l.lost++
		return 0, true
	}
	l.down++
	return len(encoded), false
}

// describe names a PDU the way the standard does. A File Data PDU carries no
// directive code, so its offset is the useful thing to print.
func describe(pdu *cfdp.PDU) string {
	if pdu.Header.IsFileData {
		data, err := cfdp.DecodeFileDataPDU(pdu.Data,
			pdu.Header.SegmentMetadataFlag, pdu.Header.LargeFile)
		if err != nil {
			return "File Data (unreadable)"
		}
		return fmt.Sprintf("File Data @ %d", data.Offset)
	}

	code, err := cfdp.DirectiveCodeOf(pdu.Data)
	if err != nil {
		return "Directive (unreadable)"
	}
	return code.String()
}

// scienceImage is stand-in payload. Any bytes would do; a repeating pattern
// makes a misplaced segment obvious when you print the file.
func scienceImage(size int) []byte {
	image := make([]byte, size)
	for i := range image {
		image[i] = byte(i % 251) // a prime, so the pattern does not align to
	} // the segment size
	return image
}
