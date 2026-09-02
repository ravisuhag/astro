// Example: Making a capture to practise debugging on
//
// The other examples build a link and check that it works. This one builds a
// capture file that looks like something an antenna recorded, so the
// "Debug a real capture" guide has something realistic to work on:
//
//   - a run of noise before the first sync marker, because a recorder
//     starts before the signal locks
//   - two virtual channels multiplexed together
//   - real Space Packets from two applications, some of them spanning frames
//   - one frame missing, so the counters show a gap
//   - one frame with bit errors in it, so the CRC has something to reject
//
// It writes capture.bin next to itself and prints nothing but where it went
// and what is in it. Everything the guide works out from the file, it works
// out with the command line, not from this file.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

const (
	spacecraftID = 26  // the value the guide has to work out
	frameLength  = 256 // and this one
	apidHK       = 100
	apidScience  = 200
	vcidHK       = 0
	vcidScience  = 1

	dropFrame    = 5 // this frame never reaches the ground
	corruptFrame = 9 // this one arrives with bit errors
)

// The channel configuration. Nothing in the capture states any of it, which
// is the whole difficulty of reading someone else's recording.
var config = tmdl.ChannelConfig{
	FrameLength: frameLength,
	HasOCF:      false,
	HasFEC:      true,
}

func main() {
	frames := buildFrames()

	capture := new(bytes.Buffer)

	// Noise before the signal locks. A real recorder writes whatever the
	// demodulator produces, and that starts before there is anything to
	// demodulate.
	capture.Write(noise(137))

	kept, dropped, corrupted := 0, 0, 0
	for i, frame := range frames {
		if i == dropFrame {
			dropped++
			continue
		}

		cadu := tmsc.WrapCADU(frame, tmsc.DefaultASM(), true)

		if i == corruptFrame {
			// Flip a few bits in the middle. The randomizer spreads them, and
			// with no Reed-Solomon on this channel the frame CRC is the only
			// thing that will catch it.
			cadu[100] ^= 0x40
			cadu[101] ^= 0x02
			corrupted++
		}

		capture.Write(cadu)
		kept++
	}

	path := output()
	if err := os.WriteFile(path, capture.Bytes(), 0o644); err != nil {
		log.Fatalf("writing the capture: %v", err)
	}

	fmt.Printf("wrote %s\n", path)
	fmt.Printf("  %d octets\n", capture.Len())
	fmt.Printf("  %d CADUs, %d frame(s) dropped, %d corrupted\n",
		kept, dropped, corrupted)
	fmt.Println()
	fmt.Println("Now go and work out what is in it:")
	fmt.Println("  docs/content/docs/guides/debug-a-capture.md")
}

// buildFrames runs a normal two-channel downlink and returns the encoded
// frames. This is the downlink guide, compressed.
func buildFrames() [][]byte {
	physical := tmdl.NewPhysicalChannel("capture", config)
	master := tmdl.NewMasterChannel(spacecraftID, config)
	physical.AddMasterChannel(master, 1)

	vcHK := tmdl.NewVirtualChannel(vcidHK, 32)
	vcScience := tmdl.NewVirtualChannel(vcidScience, 32)
	master.AddVirtualChannel(vcHK, 3)
	master.AddVirtualChannel(vcScience, 1)

	// One counter for the whole master channel, so the MC count increments
	// across both virtual channels.
	counter := tmdl.NewFrameCounter()

	hk := tmdl.NewVirtualChannelPacketService(
		spacecraftID, vcidHK, vcHK, config, counter)
	hk.SetPacketSizer(spp.PacketSizer)

	science := tmdl.NewVirtualChannelPacketService(
		spacecraftID, vcidScience, vcScience, config, counter)
	science.SetPacketSizer(spp.PacketSizer)

	// Interleave the two streams the way a real spacecraft does: housekeeping
	// steadily, science in bursts. Frames are taken out of each virtual
	// channel as soon as the service releases one, which keeps the Master
	// Channel Frame Count in the order it was assigned.
	var frames [][]byte

	for i := range 60 {
		// Housekeeping: small packets, several to a frame.
		send(hk, packet(apidHK, uint16(i), housekeeping(i)))
		frames = append(frames, drain(config, vcHK, vcScience)...)

		// Science: large packets, each spanning frames.
		if i%10 == 0 {
			send(science, packet(apidScience, uint16(i/10),
				bytes.Repeat([]byte{byte(0xC0 + i/10)}, 400)))
			frames = append(frames, drain(config, vcHK, vcScience)...)
		}
	}

	// Flush one service at a time, so the frames still come out in counter
	// order. Both flushes complete a partial frame with a real idle packet.
	if err := hk.Flush(); err != nil {
		log.Fatalf("flushing housekeeping: %v", err)
	}
	frames = append(frames, drain(config, vcHK)...)

	if err := science.Flush(); err != nil {
		log.Fatalf("flushing science: %v", err)
	}
	frames = append(frames, drain(config, vcScience)...)

	return frames
}

// drain takes every frame waiting in the given channels and encodes it.
func drain(config tmdl.ChannelConfig, channels ...*tmdl.VirtualChannel) [][]byte {
	var out [][]byte
	for _, vc := range channels {
		for vc.Len() > 0 {
			frame, err := vc.Next()
			if err != nil {
				break
			}
			encoded, err := frame.EncodeWithConfig(config)
			if err != nil {
				log.Fatalf("encoding a frame: %v", err)
			}
			out = append(out, encoded)
		}
	}
	return out
}

func send(service *tmdl.VirtualChannelPacketService, encoded []byte) {
	if err := service.Send(encoded); err != nil {
		log.Fatalf("sending a packet: %v", err)
	}
}

func packet(apid, seq uint16, payload []byte) []byte {
	built, err := spp.NewTMPacket(apid, payload,
		spp.WithSequenceCount(seq), spp.WithErrorControl())
	if err != nil {
		log.Fatalf("building a packet: %v", err)
	}
	encoded, err := built.Encode()
	if err != nil {
		log.Fatalf("encoding a packet: %v", err)
	}
	return encoded
}

// housekeeping is a small plausible payload: elapsed time, battery, and
// temperature.
func housekeeping(i int) []byte {
	payload := new(bytes.Buffer)
	_ = binary.Write(payload, binary.BigEndian, uint32(1000+i))
	_ = binary.Write(payload, binary.BigEndian, float32(28.1-float32(i)*0.05))
	_ = binary.Write(payload, binary.BigEndian, float32(22.5+float32(i)*0.2))
	payload.WriteByte(1) // mode: nominal
	return payload.Bytes()
}

// noise is deterministic rubbish, so the capture is byte-identical on every
// run and the guide's offsets stay true.
func noise(n int) []byte {
	out := make([]byte, n)
	state := uint32(0x13579BDF)
	for i := range out {
		state = state*1664525 + 1013904223
		out[i] = byte(state >> 24)
	}
	return out
}

// output puts capture.bin next to this source file when run from the
// repository, and in the working directory otherwise.
func output() string {
	if _, err := os.Stat("examples/capture"); err == nil {
		return filepath.Join("examples", "capture", "capture.bin")
	}
	return "capture.bin"
}
