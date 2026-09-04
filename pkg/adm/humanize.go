package adm

import (
	"fmt"
	"strings"
	"time"
)

// Humanize returns a human-readable summary of the message.
func (m *APM) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS Attitude Parameter Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))
	fmt.Fprintf(&sb, "  Object .......... %s (%s)\n", m.Metadata.ObjectName, m.Metadata.ObjectID)
	if m.Metadata.CenterName != "" {
		fmt.Fprintf(&sb, "  Center .......... %s\n", m.Metadata.CenterName)
	}
	fmt.Fprintf(&sb, "  Time system ..... %s\n", m.Metadata.TimeSystem)
	fmt.Fprintf(&sb, "  Epoch ........... %s\n", m.Epoch.Format("2006-01-02T15:04:05.999999999"))

	if b := m.Quaternion; b != nil {
		// The scalar is named rather than left as the fourth number, because
		// which end it belongs on is the thing people get wrong.
		fmt.Fprintf(&sb, "  Quaternion ...... %s\n", b.rotation())
		fmt.Fprintf(&sb, "    vector ........ %.6f %.6f %.6f\n", b.Q1, b.Q2, b.Q3)
		fmt.Fprintf(&sb, "    scalar (QC) ... %.6f\n", b.QC)
		if b.HasDerivative {
			fmt.Fprintf(&sb, "    derivative .... %.6g %.6g %.6g, scalar %.6g 1/s\n",
				b.Derivative.Q1, b.Derivative.Q2, b.Derivative.Q3, b.Derivative.QC)
		}
	}
	if b := m.Euler; b != nil {
		fmt.Fprintf(&sb, "  Euler angles .... %s, sequence %s\n", b.rotation(), b.RotSeq)
		fmt.Fprintf(&sb, "    angles ........ %.6f %.6f %.6f deg\n", b.Angle1, b.Angle2, b.Angle3)
		if b.HasRates {
			fmt.Fprintf(&sb, "    rates ......... %.6f %.6f %.6f deg/s\n", b.Rate1, b.Rate2, b.Rate3)
		}
	}
	if b := m.AngVel; b != nil {
		fmt.Fprintf(&sb, "  Angular velocity  %.6f %.6f %.6f deg/s in %s\n", b.X, b.Y, b.Z, b.Frame)
	}
	if b := m.Spin; b != nil {
		fmt.Fprintf(&sb, "  Spin ............ %s\n", b.rotation())
		fmt.Fprintf(&sb, "    axis .......... alpha %.6f, delta %.6f deg\n", b.Alpha, b.Delta)
		fmt.Fprintf(&sb, "    phase ......... %.6f deg at %.6f deg/s\n", b.Angle, b.AngleVel)
		if b.HasNutation {
			fmt.Fprintf(&sb, "    nutation ...... %.6f deg, period %.6f s, phase %.6f deg\n",
				b.Nutation, b.NutationPeriod, b.NutationPhase)
		}
		if b.HasMomentum {
			fmt.Fprintf(&sb, "    momentum ...... alpha %.6f, delta %.6f deg at %.6f deg/s\n",
				b.MomentumAlpha, b.MomentumDelta, b.NutationVel)
		}
	}
	if b := m.Inertia; b != nil {
		fmt.Fprintf(&sb, "  Inertia ......... in %s, kg*m**2\n", b.Frame)
		fmt.Fprintf(&sb, "    moments ....... %.1f %.1f %.1f\n", b.IXX, b.IYY, b.IZZ)
		fmt.Fprintf(&sb, "    cross products  %.1f %.1f %.1f\n", b.IXY, b.IXZ, b.IYZ)
	}
	for i, man := range m.Maneuvers {
		kind := fmt.Sprintf("%.2f s", man.Duration)
		if man.Duration == 0 {
			kind = "impulsive"
		}
		fmt.Fprintf(&sb, "  Maneuver %d ...... %s at %s\n", i+1, kind,
			man.EpochStart.Format("2006-01-02T15:04:05.999999999"))
		fmt.Fprintf(&sb, "    torque ........ %.6f %.6f %.6f N*m in %s\n",
			man.TorqueX, man.TorqueY, man.TorqueZ, man.RefFrame)
	}
	return sb.String()
}

// rotation names the transformation a block describes.
func (f frames) rotation() string { return f.FrameA + " to " + f.FrameB }

// Humanize returns a human-readable summary of the message.
//
// The attitude lines are counted rather than printed, and the type is named,
// because the type is what says how to read them.
func (m *AEM) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS Attitude Ephemeris Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))
	fmt.Fprintf(&sb, "  Records ......... %d in %d block(s)\n", m.Records(), len(m.Blocks))

	for i := range m.Blocks {
		b := &m.Blocks[i]
		md := &b.Metadata

		fmt.Fprintf(&sb, "  Block %d\n", i+1)
		fmt.Fprintf(&sb, "    Object ........ %s (%s)\n", md.ObjectName, md.ObjectID)
		fmt.Fprintf(&sb, "    Rotation ...... %s\n", md.rotation())
		fmt.Fprintf(&sb, "    Time system ... %s\n", md.TimeSystem)
		fmt.Fprintf(&sb, "    Total span .... %s to %s\n",
			md.StartTime.Format("2006-01-02T15:04:05.999"), md.StopTime.Format("2006-01-02T15:04:05.999"))
		if md.UseableStartTime != nil || md.UseableStopTime != nil {
			fmt.Fprintf(&sb, "    Useable span .. %s to %s\n",
				optionalTime(md.UseableStartTime), optionalTime(md.UseableStopTime))
		}

		fields, _ := md.Type.Fields()
		fmt.Fprintf(&sb, "    Attitude type . %s, %d value(s) per line\n", md.Type, fields)
		if md.Type.IsEuler() {
			fmt.Fprintf(&sb, "    Rotation seq .. %s\n", md.RotSeq)
		}
		if md.AngVelFrame != "" {
			fmt.Fprintf(&sb, "    AngVel frame .. %s\n", md.AngVelFrame)
		}
		if md.InterpolationMethod != "" {
			fmt.Fprintf(&sb, "    Interpolation . %s, degree %d\n", md.InterpolationMethod, md.InterpolationDegree)
		}
		fmt.Fprintf(&sb, "    Records ....... %d\n", len(b.Lines))
	}
	return sb.String()
}

func optionalTime(t *time.Time) string {
	if t == nil {
		return "(not given)"
	}
	return t.Format("2006-01-02T15:04:05.999")
}
