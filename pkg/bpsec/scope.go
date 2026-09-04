package bpsec

import (
	"github.com/ravisuhag/astro/internal/cbor"
	"github.com/ravisuhag/astro/pkg/bp"
)

// ScopeFlags says how much of the bundle a security operation binds itself to,
// beyond the contents of its target.
//
// The same three bits mean the same three things in both contexts RFC 9173
// defines: clause 3.3.3 calls them the integrity scope flags and clause 4.3.4
// calls them the AAD scope flags. The field is processed as 16 bits wide.
type ScopeFlags uint16

const (
	// ScopePrimaryBlock binds the operation to the bundle's primary block, so
	// that a target lifted into a different bundle stops verifying
	// (RFC 9173 clauses 3.2 and 4.2).
	ScopePrimaryBlock ScopeFlags = 1 << 0
	// ScopeTargetHeader binds the operation to the target's own type code,
	// block number and processing control flags, so that changing how the
	// target is handled breaks the check.
	ScopeTargetHeader ScopeFlags = 1 << 1
	// ScopeSecurityHeader binds the operation to the security block's own type
	// code, block number and processing control flags.
	ScopeSecurityHeader ScopeFlags = 1 << 2

	// ScopeAll sets every assigned flag. RFC 9173 clauses 3.3.3 and 4.3.4 make
	// it the value to assume when the parameter is absent.
	ScopeAll = ScopePrimaryBlock | ScopeTargetHeader | ScopeSecurityHeader
	// ScopeNone covers the target contents and nothing else.
	ScopeNone ScopeFlags = 0
)

// Validate rejects any bit outside the three assigned ones. RFC 9173
// clauses 3.3.3 and 4.3.4 reserve bits 3 to 7, leave bits 8 to 15 unassigned,
// and require a security source to leave all of them zero.
func (f ScopeFlags) Validate() error {
	if f&^ScopeAll != 0 {
		return ErrReservedScopeFlag
	}
	return nil
}

// Has reports whether every flag in mask is set.
func (f ScopeFlags) Has(mask ScopeFlags) bool { return f&mask == mask }

// assignedBlockFlags are the block processing control flag bits RFC 9171
// clause 4.2.4 assigns: 0, 1, 2 and 4. Bits 3, 5 and 6 are reserved and bits 7
// upward are unassigned.
//
// RFC 9172 clause 4 requires the reserved and unassigned ones to be zero in a
// canonical form, "as it is not known if those flags will change in transit".
// A sender that leaves a reserved bit set would otherwise compute a hash no
// receiver could reproduce once some later node cleared it.
const assignedBlockFlags = uint64(bp.BlockFlagReplicateInEveryFragment |
	bp.BlockFlagReportIfUnprocessable |
	bp.BlockFlagDeleteBundleIfUnprocessable |
	bp.BlockFlagDiscardBlockIfUnprocessable)

// canonicalFlags masks a block's processing control flags down to the bits
// RFC 9171 assigns.
func canonicalFlags(f bp.BlockControlFlags) uint64 {
	return uint64(f) & assignedBlockFlags
}

// appendBlockMetadata writes the three header fields a scope flag pulls in:
// block type code, block number and block processing control flags, in that
// order (RFC 9173 clauses 3.7 and 4.7.2, and the block-metadata rule of the
// CDDL in appendix B).
func appendBlockMetadata(dst []byte, b *bp.CanonicalBlock) []byte {
	dst = cbor.AppendUint(dst, uint64(b.Type))
	dst = cbor.AppendUint(dst, b.Number)
	return cbor.AppendUint(dst, canonicalFlags(b.Flags))
}

// IPPT builds the Integrity-Protected Plaintext of RFC 9173 clause 3.7: the
// octets a BIB's keyed hash is taken over.
//
// targetNumber names the block the BIB covers. Block number 0 means the
// bundle's primary block, which RFC 9171 clause 4.1 numbers implicitly.
// security is the BIB itself, which the security header flag pulls in.
//
// This is exported because it is the part of BPSec two implementations are
// most likely to disagree about, and a disagreement here shows up as an
// integrity failure with nothing to point at. Building the IPPT on its own and
// comparing octets is the fastest way to find out who is right.
func IPPT(b *bp.Bundle, scope ScopeFlags, targetNumber uint64, security *bp.CanonicalBlock) ([]byte, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if b == nil || b.Primary == nil {
		return nil, bp.ErrNoPrimaryBlock
	}

	targetIsPrimary := targetNumber == bp.PrimaryBlockNumber

	var target *bp.CanonicalBlock
	if !targetIsPrimary {
		if target = b.Block(targetNumber); target == nil {
			return nil, ErrTargetNotInBundle
		}
	}

	// Step 1: the scope flags themselves, always, even when the BIB does not
	// carry them as a parameter. Without this a verifier configured with a
	// different scope would compute a different IPPT and never learn why.
	dst := cbor.AppendUint(nil, uint64(scope))

	// Steps 2 and 3 are skipped when the target is the primary block. The note
	// in clause 3.7 gives both reasons: including the primary block twice
	// would be pointless, and the primary block has no block number or
	// processing control flags to include.
	if scope.Has(ScopePrimaryBlock) && !targetIsPrimary {
		primary, err := b.Primary.Encode()
		if err != nil {
			return nil, err
		}
		dst = append(dst, primary...)
	}
	if scope.Has(ScopeTargetHeader) && !targetIsPrimary {
		dst = appendBlockMetadata(dst, target)
	}

	// Step 4: the BIB's own header.
	if scope.Has(ScopeSecurityHeader) {
		if security == nil {
			return nil, ErrMalformedASB
		}
		dst = appendBlockMetadata(dst, security)
	}

	// Step 5: the target itself, as a CBOR byte string. For an ordinary block
	// that is the block-type-specific data with the head it already has on the
	// wire, which is what RFC 9172 clause 4 means by "these fields MUST
	// include their own CBOR encoding".
	//
	// When the target is the primary block, the whole block is wrapped in a
	// byte string head that does not appear anywhere in the bundle. The
	// primary block has no block-type-specific data field to quote, so the
	// step quotes the block instead, and still quotes it as a byte string.
	// Appendix A.3 of RFC 9173 prints the result: the primary block IPPT
	// begins 00 581c 8807, where 581c is a 28-octet byte string head wrapping
	// the 28-octet primary block.
	//
	// Note that this is not what step 2 does with the same block. When the
	// primary block flag pulls the primary block into some other target's
	// IPPT, it goes in raw — appendix A.4 shows 07 8807, with no head. The
	// same octets are framed one way as a target and another as context.
	if targetIsPrimary {
		primary, err := b.Primary.Encode()
		if err != nil {
			return nil, err
		}
		return cbor.AppendByteString(dst, primary), nil
	}
	return cbor.AppendByteString(dst, target.Data), nil
}

// AAD builds the additional authenticated data of RFC 9173 clause 4.7.2: the
// octets a BCB authenticates alongside the ciphertext without encrypting them.
//
// targetNumber names the block the BCB covers, and security is the BCB itself.
// Unlike the IPPT there is no exception for the primary block, because
// RFC 9172 clause 3.8 forbids a BCB from targeting it at all.
func AAD(b *bp.Bundle, scope ScopeFlags, targetNumber uint64, security *bp.CanonicalBlock) ([]byte, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if b == nil || b.Primary == nil {
		return nil, bp.ErrNoPrimaryBlock
	}
	if targetNumber == bp.PrimaryBlockNumber {
		return nil, ErrConfidentialityTargetsPrimary
	}

	target := b.Block(targetNumber)
	if target == nil {
		return nil, ErrTargetNotInBundle
	}

	dst := cbor.AppendUint(nil, uint64(scope))

	if scope.Has(ScopePrimaryBlock) {
		primary, err := b.Primary.Encode()
		if err != nil {
			return nil, err
		}
		dst = append(dst, primary...)
	}
	if scope.Has(ScopeTargetHeader) {
		dst = appendBlockMetadata(dst, target)
	}
	// Clause 4.7.2 step 4 says "associated with the BIB". It means the
	// security block this AAD belongs to, which for a BCB is the BCB: the
	// worked example in appendix A.4 appends 0c0201, the BCB's own type code,
	// block number and flags, and no BIB header appears in the payload AAD at
	// all.
	if scope.Has(ScopeSecurityHeader) {
		if security == nil {
			return nil, ErrMalformedASB
		}
		dst = appendBlockMetadata(dst, security)
	}
	return dst, nil
}
