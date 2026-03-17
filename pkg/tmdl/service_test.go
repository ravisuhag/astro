package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

func TestVCPService_SendReceive(t *testing.T) {
	svc := tmdl.NewVirtualChannelPacketService(933, 1)

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
	svc := tmdl.NewVirtualChannelPacketService(933, 1)
	err := svc.Send([]byte{})
	if !errors.Is(err, tmdl.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
}

func TestVCPService_ReceiveEmpty(t *testing.T) {
	svc := tmdl.NewVirtualChannelPacketService(933, 1)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestVCPService_MultipleSendReceive(t *testing.T) {
	svc := tmdl.NewVirtualChannelPacketService(500, 2)

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
	svc := tmdl.NewVirtualChannelFrameService(1)

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
	svc := tmdl.NewVirtualChannelFrameService(1)
	err := svc.Send([]byte{})
	if !errors.Is(err, tmdl.ErrEmptyData) {
		t.Errorf("Expected ErrEmptyData, got %v", err)
	}
	if err := svc.Send([]byte{0xFF}); err == nil {
		t.Error("Expected error for invalid frame bytes")
	}
}

func TestVCFService_ReceiveEmpty(t *testing.T) {
	svc := tmdl.NewVirtualChannelFrameService(1)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestVCAService_SendReceive(t *testing.T) {
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8)

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
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8)
	err := svc.Send([]byte("short"))
	if !errors.Is(err, tmdl.ErrSizeMismatch) {
		t.Errorf("Expected ErrSizeMismatch, got %v", err)
	}
}

func TestVCAService_ReceiveEmpty(t *testing.T) {
	svc := tmdl.NewVirtualChannelAccessService(933, 1, 8)
	_, err := svc.Receive()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}

func TestMasterChannelService_AddAndGet(t *testing.T) {
	mc := tmdl.NewMasterChannelService(933)

	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	if err := mc.AddFrame(frame); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	if !mc.HasFrames() {
		t.Error("Expected HasFrames to be true")
	}

	got, err := mc.GetNextFrame()
	if err != nil {
		t.Fatalf("GetNextFrame failed: %v", err)
	}

	if got != frame {
		t.Error("Expected same frame back")
	}

	if mc.HasFrames() {
		t.Error("Expected HasFrames to be false after draining")
	}
}

func TestMasterChannelService_SCIDMismatch(t *testing.T) {
	mc := tmdl.NewMasterChannelService(933)

	frame, err := tmdl.NewTMTransferFrame(500, 1, []byte("data"), nil, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	err = mc.AddFrame(frame)
	if !errors.Is(err, tmdl.ErrSCIDMismatch) {
		t.Errorf("Expected ErrSCIDMismatch, got %v", err)
	}
}

func TestMasterChannelService_GetEmpty(t *testing.T) {
	mc := tmdl.NewMasterChannelService(933)
	_, err := mc.GetNextFrame()
	if !errors.Is(err, tmdl.ErrNoFramesAvailable) {
		t.Errorf("Expected ErrNoFramesAvailable, got %v", err)
	}
}
