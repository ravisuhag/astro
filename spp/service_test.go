package spp_test

import (
	"bytes"
	"github.com/ravisuhag/astro/spp"
	"testing"
)

func TestEncapsulateOctetString(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.EncapsulateOctetString(100, data)
	if err != nil {
		t.Fatalf("Failed to encapsulate octet string: %v", err)
	}

	if packet.PrimaryHeader.APID != 100 {
		t.Errorf("Expected APID 100, got %d", packet.PrimaryHeader.APID)
	}

	if !bytes.Equal(packet.UserData, data) {
		t.Errorf("User data does not match. Got %v, want %v", packet.UserData, data)
	}
}

func TestDecapsulateOctetString(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.NewSpacePacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	userData, err := spp.DecapsulateOctetString(packet)
	if err != nil {
		t.Fatalf("Failed to decapsulate octet string: %v", err)
	}

	if !bytes.Equal(userData, data) {
		t.Errorf("User data does not match. Got %v, want %v", userData, data)
	}
}

func TestSendPacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.NewSpacePacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	var buf bytes.Buffer
	err = spp.SendPacket(packet, &buf)
	if err != nil {
		t.Fatalf("Failed to send space packet: %v", err)
	}

	if buf.Len() == 0 {
		t.Errorf("Expected buffer to have data")
	}
}

func TestReceivePacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.NewSpacePacket(100, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}
	var buf bytes.Buffer
	err = spp.SendPacket(packet, &buf)
	if err != nil {
		t.Fatalf("Failed to send space packet: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("Buffer is empty after sending packet")
	}
	receivedPacket, err := spp.ReceivePacket(&buf)
	if err != nil {
		t.Fatalf("Failed to receive space packet: %v", err)
	}
	if packet.PrimaryHeader != receivedPacket.PrimaryHeader {
		t.Errorf("Primary header does not match. Got %+v, want %+v", receivedPacket.PrimaryHeader, packet.PrimaryHeader)
	}

	if !bytes.Equal(packet.UserData, receivedPacket.UserData) {
		t.Errorf("User data does not match. Got %v, want %v", receivedPacket.UserData, packet.UserData)
	}
}
