package adm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

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
	v, err := ndm.ParseValue(raw)
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
	t, err := ndm.ParseEpoch(raw)
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
