package cdm

import (
	"fmt"
	"math"
	"strings"
)

// Humanize returns a human-readable summary of the message.
//
// It leads with what an operator acts on: when the pass is, how close, how
// fast, and whether either object can move. The covariance is reported as
// one-sigma position uncertainty in each axis rather than as a matrix, because
// a nine-by-nine of numbers in the RTN frame is not something anyone reads at
// a glance.
func (m *CDM) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS Conjunction Data Message %s\n", m.Header.Version)
	fmt.Fprintf(&sb, "  Originator ...... %s\n", m.Header.Originator)
	fmt.Fprintf(&sb, "  Message ID ...... %s\n", m.Header.MessageID)
	if m.Header.MessageFor != "" {
		fmt.Fprintf(&sb, "  Message for ..... %s\n", m.Header.MessageFor)
	}
	fmt.Fprintf(&sb, "  Created ......... %s UTC\n", m.Header.CreationDate.Format("2006-01-02T15:04:05"))

	if tca, ok := m.TCA(); ok {
		fmt.Fprintf(&sb, "  Closest approach  %s\n", tca.Format("2006-01-02T15:04:05.999"))
	}
	if miss, ok := m.MissDistance(); ok {
		fmt.Fprintf(&sb, "  Miss distance ... %.1f m\n", miss)
	}
	if speed, ok := m.RelativeSpeed(); ok {
		fmt.Fprintf(&sb, "  Relative speed .. %.1f m/s\n", speed)
	}
	if p, method, ok := m.CollisionProbability(); ok {
		// The method is printed with the number because the number is not
		// comparable between methods.
		fmt.Fprintf(&sb, "  Probability ..... %.4g by %s\n", p, method)
	}

	for i := range m.Objects {
		fmt.Fprintf(&sb, "  Object %d\n", i+1)
		sb.WriteString(m.Objects[i].Humanize())
	}
	return sb.String()
}

// Humanize returns a human-readable summary of one object.
func (o Object) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "    Name .......... %s\n", o.Name())
	fmt.Fprintf(&sb, "    Designator .... %s in %s\n", o.Designator(), o.CatalogName())
	if designator, ok := o.Get("INTERNATIONAL_DESIGNATOR"); ok {
		fmt.Fprintf(&sb, "    International . %s\n", designator)
	}

	switch canMove, given := o.Maneuverable(); {
	case !given:
		fmt.Fprintf(&sb, "    Maneuverable .. not stated\n")
	case canMove:
		fmt.Fprintf(&sb, "    Maneuverable .. yes\n")
	default:
		fmt.Fprintf(&sb, "    Maneuverable .. no\n")
	}

	if method, ok := o.Get("COVARIANCE_METHOD"); ok {
		fmt.Fprintf(&sb, "    Covariance .... %s, %dx%d\n", method, o.CovarianceOrder(), o.CovarianceOrder())
	}

	c := o.Covariance()
	fmt.Fprintf(&sb, "      1-sigma R/T/N %.1f %.1f %.1f m\n",
		sigma(c[0][0]), sigma(c[1][1]), sigma(c[2][2]))

	if position, velocity, ok := o.StateVector(); ok {
		fmt.Fprintf(&sb, "    Position ...... %.6f %.6f %.6f km in %s\n",
			position[0], position[1], position[2], o.RefFrame())
		fmt.Fprintf(&sb, "    Velocity ...... %.6f %.6f %.6f km/s\n",
			velocity[0], velocity[1], velocity[2])
	}
	return sb.String()
}

// sigma returns the square root of a variance, and zero for a negative one.
//
// A variance cannot be negative in a well-formed matrix, and this package does
// not check that a message's numbers are physically sensible, so a dump has to
// survive one that is not.
func sigma(variance float64) float64 {
	if variance <= 0 {
		return 0
	}
	return math.Sqrt(variance)
}
