package pxsc

import "fmt"

// Viterbi decoding of the rate 1/2, constraint-length 7 convolutional code,
// per CCSDS 211.2-B-3 clause 3.4.3.
//
// The encoder turns each input bit into two symbols whose values depend on the
// last seven input bits. A receiver cannot invert that bit by bit, because any
// single symbol pair is consistent with many histories. What it can do is
// track every history at once and keep only the cheapest way to reach each
// one, which is what the Viterbi algorithm is.
//
// # The trellis
//
// Six bits of history decide what the next input bit will produce, so there
// are 64 states. From each, an input of 0 or 1 leads to one of two successors,
// and every state has exactly two predecessors. At each step the decoder scores
// both ways into a state and keeps the better, so the work is constant per
// received symbol pair rather than growing with the message.
//
// # Hard decisions
//
// This decoder takes hard decisions: each symbol is already a bit. Clause 3.4.3.3
// recommends three-bit soft decisions instead, which buy roughly 2 dB, but
// soft symbols have to come from the demodulator and nothing in this library
// produces them, DecodeSoft is there for a receiver that has them.

// numStates is the number of trellis states: two to the power of the
// constraint length less one.
const numStates = 1 << (ConstraintLength - 1)

// stateMask keeps a value inside the trellis.
const stateMask = numStates - 1

// tracebackDepth is how far the decoder looks back before committing a bit.
//
// Survivor paths through the trellis merge: after enough steps every surviving
// path agrees about what was sent long ago, and five constraint lengths is the
// usual rule of thumb for how long that takes. Deciding at that depth rather
// than at the end of the block keeps memory flat and costs nothing in
// correctness, because the paths have already converged.
const tracebackDepth = 5 * ConstraintLength

// survivorRing is the size of the survivor history buffer. It only has to hold
// tracebackDepth steps plus the one just taken, but it is rounded up to a
// power of two so the ring index is a mask rather than a division. The
// traceback does tracebackDepth of them for every decoded bit.
const survivorRing = 64

// The trellis, flattened: a transition is indexed by state<<1 | bit, so the
// two branches out of a state sit next to each other and the inner loop reads
// one array rather than indexing two dimensions.

// outputTable holds the two symbols the encoder emits for each transition,
// packed as c1<<1 | c2.
var outputTable [numStates * 2]uint8

// nextState holds the state each transition leads to.
var nextState [numStates * 2]uint8

func init() {
	for state := range numStates {
		for bit := range 2 {
			// Mirror ConvolutionalEncoder.EncodeBit exactly: the register is
			// the six bits of history with the new bit shifted in.
			reg := uint8(state)<<1 | uint8(bit)
			reg &= 1<<ConstraintLength - 1

			c1 := parity(reg & g1Mask)
			c2 := parity(reg&g2Mask) ^ 1 // clause 3.4.3.1 note 1: the G2 path is inverted

			outputTable[state<<1|bit] = c1<<1 | c2
			nextState[state<<1|bit] = reg & stateMask
		}
	}
}

// ViterbiDecoder decodes a continuous convolutionally encoded stream.
//
// It holds the trellis between calls, so a stream arriving in pieces decodes
// as one, which matters because clause 3.4.3.2 encodes everything transmitted as a
// single stream, PLTUs and idle data alike.
//
// A ViterbiDecoder is not safe for concurrent use, and Decode must not be
// called re-entrantly: it reuses the decoder's own scratch buffer, so a
// second call nested inside the first would corrupt it.
type ViterbiDecoder struct {
	// metrics is the cost of the cheapest path reaching each state.
	metrics [numStates]uint32
	next    [numStates]uint32

	// survivors records, per step, which of the two predecessors won for each
	// state. One bit per state per step; the traceback walks it backwards.
	//
	// Only the last tracebackDepth steps are ever needed, so this is a ring:
	// oldest at head, count entries live. A plain slice would have to shift
	// every entry down on each decoded bit.
	survivors [survivorRing]uint64
	head      int
	count     int

	// best is the state ending the cheapest path, found while normalising so
	// the traceback does not have to scan the metrics again.
	best int

	// pending holds decoded bits not yet packed into an octet.
	pending    uint8
	pendingLen int

	// soft is scratch space Decode expands hard decisions into, kept between
	// calls so a stream of frames does not allocate it fresh every time.
	soft []int8
}

// unreachable is the metric of a state no path has reached. It is large enough
// to lose every comparison and small enough that adding a branch metric cannot
// wrap.
const unreachable = uint32(1) << 30

// NewViterbiDecoder returns a decoder positioned at the start of a stream.
//
// Only state zero has a finite metric, which pins the decoder to the encoder's
// cleared register: no path can start anywhere else.
func NewViterbiDecoder() *ViterbiDecoder {
	d := &ViterbiDecoder{}
	d.Reset()
	return d
}

// Reset returns the decoder to the start of a stream.
func (d *ViterbiDecoder) Reset() {
	for i := range d.metrics {
		d.metrics[i] = unreachable
	}
	d.metrics[0] = 0
	d.head, d.count, d.best = 0, 0, 0
	d.pending, d.pendingLen = 0, 0
}

// Decode decodes convolutionally encoded symbols and returns the input bits
// they carry, packed most significant bit first.
//
// symbols holds two coded bits per input bit, in the order Encode produced
// them, so the input must be an even number of octets: eight input bits become
// two octets of symbols.
func (d *ViterbiDecoder) Decode(symbols []byte) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	if len(symbols)%2 != 0 {
		return nil, fmt.Errorf("%w: %d octets of symbols is not a whole number of input octets",
			ErrInvalidLength, len(symbols))
	}

	// Two coded bits per input bit.
	pairs := len(symbols) * 8 / 2
	// Reuse the scratch buffer across calls: reslicing to zero length keeps
	// the backing array, so a stream of frames stops paying for this after
	// the first one grows it to the largest frame seen.
	soft := d.soft[:0]
	if cap(soft) < pairs*2 {
		soft = make([]int8, 0, pairs*2)
	}

	for _, b := range symbols {
		for i := 7; i >= 0; i-- {
			// A hard decision is a soft decision at full confidence.
			if b>>uint(i)&1 == 1 {
				soft = append(soft, 1)
			} else {
				soft = append(soft, -1)
			}
		}
	}
	d.soft = soft
	return d.decodeSoft(soft)
}

// DecodeSoft decodes from soft decisions, which clause 3.4.3.3 recommends over hard
// ones.
//
// Each entry is the demodulator's confidence in one coded symbol: positive for
// a one, negative for a zero, and further from zero for more confident. Two
// entries per input bit, so the slice length must be even. The scale does not
// matter, only the sign and the relative magnitude; three-bit decisions in the
// range -4 to 3 are what clause 3.4.3.3 has in mind.
func (d *ViterbiDecoder) DecodeSoft(symbols []int8) ([]byte, error) {
	if len(symbols)%2 != 0 {
		return nil, fmt.Errorf("%w: %d soft symbols is not a whole number of input bits",
			ErrInvalidLength, len(symbols))
	}
	return d.decodeSoft(symbols)
}

// decodeSoft runs the trellis over pairs of soft symbols.
func (d *ViterbiDecoder) decodeSoft(symbols []int8) ([]byte, error) {
	// Two soft symbols per input bit, eight input bits per output octet.
	out := make([]byte, 0, len(symbols)/16)

	for i := 0; i+1 < len(symbols); i += 2 {
		d.step(symbols[i], symbols[i+1])

		// Once the survivor history is deep enough, the oldest decision is
		// settled and can be emitted.
		if d.count > tracebackDepth {
			bit := d.traceback()
			out = d.emit(out, bit)
		}
	}
	return out, nil
}

// step advances the trellis by one symbol pair.
func (d *ViterbiDecoder) step(s1, s2 int8) {
	for i := range d.next {
		d.next[i] = unreachable
	}

	// Every one of the 128 transitions emits one of only four symbol pairs, so
	// score the four once rather than scoring each transition.
	var cost [4]uint32
	for pair := range 4 {
		cost[pair] = symbolCost(uint8(pair)>>1, s1) + symbolCost(uint8(pair)&1, s2)
	}

	var survivor uint64

	for state := range numStates {
		metric := d.metrics[state]
		if metric >= unreachable {
			continue
		}
		for bit := range 2 {
			transition := state<<1 | bit
			total := metric + cost[outputTable[transition]]

			target := nextState[transition]
			if total < d.next[target] {
				d.next[target] = total

				// Record which predecessor won. A transition shifts the state
				// left, so the target keeps all of the predecessor except its
				// top bit, and that top bit is the only thing distinguishing
				// the two states that lead here. Storing it is what makes the
				// walk back unambiguous.
				//
				// The input bit is not what to store: it is already the
				// target's low bit, so traceback can read it off the state.
				if state>>(ConstraintLength-2)&1 == 1 {
					survivor |= 1 << uint(target)
				} else {
					survivor &^= 1 << uint(target)
				}
			}
		}
	}

	d.metrics, d.next = d.next, d.metrics

	d.survivors[(d.head+d.count)&(survivorRing-1)] = survivor
	d.count++

	d.normalise()
}

// symbolCost is the penalty for the trellis expecting want where the
// demodulator reported got.
//
// A symbol the demodulator is confident about costs more to contradict, which
// is the whole benefit of soft decisions: a marginal symbol bends cheaply and
// a clear one does not.
func symbolCost(want uint8, got int8) uint32 {
	// Confidence, as a non-negative magnitude.
	magnitude := int32(got)
	if magnitude < 0 {
		magnitude = -magnitude
	}

	received := uint8(0)
	if got > 0 {
		received = 1
	}
	if received == want {
		return 0
	}
	return uint32(magnitude)
}

// normalise subtracts the smallest metric from all of them, and records which
// state that was.
//
// Metrics only ever grow, so over a long stream they would eventually
// overflow. Only their differences matter, so shifting them all down changes
// no decision.
//
// The state holding the smallest metric is where the traceback starts.
// Clause 3.4.3.2 encodes a continuous stream with no tail bits, so the decoder
// cannot assume it finishes in state zero the way a terminated block would; it
// takes whichever state the evidence favours.
func (d *ViterbiDecoder) normalise() {
	best, bestMetric := 0, unreachable
	for state, m := range d.metrics {
		if m < bestMetric {
			best, bestMetric = state, m
		}
	}
	d.best = best

	if bestMetric == 0 || bestMetric >= unreachable {
		return
	}
	for i, m := range d.metrics {
		if m < unreachable {
			d.metrics[i] = m - bestMetric
		}
	}
}

// traceback walks the survivor history back from the best current state and
// returns the input bit decided at the oldest recorded step, dropping it.
func (d *ViterbiDecoder) traceback() uint8 {
	state := d.best

	// Walk back to the state the stream was in after the oldest recorded step.
	for i := d.count - 1; i > 0; i-- {
		high := uint8(d.survivors[(d.head+i)&(survivorRing-1)] >> uint(state) & 1)
		state = predecessor(state, high)
	}

	// The input bit was shifted into the low end of that state, so it is
	// simply there to be read.
	bit := uint8(state & 1)

	// Drop the step just decided.
	d.head = (d.head + 1) & (survivorRing - 1)
	d.count--
	return bit
}

// predecessor returns the state that led into target, given the top bit the
// transition shifted out.
//
// A transition shifts the state left by one, so the predecessor is the target
// shifted back down with that lost top bit put back. The bit is not
// recoverable from the target alone. It is what the survivor record holds.
func predecessor(target int, high uint8) int {
	return (target >> 1) | int(high)<<(ConstraintLength-2)
}

// emit appends one decoded bit, packing octets most significant bit first.
func (d *ViterbiDecoder) emit(out []byte, bit uint8) []byte {
	d.pending = d.pending<<1 | bit&1
	d.pendingLen++
	if d.pendingLen == 8 {
		out = append(out, d.pending)
		d.pending, d.pendingLen = 0, 0
	}
	return out
}

// Flush returns the bits still held in the traceback window, ending the
// stream.
//
// Decode only emits a bit once the survivor paths have had tracebackDepth
// steps to converge, so the last few decisions are still pending when the
// symbols run out. Flush forces them out along the best surviving path, where
// the usual convergence argument no longer applies. The final few bits are
// the least reliable in the stream.
func (d *ViterbiDecoder) Flush() []byte {
	// At most tracebackDepth bits are ever still pending here, since Decode
	// emits everything beyond that as it goes.
	out := make([]byte, 0, (tracebackDepth+7)/8)

	for d.count > 0 {
		bit := d.traceback()
		out = d.emit(out, bit)
	}

	// Any partial octet is padded with zeros, since an octet is the smallest
	// thing this can return.
	if d.pendingLen > 0 {
		out = append(out, d.pending<<(8-uint(d.pendingLen)))
		d.pending, d.pendingLen = 0, 0
	}
	return out
}

// ViterbiDecode decodes a complete stream with a fresh decoder, flushing the
// traceback window at the end.
//
// Use a ViterbiDecoder directly when the stream continues across calls.
func ViterbiDecode(symbols []byte) ([]byte, error) {
	d := NewViterbiDecoder()

	out, err := d.Decode(symbols)
	if err != nil {
		return nil, err
	}
	return append(out, d.Flush()...), nil
}
