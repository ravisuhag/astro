package cop

import "sync"

// FOPState represents the FOP-1 state machine state.
type FOPState int

const (
	FOPActive  FOPState = iota // S1: active, accepting frames
	FOPInitial                  // S6: initial (not started)
)

// SentFrame tracks a transmitted Type-A frame awaiting acknowledgment.
type SentFrame struct {
	SequenceNum uint8
	Data        []byte // encoded frame bytes for retransmission
}

// FOP implements the Flight Operations Procedure (FOP-1)
// per CCSDS 232.1-B-2 Section 4.
//
// FOP-1 runs on the ground side. It manages Type-A (sequence-controlled)
// frame transmission with sliding window acknowledgment via CLCW.
//
// Usage:
//  1. Create with NewFOP
//  2. Call Initialize() to start
//  3. Call TransmitFrame() to queue Type-A frames
//  4. Call GetNextFrame() to get the next frame to send
//  5. Call ProcessCLCW() when a CLCW arrives on the TM return link
type FOP struct {
	mu           sync.Mutex
	state        FOPState
	vs           uint8        // V(S): next sequence number to assign
	nnr          uint8        // N(N)R: last acknowledged sequence number from CLCW
	sentQueue    []SentFrame  // frames sent, awaiting acknowledgment
	waitQueue    [][]byte     // encoded frames waiting to be transmitted
	windowWidth  uint8        // FW: sliding window width
	scid         uint16
	vcid         uint8
}

// NewFOP creates a new FOP-1 instance.
// windowWidth is the sliding window size (must match FARM's window width).
func NewFOP(scid uint16, vcid uint8, windowWidth uint8) *FOP {
	return &FOP{
		state:       FOPInitial,
		scid:        scid,
		vcid:        vcid,
		windowWidth: windowWidth,
	}
}

// Initialize starts FOP-1, setting it to Active state.
// Sets V(S) to the given initial sequence number.
func (f *FOP) Initialize(initialVS uint8) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vs = initialVS
	f.nnr = initialVS
	f.sentQueue = nil
	f.waitQueue = nil
	f.state = FOPActive
}

// TransmitFrame queues an encoded Type-A frame for transmission.
// The frame will be assigned the next sequence number V(S).
// Returns ErrFOPWindowFull if the sliding window is exhausted.
func (f *FOP) TransmitFrame(encodedFrame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != FOPActive {
		return ErrFOPLockout
	}

	// Check if window is full: V(S) - N(N)R >= FW
	outstanding := (f.vs - f.nnr) & 0xFF
	if outstanding >= f.windowWidth {
		return ErrFOPWindowFull
	}

	// Assign sequence number and queue
	sf := SentFrame{
		SequenceNum: f.vs,
		Data:        encodedFrame,
	}
	f.sentQueue = append(f.sentQueue, sf)
	f.vs++

	return nil
}

// GetNextFrame returns the next frame to transmit.
// First serves new frames from the wait queue, then retransmissions
// if the retransmit flag was set by ProcessCLCW.
func (f *FOP) GetNextFrame() ([]byte, uint8, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.waitQueue) > 0 {
		data := f.waitQueue[0]
		f.waitQueue = f.waitQueue[1:]
		return data, 0, true
	}

	return nil, 0, false
}

// ProcessCLCW processes a CLCW received on the TM return link.
// Acknowledges frames, detects lockout, and triggers retransmission.
func (f *FOP) ProcessCLCW(clcw *CLCW) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if clcw.LockoutFlag {
		f.state = FOPInitial
		return ErrFOPLockout
	}

	// Acknowledge frames: remove all sent frames with seq < V(R)
	vr := clcw.ReportValue
	f.nnr = vr

	var remaining []SentFrame
	for _, sf := range f.sentQueue {
		// Frame is acknowledged if its seq num is before V(R)
		diff := (vr - sf.SequenceNum) & 0xFF
		if diff == 0 || diff > 128 {
			// Not yet acknowledged (seq >= V(R) in modular arithmetic)
			remaining = append(remaining, sf)
		}
	}
	f.sentQueue = remaining

	// If retransmit flag is set, re-queue unacknowledged frames
	if clcw.RetransmitFlag && len(f.sentQueue) > 0 {
		for _, sf := range f.sentQueue {
			f.waitQueue = append(f.waitQueue, sf.Data)
		}
	}

	return nil
}

// State returns the current FOP-1 state.
func (f *FOP) State() FOPState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// VS returns the current V(S) value.
func (f *FOP) VS() uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.vs
}

// PendingCount returns the number of unacknowledged frames.
func (f *FOP) PendingCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sentQueue)
}
