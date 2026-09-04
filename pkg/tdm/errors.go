package tdm

import "errors"

// Sentinel errors from the message structure.
var (
	// ErrNotATDM indicates a file whose first keyword is not CCSDS_TDM_VERS.
	ErrNotATDM = errors.New("tdm: file is not a Tracking Data Message")

	// ErrNoSegment indicates a message with no segment. Clause 3.1.3 requires
	// a body of at least one, and each segment at least one Tracking Data
	// Record.
	ErrNoSegment = errors.New("tdm: a TDM must carry at least one segment")

	// ErrUnterminatedBlock indicates a META_START or DATA_START that never
	// closes.
	ErrUnterminatedBlock = errors.New("tdm: a delimited block was not closed")

	// ErrUnexpectedDelimiter indicates a delimiter where the structure of
	// clause 3.1.3 does not allow one.
	ErrUnexpectedDelimiter = errors.New("tdm: block delimiter is out of place")

	// ErrMissingDataSection indicates a metadata section with no data section
	// after it. Clause 3.3.1.3 pairs them: a segment is both or neither.
	ErrMissingDataSection = errors.New("tdm: every metadata section must be followed by a data section")

	// ErrUnknownKeyword indicates a keyword neither table 3-3 nor table 3-5
	// lists. Clause 3.3.1.7 says only the metadata keywords in table 3-3 may
	// be used, and 3.5.1 lists the data ones.
	ErrUnknownKeyword = errors.New("tdm: keyword is not one the tables allow")

	// ErrDuplicateKeyword indicates a metadata keyword given twice in one
	// section.
	ErrDuplicateKeyword = errors.New("tdm: metadata keyword appears more than once in a section")

	// ErrMissingTimeSystem indicates a metadata section with no TIME_SYSTEM.
	// It is the one metadata keyword table 3-3 marks mandatory, because
	// without it a timetag cannot be placed on any scale.
	ErrMissingTimeSystem = errors.New("tdm: every metadata section must give TIME_SYSTEM")

	// ErrMissingParticipant indicates a metadata section with no
	// PARTICIPANT_n. Table 3-3 requires at least one: a measurement with no
	// participant belongs to nobody.
	ErrMissingParticipant = errors.New("tdm: every metadata section must give at least one PARTICIPANT_n")

	// ErrParticipantIndex indicates a participant index outside 1 to 5, which
	// table 3-3 caps so that the other keywords referring to a participant by
	// number stay unambiguous.
	ErrParticipantIndex = errors.New("tdm: participant index must be 1 to 5")

	// ErrMalformedRecord indicates a Tracking Data Record whose value is not a
	// timetag followed by one measurement (clause 3.4.3).
	ErrMalformedRecord = errors.New("tdm: a tracking data record must be a timetag and one measurement")

	// ErrNoRecords indicates a data section with no Tracking Data Record.
	// Clause 3.1.3 requires a minimum of one.
	ErrNoRecords = errors.New("tdm: a data section must hold at least one tracking data record")
)
