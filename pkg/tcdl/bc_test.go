package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

func TestControlCommand_UnlockRoundTrip(t *testing.T) {
	data := tcdl.BuildUnlockCommand()
	if !bytes.Equal(data, []byte{0x00}) {
		t.Fatalf("Unlock data = %x, want 00", data)
	}
	typ, vr, err := tcdl.ParseControlCommand(data)
	if err != nil {
		t.Fatal(err)
	}
	if typ != tcdl.ControlUnlock || vr != 0 {
		t.Errorf("parsed (%v, %d), want (ControlUnlock, 0)", typ, vr)
	}
}

func TestControlCommand_SetVRRoundTrip(t *testing.T) {
	data := tcdl.BuildSetVRCommand(42)
	if !bytes.Equal(data, []byte{0x82, 0x00, 42}) {
		t.Fatalf("Set V(R) data = %x, want 82002a", data)
	}
	typ, vr, err := tcdl.ParseControlCommand(data)
	if err != nil {
		t.Fatal(err)
	}
	if typ != tcdl.ControlSetVR || vr != 42 {
		t.Errorf("parsed (%v, %d), want (ControlSetVR, 42)", typ, vr)
	}
}

func TestControlCommand_InvalidContents(t *testing.T) {
	bad := [][]byte{
		nil,
		{0x01},
		{0x82, 0x01, 0x05},
		{0x82, 0x00},
		{0x00, 0x00},
		{0x82, 0x00, 0x05, 0x00},
	}
	for _, data := range bad {
		if _, _, err := tcdl.ParseControlCommand(data); !errors.Is(err, tcdl.ErrInvalidControlCommand) {
			t.Errorf("data %x: expected ErrInvalidControlCommand, got %v", data, err)
		}
	}
}

func TestNewUnlockFrame(t *testing.T) {
	frame, err := tcdl.NewUnlockFrame(42, 1)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Header.BypassFlag != 1 || frame.Header.ControlCommandFlag != 1 {
		t.Errorf("flags = (%d,%d), want Type-BC (1,1)",
			frame.Header.BypassFlag, frame.Header.ControlCommandFlag)
	}
	if frame.Header.FrameSequenceNum != 0 {
		t.Errorf("N(S) = %d, want 0", frame.Header.FrameSequenceNum)
	}
	if frame.SegmentHeader != nil {
		t.Error("BC frame must not carry a segment header")
	}
	if !bytes.Equal(frame.DataField, []byte{0x00}) {
		t.Errorf("data field = %x, want 00", frame.DataField)
	}

	// And it survives an encode/decode round trip.
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tcdl.DecodeTCTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	typ, _, err := tcdl.ParseControlCommand(decoded.DataField)
	if err != nil || typ != tcdl.ControlUnlock {
		t.Errorf("decoded control command = (%v, %v), want ControlUnlock", typ, err)
	}
}

func TestNewSetVRFrame(t *testing.T) {
	frame, err := tcdl.NewSetVRFrame(42, 1, 99)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tcdl.DecodeTCTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	typ, vr, err := tcdl.ParseControlCommand(decoded.DataField)
	if err != nil {
		t.Fatal(err)
	}
	if typ != tcdl.ControlSetVR || vr != 99 {
		t.Errorf("parsed (%v, %d), want (ControlSetVR, 99)", typ, vr)
	}
}

func TestDecode_RejectsInvalidFrameType(t *testing.T) {
	// Hand-build a frame with Bypass=0, Control Command=1: byte 0 has the
	// control command bit (bit 4) set and the bypass bit (bit 5) clear.
	frame, err := tcdl.NewTCTransferFrame(42, 1, []byte{0x00})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] |= 1 << 4 // set Control Command, Bypass stays 0
	if _, err := tcdl.DecodeTCTransferFrame(encoded); !errors.Is(err, tcdl.ErrInvalidFrameType) {
		t.Errorf("expected ErrInvalidFrameType, got %v", err)
	}
}
