package cdm

import "github.com/ravisuhag/astro/internal/ndm"

// The XML form, CCSDS 508.0-B-1 section 4 with the structure of
// CCSDS 505.0-B-3.
//
// The CDM maps onto the XML structure more directly than any other navigation
// message, because its sections are already flat keyword lists. Clause 3.4.2
// of the XML standard describes it as a variant of substructure 2: exactly two
// segments, with the relative metadata and data in a
// <relativeMetadataData> element before the first.
//
// Three things differ between the forms, and none of them is cosmetic.
//
// The OBJECT keyword is the section boundary in the key-value form; in XML the
// segments are, and OBJECT becomes an ordinary element that could disagree
// with the segment it sits in. checkXMLObjects is what stops that, because a
// disagreement would flip which object an operator thinks can manoeuvre.
//
// Units are an attribute rather than part of the value. The key-value form
// writes "715 [m]" and the XML form writes
// <MISS_DISTANCE units="m">715</MISS_DISTANCE>.
//
// The XML form nests blocks the key-value form leaves flat. A CDM's data
// section is one run of keywords in KVN and four elements in XML —
// odParameters, additionalParameters, stateVector and covarianceMatrix — so
// converting between the forms means knowing which keyword belongs to which
// block. xmlBlocks is that table.

// xmlRoot is the root element clause 4.3.2 assigns this message type.
const xmlRoot = "cdm"

// XML block names, from the worked example in section 4.
const (
	xmlRelativeStateVector = "relativeStateVector"
	xmlODParameters        = "odParameters"
	xmlAdditionalParams    = "additionalParameters"
	xmlStateVector         = "stateVector"
	xmlCovarianceMatrix    = "covarianceMatrix"
)

// xmlRelativeBlocks says which relative keywords sit inside a block in the XML
// form. Everything else in that section is a direct child.
var xmlRelativeBlocks = map[string]string{
	"RELATIVE_POSITION_R": xmlRelativeStateVector,
	"RELATIVE_POSITION_T": xmlRelativeStateVector,
	"RELATIVE_POSITION_N": xmlRelativeStateVector,
	"RELATIVE_VELOCITY_R": xmlRelativeStateVector,
	"RELATIVE_VELOCITY_T": xmlRelativeStateVector,
	"RELATIVE_VELOCITY_N": xmlRelativeStateVector,
}

// xmlObjectBlocks says which object keywords sit inside a data block. The
// metadata keywords of table 3-3 are not here: they are direct children of
// <metadata>.
var xmlObjectBlocks = func() map[string]string {
	out := map[string]string{}
	for _, keyword := range []string{
		"TIME_LASTOB_START", "TIME_LASTOB_END",
		"RECOMMENDED_OD_SPAN", "ACTUAL_OD_SPAN",
		"OBS_AVAILABLE", "OBS_USED", "TRACKS_AVAILABLE", "TRACKS_USED",
		"RESIDUALS_ACCEPTED", "WEIGHTED_RMS",
	} {
		out[keyword] = xmlODParameters
	}
	for _, keyword := range []string{
		"AREA_PC", "AREA_DRG", "AREA_SRP", "MASS",
		"CD_AREA_OVER_MASS", "CR_AREA_OVER_MASS",
		"THRUST_ACCELERATION", "SEDR",
	} {
		out[keyword] = xmlAdditionalParams
	}
	for _, keyword := range []string{"X", "Y", "Z", "X_DOT", "Y_DOT", "Z_DOT"} {
		out[keyword] = xmlStateVector
	}
	for keyword := range covarianceIndex {
		out[keyword] = xmlCovarianceMatrix
	}
	return out
}()

// xmlBlockOrder is the order the blocks appear in, which section 4's example
// fixes. A map has no order, so the order lives here.
var xmlBlockOrder = []string{
	xmlRelativeStateVector,
	xmlODParameters, xmlAdditionalParams, xmlStateVector, xmlCovarianceMatrix,
}

// nestSection turns a flat key-value section into the nested XML form, using
// the given keyword-to-block table. Keywords with no block stay at the top,
// in the order they arrived; blocks follow in xmlBlockOrder.
func nestSection(s Section, blocks map[string]string) []ndm.Element {
	top := ndm.Comments(s.Comments)
	grouped := map[string][]ndm.Element{}

	for _, f := range s.Fields {
		leaf := ndm.SplitLeaf(f.Keyword, f.Value)
		if block, ok := blocks[f.Keyword]; ok {
			grouped[block] = append(grouped[block], leaf)
			continue
		}
		top = append(top, leaf)
	}

	for _, name := range xmlBlockOrder {
		if children := grouped[name]; len(children) > 0 {
			top = append(top, ndm.Block(name, children...))
		}
	}
	return top
}

// flattenSection is nestSection in reverse: it walks the XML elements, top
// level and blocks alike, and produces the flat keyword list the key-value
// form uses.
func flattenSection(elements []ndm.Element) (comments []string, fields []Field) {
	for _, e := range elements {
		if len(e.Children) > 0 {
			blockComments, blockFields := flattenSection(e.Children)
			comments = append(comments, blockComments...)
			fields = append(fields, blockFields...)
			continue
		}
		if e.Name == ndm.KeywordComment {
			comments = append(comments, e.Value)
			continue
		}
		fields = append(fields, Field{Keyword: e.Name, Value: e.JoinValue()})
	}
	return comments, fields
}

// EncodeXML writes the message in the XML form.
func (m *CDM) EncodeXML() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	message := &ndm.XMLMessage{
		Root:    xmlRoot,
		ID:      "CCSDS_CDM_VERS",
		Version: m.Header.Version,
		// Not the ODM's schema. CCSDS 508.0-B-1 names issue 2.0, and writing
		// another message's location here produces a file that validates
		// against the wrong schema, or fails to validate at all.
		Schema:   ndm.XMLSchemaCDM,
		Header:   m.xmlHeader(),
		Relative: nestSection(m.Relative, xmlRelativeBlocks),
	}
	for i := range m.Objects {
		metadata, data := splitObjectSection(m.Objects[i].Section)
		message.Segments = append(message.Segments, ndm.Segment{
			Metadata: metadata,
			Data:     data,
		})
	}
	return message.EncodeXML()
}

func (m *CDM) xmlHeader() []ndm.Element {
	out := ndm.Comments(m.Header.Comments)

	created, err := ndm.FormatEpoch(m.Header.CreationDate.UTC(), 3)
	if err == nil {
		out = append(out, ndm.Leaf(ndm.KeywordCreationDate, created))
	}
	out = append(out, ndm.Leaf(ndm.KeywordOriginator, m.Header.Originator))
	if m.Header.MessageFor != "" {
		out = append(out, ndm.Leaf(ndm.KeywordMessageFor, m.Header.MessageFor))
	}
	return append(out, ndm.Leaf(ndm.KeywordMessageID, m.Header.MessageID))
}

// splitObjectSection divides one object's flat keyword list into the XML
// form's metadata and data halves.
//
// The key-value form does not separate them — an object section is one run of
// keywords from OBJECT to the last covariance element — so the split comes
// from the tables: table 3-3 is metadata and tables 3-5 through 3-8 are data.
func splitObjectSection(s Section) (metadata, data []ndm.Element) {
	var metaSection, dataSection Section
	metaSection.Comments = s.Comments

	for _, f := range s.Fields {
		if objectMetadataKeywords[f.Keyword] {
			metaSection.Fields = append(metaSection.Fields, f)
			continue
		}
		dataSection.Fields = append(dataSection.Fields, f)
	}
	return nestSection(metaSection, nil), nestSection(dataSection, xmlObjectBlocks)
}

// DecodeXML reads a Conjunction Data Message in the XML form.
func DecodeXML(data []byte) (*CDM, error) {
	message, err := ndm.DecodeXML(data, xmlRoot)
	if err != nil {
		return nil, err
	}
	if message.ID != "CCSDS_CDM_VERS" {
		return nil, ErrNotACDM
	}
	// Clause 3.4.2: exactly two segments, one per object.
	if len(message.Segments) != 2 {
		return nil, ErrMissingObject
	}

	m := &CDM{Header: Header{Version: message.Version}}
	if err := m.readXMLHeader(message.Header); err != nil {
		return nil, err
	}

	if err := readXMLSection(&m.Relative, message.Relative, -1); err != nil {
		return nil, err
	}
	for i := range message.Segments {
		// The two halves rejoin here, because the key-value form has one
		// section per object and the accessors read from it.
		elements := append([]ndm.Element{}, message.Segments[i].Metadata...)
		elements = append(elements, message.Segments[i].Data...)
		if err := readXMLSection(&m.Objects[i].Section, elements, i); err != nil {
			return nil, err
		}
	}

	if err := m.checkXMLObjects(); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *CDM) readXMLHeader(elements []ndm.Element) error {
	m.Header.Comments = ndm.CollectComments(elements)

	created, ok := ndm.Find(elements, ndm.KeywordCreationDate)
	if !ok {
		return ndm.ErrMissingHeaderField
	}
	t, err := ndm.ParseEpoch(created)
	if err != nil {
		return err
	}
	m.Header.CreationDate = t

	if m.Header.Originator, ok = ndm.Find(elements, ndm.KeywordOriginator); !ok {
		return ndm.ErrMissingHeaderField
	}
	if m.Header.MessageID, ok = ndm.Find(elements, ndm.KeywordMessageID); !ok {
		return ndm.ErrMissingHeaderField
	}
	m.Header.MessageFor, _ = ndm.Find(elements, ndm.KeywordMessageFor)
	return nil
}

// readXMLSection fills a section from its elements, flattening the blocks and
// checking each keyword belongs where it appeared. current is -1 for the
// relative section.
func readXMLSection(s *Section, elements []ndm.Element, current int) error {
	comments, fields := flattenSection(elements)
	s.Comments = append(s.Comments, comments...)

	seen := make(map[string]bool)
	for _, f := range fields {
		if err := checkPlacement(current, f.Keyword); err != nil {
			return err
		}
		if seen[f.Keyword] {
			return ErrDuplicateKeyword
		}
		seen[f.Keyword] = true
		s.Fields = append(s.Fields, f)
	}
	return nil
}

// checkXMLObjects enforces what the segments cannot: that the two OBJECT
// values are OBJECT1 and OBJECT2, in that order.
//
// In the key-value form the OBJECT keyword is the section boundary, so a wrong
// value is caught while reading. Here the segments already separate the
// objects, and OBJECT becomes an ordinary element that could disagree with the
// segment it sits in — which would flip which object an operator thinks can
// manoeuvre.
func (m *CDM) checkXMLObjects() error {
	for i := range m.Objects {
		value, ok := m.Objects[i].Get("OBJECT")
		if !ok {
			return ErrMissingKeyword
		}
		index, err := objectIndex(value)
		if err != nil {
			return err
		}
		if index != i {
			return ErrObjectRepeated
		}
	}
	return nil
}
