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
