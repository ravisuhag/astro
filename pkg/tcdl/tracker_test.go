package tcdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

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
