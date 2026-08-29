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

	// ErrSecondaryHeaderFlagClear indicates a packet carries a SecondaryHeader
	// while its Secondary Header Flag is '0'. CCSDS 133.0-B-2 4.1.3.3.3.2 makes
	// the flag the sole signal of the header's presence, so the two must agree:
	// encoding such a packet would declare a data field longer than the octets
	// actually written.
	ErrSecondaryHeaderFlagClear = errors.New("secondary header is set but the secondary header flag is 0")

	// ErrSecondaryHeaderTwice indicates a packet was given both a parsed
	// SecondaryHeader and a Secondary Header Indicator saying the octets are
	// already in the user data. Honoring both would count the header twice in
	// the Packet Data Length (4.1.3.5.3) and write it twice on the wire.
	ErrSecondaryHeaderTwice = errors.New("secondary header supplied both as a parsed header and as user data octets")

	// ErrSecondaryHeaderExceedsDataField indicates the configured secondary
	// header decoder wants more octets than the packet data field holds. This
	// is a decoder/packet mismatch, not a truncated buffer.
	ErrSecondaryHeaderExceedsDataField = errors.New("secondary header size exceeds the packet data field")

	// ErrSecondaryHeaderTooSmall indicates the secondary header is less than 1 octet.
	ErrSecondaryHeaderTooSmall = errors.New("secondary header must be at least 1 octet")

	// ErrSecondaryHeaderSizeMismatch indicates SecondaryHeader.Encode() returned
	// a byte count different from SecondaryHeader.Size().
	ErrSecondaryHeaderSizeMismatch = errors.New("secondary header encoded size does not match Size()")

	// ErrIdleWithSecondaryHeader indicates an idle packet (APID 0x7FF) carries a
	// secondary header, which CCSDS 133.0-B-2 4.1.3.3.3.4 forbids.
	ErrIdleWithSecondaryHeader = errors.New("idle packet must not contain a secondary header")

	// ErrCRCValidationFailed indicates that the CRC validation of the packet failed.
	ErrCRCValidationFailed = errors.New("CRC validation failed: data integrity check failed")

	// ErrQoSUnsupported indicates a QoS Requirement was passed to SendPacket
	// but the transport does not implement QoSWriter, so the requested service
	// level cannot be honored. Sending anyway would silently downgrade the
	// packet, which is worse than refusing.
	ErrQoSUnsupported = errors.New("transport does not support QoS: it does not implement QoSWriter")
)
