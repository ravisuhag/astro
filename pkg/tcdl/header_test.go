package tcdl_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

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
