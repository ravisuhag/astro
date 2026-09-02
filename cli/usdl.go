package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/usdl"
	"github.com/spf13/cobra"
)

func usdlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usdl <command>",
		Short: "USLP Transfer Frame operations",
		Long:  "Encode, decode, inspect, and generate USLP Transfer Frames (CCSDS 732.1-B-3).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		usdlDecodeCmd(),
		usdlEncodeCmd(),
		usdlInspectCmd(),
		usdlGenCmd(),
	)

	return cmd
}

// usdlFrameJSON is the JSON-serializable representation of a USLP Transfer Frame.
type usdlFrameJSON struct {
	TFVN             uint8  `json:"tfvn"`
	SpacecraftID     uint16 `json:"spacecraft_id"`
	SourceOrDest     uint8  `json:"source_or_dest"`
	VirtualChannelID uint8  `json:"virtual_channel_id"`
	MAPID            uint8  `json:"map_id"`
	Truncated        bool   `json:"truncated"`
	FrameLength      uint16 `json:"frame_length,omitempty"`
	BypassSeqCtrl    bool   `json:"bypass_seq_ctrl,omitempty"`
	ProtCtrlCmd      bool   `json:"protocol_ctrl_cmd,omitempty"`
	OCFFlag          bool   `json:"ocf_flag,omitempty"`
	VCFCountLen      uint8  `json:"vcf_count_len,omitempty"`
	VCFCount         uint64 `json:"vcf_count,omitempty"`
	ConstructionRule uint8  `json:"construction_rule"`
	UPID             uint8  `json:"upid"`
	Pointer          uint16 `json:"pointer,omitempty"`
	MCID             uint32 `json:"mcid"`
	GVCID            uint32 `json:"gvcid"`
	InsertZone       string `json:"insert_zone,omitempty"`
	DataField        string `json:"data_field"`
	OCF              string `json:"ocf,omitempty"`
	FECF             string `json:"fecf,omitempty"`
	IsIdle           bool   `json:"is_idle"`
}

func toUSDLFrameJSON(f *usdl.TransferFrame) usdlFrameJSON {
	j := usdlFrameJSON{
		TFVN:             f.Header.TFVN,
		SpacecraftID:     f.Header.SCID,
		SourceOrDest:     f.Header.SourceOrDest,
		VirtualChannelID: f.Header.VCID,
		MAPID:            f.Header.MAPID,
		Truncated:        f.Header.EndOfFPH,
		ConstructionRule: f.DataFieldHeader.ConstructionRule,
		UPID:             f.DataFieldHeader.UPID,
		MCID:             f.Header.MCID(),
		GVCID:            f.Header.GVCID(),
		DataField:        hex.EncodeToString(f.DataField),
		FECF:             hex.EncodeToString(f.FECF),
		IsIdle:           usdl.IsIdleFrame(f),
	}
	if !f.Header.EndOfFPH {
		j.FrameLength = f.Header.FrameLength
		j.BypassSeqCtrl = f.Header.BypassSeqCtrl
		j.ProtCtrlCmd = f.Header.ProtCtrlCmd
		j.OCFFlag = f.Header.OCFFlag
		j.VCFCountLen = f.Header.VCFCountLen
		j.VCFCount = f.Header.VCFCount
	}
	if f.DataFieldHeader.HasPointer() {
		j.Pointer = f.DataFieldHeader.Pointer
	}
	if len(f.InsertZone) > 0 {
		j.InsertZone = hex.EncodeToString(f.InsertZone)
	}
	if len(f.OCF) > 0 {
		j.OCF = hex.EncodeToString(f.OCF)
	}
	return j
}

// usdlFECSize maps the decode flags to the managed FECF size. The USLP
// FECF, when present, is always the 16-bit CRC (CCSDS 732.1-B-3
// Clause 4.1.6.2.2).
func usdlFECSize(noFECF bool) int {
	if noFECF {
		return 0
	}
	return usdl.FECSize16
}

func usdlDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string
	var noFECF bool
	var insertZoneLen int

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a USLP Transfer Frame",
		Long:  "Decode a binary or hex-encoded USLP Transfer Frame, printing its header fields and data. OCF presence is signaled in-band by the OCF flag.",
		Example: `  # Decode from hex stdin
  echo "c00640..." | astro usdl decode --input hex

  # Decode a frame without a FECF
  astro usdl decode --input hex --no-fecf < frame.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := usdl.DecodeTransferFrame(data, usdlFECSize(noFECF), insertZoneLen)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			return printUSDLFrame(frame, data, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	cmd.Flags().BoolVar(&noFECF, "no-fecf", false, "Frame carries no FECF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")

	return cmd
}

func usdlEncodeCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		mapid     uint8
		dataHex   string
		ocfHex    string
		vcfLen    uint8
		vcfCount  uint64
		rule      uint8
		upid      uint8
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct a USLP Transfer Frame from fields",
		Long:  "Build a USLP Transfer Frame from header fields and data. Frame length, OCF flag, and FECF are computed automatically.",
		Example: `  # Encode a basic USLP frame
  astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405

  # Encode with an OCF
  astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 --ocf 00000000

  # Encode with a 2-octet VCF count
  astro usdl encode --scid 100 --vcid 1 --data 0102 --vcf-len 2 --vcf-count 42`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userData, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			opts := []usdl.FrameOption{
				usdl.WithConstructionRule(rule),
				usdl.WithUPID(upid),
			}
			if vcfLen > 0 {
				opts = append(opts, usdl.WithVCFCount(vcfLen, vcfCount))
			}

			if ocfHex != "" {
				ocf, err := hex.DecodeString(ocfHex)
				if err != nil {
					return fmt.Errorf("decoding --ocf hex: %w", err)
				}
				if len(ocf) != usdl.OCFSize {
					return fmt.Errorf("OCF must be exactly %d bytes, got %d", usdl.OCFSize, len(ocf))
				}
				opts = append(opts, usdl.WithOCF(ocf))
			}

			frame, err := usdl.NewTransferFrame(scid, vcid, mapid, userData, opts...)
			if err != nil {
				return fmt.Errorf("building frame: %w", err)
			}

			encoded, err := frame.Encode()
			if err != nil {
				return fmt.Errorf("encoding frame: %w", err)
			}

			return printUSDLFrame(frame, encoded, outputFmt)
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-65535)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().Uint8Var(&mapid, "mapid", 0, "MAP ID (0-15)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data field as hex string")
	cmd.Flags().StringVar(&ocfHex, "ocf", "", "Operational Control Field as hex string (4 bytes)")
	cmd.Flags().Uint8Var(&vcfLen, "vcf-len", 0, "VCF count field length in octets (0-7)")
	cmd.Flags().Uint64Var(&vcfCount, "vcf-count", 0, "VCF count value")
	cmd.Flags().Uint8Var(&rule, "rule", usdl.RuleNoSegmentation, "TFDZ construction rule (0-7)")
	cmd.Flags().Uint8Var(&upid, "upid", usdl.UPIDSpacePackets, "USLP protocol identifier (0-31)")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	_ = cmd.MarkFlagRequired("data")

	return cmd
}

func usdlInspectCmd() *cobra.Command {
	var inputFmt string
	var noFECF bool
	var insertZoneLen int

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print a USLP Transfer Frame with hex dump",
		Long:  "Display an annotated breakdown of a USLP Transfer Frame showing header fields, data regions, and hex dump.",
		Example: `  # Inspect from hex stdin
  astro usdl encode --scid 100 --vcid 1 --mapid 0 --data 0102030405 | astro usdl inspect --input hex

  # Inspect binary file
  astro usdl inspect --input bin frame.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := usdl.DecodeTransferFrame(data, usdlFECSize(noFECF), insertZoneLen)
			if err != nil {
				return fmt.Errorf("decoding frame: %w", err)
			}

			printUSDLInspect(frame, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().BoolVar(&noFECF, "no-fecf", false, "Frame carries no FECF")
	cmd.Flags().IntVar(&insertZoneLen, "insert-len", 0, "Insert zone length in bytes")

	return cmd
}

func usdlGenCmd() *cobra.Command {
	var (
		scid      uint16
		vcid      uint8
		mapid     uint8
		count     int
		dataSize  int
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate synthetic USLP Transfer Frames",
		Long:  "Generate a stream of synthetic USLP Transfer Frames with incrementing VCF counts and random data.",
		Example: `  # Generate 10 USLP frames
  astro usdl gen --scid 100 --vcid 1 --mapid 0 --count 10 --data-size 64

  # Generate hex output
  astro usdl gen --scid 100 --vcid 1 --count 5 --data-size 32 --format hex`,
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

				opts := []usdl.FrameOption{
					usdl.WithVCFCount(2, uint64(i)&0xFFFF),
				}

				frame, err := usdl.NewTransferFrame(scid, vcid, mapid, data, opts...)
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

			fmt.Fprintf(os.Stderr, "Generated %d frame(s), SCID=%d VCID=%d MAP=%d, %d bytes each\n",
				count, scid, vcid, mapid, frameSize)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (0-65535)")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID (0-63)")
	cmd.Flags().Uint8Var(&mapid, "mapid", 0, "MAP ID (0-15)")
	cmd.Flags().IntVar(&count, "count", 10, "Number of frames to generate")
	cmd.Flags().IntVar(&dataSize, "data-size", 64, "Data field size in bytes per frame")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")

	return cmd
}

// printUSDLFrame outputs a decoded USLP frame in the specified format.
func printUSDLFrame(f *usdl.TransferFrame, raw []byte, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toUSDLFrameJSON(f), "", "  ")
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

// printUSDLInspect displays an annotated breakdown of a USLP Transfer Frame.
func printUSDLInspect(f *usdl.TransferFrame, raw []byte) {
	h := f.Header
	dfh := f.DataFieldHeader

	fmt.Println("USLP Transfer Frame Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// Primary Header
	fmt.Printf("Primary Header (%d bytes)\n", h.Size())
	fmt.Printf("  TFVN ................. %d (0b%04b)\n", h.TFVN, h.TFVN)
	fmt.Printf("  Spacecraft ID ........ %d (0x%04X)\n", h.SCID, h.SCID)
	srcDst := "Source"
	if h.SourceOrDest == 1 {
		srcDst = "Destination"
	}
	fmt.Printf("  Source/Dest .......... %s\n", srcDst)
	fmt.Printf("  Virtual Channel ID ... %d\n", h.VCID)
	fmt.Printf("  MAP ID ............... %d\n", h.MAPID)
	fmt.Printf("  Truncated Header ..... %v\n", h.EndOfFPH)
	if !h.EndOfFPH {
		fmt.Printf("  Frame Length ......... %d bytes\n", int(h.FrameLength)+1)
		fmt.Printf("  Bypass/Seq Ctrl ...... %v\n", h.BypassSeqCtrl)
		fmt.Printf("  Protocol Ctrl Cmd .... %v\n", h.ProtCtrlCmd)
		fmt.Printf("  OCF Flag ............. %v\n", h.OCFFlag)
		fmt.Printf("  VCF Count Length ..... %d\n", h.VCFCountLen)
		if h.VCFCountLen > 0 {
			fmt.Printf("  VCF Count ............ %d\n", h.VCFCount)
		}
	}
	fmt.Printf("  MCID ................. %d\n", h.MCID())
	fmt.Printf("  GVCID ................ %d\n", h.GVCID())

	// Insert Zone
	if len(f.InsertZone) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Insert Zone (%d bytes)\n", len(f.InsertZone))
		fmt.Print(hexDump(f.InsertZone, "  "))
	}

	// Data Field Header
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("TFDF Header (%d bytes)\n", dfh.Size())
	fmt.Printf("  Construction Rule .... %d\n", dfh.ConstructionRule)
	fmt.Printf("  UPID ................. %d\n", dfh.UPID)
	if dfh.HasPointer() {
		fmt.Printf("  FHP/LVO Pointer ...... %d (0x%04X)\n", dfh.Pointer, dfh.Pointer)
	}

	if usdl.IsIdleFrame(f) {
		fmt.Println("  [IDLE FRAME]")
	}

	// Data Field
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Data Zone (%d bytes)\n", len(f.DataField))
	if len(f.DataField) > 0 {
		fmt.Print(hexDump(f.DataField, "  "))
	}

	// OCF
	if len(f.OCF) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Operational Control Field (4 bytes): %s\n", hex.EncodeToString(f.OCF))
	}

	// FECF
	if len(f.FECF) > 0 {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Frame Error Control: 0x%s (CRC-16-CCITT)\n", hex.EncodeToString(f.FECF))
	}

	// Full hex dump
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw Frame (%d bytes)\n", len(raw))
	fmt.Print(hexDump(raw, "  "))
}
