package bp

import "sort"

// Fragment splits a bundle into pieces whose payloads are at most maxPayload
// octets each (RFC 9171 clause 5.8).
//
// Each fragment is a bundle in its own right, sharing the original's source
// node ID and creation timestamp — together those identify which original a
// fragment belongs to. The fragment flag is set on every piece, the offset and
// total length are filled in, and every checksum is recomputed, because the
// primary blocks now differ.
//
// Extension blocks follow clause 5.8's two replication rules: a block whose
// "replicate in every fragment" flag is set goes into all of them, and the
// offset-zero fragment additionally carries every other extension block.
func (b *Bundle) Fragment(maxPayload int) ([]*Bundle, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if maxPayload < 1 {
		return nil, ErrFragmentSizeTooSmall
	}
	if b.Primary.Flags.Has(FlagMustNotFragment) {
		return nil, ErrMustNotFragment
	}

	payload := b.Payload()
	if len(payload) <= maxPayload {
		return []*Bundle{b}, nil
	}

	// A bundle that is already a fragment carries a slice of the original ADU,
	// so its pieces are offset from where this one sits, not from zero.
	baseOffset := uint64(0)
	total := uint64(len(payload))
	if b.Primary.Flags.Has(FlagIsFragment) {
		baseOffset = b.Primary.FragmentOffset
		total = b.Primary.TotalADULength
	}

	var fragments []*Bundle
	for start := 0; start < len(payload); start += maxPayload {
		end := start + maxPayload
		if end > len(payload) {
			end = len(payload)
		}

		primary := *b.Primary
		primary.Flags |= FlagIsFragment
		primary.FragmentOffset = baseOffset + uint64(start)
		primary.TotalADULength = total

		fragment := &Bundle{Primary: &primary}
		for _, blk := range b.Blocks {
			if blk.Type == BlockTypePayload {
				continue
			}
			// Clause 5.8: the replicate flag puts a block in every fragment;
			// everything else rides along only on the offset-zero piece.
			if blk.Flags.Has(BlockFlagReplicateInEveryFragment) || primary.FragmentOffset == 0 {
				copied := *blk
				copied.Data = append([]byte(nil), blk.Data...)
				fragment.Blocks = append(fragment.Blocks, &copied)
			}
		}

		piece := append([]byte(nil), payload[start:end]...)
		fragment.Blocks = append(fragment.Blocks, NewPayloadBlock(piece))

		if err := fragment.Validate(); err != nil {
			return nil, err
		}
		fragments = append(fragments, fragment)
	}
	return fragments, nil
}

// Reassemble rebuilds the original bundle from a set of fragments
// (RFC 9171 clause 5.9).
//
// Fragments may arrive out of order, and they may overlap: clause 5.8 allows
// separate fragmentation episodes in different parts of the network to produce
// overlapping slices of the same payload. So this works in "material extents"
// — the parts of each fragment that add something not already held — and
// reports ErrIncompleteReassembly until they cover the whole application data
// unit with no gap.
//
// Every fragment must share a source node ID and creation timestamp; a set
// that mixes two originals is refused rather than silently spliced.
func Reassemble(fragments []*Bundle) (*Bundle, error) {
	if len(fragments) == 0 {
		return nil, ErrNoFragments
	}

	var (
		first      *Bundle
		total      uint64
		haveTotal  bool
		extents    []extent
		zeroOffset *Bundle
	)

	for _, f := range fragments {
		if err := f.Validate(); err != nil {
			return nil, err
		}
		if !f.Primary.Flags.Has(FlagIsFragment) {
			return nil, ErrNotAFragment
		}

		if first == nil {
			first = f
			total = f.Primary.TotalADULength
			haveTotal = true
		} else if f.Primary.Source != first.Primary.Source ||
			f.Primary.Timestamp != first.Primary.Timestamp {
			// Clause 5.9 keys reassembly on source node ID and creation
			// timestamp together. Anything else belongs to another original.
			return nil, ErrFragmentsDoNotMatch
		} else if haveTotal && f.Primary.TotalADULength != total {
			return nil, ErrFragmentsDoNotMatch
		}

		payload := f.Payload()
		off := f.Primary.FragmentOffset
		if off+uint64(len(payload)) > total {
			return nil, ErrFragmentPastEnd
		}
		if off == 0 {
			zeroOffset = f
		}
		extents = append(extents, extent{offset: off, data: payload})
	}

	// Lay the pieces down in order. Later writes of an overlapping region
	// agree with earlier ones by construction: clause 5.8 requires every
	// fragment's payload to be the original's bytes at that offset.
	adu := make([]byte, total)
	covered := make([]bool, total)
	sort.Slice(extents, func(i, j int) bool { return extents[i].offset < extents[j].offset })
	for _, e := range extents {
		copy(adu[e.offset:], e.data)
		for i := e.offset; i < e.offset+uint64(len(e.data)); i++ {
			covered[i] = true
		}
	}
	for _, ok := range covered {
		if !ok {
			return nil, ErrIncompleteReassembly
		}
	}

	if zeroOffset == nil {
		return nil, ErrIncompleteReassembly
	}

	// Clause 5.9: the reassembled unit replaces the payload of the fragment
	// holding offset zero, and that bundle stops being a fragment.
	primary := *zeroOffset.Primary
	primary.Flags &^= FlagIsFragment
	primary.FragmentOffset = 0
	primary.TotalADULength = 0

	out := &Bundle{Primary: &primary}
	for _, blk := range zeroOffset.Blocks {
		if blk.Type == BlockTypePayload {
			continue
		}
		copied := *blk
		copied.Data = append([]byte(nil), blk.Data...)
		out.Blocks = append(out.Blocks, &copied)
	}
	out.Blocks = append(out.Blocks, NewPayloadBlock(adu))

	if err := out.Validate(); err != nil {
		return nil, err
	}
	return out, nil
}

// extent is one fragment's payload and where it sits in the original unit.
type extent struct {
	offset uint64
	data   []byte
}
