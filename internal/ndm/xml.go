package ndm

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The XML form of a navigation data message, CCSDS 505.0-B-3.
//
// One codec serves every message type, because the mapping from the key-value
// form is almost entirely mechanical. A keyword becomes an element of the same
// name, in the same upper case: OBJECT_NAME becomes <OBJECT_NAME>. What the
// key-value form leaves implicit — which logical block a keyword belongs to —
// becomes a wrapper element named in lower camel case: <stateVector>,
// <spacecraftParameters>, <covarianceMatrix>.
//
// So a package that already knows its keywords and its blocks knows its XML.
// This carries the structure around them: the root element and its attributes,
// the header, and the body of segments that clauses 3.2 to 3.4 require.

// XMLNamespaceInstance is the schema instance namespace clause 4.3.3 requires
// in the root element of every instantiation, "exactly as shown". Note that
// http is correct and https is not: the string names a namespace, not a
// protocol, and clause 4.3.3 says so in a note because people change it.
const XMLNamespaceInstance = "http://www.w3.org/2001/XMLSchema-instance"

// XMLNamespace is the NDM/XML namespace clause 4.3.4 says "must next be
// coded, exactly as shown".
//
// The documents disagree about whether it appears. The TDM's worked example in
// section 5 declares it and the ODM's figure G-5 does not, while clause 4.3.4
// of the XML standard says it must. It is written here: an unqualified
// instantiation is unharmed by a namespace declaration it never uses, and
// clause 4.3.5 makes the prefix a matter of whether the elements are qualified
// rather than of whether the declaration is present.
const XMLNamespace = "urn:ccsds:schema:ndmxml"

// XMLSchemaBase is where the unqualified schema set lives (clause 4.3.6). The
// file name goes after it, and it is not the same file for every message.
const XMLSchemaBase = "https://sanaregistry.org/r/ndmxml_unqualified/"

// Master schema file names, one per navigation standard.
//
// These are not interchangeable, and the numbers do not track the NDM/XML
// document. Each standard names the schema issue it was written against:
// CCSDS 502.0-B-3 gives 3.0, CCSDS 504.0-B-2 gives 4.0, and both
// CCSDS 503.0-B-2 and CCSDS 508.0-B-1 give 2.0. Writing one message's
// location into another's instantiation produces a file that validates against
// the wrong schema, or fails to validate at all.
const (
	XMLSchemaODM = "ndmxml-3.0.0-master-3.0.xsd"
	XMLSchemaADM = "ndmxml-4.0.0-master-4.0.xsd"
	XMLSchemaTDM = "ndmxml-2.0.0-master-2.0.xsd"
	XMLSchemaCDM = "ndmxml-2.0.0-master-2.0.xsd"
)

// maxXMLDepth bounds element nesting. Real navigation messages nest about six
// levels (ndm, message, segment, data, block, element); the limit exists
// because readElement recurses once per level, so without one a file of
// repeated open tags exhausts the goroutine stack rather than failing
// cleanly.
const maxXMLDepth = 64

// Element is one element of a message: either a leaf carrying a value or a
// block carrying children. A block is a wrapper the key-value form has no
// equivalent for.
type Element struct {
	Name string
	// Value is a leaf's text, without its units.
	Value string
	// Units is the units attribute, when the element has one.
	//
	// This is where the two forms genuinely differ rather than merely look
	// different. The key-value form writes the unit inside the value, after a
	// blank and in square brackets — "715 [m]" — and clause 7.7.1.1 of the
	// ODM calls that documentation. The XML form writes it as an attribute:
	// <MISS_DISTANCE units="m">715</MISS_DISTANCE>. Carrying it in the value
	// here would put the brackets inside the element text, which no schema
	// accepts.
	Units string
	// Parameter is the parameter attribute, which one element uses.
	//
	// The user-defined keyword is the only place the two forms name a thing
	// differently rather than merely place it differently. In the key-value
	// form the name is part of the keyword — USER_DEFINED_EARTH_MODEL — and in
	// XML it is an attribute on a fixed element name:
	// <USER_DEFINED parameter="EARTH_MODEL">WGS-84</USER_DEFINED>.
	Parameter string
	Children  []Element
}

// Leaf returns an element carrying a value.
func Leaf(name, value string) Element { return Element{Name: name, Value: value} }

// LeafWithUnits returns an element carrying a value and a units attribute.
// A value whose units are empty is written without the attribute.
func LeafWithUnits(name, value, units string) Element {
	return Element{Name: name, Value: value, Units: units}
}

// UserDefined returns a user-defined parameter element.
func UserDefined(name, value string) Element {
	return Element{Name: KeywordUserDefined, Value: value, Parameter: name}
}

// SplitLeaf returns an element from a key-value form value, moving any unit
// suffix out of the text and into the attribute. A value with no suffix comes
// back unchanged.
func SplitLeaf(name, value string) Element {
	number, units, err := SplitUnits(value)
	if err != nil {
		// Not a unit suffix this package recognises, so the value stands as
		// it is rather than being mangled.
		return Leaf(name, value)
	}
	return LeafWithUnits(name, number, units)
}

// JoinValue returns the key-value form of a leaf: its value with the units
// appended in the brackets clause 7.7.1.1 defines, or the bare value when the
// element has none.
func (e Element) JoinValue() string {
	if e.Units == "" {
		return e.Value
	}
	return e.Value + " [" + e.Units + "]"
}

// Block returns an element carrying children, or the zero Element when there
// are none — an empty block is written as nothing rather than as an empty tag,
// because a block exists only to group the keywords inside it.
func Block(name string, children ...Element) Element {
	children = present(children)
	if len(children) == 0 {
		return Element{}
	}
	return Element{Name: name, Children: children}
}

// present drops the zero elements that Block and the optional-value helpers
// return, so a caller can build a block from a fixed list without testing each
// entry.
func present(elements []Element) []Element {
	out := make([]Element, 0, len(elements))
	for _, e := range elements {
		if e.Name != "" {
			out = append(out, e)
		}
	}
	return out
}

// Comments returns one element per comment line. The XML form keeps COMMENT as
// an element like any other, so a multi-line comment is several of them.
func Comments(texts []string) []Element {
	out := make([]Element, 0, len(texts))
	for _, text := range texts {
		out = append(out, Leaf(KeywordComment, text))
	}
	return out
}

// Segment is a metadata and data pair. Clause 3.2.3 makes the pairing the
// point of the segment: it is what enforces the ordering that the key-value
// form leaves to convention.
type Segment struct {
	Metadata []Element
	Data     []Element
}

// XMLMessage is one navigation data message in XML.
type XMLMessage struct {
	// Root is the element name clause 4.3.2 assigns the message type: opm,
	// omm, oem, apm, aem, tdm or cdm.
	Root string
	// ID is the version keyword, such as CCSDS_OPM_VERS, and Version its
	// value. Together they are the root element's id and version attributes.
	ID      string
	Version string
	// Schema is the master schema file name, one of the XMLSchema constants.
	// It goes after XMLSchemaBase in the xsi:noNamespaceSchemaLocation
	// attribute.
	Schema string

	Header []Element
	// Relative holds the CDM's relative metadata and data, which clause 3.4.2
	// puts before the first segment and which no other message has.
	Relative []Element
	Segments []Segment
}

// EncodeXML writes the message as a document of its own.
func (m *XMLMessage) EncodeXML() ([]byte, error) {
	if m.Schema == "" {
		return nil, ErrMissingHeaderField
	}

	var b strings.Builder
	b.WriteString(xml.Header)
	if err := m.write(&b, "", m.Schema); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// write renders the message's element and everything under it.
//
// schema is the master schema location for the root element, or empty for a
// message inside a combined instantiation: clause 4.11.5 of CCSDS 505.0-B-3
// allows a constituent message tag no attributes but 'id' and 'version',
// because the namespace and schema attributes belong to the <ndm> root that
// wraps it.
func (m *XMLMessage) write(b *strings.Builder, indent, schema string) error {
	if m.Root == "" || m.ID == "" || m.Version == "" {
		return ErrMissingHeaderField
	}

	if schema != "" {
		fmt.Fprintf(b, "%s<%s xmlns:xsi=%q\n", indent, m.Root, XMLNamespaceInstance)
		fmt.Fprintf(b, "%s     xmlns:ndm=%q\n", indent, XMLNamespace)
		fmt.Fprintf(b, "%s     xsi:noNamespaceSchemaLocation=%q\n", indent, XMLSchemaBase+schema)
		fmt.Fprintf(b, "%s     id=%q version=%q>\n", indent, m.ID, m.Version)
	} else {
		fmt.Fprintf(b, "%s<%s id=%q version=%q>\n", indent, m.Root, m.ID, m.Version)
	}

	writeElements(b, indent+"  ", []Element{{Name: "header", Children: present(m.Header)}})

	fmt.Fprintf(b, "%s  <body>\n", indent)
	if len(m.Relative) > 0 {
		writeElements(b, indent+"    ", []Element{
			{Name: "relativeMetadataData", Children: present(m.Relative)},
		})
	}
	for _, segment := range m.Segments {
		children := []Element{{Name: "metadata", Children: present(segment.Metadata)}}
		if data := present(segment.Data); len(data) > 0 {
			children = append(children, Element{Name: "data", Children: data})
		}
		writeElements(b, indent+"    ", []Element{{Name: "segment", Children: children}})
	}
	fmt.Fprintf(b, "%s  </body>\n", indent)

	fmt.Fprintf(b, "%s</%s>\n", indent, m.Root)
	return nil
}

// writeElements writes a tree, indenting as it descends.
func writeElements(b *strings.Builder, indent string, elements []Element) {
	for _, e := range elements {
		if len(e.Children) == 0 {
			fmt.Fprintf(b, "%s<%s%s>%s</%s>\n",
				indent, e.Name, attributes(e), escapeXML(e.Value), e.Name)
			continue
		}
		fmt.Fprintf(b, "%s<%s>\n", indent, e.Name)
		writeElements(b, indent+"  ", e.Children)
		fmt.Fprintf(b, "%s</%s>\n", indent, e.Name)
	}
}

// attributes renders the attributes an element carries, in the order the
// schema set writes them.
func attributes(e Element) string {
	var b strings.Builder
	if e.Parameter != "" {
		fmt.Fprintf(&b, " parameter=%q", e.Parameter)
	}
	if e.Units != "" {
		fmt.Fprintf(&b, " units=%q", e.Units)
	}
	return b.String()
}

// escapeXML escapes the five characters XML reserves.
func escapeXML(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// EscapeText only fails when the writer does, and a strings.Builder
		// does not.
		return s
	}
	return b.String()
}

// DecodeXML reads a message, checking the root element is the one expected.
//
// The structure is checked and the element names are not: which keywords a
// message may carry is its own standard's business, and the caller knows that
// table. What this enforces is clauses 3.2 to 3.4 — a header, a body, segments
// of metadata and data — and the root attributes of clause 4.3.
func DecodeXML(data []byte, root string) (*XMLMessage, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))

	start, err := nextStart(d)
	if err != nil {
		return nil, err
	}
	if start.Name.Local != root {
		return nil, ErrWrongMessageType
	}

	return decodeMessage(d, start, 0)
}

// decodeMessage reads one message's attributes and content, the decoder having
// just returned its start element.
//
// It serves both a standalone instantiation and a constituent of a combined
// one. The difference is only the attributes: clause 4.11.5 gives a
// constituent no schema location, so its Schema is left empty here and the
// caller fills in the one the <ndm> root carried.
//
// depth is the nesting depth of start itself: 0 for a standalone document's
// root, or the depth of a constituent tag inside a combined instantiation.
func decodeMessage(d *xml.Decoder, start xml.StartElement, depth int) (*XMLMessage, error) {
	m := &XMLMessage{Root: start.Name.Local}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			m.ID = attr.Value
		case "version":
			m.Version = attr.Value
		case "noNamespaceSchemaLocation":
			// Kept so that a decoded message re-encodes with the schema it
			// arrived under. Each standard names its own, and substituting
			// one for another produces a file that validates against the
			// wrong schema.
			m.Schema = strings.TrimPrefix(attr.Value, XMLSchemaBase)
		}
	}
	if m.ID == "" || m.Version == "" {
		// Clause 4.3.8 and 4.3.9 make both attributes mandatory, and without
		// the id there is nothing to say which message this is.
		return nil, ErrNoVersionLine
	}
	if m.Schema == "" {
		// Clause 4.3.6 makes the schema location optional — it is there to
		// validate against, not to read the message. A caller that re-encodes
		// needs one, so the default is the message type's own.
		m.Schema = defaultSchema(m.Root)
	}

	children, err := readChildren(d, depth+1)
	if err != nil {
		return nil, err
	}
	for _, e := range children {
		switch e.Name {
		case "header":
			m.Header = e.Children
		case "body":
			if err := readBody(m, e.Children); err != nil {
				return nil, err
			}
		default:
			return nil, ErrUnknownHeaderKeyword
		}
	}
	if len(m.Segments) == 0 {
		return nil, ErrMalformedXML
	}
	return m, nil
}

// defaultSchema returns the master schema a message type names, for an
// instantiation that carried no location attribute.
func defaultSchema(root string) string {
	switch root {
	case "opm", "omm", "oem", "ocm":
		return XMLSchemaODM
	case "apm", "aem", "acm":
		return XMLSchemaADM
	case "tdm":
		return XMLSchemaTDM
	case "cdm":
		return XMLSchemaCDM
	}
	return XMLSchemaODM
}

// readBody sorts a body's children into the relative section and the segments.
func readBody(m *XMLMessage, children []Element) error {
	for _, e := range children {
		switch e.Name {
		case "relativeMetadataData":
			m.Relative = e.Children
		case "segment":
			var segment Segment
			for _, part := range e.Children {
				switch part.Name {
				case "metadata":
					segment.Metadata = part.Children
				case "data":
					segment.Data = part.Children
				default:
					return ErrMalformedXML
				}
			}
			m.Segments = append(m.Segments, segment)
		default:
			return ErrMalformedXML
		}
	}
	return nil
}

// nextStart returns the next start element, skipping the declaration,
// comments and whitespace.
func nextStart(d *xml.Decoder) (xml.StartElement, error) {
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return xml.StartElement{}, ErrMalformedXML
		}
		if err != nil {
			return xml.StartElement{}, err
		}
		if start, ok := token.(xml.StartElement); ok {
			return start, nil
		}
	}
}

// readChildren reads elements until the enclosing element closes.
//
// An element with element children is a block and an element with text is a
// leaf. Mixed content — text and elements together — is not something the
// schema set produces, and is refused rather than half-read.
//
// depth is the depth of the children it reads, i.e. one more than the
// enclosing element's own depth.
func readChildren(d *xml.Decoder, depth int) ([]Element, error) {
	var out []Element

	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil, ErrMalformedXML
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			child, err := readElement(d, t, depth)
			if err != nil {
				return nil, err
			}
			out = append(out, child)
		case xml.EndElement:
			return out, nil
		}
	}
}

// readElement reads one element, its attributes and its content.
//
// depth is the nesting depth of start itself. It is checked against
// maxXMLDepth before anything else is done with the element, so a file of
// deeply nested open tags is refused instead of recursing until the
// goroutine stack runs out — Go's own depth cap on encoding/xml applies only
// to Unmarshal/Decode, not to a reader that drives Token directly, as this
// one does.
func readElement(d *xml.Decoder, start xml.StartElement, depth int) (Element, error) {
	if depth > maxXMLDepth {
		return Element{}, ErrMalformedXML
	}

	e := Element{Name: start.Name.Local}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "units":
			e.Units = attr.Value
		case "parameter":
			e.Parameter = attr.Value
		}
	}

	var text strings.Builder

	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return e, ErrMalformedXML
		}
		if err != nil {
			return e, err
		}

		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			child, err := readElement(d, t, depth+1)
			if err != nil {
				return e, err
			}
			e.Children = append(e.Children, child)
		case xml.EndElement:
			if len(e.Children) > 0 {
				if strings.TrimSpace(text.String()) != "" {
					return e, ErrMalformedXML
				}
				return e, nil
			}
			// A leaf's value keeps its inner whitespace but not the
			// indentation around it.
			e.Value = strings.TrimSpace(text.String())
			return e, nil
		}
	}
}

// Find returns the value of the first leaf with this name among the elements,
// and whether it was there. The units are not included; use FindLeaf when they
// are wanted.
func Find(elements []Element, name string) (string, bool) {
	e, ok := FindLeaf(elements, name)
	return e.Value, ok
}

// FindLeaf returns the first leaf with this name, units included.
func FindLeaf(elements []Element, name string) (Element, bool) {
	for _, e := range elements {
		if e.Name == name && len(e.Children) == 0 {
			return e, true
		}
	}
	return Element{}, false
}

// FindBlock returns the children of the first block with this name.
func FindBlock(elements []Element, name string) ([]Element, bool) {
	for _, e := range elements {
		if e.Name == name && len(e.Children) > 0 {
			return e.Children, true
		}
	}
	return nil, false
}

// CollectComments returns the values of the COMMENT elements among these.
func CollectComments(elements []Element) []string {
	var out []string
	for _, e := range elements {
		if e.Name == KeywordComment && len(e.Children) == 0 {
			out = append(out, e.Value)
		}
	}
	return out
}
