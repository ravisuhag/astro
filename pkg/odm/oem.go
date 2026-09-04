package odm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// oemHeaderSpec is how table 5-2 treats the shared header keywords. It matches
// the OPM's: the two tables differ only in the version keyword.
var oemHeaderSpec = ndm.HeaderSpec{
	VersionKeyword: "CCSDS_OEM_VERS",
	Classification: ndm.Optional,
	MessageFor:     ndm.Absent,
	MessageID:      ndm.Optional,
}

// Block delimiters. The OEM is the first message here with them: clause 5.2.3.3
// wraps each metadata group in META_START and META_STOP, and clause 5.2.5.2
// wraps a covariance section in COVARIANCE_START and COVARIANCE_STOP.
//
// Note the covariance names. Clause 7.8.9, listing where comments may appear,
// calls the opening delimiter 'COV_START'. No such keyword exists in the OEM:
// table 5-4 defines COVARIANCE_START. The OPM's covariance keywords are the
// COV_ family, which is where the shorter name belongs.
const (
	keywordMetaStart       = "META_START"
	keywordMetaStop        = "META_STOP"
	keywordCovarianceStart = "COVARIANCE_START"
	keywordCovarianceStop  = "COVARIANCE_STOP"
)

// OEMMetadata is one OEM metadata group (table 5-3).
type OEMMetadata struct {
	// Comments may appear only immediately after META_START.
	Comments []string
	// ObjectName and ObjectID name the spacecraft.
	ObjectName string
	ObjectID   string
	// CenterName is the origin of the reference frame. Unlike the OPM's, it
	// may be another spacecraft: table 5-3 allows a formation-flying chief.
	CenterName string
	// RefFrame is the frame the ephemeris is in.
	RefFrame string
	// RefFrameEpoch is the frame's epoch when its definition carries none.
	RefFrameEpoch *time.Time
	// TimeSystem is the time scale. Clause 5.2.4.5 requires the same value in
	// every metadata group of one OEM.
	TimeSystem string
	// StartTime and StopTime bound the total span this block covers.
	StartTime time.Time
	StopTime  time.Time
	// UseableStartTime and UseableStopTime bound the span a consumer should
	// actually use. They exist so a producer can pad the ends with smooth
	// fictitious nodes for an interpolator that needs more than two, which
	// table 5-3 describes and this package neither generates nor trims.
	UseableStartTime *time.Time
	UseableStopTime  *time.Time
	// Interpolation is the recommended method: HERMITE, LINEAR or LAGRANGE in
	// the examples. It is a recommendation to the consumer, not something this
	// package acts on.
	Interpolation string
	// InterpolationDegree is mandatory whenever Interpolation is given.
	InterpolationDegree int32
}

// EphemerisLine is one ephemeris record (clause 5.2.4.1). The order of the
// fields on the wire is fixed and positional: epoch, position, velocity, and
// optionally acceleration.
type EphemerisLine struct {
	Epoch time.Time
	// X, Y and Z are the position components in km.
	X, Y, Z float64
	// XDot, YDot and ZDot are the velocity components in km/s.
	XDot, YDot, ZDot float64
	// XDDot, YDDot and ZDDot are the acceleration components in km/s**2.
	// Clause 5.2.4.2 makes them optional; HasAcceleration says whether the
	// line carried them, since zero acceleration is a legal value.
	XDDot, YDDot, ZDDot float64
	HasAcceleration     bool
}

// OEMCovariance is one covariance matrix from a covariance section.
type OEMCovariance struct {
	// Epoch is the epoch of the navigation solution the matrix belongs to.
	Epoch time.Time
	// RefFrame is given only when it differs from the ephemeris frame
	// (clause 5.2.5.3).
	RefFrame string
	// Matrix holds the full symmetric 6x6. Only the lower triangle goes on the
	// wire; the upper triangle is filled in on decode.
	Matrix [6][6]float64
}

// EphemerisBlock is one metadata group with the data that follows it.
//
// An OEM may carry several. Clause 5.2.4.6 gives that meaning: a second
// metadata group says a consumer must not interpolate across the boundary,
// which is how a manoeuvre or an eclipse entry is marked.
type EphemerisBlock struct {
	Metadata OEMMetadata
	// Comments at the head of the ephemeris data section, after META_STOP.
	// Figure G-11 puts its file-provenance comments here rather than inside
	// the metadata group.
	Comments []string
	Lines    []EphemerisLine
	// CovarianceComments are the comments at the head of the covariance
	// section, after COVARIANCE_START.
	CovarianceComments []string
	Covariances        []OEMCovariance
}

// OEM is an Orbit Ephemeris Message: a table of state vectors over a span,
// with the interpolation a consumer should use between them
// (CCSDS 502.0-B-3 section 5).
type OEM struct {
	Header Header
	Blocks []EphemerisBlock
}

// Span reports the total time span the message covers, from the earliest
// START_TIME to the latest STOP_TIME across its blocks.
func (m *OEM) Span() (start, stop time.Time) {
	for i, b := range m.Blocks {
		if i == 0 || b.Metadata.StartTime.Before(start) {
			start = b.Metadata.StartTime
		}
		if i == 0 || b.Metadata.StopTime.After(stop) {
			stop = b.Metadata.StopTime
		}
	}
	return start, stop
}

// Records reports how many ephemeris lines the message holds in total.
func (m *OEM) Records() int {
	n := 0
	for _, b := range m.Blocks {
		n += len(b.Lines)
	}
	return n
}

// Validate checks the message against the rules section 5 states.
//
// As with the OPM, this is structure rather than physics: clause 1.2 puts
// orbit accuracy outside the standard. What is checked is that the mandatory
// keywords are present, that an interpolation method carries a degree, that
// the time system does not change, and that covariance epochs increase.
func (m *OEM) Validate() error {
	if m.Header.Version == "" || m.Header.Originator == "" {
		return ndm.ErrMissingHeaderField
	}
	if len(m.Blocks) == 0 {
		return ErrNoEphemerisBlock
	}

	for i := range m.Blocks {
		b := &m.Blocks[i]

		for _, field := range []string{
			b.Metadata.ObjectName,
			b.Metadata.ObjectID,
			b.Metadata.CenterName,
			b.Metadata.RefFrame,
			b.Metadata.TimeSystem,
		} {
			if field == "" {
				return ErrMissingKeyword
			}
		}
		if b.Metadata.StartTime.IsZero() || b.Metadata.StopTime.IsZero() {
			return ErrMissingKeyword
		}
		// Table 5-3: the degree is mandatory once the method is given.
		if b.Metadata.Interpolation != "" && b.Metadata.InterpolationDegree == 0 {
			return ErrInterpolationDegreeMissing
		}
		// Clause 5.2.4.5.
		if b.Metadata.TimeSystem != m.Blocks[0].Metadata.TimeSystem {
			return ErrTimeSystemChanged
		}
		// Clause 5.2.5.7.
		for j := 1; j < len(b.Covariances); j++ {
			if !b.Covariances[j].Epoch.After(b.Covariances[j-1].Epoch) {
				return ErrCovarianceOutOfOrder
			}
		}
	}
	return nil
}
