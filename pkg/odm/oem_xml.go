package odm

import "github.com/ravisuhag/astro/internal/ndm"

// The OEM's XML form is where the two representations diverge most.
//
// In the key-value form an ephemeris record is a positional line — epoch, then
// six or nine numbers in the order clause 5.2.4.1 fixes — and a covariance
// matrix is a run of values between COVARIANCE_START and COVARIANCE_STOP.
//
// In XML, clause 8.10.14 requires every component of an ephemeris line to be
// named, so each record becomes its own <stateVector> block. Clause 8.10.19
// gives the covariance the OPM's named CX_X family instead of positional rows.
// The delimiters disappear entirely: the blocks are the delimiters.
//
// So an OEM converted between the forms carries the same numbers in a
// genuinely different shape. That is worth knowing before comparing two files
// by eye and concluding they disagree.

// EncodeXML writes the message in the XML form (clause 8.10).
func (m *OEM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "oem",
		ID:      "CCSDS_OEM_VERS",
		Version: m.Header.Version,
		Header:  m.Header.xmlHeader(),
	}
	for i := range m.Blocks {
		block := &m.Blocks[i]
		metadata, err := xmlOEMMetadata(&block.Metadata)
		if err != nil {
			return nil, err
		}
		data, err := xmlOEMData(block)
		if err != nil {
			return nil, err
		}
		message.Segments = append(message.Segments, ndm.Segment{
			Metadata: metadata,
			Data:     data,
		})
	}
	return message.EncodeXML()
}

func xmlOEMMetadata(md *OEMMetadata) ([]ndm.Element, error) {
	out := ndm.Comments(md.Comments)
	out = append(out,
		ndm.Leaf("OBJECT_NAME", md.ObjectName),
		ndm.Leaf("OBJECT_ID", md.ObjectID),
		ndm.Leaf("CENTER_NAME", md.CenterName),
		ndm.Leaf("REF_FRAME", md.RefFrame),
	)
	if md.RefFrameEpoch != nil {
		epoch, err := ndm.FormatEpoch(*md.RefFrameEpoch, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, ndm.Leaf("REF_FRAME_EPOCH", epoch))
	}
	out = append(out, ndm.Leaf("TIME_SYSTEM", md.TimeSystem))

	start, err := ndm.FormatEpoch(md.StartTime, epochPrecision(md.StartTime))
	if err != nil {
		return nil, err
	}
	out = append(out, ndm.Leaf("START_TIME", start))

	if md.UseableStartTime != nil {
		v, err := ndm.FormatEpoch(*md.UseableStartTime, epochPrecision(*md.UseableStartTime))
		if err != nil {
			return nil, err
		}
		out = append(out, ndm.Leaf("USEABLE_START_TIME", v))
	}
	if md.UseableStopTime != nil {
		v, err := ndm.FormatEpoch(*md.UseableStopTime, epochPrecision(*md.UseableStopTime))
		if err != nil {
			return nil, err
		}
		out = append(out, ndm.Leaf("USEABLE_STOP_TIME", v))
	}

	stop, err := ndm.FormatEpoch(md.StopTime, epochPrecision(md.StopTime))
	if err != nil {
		return nil, err
	}
	out = append(out, ndm.Leaf("STOP_TIME", stop))

	if md.Interpolation != "" {
		out = append(out,
			ndm.Leaf("INTERPOLATION", md.Interpolation),
			ndm.Leaf("INTERPOLATION_DEGREE", ndm.FormatInt(md.InterpolationDegree)),
		)
	}
	return out, nil
}

func xmlOEMData(block *EphemerisBlock) ([]ndm.Element, error) {
	out := ndm.Comments(block.Comments)

	// One block per record, with every component named.
	for _, line := range block.Lines {
		epoch, err := ndm.FormatEpoch(line.Epoch, epochPrecision(line.Epoch))
		if err != nil {
			return nil, err
		}
		children := []ndm.Element{
			ndm.Leaf("EPOCH", epoch),
			leaf("X", formatValue(line.X)), leaf("Y", formatValue(line.Y)), leaf("Z", formatValue(line.Z)),
			leaf("X_DOT", formatValue(line.XDot)),
			leaf("Y_DOT", formatValue(line.YDot)),
			leaf("Z_DOT", formatValue(line.ZDot)),
		}
		if line.HasAcceleration {
			children = append(children,
				leaf("X_DDOT", formatValue(line.XDDot)),
				leaf("Y_DDOT", formatValue(line.YDDot)),
				leaf("Z_DDOT", formatValue(line.ZDDot)),
			)
		}
		out = append(out, ndm.Block(xmlStateVector, children...))
	}

	// And one per covariance matrix, with the OPM's named keywords.
	for i, c := range block.Covariances {
		epoch, err := ndm.FormatEpoch(c.Epoch, epochPrecision(c.Epoch))
		if err != nil {
			return nil, err
		}
		children := []ndm.Element{ndm.Leaf("EPOCH", epoch)}
		if i == 0 {
			children = append(ndm.Comments(block.CovarianceComments), children...)
		}
		if c.RefFrame != "" {
			children = append(children, ndm.Leaf("COV_REF_FRAME", c.RefFrame))
		}
		for _, e := range covarianceElements {
			children = append(children,
				ndm.LeafWithUnits(e.keyword, formatValue(c.Matrix[e.row][e.col]), e.units))
		}
		out = append(out, ndm.Block(xmlCovarianceMatrix, children...))
	}
	return out, nil
}

// DecodeXMLOEM reads an Orbit Ephemeris Message in the XML form.
func DecodeXMLOEM(data []byte) (*OEM, error) {
	message, err := ndm.DecodeXML(data, "oem")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_OEM_VERS" {
		return nil, ErrNotAnOEM
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &OEM{Header: header}

	for _, segment := range message.Segments {
		var block EphemerisBlock
		if err := readXMLOEMMetadata(&block.Metadata, segment.Metadata); err != nil {
			return nil, err
		}
		if err := readXMLOEMData(&block, segment.Data); err != nil {
			return nil, err
		}
		m.Blocks = append(m.Blocks, block)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func readXMLOEMMetadata(md *OEMMetadata, elements []ndm.Element) error {
	md.Comments = ndm.CollectComments(elements)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if !oemMetadataKeywords[e.Name] {
			return ErrUnknownKeyword
		}
		if err := assignOEMMetadata(md, e.Name, e.Value); err != nil {
			return err
		}
	}
	return nil
}

func readXMLOEMData(block *EphemerisBlock, elements []ndm.Element) error {
	for _, e := range elements {
		if len(e.Children) == 0 {
			if e.Name == ndm.KeywordComment {
				block.Comments = append(block.Comments, e.Value)
				continue
			}
			return ErrUnknownKeyword
		}

		switch e.Name {
		case xmlStateVector:
			line, err := readXMLEphemerisLine(e.Children)
			if err != nil {
				return err
			}
			block.Lines = append(block.Lines, line)
		case xmlCovarianceMatrix:
			c, err := readXMLOEMCovariance(e.Children)
			if err != nil {
				return err
			}
			block.CovarianceComments = append(block.CovarianceComments, ndm.CollectComments(e.Children)...)
			block.Covariances = append(block.Covariances, c)
		default:
			return ErrUnknownKeyword
		}
	}
	return nil
}

func readXMLEphemerisLine(elements []ndm.Element) (EphemerisLine, error) {
	var line EphemerisLine

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if e.Name == "EPOCH" {
			t, err := parseEpochValue(e.Value)
			if err != nil {
				return line, err
			}
			line.Epoch = t
			continue
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return line, err
		}
		switch e.Name {
		case "X":
			line.X = v
		case "Y":
			line.Y = v
		case "Z":
			line.Z = v
		case "X_DOT":
			line.XDot = v
		case "Y_DOT":
			line.YDot = v
		case "Z_DOT":
			line.ZDot = v
		case "X_DDOT":
			line.XDDot, line.HasAcceleration = v, true
		case "Y_DDOT":
			line.YDDot, line.HasAcceleration = v, true
		case "Z_DDOT":
			line.ZDDot, line.HasAcceleration = v, true
		default:
			return line, ErrUnknownKeyword
		}
	}
	return line, nil
}

func readXMLOEMCovariance(elements []ndm.Element) (OEMCovariance, error) {
	var c OEMCovariance

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		switch e.Name {
		case "EPOCH":
			t, err := parseEpochValue(e.Value)
			if err != nil {
				return c, err
			}
			c.Epoch = t
			continue
		case "COV_REF_FRAME":
			c.RefFrame = e.Value
			continue
		}

		at, known := covarianceIndex[e.Name]
		if !known {
			return c, ErrUnknownKeyword
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return c, err
		}
		c.Matrix[at[0]][at[1]] = v
		c.Matrix[at[1]][at[0]] = v
	}
	return c, nil
}
