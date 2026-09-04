package odm

import (
	"fmt"
	"strings"
	"time"
)

// Humanize returns a human-readable summary of the message.
func (m *OPM) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS Orbit Parameter Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))
	if m.Header.MessageID != "" {
		fmt.Fprintf(&sb, "  Message ID ...... %s\n", m.Header.MessageID)
	}
	if m.Header.Classification != "" {
		fmt.Fprintf(&sb, "  Classification .. %s\n", m.Header.Classification)
	}

	sb.WriteString(m.Metadata.Humanize())
	sb.WriteString(m.Data.StateVector.Humanize())

	if k := m.Data.Keplerian; k != nil {
		sb.WriteString(k.Humanize())
	}
	if s := m.Data.Spacecraft; s != nil {
		sb.WriteString(s.Humanize())
	}
	if c := m.Data.Covariance; c != nil {
		sb.WriteString(c.Humanize())
	}
	for i, man := range m.Data.Maneuvers {
		fmt.Fprintf(&sb, "  Maneuver %d\n", i+1)
		sb.WriteString(man.Humanize())
	}
	if len(m.Data.UserDefined) > 0 {
		fmt.Fprintf(&sb, "  User-defined .... %d parameter(s)\n", len(m.Data.UserDefined))
		for _, u := range m.Data.UserDefined {
			fmt.Fprintf(&sb, "    %s = %s\n", u.Name, u.Value)
		}
	}
	return sb.String()
}

// Humanize returns a human-readable summary of the metadata section.
func (md OPMMetadata) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "  Object .......... %s (%s)\n", md.ObjectName, md.ObjectID)
	fmt.Fprintf(&sb, "  Center .......... %s\n", md.CenterName)
	fmt.Fprintf(&sb, "  Reference frame . %s", md.RefFrame)
	if md.RefFrameEpoch != nil {
		fmt.Fprintf(&sb, " at %s", md.RefFrameEpoch.Format("2006-01-02T15:04:05"))
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "  Time system ..... %s\n", md.TimeSystem)
	return sb.String()
}

// Humanize returns a human-readable summary of the state vector.
//
// The units are the ones table 3-3 fixes and are printed whether or not the
// message spelled them out, because a state vector without units is a set of
// numbers nobody can act on.
func (sv StateVector) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "  Epoch ........... %s\n", sv.Epoch.Format("2006-01-02T15:04:05.999999999"))
	fmt.Fprintf(&sb, "  Position ........ %.6f %.6f %.6f km\n", sv.X, sv.Y, sv.Z)
	fmt.Fprintf(&sb, "  Velocity ........ %.6f %.6f %.6f km/s\n", sv.XDot, sv.YDot, sv.ZDot)
	return sb.String()
}

// Humanize returns a human-readable summary of the Keplerian elements.
func (k KeplerianElements) Humanize() string {
	anomaly := "True anomaly"
	if k.AnomalyIsMean {
		anomaly = "Mean anomaly"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  Semi-major axis . %.4f km\n", k.SemiMajorAxis)
	fmt.Fprintf(&sb, "  Eccentricity .... %.9f\n", k.Eccentricity)
	fmt.Fprintf(&sb, "  Inclination ..... %.6f deg\n", k.Inclination)
	fmt.Fprintf(&sb, "  RA of asc node .. %.6f deg\n", k.RAOfAscNode)
	fmt.Fprintf(&sb, "  Arg of pericenter %.6f deg\n", k.ArgOfPericenter)
	fmt.Fprintf(&sb, "  %-16s %.6f deg\n", anomaly, k.Anomaly)
	fmt.Fprintf(&sb, "  GM .............. %.4f km**3/s**2\n", k.GM)
	return sb.String()
}

// Humanize returns a human-readable summary of the spacecraft parameters.
//
// A zero coefficient is not the same as an absent one: clauses 3.2.4.5 and
// 3.2.4.6 give zero the meaning "do not consider this force at all", so it is
// spelled out rather than printed as a bare 0.
func (s SpacecraftParameters) Humanize() string {
	var sb strings.Builder

	if s.hasMass {
		fmt.Fprintf(&sb, "  Mass ............ %.3f kg\n", s.Mass)
	}
	fmt.Fprintf(&sb, "  Solar rad ....... area %.3f m**2, %s\n",
		s.SolarRadArea, coefficient(s.SolarRadCoeff, "solar radiation pressure"))
	fmt.Fprintf(&sb, "  Drag ............ area %.3f m**2, %s\n",
		s.DragArea, coefficient(s.DragCoeff, "drag"))
	return sb.String()
}

// coefficient describes a drag or solar radiation coefficient, naming the
// force that a zero switches off.
func coefficient(v float64, force string) string {
	if v == 0 {
		return fmt.Sprintf("coefficient 0 (no %s)", force)
	}
	return fmt.Sprintf("coefficient %.3f", v)
}

// Humanize returns a human-readable summary of the covariance matrix.
//
// The full 6x6 is not printed. What a reader wants at a glance is how big the
// uncertainty is, which is the square root of the diagonal: a one-sigma
// position and velocity in the units of the matrix.
func (c Covariance) Humanize() string {
	frame := c.RefFrame
	if frame == "" {
		frame = "same as REF_FRAME"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  Covariance ...... 6x6, frame %s\n", frame)
	fmt.Fprintf(&sb, "    1-sigma position %.6f %.6f %.6f km\n",
		sqrt(c.Matrix[0][0]), sqrt(c.Matrix[1][1]), sqrt(c.Matrix[2][2]))
	fmt.Fprintf(&sb, "    1-sigma velocity %.9f %.9f %.9f km/s\n",
		sqrt(c.Matrix[3][3]), sqrt(c.Matrix[4][4]), sqrt(c.Matrix[5][5]))
	return sb.String()
}

// Humanize returns a human-readable summary of one maneuver.
func (m Maneuver) Humanize() string {
	kind := fmt.Sprintf("%.2f s", m.Duration)
	if m.Duration == 0 {
		// Clause 3.2.4.7 gives a zero duration this meaning.
		kind = "impulsive"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "    Ignition ...... %s\n", m.EpochIgnition.Format("2006-01-02T15:04:05.999999999"))
	fmt.Fprintf(&sb, "    Duration ...... %s\n", kind)
	fmt.Fprintf(&sb, "    Mass change ... %.3f kg\n", m.DeltaMass)
	fmt.Fprintf(&sb, "    Delta-v ....... %.8f %.8f %.8f km/s in %s\n",
		m.DV[0], m.DV[1], m.DV[2], m.RefFrame)
	return sb.String()
}

// Humanize returns a human-readable summary of the message.
//
// An OEM can hold tens of thousands of ephemeris records, so the records
// themselves are counted rather than printed. What a reader wants at a glance
// is the span, the cadence and where the discontinuities are.
func (m *OEM) Humanize() string {
	start, stop := m.Span()

	var sb strings.Builder
	fmt.Fprintf(&sb, "CCSDS Orbit Ephemeris Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))
	fmt.Fprintf(&sb, "  Span ............ %s to %s\n",
		start.Format("2006-01-02T15:04:05.999"), stop.Format("2006-01-02T15:04:05.999"))
	fmt.Fprintf(&sb, "  Records ......... %d in %d block(s)\n", m.Records(), len(m.Blocks))

	for i := range m.Blocks {
		b := &m.Blocks[i]
		if len(m.Blocks) > 1 {
			// Clause 5.2.4.6: a second block is a fence, not a convenience.
			fmt.Fprintf(&sb, "  Block %d ......... do not interpolate across the boundary\n", i+1)
		}
		sb.WriteString(b.Metadata.Humanize())
		fmt.Fprintf(&sb, "    Records ....... %d", len(b.Lines))
		if len(b.Lines) > 0 && b.Lines[0].HasAcceleration {
			sb.WriteString(", with acceleration")
		}
		sb.WriteString("\n")
		if len(b.Covariances) > 0 {
			fmt.Fprintf(&sb, "    Covariance .... %d matrix(es)\n", len(b.Covariances))
		}
	}
	return sb.String()
}

// Humanize returns a human-readable summary of an OEM metadata group.
func (md OEMMetadata) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "    Object ........ %s (%s)\n", md.ObjectName, md.ObjectID)
	fmt.Fprintf(&sb, "    Center ........ %s\n", md.CenterName)
	fmt.Fprintf(&sb, "    Frame ......... %s, time system %s\n", md.RefFrame, md.TimeSystem)
	fmt.Fprintf(&sb, "    Total span .... %s to %s\n",
		md.StartTime.Format("2006-01-02T15:04:05.999"), md.StopTime.Format("2006-01-02T15:04:05.999"))

	if md.UseableStartTime != nil || md.UseableStopTime != nil {
		// The useable span is narrower than the total when the producer padded
		// the ends with fictitious nodes for an interpolator.
		fmt.Fprintf(&sb, "    Useable span .. %s to %s\n",
			optionalTime(md.UseableStartTime), optionalTime(md.UseableStopTime))
	}
	if md.Interpolation != "" {
		fmt.Fprintf(&sb, "    Interpolation . %s, degree %d\n", md.Interpolation, md.InterpolationDegree)
	}
	return sb.String()
}

// optionalTime formats a time that may be absent.
func optionalTime(t *time.Time) string {
	if t == nil {
		return "(not given)"
	}
	return t.Format("2006-01-02T15:04:05.999")
}
