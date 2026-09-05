package ndm

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The NDM combined instantiation, CCSDS 505.0-B-3 clause 4.11.
//
// One XML file may carry any number of navigation messages of any types, in
// any order, wrapped in an <ndm> root. Clause 4.11.2 gives the reasons: a
// constellation's ephemerides together, an attitude message beside the orbit
// it depends on, an ephemeris with the tracking data that produced it. The
// two standards repeat the rules for their own messages in CCSDS 502.0-B-3
// clause 8.12 and CCSDS 504.0-B-2 clause 7.8.
//
// Three rules shape the file, and all three are about attributes:
//
//   - Clause 4.11.3: the root is <ndm> rather than a message tag.
//   - Clause 4.11.4: <ndm> carries the namespace and schema attributes but
//     neither 'id' nor 'version' — it is not a message and has no version.
//   - Clause 4.11.5: a constituent message tag carries 'id' and 'version' and
//     nothing else. The attributes it would have as a standalone document move
//     to the root.
//
// So a combined instantiation is not a concatenation of files. Splicing whole
// documents together would leave each message's namespace and schema
// attributes where clause 4.11.5 forbids them, and leave several XML
// declarations in one file.

// XMLCombinedRoot is the root element clause 4.11.3 requires.
const XMLCombinedRoot = "ndm"

// CombinedXML is an NDM combined instantiation.
type CombinedXML struct {
	// Schema is the master schema file name for the whole file, going after
	// XMLSchemaBase in the root's xsi:noNamespaceSchemaLocation attribute.
	//
	// One file, one location — which is awkward when the constituents come
	// from standards that name different masters, and the documents show it.
	// Figure 7-3 of CCSDS 504.0-B-2 writes ndmxml-4.0.0-master-4.0.xsd over a
	// file of ADM messages and its own figure G-12 writes
	// ndmxml-3.0.0-master-3.0.xsd over another. This carries whatever the file
	// had and leaves the choice to the caller.
	Schema string
	// Comments are the COMMENT elements directly under the root, which the
	// figures of clause 4.11.9 show before the first message.
	Comments []string
	// Messages are the constituents, in the order they appeared. Clause 4.11.7
	// allows any combination of types.
	Messages []*XMLMessage
}

// EncodeXML writes the combined instantiation.
func (c *CombinedXML) EncodeXML() ([]byte, error) {
	if c.Schema == "" {
		return nil, ErrMissingHeaderField
	}

	var b strings.Builder
	b.WriteString(xml.Header)

	fmt.Fprintf(&b, "<%s xmlns:xsi=%q\n", XMLCombinedRoot, XMLNamespaceInstance)
	fmt.Fprintf(&b, "     xmlns:ndm=%q\n", XMLNamespace)
	// Clause 4.11.4: the standard attributes, but no id and no version.
	fmt.Fprintf(&b, "     xsi:noNamespaceSchemaLocation=%q>\n", XMLSchemaBase+c.Schema)

	writeElements(&b, "  ", Comments(c.Comments))

	for _, m := range c.Messages {
		// The empty schema is what tells write to emit a constituent tag
		// rather than a root one: id and version, and nothing else.
		if err := m.write(&b, "  ", ""); err != nil {
			return nil, err
		}
	}

	fmt.Fprintf(&b, "</%s>\n", XMLCombinedRoot)
	return []byte(b.String()), nil
}

// DecodeCombinedXML reads a combined instantiation.
//
// A constituent inherits the root's schema location, and one written on a
// constituent in spite of clause 4.11.5 is discarded rather than kept. That
// matters on re-encoding: a message pulled out of a combined file and written
// as a document of its own needs some location, and the root of the file it
// came from is where the standard says to look.
func DecodeCombinedXML(data []byte) (*CombinedXML, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))

	start, err := nextStart(d)
	if err != nil {
		return nil, err
	}
	if start.Name.Local != XMLCombinedRoot {
		return nil, ErrWrongMessageType
	}

	c := &CombinedXML{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "noNamespaceSchemaLocation":
			c.Schema = strings.TrimPrefix(attr.Value, XMLSchemaBase)
		case "id", "version":
			// Clause 4.11.4 says neither is associated with the <ndm> tag.
			// A file carrying one was written against the single-message
			// rules and its constituents cannot be trusted to follow 4.11.5.
			return nil, ErrMalformedXML
		}
	}
	if c.Schema == "" {
		// Clause 4.3.6 makes the location optional. A caller that re-encodes
		// needs one, and the combined schema is imported by every master, so
		// the ODM's serves as well as any.
		c.Schema = XMLSchemaODM
	}

	if err := readCombinedChildren(d, c); err != nil {
		return nil, err
	}
	return c, nil
}

// readCombinedChildren reads what lies between the <ndm> tags.
//
// It walks the token stream itself rather than going through readChildren,
// because a constituent's 'id' and 'version' attributes live on its start
// element and readElement keeps only the two attributes a leaf may have.
func readCombinedChildren(d *xml.Decoder, c *CombinedXML) error {
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			// The root never closed.
			return ErrMalformedXML
		}
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// t is a direct child of <ndm>, at depth 1: the root itself is
			// depth 0, read outside this function by nextStart.
			if t.Name.Local == KeywordComment {
				comment, err := readElement(d, t, 1)
				if err != nil {
					return err
				}
				c.Comments = append(c.Comments, comment.Value)
				continue
			}
			message, err := decodeMessage(d, t, 1)
			if err != nil {
				return err
			}
			message.Schema = c.Schema
			c.Messages = append(c.Messages, message)

		case xml.EndElement:
			// Clause 4.11.8 says a combined instantiation "should" consist of
			// at least one constituent message. It is a should rather than a
			// shall, so an empty one is odd but not malformed, and refusing it
			// would refuse a file the standard permits.
			return nil
		}
	}
}

// XMLRootName reports the root element of an instantiation without reading the
// rest of it, so a caller handed a file can tell a combined instantiation from
// a single message before choosing a decoder.
func XMLRootName(data []byte) (string, error) {
	start, err := nextStart(xml.NewDecoder(strings.NewReader(string(data))))
	if err != nil {
		return "", err
	}
	return start.Name.Local, nil
}
