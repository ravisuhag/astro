package cbor

import "errors"

// Sentinel errors returned by the reader. Callers that want their own names
// for these can alias them: the identity is what errors.Is compares.
var (
	// ErrTruncated indicates the input ended in the middle of an item.
	ErrTruncated = errors.New("cbor: input ended before the item did")

	// ErrInvalidCBOR indicates a head byte CBOR does not define: one of the
	// reserved additional-information values 28 to 30, or an indefinite-length
	// marker on a type that cannot have one (RFC 8949 clause 3).
	ErrInvalidCBOR = errors.New("cbor: malformed head")

	// ErrNotDeterministic indicates an argument encoded wider than it needed
	// to be. The core deterministic encoding of RFC 8949 clause 4.2.1 requires
	// the shortest head that holds the value.
	ErrNotDeterministic = errors.New("cbor: argument is not in shortest form")

	// ErrWrongCBORType indicates a well-formed item of the wrong major type,
	// such as a text string where the caller wants an integer.
	ErrWrongCBORType = errors.New("cbor: item is not the type this field needs")

	// ErrIndefiniteByteString indicates an indefinite-length byte string where
	// the caller requires a definite-length one.
	ErrIndefiniteByteString = errors.New("cbor: byte string must be definite-length")

	// ErrExpectedBreak indicates a missing break stop code at the end of an
	// indefinite-length item.
	ErrExpectedBreak = errors.New("cbor: expected a break stop code")

	// ErrNestingTooDeep indicates an item nested past the depth limit Skip
	// enforces. Arbitrary input must not be able to drive the walker into
	// unbounded recursion.
	ErrNestingTooDeep = errors.New("cbor: item nesting is too deep")
)
