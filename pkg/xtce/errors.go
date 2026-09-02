package xtce

import "errors"

// Sentinel errors returned when loading, resolving and validating a mission
// database.
var (
	// ErrNotSpaceSystem indicates a document whose root element is not an XTCE
	// SpaceSystem in the 1.2 namespace.
	ErrNotSpaceSystem = errors.New("root element is not an XTCE 1.2 SpaceSystem")

	// ErrInputTooLarge indicates a document larger than the configured limit.
	// The limit is not in the standard: XTCE puts no ceiling on a file, so a
	// cap is what stops one hostile document exhausting memory.
	ErrInputTooLarge = errors.New("XTCE document exceeds the maximum size this loader accepts")

	// ErrTooDeep indicates element nesting beyond the configured depth. The
	// SpaceSystem tree is recursive, so an adversarial file can nest deeply
	// enough to exhaust the stack during decoding.
	ErrTooDeep = errors.New("XTCE document nests deeper than this loader accepts")

	// ErrMalformedXML indicates a document that is not well-formed XML.
	ErrMalformedXML = errors.New("malformed XML")

	// ErrInvalidValue indicates a document that is well-formed XML with the
	// right root, but which spells a value in a way its schema type cannot
	// hold — a FixedValue that is not a number, say. It is distinct from
	// ErrNotSpaceSystem so a bad value in the middle of a real database is not
	// misreported as the wrong kind of document.
	ErrInvalidValue = errors.New("XTCE document has a value that cannot be read as its schema type")

	// ErrInvalidEncoding indicates a data encoding attribute — encoding,
	// bitOrder or byteOrder — whose value is not one of the schema's legal
	// enumeration members.
	ErrInvalidEncoding = errors.New("data encoding attribute is not a legal value")

	// ErrUnresolvedReference indicates a name reference that names nothing.
	ErrUnresolvedReference = errors.New("name reference does not resolve")

	// ErrContainerCycle indicates a BaseContainer chain that leads back to
	// itself, which would make container inheritance infinite.
	ErrContainerCycle = errors.New("container inheritance forms a cycle")

	// ErrDuplicateName indicates two things with the same name in one set of
	// one SpaceSystem, which the schema's uniqueness keys forbid.
	ErrDuplicateName = errors.New("duplicate name within a SpaceSystem")

	// ErrInvalidReference indicates a name reference whose text does not match
	// the NameReferenceType pattern.
	ErrInvalidReference = errors.New("malformed name reference")

	// ErrNotFound indicates a lookup that found nothing.
	ErrNotFound = errors.New("not found")

	// ErrDynamicSize indicates a field whose position or width depends on the
	// contents of the packet rather than on the database alone, which a layout
	// built ahead of any packet cannot settle.
	ErrDynamicSize = errors.New("the field's size or position depends on the packet contents")

	// ErrUnsupportedEntry indicates an entry this package can parse but cannot
	// place in a layout.
	ErrUnsupportedEntry = errors.New("entry cannot be placed in a layout")

	// ErrUnsupportedEncoding indicates a data encoding this package can parse
	// but cannot decode a value from.
	ErrUnsupportedEncoding = errors.New("data encoding is not supported")

	// ErrPacketTooShort indicates a packet that ends before a field the layout
	// says it carries.
	ErrPacketTooShort = errors.New("packet is too short for the container layout")

	// ErrUnsupportedCalibrator indicates a calibrator this package can parse
	// but cannot evaluate.
	ErrUnsupportedCalibrator = errors.New("calibrator is not supported")

	// ErrNoMatch indicates a packet that satisfies no container's restriction
	// criteria. It is a normal thing for a ground station to see.
	ErrNoMatch = errors.New("no container matches the packet")

	// ErrUnsupportedCriteria indicates restriction criteria this package can
	// parse but cannot evaluate against a single packet.
	ErrUnsupportedCriteria = errors.New("restriction criteria are not supported")

	// ErrInvalidComparison indicates a Comparison whose value attribute cannot
	// be read as the parameter's type.
	ErrInvalidComparison = errors.New("comparison value cannot be parsed")

	// ErrInvalidMathOperation indicates a MathOperationCalibrator whose
	// postfix expression does not evaluate: an unbalanced stack, an operator
	// outside the schema's set, or an operand outside an operator's domain.
	ErrInvalidMathOperation = errors.New("math operation cannot be evaluated")
)
