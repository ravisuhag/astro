package spp

import (
	"errors"
	"io"
)

const PrimaryHeaderSize = 6 // CCSDS Packet Header is 6 bytes

// EncapsulateOctetString encapsulates raw data (Octet String) into a Space Packet.
func EncapsulateOctetString(apid uint16, data []byte) (*SpacePacket, error) {
	if apid > 2047 {
		return nil, errors.New("invalid APID: must be in range 0-2047")
	}
	return NewSpacePacket(apid, data)
}

// DecapsulateOctetString decapsulates a Space Packet to extract the Octet String (raw data).
func DecapsulateOctetString(packet *SpacePacket) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("invalid packet: cannot decapsulate nil packet")
	}
	return packet.UserData, nil
}

// SendPacket writes a pre-formatted Space Packet to an io.Writer.
func SendPacket(packet *SpacePacket, writer io.Writer) error {
	if packet == nil {
		return errors.New("invalid packet: cannot send nil packet")
	}

	data, err := packet.Encode()
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// ReceivePacket reads a Space Packet from an io.Reader.
func ReceivePacket(reader io.Reader) (*SpacePacket, error) {
	header := make([]byte, PrimaryHeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	// Calculate the total packet size using the header
	totalPacketSize, err := CalculatePacketSize(header)
	if err != nil {
		return nil, err
	}

	// Ensure the total packet size is reasonable
	if totalPacketSize < PrimaryHeaderSize {
		return nil, errors.New("calculated packet size is smaller than header size")
	}
	// Read the full packet
	buffer := make([]byte, totalPacketSize)
	copy(buffer[:PrimaryHeaderSize], header) // Copy the header into the buffer
	if _, err := io.ReadFull(reader, buffer[PrimaryHeaderSize:]); err != nil {
		return nil, err
	}
	// Decode the packet
	return Decode(buffer)
}
