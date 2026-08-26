package spp_test

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"sync"
	"testing"

	spp2 "github.com/ravisuhag/astro/pkg/spp"
)

func TestServiceSendReceivePacket(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewTMPacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create packet: %v", err)
	}

	if err := svc.SendPacket(packet); err != nil {
		t.Fatalf("SendPacket failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}

	if received.PrimaryHeader.APID != packet.PrimaryHeader.APID {
		t.Errorf("APID mismatch. Got %d, want %d", received.PrimaryHeader.APID, packet.PrimaryHeader.APID)
	}
	if received.PrimaryHeader.SequenceCount != 0 {
		t.Errorf("Expected sequence count 0, got %d", received.PrimaryHeader.SequenceCount)
	}
	if !bytes.Equal(packet.UserData, received.UserData) {
		t.Errorf("User data mismatch. Got %v, want %v", received.UserData, packet.UserData)
	}
}

func TestServiceSequenceCounting(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	// Send 3 packets on APID 100, expect sequence counts 0, 1, 2
	for range 3 {
		packet, err := spp2.NewTMPacket(100, []byte{0x01})
		if err != nil {
			t.Fatalf("Failed to create packet: %v", err)
		}
		if err := svc.SendPacket(packet); err != nil {
			t.Fatalf("SendPacket failed: %v", err)
		}
	}

	// Send 2 packets on APID 200, expect sequence counts 0, 1
	for range 2 {
		if err := svc.SendBytes(200, []byte{0x02}); err != nil {
			t.Fatalf("SendBytes failed: %v", err)
		}
	}

	// Verify APID 100 sequence counts
	for i := range 3 {
		received, err := svc.ReceivePacket()
		if err != nil {
			t.Fatalf("ReceivePacket failed: %v", err)
		}
		if received.PrimaryHeader.SequenceCount != uint16(i) {
			t.Errorf("APID 100 packet %d: sequence count = %d, want %d",
				i, received.PrimaryHeader.SequenceCount, i)
		}
	}

	// Verify APID 200 sequence counts (independent from APID 100)
	for i := range 2 {
		received, err := svc.ReceivePacket()
		if err != nil {
			t.Fatalf("ReceivePacket failed: %v", err)
		}
		if received.PrimaryHeader.SequenceCount != uint16(i) {
			t.Errorf("APID 200 packet %d: sequence count = %d, want %d",
				i, received.PrimaryHeader.SequenceCount, i)
		}
	}
}

func TestServiceSequenceCountWrap(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	// Send 16384 packets (0..16383), then one more that should wrap to 0
	for range 16385 {
		packet, err := spp2.NewTMPacket(1, []byte{0x01})
		if err != nil {
			t.Fatalf("Failed to create packet: %v", err)
		}
		if err := svc.SendPacket(packet); err != nil {
			t.Fatalf("SendPacket failed: %v", err)
		}
	}

	// Discard first 16384 packets
	for i := range 16384 {
		if _, err := svc.ReceivePacket(); err != nil {
			t.Fatalf("ReceivePacket failed at %d: %v", i, err)
		}
	}

	// The 16385th packet should have wrapped to 0
	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if received.PrimaryHeader.SequenceCount != 0 {
		t.Errorf("Expected wrapped sequence count 0, got %d", received.PrimaryHeader.SequenceCount)
	}
}

func TestServiceSendPacketNil(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{})

	if err := svc.SendPacket(nil); err == nil {
		t.Error("Expected error when sending nil packet")
	}
}

func TestServiceSendReceiveBytes(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	data := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	if err := svc.SendBytes(200, data); err != nil {
		t.Fatalf("SendBytes failed: %v", err)
	}

	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatalf("ReceiveBytes failed: %v", err)
	}

	if ind.APID != 200 {
		t.Errorf("APID mismatch. Got %d, want 200", ind.APID)
	}
	if !bytes.Equal(ind.Data, data) {
		t.Errorf("Data mismatch. Got %v, want %v", ind.Data, data)
	}
	if ind.SecondaryHeaderIndicator {
		t.Error("Secondary Header Indicator set on a packet without one")
	}
	if ind.DataLoss {
		t.Errorf("Data Loss Indicator set on a continuous stream (%d lost)", ind.PacketsLost)
	}
}

func TestServiceSendReceiveBytesWithErrorControl(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:   spp2.PacketTypeTM,
		ErrorControl: true,
	})

	data := []byte{0xCA, 0xFE}

	// CRC is auto-computed during encode
	if err := svc.SendBytes(100, data, spp2.WithSendErrorControl()); err != nil {
		t.Fatalf("SendBytes failed: %v", err)
	}

	// Receive — Service should validate CRC
	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatalf("ReceiveBytes failed: %v", err)
	}
	if ind.APID != 100 {
		t.Errorf("APID mismatch. Got %d, want 100", ind.APID)
	}
	if !bytes.Equal(ind.Data, data) {
		t.Errorf("Data mismatch. Got %v, want %v", ind.Data, data)
	}
}

func TestServiceSendBytesWithSecondaryHeader(t *testing.T) {
	var buf bytes.Buffer
	sh := &testSecondaryHeader{Timestamp: 0x0102030405060708}
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:         spp2.PacketTypeTC,
		NewSecondaryHeader: func() spp2.SecondaryHeader { return &testSecondaryHeader{} },
	})

	data := []byte{0xDE, 0xAD}
	if err := svc.SendBytes(42, data, spp2.WithSendSecondaryHeader(sh)); err != nil {
		t.Fatalf("SendBytes with secondary header failed: %v", err)
	}

	packet, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}

	if packet.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Error("Expected secondary header flag to be set")
	}
	if packet.PrimaryHeader.APID != 42 {
		t.Errorf("APID mismatch. Got %d, want 42", packet.PrimaryHeader.APID)
	}
	if packet.PrimaryHeader.Type != spp2.PacketTypeTC {
		t.Errorf("Packet type mismatch. Got %d, want TC", packet.PrimaryHeader.Type)
	}
	if !bytes.Equal(packet.UserData, data) {
		t.Errorf("User data mismatch. Got %v, want %v", packet.UserData, data)
	}
}

func TestServiceSendBytesInvalidAPID(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	if err := svc.SendBytes(3000, []byte{0x01}); err == nil {
		t.Error("Expected error for invalid APID")
	}
}

func TestServiceMaxPacketLength(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:      spp2.PacketTypeTM,
		MaxPacketLength: 10, // very small limit
	})

	// 6 byte header + 5 bytes data = 11 > 10
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := svc.SendBytes(1, data); err == nil {
		t.Error("Expected error for packet exceeding max length")
	}
}

func TestServiceDefaultMaxPacketLength(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{})

	// Should not panic or error with default config
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestServiceSendBytesRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	// Send multiple packets, receive in order
	payloads := []struct {
		apid uint16
		data []byte
	}{
		{10, []byte{0x01}},
		{20, []byte{0x02, 0x03}},
		{30, []byte{0x04, 0x05, 0x06}},
	}

	for _, p := range payloads {
		if err := svc.SendBytes(p.apid, p.data); err != nil {
			t.Fatalf("SendBytes(apid=%d) failed: %v", p.apid, err)
		}
	}

	for _, p := range payloads {
		ind, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatalf("ReceiveBytes failed: %v", err)
		}
		if ind.APID != p.apid {
			t.Errorf("APID mismatch. Got %d, want %d", ind.APID, p.apid)
		}
		if !bytes.Equal(ind.Data, p.data) {
			t.Errorf("Data mismatch for APID %d. Got %v, want %v", p.apid, ind.Data, p.data)
		}
	}
}

// --- Service with Segmented Packets ---

func TestServiceWithSegmentedPackets(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	flags := []uint8{
		spp2.SeqFlagFirstSegment,
		spp2.SeqFlagContinuation,
		spp2.SeqFlagContinuation,
		spp2.SeqFlagLastSegment,
	}

	for _, flag := range flags {
		pkt, err := spp2.NewTMPacket(100, []byte{0x01}, spp2.WithSequenceFlags(flag))
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.SendPacket(pkt); err != nil {
			t.Fatal(err)
		}
	}

	for i, expectedFlag := range flags {
		received, err := svc.ReceivePacket()
		if err != nil {
			t.Fatal(err)
		}
		if received.PrimaryHeader.SequenceFlags != expectedFlag {
			t.Errorf("Packet %d: flags = %d, want %d", i, received.PrimaryHeader.SequenceFlags, expectedFlag)
		}
		if received.PrimaryHeader.SequenceCount != uint16(i) {
			t.Errorf("Packet %d: seq = %d, want %d", i, received.PrimaryHeader.SequenceCount, i)
		}
	}
}

// --- Service with Boundary APIDs ---

func TestServiceWithBoundaryAPIDs(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
	})

	// APID 0
	if err := svc.SendBytes(0, []byte{0x01}); err != nil {
		t.Fatalf("APID 0 should be valid: %v", err)
	}
	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.APID != 0 {
		t.Errorf("Expected APID 0, got %d", ind.APID)
	}

	// APID 2047 (max, also idle APID)
	if err := svc.SendBytes(2047, []byte{0x02}); err != nil {
		t.Fatalf("APID 2047 should be valid: %v", err)
	}
	ind, err = svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.APID != 2047 {
		t.Errorf("Expected APID 2047, got %d", ind.APID)
	}

	// Verify independent sequence counting for both
	for range 3 {
		_ = svc.SendBytes(0, []byte{0x01})
	}
	for range 2 {
		_ = svc.SendBytes(2047, []byte{0x02})
	}

	for i := 1; i <= 3; i++ {
		pkt, _ := svc.ReceivePacket()
		if pkt.PrimaryHeader.APID != 0 || pkt.PrimaryHeader.SequenceCount != uint16(i) {
			t.Errorf("APID 0 packet %d: seq = %d, want %d", i, pkt.PrimaryHeader.SequenceCount, i)
		}
	}

	for i := 1; i <= 2; i++ {
		pkt, _ := svc.ReceivePacket()
		if pkt.PrimaryHeader.APID != 2047 || pkt.PrimaryHeader.SequenceCount != uint16(i) {
			t.Errorf("APID 2047 packet %d: seq = %d, want %d", i, pkt.PrimaryHeader.SequenceCount, i)
		}
	}
}

// TestSendPacketRespectsPinnedSequenceCount checks that a packet whose
// sequence count was pinned with WithSequenceCount is sent with that count,
// and that the service then resynchronizes its per-APID counter to one past
// it. CCSDS 133.0-B-2 4.1.3.4.3.4 wants the count continuous modulo 16384: if
// the counter stayed where it was, the APID would emit 0, 1, 1234, 2 and a
// receiver would read two losses that never happened.
func TestSendPacketRespectsPinnedSequenceCount(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	pinned, err := spp2.NewTMPacket(9, []byte{0x01}, spp2.WithSequenceCount(1234))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(pinned); err != nil {
		t.Fatalf("SendPacket(pinned) failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if received.PrimaryHeader.SequenceCount != 1234 {
		t.Errorf("Pinned seq count = %d, want 1234", received.PrimaryHeader.SequenceCount)
	}

	// The next unpinned packet on the same APID continues from the pinned one.
	unpinned, err := spp2.NewTMPacket(9, []byte{0x02})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(unpinned); err != nil {
		t.Fatalf("SendPacket(unpinned) failed: %v", err)
	}
	received, err = svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if received.PrimaryHeader.SequenceCount != 1235 {
		t.Errorf("Unpinned seq count = %d, want 1235", received.PrimaryHeader.SequenceCount)
	}

	// Another APID keeps its own counter; the pin does not leak across APIDs.
	other, err := spp2.NewTMPacket(10, []byte{0x03})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(other); err != nil {
		t.Fatal(err)
	}
	received, err = svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if received.PrimaryHeader.SequenceCount != 0 {
		t.Errorf("APID 10 seq count = %d, want 0", received.PrimaryHeader.SequenceCount)
	}
}

// TestSendPacketPinnedCountResyncWraps checks the resynchronization is done
// modulo 16384, so pinning the last count rolls the counter to zero.
func TestSendPacketPinnedCountResyncWraps(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	pinned, err := spp2.NewTMPacket(3, []byte{0x01}, spp2.WithSequenceCount(16383))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(pinned); err != nil {
		t.Fatal(err)
	}
	next, err := spp2.NewTMPacket(3, []byte{0x02})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(next); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ReceivePacket(); err != nil {
		t.Fatal(err)
	}
	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if received.PrimaryHeader.SequenceCount != 0 {
		t.Errorf("seq count after pinned 16383 = %d, want 0", received.PrimaryHeader.SequenceCount)
	}
}

// --- Secondary header decoding per packet (F2) ---

// TestReceivePacketGivesEachPacketItsOwnSecondaryHeader checks that decoded
// secondary headers are not shared. The service used to decode every inbound
// packet into the single instance held in ServiceConfig, so all delivered
// packets pointed at the same header and every one of them read back the
// values of whichever packet was decoded last.
func TestReceivePacketGivesEachPacketItsOwnSecondaryHeader(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:         spp2.PacketTypeTM,
		NewSecondaryHeader: func() spp2.SecondaryHeader { return &testSecondaryHeader{} },
	})

	stamps := []uint64{0x1111111111111111, 0x2222222222222222, 0x3333333333333333}
	for _, ts := range stamps {
		if err := svc.SendBytes(7, []byte{0x01},
			spp2.WithSendSecondaryHeader(&testSecondaryHeader{Timestamp: ts})); err != nil {
			t.Fatal(err)
		}
	}

	var got []*spp2.SpacePacket
	for range stamps {
		pkt, err := svc.ReceivePacket()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pkt)
	}

	for i, want := range stamps {
		sh, ok := got[i].SecondaryHeader.(*testSecondaryHeader)
		if !ok {
			t.Fatalf("packet %d: secondary header is %T, want *testSecondaryHeader", i, got[i].SecondaryHeader)
		}
		if sh.Timestamp != want {
			t.Errorf("packet %d timestamp = %#x, want %#x", i, sh.Timestamp, want)
		}
	}
}

// sizedSecondaryHeader is a secondary header whose width lives in the value,
// which is the usual shape for a real one — a PUS header reads its width from
// its mission profile. A service must decode into headers the caller's factory
// built, never into something it constructed itself from the type, because
// only the caller's value carries the width.
type sizedSecondaryHeader struct {
	Width   int
	Payload []byte
}

func (h *sizedSecondaryHeader) Size() int { return h.Width }

func (h *sizedSecondaryHeader) Encode() ([]byte, error) {
	out := make([]byte, h.Width)
	copy(out, h.Payload)
	return out, nil
}

func (h *sizedSecondaryHeader) Decode(data []byte) error {
	if len(data) != h.Width {
		return spp2.ErrDataTooShort
	}
	h.Payload = append([]byte(nil), data...)
	return nil
}

// TestReceivePacketUsesTheFactorysConfiguredHeader checks that the width the
// caller configured on the header its factory returns is the width the service
// decodes. An earlier revision cloned a single configured instance by
// reflecting on its type, which produced a zero value: a 13-octet PUS header
// was read as 7 octets and the remaining 6 leaked into UserData with no error
// raised anywhere.
func TestReceivePacketUsesTheFactorysConfiguredHeader(t *testing.T) {
	const width = 12
	var buf bytes.Buffer

	userData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	pkt, err := spp2.NewTMPacket(100, userData, spp2.WithSecondaryHeader(
		&sizedSecondaryHeader{Width: width, Payload: bytes.Repeat([]byte{0xA5}, width)}))
	if err != nil {
		t.Fatal(err)
	}
	wire, err := pkt.Encode()
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(wire)

	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType: spp2.PacketTypeTM,
		NewSecondaryHeader: func() spp2.SecondaryHeader {
			return &sizedSecondaryHeader{Width: width}
		},
	})

	got, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket: %v", err)
	}
	sh, ok := got.SecondaryHeader.(*sizedSecondaryHeader)
	if !ok {
		t.Fatalf("secondary header is %T, want *sizedSecondaryHeader", got.SecondaryHeader)
	}
	if sh.Size() != width {
		t.Errorf("decoded header width = %d, want %d", sh.Size(), width)
	}
	if len(sh.Payload) != width {
		t.Errorf("decoded header payload = %d octets, want %d", len(sh.Payload), width)
	}
	if !bytes.Equal(got.UserData, userData) {
		t.Errorf("UserData = %x, want %x — secondary header octets leaked into it",
			got.UserData, userData)
	}
}

// TestReceivePacketDoesNotShareHeadersAcrossPackets holds every delivered
// packet before reading any of their headers, so a service that decoded them
// all into one instance shows up. Reading each header right after its own
// ReceivePacket cannot detect that: the shared instance holds the right value
// at that moment and is only overwritten by the next arrival.
func TestReceivePacketDoesNotShareHeadersAcrossPackets(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:         spp2.PacketTypeTM,
		NewSecondaryHeader: func() spp2.SecondaryHeader { return &testSecondaryHeader{} },
	})

	stamps := []uint64{0xAAAA, 0xBBBB, 0xCCCC}
	for _, ts := range stamps {
		if err := svc.SendBytes(7, []byte{0x01},
			spp2.WithSendSecondaryHeader(&testSecondaryHeader{Timestamp: ts})); err != nil {
			t.Fatal(err)
		}
	}

	// Collect everything first; only then inspect.
	var got []*spp2.SpacePacket
	for range stamps {
		pkt, err := svc.ReceivePacket()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pkt)
	}

	seen := make(map[spp2.SecondaryHeader]bool, len(got))
	for i, want := range stamps {
		sh, ok := got[i].SecondaryHeader.(*testSecondaryHeader)
		if !ok {
			t.Fatalf("packet %d: secondary header is %T", i, got[i].SecondaryHeader)
		}
		if seen[got[i].SecondaryHeader] {
			t.Errorf("packet %d shares its secondary header instance with an earlier packet", i)
		}
		seen[got[i].SecondaryHeader] = true
		if sh.Timestamp != want {
			t.Errorf("packet %d timestamp = %#x, want %#x", i, sh.Timestamp, want)
		}
	}
}

// --- Packet Sequence Count continuity (F5, CCSDS 4.3.2.2) ---

// writePackets encodes packets with the given APID and sequence counts
// straight into a buffer, bypassing the sending service's own counter.
func writePackets(t *testing.T, buf *bytes.Buffer, apid uint16, counts ...uint16) {
	t.Helper()
	for _, c := range counts {
		pkt, err := spp2.NewTMPacket(apid, []byte{0x01}, spp2.WithSequenceCount(c))
		if err != nil {
			t.Fatal(err)
		}
		wire, err := pkt.Encode()
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(wire)
	}
}

func TestContinuityInSequence(t *testing.T) {
	var buf bytes.Buffer
	writePackets(t, &buf, 5, 10, 11, 12)
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	for i := range 3 {
		ind, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatal(err)
		}
		if ind.DataLoss {
			t.Errorf("packet %d: DataLoss set on a continuous stream (%d lost)", i, ind.PacketsLost)
		}
		if ind.PacketsLost != 0 {
			t.Errorf("packet %d: PacketsLost = %d, want 0", i, ind.PacketsLost)
		}
	}
}

func TestContinuityReportsGap(t *testing.T) {
	var buf bytes.Buffer
	// 41, 42, then 45: two packets (43 and 44) went missing.
	writePackets(t, &buf, 5, 41, 42, 45)
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	for range 2 {
		ind, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatal(err)
		}
		if ind.DataLoss {
			t.Fatalf("unexpected loss before the gap (%d)", ind.PacketsLost)
		}
	}

	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !ind.DataLoss {
		t.Error("DataLoss = false after counts 42 -> 45")
	}
	if ind.PacketsLost != 2 {
		t.Errorf("PacketsLost = %d, want 2", ind.PacketsLost)
	}
	if got := svc.LastDataLoss(); got != 2 {
		t.Errorf("LastDataLoss() = %d, want 2", got)
	}

	// Counting is per APID: a first packet on another APID is never a loss,
	// whatever count it carries.
	var buf2 bytes.Buffer
	writePackets(t, &buf2, 9, 9000)
	svc2 := spp2.NewService(&buf2, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})
	ind, err = svc2.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.DataLoss {
		t.Errorf("first packet on an APID reported %d lost, want 0", ind.PacketsLost)
	}
}

func TestContinuityAcrossWrap(t *testing.T) {
	var buf bytes.Buffer
	// The count wraps modulo 16384, so 16383 -> 0 is continuous and
	// 0 -> 2 loses two.
	writePackets(t, &buf, 5, 16382, 16383, 0, 2)
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	for i := range 3 {
		ind, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatal(err)
		}
		if ind.DataLoss {
			t.Errorf("packet %d: wrap reported as a loss of %d", i, ind.PacketsLost)
		}
	}

	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.PacketsLost != 1 {
		t.Errorf("PacketsLost after 0 -> 2 = %d, want 1", ind.PacketsLost)
	}

	svc.ResetContinuity()
	if got := svc.LastDataLoss(); got != 0 {
		t.Errorf("LastDataLoss() after ResetContinuity = %d, want 0", got)
	}
}

// --- Octet String indication parameters (F6, CCSDS 3.4.3.3.2) ---

func TestReceiveBytesSurfacesSecondaryHeaderIndicator(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	if err := svc.SendBytes(11, []byte{0x0A}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendBytes(12, []byte{0x0B},
		spp2.WithSendSecondaryHeader(&testSecondaryHeader{Timestamp: 7})); err != nil {
		t.Fatal(err)
	}

	plain, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if plain.SecondaryHeaderIndicator {
		t.Error("Secondary Header Indicator set on a packet without one")
	}

	withSH, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !withSH.SecondaryHeaderIndicator {
		t.Error("Secondary Header Indicator clear on a packet that carries one")
	}
	// No decoder is configured, so the header octets lead the octet string.
	if len(withSH.Data) != 9 {
		t.Errorf("octet string = %d octets, want 9 (8 header + 1 user)", len(withSH.Data))
	}
}

// TestReceiveBytesOctetStringKeepsTheSecondaryHeader checks that the Octet
// String is the whole Packet Data Field even when a decoder is configured.
//
// 4.3.2.2 defines the Octet String as what is left after the Packet Extraction
// Function removes the Packet *Primary* Header, and defines the Secondary
// Header Indicator as reporting a secondary header "at the start of the Octet
// String". An earlier revision stripped the header octets whenever a decoder
// was configured while still setting the indicator, so the indicator described
// something that was no longer there.
func TestReceiveBytesOctetStringKeepsTheSecondaryHeader(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:         spp2.PacketTypeTM,
		NewSecondaryHeader: func() spp2.SecondaryHeader { return &testSecondaryHeader{} },
	})

	if err := svc.SendBytes(12, []byte{0x0B},
		spp2.WithSendSecondaryHeader(&testSecondaryHeader{Timestamp: 0x0102030405060708})); err != nil {
		t.Fatal(err)
	}

	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !ind.SecondaryHeaderIndicator {
		t.Fatal("Secondary Header Indicator clear on a packet that carries one")
	}
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0B}
	if !bytes.Equal(ind.Data, want) {
		t.Errorf("octet string = %x, want %x (header octets must lead it)", ind.Data, want)
	}
	// The parsed header is offered alongside, not instead.
	sh, ok := ind.SecondaryHeader.(*testSecondaryHeader)
	if !ok {
		t.Fatalf("Indication.SecondaryHeader is %T, want *testSecondaryHeader", ind.SecondaryHeader)
	}
	if sh.Timestamp != 0x0102030405060708 {
		t.Errorf("parsed timestamp = %#x", sh.Timestamp)
	}
}

// TestReceiveBytesExcludesErrorControl checks that the two error control
// octets this layer consumes and verifies are not handed to the octet string
// user as part of their data.
func TestReceiveBytesExcludesErrorControl(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:   spp2.PacketTypeTM,
		ErrorControl: true,
	})

	payload := []byte{0x11, 0x22, 0x33}
	if err := svc.SendBytes(13, payload, spp2.WithSendErrorControl()); err != nil {
		t.Fatal(err)
	}
	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ind.Data, payload) {
		t.Errorf("octet string = %x, want %x", ind.Data, payload)
	}
}

// --- Secondary Header Indicator on the request side (CCSDS 3.4.2.3.2) ---

// TestSendBytesSecondaryHeaderIndicator checks that a user holding a
// pre-formatted data field can set the Secondary Header Flag without having to
// supply a SecondaryHeader implementation. Per 3.4.2.3.2 the parameter is a
// signal about octets the user already assembled, not a header object.
func TestSendBytesSecondaryHeaderIndicator(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	octets := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0x01, 0x02, 0x03}
	if err := svc.SendBytes(14, octets,
		spp2.WithSendSecondaryHeaderIndicator(true)); err != nil {
		t.Fatal(err)
	}

	wire := buf.Bytes()
	if flag := (wire[0] >> 3) & 0x01; flag != 1 {
		t.Errorf("Secondary Header Flag = %d, want 1", flag)
	}
	if declared := int(wire[4])<<8 | int(wire[5]); declared+1 != len(wire)-spp2.PrimaryHeaderSize {
		t.Errorf("Packet Data Length declares %d octets, %d written",
			declared+1, len(wire)-spp2.PrimaryHeaderSize)
	}

	ind, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !ind.SecondaryHeaderIndicator {
		t.Error("indicator did not survive the round trip")
	}
	if !bytes.Equal(ind.Data, octets) {
		t.Errorf("octet string = %x, want %x", ind.Data, octets)
	}
}

// TestSendBytesRejectsBothSecondaryHeaderForms checks that supplying a parsed
// header and an indicator together is refused rather than counting the header
// twice in the Packet Data Length.
func TestSendBytesRejectsBothSecondaryHeaderForms(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	err := svc.SendBytes(15, []byte{0x01, 0x02},
		spp2.WithSendSecondaryHeader(&testSecondaryHeader{}),
		spp2.WithSendSecondaryHeaderIndicator(true))
	if !errors.Is(err, spp2.ErrSecondaryHeaderTwice) {
		t.Errorf("SendBytes = %v, want ErrSecondaryHeaderTwice", err)
	}
	if buf.Len() != 0 {
		t.Errorf("%d octets written despite the error", buf.Len())
	}
}

// --- Octet String request parameters (M1, CCSDS 3.4.3.2.2) ---

func TestSendBytesPacketTypeAndSequenceCount(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	if err := svc.SendBytes(21, []byte{0x01},
		spp2.WithSendPacketType(spp2.PacketTypeTC),
		spp2.WithSendPacketName(4242)); err != nil {
		t.Fatal(err)
	}
	// The next packet on the same APID continues from the pinned value.
	if err := svc.SendBytes(21, []byte{0x02}); err != nil {
		t.Fatal(err)
	}

	first, err := svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if first.PrimaryHeader.Type != spp2.PacketTypeTC {
		t.Errorf("packet type = %d, want TC", first.PrimaryHeader.Type)
	}
	if first.PrimaryHeader.SequenceCount != 4242 {
		t.Errorf("packet name = %d, want 4242", first.PrimaryHeader.SequenceCount)
	}

	second, err := svc.ReceivePacket()
	if err != nil {
		t.Fatal(err)
	}
	if second.PrimaryHeader.Type != spp2.PacketTypeTM {
		t.Errorf("packet type = %d, want TM (the service default)", second.PrimaryHeader.Type)
	}
	if second.PrimaryHeader.SequenceCount != 4243 {
		t.Errorf("sequence count = %d, want 4243", second.PrimaryHeader.SequenceCount)
	}
}

// --- Idle packet discard (M2) ---

func TestDiscardIdle(t *testing.T) {
	var buf bytes.Buffer
	sender := spp2.NewService(&buf, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	idle, err := spp2.NewIdlePacket([]byte{0xFF, 0xFF})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SendPacket(idle); err != nil {
		t.Fatal(err)
	}
	if err := sender.SendBytes(31, []byte{0xAB}); err != nil {
		t.Fatal(err)
	}

	receiver := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:  spp2.PacketTypeTM,
		DiscardIdle: true,
	})

	ind, err := receiver.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.APID != 31 {
		t.Errorf("APID = %d, want 31 (the idle packet should have been dropped)", ind.APID)
	}
	if !bytes.Equal(ind.Data, []byte{0xAB}) {
		t.Errorf("data = %v, want AB", ind.Data)
	}

	// Without the option, the idle packet is delivered as it always was.
	var buf2 bytes.Buffer
	sender2 := spp2.NewService(&buf2, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})
	idle2, err := spp2.NewIdlePacket([]byte{0xFF, 0xFF})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender2.SendPacket(idle2); err != nil {
		t.Fatal(err)
	}
	plain := spp2.NewService(&buf2, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})
	ind, err = plain.ReceiveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ind.APID != spp2.APIDIdle {
		t.Errorf("APID = %d, want the idle APID", ind.APID)
	}
}

// --- Packet Transfer Function: a relay must not renumber (CCSDS 3.3.1, 4.2.3) ---

// TestSendPacketForwardsADecodedPacketUnchanged checks that a packet read off
// the wire and handed straight back to SendPacket goes out exactly as it came
// in. 3.3.1 requires Packet Service SDUs to be transferred "without further
// formatting", 4.1.3.4.3.3 makes the count the property of the originating
// application, and the Packet Transfer Function of 4.2.3 does not renumber.
// The sequence counter belongs to the Packet Assembly Function (4.2.2.4),
// which serves the Octet String Service.
func TestSendPacketForwardsADecodedPacketUnchanged(t *testing.T) {
	// Build an inbound packet carrying sequence count 500 on APID 100.
	source, err := spp2.NewTMPacket(100, []byte{1, 2, 3}, spp2.WithSequenceCount(500))
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := source.Encode()
	if err != nil {
		t.Fatal(err)
	}

	received, err := spp2.Decode(inbound)
	if err != nil {
		t.Fatal(err)
	}
	if received.PrimaryHeader.SequenceCount != 500 {
		t.Fatalf("decoded count = %d, want 500", received.PrimaryHeader.SequenceCount)
	}

	var out bytes.Buffer
	relay := spp2.NewService(&out, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})
	if err := relay.SendPacket(received); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), inbound) {
		t.Errorf("relayed packet changed:\n got %x\nwant %x", out.Bytes(), inbound)
	}
	if received.PrimaryHeader.SequenceCount != 500 {
		t.Errorf("relay renumbered the packet in place: count = %d, want 500",
			received.PrimaryHeader.SequenceCount)
	}

	// The relay's own counter for that APID resynchronizes to one past what it
	// forwarded, so a packet it originates itself continues the sequence
	// rather than jumping back (4.1.3.4.3.4).
	own, err := spp2.NewTMPacket(100, []byte{9})
	if err != nil {
		t.Fatal(err)
	}
	if err := relay.SendPacket(own); err != nil {
		t.Fatal(err)
	}
	if own.PrimaryHeader.SequenceCount != 501 {
		t.Errorf("next originated count = %d, want 501", own.PrimaryHeader.SequenceCount)
	}
}

// TestSendPacketStampsAPacketBuiltWithoutACount checks the ordinary case is
// unaffected: a freshly built packet still gets the service's next count.
func TestSendPacketStampsAPacketBuiltWithoutACount(t *testing.T) {
	var out bytes.Buffer
	svc := spp2.NewService(&out, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})
	for want := uint16(0); want < 3; want++ {
		pkt, err := spp2.NewTMPacket(4, []byte{1})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.SendPacket(pkt); err != nil {
			t.Fatal(err)
		}
		if pkt.PrimaryHeader.SequenceCount != want {
			t.Errorf("count = %d, want %d", pkt.PrimaryHeader.SequenceCount, want)
		}
	}
}

// TestSendPacketRestoresTheCounterOnFailure checks that a send which never
// reaches the transport does not spend a sequence count. Spending one would
// leave a hole the receiver reads as a lost packet (4.1.3.4.3.4).
func TestSendPacketRestoresTheCounterOnFailure(t *testing.T) {
	var out bytes.Buffer
	svc := spp2.NewService(&out, spp2.ServiceConfig{
		PacketType:      spp2.PacketTypeTM,
		MaxPacketLength: 10,
	})

	tooBig, err := spp2.NewTMPacket(6, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	countBefore := tooBig.PrimaryHeader.SequenceCount
	if err := svc.SendPacket(tooBig); !errors.Is(err, spp2.ErrPacketTooLarge) {
		t.Fatalf("SendPacket = %v, want ErrPacketTooLarge", err)
	}
	if out.Len() != 0 {
		t.Errorf("%d octets written despite the error", out.Len())
	}
	if tooBig.PrimaryHeader.SequenceCount != countBefore {
		t.Errorf("rejected packet was left stamped: count = %d, want %d",
			tooBig.PrimaryHeader.SequenceCount, countBefore)
	}

	fits, err := spp2.NewTMPacket(6, []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SendPacket(fits); err != nil {
		t.Fatal(err)
	}
	if fits.PrimaryHeader.SequenceCount != 0 {
		t.Errorf("first packet actually sent got count %d, want 0 — the rejected "+
			"send consumed a count", fits.PrimaryHeader.SequenceCount)
	}
}

// --- Concurrency (CCSDS 4.1.3.4.3.4) ---

// countingWriter records each Write as one unit and notes the sequence count
// it carried, so a test can check the order packets reached the transport.
type countingWriter struct {
	mu     sync.Mutex
	counts []uint16
}

func (w *countingWriter) Read([]byte) (int, error) { return 0, io.EOF }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) >= 4 {
		w.counts = append(w.counts, uint16(p[2]&0x3F)<<8|uint16(p[3]))
	}
	return len(p), nil
}

// TestConcurrentSendPacketKeepsTheCountContinuous checks that concurrent
// senders put packets on the wire in the same order as the counts they were
// given. Allocating the count under a lock but writing outside it satisfies
// -race and still emits a discontinuous sequence, which 4.1.3.4.3.4 forbids
// and a receiver reads as loss.
func TestConcurrentSendPacketKeepsTheCountContinuous(t *testing.T) {
	w := &countingWriter{}
	svc := spp2.NewService(w, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	const senders = 64
	var wg sync.WaitGroup
	for range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pkt, err := spp2.NewTMPacket(100, []byte{1, 2, 3, 4})
			if err != nil {
				t.Error(err)
				return
			}
			if err := svc.SendPacket(pkt); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if len(w.counts) != senders {
		t.Fatalf("%d packets reached the transport, want %d", len(w.counts), senders)
	}
	for i, got := range w.counts {
		if got != uint16(i) {
			t.Fatalf("packet %d on the wire carries count %d: the sequence is not "+
				"continuous (full order %v)", i, got, w.counts)
		}
	}
}

// lockstepTransport is a byte pipe safe for concurrent use, so a concurrency
// test measures the Service rather than the test's own plumbing.
//
// It hands back one octet per Read and yields the processor in between, which
// is what a real socket does under load: io.ReadFull loops, and any reader
// that does not hold a lock across its reads will have another reader's octets
// spliced into the middle of its packet. A transport that satisfied every read
// in one call would hide exactly the defect this exists to detect.
type lockstepTransport struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (t *lockstepTransport) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.buf.Len() == 0 {
		return 0, io.EOF
	}
	n, err := t.buf.Read(p[:1])
	runtime.Gosched()
	return n, err
}

func (t *lockstepTransport) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.Write(p)
}

// TestConcurrentReceivePacketDoesNotSplicePackets checks that a packet's
// header read and body read are not interleaved with another receiver's. Each
// packet's user data is four copies of one octet, so a spliced packet shows up
// immediately.
func TestConcurrentReceivePacketDoesNotSplicePackets(t *testing.T) {
	tr := &lockstepTransport{}
	const packets = 64
	for i := range packets {
		pkt, err := spp2.NewTMPacket(100, bytes.Repeat([]byte{byte(i)}, 4))
		if err != nil {
			t.Fatal(err)
		}
		wire, err := pkt.Encode()
		if err != nil {
			t.Fatal(err)
		}
		tr.buf.Write(wire)
	}

	svc := spp2.NewService(tr, spp2.ServiceConfig{PacketType: spp2.PacketTypeTM})

	var wg sync.WaitGroup
	var mu sync.Mutex
	var delivered int
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				pkt, err := svc.ReceivePacket()
				if err != nil {
					return
				}
				u := pkt.UserData
				mu.Lock()
				delivered++
				mu.Unlock()
				if len(u) != 4 || u[0] != u[1] || u[1] != u[2] || u[2] != u[3] {
					t.Errorf("spliced packet: UserData = %x", u)
					return
				}
			}
		}()
	}
	wg.Wait()

	if delivered != packets {
		t.Errorf("delivered %d packets, want %d", delivered, packets)
	}
}

// --- Oversize packets must not desynchronize the reader (CCSDS 4.3.3) ---

// TestReceivePacketResynchronizesAfterAnOversizePacket checks that rejecting a
// packet longer than the managed Maximum Packet Length leaves the reader on a
// real packet boundary. The primary header is already consumed when the limit
// is found, so leaving the body behind made the next read parse payload as a
// primary header and deliver packets that were never sent.
func TestReceivePacketResynchronizesAfterAnOversizePacket(t *testing.T) {
	var buf bytes.Buffer

	oversize, err := spp2.NewTMPacket(5, make([]byte, 100))
	if err != nil {
		t.Fatal(err)
	}
	wire, err := oversize.Encode()
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(wire)

	wanted, err := spp2.NewTMPacket(9, []byte{0xAB, 0xCD})
	if err != nil {
		t.Fatal(err)
	}
	wantedWire, err := wanted.Encode()
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(wantedWire)

	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:      spp2.PacketTypeTM,
		MaxPacketLength: 50,
	})

	if _, err := svc.ReceivePacket(); !errors.Is(err, spp2.ErrPacketTooLarge) {
		t.Fatalf("first ReceivePacket = %v, want ErrPacketTooLarge", err)
	}

	got, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("second ReceivePacket = %v, want the packet after the oversize one", err)
	}
	if got.PrimaryHeader.APID != 9 {
		t.Errorf("APID = %d, want 9 — the reader resynchronized onto the wrong octet",
			got.PrimaryHeader.APID)
	}
	if !bytes.Equal(got.UserData, []byte{0xAB, 0xCD}) {
		t.Errorf("UserData = %x, want abcd", got.UserData)
	}
}
