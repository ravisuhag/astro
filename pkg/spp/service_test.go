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

	if packet.PrimaryHeader != received.PrimaryHeader {
		t.Errorf("Primary header mismatch. Got %+v, want %+v", received.PrimaryHeader, packet.PrimaryHeader)
	}
	if !bytes.Equal(packet.UserData, received.UserData) {
		t.Errorf("User data mismatch. Got %v, want %v", received.UserData, packet.UserData)
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

	apid, received, err := svc.ReceiveBytes()
	if err != nil {
		t.Fatalf("ReceiveBytes failed: %v", err)
	}

	if apid != 200 {
		t.Errorf("APID mismatch. Got %d, want 200", apid)
	}
	if !bytes.Equal(received, data) {
		t.Errorf("Data mismatch. Got %v, want %v", received, data)
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
		apid, data, err := svc.ReceiveBytes()
		if err != nil {
			t.Fatalf("ReceiveBytes failed: %v", err)
		}
		if apid != p.apid {
			t.Errorf("APID mismatch. Got %d, want %d", apid, p.apid)
		}
		if !bytes.Equal(data, p.data) {
			t.Errorf("Data mismatch for APID %d. Got %v, want %v", p.apid, data, p.data)
		}
	}
}
