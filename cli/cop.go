package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/spf13/cobra"
)

func copCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cop <command>",
		Short: "COP-1 communications operation procedure",
		Long: "Build and read the Communications Link Control Word (CCSDS 232.1-B-2).\n\n" +
			"The CLCW is the part of COP-1 that travels on the wire: FARM-1 on the spacecraft generates it and it comes back in a telemetry frame's Operational Control Field, where it tells FOP-1 on the ground what the receiver has accepted.\n\n" +
			"FOP-1 and FARM-1 themselves are state machines driven by a session, not by a single invocation, so there is nothing here for them. Use the library.",
		Annotations: map[string]string{
			"group": "protocol",
		},
	}

	cmd.AddCommand(
		copCLCWEncodeCmd(),
		copCLCWDecodeCmd(),
	)
	return cmd
}

func copCLCWEncodeCmd() *cobra.Command {
	var (
		outputFmt    string
		scid         uint16
		vcid         uint8
		reportValue  uint8
		noRF         bool
		noBitLock    bool
		lockout      bool
		wait         bool
		retransmit   bool
		farmBCounter uint8
	)

	cmd := &cobra.Command{
		Use:   "clcw-encode",
		Short: "Build a CLCW from fields",
		Long:  "Construct a four-octet Communications Link Control Word, ready to go in a telemetry frame's Operational Control Field.",
		Example: `  # A nominal CLCW reporting the next expected frame
  astro cop clcw-encode --scid 42 --vcid 0 --report-value 7

  # The receiver has no buffer, so it sets the wait flag
  astro cop clcw-encode --scid 42 --vcid 0 --report-value 7 --wait

  # Put it in a TM frame's OCF
  astro tm encode --scid 42 --vcid 0 --data 0102 \
    --ocf "$(astro cop clcw-encode --scid 42 --vcid 0 --report-value 7)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clcw := &cop.CLCW{
				// COP In Effect is 01 for COP-1 (CCSDS 232.1-B-2 table 4-1).
				// It is the only value this package's FOP-1 and FARM-1
				// implement, so it is set rather than offered as a flag.
				COPInEffect:       1,
				VirtualChannelID:  vcid,
				ReportValue:       reportValue,
				NoRFAvailableFlag: noRF,
				NoBitLockFlag:     noBitLock,
				LockoutFlag:       lockout,
				WaitFlag:          wait,
				RetransmitFlag:    retransmit,
				FARMBCounter:      farmBCounter,
			}
			if err := clcw.Validate(); err != nil {
				return fmt.Errorf("building the CLCW: %w", err)
			}

			encoded, err := clcw.Encode()
			if err != nil {
				return fmt.Errorf("encoding the CLCW: %w", err)
			}

			out := cmd.OutOrStdout()
			switch outputFmt {
			case "hex":
				_, _ = fmt.Fprintln(out, hex.EncodeToString(encoded))
			case "text":
				_, _ = fmt.Fprintln(out, clcw.Humanize())
				_, _ = fmt.Fprintln(out, hex.EncodeToString(encoded))
			default:
				return fmt.Errorf("unknown format: %s (use 'text' or 'hex')", outputFmt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFmt, "format", "hex", "Output format: text or hex")
	cmd.Flags().Uint16Var(&scid, "scid", 0, "Spacecraft ID, for the text summary only")
	cmd.Flags().Uint8Var(&vcid, "vcid", 0, "Virtual Channel ID this CLCW reports on (0-63)")
	cmd.Flags().Uint8Var(&reportValue, "report-value", 0,
		"Next frame sequence number the receiver expects, N(R)")
	cmd.Flags().BoolVar(&noRF, "no-rf", false, "Set the No RF Available flag")
	cmd.Flags().BoolVar(&noBitLock, "no-bit-lock", false, "Set the No Bit Lock flag")
	cmd.Flags().BoolVar(&lockout, "lockout", false, "Set the Lockout flag")
	cmd.Flags().BoolVar(&wait, "wait", false, "Set the Wait flag: the receiver has no buffer")
	cmd.Flags().BoolVar(&retransmit, "retransmit", false, "Set the Retransmit flag")
	cmd.Flags().Uint8Var(&farmBCounter, "farm-b-counter", 0, "FARM-B counter (2 bits)")
	return cmd
}

func copCLCWDecodeCmd() *cobra.Command {
	var inputFmt, outputFmt string

	cmd := &cobra.Command{
		Use:   "clcw-decode [file]",
		Short: "Decode a CLCW",
		Long: "Decode a four-octet Communications Link Control Word and print its fields.\n\n" +
			"This is what you want when a telemetry frame's Operational Control Field is not saying what you expected: it shows every flag that decides how FOP-1 will react.",
		Example: `  # Decode an OCF
  astro cop clcw-decode --input hex < ocf.hex

  # Pull it out of a TM frame first
  astro tm decode --input hex --format json < frame.hex`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args, inputFmt)
			if err != nil {
				return err
			}

			var clcw cop.CLCW
			if err := clcw.Decode(data); err != nil {
				return fmt.Errorf("decoding the CLCW: %w", err)
			}

			switch outputFmt {
			case "json":
				b, err := json.MarshalIndent(clcwJSON{
					ControlWordType:   clcw.ControlWordType,
					Version:           clcw.Version,
					StatusField:       clcw.StatusField,
					COPInEffect:       clcw.COPInEffect,
					VirtualChannelID:  clcw.VirtualChannelID,
					NoRFAvailableFlag: clcw.NoRFAvailableFlag,
					NoBitLockFlag:     clcw.NoBitLockFlag,
					LockoutFlag:       clcw.LockoutFlag,
					WaitFlag:          clcw.WaitFlag,
					RetransmitFlag:    clcw.RetransmitFlag,
					FARMBCounter:      clcw.FARMBCounter,
					ReportValue:       clcw.ReportValue,
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("encoding JSON output: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			case "text":
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), clcw.Humanize())
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

// clcwJSON is the JSON-serializable form of a CLCW.
type clcwJSON struct {
	ControlWordType   uint8 `json:"control_word_type"`
	Version           uint8 `json:"version"`
	StatusField       uint8 `json:"status_field"`
	COPInEffect       uint8 `json:"cop_in_effect"`
	VirtualChannelID  uint8 `json:"virtual_channel_id"`
	NoRFAvailableFlag bool  `json:"no_rf_available"`
	NoBitLockFlag     bool  `json:"no_bit_lock"`
	LockoutFlag       bool  `json:"lockout"`
	WaitFlag          bool  `json:"wait"`
	RetransmitFlag    bool  `json:"retransmit"`
	FARMBCounter      uint8 `json:"farm_b_counter"`
	ReportValue       uint8 `json:"report_value"`
}
