package csts

import "errors"

// Sentinel errors returned by the CSTS framework codec.
//
// The BER-level errors come from internal/ber and are not repeated here: a
// caller checking for a malformed encoding compares against those.
var (
	// ErrMalformedPDU indicates a framework PDU whose structure is not what
	// annex F3.15 defines: a tag outside the context class, or an alternative
	// whose content does not match its type.
	ErrMalformedPDU = errors.New("csts: malformed framework PDU")

	// ErrUnknownOperation indicates a CHOICE tag that is not one of the
	// twenty alternatives of annex F3.15. The twenty are the whole CHOICE, so
	// a twenty-first is a message this implementation cannot claim to have
	// understood rather than one it can pass along.
	ErrUnknownOperation = errors.New("csts: not an operation the framework defines")

	// ErrTrailingContent indicates octets after the structure ended. A SEQUENCE
	// with an extra element is a different message from the one expected.
	ErrTrailingContent = errors.New("csts: unexpected octets after the structure ended")

	// ErrMissingField indicates a PDU built without the message its type
	// names, or an operation missing a field the ASN.1 makes mandatory.
	ErrMissingField = errors.New("csts: a mandatory field is missing")

	// ErrMalformedHeader indicates a standard invocation or return header
	// that is not the SEQUENCE annex F3.3 defines.
	ErrMalformedHeader = errors.New("csts: malformed standard operation header")

	// ErrMalformedCredentials indicates a Credentials CHOICE that is neither
	// 'unused' nor 'used'.
	ErrMalformedCredentials = errors.New("csts: malformed credentials")

	// ErrCredentialsLength indicates used credentials outside the 8 to 256
	// octets the SIZE constraint of annex F3.3 allows.
	ErrCredentialsLength = errors.New("csts: credentials must be 8 to 256 octets")

	// ErrInvalidProcedureName indicates a procedure-name whose role is not
	// one of the three the CHOICE allows, or a secondary procedure with
	// instance number zero — annex F3.3 types it IntPos, which starts at 1.
	ErrInvalidProcedureName = errors.New("csts: invalid procedure name")

	// ErrMalformedDiagnostic indicates a Diagnostic CHOICE outside the five
	// alternatives of annex F3.3.
	ErrMalformedDiagnostic = errors.New("csts: malformed diagnostic")

	// ErrAppellationLength indicates an appellation outside the 1 to 128
	// characters the SIZE constraint allows.
	ErrAppellationLength = errors.New("csts: an appellation must be 1 to 128 characters")

	// ErrIdentifierLength indicates an authority identifier or port name
	// outside its SIZE constraint: 3 to 16 characters for an authority, 1 to
	// 128 for a logical port name.
	ErrIdentifierLength = errors.New("csts: identifier is outside the length the standard allows")

	// ErrIdentifierHasBlank indicates a blank in an IdentifierString, which
	// annex F3.3 defines as VisibleString (FROM (ALL EXCEPT " ")).
	ErrIdentifierHasBlank = errors.New("csts: an identifier must not contain a blank")

	// ErrInvalidVersion indicates a BIND version number of zero. Annex F3.5
	// types it IntPos, which starts at 1.
	ErrInvalidVersion = errors.New("csts: a version number must be at least 1")

	// ErrIntegerRange indicates an integer outside the range its ASN.1 type
	// allows. IntUnsigned is 0 to 2^32-1 and IntPos is 1 to 2^32-1, and Go's
	// int64 holds values neither of them permits.
	ErrIntegerRange = errors.New("csts: integer is outside the range its type allows")
)
