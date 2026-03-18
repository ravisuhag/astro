package tmdl_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// makeTestPacket builds a minimal CCSDS Space Packet with the given payload.
// PVN=0, Type=0, SecHdr=0, APID=1, SeqFlags=3, SeqCount=0.
// Total length = 6 (header) + len(payload).
func makeTestPacket(payload []byte) []byte {
	pkt := make([]byte, 6+len(payload))
	pkt[0] = 0x00 // PVN=0, Type=0, SecHdr=0, APID high=0
	pkt[1] = 0x01 // APID low = 1
	pkt[2] = 0xC0 // SeqFlags=3, SeqCount high=0
	pkt[3] = 0x00 // SeqCount low=0
	binary.BigEndian.PutUint16(pkt[4:6], uint16(len(payload)-1))
	copy(pkt[6:], payload)
	return pkt
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
	// capacity = 40, packet = 8 bytes (6 hdr + 2 payload) → fits in one frame
	config := tmdl.ChannelConfig{FrameLength: 48, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

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
	// capacity = 10, packet = 16 bytes (6 hdr + 10 payload) → spans 2 frames
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	pkt := makeTestPacket(bytes.Repeat([]byte{0xAB}, 10))
	if err := svc.Send(pkt); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(); err != nil {
		t.Fatal(err)
	}

	// 16 bytes → ceil(16/10) = 2 frames
	if vc.Len() != 2 {
		t.Fatalf("Expected 2 frames, got %d", vc.Len())
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
	// capacity = 20, two packets of 8 bytes each = 16 bytes → fit in one frame
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

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

	// Both packets packed into 1 frame (16 bytes < 20 capacity)
	if vc.Len() != 1 {
		t.Fatalf("Expected 1 frame (two packets packed), got %d", vc.Len())
	}

	// Verify FHP = 0 (first packet starts at byte 0)
	frame, _ := vc.GetNextFrame()
	if frame.Header.FirstHeaderPtr != 0 {
		t.Errorf("FHP = %d, want 0", frame.Header.FirstHeaderPtr)
	}

	// Re-send and receive both packets
	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 1, vc2, config, nil)
	svc2.Send(pkt1)
	svc2.Send(pkt2)
	svc2.Flush()

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
	// capacity = 12, pkt1=8 bytes, pkt2=8 bytes → total 16 bytes
	// Frame 1: [pkt1(8) + pkt2_start(4)] FHP=0
	// Frame 2: [pkt2_end(4) + idle(8)] FHP=0x07FE
	config := tmdl.ChannelConfig{FrameLength: 20, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	pkt1 := makeTestPacket([]byte{0x01, 0x02})
	pkt2 := makeTestPacket([]byte{0x03, 0x04})

	svc.Send(pkt1)
	svc.Send(pkt2)
	svc.Flush()

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
	// Frame 2: [pkt2_end(4)|idle(8)] FHP=0x07FE
	config := tmdl.ChannelConfig{FrameLength: 20, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	pkt1 := makeTestPacket([]byte{0x01, 0x02}) // 8 bytes
	pkt2 := makeTestPacket([]byte{0x03, 0x04}) // 8 bytes

	svc.Send(pkt1)
	svc.Send(pkt2)
	svc.Flush()

	f1, _ := vc.GetNextFrame()
	if f1.Header.FirstHeaderPtr != 0 {
		t.Errorf("Frame 1: FHP = %d, want 0", f1.Header.FirstHeaderPtr)
	}

	f2, _ := vc.GetNextFrame()
	if f2.Header.FirstHeaderPtr != 0x07FE {
		t.Errorf("Frame 2: FHP = 0x%04X, want 0x07FE", f2.Header.FirstHeaderPtr)
	}
}

func TestVCPService_Packing_FHP_MidFrame(t *testing.T) {
	// capacity = 10, pkt1=7 bytes → Frame 1: [pkt1(7)|pkt2_start(3)] FHP=0
	// pkt2=7 bytes → pkt2 starts at offset 7 in frame 1
	// But actually: frame 1 has capacity 10. After pkt1 (7 bytes), pkt2 starts at offset 7.
	// If total < capacity (14 bytes > 10), frame 1 = first 10 bytes, with pkt2 at offset 7.
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true} // capacity = 10
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	pkt1 := makeTestPacket([]byte{0xAA}) // 7 bytes
	pkt2 := makeTestPacket([]byte{0xBB}) // 7 bytes

	svc.Send(pkt1) // buffer: 7 bytes, offsets: [0]
	svc.Send(pkt2) // buffer: 14 bytes, offsets: [0, 7] → generates 1 full frame (10 bytes)
	svc.Flush()     // flushes remaining 4 bytes

	// Frame 1 should have FHP=0 (pkt1 at offset 0, pkt2 at offset 7)
	f1, _ := vc.GetNextFrame()
	if f1.Header.FirstHeaderPtr != 0 {
		t.Errorf("Frame 1 FHP = %d, want 0", f1.Header.FirstHeaderPtr)
	}

	// Frame 2 should have FHP=0x07FE (continuation of pkt2)
	f2, _ := vc.GetNextFrame()
	if f2.Header.FirstHeaderPtr != 0x07FE {
		t.Errorf("Frame 2 FHP = 0x%04X, want 0x07FE", f2.Header.FirstHeaderPtr)
	}
}

func TestVCPService_Packing_IdleFrameSkip(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	pkt := makeTestPacket([]byte{0x01, 0x02})
	svc.Send(pkt)
	svc.Flush()

	// Insert idle before data frame
	idle, _ := tmdl.NewIdleFrame(933, 1, config)
	var frames []*tmdl.TMTransferFrame
	for vc.HasFrames() {
		f, _ := vc.GetNextFrame()
		frames = append(frames, f)
	}
	vc.AddFrame(idle)
	for _, f := range frames {
		vc.AddFrame(f)
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

	pkt1 := makeTestPacket([]byte{0x01, 0x02})
	pkt2 := makeTestPacket([]byte{0x03, 0x04})

	svc.Send(pkt1)
	svc.Send(pkt2)
	svc.Flush()

	// Remove the first frame (simulate loss)
	vc.GetNextFrame() // discard pkt1's frame

	// pkt2's frame remains — it has FHP=0x07FE (continuation) or FHP with offset
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

	bigPkt1 := makeTestPacket(bytes.Repeat([]byte{0xAA}, 5))  // 11 bytes
	bigPkt2 := makeTestPacket(bytes.Repeat([]byte{0xBB}, 5))  // 11 bytes

	svc2.Send(bigPkt1) // 11 bytes, generates 1 full frame (10 bytes), 1 byte remains
	svc2.Send(bigPkt2) // +11=12 bytes remaining, generates 1 full frame, 2 remain
	svc2.Flush()        // flush remaining 2 bytes

	// Should have 3 frames: F0(VCcount=0), F1(VCcount=1), F2(VCcount=2)
	// Remove F0 (contains start of pkt1)
	vc2.GetNextFrame() // discard

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

	// Custom sizer: fixed 5-byte packets
	svc.SetPacketSizer(func(data []byte) int {
		if len(data) < 5 {
			return -1
		}
		return 5
	})

	pkt := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	svc.Send(pkt)
	svc.Flush()

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

	if err := svc.Flush(); err != nil {
		t.Errorf("Empty flush should be no-op: %v", err)
	}
	if vc.Len() != 0 {
		t.Errorf("No frames should be generated: got %d", vc.Len())
	}
}

// --- PVN Validation Tests ---

func TestVCPService_PVNValidation(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	svc.SetValidPVNs(0)

	if err := svc.Send([]byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("Valid PVN=0 rejected: %v", err)
	}
	err := svc.Send([]byte{0x20, 0x01, 0x02}) // PVN=1
	if !errors.Is(err, tmdl.ErrInvalidPVN) {
		t.Errorf("Expected ErrInvalidPVN for PVN=1, got %v", err)
	}
}

func TestVCPService_PVNValidation_MultiplePVNs(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	svc.SetValidPVNs(0, 1)

	if err := svc.Send([]byte{0x00, 0x01}); err != nil {
		t.Errorf("PVN=0 should be valid: %v", err)
	}
	if err := svc.Send([]byte{0x20, 0x01}); err != nil {
		t.Errorf("PVN=1 should be valid: %v", err)
	}
	err := svc.Send([]byte{0x40, 0x01})
	if !errors.Is(err, tmdl.ErrInvalidPVN) {
		t.Errorf("PVN=2 should be invalid: %v", err)
	}
}

func TestVCPService_PVNValidation_Disabled(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	if err := svc.Send([]byte{0xE0, 0x01}); err != nil {
		t.Errorf("Without PVN validation, any packet should be accepted: %v", err)
	}
}

// --- VCF Service Tests ---

func TestVCFService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("frame data"), nil, nil)
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
	svc.Send([]byte("abcd"))
	frame, _ := vc.GetNextFrame()
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
	svc.Send([]byte("abcd"))
	svc.Receive()
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
	svc.Send(data)
	frame, _ := vc.GetNextFrame()
	if len(frame.DataField) != capacity {
		t.Errorf("DataField len = %d, want %d", len(frame.DataField), capacity)
	}

	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc2, config, nil)
	svc2.Send(data)
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

// --- MasterChannel Tests ---

func TestMasterChannel_AddAndGet(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	mc.AddFrame(frame)

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
	svc.Send(makeTestPacket([]byte{0x01}))
	svc.Flush()

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
	svc1.Send([]byte("from vc1"))
	svc2.Send([]byte("from vc2"))

	f1, _ := mc.GetNextFrame()
	if string(f1.DataField) != "from vc1" {
		t.Errorf("Expected 'from vc1', got %q", f1.DataField)
	}
	f2, _ := mc.GetNextFrame()
	if string(f2.DataField) != "from vc2" {
		t.Errorf("Expected 'from vc2', got %q", f2.DataField)
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

func TestSpacePacketSizer(t *testing.T) {
	pkt := makeTestPacket([]byte{0x01, 0x02, 0x03})
	size := tmdl.SpacePacketSizer(pkt)
	if size != len(pkt) {
		t.Errorf("SpacePacketSizer = %d, want %d", size, len(pkt))
	}

	if tmdl.SpacePacketSizer([]byte{0x00}) != -1 {
		t.Error("Expected -1 for too-short data")
	}
}
