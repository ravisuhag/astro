package odm

import "github.com/ravisuhag/astro/internal/ndm"

// EncodeXML writes the message in the XML form (clause 8.8).
func (m *OPM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "opm",
		ID:      "CCSDS_OPM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaODM,
		Header:  m.Header.xmlHeader(),
		Segments: []ndm.Segment{{
			Metadata: m.xmlMetadata(),
			Data:     m.xmlData(),
		}},
	}
	return message.EncodeXML()
}

func (m *OPM) xmlMetadata() []ndm.Element {
	md := m.Metadata

	out := ndm.Comments(md.Comments)
	out = append(out,
		ndm.Leaf("OBJECT_NAME", md.ObjectName),
		ndm.Leaf("OBJECT_ID", md.ObjectID),
		ndm.Leaf("CENTER_NAME", md.CenterName),
		ndm.Leaf("REF_FRAME", md.RefFrame),
	)
	if md.RefFrameEpoch != nil {
		if epoch, err := ndm.FormatEpoch(*md.RefFrameEpoch, 0); err == nil {
			out = append(out, ndm.Leaf("REF_FRAME_EPOCH", epoch))
		}
	}
	return append(out, ndm.Leaf("TIME_SYSTEM", md.TimeSystem))
}

func (m *OPM) xmlData() []ndm.Element {
	sv := m.Data.StateVector

	state := ndm.Comments(sv.Comments)
	if epoch, err := ndm.FormatEpoch(sv.Epoch, epochPrecision(sv.Epoch)); err == nil {
		state = append(state, ndm.Leaf("EPOCH", epoch))
	}
	state = append(state,
		leaf("X", formatValue(sv.X)), leaf("Y", formatValue(sv.Y)), leaf("Z", formatValue(sv.Z)),
		leaf("X_DOT", formatValue(sv.XDot)),
		leaf("Y_DOT", formatValue(sv.YDot)),
		leaf("Z_DOT", formatValue(sv.ZDot)),
	)

	out := []ndm.Element{ndm.Block(xmlStateVector, state...)}

	if k := m.Data.Keplerian; k != nil {
		anomaly := "TRUE_ANOMALY"
		if k.AnomalyIsMean {
			anomaly = "MEAN_ANOMALY"
		}
		children := ndm.Comments(k.Comments)
		children = append(children,
			leaf("SEMI_MAJOR_AXIS", formatValue(k.SemiMajorAxis)),
			ndm.Leaf("ECCENTRICITY", formatValue(k.Eccentricity)),
			leaf("INCLINATION", formatValue(k.Inclination)),
			leaf("RA_OF_ASC_NODE", formatValue(k.RAOfAscNode)),
			leaf("ARG_OF_PERICENTER", formatValue(k.ArgOfPericenter)),
			leaf(anomaly, formatValue(k.Anomaly)),
			leaf("GM", formatValue(k.GM)),
		)
		out = append(out, ndm.Block(xmlKeplerianElements, children...))
	}

	if s := m.Data.Spacecraft; s != nil {
		out = append(out, ndm.Block(xmlSpacecraftParameters, spacecraftLeaves(s)...))
	}
	if c := m.Data.Covariance; c != nil {
		out = append(out, ndm.Block(xmlCovarianceMatrix, covarianceLeaves(c)...))
	}

	// Clause 8.8.14: each manoeuvre is its own block, so the repetition the
	// key-value form expresses by repeating keywords becomes repeated
	// elements.
	for _, man := range m.Data.Maneuvers {
		children := ndm.Comments(man.Comments)
		if epoch, err := ndm.FormatEpoch(man.EpochIgnition, epochPrecision(man.EpochIgnition)); err == nil {
			children = append(children, ndm.Leaf("MAN_EPOCH_IGNITION", epoch))
		}
		children = append(children,
			leaf("MAN_DURATION", formatValue(man.Duration)),
			leaf("MAN_DELTA_MASS", formatValue(man.DeltaMass)),
			ndm.Leaf("MAN_REF_FRAME", man.RefFrame),
			leaf("MAN_DV_1", formatValue(man.DV[0])),
			leaf("MAN_DV_2", formatValue(man.DV[1])),
			leaf("MAN_DV_3", formatValue(man.DV[2])),
		)
		out = append(out, ndm.Block(xmlManeuverParameters, children...))
	}

	return append(out, userDefinedElements(m.Data.UserDefined))
}

// spacecraftLeaves renders a spacecraft parameter block.
func spacecraftLeaves(s *SpacecraftParameters) []ndm.Element {
	out := ndm.Comments(s.Comments)
	if s.hasMass {
		out = append(out, leaf("MASS", formatValue(s.Mass)))
	}
	return append(out,
		leaf("SOLAR_RAD_AREA", formatValue(s.SolarRadArea)),
		ndm.Leaf("SOLAR_RAD_COEFF", formatValue(s.SolarRadCoeff)),
		leaf("DRAG_AREA", formatValue(s.DragArea)),
		ndm.Leaf("DRAG_COEFF", formatValue(s.DragCoeff)),
	)
}

// DecodeXMLOPM reads an Orbit Parameter Message in the XML form.
func DecodeXMLOPM(data []byte) (*OPM, error) {
	message, err := ndm.DecodeXML(data, "opm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_OPM_VERS" {
		return nil, ErrNotAnOPM
	}
	// Clause 8.8.6 gives the OPM one segment: substructure 1 of the XML
	// standard, where the segment exists only for symmetry with the messages
	// that have several.
	if len(message.Segments) != 1 {
		return nil, ndm.ErrMalformedXML
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &OPM{Header: header}

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

func (m *OPM) readXMLMetadata(elements []ndm.Element) error {
	md := &m.Metadata
	md.Comments = ndm.CollectComments(elements)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if _, known := keywordBlock[e.Name]; !known || keywordBlock[e.Name] != blockMetadata {
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

func (m *OPM) readXMLData(elements []ndm.Element) error {
	for _, e := range elements {
		if len(e.Children) == 0 {
			if e.Name == ndm.KeywordComment {
				m.Data.StateVector.Comments = append(m.Data.StateVector.Comments, e.Value)
				continue
			}
			// Clause 8.8.12 onward put every data keyword inside a block, so a
			// bare keyword in the data section belongs nowhere.
			return ErrUnknownKeyword
		}

		switch e.Name {
		case xmlStateVector:
			if err := m.readXMLStateVector(e.Children); err != nil {
				return err
			}
		case xmlKeplerianElements:
			if err := m.readXMLKeplerian(e.Children); err != nil {
				return err
			}
		case xmlSpacecraftParameters:
			s, err := readSpacecraft(e.Children)
			if err != nil {
				return err
			}
			m.Data.Spacecraft = s
		case xmlCovarianceMatrix:
			c, err := readCovariance(e.Children)
			if err != nil {
				return err
			}
			m.Data.Covariance = c
		case xmlManeuverParameters:
			man, err := readXMLManeuver(e.Children)
			if err != nil {
				return err
			}
			m.Data.Maneuvers = append(m.Data.Maneuvers, man)
		case xmlUserDefinedParams:
			m.Data.UserDefined = readUserDefined(e.Children)
		default:
			return ErrUnknownKeyword
		}
	}
	return nil
}

func (m *OPM) readXMLStateVector(elements []ndm.Element) error {
	sv := &m.Data.StateVector
	sv.Comments = append(sv.Comments, ndm.CollectComments(elements)...)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if e.Name == "EPOCH" {
			t, err := parseEpochValue(e.Value)
			if err != nil {
				return err
			}
			sv.Epoch = t
			continue
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return err
		}
		switch e.Name {
		case "X":
			sv.X = v
		case "Y":
			sv.Y = v
		case "Z":
			sv.Z = v
		case "X_DOT":
			sv.XDot = v
		case "Y_DOT":
			sv.YDot = v
		case "Z_DOT":
			sv.ZDot = v
		default:
			return ErrUnknownKeyword
		}
	}
	return nil
}

func (m *OPM) readXMLKeplerian(elements []ndm.Element) error {
	k := &KeplerianElements{Comments: ndm.CollectComments(elements)}
	seenTrue, seenMean := false, false

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return err
		}
		switch e.Name {
		case "SEMI_MAJOR_AXIS":
			k.SemiMajorAxis = v
		case "ECCENTRICITY":
			k.Eccentricity = v
		case "INCLINATION":
			k.Inclination = v
		case "RA_OF_ASC_NODE":
			k.RAOfAscNode = v
		case "ARG_OF_PERICENTER":
			k.ArgOfPericenter = v
		case "TRUE_ANOMALY":
			seenTrue = true
			k.Anomaly, k.AnomalyIsMean = v, false
		case "MEAN_ANOMALY":
			seenMean = true
			k.Anomaly, k.AnomalyIsMean = v, true
		case "GM":
			k.GM = v
		default:
			return ErrUnknownKeyword
		}
	}
	if seenTrue && seenMean {
		return ErrBothAnomalies
	}
	m.Data.Keplerian = k
	return nil
}

func readXMLManeuver(elements []ndm.Element) (Maneuver, error) {
	man := Maneuver{Comments: ndm.CollectComments(elements)}

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		switch e.Name {
		case "MAN_EPOCH_IGNITION":
			t, err := parseEpochValue(e.Value)
			if err != nil {
				return man, err
			}
			man.EpochIgnition = t
			continue
		case "MAN_REF_FRAME":
			man.RefFrame = e.Value
			continue
		}

		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return man, err
		}
		switch e.Name {
		case "MAN_DURATION":
			man.Duration = v
		case "MAN_DELTA_MASS":
			man.DeltaMass = v
		case "MAN_DV_1":
			man.DV[0] = v
		case "MAN_DV_2":
			man.DV[1] = v
		case "MAN_DV_3":
			man.DV[2] = v
		default:
			return man, ErrUnknownKeyword
		}
	}
	return man, nil
}
