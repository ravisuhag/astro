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
func TestODMVectors(t *testing.T) {
	vectors.RunFile(t, "odm/messages.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

var errUnknownVectorStructure = errors.New("odm: vector names an unknown structure")

func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}
	switch structure {
	case "opm":
		m, err := odm.DecodeOPM(input)
		if err != nil {
			return nil, err
		}
		return opmFields(m), nil
	case "oem":
		m, err := odm.DecodeOEM(input)
		if err != nil {
			return nil, err
		}
		return oemFields(m), nil
	case "omm":
		m, err := odm.DecodeOMM(input)
		if err != nil {
			return nil, err
		}
		return ommFields(m), nil
	case "ocm":
		m, err := odm.DecodeOCM(input)
		if err != nil {
			return nil, err
		}
		return ocmFields(m), nil
	case "ocm-xml":
		// The XML form reports the same fields, so a vector can hold one
		// message in both forms and assert that they say the same thing.
		m, err := odm.DecodeXMLOCM(input)
		if err != nil {
			return nil, err
		}
		return ocmFields(m), nil
	}
	return nil, errUnknownVectorStructure
}

// oemFields reports the parts of a decoded ephemeris message a vector can
// compare. The counts matter as much as the values: an OEM's shape — how many
// metadata groups, how many rows, whether a row carries acceleration, how many
// covariance matrices — is what a consumer has to read correctly before any
// single number means anything.
func oemFields(m *odm.OEM) vectors.Fields {
	first := m.Blocks[0].Metadata
	start, stop := m.Span()

	hasAcceleration := false
	covariances := 0
	for i := range m.Blocks {
		if lines := m.Blocks[i].Lines; len(lines) > 0 && lines[0].HasAcceleration {
			hasAcceleration = true
		}
		covariances += len(m.Blocks[i].Covariances)
	}

	return vectors.Fields{
		"version":              m.Header.Version,
		"originator":           m.Header.Originator,
		"creation_date":        m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"object_name":          first.ObjectName,
		"object_id":            first.ObjectID,
		"center_name":          first.CenterName,
		"ref_frame":            first.RefFrame,
		"time_system":          first.TimeSystem,
		"block_count":          uint64(len(m.Blocks)),
		"record_count":         uint64(m.Records()),
		"has_acceleration":     hasAcceleration,
		"covariance_count":     uint64(covariances),
		"interpolation":        first.Interpolation,
		"interpolation_degree": uint64(first.InterpolationDegree),
		"span_start":           start.Format("2006-01-02T15:04:05.999999999Z"),
		"span_stop":            stop.Format("2006-01-02T15:04:05.999999999Z"),
	}
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

// ommFields reports the parts of a decoded mean-elements message a vector can
// compare.
//
// Which of each paired keyword arrived is recorded, because that choice
// changes what the numbers mean: a mean motion is not a semi-major axis, and
// BTERM is not BSTAR.
func ommFields(m *odm.OMM) vectors.Fields {
	f := vectors.Fields{
		"version":             m.Header.Version,
		"originator":          m.Header.Originator,
		"message_id":          m.Header.MessageID,
		"creation_date":       m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"object_name":         m.Metadata.ObjectName,
		"object_id":           m.Metadata.ObjectID,
		"center_name":         m.Metadata.CenterName,
		"ref_frame":           m.Metadata.RefFrame,
		"time_system":         m.Metadata.TimeSystem,
		"mean_element_theory": m.Metadata.MeanElementTheory,
		"tle_based":           m.Metadata.IsTLEBased(),
		"epoch":               m.Data.Elements.Epoch.Format("2006-01-02T15:04:05.999999999Z"),
		"uses_mean_motion":    m.Data.Elements.UsesMeanMotion,
		"has_covariance":      m.Data.Covariance != nil,
	}
	if t := m.Data.TLE; t != nil {
		f["uses_bterm"] = t.UsesBTerm
		f["uses_agom"] = t.UsesAgom
		f["norad_cat_id"] = uint64(t.NoradCatID)
		f["element_set_no"] = uint64(t.ElementSetNo)
		f["rev_at_epoch"] = uint64(t.RevAtEpoch)
	}
	return f
}

// ocmFields reports the parts of a decoded comprehensive message a vector can
// compare.
//
// The shape is most of what matters. An OCM's sections are what say how to
// read its numbers — TRAJ_TYPE names a trajectory row's columns, COV_ORDERING
// says how a covariance row folds into a matrix, MAN_COMPOSITION names a
// manoeuvre row's fields — so a consumer that gets the shape wrong reads every
// value wrongly. The defaults are asserted too, because a keyword left out is
// not a keyword with no value: clause 6.2.1.3 says the recipient adopts the
// table's default.
func ocmFields(m *odm.OCM) vectors.Fields {
	f := vectors.Fields{
		"version":              m.Header.Version,
		"originator":           m.Header.Originator,
		"creation_date":        m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"message_id":           m.Header.MessageID,
		"classification":       m.Header.Classification,
		"object_name":          m.ObjectName(),
		"time_system":          m.TimeSystem(),
		"metadata_count":       uint64(len(m.Metadata.Fields)),
		"trajectory_count":     uint64(len(m.Trajectories)),
		"covariance_count":     uint64(len(m.Covariances)),
		"maneuver_count":       uint64(len(m.Maneuvers)),
		"has_physical":         m.Physical != nil,
		"has_perturbations":    m.Perturbations != nil,
		"has_orbit_det":        m.OrbitDetermination != nil,
		"user_defined_count":   uint64(len(m.UserDefined)),
		"header_comment_count": uint64(len(m.Header.Comments)),
	}
	if tzero, ok := m.EpochTZero(); ok {
		f["epoch_tzero"] = tzero.Format("2006-01-02T15:04:05.999999999Z")
	}
	if len(m.Trajectories) > 0 {
		first := m.Trajectories[0]
		f["traj_type"] = first.TrajType()
		f["traj_ref_frame"] = first.RefFrame()
		f["traj_center_name"] = first.CenterName()
		f["traj_row_count"] = uint64(len(first.Rows))
		f["traj_relative_times"] = len(first.Rows) > 0 && first.Rows[0].IsRelative()
	}
	if len(m.Covariances) > 0 {
		first := m.Covariances[0]
		f["cov_type"] = first.CovType()
		f["cov_ordering"] = first.CovOrdering()
		f["cov_row_count"] = uint64(len(first.Rows))
		if len(first.Rows) > 0 {
			if matrix, err := first.CovMatrix(first.Rows[0]); err == nil {
				f["cov_dimension"] = uint64(len(matrix))
			}
		}
	}
	if len(m.Maneuvers) > 0 {
		first := m.Maneuvers[0]
		f["man_device_id"] = first.GetOr("MAN_DEVICE_ID", "")
		f["man_field_count"] = uint64(len(first.ManComposition()))
		f["man_row_count"] = uint64(len(first.Rows))
		f["man_duty_cycle"] = first.DutyCycle()
	}
	return f
}
