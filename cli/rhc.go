package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/rhc"
	"github.com/spf13/cobra"
)

func rhcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rhc <command>",
		Short: "Robust housekeeping compression (POCKET+)",
		Long: "Compress and decompress housekeeping vectors with POCKET+ (CCSDS 124.0-B-1).\n\n" +
			"POCKET+ compresses a stream of equal-length vectors by tracking which bit positions change. It is stateful: each output depends on the ones before it, and a decompressor has to see them in order.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		rhcCompressCmd(),
		rhcDecompressCmd(),
	)
	return cmd
}

// The listing format.
//
// CCSDS 124.0-B-1 defines no file or container format: it specifies the
// coding of one cycle and leaves delivery to whatever carries it, which for
// a real mission is a Space Packet or a frame. But a compressed output is a
// bit string, not an octet string, and its length in bits is not recoverable
// from the octets, so nothing can be decompressed without being told how
// long it is.
//
// These commands therefore write a listing of their own: one line per cycle,
// the bit length then the octets. It is not an interchange format and
// nothing else will read it; it exists so that compress and decompress here
// are inverses and the coder can be exercised end to end.
const rhcListingHelp = "one line per cycle, \"<bits> <hex>\""

func rhcCompressCmd() *cobra.Command {
	var (
		inputFmt     string
		vectorBits   int
		robustness   int
		newMask      int
		sendMask     int
		uncompressed int
	)

	cmd := &cobra.Command{
		Use:   "compress [file]",
		Short: "Compress a stream of housekeeping vectors",
		Long: "Read equal-length vectors and compress each one, writing " + rhcListingHelp + ".\n\n" +
			"The listing is this command's own format, not a CCSDS one: the standard defines no container, and a compressed cycle's length in bits cannot be recovered from its octets, so decompression has to be told it.",
		Example: `  # 64-bit vectors from a binary capture
  astro rhc compress --input bin --vector-bits 64 housekeeping.bin

  # Send the whole mask every 10 cycles, so a receiver can resynchronise
  astro rhc compress --input bin --vector-bits 64 --send-mask 10 housekeeping.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if vectorBits <= 0 {
				return fmt.Errorf("--vector-bits is required and must be positive")
			}

			compressor, err := rhc.NewCompressor(rhc.Config{
				VectorLength:         vectorBits,
				Robustness:           robustness,
				NewMaskInterval:      newMask,
				SendMaskInterval:     sendMask,
				UncompressedInterval: uncompressed,
			})
			if err != nil {
				return fmt.Errorf("building the compressor: %w", err)
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			width := (vectorBits + 7) / 8
			if len(data) == 0 {
				return fmt.Errorf("no input vectors")
			}
			if len(data)%width != 0 {
				return fmt.Errorf(
					"input is %d octets, which is not a whole number of %d-octet vectors at %d bits",
					len(data), width, vectorBits)
			}

			out := cmd.OutOrStdout()
			inBits, outBits := 0, 0

			for offset := 0; offset < len(data); offset += width {
				coded, bitLen, err := compressor.Compress(data[offset : offset+width])
				if err != nil {
					return fmt.Errorf("compressing cycle %d: %w", offset/width, err)
				}
				_, _ = fmt.Fprintf(out, "%d %s\n", bitLen, hex.EncodeToString(coded))

				inBits += vectorBits
				outBits += bitLen
			}

			ratio := float64(inBits) / float64(outBits)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%d cycle(s), %d bits in, %d bits out (%.2fx)\n",
				len(data)/width, inBits, outBits, ratio)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "bin", "Input format: hex or bin")
	cmd.Flags().IntVar(&vectorBits, "vector-bits", 0,
		"Length of every input vector in bits, F: 1 to 65535 (required)")
	cmd.Flags().IntVar(&robustness, "robustness", 0,
		"Minimum required effective robustness, R_t: 0 to 7")
	cmd.Flags().IntVar(&newMask, "new-mask", 0,
		"Set the new mask flag every N cycles (0 never)")
	cmd.Flags().IntVar(&sendMask, "send-mask", 0,
		"Set the send mask flag every N cycles (0 never)")
	cmd.Flags().IntVar(&uncompressed, "uncompressed", 0,
		"Set the uncompressed flag every N cycles (0 never)")

	_ = cmd.MarkFlagRequired("vector-bits")
	return cmd
}

func rhcDecompressCmd() *cobra.Command {
	var (
		outputFmt  string
		vectorBits int
		strict     bool
	)

	cmd := &cobra.Command{
		Use:   "decompress [file]",
		Short: "Decompress a listing back to vectors",
		Long: "Read " + rhcListingHelp + " as written by compress, and recover the vectors.\n\n" +
			"The cycles have to arrive in order and none may be missing: each output depends on the ones before it. A gap is reported rather than guessed past.",
		Example: `  # Round trip
  astro rhc compress --input bin --vector-bits 64 housekeeping.bin |
    astro rhc decompress --vector-bits 64 --format bin > recovered.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if vectorBits <= 0 {
				return fmt.Errorf("--vector-bits is required and must be positive")
			}

			decompressor, err := rhc.NewDecompressor(rhc.Config{
				VectorLength: vectorBits,
				Strict:       strict,
			})
			if err != nil {
				return fmt.Errorf("building the decompressor: %w", err)
			}

			source, closer, err := openListing(args)
			if err != nil {
				return err
			}
			defer func() { _ = closer.Close() }()

			scanner := bufio.NewScanner(source)
			scanner.Buffer(make([]byte, 0, 64*1024), maxStreamUnit)

			out := cmd.OutOrStdout()
			cycle := 0
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}

				bitLen, octets, err := parseListingLine(line)
				if err != nil {
					return fmt.Errorf("cycle %d: %w", cycle, err)
				}

				vector, err := decompressor.Decompress(octets, bitLen)
				if err != nil {
					return fmt.Errorf("decompressing cycle %d: %w", cycle, err)
				}

				switch outputFmt {
				case "bin":
					if _, err := out.Write(vector); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
				case "hex":
					_, _ = fmt.Fprintln(out, hex.EncodeToString(vector))
				default:
					return fmt.Errorf("unknown format: %s (use 'bin' or 'hex')", outputFmt)
				}
				cycle++
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading the listing: %w", err)
			}
			if cycle == 0 {
				return fmt.Errorf("the listing held no cycles")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin or hex")
	cmd.Flags().IntVar(&vectorBits, "vector-bits", 0,
		"Length of every vector in bits, F (required, and must match compress)")
	cmd.Flags().BoolVar(&strict, "strict", false,
		"After a reported loss, accept nothing but an uncompressed output")

	_ = cmd.MarkFlagRequired("vector-bits")
	return cmd
}

// openListing opens the listing file, or stdin. The listing is text, so it
// does not go through the hex decoding openInput does.
func openListing(args []string) (*os.File, closerFunc, error) {
	if len(args) > 0 && args[0] != "-" {
		file, err := os.Open(args[0])
		if err != nil {
			return nil, func() error { return nil }, fmt.Errorf("reading input: %w", err)
		}
		return file, file.Close, nil
	}
	return os.Stdin, func() error { return nil }, nil
}

// closerFunc lets a plain function stand in for an io.Closer.
type closerFunc func() error

func (c closerFunc) Close() error { return c() }

// parseListingLine reads one "<bits> <hex>" line.
func parseListingLine(line string) (bitLen int, octets []byte, err error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return 0, nil, fmt.Errorf("%q is not a %s line", line, rhcListingHelp)
	}

	bitLen, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, nil, fmt.Errorf("bit length %q is not a number", fields[0])
	}
	if bitLen < 0 {
		return 0, nil, fmt.Errorf("bit length %d is negative", bitLen)
	}

	octets, err = hex.DecodeString(fields[1])
	if err != nil {
		return 0, nil, fmt.Errorf("cycle data %q is not hex", fields[1])
	}
	if want := (bitLen + 7) / 8; len(octets) < want {
		return 0, nil, fmt.Errorf(
			"the line claims %d bits but carries only %d octets", bitLen, len(octets))
	}
	return bitLen, octets, nil
}
