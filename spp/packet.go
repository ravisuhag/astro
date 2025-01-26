package spp

import (
	"errors"
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
- Packet Length: Total length of the packet minus the primary header (6 bytes)
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
		return nil, errors.New("invalid APID: must be in range 0-2047")
	}

	packetLength := len(data) + PrimaryHeaderSize
	if packetLength < 7 || packetLength > 65542 {
		return nil, errors.New("packet length must be between 7 and 65542 octets")
	}

	// Default primary header
	primaryHeader := PrimaryHeader{
		Version:             0,
		Type:                packetType,
		SecondaryHeaderFlag: 0,
		APID:                apid,
		SequenceFlags:       3, // Default sequence flag (standalone packet)
		SequenceCount:       0,
		PacketLength:        uint16(len(data)) - 1, // Subtract 1 to represent the packet length in a 0-based manner
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
	if len(data) < 6 {
		return nil, ErrDataTooShort
	}

	// Decode primary header
	primaryHeader := PrimaryHeader{}
	if err := primaryHeader.Decode(data[:6]); err != nil {
		return nil, ErrInvalidHeader
	}

	offset := 6
	var secondaryHeader *SecondaryHeader

	// Decode secondary header if flag is set
	if primaryHeader.SecondaryHeaderFlag == 1 {
		if len(data) < offset+8 {
			return nil, ErrDataTooShort
		}
		secondaryHeader = &SecondaryHeader{}
		if err := secondaryHeader.Decode(data[offset : offset+8]); err != nil {
			return nil, ErrInvalidHeader
		}
		offset += 8
	}

	// Extract user data
	userDataEnd := len(data)
	if primaryHeader.PacketLength+6 < uint16(len(data)) {
		userDataEnd = int(primaryHeader.PacketLength) + 1 + 6
	}
	userData := data[offset:userDataEnd]
	offset += len(userData)

	// Parse error control if present
	var errorControl *uint16
	if len(data) >= offset+2 {
		crc := uint16(data[offset])<<8 | uint16(data[offset+1])
		errorControl = &crc
	}

	packet := &SpacePacket{
		PrimaryHeader:   primaryHeader,
		SecondaryHeader: secondaryHeader,
		UserData:        userData,
		ErrorControl:    errorControl,
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
			return errors.New("secondary header flag is set but secondary header is missing")
		}
		if err := sp.SecondaryHeader.Validate(); err != nil {
			return err
		}
	}

	// Validate user data length
	expectedLength := int(sp.PrimaryHeader.PacketLength) + 1
	actualLength := len(sp.UserData)
	if actualLength != expectedLength {
		return errors.New("user data length does not match packet length")
	}

	packetLength := PrimaryHeaderSize + actualLength
	if packetLength < 7 || packetLength > 65542 {
		return errors.New("packet length must be between 7 and 65542 octets")
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
