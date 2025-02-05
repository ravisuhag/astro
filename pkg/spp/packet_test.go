package spp_test

import (
	"bytes"
	spp2 "github.com/ravisuhag/astro/pkg/spp"
	"testing"
)

func TestNewSpacePacket(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
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
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	encoded, err := packet.Encode()
	if err != nil {
		t.Fatalf("Failed to encode space packet: %v", err)
	}

	decoded, err := spp2.Decode(encoded)
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
	secondaryHeader := spp2.SecondaryHeader{Timestamp: 1234567890}
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithSecondaryHeader(secondaryHeader))
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
	packet, err := spp2.NewSpacePacket(100, 0, data, spp2.WithErrorControl(crc))
	if err != nil {
		t.Fatalf("Failed to create new space packet with error control: %v", err)
	}

	if packet.ErrorControl == nil || *packet.ErrorControl != crc {
		t.Errorf("Error control does not match. Got %v, want %v", packet.ErrorControl, crc)
	}
}

func TestSpacePacketValidate(t *testing.T) {
	// Test case: Valid SpacePacket
	data := []byte{0x01, 0x02, 0x03}
	packet, err := spp2.NewSpacePacket(100, 0, data)
	if err != nil {
		t.Fatalf("Failed to create new space packet: %v", err)
	}

	if err := packet.Validate(); err != nil {
		t.Errorf("Expected packet to be valid, but got error: %v", err)
	}

	// Test case: Invalid APID
	packet.PrimaryHeader.APID = 3000
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for invalid APID, but got none")
	}
	packet.PrimaryHeader.APID = 100 // Reset to valid APID

	// Test case: Invalid user data length
	packet.UserData = []byte{0x01, 0x02}
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for mismatched user data length, but got none")
	}
	packet.UserData = data // Reset to valid user data

	// Test case: Invalid packet length
	packet.PrimaryHeader.PacketLength = 65535
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for packet length exceeding maximum, but got none")
	}
	packet.PrimaryHeader.PacketLength = uint16(len(data)) - 1 // Reset to valid packet length

	// Test case: Secondary header flag set but no secondary header
	packet.PrimaryHeader.SecondaryHeaderFlag = 1
	if err := packet.Validate(); err == nil {
		t.Errorf("Expected error for missing secondary header, but got none")
	}
	packet.PrimaryHeader.SecondaryHeaderFlag = 0 // Reset to valid state

	// Test case: Valid secondary header
	secondaryHeader := spp2.SecondaryHeader{Timestamp: 1234567890}
	packet.SecondaryHeader = &secondaryHeader
	packet.PrimaryHeader.SecondaryHeaderFlag = 1
	if err := packet.Validate(); err != nil {
		t.Errorf("Expected packet to be valid with secondary header, but got error: %v", err)
	}
}
