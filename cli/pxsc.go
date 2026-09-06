package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ravisuhag/astro/pkg/pxsc"
	"github.com/spf13/cobra"
)

func pxscCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pxsc <command>",
		Short: "Proximity-1 coding and synchronisation",
		Long: "Wrap and unwrap PLTUs, and apply the convolutional code (CCSDS 211.2-B-3).\n\n" +
			"A PLTU is the Proximity-1 equivalent of a CADU: the sync marker 0xFAF320, the transfer frame, then a CRC-32 over the frame.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		pxscWrapCmd(),
		pxscUnwrapCmd(),
		pxscSyncCmd(),
		pxscEncodeCmd(),
		pxscDecodeCmd(),
	)
	return cmd
}

func pxscWrapCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "wrap [file]",
		Short: "Wrap a transfer frame as a PLTU",
		Long:  "Prepend the Proximity-1 sync marker and append the CRC-32 over the frame, producing a PLTU.",
		Example: `  # Wrap a Proximity-1 frame
  astro pxdl encode --scid 42 --port 1 --data 0102 | astro pxsc wrap --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pltu, err := pxsc.WrapPLTU(data)
			if err != nil {
				return fmt.Errorf("wrapping the frame: %w", err)
			}

			out := cmd.OutOrStdout()
			return writeOctets(out, pltu, outputFmt, func() {
				_, _ = fmt.Fprintf(out, "PLTU: %d octets (%d sync marker + %d frame + %d CRC)\n",
					len(pltu), pxsc.ASMSize, len(data), pxsc.CRC32Size)
			})
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, hex, or bin")
	return cmd
}

func pxscUnwrapCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		maxFrame  int
	)

	cmd := &cobra.Command{
		Use:   "unwrap [file]",
		Short: "Extract a transfer frame from a PLTU",
		Long: "Strip the sync marker, verify the CRC-32, and return the transfer frame.\n\n" +
			"A CRC mismatch is an error: the frame is corrupt and passing it on would put bad data into the layer above.",
		Example: `  # Unwrap and decode in one pipeline
  astro pxsc unwrap --input hex < pltu.hex | astro pxdl decode --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frame, err := pxsc.UnwrapPLTUWithLimit(data, maxFrame)
			if err != nil {
				return fmt.Errorf("unwrapping the PLTU: %w", err)
			}

			out := cmd.OutOrStdout()
			return writeOctets(out, frame, outputFmt, func() {
				_, _ = fmt.Fprintf(out, "Frame: %d octets, CRC-32 verified\n", len(frame))
			})
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, hex, or bin")
	cmd.Flags().IntVar(&maxFrame, "max-frame", pxsc.DefaultMaxFrameLength,
		"Largest frame to accept, in octets")
	return cmd
}

func pxscSyncCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "sync [file]",
		Short: "Scan a byte stream for PLTUs",
		Long: "Search a raw stream for the Proximity-1 sync marker and extract the frames whose CRC-32 checks out.\n\n" +
			"Unlike a CADU stream, a PLTU carries no fixed length, so the synchroniser has to try candidate lengths and let the CRC decide. That means a frame is reported only when its checksum agrees, which is what stops a false marker producing a bogus frame.",
		Example: `  # Find every frame in a capture
  astro pxsc sync --input bin capture.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			frames := pxsc.NewSynchronizer().ScanFrames(data)
			out := cmd.OutOrStdout()

			switch outputFmt {
			case "hex":
				for _, frame := range frames {
					_, _ = fmt.Fprintln(out, hex.EncodeToString(frame))
				}
			case "text":
				_, _ = fmt.Fprintf(out, "Found %d frame(s) in %d octet(s)\n", len(frames), len(data))
				for i, frame := range frames {
					_, _ = fmt.Fprintf(out, "--- Frame #%d (%d octets) ---\n  %s\n",
						i+1, len(frame), hex.EncodeToString(frame))
				}
			case "json":
				rows := make([]string, 0, len(frames))
				for _, frame := range frames {
					rows = append(rows, hex.EncodeToString(frame))
				}
				b, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				_, _ = fmt.Fprintln(out, string(b))
			default:
				return fmt.Errorf("unknown format: %s (use 'text', 'hex', or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, hex, or json")
	return cmd
}

// flushOctets is how many zero octets to append so that a decode returns
// exactly the input.
//
// A Viterbi decoder does not commit a bit until it has seen enough of what
// follows to be sure of it: pkg/pxsc looks back five constraint lengths, 35
// bits, before deciding. Without a tail, the last 35 bits of a stream never
// come out, and a round trip silently loses its last few octets, which is
// what this exists to prevent.
//
// Five octets is 40 bits, the smallest whole number of octets that covers 35.
const flushOctets = (5*pxsc.ConstraintLength + 7) / 8

func pxscEncodeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		flush     bool
	)

	cmd := &cobra.Command{
		Use:   "encode [file]",
		Short: "Apply the convolutional code",
		Long: "Encode octets with the rate-1/2 convolutional code of CCSDS 211.2-B-3 clause 3.3: constraint length 7, generators 171 and 133 in octal.\n\n" +
			"Every input bit becomes two output bits, so the output is twice the input. The output here is one octet per code symbol pair, which is how the decoder wants it back.\n\n" +
			"A tail of zero octets is appended by default. A Viterbi decoder holds back the last 35 bits of a stream until it has seen enough of what follows to be sure of them, so without the tail a round trip loses its last few octets. Use --flush=false when you are appending more data yourself.",
		Example: `  # Encode a PLTU for transmission
  astro pxsc wrap --input hex < frame.hex | astro pxsc encode --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("no input to encode")
			}

			input := data
			if flush {
				input = append(append([]byte{}, data...), make([]byte, flushOctets)...)
			}

			symbols := pxsc.NewConvolutionalEncoder().Encode(input)

			out := cmd.OutOrStdout()
			return writeOctets(out, symbols, outputFmt, func() {
				if flush {
					_, _ = fmt.Fprintf(out, "Encoded %d octet(s) plus a %d-octet tail into %d code symbol(s)\n",
						len(data), flushOctets, len(symbols))
					return
				}
				_, _ = fmt.Fprintf(out, "Encoded %d octet(s) into %d code symbol(s)\n", len(data), len(symbols))
			})
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, hex, or bin")
	cmd.Flags().BoolVar(&flush, "flush", true,
		"Append a zero tail so a decode returns exactly this input")
	return cmd
}

func pxscDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode the convolutional code",
		Long: "Decode code symbols with a Viterbi decoder, recovering the original octets.\n\n" +
			"The decoder corrects errors, which is the point of the code: a symbol stream with a few bits flipped still decodes to the right octets.\n\n" +
			"The last 35 bits of a stream are held back until enough of what follows has arrived to decide them. Symbols produced by encode carry a tail for exactly this, so a round trip through the two comes out whole; symbols from elsewhere may be short by a few octets at the end.",
		Example: `  # Round trip through the code
  astro pxsc encode --input hex < frame.hex | astro pxsc decode --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("no symbols to decode")
			}

			decoded, err := pxsc.NewViterbiDecoder().Decode(data)
			if err != nil {
				return fmt.Errorf("decoding the symbols: %w", err)
			}

			out := cmd.OutOrStdout()
			return writeOctets(out, decoded, outputFmt, func() {
				_, _ = fmt.Fprintf(out, "Decoded %d symbol(s) into %d octet(s)\n", len(data), len(decoded))
			})
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, hex, or bin")
	return cmd
}

// writeOctets is the output path shared by the commands whose result is a run
// of octets rather than a set of fields.
//
// summary is called for the text format, before the octets, so each command
// can say what it did in its own terms.
func writeOctets(out io.Writer, data []byte, format string, summary func()) error {
	switch format {
	case "hex":
		_, _ = fmt.Fprintln(out, hex.EncodeToString(data))
	case "bin":
		if _, err := out.Write(data); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	case "text":
		if summary != nil {
			summary()
		}
		_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
		_, _ = fmt.Fprint(out, hexDump(data, "  "))
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'hex', or 'bin')", format)
	}
	return nil
}
