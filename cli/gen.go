package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/ravisuhag/astro/pkg/crc"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/ravisuhag/astro/pkg/tcsc"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
	"github.com/spf13/cobra"
)

// randomBytes generates n random bytes.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, b)
	return b
}

func sppGenCmd() *cobra.Command {
	var (
		apid       uint16
		packetType string
		count      int
		size       int
		crcFlag    bool
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic Space Packets",
		Long:  "Generate a stream of synthetic Space Packets with incrementing sequence counts and random data.",
		Example: `  # Generate 10 TM packets of 64 bytes each
  astro spp gen --apid 100 --count 10 --size 64

  # Generate packets and pipe to stream
  astro spp gen --apid 100 --count 50 --size 128 --format bin | astro spp stream --input bin

  # Generate with CRC
  astro spp gen --apid 100 --count 5 --size 32 --crc`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var pktType uint8
			switch packetType {
			case "tm", "0":
				pktType = spp.PacketTypeTM
			case "tc", "1":
				pktType = spp.PacketTypeTC
			default:
				return fmt.Errorf("invalid --type: %s (use 'tm' or 'tc')", packetType)
			}

			for i := range count {
				data := randomBytes(size)
				opts := []spp.PacketOption{
					spp.WithSequenceCount(uint16(i) & 0x3FFF),
				}
				if crcFlag {
					opts = append(opts, spp.WithErrorControl())
				}

				pkt, err := spp.NewSpacePacket(apid, pktType, data, opts...)
				if err != nil {
					return fmt.Errorf("packet #%d: %w", i+1, err)
				}
				encoded, err := pkt.Encode()
				if err != nil {
					return fmt.Errorf("packet #%d: %w", i+1, err)
				}

				if err := writeGenOutput(encoded, outputFmt); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Generated %d packet(s), APID=%d, %d data bytes each\n", count, apid, size)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&apid, "apid", 0, "Application Process Identifier (0-2047)")
	cmd.Flags().StringVar(&packetType, "type", "tm", "Packet type: tm or tc")
	cmd.Flags().IntVar(&count, "count", 10, "Number of packets to generate")
	cmd.Flags().IntVar(&size, "size", 64, "User data size in bytes per packet")
	cmd.Flags().BoolVar(&crcFlag, "crc", false, "Append CRC-16-CCITT error control field")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

func tmGenCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		count     int
		dataSize  int
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic TM Transfer Frames",
		Long:  "Generate a stream of synthetic TM Transfer Frames with incrementing MC/VC counters and random data.",
		Example: `  # Generate 10 TM frames
  astro tm gen --scid 26 --vcid 1 --count 10 --data-size 1024

  # Generate and check for gaps (should find none)
  astro tm gen --scid 26 --vcid 0 --count 50 --data-size 100 | astro tm gaps --input bin --frame-len 108`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var frameSize int

			for i := range count {
				data := randomBytes(dataSize)

				// Build a base frame to get a valid structure, then
				// reconstruct with correct counters. NewTMTransferFrame
				// computes CRC, so we set counters first by building
				// a fresh frame each iteration.
				frame := &tmdl.TMTransferFrame{
					Header: tmdl.PrimaryHeader{
						VersionNumber:    0,
						SpacecraftID:     scid & 0x03FF,
						VirtualChannelID: vcid & 0x07,
						MCFrameCount:     uint8(i) & 0xFF,
						VCFrameCount:     uint8(i) & 0xFF,
						SegmentLengthID:  0b11,
					},
					DataField: data,
				}

				// Compute CRC over frame without FEC
				frameWithoutFEC, err := frame.EncodeWithoutFEC()
				if err != nil {
					return fmt.Errorf("frame #%d: %w", i+1, err)
				}
				frame.FrameErrorControl = crc.ComputeCRC16(frameWithoutFEC)

				encoded, err := frame.Encode()
				if err != nil {
					return fmt.Errorf("frame #%d: %w", i+1, err)
				}

				if i == 0 {
					frameSize = len(encoded)
				}

				if err := writeGenOutput(encoded, outputFmt); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Generated %d frame(s), SCID=%d VCID=%d, %d bytes each\n", count, scid, vcid, frameSize)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-7)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of frames to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 1024, "Data field size in bytes per frame")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

func tcGenCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		count     int
		dataSize  int
		bypass    bool
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic TC Transfer Frames",
		Long:  "Generate a stream of synthetic TC Transfer Frames with incrementing sequence numbers and random data.",
		Example: `  # Generate 10 TC frames
  astro tc gen --scid 26 --vcid 1 --count 10 --data-size 64

  # Generate bypass frames
  astro tc gen --scid 26 --vcid 1 --count 5 --data-size 32 --bypass`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var frameSize int

			for i := range count {
				data := randomBytes(dataSize)

				var opts []tcdl.FrameOption
				opts = append(opts, tcdl.WithSequenceNumber(uint8(i)&0xFF))
				if bypass {
					opts = append(opts, tcdl.WithBypass())
				}

				frame, err := tcdl.NewTCTransferFrame(scid, vcid, data, opts...)
				if err != nil {
					return fmt.Errorf("frame #%d: %w", i+1, err)
				}

				encoded, err := frame.Encode()
				if err != nil {
					return fmt.Errorf("frame #%d: %w", i+1, err)
				}

				if i == 0 {
					frameSize = len(encoded)
				}

				if err := writeGenOutput(encoded, outputFmt); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Generated %d frame(s), SCID=%d VCID=%d, %d bytes each\n", count, scid, vcid, frameSize)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of frames to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 64, "Data field size in bytes per frame")
	cmd.Flags().BoolVar(&bypass, "bypass", false, "Set Type-B (expedited) bypass flag")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

func caduGenCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		count     int
		dataSize  int
		randomize bool
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic CADUs",
		Long:  "Generate a stream of synthetic CADUs (ASM + TM frame), with incrementing counters and random data.",
		Example: `  # Generate 100 CADUs
  astro cadu gen --scid 1 --vcid 0 --count 100 --data-size 1024

  # Generate randomized CADUs and sync them back
  astro cadu gen --scid 1 --count 10 --data-size 100 --format bin | astro cadu sync --input bin --frame-len 112`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var caduSize int

			for i := range count {
				data := randomBytes(dataSize)

				frame := &tmdl.TMTransferFrame{
					Header: tmdl.PrimaryHeader{
						VersionNumber:    0,
						SpacecraftID:     scid & 0x03FF,
						VirtualChannelID: vcid & 0x07,
						MCFrameCount:     uint8(i) & 0xFF,
						VCFrameCount:     uint8(i) & 0xFF,
						SegmentLengthID:  0b11,
					},
					DataField: data,
				}

				frameWithoutFEC, err := frame.EncodeWithoutFEC()
				if err != nil {
					return fmt.Errorf("CADU #%d: %w", i+1, err)
				}
				frame.FrameErrorControl = crc.ComputeCRC16(frameWithoutFEC)

				frameBytes, err := frame.Encode()
				if err != nil {
					return fmt.Errorf("CADU #%d: %w", i+1, err)
				}

				cadu := tmsc.WrapCADU(frameBytes, nil, randomize)

				if i == 0 {
					caduSize = len(cadu)
				}

				if err := writeGenOutput(cadu, outputFmt); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Generated %d CADU(s), SCID=%d VCID=%d, %d bytes each\n", count, scid, vcid, caduSize)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-7)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of CADUs to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 1024, "TM frame data field size in bytes")
	cmd.Flags().BoolVar(&randomize, "randomize", false, "Apply CCSDS pseudo-randomization")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

func cltuGenCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		count     int
		dataSize  int
		randomize bool
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic CLTUs",
		Long:  "Generate a stream of synthetic CLTUs (BCH-encoded TC frames with start/tail sequences).",
		Example: `  # Generate 10 CLTUs
  astro cltu gen --scid 26 --vcid 1 --count 10 --data-size 64

  # Generate and inspect the first one
  astro cltu gen --scid 26 --vcid 1 --count 1 --data-size 32 --format hex | astro cltu inspect --input hex`,
		RunE: func(cmd *cobra.Command, args []string) error {
			for i := range count {
				data := randomBytes(dataSize)

				frame, err := tcdl.NewTCTransferFrame(scid, vcid, data,
					tcdl.WithSequenceNumber(uint8(i)&0xFF))
				if err != nil {
					return fmt.Errorf("CLTU #%d: %w", i+1, err)
				}

				frameBytes, err := frame.Encode()
				if err != nil {
					return fmt.Errorf("CLTU #%d: %w", i+1, err)
				}

				cltu, err := tcsc.WrapCLTU(frameBytes, nil, nil, randomize)
				if err != nil {
					return fmt.Errorf("CLTU #%d: %w", i+1, err)
				}

				if err := writeGenOutput(cltu, outputFmt); err != nil {
					return err
				}
			}

			fmt.Fprintf(os.Stderr, "Generated %d CLTU(s), SCID=%d VCID=%d\n", count, scid, vcid)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of CLTUs to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 64, "TC frame data field size in bytes")
	cmd.Flags().BoolVar(&randomize, "randomize", false, "Apply CCSDS pseudo-randomization")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

// writeGenOutput writes encoded data in the specified format.
// bin writes raw bytes to stdout; hex writes one hex line per item.
func writeGenOutput(data []byte, format string) error {
	switch format {
	case "bin":
		_, err := os.Stdout.Write(data)
		return err
	case "hex":
		fmt.Println(hex.EncodeToString(data))
		return nil
	default:
		return fmt.Errorf("unknown format: %s (use 'bin' or 'hex')", format)
	}
}
