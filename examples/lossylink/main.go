// Example: Lossy RF Link — Error Handling in CCSDS Telemetry
//
// This example demonstrates how CCSDS protocols handle a noisy
// communication channel. A spacecraft transmits 20 telemetry packets
// over a simulated RF link that randomly drops and corrupts CADUs.
//
// The ground station uses four CCSDS mechanisms to cope:
//   - Reed-Solomon FEC: corrupted frames are corrected before CRC check
//   - CRC-16 rejection: uncorrectable frames are detected and discarded
//   - Frame gap detection: dropped frames are identified via MC/VC counters
//   - FHP-based resync: after frame loss, the receiver re-synchronizes
//     to the next packet boundary using the First Header Pointer
//
// The transmission chain per CCSDS 131.0-B-4:
//
//	Frame (128 bytes) → RS encode (128→160 bytes) → Randomize → ASM → CADU
//	CADU → ASM strip → De-randomize → RS decode (160→128 bytes) → Frame
//
// Run with: go run ./examples/lossylink/
package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

const (
	spacecraftID = 42
	frameLength  = 128 // smaller frames to make spanning more likely
	apid         = 100
	vcid         = 0
	numPackets   = 20

	dropRate    = 0.15 // 15% of CADUs are lost
	corruptRate = 0.10 // 10% of CADUs arrive corrupted
)

// noisyLink simulates an RF channel with frame drops and bit errors.
type noisyLink struct {
	rng       *rand.Rand
	dropped   int
	corrupted int
	delivered int
}

func newNoisyLink(seed uint64) *noisyLink {
	return &noisyLink{rng: rand.New(rand.NewPCG(seed, 0))}
}

// transmit passes a CADU through the noisy channel.
// Returns the (possibly corrupted) CADU and whether it arrived at all.
func (l *noisyLink) transmit(cadu []byte) ([]byte, bool) {
	// Random drop — CADU never reaches ground station
	if l.rng.Float64() < dropRate {
		l.dropped++
		return nil, false
	}

	// Random corruption — flip bits in the frame body (not ASM)
	if l.rng.Float64() < corruptRate {
		corrupted := make([]byte, len(cadu))
		copy(corrupted, cadu)
		// Flip 1-3 random bytes after the ASM
		flips := l.rng.IntN(3) + 1
		for range flips {
			pos := l.rng.IntN(len(corrupted)-4) + 4 // skip ASM
			corrupted[pos] ^= byte(l.rng.IntN(255) + 1)
		}
		l.corrupted++
		return corrupted, true
	}

	l.delivered++
	return cadu, true
}

func main() {
	fmt.Println("=== Lossy RF Link — CCSDS Error Handling Demo ===")
	fmt.Println()

	config := tmdl.ChannelConfig{
		FrameLength: frameLength,
		HasFEC:      true,
	}

	link := newNoisyLink(12345) // fixed seed for reproducibility

	// Reed-Solomon codec: RS(255,223) can correct up to 16 symbol errors.
	// We use it to protect each frame against bit errors from the noisy channel.
	// Frames shorter than 223 bytes are zero-padded before encoding (virtual fill).
	rs := tmsc.NewRS255_223()

	// =================================================================
	// SPACECRAFT: generate packets and transmit CADUs
	// =================================================================

	scPhysical := tmdl.NewPhysicalChannel("SC-X-band", config)
	scMaster := tmdl.NewMasterChannel(spacecraftID, config)
	scPhysical.AddMasterChannel(scMaster, 1)

	vc := tmdl.NewVirtualChannel(vcid, 64)
	scMaster.AddVirtualChannel(vc, 1)

	counter := tmdl.NewFrameCounter()
	vcp := tmdl.NewVirtualChannelPacketService(spacecraftID, vcid, vc, config, counter)
	vcp.SetPacketSizer(spp.PacketSizer)

	// Create 20 telemetry packets of varying sizes.
	// Some will span multiple frames, making them vulnerable to frame loss.
	fmt.Printf("Spacecraft: generating %d telemetry packets...\n", numPackets)
	sentBytes := 0
	for i := range numPackets {
		// Vary payload size: 10-200 bytes (some will span 2+ frames)
		payloadSize := 10 + (i * 37 % 191)
		payload := make([]byte, payloadSize)
		for j := range payload {
			payload[j] = byte((i + j) & 0xFF)
		}

		pkt, err := spp.NewTMPacket(apid, payload,
			spp.WithSequenceCount(uint16(i)),
			spp.WithErrorControl(),
		)
		if err != nil {
			fmt.Printf("  ERROR creating packet %d: %v\n", i, err)
			continue
		}

		encoded, _ := pkt.Encode()
		sentBytes += len(encoded)

		if err := vcp.Send(encoded); err != nil {
			fmt.Printf("  ERROR sending packet %d: %v\n", i, err)
		}
	}
	_ = vcp.Flush()

	// Wrap all frames as CADUs with RS encoding and push through the noisy link.
	var receivedCADUs [][]byte
	totalFrames := 0
	for scPhysical.HasPendingFrames() {
		frame, _ := scPhysical.GetNextFrame()
		encoded, _ := frame.Encode()

		// RS encode: pad frame to 223 bytes (virtual fill), encode to 255 bytes.
		rsData := make([]byte, rs.DataLen())
		copy(rsData, encoded)
		rsEncoded, _ := rs.Encode(rsData)

		// Wrap as CADU: randomize + ASM.
		cadu := tmsc.WrapCADU(rsEncoded, nil, true)
		totalFrames++

		if arrived, ok := link.transmit(cadu); ok {
			receivedCADUs = append(receivedCADUs, arrived)
		}
	}

	fmt.Printf("  Sent: %d packets (%d bytes) in %d frames\n", numPackets, sentBytes, totalFrames)
	fmt.Printf("  Each frame: %d bytes → RS(%d bytes) → CADU(%d bytes)\n",
		frameLength, rs.DataLen()+rs.NRoots(), 4+rs.DataLen()+rs.NRoots())
	fmt.Printf("\nRF Link statistics:\n")
	fmt.Printf("  Delivered intact:  %d frames\n", link.delivered)
	fmt.Printf("  Dropped (lost):    %d frames\n", link.dropped)
	fmt.Printf("  Corrupted:         %d frames\n", link.corrupted)
	fmt.Println()

	// =================================================================
	// GROUND STATION: receive, detect errors, recover packets
	// =================================================================

	fmt.Println("Ground Station: processing received CADUs...")
	fmt.Println()

	gsPhysical := tmdl.NewPhysicalChannel("GS-receiver", config)
	gsMaster := tmdl.NewMasterChannel(spacecraftID, config)
	gsPhysical.AddMasterChannel(gsMaster, 1)

	gsVC := tmdl.NewVirtualChannel(vcid, 64)
	gsMaster.AddVirtualChannel(gsVC, 1)

	gsCounter := tmdl.NewFrameCounter()
	gsVCP := tmdl.NewVirtualChannelPacketService(spacecraftID, vcid, gsVC, config, gsCounter)
	gsVCP.SetPacketSizer(spp.PacketSizer)

	gapDetector := tmdl.NewFrameGapDetector()

	goodFrames := 0
	crcRejects := 0
	rsCorrections := 0
	rsFailed := 0
	mcGapsTotal := 0
	vcGapsTotal := 0

	for _, cadu := range receivedCADUs {
		// Step 1: Unwrap CADU — strip ASM, de-randomize.
		rsData, err := tmsc.UnwrapCADU(cadu, nil, true)
		if err != nil {
			crcRejects++
			continue
		}

		// Step 2: RS decode — correct symbol errors from the noisy channel.
		corrected, corr, err := rs.Decode(rsData)
		if err != nil {
			rsFailed++
			fmt.Printf("  [RS FAIL] Uncorrectable errors in frame\n")
			continue
		}
		if corr > 0 {
			rsCorrections += corr
			fmt.Printf("  [RS OK] Corrected %d symbol errors\n", corr)
		}

		// Step 3: Extract original frame (strip virtual fill) and decode.
		frameData := corrected[:frameLength]
		frame, err := tmdl.DecodeTMTransferFrame(frameData)
		if err != nil {
			crcRejects++
			continue
		}

		// Track frame gaps (detects dropped frames via counter discontinuity).
		mcGap, vcGap := gapDetector.Track(frame)
		if mcGap > 0 {
			mcGapsTotal += mcGap
			fmt.Printf("  [GAP] MC counter gap: %d frame(s) lost\n", mcGap)
		}
		if vcGap > 0 {
			vcGapsTotal += vcGap
		}

		// Route frame to virtual channel for packet extraction.
		if err := gsPhysical.AddFrame(frame); err != nil {
			fmt.Printf("  [ERROR] Frame routing: %v\n", err)
			continue
		}
		goodFrames++
	}

	fmt.Println()
	fmt.Println("Frame reception summary:")
	fmt.Printf("  Good frames accepted:  %d / %d transmitted\n", goodFrames, totalFrames)
	fmt.Printf("  RS corrections:        %d symbols across all frames\n", rsCorrections)
	fmt.Printf("  RS failures:           %d (uncorrectable, >16 errors)\n", rsFailed)
	fmt.Printf("  CRC rejects:           %d\n", crcRejects)
	fmt.Printf("  MC frame gaps:         %d (frames lost in transit)\n", mcGapsTotal)
	fmt.Println()

	// Extract packets — VCP uses FHP to resync after frame loss.
	fmt.Println("Packet recovery (FHP-based resync after gaps):")
	recovered := 0
	crcFailed := 0
	for {
		data, err := gsVCP.Receive()
		if err != nil {
			break
		}

		// Verify packet-level CRC to catch packets that span a lost frame.
		pkt, err := spp.Decode(data, spp.WithDecodeErrorControl())
		if err != nil {
			crcFailed++
			continue
		}

		fmt.Printf("  Recovered packet: APID=%d Seq=%d Size=%d bytes\n",
			pkt.PrimaryHeader.APID,
			pkt.PrimaryHeader.SequenceCount,
			len(pkt.UserData),
		)
		recovered++
	}

	fmt.Println()
	fmt.Println("=== Results ===")
	fmt.Printf("  Packets sent:       %d\n", numPackets)
	fmt.Printf("  Packets recovered:  %d (%.0f%%)\n", recovered, float64(recovered)/float64(numPackets)*100)
	fmt.Printf("  Packets lost:       %d (spanned a dropped/corrupted frame)\n", numPackets-recovered-crcFailed)
	fmt.Printf("  CRC failures:       %d (partial packet from lost frame)\n", crcFailed)
	fmt.Println()
	fmt.Println("This demonstrates why CCSDS uses four layers of protection:")
	fmt.Println("  1. Reed-Solomon FEC — corrects corrupted frames before they reach the pipeline")
	fmt.Println("  2. Frame CRC-16     — rejects frames with uncorrectable errors")
	fmt.Println("  3. Frame counters   — detects gaps so the receiver knows data was lost")
	fmt.Println("  4. First Header Ptr — re-syncs to the next intact packet after a gap")
}
