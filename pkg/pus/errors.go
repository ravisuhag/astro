package pus

import "errors"

// Sentinel errors returned by the PUS codecs.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short for the PUS field being read")

	// ErrInvalidVersion indicates a PUS version other than 2, the value
	// ECSS-E-ST-70-41C clauses 7.4.3.1c and 7.4.4.1c require for PUS-C.
	ErrInvalidVersion = errors.New("invalid PUS version: this implementation speaks PUS-C (version 2)")

	// ErrInvalidProfile indicates mission-tailorable widths that cannot work.
	ErrInvalidProfile = errors.New("invalid mission profile")

	// ErrHeaderTooLarge indicates a mission profile whose secondary header is
	// wider than this package accepts. The bound is this package's own; CCSDS
	// 133.0-B-2 puts no upper limit on a Packet Secondary Header beyond the
	// packet data field maximum.
	ErrHeaderTooLarge = errors.New("PUS secondary header exceeds the 63-octet mission profile limit")

	// ErrUnknownMessageType indicates no codec is registered for a
	// (service, subtype) pair.
	ErrUnknownMessageType = errors.New("unknown PUS message type")

	// ErrDuplicateMessageType indicates two codecs registered for one
	// (service, subtype) pair.
	ErrDuplicateMessageType = errors.New("duplicate PUS message type registration")

	// ErrWrongMessageType indicates a decoder was handed another type's bytes.
	ErrWrongMessageType = errors.New("message type does not match the decoder")

	// ErrValueTooLarge indicates a value too wide for the field the profile
	// allocates to it.
	ErrValueTooLarge = errors.New("value does not fit the width the mission profile declares")

	// ErrUnsupportedTimeFormat indicates a time format this package cannot encode.
	ErrUnsupportedTimeFormat = errors.New("unsupported time format")

	// ErrInvalidSeverity indicates an event severity outside ST[05]'s four subtypes.
	ErrInvalidSeverity = errors.New("invalid event severity")

	// ErrTrailingBytes indicates octets left over after a fixed-size message
	// body. The PUS acceptance checks verify a request against its type's
	// structure, so a body longer than its type allows is rejected rather
	// than silently truncated.
	ErrTrailingBytes = errors.New("trailing octets after a fixed-size PUS message body")

	// ErrInvalidTimeWindow indicates a time-window type outside Table 8-5's
	// four values, or a "from time tag" greater than the "to time tag".
	// Clause 6.11.10.3d makes both a rejection.
	ErrInvalidTimeWindow = errors.New("invalid ST[11] time window")

	// ErrCapabilityNotSupported indicates a message carrying sub-schedule or
	// group identifiers while the profile declares that the subservice does
	// not support them (clause 6.11.4.1). Encoding them would produce octets
	// the peer is not expecting the fields for and so cannot parse.
	ErrCapabilityNotSupported = errors.New("mission profile does not declare this ST[11] capability")

	// ErrPacketLengthMismatch indicates an embedded telecommand packet whose
	// own length field disagrees with the octets supplied. In a scheduled
	// activity list that would desynchronise every activity after it.
	ErrPacketLengthMismatch = errors.New("embedded packet length field disagrees with the octets given")

	// ErrHeaderNotWordAligned indicates a secondary header whose size is not a
	// whole number of mission words (clauses 7.4.3.1l and 7.4.4.1g), when the
	// profile declares a word size to check against.
	ErrHeaderNotWordAligned = errors.New("PUS secondary header is not a whole number of mission words")
)
