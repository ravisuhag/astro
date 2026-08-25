package bp

import "fmt"

// Bundle is a complete Bundle Protocol data unit: a primary block followed by
// one or more canonical blocks, per RFC 5050 §4.1.
//
// The last block must carry the last-block flag, and exactly one block must be
// the payload.
type Bundle struct {
	Primary *PrimaryBlock
	Blocks  []*CanonicalBlock
}

// NewBundle builds a bundle carrying one payload.
func NewBundle(primary *PrimaryBlock, payload []byte, options ...BundleOption) (*Bundle, error) {
	b := &Bundle{Primary: primary}
	for _, opt := range options {
		if err := opt(b); err != nil {
			return nil, err
		}
	}

	// The payload always goes last, and carries the last-block flag.
	b.Blocks = append(b.Blocks, &CanonicalBlock{
		Type:  BlockTypePayload,
		Flags: BlockLast,
		Data:  payload,
	})

	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}

// BundleOption configures a bundle at construction.
type BundleOption func(*Bundle) error

// WithECOS attaches an Extended Class of Service block.
//
// CCSDS 734.2-B-1 annex C, C3.1.1 requires the ECOS block to precede the
// payload, and C3.1.2 allows at most one per bundle.
func WithECOS(e ECOS) BundleOption {
	return func(b *Bundle) error {
		for _, existing := range b.Blocks {
			if existing.Type == BlockTypeECOS {
				return ErrInvalidECOS
			}
		}
		block, err := e.Block()
		if err != nil {
			return err
		}
		b.Blocks = append(b.Blocks, block)
		return nil
	}
}

// WithBlock attaches an extension block ahead of the payload.
func WithBlock(block *CanonicalBlock) BundleOption {
	return func(b *Bundle) error {
		if block == nil {
			return ErrDataTooShort
		}
		// The payload owns the last-block flag.
		block.Flags &^= BlockLast
		b.Blocks = append(b.Blocks, block)
		return nil
	}
}

// Payload returns the payload block's data.
func (b *Bundle) Payload() ([]byte, error) {
	block, err := b.PayloadBlock()
	if err != nil {
		return nil, err
	}
	return block.Data, nil
}

// PayloadBlock returns the payload block.
func (b *Bundle) PayloadBlock() (*CanonicalBlock, error) {
	for _, block := range b.Blocks {
		if block.Type == BlockTypePayload {
			return block, nil
		}
	}
	return nil, ErrMissingPayload
}

// ECOS returns the Extended Class of Service block if the bundle carries one.
func (b *Bundle) ECOS() (*ECOS, bool) {
	for _, block := range b.Blocks {
		if block.Type == BlockTypeECOS {
			e, err := DecodeECOS(block.Data)
			if err != nil {
				return nil, false
			}
			return e, true
		}
	}
	return nil, false
}

// Validate checks the bundle's structure against §4.1 and §4.5.2.
func (b *Bundle) Validate() error {
	if b.Primary == nil {
		return ErrDataTooShort
	}
	if err := b.Primary.Validate(); err != nil {
		return err
	}
	if len(b.Blocks) == 0 {
		return ErrMissingPayload
	}

	payloads := 0
	ecosBlocks := 0
	payloadIndex := -1
	ecosIndex := -1

	for i, block := range b.Blocks {
		if err := block.Validate(); err != nil {
			return err
		}
		switch block.Type {
		case BlockTypePayload:
			payloads++
			payloadIndex = i
		case BlockTypeECOS:
			ecosBlocks++
			ecosIndex = i

			// CCSDS annex C, C2 b) and c): the ECOS block replicates into
			// every fragment and carries no EID references. Enforced here,
			// not just in the construction helper, so a decoded bundle is
			// held to the same rules.
			if !block.Flags.Has(BlockReplicate) || block.Flags.Has(BlockHasEIDRefs) {
				return ErrInvalidECOS
			}
			e, err := DecodeECOS(block.Data)
			if err != nil {
				return err
			}
			// C3.1.4: ordinal 255 is reserved for custody signals, which
			// travel as administrative records.
			if e.Ordinal == ECOSCustodySignalOrdinal && !b.Primary.IsAdminRecord() {
				return ErrInvalidECOS
			}
		}
		// Only the final block may claim to be last.
		if block.IsLast() && i != len(b.Blocks)-1 {
			return ErrNoLastBlock
		}
	}

	if payloads == 0 {
		return ErrMissingPayload
	}
	if payloads > 1 {
		return ErrMultiplePayloads
	}
	if !b.Blocks[len(b.Blocks)-1].IsLast() {
		return ErrNoLastBlock
	}

	// CCSDS annex C, C3.1.2 and C3.1.1.
	if ecosBlocks > 1 {
		return ErrInvalidECOS
	}
	if ecosIndex >= 0 && payloadIndex >= 0 && ecosIndex > payloadIndex {
		return ErrInvalidECOS
	}
	return nil
}

// Encode serializes the whole bundle.
func (b *Bundle) Encode() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	out, err := b.Primary.Encode()
	if err != nil {
		return nil, err
	}
	for _, block := range b.Blocks {
		encoded, err := block.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, encoded...)
	}
	return out, nil
}

// DecodeOptions tunes bundle decoding.
type DecodeOptions struct {
	// MaxBlockLength caps a single block's body. Zero selects
	// DefaultMaxBlockLength.
	MaxBlockLength uint64
	// MaxBlocks caps how many blocks one bundle may carry. Zero selects
	// DefaultMaxBlocks.
	MaxBlocks int
}

// DefaultMaxBlocks bounds the block count when DecodeOptions leaves MaxBlocks
// at zero.
const DefaultMaxBlocks = 64

// DecodeBundle parses a complete bundle. Data continuing past the last block
// is an error: a codec that silently drops octets hides corruption. Use
// DecodeBundleN when bundles arrive back to back in one buffer.
func DecodeBundle(data []byte) (*Bundle, error) {
	return DecodeBundleWithOptions(data, DecodeOptions{})
}

// DecodeBundleWithOptions parses a complete bundle under explicit limits,
// rejecting trailing data like DecodeBundle.
func DecodeBundleWithOptions(data []byte, opts DecodeOptions) (*Bundle, error) {
	b, n, err := DecodeBundleN(data, opts)
	if err != nil {
		return nil, err
	}
	if n < len(data) {
		return nil, ErrTrailingBytes
	}
	return b, nil
}

// DecodeBundleN parses one bundle from the front of data, returning the
// bundle and the octets consumed. Trailing data is left for the caller,
// which is what a stream of concatenated bundles needs.
func DecodeBundleN(data []byte, opts DecodeOptions) (*Bundle, int, error) {
	if opts.MaxBlocks <= 0 {
		opts.MaxBlocks = DefaultMaxBlocks
	}

	primary, offset, err := DecodePrimaryBlock(data)
	if err != nil {
		return nil, 0, err
	}

	b := &Bundle{Primary: primary}
	for offset < len(data) {
		if len(b.Blocks) >= opts.MaxBlocks {
			return nil, 0, ErrBlockTooLarge
		}
		block, n, err := DecodeCanonicalBlock(data[offset:], opts.MaxBlockLength)
		if err != nil {
			return nil, 0, err
		}
		b.Blocks = append(b.Blocks, block)
		offset += n

		// §4.5.2: the last-block flag ends the bundle.
		if block.IsLast() {
			break
		}
	}

	if err := b.Validate(); err != nil {
		return nil, 0, err
	}
	return b, offset, nil
}

// Humanize returns a human-readable summary of the whole bundle.
func (b *Bundle) Humanize() string {
	if b.Primary == nil {
		return "Bundle (empty)"
	}
	out := b.Primary.Humanize()
	for _, block := range b.Blocks {
		out += "\n" + block.Humanize()
	}
	if e, ok := b.ECOS(); ok {
		out += "\n" + e.Humanize()
	}
	return out
}

// String renders a one-line description.
func (b *Bundle) String() string {
	if b.Primary == nil {
		return "bundle (empty)"
	}
	return fmt.Sprintf("bundle %s -> %s, created %d.%d",
		b.Primary.Source, b.Primary.Destination,
		b.Primary.CreationTimestamp.Time, b.Primary.CreationTimestamp.SequenceNumber)
}
