package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// --- VCP Service Tests ---

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
		t.Fatalf("Send 1 failed: %v", err)
	}
	if err := svc.Send(data2); err != nil {
		t.Fatalf("Send 2 failed: %v", err)
	}

	received1, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive 1 failed: %v", err)
	}
	received2, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive 2 failed: %v", err)
	}

	if !bytes.Equal(data1, received1) {
		t.Errorf("Expected %s, got %s", data1, received1)
	}
	if !bytes.Equal(data2, received2) {
		t.Errorf("Expected %s, got %s", data2, received2)
	}
}

// --- VCP Segmentation Tests ---

func TestVCPService_Segmentation_SingleFrame(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 100, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	data := []byte("small")
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if vc.Len() != 1 {
		t.Fatalf("Expected 1 frame, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if !bytes.Equal(data, received) {
		t.Errorf("Expected %q, got %q", data, received)
	}
}

func TestVCPService_Segmentation_MultiFrame(t *testing.T) {
	// Frame: 6 header + 10 data + 2 CRC = 18 bytes
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	capacity := config.DataFieldCapacity(0) // 10

	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	// 20 bytes of data + 2 byte prefix = 22 bytes → ceil(22/10) = 3 frames
	data := bytes.Repeat([]byte("A"), 20)
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if vc.Len() != 3 {
		t.Fatalf("Expected 3 frames, got %d", vc.Len())
	}

	// Verify frame headers without consuming (peek via separate receive svc)
	vcPeek := tmdl.NewVirtualChannel(1, 100)
	svcPeek := tmdl.NewVirtualChannelPacketService(933, 1, vcPeek, config, nil)

	// Re-send to peek VC
	if err := svcPeek.Send(data); err != nil {
		t.Fatal(err)
	}

	f1, _ := vcPeek.GetNextFrame()
	if f1.Header.FirstHeaderPtr != 0 {
		t.Errorf("Frame 1 FHP = %d, want 0", f1.Header.FirstHeaderPtr)
	}
	if len(f1.DataField) != capacity {
		t.Errorf("Frame 1 DataField len = %d, want %d", len(f1.DataField), capacity)
	}

	f2, _ := vcPeek.GetNextFrame()
	if f2.Header.FirstHeaderPtr != 0x07FE {
		t.Errorf("Frame 2 FHP = 0x%04X, want 0x07FE", f2.Header.FirstHeaderPtr)
	}

	f3, _ := vcPeek.GetNextFrame()
	if f3.Header.FirstHeaderPtr != 0x07FE {
		t.Errorf("Frame 3 FHP = 0x%04X, want 0x07FE", f3.Header.FirstHeaderPtr)
	}

	// Verify reassembly
	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if !bytes.Equal(data, received) {
		t.Errorf("Reassembly mismatch: got %d bytes, want %d", len(received), len(data))
	}
}

func TestVCPService_Segmentation_ExactFit(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	// capacity = 10: 8 bytes data + 2 byte prefix = exactly 1 frame

	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	// 8 bytes data + 2 byte prefix = 10 bytes = exactly 1 frame capacity
	data := bytes.Repeat([]byte("B"), 8)
	if err := svc.Send(data); err != nil {
		t.Fatal(err)
	}

	if vc.Len() != 1 {
		t.Fatalf("Expected 1 frame for exact fit, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, received) {
		t.Errorf("Expected %q, got %q", data, received)
	}
}

func TestVCPService_Segmentation_MultiplePackets(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	data1 := []byte("first")
	data2 := []byte("second-packet!")

	if err := svc.Send(data1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Send(data2); err != nil {
		t.Fatal(err)
	}

	received1, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	received2, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data1, received1) {
		t.Errorf("Packet 1: expected %q, got %q", data1, received1)
	}
	if !bytes.Equal(data2, received2) {
		t.Errorf("Packet 2: expected %q, got %q", data2, received2)
	}
}

func TestVCPService_Segmentation_IdleFrameSkip(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	if err := svc.Send([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	// Insert an idle frame before the data frames
	idle, err := tmdl.NewIdleFrame(933, 1, config)
	if err != nil {
		t.Fatal(err)
	}

	// Drain and re-add with idle in front
	var frames []*tmdl.TMTransferFrame
	for vc.HasFrames() {
		f, _ := vc.GetNextFrame()
		frames = append(frames, f)
	}

	if err := vc.AddFrame(idle); err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		if err := vc.AddFrame(f); err != nil {
			t.Fatal(err)
		}
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive failed (should skip idle): %v", err)
	}
	if !bytes.Equal(received, []byte("hello")) {
		t.Errorf("Expected 'hello', got %q", received)
	}
}

func TestVCPService_Segmentation_PacketTooLarge(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 18, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	data := make([]byte, 65536)
	err := svc.Send(data)
	if !errors.Is(err, tmdl.ErrPacketTooLarge) {
		t.Errorf("Expected ErrPacketTooLarge, got %v", err)
	}
}

func TestVCPService_PVNValidation(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	svc.SetValidPVNs(0) // only CCSDS v1 (PVN=000)

	// PVN=0 (bits 7-5 = 000): valid
	validPacket := []byte{0x00, 0x01, 0x02}
	if err := svc.Send(validPacket); err != nil {
		t.Fatalf("Send valid PVN failed: %v", err)
	}

	// PVN=1 (bits 7-5 = 001 → 0x20): invalid
	invalidPacket := []byte{0x20, 0x01, 0x02}
	err := svc.Send(invalidPacket)
	if !errors.Is(err, tmdl.ErrInvalidPVN) {
		t.Errorf("Expected ErrInvalidPVN, got %v", err)
	}

	// PVN=7 (bits 7-5 = 111 → 0xE0): invalid
	err = svc.Send([]byte{0xE0, 0x01})
	if !errors.Is(err, tmdl.ErrInvalidPVN) {
		t.Errorf("Expected ErrInvalidPVN for PVN=7, got %v", err)
	}
}

func TestVCPService_PVNValidation_MultiplePVNs(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	svc.SetValidPVNs(0, 1) // accept PVN 0 and 1

	// PVN=0: valid
	if err := svc.Send([]byte{0x00, 0x01}); err != nil {
		t.Errorf("PVN=0 should be valid: %v", err)
	}

	// PVN=1 (0x20): valid
	if err := svc.Send([]byte{0x20, 0x01}); err != nil {
		t.Errorf("PVN=1 should be valid: %v", err)
	}

	// PVN=2 (0x40): invalid
	err := svc.Send([]byte{0x40, 0x01})
	if !errors.Is(err, tmdl.ErrInvalidPVN) {
		t.Errorf("PVN=2 should be invalid: %v", err)
	}
}

func TestVCPService_PVNValidation_Disabled(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, tmdl.ChannelConfig{}, nil)
	// No SetValidPVNs call — validation disabled

	// Any PVN should be accepted
	if err := svc.Send([]byte{0xE0, 0x01}); err != nil {
		t.Errorf("With no PVN validation, any packet should be accepted: %v", err)
	}
}

func TestVCPService_Segmentation_DataFieldTooSmall(t *testing.T) {
	// Frame: 6 header + 2 data + 2 CRC = 10 bytes → capacity 2 (< 3)
	config := tmdl.ChannelConfig{FrameLength: 10, HasFEC: true}
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)

	err := svc.Send([]byte("x"))
	if !errors.Is(err, tmdl.ErrDataFieldTooSmall) {
		t.Errorf("Expected ErrDataFieldTooSmall, got %v", err)
	}
}

// --- VCF Service Tests ---

func TestVCFService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("frame data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if err := svc.Send(encoded); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if len(received) == 0 {
		t.Error("Expected non-empty received data")
	}
}

func TestVCFService_SendInvalid(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)
	err := svc.Send([]byte{})
	if !errors.Is(err, tmdl.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
	if err := svc.Send([]byte{0xFF}); err == nil {
		t.Error("Expected error for invalid frame bytes")
	}
}

func TestVCFService_ReceiveEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelFrameService(1, vc)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

// --- VCA Service Tests ---

func TestVCAService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)

	data := []byte("12345678")
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

func TestVCAService_SyncFlag(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 4, vc, tmdl.ChannelConfig{}, nil)

	if err := svc.Send([]byte("abcd")); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	frame, err := vc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame failed: %v", err)
	}

	if !frame.Header.SyncFlag {
		t.Error("Expected SyncFlag=true for VCA frame")
	}
	if frame.Header.FirstHeaderPtr != 0x07FF {
		t.Errorf("FirstHeaderPtr = 0x%04X, want 0x07FF for VCA frame", frame.Header.FirstHeaderPtr)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmdl.DecodeTMTransferFrame(encoded); err != nil {
		t.Errorf("Frame CRC invalid after VCA header changes: %v", err)
	}
}

func TestVCAService_Padding(t *testing.T) {
	// Frame: 6 header + 20 data + 2 CRC = 28 bytes
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	capacity := config.DataFieldCapacity(0) // 20

	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)

	data := []byte("12345678")
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Check frame data field is padded to capacity
	frame, _ := vc.GetNextFrame()
	if len(frame.DataField) != capacity {
		t.Errorf("DataField len = %d, want %d", len(frame.DataField), capacity)
	}

	// Check trailing bytes are 0xFF
	for i := 8; i < capacity; i++ {
		if frame.DataField[i] != 0xFF {
			t.Errorf("DataField[%d] = 0x%02X, want 0xFF", i, frame.DataField[i])
			break
		}
	}

	// Re-send and verify Receive trims correctly
	vc2 := tmdl.NewVirtualChannel(1, 100)
	svc2 := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc2, config, nil)
	if err := svc2.Send(data); err != nil {
		t.Fatal(err)
	}
	received, err := svc2.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, received) {
		t.Errorf("Expected %q, got %q", data, received)
	}
}

func TestVCAService_Padding_TooLarge(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 12, HasFEC: true} // capacity = 4
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, config, nil)

	err := svc.Send([]byte("12345678"))
	if !errors.Is(err, tmdl.ErrDataTooLarge) {
		t.Errorf("Expected ErrDataTooLarge, got %v", err)
	}
}

func TestVCAService_StatusFields(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 4, vc, tmdl.ChannelConfig{}, nil)

	if err := svc.Send([]byte("abcd")); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}

	status := svc.LastStatus()
	if !status.SyncFlag {
		t.Error("Expected SyncFlag=true for VCA frame")
	}
	if status.SegmentLengthID != 0b11 {
		t.Errorf("SegmentLengthID = %d, want 3", status.SegmentLengthID)
	}
}

func TestVCAService_SendSizeMismatch(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)
	err := svc.Send([]byte("short"))
	if !errors.Is(err, tmdl.ErrSizeMismatch) {
		t.Errorf("Expected ErrSizeMismatch, got %v", err)
	}
}

func TestVCAService_ReceiveEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, tmdl.ChannelConfig{}, nil)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

// --- MasterChannel Tests ---

func TestMasterChannel_AddAndGet(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	if err := mc.AddFrame(frame); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	if !mc.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be true")
	}

	got, err := mc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame failed: %v", err)
	}

	if got != frame {
		t.Error("Expected same frame back")
	}

	if mc.HasPendingFrames() {
		t.Error("Expected HasPendingFrames to be false after draining")
	}
}

func TestMasterChannel_SCIDMismatch(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, err := tmdl.NewTMTransferFrame(500, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	err = mc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrSCIDMismatch) {
		t.Errorf("Expected ErrSCIDMismatch, got %v", err)
	}
}

func TestMasterChannel_VCIDNotFound(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	err = mc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrVirtualChannelNotFound) {
		t.Errorf("Expected ErrVirtualChannelNotFound, got %v", err)
	}
}

func TestMasterChannel_GetEmpty(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	_, err := mc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoVirtualChannels) {
		t.Errorf("Expected ErrNoVirtualChannels, got %v", err)
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

	if err := svc1.Send([]byte("from vc1")); err != nil {
		t.Fatalf("Send vc1: %v", err)
	}
	if err := svc2.Send([]byte("from vc2")); err != nil {
		t.Fatalf("Send vc2: %v", err)
	}

	f1, err := mc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame 1: %v", err)
	}
	if string(f1.DataField) != "from vc1" {
		t.Errorf("Expected 'from vc1', got %q", f1.DataField)
	}

	f2, err := mc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame 2: %v", err)
	}
	if string(f2.DataField) != "from vc2" {
		t.Errorf("Expected 'from vc2', got %q", f2.DataField)
	}
}

func TestMasterChannel_GetNextFrameOrIdle_HasFrames(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, config, nil)
	if err := svc.Send([]byte("data")); err != nil {
		t.Fatal(err)
	}

	frame, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle failed: %v", err)
	}
	if tmdl.IsIdleFrame(frame) {
		t.Error("Expected data frame, got idle")
	}
}

func TestMasterChannel_GetNextFrameOrIdle_NoFrames(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	mc := tmdl.NewMasterChannel(933, config)
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	frame, err := mc.GetNextFrameOrIdle()
	if err != nil {
		t.Fatalf("GetNextFrameOrIdle failed: %v", err)
	}
	if !tmdl.IsIdleFrame(frame) {
		t.Error("Expected idle frame when no data available")
	}
}

func TestMasterChannel_GetNextFrameOrIdle_ZeroConfig(t *testing.T) {
	mc := tmdl.NewMasterChannel(933, tmdl.ChannelConfig{})
	vc := tmdl.NewVirtualChannel(1, 10)
	mc.AddVirtualChannel(vc, 1)

	_, err := mc.GetNextFrameOrIdle()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable with zero config, got %v", err)
	}
}

// --- FrameCounter Tests ---

func TestFrameCounter(t *testing.T) {
	counter := tmdl.NewFrameCounter()

	mc, vc := counter.Next(1)
	if mc != 0 || vc != 0 {
		t.Errorf("First call: MC=%d VC=%d, want 0,0", mc, vc)
	}
	mc, vc = counter.Next(1)
	if mc != 1 || vc != 1 {
		t.Errorf("Second call same VCID: MC=%d VC=%d, want 1,1", mc, vc)
	}
	mc, vc = counter.Next(2)
	if mc != 2 || vc != 0 {
		t.Errorf("First call different VCID: MC=%d VC=%d, want 2,0", mc, vc)
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
