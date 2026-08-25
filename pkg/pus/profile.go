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
// Widths the standard fixes — TC source ID, TM message type counter, TM
// destination ID, all 16 bits — are deliberately absent. They are constants.
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
		APIDBytes:                    2,
	}
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
// Both header sizes must land inside the 1-to-63 octet window CCSDS 133.0-B-2
// allows for a secondary header, since pkg/spp enforces it.
func (p MissionProfile) Validate() error {
	widths := []int{
		p.TCSpareBytes, p.TMSpareBytes, p.CUCCoarseBytes, p.CUCFineBytes,
		p.TimeRawBytes, p.StepIDBytes, p.FailureCodeBytes,
		p.EventDefinitionIDBytes, p.HousekeepingStructureIDBytes,
		p.ParameterIDBytes, p.CollectionIntervalBytes, p.CountBytes,
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

	if p.TCHeaderSize() > 63 || p.TMHeaderSize() > 63 {
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
// over a zero-width element is refused outright — otherwise a hostile count
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
