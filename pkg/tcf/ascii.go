package tcf

import (
	"fmt"
	"strconv"
	"strings"
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
// These are derived from ISO 8601 and are human-readable text representations.
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

// Decode parses a CCSDS ASCII time string into a Go time.Time value.
func (a *ASCIITime) Decode(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	// Z terminator is optional per §3.5.1.1
	if s[len(s)-1] == 'Z' {
		s = s[:len(s)-1]
	}

	var datePart, timePart string
	tIdx := strings.IndexByte(s, 'T')
	if tIdx < 0 {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	datePart = s[:tIdx]
	timePart = s[tIdx+1:]

	var year, month, day, doy int

	if a.Type == ASCIITypeA {
		// YYYY-MM-DD
		parts := strings.Split(datePart, "-")
		if len(parts) != 3 {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		var err error
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		day, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
	} else {
		// YYYY-DDD
		parts := strings.Split(datePart, "-")
		if len(parts) != 2 {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		var err error
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
		doy, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
	}

	// Parse time part: hh:mm:ss[.ddd]
	var hour, min, sec, nsec int
	timeParts := strings.SplitN(timePart, ".", 2)
	hmsParts := strings.Split(timeParts[0], ":")
	if len(hmsParts) != 3 {
		return time.Time{}, ErrInvalidASCIIFormat
	}

	var err error
	hour, err = strconv.Atoi(hmsParts[0])
	if err != nil {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	min, err = strconv.Atoi(hmsParts[1])
	if err != nil {
		return time.Time{}, ErrInvalidASCIIFormat
	}
	sec, err = strconv.Atoi(hmsParts[2])
	if err != nil {
		return time.Time{}, ErrInvalidASCIIFormat
	}

	// Parse fractional seconds
	if len(timeParts) == 2 {
		fracStr := timeParts[1]
		// Pad or truncate to 9 digits (nanoseconds)
		for len(fracStr) < 9 {
			fracStr += "0"
		}
		fracStr = fracStr[:9]
		nsec, err = strconv.Atoi(fracStr)
		if err != nil {
			return time.Time{}, ErrInvalidASCIIFormat
		}
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
