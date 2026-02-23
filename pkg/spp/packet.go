package spp

import (
	"strconv"
	"strings"
)

/*
Space Packet Protocol (SPP):

+----------------+----------------+----------------+----------------+
| Version (3b)  | Type (1b)      | SecondaryHeader| APID (11b)     |
|               |                | Flag (1b)      |                |
+----------------+----------------+----------------+----------------+
| Sequence Flags| Sequence Count (14b)                            |
| (2b)          |                                                 |
+----------------+----------------+----------------+----------------+
| Packet Length (16b)                                             |
+----------------+----------------+----------------+----------------+
| Secondary Header (Optional, mission-specific)                  |
|                                                                |
+----------------+----------------+----------------+----------------+
| User Data Field (Variable Length)                              |
|                                                                |
|                                                                |
+----------------+----------------+----------------+----------------+
| Error Control Field (Optional, 16b CRC)                       |
+----------------+----------------+----------------+----------------+

Legend:
- b = bits
- APID = Application Process Identifier
- Sequence Flags: 00 (continuation), 01 (start), 10 (end), 11 (standalone)
- Packet Length: (Packet Data Field size) - 1, where Data Field = Secondary Header + User Data + Error Control
*/

// SpacePacket represents a complete space packet as per CCSDS standards.
type SpacePacket struct {
	PrimaryHeader   PrimaryHeader    // The primary header of the space packet
	SecondaryHeader *SecondaryHeader // Optional secondary header
	UserData        []byte           // User data contained in the packet
	ErrorControl    *uint16          // Optional error control field (e.g., CRC)
}

// NewSpacePacket creates a new SpacePacket instance.
func NewSpacePacket(apid uint16, packetType uint8, data []byte, options ...PacketOption) (*SpacePacket, error) {
	if apid > 2047 {
		return nil, ErrInvalidAPID
	}

	if len(data) < 1 {
		return nil, ErrDataTooShort
	}

	// Default primary header
	primaryHeader := PrimaryHeader{
		Version:             0,
		Type:                packetType,
		SecondaryHeaderFlag: 0,
		APID:                apid,
		SequenceFlags:       3, // Default sequence flag (standalone packet)
		SequenceCount:       0,
		PacketLength:        0, // Calculated after options are applied
	}

	// Initialize SpacePacket
	packet := &SpacePacket{
		PrimaryHeader: primaryHeader,
		UserData:      data,
	}

	// Apply optional configurations
	for _, option := range options {
		if err := option(packet); err != nil {
			return nil, err
		}
	}

	// Calculate PacketLength per CCSDS: (packet data field size) - 1
	// Packet data field = secondary header + user data + error control
	dataFieldSize := len(packet.UserData)
	if packet.SecondaryHeader != nil {
		shBytes, err := packet.SecondaryHeader.Encode()
		if err != nil {
			return nil, err
		}
		dataFieldSize += len(shBytes)
	}
	if packet.ErrorControl != nil {
		dataFieldSize += 2
	}

	totalPacketSize := PrimaryHeaderSize + dataFieldSize
	if totalPacketSize < 7 || totalPacketSize > 65542 {
		return nil, ErrPacketTooLarge
	}

	packet.PrimaryHeader.PacketLength = uint16(dataFieldSize) - 1

	// Validate the packet
	if err := packet.Validate(); err != nil {
		return nil, err
	}

	return packet, nil
}

func NewTMPacket(apid uint16, data []byte, options ...PacketOption) (*SpacePacket, error) {
	return NewSpacePacket(apid, 0, data, options...)
}

func NewTCPacket(apid uint16, data []byte, options ...PacketOption) (*SpacePacket, error) {
	return NewSpacePacket(apid, 1, data, options...)
}

// PacketOption defines a function type for configuring SpacePacket options.
type PacketOption func(*SpacePacket) error

// WithSecondaryHeader adds a secondary header to the SpacePacket.
func WithSecondaryHeader(header SecondaryHeader) PacketOption {
	return func(packet *SpacePacket) error {
		packet.PrimaryHeader.SecondaryHeaderFlag = 1
		packet.SecondaryHeader = &header
		return nil
	}
}

// WithErrorControl adds an error control field to the SpacePacket.
func WithErrorControl(crc uint16) PacketOption {
	return func(packet *SpacePacket) error {
		packet.ErrorControl = &crc
		return nil
	}
}

// Encode converts the SpacePacket into a byte slice for transmission.
func (sp *SpacePacket) Encode() ([]byte, error) {
	// Encode primary header
	headerBytes, err := sp.PrimaryHeader.Encode()
	if err != nil {
		return nil, ErrInvalidHeader
	}

	packetData := append([]byte{}, headerBytes...)

	// Encode secondary header if present
	if sp.PrimaryHeader.SecondaryHeaderFlag == 1 && sp.SecondaryHeader != nil {
		secondaryBytes, err := sp.SecondaryHeader.Encode()
		if err != nil {
			return nil, ErrInvalidHeader
		}
		packetData = append(packetData, secondaryBytes...)
	} else if sp.PrimaryHeader.SecondaryHeaderFlag == 1 && sp.SecondaryHeader == nil {
		return nil, ErrSecondaryHeaderMissing
	}

	// Append user data
	packetData = append(packetData, sp.UserData...)

	// Append error control field if present
	if sp.ErrorControl != nil {
		crcBytes := make([]byte, 2)
		crcBytes[0] = byte(*sp.ErrorControl >> 8)
		crcBytes[1] = byte(*sp.ErrorControl & 0xFF)
		packetData = append(packetData, crcBytes...)
	}

	return packetData, nil
}

// Decode parses a byte slice into a SpacePacket.
func Decode(data []byte) (*SpacePacket, error) {
	if len(data) < 7 {
		return nil, ErrDataTooShort
	}

	// Decode primary header
	primaryHeader := PrimaryHeader{}
	if err := primaryHeader.Decode(data[:6]); err != nil {
		return nil, ErrInvalidHeader
	}

	// Per CCSDS: total packet = primary header + packet data field
	dataFieldSize := int(primaryHeader.PacketLength) + 1
	totalPacketSize := PrimaryHeaderSize + dataFieldSize
	if len(data) < totalPacketSize {
		return nil, ErrDataTooShort
	}

	offset := PrimaryHeaderSize
	remainingDataField := dataFieldSize
	var secondaryHeader *SecondaryHeader

	// Decode secondary header if flag is set
	if primaryHeader.SecondaryHeaderFlag == 1 {
		if remainingDataField < 8 {
			return nil, ErrDataTooShort
		}
		secondaryHeader = &SecondaryHeader{}
		if err := secondaryHeader.Decode(data[offset : offset+8]); err != nil {
			return nil, ErrInvalidHeader
		}
		offset += 8
		remainingDataField -= 8
	}

	// Remaining data field is user data
	userData := data[offset : offset+remainingDataField]

	packet := &SpacePacket{
		PrimaryHeader:   primaryHeader,
		SecondaryHeader: secondaryHeader,
		UserData:        userData,
	}

	// Validate the packet
	if err := packet.Validate(); err != nil {
		return nil, err
	}

	return packet, nil
}

// Validate checks the integrity and correctness of the SpacePacket.
func (sp *SpacePacket) Validate() error {
	// Validate primary header
	if err := sp.PrimaryHeader.Validate(); err != nil {
		return err
	}

	// Validate secondary header if present
	if sp.PrimaryHeader.SecondaryHeaderFlag == 1 {
		if sp.SecondaryHeader == nil {
			return ErrSecondaryHeaderMissing
		}
		if err := sp.SecondaryHeader.Validate(); err != nil {
			return err
		}
	}

	// Calculate total packet data field size per CCSDS
	// Packet data field = secondary header + user data + error control
	dataFieldSize := len(sp.UserData)
	if sp.SecondaryHeader != nil {
		shBytes, err := sp.SecondaryHeader.Encode()
		if err != nil {
			return err
		}
		dataFieldSize += len(shBytes)
	}
	if sp.ErrorControl != nil {
		dataFieldSize += 2
	}

	expectedLength := int(sp.PrimaryHeader.PacketLength) + 1
	if dataFieldSize != expectedLength {
		return ErrPacketLengthMismatch
	}

	totalPacketSize := PrimaryHeaderSize + dataFieldSize
	if totalPacketSize < 7 || totalPacketSize > 65542 {
		return ErrPacketTooLarge
	}

	return nil
}

// Humanize generates a human-readable representation of the SpacePacket.
func (sp *SpacePacket) Humanize() string {
	var builder strings.Builder
	builder.WriteString("SpacePacket Information:\n")
	builder.WriteString("Primary Header:\n")
	builder.WriteString(sp.PrimaryHeader.Humanize())

	if sp.SecondaryHeader != nil {
		builder.WriteString("\nSecondary Header:\n")
		builder.WriteString(sp.SecondaryHeader.Humanize())
	}

	if sp.ErrorControl != nil {
		builder.WriteString("\nError Control: ")
		builder.WriteString(strconv.Itoa(int(*sp.ErrorControl)))
	}

	return builder.String()
}
