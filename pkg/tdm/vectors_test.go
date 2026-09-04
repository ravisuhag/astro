package tdm_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/tdm"
)

// The vectors for this package live in vectors/tdm/.
//
// What they assert is the metadata that decides how a measurement must be
// read, plus the record counts. The measurements are floats, which a vector
// field cannot hold; those are checked in tdm_test.go against the same
// published text.
//
// range_units and range_modulus_given carry the most weight. A RANGE record
// is a keyword, a timetag and a number, and nothing in it says whether the
// number is kilometres, seconds or range units, or whether it is ambiguous.
func TestTrackingVectors(t *testing.T) {
	vectors.RunFile(t, "tdm/tracking.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

var errUnknownVectorStructure = errors.New("tdm: vector names an unknown structure")

func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}
	if structure != "tdm" {
		return nil, errUnknownVectorStructure
	}

	m, err := tdm.Decode(input)
	if err != nil {
		return nil, err
	}

	first := m.Segments[0]
	md := first.Metadata
	_, unitsGiven := md.Get("RANGE_UNITS")
	_, modulusGiven := md.RangeModulus()
	participants := md.Participants()

	return vectors.Fields{
		"version":              m.Header.Version,
		"originator":           m.Header.Originator,
		"creation_date":        m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"segment_count":        uint64(len(m.Segments)),
		"observation_count":    uint64(m.Observations()),
		"time_system":          md.TimeSystem(),
		"participant_1":        participants[1],
		"participant_2":        participants[2],
		"mode":                 md.Mode(),
		"path":                 md.Path(),
		"range_units":          md.RangeUnits(),
		"range_units_given":    unitsGiven,
		"range_modulus_given":  modulusGiven,
		"metadata_field_count": uint64(len(md.Fields)),
		"first_keyword":        first.Observations[0].Keyword,
		"first_epoch":          first.Observations[0].Epoch.Format("2006-01-02T15:04:05.999999999Z"),
	}, nil
}
