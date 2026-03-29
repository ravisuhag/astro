package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravisuhag/astro/pkg/epp"
	"github.com/spf13/cobra"
)

func eppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epp <command>",
		Short: "Encapsulation Packet Protocol operations",
		Long:  "Encode, decode, inspect, validate, and stream CCSDS Encapsulation Packets (CCSDS 133.1-B-3).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		eppDecodeCmd(),
		eppEncodeCmd(),
		eppInspectCmd(),
		eppValidateCmd(),
		eppStreamCmd(),
		eppGenCmd(),
	)

	return cmd
}

// eppPacketJSON is the JSON-serializable representation of a decoded Encapsulation Packet.
type eppPacketJSON struct {
	PVN                uint8  `json:"pvn"`
	ProtocolID         uint8  `json:"protocol_id"`
	ProtocolIDName     string `json:"protocol_id_name"`
	LengthOfLength     uint8  `json:"length_of_length"`
	Format             int    `json:"format"`
	HeaderSize         int    `json:"header_size"`
	PacketLength       uint32 `json:"packet_length"`
	UserDefined        uint8  `json:"user_defined,omitempty"`
	ExtendedProtocolID uint8  `json:"extended_protocol_id,omitempty"`
	CCSDSDefined       uint16 `json:"ccsds_defined,omitempty"`
	DataZone           string `json:"data_zone"`
	IsIdle             bool   `json:"is_idle"`
}

func toEPPPacketJSON(pkt *epp.EncapsulationPacket) eppPacketJSON {
	j := eppPacketJSON{
		PVN:            pkt.Header.PVN,
		ProtocolID:     pkt.Header.ProtocolID,
		ProtocolIDName: eppProtocolIDName(pkt.Header.ProtocolID),
		LengthOfLength: pkt.Header.LengthOfLength,
		Format:         pkt.Header.Format(),
		HeaderSize:     pkt.Header.Size(),
		PacketLength:   pkt.Header.PacketLength,
		DataZone:       hex.EncodeToString(pkt.Data),
		IsIdle:         pkt.IsIdle(),
	}

	switch pkt.Header.Format() {
	case 3:
		j.UserDefined = pkt.Header.UserDefined
	case 4:
		j.ExtendedProtocolID = pkt.Header.ExtendedProtocolID
	case 5:
		j.ExtendedProtocolID = pkt.Header.ExtendedProtocolID
		j.CCSDSDefined = pkt.Header.CCSDSDefined
	}

	return j
}

func eppProtocolIDName(pid uint8) string {
	switch pid {
	case epp.ProtocolIDIdle:
		return "idle"
	case epp.ProtocolIDIPE:
		return "ipe"
	case epp.ProtocolIDUserDef:
		return "user-defined"
	case epp.ProtocolIDExtended:
		return "extended"
	default:
		return "reserved"
	}
}

func formatEPPPacket(pkt *epp.EncapsulationPacket, data []byte, format string) (string, error) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toEPPPacketJSON(pkt), "", "  ")
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

func eppDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode raw bytes into Encapsulation Packet fields",
		Long:  "Decode a binary or hex-encoded Encapsulation Packet, printing its header fields and data zone.",
		Example: `  # Decode hex from stdin
  echo "740661626364" | astro epp decode --input hex

  # Decode binary file
  astro epp decode --input bin packet.bin

  # Decode with JSON output
  echo "740661626364" | astro epp decode --input hex --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pkt, err := epp.Decode(data)
			if err != nil {
				return fmt.Errorf("decoding packet: %w", err)
			}

			encoded, err := pkt.Encode()
			if err != nil {
				return fmt.Errorf("encoding packet: %w", err)
			}

			out, err := formatEPPPacket(pkt, encoded, outputFmt)
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

func eppEncodeCmd() *cobra.Command {
	var (
		protocolID uint8
		dataHex    string
		longLength bool
		userDef    uint8
		extPID     uint8
		ccsdsDef   uint16
		outputFmt  string
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Construct an Encapsulation Packet from fields",
		Long:  "Build a CCSDS Encapsulation Packet from header fields and data zone.",
		Example: `  # Encode an IPE packet
  astro epp encode --pid 2 --data 4500001400

  # Encode a user-defined packet with user-defined field
  astro epp encode --pid 6 --data a1b2c3d4 --user-defined 42

  # Encode with extended protocol ID
  astro epp encode --pid 7 --ext-pid 99 --data a1b2c3d4

  # Encode with CCSDS-defined field (Format 5)
  astro epp encode --pid 7 --ext-pid 99 --ccsds-defined 4660 --data a1b2c3d4

  # Encode an idle packet
  astro epp encode --pid 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			if cmd.Flags().Changed("data") {
				var err error
				data, err = hex.DecodeString(dataHex)
				if err != nil {
					return fmt.Errorf("decoding --data hex: %w", err)
				}
			}

			var opts []epp.PacketOption
			if cmd.Flags().Changed("long-length") && longLength {
				opts = append(opts, epp.WithLongLength())
			}
			if cmd.Flags().Changed("user-defined") {
				opts = append(opts, epp.WithUserDefined(userDef))
			}
			if cmd.Flags().Changed("ext-pid") {
				if cmd.Flags().Changed("ccsds-defined") {
					opts = append(opts, epp.WithCCSDSDefined(extPID, ccsdsDef))
				} else {
					opts = append(opts, epp.WithExtendedProtocolID(extPID))
				}
			}

			pkt, err := epp.NewPacket(protocolID, data, opts...)
			if err != nil {
				return fmt.Errorf("building packet: %w", err)
			}

			encoded, err := pkt.Encode()
			if err != nil {
				return fmt.Errorf("encoding packet: %w", err)
			}

			out, err := formatEPPPacket(pkt, encoded, outputFmt)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		},
	}

	cmd.Flags().Uint8Var(&protocolID, "pid", 2, "Protocol ID (0=idle, 2=IPE, 6=user-defined, 7=extended)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data zone as hex string (omit for idle packets)")
	cmd.Flags().BoolVar(&longLength, "long-length", false, "Force longer header format (LoL=1)")
	cmd.Flags().Uint8Var(&userDef, "user-defined", 0, "User-defined field value (Format 3)")
	cmd.Flags().Uint8Var(&extPID, "ext-pid", 0, "Extended Protocol ID (Formats 4 and 5)")
	cmd.Flags().Uint16Var(&ccsdsDef, "ccsds-defined", 0, "CCSDS-defined field value (Format 5)")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	return cmd
}

func eppInspectCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Pretty-print packet breakdown with hex dump",
		Long:  "Display an annotated breakdown of an Encapsulation Packet showing header fields, data zone, and hex dump.",
		Example: `  # Inspect from hex stdin
  echo "740661626364" | astro epp inspect --input hex

  # Inspect binary file
  astro epp inspect --input bin packet.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pkt, err := epp.Decode(data)
			if err != nil {
				return fmt.Errorf("decoding packet: %w", err)
			}

			printEPPInspect(pkt, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func printEPPInspect(pkt *epp.EncapsulationPacket, raw []byte) {
	h := pkt.Header
	hdrSize := h.Size()
	totalLen := int(h.PacketLength)
	if pkt.IsIdle() {
		totalLen = 1
	}

	fmt.Println("Encapsulation Packet Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// Header
	fmt.Printf("Header (Format %d, %d bytes)\n", h.Format(), hdrSize)
	fmt.Printf("  PVN .................. %d\n", h.PVN)
	fmt.Printf("  Protocol ID .......... %d (%s)\n", h.ProtocolID, eppProtocolIDName(h.ProtocolID))
	fmt.Printf("  Length of Length ...... %d\n", h.LengthOfLength)

	switch h.Format() {
	case 3:
		fmt.Printf("  User Defined ......... %d (0x%02X)\n", h.UserDefined, h.UserDefined)
	case 4:
		fmt.Printf("  Extended PID ......... %d\n", h.ExtendedProtocolID)
	case 5:
		fmt.Printf("  Extended PID ......... %d\n", h.ExtendedProtocolID)
		fmt.Printf("  CCSDS Defined ........ %d (0x%04X)\n", h.CCSDSDefined, h.CCSDSDefined)
	}

	if !pkt.IsIdle() {
		fmt.Printf("  Packet Length ........ %d (total packet: %d bytes)\n", h.PacketLength, totalLen)
	}

	if pkt.IsIdle() {
		fmt.Println("  [IDLE PACKET]")
	}

	// Data Zone
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Data Zone (%d bytes)\n", len(pkt.Data))
	if len(pkt.Data) > 0 {
		fmt.Print(hexDump(pkt.Data, "  "))
	}

	// Full hex dump
	fmt.Println(strings.Repeat("─", 60))
	displayLen := min(totalLen, len(raw))
	fmt.Printf("Raw Packet (%d bytes)\n", displayLen)
	fmt.Print(hexDump(raw[:displayLen], "  "))
}

func eppValidateCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate an Encapsulation Packet for correctness",
		Long:  "Check PVN, Protocol ID, header format, and packet length consistency of an Encapsulation Packet.",
		Example: `  # Validate hex input
  echo "740661626364" | astro epp validate --input hex

  # Validate a binary file
  astro epp validate --input bin packet.bin`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pkt, err := epp.Decode(data)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			if err := pkt.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			fmt.Println("Packet is valid.")
			h := pkt.Header
			fmt.Printf("  Protocol ID: %d (%s), Format: %d, Data: %d bytes\n",
				h.ProtocolID, eppProtocolIDName(h.ProtocolID), h.Format(), len(pkt.Data))
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func eppStreamCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "stream [file]",
		Short: "Decode a stream of concatenated Encapsulation Packets",
		Long:  "Continuously decode concatenated Encapsulation Packets from a file or stdin, printing each one.",
		Example: `  # Stream decode from binary file
  astro epp stream --input bin capture.bin

  # Stream decode with JSON output
  cat packets.hex | astro epp stream --input hex --format json`,
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

				if len(remaining) < 1 {
					break
				}

				pktSize := epp.PacketSizer(remaining)
				if pktSize < 0 || pktSize > len(remaining) {
					return fmt.Errorf("packet #%d at offset %d: incomplete packet (need %d bytes, have %d)",
						count+1, offset, pktSize, len(remaining))
				}

				pktData := remaining[:pktSize]
				pkt, err := epp.Decode(pktData)
				if err != nil {
					return fmt.Errorf("packet #%d at offset %d: %w", count+1, offset, err)
				}

				count++
				switch outputFmt {
				case "json":
					pj := toEPPPacketJSON(pkt)
					b, _ := json.Marshal(pj)
					fmt.Println(string(b))
				case "hex":
					fmt.Println(hex.EncodeToString(pktData))
				case "text":
					fmt.Printf("--- Packet #%d (offset %d, %d bytes) ---\n", count, offset, pktSize)
					h := pkt.Header
					fmt.Printf("  PID: %d (%s)  Format: %d  DataLen: %d\n",
						h.ProtocolID, eppProtocolIDName(h.ProtocolID), h.Format(), len(pkt.Data))
					if len(pkt.Data) <= 32 {
						fmt.Printf("  Data: %s\n", hex.EncodeToString(pkt.Data))
					} else {
						fmt.Printf("  Data: %s... (%d bytes)\n",
							hex.EncodeToString(pkt.Data[:32]), len(pkt.Data))
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
