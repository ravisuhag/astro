package cop

import "errors"

var (
	// ErrDataTooShort indicates the provided data is too short for CLCW decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode CLCW")

	// ErrInvalidCLCWType indicates the control word type is not 0.
	ErrInvalidCLCWType = errors.New("invalid CLCW: control word type must be 0")

	// ErrInvalidCLCWVersion indicates the CLCW version is not 0.
	ErrInvalidCLCWVersion = errors.New("invalid CLCW: version must be 00")

	// ErrFOPLockout indicates FOP-1 received a CLCW with the Lockout flag set.
	ErrFOPLockout = errors.New("FOP-1: lockout detected, ground must issue unlock")

	// ErrFOPWindowFull indicates the FOP-1 send window is full.
	ErrFOPWindowFull = errors.New("FOP-1: send window full, waiting for acknowledgment")

	// ErrFARMReject indicates FARM-1 rejected a frame (out of window).
	ErrFARMReject = errors.New("FARM-1: frame rejected, sequence number outside window")

	// ErrFARMLockout indicates FARM-1 is in lockout state.
	ErrFARMLockout = errors.New("FARM-1: lockout state, requires unlock command")
)
