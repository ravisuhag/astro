package cli

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/ldc"
	"github.com/spf13/cobra"
)

func ldcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ldc <command>",
		Short: "Lossless data compression (Rice coding)",
		Long: "Compress and decompress sample streams with the CCSDS lossless data compression of CCSDS 121.0-B-3.\n\n" +
			"Samples are whole numbers of a fixed width, not octets: a stream of 12-bit readings from an instrument, say. The commands read and write them big-endian at the width --resolution rounds up to.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		ldcCompressCmd(),
		ldcDecompressCmd(),
		ldcInspectCmd(),
	)
	return cmd
}

// ldcParamsFlags are the coder parameters. CCSDS 121.0-B-3 makes all of them
// mission choices, and the compressed file carries them in its header, so
// decompression does not need them repeated.
type ldcParamsFlags struct {
	blockSize   int
	resolution  uint
	signed      bool
	predictor   string
	refInterval int
	restricted  bool
	wordSize    int
}

func (f *ldcParamsFlags) register(cmd *cobra.Command) {
	defaults := ldc.DefaultParams()

	cmd.Flags().IntVar(&f.blockSize, "block-size", defaults.BlockSize,
		"Samples per block, J: 8, 16, 32 or 64")
	cmd.Flags().UintVar(&f.resolution, "resolution", defaults.Resolution,
		"Bits per input sample, n: 1 to 32")
	cmd.Flags().BoolVar(&f.signed, "signed", defaults.Signed,
		"Samples are two's complement")
	cmd.Flags().StringVar(&f.predictor, "predictor", "unit-delay",
		"Preprocessor: unit-delay, none, or bypass")
	cmd.Flags().IntVar(&f.refInterval, "reference-interval", defaults.ReferenceInterval,
		"Blocks between reference samples, r: 1 to 4096")
	cmd.Flags().BoolVar(&f.restricted, "restricted", defaults.Restricted,
		"Use the restricted code option set (resolution 4 or fewer)")
	cmd.Flags().IntVar(&f.wordSize, "word-size", 1,
		"Output word size in octets, B: 1 to 8")
}

func (f *ldcParamsFlags) params() (ldc.Params, error) {
	built := ldc.DefaultParams()
	built.BlockSize = f.blockSize
	built.Resolution = f.resolution
	built.Signed = f.signed
	built.ReferenceInterval = f.refInterval
	built.Restricted = f.restricted

	switch strings.ToLower(f.predictor) {
	case "unit-delay":
		built.Predictor = ldc.PredictorUnitDelay
	case "none":
		built.Predictor = ldc.PredictorNone
	case "bypass":
		built.Predictor = ldc.PredictorBypass
	default:
		return ldc.Params{}, fmt.Errorf(
			"unknown --predictor %q (use 'unit-delay', 'none', or 'bypass')", f.predictor)
	}

	if err := built.Validate(); err != nil {
		return ldc.Params{}, err
	}
	return built, nil
}

func ldcCompressCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		flags     ldcParamsFlags
	)

	cmd := &cobra.Command{
		Use:   "compress [file]",
		Short: "Compress a sample stream",
		Long: "Read samples and write the self-describing file format of CCSDS 121.0-B-3 section 7: a twelve-octet header carrying every parameter, the coded data, then padding to the output word size.\n\n" +
			"Because the header carries the parameters, decompress needs no flags at all.",
		Example: `  # Compress 8-bit samples
  astro ldc compress --input bin --resolution 8 readings.bin > coded.ldc

  # 12-bit samples, read big-endian in two octets each
  astro ldc compress --input bin --resolution 12 --block-size 16 readings.bin > coded.ldc`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := flags.params()
			if err != nil {
				return err
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			samples, err := samplesFromOctets(data, params.Resolution)
			if err != nil {
				return err
			}

			coded, err := ldc.CompressFile(samples, params, flags.wordSize)
			if err != nil {
				return fmt.Errorf("compressing: %w", err)
			}

			switch outputFmt {
			case "bin":
				if _, err := os.Stdout.Write(coded); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			case "hex":
				fmt.Println(hex.EncodeToString(coded))
			case "text":
				ratio := float64(len(data)) / float64(len(coded))
				fmt.Printf("Compressed %d sample(s), %d octets in, %d octets out (%.2fx)\n",
					len(samples), len(data), len(coded), ratio)
				fmt.Println(hex.EncodeToString(coded))
			default:
				return fmt.Errorf("unknown format: %s (use 'bin', 'hex', or 'text')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "bin", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin, hex, or text")
	flags.register(cmd)
	return cmd
}

func ldcDecompressCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decompress [file]",
		Short: "Decompress a file back to samples",
		Long: "Read a file written by compress and recover the samples, taking every parameter from its header.\n\n" +
			"There are no parameter flags here, and there should not be: guessing at parameters the header already states is how a decompression comes out plausible and wrong.",
		Example: `  # Round trip
  astro ldc compress --input bin --resolution 8 readings.bin |
    astro ldc decompress --input bin --resolution 8 > recovered.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			header, err := ldc.DecodeFileHeader(data)
			if err != nil {
				return fmt.Errorf("reading the file header: %w", err)
			}

			samples, err := ldc.DecompressFile(data)
			if err != nil {
				return fmt.Errorf("decompressing: %w", err)
			}

			octets, err := octetsFromSamples(samples, header.Params.Resolution)
			if err != nil {
				return err
			}

			switch outputFmt {
			case "bin":
				if _, err := os.Stdout.Write(octets); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			case "hex":
				fmt.Println(hex.EncodeToString(octets))
			case "text":
				fmt.Printf("Recovered %d sample(s) at %d bits each\n",
					len(samples), header.Params.Resolution)
				fmt.Println(hex.EncodeToString(octets))
			default:
				return fmt.Errorf("unknown format: %s (use 'bin', 'hex', or 'text')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "bin", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "bin", "Output format: bin, hex, or text")
	return cmd
}

func ldcInspectCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Show a compressed file's header",
		Long:  "Read the twelve-octet header of a compressed file and print the parameters the body was coded with, without decompressing it.",
		Example: `  # What parameters was this coded with?
  astro ldc inspect --input bin coded.ldc`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			header, err := ldc.DecodeFileHeader(data)
			if err != nil {
				return fmt.Errorf("reading the file header: %w", err)
			}

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(ldcHeaderJSON{
					WordSize:          header.WordSize,
					SampleCount:       header.SampleCount,
					BlockSize:         header.Params.BlockSize,
					Resolution:        header.Params.Resolution,
					Signed:            header.Params.Signed,
					Predictor:         predictorName(header.Params.Predictor),
					ReferenceInterval: header.Params.ReferenceInterval,
					Restricted:        header.Params.Restricted,
					TotalOctets:       len(data),
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				fmt.Println(header.Humanize())
				fmt.Printf("  File size .......... %d octets\n", len(data))
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "bin", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	return cmd
}

type ldcHeaderJSON struct {
	WordSize          int    `json:"word_size"`
	SampleCount       uint64 `json:"sample_count"`
	BlockSize         int    `json:"block_size"`
	Resolution        uint   `json:"resolution"`
	Signed            bool   `json:"signed"`
	Predictor         string `json:"predictor"`
	ReferenceInterval int    `json:"reference_interval"`
	Restricted        bool   `json:"restricted"`
	TotalOctets       int    `json:"total_octets"`
}

func predictorName(p ldc.Predictor) string {
	switch p {
	case ldc.PredictorNone:
		return "none"
	case ldc.PredictorUnitDelay:
		return "unit-delay"
	case ldc.PredictorBypass:
		return "bypass"
	default:
		return "unknown"
	}
}

// sampleWidth is how many octets one sample occupies on the way in and out.
//
// The standard codes samples of 1 to 32 bits, but a file of them has to be
// octet-aligned to be a file, so a sample is read from the smallest whole
// number of octets that holds it: one for up to 8 bits, two for up to 16,
// and so on. A 12-bit sample therefore travels in two octets with the top
// four unused.
func sampleWidth(resolution uint) int {
	return int((resolution + 7) / 8)
}

// samplesFromOctets reads big-endian samples of the resolution's width.
func samplesFromOctets(data []byte, resolution uint) ([]uint32, error) {
	width := sampleWidth(resolution)
	if width == 0 {
		return nil, fmt.Errorf("resolution %d is not a usable sample width", resolution)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("no input samples")
	}
	if len(data)%width != 0 {
		return nil, fmt.Errorf(
			"input is %d octets, which is not a whole number of %d-octet samples at %d bits",
			len(data), width, resolution)
	}

	limit := uint64(1)<<resolution - 1
	samples := make([]uint32, 0, len(data)/width)

	for offset := 0; offset < len(data); offset += width {
		var value uint32
		for _, b := range data[offset : offset+width] {
			value = value<<8 | uint32(b)
		}
		// A sample wider than the resolution claims is a mismatch between the
		// flag and the data, and coding it would silently truncate.
		if uint64(value) > limit {
			return nil, fmt.Errorf(
				"sample %d at octet %d is %d, which does not fit %d bits",
				len(samples), offset, value, resolution)
		}
		samples = append(samples, value)
	}
	return samples, nil
}

// octetsFromSamples is the inverse, so a round trip through the two returns
// the original octets.
func octetsFromSamples(samples []uint32, resolution uint) ([]byte, error) {
	width := sampleWidth(resolution)
	if width == 0 {
		return nil, fmt.Errorf("resolution %d is not a usable sample width", resolution)
	}

	out := make([]byte, 0, len(samples)*width)
	var scratch [4]byte

	for _, sample := range samples {
		binary.BigEndian.PutUint32(scratch[:], sample)
		out = append(out, scratch[4-width:]...)
	}
	return out, nil
}
