package tcsc

import "errors"

var (
	// ErrDataTooShort indicates the provided CLTU is too short to contain
	// the start sequence, at least one codeblock, and tail sequence.
	ErrDataTooShort = errors.New("provided data is too short to unwrap")

	// ErrStartSequenceMismatch indicates the CLTU does not start with the
	// expected start sequence.
	ErrStartSequenceMismatch = errors.New("CLTU start sequence mismatch")

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
