package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/spf13/cobra"
)

func sppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spp <command>",
		Short: "Space Packet Protocol operations",
		Long:  "Encode, decode, inspect, validate, and stream CCSDS Space Packets (CCSDS 133.0-B-2).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		sppDecodeCmd(),
		sppEncodeCmd(),
		sppInspectCmd(),
		sppValidateCmd(),
		sppStreamCmd(),
		sppGenCmd(),
	)

	return cmd
}

// readInput reads packet data from a file argument or stdin.
// inputFmt controls parsing: "hex" expects hex-encoded text, "bin" expects raw bytes.
func readInput(args []string, inputFmt string) ([]byte, error) {
	var raw []byte
	var err error

	if len(args) > 0 && args[0] != "-" {
		raw, err = os.ReadFile(args[0])
	} else {
		raw, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	switch inputFmt {
	case "hex":
		s := strings.TrimSpace(string(raw))
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "\n", "")
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.TrimPrefix(s, "0x")
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decoding hex input: %w", err)
		}
		return decoded, nil
	case "bin":
		return raw, nil
	default:
		return nil, fmt.Errorf("unknown input format: %s (use 'hex' or 'bin')", inputFmt)
	}
}

// packetJSON is the JSON-serializable representation of a decoded space packet.
type packetJSON struct {
	Version             uint8  `json:"version"`
	Type                uint8  `json:"type"`
	TypeName            string `json:"type_name"`
	SecondaryHeaderFlag uint8  `json:"secondary_header_flag"`
	APID                uint16 `json:"apid"`
	SequenceFlags       uint8  `json:"sequence_flags"`
	SequenceFlagsName   string `json:"sequence_flags_name"`
	SequenceCount       uint16 `json:"sequence_count"`
	PacketLength        uint16 `json:"packet_length"`
	UserData            string `json:"user_data"`
	ErrorControl        *uint16 `json:"error_control,omitempty"`
	IsIdle              bool   `json:"is_idle"`
}

func toPacketJSON(pkt *spp.SpacePacket) packetJSON {
	return packetJSON{
		Version:             pkt.PrimaryHeader.Version,
		Type:                pkt.PrimaryHeader.Type,
		TypeName:            typeName(pkt.PrimaryHeader.Type),
		SecondaryHeaderFlag: pkt.PrimaryHeader.SecondaryHeaderFlag,
		APID:                pkt.PrimaryHeader.APID,
		SequenceFlags:       pkt.PrimaryHeader.SequenceFlags,
		SequenceFlagsName:   seqFlagsName(pkt.PrimaryHeader.SequenceFlags),
		SequenceCount:       pkt.PrimaryHeader.SequenceCount,
		PacketLength:        pkt.PrimaryHeader.PacketLength,
		UserData:            hex.EncodeToString(pkt.UserData),
		ErrorControl:        pkt.ErrorControl,
		IsIdle:              pkt.IsIdle(),
	}
}

func typeName(t uint8) string {
	switch t {
	case spp.PacketTypeTM:
		return "TM"
	case spp.PacketTypeTC:
		return "TC"
	default:
		return "unknown"
	}
}

func seqFlagsName(f uint8) string {
	switch f {
	case spp.SeqFlagContinuation:
		return "continuation"
	case spp.SeqFlagFirstSegment:
		return "first"
	case spp.SeqFlagLastSegment:
		return "last"
	case spp.SeqFlagUnsegmented:
		return "unsegmented"
	default:
		return "unknown"
	}
}

func formatPacket(pkt *spp.SpacePacket, data []byte, format string) (string, error) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toPacketJSON(pkt), "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "hex":
		return hex.EncodeToString(data), nil
	case "text":
		return pkt.Humanize(), nil
	default:
		return "", fmt.Errorf("unknown format: %s (use 'json', 'text', or 'hex')", format)
	}
}

func sppDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode raw bytes into Space Packet fields",
		Long:  "Decode a binary or hex-encoded Space Packet, printing its header fields and user data.",
		Example: `  # Decode hex from stdin
  echo "0C01C000000461626364" | astro spp decode --input hex

  # Decode binary file
  astro spp decode --input bin packet.bin

  # Decode with JSON output
  astro spp decode --input hex --format json packet.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pkt, err := spp.Decode(data)
			if err != nil {
				return fmt.Errorf("decoding packet: %w", err)
			}

			out, err := formatPacket(pkt, data, outputFmt)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")

	return cmd
}

func sppEncodeCmd() *cobra.Command {
	var (
		apid       uint16
		packetType string
		dataHex    string
		seqCount   uint16
		seqFlags   uint8
		crcFlag    bool
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct a Space Packet from fields",
		Long:  "Build a CCSDS Space Packet from individual header fields and user data.",
		Example: `  # Encode a TM packet
  astro spp encode --apid 100 --type tm --data 61626364

  # Encode a TC packet with CRC
  astro spp encode --apid 200 --type tc --data a1b2c3d4 --crc

  # Encode with JSON output
  astro spp encode --apid 100 --type tm --data 61626364 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			userData, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			var pktType uint8
			switch strings.ToLower(packetType) {
			case "tm", "0":
				pktType = spp.PacketTypeTM
			case "tc", "1":
				pktType = spp.PacketTypeTC
			default:
				return fmt.Errorf("invalid --type: %s (use 'tm' or 'tc')", packetType)
			}

			var opts []spp.PacketOption
			if cmd.Flags().Changed("seq-count") {
				opts = append(opts, spp.WithSequenceCount(seqCount))
			}
			if cmd.Flags().Changed("seq-flags") {
				opts = append(opts, spp.WithSequenceFlags(seqFlags))
			}
			if crcFlag {
				opts = append(opts, spp.WithErrorControl())
			}

			pkt, err := spp.NewSpacePacket(apid, pktType, userData, opts...)
			if err != nil {
				return fmt.Errorf("building packet: %w", err)
			}

			encoded, err := pkt.Encode()
			if err != nil {
				return fmt.Errorf("encoding packet: %w", err)
			}

			out, err := formatPacket(pkt, encoded, outputFmt)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		},
	}

	cmd.Flags().Uint16Var(&apid, "apid", 0, "Application Process Identifier (0-2047)")
	cmd.Flags().StringVar(&packetType, "type", "tm", "Packet type: tm or tc")
	cmd.Flags().StringVar(&dataHex, "data", "", "User data as hex string")
	cmd.Flags().Uint16Var(&seqCount, "seq-count", 0, "Sequence count (0-16383)")
	cmd.Flags().Uint8Var(&seqFlags, "seq-flags", 3, "Sequence flags (0-3)")
	cmd.Flags().BoolVar(&crcFlag, "crc", false, "Append CRC-16-CCITT error control field")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	_ = cmd.MarkFlagRequired("data")

	return cmd
}

func sppInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print packet breakdown with hex dump",
		Long:  "Display an annotated breakdown of a Space Packet showing header fields, user data, and a hex dump.",
		Example: `  # Inspect from hex stdin
  echo "0C01C000000461626364" | astro spp inspect --input hex

  # Inspect binary file
  astro spp inspect --input bin packet.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pkt, err := spp.Decode(data)
			if err != nil {
				return fmt.Errorf("decoding packet: %w", err)
			}

			printInspect(pkt, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func printInspect(pkt *spp.SpacePacket, raw []byte) {
	h := pkt.PrimaryHeader
	totalLen := spp.PrimaryHeaderSize + int(h.PacketLength) + 1

	fmt.Println("Space Packet Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// Primary Header
	fmt.Println("Primary Header (6 bytes)")
	fmt.Printf("  Version .............. %d\n", h.Version)
	fmt.Printf("  Type ................. %d (%s)\n", h.Type, typeName(h.Type))
	fmt.Printf("  Secondary Header Flag  %d\n", h.SecondaryHeaderFlag)
	fmt.Printf("  APID ................. %d (0x%03X)\n", h.APID, h.APID)
	fmt.Printf("  Sequence Flags ....... %d (%s)\n", h.SequenceFlags, seqFlagsName(h.SequenceFlags))
	fmt.Printf("  Sequence Count ....... %d\n", h.SequenceCount)
	fmt.Printf("  Packet Data Length ... %d (total packet: %d bytes)\n", h.PacketLength, totalLen)

	if pkt.IsIdle() {
		fmt.Println("  [IDLE PACKET]")
	}

	// User Data
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("User Data (%d bytes)\n", len(pkt.UserData))
	if len(pkt.UserData) > 0 {
		fmt.Print(hexDump(pkt.UserData, "  "))
	}

	// Error Control
	if pkt.ErrorControl != nil {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Printf("Error Control: 0x%04X (CRC-16-CCITT)\n", *pkt.ErrorControl)
	}

	// Full hex dump
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw Packet (%d bytes)\n", len(raw[:totalLen]))
	fmt.Print(hexDump(raw[:totalLen], "  "))
}

// hexDump produces a classic hex dump with offset, hex bytes, and ASCII.
func hexDump(data []byte, indent string) string {
	var sb strings.Builder
	for i := 0; i < len(data); i += 16 {
		end := min(i+16, len(data))
		chunk := data[i:end]

		// Offset
		fmt.Fprintf(&sb, "%s%04x  ", indent, i)

		// Hex bytes
		for j, b := range chunk {
			fmt.Fprintf(&sb, "%02x ", b)
			if j == 7 {
				sb.WriteByte(' ')
			}
		}
		// Pad if short row
		for j := len(chunk); j < 16; j++ {
			sb.WriteString("   ")
			if j == 7 {
				sb.WriteByte(' ')
			}
		}

		// ASCII
		sb.WriteString(" |")
		for _, b := range chunk {
			if b >= 0x20 && b <= 0x7e {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	return sb.String()
}

func sppValidateCmd() *cobra.Command {
	var inputFmt string
	var crcFlag bool

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a Space Packet for correctness",
		Long:  "Check field ranges, length consistency, and optionally verify the CRC of a Space Packet.",
		Example: `  # Validate hex input
  echo "0C01C000000461626364" | astro spp validate --input hex

  # Validate with CRC check
  astro spp validate --input hex --crc packet.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			var opts []spp.DecodeOption
			if crcFlag {
				opts = append(opts, spp.WithDecodeErrorControl())
			}

			pkt, err := spp.Decode(data, opts...)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			if err := pkt.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			fmt.Println("Packet is valid.")
			h := pkt.PrimaryHeader
			fmt.Printf("  Type: %s, APID: %d, SeqCount: %d, Data: %d bytes\n",
				typeName(h.Type), h.APID, h.SequenceCount, len(pkt.UserData))
			if crcFlag && pkt.ErrorControl != nil {
				fmt.Printf("  CRC: 0x%04X (OK)\n", *pkt.ErrorControl)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().BoolVar(&crcFlag, "crc", false, "Verify CRC-16-CCITT error control field")

	return cmd
}

func sppStreamCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "stream [file]",
		Short: "Decode a stream of concatenated Space Packets",
		Long:  "Continuously decode concatenated Space Packets from a file or stdin, printing each one.",
		Example: `  # Stream decode from binary file
  astro spp stream --input bin capture.bin

  # Stream decode from hex stdin with JSON output
  cat packets.hex | astro spp stream --input hex --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			count := 0
			offset := 0
			for offset < len(data) {
				remaining := data[offset:]

				// Need at least 6 bytes for header to determine packet size
				if len(remaining) < spp.PrimaryHeaderSize {
					if len(remaining) > 0 {
						fmt.Fprintf(os.Stderr, "Warning: %d trailing bytes ignored\n", len(remaining))
					}
					break
				}

				pktSize := spp.PacketSizer(remaining)
				if pktSize < 0 || pktSize > len(remaining) {
					return fmt.Errorf("packet #%d at offset %d: incomplete packet (need %d bytes, have %d)",
						count+1, offset, pktSize, len(remaining))
				}

				pktData := remaining[:pktSize]
				pkt, err := spp.Decode(pktData)
				if err != nil {
					return fmt.Errorf("packet #%d at offset %d: %w", count+1, offset, err)
				}

				count++
				switch outputFmt {
				case "json":
					pj := toPacketJSON(pkt)
					b, _ := json.Marshal(pj)
					fmt.Println(string(b))
				case "hex":
					fmt.Println(hex.EncodeToString(pktData))
				case "text":
					fmt.Printf("--- Packet #%d (offset %d, %d bytes) ---\n", count, offset, pktSize)
					h := pkt.PrimaryHeader
					fmt.Printf("  Type: %s  APID: %d  SeqFlags: %s  SeqCount: %d  DataLen: %d\n",
						typeName(h.Type), h.APID, seqFlagsName(h.SequenceFlags), h.SequenceCount,
						len(pkt.UserData))
					if len(pkt.UserData) <= 32 {
						fmt.Printf("  Data: %s\n", hex.EncodeToString(pkt.UserData))
					} else {
						fmt.Printf("  Data: %s... (%d bytes)\n",
							hex.EncodeToString(pkt.UserData[:32]), len(pkt.UserData))
					}
				}

				offset += pktSize
			}

			if outputFmt == "text" {
				fmt.Printf("\nDecoded %d packet(s), %d bytes total.\n", count, offset)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")

	return cmd
}
