package epp

import (
	"strconv"
	"strings"
)

/*
Encapsulation Packet Protocol (EPP):

Per CCSDS 133.1-B-3, an Encapsulation Packet consists of:
  - A variable-length Packet Header (1, 2, 4, or 8 bytes)
  - A Data Zone (variable length, containing encapsulated protocol data)

The first four bits of the header are always 0111 (PVN=7), which distinguishes
Encapsulation Packets from Space Packets (PVN=0).

The Protocol ID field identifies the encapsulated payload type (e.g., IPv4/IPv6,
user-defined, or an extended protocol via extension byte).

Unlike SPP, EPP has no APID, sequence count, or error control field — it is a
thin encapsulation shim designed to carry network-layer PDUs over space links.
*/

// EncapsulationPacket represents a complete Encapsulation Packet per CCSDS 133.1-B-3.
type EncapsulationPacket struct {
	Header Header // Variable-length packet header
	Data   []byte // Data zone (encapsulated protocol data)
}

// PacketOption defines a function type for configuring EncapsulationPacket options.
type PacketOption func(*EncapsulationPacket) error

// WithUserDefined sets the user-defined field (Format 3 header).
// This also sets LengthOfLength to 1 to select the medium header format.
func WithUserDefined(value uint8) PacketOption {
	return func(ep *EncapsulationPacket) error {
		ep.Header.LengthOfLength = 1
		ep.Header.UserDefined = value
		return nil
	}
}

// WithExtendedProtocolID sets an extended protocol ID (Formats 4 and 5).
// This sets ProtocolID to 7 (Protocol ID Extension).
func WithExtendedProtocolID(extPID uint8) PacketOption {
	return func(ep *EncapsulationPacket) error {
		ep.Header.ProtocolID = ProtocolIDExtended
		ep.Header.ExtendedProtocolID = extPID
		return nil
	}
}

// WithCCSDSDefined sets the CCSDS-defined field (Format 5 header).
// This also sets LengthOfLength to 1 and ProtocolID to 7.
func WithCCSDSDefined(extPID uint8, value uint16) PacketOption {
	return func(ep *EncapsulationPacket) error {
		ep.Header.ProtocolID = ProtocolIDExtended
		ep.Header.LengthOfLength = 1
		ep.Header.ExtendedProtocolID = extPID
		ep.Header.CCSDSDefined = value
		return nil
	}
}

// WithLongLength forces the use of longer header formats by setting LengthOfLength to 1.
// For standard Protocol IDs this selects Format 3 (4-byte header with 16-bit length).
// For extended Protocol IDs this selects Format 5 (8-byte header with 32-bit length).
func WithLongLength() PacketOption {
	return func(ep *EncapsulationPacket) error {
		ep.Header.LengthOfLength = 1
		return nil
	}
}

// NewPacket creates a new EncapsulationPacket with the given Protocol ID and data.
func NewPacket(protocolID uint8, data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	if protocolID > 7 {
		return nil, ErrInvalidProtocolID
	}

	ep := &EncapsulationPacket{
		Header: Header{
			PVN:            PVN,
			ProtocolID:     protocolID,
			LengthOfLength: 0,
		},
		Data: data,
	}

	for _, option := range options {
		if err := option(ep); err != nil {
			return nil, err
		}
	}

	// Idle packets: PID=0 is always idle per CCSDS 133.1-B-3 Section 4.1.2.3.
	// Force Format 1 (LoL=0, 1-byte header, no data zone).
	if ep.Header.ProtocolID == ProtocolIDIdle {
		if len(data) > 0 {
			return nil, ErrIdleWithData
		}
		ep.Header.LengthOfLength = 0
		ep.Header.PacketLength = 1
		if err := ep.Validate(); err != nil {
			return nil, err
		}
		return ep, nil
	}

	// Non-idle packets must have data
	if len(ep.Data) == 0 {
		return nil, ErrEmptyData
	}

	// Calculate and validate packet length
	headerSize := ep.Header.Size()
	totalSize := uint64(headerSize) + uint64(len(ep.Data))

	// Check against maximum for the header format
	format := ep.Header.Format()
	switch format {
	case 2:
		if totalSize > MaxPacketLengthShort {
			return nil, ErrPacketTooLarge
		}
	case 3, 4:
		if totalSize > MaxPacketLengthMedium {
			return nil, ErrPacketTooLarge
		}
	case 5:
		if totalSize > MaxPacketLengthExtendedLong {
			return nil, ErrPacketTooLarge
		}
	}

	ep.Header.PacketLength = uint32(totalSize)

	if err := ep.Validate(); err != nil {
		return nil, err
	}

	return ep, nil
}

// NewIdlePacket creates an idle Encapsulation Packet (Protocol ID = 0, 1-byte header).
func NewIdlePacket() (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDIdle, nil)
}

// NewIPEPacket creates an Internet Protocol Extension Encapsulation Packet.
func NewIPEPacket(data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDIPE, data, options...)
}

// NewUserDefinedPacket creates a user-defined Encapsulation Packet.
func NewUserDefinedPacket(data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDUserDef, data, options...)
}

// Encode converts the EncapsulationPacket into a byte slice for transmission.
func (ep *EncapsulationPacket) Encode() ([]byte, error) {
	headerBytes, err := ep.Header.Encode()
	if err != nil {
		return nil, err
	}

	result := append([]byte{}, headerBytes...)
	result = append(result, ep.Data...)

	return result, nil
}

// Decode parses a byte slice into an EncapsulationPacket.
// The returned packet's Data field is a sub-slice of the input and shares
// the same backing array. Callers that reuse the input buffer should copy
// the Data field before modifying the buffer.
func Decode(data []byte) (*EncapsulationPacket, error) {
	if len(data) < 1 {
		return nil, ErrDataTooShort
	}

	header := Header{}
	if err := header.Decode(data); err != nil {
		return nil, err
	}

	headerSize := header.Size()

	// For idle packets (Format 1), there is no data zone
	if header.Format() == 1 {
		ep := &EncapsulationPacket{
			Header: header,
		}
		return ep, nil
	}

	// Verify packet length is at least as large as the header
	totalSize := int(header.PacketLength)
	if totalSize < headerSize {
		return nil, ErrPacketLengthMismatch
	}

	// Verify we have enough data for the declared packet length
	if len(data) < totalSize {
		return nil, ErrDataTooShort
	}

	ep := &EncapsulationPacket{
		Header: header,
		Data:   data[headerSize:totalSize],
	}

	if err := ep.Validate(); err != nil {
		return nil, err
	}

	return ep, nil
}

// Validate checks the integrity and correctness of the EncapsulationPacket.
func (ep *EncapsulationPacket) Validate() error {
	if err := ep.Header.Validate(); err != nil {
		return err
	}

	format := ep.Header.Format()

	// Idle packets must have no data
	if format == 1 {
		if len(ep.Data) > 0 {
			return ErrIdleWithData
		}
		return nil
	}

	// Non-idle packets must have data
	if len(ep.Data) == 0 {
		return ErrEmptyData
	}

	// Verify packet length matches actual size
	expectedLength := ep.Header.Size() + len(ep.Data)
	if uint32(expectedLength) != ep.Header.PacketLength {
		return ErrPacketLengthMismatch
	}

	return nil
}

// IsIdle reports whether the packet is an idle packet (Protocol ID = 0).
func (ep *EncapsulationPacket) IsIdle() bool {
	return ep.Header.ProtocolID == ProtocolIDIdle && ep.Header.LengthOfLength == 0
}

// Humanize generates a human-readable representation of the EncapsulationPacket.
func (ep *EncapsulationPacket) Humanize() string {
	var builder strings.Builder
	builder.WriteString("EncapsulationPacket Information:\n")
	builder.WriteString("Header:\n")
	builder.WriteString(ep.Header.Humanize())

	builder.WriteString("\nData Zone: ")
	builder.WriteString(strconv.Itoa(len(ep.Data)))
	builder.WriteString(" bytes")

	return builder.String()
}

// PacketSizer returns the total length in bytes of the Encapsulation Packet
// starting at data[0], or -1 if the data is too short to determine length.
// This implements the sdl.PacketSizer signature for use with data link services.
func PacketSizer(data []byte) int {
	if len(data) < 1 {
		return -1
	}

	hdrSize := HeaderSize(data)
	if hdrSize < 0 {
		return -1
	}

	// Idle packets are exactly 1 byte
	pid := (data[0] >> 1) & 0x07
	lol := data[0] & 0x01
	if pid == ProtocolIDIdle && lol == 0 {
		return 1
	}

	if len(data) < hdrSize {
		return -1
	}

	// Read packet length from the header
	var h Header
	if err := h.Decode(data[:hdrSize]); err != nil {
		return -1
	}

	pktLen := int(h.PacketLength)
	if pktLen < hdrSize {
		return -1
	}

	return pktLen
}
