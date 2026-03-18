// Example: Spacecraft-to-Ground Station Telemetry Simulation
//
// This example demonstrates a complete CCSDS telemetry chain:
//
//   Spacecraft Side:
//     1. Generate telemetry data as Space Packets (SPP)
//     2. Frame packets into TM Transfer Frames (TMDL)
//     3. Wrap frames as CADUs (ASM + optional randomization)
//     4. Transmit over a simulated RF link
//
//   Ground Station Side:
//     1. Receive CADUs from the RF link
//     2. Unwrap to extract TM Transfer Frames
//     3. Demultiplex virtual channels
//     4. Extract original Space Packets from frames
//
// The spacecraft uses two virtual channels:
//   - VC0 (priority 3): Housekeeping telemetry (APID 100)
//   - VC1 (priority 1): Science payload data (APID 200)
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

const (
	spacecraftID = 42   // 10-bit Spacecraft Identifier
	frameLength  = 256  // Fixed TM frame length in octets
	apidHK       = 100  // APID for housekeeping telemetry
	apidScience  = 200  // APID for science data
	vcidHK       = 0    // Virtual Channel for housekeeping
	vcidScience  = 1    // Virtual Channel for science
)

// housekeepingTelemetry represents a simple HK packet payload.
type housekeepingTelemetry struct {
	Timestamp   uint32  // Mission elapsed time (seconds)
	BatteryV    float32 // Battery voltage
	TempC       float32 // Temperature in Celsius
	CPUPercent  uint8   // CPU usage percentage
	MemPercent  uint8   // Memory usage percentage
	ModeFlag    uint8   // Spacecraft mode (0=safe, 1=nominal, 2=science)
}

func (hk housekeepingTelemetry) encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, hk.Timestamp)
	binary.Write(buf, binary.BigEndian, hk.BatteryV)
	binary.Write(buf, binary.BigEndian, hk.TempC)
	buf.WriteByte(hk.CPUPercent)
	buf.WriteByte(hk.MemPercent)
	buf.WriteByte(hk.ModeFlag)
	return buf.Bytes()
}

func decodeHousekeeping(data []byte) (housekeepingTelemetry, error) {
	if len(data) < 15 {
		return housekeepingTelemetry{}, fmt.Errorf("housekeeping data too short: %d bytes", len(data))
	}
	r := bytes.NewReader(data)
	var hk housekeepingTelemetry
	binary.Read(r, binary.BigEndian, &hk.Timestamp)
	binary.Read(r, binary.BigEndian, &hk.BatteryV)
	binary.Read(r, binary.BigEndian, &hk.TempC)
	hk.CPUPercent, _ = r.ReadByte()
	hk.MemPercent, _ = r.ReadByte()
	hk.ModeFlag, _ = r.ReadByte()
	return hk, nil
}

// simulatedRFLink represents the space-to-ground communication channel.
// In a real system, this would be an RF modem or network socket.
type simulatedRFLink struct {
	cadus [][]byte
}

func (link *simulatedRFLink) transmit(cadu []byte) {
	link.cadus = append(link.cadus, cadu)
}

func (link *simulatedRFLink) receive() ([]byte, bool) {
	if len(link.cadus) == 0 {
		return nil, false
	}
	cadu := link.cadus[0]
	link.cadus = link.cadus[1:]
	return cadu, true
}

func main() {
	fmt.Println("=== CCSDS Spacecraft-to-Ground Station Simulation ===")
	fmt.Println()

	// --- Channel Configuration (shared between spacecraft and ground) ---
	config := tmdl.ChannelConfig{
		FrameLength: frameLength,
		HasOCF:      false,
		HasFEC:      true,
	}

	// --- Simulated RF Link ---
	link := &simulatedRFLink{}

	// =====================================================================
	// SPACECRAFT SIDE
	// =====================================================================
	fmt.Println("--- Spacecraft Side ---")
	fmt.Println()

	// Set up the TMDL physical channel with ASM framing.
	scPhysical := tmdl.NewPhysicalChannel("SC-downlink", config)

	// Create master channel for our spacecraft.
	scMaster := tmdl.NewMasterChannel(spacecraftID, config)
	scPhysical.AddMasterChannel(scMaster, 1)

	// Create virtual channels with buffering.
	vcHK := tmdl.NewVirtualChannel(vcidHK, 32)
	vcSci := tmdl.NewVirtualChannel(vcidScience, 32)
	scMaster.AddVirtualChannel(vcHK, 3)   // housekeeping = higher priority
	scMaster.AddVirtualChannel(vcSci, 1)  // science = lower priority

	// Create frame counter for sequence numbering.
	counter := tmdl.NewFrameCounter()

	// Create VCP (Virtual Channel Packet) services for each VC.
	// VCP multiplexes variable-length Space Packets into fixed-length frames.
	vpcHK := tmdl.NewVirtualChannelPacketService(spacecraftID, vcidHK, vcHK, config, counter)
	vpcHK.SetPacketSizer(tmdl.SpacePacketSizer)

	vpcSci := tmdl.NewVirtualChannelPacketService(spacecraftID, vcidScience, vcSci, config, counter)
	vpcSci.SetPacketSizer(tmdl.SpacePacketSizer)

	// Generate and send housekeeping telemetry packets.
	now := uint32(time.Now().Unix())
	hkSamples := []housekeepingTelemetry{
		{Timestamp: now, BatteryV: 28.1, TempC: 22.5, CPUPercent: 35, MemPercent: 60, ModeFlag: 1},
		{Timestamp: now + 1, BatteryV: 28.0, TempC: 22.7, CPUPercent: 42, MemPercent: 61, ModeFlag: 1},
		{Timestamp: now + 2, BatteryV: 27.9, TempC: 23.0, CPUPercent: 38, MemPercent: 60, ModeFlag: 2},
	}

	fmt.Printf("Generating %d housekeeping packets (APID %d, VC%d)...\n", len(hkSamples), apidHK, vcidHK)
	for i, sample := range hkSamples {
		// Create a CCSDS Space Packet with CRC.
		pkt, err := spp.NewTMPacket(apidHK, sample.encode(),
			spp.WithSequenceCount(uint16(i)),
			spp.WithErrorControl(),
		)
		if err != nil {
			log.Fatalf("Failed to create HK packet %d: %v", i, err)
		}

		encoded, err := pkt.Encode()
		if err != nil {
			log.Fatalf("Failed to encode HK packet %d: %v", i, err)
		}

		fmt.Printf("  Packet %d: %s\n", i, pkt.Humanize())

		// Send encoded packet into the VCP service, which packs it into TM frames.
		if err := vpcHK.Send(encoded); err != nil {
			log.Fatalf("Failed to send HK packet %d: %v", i, err)
		}
	}
	// Flush remaining buffered data into a final frame.
	if err := vpcHK.Flush(); err != nil {
		log.Fatalf("Failed to flush HK service: %v", err)
	}

	// Generate science data packets (simulated sensor readings).
	fmt.Printf("\nGenerating science packets (APID %d, VC%d)...\n", apidScience, vcidScience)
	for i := 0; i < 2; i++ {
		// Simulate a science data payload: 100 float32 samples.
		sciData := new(bytes.Buffer)
		for s := 0; s < 100; s++ {
			val := float32(math.Sin(float64(s)*0.1 + float64(i)))
			binary.Write(sciData, binary.BigEndian, val)
		}

		pkt, err := spp.NewTMPacket(apidScience, sciData.Bytes(),
			spp.WithSequenceCount(uint16(i)),
			spp.WithErrorControl(),
		)
		if err != nil {
			log.Fatalf("Failed to create science packet %d: %v", i, err)
		}

		encoded, err := pkt.Encode()
		if err != nil {
			log.Fatalf("Failed to encode science packet %d: %v", i, err)
		}

		fmt.Printf("  Packet %d: %d bytes payload (%d bytes on wire)\n", i, len(sciData.Bytes()), len(encoded))

		if err := vpcSci.Send(encoded); err != nil {
			log.Fatalf("Failed to send science packet %d: %v", i, err)
		}
	}
	if err := vpcSci.Flush(); err != nil {
		log.Fatalf("Failed to flush science service: %v", err)
	}

	// Extract frames from the physical channel and wrap as CADUs for transmission.
	caduCount := 0
	for scPhysical.HasPendingFrames() {
		frame, err := scPhysical.GetNextFrame()
		if err != nil {
			log.Fatalf("Failed to get frame: %v", err)
		}

		// Wrap the frame: prepend ASM and apply pseudo-randomization.
		cadu, err := scPhysical.Wrap(frame)
		if err != nil {
			log.Fatalf("Failed to wrap frame as CADU: %v", err)
		}

		link.transmit(cadu)
		caduCount++
	}
	fmt.Printf("\nTransmitted %d CADUs over RF link (%d bytes each)\n", caduCount, len(tmdl.DefaultASM())+frameLength)

	// =====================================================================
	// GROUND STATION SIDE
	// =====================================================================
	fmt.Println()
	fmt.Println("--- Ground Station Side ---")
	fmt.Println()

	// Set up ground station physical channel for reception.
	gsPhysical := tmdl.NewPhysicalChannel("GS-receiver", config)

	// Create master channel and virtual channels (mirror spacecraft config).
	gsMaster := tmdl.NewMasterChannel(spacecraftID, config)
	gsPhysical.AddMasterChannel(gsMaster, 1)

	gsVcHK := tmdl.NewVirtualChannel(vcidHK, 32)
	gsVcSci := tmdl.NewVirtualChannel(vcidScience, 32)
	gsMaster.AddVirtualChannel(gsVcHK, 3)
	gsMaster.AddVirtualChannel(gsVcSci, 1)

	// Create VCP services for packet extraction on the ground side.
	gsCounter := tmdl.NewFrameCounter()
	gsVpcHK := tmdl.NewVirtualChannelPacketService(spacecraftID, vcidHK, gsVcHK, config, gsCounter)
	gsVpcHK.SetPacketSizer(tmdl.SpacePacketSizer)

	gsVpcSci := tmdl.NewVirtualChannelPacketService(spacecraftID, vcidScience, gsVcSci, config, gsCounter)
	gsVpcSci.SetPacketSizer(tmdl.SpacePacketSizer)

	// Receive CADUs, unwrap, and route to virtual channels.
	receivedFrames := 0
	for {
		cadu, ok := link.receive()
		if !ok {
			break
		}

		// Unwrap CADU: strip ASM, de-randomize, verify CRC.
		frame, err := gsPhysical.Unwrap(cadu)
		if err != nil {
			log.Printf("Warning: failed to unwrap CADU: %v", err)
			continue
		}

		// Route frame to the correct master/virtual channel.
		if err := gsPhysical.AddFrame(frame); err != nil {
			log.Printf("Warning: failed to route frame: %v", err)
			continue
		}
		receivedFrames++
	}
	fmt.Printf("Received and demultiplexed %d frames\n\n", receivedFrames)

	// Extract housekeeping packets from VC0.
	// VCP Receive() maintains an internal buffer, so we keep calling it
	// until it returns an error (no more frames and buffer empty).
	fmt.Println("Extracting housekeeping packets from VC0:")
	hkCount := 0
	for {
		data, err := gsVpcHK.Receive()
		if err != nil {
			break
		}

		// Decode the Space Packet.
		pkt, err := spp.Decode(data, spp.WithDecodeErrorControl())
		if err != nil {
			log.Printf("  Warning: failed to decode space packet: %v", err)
			continue
		}

		// Extract housekeeping telemetry from packet payload.
		hk, err := decodeHousekeeping(pkt.UserData)
		if err != nil {
			log.Printf("  Warning: failed to decode HK data: %v", err)
			continue
		}

		fmt.Printf("  HK Packet (APID=%d, Seq=%d): Battery=%.1fV, Temp=%.1f°C, CPU=%d%%, Mem=%d%%, Mode=%d\n",
			pkt.PrimaryHeader.APID,
			pkt.PrimaryHeader.SequenceCount,
			hk.BatteryV, hk.TempC,
			hk.CPUPercent, hk.MemPercent, hk.ModeFlag,
		)
		hkCount++
	}
	fmt.Printf("  Total: %d housekeeping packets recovered\n", hkCount)

	// Extract science packets from VC1.
	fmt.Println("\nExtracting science packets from VC1:")
	sciCount := 0
	for {
		data, err := gsVpcSci.Receive()
		if err != nil {
			break
		}

		pkt, err := spp.Decode(data, spp.WithDecodeErrorControl())
		if err != nil {
			log.Printf("  Warning: failed to decode science packet: %v", err)
			continue
		}

		sampleCount := len(pkt.UserData) / 4
		fmt.Printf("  Science Packet (APID=%d, Seq=%d): %d float32 samples, %d bytes\n",
			pkt.PrimaryHeader.APID,
			pkt.PrimaryHeader.SequenceCount,
			sampleCount,
			len(pkt.UserData),
		)
		sciCount++
	}
	fmt.Printf("  Total: %d science packets recovered\n", sciCount)

	fmt.Println()
	fmt.Println("=== Simulation Complete ===")
}
