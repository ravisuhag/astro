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

func TestFOP_T1Expiry_RetransmitThenAlertLimit(t *testing.T) {
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

	// First expiry: transmission count 1 < limit 2 -> retransmission.
	if err := fop.Tick(5); err != nil {
		t.Fatalf("first T1 expiry: %v", err)
	}
	if fop.State() != cop.FOPRetransmitWithoutWait {
		t.Errorf("state = %d, want FOPRetransmitWithoutWait", fop.State())
	}
	data, seq, ok := fop.GetNextFrame()
	if !ok || seq != 0 || !bytes.Equal(data, []byte("frame-0")) {
		t.Fatalf("expected retransmission of frame-0, got %q (N(S)=%d, ok=%v)", data, seq, ok)
	}
	if !fop.TimerRunning() {
		t.Fatal("T1 must restart after a timer-driven retransmission")
	}

	// Second expiry: limit reached -> Alert(LIMIT), purge, Initial.
	err := fop.Tick(5)
	if !errors.Is(err, cop.ErrFOPLimit) {
		t.Fatalf("expected ErrFOPLimit, got %v", err)
	}
	if fop.State() != cop.FOPInitial {
		t.Errorf("state = %d, want FOPInitial", fop.State())
	}
	if fop.LastAlert() != cop.AlertLimit {
		t.Errorf("alert = %d, want AlertLimit", fop.LastAlert())
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
