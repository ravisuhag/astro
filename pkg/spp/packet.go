package spp

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/crc"
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
		return nil, ErrEmptyPacket
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

// WithErrorControl enables the error control field on the SpacePacket.
// The CRC-16-CCITT checksum is computed automatically during Encode().
func WithErrorControl() PacketOption {
	return func(packet *SpacePacket) error {
		crc := uint16(0)
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
		crc := crc.ComputeCRC16(packetData)
		*sp.ErrorControl = crc
		packetData = append(packetData, byte(crc>>8), byte(crc&0xFF))
	}

	return packetData, nil
}

// DecodeOption configures optional decoding behavior.
type DecodeOption func(*decodeConfig)

type decodeConfig struct {
	sh           SecondaryHeader
	errorControl bool
}

// WithDecodeSecondaryHeader provides a SecondaryHeader implementation for decoding.
// If the packet's secondary header flag is set, this decoder will be used.
// Otherwise, secondary header bytes are included in UserData.
func WithDecodeSecondaryHeader(sh SecondaryHeader) DecodeOption {
	return func(cfg *decodeConfig) { cfg.sh = sh }
}

// WithDecodeErrorControl indicates the packet contains a trailing 2-byte error
// control field. The CRC is extracted, verified against the packet contents
// using CRC-16-CCITT, and stored in the decoded SpacePacket.
func WithDecodeErrorControl() DecodeOption {
	return func(cfg *decodeConfig) { cfg.errorControl = true }
}

// Decode parses a byte slice into a SpacePacket.
func Decode(data []byte, opts ...DecodeOption) (*SpacePacket, error) {
	var cfg decodeConfig
	for _, o := range opts {
		o(&cfg)
	}

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
	if primaryHeader.SecondaryHeaderFlag == 1 && cfg.sh != nil {
		secondaryHeader = cfg.sh
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

	// Extract error control field if expected
	var errorControl *uint16
	if cfg.errorControl {
		if remainingDataField < 2 {
			return nil, ErrDataTooShort
		}
		// Verify CRC over everything before the error control field
		crcOffset := PrimaryHeaderSize + dataFieldSize - 2
		expected := crc.ComputeCRC16(data[:crcOffset])
		actual := uint16(data[crcOffset])<<8 | uint16(data[crcOffset+1])
		if actual != expected {
			return nil, ErrCRCValidationFailed
		}
		errorControl = &actual
		remainingDataField -= 2
	}

	userData := data[offset : offset+remainingDataField]

	packet := &SpacePacket{
		PrimaryHeader:   primaryHeader,
		SecondaryHeader: secondaryHeader,
		UserData:        userData,
		ErrorControl:    errorControl,
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
		return ErrEmptyPacket
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

// IsIdle reports whether the packet is an idle packet (APID 0x7FF).
func (sp *SpacePacket) IsIdle() bool {
	return sp.PrimaryHeader.APID == 0x7FF
}

// PacketSizer returns the total length in bytes of the Space Packet starting
// at data[0], or -1 if the data is too short to determine length.
// It reads the Packet Data Length field (bytes 4-5) and returns
// total packet length: 6 (primary header) + PacketDataLength + 1.
func PacketSizer(data []byte) int {
	if len(data) < PrimaryHeaderSize {
		return -1
	}
	dataLen := int(binary.BigEndian.Uint16(data[4:6]))
	return PrimaryHeaderSize + 1 + dataLen
}
