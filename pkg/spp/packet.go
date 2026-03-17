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
| Secondary Header (Optional, mission-specific, 1-63 bytes)      |
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
	PrimaryHeader   PrimaryHeader   // The primary header of the space packet
	SecondaryHeader SecondaryHeader // Optional mission-specific secondary header
	UserData        []byte          // User data contained in the packet
	ErrorControl    *uint16         // Optional error control field (e.g., CRC)
}

// NewSpacePacket creates a new SpacePacket instance.
// Per CCSDS C1/C2: a packet must contain at least a secondary header or user data.
// User data may be nil/empty if a secondary header is provided, and vice versa.
func NewSpacePacket(apid uint16, packetType uint8, data []byte, options ...PacketOption) (*SpacePacket, error) {
	if apid > 2047 {
		return nil, ErrInvalidAPID
	}

	primaryHeader := PrimaryHeader{
		Version:             0,
		Type:                packetType,
		SecondaryHeaderFlag: 0,
		APID:                apid,
		SequenceFlags:       SeqFlagUnsegmented,
		SequenceCount:       0,
		PacketLength:        0, // Calculated after options are applied
	}

	packet := &SpacePacket{
		PrimaryHeader: primaryHeader,
		UserData:      data,
	}

	for _, option := range options {
		if err := option(packet); err != nil {
			return nil, err
		}
	}

	// CCSDS C1/C2: packet must contain at least a secondary header or user data
	if len(packet.UserData) == 0 && packet.SecondaryHeader == nil {
		return nil, ErrDataTooShort
	}

	// Calculate PacketLength per CCSDS: (packet data field size) - 1
	// Packet data field = secondary header + user data + error control
	dataFieldSize := len(packet.UserData)
	if packet.SecondaryHeader != nil {
		dataFieldSize += packet.SecondaryHeader.Size()
	}
	if packet.ErrorControl != nil {
		dataFieldSize += 2
	}

	totalPacketSize := PrimaryHeaderSize + dataFieldSize
	if totalPacketSize < 7 || totalPacketSize > 65542 {
		return nil, ErrPacketTooLarge
	}

	packet.PrimaryHeader.PacketLength = uint16(dataFieldSize) - 1

	if err := packet.Validate(); err != nil {
		return nil, err
	}

	return packet, nil
}

// NewTMPacket creates a new telemetry SpacePacket.
func NewTMPacket(apid uint16, data []byte, options ...PacketOption) (*SpacePacket, error) {
	return NewSpacePacket(apid, PacketTypeTM, data, options...)
}

// NewTCPacket creates a new telecommand SpacePacket.
func NewTCPacket(apid uint16, data []byte, options ...PacketOption) (*SpacePacket, error) {
	return NewSpacePacket(apid, PacketTypeTC, data, options...)
}

// PacketOption defines a function type for configuring SpacePacket options.
type PacketOption func(*SpacePacket) error

// WithSecondaryHeader adds a secondary header to the SpacePacket.
func WithSecondaryHeader(header SecondaryHeader) PacketOption {
	return func(packet *SpacePacket) error {
		if err := validateSecondaryHeader(header); err != nil {
			return err
		}
		packet.PrimaryHeader.SecondaryHeaderFlag = 1
		packet.SecondaryHeader = header
		return nil
	}
}

// WithSequenceCount sets the sequence count on the SpacePacket.
// Use this when constructing packets outside of a Service.
func WithSequenceCount(n uint16) PacketOption {
	return func(packet *SpacePacket) error {
		if n > 16383 {
			return ErrInvalidSequenceCount
		}
		packet.PrimaryHeader.SequenceCount = n
		return nil
	}
}

// WithSequenceFlags sets the sequence flags on the SpacePacket.
func WithSequenceFlags(flags uint8) PacketOption {
	return func(packet *SpacePacket) error {
		if flags > 3 {
			return ErrInvalidSequenceFlags
		}
		packet.PrimaryHeader.SequenceFlags = flags
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
	headerBytes, err := sp.PrimaryHeader.Encode()
	if err != nil {
		return nil, err
	}

	packetData := append([]byte{}, headerBytes...)

	// Encode secondary header if present
	if sp.PrimaryHeader.SecondaryHeaderFlag == 1 {
		if sp.SecondaryHeader == nil {
			return nil, ErrSecondaryHeaderMissing
		}
		secondaryBytes, err := sp.SecondaryHeader.Encode()
		if err != nil {
			return nil, err
		}
		packetData = append(packetData, secondaryBytes...)
	}

	packetData = append(packetData, sp.UserData...)

	if sp.ErrorControl != nil {
		crcBytes := make([]byte, 2)
		crcBytes[0] = byte(*sp.ErrorControl >> 8)
		crcBytes[1] = byte(*sp.ErrorControl & 0xFF)
		packetData = append(packetData, crcBytes...)
	}

	return packetData, nil
}

// Decode parses a byte slice into a SpacePacket.
// If the packet has a secondary header (flag = 1) and a SecondaryHeader
// implementation is provided, it will be used to decode the secondary header.
// Otherwise, secondary header bytes are included in UserData.
func Decode(data []byte, sh ...SecondaryHeader) (*SpacePacket, error) {
	if len(data) < 7 {
		return nil, ErrDataTooShort
	}

	primaryHeader := PrimaryHeader{}
	if err := primaryHeader.Decode(data[:6]); err != nil {
		return nil, err
	}

	// Per CCSDS: total packet = primary header + packet data field
	dataFieldSize := int(primaryHeader.PacketLength) + 1
	totalPacketSize := PrimaryHeaderSize + dataFieldSize
	if len(data) < totalPacketSize {
		return nil, ErrDataTooShort
	}

	offset := PrimaryHeaderSize
	remainingDataField := dataFieldSize
	var secondaryHeader SecondaryHeader

	// Decode secondary header if flag is set and a decoder is provided
	if primaryHeader.SecondaryHeaderFlag == 1 && len(sh) > 0 && sh[0] != nil {
		secondaryHeader = sh[0]
		shSize := secondaryHeader.Size()
		if remainingDataField < shSize {
			return nil, ErrDataTooShort
		}
		if err := secondaryHeader.Decode(data[offset : offset+shSize]); err != nil {
			return nil, err
		}
		offset += shSize
		remainingDataField -= shSize
	}

	userData := data[offset : offset+remainingDataField]

	packet := &SpacePacket{
		PrimaryHeader:   primaryHeader,
		SecondaryHeader: secondaryHeader,
		UserData:        userData,
	}

	if err := packet.Validate(); err != nil {
		return nil, err
	}

	return packet, nil
}

// Validate checks the integrity and correctness of the SpacePacket.
func (sp *SpacePacket) Validate() error {
	if err := sp.PrimaryHeader.Validate(); err != nil {
		return err
	}

	// Validate secondary header structural constraints
	if sp.SecondaryHeader != nil {
		if err := validateSecondaryHeader(sp.SecondaryHeader); err != nil {
			return err
		}
	}

	// CCSDS C1/C2: packet must contain at least a secondary header or user data.
	// Note: when the secondary header flag is set but no decoder was provided
	// during Decode(), the secondary header bytes are included in UserData,
	// so this check still holds.
	if len(sp.UserData) == 0 && sp.SecondaryHeader == nil {
		return ErrDataTooShort
	}

	// Calculate total packet data field size per CCSDS
	dataFieldSize := len(sp.UserData)
	if sp.SecondaryHeader != nil {
		dataFieldSize += sp.SecondaryHeader.Size()
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
		builder.WriteString("\nSecondary Header: present (")
		builder.WriteString(strconv.Itoa(sp.SecondaryHeader.Size()))
		builder.WriteString(" bytes)")
	}

	if sp.ErrorControl != nil {
		builder.WriteString("\nError Control: ")
		builder.WriteString(strconv.Itoa(int(*sp.ErrorControl)))
	}

	return builder.String()
}
