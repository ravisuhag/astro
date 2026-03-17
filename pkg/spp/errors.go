package spp

import "errors"

var (
	// ErrInvalidHeader indicates an invalid primary or secondary header.
	ErrInvalidHeader = errors.New("invalid header: header does not conform to CCSDS standards")

	// ErrInvalidVersion indicates the version number is not 0 (CCSDS v1).
	ErrInvalidVersion = errors.New("invalid version: must be 0 for CCSDS v1")

	// ErrInvalidType indicates the packet type is not 0 (TM) or 1 (TC).
	ErrInvalidType = errors.New("invalid packet type: must be 0 (TM) or 1 (TC)")

	// ErrInvalidAPID indicates that the provided APID is out of range.
	ErrInvalidAPID = errors.New("invalid APID: must be in the range 0-2047")

	// ErrInvalidSequenceFlags indicates the sequence flags are out of range.
	ErrInvalidSequenceFlags = errors.New("invalid sequence flags: must be in the range 0-3")

	// ErrInvalidSequenceCount indicates the sequence count is out of range.
	ErrInvalidSequenceCount = errors.New("invalid sequence count: must be in the range 0-16383")

	// ErrEmptyPacket indicates a packet has neither a secondary header nor user data (CCSDS C1/C2).
	ErrEmptyPacket = errors.New("packet must contain a secondary header or user data")

	// ErrNilPacket indicates a nil packet was provided.
	ErrNilPacket = errors.New("packet must not be nil")

	// ErrPacketTooLarge indicates that the packet size exceeds the allowable limit.
	ErrPacketTooLarge = errors.New("packet length must be between 7 and 65542 octets")

	// ErrDataTooShort indicates that the provided data is too short for decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode the packet")

	// ErrPacketLengthMismatch indicates that the packet data field size does not match the packet length.
	ErrPacketLengthMismatch = errors.New("packet data field size does not match packet length")

	// ErrSecondaryHeaderMissing indicates that a required secondary header is missing.
	ErrSecondaryHeaderMissing = errors.New("secondary header flag is set but no secondary header is provided")

	// ErrSecondaryHeaderTooSmall indicates the secondary header is less than 1 octet.
	ErrSecondaryHeaderTooSmall = errors.New("secondary header must be at least 1 octet")

	// ErrSecondaryHeaderTooLarge indicates the secondary header exceeds 63 octets.
	ErrSecondaryHeaderTooLarge = errors.New("secondary header must not exceed 63 octets")

	// ErrCRCValidationFailed indicates that the CRC validation of the packet failed.
	ErrCRCValidationFailed = errors.New("CRC validation failed: data integrity check failed")
)
