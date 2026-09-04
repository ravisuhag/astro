package ber

import "errors"

// Sentinel errors returned by the codec.
//
// pkg/sle aliases these, so a caller that has been comparing against
// sle.ErrInvalidTag keeps working: the two names are the same value.
var (
	// ErrDataTooShort indicates the input ended before a field it must
	// contain.
	ErrDataTooShort = errors.New("data too short for the field being read")

	// ErrInvalidTag indicates a BER tag the decoder did not expect here.
	ErrInvalidTag = errors.New("unexpected BER tag")

	// ErrInvalidLength indicates a BER length that is malformed or beyond the
	// bytes available.
	ErrInvalidLength = errors.New("invalid BER length")

	// ErrIndefiniteLength indicates the indefinite-length form on a primitive
	// encoding, which X.690 clause 8.1.3.2 forbids. Constructed
	// indefinite-length encodings are accepted.
	ErrIndefiniteLength = errors.New("indefinite BER length on a primitive encoding")

	// ErrInvalidObjectIdentifier indicates an OBJECT IDENTIFIER that cannot
	// be encoded or decoded.
	ErrInvalidObjectIdentifier = errors.New("invalid object identifier")

	// ErrLengthTooLarge indicates a BER length beyond the configured maximum.
	// A length field can name far more than any real PDU contains, so a cap
	// is what stops one hostile message exhausting memory.
	ErrLengthTooLarge = errors.New("BER length exceeds the maximum this decoder accepts")

	// ErrIntegerOverflow indicates a BER INTEGER too large for the Go type
	// receiving it.
	ErrIntegerOverflow = errors.New("BER integer does not fit")
)
