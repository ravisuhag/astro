package spp

import (
	"errors"
	"io"
)

const PrimaryHeaderSize = 6 // CCSDS Packet Header is 6 bytes

// WritePacket writes a single pre-formatted Space Packet to an io.Writer.
// This function only writes one packet at a time.
func WritePacket(packet *SpacePacket, writer io.Writer) error {
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

// ReadPacket reads a single Space Packet from an io.Reader.
// It reads the primary header first to determine the total packet size
// and then reads the rest of the packet based on that size.
// This function only reads one packet at a time.
func ReadPacket(reader io.Reader) (*SpacePacket, error) {
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
