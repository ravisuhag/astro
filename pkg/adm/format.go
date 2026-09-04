package adm

import (
	"strconv"
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// maxValueDigits is the ceiling clause 6.5 puts on the digits of a non-integer
// value, matching the ODM's clause 7.5.
const maxValueDigits = 16

// parseEpoch reads an absolute time, tolerating a unit suffix that no epoch
// should carry.
func parseEpoch(raw string) (time.Time, error) {
	value, _, err := ndm.SplitUnits(raw)
	if err != nil {
		return time.Time{}, err
	}
	return ndm.ParseEpoch(value)
}

// parseValue reads a number that may carry a unit suffix. Clause 6.6 makes the
// units documentation rather than data, exactly as the ODM's clause 7.7 does,
// and the worked examples in annex G write them on most values.
func parseValue(raw string) (float64, error) {
	value, _, err := ndm.SplitUnits(raw)
	if err != nil {
		return 0, err
	}
	return ndm.ParseFloat(value)
}

// formatValue writes a number in fixed-point notation, falling back to
// floating-point when the fixed form would exceed the digit ceiling.
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

// epochPrecision picks how many fractional second digits an epoch needs.
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

// fieldSet reads the values a block collected, remembering the first failure so
// that a caller can check once at the end rather than after every field.
type fieldSet struct {
	fields map[string]string
	err    error
}

func newFieldSet(fields map[string]string) *fieldSet {
	return &fieldSet{fields: fields}
}

// has reports whether the block carried a keyword.
func (f *fieldSet) has(keyword string) bool {
	_, ok := f.fields[keyword]
	return ok
}

// num returns a numeric value, recording an error for a keyword that is
// absent when it was required.
func (f *fieldSet) num(keyword string, required bool) float64 {
	raw, ok := f.fields[keyword]
	if !ok {
		if required && f.err == nil {
			f.err = ErrMissingKeyword
		}
		return 0
	}
	v, err := parseValue(raw)
	if err != nil && f.err == nil {
		f.err = err
	}
	return v
}

// epoch returns a time value.
func (f *fieldSet) epoch(keyword string, required bool) time.Time {
	raw, ok := f.fields[keyword]
	if !ok {
		if required && f.err == nil {
			f.err = ErrMissingKeyword
		}
		return time.Time{}
	}
	t, err := parseEpoch(raw)
	if err != nil && f.err == nil {
		f.err = err
	}
	return t
}

// require records a missing mandatory string.
func (f *fieldSet) require(keyword string) string {
	if !f.has(keyword) && f.err == nil {
		f.err = ErrMissingKeyword
	}
	return f.fields[keyword]
}
