package cli

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/sdls"
	"github.com/spf13/cobra"
)

func sdlsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdls <command>",
		Short: "Space Data Link Security",
		Long: "Protect a frame data field, check and recover one, or read its Security Header (CCSDS 355.0-B-2).\n\n" +
			"apply and process need the Security Association's key. They read it from a file named by --key-file and never take it as a flag value, because a key on the command line lands in shell history and in the process table where anyone on the machine can read it.\n\n" +
			"Each run is one frame. A long-running sender keeps its counters in memory; a command does not, so apply asks for the last IV or sequence number sent under the key and reports the new one. Reusing an IV under an AES-GCM key breaks it, so keep that value somewhere safe between runs.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(sdlsInspectCmd(), sdlsApplyCmd(), sdlsProcessCmd())
	return cmd
}

// sdlsSAFlags are the Security Association settings apply and process share.
type sdlsSAFlags struct {
	keyFile   string
	spi       uint16
	mode      string
	authAlg   string
	ivLen     int
	seqLen    int
	padLen    int
	macLen    int
	frameHdr  string
	authMask  string
	inputFmt  string
	outputFmt string
}

func (f *sdlsSAFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.keyFile, "key-file", "", "File holding the 32-octet AES-256 key, raw or as hex (required)")
	cmd.Flags().Uint16Var(&f.spi, "spi", 0, "Security Parameter Index (required; 0 and 65535 are reserved)")
	cmd.Flags().StringVar(&f.mode, "mode", "aead", "Service type: aead (AES-GCM authenticated encryption) or auth (authentication only)")
	cmd.Flags().StringVar(&f.authAlg, "auth-alg", "gmac", "Authentication algorithm when --mode auth: gmac or cmac")
	cmd.Flags().IntVar(&f.ivLen, "iv", sdls.GCMIVSize, "Initialisation vector length in octets (0 for cmac)")
	cmd.Flags().IntVar(&f.seqLen, "seq", 0, "Anti-replay sequence number length in octets")
	cmd.Flags().IntVar(&f.padLen, "pad", 0, "Pad length field width in octets")
	cmd.Flags().IntVar(&f.macLen, "mac", sdls.MaxMACSize, "Message authentication code length in octets")
	cmd.Flags().StringVar(&f.frameHdr, "frame-header", "", "Frame header octets in hex, authenticated but not encrypted")
	cmd.Flags().StringVar(&f.authMask, "auth-mask", "", "Authentication bit mask in hex (default: authenticate every header octet)")
	cmd.Flags().StringVar(&f.inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&f.outputFmt, "format", "hex", "Output format: hex, bin, or text")
	_ = cmd.MarkFlagRequired("key-file")
	_ = cmd.MarkFlagRequired("spi")
}

// association builds the SA the flags describe. warn receives a note when the
// key file can be read by other users.
func (f *sdlsSAFlags) association(warn io.Writer) (*sdls.SecurityAssociation, []byte, error) {
	key, err := readKeyFile(f.keyFile, warn)
	if err != nil {
		return nil, nil, err
	}

	sa := &sdls.SecurityAssociation{
		SPI: f.spi,
		Key: key,
		FieldLengths: sdls.FieldLengths{
			IV:     f.ivLen,
			SeqNum: f.seqLen,
			PadLen: f.padLen,
			MAC:    f.macLen,
		},
	}
	switch f.mode {
	case "aead":
		sa.Mode = sdls.AuthenticatedEncryption
	case "auth":
		sa.Mode = sdls.Authentication
	default:
		return nil, nil, fmt.Errorf("unknown mode: %s (use 'aead' or 'auth')", f.mode)
	}
	switch f.authAlg {
	case "gmac":
		sa.AuthAlgorithm = sdls.AuthGMAC
	case "cmac":
		sa.AuthAlgorithm = sdls.AuthCMAC
	default:
		return nil, nil, fmt.Errorf("unknown auth algorithm: %s (use 'gmac' or 'cmac')", f.authAlg)
	}
	if f.authMask != "" {
		if sa.AuthMask, err = hex.DecodeString(f.authMask); err != nil {
			return nil, nil, fmt.Errorf("decoding --auth-mask: %w", err)
		}
	}
	if err := sa.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid Security Association: %w", err)
	}

	var frameHeader []byte
	if f.frameHdr != "" {
		if frameHeader, err = hex.DecodeString(f.frameHdr); err != nil {
			return nil, nil, fmt.Errorf("decoding --frame-header: %w", err)
		}
	}
	return sa, frameHeader, nil
}

// readKeyFile loads an AES-256 key: exactly 32 raw octets, or 64 hex digits
// with optional surrounding whitespace.
func readKeyFile(path string, warn io.Writer) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading the key file: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		_, _ = fmt.Fprintf(warn, "warning: %s can be read by other users; chmod 600 it\n", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the key file: %w", err)
	}
	if len(raw) == sdls.AESKeySize {
		return raw, nil
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != sdls.AESKeySize {
		return nil, fmt.Errorf("the key file must hold %d raw octets or %d hex digits", sdls.AESKeySize, 2*sdls.AESKeySize)
	}
	return key, nil
}

func sdlsApplyCmd() *cobra.Command {
	var (
		sa      sdlsSAFlags
		lastIV  string
		lastSeq string
	)

	cmd := &cobra.Command{
		Use:   "apply [file]",
		Short: "Protect a frame data field",
		Long: "Apply security to one Transfer Frame Data Field and write Security Header, protected data and Security Trailer, ready to place in the carrier frame.\n\n" +
			"The SA sends IV and sequence number values counting up from one, and a command keeps no state between runs. So when the SA has an IV, --last-iv is required: the last IV sent under this key, all zeros for a key that has sent nothing. The same goes for --last-seq when it has a sequence number. The new values go to stderr; save them for the next run. Sending an IV twice under one AES-GCM key breaks the key.",
		Example: `  # The clause E1 baseline: AES-GCM, 12-octet IV, 16-octet MAC, first frame
  astro sdls apply --key-file sa7.key --spi 7 --last-iv 000000000000000000000000 < data.hex

  # The clause E2 telecommand baseline: AES-CMAC with a 4-octet sequence number
  astro sdls apply --key-file tc.key --spi 9 --mode auth --auth-alg cmac \
    --iv 0 --seq 4 --last-seq 00000041 < command.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stderr := cmd.ErrOrStderr()
			assoc, frameHeader, err := sa.association(stderr)
			if err != nil {
				return err
			}
			if err := restoreCounter("--last-iv", lastIV, sa.ivLen, assoc.SetIVCounter); err != nil {
				return err
			}
			if err := restoreCounter("--last-seq", lastSeq, sa.seqLen, assoc.SetSeqCounter); err != nil {
				return err
			}

			data, err := readInput(args, sa.inputFmt)
			if err != nil {
				return err
			}
			protected, err := assoc.ApplySecurity(frameHeader, data)
			if err != nil {
				return fmt.Errorf("applying security: %w", err)
			}

			if iv := assoc.IVCounter(); iv != nil {
				_, _ = fmt.Fprintf(stderr, "last IV sent: %s\n", hex.EncodeToString(iv))
			}
			if seq := assoc.SeqCounter(); seq != nil {
				_, _ = fmt.Fprintf(stderr, "last sequence number sent: %s\n", hex.EncodeToString(seq))
			}

			out := cmd.OutOrStdout()
			return writeOctets(out, protected, sa.outputFmt, func() {
				_, _ = fmt.Fprintf(out, "SDLS protected data field: SPI %d, %s, %d octets in, %d out\n",
					assoc.SPI, assoc.Mode, len(data), len(protected))
			})
		},
	}

	sa.register(cmd)
	cmd.Flags().StringVar(&lastIV, "last-iv", "", "Last IV sent under this key, in hex (required when --iv > 0)")
	cmd.Flags().StringVar(&lastSeq, "last-seq", "", "Last sequence number sent under this key, in hex (required when --seq > 0)")
	return cmd
}

// restoreCounter loads a counter flag into the SA. A field the SA does not
// carry needs no value; one it does carry must have one, because the default
// would be to start again from one.
func restoreCounter(flag, value string, width int, set func([]byte) error) error {
	if width == 0 {
		if value != "" {
			return fmt.Errorf("%s given, but the SA has no such field", flag)
		}
		return nil
	}
	if value == "" {
		return fmt.Errorf("%s is required: the last value sent under this key, or %s for a key that has sent nothing",
			flag, strings.Repeat("00", width))
	}
	v, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", flag, err)
	}
	if err := set(v); err != nil {
		return fmt.Errorf("%s must be %d octets: %w", flag, width, err)
	}
	return nil
}

func sdlsProcessCmd() *cobra.Command {
	var sa sdlsSAFlags

	cmd := &cobra.Command{
		Use:   "process [file]",
		Short: "Check and recover a protected data field",
		Long: "Process security on one protected frame data field: check the SPI matches, verify the MAC, decrypt, and write the recovered Transfer Frame Data Field. On any failure nothing is written but the error.\n\n" +
			"Anti-replay needs the last value accepted, and a single run has none, so this command does not check it. A receiver that must reject replays keeps an SA alive in the library instead.",
		Example: `  # Recover what apply protected
  astro sdls process --key-file sa7.key --spi 7 < protected.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			assoc, frameHeader, err := sa.association(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			data, err := readInput(args, sa.inputFmt)
			if err != nil {
				return err
			}
			header, plaintext, err := sdls.ProcessSecurity(data, frameHeader, sdls.StaticLookup(assoc))
			if err != nil {
				if errors.Is(err, sdls.ErrUnknownSPI) {
					return fmt.Errorf("processing security: the frame is not for SPI %d: %w", assoc.SPI, err)
				}
				return fmt.Errorf("processing security: %w", err)
			}

			out := cmd.OutOrStdout()
			return writeOctets(out, plaintext, sa.outputFmt, func() {
				_, _ = fmt.Fprintf(out, "SDLS verified: SPI %d, %s, %d octets recovered\n",
					header.SPI, assoc.Mode, len(plaintext))
			})
		},
	}

	sa.register(cmd)
	return cmd
}

func sdlsInspectCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		ivLen     int
		seqLen    int
		padLen    int
		macLen    int
	)

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Decode a Security Header",
		Long: "Decode the Security Header at the front of a protected frame's data field: the Security Parameter Index, and whichever of the initialisation vector, sequence number and pad length the Security Association carries.\n\n" +
			"The field widths are per Security Association, not per frame, and nothing in the header states them, so they are flags. Getting them wrong shifts everything after the SPI, which is why the SPI is reported separately: it is the one field whose position is fixed.",
		Example: `  # A header with a 12-octet IV and nothing else
  astro sdls inspect --input hex --iv 12 < frame-data.hex

  # An authentication-only SA: sequence number, no IV
  astro sdls inspect --input hex --iv 0 --seq 4 --mac 16 < frame-data.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lengths := sdls.FieldLengths{
				IV:     ivLen,
				SeqNum: seqLen,
				PadLen: padLen,
				MAC:    macLen,
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			header, consumed, err := sdls.DecodeSecurityHeader(data, lengths)
			if err != nil {
				return fmt.Errorf("decoding the Security Header: %w", err)
			}

			// What is left after the header, minus whatever trailer the SA
			// says is at the end. The middle is the protected data, which
			// without keys stays exactly that: protected.
			remainder := data[consumed:]
			var mac []byte
			if macLen > 0 && len(remainder) >= macLen {
				mac = remainder[len(remainder)-macLen:]
				remainder = remainder[:len(remainder)-macLen]
			}

			out := cmd.OutOrStdout()
			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(sdlsHeaderJSON{
					SPI:           header.SPI,
					IV:            hex.EncodeToString(header.IV),
					SeqNum:        hex.EncodeToString(header.SeqNum),
					PadLength:     hex.EncodeToString(header.PadLength),
					HeaderOctets:  consumed,
					PayloadOctets: len(remainder),
					MAC:           hex.EncodeToString(mac),
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				_, _ = fmt.Fprintln(out, string(b))
			case "text":
				_, _ = fmt.Fprintln(out, "SDLS Security Header")
				_, _ = fmt.Fprintf(out, "  SPI ............... %d\n", header.SPI)
				if len(header.IV) > 0 {
					_, _ = fmt.Fprintf(out, "  IV ................ %s\n", hex.EncodeToString(header.IV))
				}
				if len(header.SeqNum) > 0 {
					_, _ = fmt.Fprintf(out, "  Sequence number ... %s\n", hex.EncodeToString(header.SeqNum))
				}
				if len(header.PadLength) > 0 {
					_, _ = fmt.Fprintf(out, "  Pad length ........ %s\n", hex.EncodeToString(header.PadLength))
				}
				_, _ = fmt.Fprintf(out, "  Header ............ %d octets\n", consumed)
				_, _ = fmt.Fprintf(out, "  Protected data .... %d octets (not decrypted: no keys here)\n",
					len(remainder))
				if len(mac) > 0 {
					_, _ = fmt.Fprintf(out, "  MAC ............... %s (not verified: no keys here)\n",
						hex.EncodeToString(mac))
				}
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().IntVar(&ivLen, "iv", 0, "Initialisation vector length in octets")
	cmd.Flags().IntVar(&seqLen, "seq", 0, "Anti-replay sequence number length in octets")
	cmd.Flags().IntVar(&padLen, "pad", 0, "Pad length field width in octets")
	cmd.Flags().IntVar(&macLen, "mac", 0, "Message authentication code length in octets")
	return cmd
}

type sdlsHeaderJSON struct {
	SPI           uint16 `json:"spi"`
	IV            string `json:"iv,omitempty"`
	SeqNum        string `json:"sequence_number,omitempty"`
	PadLength     string `json:"pad_length,omitempty"`
	HeaderOctets  int    `json:"header_octets"`
	PayloadOctets int    `json:"payload_octets"`
	MAC           string `json:"mac,omitempty"`
}
