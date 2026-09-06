package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The XML form, CCSDS 502.0-B-3 section 8 with the structure of
// CCSDS 505.0-B-3.
//
// Section 8 says the rules for which keywords may appear are the same as for
// the key-value form (clauses 8.8.8, 8.9.8, 8.10.8), and that the keyword tags
// are the keywords in upper case (clause 8.10.9). What the XML form adds is
// the block elements that the key-value form leaves implicit, and two
// attributes.
//
// Units become an attribute rather than a bracketed suffix. Clause 8.10.10
// makes them optional in XML as clause 7.7.1.1 does in KVN, so a message that
// omits them says the same thing.
//
// A user-defined parameter changes shape. In KVN the name is part of the
// keyword, USER_DEFINED_EARTH_MODEL; in XML the element is always
// USER_DEFINED and the name is an attribute.
//
// The OEM is where the two forms diverge most. Its ephemeris data lines are
// positional in KVN and one <stateVector> element each in XML, with every
// component named (clause 8.10.14). Its covariance matrices are positional
// rows in KVN and the OPM's named CX_X family in XML (clause 8.10.19). So the
// XML form of an OEM is the same numbers in a different shape, not the same
// text in a wrapper.

// XML block element names, from section 8 and the annex G examples.
const (
	xmlStateVector          = "stateVector"
	xmlKeplerianElements    = "keplerianElements"
	xmlMeanElements         = "meanElements"
	xmlSpacecraftParameters = "spacecraftParameters"
	xmlTLEParameters        = "tleParameters"
	xmlCovarianceMatrix     = "covarianceMatrix"
	xmlManeuverParameters   = "maneuverParameters"
	xmlUserDefinedParams    = "userDefinedParameters"
)

// xmlUnits gives the units clause 8.10.11 requires to match section 5, for
// every keyword that has one. A keyword absent from this map has none.
var xmlUnits = map[string]string{
	"X": "km", "Y": "km", "Z": "km",
	"X_DOT": "km/s", "Y_DOT": "km/s", "Z_DOT": "km/s",
	"X_DDOT": "km/s**2", "Y_DDOT": "km/s**2", "Z_DDOT": "km/s**2",
	"SEMI_MAJOR_AXIS": "km", "GM": "km**3/s**2",
	"INCLINATION": "deg", "RA_OF_ASC_NODE": "deg",
	"ARG_OF_PERICENTER": "deg", "TRUE_ANOMALY": "deg", "MEAN_ANOMALY": "deg",
	"MEAN_MOTION": "rev/day", "MEAN_MOTION_DOT": "rev/day**2",
	"MEAN_MOTION_DDOT": "rev/day**3",
	"BSTAR":            "1/[Earth radii]", "BTERM": "m**2/kg", "AGOM": "m**2/kg",
	"MASS": "kg", "SOLAR_RAD_AREA": "m**2", "DRAG_AREA": "m**2",
	"MAN_DURATION": "s", "MAN_DELTA_MASS": "kg",
	"MAN_DV_1": "km/s", "MAN_DV_2": "km/s", "MAN_DV_3": "km/s",
}

// leaf returns an element with the units section 5 gives its keyword.
func leaf(keyword, value string) ndm.Element {
	return ndm.LeafWithUnits(keyword, value, xmlUnits[keyword])
}

// covarianceLeaves returns the 21 covariance elements in the order
// clause 3.2.4.10 fixes, with the units each one's axes imply.
func covarianceLeaves(c *Covariance) []ndm.Element {
	out := ndm.Comments(c.Comments)
	if c.RefFrame != "" {
		out = append(out, ndm.Leaf("COV_REF_FRAME", c.RefFrame))
	}
	for _, e := range covarianceElements {
		out = append(out, ndm.LeafWithUnits(e.keyword, ndm.FormatValue(c.Matrix[e.row][e.col]), e.units))
	}
	return out
}

// readCovariance fills a covariance matrix from its elements.
func readCovariance(elements []ndm.Element) (*Covariance, error) {
	c := &Covariance{Comments: ndm.CollectComments(elements)}

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if err := assignCovarianceKeyword(c, e.Name, e.Value); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// readSpacecraft fills a spacecraft parameter block from its elements.
func readSpacecraft(elements []ndm.Element) (*SpacecraftParameters, error) {
	s := &SpacecraftParameters{Comments: ndm.CollectComments(elements)}

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if err := assignSpacecraftKeyword(s, e.Name, e.Value); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// userDefinedElements returns the user-defined block, or nothing when there
// are no parameters.
func userDefinedElements(params []UserDefined) ndm.Element {
	children := make([]ndm.Element, 0, len(params))
	for _, u := range params {
		children = append(children, ndm.UserDefined(strings.ToUpper(u.Name), u.Value))
	}
	return ndm.Block(xmlUserDefinedParams, children...)
}

// readUserDefined reads the user-defined block, taking each parameter's name
// from its attribute rather than from the element name.
//
// A parameter with no name is refused. The name is the whole of what a
// user-defined parameter means — clause 3.2.4.12 and table 6-12 both describe
// the keyword as USER_DEFINED_ with the name substituted in — and the
// key-value form of a nameless one is the bare 'USER_DEFINED_ = value', which
// this package's own reader will not take back.
func readUserDefined(elements []ndm.Element) ([]UserDefined, error) {
	var out []UserDefined
	for _, e := range elements {
		if e.Name != ndm.KeywordUserDefined {
			continue
		}
		if e.Parameter == "" {
			return nil, ndm.ErrEmptyKeyword
		}
		out = append(out, UserDefined{Name: e.Parameter, Value: e.Value})
	}
	return out, nil
}

// xmlHeader renders a header for the XML form.
func (h Header) xmlHeader() []ndm.Element {
	out := ndm.Comments(h.Comments)
	if h.Classification != "" {
		out = append(out, ndm.Leaf(ndm.KeywordClassification, h.Classification))
	}
	if created, err := ndm.FormatEpoch(h.CreationDate.UTC(), 0); err == nil {
		out = append(out, ndm.Leaf(ndm.KeywordCreationDate, created))
	}
	out = append(out, ndm.Leaf(ndm.KeywordOriginator, h.Originator))
	if h.MessageID != "" {
		out = append(out, ndm.Leaf(ndm.KeywordMessageID, h.MessageID))
	}
	return out
}

// readXMLHeader fills a header from its elements.
func readXMLHeader(version string, elements []ndm.Element) (Header, error) {
	h := Header{Version: version, Comments: ndm.CollectComments(elements)}

	created, ok := ndm.Find(elements, ndm.KeywordCreationDate)
	if !ok {
		return h, ndm.ErrMissingHeaderField
	}
	t, err := ndm.ParseEpoch(created)
	if err != nil {
		return h, err
	}
	h.CreationDate = t

	if h.Originator, ok = ndm.Find(elements, ndm.KeywordOriginator); !ok {
		return h, ndm.ErrMissingHeaderField
	}
	h.Classification, _ = ndm.Find(elements, ndm.KeywordClassification)
	h.MessageID, _ = ndm.Find(elements, ndm.KeywordMessageID)
	return h, nil
}
