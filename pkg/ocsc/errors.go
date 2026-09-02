package ocsc

import "errors"

// Sentinel errors returned by the optical coding and synchronization codecs.
var (
	// ErrDataTooShort indicates the input is shorter than the fields it must contain.
	ErrDataTooShort = errors.New("data too short for the optical field being read")

	// ErrInvalidASM indicates the Attached Sync Marker is not the 1ACFFC1D of
	// CCSDS 142.0-B-1 clause 3.3.2.
	ErrInvalidASM = errors.New("invalid attached sync marker: expected 1ACFFC1D")

	// ErrInvalidCodeRate indicates a code rate outside the three of table 3-1.
	ErrInvalidCodeRate = errors.New("invalid code rate: must be 1/3, 1/2, or 2/3")

	// ErrCRCMismatch indicates a code block whose attached CRC-32 did not verify.
	ErrCRCMismatch = errors.New("CRC-32 mismatch: the code block is corrupt")

	// ErrInvalidBlockLength indicates a code block that is not the k-hat
	// length its code rate requires.
	ErrInvalidBlockLength = errors.New("code block is not the length its code rate requires")

	// ErrInvalidTermination indicates a code block whose two termination bits
	// are not zero, contrary to clause 3.7.
	ErrInvalidTermination = errors.New("termination bits are not zero")

	// ErrEmptyFrame indicates an attempt to mark an empty transfer frame.
	ErrEmptyFrame = errors.New("cannot attach a sync marker to an empty transfer frame")

	// ErrFrameTooLong indicates a transfer frame longer than the 65536-octet
	// bound of the managed parameters (CCSDS 142.0-B-1 clause 5.2, table 5-1).
	ErrFrameTooLong = errors.New("transfer frame exceeds the 65536-octet managed-parameter bound")

	// ErrConditionerClosed indicates use of a Conditioner after Close: Clause 3.4.2.1.1
	// permits zero fill only at transmission closure, so a closed stream is over.
	ErrConditionerClosed = errors.New("the conditioner is closed: transmission closure has been declared")
)
