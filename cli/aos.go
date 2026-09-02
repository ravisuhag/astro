package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/aos"
	"github.com/spf13/cobra"
)

func aosCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aos <command>",
		Short: "AOS Transfer Frame operations",
		Long:  "Encode, decode, inspect, and generate AOS Transfer Frames (CCSDS 732.0-B-4).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		aosEncodeCmd(),
		aosDecodeCmd(),
		aosInspectCmd(),
		aosGapsCmd(),
		aosDemuxCmd(),
		aosGenCmd(),
	)
	return cmd
}

// aosFrameJSON is the JSON-serializable representation of an AOS Transfer Frame.
type aosFrameJSON struct {
	TFVN              uint8  `json:"tfvn"`
	SpacecraftID      uint8  `json:"spacecraft_id"`
	VirtualChannelID  uint8  `json:"virtual_channel_id"`
	VCFrameCount      uint32 `json:"vc_frame_count"`
	ReplayFlag        bool   `json:"replay_flag"`
	VCFCUsageFlag     bool   `json:"vcfc_usage_flag"`
	VCFrameCountCycle uint8  `json:"vc_frame_count_cycle"`
	MCID              uint16 `json:"mcid"`
	GVCID             uint32 `json:"gvcid"`
	InsertZone        string `json:"insert_zone,omitempty"`
	DataField         string `json:"data_field"`
	OCF               string `json:"ocf,omitempty"`
	FECF              string `json:"fecf,omitempty"`
	IsIdle            bool   `json:"is_idle"`
}

func toAOSFrameJSON(f *aos.TransferFrame) aosFrameJSON {
	j := aosFrameJSON{
		TFVN:              f.Header.TFVN,
		SpacecraftID:      f.Header.SCID,
		VirtualChannelID:  f.Header.VCID,
		VCFrameCount:      f.Header.VCFrameCount,
		ReplayFlag:        f.Header.ReplayFlag,
		VCFCUsageFlag:     f.Header.VCFCUsageFlag,
		VCFrameCountCycle: f.Header.VCFrameCountCycle,
		MCID:              f.Header.MCID(),
		GVCID:             f.Header.GVCID(),
		DataField:         hex.EncodeToString(f.DataField),
		IsIdle:            aos.IsIdleFrame(f),
	}
	if len(f.InsertZone) > 0 {
		j.InsertZone = hex.EncodeToString(f.InsertZone)
	}
	if len(f.OCF) > 0 {
		j.OCF = hex.EncodeToString(f.OCF)
	}
	if len(f.FECF) > 0 {
		j.FECF = hex.EncodeToString(f.FECF)
	}
	return j
}

func aosEncodeCmd() *cobra.Command {
	var (
		scid      uint8
		vcid      uint8
		dataHex   string
		ocfHex    string
		insertHex string
		fecf      bool
		vcCount   uint32
		replay    bool
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct an AOS Transfer Frame from fields",
		Long:  "Build an AOS Transfer Frame from header fields and data. FECF is computed automatically when enabled.",
		Example: `  # Encode a basic AOS frame
  astro aos encode --scid 50 --vcid 1 --data 0102030405

  # Encode with FECF and OCF
  astro aos encode --scid 50 --vcid 1 --data 0102030405 --ocf 00000000 --fecf

  # Encode with JSON output
  astro aos encode --scid 50 --vcid 1 --data 0102030405 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userData, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			var opts []aos.FrameOption
			opts = append(opts, aos.WithVCFrameCount(vcCount))
			if replay {
				opts = append(opts, aos.WithReplayFlag())
			}
			if ocfHex != "" {
				ocf, err := hex.DecodeString(ocfHex)
				if err != nil {
					return fmt.Errorf("decoding --ocf hex: %w", err)
				}
				if len(ocf) != aos.OCFSize {
					return fmt.Errorf("OCF must be exactly %d bytes, got %d", aos.OCFSize, len(ocf))
				}
				opts = append(opts, aos.WithOCF(ocf))
			}
			if insertHex != "" {
				iz, err := hex.DecodeString(insertHex)
				if err != nil {
					return fmt.Errorf("decoding --insert hex: %w", err)
				}
				opts = append(opts, aos.WithInsertZone(iz))
			}
			if fecf {
				opts = append(opts, aos.WithFECF())
			}

			frame, err := aos.NewTransferFrame(scid, vcid, userData, opts...)
			if err != nil {
				return fmt.Errorf("building frame: %w", err)
			}
			encoded, err := frame.Encode()
			if err != nil {
				return fmt.Errorf("encoding frame: %w", err)
			}
			return printAOSFrame(frame, encoded, outputFmt)
		},
	}

	cmd.Flags().Uint8Var(&scid, "scid", 0, "Spacecraft ID (0-255)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data field as hex string")
	cmd.Flags().StringVar(&ocfHex, "ocf", "", "Operational Control Field as hex string (4 bytes)")
	cmd.Flags().StringVar(&insertHex, "insert", "", "Insert Zone as hex string")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Append CRC-16 Frame Error Control Field")
	cmd.Flags().Uint32Var(&vcCount, "vc-count", 0, "VC Frame Count (24-bit)")
	cmd.Flags().BoolVar(&replay, "replay", false, "Set the Replay Flag")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	_ = cmd.MarkFlagRequired("data")
	return cmd
}

func aosDecodeCmd() *cobra.Command {
	var (
		inputFmt      string
		outputFmt     string
		fecf          bool
		ocf           bool
		insertZoneLen int
	)

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode an AOS Transfer Frame",
		Long:  "Decode a binary or hex-encoded AOS Transfer Frame, printing its header fields and data.",
		Example: `  # Decode from hex stdin
  echo "40320000000000..." | astro aos decode --input hex

  # Decode with FECF
  astro aos decode --input hex --fecf < frame.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			frame, err := aos.DecodeTransferFrame(data, insertZoneLen, ocf, fecf)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}
			return printAOSFrame(frame, data, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Frame includes a 2-byte FECF")
	cmd.Flags().BoolVar(&ocf, "ocf", false, "Frame includes a 4-byte OCF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")
	return cmd
}

func aosInspectCmd() *cobra.Command {
	var (
		inputFmt      string
		fecf          bool
		ocf           bool
		insertZoneLen int
	)

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print an AOS Transfer Frame with hex dump",
		Long:  "Display an annotated breakdown of an AOS Transfer Frame showing header fields, data regions, and hex dump.",
		Example: `  # Inspect from hex stdin
  astro aos encode --scid 50 --vcid 1 --data 0102030405 --fecf | astro aos inspect --input hex --fecf`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			frame, err := aos.DecodeTransferFrame(data, insertZoneLen, ocf, fecf)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}
			printAOSInspect(frame, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Frame includes a 2-byte FECF")
	cmd.Flags().BoolVar(&ocf, "ocf", false, "Frame includes a 4-byte OCF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")
	return cmd
}

func aosGenCmd() *cobra.Command {
	var (
		scid      uint8
		vcid      uint8
		count     int
		dataSize  int
		fecf      bool
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic AOS Transfer Frames",
		Long:  "Generate a stream of synthetic AOS Transfer Frames with incrementing VC frame counts and random data.",
		Example: `  # Generate 10 AOS frames
  astro aos gen --scid 50 --vcid 1 --count 10 --data-size 64

  # Generate with FECF
  astro aos gen --scid 50 --vcid 1 --count 5 --data-size 32 --fecf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var frameSize int
			if count < 0 {
				return fmt.Errorf("count must be >= 0, got %d", count)
			}

			for i := range count {
				data, err := randomBytes(dataSize)
				if err != nil {
					return err
				}

				opts := []aos.FrameOption{
					aos.WithVCFrameCount(uint32(i) & aos.MaxVCFrameCount),
				}
				if fecf {
					opts = append(opts, aos.WithFECF())
				}

				frame, err := aos.NewTransferFrame(scid, vcid, data, opts...)
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
			fmt.Fprintf(os.Stderr, "Generated %d frame(s), SCID=%d VCID=%d, %d bytes each\n",
				count, scid, vcid, frameSize)
			return nil
		},
	}

	cmd.Flags().Uint8Var(&scid, "scid", 0, "Spacecraft ID (0-255)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of frames to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 64, "Data field size in bytes per frame")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Append CRC-16 Frame Error Control Field")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")
	return cmd
}

func aosGapsCmd() *cobra.Command {
	var (
		inputFmt      string
		frameLen      int
		fecf          bool
		ocf           bool
		insertZoneLen int
	)

	cmd := &cobra.Command{
		Use:   "gaps [file]",
		Short: "Detect frame gaps using the VC frame counter",
		Long: "Scan a stream of concatenated AOS Transfer Frames and report gaps in the Virtual Channel frame counter.\n\n" +
			"AOS has no master channel frame count, so only virtual channel gaps are reported. Where a frame sets the VC Frame Count Usage Flag, the 4-bit cycle is folded in above the 24-bit count and the pair is treated as one 28-bit counter.",
		Example: `  # Detect gaps in a binary capture
  astro aos gaps --input bin --frame-len 1024 capture.bin

  # Frames carrying an insert zone and FECF
  astro aos gaps --input bin --frame-len 1115 --insert-len 8 --fecf capture.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if frameLen <= 0 {
				return fmt.Errorf("--frame-len is required and must be positive")
			}

			source, closer, err := openInput(args, inputFmt)
			if err != nil {
				return err
			}
			defer func() { _ = closer.Close() }()

			scanner := newGapScanner(aosProtocol(insertZoneLen, ocf, fecf), os.Stdout, os.Stderr)
			if err := streamFixed(source, frameLen,
				scanner.track, trailingWarning(os.Stderr, frameLen)); err != nil {
				return err
			}
			scanner.summary()

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Fixed frame length in bytes (required)")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Frames include a 2-byte FECF")
	cmd.Flags().BoolVar(&ocf, "ocf", false, "Frames include a 4-byte OCF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")

	_ = cmd.MarkFlagRequired("frame-len")

	return cmd
}

func aosDemuxCmd() *cobra.Command {
	var (
		inputFmt      string
		outputFmt     string
		frameLen      int
		filterVCID    uint8
		fecf          bool
		ocf           bool
		insertZoneLen int
	)

	cmd := &cobra.Command{
		Use:   "demux [file]",
		Short: "Filter AOS frames by Virtual Channel ID",
		Long:  "Demultiplex a stream of concatenated AOS Transfer Frames, passing on only the frames matching the given VCID.",
		Example: `  # Extract VCID 2 frames from a binary capture
  astro aos demux --input bin --frame-len 1024 --vcid 2 capture.bin

  # Demux with JSON output
  astro aos demux --input hex --frame-len 70 --vcid 0 --format json frames.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if frameLen <= 0 {
				return fmt.Errorf("--frame-len is required and must be positive")
			}
			if err := checkFrameFormat(outputFmt); err != nil {
				return err
			}

			source, closer, err := openInput(args, inputFmt)
			if err != nil {
				return err
			}
			defer func() { _ = closer.Close() }()

			demux := &demuxer{
				proto:  aosProtocol(insertZoneLen, ocf, fecf),
				vcid:   filterVCID,
				out:    os.Stdout,
				errOut: os.Stderr,
				emit:   emitAOSFrame(outputFmt, insertZoneLen, ocf, fecf),
			}
			if err := streamFixed(source, frameLen,
				demux.filter, trailingWarning(os.Stderr, frameLen)); err != nil {
				return err
			}
			if outputFmt == "text" {
				demux.summary()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Fixed frame length in bytes (required)")
	cmd.Flags().Uint8Var(&filterVCID, "vcid", 0, "Virtual Channel ID to filter (0-63)")
	cmd.Flags().BoolVar(&fecf, "fecf", false, "Frames include a 2-byte FECF")
	cmd.Flags().BoolVar(&ocf, "ocf", false, "Frames include a 4-byte OCF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")

	_ = cmd.MarkFlagRequired("frame-len")
	_ = cmd.MarkFlagRequired("vcid")

	return cmd
}

// aosProtocol describes AOS Transfer Frames to the receive loop.
//
// The VC frame count is twenty-four bits (CCSDS 732.0-B-4 §4.1.2.3), and
// where the VC Frame Count Usage Flag is set the signalling field carries a
// four-bit cycle that increments on each wrap (§4.1.2.5.5.3). There is no
// master channel frame count in an AOS header, so mcMask stays zero and no
// master channel gaps are reported.
func aosProtocol(insertZoneLen int, hasOCF, hasFECF bool) frameProtocol {
	return frameProtocol{
		vcMask:    aos.MaxVCFrameCount,
		cycleMask: 0x0F,
		ident: func(raw []byte) (frameIdent, error) {
			frame, err := aos.DecodeTransferFrame(raw, insertZoneLen, hasOCF, hasFECF)
			if err != nil {
				return frameIdent{}, err
			}
			h := frame.Header
			return frameIdent{
				scid:     uint16(h.SCID),
				vcid:     h.VCID,
				vcCount:  uint64(h.VCFrameCount),
				cycle:    h.VCFrameCountCycle,
				hasCycle: h.VCFCUsageFlag,
			}, nil
		},
	}
}

// emitAOSFrame returns the demux emitter for one output format.
func emitAOSFrame(outputFmt string, insertZoneLen int, hasOCF, hasFECF bool) func(raw []byte, index int, ident frameIdent) error {
	return func(raw []byte, index int, ident frameIdent) error {
		frame, err := aos.DecodeTransferFrame(raw, insertZoneLen, hasOCF, hasFECF)
		if err != nil {
			return err
		}

		switch outputFmt {
		case "json":
			b, err := json.Marshal(toAOSFrameJSON(frame))
			if err != nil {
				return fmt.Errorf("encoding JSON output: %w", err)
			}
			fmt.Println(string(b))
		case "hex":
			fmt.Println(hex.EncodeToString(raw))
		case "text":
			fmt.Printf("--- Frame #%d (SCID=%d VCID=%d VC=%d) ---\n",
				index, ident.scid, ident.vcid, ident.vcCount)
			fmt.Printf("  Data: %d bytes", len(frame.DataField))
			if aos.IsIdleFrame(frame) {
				fmt.Print(" [IDLE]")
			}
			fmt.Println()
		}

		return nil
	}
}

// printAOSFrame outputs a decoded AOS frame in the specified format.
func printAOSFrame(f *aos.TransferFrame, raw []byte, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toAOSFrameJSON(f), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "hex":
		fmt.Println(hex.EncodeToString(raw))
	case "text":
		fmt.Println(f.Humanize())
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'json', or 'hex')", format)
	}
	return nil
}

// printAOSInspect displays an annotated breakdown of an AOS Transfer Frame.
func printAOSInspect(f *aos.TransferFrame, raw []byte) {
	h := f.Header

	fmt.Println("AOS Transfer Frame Inspector")
	fmt.Println(strings.Repeat("─", 60))

	fmt.Printf("Primary Header (%d bytes)\n", aos.PrimaryHeaderSize)
	fmt.Printf("  TFVN ........................ %d (0b%02b)\n", h.TFVN, h.TFVN)
	fmt.Printf("  Spacecraft ID ............... %d (0x%02X)\n", h.SCID, h.SCID)
	fmt.Printf("  Virtual Channel ID .......... %d\n", h.VCID)
	fmt.Printf("  VC Frame Count .............. %d (0x%06X)\n", h.VCFrameCount, h.VCFrameCount)
	fmt.Printf("  Replay Flag ................. %v\n", h.ReplayFlag)
	fmt.Printf("  VC Frame Count Usage Flag ... %v\n", h.VCFCUsageFlag)
	fmt.Printf("  VC Frame Count Cycle ........ %d\n", h.VCFrameCountCycle)
	fmt.Printf("  MCID ........................ %d\n", h.MCID())
	fmt.Printf("  GVCID ....................... %d\n", h.GVCID())

	if len(f.InsertZone) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Insert Zone (%d bytes)\n", len(f.InsertZone))
		fmt.Print(hexDump(f.InsertZone, "  "))
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Data Field (%d bytes)\n", len(f.DataField))
	if len(f.DataField) > 0 {
		fmt.Print(hexDump(f.DataField, "  "))
	}
	if aos.IsIdleFrame(f) {
		fmt.Println("  [OID FRAME — VCID 63]")
	}

	if len(f.OCF) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Operational Control Field (4 bytes): %s\n", hex.EncodeToString(f.OCF))
	}

	if len(f.FECF) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Frame Error Control: 0x%s (CRC-16-CCITT)\n", hex.EncodeToString(f.FECF))
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw Frame (%d bytes)\n", len(raw))
	fmt.Print(hexDump(raw, "  "))
}
