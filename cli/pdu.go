package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/cfdp"
	"github.com/ravisuhag/astro/pkg/ltp"
	"github.com/ravisuhag/astro/pkg/sle"
	"github.com/spf13/cobra"
)

// Decoding one protocol data unit.
//
// CFDP, LTP, BP and SLE are all the same job from a command line: you have
// some octets off a link or out of a capture, and you want to know what they
// say. None of them is a stream of fixed-length frames, so there is nothing
// here to gap-check or demultiplex; there is only "read this and tell me
// what it is".
//
// So they share one shape. Each protocol supplies a function that turns
// octets into a description, and the flags, the input handling and the output
// formats are written once.

// pduDescription is what a decoder reports back.
type pduDescription struct {
	// Kind names the unit: "File Directive PDU", "Data Segment", "Bundle".
	Kind string `json:"kind"`
	// Summary is the protocol package's own rendering, which is where the
	// field detail lives.
	Summary string `json:"summary"`
	// Octets is how much of the input the unit occupied.
	Octets int `json:"octets"`
	// Body is whatever the unit carried that the decoder did not interpret,
	// as hex. Empty when there is none.
	Body string `json:"body,omitempty"`
	// Note records something the caller should know: a body this build does
	// not decode, say.
	Note string `json:"note,omitempty"`
}

// pduDecoder turns octets into a description.
type pduDecoder func(data []byte) (pduDescription, error)

// newPDUDecodeCmd builds a decode command for one protocol.
func newPDUDecodeCmd(use, short, long, example string, decode pduDecoder) *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			described, err := decode(data)
			if err != nil {
				return err
			}

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(described, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				fmt.Println(described.Kind)
				fmt.Println(described.Summary)
				if described.Body != "" {
					fmt.Printf("Body: %d octets\n  %s\n", len(described.Body)/2, described.Body)
				}
				if described.Note != "" {
					fmt.Printf("  [%s]\n", described.Note)
				}
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	return cmd
}

// --- CFDP ---

func cfdpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cfdp <command>",
		Short: "CCSDS File Delivery Protocol operations",
		Long:  "Decode CFDP Protocol Data Units (CCSDS 727.0-B-5).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(newPDUDecodeCmd(
		"decode [file]",
		"Decode a CFDP PDU",
		"Decode a CFDP Protocol Data Unit: the fixed header, then the file directive or file data it carries.\n\n"+
			"The CRC is verified when the header says one is present, and a mismatch is an error — §4.1.2 requires the receiver to discard such a PDU.",
		`  # Decode a PDU
  astro cfdp decode --input hex < pdu.hex

  # From a capture
  astro cfdp decode --input bin pdu.bin --format json`,
		decodeCFDPPDU,
	))
	return cmd
}

func decodeCFDPPDU(data []byte) (pduDescription, error) {
	pdu, err := cfdp.DecodePDU(data)
	if err != nil {
		return pduDescription{}, fmt.Errorf("decoding the PDU: %w", err)
	}

	described := pduDescription{
		Kind:    "CFDP File Directive PDU",
		Summary: pdu.Header.Humanize(),
		Octets:  len(data),
	}
	if pdu.Header.IsFileData {
		described.Kind = "CFDP File Data PDU"
		described.Body = hex.EncodeToString(pdu.Data)
		described.Note = "file data is not interpreted"
		return described, nil
	}

	// A file directive names itself in its first octet (table 5-4), so the
	// body can be decoded properly rather than shown as octets.
	if len(pdu.Data) == 0 {
		described.Note = "the directive PDU carries no directive code"
		return described, nil
	}

	code := cfdp.DirectiveCode(pdu.Data[0])
	described.Kind = "CFDP File Directive PDU: " + code.String()

	// The directive decoders take the whole data field, code included: each
	// one validates that the code is the one it expects before reading the
	// rest. So the code is read here only to name the directive and pick the
	// decoder, not stripped off.
	detail, err := describeCFDPDirective(code, pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		described.Body = hex.EncodeToString(pdu.Data)
		described.Note = "not decoded: " + err.Error()
		return described, nil
	}
	described.Summary += "\n" + detail
	return described, nil
}

// describeCFDPDirective renders one directive's data field.
//
// body is the whole data field with its directive code still on the front,
// which is what every decoder here expects.
func describeCFDPDirective(code cfdp.DirectiveCode, body []byte, largeFile bool) (string, error) {
	switch code {
	case cfdp.DirectiveEOF:
		pdu, err := cfdp.DecodeEOFPDU(body, largeFile)
		if err != nil {
			return "", err
		}
		return pdu.Humanize(), nil
	case cfdp.DirectiveFinished:
		pdu, err := cfdp.DecodeFinishedPDU(body)
		if err != nil {
			return "", err
		}
		return pdu.Humanize(), nil
	case cfdp.DirectiveACK:
		pdu, err := cfdp.DecodeACKPDU(body)
		if err != nil {
			return "", err
		}
		return pdu.Humanize(), nil
	case cfdp.DirectiveMetadata:
		pdu, err := cfdp.DecodeMetadataPDU(body, largeFile)
		if err != nil {
			return "", err
		}

		summary := pdu.Humanize()
		// Part 2's User Operations travel as Reserved CFDP Messages in the
		// metadata, so a proxy put or a directory listing shows up here
		// rather than as a PDU of its own.
		if operations := describeUserMessages(pdu.Options); operations != "" {
			summary += "\n" + operations
		}
		return summary, nil
	case cfdp.DirectiveNAK:
		pdu, err := cfdp.DecodeNAKPDU(body, largeFile)
		if err != nil {
			return "", err
		}
		return pdu.Humanize(), nil
	case cfdp.DirectivePrompt:
		pdu, err := cfdp.DecodePromptPDU(body)
		if err != nil {
			return "", err
		}
		return pdu.Humanize(), nil
	default:
		return "", fmt.Errorf("directive code 0x%02X is not one this build decodes", uint8(code))
	}
}

// describeUserMessages renders the Part 2 User Operations a metadata PDU
// carries.
//
// Section 6 puts every proxy, directory, remote status, suspend and resume
// operation in a Message to User TLV rather than in a PDU of its own, so
// this is where they surface. Application messages in the same run are left
// alone.
func describeUserMessages(options []cfdp.TLV) string {
	messages := cfdp.UserMessagesFrom(options)
	if len(messages) == 0 {
		return ""
	}

	var out strings.Builder
	fmt.Fprintf(&out, "User operations (%d message(s)):", len(messages))

	for _, message := range messages {
		fmt.Fprintf(&out, "\n  %s", message.Type)
		if detail := describeUserMessageContent(message); detail != "" {
			fmt.Fprintf(&out, "\n%s", detail)
		}
	}
	return out.String()
}

// describeUserMessageContent renders one message's body where the content is
// modeled, and says nothing where it is not — an empty string rather than a
// guess, so the caller prints just the type.
func describeUserMessageContent(message *cfdp.UserMessage) string {
	switch message.Type {
	case cfdp.MsgOriginatingTransactionID:
		if m, err := cfdp.DecodeOriginatingTransactionID(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgProxyPutRequest:
		if m, err := cfdp.DecodeProxyPutRequest(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgProxyPutResponse:
		if m, err := cfdp.DecodeProxyPutResponse(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgProxyFilestoreRequest:
		if m, err := cfdp.DecodeProxyFilestoreRequest(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgProxyFilestoreResponse:
		if m, err := cfdp.DecodeProxyFilestoreResponse(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgDirectoryListingRequest:
		if m, err := cfdp.DecodeDirectoryListingRequest(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgDirectoryListingResponse:
		if m, err := cfdp.DecodeDirectoryListingResponse(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgRemoteStatusReportRequest:
		if m, err := cfdp.DecodeRemoteStatusReportRequest(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgRemoteStatusReportResponse:
		if m, err := cfdp.DecodeRemoteStatusReportResponse(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgRemoteSuspendResponse:
		if m, err := cfdp.DecodeRemoteSuspendResponse(message.Content); err == nil {
			return m.Humanize()
		}
	case cfdp.MsgRemoteResumeResponse:
		if m, err := cfdp.DecodeRemoteResumeResponse(message.Content); err == nil {
			return m.Humanize()
		}
	}
	return ""
}

// --- LTP ---

func ltpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ltp <command>",
		Short: "Licklider Transmission Protocol operations",
		Long:  "Decode LTP segments (CCSDS 734.1-B-1, RFC 5326).",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(newPDUDecodeCmd(
		"decode [file]",
		"Decode an LTP segment",
		"Decode one LTP segment: the header, then the data, report, report acknowledgement or cancel content it carries.",
		`  # Decode a segment
  astro ltp decode --input hex < segment.hex`,
		decodeLTPSegment,
	))
	return cmd
}

func decodeLTPSegment(data []byte) (pduDescription, error) {
	segment, err := ltp.DecodeSegment(data)
	if err != nil {
		return pduDescription{}, fmt.Errorf("decoding the segment: %w", err)
	}

	return pduDescription{
		Kind:    "LTP Segment",
		Summary: segment.Humanize(),
		Octets:  len(data),
	}, nil
}

// --- BP ---

func bpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bp <command>",
		Short: "Bundle Protocol operations",
		Long: "Decode Bundle Protocol v6 bundles and administrative records (CCSDS 734.2-B-1, RFC 5050).\n\n" +
			"This is version 6, which is what CCSDS profiles. BPv7 (RFC 9171) encodes bundles in CBOR and is wire-incompatible; it is not implemented.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(newPDUDecodeCmd(
		"decode [file]",
		"Decode a bundle",
		"Decode one bundle: the primary block, then each canonical block it carries.",
		`  # Decode a bundle
  astro bp decode --input hex < bundle.hex`,
		decodeBPBundle,
	))
	cmd.AddCommand(newPDUDecodeCmd(
		"admin [file]",
		"Decode an administrative record",
		"Decode an administrative record: a status report or a custody signal, which is what a bundle's payload holds when it is an administrative bundle.",
		`  # Decode a status report
  astro bp admin --input hex < record.hex`,
		decodeBPAdminRecord,
	))
	return cmd
}

func decodeBPBundle(data []byte) (pduDescription, error) {
	bundle, err := bp.DecodeBundle(data)
	if err != nil {
		return pduDescription{}, fmt.Errorf("decoding the bundle: %w", err)
	}

	return pduDescription{
		Kind:    "BPv6 Bundle",
		Summary: bundle.Humanize(),
		Octets:  len(data),
	}, nil
}

func decodeBPAdminRecord(data []byte) (pduDescription, error) {
	record, err := bp.DecodeAdminRecord(data)
	if err != nil {
		return pduDescription{}, fmt.Errorf("decoding the administrative record: %w", err)
	}

	return pduDescription{
		Kind:    "BPv6 Administrative Record",
		Summary: record.Humanize(),
		Octets:  len(data),
	}, nil
}

// --- SLE ---

func sleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sle <command>",
		Short: "Space Link Extension operations",
		Long: "Decode SLE transfer service PDUs (CCSDS 911.1, 911.2, 911.5 and 912.1).\n\n" +
			"An SLE PDU's wire tag means different things in different services — the same number is one operation in RAF and another in FCLTU — so --service is required and not guessable from the octets.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(sleDecodeCmd())
	return cmd
}

func sleDecodeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		service   string
	)

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode an SLE PDU envelope",
		Long: "Decode the outer envelope of an SLE PDU: which service and operation it is, and the encoded content.\n\n" +
			"The content is reported as octets. Decoding it further needs the operation's own decoder, and which one applies depends on the service, which is why that is a flag.",
		Example: `  # Decode a RAF PDU
  astro sle decode --service raf --input hex < pdu.hex

  # An FCLTU one
  astro sle decode --service fcltu --input hex < pdu.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := sleServiceKind(service)
			if err != nil {
				return err
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			pdu, err := sle.DecodePDU(data, kind)
			if err != nil {
				return fmt.Errorf("decoding the PDU: %w", err)
			}

			described := pduDescription{
				Kind: fmt.Sprintf("SLE %s PDU", pdu.Service),
				Summary: fmt.Sprintf("  Service ..... %s\n  Operation ... %s\n  Wire tag .... %d",
					pdu.Service, pdu.Operation, pdu.Tag),
				Octets: len(data),
				Body:   hex.EncodeToString(pdu.Content),
				Note:   "the operation content is not decoded further",
			}

			// A GET-PARAMETER return is the one operation whose content this
			// package can read, now that the per-service parameter sets are
			// modeled. Which parameter a tag names depends on the service,
			// which is why --service is required.
			if pdu.Operation == sle.OpGetParameterReturn {
				if summary, ok := describeSLEParameter(pdu.Content, kind); ok {
					described.Summary += "\n" + summary
					described.Note = ""
				}
			}

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(described, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				fmt.Println(described.Kind)
				fmt.Println(described.Summary)
				fmt.Printf("Content: %d octets\n  %s\n", len(pdu.Content), described.Body)
				if described.Note != "" {
					fmt.Printf("  [%s]\n", described.Note)
				}
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&service, "service", "", "Transfer service: raf, rcf, rocf, or fcltu (required)")

	_ = cmd.MarkFlagRequired("service")
	return cmd
}

// describeSLEParameter renders the parameter a GET-PARAMETER return carries.
//
// A negative return has no parameter — the provider answering that it does
// not have the one asked for, which the specs define — and a return this
// cannot read is left to the raw-octet path rather than reported as a
// failure, because the envelope decoded fine and that is what was asked for.
func describeSLEParameter(content []byte, service sle.ServiceKind) (string, bool) {
	ret, err := sle.DecodeGetParameterReturn(content)
	if err != nil {
		return "", false
	}

	parameter, ok, err := ret.DecodeParameter(service)
	if err != nil {
		return fmt.Sprintf("  [parameter not decoded: %v]", err), true
	}
	if !ok {
		return fmt.Sprintf("  Negative: %s", ret.SpecificDiagnostic), true
	}
	return parameter.Humanize(), true
}

// sleServiceKind maps the flag to the service, which the wire tag alone
// cannot tell us.
func sleServiceKind(name string) (sle.ServiceKind, error) {
	switch strings.ToLower(name) {
	case "raf":
		return sle.ServiceRAF, nil
	case "rcf":
		return sle.ServiceRCF, nil
	case "rocf":
		return sle.ServiceROCF, nil
	case "fcltu":
		return sle.ServiceFCLTU, nil
	default:
		return 0, fmt.Errorf(
			"unknown --service %q (use 'raf', 'rcf', 'rocf', or 'fcltu')", name)
	}
}
