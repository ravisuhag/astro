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
)
