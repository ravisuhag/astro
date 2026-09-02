// Package rhc implements Robust Compression of Fixed-Length Housekeeping Data
// per CCSDS 124.0-B-1, the POCKET+ algorithm.
//
// Housekeeping telemetry barely changes. A voltage reading, a mode word, a
// thermistor count: most bits are the same in this packet as in the last one,
// and this algorithm exists to send only the ones that are not.
//
// It keeps a mask, one bit per position, saying whether that position is
// predictable — unchanged since the last packet — or not. Predictable
// positions are not sent at all; the decompressor already knows them. Each
// output vector carries three things (§5.3.1):
//
//	h_t  what changed in the mask lately, and how far back that reaches
//	q_t  the whole mask, when asked for
//	u_t  the values of the unpredictable bits, or the whole input when asked
//
// # Using it
//
//	config := rhc.Config{
//	    VectorLength:         512,
//	    Robustness:           3,
//	    NewMaskInterval:      32,
//	    SendMaskInterval:     16,
//	    UncompressedInterval: 16,
//	}
//	compressor, err := rhc.NewCompressor(config)
//	coded, bitLen, err := compressor.Compress(packet)
//
//	decompressor, err := rhc.NewDecompressor(config)
//	packet, err := decompressor.Decompress(coded, bitLen)
//
// Both types hold state across the whole stream and neither is safe for
// concurrent use. One stream, one pair, one goroutine.
//
// # Loss
//
// The algorithm is built for a lossy link, and it is worth being precise about
// what that buys. Each output says how many outputs may have been lost
// immediately before it without stopping it being decoded — its effective
// robustness level. Set Robustness to the floor you want.
//
// But the decompressor cannot tell that anything was lost. §2.2 says so
// outright: the standard "does not provide a mechanism for identifying the
// number of sequential output binary vectors that were lost", and suggests
// packet sequence counters as the mission's answer. So the caller must notice
// gaps and call NotifyLoss. Without that, a decompressor fed a stream with
// holes in it will reconstruct wrong bytes and not know.
//
// Nor are there sync markers: a foreign or corrupt vector that happens to
// parse will be taken for a real one. Framing is the mission's job too.
//
// After a reported gap, recovery normally trusts the next output's
// self-declared effective robustness level. Config.Strict withdraws that
// trust: a strict decompressor then accepts nothing but an uncompressed
// output.
//
// # What is here
//
// The compressor is the whole of the standard's normative content: inputs
// (§3), mask update (§4) and encoder (§5). The standard specifies nothing
// else — there is no decoder section and the conformance list in annex A has
// only encoder items — so the decompressor here is the encoder run backwards,
// which §2.1 lays out the requirements for. See
// docs/content/conformance/rhc.md.
package rhc

import "fmt"

// The compressor of CCSDS 124.0-B-1, sections 3 to 5.
//
// One cycle per input vector, and each cycle is two stages: update the mask
// (§4), then encode (§5). The output is three concatenated binary vectors:
//
//	h_t  what changed in the mask recently, and how far back that covers
//	q_t  the whole mask, when asked for
//	u_t  the unpredictable bit values, or the whole input when asked for
//
// The state that survives between cycles is small: the mask, the build, the
// previous input, and enough history of change vectors and new-mask flags to
// compute how far back this output reaches.

// maxHistory is how many past cycles the encoder must remember.
//
// §5.3.2.2 bounds C_t by min(t,15) - R_t, so the change vectors of the last
// fifteen cycles plus the current one are the most that can ever be needed.
// The same bound holds for the new-mask flags, since V_t <= 15.
const maxHistory = 16

// MaxRobustness is the largest robustness level §3.3.2a allows.
const MaxRobustness = 7

// MaxVectorLength is the longest input vector §3.2 allows.
const MaxVectorLength = 1<<16 - 1

// Config fixes what does not change from cycle to cycle.
type Config struct {
	// VectorLength is F, the length of every input vector in bits. §3.2
	// allows 1 to 65535.
	VectorLength int

	// InitialMask is M_0, the mask the compressor starts from (§3.3.1). A nil
	// value means all zeros, which the note under §3.3.1 calls "often a
	// reasonable default": every position starts out predictable.
	InitialMask Vector

	// Robustness is R_t, the minimum required effective robustness level
	// (§3.3.2a), 0 to 7. It is how many consecutive output vectors may be
	// lost before this one and still leave the mask recoverable.
	//
	// Higher costs bits: the change information in h_t is ORed over R_t+1
	// cycles, so more positions appear in it.
	Robustness int

	// NewMaskInterval is how often to set the new mask flag, in cycles. Zero
	// never sets it.
	//
	// This is policy, not protocol. §3.3.2b makes the flag user-specified at
	// every cycle and says nothing about when to set it. Setting it lets
	// positions go back to being predictable, which is what stops the mask
	// filling up with ones over a long run; how often to pay for that is a
	// mission decision.
	NewMaskInterval int

	// SendMaskInterval is how often to set the send mask flag, in cycles.
	// Zero never sets it beyond what §3.3.2c forces.
	//
	// Also policy. Sending the whole mask lets a decompressor that has lost
	// its place recover the mask without waiting for changes to describe it.
	SendMaskInterval int

	// UncompressedInterval is how often to set the uncompressed flag, in
	// cycles. Zero never sets it beyond what §3.3.2d forces.
	//
	// Also policy, and the one that matters most for recovery: an
	// uncompressed output carries the whole input vector, which is the only
	// thing that restores a decompressor's previous-vector state after a gap.
	UncompressedInterval int

	// Strict makes the decompressor accept nothing but an uncompressed
	// output after a reported loss — NotifyLoss, or an output that failed to
	// parse — even when a later output's effective robustness level claims to
	// reach back across the gap.
	//
	// The point is trust. The standard's recovery gate compares the gap
	// against V_t (§5.3.2.2), a field the output vector declares about
	// itself; nothing in the format lets a decompressor verify it, so a
	// corrupt or hostile vector arriving right after a gap can claim any
	// reach up to 15 and be believed. Strict mode drops that trust and waits
	// for the one output that proves itself by carrying the whole input.
	// The cost is availability: everything between the gap and the next
	// uncompressed output is refused.
	//
	// It has no effect on the compressor, and none on a decompressor that
	// has not been told of any loss.
	Strict bool
}

// Validate checks the configuration against the standard's limits.
func (c Config) Validate() error {
	if c.VectorLength < 1 || c.VectorLength > MaxVectorLength {
		return fmt.Errorf("%w: got %d", ErrInvalidVectorLength, c.VectorLength)
	}
	if c.Robustness < 0 || c.Robustness > MaxRobustness {
		return fmt.Errorf("%w: got %d", ErrInvalidRobustness, c.Robustness)
	}
	if c.InitialMask.Len() != 0 && c.InitialMask.Len() != c.VectorLength {
		return fmt.Errorf("%w: initial mask is %d bits, vector length is %d",
			ErrInvalidPacketLength, c.InitialMask.Len(), c.VectorLength)
	}
	for _, interval := range []int{c.NewMaskInterval, c.SendMaskInterval, c.UncompressedInterval} {
		if interval < 0 {
			return fmt.Errorf("%w: got %d", ErrInvalidInterval, interval)
		}
	}
	return nil
}

// CycleParams are the per-cycle parameters of §3.3.2.
//
// Compress derives these from the Config. CompressWith takes them directly,
// for a caller driving the flags from its own logic — which §2.1 explicitly
// allows: "the decompressor is not required to actively change user defined
// parameters as all the information required for decompression is contained
// in the output bit vectors".
type CycleParams struct {
	// Robustness is R_t (§3.3.2a).
	Robustness int
	// NewMask is the new mask flag, p-dot_t (§3.3.2b).
	NewMask bool
	// SendMask is the send mask flag, f-dot_t (§3.3.2c).
	SendMask bool
	// Uncompressed is the uncompressed flag, r-dot_t (§3.3.2d).
	Uncompressed bool
}

// Compressor holds the state one binary vector stream needs.
//
// It is not safe for concurrent use. One stream, one Compressor, one
// goroutine.
type Compressor struct {
	config Config

	// t is the index of the next input vector.
	t int
	// mask is M_{t-1} between cycles, M_t during one.
	mask Vector
	// build is B_{t-1}, the mask being assembled for the next new-mask flag.
	build Vector
	// previous is I_{t-1}.
	previous Vector
	// hasPrevious is false before the first input.
	hasPrevious bool

	// changes holds recent change vectors D, most recent last, capped at
	// maxHistory. Needed for X_t (§5.3.3.1) and C_t (§5.3.2.2).
	changes []Vector
	// newMaskFlags holds recent values of the new mask flag, most recent
	// last, capped at maxHistory. Needed for c_t (§5.3.3.1 equation 20).
	newMaskFlags []bool

	// forceNewMask, forceSendMask and forceUncompressed make the next cycle
	// set the corresponding flag whatever the interval says.
	forceNewMask      bool
	forceSendMask     bool
	forceUncompressed bool
}

// NewCompressor prepares a compressor.
func NewCompressor(config Config) (*Compressor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	c := &Compressor{config: config}
	c.Reset()
	return c, nil
}

// Reset returns the compressor to its initial state: t = 0, the mask back to
// M_0, the build to zero (§4.1), and no history.
func (c *Compressor) Reset() {
	c.t = 0
	if c.config.InitialMask.Len() == c.config.VectorLength {
		c.mask = c.config.InitialMask.Clone()
	} else {
		c.mask = NewVector(c.config.VectorLength)
	}
	// §4.1: "build is initialized to a zero vector, that is, B_0 = 0".
	c.build = NewVector(c.config.VectorLength)
	c.previous = NewVector(c.config.VectorLength)
	c.hasPrevious = false
	c.changes = nil
	c.newMaskFlags = nil
	c.forceNewMask = false
	c.forceSendMask = false
	c.forceUncompressed = false
}

// ForceNewMask makes the next cycle set the new mask flag.
func (c *Compressor) ForceNewMask() { c.forceNewMask = true }

// ForceSendMask makes the next cycle send the whole mask.
func (c *Compressor) ForceSendMask() { c.forceSendMask = true }

// ForceUncompressed makes the next cycle carry the whole input vector.
//
// This is the recovery lever: a decompressor that has lost its place is
// restored by an uncompressed output and nothing else.
func (c *Compressor) ForceUncompressed() { c.forceUncompressed = true }

// Mask returns a copy of the current mask, for inspection.
func (c *Compressor) Mask() Vector { return c.mask.Clone() }

// Index returns the time index of the next input vector.
func (c *Compressor) Index() int { return c.t }

// paramsFor derives the cycle parameters from the configuration.
func (c *Compressor) paramsFor() CycleParams {
	p := CycleParams{Robustness: c.config.Robustness}

	if c.forceNewMask || interval(c.config.NewMaskInterval, c.t) {
		p.NewMask = true
	}
	if c.forceSendMask || interval(c.config.SendMaskInterval, c.t) {
		p.SendMask = true
	}
	if c.forceUncompressed || interval(c.config.UncompressedInterval, c.t) {
		p.Uncompressed = true
	}
	return p
}

// interval reports whether a periodic knob fires at index t.
func interval(every, t int) bool {
	return every > 0 && t%every == 0
}

// Compress consumes one input vector and returns the output vector, with its
// length in bits.
//
// The octet slice is padded with zeros to the next octet; §2.2 leaves framing
// to the mission, so the bit length is what the caller must carry alongside if
// it is packing several outputs together.
func (c *Compressor) Compress(input []byte) (out []byte, bitLen int, err error) {
	return c.CompressWith(input, c.paramsFor())
}

// CompressWith consumes one input vector using explicit cycle parameters.
func (c *Compressor) CompressWith(input []byte, params CycleParams) (out []byte, bitLen int, err error) {
	if len(input) != (c.config.VectorLength+7)/8 {
		return nil, 0, fmt.Errorf("%w: got %d octets, want %d",
			ErrInvalidPacketLength, len(input), (c.config.VectorLength+7)/8)
	}
	if params.Robustness < 0 || params.Robustness > MaxRobustness {
		return nil, 0, fmt.Errorf("%w: got %d", ErrInvalidRobustness, params.Robustness)
	}

	// §3.3.2c and d: both flags are forced while t <= R_t. That is what makes
	// the first output self-describing, and it is not optional.
	if c.t <= params.Robustness {
		params.SendMask = true
		params.Uncompressed = true
	}

	current := VectorFromBytes(input, c.config.VectorLength)

	change := c.updateMask(current, params.NewMask)
	writer, err := c.encode(current, change, params)
	if err != nil {
		return nil, 0, err
	}

	// Commit the cycle.
	c.previous = current
	c.hasPrevious = true
	c.pushChange(change)
	c.pushNewMaskFlag(params.NewMask)
	c.t++
	c.forceNewMask = false
	c.forceSendMask = false
	c.forceUncompressed = false

	return writer.Bytes(), writer.BitLen(), nil
}

// updateMask runs the mask update stage of §4.2 and returns the change vector
// D_t of §4.2.3.
func (c *Compressor) updateMask(current Vector, newMask bool) Vector {
	previousMask := c.mask

	if !c.hasPrevious {
		// t = 0. §3.3.1 gives the mask; §4.1 gives the build. Equation 8 makes
		// D_0 = 0.
		c.build = NewVector(c.config.VectorLength)
		return NewVector(c.config.VectorLength)
	}

	difference := current.XOR(c.previous)

	// Equation 7: the mask takes in the new difference, over the build rather
	// than over itself when the new mask flag is set.
	if newMask {
		c.mask = difference.OR(c.build)
	} else {
		c.mask = difference.OR(c.mask)
	}

	// Equation 6: the build takes in the difference too, but resets to zero
	// whenever the new mask flag is set.
	if newMask {
		c.build = NewVector(c.config.VectorLength)
	} else {
		c.build = difference.OR(c.build)
	}

	// Equation 8.
	return c.mask.XOR(previousMask)
}

// pushChange records a change vector, dropping the oldest past maxHistory.
func (c *Compressor) pushChange(d Vector) {
	c.changes = append(c.changes, d)
	if len(c.changes) > maxHistory {
		c.changes = c.changes[len(c.changes)-maxHistory:]
	}
}

// pushNewMaskFlag records a new-mask flag, dropping the oldest past
// maxHistory.
func (c *Compressor) pushNewMaskFlag(set bool) {
	c.newMaskFlags = append(c.newMaskFlags, set)
	if len(c.newMaskFlags) > maxHistory {
		c.newMaskFlags = c.newMaskFlags[len(c.newMaskFlags)-maxHistory:]
	}
}

// changeAt returns the change vector for time index i, or a zero vector when
// it is outside the remembered window.
func (c *Compressor) changeAt(i int) Vector {
	// changes holds indices c.t-len(changes) .. c.t-1.
	oldest := c.t - len(c.changes)
	if i < oldest || i >= c.t {
		return NewVector(c.config.VectorLength)
	}
	return c.changes[i-oldest]
}

// newMaskAt reports whether the new mask flag was set at time index i.
func (c *Compressor) newMaskAt(i int) bool {
	oldest := c.t - len(c.newMaskFlags)
	if i < oldest || i >= c.t {
		return false
	}
	return c.newMaskFlags[i-oldest]
}

// effectiveRobustness computes V_t, per §5.3.2.2 equation 14.
//
// V_t is R_t plus C_t, the run of cycles just before the window in which the
// mask did not change at all. Those cycles cost nothing to cover — a change
// vector of zero adds nothing to the OR — so the coder reports the larger
// reach it happens to have.
func (c *Compressor) effectiveRobustness(robustness int) int {
	if c.t-robustness <= 0 {
		return robustness
	}

	// C_t is the largest integer at most min(t,15) - R_t for which every
	// change vector in {t-R_t-1, ..., t-R_t-C_t} is zero.
	limit := min(c.t, 15) - robustness
	extra := 0
	for extra < limit {
		i := c.t - robustness - 1 - extra
		if i < 0 {
			break
		}
		if !c.changeAt(i).IsZero() {
			break
		}
		extra++
	}
	return robustness + extra
}

// changeWindow computes X_t, per §5.3.3.1 equation 16: the recent change
// vectors ORed together, then reversed.
//
// The OR is the robustness mechanism. A decompressor that missed the last R_t
// outputs still sees every mask position that changed across them, because
// they are all in this one vector.
func (c *Compressor) changeWindow(currentChange Vector, robustness int) Vector {
	combined := currentChange.Clone()

	switch {
	case robustness == 0:
		// Only the current change vector.
	case c.t-robustness <= 0:
		// Everything so far, from D_1.
		for i := 1; i < c.t; i++ {
			combined = combined.OR(c.changeAt(i))
		}
	default:
		for i := c.t - robustness; i < c.t; i++ {
			combined = combined.OR(c.changeAt(i))
		}
	}

	return combined.Reverse()
}

// newMaskSetTwice reports whether the new mask flag was set more than once
// across the window equation 20 names: i in {max(0, t-V_t), ..., t}.
//
// It matters because a position that went from unpredictable to predictable
// during the window cannot be pinned down from the mask alone, so the encoder
// has to send its value too.
func (c *Compressor) newMaskSetTwice(effective int, currentNewMask bool) bool {
	count := 0
	if currentNewMask {
		count++
	}
	for i := max(0, c.t-effective); i < c.t; i++ {
		if c.newMaskAt(i) {
			count++
		}
	}
	return count > 1
}

// encode builds the output vector, per §5.3.
func (c *Compressor) encode(current, change Vector, params CycleParams) (*BitWriter, error) {
	// §5.3.2.1 equation 13.
	dFlag := !params.SendMask && !params.Uncompressed

	effective := c.effectiveRobustness(params.Robustness)
	window := c.changeWindow(change, params.Robustness)

	// Equation 17: the inverted mask read at the positions that changed. A
	// one here means the position ended up predictable.
	y := c.mask.Not().Reverse().Extract(window)

	// Equations 18 to 20.
	var hasE, eBit bool
	var hasK bool
	var hasC, cBit bool

	if effective > 0 && !window.IsZero() {
		hasE = true
		eBit = !allZero(y)
		if eBit {
			hasK = true
			hasC = true
			cBit = c.newMaskSetTwice(effective, params.NewMask)
		}
	}

	var w BitWriter

	// h_t, equation 15: RLE(X_t) || BIT4(V_t) || e_t || k_t || c_t || d-dot_t
	if err := AppendRLE(&w, window); err != nil {
		return nil, err
	}
	w.WriteBits(uint64(effective), 4)
	if hasE {
		w.WriteBit(eBit)
	}
	if hasK {
		// y is already in equation 11's emission order — last selected
		// position first; see Vector.Extract.
		for _, bit := range y {
			w.WriteBit(bit)
		}
	}
	if hasC {
		w.WriteBit(cBit)
	}
	w.WriteBit(dFlag)

	// q_t, equation 21.
	if !dFlag {
		w.WriteBit(params.SendMask)
		if params.SendMask {
			// The mask is transition coded before run-length encoding: XOR
			// with its own left shift turns each run of equal bits into a
			// single one bit at the edge, which run-length encodes far better
			// than the mask itself.
			transitions := c.mask.XOR(c.mask.ShiftLeft())
			if err := AppendRLE(&w, transitions.Reverse()); err != nil {
				return nil, err
			}
		}
	}

	// u_t, equation 22.
	selector := c.mask
	if cBit {
		// c_t = 1: also send the positions that changed in the window, since
		// a decompressor that lost outputs may not know which way they went.
		selector = window.Reverse().OR(c.mask)
	}

	switch {
	case dFlag:
		// r-dot_t is known to be zero from the last bit of h_t, so it is not
		// repeated here.
		for _, bit := range current.Extract(selector) {
			w.WriteBit(bit)
		}

	case params.Uncompressed:
		w.WriteBit(true)
		if err := AppendCount(&w, c.config.VectorLength); err != nil {
			return nil, err
		}
		for i := range current.Len() {
			w.WriteBit(current.Get(i))
		}

	default:
		w.WriteBit(false)
		for _, bit := range current.Extract(selector) {
			w.WriteBit(bit)
		}
	}

	return &w, nil
}
