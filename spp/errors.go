package spp

import "errors"

// Custom error definitions for the spp package.

var (
	// ErrInvalidHeader indicates an invalid primary or secondary header.
	ErrInvalidHeader = errors.New("invalid header: header does not conform to CCSDS standards")

	// ErrInvalidAPID indicates that the provided APID is out of range or invalid.
	ErrInvalidAPID = errors.New("invalid APID: must be in the range 0-2047")

	// ErrPacketTooLarge indicates that the packet size exceeds the allowable limit.
	ErrPacketTooLarge = errors.New("packet size exceeds the maximum allowable limit")

	// ErrDataTooShort indicates that the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode the packet")

	// ErrSecondaryHeaderMissing indicates that a required secondary header is missing.
	ErrSecondaryHeaderMissing = errors.New("secondary header flag is set but no secondary header is provided")

	// ErrCRCValidationFailed indicates that the CRC validation of the packet failed.
	ErrCRCValidationFailed = errors.New("CRC validation failed: data integrity check failed")
)
