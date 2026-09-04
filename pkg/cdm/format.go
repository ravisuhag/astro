package cdm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// parseEpoch reads an absolute time.
func parseEpoch(raw string) (time.Time, error) {
	value, _, err := ndm.SplitUnits(raw)
	if err != nil {
		return time.Time{}, err
	}
	return ndm.ParseEpoch(value)
}

// parseValue reads a number that may carry a unit suffix. Clause 6.3.3 makes
// the units documentation rather than data, and the worked example in
// clause 3.6.2 writes them on every dimensioned value.
//
// Note that nothing here formats a number back. A Section keeps the raw string
// each keyword arrived with and Encode writes it out again, so a decoded CDM
// re-encodes to the same values it came in with — including the spelling.
// That is not true of pkg/odm, which rebuilds its numbers and so loses the
// original trailing zeroes.
//
// The reason for the difference is what the two messages are. An orbit message
// is data a caller assembles and edits; a conjunction warning is a report a
// caller reads, forwards and archives, and re-emitting it unchanged is worth
// more than a tidy number format.
func parseValue(raw string) (float64, error) {
	value, _, err := ndm.SplitUnits(raw)
	if err != nil {
		return 0, err
	}
	return ndm.ParseFloat(value)
}
