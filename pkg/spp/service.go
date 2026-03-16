package spp

import (
	"errors"
	"io"
)

// WritePacket writes a single pre-formatted Space Packet to an io.Writer.
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
// If a SecondaryHeader implementation is provided and the packet has
// the secondary header flag set, it will be used to decode the secondary header.
func ReadPacket(reader io.Reader, sh ...SecondaryHeader) (*SpacePacket, error) {
	header := make([]byte, PrimaryHeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}

	totalPacketSize, err := CalculatePacketSize(header)
	if err != nil {
		return nil, err
	}

	if totalPacketSize < PrimaryHeaderSize {
		return nil, errors.New("calculated packet size is smaller than header size")
	}

	buffer := make([]byte, totalPacketSize)
	copy(buffer[:PrimaryHeaderSize], header)
	if _, err := io.ReadFull(reader, buffer[PrimaryHeaderSize:]); err != nil {
		return nil, err
	}

	return Decode(buffer, sh...)
}
