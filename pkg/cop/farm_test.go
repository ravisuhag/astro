package cop_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

func TestFARM_TypeA_InSequence(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	accepted, err := farm.ProcessFrame(0, 0, 0, nil)
	if err != nil || !accepted {
		t.Fatalf("frame 0: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 1 {
		t.Errorf("V(R) = %d, want 1", farm.VR())
	}

	accepted, err = farm.ProcessFrame(0, 0, 1, nil)
	if err != nil || !accepted {
		t.Fatalf("frame 1: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 2 {
		t.Errorf("V(R) = %d, want 2", farm.VR())
	}
}

func TestFARM_TypeA_OutOfSequence_Retransmit(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	_, _ = farm.ProcessFrame(0, 0, 0, nil)

	accepted, err := farm.ProcessFrame(0, 0, 2, nil)
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
	_, _ = farm.ProcessFrame(0, 0, 0, nil)

	accepted, err := farm.ProcessFrame(0, 0, 100, nil)
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

func TestFARM_NegativeWindow_DuplicateDiscardedSilently(t *testing.T) {
	// A duplicate of the frame just accepted lands in the negative half
	// of the window (E4): it must be discarded silently — no lockout, no
	// retransmit request, no error.
	farm := cop.NewFARM(1, 10) // W=10 -> PW=NW=5
	if _, err := farm.ProcessFrame(0, 0, 0, nil); err != nil {
		t.Fatal(err)
	}

	accepted, err := farm.ProcessFrame(0, 0, 0, nil) // duplicate of N(S)=0
	if accepted {
		t.Error("duplicate frame must be discarded")
	}
	if err != nil {
		t.Errorf("duplicate discard must be silent, got %v", err)
	}
	if farm.State() != cop.FARMOpen {
		t.Errorf("state = %d, want FARMOpen (no lockout for duplicates)", farm.State())
	}

	clcw := farm.GenerateCLCW()
	if clcw.LockoutFlag {
		t.Error("Lockout flag must not be set by a duplicate frame")
	}
	if clcw.RetransmitFlag {
		t.Error("Retransmit flag must not be set by a duplicate frame")
	}
	if clcw.ReportValue != 1 {
		t.Errorf("V(R) = %d, want 1", clcw.ReportValue)
	}
}

func TestFARM_NegativeWindow_Boundaries(t *testing.T) {
	// W=10: positive window covers V(R)+1..V(R)+4, negative window
	// V(R)-5..V(R)-1. Anything else is lockout.
	farm := cop.NewFARM(1, 10)
	for i := range 10 {
		if _, err := farm.ProcessFrame(0, 0, uint8(i), nil); err != nil {
			t.Fatal(err)
		}
	}
	// V(R) is now 10.
	if _, err := farm.ProcessFrame(0, 0, 5, nil); err != nil {
		t.Errorf("N(S)=5 (V(R)-5) is inside the negative window, got %v", err)
	}
	if _, err := farm.ProcessFrame(0, 0, 14, nil); !errors.Is(err, cop.ErrFARMReject) {
		t.Errorf("N(S)=14 (V(R)+4) is inside the positive window, got %v", err)
	}
	if farm.State() != cop.FARMOpen {
		t.Fatalf("state = %d, want FARMOpen", farm.State())
	}
	// One below the negative window: lockout.
	if _, err := farm.ProcessFrame(0, 0, 4, nil); !errors.Is(err, cop.ErrFARMLockout) {
		t.Errorf("N(S)=4 (V(R)-6) is outside both windows, got %v", err)
	}
}

func TestFARM_TypeBD_AlwaysAccepted(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	accepted, err := farm.ProcessFrame(1, 0, 0, []byte("expedited"))
	if err != nil || !accepted {
		t.Fatalf("Type-BD: accepted=%v err=%v", accepted, err)
	}

	clcw := farm.GenerateCLCW()
	if clcw.FARMBCounter != 1 {
		t.Errorf("FARMB = %d, want 1", clcw.FARMBCounter)
	}
}

func TestFARM_Unlock_ClearsLockoutWithoutTouchingVR(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	_, _ = farm.ProcessFrame(0, 0, 0, nil)   // V(R)=1
	_, _ = farm.ProcessFrame(0, 0, 100, nil) // lockout
	if farm.State() != cop.FARMLockout {
		t.Fatal("expected lockout")
	}

	// A spec-compliant Unlock: Type-BC frame (Bypass=1, CC=1), data 0x00.
	accepted, err := farm.ProcessFrame(1, 1, 0, []byte{0x00})
	if err != nil || !accepted {
		t.Fatalf("Unlock: accepted=%v err=%v", accepted, err)
	}
	if farm.State() != cop.FARMOpen {
		t.Errorf("state = %d, want FARMOpen", farm.State())
	}
	if farm.VR() != 1 {
		t.Errorf("V(R) = %d, want 1 (Unlock must not touch V(R))", farm.VR())
	}

	clcw := farm.GenerateCLCW()
	if clcw.LockoutFlag || clcw.RetransmitFlag || clcw.WaitFlag {
		t.Error("Unlock must clear Lockout, Retransmit and Wait flags")
	}
	if clcw.FARMBCounter != 1 {
		t.Errorf("FARMB = %d, want 1 (BC frames count too)", clcw.FARMBCounter)
	}
}

func TestFARM_SetVR_SetsVRFromDirectivePayload(t *testing.T) {
	farm := cop.NewFARM(1, 10)

	// Set V(R) carries the value in its payload, NOT in the frame
	// sequence number (which is 0 on Type-B frames).
	accepted, err := farm.ProcessFrame(1, 1, 0, []byte{0x82, 0x00, 42})
	if err != nil || !accepted {
		t.Fatalf("Set V(R): accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 42 {
		t.Errorf("V(R) = %d, want 42", farm.VR())
	}
	if clcw := farm.GenerateCLCW(); clcw.FARMBCounter != 1 {
		t.Errorf("FARMB = %d, want 1", clcw.FARMBCounter)
	}
}

func TestFARM_SetVR_InLockoutOnlyCountsFARMB(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	_, _ = farm.ProcessFrame(0, 0, 100, nil) // lockout, V(R)=0

	accepted, err := farm.ProcessFrame(1, 1, 0, []byte{0x82, 0x00, 42})
	if err != nil || !accepted {
		t.Fatalf("Set V(R) in lockout: accepted=%v err=%v", accepted, err)
	}
	if farm.State() != cop.FARMLockout {
		t.Error("Set V(R) must not clear lockout (only Unlock does)")
	}
	if farm.VR() != 0 {
		t.Errorf("V(R) = %d, want 0 (unchanged in lockout)", farm.VR())
	}
	if clcw := farm.GenerateCLCW(); clcw.FARMBCounter != 1 {
		t.Errorf("FARMB = %d, want 1", clcw.FARMBCounter)
	}
}

func TestFARM_InvalidControlCommand_Discarded(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	accepted, err := farm.ProcessFrame(1, 1, 0, []byte{0x99})
	if accepted {
		t.Error("invalid control command must be discarded")
	}
	if !errors.Is(err, cop.ErrInvalidControlCommand) {
		t.Errorf("expected ErrInvalidControlCommand, got %v", err)
	}
}

func TestFARM_InvalidFrameType_Discarded(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	accepted, err := farm.ProcessFrame(0, 1, 0, []byte{0x00})
	if accepted {
		t.Error("Bypass=0 + CC=1 must be discarded")
	}
	if !errors.Is(err, cop.ErrInvalidFrameType) {
		t.Errorf("expected ErrInvalidFrameType, got %v", err)
	}
}

func TestFARM_FARMBCounter_CountsBDAndBC(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	_, _ = farm.ProcessFrame(1, 0, 0, []byte("bd"))          // BD
	_, _ = farm.ProcessFrame(1, 1, 0, []byte{0x00})          // BC Unlock
	_, _ = farm.ProcessFrame(1, 1, 0, []byte{0x82, 0x00, 0}) // BC Set V(R)

	if clcw := farm.GenerateCLCW(); clcw.FARMBCounter != 3 {
		t.Errorf("FARMB = %d, want 3 (every accepted Type-B frame counts)", clcw.FARMBCounter)
	}
}

func TestFARM_WaitState_BufferExhaustion(t *testing.T) {
	farm := cop.NewFARM(1, 10)
	farm.SetBuffers(1)

	// First frame consumes the only buffer.
	accepted, err := farm.ProcessFrame(0, 0, 0, nil)
	if err != nil || !accepted {
		t.Fatalf("frame 0: accepted=%v err=%v", accepted, err)
	}
	if farm.State() != cop.FARMWait {
		t.Errorf("state = %d, want FARMWait after last buffer used", farm.State())
	}

	// Next in-sequence frame is discarded: no buffer (E2).
	accepted, err = farm.ProcessFrame(0, 0, 1, nil)
	if accepted {
		t.Error("frame must be discarded with no buffer available")
	}
	if !errors.Is(err, cop.ErrFARMWait) {
		t.Errorf("expected ErrFARMWait, got %v", err)
	}
	clcw := farm.GenerateCLCW()
	if !clcw.WaitFlag || !clcw.RetransmitFlag {
		t.Error("Wait and Retransmit flags must be set when a frame is discarded for lack of buffers")
	}
	if farm.VR() != 1 {
		t.Errorf("V(R) = %d, want 1 (discarded frame must not advance V(R))", farm.VR())
	}

	// Releasing the buffer exits Wait; the retransmitted frame is accepted.
	farm.ReleaseBuffer()
	if farm.State() != cop.FARMOpen {
		t.Errorf("state = %d, want FARMOpen after buffer release", farm.State())
	}
	if clcw := farm.GenerateCLCW(); clcw.WaitFlag {
		t.Error("Wait flag must clear after buffer release")
	}
	accepted, err = farm.ProcessFrame(0, 0, 1, nil)
	if err != nil || !accepted {
		t.Fatalf("retransmitted frame: accepted=%v err=%v", accepted, err)
	}
	if farm.VR() != 2 {
		t.Errorf("V(R) = %d, want 2", farm.VR())
	}
}

func TestFARM_GenerateCLCW(t *testing.T) {
	farm := cop.NewFARM(7, 10)
	_, _ = farm.ProcessFrame(0, 0, 0, nil)

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
	_ = decoded.Decode(encoded)
	if decoded.ReportValue != 1 {
		t.Errorf("decoded V(R) = %d, want 1", decoded.ReportValue)
	}
}
