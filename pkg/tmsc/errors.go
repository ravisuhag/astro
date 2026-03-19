package tmsc

import "errors"

var (
	// ErrDataTooShort indicates the provided CADU is too short to contain the ASM.
	ErrDataTooShort = errors.New("provided data is too short to unwrap")

	// ErrSyncMarkerMismatch indicates the CADU does not start with the expected ASM.
	ErrSyncMarkerMismatch = errors.New("attached sync marker mismatch")

	// ErrInvalidDataLength indicates the data length does not match the RS code parameters.
	ErrInvalidDataLength = errors.New("data length does not match RS code parameters")

	// ErrInvalidInterleaveDepth indicates an unsupported interleaving depth.
	ErrInvalidInterleaveDepth = errors.New("unsupported interleaving depth: must be 1, 2, 3, 4, 5, or 8")

	// ErrUncorrectable indicates the codeword has more errors than the code can correct.
	ErrUncorrectable = errors.New("uncorrectable errors: exceeds RS correction capability")
)
