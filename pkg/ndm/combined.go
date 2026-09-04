package ndm

import (
	"fmt"
	"strings"

	xmlndm "github.com/ravisuhag/astro/internal/ndm"
	"github.com/ravisuhag/astro/pkg/adm"
	"github.com/ravisuhag/astro/pkg/cdm"
	"github.com/ravisuhag/astro/pkg/odm"
	"github.com/ravisuhag/astro/pkg/tdm"
)

// Message is one constituent of a combined instantiation.
//
// Its dynamic type is one of the nine navigation messages this repository
// implements: *odm.OPM, *odm.OMM, *odm.OEM, *odm.OCM, *adm.APM, *adm.AEM,
// *adm.ACM, *tdm.TDM or *cdm.CDM. A caller reads one with a type switch, or
// asks Kind which it is.
//
// The interface is the two methods every one of them already has, rather than
// something they were made to satisfy. Nothing about a message changes for
// being inside a combined file.
type Message interface {
	// EncodeXML writes the message as a document of its own.
	EncodeXML() ([]byte, error)
	// Humanize returns a readable summary.
	Humanize() string
}

// Combined is an NDM combined instantiation (CCSDS 505.0-B-3 clause 4.11).
type Combined struct {
	// Schema is the master schema file name for the whole file. Encode fills
	// an empty one from the first message.
	Schema string
	// Comments are the COMMENT elements directly under the root.
	Comments []string
	// Messages are the constituents, in the order they appeared. Clause 4.11.7
	// allows any combination of types, and clause 4.11.8 asks for at least one
	// only as a should.
	Messages []Message
}

// IsCombined reports whether the data is a combined instantiation rather than
// a single message, by its root element alone.
//
// A caller handed a navigation file that could be either should ask this
// before choosing a decoder. Anything unreadable is reported as not combined,
// and the decoder the caller then picks gives the real error.
func IsCombined(data []byte) bool {
	root, err := xmlndm.XMLRootName(data)
	return err == nil && root == xmlndm.XMLCombinedRoot
}

// DecodeCombined reads a combined instantiation.
//
// Each constituent is handed to its own package's XML decoder, so a message
// inside a combined file is held to exactly the rules it would be held to on
// its own — the keyword tables, the block structure, the row widths. A file
// whose OPM would be refused alone is refused here.
func DecodeCombined(data []byte) (*Combined, error) {
	raw, err := xmlndm.DecodeCombinedXML(data)
	if err != nil {
		return nil, err
	}

	c := &Combined{Schema: raw.Schema, Comments: raw.Comments}
	for _, m := range raw.Messages {
		// The constituent is written back out as a document of its own and
		// read by its package's decoder. That costs a serialise and a parse
		// per message, and buys the guarantee that a message means the same
		// thing in a combined file as in a single one: there is one decoder
		// per message type, not two.
		document, err := m.EncodeXML()
		if err != nil {
			return nil, err
		}
		message, err := decodeConstituent(m.Root, document)
		if err != nil {
			return nil, err
		}
		c.Messages = append(c.Messages, message)
	}
	return c, nil
}

// decodeConstituent routes one message to its package by root element.
func decodeConstituent(root string, document []byte) (Message, error) {
	switch root {
	case "opm":
		return odm.DecodeXMLOPM(document)
	case "omm":
		return odm.DecodeXMLOMM(document)
	case "oem":
		return odm.DecodeXMLOEM(document)
	case "ocm":
		return odm.DecodeXMLOCM(document)
	case "apm":
		return adm.DecodeXMLAPM(document)
	case "aem":
		return adm.DecodeXMLAEM(document)
	case "acm":
		return adm.DecodeXMLACM(document)
	case "tdm":
		return tdm.DecodeXML(document)
	case "cdm":
		return cdm.DecodeXML(document)
	}
	// Clause 4.11.6 draws the constituents from table 3-1, which also lists
	// the Re-entry Data Message of CCSDS 508.1. That standard has no package
	// here, so an <rdm> lands in this branch rather than being half-read.
	return nil, ErrUnknownMessageType
}

// Encode writes the combined instantiation.
//
// Every constituent goes out with only the 'id' and 'version' attributes
// clause 4.11.5 allows it; the namespace and schema attributes are written
// once, on the root.
func (c *Combined) Encode() ([]byte, error) {
	raw := &xmlndm.CombinedXML{Schema: c.Schema, Comments: c.Comments}

	for _, m := range c.Messages {
		if m == nil {
			return nil, ErrNoMessage
		}
		document, err := m.EncodeXML()
		if err != nil {
			return nil, err
		}
		root, err := xmlndm.XMLRootName(document)
		if err != nil {
			return nil, err
		}
		parsed, err := xmlndm.DecodeXML(document, root)
		if err != nil {
			return nil, err
		}
		raw.Messages = append(raw.Messages, parsed)
	}

	if raw.Schema == "" {
		// One file names one master schema, and the standards disagree about
		// which. The first message's own is the least surprising choice: a
		// file of orbit messages gets the ODM's, a file of attitude messages
		// the ADM's.
		if len(raw.Messages) == 0 {
			return nil, ErrNoMessage
		}
		raw.Schema = raw.Messages[0].Schema
	}
	return raw.EncodeXML()
}

// Kind returns the message type's element name: opm, omm, oem, ocm, apm, aem,
// acm, tdm or cdm. It is the name the message would have as an XML root, and
// what clause 4.11.6 calls a message tag.
func Kind(m Message) string {
	switch m.(type) {
	case *odm.OPM:
		return "opm"
	case *odm.OMM:
		return "omm"
	case *odm.OEM:
		return "oem"
	case *odm.OCM:
		return "ocm"
	case *adm.APM:
		return "apm"
	case *adm.AEM:
		return "aem"
	case *adm.ACM:
		return "acm"
	case *tdm.TDM:
		return "tdm"
	case *cdm.CDM:
		return "cdm"
	}
	return ""
}

// Humanize returns a readable summary of the file: what it holds, then each
// message's own summary.
func (c *Combined) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CCSDS NDM combined instantiation, %d message(s)\n", len(c.Messages))
	for _, comment := range c.Comments {
		fmt.Fprintf(&sb, "  Comment ......... %s\n", comment)
	}
	for i, m := range c.Messages {
		fmt.Fprintf(&sb, "\n  [%d] %s\n", i+1, strings.ToUpper(Kind(m)))
		for _, line := range strings.Split(strings.TrimRight(m.Humanize(), "\n"), "\n") {
			fmt.Fprintf(&sb, "  %s\n", line)
		}
	}
	return sb.String()
}
