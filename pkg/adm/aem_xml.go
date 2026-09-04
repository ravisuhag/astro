package adm

import (
	"time"

	"github.com/ravisuhag/astro/internal/ndm"
)

// EncodeXML writes the message in the XML form (clause 7.6).
func (m *AEM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "aem",
		ID:      "CCSDS_AEM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaADM,
		Header:  m.Header.xmlHeader(),
	}
	for i := range m.Blocks {
		block := &m.Blocks[i]
		metadata, err := xmlAEMMetadata(&block.Metadata)
		if err != nil {
			return nil, err
		}
		data, err := xmlAEMData(block)
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

func xmlAEMMetadata(md *AEMMetadata) ([]ndm.Element, error) {
	out := ndm.Comments(md.Comments)
	out = append(out,
		ndm.Leaf("OBJECT_NAME", md.ObjectName),
		ndm.Leaf("OBJECT_ID", md.ObjectID),
	)
	if md.CenterName != "" {
		out = append(out, ndm.Leaf("CENTER_NAME", md.CenterName))
	}
	out = append(out, framesElements(md.frames)...)
	out = append(out, ndm.Leaf("TIME_SYSTEM", md.TimeSystem))

	start, err := ndm.FormatEpoch(md.StartTime, epochPrecision(md.StartTime))
	if err != nil {
		return nil, err
	}
	out = append(out, ndm.Leaf("START_TIME", start))

	for _, optional := range []struct {
		keyword string
		value   *time.Time
	}{
		{"USEABLE_START_TIME", md.UseableStartTime},
		{"USEABLE_STOP_TIME", md.UseableStopTime},
	} {
		if optional.value == nil {
			continue
		}
		v, err := ndm.FormatEpoch(*optional.value, epochPrecision(*optional.value))
		if err != nil {
			return nil, err
		}
		out = append(out, ndm.Leaf(optional.keyword, v))
	}

	stop, err := ndm.FormatEpoch(md.StopTime, epochPrecision(md.StopTime))
	if err != nil {
		return nil, err
	}
	out = append(out, ndm.Leaf("STOP_TIME", stop))

	out = append(out, ndm.Leaf("ATTITUDE_TYPE", string(md.Type)))
	if md.Type.IsEuler() {
		out = append(out, ndm.Leaf("EULER_ROT_SEQ", md.RotSeq))
	}
	if md.AngVelFrame != "" {
		out = append(out, ndm.Leaf("ANGVEL_FRAME", md.AngVelFrame))
	}
	if md.InterpolationMethod != "" {
		out = append(out,
			ndm.Leaf("INTERPOLATION_METHOD", md.InterpolationMethod),
			ndm.Leaf("INTERPOLATION_DEGREE", ndm.FormatInt(md.InterpolationDegree)),
		)
	}
	return out, nil
}

// xmlAEMData renders the attitude records.
//
// Each becomes an <attitudeState> wrapping the element table 7-5 assigns the
// segment's attitude type. The positional values of the key-value form become
// named elements, and for the quaternion types they nest one level further:
// clause 7.5.12 puts the four components in their own <quaternion> element and
// the derivatives or angular velocity in a sibling.
func xmlAEMData(block *AttitudeBlock) ([]ndm.Element, error) {
	inner, ok := attitudeInnerElement[block.Metadata.Type]
	if !ok {
		return nil, ErrUnknownAttitudeType
	}
	names := attitudeValueNames[block.Metadata.Type]

	out := ndm.Comments(block.Comments)
	for _, line := range block.Lines {
		if len(line.Values) != len(names) {
			return nil, ErrAttitudeLineFields
		}
		epoch, err := ndm.FormatEpoch(line.Epoch, epochPrecision(line.Epoch))
		if err != nil {
			return nil, err
		}

		children := []ndm.Element{ndm.Leaf("EPOCH", epoch)}
		children = append(children, groupAttitudeValues(block.Metadata.Type, names, line.Values)...)
		out = append(out, ndm.Block(xmlAttitudeState, ndm.Block(inner, children...)))
	}
	return out, nil
}

// groupAttitudeValues turns a record's values into elements, nesting the
// quaternion types as clause 7.5.12 requires and leaving the rest flat.
func groupAttitudeValues(t AttitudeType, names []string, values []float64) []ndm.Element {
	leaves := make([]ndm.Element, 0, len(names))
	for i, name := range names {
		leaves = append(leaves, leaf(name, formatValue(values[i])))
	}

	switch t {
	case Quaternion4:
		return []ndm.Element{ndm.Block(xmlQuaternion, leaves...)}
	case QuaternionDerivative:
		return []ndm.Element{
			ndm.Block(xmlQuaternion, leaves[:4]...),
			ndm.Block(xmlQuaternionDot, leaves[4:]...),
		}
	case QuaternionAngVel:
		return []ndm.Element{
			ndm.Block(xmlQuaternion, leaves[:4]...),
			ndm.Block(xmlAngVel, leaves[4:]...),
		}
	}
	return leaves
}

// DecodeXMLAEM reads an Attitude Ephemeris Message in the XML form.
func DecodeXMLAEM(data []byte) (*AEM, error) {
	message, err := ndm.DecodeXML(data, "aem")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_AEM_VERS" {
		return nil, ErrNotAnAEM
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &AEM{Header: header}

	for _, segment := range message.Segments {
		var block AttitudeBlock
		if err := readXMLAEMMetadata(&block.Metadata, segment.Metadata); err != nil {
			return nil, err
		}
		if err := readXMLAEMData(&block, segment.Data); err != nil {
			return nil, err
		}
		m.Blocks = append(m.Blocks, block)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func readXMLAEMMetadata(md *AEMMetadata, elements []ndm.Element) error {
	md.Comments = ndm.CollectComments(elements)

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if !aemMetadataKeywords[e.Name] {
			return ErrUnknownKeyword
		}
		if err := assignAEMMetadata(md, e.Name, e.Value); err != nil {
			return err
		}
	}
	return nil
}

// readXMLAEMData reads the attitude records, unwrapping the type-specific
// element and flattening the quaternion nesting back into a value list.
func readXMLAEMData(block *AttitudeBlock, elements []ndm.Element) error {
	inner, ok := attitudeInnerElement[block.Metadata.Type]
	if !ok {
		return ErrUnknownAttitudeType
	}
	names := attitudeValueNames[block.Metadata.Type]

	for _, e := range elements {
		if len(e.Children) == 0 {
			if e.Name == ndm.KeywordComment {
				block.Comments = append(block.Comments, e.Value)
				continue
			}
			return ErrUnknownKeyword
		}
		if e.Name != xmlAttitudeState {
			return ErrUnknownKeyword
		}

		state, ok := ndm.FindBlock(e.Children, inner)
		if !ok {
			// The inner element must be the one the segment's attitude type
			// names. Anything else means the type and the data disagree, which
			// is the XML form of a line of the wrong width.
			return ErrAttitudeLineFields
		}

		line, err := readXMLAttitudeLine(state, names)
		if err != nil {
			return err
		}
		block.Lines = append(block.Lines, line)
	}
	return nil
}

func readXMLAttitudeLine(elements []ndm.Element, names []string) (AttitudeLine, error) {
	var line AttitudeLine

	epoch, ok := ndm.Find(elements, "EPOCH")
	if !ok {
		return line, ErrMissingKeyword
	}
	t, err := parseEpoch(epoch)
	if err != nil {
		return line, err
	}
	line.Epoch = t

	// The values may be one level down, inside a quaternion or angVel element,
	// so both levels are collected before they are put back in table 4-4's
	// order.
	values := make(map[string]float64)
	if err := collectValues(elements, values); err != nil {
		return line, err
	}

	line.Values = make([]float64, len(names))
	for i, name := range names {
		v, ok := values[name]
		if !ok {
			return line, ErrAttitudeLineFields
		}
		line.Values[i] = v
	}
	return line, nil
}

// collectValues gathers named numbers from an element tree, one level of
// nesting included.
func collectValues(elements []ndm.Element, into map[string]float64) error {
	for _, e := range elements {
		if len(e.Children) > 0 {
			if err := collectValues(e.Children, into); err != nil {
				return err
			}
			continue
		}
		if e.Name == ndm.KeywordComment || e.Name == "EPOCH" {
			continue
		}
		v, err := ndm.ParseFloat(e.Value)
		if err != nil {
			return err
		}
		into[e.Name] = v
	}
	return nil
}
