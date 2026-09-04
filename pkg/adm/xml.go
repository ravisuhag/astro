package adm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The XML form, CCSDS 504.0-B-2 section 7 with the structure of
// CCSDS 505.0-B-3.
//
// The attitude messages nest further than the orbit ones. An APM quaternion is
// not a flat run of four keywords: clause 7.5.11 puts the frames and the
// components in a <quaternionState>, and the components themselves in a
// <quaternion> inside that, with the optional derivatives in a sibling
// <quaternionDot>.
//
// The AEM goes further still. Table 7-5 gives each of the nine attitude types
// its own inner element, so an <attitudeState> wraps a
// <quaternionEphemeris> or a <spinNutationMom> depending on what the
// segment's ATTITUDE_TYPE said. The key-value form expresses the same choice
// by changing how many numbers are on a line.
//
// So converting an AEM between the forms means mapping a positional row onto
// named elements whose names come from the metadata. attitudeValueNames is
// that mapping, and it is table 4-4 again from the other side.

// APM data block elements (clause 7.5.11).
const (
	xmlQuaternionState = "quaternionState"
	xmlQuaternion      = "quaternion"
	xmlQuaternionDot   = "quaternionDot"
	xmlEulerAngleState = "eulerAngleState"
	xmlAngularVelocity = "angularVelocity"
	xmlSpin            = "spin"
	xmlInertia         = "inertia"
	xmlManeuver        = "maneuverParameters"
	xmlAttitudeState   = "attitudeState"
	xmlAngVel          = "angVel"
)

// attitudeInnerElement is table 7-5: the element that sits inside
// <attitudeState>, chosen by the segment's attitude type.
var attitudeInnerElement = map[AttitudeType]string{
	Quaternion4:          "quaternionEphemeris",
	QuaternionDerivative: "quaternionDerivative",
	QuaternionAngVel:     "quaternionAngVel",
	EulerAngle:           "eulerAngle",
	EulerAngleDerivative: "eulerAngleDerivative",
	EulerAngleAngVel:     "eulerAngleAngVel",
	SpinType:             "spin",
	SpinNutation:         "spinNutation",
	SpinNutationMomentum: "spinNutationMom",
}

// attitudeValueNames names the values on a data line, in the order table 4-4
// puts them. This is the same table the key-value form reads as a width; here
// it is read as a list of element names.
var attitudeValueNames = map[AttitudeType][]string{
	Quaternion4:          {"Q1", "Q2", "Q3", "QC"},
	QuaternionDerivative: {"Q1", "Q2", "Q3", "QC", "Q1_DOT", "Q2_DOT", "Q3_DOT", "QC_DOT"},
	QuaternionAngVel:     {"Q1", "Q2", "Q3", "QC", "ANGVEL_X", "ANGVEL_Y", "ANGVEL_Z"},
	EulerAngle:           {"ANGLE_1", "ANGLE_2", "ANGLE_3"},
	EulerAngleDerivative: {"ANGLE_1", "ANGLE_2", "ANGLE_3", "ANGLE_1_DOT", "ANGLE_2_DOT", "ANGLE_3_DOT"},
	EulerAngleAngVel:     {"ANGLE_1", "ANGLE_2", "ANGLE_3", "ANGVEL_X", "ANGVEL_Y", "ANGVEL_Z"},
	SpinType:             {"SPIN_ALPHA", "SPIN_DELTA", "SPIN_ANGLE", "SPIN_ANGLE_VEL"},
	SpinNutation: {"SPIN_ALPHA", "SPIN_DELTA", "SPIN_ANGLE", "SPIN_ANGLE_VEL",
		"NUTATION", "NUTATION_PER", "NUTATION_PHASE"},
	SpinNutationMomentum: {"SPIN_ALPHA", "SPIN_DELTA", "SPIN_ANGLE", "SPIN_ANGLE_VEL",
		"MOMENTUM_ALPHA", "MOMENTUM_DELTA", "NUTATION_VEL"},
}

// xmlUnits gives the units section 3 assigns each keyword.
var xmlUnits = map[string]string{
	"ANGLE_1": "deg", "ANGLE_2": "deg", "ANGLE_3": "deg",
	"ANGLE_1_DOT": "deg/s", "ANGLE_2_DOT": "deg/s", "ANGLE_3_DOT": "deg/s",
	"Q1_DOT": "1/s", "Q2_DOT": "1/s", "Q3_DOT": "1/s", "QC_DOT": "1/s",
	"ANGVEL_X": "deg/s", "ANGVEL_Y": "deg/s", "ANGVEL_Z": "deg/s",
	"SPIN_ALPHA": "deg", "SPIN_DELTA": "deg", "SPIN_ANGLE": "deg",
	"SPIN_ANGLE_VEL": "deg/s",
	"NUTATION":       "deg", "NUTATION_PER": "s", "NUTATION_PHASE": "deg",
	"MOMENTUM_ALPHA": "deg", "MOMENTUM_DELTA": "deg", "NUTATION_VEL": "deg/s",
	"IXX": "kg*m**2", "IYY": "kg*m**2", "IZZ": "kg*m**2",
	"IXY": "kg*m**2", "IXZ": "kg*m**2", "IYZ": "kg*m**2",
	"MAN_DURATION": "s",
	"MAN_TOR_X":    "N*m", "MAN_TOR_Y": "N*m", "MAN_TOR_Z": "N*m",
}

// leaf returns an element with the units section 3 gives its keyword.
func leaf(keyword, value string) ndm.Element {
	return ndm.LeafWithUnits(keyword, value, xmlUnits[keyword])
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
	t, err := parseEpoch(created)
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

// framesElements renders the pair of frames a transformation block carries.
func framesElements(f frames) []ndm.Element {
	return []ndm.Element{
		ndm.Leaf("REF_FRAME_A", f.FrameA),
		ndm.Leaf("REF_FRAME_B", f.FrameB),
	}
}

// readFrames fills a frame pair from its elements.
func readFrames(f *frames, elements []ndm.Element) {
	f.FrameA, _ = ndm.Find(elements, "REF_FRAME_A")
	f.FrameB, _ = ndm.Find(elements, "REF_FRAME_B")
}

// textKeywords are the block keywords whose value is a name rather than a
// number. numbers skips them; parsing "YXY" as a float is not an error in the
// message, it is an error in the reader.
var textKeywords = map[string]bool{
	"REF_FRAME_A": true, "REF_FRAME_B": true,
	"EULER_ROT_SEQ": true, "ANGVEL_FRAME": true,
	"INERTIA_REF_FRAME": true, "MAN_REF_FRAME": true,
	"MAN_EPOCH_START": true,
}

// numbers reads the numeric elements of a block into a map, leaving the text
// ones to their own accessors.
func numbers(elements []ndm.Element) (map[string]float64, error) {
	out := make(map[string]float64, len(elements))
	for _, e := range elements {
		if e.Name == ndm.KeywordComment || len(e.Children) > 0 || textKeywords[e.Name] {
			continue
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return nil, err
		}
		out[e.Name] = v
	}
	return out, nil
}

// upperTrim normalises a value that names one of a fixed set, since the
// document writes some of them in lower case.
func upperTrim(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
