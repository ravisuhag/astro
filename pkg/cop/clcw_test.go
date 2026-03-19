package cop_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cop"
)

func TestCLCW_EncodeDecode(t *testing.T) {
	clcw := cop.CLCW{
		COPInEffect:      1,
		VirtualChannelID: 5,
		LockoutFlag:      true,
		RetransmitFlag:   true,
		FARMBCounter:     2,
		ReportValue:      42,
	}
	encoded, err := clcw.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 4 {
		t.Fatalf("encoded length = %d, want 4", len(encoded))
	}

	var decoded cop.CLCW
	if err := decoded.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.COPInEffect != 1 {
		t.Errorf("COP = %d, want 1", decoded.COPInEffect)
	}
	if decoded.VirtualChannelID != 5 {
		t.Errorf("VCID = %d, want 5", decoded.VirtualChannelID)
	}
	if !decoded.LockoutFlag {
		t.Error("LockoutFlag should be true")
	}
	if !decoded.RetransmitFlag {
		t.Error("RetransmitFlag should be true")
	}
	if decoded.FARMBCounter != 2 {
		t.Errorf("FARMB = %d, want 2", decoded.FARMBCounter)
	}
	if decoded.ReportValue != 42 {
		t.Errorf("ReportValue = %d, want 42", decoded.ReportValue)
	}
}

func TestCLCW_AllFlags(t *testing.T) {
	clcw := cop.CLCW{
		COPInEffect:       1,
		VirtualChannelID:  63,
		NoRFAvailableFlag: true,
		NoBitLockFlag:     true,
		LockoutFlag:       true,
		WaitFlag:          true,
		RetransmitFlag:    true,
		FARMBCounter:      3,
		ReportValue:       255,
	}
	encoded, _ := clcw.Encode()
	var decoded cop.CLCW
	_ = decoded.Decode(encoded)

	if !decoded.NoRFAvailableFlag {
		t.Error("NoRF should be true")
	}
	if !decoded.NoBitLockFlag {
		t.Error("NoBitLock should be true")
	}
	if !decoded.WaitFlag {
		t.Error("Wait should be true")
	}
	if decoded.VirtualChannelID != 63 {
		t.Errorf("VCID = %d, want 63", decoded.VirtualChannelID)
	}
	if decoded.FARMBCounter != 3 {
		t.Errorf("FARMB = %d, want 3", decoded.FARMBCounter)
	}
	if decoded.ReportValue != 255 {
		t.Errorf("ReportValue = %d, want 255", decoded.ReportValue)
	}
}

func TestCLCW_InvalidType(t *testing.T) {
	clcw := cop.CLCW{ControlWordType: 1}
	_, err := clcw.Encode()
	if !errors.Is(err, cop.ErrInvalidCLCWType) {
		t.Errorf("expected ErrInvalidCLCWType, got %v", err)
	}
}

func TestCLCW_TooShort(t *testing.T) {
	var clcw cop.CLCW
	if !errors.Is(clcw.Decode([]byte{0x00}), cop.ErrDataTooShort) {
		t.Error("expected ErrDataTooShort")
	}
}

func TestCLCW_RoundTrip_Zero(t *testing.T) {
	clcw := cop.CLCW{ReportValue: 0}
	encoded, _ := clcw.Encode()
	var decoded cop.CLCW
	_ = decoded.Decode(encoded)
	if decoded.ReportValue != 0 {
		t.Errorf("ReportValue = %d, want 0", decoded.ReportValue)
	}
}
