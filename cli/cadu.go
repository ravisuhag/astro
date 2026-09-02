package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/tmsc"
	"github.com/spf13/cobra"
)

func caduCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cadu <command>",
		Short: "Channel Access Data Unit operations",
		Long:  "Wrap, unwrap, inspect, and sync CCSDS Channel Access Data Units (CCSDS 131.0-B-4).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		caduWrapCmd(),
		caduUnwrapCmd(),
		caduInspectCmd(),
		caduSyncCmd(),
		caduGenCmd(),
	)

	return cmd
}

func caduWrapCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		randomize bool
	)

	cmd := &cobra.Command{
		Use:   "wrap [file]",
		Short: "Wrap a TM frame into a CADU",
		Long:  "Prepend the Attached Sync Marker and optionally apply CCSDS pseudo-randomization to produce a CADU.",
		Example: `  # Wrap a TM frame (hex input)
  astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex

  # Wrap with randomization
  astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex --randomize`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			cadu := tmsc.WrapCADU(data, nil, randomize)

			switch outputFmt {
			case "hex":
				fmt.Println(hex.EncodeToString(cadu))
			case "json":
				j := caduToJSON(cadu, randomize)
				b, err := json.MarshalIndent(j, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			case "text":
				fmt.Printf("CADU (%d bytes)\n", len(cadu))
				fmt.Printf("  ASM: %s\n", hex.EncodeToString(cadu[:4]))
				fmt.Printf("  Frame Data: %d bytes\n", len(cadu)-4)
				fmt.Printf("  Randomized: %v\n", randomize)
			default:
				return fmt.Errorf("unknown format: %s", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")
	cmd.Flags().BoolVar(&randomize, "randomize", false, "Apply CCSDS pseudo-randomization")

	return cmd
}

func caduUnwrapCmd() *cobra.Command {
	var (
		inputFmt    string
		outputFmt   string
		derandomize bool
	)

	cmd := &cobra.Command{
		Use:   "unwrap [file]",
		Short: "Unwrap a CADU to extract the TM frame",
		Long:  "Strip the Attached Sync Marker and optionally de-randomize to extract the TM Transfer Frame data.",
		Example: `  # Unwrap a CADU
  astro cadu unwrap --input hex cadu.hex

  # Unwrap with de-randomization
  cat cadu.hex | astro cadu unwrap --input hex --derandomize`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := tmsc.UnwrapCADU(data, nil, derandomize)
			if err != nil {
				return fmt.Errorf("unwrapping CADU: %w", err)
			}

			switch outputFmt {
			case "hex":
				fmt.Println(hex.EncodeToString(frame))
			case "json":
				j := map[string]any{
					"frame_data":   hex.EncodeToString(frame),
					"frame_bytes":  len(frame),
					"derandomized": derandomize,
				}
				b, err := json.MarshalIndent(j, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			case "text":
				fmt.Printf("Extracted Frame (%d bytes)\n", len(frame))
				fmt.Printf("  Derandomized: %v\n", derandomize)
				fmt.Print(hexDump(frame, "  "))
			default:
				return fmt.Errorf("unknown format: %s", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")
	cmd.Flags().BoolVar(&derandomize, "derandomize", false, "Apply CCSDS de-randomization")

	return cmd
}

func caduInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a CADU with annotated breakdown",
		Long:  "Display an annotated breakdown of a CADU showing the ASM, frame data, and randomization state.",
		Example: `  # Inspect a CADU
  astro tm encode --scid 26 --vcid 1 --data 0102030405 | astro cadu wrap --input hex | astro cadu inspect --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			printCADUInspect(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func caduSyncCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		frameLen  int
	)

	cmd := &cobra.Command{
		Use:   "sync [file]",
		Short: "Scan a byte stream for ASM markers and extract CADUs",
		Long:  "Scan a raw byte stream for CCSDS Attached Sync Markers (0x1ACFFC1D), extract aligned CADUs of the given frame length.",
		Example: `  # Sync and extract CADUs from binary stream
  astro cadu sync --input bin --frame-len 1115 capture.bin

  # Sync from hex with JSON output
  astro cadu sync --input hex --frame-len 17 stream.hex --format json`,
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

			return syncCADUs(source, frameLen, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Total CADU length in bytes including ASM (required)")

	_ = cmd.MarkFlagRequired("frame-len")

	return cmd
}

type caduJSON struct {
	ASM       string `json:"asm"`
	FrameData string `json:"frame_data"`
	TotalLen  int    `json:"total_bytes"`
	Randomize bool   `json:"randomized"`
}

func caduToJSON(cadu []byte, randomized bool) caduJSON {
	return caduJSON{
		ASM:       hex.EncodeToString(cadu[:4]),
		FrameData: hex.EncodeToString(cadu[4:]),
		TotalLen:  len(cadu),
		Randomize: randomized,
	}
}

func printCADUInspect(data []byte) {
	asm := tmsc.DefaultASM()

	fmt.Println("CADU Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// ASM
	if len(data) >= 4 {
		asmMatch := bytes.Equal(data[:4], asm)
		fmt.Printf("Attached Sync Marker (4 bytes): %s", hex.EncodeToString(data[:4]))
		if asmMatch {
			fmt.Println(" [VALID]")
		} else {
			fmt.Println(" [MISMATCH — expected 1acffc1d]")
		}
	} else {
		fmt.Println("Data too short for ASM")
		return
	}

	// Frame data
	frameData := data[4:]
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Frame Data (%d bytes)\n", len(frameData))
	if len(frameData) > 0 {
		fmt.Print(hexDump(frameData, "  "))
	}

	// Full dump
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw CADU (%d bytes)\n", len(data))
	fmt.Print(hexDump(data, "  "))
}

// syncCADUs finds frame alignment in a raw stream and extracts each CADU.
//
// A CADU stream carries no length field. CCSDS 131.0-B-5 attaches the sync
// marker ahead of every codeblock and the receiver acquires alignment by
// searching for it, so this searches too, and picks up again at the next
// marker when alignment is lost. The reading is incremental: a live pipe
// never reaches EOF, and a pass-length capture should not have to be resident
// to be scanned.
func syncCADUs(source io.Reader, frameLen int, outputFmt string) error {
	asm := tmsc.DefaultASM()

	found := 0
	var skipped int64

	handle := func(cadu []byte, offset int64) error {
		found++

		switch outputFmt {
		case "json":
			b, err := json.Marshal(map[string]any{
				"index":  found,
				"offset": offset,
				"asm":    hex.EncodeToString(cadu[:len(asm)]),
				"cadu":   hex.EncodeToString(cadu),
				"length": frameLen,
			})
			if err != nil {
				return fmt.Errorf("encoding JSON output: %w", err)
			}
			fmt.Println(string(b))
		case "hex":
			fmt.Println(hex.EncodeToString(cadu))
		case "text":
			fmt.Printf("--- CADU #%d (offset %d, %d bytes) ---\n", found, offset, frameLen)
			fmt.Printf("  ASM: %s\n", hex.EncodeToString(cadu[:len(asm)]))
			fmt.Printf("  Frame: %d bytes\n", frameLen-len(asm))
		}

		return nil
	}

	// Octets between CADUs are not an error. A capture normally begins part
	// way through a frame, and there is noise between passes.
	noise := func(offset int64, n int) {
		skipped += int64(n)
	}

	truncated := func(offset int64, n int) {
		fmt.Fprintf(os.Stderr,
			"Warning: sync marker at offset %d but only %d of %d octets follow, ignored\n",
			offset, n, frameLen)
	}

	if err := streamMarked(source, asm, frameLen, handle, noise, truncated); err != nil {
		return err
	}

	if outputFmt == "text" {
		fmt.Printf("\nFound %d CADU(s).\n", found)
		if skipped > 0 {
			fmt.Printf("%d octet(s) outside any CADU were skipped.\n", skipped)
		}
	}

	return nil
}
