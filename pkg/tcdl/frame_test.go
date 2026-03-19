package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

func TestTCFrame_NewAndEncode(t *testing.T) {
	frame, err := tcdl.NewTCTransferFrame(42, 5, []byte("command data"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 19 {
		t.Errorf("encoded length = %d, want 19", len(encoded))
	}
	if frame.Header.FrameLength != 18 {
		t.Errorf("FrameLength = %d, want 18", frame.Header.FrameLength)
	}
}

func TestTCFrame_RoundTrip(t *testing.T) {
	data := []byte("telecommand payload")
	frame, _ := tcdl.NewTCTransferFrame(42, 5, data,
		tcdl.WithBypass(),
		tcdl.WithSequenceNumber(7),
	)
	encoded, _ := frame.Encode()
	decoded, err := tcdl.DecodeTCTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.SpacecraftID != 42 {
		t.Errorf("SCID = %d, want 42", decoded.Header.SpacecraftID)
	}
	if decoded.Header.VirtualChannelID != 5 {
		t.Errorf("VCID = %d, want 5", decoded.Header.VirtualChannelID)
	}
	if decoded.Header.BypassFlag != 1 {
		t.Errorf("BypassFlag = %d, want 1", decoded.Header.BypassFlag)
	}
	if decoded.Header.FrameSequenceNum != 7 {
		t.Errorf("FrameSeqNum = %d, want 7", decoded.Header.FrameSequenceNum)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %q, want %q", decoded.DataField, data)
	}
}

func TestTCFrame_WithSegmentHeader(t *testing.T) {
	sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegUnsegmented, MAPID: 3}
	frame, err := tcdl.NewTCTransferFrame(42, 5, []byte("data"),
		tcdl.WithSegmentHeader(sh),
	)
	if err != nil {
		t.Fatal(err)
	}
	if frame.SegmentHeader == nil {
		t.Fatal("SegmentHeader should not be nil")
	}
	encoded, _ := frame.Encode()
	if len(encoded) != 12 {
		t.Errorf("encoded length = %d, want 12", len(encoded))
	}
}

func TestTCFrame_CRCMismatch(t *testing.T) {
	frame, _ := tcdl.NewTCTransferFrame(42, 5, []byte("test"))
	encoded, _ := frame.Encode()
	encoded[6] ^= 0x01
	_, err := tcdl.DecodeTCTransferFrame(encoded)
	if !errors.Is(err, tcdl.ErrCRCMismatch) {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestTCFrame_TooLarge(t *testing.T) {
	data := make([]byte, 1020)
	_, err := tcdl.NewTCTransferFrame(42, 5, data)
	if !errors.Is(err, tcdl.ErrDataTooLarge) {
		t.Errorf("expected ErrDataTooLarge, got %v", err)
	}
}

func TestTCFrame_IsControlAndBypass(t *testing.T) {
	frame, _ := tcdl.NewTCTransferFrame(42, 5, []byte{0x00},
		tcdl.WithBypass(), tcdl.WithControlCommand())
	if !tcdl.IsBypass(frame) {
		t.Error("expected IsBypass=true")
	}
	if !tcdl.IsControlFrame(frame) {
		t.Error("expected IsControlFrame=true")
	}
}
