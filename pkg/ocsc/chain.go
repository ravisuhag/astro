package ocsc

// The conditioning chain, per CCSDS 142.0-B-1 clause 3.3 through clause 3.7, and the
// receive-side frame recovery of clause 3.14 and clause 3.15.
//
// These functions run the whole deterministic front half in one call, which is
// how a ground pipeline actually uses it.

// AttachTermination appends the two zero digits of clause 3.7, producing an SCPPM
// encoder input block of k-hat digits.
func AttachTermination(block *BitString) *BitString {
	out := NewBitString(0)
	out.AppendBits(block, block.Len())
	for i := 0; i < TerminationBits; i++ {
		out.Append(0)
	}
	return out
}

// StripTermination removes the two termination digits, checking they are zero.
func StripTermination(block *BitString) (*BitString, error) {
	if block == nil || block.Len() < TerminationBits {
		return nil, ErrDataTooShort
	}
	for i := block.Len() - TerminationBits; i < block.Len(); i++ {
		if block.Bit(i) != 0 {
			return nil, ErrInvalidTermination
		}
	}
	return block.Slice(0, block.Len()-TerminationBits), nil
}

// Condition runs the full send-side chain over a run of transfer frames,
// returning SCPPM encoder input blocks.
//
// Each frame gets a sync marker (clause 3.3), the marked frames are sliced into
// k-digit blocks with zero fill (clause 3.4), each block is randomized (clause 3.5), gets
// a CRC (clause 3.6), and gets two termination digits (clause 3.7).
//
// Condition is a batch call: it treats its input as a complete transmission,
// so the call itself is the transmission closure of clause 3.4.2.1.1 and the final
// block is zero-filled. Two Condition calls are two transmissions, not one.
// To condition one transmission across several calls, carrying partial blocks
// between them, use a Conditioner.
//
// What comes back is what the SCPPM encoder would take as input. This package
// stops there.
func Condition(frames [][]byte, rate CodeRate) ([]*BitString, error) {
	c, err := NewConditioner(rate)
	if err != nil {
		return nil, err
	}
	blocks, err := c.Push(frames...)
	if err != nil {
		return nil, err
	}
	tail, err := c.Close()
	if err != nil {
		return nil, err
	}
	return append(blocks, tail...), nil
}

// RecoveredFrame is one transfer frame delivered by Recover, carrying the two
// per-frame service parameters of annex B.
type RecoveredFrame struct {
	// Data is the transfer frame.
	Data []byte

	// Valid is the Quality Indicator of clause 3.14.2: true when every block
	// carrying any of this frame's bits verified its CRC, false when the
	// frame was recovered from one or more bad blocks.
	Valid bool

	// Gap is the Sequence Indicator of clause 3.15: false ('zero') when this frame
	// is the direct successor of the previous one, true ('one') when a gap
	// was detected before it.
	Gap bool
}

// Recover reverses Condition: it takes SCPPM encoder input blocks, checks each
// CRC, and returns the transfer frames.
//
// frameLength is the transfer frame size in octets, at most 65536 (clause 5.2). It
// is needed because the slicer's zero fill (clause 3.4.2.1.1) is indistinguishable
// from frame data once it is in the stream: nothing in the conditioning chain
// records where the real data stopped. Frame length is a managed parameter
// fixed for a mission phase, so the receiver always knows it. Pass zero to
// take each frame as running to the next sync marker, which leaves the
// trailing fill attached to the last one.
//
// A block whose CRC fails is reported through badBlocks rather than silently
// dropped, and every frame recovered from a failing block comes back with
// Valid false, because clause 3.14.2 marks frames recovered from an incorrectly
// decoded codeword as invalid rather than discarding them outright. Each
// frame's Gap field is the Sequence Indicator of clause 3.15.
func Recover(blocks []*BitString, rate CodeRate, frameLength int) (frames []RecoveredFrame, badBlocks []int, err error) {
	if !rate.Valid() {
		return nil, nil, ErrInvalidCodeRate
	}
	if frameLength < 0 || frameLength > MaxFrameLength {
		return nil, nil, ErrFrameTooLong
	}
	kHat := rate.EncoderInputSize()

	stream := NewBitString(0)
	for i, block := range blocks {
		if block == nil || block.Len() != kHat {
			return nil, nil, ErrInvalidBlockLength
		}

		withCRC, err := StripTermination(block)
		if err != nil {
			return nil, nil, err
		}

		randomized, ok := VerifyCRC(withCRC)
		if !ok {
			badBlocks = append(badBlocks, i)
			// The block's bits are still appended: a frame straddling this
			// block would otherwise shift every frame after it.
		}
		stream.AppendBits(Derandomize(randomized), randomized.Len())
	}

	spans := splitFramesAtASM(stream, frameLength)

	k := rate.InformationBlockSize()
	frames = make([]RecoveredFrame, 0, len(spans))
	for _, s := range spans {
		body := stream.Slice(s.start+ASMBits, s.end)
		octets := body.Len() / 8
		frame := make([]byte, octets)
		copy(frame, body.Bytes()[:octets])

		// Clause 3.14.2: the frame is invalid if any of the blocks carrying its
		// bits (sync marker included) failed verification. Block i holds
		// stream bits [i*k, (i+1)*k).
		valid := true
		for _, b := range badBlocks {
			if b*k < s.end && (b+1)*k > s.start {
				valid = false
				break
			}
		}
		frames = append(frames, RecoveredFrame{Data: frame, Valid: valid, Gap: s.gap})
	}
	return frames, badBlocks, nil
}

// frameSpan is one located frame: the bit range of its SMTF in the stream,
// marker included, and whether a gap preceded it (clause 3.15).
type frameSpan struct {
	start int // bit offset of the sync marker
	end   int // one past the last frame bit
	gap   bool
}

// splitFramesAtASM locates transfer frames in a conditioned bit stream.
//
// With frameLength set, each frame is exactly that many octets after its
// marker, which is what discards the slicer's trailing zero fill, and it is
// what lets the receiver stay locked (clause 3.14.1): after a frame, the next
// marker is expected immediately after it, checked at that one offset only.
// This is what keeps frame data that happens to contain the marker pattern
// from producing spurious frames. Only when the expected marker is missing
// does the search fall back to hunting every bit offset, and a frame found
// that way carries a Sequence Indicator of one (clause 3.15).
//
// With frameLength zero there is nothing to lock to: every marker is found by
// hunting, a frame runs to the next marker or the end of the stream, and the
// last one carries whatever fill followed it.
func splitFramesAtASM(stream *BitString, frameLength int) []frameSpan {
	if frameLength == 0 {
		return splitFramesByHunting(stream)
	}

	frameBits := ASMBits + frameLength*8
	var spans []frameSpan
	pos, gapPending := 0, false
	for pos+ASMBits <= stream.Len() {
		start := pos
		if !matchASMAt(stream, pos) {
			// Lock lost (or never held): resync by hunting bit-by-bit.
			start = huntASM(stream, pos)
			if start < 0 {
				break
			}
			gapPending = true
		}
		end := start + frameBits
		if end > stream.Len() {
			// Not enough bits left for a whole frame: the stream was cut
			// short, so this is not a deliverable frame.
			break
		}
		spans = append(spans, frameSpan{start: start, end: end, gap: gapPending})
		gapPending = false
		pos = end
	}
	return spans
}

// splitFramesByHunting is the frameLength-zero path: hunt every marker.
func splitFramesByHunting(stream *BitString) []frameSpan {
	var starts []int
	for i := 0; i+ASMBits <= stream.Len(); i++ {
		if matchASMAt(stream, i) {
			starts = append(starts, i)
			i += ASMBits - 1 // a marker cannot overlap itself
		}
	}

	var spans []frameSpan
	gapPending := false
	for i, start := range starts {
		end := stream.Len()
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if i == 0 && start != 0 {
			// Bits before the first marker mean something was lost.
			gapPending = true
		}
		if (end-start-ASMBits)/8 == 0 {
			// No whole octet of frame body: not a deliverable frame, and its
			// absence is a discontinuity for whatever follows.
			gapPending = true
			continue
		}
		spans = append(spans, frameSpan{start: start, end: end, gap: gapPending})
		gapPending = false
	}
	return spans
}

// huntASM returns the first bit offset at or after from where the sync marker
// matches, or -1.
func huntASM(stream *BitString, from int) int {
	for i := from; i+ASMBits <= stream.Len(); i++ {
		if matchASMAt(stream, i) {
			return i
		}
	}
	return -1
}

// matchASMAt reports whether the sync marker sits at bit offset i.
func matchASMAt(stream *BitString, i int) bool {
	for j := 0; j < ASMBits; j++ {
		want := ASM[j/8] >> uint(7-j%8) & 1
		if stream.Bit(i+j) != want {
			return false
		}
	}
	return true
}
