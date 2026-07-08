package bp

import "sort"

// Fragment splits a bundle into pieces whose payloads are at most maxPayload
// octets each, per RFC 5050 §5.8.
//
// Every fragment carries the same primary block with the fragment flag set,
// its own offset into the original application data unit, and the total ADU
// length so a receiver knows when it has everything.
//
// Blocks flagged "replicate in every fragment" are copied into each piece;
// the rest travel with the first fragment only, which is what §5.8 requires.
func (b *Bundle) Fragment(maxPayload int) ([]*Bundle, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if maxPayload <= 0 {
		return nil, ErrCannotFragment
	}
	// §4.2: the flag means what it says.
	if b.Primary.Flags.Has(FlagNoFragment) {
		return nil, ErrCannotFragment
	}

	payload, err := b.Payload()
	if err != nil {
		return nil, err
	}

	// Already small enough, and not already a fragment: nothing to do.
	if len(payload) <= maxPayload && !b.Primary.IsFragment() {
		return []*Bundle{b}, nil
	}

	total := uint64(len(payload))
	baseOffset := uint64(0)
	if b.Primary.IsFragment() {
		// Fragmenting a fragment: offsets stay relative to the original ADU.
		baseOffset = b.Primary.FragmentOffset
		total = b.Primary.TotalADULength
	}

	var out []*Bundle
	for start := 0; start < len(payload); start += maxPayload {
		end := start + maxPayload
		if end > len(payload) {
			end = len(payload)
		}

		primary := *b.Primary // copy
		primary.Flags |= FlagFragment
		primary.FragmentOffset = baseOffset + uint64(start)
		primary.TotalADULength = total

		fragment := &Bundle{Primary: &primary}

		// §5.8: only blocks flagged for replication appear in every fragment.
		// Everything else rides with the first.
		first := len(out) == 0
		for _, block := range b.Blocks {
			if block.Type == BlockTypePayload {
				continue
			}
			if !first && !block.Flags.Has(BlockReplicate) {
				continue
			}
			copied := *block
			copied.Flags &^= BlockLast
			if len(block.Data) > 0 {
				copied.Data = make([]byte, len(block.Data))
				copy(copied.Data, block.Data)
			}
			fragment.Blocks = append(fragment.Blocks, &copied)
		}

		piece := make([]byte, end-start)
		copy(piece, payload[start:end])
		fragment.Blocks = append(fragment.Blocks, &CanonicalBlock{
			Type:  BlockTypePayload,
			Flags: BlockLast,
			Data:  piece,
		})

		if err := fragment.Validate(); err != nil {
			return nil, err
		}
		out = append(out, fragment)
	}
	return out, nil
}

// sameBundle reports whether two fragments came from one original bundle.
// §4.5.1 identifies a bundle by its source endpoint and creation timestamp.
func sameBundle(a, b *PrimaryBlock) bool {
	return a.Source == b.Source &&
		a.CreationTimestamp == b.CreationTimestamp &&
		a.TotalADULength == b.TotalADULength
}

// Reassemble rebuilds the original bundle from a set of fragments, per §5.9.
//
// The fragments may arrive in any order and may overlap; what matters is that
// together they cover the whole application data unit.
func Reassemble(fragments []*Bundle) (*Bundle, error) {
	if len(fragments) == 0 {
		return nil, ErrIncompleteFragments
	}

	for _, f := range fragments {
		if f == nil || f.Primary == nil {
			return nil, ErrIncompleteFragments
		}
		if !f.Primary.IsFragment() {
			return nil, ErrNotFragment
		}
		if !sameBundle(fragments[0].Primary, f.Primary) {
			return nil, ErrMismatchedFragments
		}
	}

	total := fragments[0].Primary.TotalADULength
	if total > DefaultMaxBlockLength {
		return nil, ErrBlockTooLarge
	}

	// Order by offset so the coverage check is a single pass.
	ordered := make([]*Bundle, len(fragments))
	copy(ordered, fragments)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Primary.FragmentOffset < ordered[j].Primary.FragmentOffset
	})

	adu := make([]byte, total)
	var covered uint64

	for _, f := range ordered {
		payload, err := f.Payload()
		if err != nil {
			return nil, err
		}
		start := f.Primary.FragmentOffset
		end := start + uint64(len(payload))
		if end > total {
			return nil, ErrMismatchedFragments
		}
		copy(adu[start:end], payload)

		// Gaps are what stop reassembly; overlaps are harmless.
		if start > covered {
			return nil, ErrIncompleteFragments
		}
		if end > covered {
			covered = end
		}
	}

	if covered < total {
		return nil, ErrIncompleteFragments
	}

	// Rebuild the original: clear the fragment flag and its two fields.
	primary := *fragments[0].Primary
	primary.Flags &^= FlagFragment
	primary.FragmentOffset = 0
	primary.TotalADULength = 0

	rebuilt := &Bundle{Primary: &primary}

	// Extension blocks come from the fragment that carried them, which for a
	// non-replicated block is the one at offset zero.
	for _, block := range ordered[0].Blocks {
		if block.Type == BlockTypePayload {
			continue
		}
		copied := *block
		copied.Flags &^= BlockLast
		rebuilt.Blocks = append(rebuilt.Blocks, &copied)
	}

	rebuilt.Blocks = append(rebuilt.Blocks, &CanonicalBlock{
		Type:  BlockTypePayload,
		Flags: BlockLast,
		Data:  adu,
	})

	if err := rebuilt.Validate(); err != nil {
		return nil, err
	}
	return rebuilt, nil
}
