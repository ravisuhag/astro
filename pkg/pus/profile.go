// Package pus implements the ECSS Packet Utilization Standard, PUS-C,
// per ECSS-E-ST-70-41C (15 April 2016).
//
// PUS defines what travels inside a CCSDS Space Packet. Where pkg/spp gives
// you a packet with an application-defined payload, PUS says how that payload
// is laid out: a secondary header naming a service and subtype, then the
// request or report body that pair implies.
//
// The two secondary header types here implement spp.SecondaryHeader, so they
// plug straight into spp.WithSecondaryHeader and spp.WithDecodeSecondaryHeader
// without either package knowing about the other.
//
// PUS is a tailoring standard. Several field widths are declared per mission
// rather than fixed by the text, so every codec in this package takes a
// MissionProfile. There is no package-level state and no implicit default: a
// profile is always passed explicitly, because two missions that disagree
// about widths cannot read each other's packets.
package pus

import "time"

// Version is the TC and TM packet PUS version number for PUS-C, per clauses
// 7.4.3.1c and 7.4.4.1c. Version 0 was the ESA PUS, version 1 was
// ECSS-E-70-41A.
const Version = 2

// Field widths the standard fixes outright, in octets. These are not
// mission-tailorable: Figure 7-7 and Figure 7-9 give them explicit bit counts.
const (
	// SourceIDSize is the TC source ID width (16 bits, Figure 7-9).
	SourceIDSize = 2
	// MessageTypeCounterSize is the TM message type counter width
	// (16 bits, Figure 7-7).
	MessageTypeCounterSize = 2
	// DestinationIDSize is the TM destination ID width (16 bits, Figure 7-7).
	DestinationIDSize = 2
)

// TimeFormat selects how the TM secondary header's absolute time field is
// encoded. Clause 7.4.3.1j leaves the PFC to the mission's time service.
type TimeFormat uint8

const (
	// TimeNone omits the time field. Useful for ground tooling and tests;
	// a flight profile normally carries a time.
	TimeNone TimeFormat = iota
	// TimeCUC encodes a CCSDS Unsegmented Time Code with an implicit P-field,
	// which is what PFC 3 to 46 of Table 7-10 specify: the field carries the
	// coarse and fine octets alone, because the PFC already says how wide they
	// are. This is the usual choice.
	TimeCUC
	// TimeCUCExplicit encodes a CUC that carries its own P-field, which is
	// PFC 0 of Table 7-10: "explicit definition of time format (CUC or CDS),
	// i.e. including the P-field".
	TimeCUCExplicit
	// TimeRaw carries a fixed-width opaque field the mission defines
	// elsewhere. This package moves the bytes without interpreting them.
	TimeRaw
)

// cucPFieldSize is the width of a CUC P-field for the coarse and fine ranges
// this package supports: one octet, since no extension octet is needed.
const cucPFieldSize = 1

// maxProfileHeaderSize is the widest PUS secondary header a MissionProfile may
// describe, in octets.
//
// It is a bound this package chose, not one CCSDS 133.0-B-2 imposes: the Blue
// Book leaves the Packet Secondary Header's length to the mission and caps it
// only through the packet data field maximum. Sixty-three octets is far beyond
// any realistic PUS header, so exceeding it almost always means a mistyped
// profile.
const maxProfileHeaderSize = 63

// String names the time format.
func (t TimeFormat) String() string {
	switch t {
	case TimeNone:
		return "none"
	case TimeCUC:
		return "CUC (implicit P-field)"
	case TimeCUCExplicit:
		return "CUC (explicit P-field)"
	case TimeRaw:
		return "raw"
	default:
		return "unknown"
	}
}

// MissionProfile pins every width that ECSS-E-ST-70-41C leaves to the mission.
// It is a value type and must not be mutated once codecs are using it.
//
// Widths the standard fixes (TC source ID, TM message type counter, TM
// destination ID, all 16 bits) are deliberately absent. They are constants.
type MissionProfile struct {
	// TCSpareBytes and TMSpareBytes pad each secondary header out to the
	// mission's word size. Clauses 7.4.4.1g and 7.4.3.1l make their presence
	// and size a per-application-process declaration.
	TCSpareBytes int
	TMSpareBytes int

	// TimeFormat selects the TM absolute time encoding (clause 7.4.3.1j).
	TimeFormat TimeFormat

	// CUCCoarseBytes and CUCFineBytes size a CUC time field. Used when
	// TimeFormat is TimeCUC or TimeCUCExplicit.
	CUCCoarseBytes int
	CUCFineBytes   int

	// CUCEpoch is the epoch a CUC time counts from. The zero value means the
	// CCSDS 1958 epoch that pkg/tcf defaults to.
	CUCEpoch time.Time

	// TimeRawBytes is the width of an opaque time field. Used only when
	// TimeFormat is TimeRaw.
	TimeRawBytes int

	// StepIDBytes sizes the step ID of the ST[01] progress reports, TM[1,5]
	// and TM[1,6]. Figures 8-5 and 8-6 mark it enumerated without a width.
	StepIDBytes int

	// FailureCodeBytes sizes the failure notice code of the ST[01] failure
	// reports. Figure 8-2 and its siblings mark it enumerated.
	FailureCodeBytes int

	// EventDefinitionIDBytes sizes the ST[05] event definition ID
	// (Figure 8-59, enumerated).
	EventDefinitionIDBytes int

	// Housekeeping widths for ST[03] (Figure 8-21, all enumerated or
	// unsigned integer without a stated width).
	HousekeepingStructureIDBytes int
	ParameterIDBytes             int
	CollectionIntervalBytes      int
	CountBytes                   int

	// RelativeTimeCoarseBytes and RelativeTimeFineBytes size a PTC 10
	// relative time field: the time offsets of ST[11]. Table 7-11's PFC 3 to
	// 18 allow 1 to 4 coarse octets and 0 to 3 fine, and the split is the
	// PFC's, not the absolute time field's. A mission may declare a different
	// width for the two. Zero selects 4 coarse and 0 fine, whole seconds,
	// which is what a schedule shift usually needs.
	RelativeTimeCoarseBytes int
	RelativeTimeFineBytes   int

	// Time-based scheduling widths for ST[11] (Figures 8-91 to 8-110). Every
	// one of these is enumerated or an unsigned integer with no stated width.
	SubScheduleIDBytes    int
	GroupIDBytes          int
	ScheduleCountBytes    int
	ScheduleStatusBytes   int
	TimeWindowTypeBytes   int
	ScheduleSourceIDBytes int
	ScheduleAPIDBytes     int
	ScheduleSeqCountBytes int

	// SupportsSubSchedules and SupportsGroups declare the two capabilities of
	// clause 6.11.4.1. They are not widths but they decide field presence:
	// figures 8-91, 8-93, 8-97, 8-100 and their siblings mark the sub-schedule
	// ID and group ID optional, and what makes them present is the subservice
	// supporting the capability. A decoder that guessed would mis-split every
	// activity in the list.
	SupportsSubSchedules bool
	SupportsGroups       bool

	// On-board monitoring widths for ST[12] (Figures 8-111 to 8-139). All
	// enumerated or unsigned integer with no stated width. The monitored and
	// validity parameter IDs reuse ParameterIDBytes, and the event definition
	// IDs reuse EventDefinitionIDBytes: they name the same things the ST[03]
	// and ST[05] fields name, so a mission that sized them once has sized them.
	PMONIDBytes               int
	FMONIDBytes               int
	MonitorCountBytes         int
	CheckTypeBytes            int
	PMONStatusBytes           int
	PMONCheckingStatusBytes   int
	FMONStatusBytes           int
	FMONProtectionStatusBytes int
	FMONCheckingStatusBytes   int
	MonitoringIntervalBytes   int
	RepetitionNumberBytes     int
	TransitionDelayBytes      int
	MinPMONFailingBytes       int
	DeltaValueCountBytes      int

	// The five ST[12] subservice declarations that decide field presence.
	// Like the ST[11] pair these are not widths, but getting one wrong shifts
	// every field after it.
	//
	// SupportsConditionalChecking is clause 6.12.3.3c: it decides whether a
	// parameter monitoring definition carries a check validity condition
	// (6.12.3.3g item 3).
	//
	// PerDefinitionMonitoringInterval is clause 6.12.3.3d: a subservice uses
	// either one interval for everything or one per definition, and only the
	// second puts an interval in the message (6.12.3.3g item 4).
	//
	// SupportsTransitionDelayChange is clause 6.12.3.8a: it decides whether
	// TM[12,9] leads with the current maximum transition reporting delay
	// (6.12.3.10i item 1).
	//
	// ExpectedValueSpare is clauses 8.12.2.5d, 8.12.2.7e and 8.12.2.9d: a
	// spare as wide as an event definition ID, there so all three check types
	// can share a width (Figure 8-115 note 2).
	//
	// SupportsFMONConditionalChecking is clause 6.12.4.2.1c, the functional
	// twin of SupportsConditionalChecking (6.12.4.2.1f item 2).
	//
	// SupportsMinPMONFailingNumber is clause 6.12.4.2.1d: whether a functional
	// monitoring definition says how many of its checks must fail at once
	// (6.12.4.2.1f item 4).
	//
	// SupportsFMONProtection is clause 6.12.4.6.1a: a protection status exists
	// only for a subservice that can protect definitions.
	SupportsConditionalChecking     bool
	PerDefinitionMonitoringInterval bool
	SupportsTransitionDelayChange   bool
	ExpectedValueSpare              bool
	SupportsFMONConditionalChecking bool
	SupportsMinPMONFailingNumber    bool
	SupportsFMONProtection          bool

	// Function management widths for ST[08] (Figure 8-87).
	//
	// FunctionIDBytes is the width of the fixed character-string that names
	// the function. FunctionArgumentCountBytes sizes N, and
	// FunctionArgumentIDBytes sizes the enumerated argument ID. The standard
	// states no width for any of the three.
	FunctionIDBytes            int
	FunctionArgumentCountBytes int
	FunctionArgumentIDBytes    int

	// APIDBytes sizes the APID field of TC[17,3] and TM[17,4]. Clauses
	// 8.17.2.3 and 8.17.2.4 mark it enumerated without a stated width, so it
	// is mission-tailorable like the other enumerated fields. Zero selects
	// the 2-octet width most missions use.
	APIDBytes int

	// WordSizeBytes is the mission's word size, in octets. Clauses 7.4.3.1l
	// and 7.4.4.1g size the spare fields so each secondary header ends on a
	// word boundary. When non-zero, Validate checks that both header sizes are
	// whole multiples of this value. Zero disables the check, leaving word
	// alignment to the caller-supplied spare widths.
	WordSizeBytes int
}

// DefaultProfile returns a profile using the widths most European missions
// pick: no spare padding, a 6-octet CUC time, and one or two octets for the
// enumerated identifiers.
//
// It is a convenience for tooling and tests, not a standard-mandated default.
// ECSS-E-ST-70-41C states no defaults for these fields; a real mission
// declares them.
func DefaultProfile() MissionProfile {
	return MissionProfile{
		TimeFormat:                   TimeCUC,
		CUCCoarseBytes:               4,
		CUCFineBytes:                 2,
		StepIDBytes:                  2,
		FailureCodeBytes:             2,
		EventDefinitionIDBytes:       2,
		HousekeepingStructureIDBytes: 1,
		ParameterIDBytes:             2,
		CollectionIntervalBytes:      4,
		CountBytes:                   1,
		RelativeTimeCoarseBytes:      4,
		RelativeTimeFineBytes:        0,
		SubScheduleIDBytes:           1,
		GroupIDBytes:                 1,
		ScheduleCountBytes:           1,
		ScheduleStatusBytes:          1,
		TimeWindowTypeBytes:          1,
		ScheduleSourceIDBytes:        2,
		ScheduleAPIDBytes:            2,
		ScheduleSeqCountBytes:        2,
		SupportsSubSchedules:         true,
		SupportsGroups:               true,
		PMONIDBytes:                  2,
		FMONIDBytes:                  2,
		MonitorCountBytes:            1,
		CheckTypeBytes:               1,
		PMONStatusBytes:              1,
		PMONCheckingStatusBytes:      1,
		FMONStatusBytes:              1,
		FMONProtectionStatusBytes:    1,
		FMONCheckingStatusBytes:      1,
		MonitoringIntervalBytes:      4,
		RepetitionNumberBytes:        1,
		TransitionDelayBytes:         2,
		MinPMONFailingBytes:          1,
		DeltaValueCountBytes:         1,
		FunctionIDBytes:              8,
		FunctionArgumentCountBytes:   1,
		FunctionArgumentIDBytes:      1,
		APIDBytes:                    2,
	}
}

// RelativeCoarseSize returns the coarse octets of a relative time field, or 4
// when the profile leaves it zero.
func (p MissionProfile) RelativeCoarseSize() int {
	if p.RelativeTimeCoarseBytes == 0 {
		return 4
	}
	return p.RelativeTimeCoarseBytes
}

// RelativeFineSize returns the fine octets of a relative time field. Zero is
// a real answer here (whole seconds) so there is no default to substitute.
func (p MissionProfile) RelativeFineSize() int { return p.RelativeTimeFineBytes }

// RelativeTimeSize returns the width of a PTC 10 relative time field in
// octets: coarse plus fine, with no P-field, since Table 7-11 makes it
// implicit for every PFC it lists.
func (p MissionProfile) RelativeTimeSize() int {
	return p.RelativeCoarseSize() + p.RelativeFineSize()
}

// SubScheduleIDSize returns the width of an ST[11] sub-schedule ID, or 1 when
// the profile leaves it zero.
func (p MissionProfile) SubScheduleIDSize() int {
	if p.SubScheduleIDBytes == 0 {
		return 1
	}
	return p.SubScheduleIDBytes
}

// GroupIDSize returns the width of an ST[11] group ID, or 1 by default.
func (p MissionProfile) GroupIDSize() int {
	if p.GroupIDBytes == 0 {
		return 1
	}
	return p.GroupIDBytes
}

// ScheduleCountSize returns the width of an ST[11] N field, or 1 by default.
func (p MissionProfile) ScheduleCountSize() int {
	if p.ScheduleCountBytes == 0 {
		return 1
	}
	return p.ScheduleCountBytes
}

// ScheduleStatusSize returns the width of an ST[11] sub-schedule or group
// status field, or 1 by default. Tables 8-3 and 8-4 give both the same two
// values, so one width covers both.
func (p MissionProfile) ScheduleStatusSize() int {
	if p.ScheduleStatusBytes == 0 {
		return 1
	}
	return p.ScheduleStatusBytes
}

// TimeWindowTypeSize returns the width of an ST[11] time window type field, or
// 1 by default.
func (p MissionProfile) TimeWindowTypeSize() int {
	if p.TimeWindowTypeBytes == 0 {
		return 1
	}
	return p.TimeWindowTypeBytes
}

// ScheduleSourceIDSize, ScheduleAPIDSize and ScheduleSeqCountSize return the
// three widths of an ST[11] request ID (Figure 8-92), each defaulting to 2.
func (p MissionProfile) ScheduleSourceIDSize() int {
	if p.ScheduleSourceIDBytes == 0 {
		return 2
	}
	return p.ScheduleSourceIDBytes
}

// ScheduleAPIDSize returns the width of an ST[11] application process ID.
func (p MissionProfile) ScheduleAPIDSize() int {
	if p.ScheduleAPIDBytes == 0 {
		return 2
	}
	return p.ScheduleAPIDBytes
}

// ScheduleSeqCountSize returns the width of an ST[11] sequence count.
func (p MissionProfile) ScheduleSeqCountSize() int {
	if p.ScheduleSeqCountBytes == 0 {
		return 2
	}
	return p.ScheduleSeqCountBytes
}

// ScheduleRequestIDSize returns the width of a whole ST[11] request ID.
func (p MissionProfile) ScheduleRequestIDSize() int {
	return p.ScheduleSourceIDSize() + p.ScheduleAPIDSize() + p.ScheduleSeqCountSize()
}

// The ST[12] field widths. Each returns the profile's value, or the default
// noted on the constant when the profile leaves it zero. No standard states
// any of them.
func (p MissionProfile) PMONIDSize() int { return orDefault(p.PMONIDBytes, 2) }

// FMONIDSize returns the width of a functional monitoring definition ID.
func (p MissionProfile) FMONIDSize() int { return orDefault(p.FMONIDBytes, 2) }

// MonitorCountSize returns the width of an ST[12] N field.
func (p MissionProfile) MonitorCountSize() int { return orDefault(p.MonitorCountBytes, 1) }

// CheckTypeSize returns the width of a check type field.
func (p MissionProfile) CheckTypeSize() int { return orDefault(p.CheckTypeBytes, 1) }

// PMONStatusSize returns the width of a PMON status field.
func (p MissionProfile) PMONStatusSize() int { return orDefault(p.PMONStatusBytes, 1) }

// PMONCheckingStatusSize returns the width of a PMON checking status field.
func (p MissionProfile) PMONCheckingStatusSize() int {
	return orDefault(p.PMONCheckingStatusBytes, 1)
}

// FMONStatusSize returns the width of an FMON status field.
func (p MissionProfile) FMONStatusSize() int { return orDefault(p.FMONStatusBytes, 1) }

// FMONProtectionStatusSize returns the width of an FMON protection status field.
func (p MissionProfile) FMONProtectionStatusSize() int {
	return orDefault(p.FMONProtectionStatusBytes, 1)
}

// FMONCheckingStatusSize returns the width of an FMON checking status field.
func (p MissionProfile) FMONCheckingStatusSize() int {
	return orDefault(p.FMONCheckingStatusBytes, 1)
}

// MonitoringIntervalSize returns the width of a monitoring interval field.
// Clause 6.12.3.3f expresses the interval in on-board parameter minimum
// sampling interval units, not in seconds.
func (p MissionProfile) MonitoringIntervalSize() int {
	return orDefault(p.MonitoringIntervalBytes, 4)
}

// RepetitionNumberSize returns the width of a repetition number field.
func (p MissionProfile) RepetitionNumberSize() int { return orDefault(p.RepetitionNumberBytes, 1) }

// TransitionDelaySize returns the width of a maximum transition reporting
// delay field.
func (p MissionProfile) TransitionDelaySize() int { return orDefault(p.TransitionDelayBytes, 2) }

// MinPMONFailingSize returns the width of a minimum PMON failing number field.
func (p MissionProfile) MinPMONFailingSize() int { return orDefault(p.MinPMONFailingBytes, 1) }

// DeltaValueCountSize returns the width of a number-of-consecutive-delta-values
// field.
func (p MissionProfile) DeltaValueCountSize() int { return orDefault(p.DeltaValueCountBytes, 1) }

// EventDefinitionIDSize returns the width of an event definition ID, shared by
// ST[05] and ST[12].
func (p MissionProfile) EventDefinitionIDSize() int {
	return orDefault(p.EventDefinitionIDBytes, 2)
}

// ParameterIDSize returns the width of an on-board parameter ID, shared by
// ST[03] and ST[12].
func (p MissionProfile) ParameterIDSize() int { return orDefault(p.ParameterIDBytes, 2) }

// orDefault returns width, or fallback when width is zero.
func orDefault(width, fallback int) int {
	if width == 0 {
		return fallback
	}
	return width
}

// FunctionIDSize returns the width of the ST[08] function ID field in octets:
// FunctionIDBytes, or 8 when the profile leaves it zero. Eight is this
// package's choice, not the standard's, figure 8-87 gives the field no width.
func (p MissionProfile) FunctionIDSize() int {
	if p.FunctionIDBytes == 0 {
		return 8
	}
	return p.FunctionIDBytes
}

// FunctionArgumentCountSize returns the width of the ST[08] argument count N,
// or 1 when the profile leaves it zero.
func (p MissionProfile) FunctionArgumentCountSize() int {
	if p.FunctionArgumentCountBytes == 0 {
		return 1
	}
	return p.FunctionArgumentCountBytes
}

// FunctionArgumentIDSize returns the width of the ST[08] argument ID, or 1
// when the profile leaves it zero.
func (p MissionProfile) FunctionArgumentIDSize() int {
	if p.FunctionArgumentIDBytes == 0 {
		return 1
	}
	return p.FunctionArgumentIDBytes
}

// APIDSize returns the width of the ST[17] APID field in octets: APIDBytes,
// or the 2-octet default when the profile leaves it zero.
func (p MissionProfile) APIDSize() int {
	if p.APIDBytes == 0 {
		return 2
	}
	return p.APIDBytes
}

// TimeSize returns the width of the TM absolute time field in octets.
func (p MissionProfile) TimeSize() int {
	switch p.TimeFormat {
	case TimeCUC:
		// PFC 3 to 46: the P-field is implicit, so only the T-field travels.
		return p.CUCCoarseBytes + p.CUCFineBytes
	case TimeCUCExplicit:
		// PFC 0: the P-field travels with the time value.
		return cucPFieldSize + p.CUCCoarseBytes + p.CUCFineBytes
	case TimeRaw:
		return p.TimeRawBytes
	default:
		return 0
	}
}

// TCHeaderSize returns the encoded width of a TC secondary header:
// version and ack flags (1) + service (1) + subtype (1) + source ID (2)
// + spare (Figure 7-9).
func (p MissionProfile) TCHeaderSize() int {
	return 1 + 1 + 1 + SourceIDSize + p.TCSpareBytes
}

// TMHeaderSize returns the encoded width of a TM secondary header:
// version and time reference status (1) + service (1) + subtype (1)
// + message type counter (2) + destination ID (2) + time + spare (Figure 7-7).
func (p MissionProfile) TMHeaderSize() int {
	return 1 + 1 + 1 + MessageTypeCounterSize + DestinationIDSize + p.TimeSize() + p.TMSpareBytes
}

// Validate checks the profile's widths.
//
// Both header sizes must be at least 1 octet and no more than 63. Only the
// lower bound comes from a standard: CCSDS 133.0-B-2 4.1.4.2.1.3 requires the
// Packet Secondary Header to be a whole number of octets, and pkg/spp refuses
// a zero-length one. The Blue Book sets no upper limit at all. The data field
// maximum is the only ceiling it gives. The 63-octet cap here is this
// package's own sanity bound on a mission profile: a PUS secondary header of
// that width already carries an 8-octet time code and 50-odd octets of spare,
// so a larger one is far more likely to be a mistyped profile than a real
// design. Missions that genuinely need more should raise it here.
func (p MissionProfile) Validate() error {
	widths := []int{
		p.TCSpareBytes, p.TMSpareBytes, p.CUCCoarseBytes, p.CUCFineBytes,
		p.TimeRawBytes, p.StepIDBytes, p.FailureCodeBytes,
		p.EventDefinitionIDBytes, p.HousekeepingStructureIDBytes,
		p.ParameterIDBytes, p.CollectionIntervalBytes, p.CountBytes,
		p.RelativeTimeCoarseBytes, p.RelativeTimeFineBytes,
		p.SubScheduleIDBytes, p.GroupIDBytes, p.ScheduleCountBytes,
		p.ScheduleStatusBytes, p.TimeWindowTypeBytes,
		p.ScheduleSourceIDBytes, p.ScheduleAPIDBytes, p.ScheduleSeqCountBytes,
		p.PMONIDBytes, p.FMONIDBytes, p.MonitorCountBytes,
		p.CheckTypeBytes, p.PMONStatusBytes, p.PMONCheckingStatusBytes,
		p.FMONStatusBytes, p.FMONProtectionStatusBytes, p.FMONCheckingStatusBytes,
		p.MonitoringIntervalBytes, p.RepetitionNumberBytes,
		p.TransitionDelayBytes, p.MinPMONFailingBytes, p.DeltaValueCountBytes,
		p.FunctionIDBytes, p.FunctionArgumentCountBytes,
		p.FunctionArgumentIDBytes,
		p.APIDBytes, p.WordSizeBytes,
	}
	for _, w := range widths {
		if w < 0 {
			return ErrInvalidProfile
		}
		// Every enumerated field is read into a uint64.
		if w > 8 {
			return ErrInvalidProfile
		}
	}

	switch p.TimeFormat {
	case TimeNone:
	case TimeCUC, TimeCUCExplicit:
		// Clause 7.4.3.1j and Table 7-10 cap coarse time at 4 octets for PUS;
		// pkg/tcf accepts 1 to 4 coarse and 0 to 3 fine.
		if p.CUCCoarseBytes < 1 || p.CUCCoarseBytes > 4 {
			return ErrInvalidProfile
		}
		if p.CUCFineBytes < 0 || p.CUCFineBytes > 3 {
			return ErrInvalidProfile
		}
	case TimeRaw:
		if p.TimeRawBytes < 1 {
			return ErrInvalidProfile
		}
	default:
		return ErrUnsupportedTimeFormat
	}

	// Table 7-11's PFC 3 to 18: coarse is (PFC+1)/4 and fine is (PFC+1) mod 4,
	// so coarse runs 1 to 4 and fine runs 0 to 3, the same bounds Table 7-10
	// gives the absolute field.
	if p.RelativeCoarseSize() < 1 || p.RelativeCoarseSize() > 4 {
		return ErrInvalidProfile
	}
	if p.RelativeFineSize() < 0 || p.RelativeFineSize() > 3 {
		return ErrInvalidProfile
	}

	// A mission-profile sanity bound, not a CCSDS 133.0-B-2 rule; see the
	// comment on Validate.
	if p.TCHeaderSize() > maxProfileHeaderSize || p.TMHeaderSize() > maxProfileHeaderSize {
		return ErrHeaderTooLarge
	}

	// Clauses 7.4.3.1l and 7.4.4.1g size the spare fields so each secondary
	// header ends on a word boundary. A declared word size makes that
	// checkable; zero leaves it to the caller.
	if p.WordSizeBytes > 0 {
		if p.TCHeaderSize()%p.WordSizeBytes != 0 || p.TMHeaderSize()%p.WordSizeBytes != 0 {
			return ErrHeaderNotWordAligned
		}
	}
	return nil
}

// putUint writes v big-endian across width octets, refusing a value that does
// not fit. A width of zero writes nothing, which is how absent optional fields
// are handled.
func putUint(dst []byte, v uint64, width int) ([]byte, error) {
	if width == 0 {
		return dst, nil
	}
	if width < 8 && v >= uint64(1)<<(8*width) {
		return nil, ErrValueTooLarge
	}
	for i := width - 1; i >= 0; i-- {
		dst = append(dst, byte(v>>(8*i)))
	}
	return dst, nil
}

// readUint reads a big-endian unsigned integer of the given width.
func readUint(data []byte, width int) (uint64, error) {
	if width == 0 {
		return 0, nil
	}
	if len(data) < width {
		return 0, ErrDataTooShort
	}
	var v uint64
	for i := 0; i < width; i++ {
		v = v<<8 | uint64(data[i])
	}
	return v, nil
}

// readUintList reads a count-prefixed list of big-endian unsigned integers and
// returns the values plus how many octets were consumed.
//
// The count is untrusted input, so it is checked before anything is allocated:
// a count the remaining octets cannot satisfy is refused, and a non-zero count
// over a zero-width element is refused outright, otherwise a hostile count
// would drive an unbounded allocation that consumes no input at all.
func readUintList(data []byte, countWidth, elemWidth int) ([]uint64, int, error) {
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, 0, err
	}
	offset := countWidth
	if count == 0 {
		return nil, offset, nil
	}
	if elemWidth == 0 {
		return nil, 0, ErrInvalidProfile
	}
	// Division rather than multiplication, so a huge count cannot overflow
	// its way past the bound.
	if uint64(len(data)-offset)/uint64(elemWidth) < count {
		return nil, 0, ErrDataTooShort
	}

	list := make([]uint64, 0, count)
	for i := uint64(0); i < count; i++ {
		v, err := readUint(data[offset:], elemWidth)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, v)
		offset += elemWidth
	}
	return list, offset, nil
}
