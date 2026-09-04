package odm

import "github.com/ravisuhag/astro/internal/ndm"

// The spacecraft parameter and covariance blocks are shared. Table 3-3 and
// table 4-3 define them with the same keywords, the same units and the same
// all-or-nothing rule for the matrix, so the OPM and the OMM read and write
// them through the same code rather than through two copies that could drift.

// assignSpacecraftKeyword reads one spacecraft parameter.
func assignSpacecraftKeyword(s *SpacecraftParameters, keyword, value string) error {
	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "MASS":
		s.SetMass(v)
	case "SOLAR_RAD_AREA":
		s.SolarRadArea = v
	case "SOLAR_RAD_COEFF":
		s.SolarRadCoeff = v
	case "DRAG_AREA":
		s.DragArea = v
	case "DRAG_COEFF":
		s.DragCoeff = v
	}
	return nil
}

// writeSpacecraftParameters writes the block in table order.
func writeSpacecraftParameters(w *ndm.Writer, s *SpacecraftParameters) {
	w.Comments(s.Comments)
	if s.hasMass {
		w.AssignUnits("MASS", formatValue(s.Mass), "kg")
	}
	w.AssignUnits("SOLAR_RAD_AREA", formatValue(s.SolarRadArea), "m**2")
	w.Assign("SOLAR_RAD_COEFF", formatValue(s.SolarRadCoeff))
	w.AssignUnits("DRAG_AREA", formatValue(s.DragArea), "m**2")
	w.Assign("DRAG_COEFF", formatValue(s.DragCoeff))
}

// assignCovarianceKeyword reads one covariance keyword, filling both triangles
// because the matrix is symmetric by definition.
func assignCovarianceKeyword(c *Covariance, keyword, value string) error {
	if keyword == "COV_REF_FRAME" {
		c.RefFrame = value
		return nil
	}

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	at := covarianceIndex[keyword]
	c.Matrix[at[0]][at[1]] = v
	c.Matrix[at[1]][at[0]] = v
	return nil
}

// writeCovarianceKeywords writes the block as the 21 named keywords of
// clause 3.2.4.10, in lower triangular order.
//
// Note that this is not how an OEM carries a covariance matrix. There it is a
// run of positional values between COVARIANCE_START and COVARIANCE_STOP; here
// each element has its own keyword. The same matrix, two wire forms.
func writeCovarianceKeywords(w *ndm.Writer, c *Covariance) {
	w.Comments(c.Comments)
	if c.RefFrame != "" {
		w.Assign("COV_REF_FRAME", c.RefFrame)
	}
	for _, e := range covarianceElements {
		w.AssignUnits(e.keyword, formatValue(c.Matrix[e.row][e.col]), e.units)
	}
}
