package rhc

import "fmt"

// The decompressor.
//
// CCSDS 124.0-B-1 specifies the compressor and nothing else: its normative
// sections are inputs (§3), mask update (§4) and encoder (§5), and its
// conformance list (annex A2.2.1) has five items, all of them encoder items.
// There is no decoder section to transcribe. What follows is the encoder run
// backwards, which the standard promises is possible — §2.1 lists exactly what
// a decompressor needs:
//
//	a) the last successfully reconstructed binary vector in the series;
//	b) a mask synchronized with the one used to compress the input;
//	c) the unpredictable bit values associated with the input.
//
// So the state here is (a) and (b), and every output vector carries (c) along
// with enough to keep (b) in step.
//
// # Loss
//
// §2.2 draws a line this type has to respect: the standard "describes a
// mechanism to ensure that the compressor and decompressor remain
// synchronized in the event of the loss of a configurable number of sequential
// output binary vectors. However, it does not provide a mechanism for
// identifying the number of sequential output binary vectors that were lost."
//
// Detecting loss is therefore the caller's, and §2.2 suggests how: packet
// sequence counters, if the outputs travel in space packets. When the caller
// notices a gap it calls NotifyLoss, and only then can this type tell whether
// the next output reaches back far enough. Without that call a decompressor
// cannot know a gap happened, and will happily produce wrong bytes — which is
// a property of the standard, not of this code, and the reason NotifyLoss
// exists.

// Decompressor reconstructs a stream compressed by Compressor.
//
// It is not safe for concurrent use.
type Decompressor struct {
	config Config

	// mask is the mask as last known, M_{t-1}.
	mask Vector
	// maskKnown is false until an output carrying the whole mask arrives.
	maskKnown bool

	// previous is the last reconstructed input vector.
	previous Vector
	// dataKnown is false until an uncompressed output arrives.
	dataKnown bool

	// pendingLoss counts output vectors the caller has reported missing since
	// the last one that decoded.
	pendingLoss int
}

// NewDecompressor prepares a decompressor.
//
// The configuration's VectorLength must match the compressor's. §3.3.2's note
// lists the parameters that need not be known in advance — M_0, R_t and the
// three flags — and F is deliberately not among them.
func NewDecompressor(config Config) (*Decompressor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	d := &Decompressor{config: config}
	d.Reset()
	return d, nil
}

// Reset discards all state. The next output vector must carry the whole mask
// and the whole input for decompression to resume, which §3.3.2 guarantees a
// compressor's own first output does.
func (d *Decompressor) Reset() {
	d.mask = NewVector(d.config.VectorLength)
	d.maskKnown = false
	d.previous = NewVector(d.config.VectorLength)
	d.dataKnown = false
	d.pendingLoss = 0
}

// NotifyLoss tells the decompressor that count output vectors were lost before
// the next one it will be given.
//
// Call it when a sequence counter shows a gap. Until an output arrives whose
// effective robustness level covers the gap — or which carries the whole mask
// and the whole input — Decompress will return an error rather than a vector
// it cannot vouch for.
func (d *Decompressor) NotifyLoss(count int) {
	if count > 0 {
		d.pendingLoss += count
	}
}

// Synchronized reports whether the decompressor can reconstruct the next
// output vector.
func (d *Decompressor) Synchronized() bool {
	return d.maskKnown && d.dataKnown && d.pendingLoss == 0
}

// Mask returns a copy of the mask as last known, for inspection.
func (d *Decompressor) Mask() Vector { return d.mask.Clone() }

// decoded holds what one output vector said, before any of it is committed.
//
// Parsing fills this out completely and only then does Decompress touch the
// decompressor's state. That ordering is the whole defence against a malformed
// output poisoning a stream that was decoding fine: a parse that fails leaves
// nothing changed.
type decoded struct {
	window       Vector
	effective    int
	maskValues   []bool
	haveValues   bool
	changedTwice bool
	dFlag        bool
	sendMask     bool
	uncompressed bool
	wholeMask    Vector
	haveMask     bool
	extracted    []bool
	selector     Vector
	wholeInput   Vector
	haveInput    bool
}

// Decompress consumes one output vector and returns the input vector it
// encodes.
//
// bitLen is the true length of the output vector in bits, as Compress
// returned it. Pass zero to read to the end of the slice, which works when the
// vector was carried alone in its own frame.
func (d *Decompressor) Decompress(data []byte, bitLen int) ([]byte, error) {
	if bitLen <= 0 {
		bitLen = len(data) * 8
	}
	r := NewBitReaderN(data, bitLen)

	result, err := d.parse(r)
	if err != nil {
		// An output vector that could not be parsed is an output vector that
		// did not arrive, as far as the state is concerned: whatever mask
		// changes and bit values it carried are gone. Counting it widens the
		// gap the next output has to reach across, which is the difference
		// between refusing and quietly reconstructing from a mask that has
		// drifted.
		d.pendingLoss++
		return nil, err
	}

	// Everything parsed. Decide whether the state allows reconstruction
	// before changing any of it.
	if err := d.checkRecoverable(result); err != nil {
		d.pendingLoss++
		return nil, err
	}

	return d.commit(result)
}

// parse reads one output vector without touching the decompressor's state.
func (d *Decompressor) parse(r *BitReader) (*decoded, error) {
	length := d.config.VectorLength
	out := &decoded{}

	// h_t: RLE(X_t) first.
	window, err := ReadRLE(r, length)
	if err != nil {
		return nil, err
	}
	out.window = window

	effective, err := r.ReadBits(4)
	if err != nil {
		return nil, err
	}
	out.effective = int(effective)

	// e_t, k_t and c_t are present only when the encoder had something to say
	// about mask values: equations 18 to 20.
	if out.effective > 0 && !window.IsZero() {
		eBit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}
		if eBit {
			out.haveValues = true
			out.maskValues = make([]bool, window.Weight())
			for i := range out.maskValues {
				bit, err := r.ReadBit()
				if err != nil {
					return nil, err
				}
				out.maskValues[i] = bit
			}
			out.changedTwice, err = r.ReadBit()
			if err != nil {
				return nil, err
			}
		}
	}

	out.dFlag, err = r.ReadBit()
	if err != nil {
		return nil, err
	}

	// q_t, equation 21.
	if !out.dFlag {
		out.sendMask, err = r.ReadBit()
		if err != nil {
			return nil, err
		}
		if out.sendMask {
			transitions, err := ReadRLE(r, length)
			if err != nil {
				return nil, err
			}
			out.wholeMask = maskFromTransitions(transitions.Reverse())
			out.haveMask = true
		}
	}

	// The mask this output describes, which the selector for u_t depends on.
	mask, maskOK := d.projectMask(out)

	// u_t, equation 22.
	if out.dFlag {
		// r-dot_t is zero, known from the last bit of h_t.
		if !maskOK {
			return nil, ErrMaskUnavailable
		}
		out.selector = selectorFor(mask, out.window, out.changedTwice)
		out.extracted, err = readBits(r, out.selector.Weight())
		if err != nil {
			return nil, err
		}
		return out, nil
	}

	out.uncompressed, err = r.ReadBit()
	if err != nil {
		return nil, err
	}

	if out.uncompressed {
		count, terminator, err := ReadCount(r)
		if err != nil {
			return nil, err
		}
		if terminator {
			return nil, fmt.Errorf("%w: expected a length, got the run-length terminator",
				ErrInvalidCount)
		}
		if count != length {
			return nil, fmt.Errorf("%w: the output says %d bits, the configuration says %d",
				ErrVectorLengthMismatch, count, length)
		}
		bits, err := readBits(r, length)
		if err != nil {
			return nil, err
		}
		out.wholeInput = vectorOf(bits)
		out.haveInput = true
		return out, nil
	}

	if !maskOK {
		return nil, ErrMaskUnavailable
	}
	out.selector = selectorFor(mask, out.window, out.changedTwice)
	out.extracted, err = readBits(r, out.selector.Weight())
	if err != nil {
		return nil, err
	}
	return out, nil
}

// projectMask works out the mask this output vector describes, without
// committing it.
func (d *Decompressor) projectMask(out *decoded) (Vector, bool) {
	if out.haveMask {
		return out.wholeMask, true
	}
	if !d.maskKnown {
		return Vector{}, false
	}
	return d.applyChanges(d.mask, out), true
}

// applyChanges folds an output's mask-change information into a known mask.
//
// Three cases, from equations 16 to 19:
//
//   - nothing changed, so the mask stands;
//   - V_t is zero, so X_t is exactly this cycle's change vector and the mask
//     is the old one flipped at those positions;
//   - otherwise X_t is an OR across several cycles, where flipping would be
//     wrong, so the encoder sent the resulting mask values instead.
func (d *Decompressor) applyChanges(mask Vector, out *decoded) Vector {
	if out.window.IsZero() {
		return mask.Clone()
	}

	if out.effective == 0 {
		// X_t = <D_t>, so un-reversing gives the change vector itself.
		return mask.XOR(out.window.Reverse())
	}

	updated := mask.Clone()
	// The window is reversed, and so were the mask values taken from it, so
	// walking both in the same order lines them up. Position i of the window
	// is position F-1-i of the mask.
	valueIndex := 0
	for i := range out.window.Len() {
		if !out.window.Get(i) {
			continue
		}
		position := out.window.Len() - 1 - i

		if out.haveValues {
			// Equation 17 extracted the inverted mask, so a one here means
			// the position ended up predictable.
			updated.Set(position, !out.maskValues[valueIndex])
			valueIndex++
			continue
		}
		// e_t was zero: every extracted bit of the inverted mask was zero, so
		// every changed position ended up unpredictable.
		updated.Set(position, true)
	}
	return updated
}

// selectorFor returns the positions whose values u_t carries, per equation 22.
func selectorFor(mask, window Vector, changedTwice bool) Vector {
	if changedTwice {
		return window.Reverse().OR(mask)
	}
	return mask
}

// checkRecoverable decides whether this output can be turned into a vector.
func (d *Decompressor) checkRecoverable(out *decoded) error {
	// An uncompressed output carries everything, so it always recovers the
	// data whatever came before.
	if out.haveInput {
		return nil
	}

	if !d.dataKnown {
		return ErrNotSynchronized
	}
	if !d.maskKnown && !out.haveMask {
		return ErrMaskUnavailable
	}

	// A reported gap is survivable only when this output reaches back across
	// it. §2.1 puts the bound on the effective robustness level: "the mask can
	// be synchronized even if the number of consecutive output binary vectors
	// lost immediately before this output bit vector is equal to, or less
	// than, the effective robustness level."
	//
	// Carrying the whole mask is not enough on its own. That fixes item (b) of
	// §2.1's list and leaves item (a) — the last reconstructed vector — as
	// stale as it was, and the values needed to bring it up to date went out
	// in the outputs that were lost. Only an uncompressed output, handled
	// above, repairs both.
	if d.pendingLoss > out.effective {
		return fmt.Errorf("%w: %d output vectors were lost but this one reaches back only %d",
			ErrNotSynchronized, d.pendingLoss, out.effective)
	}
	return nil
}

// commit applies a parsed output to the decompressor's state and returns the
// reconstructed input vector.
func (d *Decompressor) commit(out *decoded) ([]byte, error) {
	// The mask first, since reconstruction reads it.
	switch {
	case out.haveMask:
		d.mask = out.wholeMask
		d.maskKnown = true
	case d.maskKnown:
		d.mask = d.applyChanges(d.mask, out)
	}

	var current Vector
	if out.haveInput {
		current = out.wholeInput
	} else {
		// Every predictable position keeps the value it had; the extracted
		// bits fill in the rest. That is the promise §2.1 makes about the
		// mask, run backwards.
		current = d.previous.Clone()
		index := 0
		for i := range out.selector.Len() {
			if !out.selector.Get(i) {
				continue
			}
			current.Set(i, out.extracted[index])
			index++
		}
	}

	d.previous = current
	d.dataKnown = true
	d.pendingLoss = 0

	return current.Bytes(), nil
}

// maskFromTransitions inverts the transition coding of equation 21.
//
// The encoder sent M XOR M<<, so bit i of the transition vector is
// M[i] XOR M[i+1], with the last bit being M[F-1] alone. Reading backwards
// from there recovers the mask.
func maskFromTransitions(transitions Vector) Vector {
	length := transitions.Len()
	mask := NewVector(length)
	if length == 0 {
		return mask
	}

	mask.Set(length-1, transitions.Get(length-1))
	for i := length - 2; i >= 0; i-- {
		mask.Set(i, transitions.Get(i) != mask.Get(i+1))
	}
	return mask
}

// readBits reads n single bits.
func readBits(r *BitReader, n int) ([]bool, error) {
	out := make([]bool, n)
	for i := range out {
		bit, err := r.ReadBit()
		if err != nil {
			return nil, err
		}
		out[i] = bit
	}
	return out, nil
}

// vectorOf builds a vector from a bit slice.
func vectorOf(bits []bool) Vector {
	v := NewVector(len(bits))
	for i, bit := range bits {
		v.Set(i, bit)
	}
	return v
}
