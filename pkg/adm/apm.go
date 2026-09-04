package adm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// APM block delimiters (table 3-3). Each of the six data blocks is wrapped in
// a pair, which is what makes the APM's data section self-describing: a reader
// knows which block it is in without counting keywords.
const (
	blockQuaternion = "QUAT"
	blockEuler      = "EULER"
	blockAngVel     = "ANGVEL"
	blockSpin       = "SPIN"
	blockInertia    = "INERTIA"
	blockManeuver   = "MAN"
)

// APMMetadata is the APM metadata section (table 3-2).
//
// It is short next to the other messages'. There are no START_TIME or
// STOP_TIME keywords, because an APM describes one epoch, and CENTER_NAME is
// optional here where the orbit messages make it mandatory.
type APMMetadata struct {
	Comments   []string
	ObjectName string
	ObjectID   string
	// CenterName is the body being orbited. Optional in table 3-2: an
	// attitude is relative to a frame, and the frame need not be a body.
	CenterName string
	TimeSystem string
}

// QuaternionBlock is the attitude quaternion block, with the frames it rotates
// between and optionally the component derivatives.
type QuaternionBlock struct {
	Comments []string
	frames
	Quaternion
	// Derivative holds Q1_DOT through QC_DOT when the block carried them.
	Derivative    Quaternion
	HasDerivative bool
}

// EulerBlock is the Euler angle block.
type EulerBlock struct {
	Comments []string
	frames
	EulerAngles
	// Rates holds ANGLE_1_DOT through ANGLE_3_DOT when present, in deg/s.
	Rate1, Rate2, Rate3 float64
	HasRates            bool
}

// AngVelBlock is the angular velocity block.
type AngVelBlock struct {
	Comments []string
	frames
	AngularVelocity
}

// SpinBlock is the spin block.
type SpinBlock struct {
	Comments []string
	frames
	Spin
}

// InertiaBlock is the inertia block.
type InertiaBlock struct {
	Comments []string
	Inertia
}

// APM is an Attitude Parameter Message: one attitude at one epoch
// (CCSDS 504.0-B-2 section 3).
type APM struct {
	Header   Header
	Metadata APMMetadata
	// Comments at the head of the data section, before EPOCH.
	Comments []string
	// Epoch is the time the attitude belongs to.
	Epoch time.Time

	Quaternion *QuaternionBlock
	Euler      *EulerBlock
	AngVel     *AngVelBlock
	Spin       *SpinBlock
	Inertia    *InertiaBlock
	Maneuvers  []Maneuver
}

// Validate checks the message against the rules section 3 states.
func (m *APM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if m.Metadata.ObjectName == "" || m.Metadata.ObjectID == "" || m.Metadata.TimeSystem == "" {
		return ErrMissingKeyword
	}
	if m.Epoch.IsZero() {
		return ErrMissingKeyword
	}

	// Table 3-3 makes each block optional on its own, and says nothing about
	// needing one. But a message with no quaternion, no Euler angles and no
	// spin has not said which way the spacecraft is pointing, which is the
	// only thing an APM is for.
	if m.Quaternion == nil && m.Euler == nil && m.Spin == nil {
		return ErrNoAttitude
	}

	if m.Euler != nil && m.Euler.RotSeq == "" {
		return ErrMissingKeyword
	}
	if s := m.Spin; s != nil {
		// Table 3-3 marks the three nutation keywords conditional together.
		if s.HasNutation && (s.NutationPeriod == 0 && s.NutationPhase == 0 && s.Nutation == 0) {
			return ErrIncompleteNutation
		}
	}
	for _, man := range m.Maneuvers {
		if man.EpochStart.IsZero() || man.RefFrame == "" {
			return ErrMissingKeyword
		}
	}
	return nil
}
