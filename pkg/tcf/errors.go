package tcf

import "errors"

var (
	// ErrDataTooShort indicates the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode time code")

	// ErrInvalidPField indicates the P-field does not conform to CCSDS 301.0-B-4.
	ErrInvalidPField = errors.New("invalid P-field: does not conform to CCSDS 301.0-B-4")

	// ErrInvalidTimeCodeID indicates an unrecognized time code identification.
	ErrInvalidTimeCodeID = errors.New("invalid time code ID: must be a recognized CCSDS code type")

	// ErrInvalidCoarseOctets indicates the coarse time octet count is out of range.
	ErrInvalidCoarseOctets = errors.New("invalid coarse time: must be 1-4 basic octets (up to 7 with extension)")

	// ErrInvalidFineOctets indicates the fine time octet count is out of range.
	ErrInvalidFineOctets = errors.New("invalid fine time: must be 0-3 basic octets (up to 6 with extension)")

	// ErrInvalidDaySegment indicates the day count is negative or out of range.
	ErrInvalidDaySegment = errors.New("invalid day segment: day count out of range")

	// ErrInvalidMilliseconds indicates the milliseconds-of-day value is out of range.
	ErrInvalidMilliseconds = errors.New("invalid milliseconds: must be in range 0-86399999")

	// ErrInvalidCalendarTime indicates a calendar field is out of range.
	ErrInvalidCalendarTime = errors.New("invalid calendar time: field value out of range")

	// ErrInvalidASCIIFormat indicates the ASCII time string does not match the expected format.
	ErrInvalidASCIIFormat = errors.New("invalid ASCII time code: format does not match CCSDS Type A or Type B")

	// ErrEpochRequired indicates a custom epoch is required for Level 2 codes but was not provided.
	ErrEpochRequired = errors.New("agency-defined epoch required for Level 2 time code")

	// ErrOverflow indicates the time value exceeds the representable range.
	ErrOverflow = errors.New("time value exceeds representable range for the configured octet width")
)
