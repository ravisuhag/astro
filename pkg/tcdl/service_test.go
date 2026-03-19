package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcdl"
)

func TestMasterChannel_Routing(t *testing.T) {
	mc := tcdl.NewMasterChannel(42)
	vc1 := tcdl.NewVirtualChannel(1, 10)
	vc2 := tcdl.NewVirtualChannel(2, 10)
	mc.AddVirtualChannel(vc1, 1)
	mc.AddVirtualChannel(vc2, 1)

	f1, _ := tcdl.NewTCTransferFrame(42, 1, []byte("to-vc1"))
	f2, _ := tcdl.NewTCTransferFrame(42, 2, []byte("to-vc2"))
	mc.AddFrame(f1)
	mc.AddFrame(f2)

	got1, _ := vc1.GetNextFrame()
	if !bytes.Equal(got1.DataField, []byte("to-vc1")) {
		t.Errorf("VC1 got %q", got1.DataField)
	}
	got2, _ := vc2.GetNextFrame()
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

func TestMAPPacketService_SmallPacket(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 100)
	counter := tcdl.NewFrameCounter()
	svc := tcdl.NewMAPPacketService(42, 1, 0, false, vc, counter)
	svc.SetPacketSizer(spp.PacketSizer)

	pkt, _ := spp.NewTCPacket(100, []byte("set mode"))
	encoded, _ := pkt.Encode()

	if err := svc.Send(encoded); err != nil {
		t.Fatal(err)
	}
	if vc.Len() != 1 {
		t.Fatalf("expected 1 frame, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received, encoded) {
		t.Error("received data differs from sent")
	}
}

func TestMAPPacketService_LargePacket_Segmentation(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 100)
	svc := tcdl.NewMAPPacketService(42, 1, 0, false, vc, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	bigPayload := make([]byte, 1200)
	for i := range bigPayload {
		bigPayload[i] = byte(i & 0xFF)
	}
	pkt, _ := spp.NewTCPacket(100, bigPayload)
	encoded, _ := pkt.Encode()

	if err := svc.Send(encoded); err != nil {
		t.Fatal(err)
	}
	if vc.Len() < 2 {
		t.Fatalf("expected multiple frames for large packet, got %d", vc.Len())
	}

	received, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received, encoded) {
		t.Errorf("reassembled data differs: got %d bytes, want %d", len(received), len(encoded))
	}
}

func TestMAPPacketService_Bypass(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 100)
	svc := tcdl.NewMAPPacketService(42, 1, 0, true, vc, nil)
	svc.Send([]byte("bypass data"))
	frame, _ := vc.GetNextFrame()
	if frame.Header.BypassFlag != 1 {
		t.Error("expected BypassFlag=1 for bypass service")
	}
}

func TestFrameCounter(t *testing.T) {
	fc := tcdl.NewFrameCounter()
	n := fc.Next(1)
	if n != 0 {
		t.Errorf("first = %d, want 0", n)
	}
	n = fc.Next(1)
	if n != 1 {
		t.Errorf("second = %d, want 1", n)
	}
	n = fc.Next(2)
	if n != 0 {
		t.Errorf("different VC = %d, want 0", n)
	}
}
