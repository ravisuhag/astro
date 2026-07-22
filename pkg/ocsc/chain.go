package ocsc

// The conditioning chain, per CCSDS 142.0-B-1 §3.3 through §3.7.
//
// These functions run the whole deterministic front half in one call, which is
// how a ground pipeline actually uses it.

// AttachTermination appends the two zero digits of §3.7, producing an SCPPM
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
// Each frame gets a sync marker (§3.3), the marked frames are sliced into
// k-digit blocks with zero fill (§3.4), each block is randomized (§3.5), gets
// a CRC (§3.6), and gets two termination digits (§3.7).
//
// What comes back is what the SCPPM encoder would take as input. This package
// stops there.
func Condition(frames [][]byte, rate CodeRate) ([]*BitString, error) {
	if !rate.Valid() {
		return nil, ErrInvalidCodeRate
	}

	// §3.3: mark every frame, then treat the result as one bit stream.
	stream := NewBitString(0)
	for _, frame := range frames {
		smtf, err := AttachASM(frame)
		if err != nil {
			return nil, err
		}
		stream.AppendBits(smtf, smtf.Len())
	}

	blocks, err := Slice(stream, rate)
	if err != nil {
		return nil, err
	}

	out := make([]*BitString, 0, len(blocks))
	for _, block := range blocks {
		conditioned := AttachTermination(AttachCRC(Randomize(block)))
		out = append(out, conditioned)
	}
	return out, nil
}

// Recover reverses Condition: it takes SCPPM encoder input blocks, checks each
// CRC, and returns the transfer frames.
//
// frameLength is the transfer frame size in octets. It is needed because the
// slicer's zero fill (§3.4.2.1.1) is indistinguishable from frame data once it
// is in the stream: nothing in the conditioning chain records where the real
// data stopped. Frame length is a managed parameter fixed for a mission phase,
// so the receiver always knows it. Pass zero to take each frame as running to
// the next sync marker, which leaves the trailing fill attached to the last one.
//
// A block whose CRC fails is reported through badBlocks rather than silently
// dropped, because §2 marks frames recovered from an incorrectly decoded
// codeword as invalid rather than discarding them outright.
func Recover(blocks []*BitString, rate CodeRate, frameLength int) (frames [][]byte, badBlocks []int, err error) {
	if !rate.Valid() {
		return nil, nil, ErrInvalidCodeRate
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

	frames = splitFramesAtASM(stream, frameLength)
	return frames, badBlocks, nil
}

// splitFramesAtASM recovers transfer frames from a conditioned bit stream by
// hunting for sync markers.
//
// With frameLength set, each frame is exactly that many octets after its
// marker, which is what discards the slicer's trailing zero fill. With zero,
// a frame runs to the next marker or the end of the stream, and the last one
// carries whatever fill followed it.
func splitFramesAtASM(stream *BitString, frameLength int) [][]byte {
	var starts []int
	for i := 0; i+ASMBits <= stream.Len(); i++ {
		if matchASMAt(stream, i) {
			starts = append(starts, i)
			i += ASMBits - 1 // a marker cannot overlap itself
		}
	}
	if len(starts) == 0 {
		return nil
	}

	var out [][]byte
	for i, start := range starts {
		end := stream.Len()
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if frameLength > 0 {
			// A fixed frame length trims the zero fill the slicer added.
			if fixed := start + ASMBits + frameLength*8; fixed <= end {
				end = fixed
			} else {
				// Not enough bits left for a whole frame: the stream was cut
				// short, so this is not a deliverable frame.
				continue
			}
		}

		body := stream.Slice(start+ASMBits, end)
		// A transfer frame is a whole number of octets.
		octets := body.Len() / 8
		if octets == 0 {
			continue
		}
		frame := make([]byte, octets)
		copy(frame, body.Bytes()[:octets])
		out = append(out, frame)
	}
	return out
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
