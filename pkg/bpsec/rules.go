package bpsec

import "github.com/ravisuhag/astro/pkg/bp"

// securityBlock pairs a security block with its decoded Abstract Security
// Block, so the target rules can be checked without decoding twice.
type securityBlock struct {
	Block *bp.CanonicalBlock
	ASB   *ASB
}

// confidentialityBlocks finds every BCB in the bundle, decoded.
//
// A BCB is never itself encrypted — RFC 9172 clause 3.8 forbids a BCB from
// targeting another BCB — so every one of them must decode, and one that does
// not is an error rather than something to skip. Carrying on would mean
// deciding whether a new security operation conflicts with an existing one
// while unable to read what that operation covers.
func confidentialityBlocks(b *bp.Bundle) ([]securityBlock, error) {
	var out []securityBlock
	for _, blk := range b.Blocks {
		if blk.Type != BlockTypeConfidentiality {
			continue
		}
		asb, err := DecodeASB(blk.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, securityBlock{Block: blk, ASB: asb})
	}
	return out, nil
}

// integrityBlocks finds every BIB in the bundle that can still be read.
//
// A BIB that a BCB has encrypted is skipped rather than treated as malformed.
// Its block-type-specific data is ciphertext, so there is nothing to decode,
// and RFC 9172 clause 3.9 says such a BIB must not be checked anyway. The
// alternative — failing on it — would make a bundle unusable for exactly the
// arrangement appendix A.4 of RFC 9173 prints as an example.
func integrityBlocks(b *bp.Bundle) ([]securityBlock, error) {
	encrypted, err := encryptedTargets(b)
	if err != nil {
		return nil, err
	}

	var out []securityBlock
	for _, blk := range b.Blocks {
		if blk.Type != BlockTypeIntegrity {
			continue
		}
		if _, hidden := encrypted[blk.Number]; hidden {
			continue
		}
		asb, err := DecodeASB(blk.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, securityBlock{Block: blk, ASB: asb})
	}
	return out, nil
}

// encryptedTargets collects every block number some BCB in the bundle covers.
func encryptedTargets(b *bp.Bundle) (map[uint64]struct{}, error) {
	bcbs, err := confidentialityBlocks(b)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]struct{})
	for _, sb := range bcbs {
		for _, t := range sb.ASB.Targets {
			out[t] = struct{}{}
		}
	}
	return out, nil
}

// securityBlocksOfType finds every BIB or every BCB already in the bundle.
func securityBlocksOfType(b *bp.Bundle, t bp.BlockType) ([]securityBlock, error) {
	if t == BlockTypeConfidentiality {
		return confidentialityBlocks(b)
	}
	return integrityBlocks(b)
}

// coveringBlock returns the security block of the given type whose targets
// include this block number, or nil.
func coveringBlock(b *bp.Bundle, t bp.BlockType, target uint64) (*bp.CanonicalBlock, error) {
	blocks, err := securityBlocksOfType(b, t)
	if err != nil {
		return nil, err
	}
	for _, sb := range blocks {
		if sb.ASB.TargetIndex(target) >= 0 {
			return sb.Block, nil
		}
	}
	return nil, nil
}

// encryptedBy returns the BCB whose targets include this block number, or nil.
func encryptedBy(b *bp.Bundle, target uint64) *bp.CanonicalBlock {
	blk, err := coveringBlock(b, BlockTypeConfidentiality, target)
	if err != nil {
		return nil
	}
	return blk
}

// checkNewBlockNumber enforces the numbering of RFC 9171 clause 4.1 and
// refuses a number another block already uses.
func checkNewBlockNumber(b *bp.Bundle, number uint64) error {
	if number <= bp.PayloadBlockNumber {
		return ErrReservedBlockNumber
	}
	if b.Block(number) != nil {
		return ErrBlockNumberInUse
	}
	return nil
}

// checkTargetList enforces the rules every security block shares: at least one
// target, no duplicates, and every entry naming a block that exists
// (RFC 9172 clause 3.6).
func checkTargetList(b *bp.Bundle, targets []uint64) error {
	if len(targets) == 0 {
		return ErrNoTargets
	}
	seen := make(map[uint64]struct{}, len(targets))
	for _, t := range targets {
		if _, dup := seen[t]; dup {
			return ErrDuplicateTarget
		}
		seen[t] = struct{}{}

		// Block number 0 is the primary block, which every bundle has.
		if t == bp.PrimaryBlockNumber {
			continue
		}
		if b.Block(t) == nil {
			return ErrTargetNotInBundle
		}
	}
	return nil
}

// checkIntegrityTargets enforces what a BIB may cover.
func checkIntegrityTargets(b *bp.Bundle, targets []uint64) error {
	if err := checkTargetList(b, targets); err != nil {
		return err
	}

	for _, t := range targets {
		// Clause 3.7: a BIB must not target a security block. Integrity over a
		// BIB or a BCB is the confidentiality service's job, not this one's.
		if blk := b.Block(t); blk != nil && isSecurityBlockType(blk.Type) {
			return ErrIntegrityTargetsSecurityBlock
		}

		// Clause 3.2: security operations are unique, so a target may carry at
		// most one integrity service.
		existing, err := coveringBlock(b, BlockTypeIntegrity, t)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrDuplicateSecurityOperation
		}

		// Clause 3.9: for a given target, integrity comes before
		// confidentiality. Adding a BIB for a target a BCB already encrypts
		// would hash ciphertext and leave the processing order ambiguous.
		encrypted, err := coveringBlock(b, BlockTypeConfidentiality, t)
		if err != nil {
			return err
		}
		if encrypted != nil {
			return ErrIntegrityAfterConfidentiality
		}
	}
	return nil
}

// checkConfidentialityTargets enforces what a BCB may cover, and the flags a
// BCB must and must not set.
func checkConfidentialityTargets(b *bp.Bundle, targets []uint64, flags bp.BlockControlFlags) error {
	if err := checkTargetList(b, targets); err != nil {
		return err
	}

	// Clause 3.8: removing a BCB without decrypting its targets strands them
	// as ciphertext, so the discard flag must be clear.
	if flags.Has(bp.BlockFlagDiscardBlockIfUnprocessable) {
		return ErrBCBRemovableFlag
	}

	for _, t := range targets {
		// Clause 3.8: the primary block identifies the bundle and every node
		// on the path reads it, so it can never be encrypted.
		if t == bp.PrimaryBlockNumber {
			return ErrConfidentialityTargetsPrimary
		}
		// Clause 3.8: each fragment's payload must announce that it holds
		// ciphertext, which is what the replication flag does.
		if t == bp.PayloadBlockNumber && !flags.Has(bp.BlockFlagReplicateInEveryFragment) {
			return ErrBCBFragmentFlag
		}

		blk := b.Block(t)
		if blk != nil && blk.Type == BlockTypeConfidentiality {
			return ErrConfidentialityTargetsBCB
		}
		// Clause 3.8: a BCB may encrypt a BIB only to hide integrity results
		// for blocks the same BCB is also encrypting. Encrypting an unrelated
		// BIB would deny a waypoint a check it was entitled to make.
		if blk != nil && blk.Type == BlockTypeIntegrity {
			if err := checkBIBSharesTarget(blk, targets); err != nil {
				return err
			}
		}

		existing, err := coveringBlock(b, BlockTypeConfidentiality, t)
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrDuplicateSecurityOperation
		}
	}
	return nil
}

// checkBIBSharesTarget reports whether a BIB being encrypted shares at least
// one security target with the BCB encrypting it (RFC 9172 clause 3.8).
func checkBIBSharesTarget(bib *bp.CanonicalBlock, bcbTargets []uint64) error {
	asb, err := DecodeASB(bib.Data)
	if err != nil {
		return err
	}
	for _, t := range bcbTargets {
		if asb.TargetIndex(t) >= 0 {
			return nil
		}
	}
	return ErrBCBTargetsUnsharedBIB
}

// isSecurityBlockType reports whether a block type code is one of the two
// RFC 9172 clause 11.1 registers.
func isSecurityBlockType(t bp.BlockType) bool {
	return t == BlockTypeIntegrity || t == BlockTypeConfidentiality
}

// IsSecurityBlock reports whether a block is a BIB or a BCB.
func IsSecurityBlock(b *bp.CanonicalBlock) bool {
	return b != nil && isSecurityBlockType(b.Type)
}

// clearTargetCRC removes a checksum from a security target.
//
// RFC 9173 clauses 3.8.1 and 4.8.1 both require this before the security
// service runs: a block covered by a keyed hash or by authenticated encryption
// no longer needs the weaker unkeyed check, and RFC 9171 clause 4.3.2 stops
// requiring one. Neither the IPPT nor the AAD includes a block's CRC, so this
// changes what goes on the wire without changing what either cipher sees.
func clearTargetCRC(b *bp.Bundle, target uint64) {
	if target == bp.PrimaryBlockNumber {
		b.Primary.CRCType = bp.CRCNone
		return
	}
	if blk := b.Block(target); blk != nil {
		blk.CRCType = bp.CRCNone
	}
}

// Remove takes a security block out of the bundle and reports whether it was
// there.
//
// RFC 9172 clause 5.1 requires a security acceptor to remove a security block
// once every operation in it has been processed: a BIB left behind after its
// hashes were checked, or a BCB left beside plaintext, tells the next node
// something that is no longer true. Decrypt does this for a BCB. Verify does
// not do it for a BIB, because verifying is also what a waypoint does on its
// way past, and only the caller knows whether this node is the acceptor.
func Remove(b *bp.Bundle, security *bp.CanonicalBlock) bool {
	for i, blk := range b.Blocks {
		if blk == security {
			b.Blocks = append(b.Blocks[:i], b.Blocks[i+1:]...)
			return true
		}
	}
	return false
}

// insertSecurityBlock places a security block in the bundle: after the last
// security block already there, or ahead of every other extension block if
// this is the first one.
//
// RFC 9171 clause 4.1 fixes only that the payload block comes last, so where
// the rest sit is a convention rather than a rule. This one puts security
// blocks first, in the order they were applied, which is what every worked
// example in RFC 9173 appendix A shows and what lets a receiver meet a
// security block before the blocks it protects.
func insertSecurityBlock(b *bp.Bundle, blk *bp.CanonicalBlock) {
	at := 0
	for i, existing := range b.Blocks {
		if isSecurityBlockType(existing.Type) {
			at = i + 1
		}
	}
	// The payload stays last whatever else happens.
	if at > len(b.Blocks) {
		at = len(b.Blocks)
	}

	b.Blocks = append(b.Blocks, nil)
	copy(b.Blocks[at+1:], b.Blocks[at:])
	b.Blocks[at] = blk
}
