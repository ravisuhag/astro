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
| Secondary Header (Optional, mission-specific length/format)    |
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
- Sequence Flags (4.1.3.4.2.2): 00 (continuation segment), 01 (first segment),
  10 (last segment), 11 (unsegmented)
- Packet Length: (Packet Data Field size) - 1, where Data Field = Secondary Header + User Data + Error Control

The Error Control field is not defined by CCSDS 133.0-B-2; it is a
mission/PUS-style extension carried inside the packet data field. It is
wire-compatible with the standard because the standard leaves the data
field content to the mission.
*/

// SpacePacket represents a complete space packet as per CCSDS standards.
type SpacePacket struct {
	PrimaryHeader   PrimaryHeader   // The primary header of the space packet
	SecondaryHeader SecondaryHeader // Optional mission-specific secondary header
	UserData        []byte          // User data contained in the packet
	ErrorControl    *uint16         // Optional error control field (e.g., CRC)

	// seqCountAuthoritative records that this packet's Packet Sequence Count
	// is the caller's, not the service's to allocate, so Service.SendPacket
	// must send it unchanged.
	//
	// It is set by WithSequenceCount and by Decode. A decoded packet already
	// carries the count its originating application assigned (4.1.3.4.3.3);
	// the Packet Transfer Function of 4.2.3 forwards it, and 3.3.1 requires
	// Packet Service SDUs to travel "without further formatting", so a relay
	// must not renumber what it received.
	seqCountAuthoritative bool

	// shInUserData records that the Secondary Header Flag is '1' while the
	// secondary header octets sit at the front of UserData rather than in a
	// parsed SecondaryHeader. The packet is whole and encodes byte for byte;
	// it just has no parsed header to offer.
	//
	// Decode sets it for a packet received without a decoder configured, so a
	// relay can forward a packet it cannot itself interpret (4.2.3).
	// WithSecondaryHeaderIndicator sets it for a caller assembling such a
	// packet from octets, which is the Secondary Header Indicator parameter of
	// 3.4.2.3 and 4.2.2.3.
	shInUserData bool
}

// secondaryHeaderOctets returns how many octets the parsed secondary header
// adds to the packet data field.
//
// It is zero unless the Secondary Header Flag is '1' and a header is attached.
// CCSDS 133.0-B-2 4.1.3.3.3.2 makes the flag the only signal of the header's
// presence, so the flag decides whether the octets are written, and the same
// condition must therefore decide whether they are counted in the Packet Data
// Length (4.1.3.5.3). Header octets that Decode left inside UserData are
// already counted by len(UserData).
func (sp *SpacePacket) secondaryHeaderOctets() int {
	if sp.PrimaryHeader.SecondaryHeaderFlag == 1 && sp.SecondaryHeader != nil {
		return sp.SecondaryHeader.Size()
	}
	return 0
}

// checkSecondaryHeaderAgreement verifies that the Secondary Header Flag and
// the SecondaryHeader field say the same thing (4.1.3.3.3.2).
func (sp *SpacePacket) checkSecondaryHeaderAgreement() error {
	flagSet := sp.PrimaryHeader.SecondaryHeaderFlag == 1
	switch {
	// A parsed header and header octets already inside UserData are two
	// representations of the one Packet Secondary Header of 4.1.4.2.
	// Both would be counted in the Packet Data Length (4.1.3.5.3) and both
	// would be written, so the packet would declare and carry the header
	// twice. The length field would still match the octets sent, which is why
	// nothing downstream catches it: the receiver hands its application the
	// duplicated octets as data. The options refuse this combination, but
	// SecondaryHeader is exported and can be assigned past them, so the check
	// belongs here too.
	case sp.SecondaryHeader != nil && sp.shInUserData:
		return ErrSecondaryHeaderTwice
	case sp.SecondaryHeader != nil && !flagSet:
		return ErrSecondaryHeaderFlagClear
	case sp.SecondaryHeader == nil && flagSet && !sp.shInUserData:
		return ErrSecondaryHeaderMissing
	}
	return nil
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

	// The packet owns its user data. Decode copies out of the buffer it was
	// handed for the same reason: a caller that reuses or mutates the slice it
	// passed in must not be able to change what a built packet encodes.
	packet := &SpacePacket{
		PrimaryHeader: primaryHeader,
		UserData:      append([]byte(nil), data...),
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
	dataFieldSize := len(packet.UserData) + packet.secondaryHeaderOctets()
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

// NewIdlePacket creates an idle SpacePacket (APID 0x7FF) carrying the given
// fill data. Per CCSDS 133.0-B-2 4.1.3.3.3.4 the Secondary Header Flag of an
// idle packet is '0'; its data field content is mission-defined fill (at
// least 1 octet).
//
// The packet is a telemetry packet by default. Neither 4.1.3.3.2.3 nor
// 4.1.3.3.4.4 ties the idle APID to a packet type, so a telecommand idle
// packet is equally legal; pass WithPacketType(PacketTypeTC) for one.
func NewIdlePacket(fill []byte, options ...PacketOption) (*SpacePacket, error) {
	return NewSpacePacket(APIDIdle, PacketTypeTM, fill, options...)
}

// PacketOption defines a function type for configuring SpacePacket options.
type PacketOption func(*SpacePacket) error

// WithSecondaryHeader adds a secondary header to the SpacePacket and sets the
// Secondary Header Flag to '1' (4.1.3.3.3.2).
//
// The header's octets are written ahead of the user data on encode. Use
// WithSecondaryHeaderIndicator instead when the octets are already at the
// front of the data you are passing in.
func WithSecondaryHeader(header SecondaryHeader) PacketOption {
	return func(packet *SpacePacket) error {
		if header == nil {
			return ErrSecondaryHeaderMissing
		}
		if err := validateSecondaryHeader(header); err != nil {
			return err
		}
		if packet.shInUserData {
			return ErrSecondaryHeaderTwice
		}
		packet.PrimaryHeader.SecondaryHeaderFlag = 1
		packet.SecondaryHeader = header
		return nil
	}
}

// WithSecondaryHeaderIndicator sets the Secondary Header Flag for a packet
// whose data field already begins with the Packet Secondary Header octets.
//
// This is the Secondary Header Indicator parameter of 3.4.2.3: the service
// user signals that a secondary header leads the octets it is handing over,
// and the Packet Assembly Function translates that signal into the flag
// (3.4.2.3.3, 4.2.2.3, 4.2.2.4). No SecondaryHeader implementation is needed,
// because nothing here has to interpret the octets. They are counted in the
// Packet Data Length as part of the user data and written verbatim.
//
// Passing false is the default and clears the flag.
func WithSecondaryHeaderIndicator(present bool) PacketOption {
	return func(packet *SpacePacket) error {
		if !present {
			packet.PrimaryHeader.SecondaryHeaderFlag = 0
			packet.shInUserData = false
			return nil
		}
		if packet.SecondaryHeader != nil {
			return ErrSecondaryHeaderTwice
		}
		packet.PrimaryHeader.SecondaryHeaderFlag = 1
		packet.shInUserData = true
		return nil
	}
}

// WithPacketType overrides the Packet Type of the SpacePacket: PacketTypeTM
// (0) or PacketTypeTC (1) per CCSDS 133.0-B-2 4.1.3.3.2.
//
// NewSpacePacket, NewTMPacket, and NewTCPacket already fix the type, so this
// option is mainly for NewIdlePacket, which defaults to telemetry.
func WithPacketType(packetType uint8) PacketOption {
	return func(packet *SpacePacket) error {
		if packetType > 1 {
			return ErrInvalidType
		}
		packet.PrimaryHeader.Type = packetType
		return nil
	}
}

// WithSequenceCount pins the sequence count on the SpacePacket.
//
// Service.SendPacket honors a pinned count: it sends the packet with that
// count instead of stamping its own. It then resynchronizes the per-APID
// counter to one past the pinned value, so later unpinned packets on the
// same APID carry on from there. CCSDS 133.0-B-2 4.1.3.4.3.4 requires the
// count to stay continuous modulo 16384; leaving the counter where it was
// would make the APID emit a jump and then a jump back.
func WithSequenceCount(n uint16) PacketOption {
	return func(packet *SpacePacket) error {
		if n > 16383 {
			return ErrInvalidSequenceCount
		}
		packet.PrimaryHeader.SequenceCount = n
		packet.seqCountAuthoritative = true
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
// The Packet Data Length field is recomputed from the current secondary
// header, user data, and error control sizes, so mutating those fields after
// construction cannot produce an inconsistent length on the wire. The packet
// is validated before encoding.
//
// The Secondary Header Flag decides everything about the secondary header:
// whether its octets are written and whether its size counts towards the
// Packet Data Length (4.1.3.3.3.2 and 4.1.3.5.3). A SecondaryHeader attached
// while the flag is '0' is rejected rather than silently dropped, because
// counting it and not writing it would declare a data field longer than the
// packet carries and make the receiver eat into the next packet.
func (sp *SpacePacket) Encode() ([]byte, error) {
	if err := sp.checkSecondaryHeaderAgreement(); err != nil {
		return nil, err
	}

	// Recompute the length field from the actual data field composition.
	dataFieldSize := len(sp.UserData) + sp.secondaryHeaderOctets()
	if sp.ErrorControl != nil {
		dataFieldSize += 2
	}
	if dataFieldSize == 0 {
		return nil, ErrEmptyPacket
	}
	if PrimaryHeaderSize+dataFieldSize > 65542 {
		return nil, ErrPacketTooLarge
	}

	// Validate reads the length field, so it has to be in place first. A
	// failure puts the old value back: a rejected Encode must leave the packet
	// exactly as the caller had it.
	previousLength := sp.PrimaryHeader.PacketLength
	sp.PrimaryHeader.PacketLength = uint16(dataFieldSize) - 1
	if err := sp.Validate(); err != nil {
		sp.PrimaryHeader.PacketLength = previousLength
		return nil, err
	}

	headerBytes, err := sp.PrimaryHeader.Encode()
	if err != nil {
		return nil, err
	}

	packetData := append([]byte{}, headerBytes...)

	// Encode the secondary header when one is attached. When the flag is set
	// but no header is attached, Decode left its octets at the front of
	// UserData and they are written with the rest of it.
	if sp.secondaryHeaderOctets() > 0 {
		secondaryBytes, err := sp.SecondaryHeader.Encode()
		if err != nil {
			return nil, err
		}
		if len(secondaryBytes) != sp.SecondaryHeader.Size() {
			return nil, ErrSecondaryHeaderSizeMismatch
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

// Decode parses a byte slice into a SpacePacket. The returned packet does not
// retain the input slice; all fields are copied. Trailing bytes beyond the
// packet length declared in the primary header are ignored, so a buffer may
// carry more than one packet; use PacketSizer to find the packet boundary.
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
			// The buffer holds the whole packet; the decoder simply wants
			// more octets than this packet's data field has.
			return nil, ErrSecondaryHeaderExceedsDataField
		}
		// A copy, not a subslice: an implementation that keeps the octets it
		// was handed must not end up aliasing the caller's buffer, or this
		// function's promise not to retain the input would not hold.
		shBytes := make([]byte, shSize)
		copy(shBytes, data[offset:offset+shSize])
		if err := secondaryHeader.Decode(shBytes); err != nil {
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

	userData := make([]byte, remainingDataField)
	copy(userData, data[offset:offset+remainingDataField])

	packet := &SpacePacket{
		PrimaryHeader:   primaryHeader,
		SecondaryHeader: secondaryHeader,
		UserData:        userData,
		ErrorControl:    errorControl,
		// The flag says a secondary header is there but nothing parsed it, so
		// its octets are still at the front of UserData. Recording that keeps
		// the packet re-encodable byte for byte.
		shInUserData: primaryHeader.SecondaryHeaderFlag == 1 && secondaryHeader == nil,
		// The count belongs to the application that generated this packet
		// (4.1.3.4.3.3). A relay forwarding it through Service.SendPacket must
		// send it on unchanged rather than stamping its own.
		seqCountAuthoritative: true,
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

	// CCSDS 4.1.3.3.3.2: the Secondary Header Flag is the only signal of the
	// header's presence, so the flag and the field must agree.
	if err := sp.checkSecondaryHeaderAgreement(); err != nil {
		return err
	}

	// CCSDS 4.1.3.3.3.4: the Secondary Header Flag is '0' for idle packets.
	if sp.PrimaryHeader.APID == APIDIdle &&
		(sp.PrimaryHeader.SecondaryHeaderFlag == 1 || sp.SecondaryHeader != nil) {
		return ErrIdleWithSecondaryHeader
	}

	// CCSDS C1/C2: packet must contain at least a secondary header or user data.
	// Note: when the secondary header flag is set but no decoder was provided
	// during Decode(), the secondary header bytes are included in UserData,
	// so this check still holds.
	if len(sp.UserData) == 0 && sp.SecondaryHeader == nil {
		return ErrEmptyPacket
	}

	// Calculate total packet data field size per CCSDS 4.1.3.5.3:
	// C = (total number of octets in the packet data field) - 1.
	dataFieldSize := len(sp.UserData) + sp.secondaryHeaderOctets()
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
	return sp.PrimaryHeader.APID == APIDIdle
}

// PacketSizer returns the total length in bytes of the complete Space Packet
// at the front of data, or -1 if data does not hold a complete packet.
//
// It implements the sdl.PacketSizer signature used by the data link packet
// services to slice packets out of a reassembly buffer. Those callers hold
// whatever frame data has arrived so far, so a packet that reaches past the
// end of the buffer is not a packet yet: -1 tells them to pull another frame
// rather than to read octets that are not there.
//
// Use DeclaredPacketSize when you are reading from a stream and want the
// length before you have fetched the body.
func PacketSizer(data []byte) int {
	size := DeclaredPacketSize(data)
	if size < 1 || size > len(data) {
		return -1
	}
	return size
}

// DeclaredPacketSize returns the total packet length declared by the primary
// header at the front of data (6 (primary header) + Packet Data Length + 1)
// or -1 when data is shorter than the 6-octet primary header.
//
// The returned length may be longer than data: it is what the header claims,
// not what is present. That is exactly what a stream reader needs, since it
// must learn how many octets to fetch before it has them. Callers holding a
// fixed buffer should use PacketSizer, which refuses a packet that is not all
// there.
func DeclaredPacketSize(data []byte) int {
	if len(data) < PrimaryHeaderSize {
		return -1
	}
	dataLen := int(binary.BigEndian.Uint16(data[4:6]))
	return PrimaryHeaderSize + 1 + dataLen
}

// IsIdleBytes reports whether the encoded packet at the front of data carries
// the idle APID (0x7FF, CCSDS 133.0-B-2 4.1.3.3.4.4). Idle packets are fill:
// a receiver discards them instead of delivering them to an application.
//
// It reads only the two APID octets, so it works on a packet that is not yet
// complete in the buffer.
func IsIdleBytes(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	return uint16(data[0]&0x07)<<8|uint16(data[1]) == APIDIdle
}
