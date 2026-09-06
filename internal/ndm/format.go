package ndm

import (
	"strconv"
	"strings"
	"time"
)

// maxValueDigits is the ceiling clauses 7.5.6 and 7.5.7 put on how many
// decimal digits a non-integer value may carry. It matches maxDigits in
// value.go, which enforces the same ceiling on the way in; this is the
// mirror check on the way out.
const maxValueDigits = 16

// FormatValue writes a number in the fixed-point notation of CCSDS 502.0-B-3
// clause 7.5.6, the rule the ODM, ADM, CDM and TDM all restate for their own
// values.
//
// The shortest form that reads back as the same float64 is used, so a value
// that arrived as 6503.514000 goes out as 6503.514. Clause 7.5.6 makes
// trailing zeroes optional and clauses 7.4.5 to 7.4.7 make the surrounding
// white space insignificant, so the two files carry the same message. There is
// no way to preserve the original spelling without storing the original text
// beside every number, and a message is defined by its values.
//
// Two rules do have to be honoured. Clause 7.5.6 requires at least one digit
// on each side of the decimal point, so a whole number gains a '.0'. And the
// 16-digit ceiling means a value whose fixed-point form would run past it
// falls back to the floating-point notation of clause 7.5.7.
func FormatValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	// Count after the '.0' is on, not before it. A whole number with sixteen
	// digits is already at the ceiling, and the trailing zero clause 7.5.6
	// forces onto it puts it over — which this package's own reader then
	// refuses. Found by FuzzDecodeOPM.
	if countDigits(s) <= maxValueDigits {
		return s
	}
	return formatScientific(v)
}

// formatScientific writes a number in the floating-point notation of
// clause 7.5.7, with a decimal point in the mantissa as sub-clause (b) asks.
// strconv leaves it out for a mantissa of one digit, giving 1E+15 where the
// clause wants 1.0E+15.
func formatScientific(v float64) string {
	s := strconv.FormatFloat(v, 'E', -1, 64)

	mantissa, exponent, found := strings.Cut(s, "E")
	if !found {
		return s
	}
	if !strings.ContainsRune(mantissa, '.') {
		mantissa += ".0"
	}
	return mantissa + "E" + exponent
}

// countDigits counts the decimal digits in a formatted number.
func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n
}

// ParseValue reads a number that may carry a unit suffix. Clause 7.7.1.1 makes
// the units documentation, not data, so they are read and discarded — a
// message that spells out '[km]' and one that does not say the same thing.
//
// This applies to numeric values only. ParseEpoch does not call this: an
// epoch is not one of the items tables 3-3/4-3 assign a unit to (see the
// commentary on ParseEpoch in value.go), so a unit suffix on a time string is
// rejected rather than stripped.
func ParseValue(raw string) (float64, error) {
	number, _, err := SplitUnits(raw)
	if err != nil {
		return 0, err
	}
	return ParseFloat(number)
}

// EpochPrecision picks how many fractional second digits an epoch needs, so a
// whole second is not padded with zeroes and a fractional one is not
// truncated. Clause 7.5.10 allows as many digits as the required precision
// needs.
func EpochPrecision(t time.Time) int {
	nanos := t.Nanosecond()
	if nanos == 0 {
		return 0
	}
	// Trim trailing zeroes: 117200000 ns is 4 digits, not 9.
	digits := 9
	for nanos%10 == 0 {
		nanos /= 10
		digits--
	}
	return digits
}
