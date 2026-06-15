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
		accepted, err := farm.ProcessFrame(0, 0, uint8(i))
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
