package bp

import "github.com/ravisuhag/astro/internal/cbor"

// Bundle is a complete Bundle Protocol version 7 data unit
// (RFC 9171 clause 4.1).
//
// On the wire it is a CBOR indefinite-length array closed by a break stop
// code: the primary block, then one or more canonical blocks, the last of
// which is the payload.
//
// That the array is indefinite-length is worth stating plainly, because the
// CDDL grammar in RFC 9171 appendix B writes it as though it were definite.
// Clause 4.1 requires the indefinite form, and the appendix says the prose
// wins wherever the two disagree. An implementation that followed the grammar
// would emit bundles no conforming node accepts, while reading its own output
// back perfectly.
type Bundle struct {
	Primary *PrimaryBlock
	// Blocks holds every block after the primary one, in wire order. The last
	// entry is the payload block.
	Blocks []*CanonicalBlock
}

// NewBundle builds a bundle from a primary block, a payload, and any extension
// blocks. The payload block is created here and placed last, where clause 4.1
// requires it.
func NewBundle(primary *PrimaryBlock, payload []byte, extensions ...*CanonicalBlock) (*Bundle, error) {
	b := &Bundle{Primary: primary}
	b.Blocks = append(b.Blocks, extensions...)
	b.Blocks = append(b.Blocks, NewPayloadBlock(payload))
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}

// PayloadBlock returns the payload block, which is always the last one.
func (b *Bundle) PayloadBlock() *CanonicalBlock {
	if len(b.Blocks) == 0 {
		return nil
	}
	return b.Blocks[len(b.Blocks)-1]
}

// Payload returns the application data the bundle carries.
func (b *Bundle) Payload() []byte {
	p := b.PayloadBlock()
	if p == nil {
		return nil
	}
	return p.Data
}

// blockOfType returns the first block of the given type, or nil.
func (b *Bundle) blockOfType(t BlockType) *CanonicalBlock {
	for _, blk := range b.Blocks {
		if blk.Type == t {
			return blk
		}
	}
	return nil
}

// Validate checks every rule RFC 9171 states about a bundle as a whole, and is
// what Encode calls before writing one.
//
// It is stricter than what Decode enforces, on purpose. Decode checks only the
// rules that must hold for a bundle to be unambiguous — one payload block,
// last, block numbers unique — because a node has to be able to read and
// forward what it is given. Validate additionally checks rules binding the
// node *creating* a bundle, chiefly clause 4.4.2's requirement that a bundle
// with an unknown creation time carry a Bundle Age block.
//
// The split is not theoretical. The example bundle in RFC 9173 appendix A.1
// has a creation time of zero and no Bundle Age block, so it does not satisfy
// clause 4.4.2 — the same document adds the block in appendix A.3 and explains
// why. A decoder that refused it would reject a bundle a standards-track RFC
// prints as an example, and the implementations that copied it.
func (b *Bundle) Validate() error {
	if err := b.validateStructure(); err != nil {
		return err
	}

	// Clause 4.4.2: a bundle created by a node with no clock has no usable
	// creation time, so its age is the only way to tell when it expires. Such
	// a bundle must carry a Bundle Age block.
	if b.Primary.Timestamp.Time == DTNTimeUnknown && b.blockOfType(BlockTypeBundleAge) == nil {
		return ErrMissingBundleAgeBlock
	}
	return nil
}

// validateStructure checks the rules a bundle must satisfy to be read at all.
func (b *Bundle) validateStructure() error {
	if b.Primary == nil {
		return ErrNoPrimaryBlock
	}
	if err := b.Primary.Validate(); err != nil {
		return err
	}
	if len(b.Blocks) == 0 {
		return ErrNoPayloadBlock
	}

	seenNumbers := make(map[uint64]bool, len(b.Blocks))
	counts := make(map[BlockType]int, len(b.Blocks))

	for i, blk := range b.Blocks {
		if err := blk.Validate(); err != nil {
			return err
		}
		// Clause 4.1: a block number identifies a block within its bundle, so
		// two blocks cannot share one. BPSec blocks reference targets by
		// number, and a duplicate makes that reference ambiguous.
		if seenNumbers[blk.Number] {
			return ErrDuplicateBlockNumber
		}
		seenNumbers[blk.Number] = true
		counts[blk.Type]++

		// Clause 4.1: exactly one payload block, and it comes last.
		if blk.Type == BlockTypePayload && i != len(b.Blocks)-1 {
			return ErrPayloadBlockNotLast
		}
	}

	if counts[BlockTypePayload] != 1 {
		return ErrPayloadBlockCount
	}

	// Clauses 4.4.1, 4.4.2 and 4.4.3 each allow at most one of their block.
	for _, t := range []BlockType{BlockTypePreviousNode, BlockTypeBundleAge, BlockTypeHopCount} {
		if counts[t] > 1 {
			return ErrDuplicateExtensionBlock
		}
	}

	return nil
}

// Encode writes the bundle.
//
// Every checksum is computed here, from the fields as they stand now. A block
// mutated after it was built goes out with a checksum that matches the change,
// never a stale one replayed from construction time.
func (b *Bundle) Encode() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	out := cbor.AppendIndefiniteArrayHeader(nil)

	var err error
	if out, err = appendPrimaryBlock(out, b.Primary); err != nil {
		return nil, err
	}
	for _, blk := range b.Blocks {
		if out, err = appendCanonicalBlock(out, blk); err != nil {
			return nil, err
		}
	}
	return cbor.AppendBreak(out), nil
}

// Decode reads one bundle. It returns an error rather than a partial bundle
// for any input it cannot fully account for.
func Decode(data []byte) (*Bundle, error) {
	d := cbor.NewDecoder(data)

	_, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	if !indefinite {
		// Clause 4.1 requires the indefinite-length form. A definite-length
		// array is the mistake the appendix B grammar invites.
		return nil, ErrDefiniteLengthBundle
	}

	primary, err := decodePrimaryBlock(d)
	if err != nil {
		return nil, err
	}
	b := &Bundle{Primary: primary}

	for !d.AtBreak() {
		if d.AtEnd() {
			return nil, ErrTruncated
		}
		blk, err := decodeCanonicalBlock(d)
		if err != nil {
			return nil, err
		}
		b.Blocks = append(b.Blocks, blk)
	}
	if err := d.ReadBreak(); err != nil {
		return nil, err
	}
	if !d.AtEnd() {
		return nil, ErrTrailingBytes
	}

	if err := b.validateStructure(); err != nil {
		return nil, err
	}
	return b, nil
}

// Block returns the block carrying the given number, or nil if the bundle has
// none.
//
// Block number 0 belongs to the primary block, which is not a CanonicalBlock
// and is reached through the Primary field instead; asking for it returns nil.
// BPSec security blocks name their targets by block number
// (RFC 9172 clause 3.4), which is what this is for.
func (b *Bundle) Block(number uint64) *CanonicalBlock {
	for _, blk := range b.Blocks {
		if blk.Number == number {
			return blk
		}
	}
	return nil
}
