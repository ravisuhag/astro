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

	// ErrHeaderTooLarge indicates a secondary header beyond the 63 octets
	// CCSDS 133.0-B-2 allows, which pkg/spp enforces.
	ErrHeaderTooLarge = errors.New("PUS secondary header exceeds the 63-octet CCSDS limit")

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

	// ErrHeaderNotWordAligned indicates a secondary header whose size is not a
	// whole number of mission words (clauses 7.4.3.1l and 7.4.4.1g), when the
	// profile declares a word size to check against.
	ErrHeaderNotWordAligned = errors.New("PUS secondary header is not a whole number of mission words")
)
