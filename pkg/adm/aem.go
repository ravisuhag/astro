package adm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// AEM block delimiters (tables 4-2 and 4-3).
const (
	keywordMetaStart = "META_START"
	keywordMetaStop  = "META_STOP"
	keywordDataStart = "DATA_START"
	keywordDataStop  = "DATA_STOP"
)

// AttitudeType says what a segment's data lines carry (table 4-4).
//
// This is the AEM's load-bearing keyword. The number of values on a data line
// comes from here and from nowhere else: nothing in the line itself says
// whether four numbers after the epoch are a quaternion or a spin state, and a
// reader that assumed one width would silently misread every other type.
type AttitudeType string

const (
	Quaternion4          AttitudeType = "QUATERNION"
	QuaternionDerivative AttitudeType = "QUATERNION/DERIVATIVE"
	QuaternionAngVel     AttitudeType = "QUATERNION/ANGVEL"
	EulerAngle           AttitudeType = "EULER_ANGLE"
	EulerAngleDerivative AttitudeType = "EULER_ANGLE/DERIVATIVE"
	EulerAngleAngVel     AttitudeType = "EULER_ANGLE/ANGVEL"
	SpinType             AttitudeType = "SPIN"
	SpinNutation         AttitudeType = "SPIN/NUTATION"
	SpinNutationMomentum AttitudeType = "SPIN/NUTATION_MOM"
)

// attitudeFields is table 4-4: how many values follow the epoch, per type.
var attitudeFields = map[AttitudeType]int{
	Quaternion4:          4, // Q1, Q2, Q3, QC
	QuaternionDerivative: 8, // ... plus the four derivatives
	QuaternionAngVel:     7, // ... plus the angular velocity vector
	EulerAngle:           3, // ANGLE_1, ANGLE_2, ANGLE_3
	EulerAngleDerivative: 6, // ... plus the three rates
	EulerAngleAngVel:     6, // ... plus the angular velocity vector
	SpinType:             4, // SPIN_ALPHA, SPIN_DELTA, SPIN_ANGLE, SPIN_ANGLE_VEL
	SpinNutation:         7, // ... plus nutation, period, phase
	SpinNutationMomentum: 7, // ... plus momentum alpha, delta, nutation velocity
}

// Fields reports how many values follow the epoch on a data line of this type,
// and whether the type is one table 4-4 defines.
func (t AttitudeType) Fields() (int, bool) {
	n, ok := attitudeFields[t]
	return n, ok
}

// Valid reports whether this is one of the nine types table 4-4 defines.
func (t AttitudeType) Valid() bool {
	_, ok := attitudeFields[t]
	return ok
}

// IsEuler reports whether this type carries Euler angles, which is what makes
// EULER_ROT_SEQ mandatory: three angles without a rotation sequence do not
// define a rotation.
func (t AttitudeType) IsEuler() bool {
	switch t {
	case EulerAngle, EulerAngleDerivative, EulerAngleAngVel:
		return true
	}
	return false
}

// AEMMetadata is one AEM metadata group (table 4-3).
type AEMMetadata struct {
	Comments   []string
	ObjectName string
	ObjectID   string
	CenterName string
	// FrameA and FrameB are the two ends of the rotation the data describes.
	frames
	TimeSystem string
	// StartTime and StopTime bound the total span; the useable pair bounds
	// what a consumer should actually use, for the same reason the OEM has
	// them.
	StartTime        time.Time
	StopTime         time.Time
	UseableStartTime *time.Time
	UseableStopTime  *time.Time
	// Type says how wide a data line is. Mandatory.
	Type AttitudeType
	// RotSeq is mandatory when Type carries Euler angles and meaningless
	// otherwise.
	RotSeq string
	// AngVelFrame applies only when Type pairs an attitude with angular
	// velocities.
	AngVelFrame string
	// InterpolationMethod and InterpolationDegree are a recommendation to the
	// consumer; the degree is mandatory once the method is given.
	InterpolationMethod string
	InterpolationDegree int32
}

// AttitudeLine is one attitude ephemeris record.
//
// Values holds the numbers after the epoch, in the order table 4-4 fixes for
// the segment's type. They are not unpacked into named fields because their
// meaning changes with the type: the fourth value is QC for a quaternion and
// SPIN_ANGLE_VEL for a spin state.
type AttitudeLine struct {
	Epoch  time.Time
	Values []float64
}

// AttitudeBlock is one metadata group with the data that follows it.
type AttitudeBlock struct {
	Metadata AEMMetadata
	// Comments at the head of the data section, after DATA_START.
	Comments []string
	Lines    []AttitudeLine
}

// AEM is an Attitude Ephemeris Message: a table of attitudes over a span
// (CCSDS 504.0-B-2 section 4).
type AEM struct {
	Header Header
	Blocks []AttitudeBlock
}

// Records reports how many attitude lines the message holds in total.
func (m *AEM) Records() int {
	n := 0
	for _, b := range m.Blocks {
		n += len(b.Lines)
	}
	return n
}

// Validate checks the message against the rules section 4 states.
func (m *AEM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if len(m.Blocks) == 0 {
		return ErrNoSegment
	}

	for i := range m.Blocks {
		b := &m.Blocks[i]
		md := &b.Metadata

		if md.ObjectName == "" || md.ObjectID == "" || md.TimeSystem == "" ||
			md.FrameA == "" || md.FrameB == "" {
			return ErrMissingKeyword
		}
		if md.StartTime.IsZero() || md.StopTime.IsZero() {
			return ErrMissingKeyword
		}
		if !md.Type.Valid() {
			return ErrUnknownAttitudeType
		}
		if md.Type.IsEuler() && md.RotSeq == "" {
			return ErrEulerRotSeqMissing
		}
		if md.InterpolationMethod != "" && md.InterpolationDegree == 0 {
			return ErrInterpolationDegreeMissing
		}
		if len(b.Lines) == 0 {
			return ErrNoRecords
		}

		want, _ := md.Type.Fields()
		for _, line := range b.Lines {
			if len(line.Values) != want {
				return ErrAttitudeLineFields
			}
		}
	}
	return nil
}
