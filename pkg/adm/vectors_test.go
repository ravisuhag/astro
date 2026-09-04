package adm_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/adm"
)

// The vectors for this package live in vectors/adm/.
//
// What they assert is the shape: which blocks an APM carried, the frames a
// rotation goes between, and for an AEM the attitude type and the line width
// it implies. The numbers are floats, which a vector field cannot hold, and
// are checked in the package's own tests against the same published text.
func TestAttitudeVectors(t *testing.T) {
	vectors.RunFile(t, "adm/attitude.json", vectors.Impl{
		DecodeFn: decodeVector,
	})
}

var errUnknownVectorStructure = errors.New("adm: vector names an unknown structure")

func decodeVector(input []byte, config vectors.Fields) (vectors.Fields, error) {
	structure, err := config.Str("structure")
	if err != nil {
		return nil, err
	}

	switch structure {
	case "apm":
		m, err := adm.DecodeAPM(input)
		if err != nil {
			return nil, err
		}
		return apmFields(m), nil
	case "aem":
		m, err := adm.DecodeAEM(input)
		if err != nil {
			return nil, err
		}
		return aemFields(m), nil
	case "acm":
		m, err := adm.DecodeACM(input)
		if err != nil {
			return nil, err
		}
		return acmFields(m), nil
	case "acm-xml":
		// The XML form reports the same fields, so a vector can hold one
		// message in both forms and assert that they say the same thing.
		m, err := adm.DecodeXMLACM(input)
		if err != nil {
			return nil, err
		}
		return acmFields(m), nil
	}
	return nil, errUnknownVectorStructure
}

func apmFields(m *adm.APM) vectors.Fields {
	f := vectors.Fields{
		"version":        m.Header.Version,
		"originator":     m.Header.Originator,
		"creation_date":  m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"object_name":    m.Metadata.ObjectName,
		"object_id":      m.Metadata.ObjectID,
		"time_system":    m.Metadata.TimeSystem,
		"epoch":          m.Epoch.Format("2006-01-02T15:04:05.999999999Z"),
		"has_quaternion": m.Quaternion != nil,
		"has_euler":      m.Euler != nil,
		"has_angvel":     m.AngVel != nil,
		"has_spin":       m.Spin != nil,
		"has_inertia":    m.Inertia != nil,
		"maneuver_count": uint64(len(m.Maneuvers)),
	}
	// The frames come from whichever attitude block is present. An attitude
	// with no frames is not an attitude.
	switch {
	case m.Quaternion != nil:
		f["frame_a"], f["frame_b"] = m.Quaternion.FrameA, m.Quaternion.FrameB
	case m.Euler != nil:
		f["frame_a"], f["frame_b"] = m.Euler.FrameA, m.Euler.FrameB
		f["euler_rot_seq"] = m.Euler.RotSeq
	case m.Spin != nil:
		f["frame_a"], f["frame_b"] = m.Spin.FrameA, m.Spin.FrameB
	}
	return f
}

func aemFields(m *adm.AEM) vectors.Fields {
	md := m.Blocks[0].Metadata
	fields, _ := md.Type.Fields()

	return vectors.Fields{
		"version":              m.Header.Version,
		"originator":           m.Header.Originator,
		"message_id":           m.Header.MessageID,
		"creation_date":        m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"object_name":          md.ObjectName,
		"object_id":            md.ObjectID,
		"center_name":          md.CenterName,
		"frame_a":              md.FrameA,
		"frame_b":              md.FrameB,
		"time_system":          md.TimeSystem,
		"attitude_type":        string(md.Type),
		"values_per_line":      uint64(fields),
		"block_count":          uint64(len(m.Blocks)),
		"record_count":         uint64(m.Records()),
		"interpolation_method": md.InterpolationMethod,
		"interpolation_degree": uint64(md.InterpolationDegree),
	}
}

// acmFields reports the parts of a decoded comprehensive message a vector can
// compare.
//
// The shape is most of what matters, as it is for the ODM's OCM. What the ACM
// adds is that the shape is self-checking: ATT_TYPE and RATE_TYPE give the
// element counts of annex B4, NUMBER_STATES states the same total, and the row
// width has to match both. So the vectors carry all three.
func acmFields(m *adm.ACM) vectors.Fields {
	f := vectors.Fields{
		"version":           m.Header.Version,
		"originator":        m.Header.Originator,
		"creation_date":     m.Header.CreationDate.Format("2006-01-02T15:04:05Z"),
		"message_id":        m.Header.MessageID,
		"object_name":       m.ObjectName(),
		"time_system":       m.TimeSystem(),
		"metadata_count":    uint64(len(m.Metadata.Fields)),
		"attitude_count":    uint64(len(m.Attitudes)),
		"covariance_count":  uint64(len(m.Covariances)),
		"maneuver_count":    uint64(len(m.Maneuvers)),
		"has_physical":      m.Physical != nil,
		"has_determination": m.AttitudeDetermination != nil,
	}
	if tzero, ok := m.EpochTZero(); ok {
		f["epoch_tzero"] = tzero.Format("2006-01-02T15:04:05.999999999Z")
	}
	if len(m.Attitudes) > 0 {
		first := m.Attitudes[0]
		from, to := first.Frames()
		f["att_type"] = first.AttitudeType()
		f["rate_type"] = first.RateType()
		f["att_frame_a"], f["att_frame_b"] = from, to
		f["att_row_count"] = uint64(len(first.Rows))
		f["att_relative_times"] = len(first.Rows) > 0 && first.Rows[0].IsRelative()
		if states, ok := first.StateCount(); ok {
			f["att_state_count"] = uint64(states)
			f["att_row_width"] = uint64(states + 1)
		}
	}
	if len(m.Covariances) > 0 {
		first := m.Covariances[0]
		f["cov_type"] = first.CovarianceType()
		f["cov_row_count"] = uint64(len(first.Rows))
		if elements, ok := first.CovarianceCount(); ok {
			f["cov_dimension"] = uint64(elements)
		}
	}
	if len(m.Maneuvers) > 0 {
		first := m.Maneuvers[0]
		f["man_purpose"] = first.GetOr("MAN_PURPOSE", "")
		f["man_begin_time"] = first.GetOr("MAN_BEGIN_TIME", "")
		f["man_actuator"] = first.GetOr("ACTUATOR_USED", "")
	}
	if ad := m.AttitudeDetermination; ad != nil {
		f["ad_method"] = ad.GetOr("AD_METHOD", "")
		f["ad_source"] = ad.GetOr("ATTITUDE_SOURCE", "")
		f["sensor_count"] = uint64(len(ad.Sensors))
	}
	return f
}
