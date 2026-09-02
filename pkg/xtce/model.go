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
// # Extracting packets
//
// The point of a mission database is decoding real packets with it, which is
// what Layout and Extract do:
//
//	layout, err := db.LayoutOf("/Sat/Housekeeping")
//	packet, err := layout.Extract(octets)
//	temp, ok := packet.Get("Temp")
//
// A Layout is a container flattened into the fields a packet of that shape
// carries, with inheritance worked through and a bit offset and width settled
// for each. It depends only on the database, so it is built once per packet
// type and reused.
//
// When you do not know what a packet is, Match searches: it follows each
// container whose RestrictionCriteria the packet satisfies and takes the
// deepest one that fits. See docs/content/protocols/xtce/index.md.
package xtce

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

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
	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`

	// Header carries versioning and authorship.
	Header *Header `xml:"http://www.omg.org/spec/XTCE/20180204 Header"`

	// TelemetryMetaData describes what comes down.
	TelemetryMetaData *TelemetryMetaData `xml:"http://www.omg.org/spec/XTCE/20180204 TelemetryMetaData"`
	// CommandMetaData describes what goes up.
	CommandMetaData *CommandMetaData `xml:"http://www.omg.org/spec/XTCE/20180204 CommandMetaData"`

	// SubSystems are the nested SpaceSystems.
	SubSystems []*SpaceSystem `xml:"http://www.omg.org/spec/XTCE/20180204 SpaceSystem"`

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
// package does not model them; see docs/content/conformance/xtce.md.
type TelemetryMetaData struct {
	ParameterTypeSet *ParameterTypeSet `xml:"http://www.omg.org/spec/XTCE/20180204 ParameterTypeSet"`
	ParameterSet     *ParameterSet     `xml:"http://www.omg.org/spec/XTCE/20180204 ParameterSet"`
	ContainerSet     *ContainerSet     `xml:"http://www.omg.org/spec/XTCE/20180204 ContainerSet"`
}

// CommandMetaData holds everything about the uplink.
//
// Only the skeleton is modeled: commands and their argument names. Command
// containers, verifiers, constraints and significance are not.
type CommandMetaData struct {
	ParameterTypeSet *ParameterTypeSet `xml:"http://www.omg.org/spec/XTCE/20180204 ParameterTypeSet"`
	ParameterSet     *ParameterSet     `xml:"http://www.omg.org/spec/XTCE/20180204 ParameterSet"`
	MetaCommandSet   *MetaCommandSet   `xml:"http://www.omg.org/spec/XTCE/20180204 MetaCommandSet"`
}

// ParameterSet is the list of parameters a SpaceSystem defines.
type ParameterSet struct {
	Parameters []*Parameter `xml:"http://www.omg.org/spec/XTCE/20180204 Parameter"`
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

	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`
}

// ContainerSet is the list of containers a SpaceSystem defines.
type ContainerSet struct {
	SequenceContainers []*SequenceContainer `xml:"http://www.omg.org/spec/XTCE/20180204 SequenceContainer"`
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

	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`

	// EntryList is ordered, and the order is the wire order.
	EntryList EntryList `xml:"http://www.omg.org/spec/XTCE/20180204 EntryList"`
	// BaseContainer names the container this one extends.
	BaseContainer *BaseContainer `xml:"http://www.omg.org/spec/XTCE/20180204 BaseContainer"`

	// owner is the SpaceSystem that defines this container, set during Load.
	// References written inside the container resolve from there, which is not
	// necessarily the system doing the lookup.
	owner *SpaceSystem
}

// Owner returns the SpaceSystem that defines this container, or nil for one
// that did not come from Load.
func (c *SequenceContainer) Owner() *SpaceSystem { return c.owner }

// BaseContainer points at the container being extended.
type BaseContainer struct {
	ContainerRef string `xml:"containerRef,attr"`
	// RestrictionCriteria says which values of the base's parameters select
	// this container. It is what makes container inheritance more than a way
	// of sharing a header: it is the test a packet has to pass for this
	// container to be the right reading of it.
	RestrictionCriteria *RestrictionCriteria `xml:"http://www.omg.org/spec/XTCE/20180204 RestrictionCriteria"`
}

// RestrictionCriteria is the schema's RestrictionCriteriaType: a
// MatchCriteria, plus the option of naming the container that must follow
// this one in the stream.
type RestrictionCriteria struct {
	MatchCriteria

	// NextContainer names a container that must follow this one. Deciding it
	// needs the stream rather than the packet, so Match does not evaluate it.
	NextContainer *ContainerRef `xml:"http://www.omg.org/spec/XTCE/20180204 NextContainer"`
}

// ContainerRef names a container.
type ContainerRef struct {
	ContainerRef string `xml:"containerRef,attr"`
}

// MatchCriteria is a condition over parameter values: one comparison, a list
// of them that must all hold, an arbitrary boolean expression, or an escape to
// an external algorithm.
//
// The schema makes these a choice, so exactly one is set. The last two are
// kept raw. A BooleanExpression is a tree of ANDs and ORs that would need its
// own evaluator, and a CustomAlgorithm is by definition outside the file.
type MatchCriteria struct {
	Comparison        *Comparison     `xml:"http://www.omg.org/spec/XTCE/20180204 Comparison"`
	ComparisonList    *ComparisonList `xml:"http://www.omg.org/spec/XTCE/20180204 ComparisonList"`
	BooleanExpression *RawXML         `xml:"http://www.omg.org/spec/XTCE/20180204 BooleanExpression"`
	CustomAlgorithm   *RawXML         `xml:"http://www.omg.org/spec/XTCE/20180204 CustomAlgorithm"`
}

// ComparisonList is a set of comparisons that must all hold. The schema calls
// the "and" between them implicit.
type ComparisonList struct {
	Comparisons []Comparison `xml:"http://www.omg.org/spec/XTCE/20180204 Comparison"`
}

// Comparison tests one parameter against a value.
//
// The value is written as text whatever the parameter's type, and the schema
// says how to read it: a number is base ten unless it starts with 0x, 0o or
// 0b, an enumeration is compared by its label, and a binary value is hex.
type Comparison struct {
	// ParameterRef names the parameter to test.
	ParameterRef string `xml:"parameterRef,attr"`
	// Value is what to test it against, as text.
	Value string `xml:"value,attr"`
	// ComparisonOperator is one of ==, !=, <, <=, > and >=. It defaults to ==.
	ComparisonOperator string `xml:"comparisonOperator,attr"`
	// UseCalibratedValue compares against the engineering value rather than
	// the raw one. The schema's default is true, so it is a pointer: false has
	// to be distinguishable from absent.
	UseCalibratedValue *bool `xml:"useCalibratedValue,attr"`
	// Instance selects an earlier or later occurrence of the parameter in the
	// stream. It defaults to 0, meaning this packet's value.
	Instance int64 `xml:"instance,attr"`
}

// Operator returns the comparison operator, applying the schema's default.
func (c *Comparison) Operator() string {
	if c.ComparisonOperator == "" {
		return "=="
	}
	return c.ComparisonOperator
}

// Calibrated reports whether the comparison is against the engineering value,
// applying the schema's default of true.
func (c *Comparison) Calibrated() bool {
	return c.UseCalibratedValue == nil || *c.UseCalibratedValue
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
	// RestrictionCriteria. Evaluating it is the caller's job.
	IncludeCondition *RawXML
}

// LocationInContainer positions an entry within its container.
type LocationInContainer struct {
	// ReferenceLocation is one of containerStart, containerEnd, previousEntry
	// or nextEntry. It defaults to previousEntry; read it through
	// ReferenceLocationOrDefault.
	ReferenceLocation string `xml:"referenceLocation,attr"`
	// FixedValue is the offset in bits when it is a constant, which is the
	// usual case.
	FixedValue *FixedInteger `xml:"http://www.omg.org/spec/XTCE/20180204 FixedValue"`
	// DynamicValue and DiscreteLookupList are the non-constant forms, kept raw.
	DynamicValue       *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 DynamicValue"`
	DiscreteLookupList *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 DiscreteLookupList"`
}

// ReferenceLocationOrDefault returns the anchor the offset is measured from,
// applying the schema's default of previousEntry.
func (l *LocationInContainer) ReferenceLocationOrDefault() string {
	if l.ReferenceLocation == "" {
		return "previousEntry"
	}
	return l.ReferenceLocation
}

// Repeat repeats an entry a number of times.
type Repeat struct {
	Count  *IntegerValue `xml:"http://www.omg.org/spec/XTCE/20180204 Count"`
	Offset *IntegerValue `xml:"http://www.omg.org/spec/XTCE/20180204 Offset"`
}

// IntegerValue is the schema's IntegerValueType: a number that may be fixed,
// read from another parameter, or looked up.
type IntegerValue struct {
	FixedValue         *FixedInteger `xml:"http://www.omg.org/spec/XTCE/20180204 FixedValue"`
	DynamicValue       *RawXML       `xml:"http://www.omg.org/spec/XTCE/20180204 DynamicValue"`
	DiscreteLookupList *RawXML       `xml:"http://www.omg.org/spec/XTCE/20180204 DiscreteLookupList"`
}

// FixedInteger is the schema's FixedIntegerValueType: a union of a decimal
// integer with the hex, octal and binary spellings, so 18, 0x12, 0o22 and
// 0b10010 all mean the same number.
//
// It exists because encoding/xml reads an int64 field with strconv in base
// ten, which makes a legal 0x2A reject the entire document. Every field fed
// by the union uses this type instead.
type FixedInteger int64

// Int64 returns the value as a plain integer.
func (f FixedInteger) Int64() int64 { return int64(f) }

// UnmarshalXML reads the union's element form, <FixedValue>0x2A</FixedValue>.
func (f *FixedInteger) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var text string
	if err := d.DecodeElement(&text, &start); err != nil {
		return err
	}
	value, err := parseFixedInteger(text)
	if err != nil {
		return err
	}
	*f = FixedInteger(value)
	return nil
}

// UnmarshalXMLAttr reads the union's attribute form.
func (f *FixedInteger) UnmarshalXMLAttr(attr xml.Attr) error {
	value, err := parseFixedInteger(attr.Value)
	if err != nil {
		return err
	}
	*f = FixedInteger(value)
	return nil
}

// parseFixedInteger reads one FixedIntegerValueType spelling.
//
// The base comes from the prefix — 0x, 0o or 0b — and is ten otherwise. A
// leading zero alone does not mean octal: the schema's octal member requires
// the 0o prefix, so 010 is ten, which is why this is not strconv's base 0.
func parseFixedInteger(text string) (int64, error) {
	trimmed := strings.TrimSpace(text)

	digits := trimmed
	sign := ""
	if strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
		sign, digits = digits[:1], digits[1:]
	}

	base := 10
	if len(digits) > 2 {
		switch digits[:2] {
		case "0x", "0X":
			base, digits = 16, digits[2:]
		case "0o", "0O":
			base, digits = 8, digits[2:]
		case "0b", "0B":
			base, digits = 2, digits[2:]
		}
	}

	value, err := strconv.ParseInt(sign+digits, base, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a FixedIntegerValue", ErrInvalidValue, text)
	}
	return value, nil
}

// MetaCommandSet is the list of commands a SpaceSystem defines.
//
// The schema makes it a choice of three element kinds. MetaCommand is the
// definition; MetaCommandRef includes a command defined elsewhere; and
// BlockMetaCommand groups several commands into one. All three are kept, so
// none of a mission's commands vanishes from the model silently.
type MetaCommandSet struct {
	MetaCommands []*MetaCommand `xml:"http://www.omg.org/spec/XTCE/20180204 MetaCommand"`
	// MetaCommandRefs are commands included by reference from another
	// SpaceSystem. The reference is kept but not resolved.
	MetaCommandRefs []*MetaCommandRef `xml:"http://www.omg.org/spec/XTCE/20180204 MetaCommandRef"`
	// BlockMetaCommands are ordered groupings of commands. The name is
	// modeled; the steps are kept raw.
	BlockMetaCommands []*BlockMetaCommand `xml:"http://www.omg.org/spec/XTCE/20180204 BlockMetaCommand"`
}

// MetaCommandRef includes a command defined in another SpaceSystem. The
// schema types the element as a NameReferenceType, so the reference is the
// element's text.
type MetaCommandRef struct {
	Ref string `xml:",chardata"`
}

// BlockMetaCommand is an ordered grouping of commands sent as one.
//
// Only the identity is modeled. The step list — which commands, with which
// argument values — is kept raw, so a caller who needs it can parse it and a
// later version can model it without changing what Load accepts.
type BlockMetaCommand struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`

	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`

	// MetaCommandStepList holds the block's steps, raw.
	MetaCommandStepList *RawXML `xml:"http://www.omg.org/spec/XTCE/20180204 MetaCommandStepList"`
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

	LongDescription string `xml:"http://www.omg.org/spec/XTCE/20180204 LongDescription"`

	BaseMetaCommand *BaseMetaCommand `xml:"http://www.omg.org/spec/XTCE/20180204 BaseMetaCommand"`
	ArgumentList    *ArgumentList    `xml:"http://www.omg.org/spec/XTCE/20180204 ArgumentList"`
}

// BaseMetaCommand points at the command being extended.
type BaseMetaCommand struct {
	MetaCommandRef string `xml:"metaCommandRef,attr"`
	// ArgumentAssignmentList fixes some of the base command's arguments to
	// specific values, which is how a general command is narrowed into a
	// specific one. Dropping it would make the derived command look identical
	// to its base.
	ArgumentAssignmentList *ArgumentAssignmentList `xml:"http://www.omg.org/spec/XTCE/20180204 ArgumentAssignmentList"`
}

// ArgumentAssignmentList is the set of argument values a derived command
// fixes.
type ArgumentAssignmentList struct {
	Assignments []ArgumentAssignment `xml:"http://www.omg.org/spec/XTCE/20180204 ArgumentAssignment"`
}

// ArgumentAssignment fixes one argument of the base command to a value.
type ArgumentAssignment struct {
	Name  string `xml:"argumentName,attr"`
	Value string `xml:"argumentValue,attr"`
}

// ArgumentList is a command's arguments, in order.
type ArgumentList struct {
	Arguments []*Argument `xml:"http://www.omg.org/spec/XTCE/20180204 Argument"`
}

// Argument is one command argument.
type Argument struct {
	Name             string `xml:"name,attr"`
	ShortDescription string `xml:"shortDescription,attr"`
	ArgumentTypeRef  string `xml:"argumentTypeRef,attr"`
	InitialValue     string `xml:"initialValue,attr"`
}
