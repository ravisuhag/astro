package ocsc

// Slice cuts a stream of Sync-Marked Transfer Frames into information blocks
// of k binary digits, per CCSDS 142.0-B-1 §3.4.
//
// §3.4.2.1.1: at transmission closure the output is zero-filled with the
// minimum number of zeros needed to make its length a multiple of k. So the
// final block is always full, padded if it has to be.
//
// Slice is a batch call: the end of its input is treated as transmission
// closure, so the fill lands in this call's final block. To slice one
// transmission across several calls, use a Conditioner, which carries the
// partial block between calls and fills only at explicit Close.
//
// Note the frames are treated as one continuous bit stream, not as separate
// units: a block can straddle a frame boundary. Figure 3-3 shows exactly that.
// The ASM on each frame is what lets the receiver find the boundaries again.
func Slice(smtfs *BitString, rate CodeRate) ([]*BitString, error) {
	if !rate.Valid() {
		return nil, ErrInvalidCodeRate
	}
	k := rate.InformationBlockSize()

	if smtfs == nil || smtfs.Len() == 0 {
		return nil, nil
	}

	// §3.4.2.1.1: round up to a whole number of blocks.
	blocks := (smtfs.Len() + k - 1) / k

	out := make([]*BitString, 0, blocks)
	for i := 0; i < blocks; i++ {
		start := i * k
		end := start + k

		block := NewBitString(k)
		for j := start; j < end && j < smtfs.Len(); j++ {
			block.SetBit(j-start, smtfs.Bit(j))
		}
		// Bits past the input are already zero, which is the zero fill.
		out = append(out, block)
	}
	return out, nil
}

// Unslice concatenates information blocks back into one bit stream.
//
// The zero fill the slicer added is still there: only the receiver's frame
// synchronization, hunting for the ASM, can tell padding from data. This
// returns the bits as they are.
func Unslice(blocks []*BitString) *BitString {
	out := NewBitString(0)
	for _, b := range blocks {
		if b == nil {
			continue
		}
		out.AppendBits(b, b.Len())
	}
	return out
}
