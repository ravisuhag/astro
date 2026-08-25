package pxsc

import "errors"

// Sentinel errors returned by the Proximity-1 coding and synchronization codecs.
var (
	// ErrDataTooShort indicates the input is shorter than the fields it must contain.
	ErrDataTooShort = errors.New("data too short for the PLTU field being read")

	// ErrInvalidASM indicates the Attached Sync Marker is not the FAF320 of
	// CCSDS 211.2-B-3 §3.2.3.2.
	ErrInvalidASM = errors.New("invalid attached sync marker: expected FAF320")

	// ErrCRCMismatch indicates the attached CRC-32 did not verify, so the
	// PLTU must be discarded per §3.6.
	ErrCRCMismatch = errors.New("CRC-32 mismatch: the PLTU is corrupt")

	// ErrFrameTooLarge indicates a transfer frame beyond the configured
	// maximum PLTU length.
	ErrFrameTooLarge = errors.New("transfer frame exceeds the maximum length")

	// ErrEmptyFrame indicates an attempt to wrap nothing in a PLTU.
	ErrEmptyFrame = errors.New("cannot build a PLTU around an empty transfer frame")

	// ErrInvalidLength indicates a symbol stream that is not a whole number of
	// coded bits, so it cannot be decoded.
	ErrInvalidLength = errors.New("symbol stream length is not a whole number of coded input bits")
)
