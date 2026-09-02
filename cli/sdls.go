package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ravisuhag/astro/pkg/sdls"
	"github.com/spf13/cobra"
)

func sdlsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdls <command>",
		Short: "Space Data Link Security",
		Long: "Read the Security Header of a protected frame (CCSDS 355.0-B-2).\n\n" +
			"Only the header is here, and deliberately so. Applying or removing security needs the Security Association's keys, and a command line is the wrong place for key material: it lands in shell history and in the process table where anyone on the machine can read it. Use the library, which takes keys from wherever your mission keeps them.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(sdlsInspectCmd())
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
			"The field widths are per Security Association, not per frame, and nothing in the header states them — so they are flags. Getting them wrong shifts everything after the SPI, which is why the SPI is reported separately: it is the one field whose position is fixed.",
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
				fmt.Println(string(b))
			case "text":
				fmt.Println("SDLS Security Header")
				fmt.Printf("  SPI ............... %d\n", header.SPI)
				if len(header.IV) > 0 {
					fmt.Printf("  IV ................ %s\n", hex.EncodeToString(header.IV))
				}
				if len(header.SeqNum) > 0 {
					fmt.Printf("  Sequence number ... %s\n", hex.EncodeToString(header.SeqNum))
				}
				if len(header.PadLength) > 0 {
					fmt.Printf("  Pad length ........ %s\n", hex.EncodeToString(header.PadLength))
				}
				fmt.Printf("  Header ............ %d octets\n", consumed)
				fmt.Printf("  Protected data .... %d octets (not decrypted: no keys here)\n",
					len(remainder))
				if len(mac) > 0 {
					fmt.Printf("  MAC ............... %s (not verified: no keys here)\n",
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
