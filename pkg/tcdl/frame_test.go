package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

// --- Primary Header Tests ---

func TestPrimaryHeader_EncodeDecode(t *testing.T) {
	h := tcdl.PrimaryHeader{
		SpacecraftID:     42,
		VirtualChannelID: 5,
		BypassFlag:       1,
		FrameSequenceNum: 100,
	}

	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 5 {
		t.Fatalf("encoded length = %d, want 5", len(encoded))
	}

	var decoded tcdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SpacecraftID != 42 {
		t.Errorf("SCID = %d, want 42", decoded.SpacecraftID)
	}
	if decoded.VirtualChannelID != 5 {
		t.Errorf("VCID = %d, want 5", decoded.VirtualChannelID)
	}
	if decoded.BypassFlag != 1 {
		t.Errorf("BypassFlag = %d, want 1", decoded.BypassFlag)
	}
	if decoded.FrameSequenceNum != 100 {
		t.Errorf("FrameSeqNum = %d, want 100", decoded.FrameSequenceNum)
	}
}

func TestPrimaryHeader_Validate(t *testing.T) {
	h := tcdl.PrimaryHeader{VersionNumber: 1}
	if !errors.Is(h.Validate(), tcdl.ErrInvalidVersion) {
		t.Error("Expected ErrInvalidVersion")
	}
	h = tcdl.PrimaryHeader{Reserved: 1}
	if !errors.Is(h.Validate(), tcdl.ErrInvalidReservedBits) {
		t.Error("Expected ErrInvalidReservedBits")
	}
	h = tcdl.PrimaryHeader{SpacecraftID: 2000}
	if !errors.Is(h.Validate(), tcdl.ErrInvalidSpacecraftID) {
		t.Error("Expected ErrInvalidSpacecraftID")
	}
	h = tcdl.PrimaryHeader{VirtualChannelID: 64}
	if !errors.Is(h.Validate(), tcdl.ErrInvalidVCID) {
		t.Error("Expected ErrInvalidVCID")
	}
}

func TestPrimaryHeader_MCID_GVCID(t *testing.T) {
	h := tcdl.PrimaryHeader{SpacecraftID: 100, VirtualChannelID: 5}
	if h.MCID() != 100 {
		t.Errorf("MCID = %d, want 100", h.MCID())
	}
	expected := uint32(100)<<6 | 5
	if h.GVCID() != expected {
		t.Errorf("GVCID = %d, want %d", h.GVCID(), expected)
	}
}

func TestSegmentHeader_EncodeDecode(t *testing.T) {
	sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegFirst, MAPID: 10}
	encoded, err := sh.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 1 {
		t.Fatalf("encoded length = %d, want 1", len(encoded))
	}

	var decoded tcdl.SegmentHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SequenceFlags != tcdl.SegFirst {
		t.Errorf("SequenceFlags = %d, want %d", decoded.SequenceFlags, tcdl.SegFirst)
	}
	if decoded.MAPID != 10 {
		t.Errorf("MAPID = %d, want 10", decoded.MAPID)
	}
}

// --- Transfer Frame Tests ---

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

func TestTCFrame_DecodeWithSegmentHeaderRoundTrip(t *testing.T) {
	data := []byte("payload")
	sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegFirst, MAPID: 7}
	frame, err := tcdl.NewTCTransferFrame(100, 3, data,
		tcdl.WithSegmentHeader(sh),
		tcdl.WithSequenceNumber(42),
	)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// Decode with segment header awareness
	decoded, err := tcdl.DecodeTCTransferFrameWithSegmentHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeTCTransferFrameWithSegmentHeader failed: %v", err)
	}

	if decoded.SegmentHeader == nil {
		t.Fatal("expected SegmentHeader to be non-nil")
	}
	if decoded.SegmentHeader.SequenceFlags != tcdl.SegFirst {
		t.Errorf("SequenceFlags = %d, want %d", decoded.SegmentHeader.SequenceFlags, tcdl.SegFirst)
	}
	if decoded.SegmentHeader.MAPID != 7 {
		t.Errorf("MAPID = %d, want 7", decoded.SegmentHeader.MAPID)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %q, want %q", decoded.DataField, data)
	}

	// Re-encode and compare bytes
	reEncoded, err := decoded.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, reEncoded) {
		t.Error("roundtrip encode produced different bytes")
	}
}

func TestTCFrame_DecodeWithoutSegmentHeader(t *testing.T) {
	// Ensure the original DecodeTCTransferFrame still works as before.
	data := []byte("test")
	frame, _ := tcdl.NewTCTransferFrame(42, 5, data)
	encoded, _ := frame.Encode()

	decoded, err := tcdl.DecodeTCTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SegmentHeader != nil {
		t.Error("expected SegmentHeader to be nil for basic decode")
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %q, want %q", decoded.DataField, data)
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
