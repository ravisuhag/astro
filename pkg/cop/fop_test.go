package cop_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

func TestFOP_InitializeAndTransmit(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	if fop.VS() != 0 {
		t.Errorf("V(S) = %d, want 0", fop.VS())
	}

	err := fop.TransmitFrame([]byte("frame-0"))
	if err != nil {
		t.Fatal(err)
	}
	if fop.VS() != 1 {
		t.Errorf("V(S) = %d, want 1", fop.VS())
	}
	if fop.PendingCount() != 1 {
		t.Errorf("PendingCount = %d, want 1", fop.PendingCount())
	}
}

func TestFOP_WindowFull(t *testing.T) {
	fop := cop.NewFOP(42, 1, 3)
	fop.Initialize(0)

	for range 3 {
		_ = fop.TransmitFrame([]byte("frame"))
	}

	err := fop.TransmitFrame([]byte("overflow"))
	if !errors.Is(err, cop.ErrFOPWindowFull) {
		t.Errorf("expected ErrFOPWindowFull, got %v", err)
	}
}

func TestFOP_ProcessCLCW_Acknowledgment(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))
	_ = fop.TransmitFrame([]byte("frame-2"))

	if fop.PendingCount() != 3 {
		t.Fatalf("PendingCount = %d, want 3", fop.PendingCount())
	}

	clcw := &cop.CLCW{ReportValue: 2}
	if err := fop.ProcessCLCW(clcw); err != nil {
		t.Fatal(err)
	}

	if fop.PendingCount() != 1 {
		t.Errorf("PendingCount after ack = %d, want 1", fop.PendingCount())
	}
}

func TestFOP_ProcessCLCW_Lockout(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	_ = fop.TransmitFrame([]byte("frame"))

	clcw := &cop.CLCW{LockoutFlag: true}
	err := fop.ProcessCLCW(clcw)
	if !errors.Is(err, cop.ErrFOPLockout) {
		t.Errorf("expected ErrFOPLockout, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
}

func TestFOP_ProcessCLCW_Retransmit(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))

	clcw := &cop.CLCW{ReportValue: 0, RetransmitFlag: true}
	_ = fop.ProcessCLCW(clcw)

	data, _, ok := fop.GetNextFrame()
	if !ok {
		t.Fatal("expected retransmission frame")
	}
	if !bytes.Equal(data, []byte("frame-0")) {
		t.Errorf("retransmit data = %q, want 'frame-0'", data)
	}
}

func TestFOP_FARM_Integration(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	farm := cop.NewFARM(1, 10)
	fop.Initialize(0)

	for i := range 3 {
		_ = fop.TransmitFrame([]byte{byte(i)})
	}

	for i := range 3 {
		accepted, err := farm.ProcessFrame(0, 0, uint8(i), nil)
		if err != nil || !accepted {
			t.Fatalf("frame %d: accepted=%v err=%v", i, accepted, err)
		}
	}

	clcw := farm.GenerateCLCW()
	if clcw.ReportValue != 3 {
		t.Errorf("CLCW V(R) = %d, want 3", clcw.ReportValue)
	}

	_ = fop.ProcessCLCW(clcw)
	if fop.PendingCount() != 0 {
		t.Errorf("PendingCount = %d, want 0", fop.PendingCount())
	}
}

func TestFOP_GetNextFrameServesQueuedFrames(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	want := [][]byte{[]byte("frame-0"), []byte("frame-1"), []byte("frame-2")}
	for _, f := range want {
		if err := fop.TransmitFrame(f); err != nil {
			t.Fatalf("TransmitFrame(%q): %v", f, err)
		}
	}

	for i, w := range want {
		data, seq, ok := fop.GetNextFrame()
		if !ok {
			t.Fatalf("frame %d: GetNextFrame returned nothing", i)
		}
		if !bytes.Equal(data, w) {
			t.Errorf("frame %d data = %q, want %q", i, data, w)
		}
		if seq != uint8(i) {
			t.Errorf("frame %d sequence number = %d, want %d", i, seq, i)
		}
	}

	if _, _, ok := fop.GetNextFrame(); ok {
		t.Error("expected the queue to be drained")
	}
}

func TestFOP_RetransmitCarriesSequenceNumbers(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))

	// Drain both, as a caller pulling frames to send would.
	for i := 0; i < 2; i++ {
		if _, _, ok := fop.GetNextFrame(); !ok {
			t.Fatalf("frame %d not served", i)
		}
	}

	// ReportValue 0 acknowledges neither frame.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}

	want := [][]byte{[]byte("frame-0"), []byte("frame-1")}
	for i, w := range want {
		data, seq, ok := fop.GetNextFrame()
		if !ok {
			t.Fatalf("retransmit %d: GetNextFrame returned nothing", i)
		}
		if !bytes.Equal(data, w) {
			t.Errorf("retransmit %d data = %q, want %q", i, data, w)
		}
		if seq != uint8(i) {
			t.Errorf("retransmit %d sequence number = %d, want the original %d", i, seq, i)
		}
	}
}

func TestFOP_AckPrunesWaitQueue(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))

	// Never pulled. ReportValue 2 acknowledges both (V(R) is the next
	// expected sequence number, so 0 and 1 are accepted).
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 2}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}

	if data, seq, ok := fop.GetNextFrame(); ok {
		t.Errorf("acknowledged frame still queued: %q (N(S)=%d)", data, seq)
	}
}

// TestFOP_T1Expiry_RetransmitThenAlertT1 walks both halves of the T1
// timer rule in CCSDS 232.1-B-2 5.1.9.1: an expiry below the transmission
// limit starts recovery, and an expiry at the limit generates Alert[T1].
// The state transitions are table 5-1 E16 and E17 in the ACTIVE column.
func TestFOP_T1Expiry_RetransmitThenAlertT1(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(2); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	if !fop.TimerRunning() {
		t.Fatal("T1 must start when a frame is queued")
	}
	// Pull it, as the sender would.
	if _, _, ok := fop.GetNextFrame(); !ok {
		t.Fatal("frame not served")
	}

	// First expiry: transmission count 1 < limit 2, so E16 retransmits.
	// Its next state is (S1), so the machine stays Active.
	if err := fop.Tick(5); err != nil {
		t.Fatalf("first T1 expiry: %v", err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive (table 5-1 E16 keeps S1)", fop.State())
	}
	data, seq, ok := fop.GetNextFrame()
	if !ok || seq != 0 || !bytes.Equal(data, []byte("frame-0")) {
		t.Fatalf("expected retransmission of frame-0, got %q (N(S)=%d, ok=%v)", data, seq, ok)
	}
	if !fop.TimerRunning() {
		t.Fatal("T1 must restart after a timer-driven retransmission")
	}

	// Second expiry: the limit is reached, so E17 gives Alert[T1] — not
	// Alert[Limit], which 5.1.9.1 reserves for no timer case at all.
	err := fop.Tick(5)
	if !errors.Is(err, cop.ErrFOPTimeout) {
		t.Fatalf("expected ErrFOPTimeout, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertT1 {
		t.Errorf("alert = %d, want AlertT1", fop.LastAlert())
	}
	if fop.PendingCount() != 0 {
		t.Errorf("queues must be purged on alert, %d frames left", fop.PendingCount())
	}
	if _, _, ok := fop.GetNextFrame(); ok {
		t.Error("no frame may be served after the alert purge")
	}
}

func TestFOP_TimeoutType1_SuspendAndResume(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	_ = fop.SetT1Initial(1)
	_ = fop.SetTransmissionLimit(1)
	if err := fop.SetTimeoutType(cop.TT1); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	err := fop.Tick(1)
	if !errors.Is(err, cop.ErrFOPSuspended) {
		t.Fatalf("expected ErrFOPSuspended, got %v", err)
	}
	if fop.State() != cop.FOPInitial || fop.SuspendState() != 1 {
		t.Fatalf("state = %d ss = %d, want FOPInitial with SS=1", fop.State(), fop.SuspendState())
	}

	if err := fop.ResumeAD(); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPActive || fop.SuspendState() != 0 {
		t.Errorf("state = %d ss = %d, want FOPActive with SS=0", fop.State(), fop.SuspendState())
	}
}

func TestFOP_WaitFlag_HoldsRetransmissions(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	_ = fop.TransmitFrame([]byte("frame-0"))
	if _, _, ok := fop.GetNextFrame(); !ok {
		t.Fatal("frame not served")
	}

	// Retransmit requested but the FARM has no buffer: hold.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true, WaitFlag: true}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPRetransmitWithWait {
		t.Errorf("state = %d, want FOPRetransmitWithWait", fop.State())
	}
	if _, _, ok := fop.GetNextFrame(); ok {
		t.Error("no retransmission may be served while the Wait flag is set")
	}

	// Wait cleared, retransmit still requested: retransmit now.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPRetransmitWithoutWait {
		t.Errorf("state = %d, want FOPRetransmitWithoutWait", fop.State())
	}
	data, _, ok := fop.GetNextFrame()
	if !ok || !bytes.Equal(data, []byte("frame-0")) {
		t.Errorf("expected retransmission of frame-0, got %q (ok=%v)", data, ok)
	}
}

func TestFOP_InvalidNR_AlertNNR(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	_ = fop.TransmitFrame([]byte("frame-0")) // V(S)=1

	// N(R)=5 acknowledges frames that were never sent.
	err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 5})
	if !errors.Is(err, cop.ErrFOPInvalidNR) {
		t.Fatalf("expected ErrFOPInvalidNR, got %v", err)
	}
	if fop.State() != cop.FOPInitial || fop.LastAlert() != cop.AlertNNR {
		t.Errorf("state = %d alert = %d, want FOPInitial with AlertNNR", fop.State(), fop.LastAlert())
	}
}

func TestFOP_SynchAlert_RetransmitWithNothingOutstanding(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true})
	if !errors.Is(err, cop.ErrFOPSynch) {
		t.Fatalf("expected ErrFOPSynch, got %v", err)
	}
	if fop.LastAlert() != cop.AlertSynch {
		t.Errorf("alert = %d, want AlertSynch", fop.LastAlert())
	}
}

func TestFOP_Directives_Validation(t *testing.T) {
	fop := cop.NewFOP(42, 1, 0) // window 0 clamps to 1
	fop.Initialize(0)
	_ = fop.TransmitFrame([]byte("a"))
	if err := fop.TransmitFrame([]byte("b")); !errors.Is(err, cop.ErrFOPWindowFull) {
		t.Errorf("window must clamp to 1: expected ErrFOPWindowFull, got %v", err)
	}

	if err := fop.SetSlidingWindow(0); !errors.Is(err, cop.ErrFOPInvalidWindow) {
		t.Errorf("expected ErrFOPInvalidWindow, got %v", err)
	}
	if err := fop.SetSlidingWindow(255); err != nil {
		t.Errorf("SetSlidingWindow(255): %v", err)
	}
	if err := fop.SetTransmissionLimit(0); !errors.Is(err, cop.ErrFOPInvalidLimit) {
		t.Errorf("expected ErrFOPInvalidLimit, got %v", err)
	}
	if err := fop.SetTimeoutType(2); !errors.Is(err, cop.ErrFOPInvalidTimeoutType) {
		t.Errorf("expected ErrFOPInvalidTimeoutType, got %v", err)
	}
	if err := fop.SetVS(9); !errors.Is(err, cop.ErrFOPNotInitial) {
		t.Errorf("SetVS outside S6: expected ErrFOPNotInitial, got %v", err)
	}
	if err := fop.ResumeAD(); !errors.Is(err, cop.ErrFOPNotSuspended) {
		t.Errorf("expected ErrFOPNotSuspended, got %v", err)
	}

	fop.TerminateAD()
	if fop.State() != cop.FOPInitial {
		t.Fatalf("state = %d, want FOPInitial after Terminate", fop.State())
	}
	if err := fop.SetVS(9); err != nil {
		t.Errorf("SetVS in S6: %v", err)
	}
	if fop.VS() != 9 {
		t.Errorf("V(S) = %d, want 9", fop.VS())
	}
	if err := fop.TransmitFrame([]byte("c")); !errors.Is(err, cop.ErrFOPNotActive) {
		t.Errorf("expected ErrFOPNotActive in S6, got %v", err)
	}
}

func TestFOP_InitiateADWithCLCWCheck(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	if err := fop.InitiateADWithCLCWCheck(); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPInitialisingWithoutBC {
		t.Fatalf("state = %d, want FOPInitialisingWithoutBC", fop.State())
	}

	// A dirty CLCW does not complete initialisation.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 7, RetransmitFlag: true}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPInitialisingWithoutBC {
		t.Errorf("state = %d, want still FOPInitialisingWithoutBC", fop.State())
	}

	// A clean CLCW does, adopting its V(R).
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 7}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	if fop.VS() != 7 {
		t.Errorf("V(S) = %d, want 7 (adopted from CLCW)", fop.VS())
	}
}

func TestFOP_InitiateADWithUnlock(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	bc := []byte("encoded-unlock-frame")
	if err := fop.InitiateADWithUnlock(bc); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPInitialisingWithBC {
		t.Fatalf("state = %d, want FOPInitialisingWithBC", fop.State())
	}

	data, _, ok := fop.GetNextFrame()
	if !ok || !bytes.Equal(data, bc) {
		t.Fatalf("expected the BC frame to be served, got %q (ok=%v)", data, ok)
	}

	// While lockout persists, keep waiting.
	if err := fop.ProcessCLCW(&cop.CLCW{LockoutFlag: true, ReportValue: 3}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPInitialisingWithBC {
		t.Errorf("state = %d, want still FOPInitialisingWithBC", fop.State())
	}

	// Lockout cleared: the Unlock took effect.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 3}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	if fop.VS() != 3 {
		t.Errorf("V(S) = %d, want 3 (adopted from CLCW after Unlock)", fop.VS())
	}
}

func TestFOP_BDPath_ServedAheadOfAD(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	_ = fop.TransmitFrame([]byte("ad-frame"))
	if err := fop.TransmitBDFrame([]byte("bd-frame")); err != nil {
		t.Fatal(err)
	}

	data, seq, ok := fop.GetNextFrame()
	if !ok || !bytes.Equal(data, []byte("bd-frame")) || seq != 0 {
		t.Fatalf("expected BD frame first, got %q (N(S)=%d)", data, seq)
	}
	data, _, ok = fop.GetNextFrame()
	if !ok || !bytes.Equal(data, []byte("ad-frame")) {
		t.Fatalf("expected AD frame second, got %q", data)
	}
}

func TestFOP_RetransmitDoesNotDuplicateUnpulledFrames(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))

	// Frames are still waiting when a retransmit request arrives.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}

	count := 0
	for {
		if _, _, ok := fop.GetNextFrame(); !ok {
			break
		}
		count++
		if count > 4 {
			t.Fatal("queue never drains; frames are being duplicated")
		}
	}
	if count != 2 {
		t.Errorf("served %d frames, want 2 (no duplicates)", count)
	}
}

// toRetransmitWithoutWait drives a fresh FOP into S2 with two frames sent
// and none acknowledged.
func toRetransmitWithoutWait(t *testing.T) *cop.FOP {
	t.Helper()
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	for _, f := range [][]byte{[]byte("frame-0"), []byte("frame-1")} {
		if err := fop.TransmitFrame(f); err != nil {
			t.Fatalf("TransmitFrame(%q): %v", f, err)
		}
	}
	for i := range 2 {
		if _, _, ok := fop.GetNextFrame(); !ok {
			t.Fatalf("frame %d not served", i)
		}
	}

	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}
	if fop.State() != cop.FOPRetransmitWithoutWait {
		t.Fatalf("state = %d, want FOPRetransmitWithoutWait", fop.State())
	}
	// Drop the queued retransmissions so later assertions see only what the
	// event under test queues.
	for {
		if _, _, ok := fop.GetNextFrame(); !ok {
			break
		}
	}
	return fop
}

func TestFOP_E16_S3_TimerExpiryIsIgnored(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(10); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))
	for i := range 2 {
		if _, _, ok := fop.GetNextFrame(); !ok {
			t.Fatalf("frame %d not served", i)
		}
	}

	// Retransmission asked for while the FARM has no buffer: S3.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true, WaitFlag: true}); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPRetransmitWithWait {
		t.Fatalf("state = %d, want FOPRetransmitWithWait", fop.State())
	}

	// E16 in S3 is Ignore: the Wait flag still stands, so a T1 expiry may
	// not turn into a retransmission.
	if err := fop.Tick(5); err != nil {
		t.Fatalf("T1 expiry in S3: %v", err)
	}
	if fop.State() != cop.FOPRetransmitWithWait {
		t.Errorf("state = %d, want still FOPRetransmitWithWait", fop.State())
	}
	if data, seq, ok := fop.GetNextFrame(); ok {
		t.Errorf("T1 expiry in S3 queued %q (N(S)=%d); nothing may be sent while Wait is set", data, seq)
	}
}

func TestFOP_E5_S2_NoNewAcknowledgementsAlertsSynch(t *testing.T) {
	fop := toRetransmitWithoutWait(t)

	// N(R)=0 acknowledges nothing new and the Retransmit flag has cleared.
	err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0})
	if !errors.Is(err, cop.ErrFOPSynch) {
		t.Fatalf("expected ErrFOPSynch, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertSynch {
		t.Errorf("alert = %d, want AlertSynch", fop.LastAlert())
	}
}

func TestFOP_E6_S2_NewAcknowledgementsReturnToActive(t *testing.T) {
	fop := toRetransmitWithoutWait(t)

	// The contrast to E5: N(R)=1 acknowledges frame-0, so the retransmission
	// did make progress and the machine goes back to S1.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 1}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	if fop.LastAlert() != cop.AlertNone {
		t.Errorf("alert = %d, want AlertNone", fop.LastAlert())
	}
	if fop.PendingCount() != 1 {
		t.Errorf("PendingCount = %d, want 1 (frame-1 still outstanding)", fop.PendingCount())
	}
}

func TestFOP_E1_S1_NothingOutstandingIsIgnored(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	// E1 in S1 is Ignore, not an alert: this is the ordinary steady-state
	// CLCW with nothing to report.
	if err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0}); err != nil {
		t.Fatalf("ProcessCLCW: %v", err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	if fop.LastAlert() != cop.AlertNone {
		t.Errorf("alert = %d, want AlertNone", fop.LastAlert())
	}
}

func TestFOP_E16_S4_FirstExpiryAlertsT1(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(10); err != nil {
		t.Fatal(err)
	}
	if err := fop.InitiateADWithCLCWCheck(); err != nil {
		t.Fatal(err)
	}
	if fop.State() != cop.FOPInitialisingWithoutBC {
		t.Fatalf("state = %d, want FOPInitialisingWithoutBC", fop.State())
	}

	// E16 in S4: nothing can be retransmitted there, so the first expiry
	// ends the initialisation even with the budget untouched.
	err := fop.Tick(5)
	if !errors.Is(err, cop.ErrFOPTimeout) {
		t.Fatalf("expected ErrFOPTimeout, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertT1 {
		t.Errorf("alert = %d, want AlertT1", fop.LastAlert())
	}
}

func TestFOP_E104_S4_FirstExpirySuspends(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(10); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTimeoutType(cop.TT1); err != nil {
		t.Fatal(err)
	}
	if err := fop.InitiateADWithCLCWCheck(); err != nil {
		t.Fatal(err)
	}

	// E104 in S4: the same terminal expiry, but timeout type 1 keeps the
	// service resumable from S4.
	err := fop.Tick(5)
	if !errors.Is(err, cop.ErrFOPSuspended) {
		t.Fatalf("expected ErrFOPSuspended, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.SuspendState() != 4 {
		t.Errorf("SS = %d, want 4", fop.SuspendState())
	}
}

func TestFOP_E101_LimitOneWithNewAcknowledgements(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetTransmissionLimit(1); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))
	for i := range 2 {
		if _, _, ok := fop.GetNextFrame(); !ok {
			t.Fatalf("frame %d not served", i)
		}
	}

	// E101: N(R)=1 acknowledges frame-0, but a limit of 1 forbids sending
	// frame-1 a second time.
	err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 1, RetransmitFlag: true})
	if !errors.Is(err, cop.ErrFOPLimit) {
		t.Fatalf("expected ErrFOPLimit, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertLimit {
		t.Errorf("alert = %d, want AlertLimit", fop.LastAlert())
	}
}

func TestFOP_E102_LimitOneWithoutNewAcknowledgements(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetTransmissionLimit(1); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	_ = fop.TransmitFrame([]byte("frame-1"))
	for i := range 2 {
		if _, _, ok := fop.GetNextFrame(); !ok {
			t.Fatalf("frame %d not served", i)
		}
	}

	// E102: N(R)=0 acknowledges nothing, and the limit is still 1.
	//
	// This row already held before E101 was added: the general E10 guard
	// ("budget spent with nothing new acknowledged") catches it, because
	// Transmission_Count starts at 1 and so already equals a limit of 1.
	// Only E101, where the CLCW does acknowledge something, was missing.
	err := fop.ProcessCLCW(&cop.CLCW{ReportValue: 0, RetransmitFlag: true})
	if !errors.Is(err, cop.ErrFOPLimit) {
		t.Fatalf("expected ErrFOPLimit, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertLimit {
		t.Errorf("alert = %d, want AlertLimit", fop.LastAlert())
	}
}

func TestFOP_E17_S1_TimerExhaustionAlertsT1(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(1); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	if _, _, ok := fop.GetNextFrame(); !ok {
		t.Fatal("frame not served")
	}

	// E17 in S1: a timer-driven exhaustion reports T1, never Limit.
	err := fop.Tick(5)
	if !errors.Is(err, cop.ErrFOPTimeout) {
		t.Fatalf("expected ErrFOPTimeout, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertT1 {
		t.Errorf("alert = %d, want AlertT1 (Alert[Limit] belongs to the CLCW-driven E101/E102)", fop.LastAlert())
	}
}

func TestFOP_E16_S1_RetransmitsAndStaysActive(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)
	if err := fop.SetT1Initial(5); err != nil {
		t.Fatal(err)
	}
	if err := fop.SetTransmissionLimit(10); err != nil {
		t.Fatal(err)
	}

	_ = fop.TransmitFrame([]byte("frame-0"))
	if _, _, ok := fop.GetNextFrame(); !ok {
		t.Fatal("frame not served")
	}

	// E16 in S1 retransmits but leaves the state at S1, because E1 and E5
	// are classified differently in S1 than in S2.
	if err := fop.Tick(5); err != nil {
		t.Fatalf("T1 expiry in S1: %v", err)
	}
	if fop.State() != cop.FOPActive {
		t.Errorf("state = %d, want FOPActive", fop.State())
	}
	data, seq, ok := fop.GetNextFrame()
	if !ok || seq != 0 || !bytes.Equal(data, []byte("frame-0")) {
		t.Fatalf("expected retransmission of frame-0, got %q (N(S)=%d, ok=%v)", data, seq, ok)
	}
	if !fop.TimerRunning() {
		t.Error("T1 must restart after a timer-driven retransmission")
	}
}
