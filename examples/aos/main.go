// Example: A high-rate downlink with AOS
//
// The downlink guide uses TM frames, which carry packets and nothing else.
// AOS carries three different things, on three virtual channels, over one
// physical channel:
//
//	VC0  M_PDU  Space Packets, the same job TM does
//	VC1  B_PDU  a raw octet-aligned bitstream, with no packet structure
//	VC2  VCA    fixed-size opaque blocks, with no protocol header at all
//
// It also adds two fields TM does not have:
//
//	Insert Zone  a fixed slot in every frame, for a synchronous stream
//	FHEC         Reed-Solomon over the frame header, so a corrupted header
//	             can be detected rather than mis-routed
//
// That is what makes AOS the choice for a high-rate mission: an instrument
// producing a bitstream does not have to be dressed up as packets first.
package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/ravisuhag/astro/pkg/aos"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcf"
)

const (
	spacecraftID = 42 // AOS spacecraft IDs are 8 bits, not 10

	vcidPackets   = 0 // M_PDU
	vcidBitstream = 1 // B_PDU
	vcidBlocks    = 2 // VCA

	apidHK = 100
)

// The physical channel. Every frame on it is this long and has these fields,
// and nothing on the wire says so, so both ends need the same value.
var config = aos.ChannelConfig{
	FrameLength:   1115, // a common AOS choice: fits an RS(255,223) codeword set
	InsertZoneLen: 8,    // room for a CUC time stamp
	HasOCF:        false,
	HasFHEC:       true, // RS(10,6) over the primary header
	HasFECF:       true, // CRC-16 over the whole frame
}

func main() {
	fmt.Printf("--- The physical channel ---\n\n")
	fmt.Printf("  frame length ........ %d octets\n", config.FrameLength)
	fmt.Printf("  insert zone ......... %d\n", config.InsertZoneLen)
	fmt.Printf("  FHEC ................ %t\n", config.HasFHEC)
	fmt.Printf("  FECF ................ %t\n", config.HasFECF)
	fmt.Printf("  data field capacity . %d octets\n", config.DataFieldCapacity())
	fmt.Println()

	packets()
	bitstream()
	blocks()
	insertZone()
	headerErrorControl()
	gaps()
}

// packets is VC0: the M_PDU service, which is AOS doing what TM does. Packets
// are multiplexed into the packet zone and a First Header Pointer marks where
// the first one in each frame starts.
func packets() {
	fmt.Println("--- VC0, M_PDU: Space Packets ---")
	fmt.Println()

	// AOS has no master channel frame count. Each virtual channel counts for
	// itself, in 24 bits rather than TM's 8.
	counter := aos.NewFrameCounter()

	sendVC := aos.NewVirtualChannel(vcidPackets, 16)
	transmit := aos.NewMultiplexingService(spacecraftID, vcidPackets, sendVC, config, counter)
	transmit.SetPacketSizer(spp.PacketSizer)

	var sent [][]byte
	for i := range 3 {
		packet, err := spp.NewTMPacket(apidHK,
			bytes.Repeat([]byte{byte(0xA0 + i)}, 400),
			spp.WithSequenceCount(uint16(i)))
		if err != nil {
			log.Fatalf("building a packet: %v", err)
		}
		encoded, err := packet.Encode()
		if err != nil {
			log.Fatalf("encoding a packet: %v", err)
		}
		sent = append(sent, encoded)

		if err := transmit.Send(encoded); err != nil {
			log.Fatalf("sending a packet: %v", err)
		}
	}

	// Same rule as TM: a partly full frame goes nowhere until you flush, and
	// the leftover space is completed with a real idle packet.
	if err := transmit.Flush(); err != nil {
		log.Fatalf("flushing: %v", err)
	}

	recvVC := aos.NewVirtualChannel(vcidPackets, 16)
	receive := aos.NewMultiplexingService(spacecraftID, vcidPackets, recvVC, config, nil)
	receive.SetPacketSizer(spp.PacketSizer)

	frames := cross(sendVC, recvVC)
	fmt.Printf("  sent .......... %d packets of %d octets\n", len(sent), len(sent[0]))
	fmt.Printf("  frames ........ %d\n", frames)

	for i := range sent {
		got, err := receive.Receive()
		if err != nil {
			log.Fatalf("receiving packet %d: %v", i, err)
		}
		fmt.Printf("  recovered ..... packet %d, %d octets, identical %t\n",
			i, len(got), bytes.Equal(got, sent[i]))
	}
	fmt.Println()
}

// bitstream is VC1: the B_PDU service. An instrument that produces a stream of
// octets with no packet structure gets its own channel, and a Bitstream Data
// Pointer marks where real data stops in the last partial frame.
func bitstream() {
	fmt.Println("--- VC1, B_PDU: a raw bitstream ---")
	fmt.Println()

	counter := aos.NewFrameCounter()
	sendVC := aos.NewVirtualChannel(vcidBitstream, 16)
	transmit := aos.NewBitstreamService(spacecraftID, vcidBitstream, sendVC, config, counter)

	// 2500 octets of instrument output. Not packets, and no boundaries in it
	// that AOS knows or cares about.
	stream := make([]byte, 2500)
	for i := range stream {
		stream[i] = byte(i % 253)
	}

	if err := transmit.Send(stream); err != nil {
		log.Fatalf("sending the bitstream: %v", err)
	}
	if err := transmit.Flush(); err != nil {
		log.Fatalf("flushing: %v", err)
	}

	recvVC := aos.NewVirtualChannel(vcidBitstream, 16)
	receive := aos.NewBitstreamService(spacecraftID, vcidBitstream, recvVC, config, nil)

	frames := cross(sendVC, recvVC)
	fmt.Printf("  sent .......... %d octets\n", len(stream))
	fmt.Printf("  frames ........ %d\n", frames)

	var recovered []byte
	for {
		chunk, err := receive.Receive()
		if err != nil {
			break
		}
		recovered = append(recovered, chunk...)
	}
	fmt.Printf("  recovered ..... %d octets, identical %t\n",
		len(recovered), bytes.Equal(recovered, stream))
	fmt.Println()
}

// blocks is VC2: the VCA service. One fixed-size opaque block per frame, and
// the data field has no protocol header at all. This is what you use for
// something that is already framed, such as compressed image blocks.
func blocks() {
	fmt.Println("--- VC2, VCA: opaque fixed-size blocks ---")
	fmt.Println()

	// On a fixed-length physical channel a VCA SDU has to fill the data field
	// exactly. A short one is rejected rather than padded, because the
	// receiver has no in-band way to find where the SDU ended.
	blockSize := config.DataFieldCapacity()

	counter := aos.NewFrameCounter()
	sendVC := aos.NewVirtualChannel(vcidBlocks, 16)
	transmit := aos.NewVirtualChannelAccessService(
		spacecraftID, vcidBlocks, blockSize, sendVC, config, counter)

	var sent [][]byte
	for i := range 2 {
		block := bytes.Repeat([]byte{byte(0x10 + i)}, blockSize)
		sent = append(sent, block)
		if err := transmit.Send(block); err != nil {
			log.Fatalf("sending a block: %v", err)
		}
	}
	if err := transmit.Flush(); err != nil {
		log.Fatalf("flushing: %v", err)
	}

	recvVC := aos.NewVirtualChannel(vcidBlocks, 16)
	receive := aos.NewVirtualChannelAccessService(
		spacecraftID, vcidBlocks, blockSize, recvVC, config, nil)

	frames := cross(sendVC, recvVC)
	fmt.Printf("  sent .......... %d blocks of %d octets\n", len(sent), blockSize)
	fmt.Printf("  frames ........ %d\n", frames)

	for i := range sent {
		got, err := receive.Receive()
		if err != nil {
			log.Fatalf("receiving block %d: %v", i, err)
		}
		fmt.Printf("  recovered ..... block %d, %d octets, identical %t\n",
			i, len(got), bytes.Equal(got, sent[i]))
	}
	fmt.Println()
}

// insertZone shows the field TM has no equivalent for: a fixed slot in every
// frame, at a known offset, carrying a stream that is synchronous with the
// frames themselves. A time stamp is the usual passenger.
func insertZone() {
	fmt.Println("--- The insert zone ---")
	fmt.Println()

	stamp := time.Date(2026, 4, 17, 8, 30, 15, 250_000_000, time.UTC)
	code, err := tcf.NewCUC(stamp,
		tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(3))
	if err != nil {
		log.Fatalf("building the time code: %v", err)
	}
	tfield, err := code.EncodeTField()
	if err != nil {
		log.Fatalf("encoding the time code: %v", err)
	}

	// Eight octets of insert zone: a 7-octet CUC T-field and one spare.
	zone := make([]byte, config.InsertZoneLen)
	copy(zone, tfield)

	frame, err := aos.NewTransferFrame(spacecraftID, vcidPackets,
		make([]byte, config.DataFieldCapacity()),
		aos.WithInsertZone(zone),
		aos.WithFHEC(),
		aos.WithFECF(),
		aos.WithVCFrameCount(1))
	if err != nil {
		log.Fatalf("building the frame: %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		log.Fatalf("encoding the frame: %v", err)
	}

	decoded, err := aos.DecodeFrame(encoded, config)
	if err != nil {
		log.Fatalf("decoding the frame: %v", err)
	}

	back, err := tcf.DecodeCUCTField(decoded.InsertZone[:7], 4, 3, tcf.CCSDSEpoch)
	if err != nil {
		log.Fatalf("decoding the insert zone time: %v", err)
	}

	fmt.Printf("  frame ......... %d octets\n", len(encoded))
	fmt.Printf("  insert zone ... % x\n", decoded.InsertZone)
	fmt.Printf("  reads as ...... %s\n", back.Time().Format(time.RFC3339Nano))
	fmt.Println("  Every frame has one, at the same offset, whether or not the")
	fmt.Println("  virtual channel had anything to say.")
	fmt.Println()
}

// headerErrorControl shows the FHEC doing its job. TM has no header
// protection: a corrupted VCID sends a frame to the wrong channel and the
// frame CRC only tells you something is wrong, not that it was the header.
func headerErrorControl() {
	fmt.Println("--- Frame Header Error Control ---")
	fmt.Println()

	frame, err := aos.NewTransferFrame(spacecraftID, vcidBitstream,
		make([]byte, config.DataFieldCapacity()),
		aos.WithInsertZone(make([]byte, config.InsertZoneLen)),
		aos.WithFHEC(),
		aos.WithFECF(),
		aos.WithVCFrameCount(4095))
	if err != nil {
		log.Fatalf("building the frame: %v", err)
	}

	header, err := frame.Header.Encode()
	if err != nil {
		log.Fatalf("encoding the header: %v", err)
	}
	fhec, err := aos.ComputeFHEC(header)
	if err != nil {
		log.Fatalf("computing the FHEC: %v", err)
	}

	fmt.Printf("  header ........ % x\n", header)
	fmt.Printf("  FHEC .......... % x\n", fhec)
	fmt.Printf("  verifies ...... %t\n", aos.VerifyFHEC(header, fhec))

	// Corrupt the octet holding the virtual channel ID. Without the FHEC this
	// frame would be delivered to VC 0 and nobody would know.
	corrupted := bytes.Clone(header)
	corrupted[1] ^= 0x01
	fmt.Printf("  one bit flipped %t (VCID %d became %d)\n",
		aos.VerifyFHEC(corrupted, fhec),
		header[1]&0x3F, corrupted[1]&0x3F)
	fmt.Println()
}

// gaps shows the 24-bit per-VC frame count. TM counts to 255 and wraps, which
// at a high rate happens often enough that a gap and a wrap look alike.
func gaps() {
	fmt.Println("--- Detecting lost frames ---")
	fmt.Println()

	detector := aos.NewFrameGapDetector()

	for _, count := range []uint32{100, 101, 105, 106} {
		frame, err := aos.NewTransferFrame(spacecraftID, vcidPackets,
			make([]byte, config.DataFieldCapacity()),
			aos.WithInsertZone(make([]byte, config.InsertZoneLen)),
			aos.WithFHEC(),
			aos.WithFECF(),
			aos.WithVCFrameCount(count))
		if err != nil {
			log.Fatalf("building the frame: %v", err)
		}

		gap := detector.Track(frame)
		switch {
		case gap > 0:
			fmt.Printf("  frame %d ..... %d frame(s) missing\n", count, gap)
		default:
			fmt.Printf("  frame %d ..... in sequence\n", count)
		}
	}
	fmt.Println()
	fmt.Printf("  A 24-bit count wraps every %d frames, not every 256.\n",
		aos.MaxVCFrameCount+1)
}

// cross moves every frame from the send channel to the receive channel,
// through encode and decode, and reports how many made the trip. This is the
// whole physical layer as far as this example is concerned.
func cross(send, receive *aos.VirtualChannel) int {
	count := 0
	for {
		frame, err := send.Next()
		if err != nil {
			return count
		}

		encoded, err := frame.Encode()
		if err != nil {
			log.Fatalf("encoding a frame: %v", err)
		}
		if len(encoded) != config.FrameLength {
			log.Fatalf("frame is %d octets, the channel is %d",
				len(encoded), config.FrameLength)
		}

		decoded, err := aos.DecodeFrame(encoded, config)
		if err != nil {
			log.Fatalf("decoding a frame: %v", err)
		}
		if err := receive.Add(decoded); err != nil {
			log.Fatalf("queueing a frame: %v", err)
		}
		count++
	}
}
