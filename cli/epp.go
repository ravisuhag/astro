package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		HeaderSize:     pkt.Header.Size(),
		PacketLength:   pkt.Header.PacketLength,
		DataZone:       hex.EncodeToString(pkt.Data),
		IsIdle:         pkt.IsIdle(),
	}

	if pkt.Header.Size() >= epp.HeaderSize4 {
		j.UserDefined = pkt.Header.UserDefined
		j.ExtendedProtocolID = pkt.Header.ExtendedProtocolID
	}
	if pkt.Header.Size() == epp.HeaderSize8 {
		j.CCSDSDefined = pkt.Header.CCSDSDefined
	}

	return j
}

func eppProtocolIDName(pid uint8) string {
	switch pid {
	case epp.ProtocolIDIdle:
		return "idle"
	case epp.ProtocolIDLTP:
		return "ltp"
	case epp.ProtocolIDIPE:
		return "ipe"
	case epp.ProtocolIDExtended:
		return "extended"
	case epp.ProtocolIDMission:
		return "mission"
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
  echo "e90661626364" | astro epp decode --input hex

  # Decode binary file
  astro epp decode --input bin packet.bin

  # Decode with JSON output
  echo "e90661626364" | astro epp decode --input hex --format json`,
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

			formatted, err := formatEPPPacket(pkt, encoded, outputFmt)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), formatted)
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

  # Encode a mission-specific packet with a user defined field value
  astro epp encode --pid 7 --data a1b2c3d4 --user-defined 5

  # Encode with an extended protocol ID (4-octet header)
  astro epp encode --pid 6 --ext-pid 9 --data a1b2c3d4

  # Encode with a CCSDS-defined field (8-octet header)
  astro epp encode --pid 6 --ext-pid 9 --ccsds-defined 4660 --data a1b2c3d4

  # Encode the 1-octet idle packet (0xE0)
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
				opts = append(opts, epp.WithExtendedProtocolID(extPID))
			}
			if cmd.Flags().Changed("ccsds-defined") {
				opts = append(opts, epp.WithCCSDSDefined(ccsdsDef))
			}

			pkt, err := epp.NewPacket(protocolID, data, opts...)
			if err != nil {
				return fmt.Errorf("building packet: %w", err)
			}

			encoded, err := pkt.Encode()
			if err != nil {
				return fmt.Errorf("encoding packet: %w", err)
			}

			formatted, err := formatEPPPacket(pkt, encoded, outputFmt)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), formatted)
			return nil
		},
	}

	cmd.Flags().Uint8Var(&protocolID, "pid", 2, "Protocol ID (0=idle, 1=LTP, 2=IPE, 6=extended, 7=mission)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Data zone as hex string (omit for the 1-octet idle packet)")
	cmd.Flags().BoolVar(&longLength, "long-length", false, "Force at least a 4-octet header (2-octet length field)")
	cmd.Flags().Uint8Var(&userDef, "user-defined", 0, "User Defined Field value, 4 bits (4- and 8-octet headers)")
	cmd.Flags().Uint8Var(&extPID, "ext-pid", 0, "Protocol ID Extension, 4 bits (4- and 8-octet headers)")
	cmd.Flags().Uint16Var(&ccsdsDef, "ccsds-defined", 0, "CCSDS Defined Field value (8-octet header)")
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
  echo "e90661626364" | astro epp inspect --input hex

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

			printEPPInspect(cmd.OutOrStdout(), pkt, data)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")

	return cmd
}

func printEPPInspect(out io.Writer, pkt *epp.EncapsulationPacket, raw []byte) {
	h := pkt.Header
	hdrSize := h.Size()
	totalLen := int(h.PacketLength)

	_, _ = fmt.Fprintln(out, "Encapsulation Packet Inspector")
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))

	// Header
	_, _ = fmt.Fprintf(out, "Header (%d bytes, LoL %d)\n", hdrSize, h.LengthOfLength)
	_, _ = fmt.Fprintf(out, "  PVN .................. %d\n", h.PVN)
	_, _ = fmt.Fprintf(out, "  Protocol ID .......... %d (%s)\n", h.ProtocolID, eppProtocolIDName(h.ProtocolID))
	_, _ = fmt.Fprintf(out, "  Length of Length ...... %d\n", h.LengthOfLength)

	if hdrSize >= epp.HeaderSize4 {
		_, _ = fmt.Fprintf(out, "  User Defined ......... %d (0x%X)\n", h.UserDefined, h.UserDefined)
		_, _ = fmt.Fprintf(out, "  Protocol ID Ext ...... %d\n", h.ExtendedProtocolID)
	}
	if hdrSize == epp.HeaderSize8 {
		_, _ = fmt.Fprintf(out, "  CCSDS Defined ........ %d (0x%04X)\n", h.CCSDSDefined, h.CCSDSDefined)
	}

	_, _ = fmt.Fprintf(out, "  Packet Length ........ %d (total packet: %d bytes)\n", h.PacketLength, totalLen)

	if pkt.IsIdle() {
		_, _ = fmt.Fprintln(out, "  [IDLE PACKET]")
	}

	// Data Zone
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
	_, _ = fmt.Fprintf(out, "Data Zone (%d bytes)\n", len(pkt.Data))
	if len(pkt.Data) > 0 {
		_, _ = fmt.Fprint(out, hexDump(pkt.Data, "  "))
	}

	// Full hex dump
	_, _ = fmt.Fprintln(out, strings.Repeat("─", 60))
	displayLen := min(totalLen, len(raw))
	_, _ = fmt.Fprintf(out, "Raw Packet (%d bytes)\n", displayLen)
	_, _ = fmt.Fprint(out, hexDump(raw[:displayLen], "  "))
}

func eppValidateCmd() *cobra.Command {
	var inputFmt string

	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate an Encapsulation Packet for correctness",
		Long:  "Check PVN, Protocol ID, header format, and packet length consistency of an Encapsulation Packet.",
		Example: `  # Validate hex input
  echo "e90661626364" | astro epp validate --input hex

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

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, "Packet is valid.")
			h := pkt.Header
			_, _ = fmt.Fprintf(out, "  Protocol ID: %d (%s), Header: %d bytes, Data: %d bytes\n",
				h.ProtocolID, eppProtocolIDName(h.ProtocolID), h.Size(), len(pkt.Data))
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
			source, closer, err := openInput(args, inputFmt)
			if err != nil {
				return err
			}
			defer closer.Close() //nolint:errcheck // read-only

			out := cmd.OutOrStdout()
			count := 0
			offset := 0

			handle := func(pktData []byte) error {
				pkt, err := epp.Decode(pktData)
				if err != nil {
					return fmt.Errorf("packet #%d at offset %d: %w", count+1, offset, err)
				}
				count++

				switch outputFmt {
				case "json":
					b, err := json.Marshal(toEPPPacketJSON(pkt))
					if err != nil {
						return fmt.Errorf("encoding JSON output: %w", err)
					}
					_, _ = fmt.Fprintln(out, string(b))
				case "hex":
					_, _ = fmt.Fprintln(out, hex.EncodeToString(pktData))
				case "text":
					_, _ = fmt.Fprintf(out, "--- Packet #%d (offset %d, %d bytes) ---\n", count, offset, len(pktData))
					h := pkt.Header
					_, _ = fmt.Fprintf(out, "  PID: %d (%s)  Header: %d bytes  DataLen: %d\n",
						h.ProtocolID, eppProtocolIDName(h.ProtocolID), h.Size(), len(pkt.Data))
					if len(pkt.Data) <= 32 {
						_, _ = fmt.Fprintf(out, "  Data: %s\n", hex.EncodeToString(pkt.Data))
					} else {
						_, _ = fmt.Fprintf(out, "  Data: %s... (%d bytes)\n",
							hex.EncodeToString(pkt.Data[:32]), len(pkt.Data))
					}
				}

				offset += len(pktData)
				return nil
			}

			trailing := func(n int) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %d trailing bytes ignored\n", n)
			}

			// An Encapsulation Packet's length is readable from its first
			// octet, so one is enough to size the unit.
			if err := streamUnits(source, epp.PacketSizer, 1, handle, trailing); err != nil {
				return err
			}

			if outputFmt == "text" {
				_, _ = fmt.Fprintf(out, "\nDecoded %d packet(s), %d bytes total.\n", count, offset)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text, json, or hex")

	return cmd
}
