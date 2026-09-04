package tdm_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/tdm"
)

// Figure E-19 of CCSDS 503.0-B-2, transcribed: two-way range with ranging
// power to spectral density, from a DSN station.
//
// The RANGE_UNITS here are RU, not kilometres. That is the whole reason this
// example is worth pinning: nothing in a RANGE record says what its number is
// in, and reading 65249.677 as kilometres instead of range units is wrong by
// orders of magnitude.
const figureE19 = `CCSDS_TDM_VERS = 2.0
COMMENT CREATED BY TTC PGM V33.0.2
CREATION_DATE = 2010-050T20:15:02.000
ORIGINATOR = NASA/JPL/DSN

META_START
COMMENT SEQUENTIAL RANGE
COMMENT RANGE IS ADJUSTED FOR CORRECTION_RANGE; MEASUREMENT MINUS CORRECTION_RANGE
TIME_SYSTEM = UTC
START_TIME = 2010-215T20:04:24.000
STOP_TIME = 2010-215T20:53:24.000
PARTICIPANT_1 = DSS-14
PARTICIPANT_2 = CAS
MODE = SEQUENTIAL
PATH = 1,2,1
TRANSMIT_BAND = X
RECEIVE_BAND = X
TURNAROUND_NUMERATOR = 880
TURNAROUND_DENOMINATOR = 749
TIMETAG_REF = RECEIVE
INTEGRATION_REF = START
RANGE_MODE = COHERENT
RANGE_MODULUS = 262144
RANGE_UNITS = RU
TRANSMIT_DELAY_1 = 2.1E-07
RECEIVE_DELAY_1 = 2.1E-07
DATA_QUALITY = VALIDATED
CORRECTION_RANGE = 4999.392714
CORRECTIONS_APPLIED = YES
META_STOP

DATA_START
RANGE = 2010-215T20:04:24.000   65249.6771931631
PR_N0 = 2010-215T20:04:24.000   30.2351
RANGE = 2010-215T20:11:24.000   52234.4753877508
PR_N0 = 2010-215T20:11:24.000   32.7846
RANGE = 2010-215T20:53:24.000   64457.0270879461
PR_N0 = 2010-215T20:53:24.000   30.0224
DATA_STOP
`

func TestDecode(t *testing.T) {
	m, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if m.Header.Version != "2.0" || m.Header.Originator != "NASA/JPL/DSN" {
		t.Errorf("header = %+v", m.Header)
	}
	if len(m.Header.Comments) != 1 {
		t.Errorf("header comments = %q, want 1", m.Header.Comments)
	}
	// 2010 day 50 is 2010-02-19.
	wantCreated := time.Date(2010, 2, 19, 20, 15, 2, 0, time.UTC)
	if !m.Header.CreationDate.Equal(wantCreated) {
		t.Errorf("CreationDate = %v, want %v", m.Header.CreationDate, wantCreated)
	}

	if len(m.Segments) != 1 {
		t.Fatalf("read %d segments, want 1", len(m.Segments))
	}
	s := m.Segments[0]

	md := s.Metadata
	if md.TimeSystem() != "UTC" {
		t.Errorf("TimeSystem = %q, want UTC", md.TimeSystem())
	}
	// The units the measurements are in, which no record carries.
	if md.RangeUnits() != "RU" {
		t.Errorf("RangeUnits = %q, want RU", md.RangeUnits())
	}
	modulus, ok := md.RangeModulus()
	if !ok || modulus != 262144 {
		t.Errorf("RangeModulus = %v, %v, want 262144, true", modulus, ok)
	}
	if md.Mode() != "SEQUENTIAL" || md.Path() != "1,2,1" {
		t.Errorf("mode/path = %q / %q", md.Mode(), md.Path())
	}

	participants := md.Participants()
	if len(participants) != 2 || participants[1] != "DSS-14" || participants[2] != "CAS" {
		t.Errorf("participants = %v", participants)
	}

	start, hasStart := md.StartTime()
	if !hasStart {
		t.Fatal("START_TIME was not read")
	}
	wantStart := time.Date(2010, 8, 3, 20, 4, 24, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("StartTime = %v, want %v", start, wantStart)
	}

	// Every keyword the section carried is kept, in order, including the ones
	// this package has no accessor for.
	if delay, ok := md.Get("TRANSMIT_DELAY_1"); !ok || delay != "2.1E-07" {
		t.Errorf("TRANSMIT_DELAY_1 = %q, %v", delay, ok)
	}
	if len(md.Fields) != 21 {
		t.Errorf("read %d metadata fields, want 21", len(md.Fields))
	}
	if len(md.Comments) != 2 {
		t.Errorf("metadata comments = %q, want 2", md.Comments)
	}

	if len(s.Observations) != 6 {
		t.Fatalf("read %d observations, want 6", len(s.Observations))
	}
	first := s.Observations[0]
	if first.Keyword != "RANGE" || first.Value != 65249.6771931631 {
		t.Errorf("first observation = %+v", first)
	}
	if !first.Epoch.Equal(wantStart) {
		t.Errorf("first epoch = %v, want %v", first.Epoch, wantStart)
	}
	if second := s.Observations[1]; second.Keyword != "PR_N0" || second.Value != 30.2351 {
		t.Errorf("second observation = %+v", second)
	}

	if got := m.Observations(); got != 6 {
		t.Errorf("Observations = %d, want 6", got)
	}
}

// Clause 3.5.2.7 makes km the default when a segment gives no RANGE_UNITS,
// and says the keyword "should always be specified". A segment that meant RU
// and forgot to say so reads as km without complaint, which is why the
// accessor reports the default rather than an empty string.
func TestRangeUnitsDefault(t *testing.T) {
	input := strings.Replace(figureE19, "RANGE_UNITS = RU\n", "", 1)

	m, err := tdm.Decode([]byte(input))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	md := m.Segments[0].Metadata

	if got := md.RangeUnits(); got != tdm.DefaultRangeUnits {
		t.Errorf("RangeUnits = %q, want %q", got, tdm.DefaultRangeUnits)
	}
	if _, given := md.Get("RANGE_UNITS"); given {
		t.Error("Get reports RANGE_UNITS as present when the segment omitted it")
	}
	// And the summary says the value was defaulted rather than stated.
	if !strings.Contains(md.Humanize(), "defaulted") {
		t.Errorf("Humanize does not flag the defaulted units:\n%s", md.Humanize())
	}
}

// Clause 3.3.1.4 requires a new segment whenever a metadata value changes, so
// a mode change is a segment boundary rather than a keyword in the data.
func TestDecodeMultipleSegments(t *testing.T) {
	second := `
META_START
TIME_SYSTEM = UTC
PARTICIPANT_1 = DSS-14
PARTICIPANT_2 = CAS
MODE = SINGLE_DIFF
RANGE_UNITS = km
META_STOP

DATA_START
RANGE = 2010-215T21:00:00.000   1234.5
DATA_STOP
`
	m, err := tdm.Decode([]byte(figureE19 + second))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(m.Segments) != 2 {
		t.Fatalf("read %d segments, want 2", len(m.Segments))
	}
	// The two segments disagree about the units, which is exactly the reason
	// they are separate segments.
	if a, b := m.Segments[0].Metadata.RangeUnits(), m.Segments[1].Metadata.RangeUnits(); a != "RU" || b != "km" {
		t.Errorf("segment units = %q and %q, want RU and km", a, b)
	}
	if got := m.Observations(); got != 7 {
		t.Errorf("Observations = %d, want 7", got)
	}
}

func TestRoundTrip(t *testing.T) {
	first, err := tdm.Decode([]byte(figureE19))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	encoded, err := first.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	second, err := tdm.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode on our own output: %v\n%s", err, encoded)
	}

	if len(second.Segments) != len(first.Segments) {
		t.Fatalf("segment count changed: %d then %d", len(first.Segments), len(second.Segments))
	}
	if second.Observations() != first.Observations() {
		t.Errorf("observation count changed: %d then %d", first.Observations(), second.Observations())
	}
	for i := range first.Segments {
		a, b := first.Segments[i], second.Segments[i]
		// The metadata must survive whole: a dropped keyword changes what the
		// measurements mean.
		if len(a.Metadata.Fields) != len(b.Metadata.Fields) {
			t.Errorf("segment %d metadata field count changed: %d then %d",
				i, len(a.Metadata.Fields), len(b.Metadata.Fields))
		}
		for j := range a.Metadata.Fields {
			if a.Metadata.Fields[j] != b.Metadata.Fields[j] {
				t.Errorf("segment %d field %d changed: %+v then %+v",
					i, j, a.Metadata.Fields[j], b.Metadata.Fields[j])
			}
		}
		for j := range a.Observations {
			if a.Observations[j] != b.Observations[j] {
				t.Errorf("segment %d observation %d changed: %+v then %+v",
					i, j, a.Observations[j], b.Observations[j])
			}
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{
			name:  "no segment at all",
			input: "CCSDS_TDM_VERS = 2.0\nCREATION_DATE = 2010-050T20:15:02\nORIGINATOR = X\n",
			want:  tdm.ErrNoSegment,
		},
		{
			name:  "no TIME_SYSTEM",
			input: strings.Replace(figureE19, "TIME_SYSTEM = UTC\n", "", 1),
			want:  tdm.ErrMissingTimeSystem,
		},
		{
			name: "no participant",
			input: strings.NewReplacer(
				"PARTICIPANT_1 = DSS-14\n", "",
				"PARTICIPANT_2 = CAS\n", "",
			).Replace(figureE19),
			want: tdm.ErrMissingParticipant,
		},
		{
			name:  "a participant index past 5",
			input: strings.Replace(figureE19, "PARTICIPANT_2 = CAS", "PARTICIPANT_6 = CAS", 1),
			want:  tdm.ErrUnknownKeyword,
		},
		{
			name:  "a metadata keyword no table lists",
			input: strings.Replace(figureE19, "MODE = SEQUENTIAL", "NOT_A_KEYWORD = 1", 1),
			want:  tdm.ErrUnknownKeyword,
		},
		{
			name:  "a data keyword no table lists",
			input: strings.Replace(figureE19, "PR_N0 = 2010-215T20:04:24.000   30.2351", "NOPE = 2010-215T20:04:24.000 1.0", 1),
			want:  tdm.ErrUnknownKeyword,
		},
		{
			name:  "a record with no measurement",
			input: strings.Replace(figureE19, "RANGE = 2010-215T20:04:24.000   65249.6771931631", "RANGE = 2010-215T20:04:24.000", 1),
			want:  tdm.ErrMalformedRecord,
		},
		{
			name:  "a record with three fields",
			input: strings.Replace(figureE19, "RANGE = 2010-215T20:04:24.000   65249.6771931631", "RANGE = 2010-215T20:04:24.000 1.0 2.0", 1),
			want:  tdm.ErrMalformedRecord,
		},
		{
			name:  "a metadata section that never closes",
			input: "CCSDS_TDM_VERS = 2.0\nCREATION_DATE = 2010-050T20:15:02\nORIGINATOR = X\nMETA_START\nTIME_SYSTEM = UTC\n",
			want:  tdm.ErrUnterminatedBlock,
		},
		{
			name:  "a metadata section with no data section",
			input: strings.Replace(figureE19, "DATA_START\n", "", 1),
			want:  tdm.ErrMissingDataSection,
		},
		{
			name: "an empty data section",
			input: strings.NewReplacer(
				"RANGE = 2010-215T20:04:24.000   65249.6771931631\n", "",
				"PR_N0 = 2010-215T20:04:24.000   30.2351\n", "",
				"RANGE = 2010-215T20:11:24.000   52234.4753877508\n", "",
				"PR_N0 = 2010-215T20:11:24.000   32.7846\n", "",
				"RANGE = 2010-215T20:53:24.000   64457.0270879461\n", "",
				"PR_N0 = 2010-215T20:53:24.000   30.0224\n", "",
			).Replace(figureE19),
			want: tdm.ErrNoRecords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tdm.Decode([]byte(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("Decode = %v, want %v", err, tt.want)
			}
		})
	}
}

// The indexed data keywords go up to 5 (table 3-5), and RECEIVE_FREQ is legal
// both bare and indexed while the transmit family is indexed only.
func TestIndexedDataKeywords(t *testing.T) {
	accepted := []string{
		"RECEIVE_FREQ", "RECEIVE_FREQ_1", "RECEIVE_FREQ_5",
		"TRANSMIT_FREQ_1", "TRANSMIT_FREQ_RATE_1", "TRANSMIT_PHASE_CT_2",
		"RECEIVE_PHASE_CT_3",
	}
	for _, keyword := range accepted {
		input := strings.Replace(figureE19,
			"PR_N0 = 2010-215T20:04:24.000   30.2351",
			keyword+" = 2010-215T20:04:24.000 1.0", 1)
		if _, err := tdm.Decode([]byte(input)); err != nil {
			t.Errorf("%s was refused: %v", keyword, err)
		}
	}

	refused := []string{"RECEIVE_FREQ_6", "RECEIVE_FREQ_0", "TRANSMIT_FREQ"}
	for _, keyword := range refused {
		input := strings.Replace(figureE19,
			"PR_N0 = 2010-215T20:04:24.000   30.2351",
			keyword+" = 2010-215T20:04:24.000 1.0", 1)
		if _, err := tdm.Decode([]byte(input)); !errors.Is(err, tdm.ErrUnknownKeyword) {
			t.Errorf("%s = %v, want ErrUnknownKeyword", keyword, err)
		}
	}
}
