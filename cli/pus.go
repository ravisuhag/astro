package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
	"github.com/spf13/cobra"
)

func pusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pus <command>",
		Short: "PUS packet utilisation service operations",
		Long: "Encode and decode ECSS PUS-C secondary headers and message bodies (ECSS-E-ST-70-41C).\n\n" +
			"PUS rides inside a Space Packet's data field, so these commands work on what is left after the Space Packet primary header. Pipe through 'astro spp' to get there.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		pusDecodeCmd(),
		pusEncodeCmd(),
		pusServicesCmd(),
	)
	return cmd
}

// profileFlags are the mission-tailorable widths every PUS command needs.
//
// ECSS-E-ST-70-41C states no defaults for these: a real mission declares
// them, and two missions can disagree about the width of the same field. So
// they are flags rather than assumptions, seeded from DefaultProfile, which
// the package is explicit is a convenience for tooling rather than a
// standard-mandated default.
type profileFlags struct {
	tcSpare    int
	tmSpare    int
	timeFormat string
	cucCoarse  int
	cucFine    int
	timeRaw    int
}

func (p *profileFlags) register(cmd *cobra.Command) {
	defaults := pus.DefaultProfile()

	cmd.Flags().IntVar(&p.tcSpare, "tc-spare", defaults.TCSpareBytes,
		"Octets of spare in the TC secondary header")
	cmd.Flags().IntVar(&p.tmSpare, "tm-spare", defaults.TMSpareBytes,
		"Octets of spare in the TM secondary header")
	cmd.Flags().StringVar(&p.timeFormat, "time", "cuc",
		"TM time format: cuc, cuc-explicit, raw, or none")
	cmd.Flags().IntVar(&p.cucCoarse, "cuc-coarse", defaults.CUCCoarseBytes,
		"Octets of CUC coarse time")
	cmd.Flags().IntVar(&p.cucFine, "cuc-fine", defaults.CUCFineBytes,
		"Octets of CUC fine time")
	cmd.Flags().IntVar(&p.timeRaw, "time-raw", defaults.TimeRawBytes,
		"Octets of an opaque time field, when --time raw")
}

// profile builds the mission profile the flags describe.
func (p *profileFlags) profile() (pus.MissionProfile, error) {
	built := pus.DefaultProfile()
	built.TCSpareBytes = p.tcSpare
	built.TMSpareBytes = p.tmSpare
	built.CUCCoarseBytes = p.cucCoarse
	built.CUCFineBytes = p.cucFine
	built.TimeRawBytes = p.timeRaw

	switch strings.ToLower(p.timeFormat) {
	case "cuc":
		built.TimeFormat = pus.TimeCUC
	case "cuc-explicit":
		built.TimeFormat = pus.TimeCUCExplicit
	case "raw":
		built.TimeFormat = pus.TimeRaw
	case "none":
		built.TimeFormat = pus.TimeNone
	default:
		return pus.MissionProfile{}, fmt.Errorf(
			"unknown --time %q (use 'cuc', 'cuc-explicit', 'raw', or 'none')", p.timeFormat)
	}

	return built, nil
}

func pusDecodeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		direction string
		flags     profileFlags
	)

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a PUS secondary header and, where known, its body",
		Long: "Decode a PUS TM or TC secondary header, then decode the message body against the service registry when the service is implemented.\n\n" +
			"A body whose service is not implemented is reported as raw octets rather than guessed at. Run 'astro pus services' to see which are known.",
		Example: `  # Decode a TM report
  astro pus decode --direction tm --input hex < report.hex

  # From a Space Packet, taking the data field
  astro spp decode --input hex --format json < packet.hex

  # A mission with a two-octet spare and no time field
  astro pus decode --direction tm --tm-spare 2 --time none --input hex < report.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, err := flags.profile()
			if err != nil {
				return err
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			switch strings.ToLower(direction) {
			case "tm":
				return decodePUSTM(data, profile, outputFmt)
			case "tc":
				return decodePUSTC(data, profile, outputFmt)
			default:
				return fmt.Errorf("unknown --direction %q (use 'tm' or 'tc')", direction)
			}
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&direction, "direction", "tm", "Message direction: tm (report) or tc (request)")
	flags.register(cmd)
	return cmd
}

func pusEncodeCmd() *cobra.Command {
	var (
		outputFmt string
		direction string
		service   uint8
		subtype   uint8
		sourceID  uint16
		destID    uint16
		ackFlags  uint8
		dataHex   string
		timeTag   string
		flags     profileFlags
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Build a PUS secondary header with a body",
		Long:  "Construct a PUS TM or TC secondary header from fields and append a body, ready to go in a Space Packet's data field.",
		Example: `  # A TC[3,1] request
  astro pus encode --direction tc --service 3 --subtype 1 --data 01020304

  # Wrap it in a Space Packet
  astro pus encode --direction tc --service 3 --subtype 1 --data 0102 |
    xargs -I{} astro spp encode --apid 100 --type tc --data {}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, err := flags.profile()
			if err != nil {
				return err
			}

			body, err := hex.DecodeString(dataHex)
			if err != nil {
				return fmt.Errorf("decoding --data hex: %w", err)
			}

			var header []byte

			switch strings.ToLower(direction) {
			case "tm":
				// A TM report is time tagged, and the zero time.Time is year
				// one, which no CUC field can hold. Default to now rather
				// than leaving the caller to discover that.
				tagged := time.Now().UTC()
				if timeTag != "" {
					if tagged, err = time.Parse(time.RFC3339, timeTag); err != nil {
						return fmt.Errorf("parsing --time-tag: %w", err)
					}
				}

				tm := profile.NewTMHeader(service, subtype, destID, tagged)
				if header, err = tm.Encode(); err != nil {
					return fmt.Errorf("encoding the TM header: %w", err)
				}
			case "tc":
				tc := &pus.TCHeader{
					Profile:  profile,
					AckFlags: pus.AckFlags(ackFlags),
					Service:  service,
					Subtype:  subtype,
					SourceID: sourceID,
				}
				if profile.TCSpareBytes > 0 {
					tc.Spare = make([]byte, profile.TCSpareBytes)
				}
				if header, err = tc.Encode(); err != nil {
					return fmt.Errorf("encoding the TC header: %w", err)
				}
			default:
				return fmt.Errorf("unknown --direction %q (use 'tm' or 'tc')", direction)
			}

			message := append(header, body...)

			switch outputFmt {
			case "hex":
				fmt.Println(hex.EncodeToString(message))
			case "text":
				fmt.Printf("PUS %s[%d,%d], %d octets (%d header, %d body)\n",
					strings.ToUpper(direction), service, subtype,
					len(message), len(header), len(body))
				fmt.Println(hex.EncodeToString(message))
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'hex')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text or hex")
	cmd.Flags().StringVar(&direction, "direction", "tc", "Message direction: tm (report) or tc (request)")
	cmd.Flags().Uint8Var(&service, "service", 0, "Service type (ST)")
	cmd.Flags().Uint8Var(&subtype, "subtype", 0, "Message subtype")
	cmd.Flags().Uint16Var(&sourceID, "source", 0, "Source ID, for a TC")
	cmd.Flags().Uint16Var(&destID, "dest", 0, "Destination ID, for a TM")
	cmd.Flags().Uint8Var(&ackFlags, "ack", 0, "Acknowledgement flags, for a TC (4 bits)")
	cmd.Flags().StringVar(&dataHex, "data", "", "Message body as hex")
	cmd.Flags().StringVar(&timeTag, "time-tag", "",
		"RFC 3339 time tag for a TM report (default: now)")
	flags.register(cmd)

	_ = cmd.MarkFlagRequired("service")
	_ = cmd.MarkFlagRequired("subtype")
	return cmd
}

func pusServicesCmd() *cobra.Command {
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "services",
		Short: "List the PUS message types this build can decode",
		Long: "List the service and subtype pairs the default registry knows how to decode.\n\n" +
			"A message type that is not listed still decodes as far as its secondary header; only its body is left as raw octets.",
		Example: `  # What can be decoded
  astro pus services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := pus.NewDefaultRegistry(pus.DefaultProfile())
			if err != nil {
				return fmt.Errorf("building the registry: %w", err)
			}

			requests := keyStrings(registry.KnownRequests())
			reports := keyStrings(registry.KnownReports())

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(map[string][]string{
					"requests": requests,
					"reports":  reports,
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				fmt.Println(string(b))
			case "text":
				out := cmd.OutOrStdout()
				printNames(out, "Requests (TC)", requests)
				printNames(out, "Reports (TM)", reports)
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	return cmd
}

// keyStrings renders message keys as ST[service,subtype], sorted.
func keyStrings(keys []pus.MessageKey) []string {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Service != keys[j].Service {
			return keys[i].Service < keys[j].Service
		}
		return keys[i].Subtype < keys[j].Subtype
	})

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, fmt.Sprintf("ST[%d,%d]", key.Service, key.Subtype))
	}
	return out
}

// pusMessageJSON is a decoded PUS message.
type pusMessageJSON struct {
	Direction  string `json:"direction"`
	Service    uint8  `json:"service"`
	Subtype    uint8  `json:"subtype"`
	HeaderSize int    `json:"header_octets"`
	Body       string `json:"body"`
	BodyKnown  bool   `json:"body_decoded"`
	BodyDetail string `json:"body_detail,omitempty"`
	BodyError  string `json:"body_error,omitempty"`
}

func decodePUSTM(data []byte, profile pus.MissionProfile, outputFmt string) error {
	header := &pus.TMHeader{Profile: profile}
	if err := header.Decode(data); err != nil {
		return fmt.Errorf("decoding the TM header: %w", err)
	}

	body := data[header.Size():]
	message := pusMessageJSON{
		Direction:  "tm",
		Service:    header.Service,
		Subtype:    header.Subtype,
		HeaderSize: header.Size(),
		Body:       hex.EncodeToString(body),
	}

	registry, err := pus.NewDefaultRegistry(profile)
	if err != nil {
		return fmt.Errorf("building the registry: %w", err)
	}
	if report, err := registry.DecodeReport(header.Key(), body); err == nil {
		message.BodyKnown = true
		message.BodyDetail = humanizeOrEmpty(report)
	} else {
		message.BodyError = err.Error()
	}

	return printPUSMessage(header.Humanize(), message, outputFmt)
}

func decodePUSTC(data []byte, profile pus.MissionProfile, outputFmt string) error {
	header := &pus.TCHeader{Profile: profile}
	if err := header.Decode(data); err != nil {
		return fmt.Errorf("decoding the TC header: %w", err)
	}

	body := data[header.Size():]
	message := pusMessageJSON{
		Direction:  "tc",
		Service:    header.Service,
		Subtype:    header.Subtype,
		HeaderSize: header.Size(),
		Body:       hex.EncodeToString(body),
	}

	registry, err := pus.NewDefaultRegistry(profile)
	if err != nil {
		return fmt.Errorf("building the registry: %w", err)
	}
	if request, err := registry.DecodeRequest(header.Key(), body); err == nil {
		message.BodyKnown = true
		message.BodyDetail = humanizeOrEmpty(request)
	} else {
		message.BodyError = err.Error()
	}

	return printPUSMessage(header.Humanize(), message, outputFmt)
}

// humanizer is the optional interface the decoded bodies carry. Not every
// one does, so it is checked rather than required.
type humanizer interface{ Humanize() string }

func humanizeOrEmpty(value any) string {
	if h, ok := value.(humanizer); ok {
		return h.Humanize()
	}
	return ""
}

func printPUSMessage(headerText string, message pusMessageJSON, outputFmt string) error {
	switch outputFmt {
	case "json":
		b, err := json.MarshalIndent(message, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding JSON output: %w", err)
		}
		fmt.Println(string(b))

	case "text":
		fmt.Printf("PUS %s[%d,%d]\n",
			strings.ToUpper(message.Direction), message.Service, message.Subtype)
		fmt.Println(headerText)

		fmt.Printf("Body: %d octets\n", len(message.Body)/2)
		switch {
		case message.BodyDetail != "":
			fmt.Println(message.BodyDetail)
		case message.BodyKnown:
			// Decoded, but the type has nothing to say beyond its fields.
			fmt.Printf("  %s\n", message.Body)
		default:
			// Not implemented, so the octets are all that can honestly be
			// shown. Say why rather than printing them as if understood.
			fmt.Printf("  %s\n", message.Body)
			fmt.Printf("  [not decoded: %s]\n", message.BodyError)
		}

	default:
		return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
	}
	return nil
}
