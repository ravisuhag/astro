package tmdl

import "encoding/binary"

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
		return nil, ErrDataTooLarge
	}

	secondaryHeader := SecondaryHeader{
		DataField: secondaryHeaderData,
	}
	if len(secondaryHeaderData) > 0 {
		// Per CCSDS 132.0-B-3 §4.1.3.2.2: HeaderLength = (Data Field octets) - 1
		secondaryHeader.HeaderLength = uint8(len(secondaryHeaderData) - 1)
	}

	frame := &TMTransferFrame{
		Header: PrimaryHeader{
			VersionNumber:    0b00,          // Default CCSDS TM version
			SpacecraftID:     scid & 0x03FF, // Mask to 10 bits
			VirtualChannelID: vcid & 0x07,   // Mask to 3 bits
			OCFFlag:          len(ocf) > 0,  // Set OCF flag if present
			FSHFlag:          len(secondaryHeaderData) > 0,
			MCFrameCount:     0, // To be set dynamically
			VCFrameCount:     0, // To be set dynamically
			SyncFlag:         false,
			PacketOrderFlag:  false,
			SegmentLengthID:  0b11, // Default segment length ID
			FirstHeaderPtr:   0,    // Default "no packet start" pointer
		},
		SecondaryHeader:    secondaryHeader,
		DataField:          data,
		OperationalControl: ocf,
	}
	// FirstHeaderPtr defaults to 0: first packet starts at byte 0 of Data Field.
	// Per CCSDS 132.0-B-3 §4.1.2.7.3, FirstHeaderPtr is relative to the
	// Transfer Frame Data Field (after the Secondary Header), not the frame payload.
	// VCA service sets SyncFlag=true and FirstHeaderPtr=0x07FF separately.

	// Compute Frame Error Control (CRC-16)
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}
	frame.FrameErrorControl = ComputeCRC(encoded)

	return frame, nil
}

// Encode converts the TM Transfer Frame to a byte slice.
func (tf *TMTransferFrame) Encode() ([]byte, error) {
	frameData, err := tf.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}

	// Append CRC-16
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
	return append(frameData, crcBytes...), nil
}

// EncodeWithoutFEC converts the frame to bytes excluding the CRC field.
func (tf *TMTransferFrame) EncodeWithoutFEC() ([]byte, error) {
	header, err := tf.Header.Encode()
	if err != nil {
		return nil, err
	}

	var secondaryHeader []byte

	// Only encode secondary header if FSHFlag is set
	if tf.Header.FSHFlag {
		secondaryHeader, err = tf.SecondaryHeader.Encode()
		if err != nil {
			return nil, err
		}
	}

	// Assemble full frame
	frameData := append(header, secondaryHeader...)
	frameData = append(frameData, tf.DataField...)
	if tf.Header.OCFFlag {
		frameData = append(frameData, tf.OperationalControl...)
	}

	return frameData, nil
}

// DecodeTMTransferFrame parses a byte slice into a TM Transfer Frame.
func DecodeTMTransferFrame(data []byte) (*TMTransferFrame, error) {
	if len(data) < 8 {
		return nil, ErrDataTooShort
	}

	// Decode Primary Header
	var header PrimaryHeader
	if err := header.Decode(data[:6]); err != nil {
		return nil, err
	}

	// Compute and verify CRC-16
	receivedCRC := binary.BigEndian.Uint16(data[len(data)-2:])
	computedCRC := ComputeCRC(data[:len(data)-2])
	if receivedCRC != computedCRC {
		return nil, ErrCRCMismatch
	}

	// Extract Data Field
	primaryHeaderLength := 6
	dataStart := primaryHeaderLength
	dataEnd := len(data) - 2
	operationalControl := []byte{}

	// Decode Secondary Header if present, using self-describing length
	var secondaryHeader SecondaryHeader
	if header.FSHFlag {
		if dataStart >= dataEnd {
			return nil, ErrDataTooShort
		}
		if err := secondaryHeader.Decode(data[dataStart:dataEnd]); err != nil {
			return nil, err
		}
		dataStart += 1 + len(secondaryHeader.DataField)
	}

	// Extract OCF if present
	if header.OCFFlag {
		if dataEnd-dataStart < 4 {
			return nil, ErrDataTooShort
		}
		operationalControl = make([]byte, 4)
		copy(operationalControl, data[dataEnd-4:dataEnd])
		dataEnd -= 4
	}

	// Extract main Data Field (copy to avoid aliasing caller's buffer)
	dataField := make([]byte, dataEnd-dataStart)
	copy(dataField, data[dataStart:dataEnd])

	// Construct and return the TMTransferFrame object
	return &TMTransferFrame{
		Header:             header,
		SecondaryHeader:    secondaryHeader,
		DataField:          dataField,
		OperationalControl: operationalControl,
		FrameErrorControl:  receivedCRC,
	}, nil
}
