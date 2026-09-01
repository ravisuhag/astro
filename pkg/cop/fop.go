package cop

import "sync"

// FOPState represents the FOP-1 state machine state per CCSDS 232.1-B-2
// Section 5.1 (table 5-1).
type FOPState int

const (
	FOPActive                FOPState = iota // S1: Active
	FOPRetransmitWithoutWait                 // S2: Retransmit without Wait
	FOPRetransmitWithWait                    // S3: Retransmit with Wait
	FOPInitialisingWithoutBC                 // S4: Initialising without BC Frame
	FOPInitialisingWithBC                    // S5: Initialising with BC Frame
	FOPInitial                               // S6: Initial
)

// AlertReason identifies why FOP-1 raised an Alert and went to Initial.
type AlertReason int

const (
	AlertNone      AlertReason = iota
	AlertLimit                 // transmission limit reached
	AlertLockout               // CLCW Lockout flag seen
	AlertSynch                 // CLCW inconsistent with FOP state
	AlertNNR                   // invalid N(R) in CLCW
	AlertCLCW                  // invalid CLCW flag combination
	AlertT1                    // T1 timeout with no retransmission allowed
	AlertTerminate             // Terminate AD Service directive
)

// Timeout types for the T1 timer expiry with the transmission limit
// reached (CCSDS 232.1-B-2 5.2.6).
const (
	// TT0 raises an Alert when T1 expires with the transmission limit
	// reached.
	TT0 = 0
	// TT1 suspends the AD service when T1 expires with the transmission
	// limit reached, allowing a later Resume.
	TT1 = 1
)

// SentFrame tracks a transmitted Type-A frame awaiting acknowledgment.
type SentFrame struct {
	SequenceNum uint8
	Data        []byte // encoded frame bytes for retransmission
}

// FOP implements the Flight Operations Procedure (FOP-1)
// per CCSDS 232.1-B-2 Section 5.
//
// FOP-1 runs on the ground side. It manages Type-A (sequence-controlled)
// frame transmission with sliding window acknowledgment via CLCW, plus
// the BC (control command) and BD (expedited) transmit paths.
//
// The T1 timer is caller-driven: the FOP holds no wall clock. Configure
// the timer with SetT1Initial (in whatever unit the caller ticks in) and
// advance it with Tick. A T1 initial of 0 (the default) disables the
// timer.
//
// Usage:
//  1. Create with NewFOP
//  2. Start the AD service with Initialize() or one of the Initiate
//     directives
//  3. Call TransmitFrame() to queue Type-A frames
//  4. Call GetNextFrame() to get the next frame to send, with its N(S)
//  5. Call ProcessCLCW() when a CLCW arrives on the TM return link
//  6. Call Tick() as time passes to drive the T1 timer
type FOP struct {
	mu    sync.Mutex
	state FOPState
	ss    int // suspend state: 0 = not suspended, 1..4 = suspended from S1..S4

	vs  uint8 // V(S): next sequence number to assign
	nnr uint8 // NN(R): last acknowledged sequence number from CLCW

	sentQueue []SentFrame // AD frames sent, awaiting acknowledgment
	waitQueue []SentFrame // AD frames waiting to be (re)transmitted
	bdQueue   [][]byte    // BD frames waiting to be transmitted
	bcFrame   []byte      // BC frame for the initiate-in-progress
	bcOut     bool        // BC frame ready to be pulled by GetNextFrame
	pendingVR *uint8      // V(R) value pinned by Initiate AD with Set V(R)

	windowWidth uint8 // K: FOP sliding window width (1..255)

	t1Initial    int // T1 initial value in caller tick units; 0 = disabled
	t1Remaining  int
	timerRunning bool
	timeoutType  int // TT0 or TT1

	txLimit uint8 // Transmission_Limit (1..255)
	txCount uint8 // Transmission_Count

	lastAlert AlertReason

	scid uint16
	vcid uint8
}

// NewFOP creates a new FOP-1 instance in the Initial state (S6).
//
// windowWidth is the FOP sliding window (1..255; 0 is clamped to 1). The
// transmission limit defaults to 255, the timeout type to TT0, and the T1
// timer starts disabled (initial value 0).
func NewFOP(scid uint16, vcid uint8, windowWidth uint8) *FOP {
	if windowWidth == 0 {
		windowWidth = 1
	}
	return &FOP{
		state:       FOPInitial,
		scid:        scid,
		vcid:        vcid,
		windowWidth: windowWidth,
		txLimit:     255,
		txCount:     1,
		timeoutType: TT0,
	}
}

// --- Directives (CCSDS 232.1-B-2 5.2) ---

// Initialize starts the AD service without CLCW check, setting V(S) to
// the given value first. It is the "Set V(S)" plus "Initiate AD Service
// (without CLCW check)" directive pair.
func (f *FOP) Initialize(initialVS uint8) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.purge()
	f.vs = initialVS
	f.nnr = initialVS
	f.ss = 0
	f.txCount = 1
	f.state = FOPActive
}

// InitiateADWithoutCLCW starts the AD service immediately (E23). Only
// valid in the Initial state.
func (f *FOP) InitiateADWithoutCLCW() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial {
		return ErrFOPNotInitial
	}
	f.purge()
	f.ss = 0
	f.txCount = 1
	f.state = FOPActive
	return nil
}

// InitiateADWithCLCWCheck starts the AD service once a clean CLCW
// (Lockout=0, Wait=0, Retransmit=0) arrives (E24). V(S) and NN(R) are
// then taken from the CLCW report value. Starts T1. Only valid in the
// Initial state.
func (f *FOP) InitiateADWithCLCWCheck() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial {
		return ErrFOPNotInitial
	}
	f.purge()
	f.ss = 0
	f.txCount = 1
	f.state = FOPInitialisingWithoutBC
	f.startTimer()
	return nil
}

// InitiateADWithUnlock starts the AD service by transmitting the given
// encoded BC Unlock frame (E25). The frame is served by GetNextFrame and
// retransmitted on T1 expiry until a CLCW with Lockout=0 confirms it.
// Only valid in the Initial state.
func (f *FOP) InitiateADWithUnlock(bcFrame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial {
		return ErrFOPNotInitial
	}
	f.purge()
	f.ss = 0
	f.txCount = 1
	f.bcFrame = bcFrame
	f.bcOut = true
	f.pendingVR = nil
	f.state = FOPInitialisingWithBC
	f.startTimer()
	return nil
}

// InitiateADWithSetVR starts the AD service by transmitting the given
// encoded BC Set V(R) frame (E27). vr must match the V(R) value carried
// by the frame; V(S) and NN(R) are set to it once a CLCW with Lockout=0
// and Report Value == vr confirms the directive. Only valid in the
// Initial state.
func (f *FOP) InitiateADWithSetVR(vr uint8, bcFrame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial {
		return ErrFOPNotInitial
	}
	f.purge()
	f.ss = 0
	f.txCount = 1
	f.bcFrame = bcFrame
	f.bcOut = true
	v := vr
	f.pendingVR = &v
	f.state = FOPInitialisingWithBC
	f.startTimer()
	return nil
}

// TerminateAD terminates the AD service (E29): all queues are purged and
// the machine returns to Initial. Any suspend state is cleared.
func (f *FOP) TerminateAD() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alert(AlertTerminate)
}

// ResumeAD resumes a suspended AD service (E30-E33), restoring the state
// the machine was suspended from and restarting T1. Returns
// ErrFOPNotSuspended when the service is not suspended.
func (f *FOP) ResumeAD() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial || f.ss == 0 {
		return ErrFOPNotSuspended
	}
	switch f.ss {
	case 1:
		f.state = FOPActive
	case 2:
		f.state = FOPRetransmitWithoutWait
	case 3:
		f.state = FOPRetransmitWithWait
	case 4:
		f.state = FOPInitialisingWithoutBC
	}
	f.ss = 0
	f.txCount = 1
	f.startTimer()
	return nil
}

// SetVS sets V(S) (and NN(R)) to the given value (E35). Only valid in
// the Initial state with no suspended service.
func (f *FOP) SetVS(vs uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != FOPInitial || f.ss != 0 {
		return ErrFOPNotInitial
	}
	f.vs = vs
	f.nnr = vs
	return nil
}

// SetSlidingWindow sets the FOP sliding window width K (E36).
// Valid values are 1..255.
func (f *FOP) SetSlidingWindow(w uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if w == 0 {
		return ErrFOPInvalidWindow
	}
	f.windowWidth = w
	return nil
}

// SetT1Initial sets the T1 timer initial value in caller tick units
// (E37). A value of 0 disables the timer.
func (f *FOP) SetT1Initial(ticks int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ticks < 0 {
		return ErrFOPInvalidT1
	}
	f.t1Initial = ticks
	return nil
}

// SetTransmissionLimit sets the transmission limit (E38).
// Valid values are 1..255.
func (f *FOP) SetTransmissionLimit(n uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n == 0 {
		return ErrFOPInvalidLimit
	}
	f.txLimit = n
	return nil
}

// SetTimeoutType sets the timeout type (E39): TT0 alerts on T1 expiry
// with the transmission limit reached; TT1 suspends the AD service.
func (f *FOP) SetTimeoutType(tt int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tt != TT0 && tt != TT1 {
		return ErrFOPInvalidTimeoutType
	}
	f.timeoutType = tt
	return nil
}

// --- Transmit paths ---

// TransmitFrame queues an encoded Type-AD frame for transmission.
// The frame is assigned the next sequence number V(S).
// Returns ErrFOPWindowFull if the sliding window is exhausted and
// ErrFOPNotActive when the AD service is not active (S1-S3).
func (f *FOP) TransmitFrame(encodedFrame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != FOPActive && f.state != FOPRetransmitWithoutWait && f.state != FOPRetransmitWithWait {
		return ErrFOPNotActive
	}

	// Check if window is full: V(S) - NN(R) >= K
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
	f.waitQueue = append(f.waitQueue, sf)
	f.vs++
	f.startTimer()

	return nil
}

// TransmitBDFrame queues an encoded Type-BD (expedited) frame. BD frames
// bypass sequence control and are served by GetNextFrame ahead of AD
// frames. Allowed in any state.
func (f *FOP) TransmitBDFrame(encodedFrame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bdQueue = append(f.bdQueue, encodedFrame)
	return nil
}

// GetNextFrame returns the next frame to transmit along with the sequence
// number N(S) assigned to it (0 for BC and BD frames). Priority order:
// the pending BC frame, then BD frames, then AD frames in queue order.
// The third return value is false when nothing is pending.
func (f *FOP) GetNextFrame() ([]byte, uint8, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.bcOut && f.bcFrame != nil {
		f.bcOut = false
		return f.bcFrame, 0, true
	}

	if len(f.bdQueue) > 0 {
		data := f.bdQueue[0]
		f.bdQueue = f.bdQueue[1:]
		return data, 0, true
	}

	if len(f.waitQueue) > 0 {
		sf := f.waitQueue[0]
		f.waitQueue = f.waitQueue[1:]
		return sf.Data, sf.SequenceNum, true
	}

	return nil, 0, false
}

// --- CLCW processing (E1-E14) ---

// ProcessCLCW processes a CLCW received on the TM return link.
// Acknowledges frames, checks N(R) validity, handles the Lockout, Wait,
// and Retransmit flags, and completes any initiate-in-progress.
func (f *FOP) ProcessCLCW(clcw *CLCW) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch f.state {
	case FOPInitial:
		// E1-E14 in S6: ignored.
		return nil

	case FOPInitialisingWithoutBC:
		// Initiate AD with CLCW check (S4).
		if clcw.LockoutFlag {
			f.alert(AlertLockout)
			return ErrFOPLockout
		}
		if !clcw.WaitFlag && !clcw.RetransmitFlag {
			// Clean CLCW: adopt the reported V(R) and go Active (E1).
			f.vs = clcw.ReportValue
			f.nnr = clcw.ReportValue
			f.stopTimer()
			f.txCount = 1
			f.state = FOPActive
		}
		// Otherwise keep waiting for a clean CLCW until T1 expires.
		return nil

	case FOPInitialisingWithBC:
		// Initiate AD with Unlock or Set V(R) (S5).
		if clcw.LockoutFlag {
			// The BC frame has not taken effect yet; keep waiting.
			return nil
		}
		if f.pendingVR != nil && clcw.ReportValue != *f.pendingVR {
			// Set V(R) not confirmed yet.
			return nil
		}
		// BC frame accepted: adopt the confirmed V(R) and go Active.
		if f.pendingVR != nil {
			f.vs = *f.pendingVR
			f.nnr = *f.pendingVR
		} else {
			f.vs = clcw.ReportValue
			f.nnr = clcw.ReportValue
		}
		f.bcFrame = nil
		f.bcOut = false
		f.pendingVR = nil
		f.stopTimer()
		f.txCount = 1
		f.state = FOPActive
		return nil
	}

	// S1-S3: the AD service is running.

	if clcw.LockoutFlag {
		// E14: Lockout detected.
		f.alert(AlertLockout)
		return ErrFOPLockout
	}

	// N(R) validity (E13): NN(R) <= N(R) <= V(S) in modulo-256 arithmetic.
	nr := clcw.ReportValue
	ackCount := (nr - f.nnr) & 0xFF
	outstanding := (f.vs - f.nnr) & 0xFF
	if ackCount > outstanding {
		f.alert(AlertNNR)
		return ErrFOPInvalidNR
	}

	// Acknowledge frames up to (but excluding) N(R).
	if ackCount > 0 {
		f.nnr = nr
		f.sentQueue = pruneAcked(f.sentQueue, nr)
		f.waitQueue = pruneAcked(f.waitQueue, nr)
		// New frames acknowledged: reset the retransmission budget and
		// restart T1 for the frames still outstanding.
		f.txCount = 1
		if len(f.sentQueue) > 0 {
			f.startTimer()
		}
	}

	allAcked := nr == f.vs

	if allAcked {
		switch {
		case clcw.RetransmitFlag:
			// E4: a retransmission is asked for with nothing outstanding,
			// so the two ends disagree about what has been sent.
			f.alert(AlertSynch)
			return ErrFOPSynch
		case clcw.WaitFlag:
			// E3: the Wait flag only means something while frames are
			// outstanding.
			f.alert(AlertCLCW)
			return ErrFOPInvalidCLCW
		case ackCount == 0 && f.retransmitting():
			// E1 in S2/S3: this machine believes a retransmission is under
			// way, yet the CLCW acknowledges nothing new and no longer asks
			// for one. The two ends are out of step.
			f.alert(AlertSynch)
			return ErrFOPSynch
		default:
			// E1 in S1 (Ignore) and E2 in S1/S2/S3: everything is
			// acknowledged, so nothing is left to time out.
			f.stopTimer()
			f.txCount = 1
			f.state = FOPActive
			return nil
		}
	}

	// Frames remain outstanding.
	if !clcw.RetransmitFlag {
		if clcw.WaitFlag {
			// E7: Wait without Retransmit is invalid.
			f.alert(AlertCLCW)
			return ErrFOPInvalidCLCW
		}
		if ackCount == 0 && f.retransmitting() {
			// E5 in S2/S3: the receiver has stopped asking for a
			// retransmission without acknowledging anything new, which
			// leaves the retransmission this machine is running unexplained.
			f.alert(AlertSynch)
			return ErrFOPSynch
		}
		// E5 in S1 (Ignore) and E6 in S1/S2/S3: frames are outstanding but
		// the receiver has no complaint.
		f.state = FOPActive
		return nil
	}

	// Retransmit flag set (E8-E12, E101, E102).

	if f.txLimit == 1 {
		// E101/E102: a limit of 1 forbids sending any frame twice, so a
		// retransmission request ends the AD service whatever the Wait flag
		// says. Frames this CLCW acknowledges have already left the sent
		// queue above, which is all that separates E101 from E102.
		f.alert(AlertLimit)
		return ErrFOPLimit
	}

	if clcw.WaitFlag {
		// E11/E12: retransmission required, but the FARM has no buffer.
		// Hold retransmissions until the Wait flag clears.
		f.waitQueue = nil
		f.state = FOPRetransmitWithWait
		return nil
	}

	// Retransmission required and possible (E8/E9).
	if f.txCount >= f.txLimit && ackCount == 0 {
		// E10: the retransmission budget is spent with nothing new
		// acknowledged.
		f.alert(AlertLimit)
		return ErrFOPLimit
	}
	f.txCount++
	// Rebuild the transmission queue from the unacknowledged frames.
	// Rebuilding from sentQueue covers frames already pulled and frames
	// still waiting, without queueing either of them twice.
	f.waitQueue = append([]SentFrame(nil), f.sentQueue...)
	f.startTimer()
	f.state = FOPRetransmitWithoutWait
	return nil
}

// retransmitting reports whether the machine is in S2 or S3, the two
// states it only reaches because a retransmission is under way.
// Caller must hold f.mu.
func (f *FOP) retransmitting() bool {
	return f.state == FOPRetransmitWithoutWait || f.state == FOPRetransmitWithWait
}

// pruneAcked removes every frame acknowledged by N(R) from q.
func pruneAcked(q []SentFrame, nr uint8) []SentFrame {
	var remaining []SentFrame
	for _, sf := range q {
		diff := (nr - sf.SequenceNum) & 0xFF
		if diff == 0 || diff > 128 {
			// Not yet acknowledged (seq >= N(R) in modular arithmetic).
			remaining = append(remaining, sf)
		}
	}
	return remaining
}

// --- T1 timer (caller-driven clock) ---

// Tick advances the caller-driven clock by n units. When the T1 timer is
// running and reaches zero, the timer-expiry events fire (E16-E18 and
// E104): retransmission while the transmission limit allows it, then
// either an Alert(T1) (timeout type TT0) or a suspension of the AD
// service (timeout type TT1). Returns the error corresponding to a raised
// alert, or nil.
func (f *FOP) Tick(n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.timerRunning || n <= 0 {
		return nil
	}
	f.t1Remaining -= n
	if f.t1Remaining > 0 {
		return nil
	}

	// T1 expired.
	if f.txCount < f.txLimit {
		// E16 (timeout type TT0) and E104 (TT1): the retransmission budget
		// still allows another attempt.
		switch f.state {
		case FOPActive, FOPRetransmitWithoutWait:
			// The state is left as it was: S1 stays S1 and S2 stays S2,
			// because the next CLCW is classified differently in each.
			f.txCount++
			f.waitQueue = append([]SentFrame(nil), f.sentQueue...)
			f.startTimer()

		case FOPRetransmitWithWait:
			// Ignore in S3. The receiver has said it has no buffer, so
			// retransmitting now would only be discarded there.
			//
			// The timer is deliberately left expired rather than restarted:
			// 5.2.18 defines Ignore as no processing at all, and 5.1.9.1
			// ties the timer to an actual transmission, which S3 does not
			// make. That is not a dead end, because every exit from S3 is
			// driven by an arriving CLCW — new acknowledgements (E2/E6),
			// the Wait flag clearing (E8/E10), or an alert (E3/E7/E13/E14)
			// — and the receiver keeps reporting.

		case FOPInitialisingWithoutBC:
			// Nothing has been transmitted that could be retransmitted, so
			// the first expiry in S4 already ends the initialisation.
			if f.timeoutType == TT1 {
				f.ss = 4
				f.stopTimer()
				f.state = FOPInitial
				return ErrFOPSuspended
			}
			f.alert(AlertT1)
			return ErrFOPTimeout

		case FOPInitialisingWithBC:
			f.txCount++
			f.bcOut = true // retransmit the BC frame
			f.startTimer()
		}
		return nil
	}

	// Transmission limit reached.
	if f.timeoutType == TT1 && f.state != FOPInitialisingWithBC {
		// E18: suspend the AD service, recording the state to resume into.
		switch f.state {
		case FOPActive:
			f.ss = 1
		case FOPRetransmitWithoutWait:
			f.ss = 2
		case FOPRetransmitWithWait:
			f.ss = 3
		case FOPInitialisingWithoutBC:
			f.ss = 4
		}
		f.stopTimer()
		f.state = FOPInitial
		return ErrFOPSuspended
	}

	// E17 in any state, and E18 in S5: every timer-driven exhaustion is
	// Alert[T1]. Alert[Limit] belongs to the CLCW-driven E10/E101/E102.
	f.alert(AlertT1)
	return ErrFOPTimeout
}

// TimerRunning reports whether the T1 timer is currently running.
func (f *FOP) TimerRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.timerRunning
}

// startTimer (re)starts T1 when a positive initial value is configured.
// Caller must hold f.mu.
func (f *FOP) startTimer() {
	if f.t1Initial <= 0 {
		f.timerRunning = false
		return
	}
	f.t1Remaining = f.t1Initial
	f.timerRunning = true
}

// stopTimer cancels T1. Caller must hold f.mu.
func (f *FOP) stopTimer() {
	f.timerRunning = false
	f.t1Remaining = 0
}

// alert raises an Alert: queues are purged, the timer stops, and the
// machine returns to Initial. Caller must hold f.mu.
func (f *FOP) alert(reason AlertReason) {
	f.lastAlert = reason
	f.purge()
	f.state = FOPInitial
}

// purge discards every queued frame and stops the timer.
// Caller must hold f.mu.
func (f *FOP) purge() {
	f.sentQueue = nil
	f.waitQueue = nil
	f.bdQueue = nil
	f.bcFrame = nil
	f.bcOut = false
	f.pendingVR = nil
	f.stopTimer()
}

// --- Introspection ---

// State returns the current FOP-1 state.
func (f *FOP) State() FOPState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// SuspendState returns the suspend state SS (0 = not suspended,
// 1..4 = suspended from S1..S4).
func (f *FOP) SuspendState() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ss
}

// LastAlert returns the reason of the most recent Alert.
func (f *FOP) LastAlert() AlertReason {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastAlert
}

// VS returns the current V(S) value.
func (f *FOP) VS() uint8 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.vs
}

// PendingCount returns the number of unacknowledged AD frames.
func (f *FOP) PendingCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sentQueue)
}
