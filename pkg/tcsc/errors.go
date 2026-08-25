package tcsc

import "errors"

var (
	// ErrDataTooShort indicates the provided CLTU is too short to contain
	// the start sequence, at least one codeblock, and tail sequence.
	ErrDataTooShort = errors.New("provided data is too short to unwrap")

	// ErrStartSequenceMismatch indicates the CLTU does not start with the
	// expected start sequence.
	ErrStartSequenceMismatch = errors.New("CLTU start sequence mismatch")

	// ErrTailSequenceMismatch indicates the CLTU does not end with the
	// expected tail sequence.
	//
	// Deprecated: UnwrapCLTU now terminates on the tail sequence or on the
	// first codeblock that fails to decode, per CCSDS 231.0-B-4, so an
	// exact tail match is no longer required and this error is not returned.
	ErrTailSequenceMismatch = errors.New("CLTU tail sequence mismatch")

	// ErrInvalidCLTULength indicates the CLTU body length (excluding start
	// and tail sequences) is not a multiple of the codeblock size (8 bytes).
	//
	// Deprecated: UnwrapCLTU now tolerates trailing octets after the last
	// decodable codeblock, per CCSDS 231.0-B-4, and no longer returns this
	// error.
	ErrInvalidCLTULength = errors.New("CLTU body length is not a multiple of codeblock size")

	// ErrUncorrectable indicates that a codeblock contains more errors
	// than the BCH code can correct (more than 1 bit error).
	ErrUncorrectable = errors.New("uncorrectable error in codeblock: exceeds BCH correction capability")

	// ErrEmptyData indicates that empty data was provided for encoding.
	ErrEmptyData = errors.New("empty data provided")

	// ErrInvalidInfoLength indicates that BCHEncode was called with a slice
	// that is not exactly 7 bytes (InfoBytes).
	ErrInvalidInfoLength = errors.New("BCH info must be exactly 7 bytes")

	// ErrInvalidPLOP indicates an unknown Physical Layer Operations
	// Procedure was requested (only PLOP-1 and PLOP-2 exist).
	ErrInvalidPLOP = errors.New("invalid PLOP: must be PLOP1 or PLOP2")
)
