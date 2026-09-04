package cdm_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/cdm"
)

// The vectors for this package live in vectors/cdm/.
//
// What they assert is the conjunction and the identity of each object, plus
// the covariance order — which is not recoverable from the numbers, since an
// absent row and a row of zeroes look the same in the matrix. The floats
// themselves are checked in cdm_test.go against the same text.
func TestConjunctionVectors(t *testing.T) {
	vectors.RunFile(t, "cdm/conjunction.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

var errUnknownVectorStructure = errors.New("cdm: vector names an unknown structure")

func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}
	if structure != "cdm" {
		return nil, errUnknownVectorStructure
	}

	m, err := cdm.Decode(input)
	if err != nil {
		return nil, err
	}

	tca, _ := m.TCA()
	_, _, hasProbability := m.CollisionProbability()
	first, second := m.Objects[0], m.Objects[1]
	firstMoves, _ := first.Maneuverable()
	secondMoves, _ := second.Maneuverable()

	return vectors.Fields{
		"version":                   m.Header.Version,
		"originator":                m.Header.Originator,
		"message_id":                m.Header.MessageID,
		"creation_date":             m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"tca":                       tca.Format("2006-01-02T15:04:05.999999999Z"),
		"object1_name":              first.Name(),
		"object1_designator":        first.Designator(),
		"object1_catalog":           first.CatalogName(),
		"object1_maneuverable":      firstMoves,
		"object1_covariance_order":  uint64(first.CovarianceOrder()),
		"object1_ref_frame":         first.RefFrame(),
		"object2_name":              second.Name(),
		"object2_designator":        second.Designator(),
		"object2_maneuverable":      secondMoves,
		"object2_covariance_order":  uint64(second.CovarianceOrder()),
		"has_collision_probability": hasProbability,
	}, nil
}
