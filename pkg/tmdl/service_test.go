package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestVCPService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, nil)

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
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, nil)
	err := svc.Send([]byte{})
	if !errors.Is(err, tmdl.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestVCPService_ReceiveEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, nil)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestVCPService_MultipleSendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(2, 100)
	svc := tmdl.NewVirtualChannelPacketService(500, 2, vc, nil)

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

func TestVCAService_SendReceive(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, nil)

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

func TestVCAService_SendSizeMismatch(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, nil)
	err := svc.Send([]byte("short"))
	if !errors.Is(err, tmdl.ErrSizeMismatch) {
		t.Errorf("Expected ErrSizeMismatch, got %v", err)
	}
}

func TestVCAService_ReceiveEmpty(t *testing.T) {
	vc := tmdl.NewVirtualChannel(1, 100)
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8, vc, nil)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestMasterChannel_AddAndGet(t *testing.T) {
	mc := tmdl.NewMasterChannel(933)
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
	mc := tmdl.NewMasterChannel(933)
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
	mc := tmdl.NewMasterChannel(933)
	// No VCs registered

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
	mc := tmdl.NewMasterChannel(933)
	_, err := mc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoVirtualChannels) {
		t.Errorf("Expected ErrNoVirtualChannels, got %v", err)
	}
}

func TestMasterChannel_MultiplexesSendPath(t *testing.T) {
	mc := tmdl.NewMasterChannel(933)
	vc1 := tmdl.NewVirtualChannel(1, 10)
	vc2 := tmdl.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	svc1 := tmdl.NewVirtualChannelPacketService(933, 1, vc1, nil)
	svc2 := tmdl.NewVirtualChannelPacketService(933, 2, vc2, nil)

	if err := svc1.Send([]byte("from vc1")); err != nil {
		t.Fatalf("Send vc1: %v", err)
	}
	if err := svc2.Send([]byte("from vc2")); err != nil {
		t.Fatalf("Send vc2: %v", err)
	}

	// Mux should serve vc1 first (lower VCID), then vc2
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
	svc := tmdl.NewVirtualChannelPacketService(933, 1, vc, counter)

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
