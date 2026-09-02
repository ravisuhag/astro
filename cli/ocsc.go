package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravisuhag/astro/pkg/ocsc"
	"github.com/spf13/cobra"
)

func ocscCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ocsc <command>",
		Short: "Optical communications coding and synchronisation",
		Long: "Condition frames into SCPPM codeblocks and apply the randomiser (CCSDS 142.0-B-1).\n\n" +
			"This layer works in bits, not octets. A codeblock is a bit string whose length depends on the code rate and is not a whole number of octets, so the commands report a bit length alongside the octets and pad the last octet with zeros.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		ocscConditionCmd(),
		ocscRandomizeCmd(),
	)
	return cmd
}

// ocscRate maps the flag to a code rate.
func ocscRate(name string) (ocsc.CodeRate, error) {
	switch strings.TrimSpace(name) {
	case "1/3":
		return ocsc.RateOneThird, nil
	case "1/2":
		return ocsc.RateOneHalf, nil
	case "2/3":
		return ocsc.RateTwoThirds, nil
	default:
		return 0, fmt.Errorf("unknown --rate %q (use '1/3', '1/2', or '2/3')", name)
	}
}

func ocscConditionCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		rateName  string
		frameLen  int
	)

	cmd := &cobra.Command{
		Use:   "condition [file]",
		Short: "Condition frames into codeblocks",
		Long: "Split a stream of fixed-length frames into SCPPM codeblocks at the given code rate.\n\n" +
			"Conditioning is not one frame in, one codeblock out: the conditioner fills each codeblock from the frame stream and holds the remainder until enough arrives, so the codeblock count depends on the total input rather than the frame count.",
		Example: `  # Condition 256-octet frames at rate 1/2
  astro ocsc condition --input bin --frame-len 256 --rate 1/2 frames.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rate, err := ocscRate(rateName)
			if err != nil {
				return err
			}
			if frameLen <= 0 {
				return fmt.Errorf("--frame-len is required and must be positive")
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("no input frames")
			}
			if len(data)%frameLen != 0 {
				return fmt.Errorf(
					"input is %d octets, which is not a whole number of %d-octet frames",
					len(data), frameLen)
			}

			var frames [][]byte
			for offset := 0; offset < len(data); offset += frameLen {
				frames = append(frames, data[offset:offset+frameLen])
			}

			blocks, err := ocsc.Condition(frames, rate)
			if err != nil {
				return fmt.Errorf("conditioning: %w", err)
			}

			return printCodeblocks(blocks, len(frames), rate, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "bin", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, hex, or json")
	cmd.Flags().StringVar(&rateName, "rate", "1/2", "SCPPM code rate: 1/3, 1/2, or 2/3")
	cmd.Flags().IntVar(&frameLen, "frame-len", 0, "Fixed frame length in octets (required)")

	_ = cmd.MarkFlagRequired("frame-len")
	return cmd
}

func ocscRandomizeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		bitLen    int
	)

	cmd := &cobra.Command{
		Use:   "randomize [file]",
		Short: "Apply the randomiser to a codeblock",
		Long: "Exclusive-or a codeblock with the pseudo-randomiser sequence of CCSDS 142.0-B-1.\n\n" +
			"The randomiser is its own inverse, so this command both applies and removes it. --bits gives the codeblock's length in bits, because a codeblock is not a whole number of octets and randomising the padding would corrupt the block.",
		Example: `  # Randomise a codeblock of 2040 bits
  astro ocsc randomize --input hex --bits 2040 < block.hex

  # And back again: the randomiser is its own inverse
  astro ocsc randomize --input hex --bits 2040 < block.hex |
    astro ocsc randomize --input hex --bits 2040`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("no input codeblock")
			}

			if bitLen <= 0 {
				bitLen = len(data) * 8
			}
			if want := (bitLen + 7) / 8; len(data) < want {
				return fmt.Errorf(
					"--bits %d needs %d octets, the input has %d", bitLen, want, len(data))
			}

			block := ocsc.NewBitString(bitLen)
			for i := 0; i < bitLen; i++ {
				bit := (data[i/8] >> (7 - i%8)) & 1
				block.SetBit(i, bit)
			}

			randomized := ocsc.Randomize(block)

			return printCodeblocks([]*ocsc.BitString{randomized}, 0, 0, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, hex, or json")
	cmd.Flags().IntVar(&bitLen, "bits", 0,
		"Codeblock length in bits (default: every bit of the input)")
	return cmd
}

// codeblockJSON is one conditioned codeblock.
type codeblockJSON struct {
	Bits   int    `json:"bits"`
	Octets int    `json:"octets"`
	Data   string `json:"data"`
}

// printCodeblocks renders codeblocks, reporting the bit length alongside the
// octets because a codeblock is not a whole number of octets.
func printCodeblocks(blocks []*ocsc.BitString, frames int, rate ocsc.CodeRate, format string) error {
	switch format {
	case "hex":
		for _, block := range blocks {
			fmt.Println(hex.EncodeToString(block.Bytes()))
		}
	case "json":
		rows := make([]codeblockJSON, 0, len(blocks))
		for _, block := range blocks {
			rows = append(rows, codeblockJSON{
				Bits:   block.Len(),
				Octets: len(block.Bytes()),
				Data:   hex.EncodeToString(block.Bytes()),
			})
		}
		b, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding JSON output: %w", err)
		}
		fmt.Println(string(b))
	case "text":
		if frames > 0 {
			fmt.Printf("%d frame(s) conditioned into %d codeblock(s) at rate %s\n",
				frames, len(blocks), rate)
		}
		for i, block := range blocks {
			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf("Codeblock #%d: %d bits (%d octets, last one zero-padded)\n",
				i+1, block.Len(), len(block.Bytes()))
			fmt.Println(hex.EncodeToString(block.Bytes()))
		}
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'hex', or 'json')", format)
	}
	return nil
}
