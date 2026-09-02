package ltp

import "errors"

// Sentinel errors returned by the LTP codecs and session machines.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short for the LTP field being read")

	// ErrInvalidVersion indicates a segment version other than 0, the only
	// value RFC 5326 clause 3.1 defines.
	ErrInvalidVersion = errors.New("invalid LTP version: only version 0 is defined")

	// ErrUndefinedSegmentType indicates one of the type codes RFC 5326 clause 3.1.2
	// marks undefined: 5, 6, 10 and 11.
	ErrUndefinedSegmentType = errors.New("undefined LTP segment type code")

	// ErrWrongSegmentType indicates a decoder was handed another type's bytes.
	ErrWrongSegmentType = errors.New("segment type does not match the decoder")

	// ErrTooManyExtensions indicates more than 15 header or trailer
	// extensions, which the 4-bit counts of clause 3.1.4 cannot describe.
	ErrTooManyExtensions = errors.New("more than 15 extensions in a header or trailer")

	// ErrInvalidSerialNumber indicates a checkpoint or report serial number of
	// zero where clause 3.2.1 and clause 3.2.2 forbid it.
	ErrInvalidSerialNumber = errors.New("checkpoint and report serial numbers must not be zero")

	// ErrInvalidBounds indicates a report segment whose upper bound is below
	// its lower bound.
	ErrInvalidBounds = errors.New("report segment upper bound is below its lower bound")

	// ErrInvalidClaim indicates a reception claim of zero length, or one
	// reaching past the report's upper bound (clause 3.2.2).
	ErrInvalidClaim = errors.New("invalid reception claim")

	// ErrInvalidReasonCode indicates a cancel reason code in the reserved
	// range 06 to FF (clause 3.2.4).
	ErrInvalidReasonCode = errors.New("invalid cancel reason code")

	// ErrSessionClosed indicates an operation on a session that has already
	// finished or been cancelled.
	ErrSessionClosed = errors.New("session is closed")

	// ErrBlockTooLarge indicates a data segment naming a block position past
	// the receiver's configured maximum. A segment offset is an SDNV and can
	// reach 2^64, so a cap is what stops one bad segment exhausting memory.
	ErrBlockTooLarge = errors.New("data segment reaches past the maximum block size")

	// ErrRedGreenOrder indicates green-part data below a red-part offset, or
	// red-part data above a green-part offset, which clause 3.2.4 calls MISCOLORED.
	ErrRedGreenOrder = errors.New("miscolored block: red and green parts overlap out of order")
)
