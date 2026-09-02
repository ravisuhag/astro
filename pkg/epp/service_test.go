package epp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/epp"
)

func TestServiceSendReceivePacket(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	data := []byte{0x01, 0x02, 0x03}
	pkt, err := epp.NewIPEPacket(data)
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	if err := svc.SendPacket(pkt); err != nil {
		t.Fatalf("SendPacket failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}

	if received.Header.ProtocolID != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", received.Header.ProtocolID, epp.ProtocolIDIPE)
	}
	if !bytes.Equal(received.Data, data) {
		t.Errorf("Data mismatch. Got %v, want %v", received.Data, data)
	}
}

func TestServiceSendReceiveIdlePacket(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	idle, _ := epp.NewIdlePacket()
	if err := svc.SendPacket(idle); err != nil {
		t.Fatalf("SendPacket(idle) failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if !received.IsIdle() {
		t.Error("Expected idle packet")
	}
}

func TestServiceSendReceiveIdleFillPacket(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	idle, err := epp.NewIdleFillPacket(16, 0x55)
	if err != nil {
		t.Fatalf("NewIdleFillPacket failed: %v", err)
	}
	if err := svc.SendPacket(idle); err != nil {
		t.Fatalf("SendPacket(idle fill) failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if !received.IsIdle() {
		t.Error("Expected idle packet")
	}
	if len(received.Data) != 14 {
		t.Errorf("Idle fill data = %d bytes, want 14", len(received.Data))
	}
}

func TestServiceSendReceiveBytes(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	data := []byte{0x45, 0x00, 0x00, 0x14}
	if err := svc.SendBytes(epp.ProtocolIDIPE, data); err != nil {
		t.Fatalf("SendBytes failed: %v", err)
	}

	pid, received, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatalf("ReceiveBytes failed: %v", err)
	}
	if pid != epp.ProtocolIDIPE {
		t.Errorf("ProtocolID = %d, want %d", pid, epp.ProtocolIDIPE)
	}
	if !bytes.Equal(received, data) {
		t.Errorf("Data mismatch. Got %v, want %v", received, data)
	}
}

func TestServiceSendBytesWithOptions(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	data := []byte{0x01, 0x02}
	if err := svc.SendBytes(epp.ProtocolIDExtended, data, epp.WithExtendedProtocolID(9)); err != nil {
		t.Fatalf("SendBytes failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if received.Header.ExtendedProtocolID != 9 {
		t.Errorf("ExtendedProtocolID = %d, want 9", received.Header.ExtendedProtocolID)
	}
	if !bytes.Equal(received.Data, data) {
		t.Errorf("Data mismatch")
	}
}

func TestServiceSendNilPacket(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	if err := svc.SendPacket(nil); err == nil {
		t.Error("Expected error when sending nil packet")
	}
}

func TestServiceMaxPacketLength(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{
		MaxPacketLength: 10,
	})

	// 2-byte header + 9 bytes data = 11 > 10
	data := make([]byte, 9)
	if err := svc.SendBytes(epp.ProtocolIDIPE, data); err == nil {
		t.Error("Expected error for packet exceeding max length")
	}
}

func TestServiceDefaultAcceptsLargePackets(t *testing.T) {
	// EPP-F11: the default limit must not reject spec-valid packets that
	// need the 32-bit length field.
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	data := make([]byte, 70000)
	data[0] = 0xAA
	data[69999] = 0xBB
	if err := svc.SendBytes(epp.ProtocolIDIPE, data); err != nil {
		t.Fatalf("SendBytes(70000) failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if received.Data[0] != 0xAA || received.Data[69999] != 0xBB {
		t.Error("Large packet data corrupted with default config")
	}
}

func TestServiceMultiplePacketsRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	payloads := []struct {
		pid  uint8
		data []byte
	}{
		{epp.ProtocolIDIPE, []byte{0x01}},
		{epp.ProtocolIDMission, []byte{0x02, 0x03}},
		{epp.ProtocolIDLTP, []byte{0x04, 0x05, 0x06}},
	}

	for _, p := range payloads {
		if err := svc.SendBytes(p.pid, p.data); err != nil {
			t.Fatalf("SendBytes(pid=%d) failed: %v", p.pid, err)
		}
	}

	for _, p := range payloads {
		pid, data, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatalf("ReceiveBytes failed: %v", err)
		}
		if pid != p.pid {
			t.Errorf("ProtocolID = %d, want %d", pid, p.pid)
		}
		if !bytes.Equal(data, p.data) {
			t.Errorf("Data mismatch for PID %d. Got %v, want %v", p.pid, data, p.data)
		}
	}
}

func TestService8OctetHeaderRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{
		MaxPacketLength: 100000,
	})

	data := make([]byte, 70000)
	data[0] = 0xAA
	data[69999] = 0xBB

	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data,
		epp.WithExtendedProtocolID(1), epp.WithCCSDSDefined(0))
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if err := svc.SendPacket(pkt); err != nil {
		t.Fatalf("SendPacket failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}

	if received.Data[0] != 0xAA || received.Data[69999] != 0xBB {
		t.Error("Large packet data corrupted after service round-trip")
	}
}

func TestServiceDefaultConfig(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestServiceReceiveMaxPacketLength(t *testing.T) {
	var buf bytes.Buffer
	// Write a packet that exceeds the receiver's max length
	bigData := make([]byte, 200)
	pkt, _ := epp.NewIPEPacket(bigData, epp.WithLongLength())
	encoded, _ := pkt.Encode()
	buf.Write(encoded)

	svc := epp.NewService(&buf, epp.ServiceConfig{
		MaxPacketLength: 10,
	})

	_, err := svc.ReceivePacket()
	if err == nil {
		t.Error("Expected error for packet exceeding max length on receive")
	}
}

// TestServiceReceiveOversizeSkipsDataZone is a regression test: rejecting an
// oversize packet must also take its data zone off the transport, or the next
// receive starts in the middle of that payload and reads it as a packet.
//
// The oversize packet's payload opens with E5 05 DE AD BE. E5 is '111 001 01'
// (a valid PVN with an LTP Protocol ID and a 2-octet header declaring a total
// length of 5) so a reader left on that octet hands back a 3-byte LTP packet
// that was never sent, and never reaches the genuine packet behind it.
func TestServiceReceiveOversizeSkipsDataZone(t *testing.T) {
	payload := make([]byte, 58)
	copy(payload, []byte{0xE5, 0x05, 0xDE, 0xAD, 0xBE})

	// 2-octet header + 58 octets of payload = 60 octets, past the limit of 50.
	oversize, err := epp.NewLTPPacket(payload)
	if err != nil {
		t.Fatalf("NewLTPPacket failed: %v", err)
	}
	genuine, err := epp.NewIPEPacket([]byte{0x5A})
	if err != nil {
		t.Fatalf("NewIPEPacket failed: %v", err)
	}

	var buf bytes.Buffer
	for _, pkt := range []*epp.EncapsulationPacket{oversize, genuine} {
		encoded, err := pkt.Encode()
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		buf.Write(encoded)
	}

	svc := epp.NewService(&buf, epp.ServiceConfig{MaxPacketLength: 50})

	if _, err := svc.ReceivePacket(); !errors.Is(err, epp.ErrPacketTooLarge) {
		t.Fatalf("First ReceivePacket = %v, want ErrPacketTooLarge", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("Second ReceivePacket failed: %v. The oversize packet's data zone was "+
			"left on the transport, so the reader is no longer on a packet boundary", err)
	}
	if received.Header.ProtocolID != epp.ProtocolIDIPE || !bytes.Equal(received.Data, []byte{0x5A}) {
		t.Fatalf("Second ReceivePacket returned a fabricated packet: Protocol ID %d, data % X. "+
			"Want the genuine IPE packet (Protocol ID %d, data 5A). The oversize packet's data "+
			"zone was left on the transport and its payload was decoded as a packet",
			received.Header.ProtocolID, received.Data, epp.ProtocolIDIPE)
	}
}
