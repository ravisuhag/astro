package tcdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcdl"
)

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
	_ = svc.Send([]byte("bypass data"))
	frame, _ := vc.Next()
	if frame.Header.BypassFlag != 1 {
		t.Error("expected BypassFlag=1 for bypass service")
	}
}

func TestMAPPacketService_MultiplePacketsPerFrame(t *testing.T) {
	// A compliant sender may block several packets into one frame data
	// field. Receive must delimit them with the PacketSizer and hand them
	// out one at a time.
	vc := tcdl.NewVirtualChannel(1, 100)
	svc := tcdl.NewMAPPacketService(42, 1, 0, false, vc, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	pktA, _ := spp.NewTCPacket(100, []byte("first"))
	encA, _ := pktA.Encode()
	pktB, _ := spp.NewTCPacket(101, []byte("second"))
	encB, _ := pktB.Encode()

	blocked := append(append([]byte(nil), encA...), encB...)
	frame, err := tcdl.NewTCTransferFrame(42, 1, blocked,
		tcdl.WithSegmentHeader(tcdl.SegmentHeader{SequenceFlags: tcdl.SegUnsegmented, MAPID: 0}))
	if err != nil {
		t.Fatal(err)
	}
	if err := vc.Add(frame); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, encA) {
		t.Errorf("packet 1 = %x, want %x", got, encA)
	}
	got, err = svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, encB) {
		t.Errorf("packet 2 = %x, want %x", got, encB)
	}
}

func TestMAPPacketService_SegmentGapReturnsIncompleteSegment(t *testing.T) {
	vc := tcdl.NewVirtualChannel(1, 100)
	svc := tcdl.NewMAPPacketService(42, 1, 0, false, vc, nil)
	svc.SetPacketSizer(spp.PacketSizer)

	addSeg := func(flags uint8, data []byte) {
		frame, err := tcdl.NewTCTransferFrame(42, 1, data,
			tcdl.WithSegmentHeader(tcdl.SegmentHeader{SequenceFlags: flags, MAPID: 0}))
		if err != nil {
			t.Fatal(err)
		}
		if err := vc.Add(frame); err != nil {
			t.Fatal(err)
		}
	}

	pkt, _ := spp.NewTCPacket(100, []byte("complete packet"))
	enc, _ := pkt.Encode()

	// A First segment interrupted by an Unsegmented frame: the partial
	// packet is lost and the receiver must say so.
	addSeg(tcdl.SegFirst, []byte("partial"))
	addSeg(tcdl.SegUnsegmented, enc)

	if _, err := svc.Receive(); !errors.Is(err, tcdl.ErrIncompleteSegment) {
		t.Fatalf("expected ErrIncompleteSegment, got %v", err)
	}
	// The interrupting frame's packet is still delivered afterwards.
	got, err := svc.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, enc) {
		t.Errorf("packet after gap = %x, want %x", got, enc)
	}

	// A Continuation with no First in progress is also a gap.
	addSeg(tcdl.SegContinuation, []byte("orphan"))
	if _, err := svc.Receive(); !errors.Is(err, tcdl.ErrIncompleteSegment) {
		t.Errorf("orphan continuation: expected ErrIncompleteSegment, got %v", err)
	}

	// A Last with no First in progress as well.
	addSeg(tcdl.SegLast, []byte("orphan"))
	if _, err := svc.Receive(); !errors.Is(err, tcdl.ErrIncompleteSegment) {
		t.Errorf("orphan last: expected ErrIncompleteSegment, got %v", err)
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
