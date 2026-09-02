package bp

import "sort"

// Fragment splits a bundle into pieces whose payloads are at most maxPayload
// octets each, per RFC 5050 clause 5.8.
//
// Every fragment carries the same primary block with the fragment flag set,
// its own offset into the original application data unit, and the total ADU
// length so a receiver knows when it has everything.
//
// Blocks flagged "replicate in every fragment" are copied into each piece.
// Of the rest, clause 5.8 sends blocks that precede the payload with the first
// fragment and blocks that follow the payload with the last.
func (b *Bundle) Fragment(maxPayload int) ([]*Bundle, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if maxPayload <= 0 {
		return nil, ErrCannotFragment
	}
	// Clause 4.2: the flag means what it says.
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

	// Clause 5.8 splits the extension blocks around the payload: those preceding
	// it go with the first fragment, those following it with the last, and
	// replicate-flagged ones with every fragment.
	payloadIndex := 0
	for i, block := range b.Blocks {
		if block.Type == BlockTypePayload {
			payloadIndex = i
			break
		}
	}

	cloneBlock := func(block *CanonicalBlock) *CanonicalBlock {
		copied := *block
		copied.Flags &^= BlockLast
		if len(block.Data) > 0 {
			copied.Data = make([]byte, len(block.Data))
			copy(copied.Data, block.Data)
		}
		return &copied
	}

	var out []*Bundle
	for start := 0; start < len(payload); start += maxPayload {
		end := start + maxPayload
		if end > len(payload) {
			end = len(payload)
		}
		first := start == 0
		last := end == len(payload)

		primary := *b.Primary // copy
		primary.Flags |= FlagFragment
		primary.FragmentOffset = baseOffset + uint64(start)
		primary.TotalADULength = total

		fragment := &Bundle{Primary: &primary}

		// Blocks that precede the payload, in order.
		for _, block := range b.Blocks[:payloadIndex] {
			if first || block.Flags.Has(BlockReplicate) {
				fragment.Blocks = append(fragment.Blocks, cloneBlock(block))
			}
		}

		piece := make([]byte, end-start)
		copy(piece, payload[start:end])
		fragment.Blocks = append(fragment.Blocks, &CanonicalBlock{
			Type: BlockTypePayload,
			Data: piece,
		})

		// Blocks that follow the payload, in order.
		for _, block := range b.Blocks[payloadIndex+1:] {
			if last || block.Flags.Has(BlockReplicate) {
				fragment.Blocks = append(fragment.Blocks, cloneBlock(block))
			}
		}

		fragment.Blocks[len(fragment.Blocks)-1].Flags |= BlockLast

		if err := fragment.Validate(); err != nil {
			return nil, err
		}
		out = append(out, fragment)
	}
	return out, nil
}

// sameBundle reports whether two fragments came from one original bundle.
// Clause 4.5.1 identifies a bundle by its source endpoint and creation timestamp.
func sameBundle(a, b *PrimaryBlock) bool {
	return a.Source == b.Source &&
		a.CreationTimestamp == b.CreationTimestamp &&
		a.TotalADULength == b.TotalADULength
}

// Reassemble rebuilds the original bundle from a set of fragments, per clause 5.9.
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

	// Extension blocks come from the fragment that carried them, per clause 5.8:
	// blocks preceding the payload from the first fragment, blocks following
	// it from the last.
	blocksAround := func(f *Bundle, before bool) []*CanonicalBlock {
		var picked []*CanonicalBlock
		pre := true
		for _, block := range f.Blocks {
			if block.Type == BlockTypePayload {
				pre = false
				continue
			}
			if pre == before {
				copied := *block
				copied.Flags &^= BlockLast
				picked = append(picked, &copied)
			}
		}
		return picked
	}

	rebuilt.Blocks = append(rebuilt.Blocks, blocksAround(ordered[0], true)...)
	rebuilt.Blocks = append(rebuilt.Blocks, &CanonicalBlock{
		Type: BlockTypePayload,
		Data: adu,
	})
	rebuilt.Blocks = append(rebuilt.Blocks, blocksAround(ordered[len(ordered)-1], false)...)
	rebuilt.Blocks[len(rebuilt.Blocks)-1].Flags |= BlockLast

	if err := rebuilt.Validate(); err != nil {
		return nil, err
	}
	return rebuilt, nil
}
