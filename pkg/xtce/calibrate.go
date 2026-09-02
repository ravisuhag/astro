package xtce

import (
	"fmt"
	"math"
	"sort"
)

// Calibration: turning a raw count into an engineering value.
//
// Most analogue telemetry arrives as a plain integer from an analogue-to-
// digital converter, and the database says how to turn that count into volts
// or degrees. XTCE offers three ways to say it. Two are arithmetic and are
// implemented here; the third is an expression tree, which the model keeps as
// raw XML.

// Calibrate applies the calibrator to a raw value.
//
// A nil calibrator is the identity, which is what an uncalibrated parameter
// means: the raw count already is the engineering value.
func (c *Calibrator) Calibrate(raw float64) (float64, error) {
	switch {
	case c == nil:
		return raw, nil
	case c.Polynomial != nil:
		return c.Polynomial.Apply(raw), nil
	case c.Spline != nil:
		return c.Spline.Apply(raw)
	case c.MathOperation != nil:
		return c.MathOperation.Apply(raw)
	default:
		// A Calibrator element with nothing in it. The schema requires one of
		// the three, so this is a malformed database rather than a plain
		// uncalibrated parameter, but the identity is the harmless reading.
		return raw, nil
	}
}

// Apply evaluates the polynomial at raw.
//
// The terms are a sum of coefficient times raw to the power of exponent, and
// the schema does not require them in any order or without gaps, so each is
// evaluated on its own rather than by Horner's method.
func (p *PolynomialCalibrator) Apply(raw float64) float64 {
	var total float64
	for _, term := range p.Terms {
		total += term.Coefficient * math.Pow(raw, float64(term.Exponent))
	}
	return total
}

// Apply interpolates between the spline's points.
//
// Order 1 is a straight line between neighbouring points, which is what a
// calibration curve measured at a handful of temperatures looks like. Higher
// orders are in the schema but the schema does not say which spline they mean,
// and guessing at a curve would put wrong numbers in front of an operator.
func (s *SplineCalibrator) Apply(raw float64) (float64, error) {
	if order := s.Order; order > 1 {
		return 0, fmt.Errorf("%w: a spline of order %d", ErrUnsupportedCalibrator, order)
	}
	if len(s.Points) < 2 {
		return 0, fmt.Errorf("%w: a spline needs at least two points, this has %d",
			ErrUnsupportedCalibrator, len(s.Points))
	}

	// The schema does not require the points in order, and the search below
	// does. Sorting a copy leaves the loaded database as it was.
	points := make([]SplinePoint, len(s.Points))
	copy(points, s.Points)
	sort.Slice(points, func(i, j int) bool { return points[i].Raw < points[j].Raw })

	first, last := points[0], points[len(points)-1]

	// Outside the measured range, either extend the end segment or clamp. The
	// default is to clamp: a curve measured between 0 and 5 volts says nothing
	// about 40 volts, and inventing a number there is worse than repeating the
	// last one you have evidence for.
	if raw <= first.Raw {
		if s.Extrapolate && len(points) >= 2 {
			return interpolate(points[0], points[1], raw), nil
		}
		return first.Calibrated, nil
	}
	if raw >= last.Raw {
		if s.Extrapolate && len(points) >= 2 {
			return interpolate(points[len(points)-2], last, raw), nil
		}
		return last.Calibrated, nil
	}

	// Find the segment the value falls in.
	i := sort.Search(len(points), func(i int) bool { return points[i].Raw > raw })
	return interpolate(points[i-1], points[i], raw), nil
}

// interpolate draws the straight line through two points and reads it at raw.
func interpolate(low, high SplinePoint, raw float64) float64 {
	span := high.Raw - low.Raw
	if span == 0 {
		// Two points at the same raw value. Neither is more right than the
		// other, so take the first.
		return low.Calibrated
	}
	return low.Calibrated + (raw-low.Raw)*(high.Calibrated-low.Calibrated)/span
}

// defaultCalibrator returns the calibrator on a type's data encoding, if any.
//
// Only the integer and float encodings carry one, which follows from what a
// calibrator is for: there is no curve that turns text into a number.
func defaultCalibrator(t ParameterType) *Calibrator {
	encoding := t.Encoding()
	switch {
	case encoding == nil:
		return nil
	case encoding.Integer != nil:
		return encoding.Integer.DefaultCalibrator
	case encoding.Float != nil:
		return encoding.Float.DefaultCalibrator
	default:
		return nil
	}
}
