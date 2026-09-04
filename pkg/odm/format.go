package odm

import (
	"strconv"
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// formatValue writes a number in the fixed-point notation of
// CCSDS 502.0-B-3 clause 7.5.6.
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
func formatValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if countDigits(s) > 16 {
		return strconv.FormatFloat(v, 'E', -1, 64)
	}
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
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

// parseValue reads a number that may carry a unit suffix. Clause 7.7.1.1 makes
// the units documentation, not data, so they are read and discarded — a
// message that spells out '[km]' and one that does not say the same thing.
func parseValue(raw string) (float64, error) {
	number, _, err := ndm.SplitUnits(raw)
	if err != nil {
		return 0, err
	}
	return ndm.ParseFloat(number)
}

// parseEpochValue reads an absolute time (clause 7.5.10).
func parseEpochValue(raw string) (time.Time, error) {
	return ndm.ParseEpoch(raw)
}

// epochPrecision picks how many fractional second digits an epoch needs, so a
// whole second is not padded with zeroes and a fractional one is not truncated.
// Clause 7.5.10 allows as many digits as the required precision needs.
func epochPrecision(t time.Time) int {
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
