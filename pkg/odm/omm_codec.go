package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// ommBlock names the logical section an OMM keyword belongs to (table 4-3).
type ommBlock int

const (
	ommNone ommBlock = iota
	ommMetadata
	ommElements
	ommSpacecraft
	ommTLE
	ommCovariance
	ommUserDefined
)

var ommKeywordBlock = map[string]ommBlock{
	"OBJECT_NAME": ommMetadata, "OBJECT_ID": ommMetadata,
	"CENTER_NAME": ommMetadata, "REF_FRAME": ommMetadata,
	"REF_FRAME_EPOCH": ommMetadata, "TIME_SYSTEM": ommMetadata,
	"MEAN_ELEMENT_THEORY": ommMetadata,

	"EPOCH": ommElements, "SEMI_MAJOR_AXIS": ommElements, "MEAN_MOTION": ommElements,
	"ECCENTRICITY": ommElements, "INCLINATION": ommElements,
	"RA_OF_ASC_NODE": ommElements, "ARG_OF_PERICENTER": ommElements,
	"MEAN_ANOMALY": ommElements, "GM": ommElements,

	"MASS": ommSpacecraft, "SOLAR_RAD_AREA": ommSpacecraft,
	"SOLAR_RAD_COEFF": ommSpacecraft, "DRAG_AREA": ommSpacecraft,
	"DRAG_COEFF": ommSpacecraft,

	"EPHEMERIS_TYPE": ommTLE, "CLASSIFICATION_TYPE": ommTLE,
	"NORAD_CAT_ID": ommTLE, "ELEMENT_SET_NO": ommTLE, "REV_AT_EPOCH": ommTLE,
	"BSTAR": ommTLE, "BTERM": ommTLE, "MEAN_MOTION_DOT": ommTLE,
	"MEAN_MOTION_DDOT": ommTLE, "AGOM": ommTLE,

	"COV_REF_FRAME": ommCovariance,
}

func init() {
	for keyword := range covarianceIndex {
		ommKeywordBlock[keyword] = ommCovariance
	}
}

// DecodeOMM reads an Orbit Mean-Elements Message in 'keyword = value' notation.
func DecodeOMM(data []byte) (*OMM, error) {
	s := ndm.NewScanner(data, true)

	header, err := ndm.ReadHeader(s, ommHeaderSpec)
	if err != nil {
		return nil, err
	}

	d := &ommDecoder{
		message: &OMM{Header: headerFromNDM(header)},
		seen:    make(map[string]bool),
	}
	if err := d.run(s); err != nil {
		return nil, err
	}
	if err := d.finish(); err != nil {
		return nil, err
	}
	if err := d.message.Validate(); err != nil {
		return nil, err
	}
	return d.message, nil
}

type ommDecoder struct {
	message *OMM
	seen    map[string]bool
	pending []string
	current ommBlock
}

func (d *ommDecoder) run(s *ndm.Scanner) error {
	for s.Scan() {
		line := s.Line()

		switch line.Kind {
		case ndm.Blank:
			continue
		case ndm.Comment:
			d.pending = append(d.pending, line.Value)
			continue
		case ndm.Free:
			return ndm.At(line.Number, ErrUnknownKeyword)
		}

		keyword, value, err := line.Assignment()
		if err != nil {
			return ndm.At(line.Number, err)
		}
		if err := d.assign(keyword, value); err != nil {
			return ndm.At(line.Number, err)
		}
	}
	return s.Err()
}

func (d *ommDecoder) assign(keyword, value string) error {
	if name, ok := strings.CutPrefix(keyword, userDefinedPrefix); ok {
		d.enter(ommUserDefined)
		d.message.Data.UserDefined = append(d.message.Data.UserDefined,
			UserDefined{Name: name, Value: value})
		return nil
	}

	target, known := ommKeywordBlock[keyword]
	if !known {
		return ErrUnknownKeyword
	}
	if d.seen[keyword] {
		return ErrDuplicateKeyword
	}
	d.seen[keyword] = true
	d.enter(target)

	switch target {
	case ommMetadata:
		return d.assignMetadata(keyword, value)
	case ommElements:
		return d.assignElements(keyword, value)
	case ommSpacecraft:
		return assignSpacecraftKeyword(d.spacecraft(), keyword, value)
	case ommTLE:
		return d.assignTLE(keyword, value)
	case ommCovariance:
		return assignCovarianceKeyword(d.covariance(), keyword, value)
	}
	return ErrUnknownKeyword
}

func (d *ommDecoder) enter(target ommBlock) {
	if target == d.current {
		return
	}
	d.current = target
	if len(d.pending) == 0 {
		return
	}
	comments := d.pending
	d.pending = nil

	switch target {
	case ommMetadata:
		d.message.Metadata.Comments = append(d.message.Metadata.Comments, comments...)
	case ommElements:
		d.message.Data.Elements.Comments = append(d.message.Data.Elements.Comments, comments...)
	case ommSpacecraft:
		d.spacecraft().Comments = append(d.spacecraft().Comments, comments...)
	case ommTLE:
		d.tle().Comments = append(d.tle().Comments, comments...)
	case ommCovariance:
		d.covariance().Comments = append(d.covariance().Comments, comments...)
	}
}

func (d *ommDecoder) spacecraft() *SpacecraftParameters {
	if d.message.Data.Spacecraft == nil {
		d.message.Data.Spacecraft = &SpacecraftParameters{}
	}
	return d.message.Data.Spacecraft
}

func (d *ommDecoder) tle() *TLEParameters {
	if d.message.Data.TLE == nil {
		// Clause 4.2.4.7 gives these two defaults, and a message that omits
		// them means them rather than meaning zero and empty.
		d.message.Data.TLE = &TLEParameters{ClassificationType: "U"}
	}
	return d.message.Data.TLE
}

func (d *ommDecoder) covariance() *Covariance {
	if d.message.Data.Covariance == nil {
		d.message.Data.Covariance = &Covariance{}
	}
	return d.message.Data.Covariance
}

func (d *ommDecoder) assignMetadata(keyword, value string) error {
	md := &d.message.Metadata

	switch keyword {
	case "OBJECT_NAME":
		md.ObjectName = ndm.ParseText(value)
	case "OBJECT_ID":
		md.ObjectID = ndm.ParseText(value)
	case "CENTER_NAME":
		md.CenterName = ndm.ParseText(value)
	case "REF_FRAME":
		md.RefFrame = value
	case "TIME_SYSTEM":
		md.TimeSystem = value
	case "MEAN_ELEMENT_THEORY":
		md.MeanElementTheory = value
	case "REF_FRAME_EPOCH":
		t, err := parseEpochValue(value)
		if err != nil {
			return err
		}
		md.RefFrameEpoch = &t
	}
	return nil
}

func (d *ommDecoder) assignElements(keyword, value string) error {
	e := &d.message.Data.Elements

	if keyword == "EPOCH" {
		t, err := parseEpochValue(value)
		if err != nil {
			return err
		}
		e.Epoch = t
		return nil
	}

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "SEMI_MAJOR_AXIS":
		if d.seen["MEAN_MOTION"] {
			return ErrBothSizeKeywords
		}
		e.SemiMajorAxis = v
	case "MEAN_MOTION":
		if d.seen["SEMI_MAJOR_AXIS"] {
			return ErrBothSizeKeywords
		}
		e.MeanMotion, e.UsesMeanMotion = v, true
	case "ECCENTRICITY":
		e.Eccentricity = v
	case "INCLINATION":
		e.Inclination = v
	case "RA_OF_ASC_NODE":
		e.RAOfAscNode = v
	case "ARG_OF_PERICENTER":
		e.ArgOfPericenter = v
	case "MEAN_ANOMALY":
		e.MeanAnomaly = v
	case "GM":
		e.GM = v
	}
	return nil
}

func (d *ommDecoder) assignTLE(keyword, value string) error {
	t := d.tle()

	switch keyword {
	case "CLASSIFICATION_TYPE":
		t.ClassificationType = value
		return nil
	case "EPHEMERIS_TYPE", "NORAD_CAT_ID", "ELEMENT_SET_NO", "REV_AT_EPOCH":
		n, err := ndm.ParseInt(value)
		if err != nil {
			return err
		}
		switch keyword {
		case "EPHEMERIS_TYPE":
			t.EphemerisType = n
		case "NORAD_CAT_ID":
			t.NoradCatID = n
		case "ELEMENT_SET_NO":
			t.ElementSetNo = n
		case "REV_AT_EPOCH":
			t.RevAtEpoch = n
		}
		return nil
	}

	v, err := parseValue(value)
	if err != nil {
		return err
	}
	switch keyword {
	case "BSTAR":
		if d.seen["BTERM"] {
			return ErrBothDragKeywords
		}
		t.BStar = v
	case "BTERM":
		if d.seen["BSTAR"] {
			return ErrBothDragKeywords
		}
		t.BTerm, t.UsesBTerm = v, true
	case "MEAN_MOTION_DOT":
		t.MeanMotionDot = v
	case "MEAN_MOTION_DDOT":
		if d.seen["AGOM"] {
			return ErrBothDragKeywords
		}
		t.MeanMotionDDot = v
	case "AGOM":
		if d.seen["MEAN_MOTION_DDOT"] {
			return ErrBothDragKeywords
		}
		t.Agom, t.UsesAgom = v, true
	}
	return nil
}

// finish checks what only the whole message can answer.
func (d *ommDecoder) finish() error {
	// Table 4-3 makes the pair mandatory even though each alternative is
	// conditional, so neither one present is an error rather than a default.
	if !d.seen["SEMI_MAJOR_AXIS"] && !d.seen["MEAN_MOTION"] {
		return ErrSizeKeywordMissing
	}
	for _, keyword := range []string{
		"ECCENTRICITY", "INCLINATION", "RA_OF_ASC_NODE",
		"ARG_OF_PERICENTER", "MEAN_ANOMALY",
	} {
		if !d.seen[keyword] {
			return ErrMissingKeyword
		}
	}
	return nil
}

// Encode writes the message in 'keyword = value' notation.
func (m *OMM) Encode() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	var w ndm.Writer
	if err := m.Header.toNDM().Write(&w, ommHeaderSpec); err != nil {
		return nil, err
	}

	md := m.Metadata
	w.Comments(md.Comments)
	w.Assign("OBJECT_NAME", md.ObjectName)
	w.Assign("OBJECT_ID", md.ObjectID)
	w.Assign("CENTER_NAME", md.CenterName)
	w.Assign("REF_FRAME", md.RefFrame)
	if md.RefFrameEpoch != nil {
		if err := writeEpoch(&w, "REF_FRAME_EPOCH", *md.RefFrameEpoch); err != nil {
			return nil, err
		}
	}
	w.Assign("TIME_SYSTEM", md.TimeSystem)
	w.Assign("MEAN_ELEMENT_THEORY", md.MeanElementTheory)

	e := m.Data.Elements
	w.Comments(e.Comments)
	if err := writeEpoch(&w, "EPOCH", e.Epoch); err != nil {
		return nil, err
	}
	if e.UsesMeanMotion {
		w.AssignUnits("MEAN_MOTION", formatValue(e.MeanMotion), "rev/day")
	} else {
		w.AssignUnits("SEMI_MAJOR_AXIS", formatValue(e.SemiMajorAxis), "km")
	}
	w.Assign("ECCENTRICITY", formatValue(e.Eccentricity))
	w.AssignUnits("INCLINATION", formatValue(e.Inclination), "deg")
	w.AssignUnits("RA_OF_ASC_NODE", formatValue(e.RAOfAscNode), "deg")
	w.AssignUnits("ARG_OF_PERICENTER", formatValue(e.ArgOfPericenter), "deg")
	w.AssignUnits("MEAN_ANOMALY", formatValue(e.MeanAnomaly), "deg")
	if e.GM != 0 {
		w.AssignUnits("GM", formatValue(e.GM), "km**3/s**2")
	}

	if s := m.Data.Spacecraft; s != nil {
		writeSpacecraftParameters(&w, s)
	}
	if t := m.Data.TLE; t != nil {
		writeTLEParameters(&w, t)
	}
	if c := m.Data.Covariance; c != nil {
		writeCovarianceKeywords(&w, c)
	}
	for _, u := range m.Data.UserDefined {
		w.Assign(userDefinedPrefix+strings.ToUpper(u.Name), u.Value)
	}
	return w.Bytes(), nil
}

func writeTLEParameters(w *ndm.Writer, t *TLEParameters) {
	w.Comments(t.Comments)
	w.Assign("EPHEMERIS_TYPE", ndm.FormatInt(t.EphemerisType))
	w.Assign("CLASSIFICATION_TYPE", t.ClassificationType)
	w.Assign("NORAD_CAT_ID", ndm.FormatInt(t.NoradCatID))
	w.Assign("ELEMENT_SET_NO", ndm.FormatInt(t.ElementSetNo))
	w.Assign("REV_AT_EPOCH", ndm.FormatInt(t.RevAtEpoch))

	if t.UsesBTerm {
		w.AssignUnits("BTERM", formatValue(t.BTerm), "m**2/kg")
	} else {
		w.AssignUnits("BSTAR", formatValue(t.BStar), "1/[Earth radii]")
	}
	w.AssignUnits("MEAN_MOTION_DOT", formatValue(t.MeanMotionDot), "rev/day**2")
	if t.UsesAgom {
		w.AssignUnits("AGOM", formatValue(t.Agom), "m**2/kg")
	} else {
		w.AssignUnits("MEAN_MOTION_DDOT", formatValue(t.MeanMotionDDot), "rev/day**3")
	}
}
