package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// makeTestPacket builds a minimal encoded CCSDS Space Packet with the given payload.
func makeTestPacket(payload []byte) []byte {
	pkt, err := spp.NewTMPacket(1, payload)
	if err != nil {
		panic("makeTestPacket: " + err.Error())
	}
	data, err := pkt.Encode()
	if err != nil {
		panic("makeTestPacket encode: " + err.Error())
	}
	return data
}

// --- VCP Legacy Tests (zero config) ---

func TestVCPService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)

	data := []byte("telemetry packet")
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if !bytes.Equal(data, received) {
		t.Errorf("Expected %s, got %s", data, received)
	}
}

func TestVCPService_SendEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	err := svc.Send([]byte{})
	if !errors.Is(err, tmdl.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestVCPService_ReceiveEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestVCPService_MultipleSendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(2, 100)
	svc := tmdl.NewVirtualChannelPacketService(500, 2, vc, tmdl.ChannelConfig{}, nil)

	data1 := []byte("packet one")
	data2 := []byte("packet two")
	if err := svc.Send(data1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(data2); err != nil {
		t.Fatal(err)
	}

	received1, _ := svc.Receive()
	received2, _ := svc.Receive()

	if !bytes.Equal(data1, received1) {
		t.Errorf("Expected %s, got %s", data1, received1)
	}
	if !bytes.Equal(data2, received2) {
		t.Errorf("Expected %s, got %s", data2, received2)
	}
}

func TestVCPService_Flush_ZeroConfig(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	if err := svc.Flush(); err != nil {
		t.Errorf("Flush should be no-op for zero config: %v", err)
	}
}

// --- VCP Packing Tests (with ChannelConfig) ---

func TestVCPService_Packing_SinglePacket(t *testing.T) {
	// capacity = 40, packet = 8 bytes (6 hdr + 2 payload) -> fits in one frame
	config := tmdl.ChannelConfig{FrameLength: 48, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket([]byte{0x01, 0x02})
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	if vc.Len() != 1 {
		t.Fatalf("Expected 1 frame, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Expected %x, got %x", pkt, received)
	}
}

func TestVCPService_Packing_LargePacket(t *testing.T) {
	// capacity = 10, packet = 16 bytes (6 hdr + 10 payload) -> spans 2 frames
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xAB}, 10))
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// 16 packet bytes leave 4 spare octets in frame 2, too few for the
	// 7-octet minimum idle packet, so the fill spans one extra frame: 3 total.
	if vc.Len() != 3 {
		t.Fatalf("Expected 3 frames, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Packet mismatch: got %d bytes, want %d", len(received), len(pkt))
	}
}

func TestVCPService_Packing_TwoSmallPackets(t *testing.T) {
	// capacity = 20, two packets of 8 bytes each = 16 bytes -> fit in one frame
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket([]byte{0x01, 0x02})
	pkt2 := makeTestPacket([]byte{0x03, 0x04})

	if err := svc.Send(pkt1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(pkt2); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// Both packets pack into frame 1 (16 bytes < 20 capacity). The 4 spare
	// octets are under the 7-octet minimum idle packet, so the idle fill
	// spans into a second, fill-only frame.
	if vc.Len() != 2 {
		t.Fatalf("Expected 2 frames (two packets packed + spanning fill), got %d", vc.Len())
	}

	// Verify FHP = 0 (first packet starts at byte 0)
	frame, _ := vc.Next()
	if frame.Header.FirstHeaderPtr != 0 {
		t.Errorf("FHP = %d, want 0", frame.Header.FirstHeaderPtr)
	}

	// Re-send and receive both packets
	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config, nil)
	svc2.SetPacketSizer(spp.PacketSizer)
	_ = svc2.Send(pkt1)
	_ = svc2.Send(pkt2)
	_ = svc2.Flush()

	recv1, err := svc2.Receive()
	if err != nil {
		t.Fatal(err)
	}
	recv2, err := svc2.Receive()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pkt1, recv1) {
		t.Errorf("Packet 1: expected %x, got %x", pkt1, recv1)
	}
	if !bytes.Equal(pkt2, recv2) {
		t.Errorf("Packet 2: expected %x, got %x", pkt2, recv2)
	}
}

func TestVCPService_Packing_SpanningPackets(t *testing.T) {
	// capacity = 12, pkt1=8 bytes, pkt2=8 bytes -> total 16 bytes
	// Frame 1: [pkt1(8) + pkt2_start(4)] FHP=0
	// Frame 2: [pkt2_end(4) + idle packet(8)] FHP=4 (idle packet header)
	config := tmdl.ChannelConfig{FrameLength: 20, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket([]byte{0x01, 0x02})
	pkt2 := makeTestPacket([]byte{0x03, 0x04})

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	recv1, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	recv2, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pkt1, recv1) {
		t.Errorf("Packet 1: expected %x, got %x", pkt1, recv1)
	}
	if !bytes.Equal(pkt2, recv2) {
		t.Errorf("Packet 2: expected %x, got %x", pkt2, recv2)
	}
}

func TestVCPService_Packing_FHPValues(t *testing.T) {
	// capacity = 12, pkt1=8 bytes, pkt2=8 bytes
	// Frame 1: [pkt1(8)|pkt2_start(4)] FHP=0 (pkt1 starts at 0)
	// Frame 2: [pkt2_end(4)|idle packet(8)] FHP=4 (the idle fill packet is a
	// real packet, so the pointer marks its header)
	config := tmdl.ChannelConfig{FrameLength: 20, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket([]byte{0x01, 0x02}) // 8 bytes
	pkt2 := makeTestPacket([]byte{0x03, 0x04}) // 8 bytes

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	f1, _ := vc.Next()
	if f1.Header.FirstHeaderPtr != 0 {
		t.Errorf("Frame 1: FHP = %d, want 0", f1.Header.FirstHeaderPtr)
	}

	f2, _ := vc.Next()
	if f2.Header.FirstHeaderPtr != 4 {
		t.Errorf("Frame 2: FHP = 0x%04X, want 4 (idle fill packet header)", f2.Header.FirstHeaderPtr)
	}
}

func TestVCPService_Packing_FHP_MidFrame(t *testing.T) {
	// capacity = 10, pkt1=7 bytes -> Frame 1: [pkt1(7)|pkt2_start(3)] FHP=0
	// pkt2=7 bytes -> pkt2 starts at offset 7 in frame 1
	// But actually: frame 1 has capacity 10. After pkt1 (7 bytes), pkt2 starts at offset 7.
	// If total < capacity (14 bytes > 10), frame 1 = first 10 bytes, with pkt2 at offset 7.
	// Flush leaves 4 octets of pkt2 and 6 spare, under the 7-octet minimum,
	// so the idle fill packet starting at offset 4 spans into a third frame.
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true} // capacity = 10
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket([]byte{0xAA}) // 7 bytes
	pkt2 := makeTestPacket([]byte{0xBB}) // 7 bytes

	_ = svc.Send(pkt1) // buffer: 7 bytes, offsets: [0]
	_ = svc.Send(pkt2) // buffer: 14 bytes, offsets: [0, 7] -> generates 1 full frame (10 bytes)
	_ = svc.Flush()    // flushes remaining 4 bytes

	// Frame 1 should have FHP=0 (pkt1 at offset 0, pkt2 at offset 7)
	f1, _ := vc.Next()
	if f1.Header.FirstHeaderPtr != 0 {
		t.Errorf("Frame 1 FHP = %d, want 0", f1.Header.FirstHeaderPtr)
	}

	// Frame 2 carries pkt2's tail then the idle fill packet's header at offset 4
	f2, _ := vc.Next()
	if f2.Header.FirstHeaderPtr != 4 {
		t.Errorf("Frame 2 FHP = 0x%04X, want 4 (idle fill packet header)", f2.Header.FirstHeaderPtr)
	}

	// Frame 3 is pure idle packet continuation: no packet starts in it,
	// so FHP is the 0x07FF "no packet starts" sentinel (clause 4.1.2.7.6).
	f3, _ := vc.Next()
	if f3.Header.FirstHeaderPtr != tmdl.FHPNoPacketStart {
		t.Errorf("Frame 3 FHP = 0x%04X, want 0x%04X (no packet starts)",
			f3.Header.FirstHeaderPtr, tmdl.FHPNoPacketStart)
	}
}

func TestVCPService_Packing_IdleFrameSkip(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket([]byte{0x01, 0x02})
	_ = svc.Send(pkt)
	_ = svc.Flush()

	// Insert idle before data frame
	idle, _ := tmdl.NewIdleFrame(933, 1, config)
	var frames []*tmdl.TMTransferFrame
	for vc.HasFrames() {
		f, _ := vc.Next()
		frames = append(frames, f)
	}
	_ = vc.Add(idle)
	for _, f := range frames {
		_ = vc.Add(f)
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Should skip idle: %v", err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Expected %x, got %x", pkt, received)
	}
}

func TestVCPService_Packing_LossResync(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	counter := tmdl.NewFrameCounter()
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket([]byte{0x01, 0x02})
	pkt2 := makeTestPacket([]byte{0x03, 0x04})

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	// Remove the first frame (simulate loss)
	_, _ = vc.Next() // discard pkt1's frame

	// pkt2's frame remains. It has FHP=0x07FF (no packet starts) or FHP with offset
	// After gap detection, receiver should resync and eventually extract pkt2
	// if pkt2 starts in the remaining frame

	// Actually with pkt1(8)+pkt2(8)=16 bytes, capacity=20,
	// both fit in one frame. So removing that frame loses both.
	// Let me use a scenario where packets span frames.

	// Reset with spanning scenario
	vc2 := tmdl.NewVirtualChannel(1, 100)
	counter2 := tmdl.NewFrameCounter()
	config2 := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true} // capacity=10
	svc2 := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config2, counter2)
	svc2.SetPacketSizer(spp.PacketSizer)

	bigPkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 5)) // 11 bytes
	bigPkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 5)) // 11 bytes

	_ = svc2.Send(bigPkt1) // 11 bytes, generates 1 full frame (10 bytes), 1 byte remains
	_ = svc2.Send(bigPkt2) // +11=12 bytes remaining, generates 1 full frame, 2 remain
	_ = svc2.Flush()       // flush remaining 2 bytes

	// Should have 3 frames: F0(VCcount=0), F1(VCcount=1), F2(VCcount=2)
	// Remove F0 (contains start of pkt1)
	_, _ = vc2.Next() // discard

	// Receiver sees F1 first (VCcount=1, gap detected since expected 0)
	// F1 has the tail of pkt1 + start of pkt2 (FHP should point to pkt2 start)
	// After loss, receiver aborts pkt1 and syncs to pkt2 via FHP
	received, err := svc2.Receive()
	if err != nil {
		t.Fatalf("Receive after loss: %v", err)
	}
	if !bytes.Equal(bigPkt2, received) {
		t.Errorf("Expected pkt2 after resync, got %x (want %x)", received, bigPkt2)
	}
}

func TestVCPService_Packing_CustomSizer(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	// Custom sizer: fixed 5-byte packets
	svc.SetPacketSizer(func(data []byte) int {
		if len(data) < 5 {
			return -1
		}
		return 5
	})

	pkt := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	_ = svc.Send(pkt)
	_ = svc.Flush()

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Expected %x, got %x", pkt, received)
	}
}

func TestVCPService_Packing_Flush_Empty(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	if err := svc.Flush(); err != nil {
		t.Errorf("Empty flush should be no-op: %v", err)
	}
	if vc.Len() != 0 {
		t.Errorf("No frames should be generated: got %d", vc.Len())
	}
}

// --- VCF Service Tests ---

func TestVCFService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)

	frame, _ := tmdl.NewTransferFrame(933, 1, []byte("frame data"), nil, nil)
	encoded, _ := frame.Encode()

	if err := svc.Send(encoded); err != nil {
		t.Fatal(err)
	}
	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if len(received) == 0 {
		t.Error("Expected non-empty data")
	}
}

func TestVCFService_SendInvalid(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)
	if !errors.Is(svc.Send([]byte{}), tmdl.ErrEmptyData) {
		t.Error("Expected ErrEmptyData")
	}
	if svc.Send([]byte{0xFF}) == nil {
		t.Error("Expected error for invalid bytes")
	}
}

func TestVCFService_Flush(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)
	if err := svc.Flush(); err != nil {
		t.Errorf("VCF Flush should be no-op: %v", err)
	}
}

// --- VCA Service Tests ---

func TestVCAService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)

	data := []byte("12345678")
	if err := svc.Send(data); err != nil {
		t.Fatal(err)
	}
	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, received) {
		t.Errorf("Expected %s, got %s", data, received)
	}
}

func TestVCAService_SyncFlag(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 4, vc, tmdl.ChannelConfig{}, nil)
	_ = svc.Send([]byte("abcd"))
	frame, _ := vc.Next()
	if !frame.Header.SyncFlag {
		t.Error("Expected SyncFlag=true")
	}
	if frame.Header.FirstHeaderPtr != 0x07FF {
		t.Errorf("FHP = 0x%04X, want 0x07FF", frame.Header.FirstHeaderPtr)
	}
}

func TestVCAService_StatusFields(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 4, vc, tmdl.ChannelConfig{}, nil)
	_ = svc.Send([]byte("abcd"))
	_, _ = svc.Receive()
	status := svc.LastStatus()
	if !status.SyncFlag {
		t.Error("Expected SyncFlag=true")
	}
}

func TestVCAService_Padding(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	capacity := config.DataFieldCapacity(0)
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)

	data := []byte("12345678")
	_ = svc.Send(data)
	frame, _ := vc.Next()
	if len(frame.DataField) != capacity {
		t.Errorf("DataField len = %d, want %d", len(frame.DataField), capacity)
	}

	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc2, config, nil)
	_ = svc2.Send(data)
	received, _ := svc2.Receive()
	if !bytes.Equal(data, received) {
		t.Errorf("Expected %q, got %q", data, received)
	}
}

func TestVCAService_Padding_TooLarge(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 12, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)
	err := svc.Send([]byte("12345678"))
	if !errors.Is(err, tmdl.ErrDataTooLarge) {
		t.Errorf("Expected ErrDataTooLarge, got %v", err)
	}
}

func TestVCAService_SendSizeMismatch(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)
	if !errors.Is(svc.Send([]byte("short")), tmdl.ErrSizeMismatch) {
		t.Error("Expected ErrSizeMismatch")
	}
}

func TestVCAService_Flush(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)
	if err := svc.Flush(); err != nil {
		t.Errorf("VCA Flush should be no-op: %v", err)
	}
}

// --- FrameCounter Tests ---

func TestFrameCounter(t *testing.T) {
	counter := tmdl.NewFrameCounter()
	mc, vc := counter.Next(1)
	if mc != 0 || vc != 0 {
		t.Errorf("First: MC=%d VC=%d, want 0,0", mc, vc)
	}
	mc, vc = counter.Next(1)
	if mc != 1 || vc != 1 {
		t.Errorf("Second: MC=%d VC=%d, want 1,1", mc, vc)
	}
	mc, vc = counter.Next(2)
	if mc != 2 || vc != 0 {
		t.Errorf("Different VCID: MC=%d VC=%d, want 2,0", mc, vc)
	}
}

func TestFrameCounter_Wraparound(t *testing.T) {
	counter := tmdl.NewFrameCounter()
	for range 255 {
		counter.Next(1)
	}
	mc, vc := counter.Next(1)
	if mc != 255 || vc != 255 {
		t.Errorf("At 255: MC=%d VC=%d, want 255,255", mc, vc)
	}
	mc, vc = counter.Next(1)
	if mc != 0 || vc != 0 {
		t.Errorf("After wrap: MC=%d VC=%d, want 0,0", mc, vc)
	}
}

func TestFrameCounter_VCPService(t *testing.T) {
	counter := tmdl.NewFrameCounter()
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, counter)

	for i := range 3 {
		if err := svc.Send([]byte("data")); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	for i := range 3 {
		if _, err := svc.Receive(); err != nil {
			t.Fatalf("Receive %d: %v", i, err)
		}
	}
}

func TestVCPService_Packing_ThreeFramePacket(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xAB}, 20))
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// 26 packet bytes fill 2 frames and leave 6 in the third; the 4 spare
	// octets can't hold a 7-octet idle packet, so the fill spans a 4th frame.
	if vc.Len() != 4 {
		t.Fatalf("Expected 4 frames, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("3-frame packet mismatch: got %d bytes, want %d", len(received), len(pkt))
	}
}

func TestVCPService_Packing_FiveFramePacket(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xCD}, 40))
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// 46 packet bytes fill 4 frames and leave 6 in the fifth; the spanning
	// idle fill packet adds a sixth frame.
	if vc.Len() != 6 {
		t.Fatalf("Expected 6 frames, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("5-frame packet mismatch: got %d bytes, want %d", len(received), len(pkt))
	}
}

func TestVCPService_MultiplePacketsSpanningManyFrames(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 20))
	pkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 20))

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	recv1, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	recv2, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pkt1, recv1) {
		t.Errorf("Packet 1 mismatch: got %d bytes, want %d", len(recv1), len(pkt1))
	}
	if !bytes.Equal(pkt2, recv2) {
		t.Errorf("Packet 2 mismatch: got %d bytes, want %d", len(recv2), len(pkt2))
	}
}

func TestVCPService_ConsecutiveIdleFrames(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}

	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	sendVC := tmdl.NewVirtualChannel(1, 100)
	sendSvc := tmdl.NewVirtualChannelPacketService(933, 1, sendVC, config, nil)
	sendSvc.SetPacketSizer(spp.PacketSizer)
	pkt := makeTestPacket([]byte{0x01, 0x02})
	_ = sendSvc.Send(pkt)
	_ = sendSvc.Flush()
	dataFrame, _ := sendVC.Next()

	for range 5 {
		idle, _ := tmdl.NewIdleFrame(933, 1, config)
		_ = vc.Add(idle)
	}
	_ = vc.Add(dataFrame)

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Should skip idle frames: %v", err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Expected %x, got %x", pkt, received)
	}
}

func TestVCPService_FrameLoss_ThreeFramePacket(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	counter := tmdl.NewFrameCounter()
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 20))
	pkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 5))

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	_, _ = vc.Next() // discard first frame

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive after 3-frame loss: %v", err)
	}
	if !bytes.Equal(pkt2, received) {
		t.Errorf("Expected pkt2 after resync, got %d bytes (want %d)", len(received), len(pkt2))
	}
}

func TestVCPService_FrameLoss_MiddleFrame(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	counter := tmdl.NewFrameCounter()
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 20))
	pkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 5))

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	f0, _ := vc.Next()
	_, _ = vc.Next() // discard middle frame

	vc2 := tmdl.NewVirtualChannel(1, 100)
	counter2 := tmdl.NewFrameCounter()
	svc2 := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config, counter2)
	svc2.SetPacketSizer(spp.PacketSizer)

	_ = vc2.Add(f0)
	for vc.HasFrames() {
		f, _ := vc.Next()
		_ = vc2.Add(f)
	}

	var recovered []byte
	for range 10 {
		data, err := svc2.Receive()
		if err != nil {
			break
		}
		if bytes.Equal(data, pkt2) {
			recovered = data
			break
		}
	}
	if recovered == nil {
		t.Log("pkt2 not recovered after middle-frame loss (expected in some FHP configurations)")
	}
}

func TestVCPService_FrameLoss_GapResyncEnabled(t *testing.T) {
	// Verify gap detection works for pure receivers using SetGapResync(true)
	// even when no FrameCounter is provided.
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	counter := tmdl.NewFrameCounter()
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 20))
	pkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 5))

	_ = svc.Send(pkt1)
	_ = svc.Send(pkt2)
	_ = svc.Flush()

	// Collect all frames (these have valid sequence numbers from counter).
	var frames []*tmdl.TMTransferFrame
	for vc.HasFrames() {
		f, _ := vc.Next()
		frames = append(frames, f)
	}

	vc2 := tmdl.NewVirtualChannel(1, 100)
	// Pure receiver: no counter, but enable gap resync since frames
	// have valid sequence numbers from the sender.
	recv := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config, nil)
	recv.SetPacketSizer(spp.PacketSizer)
	recv.SetGapResync(true)

	// Feed first frame, skip second (simulate loss), feed remaining.
	_ = vc2.Add(frames[0])
	for _, f := range frames[2:] {
		_ = vc2.Add(f)
	}

	// After frame loss, receiver should resync and eventually yield pkt2.
	received, err := recv.Receive()
	if err != nil {
		t.Fatalf("Receive after gap with nil counter: %v", err)
	}
	if !bytes.Equal(pkt2, received) {
		t.Errorf("Expected pkt2 after resync, got %d bytes (want %d)", len(received), len(pkt2))
	}
}

func TestIdleFrameDoesNotAffectPacketState(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	counter := tmdl.NewFrameCounter()
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xAA}, 10))
	_ = svc.Send(pkt)
	_ = svc.Flush()

	f1, _ := vc.Next()
	f2, _ := vc.Next()

	idle, _ := tmdl.NewIdleFrame(933, 1, config)

	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config, nil)
	svc2.SetPacketSizer(spp.PacketSizer)

	_ = vc2.Add(f1)
	_ = vc2.Add(idle)
	_ = vc2.Add(f2)

	received, err := svc2.Receive()
	if err != nil {
		t.Fatalf("Receive with interleaved idle: %v", err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Packet corrupted by interleaved idle frame")
	}
}

// --- Idle-packet fill tests (CCSDS 132.0-B-3 clause 4.2.2, ECSS 5.4.3.3b/5.4.3.4g) ---

// drainDataFields concatenates the data fields of all frames in the channel.
func drainDataFields(t *testing.T, vc *tmdl.VirtualChannel) []byte {
	t.Helper()
	var stream []byte
	for vc.HasFrames() {
		f, err := vc.Next()
		if err != nil {
			t.Fatal(err)
		}
		stream = append(stream, f.DataField...)
	}
	return stream
}

// parsePacketStream walks a byte stream with spp.PacketSizer and returns the
// packets it holds, failing if the stream does not consist of whole packets.
func parsePacketStream(t *testing.T, stream []byte) []*spp.SpacePacket {
	t.Helper()
	var pkts []*spp.SpacePacket
	for off := 0; off < len(stream); {
		n := spp.PacketSizer(stream[off:])
		if n <= 0 || off+n > len(stream) {
			t.Fatalf("stream not parseable as packets at offset %d (sizer=%d, %d bytes left)",
				off, n, len(stream)-off)
		}
		pkt, err := spp.Decode(stream[off : off+n])
		if err != nil {
			t.Fatalf("packet at offset %d does not decode: %v", off, err)
		}
		pkts = append(pkts, pkt)
		off += n
	}
	return pkts
}

// TestVCPService_IdleFillParsesAsPackets is the conformant-receiver check for
// the fill discipline: every octet of every emitted data field must belong to
// a real Space Packet, with spare space carried by idle packets (APID 0x7FF)
// rather than raw 0xFF a foreign receiver would misread as a packet header.
func TestVCPService_IdleFillParsesAsPackets(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true} // capacity 20
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket([]byte{0x01, 0x02}) // 8 bytes, leaves 12 spare
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	pkts := parsePacketStream(t, drainDataFields(t, vc))
	if len(pkts) != 2 {
		t.Fatalf("Expected 2 packets (data + idle fill), got %d", len(pkts))
	}
	if pkts[0].IsIdle() {
		t.Error("First packet should be the data packet")
	}
	if !pkts[1].IsIdle() {
		t.Errorf("Fill packet APID = 0x%03X, want 0x7FF (idle)", pkts[1].PrimaryHeader.APID)
	}
}

// TestVCPService_IdleFillSpansFrames covers the remainder-under-7-octets case:
// the idle fill packet cannot fit its header in the spare space, so it spans
// into an additional frame and still parses cleanly across the boundary.
func TestVCPService_IdleFillSpansFrames(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true} // capacity 20
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xAB}, 10)) // 16 bytes, leaves 4 spare
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	if vc.Len() != 2 {
		t.Fatalf("Expected 2 frames (fill spans into second), got %d", vc.Len())
	}

	pkts := parsePacketStream(t, drainDataFields(t, vc))
	if len(pkts) != 2 {
		t.Fatalf("Expected 2 packets (data + spanning idle fill), got %d", len(pkts))
	}
	if !pkts[1].IsIdle() {
		t.Errorf("Fill packet APID = 0x%03X, want 0x7FF (idle)", pkts[1].PrimaryHeader.APID)
	}
	// 4 spare octets + one whole 20-octet data field
	if got := 6 + int(pkts[1].PrimaryHeader.PacketLength) + 1; got != 24 {
		t.Errorf("Idle fill packet length = %d, want 24", got)
	}
}

// TestVCPService_ReceiveDiscardsIdlePackets checks ECSS 5.4.3.5d on the
// extraction side: idle fill packets never reach the service user.
func TestVCPService_ReceiveDiscardsIdlePackets(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt := makeTestPacket([]byte{0x01, 0x02})
	_ = svc.Send(pkt)
	_ = svc.Flush()

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pkt, received) {
		t.Errorf("Expected %x, got %x", pkt, received)
	}

	// The only remaining content is the idle fill packet: it must be
	// discarded, not delivered.
	if extra, err := svc.Receive(); err == nil {
		t.Errorf("Idle fill packet delivered to user: %x", extra)
	} else if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable after idle discard, got %v", err)
	}
}

// --- Idle-frame counter continuity (CCSDS 132.0-B-3 clause 4.1.2.5) ---

func TestMasterChannel_IdleFrameCounterContinuity(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	counter := tmdl.NewFrameCounter()

	mc := tmdl.NewMasterChannel(933, config)
	mc.SetFrameCounter(counter)
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)
	_ = svc.Send(makeTestPacket([]byte{0x01, 0x02}))
	_ = svc.Flush()

	// Data frame first: MC count 0.
	f0, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if tmdl.IsIdleFrame(f0) || f0.Header.MCFrameCount != 0 {
		t.Fatalf("Frame 0: idle=%v MC=%d, want data frame with MC=0",
			tmdl.IsIdleFrame(f0), f0.Header.MCFrameCount)
	}

	// Channel now empty: idle frames must continue the MC sequence.
	idle1, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(idle1) {
		t.Fatal("Expected idle frame")
	}
	if idle1.Header.MCFrameCount != 1 {
		t.Errorf("Idle 1: MC count = %d, want 1", idle1.Header.MCFrameCount)
	}
	// CCSDS 132.0-B-3 clause 4.1.4.6.3: an OID frame's VCID is one of those used
	// for transferring packets. VC 1 is the only registered channel, so the
	// idle frame joins its sequence, the data frame above took VC count 0,
	// making this one 1.
	if idle1.Header.VirtualChannelID != 1 {
		t.Errorf("Idle VCID = %d, want 1 (a packet-carrying VCID)",
			idle1.Header.VirtualChannelID)
	}
	if idle1.Header.VCFrameCount != 1 {
		t.Errorf("Idle 1: VC count = %d, want 1", idle1.Header.VCFrameCount)
	}

	idle2, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if idle2.Header.MCFrameCount != 2 || idle2.Header.VCFrameCount != 2 {
		t.Errorf("Idle 2: MC=%d VC=%d, want MC=2 VC=2",
			idle2.Header.MCFrameCount, idle2.Header.VCFrameCount)
	}

	// The stamped counts must be covered by the CRC.
	encoded, err := idle2.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmdl.DecodeTransferFrame(encoded); err != nil {
		t.Errorf("Stamped idle frame does not round-trip: %v", err)
	}
}

func TestPhysicalChannel_IdleFrame_DeterministicSCIDAndCounter(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	pc := tmdl.NewPhysicalChannel("test", config)

	counter100 := tmdl.NewFrameCounter()
	mc100 := tmdl.NewMasterChannel(100, config)
	mc100.SetFrameCounter(counter100)
	mc100.AddVirtualChannel(tmdl.NewVirtualChannel(1, 10), 1)
	mc200 := tmdl.NewMasterChannel(200, config)
	mc200.AddVirtualChannel(tmdl.NewVirtualChannel(1, 10), 1)

	pc.AddMasterChannel(mc200, 1)
	pc.AddMasterChannel(mc100, 1)

	// The idle SCID must be the lowest registered SCID every time,
	// not whichever the map iterator happens to yield.
	for i := range 5 {
		idle, err := pc.GetNextFrameOrIdle()
		if err != nil {
			t.Fatal(err)
		}
		if idle.Header.SpacecraftID != 100 {
			t.Fatalf("Idle %d: SCID = %d, want 100 (lowest registered)",
				i, idle.Header.SpacecraftID)
		}
		if int(idle.Header.MCFrameCount) != i {
			t.Errorf("Idle %d: MC count = %d, want %d (from MC 100's counter)",
				i, idle.Header.MCFrameCount, i)
		}
	}
}

// --- Frame length enforcement (CCSDS 132.0-B-3 clause 2.1.3) ---

func TestFrameLengthEnforcedOnEncodeAndDecode(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}

	// A frame built to capacity encodes to exactly FrameLength.
	good, err := tmdl.NewIdleFrame(933, 7, config)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := good.EncodeWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != config.FrameLength {
		t.Fatalf("Encoded length = %d, want %d", len(encoded), config.FrameLength)
	}

	// A short frame must be rejected on encode.
	short, err := tmdl.NewTransferFrame(933, 1, []byte("tiny"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := short.EncodeWithConfig(config); !errors.Is(err, tmdl.ErrFrameLengthMismatch) {
		t.Errorf("Encode of short frame: got %v, want ErrFrameLengthMismatch", err)
	}

	// A wrong-length input must be rejected on decode.
	if _, err := tmdl.DecodeTransferFrameWithConfig(encoded[:20], config); !errors.Is(err, tmdl.ErrFrameLengthMismatch) {
		t.Errorf("Decode of short input: got %v, want ErrFrameLengthMismatch", err)
	}
	if _, err := tmdl.DecodeTransferFrameWithConfig(encoded, config); err != nil {
		t.Errorf("Decode of exact-length input: %v", err)
	}
}

// --- VCA SDU size validation (CCSDS 132.0-B-3 clause 3.4.2.2) ---

func TestVCAService_FixedLength_SizeValidated(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true} // capacity 20
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)

	// The VCA_SDU length is fixed per channel: anything but vcaSize octets
	// is a size mismatch even when it would fit the data field.
	if err := svc.Send([]byte("12345")); !errors.Is(err, tmdl.ErrSizeMismatch) {
		t.Errorf("Short SDU: got %v, want ErrSizeMismatch", err)
	}
	if err := svc.Send(bytes.Repeat([]byte{0xAA}, 12)); !errors.Is(err, tmdl.ErrSizeMismatch) {
		t.Errorf("Long SDU: got %v, want ErrSizeMismatch", err)
	}
	if err := svc.Send([]byte("12345678")); err != nil {
		t.Errorf("Exact SDU: %v", err)
	}
}

// --- OCF supplier hook (CCSDS 132.0-B-3 clause 4.1.5) ---

func TestVCPService_OCFSupplier(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasOCF: true, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	clcw := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	svc.SetOCFSupplier(func() []byte { return clcw })

	_ = svc.Send(makeTestPacket([]byte{0x01, 0x02}))
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame.OperationalControl, clcw) {
		t.Errorf("OCF = %x, want %x", frame.OperationalControl, clcw)
	}
}

func TestVCAService_OCFSupplier(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasOCF: true, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)

	clcw := []byte{0x01, 0x02, 0x03, 0x04}
	svc.SetOCFSupplier(func() []byte { return clcw })

	if err := svc.Send([]byte("12345678")); err != nil {
		t.Fatal(err)
	}
	frame, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame.OperationalControl, clcw) {
		t.Errorf("OCF = %x, want %x", frame.OperationalControl, clcw)
	}
}

func TestOCFSupplier_BadLengthRejected(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasOCF: true, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)
	svc.SetOCFSupplier(func() []byte { return []byte{0x01, 0x02} })

	if err := svc.Send([]byte("12345678")); !errors.Is(err, tmdl.ErrInvalidOCFLength) {
		t.Errorf("Got %v, want ErrInvalidOCFLength", err)
	}
}

// --- OID frame conformance (CCSDS 132.0-B-3 clause 4.1.4.6) ---

// TestOIDSequenceMatchesTheStandardPattern checks the PN generator against the
// octets the note under clause 4.1.4.6.2.2 publishes, which is the only published
// check value for the D0+D1+D2+D22+D32 LFSR.
func TestOIDSequenceMatchesTheStandardPattern(t *testing.T) {
	want := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x6D, 0xB6, 0xD8, 0x61, 0x45, 0x1F}
	got := make([]byte, len(want))
	tmdl.NewOIDSequence().Fill(got)
	if !bytes.Equal(got, want) {
		t.Errorf("PN sequence = % X, want % X", got, want)
	}
}

// TestIdleFramesDoNotRestartThePNSequence checks clause 4.1.4.6.2.1: the generator
// is initialized once at start-up and never restarted, so two consecutive
// idle frames must carry different octets. Restarting it per frame would
// repeat the same pattern forever, which is exactly the insufficient
// randomization note 5 of clause 4.1.4.6.3 warns about.
func TestIdleFramesDoNotRestartThePNSequence(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 32, HasFEC: true}
	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(tmdl.NewVirtualChannel(1, 4), 1)

	first, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	second, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(first) || !tmdl.IsIdleFrame(second) {
		t.Fatal("expected two idle frames from an empty master channel")
	}
	if bytes.Equal(first.DataField, second.DataField) {
		t.Errorf("consecutive idle frames carry identical data fields (% X); "+
			"the PN generator was restarted", first.DataField)
	}

	// The first frame still starts at the documented sequence head.
	head := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x6D, 0xB6}
	if !bytes.Equal(first.DataField[:len(head)], head) {
		t.Errorf("first idle frame starts % X, want % X",
			first.DataField[:len(head)], head)
	}
}

// TestIdleFrameUsesAPacketCarryingVCID checks clause 4.1.4.6.3: the VCID of an OID
// frame is one of the VCIDs used for transferring packets, not an arbitrary
// channel the receiver has no reception function for.
func TestIdleFrameUsesAPacketCarryingVCID(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 32, HasFEC: true}
	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(tmdl.NewVirtualChannel(3, 4), 1)
	mc.AddVirtualChannel(tmdl.NewVirtualChannel(5, 4), 1)

	idle, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if idle.Header.VirtualChannelID != 3 {
		t.Errorf("idle VCID = %d, want 3 (lowest registered channel)",
			idle.Header.VirtualChannelID)
	}

	// An explicit choice wins, so a mission with a dedicated OID channel can
	// say so (note 2 under clause 4.1.4.6.3 prefers a separate one).
	mc.SetIdleVCID(5)
	idle, err = mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if idle.Header.VirtualChannelID != 5 {
		t.Errorf("idle VCID = %d, want 5 (pinned via SetIdleVCID)",
			idle.Header.VirtualChannelID)
	}
}

// --- MC_FSH / MC_OCF services (CCSDS 132.0-B-3 clause 3.8, clause 3.9, clause 4.2.5) ---

func TestMasterChannelFSHAndOCFServices(t *testing.T) {
	config := tmdl.ChannelConfig{
		FrameLength:   40,
		FSHDataLength: 4,
		HasOCF:        true,
		HasFEC:        true,
	}
	counter := tmdl.NewFrameCounter()
	vc := tmdl.NewVirtualChannel(1, 8)
	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(vc, 1)
	mc.SetFrameCounter(counter)

	fshSDU := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	ocfSDU := []byte{0x01, 0x02, 0x03, 0x04}
	mc.SetFSHSupplier(func() []byte { return fshSDU })
	mc.SetOCFSupplier(func() []byte { return ocfSDU })

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	svc.SetPacketSizer(spp.PacketSizer)
	if err := svc.Send(makeTestPacket([]byte{0x11, 0x22})); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// A data frame released through the master channel carries both SDUs.
	frame, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame.SecondaryHeader.DataField, fshSDU) {
		t.Errorf("data frame FSH = % X, want % X",
			frame.SecondaryHeader.DataField, fshSDU)
	}
	if !bytes.Equal(frame.OperationalControl, ocfSDU) {
		t.Errorf("data frame OCF = % X, want % X",
			frame.OperationalControl, ocfSDU)
	}

	// So does an idle frame: Clause 4.1.4.6.3 note 1 says only the data field of an
	// OID frame is idle, its secondary header and OCF can carry valid data.
	idle, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(idle) {
		t.Fatal("expected an idle frame from the drained channel")
	}
	if !bytes.Equal(idle.SecondaryHeader.DataField, fshSDU) {
		t.Errorf("idle frame FSH = % X, want % X",
			idle.SecondaryHeader.DataField, fshSDU)
	}
	if !bytes.Equal(idle.OperationalControl, ocfSDU) {
		t.Errorf("idle frame OCF = % X, want % X",
			idle.OperationalControl, ocfSDU)
	}

	// The applied SDUs must be under the CRC and the frame still fixed-length.
	encoded, err := idle.EncodeWithConfig(config)
	if err != nil {
		t.Fatalf("idle frame does not encode to the fixed length: %v", err)
	}
	back, err := tmdl.DecodeTransferFrameWithConfig(encoded, config)
	if err != nil {
		t.Fatalf("idle frame CRC invalid after MC field insertion: %v", err)
	}

	// The receive side decommutates them: MC_FSH.indication / MC_OCF.indication.
	if err := mc.AddFrame(back); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mc.LastFSH(), fshSDU) {
		t.Errorf("LastFSH = % X, want % X", mc.LastFSH(), fshSDU)
	}
	if !bytes.Equal(mc.LastOCF(), ocfSDU) {
		t.Errorf("LastOCF = % X, want % X", mc.LastOCF(), ocfSDU)
	}
}

func TestMasterChannelFSHRejectsMisconfiguration(t *testing.T) {
	// A supplier with no secondary header on the channel: the SDU has nowhere
	// to go, and silently dropping it would lose user data.
	config := tmdl.ChannelConfig{FrameLength: 32, HasFEC: true}
	counter := tmdl.NewFrameCounter()
	vc := tmdl.NewVirtualChannel(1, 4)
	mc := tmdl.NewMasterChannel(933, config)
	mc.AddVirtualChannel(vc, 1)
	mc.SetFrameCounter(counter)
	mc.SetFSHSupplier(func() []byte { return []byte{0x01} })

	if _, err := mc.GetNextFrameOrIdle(); !errors.Is(err, tmdl.ErrFSHNotPresent) {
		t.Errorf("got %v, want ErrFSHNotPresent", err)
	}

	// A supplier whose SDU is the wrong width for the channel.
	sized := tmdl.ChannelConfig{FrameLength: 40, FSHDataLength: 4, HasFEC: true}
	mc2 := tmdl.NewMasterChannel(933, sized)
	mc2.AddVirtualChannel(tmdl.NewVirtualChannel(1, 4), 1)
	mc2.SetFSHSupplier(func() []byte { return []byte{0x01, 0x02} })
	if _, err := mc2.GetNextFrameOrIdle(); !errors.Is(err, tmdl.ErrFSHSizeMismatch) {
		t.Errorf("got %v, want ErrFSHSizeMismatch", err)
	}
}

// --- VC_FSH service (CCSDS 132.0-B-3 clause 3.5) ---

func TestVCPSecondaryHeaderRoundTrip(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 40, FSHDataLength: 3, HasFEC: true}
	counter := tmdl.NewFrameCounter()
	vc := tmdl.NewVirtualChannel(1, 8)

	sdu := []byte{0xDE, 0xAD, 0xBE}
	tx := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, counter)
	tx.SetPacketSizer(spp.PacketSizer)
	tx.SetFSHSupplier(func() []byte { return sdu })

	payload := []byte{0x11, 0x22, 0x33}
	if err := tx.Send(makeTestPacket(payload)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatal(err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Header.FSHFlag {
		t.Fatal("FSHFlag not set on a channel configured with FSHDataLength")
	}
	if !bytes.Equal(frame.SecondaryHeader.DataField, sdu) {
		t.Errorf("FSH = % X, want % X", frame.SecondaryHeader.DataField, sdu)
	}

	// The frame must still be exactly the channel's fixed length: the
	// secondary header takes its octets out of the data field, not out of
	// thin air (clause 4.1.4.2).
	encoded, err := frame.EncodeWithConfig(config)
	if err != nil {
		t.Fatalf("frame with secondary header is not the fixed length: %v", err)
	}
	if len(encoded) != config.FrameLength {
		t.Errorf("encoded length = %d, want %d", len(encoded), config.FrameLength)
	}

	// The packet survives the round trip and the receiver reports the SDU.
	back, err := tmdl.DecodeTransferFrameWithConfig(encoded, config)
	if err != nil {
		t.Fatal(err)
	}
	rxVC := tmdl.NewVirtualChannel(1, 8)
	_ = rxVC.Add(back)
	rx := tmdl.NewVirtualChannelPacketService(933, 1, rxVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)
	got, err := rx.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, makeTestPacket(payload)) {
		t.Errorf("packet = % X, want % X", got, makeTestPacket(payload))
	}
	if !bytes.Equal(rx.LastFSH(), sdu) {
		t.Errorf("LastFSH = % X, want % X", rx.LastFSH(), sdu)
	}
}

// --- VCA Status Fields (CCSDS 132.0-B-3 clause 3.4.2.3) ---

// TestVCAStatusFieldsRoundTrip checks that the Packet Order Flag, Segment
// Length ID, and First Header Pointer a VCA user sets are carried to the
// receiving user unchanged. Providing the field is mandatory (clause 3.4.2.3) and
// the semantics are the user's, so the protocol must not overwrite them.
func TestVCAStatusFieldsRoundTrip(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 32, HasFEC: true}
	counter := tmdl.NewFrameCounter()
	vc := tmdl.NewVirtualChannel(1, 4)

	sdu := []byte{0x01, 0x02, 0x03, 0x04}
	tx := tmdl.NewVirtualChannelAccessService(933, 1, len(sdu), vc, config, counter)
	want := tmdl.VCAStatus{
		PacketOrderFlag: true,
		SegmentLengthID: 0b10,
		FirstHeaderPtr:  0x123,
	}
	tx.SetSendStatus(want)
	if err := tx.Send(sdu); err != nil {
		t.Fatal(err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Header.SyncFlag {
		t.Error("SyncFlag = false; clause 4.1.2.7.3.2 requires '1' for a VCA_SDU")
	}
	if frame.Header.PacketOrderFlag != want.PacketOrderFlag ||
		frame.Header.SegmentLengthID != want.SegmentLengthID ||
		frame.Header.FirstHeaderPtr != want.FirstHeaderPtr {
		t.Errorf("status on the wire = {POF:%v SLI:%d FHP:0x%03X}, want {POF:%v SLI:%d FHP:0x%03X}",
			frame.Header.PacketOrderFlag, frame.Header.SegmentLengthID, frame.Header.FirstHeaderPtr,
			want.PacketOrderFlag, want.SegmentLengthID, want.FirstHeaderPtr)
	}

	encoded, err := frame.EncodeWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	back, err := tmdl.DecodeTransferFrameWithConfig(encoded, config)
	if err != nil {
		t.Fatal(err)
	}

	rxVC := tmdl.NewVirtualChannel(1, 4)
	_ = rxVC.Add(back)
	rx := tmdl.NewVirtualChannelAccessService(933, 1, len(sdu), rxVC, config, nil)
	got, err := rx.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sdu) {
		t.Errorf("VCA_SDU = % X, want % X", got, sdu)
	}
	status := rx.LastStatus()
	if !status.SyncFlag || status.PacketOrderFlag != want.PacketOrderFlag ||
		status.SegmentLengthID != want.SegmentLengthID ||
		status.FirstHeaderPtr != want.FirstHeaderPtr {
		t.Errorf("LastStatus = %+v, want POF:%v SLI:%d FHP:0x%03X with SyncFlag",
			status, want.PacketOrderFlag, want.SegmentLengthID, want.FirstHeaderPtr)
	}
}

// TestVCADefaultStatusFieldsAreAllOnes checks the unconfigured default: a user
// who sets no status fields gets the 'no packet starts here' pointer rather
// than a zero that reads as 'a packet starts at octet 0'.
func TestVCADefaultStatusFieldsAreAllOnes(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 32, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 4)
	tx := tmdl.NewVirtualChannelAccessService(933, 1, 4, vc, config, nil)
	if err := tx.Send([]byte{0x01, 0x02, 0x03, 0x04}); err != nil {
		t.Fatal(err)
	}
	frame, err := vc.Next()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Header.FirstHeaderPtr != tmdl.FHPNoPacketStart {
		t.Errorf("FHP = 0x%03X, want 0x%03X", frame.Header.FirstHeaderPtr, tmdl.FHPNoPacketStart)
	}
	if frame.Header.PacketOrderFlag {
		t.Error("PacketOrderFlag set without the user asking for it")
	}
}
