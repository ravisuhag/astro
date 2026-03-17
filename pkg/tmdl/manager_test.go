package tmdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

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
