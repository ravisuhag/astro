package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

// --- Virtual Channel Tests ---

func TestVirtualChannel_AddGetFrame(t *testing.T) {
	vc := tcdl.NewVirtualChannel(5, 10)
	frame, _ := tcdl.NewTCTransferFrame(42, 5, []byte("data"))
	if err := vc.Add(frame); err != nil {
		t.Fatal(err)
	}
	if vc.Len() != 1 {
		t.Errorf("Len = %d, want 1", vc.Len())
	}
	got, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.DataField, []byte("data")) {
		t.Error("got different frame")
	}
}

func TestVirtualChannel_BufferFull(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 1)
	frame, _ := tcdl.NewTCTransferFrame(42, 1, []byte("a"))
	_ = vc.Add(frame)
	err := vc.Add(frame)
	if !errors.Is(err, tcdl.ErrBufferFull) {
		t.Errorf("expected ErrBufferFull, got %v", err)
	}
}

// --- Frame Gap Detector Tests ---

func TestFrameGapDetector_NoGap(t *testing.T) {
	d := tcdl.NewFrameGapDetector()
	f0, _ := tcdl.NewTCTransferFrame(42, 1, []byte("a"), tcdl.WithSequenceNumber(0))
	f1, _ := tcdl.NewTCTransferFrame(42, 1, []byte("b"), tcdl.WithSequenceNumber(1))

	gap := d.Track(f0)
	if gap != 0 {
		t.Errorf("first frame gap = %d, want 0", gap)
	}
	gap = d.Track(f1)
	if gap != 0 {
		t.Errorf("sequential frame gap = %d, want 0", gap)
	}
}

func TestFrameGapDetector_Gap(t *testing.T) {
	d := tcdl.NewFrameGapDetector()
	f0, _ := tcdl.NewTCTransferFrame(42, 1, []byte("a"), tcdl.WithSequenceNumber(0))
	f3, _ := tcdl.NewTCTransferFrame(42, 1, []byte("b"), tcdl.WithSequenceNumber(3))

	d.Track(f0)
	gap := d.Track(f3)
	if gap != 2 {
		t.Errorf("gap = %d, want 2", gap)
	}
}

// --- Master Channel Tests ---

func TestMasterChannel_Routing(t *testing.T) {
	mc := tcdl.NewMasterChannel(42)
	vc1 := tcdl.NewVirtualChannel(1, 10)
	vc2 := tcdl.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	f1, _ := tcdl.NewTCTransferFrame(42, 1, []byte("to-vc1"))
	f2, _ := tcdl.NewTCTransferFrame(42, 2, []byte("to-vc2"))
	_ = mc.AddFrame(f1)
	_ = mc.AddFrame(f2)

	got1, _ := vc1.Next()
	if !bytes.Equal(got1.DataField, []byte("to-vc1")) {
		t.Errorf("VC1 got %q", got1.DataField)
	}
	got2, _ := vc2.Next()
	if !bytes.Equal(got2.DataField, []byte("to-vc2")) {
		t.Errorf("VC2 got %q", got2.DataField)
	}
}

func TestMasterChannel_SCIDMismatch(t *testing.T) {
	mc := tcdl.NewMasterChannel(42)
	vc := tcdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	frame, _ := tcdl.NewTCTransferFrame(999, 1, []byte("wrong"))
	if !errors.Is(mc.AddFrame(frame), tcdl.ErrSCIDMismatch) {
		t.Error("expected ErrSCIDMismatch")
	}
}
