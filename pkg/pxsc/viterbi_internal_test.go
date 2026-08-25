package pxsc

import "testing"

// TestTrellisMatchesTheEncoder pins the decoder's trellis tables to the
// encoder itself, transition by transition.
//
// The tables restate what EncodeBit does. A restatement can drift from what it
// restates, and the round-trip tests would not always say so: they walk the
// paths a real message takes, not all 128 transitions. This walks all of them.
func TestTrellisMatchesTheEncoder(t *testing.T) {
	for state := range numStates {
		for bit := range 2 {
			// Drive a fresh encoder into this state, then feed it the bit.
			// EncodeBit keeps the whole ConstraintLength-bit register, of
			// which the trellis state is the low ConstraintLength-1 bits.
			e := &ConvolutionalEncoder{state: uint8(state)}
			c1, c2 := e.EncodeBit(uint8(bit))

			transition := state<<1 | bit

			if want := c1<<1 | c2; outputTable[transition] != want {
				t.Errorf("state %d bit %d: outputTable = %02b, encoder emits %02b",
					state, bit, outputTable[transition], want)
			}
			if want := e.state & stateMask; nextState[transition] != want {
				t.Errorf("state %d bit %d: nextState = %d, encoder went to %d",
					state, bit, nextState[transition], want)
			}
		}
	}
}

// TestEveryStateHasTwoPredecessors is the property the traceback depends on:
// exactly two states lead into each, and they differ only in the top bit that
// the transition shifts out. If that were not so, storing one bit per state
// would not identify the survivor.
func TestEveryStateHasTwoPredecessors(t *testing.T) {
	into := make([][]int, numStates)
	for state := range numStates {
		for bit := range 2 {
			target := nextState[state<<1|bit]
			into[target] = append(into[target], state)
		}
	}

	for target, sources := range into {
		if len(sources) != 2 {
			t.Fatalf("state %d has %d predecessors, want 2", target, len(sources))
		}
		if sources[0]^sources[1] != 1<<(ConstraintLength-2) {
			t.Errorf("state %d is reached from %d and %d, which differ by more than the top bit",
				target, sources[0], sources[1])
		}
		// And the walk back must recover each of them.
		for _, source := range sources {
			high := uint8(source >> (ConstraintLength - 2) & 1)
			if got := predecessor(target, high); got != source {
				t.Errorf("predecessor(%d, %d) = %d, want %d", target, high, got, source)
			}
		}
	}
}

// TestInputBitIsTheTargetLowBit is the other half of the traceback: it reads
// the decoded bit straight off the state rather than from the survivor record.
func TestInputBitIsTheTargetLowBit(t *testing.T) {
	for state := range numStates {
		for bit := range 2 {
			target := nextState[state<<1|bit]
			if int(target)&1 != bit {
				t.Fatalf("state %d bit %d leads to state %d, whose low bit is not the input bit",
					state, bit, target)
			}
		}
	}
}
