package odm

import "github.com/ravisuhag/astro/internal/ndm"

// EncodeXML writes the message in the XML form (clause 8.9).
func (m *OMM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	md := m.Metadata
	metadata := ndm.Comments(md.Comments)
	metadata = append(metadata,
		ndm.Leaf("OBJECT_NAME", md.ObjectName),
		ndm.Leaf("OBJECT_ID", md.ObjectID),
		ndm.Leaf("CENTER_NAME", md.CenterName),
		ndm.Leaf("REF_FRAME", md.RefFrame),
	)
	if md.RefFrameEpoch != nil {
		if epoch, err := ndm.FormatEpoch(*md.RefFrameEpoch, 0); err == nil {
			metadata = append(metadata, ndm.Leaf("REF_FRAME_EPOCH", epoch))
		}
	}
	metadata = append(metadata,
		ndm.Leaf("TIME_SYSTEM", md.TimeSystem),
		ndm.Leaf("MEAN_ELEMENT_THEORY", md.MeanElementTheory),
	)

	message := &ndm.XMLMessage{
		Root:    "omm",
		ID:      "CCSDS_OMM_VERS",
		Version: m.Header.Version,
		Header:  m.Header.xmlHeader(),
		Segments: []ndm.Segment{{
			Metadata: metadata,
			Data:     m.xmlData(),
		}},
	}
	return message.EncodeXML()
}

func (m *OMM) xmlData() []ndm.Element {
	e := m.Data.Elements

	elements := ndm.Comments(e.Comments)
	if epoch, err := ndm.FormatEpoch(e.Epoch, epochPrecision(e.Epoch)); err == nil {
		elements = append(elements, ndm.Leaf("EPOCH", epoch))
	}
	// Which of the paired keywords goes out is the same decision the
	// key-value form makes, and it has to survive the change of form.
	if e.UsesMeanMotion {
		elements = append(elements, leaf("MEAN_MOTION", formatValue(e.MeanMotion)))
	} else {
		elements = append(elements, leaf("SEMI_MAJOR_AXIS", formatValue(e.SemiMajorAxis)))
	}
	elements = append(elements,
		ndm.Leaf("ECCENTRICITY", formatValue(e.Eccentricity)),
		leaf("INCLINATION", formatValue(e.Inclination)),
		leaf("RA_OF_ASC_NODE", formatValue(e.RAOfAscNode)),
		leaf("ARG_OF_PERICENTER", formatValue(e.ArgOfPericenter)),
		leaf("MEAN_ANOMALY", formatValue(e.MeanAnomaly)),
	)
	if e.GM != 0 {
		elements = append(elements, leaf("GM", formatValue(e.GM)))
	}

	out := []ndm.Element{ndm.Block(xmlMeanElements, elements...)}

	if s := m.Data.Spacecraft; s != nil {
		out = append(out, ndm.Block(xmlSpacecraftParameters, spacecraftLeaves(s)...))
	}
	if t := m.Data.TLE; t != nil {
		children := ndm.Comments(t.Comments)
		children = append(children,
			ndm.Leaf("EPHEMERIS_TYPE", ndm.FormatInt(t.EphemerisType)),
			ndm.Leaf("CLASSIFICATION_TYPE", t.ClassificationType),
			ndm.Leaf("NORAD_CAT_ID", ndm.FormatInt(t.NoradCatID)),
			ndm.Leaf("ELEMENT_SET_NO", ndm.FormatInt(t.ElementSetNo)),
			ndm.Leaf("REV_AT_EPOCH", ndm.FormatInt(t.RevAtEpoch)),
		)
		if t.UsesBTerm {
			children = append(children, leaf("BTERM", formatValue(t.BTerm)))
		} else {
			children = append(children, leaf("BSTAR", formatValue(t.BStar)))
		}
		children = append(children, leaf("MEAN_MOTION_DOT", formatValue(t.MeanMotionDot)))
		if t.UsesAgom {
			children = append(children, leaf("AGOM", formatValue(t.Agom)))
		} else {
			children = append(children, leaf("MEAN_MOTION_DDOT", formatValue(t.MeanMotionDDot)))
		}
		out = append(out, ndm.Block(xmlTLEParameters, children...))
	}
	if c := m.Data.Covariance; c != nil {
		out = append(out, ndm.Block(xmlCovarianceMatrix, covarianceLeaves(c)...))
	}
	return append(out, userDefinedElements(m.Data.UserDefined))
}

// DecodeXMLOMM reads an Orbit Mean-Elements Message in the XML form.
func DecodeXMLOMM(data []byte) (*OMM, error) {
	message, err := ndm.DecodeXML(data, "omm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_OMM_VERS" {
		return nil, ErrNotAnOMM
	}
	if len(message.Segments) != 1 {
		return nil, ndm.ErrMalformedXML
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &OMM{Header: header}

	segment := message.Segments[0]
	if err := m.readXMLMetadata(segment.Metadata); err != nil {
		return nil, err
	}
	if err := m.readXMLData(segment.Data); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *OMM) readXMLMetadata(elements []ndm.Element) error {
	md := &m.Metadata
	md.Comments = ndm.CollectComments(elements)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if ommKeywordBlock[e.Name] != ommMetadata {
			return ErrUnknownKeyword
		}
		switch e.Name {
		case "OBJECT_NAME":
			md.ObjectName = ndm.ParseText(e.Value)
		case "OBJECT_ID":
			md.ObjectID = ndm.ParseText(e.Value)
		case "CENTER_NAME":
			md.CenterName = ndm.ParseText(e.Value)
		case "REF_FRAME":
			md.RefFrame = e.Value
		case "TIME_SYSTEM":
			md.TimeSystem = e.Value
		case "MEAN_ELEMENT_THEORY":
			md.MeanElementTheory = e.Value
		case "REF_FRAME_EPOCH":
			t, err := parseEpochValue(e.Value)
			if err != nil {
				return err
			}
			md.RefFrameEpoch = &t
		}
	}
	return nil
}

func (m *OMM) readXMLData(elements []ndm.Element) error {
	seenElements := false

	for _, e := range elements {
		if len(e.Children) == 0 {
			if e.Name == ndm.KeywordComment {
				continue
			}
			return ErrUnknownKeyword
		}

		switch e.Name {
		case xmlMeanElements:
			if err := m.readXMLMeanElements(e.Children); err != nil {
				return err
			}
			seenElements = true
		case xmlSpacecraftParameters:
			s, err := readSpacecraft(e.Children)
			if err != nil {
				return err
			}
			m.Data.Spacecraft = s
		case xmlTLEParameters:
			t, err := readXMLTLE(e.Children)
			if err != nil {
				return err
			}
			m.Data.TLE = t
		case xmlCovarianceMatrix:
			c, err := readCovariance(e.Children)
			if err != nil {
				return err
			}
			m.Data.Covariance = c
		case xmlUserDefinedParams:
			m.Data.UserDefined = readUserDefined(e.Children)
		default:
			return ErrUnknownKeyword
		}
	}
	if !seenElements {
		return ErrMissingKeyword
	}
	return nil
}

func (m *OMM) readXMLMeanElements(elements []ndm.Element) error {
	e := &m.Data.Elements
	e.Comments = ndm.CollectComments(elements)
	seenAxis, seenMotion := false, false

	for _, el := range elements {
		if el.Name == ndm.KeywordComment {
			continue
		}
		if el.Name == "EPOCH" {
			t, err := parseEpochValue(el.Value)
			if err != nil {
				return err
			}
			e.Epoch = t
			continue
		}
		v, err := ndm.ParseFloat(el.Value)
		if err != nil {
			return err
		}
		switch el.Name {
		case "SEMI_MAJOR_AXIS":
			seenAxis = true
			e.SemiMajorAxis, e.UsesMeanMotion = v, false
		case "MEAN_MOTION":
			seenMotion = true
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
		default:
			return ErrUnknownKeyword
		}
	}
	if seenAxis && seenMotion {
		return ErrBothSizeKeywords
	}
	if !seenAxis && !seenMotion {
		return ErrSizeKeywordMissing
	}
	return nil
}

func readXMLTLE(elements []ndm.Element) (*TLEParameters, error) {
	// Clause 4.2.4.7's defaults apply the same way in either form.
	t := &TLEParameters{ClassificationType: "U", Comments: ndm.CollectComments(elements)}
	seenBStar, seenBTerm := false, false
	seenDDot, seenAgom := false, false

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		switch e.Name {
		case "CLASSIFICATION_TYPE":
			t.ClassificationType = e.Value
			continue
		case "EPHEMERIS_TYPE", "NORAD_CAT_ID", "ELEMENT_SET_NO", "REV_AT_EPOCH":
			n, err := ndm.ParseInt(e.Value)
			if err != nil {
				return nil, err
			}
			switch e.Name {
			case "EPHEMERIS_TYPE":
				t.EphemerisType = n
			case "NORAD_CAT_ID":
				t.NoradCatID = n
			case "ELEMENT_SET_NO":
				t.ElementSetNo = n
			case "REV_AT_EPOCH":
				t.RevAtEpoch = n
			}
			continue
		}

		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return nil, err
		}
		switch e.Name {
		case "BSTAR":
			seenBStar = true
			t.BStar, t.UsesBTerm = v, false
		case "BTERM":
			seenBTerm = true
			t.BTerm, t.UsesBTerm = v, true
		case "MEAN_MOTION_DOT":
			t.MeanMotionDot = v
		case "MEAN_MOTION_DDOT":
			seenDDot = true
			t.MeanMotionDDot, t.UsesAgom = v, false
		case "AGOM":
			seenAgom = true
			t.Agom, t.UsesAgom = v, true
		default:
			return nil, ErrUnknownKeyword
		}
	}
	if (seenBStar && seenBTerm) || (seenDDot && seenAgom) {
		return nil, ErrBothDragKeywords
	}
	return t, nil
}
