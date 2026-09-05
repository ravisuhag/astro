package xtce

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
)

// MaxDocumentSize is the largest document Load will read, 64 MiB.
//
// XTCE sets no limit (a mission database is as big as the mission) so this
// is a resource bound, not a conformance rule. Real databases run to a few
// megabytes; 64 MiB leaves room for the large ones while keeping a hostile
// file from being read into memory unbounded. LoadWithLimit overrides it.
const MaxDocumentSize = 64 << 20

// MaxDepth is the deepest element nesting Load will accept.
//
// SpaceSystem contains SpaceSystem, so the decoder recurses as it descends and
// a file nested thousands deep would exhaust the stack before any of this
// package's code ran. The check therefore happens before decoding, by counting
// depth over the token stream. Real databases nest a handful of levels.
const MaxDepth = 100

// Load reads a mission database from r.
//
// It parses and checks the shape of the document: well-formed XML, an XTCE 1.2
// SpaceSystem at the root, within the size and depth limits. It does not check
// that references resolve, call Validate for that. The two are separate
// because a database under construction usually has references that do not
// resolve yet, and refusing to read those would make the loader useless during
// authoring.
func Load(r io.Reader) (*SpaceSystem, error) {
	return LoadWithLimit(r, MaxDocumentSize)
}

// LoadWithLimit is Load with a different size cap, in octets.
func LoadWithLimit(r io.Reader, maxSize int64) (*SpaceSystem, error) {
	if maxSize <= 0 {
		maxSize = MaxDocumentSize
	}

	// Read one octet past the limit so that hitting it exactly is
	// distinguishable from exceeding it.
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%w: limit is %d octets", ErrInputTooLarge, maxSize)
	}

	if err := checkDepth(data, MaxDepth); err != nil {
		return nil, err
	}
	if err := checkRoot(data); err != nil {
		return nil, err
	}

	var system SpaceSystem
	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&system); err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return nil, fmt.Errorf("%w: document is empty", ErrNotSpaceSystem)
		case errors.Is(err, ErrInvalidValue):
			return nil, err
		default:
			var syntaxErr *xml.SyntaxError
			if errors.As(err, &syntaxErr) {
				return nil, fmt.Errorf("%w: %w", ErrMalformedXML, syntaxErr)
			}
			// checkRoot already proved the root element is a SpaceSystem in
			// the right namespace, so what remains is a value somewhere in
			// the document that its schema type cannot hold. An attribute
			// that should be a number and is not, say. Before the root check
			// this case was misreported as ErrNotSpaceSystem.
			return nil, fmt.Errorf("%w: %w", ErrInvalidValue, err)
		}
	}

	if system.Name == "" {
		return nil, fmt.Errorf("%w: root SpaceSystem has no name", ErrNotSpaceSystem)
	}

	linkParents(&system, nil)
	return &system, nil
}

// LoadFile reads a mission database from a file.
func LoadFile(path string) (*SpaceSystem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only

	system, err := Load(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return system, nil
}

// checkDepth walks the token stream counting nesting, and stops at the first
// element deeper than the limit.
//
// It runs before Decode rather than after, which is the whole point: checking
// a decoded tree would mean the recursion had already happened.
func checkDepth(data []byte, maxDepth int) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			var syntaxErr *xml.SyntaxError
			if errors.As(err, &syntaxErr) {
				return fmt.Errorf("%w: %w", ErrMalformedXML, syntaxErr)
			}
			return err
		}

		switch token.(type) {
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return fmt.Errorf("%w: limit is %d levels", ErrTooDeep, maxDepth)
			}
		case xml.EndElement:
			depth--
		}
	}
}

// checkRoot scans to the first element and confirms it is a SpaceSystem in
// the XTCE 1.2 namespace.
//
// Decode would reject a wrong root too, but as an untyped error that cannot
// be told apart from a bad value further into the document. Settling the root
// question here is what lets the Decode error handling say ErrInvalidValue
// for everything that is not a syntax error.
func checkRoot(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: document is empty", ErrNotSpaceSystem)
		}
		if err != nil {
			var syntaxErr *xml.SyntaxError
			if errors.As(err, &syntaxErr) {
				return fmt.Errorf("%w: %w", ErrMalformedXML, syntaxErr)
			}
			return err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			// Prolog: declarations, comments, processing instructions.
			continue
		}
		if start.Name.Space != Namespace || start.Name.Local != "SpaceSystem" {
			return fmt.Errorf("%w: the root is %s in namespace %q",
				ErrNotSpaceSystem, start.Name.Local, start.Name.Space)
		}
		return nil
	}
}

// linkParents fills in the parent pointers the XML cannot carry.
//
// Name references resolve by searching towards the root, so every SpaceSystem
// has to know its parent. Nothing in the document says so (the nesting is the
// only evidence) which is why this runs once after decoding.
//
// Containers get the same treatment, for the same reason. A reference written
// inside a container resolves relative to the SpaceSystem that defines the
// container, not the one doing the lookup, which matters the moment a
// container inherits from one in another system.
func linkParents(system *SpaceSystem, parent *SpaceSystem) {
	system.parent = parent

	if system.TelemetryMetaData != nil && system.TelemetryMetaData.ContainerSet != nil {
		for _, container := range system.TelemetryMetaData.ContainerSet.SequenceContainers {
			container.owner = system
		}
	}

	for _, child := range system.SubSystems {
		linkParents(child, system)
	}
}
