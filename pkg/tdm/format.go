package tdm

import (
	"strconv"
	"strings"
	"time"
)

// maxValueDigits is the ceiling clause 4.3 puts on the digits of a non-integer
// value, matching the ODM's clause 7.5.
const maxValueDigits = 16

// formatValue writes a measurement in the fixed-point notation clause 4.3
// defines, falling back to floating-point when the fixed form would run past
// the digit ceiling.
func formatValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	if countDigits(s) <= maxValueDigits {
		return s
	}

	scientific := strconv.FormatFloat(v, 'E', -1, 64)
	mantissa, exponent, found := strings.Cut(scientific, "E")
	if !found {
		return scientific
	}
	if !strings.ContainsRune(mantissa, '.') {
		mantissa += ".0"
	}
	return mantissa + "E" + exponent
}

func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n
}

// epochPrecision picks how many fractional second digits a timetag needs, so
// a whole second is not padded and a fractional one is not truncated.
func epochPrecision(t time.Time) int {
	nanos := t.Nanosecond()
	if nanos == 0 {
		return 0
	}
	digits := 9
	for nanos%10 == 0 {
		nanos /= 10
		digits--
	}
	return digits
}
