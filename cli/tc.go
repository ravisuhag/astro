package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/spf13/cobra"
)

func tcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tc <command>",
		Short: "TC Transfer Frame operations",
		Long:  "Encode, decode, and inspect CCSDS TC Transfer Frames (CCSDS 232.0-B-4).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		tcDecodeCmd(),
		tcEncodeCmd(),
		tcInspectCmd(),
		tcGenCmd(),
	)

	return cmd
}

// tcFrameJSON is the JSON-serializable representation of a TC Transfer Frame.
type tcFrameJSON struct {
	VersionNumber      uint8  `json:"version_number"`
	BypassFlag         uint8  `json:"bypass_flag"`
	BypassName         string `json:"bypass_name"`
	ControlCommandFlag uint8  `json:"control_command_flag"`
	ControlCommandName string `json:"control_command_name"`
	SpacecraftID       uint16 `json:"spacecraft_id"`
	VirtualChannelID   uint8  `json:"virtual_channel_id"`
	FrameLength        uint16 `json:"frame_length"`
	FrameSequenceNum   uint8  `json:"frame_sequence_num"`
	MCID               uint16 `json:"mcid"`
	GVCID              uint32 `json:"gvcid"`
	DataField          string `json:"data_field"`
	FEC                string `json:"fec"`
	IsBypass           bool   `json:"is_bypass"`
	IsControl          bool   `json:"is_control"`
}

func toTCFrameJSON(f *tcdl.TCTransferFrame) tcFrameJSON {
	return tcFrameJSON{
		VersionNumber:      f.Header.VersionNumber,
		BypassFlag:         f.Header.BypassFlag,
		BypassName:         bypassName(f.Header.BypassFlag),
		ControlCommandFlag: f.Header.ControlCommandFlag,
		ControlCommandName: controlName(f.Header.ControlCommandFlag),
		SpacecraftID:       f.Header.SpacecraftID,
		VirtualChannelID:   f.Header.VirtualChannelID,
		FrameLength:        f.Header.FrameLength,
		FrameSequenceNum:   f.Header.FrameSequenceNum,
		MCID:               f.Header.MCID(),
		GVCID:              f.Header.GVCID(),
		DataField:          hex.EncodeToString(f.DataField),
		FEC:                fmt.Sprintf("%04x", f.FrameErrorControl),
		IsBypass:           tcdl.IsBypass(f),
		IsControl:          tcdl.IsControlFrame(f),
	}
}

func bypassName(b uint8) string {
	if b == 1 {
		return "Type-B (expedited)"
	}
	return "Type-A (sequence-controlled)"
}

func controlName(c uint8) string {
	if c == 1 {
		return "Control Command"
	}
	return "Data"
}

func tcDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a TC Transfer Frame",
		Long:  "Decode a binary or hex-encoded TC Transfer Frame, verifying CRC and printing header fields and data.",
		Example: `  # Decode from hex stdin
  astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc decode --input hex

  # Decode with JSON output
  astro tc decode --input hex --format json frame.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := tcdl.DecodeTCTransferFrame(data)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			return printTCFrame(cmd.OutOrStdout(), frame, data, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")

	return cmd
}

func tcEncodeCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		dataHex   string
		bypass    bool
		control   bool
		seqNum    uint8
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct a TC Transfer Frame from fields",
		Long:  "Build a CCSDS TC Transfer Frame from header fields and data. CRC is computed automatically.",
		Example: `  # Encode a basic TC frame
  astro tc encode --scid 26 --vcid 1 --data 0102030405

  # Encode a Type-B (bypass/expedited) frame
  astro tc encode --scid 26 --vcid 1 --data 0102030405 --bypass

  # Encode with sequence number
  astro tc encode --scid 26 --vcid 1 --data 0102030405 --seq-num 42

  # Encode with JSON output
  astro tc encode --scid 26 --vcid 1 --data 0102030405 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userData, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			var opts []tcdl.FrameOption
			if bypass {
				opts = append(opts, tcdl.WithBypass())
			}
			if control {
				opts = append(opts, tcdl.WithControlCommand())
			}
			if cmd.Flags().Changed("seq-num") {
				opts = append(opts, tcdl.WithSequenceNumber(seqNum))
			}

			frame, err := tcdl.NewTCTransferFrame(scid, vcid, userData, opts...)
			if err != nil {
				return fmt.Errorf("building frame: %w", err)
			}

			encoded, err := frame.Encode()
			if err != nil {
				return fmt.Errorf("encoding frame: %w", err)
			}

			return printTCFrame(cmd.OutOrStdout(), frame, encoded, outputFmt)
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-1023)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data field as hex string")
	cmd.Flags().BoolVar(&bypass, "bypass", false, "Set Type-B (expedited) bypass flag")
	cmd.Flags().BoolVar(&control, "control", false, "Set control command flag")
	cmd.Flags().Uint8Var(&seqNum, "seq-num", 0, "Frame sequence number (0-255)")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	_ = cmd.MarkFlagRequired("data")

	return cmd
}

func tcInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print a TC Transfer Frame with hex dump",
		Long:  "Display an annotated breakdown of a TC Transfer Frame showing header fields, data, and hex dump.",
		Example: `  # Inspect from pipe
  astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro tc inspect --input hex

  # Inspect binary file
  astro tc inspect --input bin frame.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := tcdl.DecodeTCTransferFrame(data)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			printTCInspect(cmd.OutOrStdout(), frame, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func printTCFrame(out io.Writer, f *tcdl.TCTransferFrame, raw []byte, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toTCFrameJSON(f), "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(out, string(b))
	case "hex":
		_, _ = fmt.Fprintln(out, hex.EncodeToString(raw))
	case "text":
		_, _ = fmt.Fprintln(out, "TC Transfer Frame:")
		_, _ = fmt.Fprintln(out, "Primary Header:")
		_, _ = fmt.Fprintln(out, f.Header.Humanize())
		_, _ = fmt.Fprintf(out, "  MCID: %d\n", f.Header.MCID())
		_, _ = fmt.Fprintf(out, "  GVCID: %d\n", f.Header.GVCID())
		if f.SegmentHeader != nil {
			_, _ = fmt.Fprintln(out, "Segment Header:")
			_, _ = fmt.Fprintln(out, f.SegmentHeader.Humanize())
		}
		_, _ = fmt.Fprintf(out, "Data Field: %d bytes\n", len(f.DataField))
		_, _ = fmt.Fprintf(out, "FEC: 0x%04X\n", f.FrameErrorControl)
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'json', or 'hex')", format)
	}
	return nil
}

func printTCInspect(out io.Writer, f *tcdl.TCTransferFrame, raw []byte) {
	h := f.Header

	_, _ = fmt.Fprintln(out, "TC Transfer Frame Inspector")
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))

	_, _ = fmt.Fprintln(out, "Primary Header (5 bytes)")
	_, _ = fmt.Fprintf(out, "  Version .............. %d\n", h.VersionNumber)
	_, _ = fmt.Fprintf(out, "  Bypass Flag .......... %d (%s)\n", h.BypassFlag, bypassName(h.BypassFlag))
	_, _ = fmt.Fprintf(out, "  Control Command ...... %d (%s)\n", h.ControlCommandFlag, controlName(h.ControlCommandFlag))
	_, _ = fmt.Fprintf(out, "  Spacecraft ID ........ %d (0x%03X)\n", h.SpacecraftID, h.SpacecraftID)
	_, _ = fmt.Fprintf(out, "  Virtual Channel ID ... %d\n", h.VirtualChannelID)
	_, _ = fmt.Fprintf(out, "  Frame Length ......... %d bytes\n", h.FrameLength+1)
	_, _ = fmt.Fprintf(out, "  Frame Sequence Num ... %d\n", h.FrameSequenceNum)
	_, _ = fmt.Fprintf(out, "  MCID ................. %d\n", h.MCID())
	_, _ = fmt.Fprintf(out, "  GVCID ................ %d\n", h.GVCID())

	if f.SegmentHeader != nil {
		_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
		_, _ = fmt.Fprintln(out, "Segment Header (1 byte)")
		_, _ = fmt.Fprintln(out, f.SegmentHeader.Humanize())
	}

	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
	_, _ = fmt.Fprintf(out, "Data Field (%d bytes)\n", len(f.DataField))
	if len(f.DataField) > 0 {
		_, _ = fmt.Fprint(out, hexDump(f.DataField, "  "))
	}

	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
	_, _ = fmt.Fprintf(out, "Frame Error Control: 0x%04X (CRC-16-CCITT)\n", f.FrameErrorControl)

	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
	_, _ = fmt.Fprintf(out, "Raw Frame (%d bytes)\n", len(raw))
	_, _ = fmt.Fprint(out, hexDump(raw, "  "))
}
