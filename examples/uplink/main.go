// Example: Ground Station-to-Spacecraft Telecommand Simulation
//
// This example demonstrates a complete CCSDS telecommand (TC) uplink chain:
//
//   Ground Station Side:
//     1. Create telecommand Space Packets (SPP)
//     2. Frame packets into TC Transfer Frames (TCDL) via MAP Packet Service
//     3. Manage reliable delivery with FOP-1 sliding window (COP-1)
//     4. Wrap frames as CLTUs with BCH(63,56) error coding (TCSC)
//     5. Transmit CLTUs over a simulated RF uplink
//
//   Spacecraft Side:
//     1. Receive CLTUs from the RF uplink
//     2. Unwrap CLTUs: BCH decode with single-bit error correction
//     3. Decode TC Transfer Frames and verify CRC
//     4. Validate frame sequence with FARM-1 (COP-1)
//     5. Extract telecommand Space Packets from accepted frames
//     6. Send CLCW status back to ground via the TM return link
//
// The ground station sends commands on two virtual channels:
//   - VC0: Critical operations (mode change, safe mode) — Type-A reliable delivery
//   - VC1: Routine housekeeping requests — Type-A reliable delivery
//
// Run with: go run ./examples/uplink/
package main

import (
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/ravisuhag/astro/pkg/tcsc"
)

const (
	spacecraftID = 42    // 10-bit Spacecraft Identifier
	apidCritical = 100   // APID for critical operations commands
	apidRoutine  = 200   // APID for routine housekeeping requests
	vcidCritical = 0     // Virtual Channel for critical commands
	vcidRoutine  = 1     // Virtual Channel for routine commands
	mapID        = 0     // MAP ID (single MAP per VC in this example)
	copWindow    = 10    // COP-1 sliding window width
)

// command represents a spacecraft telecommand.
type command struct {
	Name    string
	Opcode  uint8
	Payload []byte
}

// simulatedRFUplink represents the ground-to-spacecraft communication channel.
type simulatedRFUplink struct {
	cltus [][]byte
}

func (link *simulatedRFUplink) transmit(cltu []byte) {
	link.cltus = append(link.cltus, cltu)
}

func (link *simulatedRFUplink) receive() ([]byte, bool) {
	if len(link.cltus) == 0 {
		return nil, false
	}
	cltu := link.cltus[0]
	link.cltus = link.cltus[1:]
	return cltu, true
}

// simulatedReturnLink carries CLCW status from spacecraft back to ground.
type simulatedReturnLink struct {
	clcws []*cop.CLCW
}

func (link *simulatedReturnLink) send(clcw *cop.CLCW) {
	link.clcws = append(link.clcws, clcw)
}

func (link *simulatedReturnLink) receive() (*cop.CLCW, bool) {
	if len(link.clcws) == 0 {
		return nil, false
	}
	clcw := link.clcws[0]
	link.clcws = link.clcws[1:]
	return clcw, true
}

func main() {
	fmt.Println("=== CCSDS Ground Station-to-Spacecraft Telecommand Simulation ===")
	fmt.Println()

	// --- Simulated Links ---
	uplink := &simulatedRFUplink{}
	returnLink := &simulatedReturnLink{}

	// =====================================================================
	// GROUND STATION SIDE
	// =====================================================================
	fmt.Println("--- Ground Station Side ---")
	fmt.Println()

	// Set up TC Data Link layer: virtual channels and MAP services.
	gsCounterCrit := tcdl.NewFrameCounter()
	gsCounterRout := tcdl.NewFrameCounter()

	gsVcCrit := tcdl.NewVirtualChannel(vcidCritical, 32)
	gsVcRout := tcdl.NewVirtualChannel(vcidRoutine, 32)

	// MAP Packet Service segments TC packets into TC frames.
	// bypass=false means Type-A (sequence-controlled, reliable delivery).
	gsMapCrit := tcdl.NewMAPPacketService(spacecraftID, vcidCritical, mapID, false, gsVcCrit, gsCounterCrit)
	gsMapCrit.SetPacketSizer(spp.PacketSizer)

	gsMapRout := tcdl.NewMAPPacketService(spacecraftID, vcidRoutine, mapID, false, gsVcRout, gsCounterRout)
	gsMapRout.SetPacketSizer(spp.PacketSizer)

	// FOP-1 manages the COP-1 sliding window for each VC.
	fopCrit := cop.NewFOP(spacecraftID, vcidCritical, copWindow)
	fopCrit.Initialize(0)

	fopRout := cop.NewFOP(spacecraftID, vcidRoutine, copWindow)
	fopRout.Initialize(0)

	// Define telecommands to send.
	criticalCmds := []command{
		{Name: "SET_MODE_SCIENCE", Opcode: 0x01, Payload: []byte{0x02}},
		{Name: "ENABLE_PAYLOAD", Opcode: 0x02, Payload: []byte{0x01, 0x00, 0xFF}},
		{Name: "UPDATE_ORBIT_PARAMS", Opcode: 0x03, Payload: []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60}},
	}

	routineCmds := []command{
		{Name: "REQUEST_HK_REPORT", Opcode: 0x10, Payload: []byte{0x01}},
		{Name: "REQUEST_THERMAL_DATA", Opcode: 0x11, Payload: []byte{0x02, 0x03}},
	}

	// Generate and frame critical commands.
	fmt.Printf("Generating %d critical commands (APID %d, VC%d)...\n", len(criticalCmds), apidCritical, vcidCritical)
	for i, cmd := range criticalCmds {
		// Build command payload: opcode + parameters.
		payload := append([]byte{cmd.Opcode}, cmd.Payload...)

		pkt, err := spp.NewTCPacket(apidCritical, payload,
			spp.WithSequenceCount(uint16(i)),
			spp.WithErrorControl(),
		)
		if err != nil {
			log.Fatalf("Failed to create critical command packet %d: %v", i, err)
		}

		encoded, err := pkt.Encode()
		if err != nil {
			log.Fatalf("Failed to encode critical command packet %d: %v", i, err)
		}

		fmt.Printf("  [%s] %s (opcode=0x%02X, %d bytes)\n", cmd.Name, pkt.Humanize(), cmd.Opcode, len(encoded))

		if err := gsMapCrit.Send(encoded); err != nil {
			log.Fatalf("Failed to send critical command %d: %v", i, err)
		}
	}

	// Generate and frame routine commands.
	fmt.Printf("\nGenerating %d routine commands (APID %d, VC%d)...\n", len(routineCmds), apidRoutine, vcidRoutine)
	for i, cmd := range routineCmds {
		payload := append([]byte{cmd.Opcode}, cmd.Payload...)

		pkt, err := spp.NewTCPacket(apidRoutine, payload,
			spp.WithSequenceCount(uint16(i)),
			spp.WithErrorControl(),
		)
		if err != nil {
			log.Fatalf("Failed to create routine command packet %d: %v", i, err)
		}

		encoded, err := pkt.Encode()
		if err != nil {
			log.Fatalf("Failed to encode routine command packet %d: %v", i, err)
		}

		fmt.Printf("  [%s] %s (opcode=0x%02X, %d bytes)\n", cmd.Name, pkt.Humanize(), cmd.Opcode, len(encoded))

		if err := gsMapRout.Send(encoded); err != nil {
			log.Fatalf("Failed to send routine command %d: %v", i, err)
		}
	}

	// Extract TC frames from virtual channels, register with FOP-1,
	// wrap as CLTUs, and transmit.
	fmt.Println()
	cltuCount := 0

	// Process critical VC frames.
	for gsVcCrit.Len() > 0 {
		frame, err := gsVcCrit.Next()
		if err != nil {
			log.Fatalf("Failed to get critical frame: %v", err)
		}

		encoded, err := frame.Encode()
		if err != nil {
			log.Fatalf("Failed to encode critical frame: %v", err)
		}

		// Register with FOP-1 for reliable delivery tracking.
		if err := fopCrit.TransmitFrame(encoded); err != nil {
			log.Fatalf("FOP-1 critical: %v", err)
		}

		// Wrap as CLTU: BCH(63,56) encode + start/tail sequences.
		cltu, err := tcsc.WrapCLTU(encoded, nil, nil, true)
		if err != nil {
			log.Fatalf("Failed to wrap CLTU: %v", err)
		}

		fmt.Printf("  CLTU %d: VC%d frame (%d bytes) → CLTU (%d bytes), N(S)=%d\n",
			cltuCount, vcidCritical, len(encoded), len(cltu), frame.Header.FrameSequenceNum)
		uplink.transmit(cltu)
		cltuCount++
	}

	// Process routine VC frames.
	for gsVcRout.Len() > 0 {
		frame, err := gsVcRout.Next()
		if err != nil {
			log.Fatalf("Failed to get routine frame: %v", err)
		}

		encoded, err := frame.Encode()
		if err != nil {
			log.Fatalf("Failed to encode routine frame: %v", err)
		}

		if err := fopRout.TransmitFrame(encoded); err != nil {
			log.Fatalf("FOP-1 routine: %v", err)
		}

		cltu, err := tcsc.WrapCLTU(encoded, nil, nil, true)
		if err != nil {
			log.Fatalf("Failed to wrap CLTU: %v", err)
		}

		fmt.Printf("  CLTU %d: VC%d frame (%d bytes) → CLTU (%d bytes), N(S)=%d\n",
			cltuCount, vcidRoutine, len(encoded), len(cltu), frame.Header.FrameSequenceNum)
		uplink.transmit(cltu)
		cltuCount++
	}

	fmt.Printf("\nTransmitted %d CLTUs over RF uplink\n", cltuCount)
	fmt.Printf("FOP-1 state: VC%d has %d pending, VC%d has %d pending\n",
		vcidCritical, fopCrit.PendingCount(), vcidRoutine, fopRout.PendingCount())

	// =====================================================================
	// SPACECRAFT SIDE
	// =====================================================================
	fmt.Println()
	fmt.Println("--- Spacecraft Side ---")
	fmt.Println()

	// Set up FARM-1 for each virtual channel.
	farmCrit := cop.NewFARM(vcidCritical, copWindow)
	farmRout := cop.NewFARM(vcidRoutine, copWindow)

	// Set up TC master channel and virtual channels for reception.
	scMaster := tcdl.NewMasterChannel(spacecraftID)
	scVcCrit := tcdl.NewVirtualChannel(vcidCritical, 32)
	scVcRout := tcdl.NewVirtualChannel(vcidRoutine, 32)
	scMaster.AddVirtualChannel(scVcCrit, 2)
	scMaster.AddVirtualChannel(scVcRout, 1)

	// MAP services for packet extraction.
	scCounterCrit := tcdl.NewFrameCounter()
	scCounterRout := tcdl.NewFrameCounter()
	scMapCrit := tcdl.NewMAPPacketService(spacecraftID, vcidCritical, mapID, false, scVcCrit, scCounterCrit)
	scMapCrit.SetPacketSizer(spp.PacketSizer)
	scMapRout := tcdl.NewMAPPacketService(spacecraftID, vcidRoutine, mapID, false, scVcRout, scCounterRout)
	scMapRout.SetPacketSizer(spp.PacketSizer)

	// Process received CLTUs.
	acceptedFrames := 0
	rejectedFrames := 0
	bchCorrections := 0

	fmt.Println("Processing received CLTUs...")
	for {
		cltu, ok := uplink.receive()
		if !ok {
			break
		}

		// Step 1: Unwrap CLTU — BCH decode with error correction.
		frameData, corrections, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
		if err != nil {
			fmt.Printf("  [CLTU FAIL] BCH decode error: %v\n", err)
			rejectedFrames++
			continue
		}
		if corrections > 0 {
			bchCorrections += corrections
			fmt.Printf("  [BCH] Corrected %d bit error(s)\n", corrections)
		}

		// Step 2: Decode TC Transfer Frame and verify CRC.
		frame, err := tcdl.DecodeTCTransferFrame(frameData)
		if err != nil {
			fmt.Printf("  [CRC FAIL] Frame decode error: %v\n", err)
			rejectedFrames++
			continue
		}

		// Step 3: FARM-1 validates frame sequence number.
		var farm *cop.FARM
		if frame.Header.VirtualChannelID == vcidCritical {
			farm = farmCrit
		} else {
			farm = farmRout
		}

		accepted, err := farm.ProcessFrame(
			frame.Header.BypassFlag,
			frame.Header.ControlCommandFlag,
			frame.Header.FrameSequenceNum,
		)

		if !accepted {
			fmt.Printf("  [FARM REJECT] VC%d N(S)=%d: %v\n",
				frame.Header.VirtualChannelID, frame.Header.FrameSequenceNum, err)
			rejectedFrames++
			continue
		}

		fmt.Printf("  [ACCEPTED] VC%d N(S)=%d — FARM V(R) now %d\n",
			frame.Header.VirtualChannelID, frame.Header.FrameSequenceNum, farm.VR())
		acceptedFrames++

		// Step 4: Parse segment header from data field.
		// DecodeTCTransferFrame leaves the segment header in DataField since
		// it cannot know whether a segment header is present. We parse it
		// here because we know our MAP services always include one.
		if len(frame.DataField) > 0 {
			sh := &tcdl.SegmentHeader{}
			if err := sh.Decode(frame.DataField[:1]); err == nil {
				frame.SegmentHeader = sh
				frame.DataField = frame.DataField[1:]
			}
		}

		// Step 5: Route accepted frame to master channel for packet extraction.
		if err := scMaster.AddFrame(frame); err != nil {
			fmt.Printf("  [ROUTE ERROR] %v\n", err)
			continue
		}
	}

	// Generate CLCW status for each VC and send back on return link.
	clcwCrit := farmCrit.GenerateCLCW()
	clcwRout := farmRout.GenerateCLCW()
	returnLink.send(clcwCrit)
	returnLink.send(clcwRout)

	fmt.Printf("\nFrame reception summary:\n")
	fmt.Printf("  Accepted: %d, Rejected: %d, BCH corrections: %d bits\n",
		acceptedFrames, rejectedFrames, bchCorrections)
	fmt.Printf("  FARM-1 VC%d: state=%s, V(R)=%d\n",
		vcidCritical, farmStateString(farmCrit.State()), farmCrit.VR())
	fmt.Printf("  FARM-1 VC%d: state=%s, V(R)=%d\n",
		vcidRoutine, farmStateString(farmRout.State()), farmRout.VR())

	// Extract telecommand packets from accepted frames.
	fmt.Println("\nExtracting critical commands from VC0:")
	critRecovered := extractPackets(scMapCrit, "Critical")

	fmt.Println("\nExtracting routine commands from VC1:")
	routRecovered := extractPackets(scMapRout, "Routine")

	// =====================================================================
	// GROUND STATION: Process CLCW Return Link
	// =====================================================================
	fmt.Println()
	fmt.Println("--- Return Link (CLCW Processing) ---")
	fmt.Println()

	// Ground station receives CLCW status from spacecraft.
	for {
		clcw, ok := returnLink.receive()
		if !ok {
			break
		}

		encoded, _ := clcw.Encode()
		fmt.Printf("  CLCW received: %s (%d bytes)\n", clcw.Humanize(), len(encoded))

		// FOP-1 processes CLCW to acknowledge delivered frames.
		switch clcw.VirtualChannelID {
		case vcidCritical:
			if err := fopCrit.ProcessCLCW(clcw); err != nil {
				fmt.Printf("  [FOP-1 ERROR] VC%d: %v\n", vcidCritical, err)
			}
		case vcidRoutine:
			if err := fopRout.ProcessCLCW(clcw); err != nil {
				fmt.Printf("  [FOP-1 ERROR] VC%d: %v\n", vcidRoutine, err)
			}
		}
	}

	fmt.Printf("\nFOP-1 after CLCW processing:\n")
	fmt.Printf("  VC%d: %d frames still pending\n", vcidCritical, fopCrit.PendingCount())
	fmt.Printf("  VC%d: %d frames still pending\n", vcidRoutine, fopRout.PendingCount())

	fmt.Println()
	fmt.Println("=== Simulation Summary ===")
	fmt.Printf("  Commands sent:      %d critical + %d routine = %d total\n",
		len(criticalCmds), len(routineCmds), len(criticalCmds)+len(routineCmds))
	fmt.Printf("  CLTUs transmitted:  %d\n", cltuCount)
	fmt.Printf("  Frames accepted:    %d\n", acceptedFrames)
	fmt.Printf("  Commands recovered: %d critical + %d routine = %d total\n",
		critRecovered, routRecovered, critRecovered+routRecovered)
	fmt.Printf("  All frames acknowledged by CLCW: %v\n",
		fopCrit.PendingCount() == 0 && fopRout.PendingCount() == 0)
	fmt.Println()
	fmt.Println("=== Simulation Complete ===")
}

// extractPackets reads all available packets from a MAP service and prints them.
func extractPackets(svc *tcdl.MAPPacketService, label string) int {
	count := 0
	for {
		data, err := svc.Receive()
		if err != nil {
			break
		}

		pkt, err := spp.Decode(data, spp.WithDecodeErrorControl())
		if err != nil {
			fmt.Printf("  [DECODE ERROR] %v\n", err)
			continue
		}

		opcode := uint8(0)
		if len(pkt.UserData) > 0 {
			opcode = pkt.UserData[0]
		}
		fmt.Printf("  %s command: APID=%d, Seq=%d, Opcode=0x%02X, Payload=%d bytes\n",
			label,
			pkt.PrimaryHeader.APID,
			pkt.PrimaryHeader.SequenceCount,
			opcode,
			len(pkt.UserData),
		)
		count++
	}
	fmt.Printf("  Total: %d %s commands recovered\n", count, label)
	return count
}

func farmStateString(state cop.FARMState) string {
	switch state {
	case cop.FARMOpen:
		return "Open"
	case cop.FARMWait:
		return "Wait"
	case cop.FARMLockout:
		return "Lockout"
	default:
		return "Unknown"
	}
}
