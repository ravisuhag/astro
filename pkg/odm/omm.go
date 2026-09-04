package odm

import (
	"strings"
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// ommHeaderSpec is how table 4-1 treats the shared header keywords.
var ommHeaderSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_OMM_VERS",
	Classification: ndm.Optional,
	MessageFor:     ndm.Absent,
	MessageID:      ndm.Optional,
}

// Mean element theories a message may name (table 4-2). The value decides how
// the TLE block is read, so it is a constant rather than free text.
const (
	// TheorySGP is the pair clause 4.2.4.6 attaches its TLE conventions to.
	TheorySGP    = "SGP/SGP4"
	TheorySGP4   = "SGP4"
	TheorySGP4XP = "SGP4-XP"
	TheoryPPT3   = "PPT3"
	TheoryDSST   = "DSST"
	TheoryUSM    = "USM"
)

// OMMMetadata is the OMM metadata section (table 4-2).
type OMMMetadata struct {
	Comments   []string
	ObjectName string
	ObjectID   string
	CenterName string
	// RefFrame is the frame the mean elements are in. TEME belongs to
	// TLE-based messages alone (clause 4.2.4.9).
	RefFrame      string
	RefFrameEpoch *time.Time
	TimeSystem    string
	// MeanElementTheory says how a receiver must propagate the state, and it
	// is mandatory: mean elements without their theory cannot be used. It also
	// decides which of the paired TLE keywords apply.
	MeanElementTheory string
}

// IsTLEBased reports whether this message carries a NORAD two-line element
// set, which is what clause 4.2.4.6 attaches its conventions to.
func (md OMMMetadata) IsTLEBased() bool {
	switch strings.ToUpper(md.MeanElementTheory) {
	case TheorySGP, TheorySGP4, TheorySGP4XP:
		return true
	}
	return false
}

// MeanElements is the first data block: the orbit itself.
//
// Its size is given one of two ways. SEMI_MAJOR_AXIS is preferred, and
// MEAN_MOTION is required instead when the message carries a TLE
// (clauses 4.2.4.6 and table 4-3). UsesMeanMotion says which one arrived.
type MeanElements struct {
	Comments []string
	Epoch    time.Time
	// SemiMajorAxis is in km, and is set only when UsesMeanMotion is false.
	SemiMajorAxis float64
	// MeanMotion is in revolutions per day, and is set only when
	// UsesMeanMotion is true.
	MeanMotion     float64
	UsesMeanMotion bool

	Eccentricity    float64
	Inclination     float64
	RAOfAscNode     float64
	ArgOfPericenter float64
	MeanAnomaly     float64
	// GM is the gravitational coefficient in km**3/s**2. Optional here, unlike
	// in the OPM.
	GM float64
}

// TLEParameters is the third data block, required only when the mean element
// theory is one of the SGP family.
//
// Two of its fields share a slot with another keyword, and which name applies
// depends on the theory. That is why they are not simply two more floats.
type TLEParameters struct {
	Comments []string
	// EphemerisType defaults to 0. Clause 4.2.4.7 lists the codings some
	// sources use — 0 SGP, 2 SGP4, 3 PPT3, 4 SGP4-XP, 6 special perturbations
	// — and describes them as suggestions rather than as normative.
	EphemerisType int32
	// ClassificationType defaults to "U". Clause 4.2.4.7 again reports rather
	// than fixes the coding: U unclassified, S secret.
	ClassificationType string
	// NoradCatID is the catalogue number, up to nine digits.
	NoradCatID   int32
	ElementSetNo int32
	RevAtEpoch   int32

	// BStar is the SGP4 drag term, in inverse Earth radii. BTerm is the
	// SGP4-XP ballistic coefficient CD·A/m, in m**2/kg. They occupy one slot
	// in table 4-3 and mean different things; UsesBTerm says which arrived.
	BStar     float64
	BTerm     float64
	UsesBTerm bool

	// MeanMotionDot is the first derivative of mean motion, in rev/day**2.
	MeanMotionDot float64

	// MeanMotionDDot is the second derivative, in rev/day**3, for SGP and
	// PPT3. Agom is the SGP4-XP solar radiation coefficient γ·A/m in m**2/kg.
	// One slot again; UsesAgom says which arrived.
	MeanMotionDDot float64
	Agom           float64
	UsesAgom       bool
}

// OMMData is the data section: five logical blocks, of which only the mean
// elements are mandatory.
type OMMData struct {
	Elements    MeanElements
	Spacecraft  *SpacecraftParameters
	TLE         *TLEParameters
	Covariance  *Covariance
	UserDefined []UserDefined
}

// OMM is an Orbit Mean-Elements Message: mean orbital elements with the theory
// needed to propagate them (CCSDS 502.0-B-3 section 4).
//
// This is what a NORAD two-line element set becomes when written as a CCSDS
// message, and most OMMs in circulation are exactly that. Clause 4.2.4.8 says
// plainly that manoeuvres are not accommodated: a producer wanting to describe
// one sends several OMMs at different epochs.
type OMM struct {
	Header   Header
	Metadata OMMMetadata
	Data     OMMData
}

// Validate checks the message against the rules section 4 states.
func (m *OMM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	for _, field := range []string{
		m.Metadata.ObjectName,
		m.Metadata.ObjectID,
		m.Metadata.CenterName,
		m.Metadata.RefFrame,
		m.Metadata.TimeSystem,
		m.Metadata.MeanElementTheory,
	} {
		if field == "" {
			return ErrMissingKeyword
		}
	}
	if m.Data.Elements.Epoch.IsZero() {
		return ErrMissingKeyword
	}

	tle := m.Metadata.IsTLEBased()

	// Clause 4.2.4.9: TEME is ill-defined by any international convention, and
	// the standard allows it for TLE-based messages "and in no other
	// circumstances".
	if strings.EqualFold(m.Metadata.RefFrame, "TEME") && !tle {
		return ErrTEMEWithoutTLE
	}

	// Clause 4.2.4.6 fixes four things about a TLE-based OMM. Getting any of
	// them wrong produces a message a TLE propagator will accept and
	// mispropagate, because the propagator assumes all four.
	if tle {
		if !strings.EqualFold(m.Metadata.CenterName, "EARTH") ||
			!strings.EqualFold(m.Metadata.RefFrame, "TEME") ||
			!strings.EqualFold(m.Metadata.TimeSystem, "UTC") ||
			!m.Data.Elements.UsesMeanMotion {
			return ErrTLEConventions
		}
	}
	return nil
}
