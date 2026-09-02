package bp

import "errors"

// Sentinel errors returned by the Bundle Protocol codecs.
var (
	// ErrDataTooShort indicates the input ended before a field it must contain.
	ErrDataTooShort = errors.New("data too short for the bundle field being read")

	// ErrInvalidVersion indicates a bundle version other than 6. BPv7 uses a
	// CBOR encoding that is wire-incompatible and is not implemented here.
	ErrInvalidVersion = errors.New("invalid bundle version: this package speaks version 6")

	// ErrDictionaryOffset indicates a scheme or scheme-specific-part offset
	// pointing outside the dictionary.
	ErrDictionaryOffset = errors.New("endpoint offset points outside the dictionary")

	// ErrInvalidEndpointID indicates an endpoint ID that cannot be parsed.
	ErrInvalidEndpointID = errors.New("invalid endpoint ID")

	// ErrMissingPayload indicates a bundle with no payload block, which
	// RFC 5050 clause 4.5.2 requires.
	ErrMissingPayload = errors.New("bundle has no payload block")

	// ErrMultiplePayloads indicates more than one payload block.
	ErrMultiplePayloads = errors.New("bundle has more than one payload block")

	// ErrNoLastBlock indicates a bundle whose final block does not carry the
	// last-block flag.
	ErrNoLastBlock = errors.New("the final block does not have the last-block flag set")

	// ErrFragmentFlags indicates fragment fields present without the fragment
	// flag, or the reverse.
	ErrFragmentFlags = errors.New("fragment fields do not match the fragment flag")

	// ErrNotFragment indicates a reassembly operation on a bundle that is not
	// a fragment.
	ErrNotFragment = errors.New("bundle is not a fragment")

	// ErrIncompleteFragments indicates a reassembly attempt with gaps still
	// unfilled.
	ErrIncompleteFragments = errors.New("fragments do not cover the whole application data unit")

	// ErrMismatchedFragments indicates fragments from different bundles.
	ErrMismatchedFragments = errors.New("fragments do not belong to the same bundle")

	// ErrCannotFragment indicates a bundle whose "must not be fragmented"
	// flag forbids the operation.
	ErrCannotFragment = errors.New("bundle must not be fragmented")

	// ErrAdminRecordFlags indicates an administrative record requesting
	// custody transfer or status reports, which clause 4.2 forbids.
	ErrAdminRecordFlags = errors.New("an administrative record must not request custody transfer or status reports")

	// ErrInvalidPriority indicates a class of service of 3, which RFC 5050
	// Clause 4.2 reserves.
	ErrInvalidPriority = errors.New("class of service 3 is reserved")

	// ErrAnonymousSource indicates a bundle with source dtn:none that
	// requests custody transfer or omits the "must not be fragmented" flag.
	// Clause 4.2: an anonymous bundle is not uniquely identifiable, so it can
	// neither take custody nor be fragmented.
	ErrAnonymousSource = errors.New("an anonymous bundle must not request custody and must set the no-fragment flag")

	// ErrTrailingBytes indicates data left over after a complete bundle.
	// DecodeBundle refuses it rather than silently dropping octets; use
	// DecodeBundleN when bundles arrive back to back in one buffer.
	ErrTrailingBytes = errors.New("data continues past the end of the bundle")

	// ErrNotAdminRecord indicates an administrative-record operation on a
	// bundle that is not one.
	ErrNotAdminRecord = errors.New("bundle payload is not an administrative record")

	// ErrInvalidRecordType indicates an unknown administrative record type.
	ErrInvalidRecordType = errors.New("invalid administrative record type")

	// ErrBlockTooLarge indicates a block length beyond the configured
	// maximum. A block length is an SDNV reaching 2^64, so a cap is what
	// stops one corrupt bundle exhausting memory.
	ErrBlockTooLarge = errors.New("block length exceeds the maximum this decoder accepts")

	// ErrInvalidECOS indicates an Extended Class of Service block that
	// contradicts CCSDS 734.2-B-1 annex C.
	ErrInvalidECOS = errors.New("invalid Extended Class of Service block")
)
