package ldc

import "errors"

// Sentinel errors returned by the compressor and decompressor.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short: the coded bit stream ended early")

	// ErrInvalidBlockSize indicates a block size outside the four CCSDS
	// 121.0-B-3 §3.1.6 allows.
	ErrInvalidBlockSize = errors.New("invalid block size: must be 8, 16, 32 or 64 samples")

	// ErrInvalidResolution indicates a sample resolution outside 1 to 32 bits,
	// per CCSDS 121.0-B-3 §3.1.6.
	ErrInvalidResolution = errors.New("invalid sample resolution: must be 1 to 32 bits")

	// ErrInvalidReferenceInterval indicates a reference sample interval outside
	// 1 to 4096 blocks, per CCSDS 121.0-B-3 §4.3.
	ErrInvalidReferenceInterval = errors.New("invalid reference sample interval: must be 1 to 4096 blocks")

	// ErrRestrictedNotAllowed indicates the restricted code option set at a
	// resolution above 4 bits. CCSDS 121.0-B-3 §5.2.1.1 allows it only when
	// n <= 4.
	ErrRestrictedNotAllowed = errors.New("the restricted code option set requires a resolution of 4 bits or fewer")

	// ErrSampleOutOfRange indicates a sample that does not fit the configured
	// resolution.
	ErrSampleOutOfRange = errors.New("sample does not fit the configured resolution")

	// ErrInvalidOptionID indicates an option identifier the code option table
	// of CCSDS 121.0-B-3 §5.2 does not define at this resolution.
	ErrInvalidOptionID = errors.New("invalid code option identifier")

	// ErrInvalidWordSize indicates an output word size outside 1 to 8 octets,
	// per CCSDS 121.0-B-3 §7.2.1.2.
	ErrInvalidWordSize = errors.New("invalid output word size: must be 1 to 8 octets")

	// ErrTruncatedFile indicates a compressed file shorter than its 12-octet
	// header.
	ErrTruncatedFile = errors.New("compressed file is shorter than its header")

	// ErrReservedFieldSet indicates a reserved header field that is not zero,
	// which CCSDS 121.0-B-3 table 7-1 requires it to be.
	ErrReservedFieldSet = errors.New("a reserved header field is not zero")

	// ErrUnsupportedPredictor indicates a predictor type this package does not
	// implement, such as the application-specific one.
	ErrUnsupportedPredictor = errors.New("unsupported predictor type")

	// ErrUnsupportedMapper indicates a mapper type this package does not
	// implement.
	ErrUnsupportedMapper = errors.New("unsupported mapper type")

	// ErrTooManySamples indicates a sample count past what the file header's
	// 48-bit field can hold.
	ErrTooManySamples = errors.New("too many samples for the file header")

	// ErrSampleCountMismatch indicates a coded stream that did not yield the
	// number of samples its header promised.
	ErrSampleCountMismatch = errors.New("the coded data did not yield the promised number of samples")
)
