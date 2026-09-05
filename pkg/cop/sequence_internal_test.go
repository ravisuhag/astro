package cop

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
)

// The sequence vectors in vectors/cop/ drive FARM-1 and FOP-1 through a
// scripted run: a starting state, then a list of events, checking the octets
// each one emits and the variables it leaves behind.
//
// A state machine is the one thing an encode or decode vector cannot express.
// The same event gives a different answer depending on what came before, and
// a single input-output pair has nowhere to put the "before".
//
// These are internal tests because a vector's starting state is a set of
// variables, not a constructor argument. Reaching "V(R) is 5 and the machine
// is locked out" through the public API would mean replaying the events that
// produce it, which makes the vector a test of those events instead of the one
// it means to pin.

func TestFARM1SequenceVectors(t *testing.T) {
	vectors.RunFile(t, "cop/farm1.json", vectors.Impl{MachineFn: newFARMMachine})
}

func TestFOP1SequenceVectors(t *testing.T) {
	vectors.RunFile(t, "cop/fop1.json", vectors.Impl{MachineFn: newFOPMachine})
}

// --- FARM-1 ---

type farmMachine struct{ f *FARM }

func newFARMMachine(init, config vectors.Fields) (vectors.Machine, error) {
	width, err := config.Uint("sliding_window_width")
	if err != nil {
		return nil, err
	}
	vcid, err := config.Uint("vcid")
	if err != nil {
		return nil, err
	}

	f := NewFARM(uint8(vcid), uint8(width))

	state, err := init.Str("state")
	if err != nil {
		return nil, err
	}
	switch state {
	case "open":
		f.state = FARMOpen
	case "wait":
		f.state = FARMWait
	case "lockout":
		f.state = FARMLockout
	default:
		return nil, fmt.Errorf("unknown FARM-1 state %q", state)
	}

	vr, err := init.Uint("v_r")
	if err != nil {
		return nil, err
	}
	f.vr = uint8(vr)

	if f.lockout, err = init.BoolOr("lockout_flag", f.state == FARMLockout); err != nil {
		return nil, err
	}
	if f.retransmit, err = init.BoolOr("retransmit_flag", false); err != nil {
		return nil, err
	}
	if f.wait, err = init.BoolOr("wait_flag", f.state == FARMWait); err != nil {
		return nil, err
	}
	return &farmMachine{f: f}, nil
}

func (m *farmMachine) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	switch call {
	case "receive_ad":
		ns, err := fields.Uint("n_s")
		if err != nil {
			return nil, nil, err
		}
		// Type-AD: bypass clear, control-command clear.
		//
		// The error is deliberately dropped here, and everywhere else in this
		// file. pkg/cop reports a discarded frame, a lockout and an expired
		// timer as errors, and every one of those is normal machine
		// behaviour that the standard describes — a discard sets the
		// retransmit flag, which is the negative acknowledgement of clause
		// 6.1.5. What the vector asserts is the state left behind, which is
		// where the standard puts the answer too.
		_, _ = m.f.ProcessFrame(0, 0, uint8(ns), nil)

	case "receive_bd":
		// Type-BD: bypass set. It is accepted in every state, which is what
		// makes it the way out of lockout.
		_, _ = m.f.ProcessFrame(1, 0, 0, nil)

	case "receive_unlock":
		_, _ = m.f.ProcessFrame(1, 1, 0, []byte{0x00})

	case "receive_set_v_r":
		star, err := fields.Uint("v_r_star")
		if err != nil {
			return nil, nil, err
		}
		// The Set V(R) command is three octets: 0x82, 0x00, then V(R)*.
		_, _ = m.f.ProcessFrame(1, 1, 0, []byte{0x82, 0x00, byte(star)})

	case "report":
		encoded, err := m.f.GenerateCLCW().Encode()
		if err != nil {
			return nil, nil, err
		}
		return encoded, m.state(), nil

	default:
		return nil, nil, fmt.Errorf("unknown FARM-1 call %q", call)
	}

	return nil, m.state(), nil
}

func (m *farmMachine) state() vectors.Fields {
	return vectors.Fields{
		"state":           m.f.State().String(),
		"v_r":             uint64(m.f.vr),
		"lockout_flag":    m.f.lockout,
		"retransmit_flag": m.f.retransmit,
		"wait_flag":       m.f.wait,
		"farm_b_counter":  uint64(m.f.farmBCounter),
	}
}

// --- FOP-1 ---

type fopMachine struct{ f *FOP }

func newFOPMachine(init, config vectors.Fields) (vectors.Machine, error) {
	vcid, err := config.Uint("vcid")
	if err != nil {
		return nil, err
	}
	limit, err := config.Uint("transmission_limit")
	if err != nil {
		return nil, err
	}
	timeoutType, err := config.Uint("timeout_type")
	if err != nil {
		return nil, err
	}

	f := NewFOP(0, uint8(vcid), 255)
	if err := f.SetTransmissionLimit(uint8(limit)); err != nil {
		return nil, err
	}
	if err := f.SetTimeoutType(int(timeoutType)); err != nil {
		return nil, err
	}

	state, err := init.Str("state")
	if err != nil {
		return nil, err
	}
	switch state {
	case "active":
		f.state = FOPActive
	case "initial":
		f.state = FOPInitial
	default:
		return nil, fmt.Errorf("unknown FOP-1 state %q", state)
	}

	vs, err := init.Uint("v_s")
	if err != nil {
		return nil, err
	}
	f.vs = uint8(vs)
	f.nnr = uint8(vs) - uint8(sentQueueLen(init))

	count, err := init.UintOr("transmission_count", 1)
	if err != nil {
		return nil, err
	}
	f.txCount = uint8(count)

	// A sent queue of length n stands for n AD frames awaiting acknowledgement,
	// the oldest at N(R). The timer runs whenever the queue is not empty,
	// which is what makes a tick meaningful.
	for i := 0; i < sentQueueLen(init); i++ {
		f.sentQueue = append(f.sentQueue, SentFrame{SequenceNum: f.nnr + uint8(i)})
	}
	if len(f.sentQueue) > 0 {
		f.t1Initial = 1
		f.t1Remaining = 1
		f.timerRunning = true
	}
	return &fopMachine{f: f}, nil
}

func sentQueueLen(init vectors.Fields) int {
	n, err := init.UintOr("sent_queue_length", 0)
	if err != nil {
		return 0
	}
	return int(n)
}

func (m *fopMachine) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	switch call {
	case "tick":
		// One unit of the caller's clock. FOP-1 owns no timer of its own —
		// on a light-minutes link only the mission knows what a sensible
		// timeout is — so the vector supplies the ticks.
		_ = m.f.Tick(1)

	case "receive_clcw":
		reportValue, err := fields.Uint("report_value")
		if err != nil {
			return nil, nil, err
		}
		retransmit, err := fields.BoolOr("retransmit_flag", false)
		if err != nil {
			return nil, nil, err
		}
		lockout, err := fields.BoolOr("lockout_flag", false)
		if err != nil {
			return nil, nil, err
		}
		wait, err := fields.BoolOr("wait_flag", false)
		if err != nil {
			return nil, nil, err
		}
		clcw := &CLCW{
			VirtualChannelID: m.f.vcid,
			ReportValue:      uint8(reportValue),
			RetransmitFlag:   retransmit,
			LockoutFlag:      lockout,
			WaitFlag:         wait,
		}
		_ = m.f.ProcessCLCW(clcw)

	default:
		return nil, nil, fmt.Errorf("unknown FOP-1 call %q", call)
	}

	return nil, m.state(), nil
}

func (m *fopMachine) state() vectors.Fields {
	return vectors.Fields{
		"state":              m.f.State().String(),
		"v_s":                uint64(m.f.vs),
		"transmission_count": uint64(m.f.txCount),
		"sent_queue_length":  uint64(len(m.f.sentQueue)),
		"alert":              m.f.lastAlert != AlertNone,
		// Clause 5.1.11: the suspend state records which state the service
		// was suspended from, so any non-zero value means suspended.
		"suspended":     m.f.ss != 0,
		"suspend_state": uint64(m.f.ss),
	}
}

// --- pruneAcked, past what the public API can construct ---
//
// pruneAcked is unexported. TestFOP_AckPrunesAtMaxWindow in fop_test.go
// exercises it through NewFOP and TransmitFrame, but MaxWindow bounds K at
// 127, so no legally configured FOP can ever have an acknowledged frame
// sitting more than 128 behind N(R) — the exact shape the old hardcoded
// `diff > 128` heuristic got wrong (bug B1). That makes the old heuristic
// correct for everything reachable through NewFOP today, so nothing at
// the API level still proves pruneAcked's ackCount-based math is the
// implementation doing the work. This test calls pruneAcked directly with
// a queue shaped the way the old heuristic mishandled, so the invariant
// stays pinned even though it is unconstructable through the public API —
// worth keeping in case the ceiling ever moves, for instance if a future
// standard revision widens the sequence field and MaxWindow moves with it.
func TestPruneAckedUsesTheAcknowledgementCount(t *testing.T) {
	// N(R) = 150, acknowledging sequence numbers 0..149 (ackCount = 150).
	// The frame at sequence 10 is 140 behind N(R): under the old constant
	// `diff > 128` check it would have been kept forever, mistaken for
	// still outstanding, even though it is one of the 150 frames this
	// CLCW just acknowledged and must be pruned.
	q := []SentFrame{
		{SequenceNum: 10},  // acknowledged, diff = 140: what B1 got wrong
		{SequenceNum: 149}, // acknowledged, boundary case: diff = 1
		{SequenceNum: 150}, // N(R) itself: not yet acknowledged, diff = 0
		{SequenceNum: 200}, // ahead of N(R): not yet acknowledged
	}

	got := pruneAcked(q, 150, 150)

	var remaining []uint8
	for _, sf := range got {
		remaining = append(remaining, sf.SequenceNum)
	}
	want := []uint8{150, 200}
	if len(remaining) != len(want) {
		t.Fatalf("pruneAcked(q, 150, 150) kept %v, want %v", remaining, want)
	}
	for i := range want {
		if remaining[i] != want[i] {
			t.Fatalf("pruneAcked(q, 150, 150) kept %v, want %v", remaining, want)
		}
	}
}
