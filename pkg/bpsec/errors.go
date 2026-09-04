package bpsec

import "errors"

// Sentinel errors from the Abstract Security Block codec.
var (
	// ErrNoTargets indicates a security block with an empty target list.
	// RFC 9172 clause 3.6 requires at least one entry.
	ErrNoTargets = errors.New("bpsec: security block has no targets")

	// ErrDuplicateTarget indicates the same block number twice in one target
	// list, which clause 3.6 forbids.
	ErrDuplicateTarget = errors.New("bpsec: security block names the same target twice")

	// ErrResultCountMismatch indicates a security results array whose length
	// differs from the target list. Clause 3.6 requires one set of results per
	// target, ordered the same way.
	ErrResultCountMismatch = errors.New("bpsec: security block has a different number of result sets than targets")

	// ErrReservedContextFlag indicates a security context flag other than bit
	// 0. Clause 3.6 reserves every other bit and requires a writer to leave
	// them zero.
	ErrReservedContextFlag = errors.New("bpsec: security context flags set a reserved bit")

	// ErrParametersFlagDisagrees indicates a security block whose parameters
	// flag does not match whether parameters are present. Clause 3.6 makes the
	// field mandatory when the bit is set and forbidden when it is not.
	ErrParametersFlagDisagrees = errors.New("bpsec: security context flags disagree with the parameters present")

	// ErrMalformedASB indicates an Abstract Security Block whose CBOR does not
	// have the shape of clause 3.6.
	ErrMalformedASB = errors.New("bpsec: malformed abstract security block")

	// ErrTrailingBytes indicates octets after the end of an Abstract Security
	// Block.
	ErrTrailingBytes = errors.New("bpsec: bytes remain after the end of the abstract security block")

	// ErrNotASecurityBlock indicates a block whose type code is neither 11 nor
	// 12, handed to something that reads security blocks
	// (RFC 9172 clause 11.1).
	ErrNotASecurityBlock = errors.New("bpsec: block is not a BIB or a BCB")

	// ErrWrongSecurityBlockType indicates a BCB handed to the integrity
	// service, or a BIB handed to the confidentiality service.
	ErrWrongSecurityBlockType = errors.New("bpsec: security block is not the type this operation processes")
)

// Sentinel errors from the target rules of RFC 9172 clauses 3.7 to 3.9.
var (
	// ErrTargetNotInBundle indicates a target block number that no block in
	// the bundle carries. Clause 3.6 requires every entry to name a block that
	// exists.
	ErrTargetNotInBundle = errors.New("bpsec: security block targets a block the bundle does not have")

	// ErrIntegrityTargetsSecurityBlock indicates a BIB targeting a BIB or a
	// BCB, which clause 3.7 forbids.
	ErrIntegrityTargetsSecurityBlock = errors.New("bpsec: a BIB must not target another security block")

	// ErrConfidentialityTargetsBCB indicates a BCB targeting another BCB,
	// which clause 3.8 forbids.
	ErrConfidentialityTargetsBCB = errors.New("bpsec: a BCB must not target another BCB")

	// ErrConfidentialityTargetsPrimary indicates a BCB targeting the primary
	// block. Clause 3.8 forbids it: the primary block identifies the bundle
	// and every node on the path has to read it.
	ErrConfidentialityTargetsPrimary = errors.New("bpsec: a BCB must not target the primary block")

	// ErrBCBTargetsUnsharedBIB indicates a BCB targeting a BIB with which it
	// shares no security target. Clause 3.8 allows encrypting a BIB only to
	// hide integrity results for blocks the same BCB is encrypting.
	ErrBCBTargetsUnsharedBIB = errors.New("bpsec: a BCB may target a BIB only when the two share a security target")

	// ErrDuplicateSecurityOperation indicates a second security block applying
	// the same service to a target that already has it. Clause 3.2 requires
	// security operations in a bundle to be unique.
	ErrDuplicateSecurityOperation = errors.New("bpsec: the bundle already applies this security service to that target")

	// ErrIntegrityAfterConfidentiality indicates a BIB added for a target that
	// a BCB already encrypts. Clause 3.9 fixes the order: for a given target,
	// integrity comes before confidentiality.
	ErrIntegrityAfterConfidentiality = errors.New("bpsec: a BIB must not be added for a target a BCB already encrypts")

	// ErrBCBRemovableFlag indicates a BCB with the "discard block if it can't
	// be processed" flag set. Clause 3.8 forbids it: dropping a BCB leaves its
	// targets as ciphertext nobody can read.
	ErrBCBRemovableFlag = errors.New("bpsec: a BCB must not set the discard-if-unprocessable flag")

	// ErrBCBFragmentFlag indicates a BCB targeting the payload block without
	// the "replicate in every fragment" flag. Clause 3.8 requires it, so that
	// each fragment says its payload is ciphertext.
	ErrBCBFragmentFlag = errors.New("bpsec: a BCB targeting the payload must set the replicate-in-every-fragment flag")
)

// Sentinel errors from the security contexts of RFC 9173.
var (
	// ErrUnknownContext indicates a security context identifier this package
	// does not implement. RFC 9173 defines 1 and 2; RFC 9172 clause 9 lets a
	// deployment define others, which decode here but do not process.
	ErrUnknownContext = errors.New("bpsec: unknown security context identifier")

	// ErrUnknownSHAVariant indicates a SHA variant other than 5, 6 or 7
	// (RFC 9173 clause 3.3.1).
	ErrUnknownSHAVariant = errors.New("bpsec: unknown SHA variant")

	// ErrUnknownAESVariant indicates an AES variant other than 1 or 3.
	// RFC 9173 clause 4.3.2 defines A128GCM and A256GCM and no others; note
	// that 2, which would be A192GCM, is deliberately absent.
	ErrUnknownAESVariant = errors.New("bpsec: unknown AES variant")

	// ErrReservedScopeFlag indicates a scope flag outside bits 0 to 2.
	// RFC 9173 clauses 3.3.3 and 4.3.4 reserve bits 3 to 7, leave 8 to 15
	// unassigned, and require a writer to leave all of them zero.
	ErrReservedScopeFlag = errors.New("bpsec: scope flags set a reserved or unassigned bit")

	// ErrIVLength indicates an initialisation vector outside 8 to 16 octets
	// (RFC 9173 clause 4.3.1).
	ErrIVLength = errors.New("bpsec: initialisation vector must be 8 to 16 octets")

	// ErrMissingIV indicates a BCB with no initialisation vector parameter.
	// Clause 4.8.2 lets a node treat that as an error, and this package does:
	// nothing else in the block says what the IV was.
	ErrMissingIV = errors.New("bpsec: BCB carries no initialisation vector")

	// ErrTagLength indicates an authentication tag that is not 128 bits.
	// Clause 4.4.1 fixes the length regardless of the AES variant.
	ErrTagLength = errors.New("bpsec: authentication tag must be 16 octets")

	// ErrMissingTag indicates a BCB target with no authentication tag security
	// result. This package always writes the tag as a security result rather
	// than appending it to the ciphertext, and requires one when reading.
	ErrMissingTag = errors.New("bpsec: BCB carries no authentication tag for this target")

	// ErrMissingHMAC indicates a BIB target with no expected-HMAC security
	// result (RFC 9173 clause 3.4).
	ErrMissingHMAC = errors.New("bpsec: BIB carries no expected HMAC for this target")

	// ErrHMACLength indicates an expected HMAC whose length is not the output
	// size of the SHA variant the block names. Clause 3.1 requires the two to
	// agree.
	ErrHMACLength = errors.New("bpsec: expected HMAC length does not match the SHA variant")

	// ErrNoKey indicates a security operation with no key to use: the block
	// carries no wrapped key and the caller supplied none.
	ErrNoKey = errors.New("bpsec: no key available for this security operation")

	// ErrIntegrityCheckFailed indicates a BIB whose recomputed keyed hash does
	// not match the one the block carries (RFC 9173 clause 3.8.2).
	ErrIntegrityCheckFailed = errors.New("bpsec: integrity check failed")

	// ErrDecryptionFailed indicates a BCB whose ciphertext did not
	// authenticate (RFC 9173 clause 4.8.2). It covers a wrong key, a wrong IV,
	// altered ciphertext and altered additional authenticated data alike; AEAD
	// cannot tell a caller which, and saying more would be a guess.
	ErrDecryptionFailed = errors.New("bpsec: decryption failed to authenticate")
)

// Sentinel errors from building a security block.
var (
	// ErrBlockNumberInUse indicates a security block numbered the same as a
	// block already in the bundle.
	ErrBlockNumberInUse = errors.New("bpsec: another block already has this block number")

	// ErrReservedBlockNumber indicates a security block numbered 0 or 1.
	// RFC 9171 clause 4.1 gives 0 to the primary block and 1 to the payload.
	ErrReservedBlockNumber = errors.New("bpsec: block numbers 0 and 1 are reserved for the primary and payload blocks")

	// ErrKeyLength indicates a key whose length does not suit the variant it
	// is used with.
	ErrKeyLength = errors.New("bpsec: key length does not match the variant")
)
