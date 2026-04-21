package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ravisuhag/astro/pkg/tcf"
	"github.com/spf13/cobra"
)

func timeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time <command>",
		Short: "CCSDS Time Code Format operations",
		Long:  "Encode, decode, and inspect CCSDS time codes (CCSDS 301.0-B-4). Supports CUC, CDS, CCS, and ASCII formats.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		timeDecodeCmd(),
		timeEncodeCmd(),
		timeInspectCmd(),
		timeNowCmd(),
	)

	return cmd
}

// timeJSON is the JSON-serializable representation of a decoded time code.
type timeJSON struct {
	Format string `json:"format"`
	Time   string `json:"time"`
	Hex    string `json:"hex,omitempty"`
	ASCII  string `json:"ascii,omitempty"`

	// CUC fields
	CoarseTime  *uint64 `json:"coarse_time,omitempty"`
	FineTime    *uint64 `json:"fine_time,omitempty"`
	CoarseBytes *uint8  `json:"coarse_bytes,omitempty"`
	FineBytes   *uint8  `json:"fine_bytes,omitempty"`

	// CDS fields
	Day             *uint32 `json:"day,omitempty"`
	Milliseconds    *uint32 `json:"milliseconds,omitempty"`
	Submilliseconds *uint32 `json:"submilliseconds,omitempty"`
	DayBytes        *uint8  `json:"day_bytes,omitempty"`
	SubmsBytes      *uint8  `json:"subms_bytes,omitempty"`

	// CCS fields
	Year       *uint16 `json:"year,omitempty"`
	Month      *uint8  `json:"month,omitempty"`
	DayOfMonth *uint8  `json:"day_of_month,omitempty"`
	DayOfYear  *uint16 `json:"day_of_year,omitempty"`
	Hour       *uint8  `json:"hour,omitempty"`
	Minute     *uint8  `json:"minute,omitempty"`
	Second     *uint8  `json:"second,omitempty"`
	MonthDay   *bool   `json:"month_day,omitempty"`
}

func cucToJSON(c *tcf.CUC, encoded []byte) timeJSON {
	return timeJSON{
		Format:      "CUC",
		Time:        c.Time().UTC().Format(time.RFC3339Nano),
		Hex:         hex.EncodeToString(encoded),
		CoarseTime:  &c.CoarseTime,
		FineTime:    &c.FineTime,
		CoarseBytes: &c.CoarseBytes,
		FineBytes:   &c.FineBytes,
	}
}

func cdsToJSON(c *tcf.CDS, encoded []byte) timeJSON {
	return timeJSON{
		Format:          "CDS",
		Time:            c.Time().UTC().Format(time.RFC3339Nano),
		Hex:             hex.EncodeToString(encoded),
		Day:             &c.Day,
		Milliseconds:    &c.Milliseconds,
		Submilliseconds: &c.Submilliseconds,
		DayBytes:        &c.DayBytes,
		SubmsBytes:      &c.SubmsBytes,
	}
}

func ccsToJSON(c *tcf.CCS, encoded []byte) timeJSON {
	j := timeJSON{
		Format:   "CCS",
		Time:     c.Time().UTC().Format(time.RFC3339Nano),
		Hex:      hex.EncodeToString(encoded),
		Year:     &c.Year,
		Hour:     &c.Hour,
		Minute:   &c.Minute,
		Second:   &c.Second,
		MonthDay: &c.MonthDay,
	}
	if c.MonthDay {
		j.Month = &c.Month
		j.DayOfMonth = &c.DayOfMonth
	} else {
		j.DayOfYear = &c.DayOfYear
	}
	return j
}

func asciiToJSON(s string, t time.Time) timeJSON {
	return timeJSON{
		Format: "ASCII",
		Time:   t.UTC().Format(time.RFC3339Nano),
		ASCII:  s,
	}
}

func timeDecodeCmd() *cobra.Command {
	var (
		inputFmt  string
		outputFmt string
		codec     string
	)

	cmd := &cobra.Command{
		Use:   "decode [file]",
		Short: "Decode a CCSDS time code into a timestamp",
		Long:  "Decode a binary or hex-encoded CCSDS time code (CUC, CDS, or CCS) into a human-readable timestamp.",
		Example: `  # Auto-detect and decode a CUC time code from hex
  echo "1e0c22f380" | astro time decode --input hex

  # Decode a CDS time code
  echo "4400614b4093e0" | astro time decode --codec cds --input hex

  # Decode an ASCII Type A time string
  echo "2025-03-15T12:30:45.123Z" | astro time decode --codec ascii-a

  # Decode with JSON output
  echo "1e0c22f380" | astro time decode --input hex --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// ASCII formats are text input, not hex/bin
			if codec == "ascii-a" || codec == "ascii-b" {
				return decodeASCII(args, codec, outputFmt)
			}

			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			if codec == "" {
				codec, err = detectTimeCodec(data)
				if err != nil {
					return err
				}
			}

			return decodeTimeCode(data, codec, outputFmt)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&codec, "codec", "", "Time code format: cuc, cds, ccs, ascii-a, ascii-b (auto-detect if empty)")

	return cmd
}

func detectTimeCodec(data []byte) (string, error) {
	if len(data) < 1 {
		return "", fmt.Errorf("data too short to detect time code format")
	}
	id := (data[0] >> 4) & 0x07
	switch id {
	case tcf.TimeCodeCUCLevel1, tcf.TimeCodeCUCLevel2:
		return "cuc", nil
	case tcf.TimeCodeCDS:
		return "cds", nil
	case tcf.TimeCodeCCS:
		return "ccs", nil
	default:
		return "", fmt.Errorf("unrecognized time code ID %d in P-field; use --codec to specify format", id)
	}
}

func decodeTimeCode(data []byte, codec, outputFmt string) error {
	switch codec {
	case "cuc":
		c, err := tcf.DecodeCUC(data, time.Time{})
		if err != nil {
			return fmt.Errorf("decoding CUC: %w", err)
		}
		return printTime(c.Humanize(), cucToJSON(c, data), outputFmt)

	case "cds":
		c, err := tcf.DecodeCDS(data, time.Time{})
		if err != nil {
			return fmt.Errorf("decoding CDS: %w", err)
		}
		return printTime(c.Humanize(), cdsToJSON(c, data), outputFmt)

	case "ccs":
		c, err := tcf.DecodeCCS(data)
		if err != nil {
			return fmt.Errorf("decoding CCS: %w", err)
		}
		return printTime(c.Humanize(), ccsToJSON(c, data), outputFmt)

	default:
		return fmt.Errorf("unknown codec: %s (use cuc, cds, ccs, ascii-a, or ascii-b)", codec)
	}
}

func decodeASCII(args []string, codec, outputFmt string) error {
	raw, err := readRawInput(args)
	if err != nil {
		return err
	}
	s := strings.TrimSpace(string(raw))

	var typ string
	switch codec {
	case "ascii-a":
		typ = tcf.ASCIITypeA
	case "ascii-b":
		typ = tcf.ASCIITypeB
	default:
		return fmt.Errorf("unknown ASCII codec: %s", codec)
	}

	a, err := tcf.NewASCIITime(typ)
	if err != nil {
		return err
	}
	t, err := a.Decode(s)
	if err != nil {
		return fmt.Errorf("decoding ASCII time: %w", err)
	}

	textOut := fmt.Sprintf("ASCII Time Code (Type %s):\n  Input: %s\n  Time:  %s", typ, s, t.UTC().Format(time.RFC3339Nano))
	return printTime(textOut, asciiToJSON(s, t), outputFmt)
}

func timeEncodeCmd() *cobra.Command {
	var (
		codec     string
		timestamp string
		outputFmt string
		// CUC options
		coarseBytes uint8
		fineBytes   uint8
		// CDS options
		dayBytes   uint8
		submsBytes uint8
		// CCS options
		monthDay    bool
		subSecBytes uint8
		// ASCII options
		precision int
	)

	cmd := &cobra.Command{
		Use:   "encode",
		Short: "Encode a timestamp into a CCSDS time code",
		Long:  "Convert a timestamp (RFC3339 or 'now') into a CCSDS time code in the specified format.",
		Example: `  # Encode current time as CUC
  astro time encode --codec cuc

  # Encode a specific timestamp as CDS
  astro time encode --codec cds --time "2025-03-15T12:30:45Z"

  # Encode as CCS with month/day variant
  astro time encode --codec ccs --time "2025-03-15T12:30:45Z" --month-day

  # Encode as ASCII Type A with 6 digit precision
  astro time encode --codec ascii-a --time "2025-03-15T12:30:45.123456Z" --precision 6

  # Encode with JSON output
  astro time encode --codec cuc --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseTimestamp(timestamp)
			if err != nil {
				return err
			}

			switch codec {
			case "cuc":
				return encodeCUC(t, coarseBytes, fineBytes, outputFmt)
			case "cds":
				return encodeCDS(t, dayBytes, submsBytes, outputFmt)
			case "ccs":
				return encodeCCS(t, monthDay, subSecBytes, outputFmt)
			case "ascii-a":
				return encodeASCII(t, tcf.ASCIITypeA, precision, outputFmt)
			case "ascii-b":
				return encodeASCII(t, tcf.ASCIITypeB, precision, outputFmt)
			default:
				return fmt.Errorf("unknown codec: %s (use cuc, cds, ccs, ascii-a, or ascii-b)", codec)
			}
		},
	}

	cmd.Flags().StringVar(&codec, "codec", "cuc", "Time code format: cuc, cds, ccs, ascii-a, ascii-b")
	cmd.Flags().StringVar(&timestamp, "time", "now", "Timestamp to encode (RFC3339 or 'now')")
	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text, json, or hex")

	// CUC
	cmd.Flags().Uint8Var(&coarseBytes, "coarse-bytes", 4, "CUC: coarse time octets (1-4)")
	cmd.Flags().Uint8Var(&fineBytes, "fine-bytes", 0, "CUC: fine time octets (0-3)")

	// CDS
	cmd.Flags().Uint8Var(&dayBytes, "day-bytes", 2, "CDS: day segment width (2 or 3)")
	cmd.Flags().Uint8Var(&submsBytes, "subms-bytes", 0, "CDS: sub-millisecond width (0, 2, or 4)")

	// CCS
	cmd.Flags().BoolVar(&monthDay, "month-day", false, "CCS: use month/day variant instead of day-of-year")
	cmd.Flags().Uint8Var(&subSecBytes, "sub-sec-bytes", 0, "CCS: sub-second octets (0-6)")

	// ASCII
	cmd.Flags().IntVar(&precision, "precision", 3, "ASCII: fractional second digits (0-9)")

	return cmd
}

func parseTimestamp(s string) (time.Time, error) {
	if s == "" || strings.ToLower(s) == "now" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp: %w (expected RFC3339 format)", err)
	}
	return t, nil
}

func encodeCUC(t time.Time, coarseBytes, fineBytes uint8, outputFmt string) error {
	opts := []tcf.CUCOption{
		tcf.WithCUCCoarseBytes(coarseBytes),
		tcf.WithCUCFineBytes(fineBytes),
	}
	c, err := tcf.NewCUC(t, opts...)
	if err != nil {
		return fmt.Errorf("encoding CUC: %w", err)
	}
	encoded, err := c.Encode()
	if err != nil {
		return fmt.Errorf("encoding CUC: %w", err)
	}
	return printTimeEncoded(c.Humanize(), cucToJSON(c, encoded), encoded, outputFmt)
}

func encodeCDS(t time.Time, dayBytes, submsBytes uint8, outputFmt string) error {
	opts := []tcf.CDSOption{
		tcf.WithCDSDayBytes(dayBytes),
		tcf.WithCDSSubmsBytes(submsBytes),
	}
	c, err := tcf.NewCDS(t, opts...)
	if err != nil {
		return fmt.Errorf("encoding CDS: %w", err)
	}
	encoded, err := c.Encode()
	if err != nil {
		return fmt.Errorf("encoding CDS: %w", err)
	}
	return printTimeEncoded(c.Humanize(), cdsToJSON(c, encoded), encoded, outputFmt)
}

func encodeCCS(t time.Time, monthDay bool, subSecBytes uint8, outputFmt string) error {
	var opts []tcf.CCSOption
	if monthDay {
		opts = append(opts, tcf.WithCCSMonthDay())
	}
	if subSecBytes > 0 {
		opts = append(opts, tcf.WithCCSSubSecBytes(subSecBytes))
	}
	c, err := tcf.NewCCS(t, opts...)
	if err != nil {
		return fmt.Errorf("encoding CCS: %w", err)
	}
	encoded, err := c.Encode()
	if err != nil {
		return fmt.Errorf("encoding CCS: %w", err)
	}
	return printTimeEncoded(c.Humanize(), ccsToJSON(c, encoded), encoded, outputFmt)
}

func encodeASCII(t time.Time, typ string, precision int, outputFmt string) error {
	a, err := tcf.NewASCIITime(typ, tcf.WithASCIIPrecision(precision))
	if err != nil {
		return fmt.Errorf("encoding ASCII: %w", err)
	}
	s, err := a.Encode(t)
	if err != nil {
		return fmt.Errorf("encoding ASCII: %w", err)
	}

	textOut := fmt.Sprintf("ASCII Time Code (Type %s):\n  %s", typ, s)
	j := asciiToJSON(s, t)

	switch outputFmt {
	case "json":
		b, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "text":
		fmt.Println(textOut)
	case "hex":
		// For ASCII, output the string itself (not hex bytes)
		fmt.Println(s)
	default:
		return fmt.Errorf("unknown format: %s", outputFmt)
	}
	return nil
}

func timeInspectCmd() *cobra.Command {
	var inputFmt, codec string

	cmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a time code with annotated field breakdown",
		Long:  "Display an annotated breakdown of a CCSDS time code showing P-field, T-field segments, and hex dump.",
		Example: `  # Inspect a CUC time code
  echo "1e0c22f380" | astro time inspect --input hex

  # Inspect a CDS time code
  echo "4400614b4093e0" | astro time inspect --codec cds --input hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			if codec == "" {
				codec, err = detectTimeCodec(data)
				if err != nil {
					return err
				}
			}

			return inspectTimeCode(data, codec)
		},
	}

	cmd.Flags().StringVar(&inputFmt, "input", "hex", "Input format: hex or bin")
	cmd.Flags().StringVar(&codec, "codec", "", "Time code format: cuc, cds, ccs (auto-detect if empty)")

	return cmd
}

func inspectTimeCode(data []byte, codec string) error {
	fmt.Println("Time Code Inspector")
	fmt.Println(strings.Repeat("─", 60))

	// P-field
	var pf tcf.PField
	if err := pf.Decode(data); err != nil {
		return fmt.Errorf("decoding P-field: %w", err)
	}
	fmt.Printf("P-Field (%d byte%s)\n", pf.Size(), pluralS(pf.Size()))
	fmt.Printf("  Extension ............ %v\n", pf.Extension)
	fmt.Printf("  Time Code ID ......... %d (%s)\n", pf.TimeCodeID, timeCodeIDName(pf.TimeCodeID))
	fmt.Printf("  Detail Bits .......... 0x%X\n", pf.Detail)
	if pf.Extension {
		fmt.Printf("  Extension Detail ..... 0x%02X\n", pf.ExtDetail)
	}

	fmt.Println(strings.Repeat("─", 60))

	switch codec {
	case "cuc":
		return inspectCUC(data)
	case "cds":
		return inspectCDS(data)
	case "ccs":
		return inspectCCS(data)
	default:
		return fmt.Errorf("unknown codec: %s", codec)
	}
}

func inspectCUC(data []byte) error {
	c, err := tcf.DecodeCUC(data, time.Time{})
	if err != nil {
		return fmt.Errorf("decoding CUC: %w", err)
	}

	level := "Level 1 (CCSDS epoch: 1958-01-01)"
	if c.PField.TimeCodeID == tcf.TimeCodeCUCLevel2 {
		level = "Level 2 (agency-defined epoch)"
	}

	fmt.Println("CUC T-Field")
	fmt.Printf("  Level ................ %s\n", level)
	fmt.Printf("  Coarse Octets ........ %d\n", c.CoarseBytes)
	fmt.Printf("  Fine Octets .......... %d\n", c.FineBytes)
	fmt.Printf("  Coarse Time .......... %d s\n", c.CoarseTime)
	if c.FineBytes > 0 {
		fmt.Printf("  Fine Time ............ %d\n", c.FineTime)
	}
	fmt.Printf("  Resolved Time ........ %s\n", c.Time().UTC().Format(time.RFC3339Nano))

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw (%d bytes)\n", len(data))
	fmt.Print(hexDump(data, "  "))
	return nil
}

func inspectCDS(data []byte) error {
	c, err := tcf.DecodeCDS(data, time.Time{})
	if err != nil {
		return fmt.Errorf("decoding CDS: %w", err)
	}

	fmt.Println("CDS T-Field")
	fmt.Printf("  Day Octets ........... %d\n", c.DayBytes)
	fmt.Printf("  Day .................. %d\n", c.Day)
	fmt.Printf("  Milliseconds ......... %d\n", c.Milliseconds)
	if c.SubmsBytes > 0 {
		label := "Microseconds"
		if c.SubmsBytes == 4 {
			label = "Picoseconds"
		}
		fmt.Printf("  %s ...... %d\n", label, c.Submilliseconds)
	}
	fmt.Printf("  Resolved Time ........ %s\n", c.Time().UTC().Format(time.RFC3339Nano))

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw (%d bytes)\n", len(data))
	fmt.Print(hexDump(data, "  "))
	return nil
}

func inspectCCS(data []byte) error {
	c, err := tcf.DecodeCCS(data)
	if err != nil {
		return fmt.Errorf("decoding CCS: %w", err)
	}

	variant := "Day-of-Year"
	if c.MonthDay {
		variant = "Month-Day"
	}

	fmt.Println("CCS T-Field")
	fmt.Printf("  Variant .............. %s\n", variant)
	fmt.Printf("  Year ................. %d\n", c.Year)
	if c.MonthDay {
		fmt.Printf("  Month ................ %d\n", c.Month)
		fmt.Printf("  Day .................. %d\n", c.DayOfMonth)
	} else {
		fmt.Printf("  Day of Year .......... %d\n", c.DayOfYear)
	}
	fmt.Printf("  Hour ................. %d\n", c.Hour)
	fmt.Printf("  Minute ............... %d\n", c.Minute)
	fmt.Printf("  Second ............... %d\n", c.Second)
	if c.SubSecBytes > 0 {
		fmt.Printf("  Sub-second Octets .... %d\n", c.SubSecBytes)
	}
	fmt.Printf("  Resolved Time ........ %s\n", c.Time().UTC().Format(time.RFC3339Nano))

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Raw (%d bytes)\n", len(data))
	fmt.Print(hexDump(data, "  "))
	return nil
}

func timeNowCmd() *cobra.Command {
	var (
		codec     string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:   "now",
		Short: "Encode the current time in all CCSDS formats",
		Long:  "Display the current UTC time encoded in all supported CCSDS time code formats.",
		Example: `  # Show current time in all formats
  astro time now

  # Show current time in a specific format
  astro time now --codec cuc

  # JSON output
  astro time now --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now().UTC()

			if codec != "" {
				return encodeSingleNow(now, codec, outputFmt)
			}

			return encodeAllNow(now, outputFmt)
		},
	}

	cmd.Flags().StringVar(&codec, "codec", "", "Specific format: cuc, cds, ccs, ascii-a, ascii-b (all if empty)")
	cmd.Flags().StringVar(&outputFmt, "format", "text", "Output format: text or json")

	return cmd
}

func encodeSingleNow(t time.Time, codec, outputFmt string) error {
	switch codec {
	case "cuc":
		return encodeCUC(t, 4, 2, outputFmt)
	case "cds":
		return encodeCDS(t, 2, 0, outputFmt)
	case "ccs":
		return encodeCCS(t, false, 2, outputFmt)
	case "ascii-a":
		return encodeASCII(t, tcf.ASCIITypeA, 3, outputFmt)
	case "ascii-b":
		return encodeASCII(t, tcf.ASCIITypeB, 3, outputFmt)
	default:
		return fmt.Errorf("unknown codec: %s", codec)
	}
}

func encodeAllNow(t time.Time, outputFmt string) error {
	fmt.Printf("Current UTC: %s\n", t.Format(time.RFC3339Nano))
	fmt.Println(strings.Repeat("─", 60))

	// CUC
	cuc, err := tcf.NewCUC(t, tcf.WithCUCCoarseBytes(4), tcf.WithCUCFineBytes(2))
	if err != nil {
		return err
	}
	cucBytes, err := cuc.Encode()
	if err != nil {
		return err
	}

	// CDS
	cds, err := tcf.NewCDS(t)
	if err != nil {
		return err
	}
	cdsBytes, err := cds.Encode()
	if err != nil {
		return err
	}

	// CCS
	ccs, err := tcf.NewCCS(t, tcf.WithCCSSubSecBytes(2))
	if err != nil {
		return err
	}
	ccsBytes, err := ccs.Encode()
	if err != nil {
		return err
	}

	// ASCII
	ascA, _ := tcf.NewASCIITime(tcf.ASCIITypeA)
	ascAStr, _ := ascA.Encode(t)
	ascB, _ := tcf.NewASCIITime(tcf.ASCIITypeB)
	ascBStr, _ := ascB.Encode(t)

	if outputFmt == "json" {
		out := map[string]any{
			"utc":     t.Format(time.RFC3339Nano),
			"cuc":     cucToJSON(cuc, cucBytes),
			"cds":     cdsToJSON(cds, cdsBytes),
			"ccs":     ccsToJSON(ccs, ccsBytes),
			"ascii_a": asciiToJSON(ascAStr, t),
			"ascii_b": asciiToJSON(ascBStr, t),
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("CUC .... %s\n", hex.EncodeToString(cucBytes))
	fmt.Printf("CDS .... %s\n", hex.EncodeToString(cdsBytes))
	fmt.Printf("CCS .... %s\n", hex.EncodeToString(ccsBytes))
	fmt.Printf("ASCII-A  %s\n", ascAStr)
	fmt.Printf("ASCII-B  %s\n", ascBStr)
	return nil
}

func printTime(text string, j timeJSON, outputFmt string) error {
	switch outputFmt {
	case "json":
		b, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "text":
		fmt.Println(text)
	default:
		return fmt.Errorf("unknown format: %s (use 'text' or 'json')", outputFmt)
	}
	return nil
}

func printTimeEncoded(text string, j timeJSON, encoded []byte, outputFmt string) error {
	switch outputFmt {
	case "hex":
		fmt.Println(hex.EncodeToString(encoded))
	case "json":
		b, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "text":
		fmt.Println(text)
	default:
		return fmt.Errorf("unknown format: %s (use 'text', 'json', or 'hex')", outputFmt)
	}
	return nil
}

func readRawInput(args []string) ([]byte, error) {
	return readInput(args, "bin")
}

func timeCodeIDName(id uint8) string {
	switch id {
	case tcf.TimeCodeCUCLevel1:
		return "CUC Level 1"
	case tcf.TimeCodeCUCLevel2:
		return "CUC Level 2"
	case tcf.TimeCodeCDS:
		return "CDS"
	case tcf.TimeCodeCCS:
		return "CCS"
	default:
		return "unknown"
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
