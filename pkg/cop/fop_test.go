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
		fop.TransmitFrame([]byte("frame"))
	}

	err := fop.TransmitFrame([]byte("overflow"))
	if !errors.Is(err, cop.ErrFOPWindowFull) {
		t.Errorf("expected ErrFOPWindowFull, got %v", err)
	}
}

func TestFOP_ProcessCLCW_Acknowledgment(t *testing.T) {
	fop := cop.NewFOP(42, 1, 10)
	fop.Initialize(0)

	fop.TransmitFrame([]byte("frame-0"))
	fop.TransmitFrame([]byte("frame-1"))
	fop.TransmitFrame([]byte("frame-2"))

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
	fop.TransmitFrame([]byte("frame"))

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

	fop.TransmitFrame([]byte("frame-0"))
	fop.TransmitFrame([]byte("frame-1"))

	clcw := &cop.CLCW{ReportValue: 0, RetransmitFlag: true}
	fop.ProcessCLCW(clcw)

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
		fop.TransmitFrame([]byte{byte(i)})
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

	fop.ProcessCLCW(clcw)
	if fop.PendingCount() != 0 {
		t.Errorf("PendingCount = %d, want 0", fop.PendingCount())
	}
}
