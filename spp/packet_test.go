package spp_test

import (
	"bytes"
	"github.com/ravisuhag/astro/spp"
	"testing"
)

func TestNewSpacePacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	if packet.PrimaryHeader.APID != 100 {
		t.Errorf("Expected APID 100, got %d", packet.PrimaryHeader.APID)
	}

	if !bytes.Equal(packet.UserData, data) {
		t.Errorf("User data does not match. Got %v, want %v", packet.UserData, data)
	}
}

func TestSpacePacketEncodeDecode(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode space packet: %v", err)
	}

	decoded, err := spp.Decode(encoded)
	if err != nil {
		t.Fatalf("Failed to decode space packet: %v", err)
	}

	if packet.PrimaryHeader != decoded.PrimaryHeader {
		t.Errorf("Primary header does not match. Got %+v, want %+v", decoded.PrimaryHeader, packet.PrimaryHeader)
	}

	if !bytes.Equal(packet.UserData, decoded.UserData) {
		t.Errorf("User data does not match. Got %v, want %v", decoded.UserData, packet.UserData)
	}
}

func TestSpacePacketWithSecondaryHeader(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	secondaryHeader := spp.SecondaryHeader{Timestamp: 1234567890}
	packet, err := spp.NewSpacePacket(100, 0, data, spp.WithSecondaryHeader(secondaryHeader))
	if err != nil {
		t.Fatalf("Failed to create new space packet with secondary header: %v", err)
	}

	if packet.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Errorf("Expected SecondaryHeaderFlag 1, got %d", packet.PrimaryHeader.SecondaryHeaderFlag)
	}

	if packet.SecondaryHeader == nil || packet.SecondaryHeader.Timestamp != 1234567890 {
		t.Errorf("Secondary header does not match. Got %+v, want %+v", packet.SecondaryHeader, secondaryHeader)
	}
}

func TestSpacePacketWithErrorControl(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	crc := uint16(0xABCD)
	packet, err := spp.NewSpacePacket(100, 0, data, spp.WithErrorControl(crc))
	if err != nil {
		t.Fatalf("Failed to create new space packet with error control: %v", err)
	}

	if packet.ErrorControl == nil || *packet.ErrorControl != crc {
		t.Errorf("Error control does not match. Got %v, want %v", packet.ErrorControl, crc)
	}
}
