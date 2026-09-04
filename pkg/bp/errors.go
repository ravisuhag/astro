package bp

import (
	"errors"

	"github.com/ravisuhag/astro/internal/cbor"
)

// Sentinel errors returned by the CBOR reader. A bundle is CBOR before it is
// anything else, so these come back from every decoder in the package.
//
// They are the values internal/cbor returns, re-exported under this package's
// names. pkg/bpsec reads the same octets with the same reader, and a caller
// that checks errors.Is against one of these must get the same answer whichever
// package handed the error back.
var (
	// ErrTruncated indicates the input ended in the middle of an item.
	ErrTruncated = cbor.ErrTruncated

	// ErrInvalidCBOR indicates a head byte CBOR does not define: one of the
	// reserved additional-information values 28 to 30, or an indefinite-length
	// marker on a type that cannot have one (RFC 8949 clause 3).
	ErrInvalidCBOR = cbor.ErrInvalidCBOR

	// ErrNotDeterministic indicates an argument encoded wider than it needed to
	// be. RFC 9171 clause 4.1 requires the core deterministic encoding of
	// RFC 8949 clause 4.2.1, which means the shortest head that holds the value.
	ErrNotDeterministic = cbor.ErrNotDeterministic

	// ErrWrongCBORType indicates a well-formed item of the wrong major type,
	// such as a text string where the bundle format wants an integer.
	ErrWrongCBORType = cbor.ErrWrongCBORType

	// ErrIndefiniteByteString indicates an indefinite-length byte string.
	// RFC 9171 clause 4.3.2 requires block-type-specific data to be a
	// definite-length byte string, and no other bundle field is a byte string.
	ErrIndefiniteByteString = cbor.ErrIndefiniteByteString

	// ErrExpectedBreak indicates a missing break stop code at the end of an
	// indefinite-length item.
	ErrExpectedBreak = cbor.ErrExpectedBreak
)

// Sentinel errors from the endpoint identifier and timestamp codecs.
var (
	// ErrUnknownURIScheme indicates a URI scheme code this package does not
	// implement. RFC 9171 clause 9.6 keeps an open registry; only the dtn and
	// ipn schemes it defines are handled here, because a scheme's
	// scheme-specific part cannot be parsed without its defining document.
	ErrUnknownURIScheme = errors.New("bpv7: unknown URI scheme code in an endpoint ID")

	// ErrMalformedEID indicates an endpoint ID whose shape is wrong: an array
	// of the wrong length, or a dtn scheme-specific part that is a number
	// other than zero (RFC 9171 clause 4.2.5.1).
	ErrMalformedEID = errors.New("bpv7: malformed endpoint ID")

	// ErrIPNComponentTooLarge indicates an ipn allocator identifier or node
	// number of 2^32 or more. RFC 9758 clause 6.3 bounds both so the
	// two-element encoding can pack them into one 64-bit number.
	ErrIPNComponentTooLarge = errors.New("bpv7: ipn allocator or node number does not fit in 32 bits")

	// ErrMalformedTimestamp indicates a creation timestamp that is not the
	// two-item array of RFC 9171 clause 4.2.7.
	ErrMalformedTimestamp = errors.New("bpv7: malformed creation timestamp")
)

// Sentinel errors from the block checksums.
var (
	// ErrInvalidCRCType indicates a checksum type code other than 0, 1 or 2.
	// RFC 9171 clause 4.2.1 defines those values "and no others".
	ErrInvalidCRCType = errors.New("bpv7: undefined CRC type code")

	// ErrWrongCRCWidth indicates a checksum field whose width does not match
	// its type: two octets for CRC-16, four for CRC-32C
	// (RFC 9171 clause 4.2.2).
	ErrWrongCRCWidth = errors.New("bpv7: CRC field width does not match the CRC type")

	// ErrCRCMismatch indicates a block whose checksum does not match its
	// contents.
	ErrCRCMismatch = errors.New("bpv7: CRC does not match the block contents")
)

// Sentinel errors from the primary block.
var (
	// ErrUnsupportedVersion indicates a version field other than 7. Version 6
	// is a different protocol (RFC 5050), not an earlier revision of this one.
	ErrUnsupportedVersion = errors.New("bpv7: bundle is not version 7")

	// ErrMalformedPrimaryBlock indicates a primary block that is not a
	// definite-length array of 8 to 11 items (RFC 9171 clause 4.3.1).
	ErrMalformedPrimaryBlock = errors.New("bpv7: malformed primary block")

	// ErrPrimaryBlockLengthMismatch indicates an array length that disagrees
	// with the fields the flags and CRC type say are present. Clause 4.3.1
	// fixes the length at 8, 9, 10 or 11 by exactly those two facts.
	ErrPrimaryBlockLengthMismatch = errors.New("bpv7: primary block length does not match its flags and CRC type")

	// ErrAdminRecordWantsReports indicates a bundle flagged as an
	// administrative record that also asks for status reports. Clause 4.2.3
	// requires every status report request flag to be zero on such a bundle.
	ErrAdminRecordWantsReports = errors.New("bpv7: an administrative record must not request status reports")

	// ErrAnonymousBundleFragmentable indicates a bundle sourced at the null
	// endpoint without the must-not-fragment flag. Clause 4.2.3 requires it,
	// because an anonymous bundle has no identity to reassemble against.
	ErrAnonymousBundleFragmentable = errors.New("bpv7: an anonymous bundle must set the must-not-fragment flag")

	// ErrAnonymousBundleWantsReports indicates a bundle sourced at the null
	// endpoint that asks for status reports. Clause 4.2.3 forbids it, for the
	// same reason: there is no identity for a report to name.
	ErrAnonymousBundleWantsReports = errors.New("bpv7: an anonymous bundle must not request status reports")
)

// Sentinel errors from canonical and extension blocks.
var (
	// ErrMalformedCanonicalBlock indicates a block that is not a
	// definite-length array of five or six items (RFC 9171 clause 4.3.2).
	ErrMalformedCanonicalBlock = errors.New("bpv7: malformed canonical block")

	// ErrCanonicalBlockLengthMismatch indicates an array length that disagrees
	// with the CRC type: five items without a checksum, six with one.
	ErrCanonicalBlockLengthMismatch = errors.New("bpv7: canonical block length does not match its CRC type")

	// ErrReservedBlockType indicates block type code 0, which
	// RFC 9171 clause 9.1 reserves.
	ErrReservedBlockType = errors.New("bpv7: block type code 0 is reserved")

	// ErrReservedBlockNumber indicates an extension block numbered 0 or 1.
	// Clause 4.1 gives 0 to the primary block and 1 to the payload.
	ErrReservedBlockNumber = errors.New("bpv7: block numbers 0 and 1 are reserved for the primary and payload blocks")

	// ErrPayloadBlockNumber indicates a payload block numbered anything but 1,
	// which clause 4.1 requires.
	ErrPayloadBlockNumber = errors.New("bpv7: the payload block must be block number 1")

	// ErrWrongBlockType indicates an extension accessor called on a block of
	// another type.
	ErrWrongBlockType = errors.New("bpv7: block is not the type this accessor reads")

	// ErrMalformedBlockData indicates block-type-specific data that does not
	// match the shape its block type defines.
	ErrMalformedBlockData = errors.New("bpv7: malformed block-type-specific data")

	// ErrHopLimitOutOfRange indicates a hop limit outside 1 to 255, the range
	// RFC 9171 clause 4.4.3 sets.
	ErrHopLimitOutOfRange = errors.New("bpv7: hop limit must be between 1 and 255")
)

// Sentinel errors from whole-bundle assembly and parsing.
var (
	// ErrNoPrimaryBlock indicates a bundle with no primary block.
	ErrNoPrimaryBlock = errors.New("bpv7: bundle has no primary block")

	// ErrNoPayloadBlock indicates a bundle with no blocks after the primary
	// one. Clause 4.1 requires at least a payload block.
	ErrNoPayloadBlock = errors.New("bpv7: bundle has no payload block")

	// ErrPayloadBlockCount indicates a bundle with more than one payload
	// block. Clause 4.1 allows exactly one.
	ErrPayloadBlockCount = errors.New("bpv7: bundle must have exactly one payload block")

	// ErrPayloadBlockNotLast indicates a payload block that is not the final
	// block, which clause 4.1 requires it to be.
	ErrPayloadBlockNotLast = errors.New("bpv7: the payload block must be the last block")

	// ErrDuplicateBlockNumber indicates two blocks sharing a number. Clause
	// 4.1 requires the number to identify a block within its bundle, and BPSec
	// blocks name their targets by it.
	ErrDuplicateBlockNumber = errors.New("bpv7: two blocks share a block number")

	// ErrDuplicateExtensionBlock indicates more than one Previous Node, Bundle
	// Age or Hop Count block. Clauses 4.4.1, 4.4.2 and 4.4.3 each allow one.
	ErrDuplicateExtensionBlock = errors.New("bpv7: more than one of an extension block that may appear once")

	// ErrMissingBundleAgeBlock indicates a bundle whose creation time is
	// unknown and which carries no Bundle Age block. Clause 4.4.2 requires one,
	// because nothing else says when the bundle expires.
	ErrMissingBundleAgeBlock = errors.New("bpv7: a bundle with an unknown creation time must carry a Bundle Age block")

	// ErrDefiniteLengthBundle indicates a bundle encoded as a definite-length
	// array. Clause 4.1 requires the indefinite-length form closed by a break.
	ErrDefiniteLengthBundle = errors.New("bpv7: bundle must be a CBOR indefinite-length array")

	// ErrTrailingBytes indicates octets after the bundle's break stop code.
	ErrTrailingBytes = errors.New("bpv7: bytes remain after the end of the bundle")
)

// Sentinel errors from fragmentation and reassembly.
var (
	// ErrMustNotFragment indicates an attempt to fragment a bundle whose
	// must-not-fragment flag is set (RFC 9171 clause 5.8).
	ErrMustNotFragment = errors.New("bpv7: bundle must not be fragmented")

	// ErrFragmentSizeTooSmall indicates a maximum payload size below one octet.
	// Clause 5.8 requires 0 < N < M for a split.
	ErrFragmentSizeTooSmall = errors.New("bpv7: fragment payload size must be at least one octet")

	// ErrNoFragments indicates reassembly with nothing to reassemble.
	ErrNoFragments = errors.New("bpv7: no fragments to reassemble")

	// ErrNotAFragment indicates a bundle without the fragment flag handed to
	// the reassembler.
	ErrNotAFragment = errors.New("bpv7: bundle is not a fragment")

	// ErrFragmentsDoNotMatch indicates fragments that do not share a source
	// node ID, creation timestamp and total length. Clause 5.9 keys reassembly
	// on those, so a mismatch means two different originals.
	ErrFragmentsDoNotMatch = errors.New("bpv7: fragments do not belong to the same bundle")

	// ErrFragmentPastEnd indicates a fragment claiming bytes beyond the total
	// application data unit length.
	ErrFragmentPastEnd = errors.New("bpv7: fragment extends past the total application data unit length")

	// ErrIncompleteReassembly indicates the fragments so far leave a gap, or
	// none of them covers offset zero.
	ErrIncompleteReassembly = errors.New("bpv7: fragments do not cover the whole application data unit")
)

// Sentinel errors from administrative records.
var (
	// ErrMalformedAdminRecord indicates a record that is not the two-item
	// array of RFC 9171 clause 6.1.
	ErrMalformedAdminRecord = errors.New("bpv7: malformed administrative record")

	// ErrUnknownAdminRecordType indicates a record type other than 1. Clause
	// 6.1 defines only the bundle status report for version 7.
	ErrUnknownAdminRecordType = errors.New("bpv7: unknown administrative record type")

	// ErrMalformedStatusReport indicates a status report whose arrays are the
	// wrong length: not four or six elements overall, fewer than four status
	// assertions, or a status item that is not one or two items
	// (RFC 9171 clause 6.1.1).
	ErrMalformedStatusReport = errors.New("bpv7: malformed bundle status report")

	// ErrStatusTimeWithoutAssertion indicates a status item carrying a time
	// while its indicator is false. Clause 6.1.1 allows the second element
	// only when the status is asserted.
	ErrStatusTimeWithoutAssertion = errors.New("bpv7: a status item carries a time without asserting the status")

	// ErrNotAnAdminRecord indicates a bundle whose payload was read as an
	// administrative record without the flag saying it is one.
	ErrNotAnAdminRecord = errors.New("bpv7: bundle payload is not an administrative record")
)

// errUnknownVectorStructure is returned by the vector harness when a fixture
// names a structure it does not know how to build. It is unexported because
// nothing outside the tests can reach it.
var errUnknownVectorStructure = errors.New("bpv7: vector names an unknown structure")
