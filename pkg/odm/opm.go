package odm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// Header is the header section every orbit data message opens with
// (CCSDS 502.0-B-3 table 3-1).
type Header struct {
	// Version is the value of the CCSDS_*_VERS keyword, in the form 'x.y'.
	// For CCSDS 502.0-B-3 that is "3.0".
	Version string
	// Comments may appear only immediately after the version keyword.
	Comments []string
	// Classification is free text whose values the exchanging parties agree
	// between themselves.
	Classification string
	// CreationDate is when the file was made. Its time system is UTC whatever
	// TIME_SYSTEM says, which clause 7.5.11 states outright.
	CreationDate time.Time
	// Originator is the creating agency or operator.
	Originator string
	// MessageID uniquely identifies a message from a given originator, in
	// whatever format the originator likes.
	MessageID string
}

// opmHeaderSpec is how table 3-1 treats the shared header keywords.
var opmHeaderSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_OPM_VERS",
	Classification: ndm.Optional,
	MessageFor:     ndm.Absent,
	MessageID:      ndm.Optional,
}

// OPMMetadata is the OPM metadata section (table 3-2).
type OPMMetadata struct {
	// Comments may appear at the very beginning of the metadata section.
	Comments []string
	// ObjectName is the spacecraft name. Table 3-2 recommends a name from the
	// UN Office of Outer Space Affairs index, and UNKNOWN when there is none
	// or it cannot be disclosed.
	ObjectName string
	// ObjectID is the international designator, recommended as YYYY-NNNP{PP}.
	ObjectID string
	// CenterName is the origin of the reference frame, which must be a natural
	// solar system body or a barycenter.
	CenterName string
	// RefFrame is the frame the state vector and Keplerian elements are in.
	// Clause 3.2.3.3 lists the expected values; anything else should be
	// documented in an interface control document.
	RefFrame string
	// RefFrameEpoch is the frame's epoch, when the frame definition does not
	// carry one intrinsically. Nil when absent.
	RefFrameEpoch *time.Time
	// TimeSystem is the time scale for the state vector, manoeuvre and
	// covariance data. Clause 3.2.3.2 lists the expected values.
	TimeSystem string
}

// StateVector is the first logical block of the OPM data section: a position
// and velocity at one epoch, in the frame the metadata names.
type StateVector struct {
	Comments []string
	Epoch    time.Time
	// X, Y and Z are the position components in km.
	X, Y, Z float64
	// XDot, YDot and ZDot are the velocity components in km/s.
	XDot, YDot, ZDot float64
}

// KeplerianElements is the osculating Keplerian element block. Table 3-3 makes
// it all-or-nothing: give every parameter or none of them.
type KeplerianElements struct {
	Comments []string
	// SemiMajorAxis is in km.
	SemiMajorAxis float64
	Eccentricity  float64
	// Inclination, RAOfAscNode, ArgOfPericenter and the anomaly are in degrees.
	Inclination     float64
	RAOfAscNode     float64
	ArgOfPericenter float64
	// Anomaly is the true anomaly or the mean anomaly; AnomalyIsMean says
	// which keyword carried it. Table 3-3 offers the two as alternatives.
	Anomaly       float64
	AnomalyIsMean bool
	// GM is the gravitational coefficient in km**3/s**2.
	GM float64
}

// SpacecraftParameters is the mass and the drag and solar radiation model
// coefficients.
type SpacecraftParameters struct {
	Comments []string
	// Mass is in kg.
	Mass float64
	// SolarRadArea and DragArea are in m**2.
	SolarRadArea float64
	// SolarRadCoeff is CR. Clause 3.2.4.5: zero means no solar radiation
	// pressure is to be considered, which is not the same as absent.
	SolarRadCoeff float64
	DragArea      float64
	// DragCoeff is CD. Clause 3.2.4.6 gives zero the same meaning: no drag.
	DragCoeff float64

	// hasMass records whether MASS was given, since zero is a legal value the
	// caller might mean.
	hasMass bool
}

// Covariance is the 6x6 position/velocity covariance matrix. Clause 3.2.4.10
// stores it in lower triangular form, row by row from [1,1] to [6,6].
type Covariance struct {
	Comments []string
	// RefFrame may be omitted when it is the same as the metadata's REF_FRAME.
	RefFrame string
	// Matrix holds the full symmetric matrix. Only the lower triangle goes on
	// the wire; the upper triangle is filled in on decode, because a
	// covariance matrix is symmetric by definition and a caller reading
	// Matrix[1][2] should not get a zero.
	Matrix [6][6]float64
}

// Maneuver is one planned manoeuvre. Clause 3.2.4.8 allows any number of them,
// each repeating all seven parameters in the order table 3-3 fixes.
type Maneuver struct {
	Comments      []string
	EpochIgnition time.Time
	// Duration is in seconds. Clause 3.2.4.7: zero means an impulsive
	// manoeuvre.
	Duration float64
	// DeltaMass is the mass change in kg, and clause 3.2.4.7 requires it to be
	// negative.
	DeltaMass float64
	// RefFrame is the frame the velocity increment is given in, from the set
	// in clause 3.2.4.11: RSW, RTN or TNW.
	RefFrame string
	// DV holds the three velocity increment components in km/s.
	DV [3]float64
}

// UserDefined is one USER_DEFINED_x parameter. Clause 3.2.4.12 allows any
// number of them and asks that they be used sparingly, because every one is
// something the receiver can only understand from an interface control
// document.
type UserDefined struct {
	// Name is the part after the USER_DEFINED_ prefix.
	Name  string
	Value string
}

// OPMData is the data section: six logical blocks, of which only the state
// vector is mandatory.
type OPMData struct {
	StateVector StateVector
	Keplerian   *KeplerianElements
	Spacecraft  *SpacecraftParameters
	Covariance  *Covariance
	Maneuvers   []Maneuver
	UserDefined []UserDefined
}

// OPM is an Orbit Parameter Message: one state vector at one time, with
// whatever optional blocks the sender chose to include
// (CCSDS 502.0-B-3 section 3).
type OPM struct {
	Header   Header
	Metadata OPMMetadata
	Data     OPMData
}

// toNDM converts the public header to the shared carrier.
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

// headerFromNDM converts the shared carrier to the public header.
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
