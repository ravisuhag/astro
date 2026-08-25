package cop

import "errors"

var (
	// ErrDataTooShort indicates the provided data is too short for CLCW decoding.
	ErrDataTooShort = errors.New("provided data is too short to decode CLCW")

	// ErrInvalidCLCWType indicates the control word type is not 0.
	ErrInvalidCLCWType = errors.New("invalid CLCW: control word type must be 0")

	// ErrInvalidCLCWVersion indicates the CLCW version is not 0.
	ErrInvalidCLCWVersion = errors.New("invalid CLCW: version must be 00")

	// ErrFOPLockout indicates FOP-1 received a CLCW with the Lockout flag set.
	ErrFOPLockout = errors.New("FOP-1: lockout detected, ground must issue unlock")

	// ErrFOPWindowFull indicates the FOP-1 send window is full.
	ErrFOPWindowFull = errors.New("FOP-1: send window full, waiting for acknowledgment")

	// ErrFARMReject indicates FARM-1 rejected a frame (out of window).
	ErrFARMReject = errors.New("FARM-1: frame rejected, sequence number outside window")

	// ErrFARMLockout indicates FARM-1 is in lockout state.
	ErrFARMLockout = errors.New("FARM-1: lockout state, requires unlock command")

	// ErrFARMWait indicates FARM-1 discarded an in-sequence frame because
	// no frame buffer is available (Wait state).
	ErrFARMWait = errors.New("FARM-1: wait state, no frame buffer available")

	// ErrInvalidFrameType indicates a frame with Bypass=0 and Control
	// Command=1, an invalid type per CCSDS 232.0-B-4 4.1.2.3.
	ErrInvalidFrameType = errors.New("invalid frame type: Bypass=0 with Control Command=1")

	// ErrInvalidControlCommand indicates a Type-BC frame whose data field
	// is neither Unlock (0x00) nor Set V(R) (0x82 0x00 <V(R)>).
	ErrInvalidControlCommand = errors.New("invalid control command: expected Unlock (0x00) or Set V(R) (0x82 0x00 vr)")

	// ErrFOPNotActive indicates the AD service is not active (S1-S3).
	ErrFOPNotActive = errors.New("FOP-1: AD service not active")

	// ErrFOPNotInitial indicates a directive that is only valid in the
	// Initial state (S6) was issued elsewhere.
	ErrFOPNotInitial = errors.New("FOP-1: directive only valid in the Initial state")

	// ErrFOPNotSuspended indicates Resume was issued with no suspended
	// AD service.
	ErrFOPNotSuspended = errors.New("FOP-1: AD service is not suspended")

	// ErrFOPInvalidNR indicates a CLCW whose N(R) is outside
	// NN(R)..V(S) — the ground and spacecraft have lost synchronization.
	ErrFOPInvalidNR = errors.New("FOP-1: invalid N(R) in CLCW (alert NNR)")

	// ErrFOPSynch indicates a CLCW inconsistent with the FOP state
	// (retransmit requested with nothing outstanding).
	ErrFOPSynch = errors.New("FOP-1: CLCW inconsistent with FOP state (alert SYNCH)")

	// ErrFOPInvalidCLCW indicates an invalid CLCW flag combination
	// (Wait without Retransmit).
	ErrFOPInvalidCLCW = errors.New("FOP-1: invalid CLCW flag combination (alert CLCW)")

	// ErrFOPLimit indicates the transmission limit was reached
	// without progress (alert LIMIT).
	ErrFOPLimit = errors.New("FOP-1: transmission limit reached (alert LIMIT)")

	// ErrFOPTimeout indicates T1 expired during initialisation
	// (alert T1).
	ErrFOPTimeout = errors.New("FOP-1: T1 timer expired (alert T1)")

	// ErrFOPSuspended indicates T1 expired with timeout type TT1 and the
	// AD service was suspended; ResumeAD can continue it.
	ErrFOPSuspended = errors.New("FOP-1: AD service suspended (timeout type 1)")

	// ErrFOPInvalidWindow indicates an invalid FOP sliding window width
	// (valid: 1..255).
	ErrFOPInvalidWindow = errors.New("FOP-1: invalid sliding window width (valid 1-255)")

	// ErrFOPInvalidLimit indicates an invalid transmission limit
	// (valid: 1..255).
	ErrFOPInvalidLimit = errors.New("FOP-1: invalid transmission limit (valid 1-255)")

	// ErrFOPInvalidT1 indicates a negative T1 initial value.
	ErrFOPInvalidT1 = errors.New("FOP-1: invalid T1 initial value")

	// ErrFOPInvalidTimeoutType indicates a timeout type other than TT0
	// or TT1.
	ErrFOPInvalidTimeoutType = errors.New("FOP-1: invalid timeout type (valid 0 or 1)")
)
