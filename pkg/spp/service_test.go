package spp_test

import (
	"bytes"
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
		PacketType:      spp2.PacketTypeTC,
		SecondaryHeader: &testSecondaryHeader{},
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

// TestDeprecatedSecondaryHeaderConfigStillClonesPerPacket checks the old
// single-instance configuration keeps working and no longer aliases.
func TestDeprecatedSecondaryHeaderConfigStillClonesPerPacket(t *testing.T) {
	var buf bytes.Buffer
	svc := spp2.NewService(&buf, spp2.ServiceConfig{
		PacketType:      spp2.PacketTypeTM,
		SecondaryHeader: &testSecondaryHeader{},
	})

	stamps := []uint64{0xAAAA, 0xBBBB, 0xCCCC}
	for _, ts := range stamps {
		if err := svc.SendBytes(7, []byte{0x01},
			spp2.WithSendSecondaryHeader(&testSecondaryHeader{Timestamp: ts})); err != nil {
			t.Fatal(err)
		}
	}

	var got []uint64
	for range stamps {
		pkt, err := svc.ReceivePacket()
		if err != nil {
			t.Fatal(err)
		}
		sh, ok := pkt.SecondaryHeader.(*testSecondaryHeader)
		if !ok {
			t.Fatalf("secondary header is %T, want *testSecondaryHeader", pkt.SecondaryHeader)
		}
		got = append(got, sh.Timestamp)
	}

	for i, want := range stamps {
		if got[i] != want {
			t.Errorf("packet %d timestamp = %#x, want %#x", i, got[i], want)
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
