package tcf

import (
	"fmt"
	"time"
)

// ASCII time code format types per CCSDS 301.0-B-4 §3.5.
const (
	// ASCIITypeA is the calendar date-time format: YYYY-MM-DDThh:mm:ss.dddZ
	ASCIITypeA = "A"
	// ASCIITypeB is the ordinal date-time format: YYYY-DDDThh:mm:ss.dddZ
	ASCIITypeB = "B"
)

// ASCIITime represents a CCSDS ASCII time code per CCSDS 301.0-B-4 §3.5.
//
// Type A: YYYY-MM-DDThh:mm:ss.d...dZ (calendar date-time)
// Type B: YYYY-DDDThh:mm:ss.d...dZ   (ordinal date-time)
//
// These are fixed-field subsets of ISO 8601: every field has a fixed width,
// the separators are mandatory, the fraction (if present) is 1-9 digits, and
// the trailing Z is optional. Decode enforces the subset strictly.
type ASCIITime struct {
	Type      string // "A" or "B"
	Precision int    // Number of decimal digits for fractional seconds (0-9)
}

// ASCIIOption configures an ASCII time code.
type ASCIIOption func(*ASCIITime) error

// WithASCIIPrecision sets the number of fractional second digits (0-9).
func WithASCIIPrecision(n int) ASCIIOption {
	return func(a *ASCIITime) error {
		if n < 0 || n > 9 {
			return ErrInvalidCalendarTime
		}
		a.Precision = n
		return nil
	}
}

// NewASCIITime creates an ASCIITime encoder/decoder.
// typ must be ASCIITypeA or ASCIITypeB.
// Defaults to 3 digits of fractional seconds.
func NewASCIITime(typ string, opts ...ASCIIOption) (*ASCIITime, error) {
	if typ != ASCIITypeA && typ != ASCIITypeB {
		return nil, ErrInvalidASCIIFormat
	}

	a := &ASCIITime{
		Type:      typ,
		Precision: 3,
	}

	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	return a, nil
}

// Encode formats a Go time.Time value into a CCSDS ASCII time string.
func (a *ASCIITime) Encode(t time.Time) (string, error) {
	t = t.UTC()
	if t.Year() < 0 || t.Year() > 9999 {
		return "", ErrInvalidCalendarTime
	}

	var base string
	if a.Type == ASCIITypeA {
		base = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d",
			t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	} else {
		base = fmt.Sprintf("%04d-%03dT%02d:%02d:%02d",
			t.Year(), t.YearDay(), t.Hour(), t.Minute(), t.Second())
	}

	if a.Precision > 0 {
		frac := t.Nanosecond()
		// Scale to desired precision
		divisor := 1
		for range 9 - a.Precision {
			divisor *= 10
		}
		fracVal := frac / divisor
		base += "." + fmt.Sprintf("%0*d", a.Precision, fracVal)
	}

	return base + "Z", nil
}

// parseFixedDigits parses exactly len(s) decimal digits. Unlike
// strconv.Atoi it rejects signs, spaces, and any non-digit byte, enforcing
// the fixed-width fields of the §3.5 subsets.
func parseFixedDigits(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	v := 0
	for i := 0; i < len(s); i++ {
		d := s[i]
		if d < '0' || d > '9' {
			return 0, false
		}
		v = v*10 + int(d-'0')
	}
	return v, true
}

// Decode parses a CCSDS ASCII time string into a Go time.Time value.
//
// The §3.5 subsets are enforced strictly: fixed field widths
// (Type A "YYYY-MM-DDThh:mm:ss[.d...d][Z]", Type B
// "YYYY-DDDThh:mm:ss[.d...d][Z]"), digits only, mandatory separators, a
// fraction of 1-9 digits when present, and value ranges checked against the
// calendar (month 1-12, day valid for the month and year, day-of-year valid
// for the year's leap status, hour 0-23, minute 0-59).
//
// Second 60 is accepted only at 23:59:60 (a positive leap second); because
// Go's time.Time cannot represent second 60, the returned value is
// normalized to 00:00:00 of the following day.
func (a *ASCIITime) Decode(s string) (time.Time, error) {
	// Optional Z terminator per §3.5
	if len(s) > 0 && s[len(s)-1] == 'Z' {
		s = s[:len(s)-1]
	}

	// Fixed date field widths: Type A "YYYY-MM-DD" (10), Type B "YYYY-DDD" (8).
	dateLen := 10
	if a.Type == ASCIITypeB {
		dateLen = 8
	}
	// Minimum remaining: 'T' + "hh:mm:ss"
	if len(s) < dateLen+1+8 {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	if s[dateLen] != 'T' {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	datePart := s[:dateLen]
	timePart := s[dateLen+1:]

	var year, month, day, doy int
	var ok bool

	if a.Type == ASCIITypeA {
		// YYYY-MM-DD
		if datePart[4] != '-' || datePart[7] != '-' {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if year, ok = parseFixedDigits(datePart[0:4]); !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if month, ok = parseFixedDigits(datePart[5:7]); !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if day, ok = parseFixedDigits(datePart[8:10]); !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if month < 1 || month > 12 {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if day < 1 || day > int(daysInMonth(year, month)) {
			return time.Time{}, ErrInvalidASCIIFormat
		}
	} else {
		// YYYY-DDD
		if datePart[4] != '-' {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if year, ok = parseFixedDigits(datePart[0:4]); !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		if doy, ok = parseFixedDigits(datePart[5:8]); !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		maxDOY := 365
		if isLeapYear(year) {
			maxDOY = 366
		}
		if doy < 1 || doy > maxDOY {
			return time.Time{}, ErrInvalidASCIIFormat
		}
	}

	// Time part: hh:mm:ss with optional .d...d (1-9 digits)
	if len(timePart) < 8 || timePart[2] != ':' || timePart[5] != ':' {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	hour, ok := parseFixedDigits(timePart[0:2])
	if !ok {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	min, ok := parseFixedDigits(timePart[3:5])
	if !ok {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	sec, ok := parseFixedDigits(timePart[6:8])
	if !ok {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	if hour > 23 || min > 59 || sec > 60 {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	if sec == 60 && (hour != 23 || min != 59) {
		// A positive leap second occurs only at UTC 23:59:60.
		return time.Time{}, ErrInvalidASCIIFormat
	}

	nsec := 0
	if len(timePart) > 8 {
		if timePart[8] != '.' {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		fracStr := timePart[9:]
		if len(fracStr) < 1 || len(fracStr) > 9 {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		frac, ok := parseFixedDigits(fracStr)
		if !ok {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		// Scale to nanoseconds.
		for range 9 - len(fracStr) {
			frac *= 10
		}
		nsec = frac
	}

	var t time.Time
	if a.Type == ASCIITypeA {
		t = time.Date(year, time.Month(month), day, hour, min, sec, nsec, time.UTC)
	} else {
		t = time.Date(year, 1, 1, hour, min, sec, nsec, time.UTC)
		t = t.AddDate(0, 0, doy-1)
	}

	return t, nil
}
