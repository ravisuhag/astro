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
		HeaderLength: uint8(len(secondaryHeaderData)),
		DataField:    secondaryHeaderData,
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
	if !frame.Header.SyncFlag {
		// FirstHeaderPtr = total encoded secondary header size (1 prefix byte + data)
		if len(secondaryHeaderData) > 0 {
			frame.Header.FirstHeaderPtr = uint16(1 + len(secondaryHeaderData))
		}
	} else {
		frame.Header.FirstHeaderPtr = 0x07FF // Undefined when SyncFlag is set (all 1s in 11 bits)
	}

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
	if len(data) < 7 {
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
	frameSecondaryData := []byte{}
	operationalControl := []byte{}

	// Extract Secondary Header if present
	if header.FSHFlag {
		if int(header.FirstHeaderPtr) > len(data)-primaryHeaderLength {
			return nil, ErrInvalidFirstHeaderPtr
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
		Header:             header,
		SecondaryHeader:    secondaryHeader,
		DataField:          dataField,
		OperationalControl: operationalControl,
		FrameErrorControl:  receivedCRC,
	}, nil
}
