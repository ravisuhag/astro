package epp

import (
	"math"
	"strconv"
	"strings"
)

/*
Encapsulation Packet Protocol (EPP):

Per CCSDS 133.1-B-3, an Encapsulation Packet consists of:
  - A variable-length Packet Header (1, 2, 4, or 8 bytes, selected by the
    2-bit Length of Length field)
  - An Encapsulated Data Field (variable length)

The first three bits of the header are always '111' (PVN=7), which
distinguishes Encapsulation Packets from Space Packets (PVN=0). A 1-octet
idle packet therefore encodes as the single byte 0xE0.

The Protocol ID field identifies the encapsulated payload type (e.g., LTP,
Internet Protocol Extension, or an extended protocol via the 4-bit Protocol
ID Extension field when the Protocol ID is '110').

The Packet Length field carries the total packet length in octets, header
included (4.1.2.8.2).

Unlike SPP, EPP has no APID, sequence count, or error control field. It is a
thin encapsulation shim designed to carry network-layer PDUs over space links.
*/

// EncapsulationPacket represents a complete Encapsulation Packet per CCSDS 133.1-B-3.
type EncapsulationPacket struct {
	Header Header // Variable-length packet header
	Data   []byte // Encapsulated Data Field
}

// PacketOption defines a function type for configuring EncapsulationPacket options.
type PacketOption func(*EncapsulationPacket) error

// bumpLoL raises the header's Length of Length to at least min.
func bumpLoL(ep *EncapsulationPacket, min uint8) {
	if ep.Header.LengthOfLength < min {
		ep.Header.LengthOfLength = min
	}
}

// WithUserDefined sets the 4-bit User Defined Field, present in 4- and
// 8-octet headers. This raises the header size to at least 4 octets.
func WithUserDefined(value uint8) PacketOption {
	return func(ep *EncapsulationPacket) error {
		if value > 0x0F {
			return ErrInvalidUserDefined
		}
		ep.Header.UserDefined = value
		bumpLoL(ep, LoL2Octet)
		return nil
	}
}

// WithExtendedProtocolID sets the 4-bit Protocol ID Extension field and the
// Protocol ID to '110' (ProtocolIDExtended). This raises the header size to
// at least 4 octets, since only 4- and 8-octet headers carry the field.
func WithExtendedProtocolID(extPID uint8) PacketOption {
	return func(ep *EncapsulationPacket) error {
		if extPID > 0x0F {
			return ErrInvalidExtendedProtocolID
		}
		ep.Header.ProtocolID = ProtocolIDExtended
		ep.Header.ExtendedProtocolID = extPID
		bumpLoL(ep, LoL2Octet)
		return nil
	}
}

// WithCCSDSDefined sets the 2-octet CCSDS Defined Field, present only in the
// 8-octet header. This forces the 8-octet header. The field is reserved by
// CCSDS and is by convention 'all zeros' (4.1.2.7.2).
func WithCCSDSDefined(value uint16) PacketOption {
	return func(ep *EncapsulationPacket) error {
		ep.Header.CCSDSDefined = value
		bumpLoL(ep, LoL4Octet)
		return nil
	}
}

// WithLongLength forces at least a 4-octet header (2-octet Packet Length
// field). NewPacket otherwise picks the smallest header that fits the data.
func WithLongLength() PacketOption {
	return func(ep *EncapsulationPacket) error {
		bumpLoL(ep, LoL2Octet)
		return nil
	}
}

// NewPacket creates a new EncapsulationPacket with the given Protocol ID and
// data. The smallest header that fits the data is selected, unless an option
// forces a larger one. Protocol ID 0 with no data yields the 1-octet idle
// packet; Protocol ID 0 with data yields a multi-octet idle fill packet.
func NewPacket(protocolID uint8, data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	if protocolID > 7 {
		return nil, ErrInvalidProtocolID
	}

	ep := &EncapsulationPacket{
		Header: Header{
			PVN:            PVN,
			ProtocolID:     protocolID,
			LengthOfLength: LoLNone,
		},
		Data: data,
	}

	for _, option := range options {
		if err := option(ep); err != nil {
			return nil, err
		}
	}

	minLoL := ep.Header.LengthOfLength

	// PID '110' needs the Protocol ID Extension field, which only 4- and
	// 8-octet headers carry.
	if ep.Header.ProtocolID == ProtocolIDExtended && minLoL < LoL2Octet {
		minLoL = LoL2Octet
	}

	// 1-octet idle packet: idle protocol ID, no data, no forced longer header.
	if ep.Header.ProtocolID == ProtocolIDIdle && len(ep.Data) == 0 && minLoL == LoLNone {
		ep.Header.PacketLength = 1
		if err := ep.Validate(); err != nil {
			return nil, err
		}
		return ep, nil
	}

	// 4.1.3.1.5: only idle packets may omit the data field.
	if ep.Header.ProtocolID != ProtocolIDIdle && len(ep.Data) == 0 {
		return nil, ErrEmptyData
	}

	if minLoL == LoLNone {
		minLoL = LoL1Octet
	}
	lol, err := fitLengthOfLength(minLoL, len(ep.Data))
	if err != nil {
		return nil, err
	}
	ep.Header.LengthOfLength = lol
	ep.Header.PacketLength = uint32(ep.Header.Size() + len(ep.Data))

	if err := ep.Validate(); err != nil {
		return nil, err
	}

	return ep, nil
}

// fitLengthOfLength returns the smallest Length of Length value >= min whose
// header plus dataLen octets of data stays within the format's maximum
// total packet length.
func fitLengthOfLength(min uint8, dataLen int) (uint8, error) {
	for lol := min; lol <= LoL4Octet; lol++ {
		total := uint64(1)<<lol + uint64(dataLen)
		var max uint64
		switch lol {
		case LoL1Octet:
			max = MaxPacketLength2
		case LoL2Octet:
			max = MaxPacketLength4
		default:
			max = MaxPacketLength8
		}
		if total <= max {
			return lol, nil
		}
	}
	return 0, ErrPacketTooLarge
}

// NewIdlePacket creates the 1-octet idle Encapsulation Packet (0xE0).
func NewIdlePacket() (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDIdle, nil)
}

// NewIdleFillPacket creates an idle Encapsulation Packet of exactly
// totalLength octets, filling the data zone with the given fill byte.
// Use it to fill a fixed-length transfer frame data field. A totalLength of
// 1 yields the 1-octet idle packet.
func NewIdleFillPacket(totalLength int, fill byte) (*EncapsulationPacket, error) {
	if totalLength < 1 {
		return nil, ErrInvalidIdleLength
	}
	if totalLength == 1 {
		return NewIdlePacket()
	}

	var lol uint8
	var headerSize int
	switch {
	case totalLength <= MaxPacketLength2:
		lol, headerSize = LoL1Octet, HeaderSize2
	case totalLength <= MaxPacketLength4:
		lol, headerSize = LoL2Octet, HeaderSize4
	case uint64(totalLength) <= MaxPacketLength8:
		lol, headerSize = LoL4Octet, HeaderSize8
	default:
		return nil, ErrPacketTooLarge
	}

	var data []byte
	if n := totalLength - headerSize; n > 0 {
		data = make([]byte, n)
		for i := range data {
			data[i] = fill
		}
	}

	ep := &EncapsulationPacket{
		Header: Header{
			PVN:            PVN,
			ProtocolID:     ProtocolIDIdle,
			LengthOfLength: lol,
			PacketLength:   uint32(totalLength),
		},
		Data: data,
	}
	if err := ep.Validate(); err != nil {
		return nil, err
	}
	return ep, nil
}

// NewIPEPacket creates an Internet Protocol Extension Encapsulation Packet.
func NewIPEPacket(data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDIPE, data, options...)
}

// NewLTPPacket creates an LTP Encapsulation Packet.
func NewLTPPacket(data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDLTP, data, options...)
}

// NewMissionPacket creates a mission-specific ('111') Encapsulation Packet
// carrying privately defined data.
func NewMissionPacket(data []byte, options ...PacketOption) (*EncapsulationPacket, error) {
	return NewPacket(ProtocolIDMission, data, options...)
}

// Encode converts the EncapsulationPacket into a byte slice for transmission.
func (ep *EncapsulationPacket) Encode() ([]byte, error) {
	if err := ep.Validate(); err != nil {
		return nil, err
	}

	headerBytes, err := ep.Header.Encode()
	if err != nil {
		return nil, err
	}

	result := append([]byte{}, headerBytes...)
	result = append(result, ep.Data...)

	return result, nil
}

// Decode parses a byte slice into an EncapsulationPacket. Trailing bytes
// beyond the declared packet length are ignored.
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

	// 1-octet idle packet: no data zone.
	if header.LengthOfLength == LoLNone {
		return &EncapsulationPacket{Header: header}, nil
	}

	headerSize := header.Size()
	totalSize := int(header.PacketLength)

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

	// 1-octet idle packet carries no data zone.
	if ep.Header.LengthOfLength == LoLNone {
		if len(ep.Data) > 0 {
			return ErrIdleWithData
		}
		return nil
	}

	// 4.1.3.1.4/4.1.3.1.5: the data field may be absent only in idle packets.
	if len(ep.Data) == 0 && ep.Header.ProtocolID != ProtocolIDIdle {
		return ErrEmptyData
	}

	// Verify packet length matches actual size
	expectedLength := uint64(ep.Header.Size()) + uint64(len(ep.Data))
	if expectedLength != uint64(ep.Header.PacketLength) {
		return ErrPacketLengthMismatch
	}

	return nil
}

// IsIdle reports whether the packet is an idle packet (Protocol ID = 0).
func (ep *EncapsulationPacket) IsIdle() bool {
	return ep.Header.ProtocolID == ProtocolIDIdle
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
// starting at data[0], or -1 if the data is too short to determine length,
// or if the declared length cannot be represented as an int on this
// platform.
// This implements the sdl.PacketSizer signature for use with data link services.
func PacketSizer(data []byte) int {
	hdrSize := HeaderSize(data)
	if hdrSize < 0 {
		return -1
	}

	// 1-octet idle packets are exactly 1 byte.
	if hdrSize == HeaderSize1 {
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

	// Bound before narrowing: h.PacketLength is a uint32 and, for the LoL
	// '11' 8-octet header, the only validation is >= HeaderSize8 (see
	// Header.Validate), so a peer can declare up to MaxPacketLength8
	// (0xFFFFFFFF). On a 32-bit build (GOARCH=386, arm, wasm) int is 32
	// bits, so a value above math.MaxInt would wrap to a negative int once
	// narrowed. Report the length as indeterminate (the same sentinel used
	// for "too short") instead of returning a spurious negative total; on a
	// 64-bit build this branch is never taken, since MaxPacketLength8
	// always fits in a 64-bit int.
	if uint64(h.PacketLength) > math.MaxInt {
		return -1
	}

	return int(h.PacketLength)
}
