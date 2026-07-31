// Package xtce implements the XML Telemetric and Command Exchange format,
// version 1.2, as published by the OMG and described by CCSDS 660.1-G-2.
//
// XTCE is the odd one out in this library. Every other package moves bytes to
// or from a spacecraft. XTCE moves no bytes at all: it is the mission
// database, the file that says what the bytes mean. A ground system loads one
// to learn a mission's parameters, how they are encoded, and which packets
// carry them.
//
// So there is no Encode here, and that is a decision rather than an omission.
// This package reads mission databases; writing them is a different job, done
// by database editors, and adding a writer would mean committing to a
// round-trip fidelity this package does not need.
//
// # What loading gives you
//
//	db, err := xtce.LoadFile("mission.xml")   // parse
//	err = db.Validate()                       // check the references resolve
//	param, err := db.FindParameter("/Root/Sub/Voltage")
//
// Validate is separate from Load on purpose. A database being edited often has
// references that do not resolve yet, and a loader that refused to parse those
// would be useless during authoring. Load tells you the file is well-formed
// XTCE; Validate tells you it is coherent.
//
// # No XSD validation
//
// The Go standard library has no XSD validator, and this package takes no
// dependencies. So "validation" here means semantic checks written in Go:
// references resolve, inheritance does not loop, names do not collide. A file
// that violates the XSD in a way these checks do not cover will load. If you
// need real schema validation, run xmllint over the file first.
//
// # Security
//
// Go's encoding/xml does not fetch DTDs and does not expand external entities,
// so the classic XML attacks — XXE, entity expansion bombs, network callbacks
// from a document — do not apply. What remains is plain resource abuse: a very
// large file, or one nested very deeply. MaxDocumentSize and MaxDepth bound
// both, and the depth check runs as a token scan before any decoding, so deep
// input is refused rather than recursed into.
//
// # This is the layer under an extraction engine
//
// The point of a mission database is decoding real packets with it. That
// engine is not here, but the model keeps what it will need: entry order
// within a container, bit sizes and locations, encoding parameters, and
// calibrators. See docs/guides/xtce.md.
package xtce

import "encoding/xml"

// Namespace is the XTCE 1.2 target namespace, from the schema's
// targetNamespace attribute.
//
// The date in the URI is the schema's publication, not a version of its own:
// this URI is what version 1.2 uses.
const Namespace = "http://www.omg.org/spec/XTCE/20180204"

// SpaceSystem is the root of a mission database and also its only recursive
// element: a SpaceSystem contains SpaceSystems.
//
// The tree is a namespace. A parameter's full name is the path of system names
// down to it, so /Spacecraft/Power/BusVoltage names BusVoltage inside Power
// inside Spacecraft.
type SpaceSystem struct {
	XMLName xml.Name `xml:"http://www.omg.org/spec/XTCE/20180204 SpaceSystem"`

	// Name identifies this system among its siblings.
	Name string `xml:"name,attr"`
	// ShortDescription is a one-line summary.
	ShortDescription string `xml:"shortDescription,attr"`
	// OperationalStatus is a mission-defined token.
	OperationalStatus string `xml:"operationalStatus,attr"`

	// LongDescription is free text.
	LongDescription string `xml:"LongDescription"`

	// Header carries versioning and authorship.
	Header *Header `xml:"Header"`

	// TelemetryMetaData describes what comes down.
	TelemetryMetaData *TelemetryMetaData `xml:"TelemetryMetaData"`
	// CommandMetaData describes what goes up.
	CommandMetaData *CommandMetaData `xml:"CommandMetaData"`

	// SubSystems are the nested SpaceSystems.
	SubSystems []*SpaceSystem `xml:"SpaceSystem"`

	// parent links back up the tree. It is set during Load rather than decoded,
	// because a name reference resolves by walking towards the root and the XML
	// has no way to express the link.
	parent *SpaceSystem
}

// Parent returns the enclosing SpaceSystem, or nil at the root.
func (s *SpaceSystem) Parent() *SpaceSystem { return s.parent }

// Header is the HeaderType of the schema: who produced this database and when.
type Header struct {
	Version        string `xml:"version,attr"`
	Date           string `xml:"date,attr"`
	Classification string `xml:"classification,attr"`
	Validation     string `xml:"validationStatus,attr"`
}

// TelemetryMetaData holds everything about the downlink.
//
// The schema also allows MessageSet, StreamSet and AlgorithmSet here. This
// package does not model them; see docs/pics/xtce-coverage.md.
type TelemetryMetaData struct {
	ParameterTypeSet *ParameterTypeSet `xml:"ParameterTypeSet"`
	ParameterSet     *ParameterSet     `xml:"ParameterSet"`
	ContainerSet     *ContainerSet     `xml:"ContainerSet"`
}

// CommandMetaData holds everything about the uplink.
//
// Only the skeleton is modeled: commands and their argument names. Command
// containers, verifiers, constraints and significance are not.
type CommandMetaData struct {
	ParameterTypeSet *ParameterTypeSet `xml:"ParameterTypeSet"`
	ParameterSet     *ParameterSet     `xml:"ParameterSet"`
	MetaCommandSet   *MetaCommandSet   `xml:"MetaCommandSet"`
}

// ParameterSet is the list of parameters a SpaceSystem defines.
type ParameterSet struct {
	Parameters []*Parameter `xml:"Parameter"`
}

// Parameter is one named piece of telemetry or command data.
//
// A Parameter says almost nothing itself. Its shape lives in the parameter
// type it points at, which is why an unresolvable ParameterTypeRef makes the
// parameter meaningless and Validate treats it as an error.
type Parameter struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	// ParameterTypeRef names the type, as a NameReference.
	ParameterTypeRef string `xml:"parameterTypeRef,attr"`
	InitialValue     string `xml:"initialValue,attr"`

	LongDescription string `xml:"LongDescription"`
}

// ContainerSet is the list of containers a SpaceSystem defines.
type ContainerSet struct {
	SequenceContainers []*SequenceContainer `xml:"SequenceContainer"`
}

// SequenceContainer describes a packet layout: an ordered list of entries,
// each naming a parameter or another container.
//
// Containers inherit. A container names a BaseContainer, and its own entries
// follow the base's — which is how a mission describes "a CCSDS packet" once
// and then twenty packet types that extend it.
type SequenceContainer struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	// Abstract marks a container that is only ever inherited from, never
	// matched against a packet on its own.
	Abstract bool `xml:"abstract,attr"`
	// IdlePattern fills unused space, written as a FixedIntegerValue.
	IdlePattern string `xml:"idlePattern,attr"`

	LongDescription string `xml:"LongDescription"`

	// EntryList is ordered, and the order is the wire order.
	EntryList EntryList `xml:"EntryList"`
	// BaseContainer names the container this one extends.
	BaseContainer *BaseContainer `xml:"BaseContainer"`
}

// BaseContainer points at the container being extended.
type BaseContainer struct {
	ContainerRef string `xml:"containerRef,attr"`
	// RestrictionCriteria says which values of the base's parameters select
	// this container. It is kept as raw XML: evaluating it is the extraction
	// engine's job, and modeling MatchCriteria in full is a large job for no
	// benefit here.
	RestrictionCriteria *RawXML `xml:"RestrictionCriteria"`
}

// RawXML holds an element this package parses but does not model.
//
// Keeping the bytes rather than dropping them means a later version can model
// the element without changing what Load accepts, and a caller who needs it
// today can parse it themselves.
type RawXML struct {
	Inner []byte `xml:",innerxml"`
}

// EntryKind says which kind of entry an Entry is.
type EntryKind int

const (
	// EntryParameterRef names a parameter.
	EntryParameterRef EntryKind = iota
	// EntryContainerRef names another container, whose entries are spliced in
	// at this position.
	EntryContainerRef
	// EntryOther is an entry kind this package parses but does not model:
	// ParameterSegmentRefEntry, ContainerSegmentRefEntry, StreamSegmentEntry,
	// IndirectParameterRefEntry, ArrayParameterRefEntry.
	EntryOther
)

// String names the kind.
func (k EntryKind) String() string {
	switch k {
	case EntryParameterRef:
		return "ParameterRefEntry"
	case EntryContainerRef:
		return "ContainerRefEntry"
	default:
		return "other entry"
	}
}

// Entry is one element of an EntryList.
//
// The schema makes EntryList a choice repeated without limit, so the entries
// are of mixed kinds in a meaningful order — the order they appear in the
// packet. Go's encoding/xml cannot preserve order across separate struct
// fields, so EntryList decodes itself and keeps one ordered slice. That is why
// this type exists rather than a struct per entry kind.
type Entry struct {
	Kind EntryKind
	// ElementName is the XML element this entry came from, which matters for
	// the kinds folded into EntryOther.
	ElementName string

	// Ref is the parameterRef or containerRef, whichever this entry carries.
	Ref string
	// ShortDescription is the entry's own description, not the referent's.
	ShortDescription string

	// LocationInContainerInBits places the entry when it does not simply
	// follow the previous one.
	LocationInContainerInBits *LocationInContainer
	// RepeatEntry repeats it.
	RepeatEntry *Repeat
	// IncludeCondition makes it conditional. Kept raw, like
	// RestrictionCriteria.
	IncludeCondition *RawXML
}

// LocationInContainer positions an entry within its container.
type LocationInContainer struct {
	// ReferenceLocation is one of containerStart, containerEnd, previousEntry
	// or nextEntry. It defaults to previousEntry.
	ReferenceLocation string `xml:"referenceLocation,attr"`
	// FixedValue is the offset in bits when it is a constant, which is the
	// usual case.
	FixedValue *int64 `xml:"FixedValue"`
	// DynamicValue and DiscreteLookupList are the non-constant forms, kept raw.
	DynamicValue       *RawXML `xml:"DynamicValue"`
	DiscreteLookupList *RawXML `xml:"DiscreteLookupList"`
}

// Repeat repeats an entry a number of times.
type Repeat struct {
	Count  *IntegerValue `xml:"Count"`
	Offset *IntegerValue `xml:"Offset"`
}

// IntegerValue is the schema's IntegerValueType: a number that may be fixed,
// read from another parameter, or looked up.
type IntegerValue struct {
	FixedValue         *int64  `xml:"FixedValue"`
	DynamicValue       *RawXML `xml:"DynamicValue"`
	DiscreteLookupList *RawXML `xml:"DiscreteLookupList"`
}

// MetaCommandSet is the list of commands a SpaceSystem defines.
type MetaCommandSet struct {
	MetaCommands []*MetaCommand `xml:"MetaCommand"`
}

// MetaCommand is one command, modeled as a skeleton: its name, what it
// extends, and its argument names and types.
//
// Everything that makes a command safe to send — verifiers, transmission
// constraints, significance, the command container's bit layout — is out of
// scope here and is not parsed into the model.
type MetaCommand struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	Abstract         bool   `xml:"abstract,attr"`

	LongDescription string `xml:"LongDescription"`

	BaseMetaCommand *BaseMetaCommand `xml:"BaseMetaCommand"`
	ArgumentList    *ArgumentList    `xml:"ArgumentList"`
}

// BaseMetaCommand points at the command being extended.
type BaseMetaCommand struct {
	MetaCommandRef string `xml:"metaCommandRef,attr"`
}

// ArgumentList is a command's arguments, in order.
type ArgumentList struct {
	Arguments []*Argument `xml:"Argument"`
}

// Argument is one command argument.
type Argument struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	ArgumentTypeRef  string `xml:"argumentTypeRef,attr"`
	InitialValue     string `xml:"initialValue,attr"`
}
