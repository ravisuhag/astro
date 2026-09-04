package odm

import (
	"strings"

	"github.com/ravisuhag/astro/internal/ndm"
)

// The OCM's XML form, CCSDS 502.0-B-3 clause 8.11.
//
// It is the closest of the four to its key-value form, because the OCM's data
// rows stay rows. Clause 8.11.15 gives <trajLine>, <covLine> and <manLine> the
// type xsd:string: the schema does not look inside them, and the recipient
// still has to split the line on blanks and read the columns by whatever
// TRAJ_TYPE, COV_TYPE or MAN_COMPOSITION says. That is the opposite of the
// OEM, where clause 8.10.14 names every component of an ephemeris record.
//
// What does change is the units. In the key-value form they are a bracketed
// suffix on the value; in XML they are an attribute (clause 8.10.10). So a
// message crossing between the forms keeps its numbers and moves its units.
//
// Blocks replace the delimiters. Table 8-9 names one element per section:
// <traj>, <phys>, <cov>, <man>, <pert>, <od> and <user>. Their order carries
// the same meaning table 6-1 gives the delimiters.

// The section and data line elements of table 8-9.
const (
	xmlTraj = "traj"
	xmlPhys = "phys"
	xmlCov  = "cov"
	xmlMan  = "man"
	xmlPert = "pert"
	xmlOD   = "od"
	xmlUser = "user"

	xmlTrajLine = "trajLine"
	xmlCovLine  = "covLine"
	xmlManLine  = "manLine"
)

// xmlOCMBlocks maps each section element to the delimiter prefix its keywords
// come from, and to the element its data rows use. A section with no rows —
// physical characteristics, perturbations, orbit determination — has none.
var xmlOCMBlocks = []struct {
	element string
	prefix  string
	line    string
}{
	{xmlTraj, ocmTraj, xmlTrajLine},
	{xmlPhys, ocmPhys, ""},
	{xmlCov, ocmCov, xmlCovLine},
	{xmlMan, ocmMan, xmlManLine},
	{xmlPert, ocmPert, ""},
	{xmlOD, ocmOD, ""},
	{xmlUser, ocmUser, ""},
}

// EncodeXML writes the message in the XML form (clause 8.11).
func (m *OCM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    "ocm",
		ID:      "CCSDS_OCM_VERS",
		Version: m.Header.Version,
		Schema:  ndm.XMLSchemaODM,
		Header:  m.Header.xmlHeader(),
	}

	segment := ndm.Segment{Metadata: xmlOCMSection(m.Metadata, "")}
	for _, section := range m.Trajectories {
		segment.Data = append(segment.Data, ndm.Block(xmlTraj, xmlOCMSection(section, xmlTrajLine)...))
	}
	if m.Physical != nil {
		segment.Data = append(segment.Data, ndm.Block(xmlPhys, xmlOCMSection(*m.Physical, "")...))
	}
	for _, section := range m.Covariances {
		segment.Data = append(segment.Data, ndm.Block(xmlCov, xmlOCMSection(section, xmlCovLine)...))
	}
	for _, section := range m.Maneuvers {
		segment.Data = append(segment.Data, ndm.Block(xmlMan, xmlOCMSection(section, xmlManLine)...))
	}
	if m.Perturbations != nil {
		segment.Data = append(segment.Data, ndm.Block(xmlPert, xmlOCMSection(*m.Perturbations, "")...))
	}
	if m.OrbitDetermination != nil {
		segment.Data = append(segment.Data, ndm.Block(xmlOD, xmlOCMSection(*m.OrbitDetermination, "")...))
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

// xmlOCMSection renders one section: its comments, its keywords with any unit
// suffix moved into the attribute, then its data rows under line.
func xmlOCMSection(section OCMSection, line string) []ndm.Element {
	out := ndm.Comments(section.Comments)
	for _, f := range section.Fields {
		out = append(out, ndm.SplitLeaf(f.Keyword, f.Value))
	}
	if line == "" {
		return out
	}
	for _, row := range section.Rows {
		out = append(out, ndm.Leaf(line, strings.Join(row.Fields, " ")))
	}
	return out
}

// DecodeXMLOCM reads an Orbit Comprehensive Message in the XML form.
func DecodeXMLOCM(data []byte) (*OCM, error) {
	message, err := ndm.DecodeXML(data, "ocm")
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_OCM_VERS" {
		return nil, ErrNotAnOCM
	}

	header, err := readXMLHeader(message.Version, message.Header)
	if err != nil {
		return nil, err
	}
	m := &OCM{Header: header}

	// Clause 8.11 gives the OCM one segment: it has one metadata section
	// (clause 6.2.4.3), and a segment is a metadata section with its data.
	if len(message.Segments) != 1 {
		return nil, ndm.ErrMalformedXML
	}
	segment := message.Segments[0]

	m.Metadata = readXMLOCMSection(segment.Metadata, "")
	if err := readXMLOCMData(m, segment.Data); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// readXMLOCMData walks the data blocks in document order, so the section
// ordering of table 6-1 is checked in the XML form as it is in the key-value
// one.
func readXMLOCMData(m *OCM, data []ndm.Element) error {
	previous := -1

	for _, block := range data {
		at, line, ok := xmlOCMBlock(block.Name)
		if !ok {
			return ndm.ErrMalformedXML
		}
		if at < previous {
			return ErrSectionsOutOfOrder
		}
		repeated := xmlOCMBlocks[at].element == xmlTraj ||
			xmlOCMBlocks[at].element == xmlCov ||
			xmlOCMBlocks[at].element == xmlMan
		if at == previous && !repeated {
			return ErrDuplicateSection
		}
		previous = at

		if block.Name == xmlUser {
			params, err := readUserDefined(block.Children)
			if err != nil {
				return err
			}
			m.UserComments = append(m.UserComments, ndm.CollectComments(block.Children)...)
			m.UserDefined = append(m.UserDefined, params...)
			continue
		}

		section := readXMLOCMSection(block.Children, line)
		switch block.Name {
		case xmlTraj:
			m.Trajectories = append(m.Trajectories, section)
		case xmlCov:
			m.Covariances = append(m.Covariances, section)
		case xmlMan:
			m.Maneuvers = append(m.Maneuvers, section)
		case xmlPhys:
			m.Physical = &section
		case xmlPert:
			m.Perturbations = &section
		case xmlOD:
			m.OrbitDetermination = &section
		}
	}
	return nil
}

// xmlOCMBlock returns a block's place in table 6-1 and the element its data
// rows use.
func xmlOCMBlock(name string) (int, string, bool) {
	for i, block := range xmlOCMBlocks {
		if block.element == name {
			return i, block.line, true
		}
	}
	return 0, "", false
}

// readXMLOCMSection fills a section from its elements, putting the units
// attribute back on the value so both forms hold the same text.
func readXMLOCMSection(elements []ndm.Element, line string) OCMSection {
	section := OCMSection{Comments: ndm.CollectComments(elements)}

	for _, e := range elements {
		if e.Name == ndm.KeywordComment {
			continue
		}
		if line != "" && e.Name == line {
			if fields := strings.Fields(e.Value); len(fields) > 0 {
				section.Rows = append(section.Rows, DataRow{Fields: fields})
			}
			continue
		}
		section.Fields = append(section.Fields, Field{Keyword: e.Name, Value: e.JoinValue()})
	}
	return section
}
