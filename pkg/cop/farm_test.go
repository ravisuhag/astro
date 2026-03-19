package cop_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

func TestFARM_TypeA_InSequence(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	accepted, err := farm.ProcessFrame(0, 0, 0)
	if err != nil || !accepted {
		t.Fatalf("frame 0: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 1 {
		t.Errorf("V(R) = %d, want 1", farm.VR())
	}

	accepted, err = farm.ProcessFrame(0, 0, 1)
	if err != nil || !accepted {
		t.Fatalf("frame 1: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 2 {
		t.Errorf("V(R) = %d, want 2", farm.VR())
	}
}

func TestFARM_TypeA_OutOfSequence_Retransmit(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	farm.ProcessFrame(0, 0, 0)

	accepted, err := farm.ProcessFrame(0, 0, 2)
	if accepted {
		t.Error("frame 2 should be rejected (out of sequence)")
	}
	if !errors.Is(err, cop.ErrFARMReject) {
		t.Errorf("expected ErrFARMReject, got %v", err)
	}

	clcw := farm.GenerateCLCW()
	if !clcw.RetransmitFlag {
		t.Error("Retransmit flag should be set")
	}
	if clcw.ReportValue != 1 {
		t.Errorf("V(R) = %d, want 1", clcw.ReportValue)
	}
}

func TestFARM_TypeA_OutsideWindow_Lockout(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	farm.ProcessFrame(0, 0, 0)

	accepted, err := farm.ProcessFrame(0, 0, 100)
	if accepted {
		t.Error("frame 100 should be rejected")
	}
	if !errors.Is(err, cop.ErrFARMLockout) {
		t.Errorf("expected ErrFARMLockout, got %v", err)
	}
	if farm.State() != cop.FARMLockout {
		t.Errorf("state = %d, want FARMLockout", farm.State())
	}

	clcw := farm.GenerateCLCW()
	if !clcw.LockoutFlag {
		t.Error("Lockout flag should be set")
	}
}

func TestFARM_TypeB_AlwaysAccepted(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	accepted, err := farm.ProcessFrame(1, 0, 99)
	if err != nil || !accepted {
		t.Fatalf("Type-B: accepted=%v err=%v", accepted, err)
	}

	clcw := farm.GenerateCLCW()
	if clcw.FARMBCounter != 1 {
		t.Errorf("FARMB = %d, want 1", clcw.FARMBCounter)
	}
}

func TestFARM_ControlCommand_Unlock(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	farm.ProcessFrame(0, 0, 0)
	farm.ProcessFrame(0, 0, 100) // lockout

	accepted, _ := farm.ProcessFrame(0, 1, 5)
	if !accepted {
		t.Error("control command should be accepted")
	}
	if farm.State() != cop.FARMOpen {
		t.Errorf("state = %d, want FARMOpen", farm.State())
	}
	if farm.VR() != 5 {
		t.Errorf("V(R) = %d, want 5", farm.VR())
	}
}

func TestFARM_GenerateCLCW(t *testing.T) {
	farm := cop.NewFARM(7, 10)
	farm.ProcessFrame(0, 0, 0)

	clcw := farm.GenerateCLCW()
	if clcw.VirtualChannelID != 7 {
		t.Errorf("VCID = %d, want 7", clcw.VirtualChannelID)
	}
	if clcw.COPInEffect != 1 {
		t.Errorf("COP = %d, want 1", clcw.COPInEffect)
	}
	if clcw.ReportValue != 1 {
		t.Errorf("V(R) = %d, want 1", clcw.ReportValue)
	}

	encoded, _ := clcw.Encode()
	var decoded cop.CLCW
	decoded.Decode(encoded)
	if decoded.ReportValue != 1 {
		t.Errorf("decoded V(R) = %d, want 1", decoded.ReportValue)
	}
}
