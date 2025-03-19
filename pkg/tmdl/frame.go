package tmdl

import (
	"encoding/binary"
	"errors"
)

// TMTransferFrame represents a CCSDS TM Space Data Link Protocol Transfer Frame.
type TMTransferFrame struct {
	Header             PrimaryHeader
	SecondaryHeader    SecondaryHeader
	DataField          []byte // Main telemetry data
	OperationalControl []byte // 4-byte OCF (if used)
	FrameErrorControl  uint16 // 16-bit CRC (Error Control)
}

// NewTMTransferFrame initializes a new TM Transfer Frame.
func NewTMTransferFrame(scid uint16, vcid uint8, data []byte, secondaryHeaderData []byte, ocf []byte) (*TMTransferFrame, error) {
	if len(data) > 65535 {
		return nil, errors.New("data field exceeds maximum frame length")
	}

	secondaryHeader := SecondaryHeader{
		DataField: secondaryHeaderData,
	}

	frame := &TMTransferFrame{
		Header: PrimaryHeader{
			VersionNumber:    0b01,          // Default CCSDS TM version
			SpacecraftID:     scid & 0x03FF, // Mask to 10 bits
			VirtualChannelID: vcid & 0x3F,   // Mask to 6 bits
			OCFFlag:          len(ocf) > 0,  // Set OCF flag if present
			FSHFlag:          len(secondaryHeaderData) > 0,
			MCFrameCount:     0, // To be set dynamically
			VCFrameCount:     0, // To be set dynamically
			SyncFlag:         false,
			PacketOrderFlag:  false,
			SegmentLengthID:  0, // Default segment length ID
			FirstHeaderPtr:   0, // Default "no packet start" pointer
		},
		SecondaryHeader:    secondaryHeader,
		DataField:          data,
		OperationalControl: ocf,
	}
	if !frame.Header.SyncFlag {
		frame.Header.FirstHeaderPtr = uint16(len(secondaryHeaderData))
	} else {
		frame.Header.FirstHeaderPtr = 0xFFFF // Undefined when SyncFlag is set
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
	header := tf.Header.Encode()
	var secondaryHeader []byte
	var err error

	// Only encode secondary header if FSHFlag is set
	if tf.Header.FSHFlag {
		secondaryHeader, err = tf.SecondaryHeader.Encode()
		if err != nil {
			// Handle error or truncate to FirstHeaderPtr length
			secondaryHeader = secondaryHeader[:tf.Header.FirstHeaderPtr]
		}
	}

	// Assemble full frame
	frameData := append(header, secondaryHeader...)
	frameData = append(frameData, tf.DataField...)
	if tf.Header.OCFFlag {
		frameData = append(frameData, tf.OperationalControl...)
	}

	return frameData
}

// DecodeTMTransferFrame parses a byte slice into a TM Transfer Frame.
func DecodeTMTransferFrame(data []byte) (*TMTransferFrame, error) {
	if len(data) < 7 {
		return nil, errors.New("frame too short to be a valid TM Transfer Frame")
	}

	// Decode Primary Header
	header, err := (&PrimaryHeader{}).Decode(data[:6])
	if err != nil {
		return nil, err
	}

	// Compute and verify CRC-16
	receivedCRC := binary.BigEndian.Uint16(data[len(data)-2:])
	computedCRC := ComputeCRC(data[:len(data)-2])
	if receivedCRC != computedCRC {
		return nil, errors.New("CRC mismatch: received CRC does not match computed CRC")
	}

	// Extract Data Field
	primaryHeaderLength := 6
	dataStart := primaryHeaderLength
	dataEnd := len(data) - 2
	frameSecondaryData := []byte{}
	operationalControl := []byte{}

	// Extract Secondary Header if present
	if header.FSHFlag {
		if int(header.FirstHeaderPtr) > len(data)-primaryHeaderLength {
			return nil, errors.New("invalid FirstHeaderPtr value")
		}
		frameSecondaryData = data[dataStart : dataStart+int(header.FirstHeaderPtr)]
		dataStart += int(header.FirstHeaderPtr)
	}

	// Decode Secondary Header
	var secondaryHeader SecondaryHeader
	if header.FSHFlag {
		if err := secondaryHeader.Decode(frameSecondaryData); err != nil {
			return nil, err
		}
	}

	// Extract OCF if present
	if header.OCFFlag && dataEnd-dataStart >= 4 {
		operationalControl = data[dataEnd-4 : dataEnd]
		dataEnd -= 4
	}

	// Extract main Data Field
	dataField := data[dataStart:dataEnd]

	// Construct and return the TMTransferFrame object
	return &TMTransferFrame{
		Header:             *header,
		SecondaryHeader:    secondaryHeader,
		DataField:          dataField,
		OperationalControl: operationalControl,
		FrameErrorControl:  receivedCRC,
	}, nil
}
