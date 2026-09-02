package ocsc

// Conditioner is the streaming form of Condition.
//
// The NOTE under clause 3.2 says encoding may be performed in a streaming fashion:
// not all transfer frames of a session need be available when encoding
// begins, and their total number need not be known a priori. A Conditioner
// does exactly that. Each Push marks the frames it is given (clause 3.3), appends
// them to the carried bit stream, and returns every information block that is
// now complete, conditioned (clause 3.5-clause 3.7). Bits short of a full block stay
// buffered for the next Push, no fill is inserted mid-stream, because
// Clause 3.4.2.1.1 permits zero fill only at transmission closure.
//
// Closure is explicit: Close zero-fills whatever remains into a final block
// and ends the stream. After Close, the Conditioner refuses further use.
//
// A Conditioner is not safe for concurrent use.
type Conditioner struct {
	rate      CodeRate
	remainder *BitString
	closed    bool
}

// NewConditioner returns a Conditioner for one transmission at the given rate.
func NewConditioner(rate CodeRate) (*Conditioner, error) {
	if !rate.Valid() {
		return nil, ErrInvalidCodeRate
	}
	return &Conditioner{rate: rate, remainder: NewBitString(0)}, nil
}

// Push appends transfer frames to the transmission and returns the SCPPM
// encoder input blocks completed by them, in order. A call may return no
// blocks: the bits are buffered, not lost.
func (c *Conditioner) Push(frames ...[]byte) ([]*BitString, error) {
	if c.closed {
		return nil, ErrConditionerClosed
	}
	for _, frame := range frames {
		smtf, err := AttachASM(frame)
		if err != nil {
			return nil, err
		}
		c.remainder.AppendBits(smtf, smtf.Len())
	}

	k := c.rate.InformationBlockSize()
	var out []*BitString
	for c.remainder.Len() >= k {
		block := c.remainder.Slice(0, k)
		c.remainder = c.remainder.Slice(k, c.remainder.Len())
		out = append(out, condition(block))
	}
	return out, nil
}

// Close declares transmission closure (clause 3.4.2.1.1): the buffered remainder,
// if any, is zero-filled to a full block, conditioned, and returned. The
// Conditioner cannot be used again afterwards.
func (c *Conditioner) Close() ([]*BitString, error) {
	if c.closed {
		return nil, ErrConditionerClosed
	}
	c.closed = true

	if c.remainder.Len() == 0 {
		return nil, nil
	}
	k := c.rate.InformationBlockSize()
	block := NewBitString(k)
	for i := 0; i < c.remainder.Len(); i++ {
		block.SetBit(i, c.remainder.Bit(i))
	}
	// Bits past the remainder are already zero, which is the zero fill.
	c.remainder = NewBitString(0)
	return []*BitString{condition(block)}, nil
}

// Pending reports how many bits are buffered awaiting a full block.
func (c *Conditioner) Pending() int { return c.remainder.Len() }

// condition runs one k-digit information block through clause 3.5, clause 3.6 and clause 3.7.
func condition(block *BitString) *BitString {
	return AttachTermination(AttachCRC(Randomize(block)))
}
