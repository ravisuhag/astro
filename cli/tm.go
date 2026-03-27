package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/spf13/cobra"
)

func tmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tm <command>",
		Short: "TM Transfer Frame operations",
		Long:  "Encode, decode, inspect, and analyze CCSDS TM Transfer Frames (CCSDS 132.0-B-3).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		tmDecodeCmd(),
		tmEncodeCmd(),
		tmInspectCmd(),
		tmGapsCmd(),
		tmDemuxCmd(),
	)

	return cmd
}

// tmFrameJSON is the JSON-serializable representation of a TM Transfer Frame.
type tmFrameJSON struct {
	VersionNumber    uint8  `json:"version_number"`
	SpacecraftID     uint16 `json:"spacecraft_id"`
	VirtualChannelID uint8  `json:"virtual_channel_id"`
	OCFFlag          bool   `json:"ocf_flag"`
	MCFrameCount     uint8  `json:"mc_frame_count"`
	VCFrameCount     uint8  `json:"vc_frame_count"`
	FSHFlag          bool   `json:"fsh_flag"`
	SyncFlag         bool   `json:"sync_flag"`
	PacketOrderFlag  bool   `json:"packet_order_flag"`
	SegmentLengthID  uint8  `json:"segment_length_id"`
	FirstHeaderPtr   uint16 `json:"first_header_ptr"`
	MCID             uint16 `json:"mcid"`
	GVCID            uint16 `json:"gvcid"`
	DataField        string `json:"data_field"`
	OCF              string `json:"ocf,omitempty"`
	FEC              string `json:"fec"`
	IsIdle           bool   `json:"is_idle"`
}

func toTMFrameJSON(f *tmdl.TMTransferFrame) tmFrameJSON {
	j := tmFrameJSON{
		VersionNumber:    f.Header.VersionNumber,
		SpacecraftID:     f.Header.SpacecraftID,
		VirtualChannelID: f.Header.VirtualChannelID,
		OCFFlag:          f.Header.OCFFlag,
		MCFrameCount:     f.Header.MCFrameCount,
		VCFrameCount:     f.Header.VCFrameCount,
		FSHFlag:          f.Header.FSHFlag,
		SyncFlag:         f.Header.SyncFlag,
		PacketOrderFlag:  f.Header.PacketOrderFlag,
		SegmentLengthID:  f.Header.SegmentLengthID,
		FirstHeaderPtr:   f.Header.FirstHeaderPtr,
		MCID:             f.Header.MCID(),
		GVCID:            f.Header.GVCID(),
		DataField:        hex.EncodeToString(f.DataField),
		FEC:              fmt.Sprintf("%04x", f.FrameErrorControl),
		IsIdle:           tmdl.IsIdleFrame(f),
	}
	if len(f.OperationalControl) > 0 {
		j.OCF = hex.EncodeToString(f.OperationalControl)
	}
	return j
}

func tmDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a TM Transfer Frame",
		Long:  "Decode a binary or hex-encoded TM Transfer Frame, printing its header fields and data.",
		Example: `  # Decode from hex stdin
  echo "003ec07f..." | astro tm decode --input hex

  # Decode binary file with JSON output
  astro tm decode --input bin --format json frame.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := tmdl.DecodeTMTransferFrame(data)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			return printTMFrame(frame, data, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")

	return cmd
}

func tmEncodeCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		dataHex   string
		ocfHex    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct a TM Transfer Frame from fields",
		Long:  "Build a CCSDS TM Transfer Frame from header fields and data. CRC is computed automatically.",
		Example: `  # Encode a basic TM frame
  astro tm encode --scid 26 --vcid 1 --data 0102030405

  # Encode with OCF
  astro tm encode --scid 26 --vcid 1 --data 0102030405 --ocf 00000000

  # Encode with JSON output
  astro tm encode --scid 26 --vcid 1 --data 0102030405 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userData, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			var ocf []byte
			if ocfHex != "" {
				ocf, err = hex.DecodeString(ocfHex)
				if err != nil {
					return fmt.Errorf("decoding --ocf hex: %w", err)
				}
				if len(ocf) != 4 {
					return fmt.Errorf("OCF must be exactly 4 bytes, got %d", len(ocf))
				}
			}

			frame, err := tmdl.NewTMTransferFrame(scid, vcid, userData, nil, ocf)
			if err != nil {
				return fmt.Errorf("building frame: %w", err)
			}

			encoded, err := frame.Encode()
			if err != nil {
				return fmt.Errorf("encoding frame: %w", err)
			}

			return printTMFrame(frame, encoded, outputFmt)
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-7)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data field as hex string")
	cmd.Flags().StringVar(&ocfHex, "ocf", "", "Operational Control Field as hex string (4 bytes)")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	_ = cmd.MarkFlagRequired("data")

	return cmd
}

func tmInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print a TM Transfer Frame with hex dump",
		Long:  "Display an annotated breakdown of a TM Transfer Frame showing header fields, data regions, and hex dump.",
		Example: `  # Inspect from hex stdin
  astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro tm inspect --input hex

  # Inspect binary file
  astro tm inspect --input bin frame.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := tmdl.DecodeTMTransferFrame(data)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			printTMInspect(frame, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func tmGapsCmd() *cobra.Command {
	var inputFmt string
	var frameLen int

	cmd := &cobra.Command{
		Use:   "gaps [file]",
		Short: "Detect frame gaps using MC/VC counters",
		Long:  "Scan a stream of concatenated TM Transfer Frames and report any gaps or discontinuities in the Master Channel and Virtual Channel frame counters.",
		Example: `  # Detect gaps in a binary capture
  astro tm gaps --input bin --frame-len 256 capture.bin

  # Detect gaps in hex input
  astro tm gaps --input hex --frame-len 20 frames.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if frameLen <= 0 {
				return fmt.Errorf("--frame-len is required and must be positive")
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			return detectGaps(data, frameLen)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Fixed frame length in bytes (required)")

	_ = cmd.MarkFlagRequired("frame-len")

	return cmd
}

func tmDemuxCmd() *cobra.Command {
	var inputFmt, outputFmt string
	var frameLen int
	var filterVCID uint8

	cmd := &cobra.Command{
		Use:   "demux [file]",
		Short: "Filter TM frames by Virtual Channel ID",
		Long:  "Demultiplex a stream of concatenated TM Transfer Frames, outputting only frames matching the specified VCID.",
		Example: `  # Extract VCID 2 frames from a binary capture
  astro tm demux --input bin --frame-len 256 --vcid 2 capture.bin

  # Demux with JSON output
  astro tm demux --input hex --frame-len 20 --vcid 0 --format json frames.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if frameLen <= 0 {
				return fmt.Errorf("--frame-len is required and must be positive")
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			return demuxFrames(data, frameLen, filterVCID, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Fixed frame length in bytes (required)")
	cmd.Flags().Uint8Var(&filterVCID, "vcid", 0, "Virtual Channel ID to filter (0-7)")

	_ = cmd.MarkFlagRequired("frame-len")
	_ = cmd.MarkFlagRequired("vcid")

	return cmd
}

// printTMFrame outputs a decoded TM frame in the specified format.
func printTMFrame(f *tmdl.TMTransferFrame, raw []byte, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toTMFrameJSON(f), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "hex":
		fmt.Println(hex.EncodeToString(raw))
	case "text":
		fmt.Println("TM Transfer Frame:")
		fmt.Println("Primary Header:")
		fmt.Println(f.Header.Humanize())
		fmt.Printf("  MCID: %d\n", f.Header.MCID())
		fmt.Printf("  GVCID: %d\n", f.Header.GVCID())
		if f.Header.FSHFlag {
			fmt.Println("Secondary Header:")
			fmt.Println(f.SecondaryHeader.Humanize())
		}
		fmt.Printf("Data Field: %d bytes\n", len(f.DataField))
		if len(f.OperationalControl) > 0 {
			fmt.Printf("OCF: %s\n", hex.EncodeToString(f.OperationalControl))
		}
		fmt.Printf("FEC: 0x%04X\n", f.FrameErrorControl)
		if tmdl.IsIdleFrame(f) {
			fmt.Println("[IDLE FRAME]")
		}
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'json', or 'hex')", format)
	}
	return nil
}

// printTMInspect displays an annotated breakdown of a TM Transfer Frame.
func printTMInspect(f *tmdl.TMTransferFrame, raw []byte) {
	h := f.Header

	fmt.Println("TM Transfer Frame Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// Primary Header
	fmt.Println("Primary Header (6 bytes)")
	fmt.Printf("  Version .............. %d\n", h.VersionNumber)
	fmt.Printf("  Spacecraft ID ........ %d (0x%03X)\n", h.SpacecraftID, h.SpacecraftID)
	fmt.Printf("  Virtual Channel ID ... %d\n", h.VirtualChannelID)
	fmt.Printf("  OCF Flag ............. %v\n", h.OCFFlag)
	fmt.Printf("  MC Frame Count ....... %d\n", h.MCFrameCount)
	fmt.Printf("  VC Frame Count ....... %d\n", h.VCFrameCount)
	fmt.Printf("  FSH Flag ............. %v\n", h.FSHFlag)
	fmt.Printf("  Sync Flag ............ %v\n", h.SyncFlag)
	fmt.Printf("  Packet Order Flag .... %v\n", h.PacketOrderFlag)
	fmt.Printf("  Segment Length ID .... %d\n", h.SegmentLengthID)
	fmt.Printf("  First Header Ptr ..... %d (0x%03X)\n", h.FirstHeaderPtr, h.FirstHeaderPtr)
	fmt.Printf("  MCID ................. %d\n", h.MCID())
	fmt.Printf("  GVCID ................ %d\n", h.GVCID())

	if tmdl.IsIdleFrame(f) {
		fmt.Println("  [IDLE FRAME]")
	}

	// Secondary Header
	if h.FSHFlag {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Secondary Header (%d bytes)\n", 1+len(f.SecondaryHeader.DataField))
		fmt.Println(f.SecondaryHeader.Humanize())
	}

	// Data Field
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Data Field (%d bytes)\n", len(f.DataField))
	if len(f.DataField) > 0 {
		fmt.Print(hexDump(f.DataField, "  "))
	}

	// OCF
	if len(f.OperationalControl) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Operational Control Field (4 bytes): %s\n", hex.EncodeToString(f.OperationalControl))
	}

	// FEC
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Frame Error Control: 0x%04X (CRC-16-CCITT)\n", f.FrameErrorControl)

	// Full hex dump
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw Frame (%d bytes)\n", len(raw))
	fmt.Print(hexDump(raw, "  "))
}

// splitFrames splits data into fixed-length frames.
func splitFrames(data []byte, frameLen int) ([][]byte, error) {
	if len(data)%frameLen != 0 {
		fmt.Fprintf(os.Stderr, "Warning: data length %d is not a multiple of frame length %d, %d trailing bytes ignored\n",
			len(data), frameLen, len(data)%frameLen)
	}

	var frames [][]byte
	for offset := 0; offset+frameLen <= len(data); offset += frameLen {
		frames = append(frames, data[offset:offset+frameLen])
	}
	return frames, nil
}

// detectGaps scans frames for MC/VC counter discontinuities.
func detectGaps(data []byte, frameLen int) error {
	chunks, err := splitFrames(data, frameLen)
	if err != nil {
		return err
	}

	type vcState struct {
		lastVC    uint8
		lastIndex int
	}

	lastMC := int(-1)
	lastMCIndex := -1
	vcStates := make(map[uint8]*vcState)
	gapCount := 0

	for i, chunk := range chunks {
		frame, err := tmdl.DecodeTMTransferFrame(chunk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: frame #%d decode error: %v, skipping\n", i+1, err)
			continue
		}

		h := frame.Header
		mc := int(h.MCFrameCount)

		// Check MC counter
		if lastMC >= 0 {
			expected := (lastMC + 1) & 0xFF
			if mc != expected {
				gapCount++
				fmt.Printf("MC gap: frame #%d → #%d, expected MC=%d, got MC=%d (SCID=%d)\n",
					lastMCIndex+1, i+1, expected, mc, h.SpacecraftID)
			}
		}
		lastMC = mc
		lastMCIndex = i

		// Check VC counter
		vcid := h.VirtualChannelID
		if vs, ok := vcStates[vcid]; ok {
			expectedVC := (int(vs.lastVC) + 1) & 0xFF
			if int(h.VCFrameCount) != expectedVC {
				gapCount++
				fmt.Printf("VC gap: VCID=%d, frame #%d → #%d, expected VC=%d, got VC=%d\n",
					vcid, vs.lastIndex+1, i+1, expectedVC, h.VCFrameCount)
			}
		}
		vcStates[vcid] = &vcState{lastVC: h.VCFrameCount, lastIndex: i}
	}

	fmt.Printf("\nScanned %d frame(s), found %d gap(s).\n", len(chunks), gapCount)
	return nil
}

// demuxFrames filters frames by VCID.
func demuxFrames(data []byte, frameLen int, vcid uint8, outputFmt string) error {
	chunks, err := splitFrames(data, frameLen)
	if err != nil {
		return err
	}

	matched := 0
	for i, chunk := range chunks {
		frame, err := tmdl.DecodeTMTransferFrame(chunk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: frame #%d decode error: %v, skipping\n", i+1, err)
			continue
		}

		if frame.Header.VirtualChannelID != vcid {
			continue
		}

		matched++
		switch outputFmt {
		case "json":
			j := toTMFrameJSON(frame)
			b, _ := json.Marshal(j)
			fmt.Println(string(b))
		case "hex":
			fmt.Println(hex.EncodeToString(chunk))
		case "text":
			fmt.Printf("--- Frame #%d (SCID=%d VCID=%d MC=%d VC=%d) ---\n",
				i+1, frame.Header.SpacecraftID, frame.Header.VirtualChannelID,
				frame.Header.MCFrameCount, frame.Header.VCFrameCount)
			fmt.Printf("  Data: %d bytes", len(frame.DataField))
			if tmdl.IsIdleFrame(frame) {
				fmt.Print(" [IDLE]")
			}
			fmt.Println()
		}
	}

	if outputFmt == "text" {
		fmt.Printf("\nMatched %d of %d frame(s) on VCID=%d.\n", matched, len(chunks), vcid)
	}
	return nil
}
