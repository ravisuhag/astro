package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// --- Virtual Channel Tests ---

func TestNewVirtualChannel(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 10)

	if vc.ID != 0x01 {
		t.Errorf("Expected VCID 0x01, got %v", vc.ID)
	}

	if vc.Len() != 0 {
		t.Errorf("Expected Len 0, got %v", vc.Len())
	}

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}
}

func TestAddFrame(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	frame := &tmdl.TMTransferFrame{}
	if err := vc.Add(frame); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if vc.Len() != 1 {
		t.Errorf("Expected Len 1, got %v", vc.Len())
	}

	if err := vc.Add(frame); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err := vc.Add(frame)
	if !errors.Is(err, tmdl.ErrBufferFull) {
		t.Fatalf("Expected ErrBufferFull, got %v", err)
	}

	if vc.Len() != 2 {
		t.Errorf("Expected Len 2, got %v", vc.Len())
	}
}

func TestGetNextFrame(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}

	if err := vc.Add(frame1); err != nil {
		t.Fatalf("Failed to add frame1: %v", err)
	}
	if err := vc.Add(frame2); err != nil {
		t.Fatalf("Failed to add frame2: %v", err)
	}

	retrieved, err := vc.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved != frame1 {
		t.Errorf("Expected frame1, got %v", retrieved)
	}

	retrieved, err = vc.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved != frame2 {
		t.Errorf("Expected frame2, got %v", retrieved)
	}

	_, err = vc.Next()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Fatalf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestHasFrames(t *testing.T) {
	vc := tmdl.NewVirtualChannel(0x01, 2)

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}

	if err := vc.Add(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatalf("Failed to add frame: %v", err)
	}

	if !vc.HasFrames() {
		t.Error("Expected HasFrames to be true")
	}

	if _, err := vc.Next(); err != nil {
		t.Fatalf("Failed to retrieve frame: %v", err)
	}

	if vc.HasFrames() {
		t.Error("Expected HasFrames to be false")
	}
}

// --- Multiplexer Tests ---

func TestNewMultiplexer(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	if mux.Len() != 0 {
		t.Errorf("Expected Len 0, got %v", mux.Len())
	}

	if mux.HasPending() {
		t.Error("Expected HasPendingFrames to be false")
	}
}

func TestMultiplexerAddVirtualChannel(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vc := tmdl.NewVirtualChannel(0x01, 10)
	mux.AddChannel(vc, 1)

	if mux.Len() != 1 {
		t.Errorf("Expected Len 1, got %v", mux.Len())
	}
}

func TestMultiplexerGetNextFrame(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	vc1 := tmdl.NewVirtualChannel(0x01, 10)
	vc2 := tmdl.NewVirtualChannel(0x02, 10)
	mux.AddChannel(vc1, 1)
	mux.AddChannel(vc2, 1)

	frame1 := &tmdl.TMTransferFrame{}
	frame2 := &tmdl.TMTransferFrame{}
	if err := vc1.Add(frame1); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if err := vc2.Add(frame2); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// With sorted VCIDs and priority 1, should get VC1 first, then VC2
	retrieved1, err := mux.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved1 != frame1 {
		t.Errorf("Expected frame1 first (lower VCID has priority)")
	}

	retrieved2, err := mux.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved2 != frame2 {
		t.Errorf("Expected frame2 second")
	}

	_, err = mux.Next()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Fatalf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestMultiplexerPriorityWeighting(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	vc1 := tmdl.NewVirtualChannel(0x01, 10)
	vc2 := tmdl.NewVirtualChannel(0x02, 10)
	mux.AddChannel(vc1, 2) // Priority 2: gets 2 turns
	mux.AddChannel(vc2, 1) // Priority 1: gets 1 turn

	// Add 3 frames to vc1 and 3 to vc2
	for i := range 3 {
		if err := vc1.Add(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: uint8(i)}}); err != nil {
			t.Fatalf("Failed to add frame to vc1: %v", err)
		}
		if err := vc2.Add(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: uint8(10 + i)}}); err != nil {
			t.Fatalf("Failed to add frame to vc2: %v", err)
		}
	}

	// Expected order: vc1, vc1 (weight 2), vc2 (weight 1), vc1, vc1 (weight 2), vc2 (weight 1)
	expected := []uint8{0, 1, 10, 2, 11, 12}
	for i, exp := range expected {
		frame, err := mux.Next()
		if err != nil {
			t.Fatalf("Frame %d: unexpected error: %v", i, err)
		}
		if frame.Header.MCFrameCount != exp {
			t.Errorf("Frame %d: expected MCFrameCount %d, got %d", i, exp, frame.Header.MCFrameCount)
		}
	}
}

func TestMultiplexerHasPendingFrames(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	vc := tmdl.NewVirtualChannel(0x01, 10)
	mux.AddChannel(vc, 1)

	if mux.HasPending() {
		t.Error("Expected HasPendingFrames to be false")
	}

	if err := vc.Add(&tmdl.TMTransferFrame{}); err != nil {
		t.Fatalf("Failed to add frame: %v", err)
	}

	if !mux.HasPending() {
		t.Error("Expected HasPendingFrames to be true")
	}

	if _, err := mux.Next(); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if mux.HasPending() {
		t.Error("Expected HasPendingFrames to be false")
	}
}

func TestMultiplexerPriorityClamp(t *testing.T) {
	mux := tmdl.NewMultiplexer()

	vc1 := tmdl.NewVirtualChannel(0x01, 10)
	vc2 := tmdl.NewVirtualChannel(0x02, 10)
	mux.AddChannel(vc1, 0)  // should be clamped to 1
	mux.AddChannel(vc2, -5) // should be clamped to 1

	if err := vc1.Add(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := vc1.Add(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := vc2.Add(&tmdl.TMTransferFrame{Header: tmdl.PrimaryHeader{MCFrameCount: 10}}); err != nil {
		t.Fatal(err)
	}

	// With clamped priority=1, should alternate: vc1, vc2, vc1
	expected := []uint8{1, 10, 2}
	for i, exp := range expected {
		frame, err := mux.Next()
		if err != nil {
			t.Fatalf("Frame %d: %v", i, err)
		}
		if frame.Header.MCFrameCount != exp {
			t.Errorf("Frame %d: MCFrameCount = %d, want %d", i, frame.Header.MCFrameCount, exp)
		}
	}
}

func TestMultiplexerNoChannels(t *testing.T) {
	mux := tmdl.NewMultiplexer()
	_, err := mux.Next()
	if !errors.Is(err, tmdl.ErrNoVirtualChannels) {
		t.Errorf("Expected ErrNoVirtualChannels, got %v", err)
	}
}

// --- Service Manager Tests ---

func TestTMServiceManager_VCPService(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	mgr.RegisterVirtualService(1, tmdl.VCP, svc)

	data := []byte("packet data")
	if err := mgr.SendData(1, tmdl.VCP, data); err != nil {
		t.Fatalf("SendData failed: %v", err)
	}

	received, err := mgr.ReceiveData(1, tmdl.VCP)
	if err != nil {
		t.Fatalf("ReceiveData failed: %v", err)
	}

	if !bytes.Equal(data, received) {
		t.Errorf("Expected %s, got %s", data, received)
	}
}

func TestTMServiceManager_VCAService(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()
	vc := tmdl.NewVirtualChannel(2, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 2, 8, vc, tmdl.ChannelConfig{}, nil)
	mgr.RegisterVirtualService(2, tmdl.VCA, svc)

	data := []byte("12345678")
	if err := mgr.SendData(2, tmdl.VCA, data); err != nil {
		t.Fatalf("SendData failed: %v", err)
	}

	received, err := mgr.ReceiveData(2, tmdl.VCA)
	if err != nil {
		t.Fatalf("ReceiveData failed: %v", err)
	}

	if !bytes.Equal(data, received) {
		t.Errorf("Expected %s, got %s", data, received)
	}
}

func TestTMServiceManager_VCFService(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()
	vc := tmdl.NewVirtualChannel(3, 100)
	svc := tmdl.NewVirtualChannelFrameService(3, vc)
	mgr.RegisterVirtualService(3, tmdl.VCF, svc)

	frame, err := tmdl.NewTMTransferFrame(933, 3, []byte("frame data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if err := mgr.SendData(3, tmdl.VCF, encoded); err != nil {
		t.Fatalf("SendData failed: %v", err)
	}

	received, err := mgr.ReceiveData(3, tmdl.VCF)
	if err != nil {
		t.Fatalf("ReceiveData failed: %v", err)
	}

	if len(received) == 0 {
		t.Error("Expected non-empty received data")
	}
}

func TestTMServiceManager_UnregisteredService(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()

	err := mgr.SendData(99, tmdl.VCP, []byte("data"))
	if !errors.Is(err, tmdl.ErrServiceNotFound) {
		t.Errorf("Expected ErrServiceNotFound, got %v", err)
	}

	_, err = mgr.ReceiveData(99, tmdl.VCP)
	if !errors.Is(err, tmdl.ErrServiceNotFound) {
		t.Errorf("Expected ErrServiceNotFound, got %v", err)
	}
}

func TestTMServiceManager_MasterChannel(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 100)
	mc.AddVirtualChannel(vc, 1)
	mgr.RegisterMasterChannel(933, mc)

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("mc data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	if err := mgr.AddFrameToMasterChannel(933, frame); err != nil {
		t.Fatalf("AddFrameToMasterChannel failed: %v", err)
	}

	if !mgr.HasPendingFramesInMasterChannel(933) {
		t.Error("Expected pending frames")
	}

	got, err := mgr.GetNextFrameFromMasterChannel(933)
	if err != nil {
		t.Fatalf("GetNextFrameFromMasterChannel failed: %v", err)
	}

	if got != frame {
		t.Error("Expected same frame back")
	}

	if mgr.HasPendingFramesInMasterChannel(933) {
		t.Error("Expected no pending frames after draining")
	}
}

func TestTMServiceManager_UnregisteredMasterChannel(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	err = mgr.AddFrameToMasterChannel(999, frame)
	if !errors.Is(err, tmdl.ErrMasterChannelNotFound) {
		t.Errorf("Expected ErrMasterChannelNotFound, got %v", err)
	}

	_, err = mgr.GetNextFrameFromMasterChannel(999)
	if !errors.Is(err, tmdl.ErrMasterChannelNotFound) {
		t.Errorf("Expected ErrMasterChannelNotFound, got %v", err)
	}

	if mgr.HasPendingFramesInMasterChannel(999) {
		t.Error("Expected false for unregistered master channel")
	}
}

func TestTMServiceManager_FullPipeline(t *testing.T) {
	mgr := tmdl.NewTMServiceManager()
	counter := tmdl.NewFrameCounter()

	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc1 := tmdl.NewVirtualChannel(1, 100)
	vc2 := tmdl.NewVirtualChannel(2, 100)
	mc.AddVirtualChannel(vc1, 2)
	mc.AddVirtualChannel(vc2, 1)
	mgr.RegisterMasterChannel(933, mc)

	svc1 := tmdl.NewVirtualChannelPacketService(933, 1, vc1, tmdl.ChannelConfig{}, counter)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 2, vc2, tmdl.ChannelConfig{}, counter)
	mgr.RegisterVirtualService(1, tmdl.VCP, svc1)
	mgr.RegisterVirtualService(2, tmdl.VCP, svc2)

	if err := mgr.SendData(1, tmdl.VCP, []byte("priority")); err != nil {
		t.Fatalf("SendData vc1: %v", err)
	}
	if err := mgr.SendData(2, tmdl.VCP, []byte("normal")); err != nil {
		t.Fatalf("SendData vc2: %v", err)
	}

	f1, err := mgr.GetNextFrameFromMasterChannel(933)
	if err != nil {
		t.Fatalf("GetNextFrame 1: %v", err)
	}
	if string(f1.DataField) != "priority" {
		t.Errorf("Expected 'priority' first (higher weight), got %q", f1.DataField)
	}

	f2, err := mgr.GetNextFrameFromMasterChannel(933)
	if err != nil {
		t.Fatalf("GetNextFrame 2: %v", err)
	}
	if string(f2.DataField) != "normal" {
		t.Errorf("Expected 'normal' second, got %q", f2.DataField)
	}

	if mgr.HasPendingFramesInMasterChannel(933) {
		t.Error("Expected no pending frames")
	}
}

// --- Frame Gap Detector Tests ---

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

// --- Master Channel Tests ---

func TestMasterChannel_AddAndGet(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	_ = mc.AddFrame(frame)

	got, _ := mc.GetNextFrame()
	if got != frame {
		t.Error("Expected same frame back")
	}
}

func TestMasterChannel_SCIDMismatch(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)
	frame, _ := tmdl.NewTMTransferFrame(500, 1, []byte("data"), nil, nil)
	if !errors.Is(mc.AddFrame(frame), tmdl.ErrSCIDMismatch) {
		t.Error("Expected ErrSCIDMismatch")
	}
}

func TestMasterChannel_VCIDNotFound(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if !errors.Is(mc.AddFrame(frame), tmdl.ErrVirtualChannelNotFound) {
		t.Error("Expected ErrVirtualChannelNotFound")
	}
}

func TestMasterChannel_GetNextFrameOrIdle(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := mc.GetNextFrameOrIdle()
	if !tmdl.IsIdleFrame(frame) {
		t.Error("Expected idle when empty")
	}

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)
	_ = svc.Send(makeChannelTestPacket([]byte{0x01}))
	_ = svc.Flush()

	frame, _ = mc.GetNextFrameOrIdle()
	if tmdl.IsIdleFrame(frame) {
		t.Error("Expected data frame")
	}
}

func TestMasterChannel_MultiplexesSendPath(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc1 := tmdl.NewVirtualChannel(1, 10)
	vc2 := tmdl.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(933, 1, vc1, tmdl.ChannelConfig{}, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 2, vc2, tmdl.ChannelConfig{}, nil)
	_ = svc1.Send([]byte("from vc1"))
	_ = svc2.Send([]byte("from vc2"))

	f1, _ := mc.GetNextFrame()
	if string(f1.DataField) != "from vc1" {
		t.Errorf("Expected 'from vc1', got %q", f1.DataField)
	}
	f2, _ := mc.GetNextFrame()
	if string(f2.DataField) != "from vc2" {
		t.Errorf("Expected 'from vc2', got %q", f2.DataField)
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

// makeChannelTestPacket builds a minimal encoded CCSDS Space Packet with the given payload.
func makeChannelTestPacket(payload []byte) []byte {
	pkt, err := spp.NewTMPacket(1, payload)
	if err != nil {
		panic("makeChannelTestPacket: " + err.Error())
	}
	data, err := pkt.Encode()
	if err != nil {
		panic("makeChannelTestPacket encode: " + err.Error())
	}
	return data
}
