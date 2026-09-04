package odm_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/odm"
)

// The vectors for this package live in vectors/odm/.
//
// An orbit data message is ASCII rather than an octet string, so the two
// worked examples of annex G are shipped twice over: as readable .kvn files
// under the file's corpus list, and as decode vectors carrying the same bytes
// hex-encoded, because hex is the only input form the schema has.
//
// What the vectors assert is the text and integer content. A vector field has
// no float accessor, and pinning the state vector as formatted strings would
// test this package's choice of number formatting rather than anything the
// standard says. The numeric values are checked in opm_test.go against the
// same published text.
func TestOPMVectors(t *testing.T) {
	vectors.RunFile(t, "odm/opm.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

var errUnknownVectorStructure = errors.New("odm: vector names an unknown structure")

func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}
	if structure != "opm" {
		return nil, errUnknownVectorStructure
	}

	m, err := odm.DecodeOPM(input)
	if err != nil {
		return nil, err
	}
	return opmFields(m), nil
}

// opmFields reports the parts of a decoded message a vector can compare.
func opmFields(m *odm.OPM) vectors.Fields {
	return vectors.Fields{
		"version":                m.Header.Version,
		"originator":             m.Header.Originator,
		"creation_date":          m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"object_name":            m.Metadata.ObjectName,
		"object_id":              m.Metadata.ObjectID,
		"center_name":            m.Metadata.CenterName,
		"ref_frame":              m.Metadata.RefFrame,
		"time_system":            m.Metadata.TimeSystem,
		"epoch":                  m.Data.StateVector.Epoch.Format("2006-01-02T15:04:05.999999999Z"),
		"maneuver_count":         uint64(len(m.Data.Maneuvers)),
		"has_keplerian":          m.Data.Keplerian != nil,
		"has_covariance":         m.Data.Covariance != nil,
		"header_comment_count":   uint64(len(m.Header.Comments)),
		"metadata_comment_count": uint64(len(m.Metadata.Comments)),
	}
}
