package adm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// Header is the header both messages open with (tables 3-1 and 4-1).
type Header struct {
	Version        string
	Comments       []string
	Classification string
	CreationDate   time.Time
	Originator     string
	MessageID      string
}

func headerSpec(versionKeyword string) ndm.HeaderSpec {
	return ndm.HeaderSpec{
		VersionKeyword: versionKeyword,
		Classification: ndm.Optional,
		MessageFor:     ndm.Absent,
		MessageID:      ndm.Optional,
	}
}

func (h Header) toNDM() ndm.Header {
	return ndm.Header{
		Version:        h.Version,
		Comments:       h.Comments,
		Classification: h.Classification,
		CreationDate:   h.CreationDate,
		Originator:     h.Originator,
		MessageID:      h.MessageID,
	}
}

func headerFromNDM(h ndm.Header) Header {
	return Header{
		Version:        h.Version,
		Comments:       h.Comments,
		Classification: h.Classification,
		CreationDate:   h.CreationDate,
		Originator:     h.Originator,
		MessageID:      h.MessageID,
	}
}

// Quaternion is an attitude quaternion as table 3-3 writes it.
//
// The components are named rather than indexed on purpose. On the wire they
// are Q1, Q2, Q3, QC: the vector part first and the scalar last. A great many
// libraries take the scalar first, so reading these into a [4]float64 and
// passing it on gives a rotation that is wrong and looks reasonable.
type Quaternion struct {
	// Q1, Q2 and Q3 are the vector part, e_n * sin(phi/2).
	Q1, Q2, Q3 float64
	// QC is the scalar part, cos(phi/2).
	QC float64
}

// EulerAngles is a rotation as three angles about named axes.
//
// RotSeq is what makes them a rotation. Without it the three numbers are just
// numbers: XYZ and ZXZ describe different orientations from the same angles.
type EulerAngles struct {
	// RotSeq is the axis order, such as "ZXZ" or "YXY", read left to right.
	RotSeq string
	// Angle1, Angle2 and Angle3 are in degrees.
	Angle1, Angle2, Angle3 float64
}

// AngularVelocity is the angular velocity vector, in degrees per second.
type AngularVelocity struct {
	// Frame is the frame the components are given in.
	Frame   string
	X, Y, Z float64
}

// Spin describes a spinning spacecraft: the spin axis, the phase about it, and
// optionally the nutation and angular momentum that go with a coning motion.
type Spin struct {
	// Alpha and Delta are the right ascension and declination of the spin
	// axis in frame A, in degrees.
	Alpha, Delta float64
	// Angle is the phase about the spin axis, and AngleVel its rate.
	Angle    float64
	AngleVel float64

	// Nutation, NutationPeriod and NutationPhase describe the coning of the
	// spin axis. Table 3-3 makes them conditional together: an angle without
	// its period and phase cannot be applied.
	Nutation       float64
	NutationPeriod float64
	NutationPhase  float64
	HasNutation    bool

	// MomentumAlpha and MomentumDelta point the angular momentum vector, and
	// NutationVel is the rate of the spin axis about it.
	MomentumAlpha float64
	MomentumDelta float64
	NutationVel   float64
	HasMomentum   bool
}

// Inertia is the spacecraft inertia tensor, in kg*m**2.
type Inertia struct {
	// Frame is the coordinate system the tensor is expressed in.
	Frame string
	// IXX, IYY and IZZ are the moments; IXY, IXZ and IYZ the cross products.
	IXX, IYY, IZZ float64
	IXY, IXZ, IYZ float64
}

// Maneuver is one planned attitude manoeuvre: a torque held for a duration.
type Maneuver struct {
	Comments   []string
	EpochStart time.Time
	// Duration is in seconds.
	Duration float64
	// RefFrame is the frame the torque vector is given in.
	RefFrame string
	// Torque is the torque vector in N*m.
	TorqueX, TorqueY, TorqueZ float64
}

// frames names the two ends of a rotation. A block that describes a
// transformation carries both, because an attitude is meaningless without
// saying what it is relative to.
type frames struct {
	// FrameA is where the transformation starts, FrameB where it ends.
	FrameA string
	FrameB string
}
