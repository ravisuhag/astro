package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravisuhag/astro/pkg/tcsc"
	"github.com/spf13/cobra"
)

func cltuCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cltu <command>",
		Short: "Command Link Transmission Unit operations",
		Long:  "Wrap, unwrap, and inspect CCSDS Command Link Transmission Units (CCSDS 231.0-B-4).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		cltuWrapCmd(),
		cltuUnwrapCmd(),
		cltuInspectCmd(),
		cltuGenCmd(),
	)

	return cmd
}

func cltuWrapCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		randomize bool
	)

	cmd := &cobra.Command{
		Use:   "wrap [file]",
		Short: "Wrap a TC frame into a CLTU",
		Long:  "Pad, BCH-encode, and add start/tail sequences to produce a CLTU from TC Transfer Frame data.",
		Example: `  # Wrap a TC frame
  astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex

  # Wrap with randomization
  astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex --randomize`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			cltu, err := tcsc.WrapCLTU(data, nil, nil, randomize)
			if err != nil {
				return fmt.Errorf("wrapping CLTU: %w", err)
			}

			switch outputFmt {
			case "hex":
				fmt.Println(hex.EncodeToString(cltu))
			case "json":
				j := cltuToJSON(cltu, randomize)
				b, err := json.MarshalIndent(j, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			case "text":
				startSeq := tcsc.DefaultStartSequence()
				tailSeq := tcsc.DefaultTailSequence()
				bodyLen := len(cltu) - len(startSeq) - len(tailSeq)
				numBlocks := bodyLen / tcsc.CodeblockBytes
				fmt.Printf("CLTU (%d bytes)\n", len(cltu))
				fmt.Printf("  Start Sequence: %s\n", hex.EncodeToString(cltu[:len(startSeq)]))
				fmt.Printf("  Codeblocks: %d (%d bytes each)\n", numBlocks, tcsc.CodeblockBytes)
				fmt.Printf("  Tail Sequence: %s\n", hex.EncodeToString(cltu[len(cltu)-len(tailSeq):]))
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

func cltuUnwrapCmd() *cobra.Command {
	var (
		inputFmt    string
		outputFmt   string
		derandomize bool
	)

	cmd := &cobra.Command{
		Use:   "unwrap [file]",
		Short: "Unwrap a CLTU to extract the TC frame",
		Long:  "Validate start/tail sequences, BCH-decode codeblocks, and optionally de-randomize to extract TC Transfer Frame data.",
		Example: `  # Unwrap a CLTU
  astro cltu unwrap --input hex cltu.hex

  # Unwrap with de-randomization
  cat cltu.hex | astro cltu unwrap --input hex --derandomize`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, corrections, err := tcsc.UnwrapCLTU(data, nil, nil, derandomize)
			if err != nil {
				return fmt.Errorf("unwrapping CLTU: %w", err)
			}

			switch outputFmt {
			case "hex":
				fmt.Println(hex.EncodeToString(frame))
			case "json":
				j := map[string]any{
					"frame_data":   hex.EncodeToString(frame),
					"frame_bytes":  len(frame),
					"corrections":  corrections,
					"derandomized": derandomize,
				}
				b, err := json.MarshalIndent(j, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
			case "text":
				fmt.Printf("Extracted Frame (%d bytes)\n", len(frame))
				fmt.Printf("  BCH Corrections: %d\n", corrections)
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

func cltuInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a CLTU with annotated breakdown",
		Long:  "Display an annotated breakdown of a CLTU showing start/tail sequences, codeblock boundaries, and BCH parity.",
		Example: `  # Inspect a CLTU
  astro tc encode --scid 26 --vcid 1 --data 0102030405 | astro cltu wrap --input hex | astro cltu inspect --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			printCLTUInspect(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

type cltuJSON struct {
	StartSequence string `json:"start_sequence"`
	TailSequence  string `json:"tail_sequence"`
	NumCodeblocks int    `json:"num_codeblocks"`
	TotalLen      int    `json:"total_bytes"`
	Randomized    bool   `json:"randomized"`
	CLTU          string `json:"cltu"`
}

func cltuToJSON(cltu []byte, randomized bool) cltuJSON {
	startSeq := tcsc.DefaultStartSequence()
	tailSeq := tcsc.DefaultTailSequence()
	bodyLen := len(cltu) - len(startSeq) - len(tailSeq)
	numBlocks := bodyLen / tcsc.CodeblockBytes

	return cltuJSON{
		StartSequence: hex.EncodeToString(cltu[:len(startSeq)]),
		TailSequence:  hex.EncodeToString(cltu[len(cltu)-len(tailSeq):]),
		NumCodeblocks: numBlocks,
		TotalLen:      len(cltu),
		Randomized:    randomized,
		CLTU:          hex.EncodeToString(cltu),
	}
}

func printCLTUInspect(data []byte) {
	startSeq := tcsc.DefaultStartSequence()
	tailSeq := tcsc.DefaultTailSequence()

	fmt.Println("CLTU Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// Start sequence
	if len(data) < len(startSeq) {
		fmt.Println("Data too short for start sequence")
		return
	}

	startMatch := hex.EncodeToString(data[:len(startSeq)]) == hex.EncodeToString(startSeq)
	fmt.Printf("Start Sequence (%d bytes): %s", len(startSeq), hex.EncodeToString(data[:len(startSeq)]))
	if startMatch {
		fmt.Println(" [VALID]")
	} else {
		fmt.Printf(" [MISMATCH — expected %s]\n", hex.EncodeToString(startSeq))
	}

	// Tail sequence
	if len(data) < len(startSeq)+len(tailSeq) {
		fmt.Println("Data too short for tail sequence")
		return
	}

	tailMatch := hex.EncodeToString(data[len(data)-len(tailSeq):]) == hex.EncodeToString(tailSeq)
	fmt.Printf("Tail Sequence (%d bytes): %s", len(tailSeq), hex.EncodeToString(data[len(data)-len(tailSeq):]))
	if tailMatch {
		fmt.Println(" [VALID]")
	} else {
		fmt.Printf(" [MISMATCH — expected %s]\n", hex.EncodeToString(tailSeq))
	}

	// Codeblocks
	body := data[len(startSeq) : len(data)-len(tailSeq)]
	numBlocks := len(body) / tcsc.CodeblockBytes
	remainder := len(body) % tcsc.CodeblockBytes

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Codeblocks: %d (%d bytes each = %d info + 1 parity)\n",
		numBlocks, tcsc.CodeblockBytes, tcsc.InfoBytes)
	if remainder > 0 {
		fmt.Printf("  Warning: %d trailing bytes after codeblocks\n", remainder)
	}

	for i := range numBlocks {
		cb := body[i*tcsc.CodeblockBytes : (i+1)*tcsc.CodeblockBytes]
		info := cb[:tcsc.InfoBytes]
		parity := cb[tcsc.InfoBytes]
		fmt.Printf("  Block %d: info=%s parity=%02x\n", i+1, hex.EncodeToString(info), parity)
	}

	// Full dump
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw CLTU (%d bytes)\n", len(data))
	fmt.Print(hexDump(data, "  "))
}
