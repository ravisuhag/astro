package tdm

import (
	"strconv"
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// Header is the TDM header (table 3-2).
//
// It is the shared navigation message header minus one field: table 3-2 has no
// CLASSIFICATION, which the orbit and attitude messages both carry.
type Header struct {
	Version      string
	Comments     []string
	CreationDate time.Time
	Originator   string
	MessageID    string
}

var headerSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_TDM_VERS",
	Classification: ndm.Absent,
	MessageFor:     ndm.Absent,
	MessageID:      ndm.Optional,
}

// Block delimiters (clauses 3.3.1.5 and 3.4.7).
const (
	keywordMetaStart = "META_START"
	keywordMetaStop  = "META_STOP"
	keywordDataStart = "DATA_START"
	keywordDataStop  = "DATA_STOP"
)

// Metadata keywords whose value changes how a measurement must be read.
const (
	KeywordTimeSystem   = "TIME_SYSTEM"
	KeywordRangeUnits   = "RANGE_UNITS"
	KeywordRangeModulus = "RANGE_MODULUS"
	KeywordAngleType    = "ANGLE_TYPE"
	KeywordMode         = "MODE"
	KeywordPath         = "PATH"
	KeywordStartTime    = "START_TIME"
	KeywordStopTime     = "STOP_TIME"
)

// DefaultRangeUnits is what clause 3.5.2.7 says to assume when a segment gives
// no RANGE_UNITS. The clause calls km the preferred value and says the keyword
// "should always be specified" — a range in RU read as km is wrong by orders
// of magnitude, and nothing in the record says which it is.
const DefaultRangeUnits = "km"

// Field is one metadata assignment, kept in the order it arrived.
type Field struct {
	Keyword string
	Value   string
}

// Metadata is one segment's metadata section (table 3-3).
//
// It is held as an ordered list rather than a struct of named fields, because
// that is what it is: table 3-3 marks only TIME_SYSTEM and PARTICIPANT_n
// mandatory and leaves the other forty-odd keywords optional, most of them
// describing station configuration whose meaning lives in an interface control
// document. A struct would be forty pointers, and a caller reading an
// unfamiliar keyword would have no way to see it.
//
// The accessors below cover the keywords that change how a number must be
// read. Everything else is reachable through Get.
type Metadata struct {
	Comments []string
	Fields   []Field
}

// Get returns the value of a keyword and whether the section carried it.
func (md Metadata) Get(keyword string) (string, bool) {
	for _, f := range md.Fields {
		if f.Keyword == keyword {
			return f.Value, true
		}
	}
	return "", false
}

// TimeSystem is the scale every timetag in the segment is on. Table 3-3 makes
// it the one mandatory metadata keyword.
func (md Metadata) TimeSystem() string {
	v, _ := md.Get(KeywordTimeSystem)
	return v
}

// RangeUnits is what a RANGE observation in this segment is measured in: km,
// s or RU. When the segment does not say, clause 3.5.2.7 makes it km.
func (md Metadata) RangeUnits() string {
	if v, ok := md.Get(KeywordRangeUnits); ok {
		return v
	}
	return DefaultRangeUnits
}

// RangeModulus is the ambiguity interval, and whether the segment gave one.
//
// A non-zero modulus means the RANGE observations are ambiguous: clause
// 3.5.2.7 says such a value "does not represent the actual range to the
// spacecraft" until a calculation using the modulus has been done. This
// package does not do that calculation, so a caller that ignores this is
// reading a number that is not the range.
func (md Metadata) RangeModulus() (float64, bool) {
	v, ok := md.Get(KeywordRangeModulus)
	if !ok {
		return 0, false
	}
	modulus, err := ndm.ParseFloat(v)
	if err != nil {
		return 0, false
	}
	return modulus, true
}

// AngleType says what ANGLE_1 and ANGLE_2 mean in this segment — azimuth and
// elevation, right ascension and declination, and so on. Without it the two
// angles are a pair of numbers with no frame.
func (md Metadata) AngleType() string {
	v, _ := md.Get(KeywordAngleType)
	return v
}

// Mode is the tracking mode: SEQUENTIAL, SINGLE_DIFF and the rest.
func (md Metadata) Mode() string {
	v, _ := md.Get(KeywordMode)
	return v
}

// Path is the signal path through the participants, such as "1,2,1" for a
// two-way measurement that left participant 1, turned around at 2 and came
// back to 1.
func (md Metadata) Path() string {
	v, _ := md.Get(KeywordPath)
	return v
}

// Participants returns the tracking participants by index. Table 3-3 indexes
// them 1 to 5 so that the keywords referring to a participant by number stay
// unambiguous.
func (md Metadata) Participants() map[int]string {
	out := make(map[int]string)
	for _, f := range md.Fields {
		if index, ok := participantIndex(f.Keyword); ok {
			out[index] = f.Value
		}
	}
	return out
}

// StartTime and StopTime bound the segment, when it says so. Both are optional
// in table 3-3, unlike the OEM's.
func (md Metadata) StartTime() (time.Time, bool) { return md.epoch(KeywordStartTime) }
func (md Metadata) StopTime() (time.Time, bool)  { return md.epoch(KeywordStopTime) }

func (md Metadata) epoch(keyword string) (time.Time, bool) {
	v, ok := md.Get(keyword)
	if !ok {
		return time.Time{}, false
	}
	t, err := ndm.ParseEpoch(v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// participantIndex reports the index of a PARTICIPANT_n keyword.
func participantIndex(keyword string) (int, bool) {
	suffix, ok := strings.CutPrefix(keyword, "PARTICIPANT_")
	if !ok {
		return 0, false
	}
	index, err := strconv.Atoi(suffix)
	if err != nil {
		return 0, false
	}
	return index, true
}

// Observation is one Tracking Data Record: a data type keyword, the time the
// observable belongs to, and the observable itself (clause 3.4.1).
//
// The units are not here. They come from the keyword and, for RANGE, from the
// segment's RANGE_UNITS — see Metadata.RangeUnits.
type Observation struct {
	Keyword string
	Epoch   time.Time
	Value   float64
}

// Segment is a metadata section with the data section that follows it
// (clause 3.1.2).
//
// Clause 3.3.1.4 requires a new segment whenever any metadata value changes,
// so a segment boundary marks a change in how the tracking was done — a switch
// from one-way to two-way, a different band, a different station.
type Segment struct {
	Metadata Metadata
	// Comments at the head of the data section, after DATA_START.
	Comments     []string
	Observations []Observation
}

// TDM is a Tracking Data Message (CCSDS 503.0-B-2).
type TDM struct {
	Header   Header
	Segments []Segment
}

// Observations reports how many tracking data records the message holds.
func (m *TDM) Observations() int {
	n := 0
	for _, s := range m.Segments {
		n += len(s.Observations)
	}
	return n
}

// Validate checks the message against the rules section 3 states.
func (m *TDM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if len(m.Segments) == 0 {
		return ErrNoSegment
	}

	for i := range m.Segments {
		s := &m.Segments[i]

		if s.Metadata.TimeSystem() == "" {
			return ErrMissingTimeSystem
		}
		if len(s.Metadata.Participants()) == 0 {
			return ErrMissingParticipant
		}
		for index := range s.Metadata.Participants() {
			if index < 1 || index > 5 {
				return ErrParticipantIndex
			}
		}
		if len(s.Observations) == 0 {
			return ErrNoRecords
		}
	}
	return nil
}
