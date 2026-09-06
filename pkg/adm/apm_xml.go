package adm

import "github.com/ravisuhag/astro/internal/ndm"

// EncodeXML writes the message in the XML form (clause 7.5).
func (m *APM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	md := m.Metadata
	metadata := ndm.Comments(md.Comments)
	metadata = append(metadata,
		ndm.Leaf("OBJECT_NAME", md.ObjectName),
		ndm.Leaf("OBJECT_ID", md.ObjectID),
	)
	if md.CenterName != "" {
		metadata = append(metadata, ndm.Leaf("CENTER_NAME", md.CenterName))
	}
	metadata = append(metadata, ndm.Leaf("TIME_SYSTEM", md.TimeSystem))

	data, err := m.xmlData()
	if err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "apm",
		ID:      "CCSDS_APM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaADM,
		Header:  m.Header.xmlHeader(),
		Segments: []ndm.Segment{{
			Metadata: metadata,
			Data:     data,
		}},
	}
	return message.EncodeXML()
}

func (m *APM) xmlData() ([]ndm.Element, error) {
	epoch, err := ndm.FormatEpoch(m.Epoch, ndm.EpochPrecision(m.Epoch))
	if err != nil {
		return nil, err
	}

	out := ndm.Comments(m.Comments)
	out = append(out, ndm.Leaf("EPOCH", epoch))

	if b := m.Quaternion; b != nil {
		// Clause 7.5.11: the components sit in their own element inside the
		// state, with the derivatives as a sibling rather than more
		// components.
		children := ndm.Comments(b.Comments)
		children = append(children, framesElements(b.frames)...)
		children = append(children, ndm.Block(xmlQuaternion,
			ndm.Leaf("Q1", ndm.FormatValue(b.Q1)),
			ndm.Leaf("Q2", ndm.FormatValue(b.Q2)),
			ndm.Leaf("Q3", ndm.FormatValue(b.Q3)),
			ndm.Leaf("QC", ndm.FormatValue(b.QC)),
		))
		if b.HasDerivative {
			children = append(children, ndm.Block(xmlQuaternionDot,
				leaf("Q1_DOT", ndm.FormatValue(b.Derivative.Q1)),
				leaf("Q2_DOT", ndm.FormatValue(b.Derivative.Q2)),
				leaf("Q3_DOT", ndm.FormatValue(b.Derivative.Q3)),
				leaf("QC_DOT", ndm.FormatValue(b.Derivative.QC)),
			))
		}
		out = append(out, ndm.Block(xmlQuaternionState, children...))
	}

	if b := m.Euler; b != nil {
		children := ndm.Comments(b.Comments)
		children = append(children, framesElements(b.frames)...)
		children = append(children,
			ndm.Leaf("EULER_ROT_SEQ", b.RotSeq),
			leaf("ANGLE_1", ndm.FormatValue(b.Angle1)),
			leaf("ANGLE_2", ndm.FormatValue(b.Angle2)),
			leaf("ANGLE_3", ndm.FormatValue(b.Angle3)),
		)
		if b.HasRates {
			children = append(children,
				leaf("ANGLE_1_DOT", ndm.FormatValue(b.Rate1)),
				leaf("ANGLE_2_DOT", ndm.FormatValue(b.Rate2)),
				leaf("ANGLE_3_DOT", ndm.FormatValue(b.Rate3)),
			)
		}
		out = append(out, ndm.Block(xmlEulerAngleState, children...))
	}

	if b := m.AngVel; b != nil {
		children := ndm.Comments(b.Comments)
		children = append(children, framesElements(b.frames)...)
		children = append(children,
			ndm.Leaf("ANGVEL_FRAME", b.Frame),
			leaf("ANGVEL_X", ndm.FormatValue(b.X)),
			leaf("ANGVEL_Y", ndm.FormatValue(b.Y)),
			leaf("ANGVEL_Z", ndm.FormatValue(b.Z)),
		)
		out = append(out, ndm.Block(xmlAngularVelocity, children...))
	}

	if b := m.Spin; b != nil {
		children := ndm.Comments(b.Comments)
		children = append(children, framesElements(b.frames)...)
		children = append(children,
			leaf("SPIN_ALPHA", ndm.FormatValue(b.Alpha)),
			leaf("SPIN_DELTA", ndm.FormatValue(b.Delta)),
			leaf("SPIN_ANGLE", ndm.FormatValue(b.Angle)),
			leaf("SPIN_ANGLE_VEL", ndm.FormatValue(b.AngleVel)),
		)
		if b.HasNutation {
			children = append(children,
				leaf("NUTATION", ndm.FormatValue(b.Nutation)),
				leaf("NUTATION_PER", ndm.FormatValue(b.NutationPeriod)),
				leaf("NUTATION_PHASE", ndm.FormatValue(b.NutationPhase)),
			)
		}
		if b.HasMomentum {
			children = append(children,
				leaf("MOMENTUM_ALPHA", ndm.FormatValue(b.MomentumAlpha)),
				leaf("MOMENTUM_DELTA", ndm.FormatValue(b.MomentumDelta)),
				leaf("NUTATION_VEL", ndm.FormatValue(b.NutationVel)),
			)
		}
		out = append(out, ndm.Block(xmlSpin, children...))
	}

	if b := m.Inertia; b != nil {
		children := ndm.Comments(b.Comments)
		children = append(children, ndm.Leaf("INERTIA_REF_FRAME", b.Frame))
		for _, e := range []struct {
			keyword string
			value   float64
		}{
			{"IXX", b.IXX}, {"IYY", b.IYY}, {"IZZ", b.IZZ},
			{"IXY", b.IXY}, {"IXZ", b.IXZ}, {"IYZ", b.IYZ},
		} {
			children = append(children, leaf(e.keyword, ndm.FormatValue(e.value)))
		}
		out = append(out, ndm.Block(xmlInertia, children...))
	}

	for _, man := range m.Maneuvers {
		start, err := ndm.FormatEpoch(man.EpochStart, ndm.EpochPrecision(man.EpochStart))
		if err != nil {
			return nil, err
		}
		children := ndm.Comments(man.Comments)
		children = append(children,
			ndm.Leaf("MAN_EPOCH_START", start),
			leaf("MAN_DURATION", ndm.FormatValue(man.Duration)),
			ndm.Leaf("MAN_REF_FRAME", man.RefFrame),
			leaf("MAN_TOR_X", ndm.FormatValue(man.TorqueX)),
			leaf("MAN_TOR_Y", ndm.FormatValue(man.TorqueY)),
			leaf("MAN_TOR_Z", ndm.FormatValue(man.TorqueZ)),
		)
		out = append(out, ndm.Block(xmlManeuver, children...))
	}
	return out, nil
}

// DecodeXMLAPM reads an Attitude Parameter Message in the XML form.
func DecodeXMLAPM(data []byte) (*APM, error) {
	message, err := ndm.DecodeXML(data, "apm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_APM_VERS" {
		return nil, ErrNotAnAPM
	}
	if len(message.Segments) != 1 {
		return nil, ndm.ErrMalformedXML
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &APM{Header: header}

	segment := message.Segments[0]
	m.Metadata.Comments = ndm.CollectComments(segment.Metadata)
	for _, e := range segment.Metadata {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if !apmMetadataKeywords[e.Name] {
			return nil, ErrUnknownKeyword
		}
		switch e.Name {
		case "OBJECT_NAME":
			m.Metadata.ObjectName = ndm.ParseText(e.Value)
		case "OBJECT_ID":
			m.Metadata.ObjectID = ndm.ParseText(e.Value)
		case "CENTER_NAME":
			m.Metadata.CenterName = ndm.ParseText(e.Value)
		case "TIME_SYSTEM":
			m.Metadata.TimeSystem = e.Value
		}
	}

	if err := m.readXMLData(segment.Data); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *APM) readXMLData(elements []ndm.Element) error {
	for _, e := range elements {
		if len(e.Children) == 0 {
			switch e.Name {
			case ndm.KeywordComment:
				m.Comments = append(m.Comments, e.Value)
			case "EPOCH":
				t, err := ndm.ParseEpoch(e.Value)
				if err != nil {
					return err
				}
				m.Epoch = t
			default:
				return ErrUnknownKeyword
			}
			continue
		}

		var err error
		switch e.Name {
		case xmlQuaternionState:
			err = m.readXMLQuaternion(e.Children)
		case xmlEulerAngleState:
			err = m.readXMLEuler(e.Children)
		case xmlAngularVelocity:
			err = m.readXMLAngVel(e.Children)
		case xmlSpin:
			err = m.readXMLSpin(e.Children)
		case xmlInertia:
			err = m.readXMLInertia(e.Children)
		case xmlManeuver:
			err = m.readXMLManeuver(e.Children)
		default:
			return ErrUnknownKeyword
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *APM) readXMLQuaternion(elements []ndm.Element) error {
	block := &QuaternionBlock{Comments: ndm.CollectComments(elements)}
	readFrames(&block.frames, elements)

	components, ok := ndm.FindBlock(elements, xmlQuaternion)
	if !ok {
		return ErrMissingKeyword
	}
	values, err := numbers(components)
	if err != nil {
		return err
	}
	block.Quaternion = Quaternion{
		Q1: values["Q1"], Q2: values["Q2"], Q3: values["Q3"], QC: values["QC"],
	}

	if rates, ok := ndm.FindBlock(elements, xmlQuaternionDot); ok {
		v, err := numbers(rates)
		if err != nil {
			return err
		}
		block.HasDerivative = true
		block.Derivative = Quaternion{
			Q1: v["Q1_DOT"], Q2: v["Q2_DOT"], Q3: v["Q3_DOT"], QC: v["QC_DOT"],
		}
	}
	m.Quaternion = block
	return nil
}

func (m *APM) readXMLEuler(elements []ndm.Element) error {
	block := &EulerBlock{Comments: ndm.CollectComments(elements)}
	readFrames(&block.frames, elements)
	block.RotSeq, _ = ndm.Find(elements, "EULER_ROT_SEQ")
	block.RotSeq = upperTrim(block.RotSeq)

	values, err := numbers(elements)
	if err != nil {
		return err
	}
	block.Angle1, block.Angle2, block.Angle3 = values["ANGLE_1"], values["ANGLE_2"], values["ANGLE_3"]
	if _, ok := values["ANGLE_1_DOT"]; ok {
		block.HasRates = true
		block.Rate1, block.Rate2, block.Rate3 =
			values["ANGLE_1_DOT"], values["ANGLE_2_DOT"], values["ANGLE_3_DOT"]
	}
	m.Euler = block
	return nil
}

func (m *APM) readXMLAngVel(elements []ndm.Element) error {
	block := &AngVelBlock{Comments: ndm.CollectComments(elements)}
	readFrames(&block.frames, elements)
	block.Frame, _ = ndm.Find(elements, "ANGVEL_FRAME")

	values, err := numbers(elements)
	if err != nil {
		return err
	}
	block.X, block.Y, block.Z = values["ANGVEL_X"], values["ANGVEL_Y"], values["ANGVEL_Z"]
	m.AngVel = block
	return nil
}

func (m *APM) readXMLSpin(elements []ndm.Element) error {
	block := &SpinBlock{Comments: ndm.CollectComments(elements)}
	readFrames(&block.frames, elements)

	values, err := numbers(elements)
	if err != nil {
		return err
	}
	block.Alpha, block.Delta = values["SPIN_ALPHA"], values["SPIN_DELTA"]
	block.Angle, block.AngleVel = values["SPIN_ANGLE"], values["SPIN_ANGLE_VEL"]

	if _, ok := values["NUTATION"]; ok {
		block.HasNutation = true
		block.Nutation, block.NutationPeriod, block.NutationPhase =
			values["NUTATION"], values["NUTATION_PER"], values["NUTATION_PHASE"]
	}
	if _, ok := values["MOMENTUM_ALPHA"]; ok {
		block.HasMomentum = true
		block.MomentumAlpha, block.MomentumDelta, block.NutationVel =
			values["MOMENTUM_ALPHA"], values["MOMENTUM_DELTA"], values["NUTATION_VEL"]
	}
	m.Spin = block
	return nil
}

func (m *APM) readXMLInertia(elements []ndm.Element) error {
	block := &InertiaBlock{Comments: ndm.CollectComments(elements)}
	block.Frame, _ = ndm.Find(elements, "INERTIA_REF_FRAME")

	values, err := numbers(elements)
	if err != nil {
		return err
	}
	block.IXX, block.IYY, block.IZZ = values["IXX"], values["IYY"], values["IZZ"]
	block.IXY, block.IXZ, block.IYZ = values["IXY"], values["IXZ"], values["IYZ"]
	m.Inertia = block
	return nil
}

func (m *APM) readXMLManeuver(elements []ndm.Element) error {
	man := Maneuver{Comments: ndm.CollectComments(elements)}

	start, ok := ndm.Find(elements, "MAN_EPOCH_START")
	if !ok {
		return ErrMissingKeyword
	}
	t, err := ndm.ParseEpoch(start)
	if err != nil {
		return err
	}
	man.EpochStart = t
	man.RefFrame, _ = ndm.Find(elements, "MAN_REF_FRAME")

	values, err := numbers(elements)
	if err != nil {
		return err
	}
	man.Duration = values["MAN_DURATION"]
	man.TorqueX, man.TorqueY, man.TorqueZ =
		values["MAN_TOR_X"], values["MAN_TOR_Y"], values["MAN_TOR_Z"]

	m.Maneuvers = append(m.Maneuvers, man)
	return nil
}
