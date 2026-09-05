package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ravisuhag/astro/pkg/pxdl"
	"github.com/spf13/cobra"
)

func pxdlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pxdl <command>",
		Short: "Proximity-1 data link operations",
		Long:  "Encode, decode and inspect Proximity-1 transfer frames and supervisory protocol data units (CCSDS 211.0-B-6).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		pxdlEncodeCmd(),
		pxdlDecodeCmd(),
		pxdlSPDUCmd(),
	)
	return cmd
}

func pxdlEncodeCmd() *cobra.Command {
	var (
		outputFmt string
		scid      uint16
		portID    uint8
		dataHex   string
		seqNum    uint8
		pcid      uint8
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Build a Proximity-1 transfer frame",
		Long:  "Construct a Proximity-1 transfer frame from header fields and data.",
		Example: `  # A frame on port 1
  astro pxdl encode --scid 42 --port 1 --data 0102030405

  # With a sequence number, for a sequence-controlled frame
  astro pxdl encode --scid 42 --port 1 --data 0102 --seq 7`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			opts := []pxdl.FrameOption{
				pxdl.WithSequenceNumber(seqNum),
				pxdl.WithPCID(pcid),
			}

			frame, err := pxdl.NewTransferFrame(scid, portID, data, opts...)
			if err != nil {
				return fmt.Errorf("building the frame: %w", err)
			}
			encoded, err := frame.Encode()
			if err != nil {
				return fmt.Errorf("encoding the frame: %w", err)
			}

			return printPXDLFrame(cmd.OutOrStdout(), frame, encoded, outputFmt)
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")
	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID (10 bits)")
	cmd.Flags().Uint8Var(&portID, "port", 0, "Port ID (3 bits)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Frame data as hex")
	cmd.Flags().Uint8Var(&seqNum, "seq", 0, "Frame sequence number")
	cmd.Flags().Uint8Var(&pcid, "pcid", 0, "Physical Channel ID (1 bit)")

	_ = cmd.MarkFlagRequired("data")
	return cmd
}

func pxdlDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a Proximity-1 transfer frame",
		Long:  "Decode a Proximity-1 transfer frame and print its header fields and data.",
		Example: `  # Decode a frame
  astro pxdl decode --input hex < frame.hex

  # Round trip
  astro pxdl encode --scid 42 --port 1 --data 0102 | astro pxdl decode --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := pxdl.DecodeTransferFrame(data)
			if err != nil {
				return fmt.Errorf("decoding the frame: %w", err)
			}

			return printPXDLFrame(cmd.OutOrStdout(), frame, data, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	return cmd
}

func pxdlSPDUCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "spdu [file]",
		Short: "Decode supervisory protocol data units",
		Long: "Decode the Supervisory Protocol Data Units a supervisory frame carries: the PLCW that reports link status, and the variable-length SPDUs that carry directives.\n\n" +
			"Several SPDUs can share one frame, so this decodes all of them and reports each.",
		Example: `  # Decode the SPDUs from a supervisory frame's data field
  astro pxdl spdu --input hex < spdus.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			spdus, err := pxdl.DecodeSPDUs(data)
			if err != nil {
				return fmt.Errorf("decoding the SPDUs: %w", err)
			}
			if len(spdus) == 0 {
				return fmt.Errorf("no SPDUs in %d octet(s)", len(data))
			}

			out := cmd.OutOrStdout()
			switch outputFmt {
			case "json":
				rows := make([]spduJSON, 0, len(spdus))
				for _, spdu := range spdus {
					rows = append(rows, spduJSON{
						Kind:    fmt.Sprintf("%T", spdu),
						Summary: humanizeOrEmpty(spdu),
					})
				}
				b, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Fprintln(out, string(b))
			case "text":
				fmt.Fprintf(out, "%d SPDU(s) in %d octet(s)\n", len(spdus), len(data))
				for i, spdu := range spdus {
					fmt.Fprintln(out, strings.Repeat("─", 60))
					fmt.Fprintf(out, "SPDU #%d\n", i+1)
					if summary := humanizeOrEmpty(spdu); summary != "" {
						fmt.Fprintln(out, summary)
					} else {
						fmt.Fprintf(out, "  %T\n", spdu)
					}
				}
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	return cmd
}

type spduJSON struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary,omitempty"`
}

func printPXDLFrame(out io.Writer, frame *pxdl.TransferFrame, raw []byte, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(pxdlFrameJSON{
			SpacecraftID: frame.Header.SCID,
			PortID:       frame.Header.PortID,
			Data:         hex.EncodeToString(frame.DataField),
			TotalOctets:  len(raw),
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding JSON output: %w", err)
		}
		fmt.Fprintln(out, string(b))
	case "hex":
		fmt.Fprintln(out, hex.EncodeToString(raw))
	case "text":
		fmt.Fprintln(out, frame.Humanize())
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'json', or 'hex')", format)
	}
	return nil
}

type pxdlFrameJSON struct {
	SpacecraftID uint16 `json:"spacecraft_id"`
	PortID       uint8  `json:"port_id"`
	Data         string `json:"data"`
	TotalOctets  int    `json:"total_octets"`
}
