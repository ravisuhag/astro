package tmdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestFrameGapDetector_NoGap(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	frames := []*tmdl.TMTransferFrame{
		{Header: tmdl.PrimaryHeader{MCFrameCount: 0, VCFrameCount: 0, VirtualChannelID: 1, SegmentLengthID: 0b11}},
		{Header: tmdl.PrimaryHeader{MCFrameCount: 1, VCFrameCount: 1, VirtualChannelID: 1, SegmentLengthID: 0b11}},
		{Header: tmdl.PrimaryHeader{MCFrameCount: 2, VCFrameCount: 2, VirtualChannelID: 1, SegmentLengthID: 0b11}},
	}

	for i, f := range frames {
		mcGap, vcGap := d.Track(f)
		if mcGap != 0 {
			t.Errorf("Frame %d: MCGap = %d, want 0", i, mcGap)
		}
		if vcGap != 0 {
			t.Errorf("Frame %d: VCGap = %d, want 0", i, vcGap)
		}
	}
}

func TestFrameGapDetector_MCGap(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	// Frame 0: MC=0
	d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 0, VCFrameCount: 0, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	// Frame 1: MC=3 (skipped MC=1,2 → gap of 2)
	mcGap, _ := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 3, VCFrameCount: 1, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	if mcGap != 2 {
		t.Errorf("MCGap = %d, want 2", mcGap)
	}
	if d.MCFrameGap() != 2 {
		t.Errorf("MCFrameGap() = %d, want 2", d.MCFrameGap())
	}
}

func TestFrameGapDetector_VCGap(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 0, VCFrameCount: 0, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	// VC=5 (skipped VC=1,2,3,4 → gap of 4)
	_, vcGap := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 1, VCFrameCount: 5, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	if vcGap != 4 {
		t.Errorf("VCGap = %d, want 4", vcGap)
	}
}

func TestFrameGapDetector_Wraparound(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	// MC=254
	d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 254, VCFrameCount: 254, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	// MC=255: no gap
	mcGap, _ := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 255, VCFrameCount: 255, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})
	if mcGap != 0 {
		t.Errorf("At 255: MCGap = %d, want 0", mcGap)
	}

	// MC=0: wraps, no gap
	mcGap, vcGap := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 0, VCFrameCount: 0, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})
	if mcGap != 0 {
		t.Errorf("At wrap to 0: MCGap = %d, want 0", mcGap)
	}
	if vcGap != 0 {
		t.Errorf("At wrap to 0: VCGap = %d, want 0", vcGap)
	}

	// MC=3: gap of 2 after wrap (skipped 1,2)
	mcGap, _ = d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 3, VCFrameCount: 1, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})
	if mcGap != 2 {
		t.Errorf("After wrap gap: MCGap = %d, want 2", mcGap)
	}
}

func TestFrameGapDetector_MultipleVCIDs(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	// VC1: count 0
	d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 0, VCFrameCount: 0, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})

	// VC2: count 0 (first for this VCID, no gap)
	_, vcGap := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 1, VCFrameCount: 0, VirtualChannelID: 2, SegmentLengthID: 0b11,
	}})
	if vcGap != 0 {
		t.Errorf("First VC2 frame: VCGap = %d, want 0", vcGap)
	}

	// VC1: count 1 (sequential, no gap)
	_, vcGap = d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 2, VCFrameCount: 1, VirtualChannelID: 1, SegmentLengthID: 0b11,
	}})
	if vcGap != 0 {
		t.Errorf("Sequential VC1: VCGap = %d, want 0", vcGap)
	}

	// VC2: count 3 (skipped 1,2 → gap of 2)
	_, vcGap = d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 3, VCFrameCount: 3, VirtualChannelID: 2, SegmentLengthID: 0b11,
	}})
	if vcGap != 2 {
		t.Errorf("VC2 gap: VCGap = %d, want 2", vcGap)
	}
}

func TestFrameGapDetector_FirstFrame(t *testing.T) {
	d := tmdl.NewFrameGapDetector()

	// First frame should never report a gap regardless of count values
	mcGap, vcGap := d.Track(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{
		MCFrameCount: 42, VCFrameCount: 99, VirtualChannelID: 3, SegmentLengthID: 0b11,
	}})

	if mcGap != 0 {
		t.Errorf("First frame: MCGap = %d, want 0", mcGap)
	}
	if vcGap != 0 {
		t.Errorf("First frame: VCGap = %d, want 0", vcGap)
	}
}

func TestMasterChannel_FrameGapDetection(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)

	// Frame 1: MC=0, VC=0
	f1, _ := tmdl.NewTMTransferFrame(933, 1, []byte("a"), nil, nil)
	f1.Header.MCFrameCount = 0
	f1.Header.VCFrameCount = 0
	if err := mc.AddFrame(f1); err != nil {
		t.Fatal(err)
	}
	if mc.MCFrameGap() != 0 {
		t.Errorf("Frame 1: MCFrameGap = %d, want 0", mc.MCFrameGap())
	}
	if mc.VCFrameGap() != 0 {
		t.Errorf("Frame 1: VCFrameGap = %d, want 0", mc.VCFrameGap())
	}

	// Frame 2: MC=3, VC=2 (MC gap of 2, VC gap of 1)
	f2, _ := tmdl.NewTMTransferFrame(933, 1, []byte("b"), nil, nil)
	f2.Header.MCFrameCount = 3
	f2.Header.VCFrameCount = 2
	if err := mc.AddFrame(f2); err != nil {
		t.Fatal(err)
	}
	if mc.MCFrameGap() != 2 {
		t.Errorf("Frame 2: MCFrameGap = %d, want 2", mc.MCFrameGap())
	}
	if mc.VCFrameGap() != 1 {
		t.Errorf("Frame 2: VCFrameGap = %d, want 1", mc.VCFrameGap())
	}

	// Frame 3: MC=4, VC=3 (no gaps)
	f3, _ := tmdl.NewTMTransferFrame(933, 1, []byte("c"), nil, nil)
	f3.Header.MCFrameCount = 4
	f3.Header.VCFrameCount = 3
	if err := mc.AddFrame(f3); err != nil {
		t.Fatal(err)
	}
	if mc.MCFrameGap() != 0 {
		t.Errorf("Frame 3: MCFrameGap = %d, want 0", mc.MCFrameGap())
	}
	if mc.VCFrameGap() != 0 {
		t.Errorf("Frame 3: VCFrameGap = %d, want 0", mc.VCFrameGap())
	}
}
