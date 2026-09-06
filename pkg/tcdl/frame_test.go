package tcdl_test

import (
	"bytes"
	"errors"
	"fmt"
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
	frame, err := tcdl.NewTransferFrame(42, 5, []byte("command data"))
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

// TestTCFrame_EncodeRefreshesFrameLengthAfterMutation reproduces B2:
// DataField is exported, so a caller can grow or shrink it after
// construction. Encode must recompute Header.FrameLength from what is
// actually there, not reuse the value NewTCTransferFrame set once, or the
// receiver reads the CRC from the wrong offset and rejects the frame.
func TestTCFrame_EncodeRefreshesFrameLengthAfterMutation(t *testing.T) {
	frame, err := tcdl.NewTransferFrame(42, 5, []byte("short"))
	if err != nil {
		t.Fatal(err)
	}

	frame.DataField = []byte("a much longer command data field than before")

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := tcdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatalf("Decode: %v (FrameLength = %d, encoded length = %d)",
			err, frame.Header.FrameLength, len(encoded))
	}
	if !bytes.Equal(decoded.DataField, frame.DataField) {
		t.Errorf("DataField = %q, want %q", decoded.DataField, frame.DataField)
	}
}

func TestTCFrame_RoundTrip(t *testing.T) {
	data := []byte("telecommand payload")
	frame, _ := tcdl.NewTransferFrame(42, 5, data,
		tcdl.WithSequenceNumber(7),
	)
	encoded, _ := frame.Encode()
	decoded, err := tcdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Header.SpacecraftID != 42 {
		t.Errorf("SCID = %d, want 42", decoded.Header.SpacecraftID)
	}
	if decoded.Header.VirtualChannelID != 5 {
		t.Errorf("VCID = %d, want 5", decoded.Header.VirtualChannelID)
	}
	if decoded.Header.BypassFlag != 0 {
		t.Errorf("BypassFlag = %d, want 0 (Type-A carries N(S))", decoded.Header.BypassFlag)
	}
	if decoded.Header.FrameSequenceNum != 7 {
		t.Errorf("FrameSeqNum = %d, want 7", decoded.Header.FrameSequenceNum)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %q, want %q", decoded.DataField, data)
	}
}

func TestTCFrame_TypeBForcesZeroSequenceNumber(t *testing.T) {
	// Per CCSDS 232.0-B-4 4.1.2.7, Type-B frames carry N(S) = all zeros.
	frame, err := tcdl.NewTransferFrame(42, 5, []byte("x"),
		tcdl.WithBypass(),
		tcdl.WithSequenceNumber(7),
	)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Header.FrameSequenceNum != 0 {
		t.Errorf("Type-B FrameSeqNum = %d, want 0", frame.Header.FrameSequenceNum)
	}
}

func TestTCFrame_RejectsInvalidType(t *testing.T) {
	// Bypass=0 with Control Command=1 is invalid (CCSDS 232.0-B-4 4.1.2.3).
	h := tcdl.PrimaryHeader{
		BypassFlag:         0,
		ControlCommandFlag: 1,
		SpacecraftID:       42,
		FrameLength:        7,
	}
	if err := h.Validate(); err == nil {
		t.Error("expected error for Bypass=0 + Control Command=1, got nil")
	}
}

func TestTCFrame_WithSegmentHeader(t *testing.T) {
	sh := tcdl.SegmentHeader{SequenceFlags: tcdl.SegUnsegmented, MAPID: 3}
	frame, err := tcdl.NewTransferFrame(42, 5, []byte("data"),
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
	frame, err := tcdl.NewTransferFrame(100, 3, data,
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
	frame, _ := tcdl.NewTransferFrame(42, 5, data)
	encoded, _ := frame.Encode()

	decoded, err := tcdl.DecodeTransferFrame(encoded)
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
	frame, _ := tcdl.NewTransferFrame(42, 5, []byte("test"))
	encoded, _ := frame.Encode()
	encoded[6] ^= 0x01
	_, err := tcdl.DecodeTransferFrame(encoded)
	if !errors.Is(err, tcdl.ErrCRCMismatch) {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestTCFrame_TooLarge(t *testing.T) {
	data := make([]byte, 1020)
	_, err := tcdl.NewTransferFrame(42, 5, data)
	if !errors.Is(err, tcdl.ErrDataTooLarge) {
		t.Errorf("expected ErrDataTooLarge, got %v", err)
	}
}

func TestTCFrame_IsControlAndBypass(t *testing.T) {
	frame, _ := tcdl.NewTransferFrame(42, 5, []byte{0x00},
		tcdl.WithBypass(), tcdl.WithControlCommand())
	if !tcdl.IsBypass(frame) {
		t.Error("expected IsBypass=true")
	}
	if !tcdl.IsControlFrame(frame) {
		t.Error("expected IsControlFrame=true")
	}
}

func TestDecodeTCFrame_MalformedLengthDoesNotPanic(t *testing.T) {
	cases := map[string][]byte{
		// FrameLength=0 header: 7 zero bytes decode as a valid version-0 header,
		// expectedLen=1 used to slice data[-1:1].
		"zero length field": make([]byte, 7),
	}
	// FrameLength values 1..5 give expectedLen 2..6, below header+FECF.
	for fl := 1; fl <= 5; fl++ {
		b := make([]byte, 7)
		b[2] = byte(fl >> 8 & 0x03)
		b[3] = byte(fl & 0xFF)
		cases[fmt.Sprintf("length field %d", fl)] = b
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := tcdl.DecodeTransferFrame(data); err == nil {
				t.Fatal("expected error for malformed frame, got nil")
			}
			if _, err := tcdl.DecodeTCTransferFrameWithSegmentHeader(data); err == nil {
				t.Fatal("expected error for malformed frame, got nil")
			}
		})
	}
}

func TestTCFrame_EncodeRecomputesCRCAfterMutation(t *testing.T) {
	frame, err := tcdl.NewTransferFrame(42, 1, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	frame.Header.FrameSequenceNum = 9

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tcdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatalf("re-encoded frame does not decode: %v", err)
	}
	if decoded.Header.FrameSequenceNum != 9 {
		t.Errorf("FrameSequenceNum = %d, want 9", decoded.Header.FrameSequenceNum)
	}
}
