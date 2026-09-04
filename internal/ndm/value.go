package ndm

import (
	"strconv"
	"strings"
	"time"

	"github.com/ravisuhag/astro/pkg/tcf"
)

// maxDigits is the ceiling clauses 7.5.6 and 7.5.7 put on how many decimal
// digits a non-integer value may carry.
const maxDigits = 16

// SplitUnits separates a value from an optional unit suffix.
//
// Clause 7.7.1.1 allows units to follow a value "for documentation purposes
// and clarity only": at least one blank, then the unit in square brackets. The
// unit is not data — two messages carrying the same number mean the same thing
// whether or not one of them spells out '[km]'.
//
// Clause 7.7.1.3 forbids the literal '[n/a]', which is what the keyword tables
// print for a dimensionless item. It means "this item has no unit", not "write
// n/a here".
func SplitUnits(value string) (number, units string, err error) {
	open := strings.IndexByte(value, '[')
	if open < 0 {
		return value, "", nil
	}
	if !strings.HasSuffix(value, "]") {
		return "", "", ErrMalformedUnits
	}
	// Clause 7.7.1.1(a): at least one blank between the value and the bracket.
	if open == 0 || value[open-1] != ' ' {
		return "", "", ErrMalformedUnits
	}

	units = value[open+1 : len(value)-1]
	if units == "n/a" {
		return "", "", ErrUnitsNotApplicable
	}
	return strings.TrimSpace(value[:open]), units, nil
}

// ParseInt reads an integer value (clause 7.5.4): decimal digits with an
// optional leading sign, leading zeroes allowed, bounded to signed 32 bits.
func ParseInt(value string) (int32, error) {
	if err := checkNoBlanks(value); err != nil {
		return 0, err
	}
	if value == "" {
		return 0, ErrNotAnInteger
	}

	digits := value
	if digits[0] == '+' || digits[0] == '-' {
		digits = digits[1:]
	}
	if digits == "" {
		return 0, ErrNotAnInteger
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return 0, ErrNotAnInteger
		}
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		// Only a range failure can get here; the digits were checked above.
		return 0, ErrIntegerOutOfRange
	}
	// Clause 7.5.4 states the range explicitly rather than leaving it to the
	// reader's word size.
	if n < -2147483648 || n > 2147483647 {
		return 0, ErrIntegerOutOfRange
	}
	return int32(n), nil
}

// FormatInt writes an integer value.
func FormatInt(v int32) string { return strconv.FormatInt(int64(v), 10) }

// ParseFloat reads a non-integer numeric value: either the fixed-point form of
// clause 7.5.6 or the floating-point form of clause 7.5.7.
//
// One part of clause 7.5.7 is deliberately not enforced. Sub-clause (b) says
// the mantissa must carry its decimal point "in the second position of the
// ASCII string", which would allow 1.5E+03 and refuse 15.0E+02 or 0.15E+04.
// Messages in the wild break that constantly, and the worked examples in the
// standard's own annex G are not all normalized either. Reading is lenient;
// FormatFloat writes the conforming form.
func ParseFloat(value string) (float64, error) {
	if err := checkNoBlanks(value); err != nil {
		return 0, err
	}
	if value == "" {
		return 0, ErrNotANumber
	}
	if err := checkDigitCount(value); err != nil {
		return 0, err
	}

	// strconv accepts forms the clause does not: hexadecimal floats, Inf, NaN
	// and an underscore digit separator. Screen those out first so that a
	// value the standard cannot express is not silently read.
	for i := 0; i < len(value); i++ {
		switch c := value[i]; {
		case c >= '0' && c <= '9':
		case c == '+' || c == '-' || c == '.':
		case c == 'e' || c == 'E':
		default:
			return 0, ErrNotANumber
		}
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, ErrNotANumber
	}
	return f, nil
}

// FormatFloat writes a non-integer value in fixed-point notation with the
// given number of fractional digits (clause 7.5.6).
func FormatFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// FormatScientific writes a non-integer value in the floating-point notation
// of clause 7.5.7, with the decimal point in the second position of the
// mantissa as sub-clause (b) requires.
func FormatScientific(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'E', decimals, 64)
}

// checkDigitCount enforces the 16-digit ceiling of clauses 7.5.6 and 7.5.7.
// Only the mantissa counts: the exponent is a separate integer under
// sub-clause (d).
func checkDigitCount(value string) error {
	mantissa := value
	if i := strings.IndexAny(value, "eE"); i >= 0 {
		mantissa = value[:i]
	}

	digits := 0
	for i := 0; i < len(mantissa); i++ {
		if mantissa[i] >= '0' && mantissa[i] <= '9' {
			digits++
		}
	}
	if digits > maxDigits {
		return ErrTooManyDigits
	}
	return nil
}

// checkNoBlanks enforces clause 7.5.8, which forbids whitespace inside a
// numeric value or a time string.
func checkNoBlanks(value string) error {
	if strings.ContainsAny(value, " \t") {
		return ErrBlankInValue
	}
	return nil
}

// ParseText reads a free-text value under clause 7.5.9: an underscore stands
// for a single blank, a single blank stays significant, and a run of blanks
// collapses to one.
//
// The underscore rule is why a spacecraft called MARS_PATHFINDER and one
// called MARS PATHFINDER are the same object. It also means a name that really
// does contain an underscore cannot be written.
func ParseText(value string) string {
	value = strings.ReplaceAll(value, "_", " ")

	var b strings.Builder
	b.Grow(len(value))
	lastWasBlank := false
	for i := 0; i < len(value); i++ {
		if value[i] == ' ' {
			if !lastWasBlank {
				b.WriteByte(' ')
			}
			lastWasBlank = true
			continue
		}
		lastWasBlank = false
		b.WriteByte(value[i])
	}
	return b.String()
}

// ParseEpoch reads an absolute time (clause 7.5.10).
//
// Both forms are allowed in one file, so which one this is comes from the
// value rather than from configuration: a calendar date carries two hyphens
// before the 'T', an ordinal date one. The parsing itself is pkg/tcf's, which
// implements the same ASCII time codes A and B under CCSDS 301.0-B-4
// clause 3.5 — including second 60 at a leap second, which clause 7.5.10 calls
// out in its note and which Go's own time package refuses.
func ParseEpoch(value string) (time.Time, error) {
	if err := checkNoBlanks(value); err != nil {
		return time.Time{}, err
	}

	kind, err := epochType(value)
	if err != nil {
		return time.Time{}, err
	}
	codec, err := tcf.NewASCIITime(kind)
	if err != nil {
		return time.Time{}, err
	}
	t, err := codec.Decode(value)
	if err != nil {
		return time.Time{}, ErrNotAnEpoch
	}
	return t, nil
}

// epochType decides whether a time string is ASCII time code A or B.
func epochType(value string) (string, error) {
	t := strings.IndexByte(value, 'T')
	if t < 0 {
		return "", ErrNotAnEpoch
	}
	switch strings.Count(value[:t], "-") {
	case 2:
		return tcf.ASCIITypeA, nil
	case 1:
		return tcf.ASCIITypeB, nil
	}
	return "", ErrNotAnEpoch
}

// FormatEpoch writes an absolute time as ASCII time code A with the given
// number of fractional second digits, which is the form the worked examples in
// every one of these standards use.
func FormatEpoch(t time.Time, decimals int) (string, error) {
	codec, err := tcf.NewASCIITime(tcf.ASCIITypeA, tcf.WithASCIIPrecision(decimals))
	if err != nil {
		return "", err
	}
	return codec.Encode(t)
}
