package tmdl

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// TMTransferFrame represents a CCSDS TM Space Data Link Protocol Transfer Frame.
type TMTransferFrame struct {
	VersionNumber        uint8  // 2 bits
	SpacecraftID         uint16 // 10 bits
	VirtualChannelID     uint8  // 6 bits
	FrameLength          uint16 // Length of the frame
	FrameSecondaryHeader []byte // Optional secondary header
	DataField            []byte // Main telemetry data
	OperationalControl   []byte // 4-byte OCF (if used)
	FrameErrorControl    uint16 // 16-bit CRC (Error Control)
}

// NewTMTransferFrame initializes a new TM Transfer Frame.
func NewTMTransferFrame(scid uint16, vcid uint8, data []byte, secondaryHeader []byte, ocf []byte) (*TMTransferFrame, error) {
	if len(data) > 65535 {
		return nil, errors.New("data field exceeds maximum frame length")
	}

	frame := &TMTransferFrame{
		VersionNumber:        0b01,                                                        // Default CCSDS TM version
		SpacecraftID:         scid & 0x03FF,                                               // Mask to 10 bits
		VirtualChannelID:     vcid & 0x3F,                                                 // Mask to 6 bits
		FrameLength:          uint16(5 + len(secondaryHeader) + len(data) + len(ocf) + 2), // Total frame length including headers and CRC
		FrameSecondaryHeader: secondaryHeader,
		DataField:            data,
		OperationalControl:   ocf,
	}

	// Compute Frame Error Control (CRC-16)
	frame.FrameErrorControl = ComputeCRC(frame.EncodeWithoutFEC())

	return frame, nil
}

// Encode converts the TM Transfer Frame to a byte slice.
func (tf *TMTransferFrame) Encode() []byte {
	frameData := tf.EncodeWithoutFEC()

	// Append CRC-16
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
	return append(frameData, crcBytes...)
}

// EncodeWithoutFEC converts the frame to bytes excluding the CRC field.
func (tf *TMTransferFrame) EncodeWithoutFEC() []byte {
	header := make([]byte, 5)

	// First 5 bytes: TFVN, SCID, VCID, Length
	header[0] = (tf.VersionNumber << 6) | byte(tf.SpacecraftID>>8)
	header[1] = byte(tf.SpacecraftID & 0xFF)
	header[2] = tf.VirtualChannelID
	binary.BigEndian.PutUint16(header[3:], tf.FrameLength)

	// Assemble full frame
	frameData := append(header, tf.FrameSecondaryHeader...)
	frameData = append(frameData, tf.DataField...)
	frameData = append(frameData, tf.OperationalControl...)

	return frameData
}

// DecodeTMTransferFrame parses a byte slice into a TM Transfer Frame.
func DecodeTMTransferFrame(data []byte) (*TMTransferFrame, error) {
	if len(data) < 7 {
		return nil, errors.New("frame too short to be a valid TM Transfer Frame")
	}

	// Extract Version Number, SCID, and VCID
	version := (data[0] >> 6) & 0x03
	scid := (uint16(data[0]&0x03) << 8) | uint16(data[1])
	vcid := data[2]

	// Extract Frame Length
	frameLength := binary.BigEndian.Uint16(data[3:5])

	// Check if the received frame length matches the actual data length
	if int(frameLength) != len(data) {
		return nil, fmt.Errorf("frame length mismatch: expected %d, got %d", frameLength, len(data))
	}

	// Compute and verify CRC-16
	receivedCRC := binary.BigEndian.Uint16(data[len(data)-2:])
	computedCRC := ComputeCRC(data[:len(data)-2])
	if receivedCRC != computedCRC {
		return nil, fmt.Errorf("CRC mismatch: expected %04X, got %04X", receivedCRC, computedCRC)
	}

	// Extract Data Field
	dataStart := 5
	dataEnd := len(data) - 2
	frameSecondaryHeader := []byte{}
	operationalControl := []byte{}

	// Check if Secondary Header exists (Mission-dependent)
	if dataStart < dataEnd {
		frameSecondaryHeader = data[dataStart : dataStart+2] // Assuming a 2-byte header
		dataStart += 2
	}

	// Extract Operational Control Field (OCF) if present
	if dataEnd-dataStart >= 4 {
		operationalControl = data[dataEnd-4 : dataEnd]
		dataEnd -= 4
	}

	// Extract the main Data Field
	dataField := data[dataStart:dataEnd]

	// Construct the TMTransferFrame object
	frame := &TMTransferFrame{
		VersionNumber:        version,
		SpacecraftID:         scid,
		VirtualChannelID:     vcid,
		FrameLength:          frameLength,
		FrameSecondaryHeader: frameSecondaryHeader,
		DataField:            dataField,
		OperationalControl:   operationalControl,
		FrameErrorControl:    receivedCRC,
	}

	return frame, nil
}
