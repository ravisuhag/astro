package cdm

import "errors"

// Sentinel errors from the message structure.
var (
	// ErrNotACDM indicates a file whose first keyword is not CCSDS_CDM_VERS.
	ErrNotACDM = errors.New("cdm: file is not a Conjunction Data Message")

	// ErrUnknownKeyword indicates a keyword none of the tables in section 3
	// lists.
	ErrUnknownKeyword = errors.New("cdm: keyword is not one the tables allow")

	// ErrDuplicateKeyword indicates a keyword given twice in one section.
	ErrDuplicateKeyword = errors.New("cdm: keyword appears more than once in a section")

	// ErrMissingKeyword indicates an obligatory keyword that is not present.
	ErrMissingKeyword = errors.New("cdm: obligatory keyword is missing")

	// ErrObjectOutOfOrder indicates an object section keyword before any
	// OBJECT keyword has said which object it belongs to.
	ErrObjectOutOfOrder = errors.New("cdm: object data appears before the OBJECT keyword that names it")

	// ErrObjectValue indicates an OBJECT value other than OBJECT1 or OBJECT2.
	// Table 3-3 gives those two as normative values, and a CDM describes
	// exactly two objects.
	ErrObjectValue = errors.New("cdm: OBJECT must be OBJECT1 or OBJECT2")

	// ErrObjectRepeated indicates the same object named twice.
	ErrObjectRepeated = errors.New("cdm: the same OBJECT appears twice")

	// ErrMissingObject indicates a message without both object sections. A
	// conjunction is between two objects; one section describes no conjunction.
	ErrMissingObject = errors.New("cdm: a CDM must describe both OBJECT1 and OBJECT2")
)
