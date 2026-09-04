package ndm

import "errors"

// Sentinel errors from the line syntax of CCSDS 502.0-B-3 clause 7.3 and 7.4,
// which the other three navigation standards restate in their own words.
var (
	// ErrControlCharacter indicates a control character in the file. Clause
	// 7.3.4 allows only printable ASCII and blanks, and calls out TAB by name.
	ErrControlCharacter = errors.New("ndm: only printable ASCII and blanks are allowed")

	// ErrLineTooLong indicates a line past the 254-character limit of clause
	// 7.3.2. The limit does not apply to every message: clause 7.3.3 exempts
	// the OCM, so the check is opt-in.
	ErrLineTooLong = errors.New("ndm: line exceeds 254 characters")

	// ErrNotAnAssignment indicates a line that is not 'keyword = value' where
	// one was required.
	ErrNotAnAssignment = errors.New("ndm: line is not a keyword = value assignment")

	// ErrEmptyKeyword indicates a line starting with an equals sign.
	ErrEmptyKeyword = errors.New("ndm: assignment has no keyword")

	// ErrKeywordNotUppercase indicates a lowercase keyword. Clause 7.4.4
	// requires uppercase and forbids blanks inside a keyword.
	ErrKeywordNotUppercase = errors.New("ndm: keyword must be uppercase and contain no blanks")

	// ErrEmptyValue indicates a mandatory keyword with nothing assigned to it
	// (clause 7.5.1).
	ErrEmptyValue = errors.New("ndm: mandatory keyword has an empty value")
)

// Sentinel errors from the value formats of clause 7.5.
var (
	// ErrNotAnInteger indicates a value that is not the signed decimal string
	// clause 7.5.4 defines.
	ErrNotAnInteger = errors.New("ndm: value is not an integer")

	// ErrIntegerOutOfRange indicates an integer outside the signed 32-bit
	// range clause 7.5.4 sets.
	ErrIntegerOutOfRange = errors.New("ndm: integer is outside the range clause 7.5.4 allows")

	// ErrNotANumber indicates a value that is neither the fixed-point form of
	// clause 7.5.6 nor the floating-point form of clause 7.5.7.
	ErrNotANumber = errors.New("ndm: value is not a number")

	// ErrBlankInValue indicates whitespace inside a numeric value or a time
	// string, which clause 7.5.8 forbids.
	ErrBlankInValue = errors.New("ndm: numeric values and time strings must not contain blanks")

	// ErrTooManyDigits indicates a number written with more than the 16
	// digits clauses 7.5.6 and 7.5.7 allow.
	ErrTooManyDigits = errors.New("ndm: number has more than 16 digits")

	// ErrNotAnEpoch indicates a time that is neither of the two forms clause
	// 7.5.10 defines.
	ErrNotAnEpoch = errors.New("ndm: value is not a CCSDS ASCII time code A or B")

	// ErrMalformedUnits indicates a unit suffix that is not a bracketed string
	// separated from the value by at least one blank (clause 7.7.1.1).
	ErrMalformedUnits = errors.New("ndm: malformed unit suffix")

	// ErrUnitsNotApplicable indicates the literal '[n/a]', which clause
	// 7.7.1.3 forbids appearing in a message.
	ErrUnitsNotApplicable = errors.New("ndm: '[n/a]' must not appear as a unit")
)

// Sentinel errors from the shared header.
var (
	// ErrNoVersionLine indicates a file whose first non-blank line is not the
	// CCSDS_*_VERS assignment. Clause 7.3.6 requires the first header line to
	// come first.
	ErrNoVersionLine = errors.New("ndm: the first non-blank line must be the version keyword")

	// ErrWrongMessageType indicates a version keyword naming a different
	// message type from the one being read.
	ErrWrongMessageType = errors.New("ndm: file is a different navigation message type")

	// ErrMissingHeaderField indicates a mandatory header keyword that is not
	// present.
	ErrMissingHeaderField = errors.New("ndm: mandatory header keyword is missing")

	// ErrUnknownHeaderKeyword indicates a keyword in the header section that
	// the standard's header table does not list. Each standard says only the
	// keywords in its table may be used.
	ErrUnknownHeaderKeyword = errors.New("ndm: keyword is not one the header table allows")

	// ErrDuplicateHeaderKeyword indicates a header keyword given twice.
	ErrDuplicateHeaderKeyword = errors.New("ndm: header keyword appears more than once")
)
