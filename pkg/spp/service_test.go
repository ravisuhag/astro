package spp_test

import (
	"bytes"
	spp2 "github.com/ravisuhag/astro/pkg/spp"
	"testing"
)

func TestWritePacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	var buf bytes.Buffer
	err = spp2.WritePacket(packet, &buf)
	if err != nil {
		t.Fatalf("Failed to send space packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode packet: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), encoded) {
		t.Errorf("Written bytes don't match encoded packet. Got %v, want %v", buf.Bytes(), encoded)
	}
}

func TestSendPacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}
	var buf bytes.Buffer
	err = spp2.WritePacket(packet, &buf)
	if err != nil {
		t.Fatalf("Failed to send space packet: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("Buffer is empty after sending packet")
	}
	receivedPacket, err := spp2.ReadPacket(&buf)
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
