package usdl_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/usdl"
)

func TestMAPPacketService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true} // FrameLength=0 -> variable-length mode
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)

	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("Receive() = %x, want %x", got, data)
	}
}

func TestMAPPacketService_VariableLength_UsesRule111(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)

	if err := svc.Send([]byte{0x01, 0x02}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	frame, err := vc.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	// Variable-length TFDZs never use the fixed-length rule '000'.
	if frame.DataFieldHeader.ConstructionRule != usdl.RuleNoSegmentation {
		t.Errorf("rule = %d, want %d (No Segmentation)",
			frame.DataFieldHeader.ConstructionRule, usdl.RuleNoSegmentation)
	}
	// Ordinary traffic uses the full, non-truncated header.
	if frame.Header.EndOfFPH {
		t.Error("ordinary frame must not use the truncated header")
	}
	if frame.DataFieldHeader.UPID != usdl.UPIDSpacePackets {
		t.Errorf("UPID = %d, want %d", frame.DataFieldHeader.UPID, usdl.UPIDSpacePackets)
	}
}

func TestMAPPacketService_EmptyData(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)

	if err := svc.Send([]byte{}); err != usdl.ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestMAPPacketService_VCFCount(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true, VCFCountLen: 2}
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, counter)

	if err := svc.Send([]byte{0x01}); err != nil {
		t.Fatalf("Send(1) error = %v", err)
	}
	if err := svc.Send([]byte{0x02}); err != nil {
		t.Fatalf("Send(2) error = %v", err)
	}

	f1, _ := vc.Next()
	f2, _ := vc.Next()

	if f1.Header.VCFCountLen != 2 || f2.Header.VCFCountLen != 2 {
		t.Fatalf("VCFCountLen = %d, %d; want 2, 2", f1.Header.VCFCountLen, f2.Header.VCFCountLen)
	}
	if f1.Header.VCFCount != 0 {
		t.Errorf("frame1 VCF count = %d, want 0", f1.Header.VCFCount)
	}
	if f2.Header.VCFCount != 1 {
		t.Errorf("frame2 VCF count = %d, want 1", f2.Header.VCFCount)
	}
}

// makeSPP builds a minimal valid Space Packet for MAPP fixed-length tests.
func makeSPP(t *testing.T, apid uint16, payload []byte) []byte {
	t.Helper()
	pkt, err := spp.NewSpacePacket(apid, spp.PacketTypeTM, payload)
	if err != nil {
		t.Fatalf("NewSpacePacket() error = %v", err)
	}
	encoded, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	return encoded
}

func TestMAPPacketService_FixedLength_RoundTrip(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true, VCFCountLen: 2}
	sendVC := usdl.NewVirtualChannel(1, 100)
	recvVC := usdl.NewVirtualChannel(1, 100)
	counter := usdl.NewFrameCounter()

	tx := usdl.NewMAPPacketService(100, 1, 0, sendVC, config, counter)
	rx := usdl.NewMAPPacketService(100, 1, 0, recvVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)

	pkts := [][]byte{
		makeSPP(t, 100, make([]byte, 8)),
		makeSPP(t, 100, make([]byte, 40)), // spans frames
		makeSPP(t, 100, make([]byte, 16)),
	}
	for i, pkt := range pkts {
		pkt[len(pkt)-1] = byte(i + 1)
		if err := tx.Send(pkt); err != nil {
			t.Fatalf("Send(%d) error = %v", i, err)
		}
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	for {
		f, err := sendVC.Next()
		if err != nil {
			break
		}
		// Fixed-length packet frames use rule '000' and the full header.
		if f.DataFieldHeader.ConstructionRule != usdl.RulePacketsSpanning {
			t.Fatalf("rule = %d, want %d", f.DataFieldHeader.ConstructionRule, usdl.RulePacketsSpanning)
		}
		if f.Header.EndOfFPH {
			t.Fatal("fixed-length frame must not use the truncated header")
		}
		encoded, err := f.Encode()
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		if len(encoded) != config.FrameLength {
			t.Fatalf("frame length = %d, want %d", len(encoded), config.FrameLength)
		}
		if err := recvVC.Add(f); err != nil {
			t.Fatalf("recvVC.Add() error = %v", err)
		}
	}

	for i, want := range pkts {
		got, err := rx.Receive()
		if err != nil {
			t.Fatalf("Receive(%d) error = %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("packet %d mismatch:\n got %x\nwant %x", i, got, want)
		}
	}
	// The idle fill must not surface as user data.
	if extra, err := rx.Receive(); err == nil {
		t.Fatalf("idle fill delivered as user data: %x", extra)
	}
}

func TestMAPPacketService_MAPDemultiplexing(t *testing.T) {
	// Two MAP channels sharing one VC: each service must receive its own
	// MAP's traffic, and pulling one MAP's frame past the other's must not
	// discard the other's data (clause 4.3 MAP demultiplexing).
	config := usdl.ChannelConfig{HasFECF: true}
	vc := usdl.NewVirtualChannel(1, 100)

	txA := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)
	txB := usdl.NewMAPPacketService(100, 1, 5, vc, config, nil)

	if err := txB.Send([]byte{0xB0, 0xB1}); err != nil {
		t.Fatalf("Send(B) error = %v", err)
	}
	if err := txA.Send([]byte{0xA0, 0xA1}); err != nil {
		t.Fatalf("Send(A) error = %v", err)
	}

	// MAP 0 receives first: its frame sits behind MAP 5's in the shared
	// VC buffer, so the demux must hold the MAP 5 frame for its service.
	rxA := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)
	rxB := usdl.NewMAPPacketService(100, 1, 5, vc, config, nil)

	got, err := rxA.Receive()
	if err != nil {
		t.Fatalf("Receive(A) error = %v", err)
	}
	if !bytes.Equal(got, []byte{0xA0, 0xA1}) {
		t.Errorf("MAP 0 received %x, want a0a1 (MAP 5 frame must be filtered)", got)
	}

	gotB, err := rxB.Receive()
	if err != nil {
		t.Fatalf("Receive(B) error = %v (frame discarded by the other MAP's service?)", err)
	}
	if !bytes.Equal(gotB, []byte{0xB0, 0xB1}) {
		t.Errorf("MAP 5 received %x, want b0b1", gotB)
	}
}

func TestMAPAccessService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, counter)

	data := make([]byte, 8)
	for i := range data {
		data[i] = byte(i)
	}

	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("Receive() = %x, want %x", got, data)
	}
}

func TestMAPAccessService_SizeMismatch(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, nil)

	if err := svc.Send([]byte{0x01}); err != usdl.ErrSizeMismatch {
		t.Errorf("expected ErrSizeMismatch, got %v", err)
	}
}

func TestMAPAccessService_FixedLength_SingleFrame(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 48, HasFECF: true}
	vc := usdl.NewVirtualChannel(1, 10)
	counter := usdl.NewFrameCounter()
	tx := usdl.NewMAPAccessService(100, 1, 0, 16, vc, config, counter)

	data := make([]byte, 16)
	for i := range data {
		data[i] = byte(i + 0x10)
	}
	if err := tx.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	frame, err := vc.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if frame.DataFieldHeader.ConstructionRule != usdl.RuleStartOfSDU {
		t.Errorf("rule = %d, want %d ('001')", frame.DataFieldHeader.ConstructionRule, usdl.RuleStartOfSDU)
	}
	if frame.DataFieldHeader.Pointer != 15 {
		t.Errorf("LVOP = %d, want 15 (last valid octet)", frame.DataFieldHeader.Pointer)
	}
	if frame.Header.EndOfFPH {
		t.Error("fixed-length frame must not use the truncated header")
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != config.FrameLength {
		t.Errorf("frame length = %d, want %d", len(encoded), config.FrameLength)
	}

	// Receive trims to the SDU with no out-of-band size knowledge.
	rxVC := usdl.NewVirtualChannel(1, 10)
	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	_ = rxVC.Add(decoded)
	rx := usdl.NewMAPAccessService(100, 1, 0, 16, rxVC, config, nil)
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("Receive() = %x, want %x", got, data)
	}
}

func TestMAPAccessService_FixedLength_SpanningSDU(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 32, HasFECF: true}
	capacity := config.DataFieldCapacity(3)
	sduSize := capacity*2 + 5 // spans three frames
	vc := usdl.NewVirtualChannel(1, 10)
	tx := usdl.NewMAPAccessService(100, 1, 0, sduSize, vc, config, nil)

	data := make([]byte, sduSize)
	for i := range data {
		data[i] = byte(i)
	}
	if err := tx.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rxVC := usdl.NewVirtualChannel(1, 10)
	rules := []uint8{}
	for {
		f, err := vc.Next()
		if err != nil {
			break
		}
		rules = append(rules, f.DataFieldHeader.ConstructionRule)
		_ = rxVC.Add(f)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(rules))
	}
	want := []uint8{usdl.RuleStartOfSDU, usdl.RuleContinuingSDU, usdl.RuleContinuingSDU}
	for i := range rules {
		if rules[i] != want[i] {
			t.Errorf("frame %d rule = %d, want %d", i, rules[i], want[i])
		}
	}

	rx := usdl.NewMAPAccessService(100, 1, 0, sduSize, rxVC, config, nil)
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("reassembled SDU mismatch: len %d, want %d", len(got), len(data))
	}
}

func TestMAPOctetStreamService_VariableLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	counter := usdl.NewFrameCounter()

	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, counter)

	data := []byte{0xAA, 0xBB, 0xCC}
	if err := svc.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("Receive() = %x, want %x", got, data)
	}
}

func TestMAPOctetStreamService_UsesRule011(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, nil)

	if err := svc.Send([]byte{0x01}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	frame, _ := vc.Next()
	if frame.DataFieldHeader.ConstructionRule != usdl.RuleOctetStream {
		t.Errorf("rule = %d, want %d ('011')",
			frame.DataFieldHeader.ConstructionRule, usdl.RuleOctetStream)
	}
	if frame.DataFieldHeader.UPID != usdl.UPIDUserOctetStream {
		t.Errorf("UPID = %d, want %d", frame.DataFieldHeader.UPID, usdl.UPIDUserOctetStream)
	}
}

func TestMAPOctetStreamService_RejectsFixedLength(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true}
	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, nil)

	// Clause 4.2.4.1 note 1: octet streams cannot ride fixed-length frames.
	if err := svc.Send([]byte{0x01}); err != usdl.ErrOctetStreamFixedLength {
		t.Errorf("expected ErrOctetStreamFixedLength, got %v", err)
	}
}

func TestMAPOctetStreamService_EmptyData(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, nil)

	if err := svc.Send([]byte{}); err != usdl.ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestFrameCounter(t *testing.T) {
	fc := usdl.NewFrameCounter()

	if got := fc.Next(1, false); got != 0 {
		t.Errorf("Next(1) #1 = %d, want 0", got)
	}
	if got := fc.Next(1, false); got != 1 {
		t.Errorf("Next(1) #2 = %d, want 1", got)
	}
	if got := fc.Next(2, false); got != 0 {
		t.Errorf("Next(2) = %d, want 0 (separate VC)", got)
	}
	// Clause 4.1.2.12.4-12.5: the expedited count of a VC is independent of its
	// sequence-controlled count.
	if got := fc.Next(1, true); got != 0 {
		t.Errorf("Next(1, expedited) = %d, want 0 (separate QoS counter)", got)
	}
	if got := fc.Next(1, false); got != 2 {
		t.Errorf("Next(1) #3 = %d, want 2 (expedited count must not advance it)", got)
	}
}

func TestMAPAccessService_Flush(t *testing.T) {
	vc := usdl.NewVirtualChannel(1, 100)
	config := usdl.ChannelConfig{HasFECF: true}
	svc := usdl.NewMAPAccessService(100, 1, 0, 8, vc, config, nil)

	if err := svc.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
}

// Clause 4.1.4.2.4.3-4.4.4: the FHP points at the first packet header starting
// in the TFDZ; 'all ones' only when NO packet starts there. A flush frame
// that carries just a spanning packet's tail still has the appended
// Encapsulation Idle Packet starting in it, so the FHP must point at the
// idle packet, and a receiver that lost the preceding frame must be able
// to resynchronize on the flush frame.
func TestMAPPacketService_FlushFHP_ResyncAfterLoss(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true, VCFCountLen: 2}
	capacity := config.DataFieldCapacity(3)
	sendVC := usdl.NewVirtualChannel(1, 100)
	counter := usdl.NewFrameCounter()
	tx := usdl.NewMAPPacketService(100, 1, 0, sendVC, config, counter)

	// pkt1 spans two frames: capacity octets in frame 1, a 10-octet tail
	// in the flush frame.
	pkt1 := makeSPP(t, 100, make([]byte, capacity+10-6))
	if err := tx.Send(pkt1); err != nil {
		t.Fatalf("Send(pkt1) error = %v", err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush(1) error = %v", err)
	}
	pkt2 := makeSPP(t, 200, []byte{0xCA, 0xFE})
	if err := tx.Send(pkt2); err != nil {
		t.Fatalf("Send(pkt2) error = %v", err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush(2) error = %v", err)
	}

	frame1, _ := sendVC.Next()
	frame2, _ := sendVC.Next()
	frame3, _ := sendVC.Next()
	if frame1.DataFieldHeader.Pointer != 0 {
		t.Errorf("frame1 FHP = 0x%04X, want 0 (pkt1 starts at octet 0)", frame1.DataFieldHeader.Pointer)
	}
	if frame2.DataFieldHeader.Pointer != 10 {
		t.Errorf("frame2 FHP = 0x%04X, want 10 (the idle packet behind pkt1's tail)",
			frame2.DataFieldHeader.Pointer)
	}
	if frame3.DataFieldHeader.Pointer != 0 {
		t.Errorf("frame3 FHP = 0x%04X, want 0", frame3.DataFieldHeader.Pointer)
	}

	// Lose frame 1. The receiver must resynchronize at the flush frame's
	// FHP and still deliver pkt2 intact.
	recvVC := usdl.NewVirtualChannel(1, 100)
	_ = recvVC.Add(frame2)
	_ = recvVC.Add(frame3)
	rx := usdl.NewMAPPacketService(100, 1, 0, recvVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)

	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if !bytes.Equal(got, pkt2) {
		t.Fatalf("resynced packet mismatch:\n got %x\nwant %x", got, pkt2)
	}
	if extra, err := rx.Receive(); err == nil {
		t.Fatalf("unexpected extra packet: %x", extra)
	}
}

// User data that happens to coincide with the project idle pattern must
// not be dropped: rule '000' fill is exactly delimited by Encapsulation
// Idle Packets, so no pattern heuristic may run on user data.
func TestMAPPacketService_PatternLikePayloadDelivered(t *testing.T) {
	config := usdl.ChannelConfig{FrameLength: 64, HasFECF: true, VCFCountLen: 2}
	capacity := config.DataFieldCapacity(3)
	sendVC := usdl.NewVirtualChannel(1, 100)
	tx := usdl.NewMAPPacketService(100, 1, 0, sendVC, config, usdl.NewFrameCounter())

	// Two fixed-size packets exactly fill one frame; the second is all
	// 0x55, byte-identical to the default idle pattern.
	half := capacity / 2
	pkt1 := bytes.Repeat([]byte{0x11}, half)
	pkt2 := bytes.Repeat([]byte{usdl.DefaultIdleFill}, capacity-half)
	if err := tx.Send(pkt1); err != nil {
		t.Fatalf("Send(pkt1) error = %v", err)
	}
	if err := tx.Send(pkt2); err != nil {
		t.Fatalf("Send(pkt2) error = %v", err)
	}

	rx := usdl.NewMAPPacketService(100, 1, 0, sendVC, config, nil)
	sizes := []int{len(pkt1), len(pkt2)}
	i := 0
	rx.SetPacketSizer(func(data []byte) int {
		if i >= len(sizes) || len(data) < sizes[i] {
			return -1
		}
		n := sizes[i]
		i++
		return n
	})

	got1, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive(1) error = %v", err)
	}
	if !bytes.Equal(got1, pkt1) {
		t.Fatalf("packet 1 mismatch: %x", got1)
	}
	got2, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive(2) error = %v (pattern-like payload dropped?)", err)
	}
	if !bytes.Equal(got2, pkt2) {
		t.Fatalf("packet 2 mismatch: %x", got2)
	}
}

// A channel configured with HasOCF requires an OCF supplier; the OCF is
// then carried on every emitted frame (clause 4.1.5).
func TestMAPServices_OCFSupplier(t *testing.T) {
	config := usdl.ChannelConfig{HasFECF: true, HasOCF: true}
	clcw := []byte{0xC1, 0xC2, 0xC3, 0xC4}

	t.Run("MAPP", func(t *testing.T) {
		vc := usdl.NewVirtualChannel(1, 10)
		svc := usdl.NewMAPPacketService(100, 1, 0, vc, config, nil)
		if err := svc.Send([]byte{0x01}); err != usdl.ErrNoOCFSupplier {
			t.Fatalf("expected ErrNoOCFSupplier, got %v", err)
		}
		svc.SetOCFSupplier(func() []byte { return clcw })
		if err := svc.Send([]byte{0x01}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		frame, _ := vc.Next()
		if !bytes.Equal(frame.OCF, clcw) {
			t.Errorf("OCF = %x, want %x", frame.OCF, clcw)
		}
	})

	t.Run("MAPA", func(t *testing.T) {
		vc := usdl.NewVirtualChannel(1, 10)
		svc := usdl.NewMAPAccessService(100, 1, 0, 4, vc, config, nil)
		if err := svc.Send([]byte{1, 2, 3, 4}); err != usdl.ErrNoOCFSupplier {
			t.Fatalf("expected ErrNoOCFSupplier, got %v", err)
		}
		svc.SetOCFSupplier(func() []byte { return clcw })
		if err := svc.Send([]byte{1, 2, 3, 4}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		frame, _ := vc.Next()
		if !bytes.Equal(frame.OCF, clcw) {
			t.Errorf("OCF = %x, want %x", frame.OCF, clcw)
		}
	})

	t.Run("MAPO", func(t *testing.T) {
		vc := usdl.NewVirtualChannel(1, 10)
		svc := usdl.NewMAPOctetStreamService(100, 1, 0, vc, config, nil)
		if err := svc.Send([]byte{0x01}); err != usdl.ErrNoOCFSupplier {
			t.Fatalf("expected ErrNoOCFSupplier, got %v", err)
		}
		svc.SetOCFSupplier(func() []byte { return clcw })
		if err := svc.Send([]byte{0x01}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		frame, _ := vc.Next()
		if !bytes.Equal(frame.OCF, clcw) {
			t.Errorf("OCF = %x, want %x", frame.OCF, clcw)
		}
	})
}
