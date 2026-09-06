package ndm

import (
	"errors"
	"math"
	"testing"
	"time"
)

// Clause 7.5.4 states the integer range in the text rather than leaving it to
// the reader's word size, so the bounds are pinned here.
func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"0", 0},
		{"42", 42},
		{"+42", 42},
		{"-42", -42},
		{"007", 7}, // clause 7.5.4 allows leading zeroes
		{"2147483647", 1<<31 - 1},
		{"-2147483648", -1 << 31},
	}
	for _, tt := range tests {
		got, err := ParseInt(tt.input)
		if err != nil {
			t.Errorf("ParseInt(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseIntRejects(t *testing.T) {
	tests := []struct {
		input string
		want  error
	}{
		{"", ErrNotAnInteger},
		{"+", ErrNotAnInteger},
		{"1.0", ErrNotAnInteger},
		{"1e3", ErrNotAnInteger},
		{"abc", ErrNotAnInteger},
		{"1 2", ErrBlankInValue},
		{"2147483648", ErrIntegerOutOfRange},
		{"-2147483649", ErrIntegerOutOfRange},
	}
	for _, tt := range tests {
		if _, err := ParseInt(tt.input); !errors.Is(err, tt.want) {
			t.Errorf("ParseInt(%q) = %v, want %v", tt.input, err, tt.want)
		}
	}
}

// The values here are transcribed from the worked examples in annex G of
// CCSDS 502.0-B-3, so they are shapes real messages actually carry.
func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"6503.514000", 6503.514},
		{"-717.490000", -717.49},
		{"0.020842611", 0.020842611},
		{"398600.4415", 398600.4415},
		{"-0.00101495", -0.00101495},
		{"1.5E+03", 1500},
		{"1.5e-03", 0.0015},
		{"+2.5", 2.5},
		{"3", 3}, // clause 7.5.7(c): an omitted exponent means zero
	}
	for _, tt := range tests {
		got, err := ParseFloat(tt.input)
		if err != nil {
			t.Errorf("ParseFloat(%q): %v", tt.input, err)
			continue
		}
		if math.Abs(got-tt.want) > 1e-12*math.Max(1, math.Abs(tt.want)) {
			t.Errorf("ParseFloat(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// strconv reads more than clause 7.5 allows. Those extra forms must not slip
// through, because a value the standard cannot express is a value the far end
// will not understand.
func TestParseFloatRejectsFormsTheClauseCannotExpress(t *testing.T) {
	tests := []struct {
		input string
		want  error
	}{
		{"", ErrNotANumber},
		{"Inf", ErrNotANumber},
		{"NaN", ErrNotANumber},
		{"0x1p-2", ErrNotANumber},
		{"1_000.0", ErrNotANumber},
		{"1 000.0", ErrBlankInValue},
		{"12345678901234567.0", ErrTooManyDigits},
	}
	for _, tt := range tests {
		if _, err := ParseFloat(tt.input); !errors.Is(err, tt.want) {
			t.Errorf("ParseFloat(%q) = %v, want %v", tt.input, err, tt.want)
		}
	}
}

// The 16-digit ceiling counts the mantissa. Clause 7.5.7(d) makes the exponent
// a separate integer, so its digits do not count against the same budget.
func TestDigitCeilingCountsTheMantissaOnly(t *testing.T) {
	if _, err := ParseFloat("1.234567890123456E+308"); err != nil {
		t.Errorf("a 16-digit mantissa with a 3-digit exponent was refused: %v", err)
	}
	if _, err := ParseFloat("1.2345678901234567E+3"); !errors.Is(err, ErrTooManyDigits) {
		t.Errorf("a 17-digit mantissa was accepted")
	}
}

// Clause 7.7.1.1 lets a value carry its units for readability. The units are
// not data, and clause 7.7.1.3 forbids the '[n/a]' the tables print for a
// dimensionless item.
func TestSplitUnits(t *testing.T) {
	tests := []struct {
		input, number, units string
	}{
		{"6655.9942 [km]", "6655.9942", "km"},
		{"41399.5123            [km]", "41399.5123", "km"},
		{"3.11548208    [km/s]", "3.11548208", "km/s"},
		{"398600.4415            [km**3/s**2]", "398600.4415", "km**3/s**2"},
		{"0.020842611", "0.020842611", ""},
	}
	for _, tt := range tests {
		number, units, err := SplitUnits(tt.input)
		if err != nil {
			t.Errorf("SplitUnits(%q): %v", tt.input, err)
			continue
		}
		if number != tt.number || units != tt.units {
			t.Errorf("SplitUnits(%q) = %q, %q, want %q, %q", tt.input, number, units, tt.number, tt.units)
		}
	}
}

func TestSplitUnitsRejects(t *testing.T) {
	tests := []struct {
		input string
		want  error
	}{
		{"6655.9942 [km", ErrMalformedUnits},
		{"6655.9942[km]", ErrMalformedUnits}, // clause 7.7.1.1(a) wants a blank
		{"[km]", ErrMalformedUnits},
		{"1.0 [n/a]", ErrUnitsNotApplicable},
	}
	for _, tt := range tests {
		if _, _, err := SplitUnits(tt.input); !errors.Is(err, tt.want) {
			t.Errorf("SplitUnits(%q) = %v, want %v", tt.input, err, tt.want)
		}
	}
}

// Clause 7.5.9 makes an underscore stand for a blank and collapses runs of
// blanks. It is why MARS_PATHFINDER and MARS PATHFINDER are the same object,
// and why a name that really contains an underscore cannot be written.
func TestParseText(t *testing.T) {
	tests := []struct{ input, want string }{
		{"MARS_PATHFINDER", "MARS PATHFINDER"},
		{"MARS PATHFINDER", "MARS PATHFINDER"},
		{"EUTELSAT  W4", "EUTELSAT W4"},
		{"A___B", "A B"},
		{"OSPREY 5", "OSPREY 5"},
	}
	for _, tt := range tests {
		if got := ParseText(tt.input); got != tt.want {
			t.Errorf("ParseText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// A mandatory field's value must survive being written back out, and a value
// that is nothing but blanks does not: an encoder writes it verbatim, so it
// comes back as an empty field. ParseTextRequired is what a mandatory field
// calls instead of ParseText, so it must refuse a lone underscore and any
// other value that collapses to nothing, while still accepting anything with
// real content, blanks included.
func TestParseTextRequired(t *testing.T) {
	blank := []string{"_", "", "___", "   "}
	for _, input := range blank {
		if _, err := ParseTextRequired(input); !errors.Is(err, ErrBlankTextValue) {
			t.Errorf("ParseTextRequired(%q) = %v, want ErrBlankTextValue", input, err)
		}
	}

	ok := []struct{ input, want string }{
		{"MARS_PATHFINDER", "MARS PATHFINDER"},
		{"MARS PATHFINDER", "MARS PATHFINDER"},
		{"_TEST_", " TEST "},
		{"OSPREY 5", "OSPREY 5"},
	}
	for _, tt := range ok {
		got, err := ParseTextRequired(tt.input)
		if err != nil {
			t.Errorf("ParseTextRequired(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTextRequired(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Clause 7.5.10 allows a calendar date or an ordinal one, with or without the
// Z terminator, in the same file. Which one a value is comes from the value.
func TestParseEpoch(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"2022-12-18T14:28:15.1172", time.Date(2022, 12, 18, 14, 28, 15, 117200000, time.UTC)},
		{"2001-11-06T11:17:33", time.Date(2001, 11, 6, 11, 17, 33, 0, time.UTC)},
		{"2002-204T15:56:23Z", time.Date(2002, 7, 23, 15, 56, 23, 0, time.UTC)},
		{"2006-001T00:00:00Z", time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"2021-06-03T05:33:00.000", time.Date(2021, 6, 3, 5, 33, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := ParseEpoch(tt.input)
		if err != nil {
			t.Errorf("ParseEpoch(%q): %v", tt.input, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseEpoch(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// The note under clause 7.5.10 says the seconds field is 60 during a leap
// second. Go's own time package refuses that, so this is the kind of value an
// implementation silently gets wrong.
func TestParseEpochAtALeapSecond(t *testing.T) {
	got, err := ParseEpoch("2016-12-31T23:59:60Z")
	if err != nil {
		t.Fatalf("ParseEpoch at a leap second: %v", err)
	}
	want := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseEpoch = %v, want %v", got, want)
	}
}

func TestParseEpochRejects(t *testing.T) {
	for _, input := range []string{
		"",
		"2022-12-18",          // no time part
		"20221218T142815",     // no separators
		"2022-12-18 14:28:15", // clause 7.5.8 forbids the blank
		"2022-13-18T14:28:15", // month out of range
		"2022-999T14:28:15",   // day of year out of range
		"2022-12-18T25:00:00", // hour out of range
	} {
		if _, err := ParseEpoch(input); err == nil {
			t.Errorf("ParseEpoch(%q) was accepted", input)
		}
	}
}

// TestParseEpochRejectsAUnitSuffix pins the decision that an epoch never
// carries a unit suffix (see the commentary on ParseEpoch above).
//
// Clause 7.5.10 spells out the epoch grammar
// (YYYY-MM-DDThh:mm:ss[.d→d][Z], or the ordinal form) with no unit in it.
// Clause 7.7.1.1's unit allowance is scoped to "OPM/OMM UNITS" and ties any
// suffix to the units named for that keyword in tables 3-3/4-3; the EPOCH row
// of both tables leaves its Units column blank, so a suffix has nothing to
// match. A time string followed by " [s]" is therefore not a variant
// spelling of a valid epoch — it is a different, invalid value, and it must
// be rejected rather than silently accepted by stripping the suffix.
func TestParseEpochRejectsAUnitSuffix(t *testing.T) {
	const bare = "2022-12-18T14:28:15Z"
	if _, err := ParseEpoch(bare); err != nil {
		t.Fatalf("ParseEpoch(%q): %v", bare, err)
	}

	const suffixed = "2022-12-18T14:28:15Z [s]"
	if _, err := ParseEpoch(suffixed); err == nil {
		t.Errorf("ParseEpoch(%q) was accepted; clause 7.5.10 has no unit suffix in the epoch grammar", suffixed)
	}
}

func TestFormatEpochRoundTrips(t *testing.T) {
	want := time.Date(2022, 12, 18, 14, 28, 15, 117200000, time.UTC)

	s, err := FormatEpoch(want, 4)
	if err != nil {
		t.Fatalf("FormatEpoch: %v", err)
	}
	if s != "2022-12-18T14:28:15.1172Z" {
		t.Errorf("FormatEpoch = %q", s)
	}

	got, err := ParseEpoch(s)
	if err != nil {
		t.Fatalf("ParseEpoch: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("round trip = %v, want %v", got, want)
	}
}
