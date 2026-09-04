package adm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The ACM's XML form, CCSDS 504.0-B-2 clause 7.7.
//
// It keeps the data rows as rows. Clause 7.7.13.3 gives <attLine> and
// <covLine> the type xsd:string, so the schema does not look inside them and
// the recipient still splits the line and reads its columns by ATT_TYPE,
// RATE_TYPE or COV_TYPE. That is the opposite of the AEM, whose clause 7.6
// names every component of an attitude record.
//
// Blocks replace the delimiters. Table 7-7 names one element per section:
// <att>, <phys>, <cov>, <man>, <ad> and <user>. The sensor sub-blocks nested
// inside the attitude determination section become <sensorData> elements,
// which clause 7.7.14 shows rather than states — it is the one part of the ACM
// mapping the standard leaves to an example.
//
// Units move from a bracketed suffix into an attribute (clauses 7.7.10 to
// 7.7.12), as in every other message here.

// The section, data line and sensor elements of table 7-7 and clause 7.7.14.
const (
	xmlAtt        = "att"
	xmlPhys       = "phys"
	xmlCov        = "cov"
	xmlMan        = "man"
	xmlAD         = "ad"
	xmlUser       = "user"
	xmlSensorData = "sensorData"

	xmlAttLine = "attLine"
	xmlCovLine = "covLine"
)

// xmlACMBlocks maps each section element to the delimiter prefix its keywords
// come from, and to the element its data rows use. A section with no rows has
// none.
var xmlACMBlocks = []struct {
	element  string
	prefix   string
	line     string
	repeated bool
}{
	{xmlAtt, acmAtt, xmlAttLine, true},
	{xmlPhys, acmPhys, "", false},
	{xmlCov, acmCov, xmlCovLine, true},
	{xmlMan, acmMan, "", true},
	{xmlAD, acmAD, "", false},
	{xmlUser, acmUser, "", false},
}

// EncodeXML writes the message in the XML form (clause 7.7).
func (m *ACM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "acm",
		ID:      "CCSDS_ACM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaADM,
		Header:  m.Header.xmlHeader(),
	}

	// Clause 7.7.6: the body is a single segment, which follows from the ACM
	// having a single metadata section.
	segment := ndm.Segment{Metadata: xmlACMSection(m.Metadata, "")}
	for _, section := range m.Attitudes {
		segment.Data = append(segment.Data, ndm.Block(xmlAtt, xmlACMSection(section, xmlAttLine)...))
	}
	if m.Physical != nil {
		segment.Data = append(segment.Data, ndm.Block(xmlPhys, xmlACMSection(*m.Physical, "")...))
	}
	for _, section := range m.Covariances {
		segment.Data = append(segment.Data, ndm.Block(xmlCov, xmlACMSection(section, xmlCovLine)...))
	}
	for _, section := range m.Maneuvers {
		segment.Data = append(segment.Data, ndm.Block(xmlMan, xmlACMSection(section, "")...))
	}
	if m.AttitudeDetermination != nil {
		segment.Data = append(segment.Data, ndm.Block(xmlAD, xmlACMSection(*m.AttitudeDetermination, "")...))
	}
	if len(m.UserDefined) > 0 {
		children := ndm.Comments(m.UserComments)
		for _, u := range m.UserDefined {
			children = append(children, ndm.UserDefined(strings.ToUpper(u.Name), u.Value))
		}
		segment.Data = append(segment.Data, ndm.Block(xmlUser, children...))
	}

	message.Segments = []ndm.Segment{segment}
	return message.EncodeXML()
}

// xmlACMSection renders one section: its comments, its keywords with any unit
// suffix moved into the attribute, its sensor sub-blocks, then its data rows.
func xmlACMSection(section ACMSection, line string) []ndm.Element {
	out := ndm.Comments(section.Comments)
	for _, f := range section.Fields {
		out = append(out, ndm.SplitLeaf(f.Keyword, f.Value))
	}
	for _, sensor := range section.Sensors {
		children := ndm.Comments(sensor.Comments)
		for _, f := range sensor.Fields {
			children = append(children, ndm.SplitLeaf(f.Keyword, f.Value))
		}
		out = append(out, ndm.Block(xmlSensorData, children...))
	}
	if line == "" {
		return out
	}
	for _, row := range section.Rows {
		out = append(out, ndm.Leaf(line, strings.Join(row.Fields, " ")))
	}
	return out
}

// DecodeXMLACM reads an Attitude Comprehensive Message in the XML form.
func DecodeXMLACM(data []byte) (*ACM, error) {
	message, err := ndm.DecodeXML(data, "acm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_ACM_VERS" {
		return nil, ErrNotAnACM
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &ACM{Header: header}

	if len(message.Segments) != 1 {
		return nil, ndm.ErrMalformedXML
	}
	segment := message.Segments[0]

	if m.Metadata, err = readXMLACMSection(segment.Metadata, "", false); err != nil {
		return nil, err
	}
	if err := readXMLACMData(m, segment.Data); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readXMLACMData walks the data blocks in document order, so clause 5.3.1.2's
// section ordering is checked in the XML form as it is in the key-value one.
func readXMLACMData(m *ACM, data []ndm.Element) error {
	previous := -1

	for _, block := range data {
		at := xmlACMBlockRank(block.Name)
		if at < 0 {
			return ndm.ErrMalformedXML
		}
		if at < previous {
			return ErrSectionsOutOfOrder
		}
		if at == previous && !xmlACMBlocks[at].repeated {
			return ErrDuplicateSection
		}
		previous = at

		if block.Name == xmlUser {
			params, err := readXMLUserDefined(block.Children)
			if err != nil {
				return err
			}
			m.UserComments = append(m.UserComments, ndm.CollectComments(block.Children)...)
			m.UserDefined = append(m.UserDefined, params...)
			continue
		}

		section, err := readXMLACMSection(block.Children, xmlACMBlocks[at].line, block.Name == xmlAD)
		if err != nil {
			return err
		}
		switch block.Name {
		case xmlAtt:
			m.Attitudes = append(m.Attitudes, section)
		case xmlCov:
			m.Covariances = append(m.Covariances, section)
		case xmlMan:
			m.Maneuvers = append(m.Maneuvers, section)
		case xmlPhys:
			m.Physical = &section
		case xmlAD:
			m.AttitudeDetermination = &section
		}
	}
	return nil
}

// xmlACMBlockRank returns a block's place in table 5-1, or -1.
func xmlACMBlockRank(name string) int {
	for i, block := range xmlACMBlocks {
		if block.element == name {
			return i
		}
	}
	return -1
}

// readXMLACMSection fills a section from its elements, putting the units
// attribute back on the value so both forms hold the same text.
//
// sensors says whether this section may hold <sensorData> children. Only the
// attitude determination section may: clause 5.3.9.6 puts the sensor blocks
// inside AD_START and AD_STOP and nowhere else. Accepting one anywhere would
// produce a message whose key-value form this package refuses to read back —
// which is how FuzzDecodeXMLACM found it.
func readXMLACMSection(elements []ndm.Element, line string, sensors bool) (ACMSection, error) {
	section := ACMSection{Comments: ndm.CollectComments(elements)}

	for _, e := range elements {
		switch {
		case e.Name == ndm.KeywordComment:
			continue
		case line != "" && e.Name == line:
			if fields := strings.Fields(e.Value); len(fields) > 0 {
				section.Rows = append(section.Rows, DataRow{Fields: fields})
			}
		case e.Name == xmlSensorData:
			if !sensors {
				return section, ErrUnexpectedDelimiter
			}
			sensor := ACMSensor{Comments: ndm.CollectComments(e.Children)}
			for _, child := range e.Children {
				if child.Name == ndm.KeywordComment {
					continue
				}
				sensor.Fields = append(sensor.Fields,
					Field{Keyword: child.Name, Value: child.JoinValue()})
			}
			section.Sensors = append(section.Sensors, sensor)
		default:
			section.Fields = append(section.Fields, Field{Keyword: e.Name, Value: e.JoinValue()})
		}
	}
	return section, nil
}

// readXMLUserDefined reads the user-defined block, taking each parameter's
// name from its attribute rather than from the element name.
//
// A parameter with no name is refused, as in pkg/odm: the key-value form of a
// nameless one is the bare 'USER_DEFINED_ = value', which this package's own
// reader will not take back.
func readXMLUserDefined(elements []ndm.Element) ([]UserDefined, error) {
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
