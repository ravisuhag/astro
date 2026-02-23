package spp

import "errors"

// Custom error definitions for the spp package.

var (
	// ErrInvalidHeader indicates an invalid primary or secondary header.
	ErrInvalidHeader = errors.New("invalid header: header does not conform to CCSDS standards")

	// ErrInvalidAPID indicates that the provided APID is out of range or invalid.
	ErrInvalidAPID = errors.New("invalid APID: must be in the range 0-2047")

	// ErrAPIDAlreadyReserved indicates that the APID is already reserved.
	ErrAPIDAlreadyReserved = errors.New("APID is already reserved")

	// ErrPacketTooLarge indicates that the packet size exceeds the allowable limit.
	ErrPacketTooLarge = errors.New("packet length must be between 7 and 65542 octets")

	// ErrDataTooShort indicates that the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode the packet")

	// ErrPacketLengthMismatch indicates that the packet data field size does not match the packet length.
	ErrPacketLengthMismatch = errors.New("packet data field size does not match packet length")

	// ErrSecondaryHeaderMissing indicates that a required secondary header is missing.
	ErrSecondaryHeaderMissing = errors.New("secondary header flag is set but no secondary header is provided")

	// ErrCRCValidationFailed indicates that the CRC validation of the packet failed.
	ErrCRCValidationFailed = errors.New("CRC validation failed: data integrity check failed")
)
