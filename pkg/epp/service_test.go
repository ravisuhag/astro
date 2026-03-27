package epp_test

import (
	"bytes"
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
	if err := svc.SendBytes(epp.ProtocolIDExtended, data, epp.WithExtendedProtocolID(99)); err != nil {
		t.Fatalf("SendBytes failed: %v", err)
	}

	received, err := svc.ReceivePacket()
	if err != nil {
		t.Fatalf("ReceivePacket failed: %v", err)
	}
	if received.Header.ExtendedProtocolID != 99 {
		t.Errorf("ExtendedProtocolID = %d, want 99", received.Header.ExtendedProtocolID)
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

func TestServiceMultiplePacketsRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{})

	payloads := []struct {
		pid  uint8
		data []byte
	}{
		{epp.ProtocolIDIPE, []byte{0x01}},
		{epp.ProtocolIDUserDef, []byte{0x02, 0x03}},
		{epp.ProtocolIDIPE, []byte{0x04, 0x05, 0x06}},
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

func TestServiceFormat5RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	svc := epp.NewService(&buf, epp.ServiceConfig{
		MaxPacketLength: 100000,
	})

	data := make([]byte, 70000)
	data[0] = 0xAA
	data[69999] = 0xBB

	pkt, err := epp.NewPacket(epp.ProtocolIDExtended, data, epp.WithCCSDSDefined(1, 0))
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
