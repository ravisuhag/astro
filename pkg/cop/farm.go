package cop

import "sync"

// FARMState represents the FARM-1 state machine state.
type FARMState int

const (
	FARMOpen    FARMState = iota // S1: accepting frames in window
	FARMWait                     // S2: wait state
	FARMLockout                  // S3: lockout (requires ground unlock)
)

// FARM implements the Frame Acceptance and Reporting Mechanism (FARM-1)
// per CCSDS 232.1-B-2 Section 5.
//
// FARM-1 runs on the spacecraft side. It validates incoming TC frame
// sequence numbers and generates CLCW status reports for the return link.
type FARM struct {
	mu           sync.Mutex
	state        FARMState
	vr           uint8 // V(R): next expected frame sequence number
	farmBCounter uint8 // Type-B acceptance counter (2 bits, wraps at 4)
	windowWidth  uint8 // W: positive sliding window width
	vcid         uint8
	lockout      bool
	wait         bool
	retransmit   bool
}

// NewFARM creates a new FARM-1 instance for the given VCID.
// windowWidth is the positive sliding window size (typically 10).
func NewFARM(vcid uint8, windowWidth uint8) *FARM {
	return &FARM{
		state:       FARMOpen,
		vcid:        vcid,
		windowWidth: windowWidth,
	}
}

// ProcessFrame validates an incoming TC frame per FARM-1 rules.
// Returns whether the frame was accepted.
//
// Type-B (bypass) frames are always accepted.
// Type-A (sequence-controlled) frames are checked against V(R):
//   - N(S) == V(R): accepted, V(R) incremented
//   - N(S) within window but != V(R): rejected, retransmit flag set
//   - N(S) outside window: rejected, lockout entered
//
// bypassFlag: 0=Type-A, 1=Type-B
// controlCommandFlag: 0=data, 1=control command
// frameSeqNum: N(S) from the frame header
func (f *FARM) ProcessFrame(bypassFlag, controlCommandFlag uint8, frameSeqNum uint8) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Type-B frames: always accept
	if bypassFlag == 1 {
		if controlCommandFlag == 0 {
			f.farmBCounter = (f.farmBCounter + 1) & 0x03
		}
		return true, nil
	}

	// Control commands (Type-A with ControlCommand=1)
	if controlCommandFlag == 1 {
		return f.processControlCommand(frameSeqNum), nil
	}

	// Type-A data frame
	if f.state == FARMLockout {
		return false, ErrFARMLockout
	}

	ns := frameSeqNum

	if ns == f.vr {
		// In sequence: accept and advance
		f.vr++
		f.retransmit = false
		return true, nil
	}

	// Check if within positive window: V(R) < N(S) < V(R) + W (mod 256)
	diff := (ns - f.vr) & 0xFF
	if diff > 0 && diff < uint8(f.windowWidth) {
		// Within window but not V(R): set retransmit
		f.retransmit = true
		return false, ErrFARMReject
	}

	// Outside window: lockout
	f.state = FARMLockout
	f.lockout = true
	f.retransmit = false
	return false, ErrFARMLockout
}

// processControlCommand handles unlock and set-V(R) directives.
// Per CCSDS 232.1-B-2, control commands use specific reserved patterns.
// Unlock: clears lockout. Set V(R): sets V(R) to the frame sequence number.
func (f *FARM) processControlCommand(frameSeqNum uint8) bool {
	// Unlock directive (BC frame with ControlCommand=1, Bypass=0)
	// Clears lockout and resets state to Open
	f.state = FARMOpen
	f.lockout = false
	f.wait = false
	f.retransmit = false
	f.vr = frameSeqNum
	return true
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
