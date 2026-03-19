package tmsc

import "errors"

var (
	// ErrDataTooShort indicates the provided CADU is too short to contain the ASM.
	ErrDataTooShort = errors.New("provided data is too short to unwrap")

	// ErrSyncMarkerMismatch indicates the CADU does not start with the expected ASM.
	ErrSyncMarkerMismatch = errors.New("attached sync marker mismatch")
)
