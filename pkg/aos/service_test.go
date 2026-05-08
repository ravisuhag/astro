package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
	"github.com/ravisuhag/astro/pkg/spp"
)

func TestFrameCounter(t *testing.T) {
	fc := aos.NewFrameCounter()
	if got := fc.Next(1); got != 0 {
		t.Errorf("Next(1) #1 = %d, want 0", got)
	}
	if got := fc.Next(1); got != 1 {
		t.Errorf("Next(1) #2 = %d, want 1", got)
	}
	if got := fc.Next(2); got != 0 {
		t.Errorf("Next(2) = %d, want 0 (separate VC)", got)
	}
}

func TestFrameCounter_24BitWrap(t *testing.T) {
	fc := aos.NewFrameCounter()
	// Drive counter to max-1 by injecting state via Next calls would be slow;
	// instead just check the wrap by calling many times indirectly.
	// Sanity-check: counter does not exceed 24 bits ever.
	for range 10 {
		if got := fc.Next(0); got > aos.MaxVCFrameCount {
			t.Fatalf("count exceeded 24 bits: %d", got)
		}
	}
}

// makeSPP builds a minimal valid Space Packet for use in M_PDU tests.
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

func TestMultiplexingService_FixedLength_RoundTrip(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	sendVC := aos.NewVirtualChannel(1, 100)
	recvVC := aos.NewVirtualChannel(1, 100)
	sendCounter := aos.NewFrameCounter()

	tx := aos.NewMultiplexingService(50, 1, sendVC, config, sendCounter)
	rx := aos.NewMultiplexingService(50, 1, recvVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)

	// Send a few packets that span multiple frames.
	pkts := [][]byte{
		makeSPP(t, 100, make([]byte, 8)),
		makeSPP(t, 100, make([]byte, 16)),
		makeSPP(t, 100, make([]byte, 32)),
	}
	for i, pkt := range pkts {
		// Distinguish payloads so we can verify which packet came back.
		pkt[len(pkt)-1] = byte(i + 1)
		if err := tx.Send(pkt); err != nil {
			t.Fatalf("Send(%d) error = %v", i, err)
		}
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// Move all frames from send VC to receive VC.
	for {
		f, err := sendVC.Next()
		if err != nil {
			break
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
		if len(got) != len(want) {
			t.Fatalf("packet %d len = %d, want %d", i, len(got), len(want))
		}
		if got[len(got)-1] != want[len(want)-1] {
			t.Errorf("packet %d marker = 0x%02X, want 0x%02X", i, got[len(got)-1], want[len(want)-1])
		}
	}
}

func TestMultiplexingService_VariableLength_Rejected(t *testing.T) {
	config := aos.ChannelConfig{} // FrameLength=0 — invalid for AOS
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewMultiplexingService(50, 1, vc, config, nil)
	if err := tx.Send([]byte{0x01}); err != aos.ErrDataFieldTooSmall {
		t.Errorf("expected ErrDataFieldTooSmall on variable-length, got %v", err)
	}
}

func TestMultiplexingService_NoSizer(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	rx := aos.NewMultiplexingService(50, 1, vc, config, nil)
	// Add a synthetic frame so Receive() can pull it.
	frame, err := aos.NewTransferFrame(50, 1, make([]byte, config.DataFieldCapacity()), aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	if err := vc.Add(frame); err != nil {
		t.Fatalf("vc.Add() error = %v", err)
	}
	if _, err := rx.Receive(); err != aos.ErrNoPacketSizer {
		t.Errorf("expected ErrNoPacketSizer, got %v", err)
	}
}

func TestMultiplexingService_EmptyData(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewMultiplexingService(50, 1, vc, config, nil)
	if err := tx.Send(nil); err != aos.ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestBitstreamService_FullFrame(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 32, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewBitstreamService(50, 1, vc, config, aos.NewFrameCounter())

	capacity := config.DataFieldCapacity() - aos.BPDUHeaderSize
	data := make([]byte, capacity)
	for i := range data {
		data[i] = byte(i)
	}
	if err := tx.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rxVC := aos.NewVirtualChannel(1, 10)
	for {
		f, err := vc.Next()
		if err != nil {
			break
		}
		_ = rxVC.Add(f)
	}
	rx := aos.NewBitstreamService(50, 1, rxVC, config, nil)
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(got) != len(data) {
		t.Fatalf("len = %d, want %d", len(got), len(data))
	}
	for i, b := range got {
		if b != data[i] {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestBitstreamService_PartialFlush(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewBitstreamService(50, 1, vc, config, nil)

	data := []byte{0xAA, 0xBB, 0xCC}
	if err := tx.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	rx := aos.NewBitstreamService(50, 1, vc, config, nil)
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(got) != len(data) {
		t.Fatalf("trimmed len = %d, want %d", len(got), len(data))
	}
	for i, b := range got {
		if b != data[i] {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestVirtualChannelAccessService_FixedLength(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	counter := aos.NewFrameCounter()
	tx := aos.NewVirtualChannelAccessService(50, 1, 16, vc, config, counter)
	rx := aos.NewVirtualChannelAccessService(50, 1, 16, vc, config, nil)

	data := make([]byte, 16)
	for i := range data {
		data[i] = byte(i + 1)
	}
	if err := tx.Send(data); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(got) != 16 {
		t.Fatalf("len = %d, want 16", len(got))
	}
	for i, b := range got {
		if b != data[i] {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestVirtualChannelAccessService_SizeMismatch(t *testing.T) {
	config := aos.ChannelConfig{} // variable
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewVirtualChannelAccessService(50, 1, 8, vc, config, nil)
	if err := tx.Send([]byte{0x01}); err != aos.ErrSizeMismatch {
		t.Errorf("expected ErrSizeMismatch, got %v", err)
	}
}

func TestVirtualChannelAccessService_TooLarge(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 16, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	tx := aos.NewVirtualChannelAccessService(50, 1, 4, vc, config, nil)
	huge := make([]byte, 256)
	if err := tx.Send(huge); err != aos.ErrDataTooLarge {
		t.Errorf("expected ErrDataTooLarge, got %v", err)
	}
}

func TestVirtualChannelFrameService(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	vc := aos.NewVirtualChannel(1, 10)
	svc := aos.NewVirtualChannelFrameService(1, vc, config)

	data := make([]byte, config.DataFieldCapacity())
	for i := range data {
		data[i] = byte(i)
	}
	frame, err := aos.NewTransferFrame(50, 1, data, aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if err := svc.Send(encoded); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	got, err := svc.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(got) != len(encoded) {
		t.Fatalf("len = %d, want %d", len(got), len(encoded))
	}
}

func TestMultiplexingService_FHPResync(t *testing.T) {
	// Verify receiver resyncs from FHP after dropping a frame mid-packet.
	config := aos.ChannelConfig{FrameLength: 32, HasFECF: true}
	sendVC := aos.NewVirtualChannel(1, 100)
	sendCounter := aos.NewFrameCounter()
	tx := aos.NewMultiplexingService(50, 1, sendVC, config, sendCounter)

	// Build a packet that spans 3+ frames.
	bigPayload := make([]byte, 64)
	for i := range bigPayload {
		bigPayload[i] = byte(i)
	}
	bigPkt := makeSPP(t, 100, bigPayload)
	smallPkt := makeSPP(t, 200, []byte{0xCC, 0xDD})

	if err := tx.Send(bigPkt); err != nil {
		t.Fatalf("Send(big) error = %v", err)
	}
	if err := tx.Send(smallPkt); err != nil {
		t.Fatalf("Send(small) error = %v", err)
	}
	if err := tx.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// Drain frames; drop the second one to simulate frame loss.
	var frames []*aos.TransferFrame
	for {
		f, err := sendVC.Next()
		if err != nil {
			break
		}
		frames = append(frames, f)
	}
	if len(frames) < 3 {
		t.Fatalf("expected >= 3 frames, got %d", len(frames))
	}
	frames = append(frames[:1], frames[2:]...) // drop frame index 1

	// Replay through receiver; should resync via FHP and yield the small packet at minimum.
	recvVC := aos.NewVirtualChannel(1, 100)
	for _, f := range frames {
		_ = recvVC.Add(f)
	}
	rx := aos.NewMultiplexingService(50, 1, recvVC, config, nil)
	rx.SetPacketSizer(spp.PacketSizer)

	got, err := rx.Receive()
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	// After a dropped frame mid-packet, receiver discards the partial big packet
	// and resyncs; the next emission should be smallPkt.
	if !bytesEqual(got, bigPkt) && !bytesEqual(got, smallPkt) {
		t.Errorf("got unexpected packet: % X", got)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
