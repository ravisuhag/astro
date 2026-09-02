package cop

import "sync"

// FARMState represents the FARM-1 state machine state.
type FARMState int

const (
	FARMOpen    FARMState = iota // S1: accepting frames in window
	FARMWait                     // S2: wait state (no buffer available)
	FARMLockout                  // S3: lockout (requires Unlock BC frame)
)

// String names the state.
//
// Lockout is the one worth reading in a log: a FARM-1 in lockout accepts
// nothing until an Unlock BC frame arrives, and setting V(R) will not clear
// it (CCSDS 232.1-B-2 clause 6.1).
func (s FARMState) String() string {
	switch s {
	case FARMOpen:
		return "open"
	case FARMWait:
		return "wait"
	case FARMLockout:
		return "lockout"
	default:
		return "unknown"
	}
}

// FARM implements the Frame Acceptance and Reporting Mechanism (FARM-1)
// per CCSDS 232.1-B-2 Section 6.
//
// FARM-1 runs on the spacecraft side. It validates incoming TC frame
// sequence numbers and generates CLCW status reports for the return link.
//
// The sliding window W is split into a positive half PW and a negative
// half NW (PW = NW = W/2, W even, per CCSDS 232.1-B-2 6.1.5):
//
//   - N(S) == V(R): frame accepted, V(R) incremented (E1), unless no
//     buffer is available, in which case the frame is discarded and the
//     Wait and Retransmit flags are set (E2).
//   - V(R) < N(S) <= V(R)+PW-1: inside the positive window but out of
//     sequence, discarded, Retransmit flag set (E3).
//   - V(R)-NW <= N(S) < V(R): inside the negative window, a duplicate of
//     an already-accepted frame. Discarded silently, no flags change (E4).
//   - Otherwise: outside both windows, Lockout is entered (E5). The
//     Retransmit flag is left untouched.
type FARM struct {
	mu           sync.Mutex
	state        FARMState
	vr           uint8 // V(R): next expected frame sequence number
	farmBCounter uint8 // FARM-B counter (2 bits, wraps at 4)
	windowWidth  uint8 // W: sliding window width (even)
	pw           uint8 // PW: positive window width (W/2)
	nw           uint8 // NW: negative window width (W/2)
	vcid         uint8
	lockout      bool
	wait         bool
	retransmit   bool

	// Buffer accounting for the Wait state. buffers is the number of
	// frame buffers currently free; unlimited disables the accounting.
	buffers   int
	unlimited bool
}

// NewFARM creates a new FARM-1 instance for the given VCID.
//
// windowWidth is the sliding window W. Per CCSDS 232.1-B-2 it must be an
// even value in 2..254 (PW = NW = W/2); out-of-range values are clamped
// and odd values rounded down. By default buffer accounting is disabled
// (the Wait state is never entered); use SetBuffers to enable it.
func NewFARM(vcid uint8, windowWidth uint8) *FARM {
	w := windowWidth
	if w < 2 {
		w = 2
	}
	if w > 254 {
		w = 254
	}
	w &^= 1 // W must be even so that PW = NW = W/2
	return &FARM{
		state:       FARMOpen,
		vcid:        vcid,
		windowWidth: w,
		pw:          w / 2,
		nw:          w / 2,
		unlimited:   true,
	}
}

// SetBuffers configures the number of free frame buffers. When a Type-A
// frame is accepted, one buffer is consumed; when none are free, the FARM
// enters the Wait state (E2) and discards in-sequence frames until
// ReleaseBuffer is called. A negative n disables buffer accounting.
func (f *FARM) SetBuffers(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n < 0 {
		f.unlimited = true
		f.buffers = 0
		return
	}
	f.unlimited = false
	f.buffers = n
	f.updateWait()
}

// ReleaseBuffer signals that the higher layer has consumed one accepted
// frame, freeing its buffer. Leaving the buffer-exhausted condition clears
// the Wait flag (E10: "buffer release" signal from the higher procedures).
func (f *FARM) ReleaseBuffer() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.unlimited {
		return
	}
	f.buffers++
	f.updateWait()
}

// updateWait recomputes the Wait flag from buffer availability. It models
// the buffer signals only, E10 ("buffer release") and the buffer bookkeeping
// behind E1/E2. The Type-BC directives E7 (Unlock) and E8 (Set V(R)) must NOT
// use it: table 6-1 sends both to (S1) with Wait_Flag := 0 whatever the buffer
// count says.
//
// Note that in Lockout the state is left alone: E10 in S3 clears the Wait
// flag but keeps the machine in (S3).
//
// Caller must hold f.mu.
func (f *FARM) updateWait() {
	if f.unlimited || f.buffers > 0 {
		f.wait = false
		if f.state == FARMWait {
			f.state = FARMOpen
		}
		return
	}
	f.wait = true
	if f.state == FARMOpen {
		f.state = FARMWait
	}
}

// ProcessFrame validates an incoming TC frame per FARM-1 rules.
// Returns whether the frame was accepted.
//
// Frame types per CCSDS 232.0-B-4 4.1.2.3:
//   - Type-AD (bypass=0, cc=0): sequence-controlled data, checked
//     against V(R) and the sliding window.
//   - Type-BD (bypass=1, cc=0): expedited data, always accepted;
//     increments the FARM-B counter (E6).
//   - Type-BC (bypass=1, cc=1): control command. dataField must contain
//     Unlock (0x00) or Set V(R) (0x82 0x00 <V(R)>). Both increment the
//     FARM-B counter. Unlock clears the Lockout, Wait, and Retransmit
//     flags and leaves V(R) untouched (E7). Set V(R) sets V(R) from the
//     directive payload and clears Wait and Retransmit; in Lockout it is
//     accepted but changes nothing except FARM-B (E8).
//   - bypass=0, cc=1 is an invalid type and is discarded.
//
// bypassFlag: 0=Type-A, 1=Type-B
// controlCommandFlag: 0=data, 1=control command
// frameSeqNum: N(S) from the frame header (ignored for Type-B frames)
// dataField: the frame data field (used only for Type-BC frames)
func (f *FARM) ProcessFrame(bypassFlag, controlCommandFlag uint8, frameSeqNum uint8, dataField []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Type-B frames
	if bypassFlag == 1 {
		if controlCommandFlag == 1 {
			return f.processControlCommand(dataField)
		}
		// Type-BD expedited data (E6): always accepted.
		f.farmBCounter = (f.farmBCounter + 1) & 0x03
		return true, nil
	}

	// Bypass=0 with Control Command=1 is an invalid frame type
	// (CCSDS 232.0-B-4 4.1.2.3): discard.
	if controlCommandFlag == 1 {
		return false, ErrInvalidFrameType
	}

	// Type-AD data frame
	if f.lockout {
		// E1-E5 in S3: discard everything until Unlock.
		return false, ErrFARMLockout
	}

	diff := (frameSeqNum - f.vr) & 0xFF

	switch {
	case diff == 0:
		// In sequence. E1: accept if a buffer is available; E2: no
		// buffer, discard, set Wait and Retransmit.
		if !f.unlimited && f.buffers <= 0 {
			f.wait = true
			f.retransmit = true
			f.state = FARMWait
			return false, ErrFARMWait
		}
		f.vr++
		f.retransmit = false
		if !f.unlimited {
			f.buffers--
			f.updateWait()
		}
		return true, nil

	case diff < uint8(f.pw):
		// Inside the positive window but not next in sequence (E3):
		// a frame was lost, discard, request retransmission.
		f.retransmit = true
		return false, ErrFARMReject

	case (f.vr-frameSeqNum)&0xFF <= f.nw:
		// Inside the negative window (E4): a duplicate of a frame
		// already accepted. Discard silently; no flags change and no
		// lockout is declared.
		return false, nil

	default:
		// Outside both windows (E5): enter Lockout. The Retransmit
		// flag is deliberately left as it was.
		f.state = FARMLockout
		f.lockout = true
		return false, ErrFARMLockout
	}
}

// processControlCommand decodes and executes a Type-BC frame data field.
// Caller must hold f.mu.
func (f *FARM) processControlCommand(dataField []byte) (bool, error) {
	switch {
	case len(dataField) == 1 && dataField[0] == 0x00:
		// Unlock (E7): clear Lockout, Wait, and Retransmit. V(R) is NOT
		// touched, only Set V(R) changes it. Per table 6-1 the next state
		// is (S1) unconditionally, so the Wait flag is cleared outright
		// rather than re-derived from the buffer count: a directive must
		// never leave the machine in S2.
		f.lockout = false
		f.retransmit = false
		f.wait = false
		f.farmBCounter = (f.farmBCounter + 1) & 0x03
		f.state = FARMOpen
		return true, nil

	case len(dataField) == 3 && dataField[0] == 0x82 && dataField[1] == 0x00:
		// Set V(R) (E8): in Lockout the directive is accepted (FARM-B
		// still counts it) but has no other effect; otherwise V(R) is
		// set from the directive payload and Wait and Retransmit cleared,
		// with (S1) as the next state, again unconditionally.
		f.farmBCounter = (f.farmBCounter + 1) & 0x03
		if !f.lockout {
			f.vr = dataField[2]
			f.retransmit = false
			f.wait = false
			f.state = FARMOpen
		}
		return true, nil

	default:
		// Invalid control command contents (E9): discard.
		return false, ErrInvalidControlCommand
	}
}

// GenerateCLCW returns a CLCW reflecting the current FARM-1 state.
func (f *FARM) GenerateCLCW() *CLCW {
	f.mu.Lock()
	defer f.mu.Unlock()

	return &CLCW{
		ControlWordType:  0,
		Version:          0,
		COPInEffect:      1, // COP-1
		VirtualChannelID: f.vcid,
		LockoutFlag:      f.lockout,
		WaitFlag:         f.wait,
		RetransmitFlag:   f.retransmit,
		FARMBCounter:     f.farmBCounter,
		ReportValue:      f.vr,
	}
}

// State returns the current FARM-1 state.
func (f *FARM) State() FARMState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// VR returns the current V(R) value.
func (f *FARM) VR() uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.vr
}
