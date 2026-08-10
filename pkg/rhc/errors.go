package rhc

import "errors"

// Sentinel errors returned by the compressor and decompressor.
var (
	// ErrDataTooShort indicates the coded bit stream ended before a field it
	// must contain.
	ErrDataTooShort = errors.New("data too short: the coded bit stream ended early")

	// ErrInvalidVectorLength indicates an input vector length outside the
	// 1 to 65535 of CCSDS 124.0-B-1 §3.2.
	ErrInvalidVectorLength = errors.New("invalid vector length: must be 1 to 65535 bits")

	// ErrInvalidPacketLength indicates an input packet that is not the
	// configured length.
	ErrInvalidPacketLength = errors.New("input packet is not the configured vector length")

	// ErrInvalidRobustness indicates a robustness level outside the 0 to 7 of
	// §3.3.2a.
	ErrInvalidRobustness = errors.New("invalid robustness level: must be 0 to 7")

	// ErrInvalidCount indicates a counter codeword the table of §5.2.2 does
	// not define.
	ErrInvalidCount = errors.New("invalid counter codeword")

	// ErrInvalidRunLength indicates a run-length codeword that runs past the
	// end of the vector it describes.
	ErrInvalidRunLength = errors.New("run-length codeword runs past the end of the vector")

	// ErrNotSynchronized indicates the decompressor has no state to work from.
	//
	// §3.3.2 forces the send mask and uncompressed flags to one while
	// t <= R_t, so a compressor's first output always carries a whole mask and
	// a whole input vector. Until one of those arrives — at the start, or
	// after Reset, or after losing more packets than the robustness level
	// covers — the decompressor cannot vouch for anything and says so rather
	// than guessing.
	ErrNotSynchronized = errors.New("decompressor is not synchronized: waiting for an uncompressed output vector")

	// ErrVectorLengthMismatch indicates a coded vector whose embedded length
	// disagrees with the configured one.
	ErrVectorLengthMismatch = errors.New("the coded vector length does not match the configured one")

	// ErrMaskUnavailable indicates a coded vector that needs mask state the
	// decompressor does not have.
	ErrMaskUnavailable = errors.New("the mask is not known: an earlier output vector carrying it was lost")
)
