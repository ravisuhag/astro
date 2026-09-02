package epp

import (
	"encoding/binary"
	"strconv"
	"strings"
)

/*
Encapsulation Packet Protocol (EPP) Header per CCSDS 133.1-B-3 Section 4.1.2:

Octet 0 (always present):
+---+---+---+---+---+---+---+---+
| PVN (3b)  | Protocol  | LoL   |
| = '111'   | ID (3b)   | (2b)  |
+---+---+---+---+---+---+---+---+
 MSB                         LSB

The header size is a pure function of the 2-bit Length of Length field
(table 4-1 / figure 4-2):

LoL '00', 1-octet header (idle packets only):
  +--------+
  | Octet0 |
  +--------+

LoL '01', 2-octet header:
  +--------+------------------+
  | Octet0 | Packet Length 8b |
  +--------+------------------+

LoL '10', 4-octet header:
  +--------+-----------+-----------+---------------------+
  | Octet0 | UDF (4b)  | PIE (4b)  | Packet Length 16b   |
  +--------+-----------+-----------+---------------------+

LoL '11', 8-octet header:
  +--------+-----------+-----------+----------------+---------------------+
  | Octet0 | UDF (4b)  | PIE (4b)  | CCSDS Defined  | Packet Length 32b   |
  |        |           |           | Field (16b)    |                     |
  +--------+-----------+-----------+----------------+---------------------+

Legend:
  - PVN = Packet Version Number (3 bits, always '111' = 7)
  - Protocol ID = Encapsulation Protocol ID (3 bits)
  - LoL = Length of Length (2 bits): '00'->1, '01'->2, '10'->4, '11'->8 octet header
  - UDF = User Defined Field (4 bits, 4- and 8-octet headers)
  - PIE = Encapsulation Protocol ID Extension (4 bits, 4- and 8-octet headers)
  - CCSDS Defined Field = 2 octets, 8-octet header only, reserved ('all zeros')
  - Packet Length = total octets in the entire encapsulation packet,
    including the header (4.1.2.8.2)
*/

// PVN is the Packet Version Number for Encapsulation Packets ('111') per
// CCSDS 133.1-B-3 Section 4.1.2.2.
const PVN = 7

// Encapsulation Protocol ID values per CCSDS 133.1-B-3 Section 4.1.2.3 and
// the SANA Encapsulation Protocol ID registry.
const (
	ProtocolIDIdle     uint8 = 0 // '000' Encapsulation Idle Packet (fill data)
	ProtocolIDLTP      uint8 = 1 // '001' Licklider Transmission Protocol (CCSDS 734.1)
	ProtocolIDIPE      uint8 = 2 // '010' Internet Protocol Extension
	ProtocolIDCFDP     uint8 = 3 // '011' CCSDS File Delivery Protocol (CCSDS 727.0)
	ProtocolIDBP       uint8 = 4 // '100' Bundle Protocol (CCSDS 734.2)
	ProtocolIDExtended uint8 = 6 // '110' protocol identified by the Protocol ID Extension field
	ProtocolIDMission  uint8 = 7 // '111' mission-specific, privately defined data
)

// Length of Length values per CCSDS 133.1-B-3 table 4-1. The value selects
// both the size of the Packet Length field and the total header size.
const (
	LoLNone   uint8 = 0 // '00' (no Packet Length field; 1-octet header (idle only)
	LoL1Octet uint8 = 1 // '01') 1-octet Packet Length; 2-octet header
	LoL2Octet uint8 = 2 // '10' (2-octet Packet Length; 4-octet header
	LoL4Octet uint8 = 3 // '11') 4-octet Packet Length; 8-octet header
)

// Header sizes in octets, a pure function of the Length of Length field.
const (
	HeaderSize1 = 1 // LoL '00'
	HeaderSize2 = 2 // LoL '01'
	HeaderSize4 = 4 // LoL '10'
	HeaderSize8 = 8 // LoL '11'
)

// Maximum total packet lengths (header included) per header size.
const (
	MaxPacketLength2 = 255        // 1-octet Packet Length field (2-octet header)
	MaxPacketLength4 = 65535      // 2-octet Packet Length field (4-octet header)
	MaxPacketLength8 = 4294967295 // 4-octet Packet Length field (8-octet header)
)

// Header represents the variable-length header of an Encapsulation Packet.
type Header struct {
	PVN                uint8  // Packet Version Number (3 bits, must be 7)
	ProtocolID         uint8  // Encapsulation Protocol ID (3 bits, 0-7)
	LengthOfLength     uint8  // Length of Length (2 bits, 0-3)
	UserDefined        uint8  // User Defined Field (4 bits, 4- and 8-octet headers)
	ExtendedProtocolID uint8  // Protocol ID Extension (4 bits, 4- and 8-octet headers)
	CCSDSDefined       uint16 // CCSDS Defined Field (16 bits, 8-octet header only)
	PacketLength       uint32 // Total packet length in octets, header included
}

// Size returns the header size in octets, determined solely by the
// Length of Length field: '00'->1, '01'->2, '10'->4, '11'->8.
func (h *Header) Size() int {
	return 1 << (h.LengthOfLength & 0x03)
}

// Encode serializes the Header into bytes.
func (h *Header) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	octet0 := (h.PVN << 5) | (h.ProtocolID << 2) | h.LengthOfLength

	switch h.LengthOfLength {
	case LoLNone:
		return []byte{octet0}, nil

	case LoL1Octet:
		return []byte{octet0, byte(h.PacketLength)}, nil

	case LoL2Octet:
		buf := make([]byte, HeaderSize4)
		buf[0] = octet0
		buf[1] = (h.UserDefined << 4) | (h.ExtendedProtocolID & 0x0F)
		binary.BigEndian.PutUint16(buf[2:4], uint16(h.PacketLength))
		return buf, nil

	default: // LoL4Octet
		buf := make([]byte, HeaderSize8)
		buf[0] = octet0
		buf[1] = (h.UserDefined << 4) | (h.ExtendedProtocolID & 0x0F)
		binary.BigEndian.PutUint16(buf[2:4], h.CCSDSDefined)
		binary.BigEndian.PutUint32(buf[4:8], h.PacketLength)
		return buf, nil
	}
}

// Decode deserializes bytes into a Header. At least 1 byte must be provided;
// additional bytes are read as needed based on the Length of Length field.
func (h *Header) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}

	h.PVN = data[0] >> 5
	h.ProtocolID = (data[0] >> 2) & 0x07
	h.LengthOfLength = data[0] & 0x03
	h.UserDefined = 0
	h.ExtendedProtocolID = 0
	h.CCSDSDefined = 0

	if h.PVN != PVN {
		return ErrInvalidPVN
	}

	if len(data) < h.Size() {
		return ErrDataTooShort
	}

	switch h.LengthOfLength {
	case LoLNone:
		h.PacketLength = 1

	case LoL1Octet:
		h.PacketLength = uint32(data[1])

	case LoL2Octet:
		h.UserDefined = data[1] >> 4
		h.ExtendedProtocolID = data[1] & 0x0F
		h.PacketLength = uint32(binary.BigEndian.Uint16(data[2:4]))

	default: // LoL4Octet
		h.UserDefined = data[1] >> 4
		h.ExtendedProtocolID = data[1] & 0x0F
		h.CCSDSDefined = binary.BigEndian.Uint16(data[2:4])
		h.PacketLength = binary.BigEndian.Uint32(data[4:8])
	}

	return h.Validate()
}

// Validate checks that all fields conform to CCSDS 133.1-B-3.
func (h *Header) Validate() error {
	if h.PVN != PVN {
		return ErrInvalidPVN
	}
	if h.ProtocolID > 7 {
		return ErrInvalidProtocolID
	}
	if h.LengthOfLength > LoL4Octet {
		return ErrInvalidLengthOfLength
	}
	if h.UserDefined > 0x0F {
		return ErrInvalidUserDefined
	}
	if h.ExtendedProtocolID > 0x0F {
		return ErrInvalidExtendedProtocolID
	}

	// 4.1.2.4.4: a 1-octet header (LoL '00') is only valid for idle packets.
	if h.LengthOfLength == LoLNone && h.ProtocolID != ProtocolIDIdle {
		return ErrNonIdleOneOctetHeader
	}

	// 4.1.2.6.2: the Protocol ID Extension identifies the protocol when the
	// Protocol ID is '110'; the field only exists in 4- and 8-octet headers.
	if h.ProtocolID == ProtocolIDExtended && h.LengthOfLength < LoL2Octet {
		return ErrExtendedNeedsLongHeader
	}

	// 4.1.2.6.3: when the Protocol ID is not '110', the Protocol ID
	// Extension shall be 'all zeros'.
	if h.ProtocolID != ProtocolIDExtended && h.ExtendedProtocolID != 0 {
		return ErrExtensionMustBeZero
	}

	// Fields that do not exist in the selected header size must be zero.
	if h.LengthOfLength < LoL2Octet && (h.UserDefined != 0 || h.ExtendedProtocolID != 0) {
		return ErrFieldNeedsLongerHeader
	}
	if h.LengthOfLength < LoL4Octet && h.CCSDSDefined != 0 {
		return ErrFieldNeedsLongerHeader
	}

	// 4.1.2.8.2: the Packet Length is the total packet length in octets,
	// header included, so it can never be smaller than the header itself.
	switch h.LengthOfLength {
	case LoLNone:
		if h.PacketLength != 0 && h.PacketLength != 1 {
			return ErrPacketLengthMismatch
		}
	case LoL1Octet:
		if h.PacketLength < HeaderSize2 || h.PacketLength > MaxPacketLength2 {
			return ErrPacketLengthMismatch
		}
	case LoL2Octet:
		if h.PacketLength < HeaderSize4 || h.PacketLength > MaxPacketLength4 {
			return ErrPacketLengthMismatch
		}
	default: // LoL4Octet
		if h.PacketLength < HeaderSize8 {
			return ErrPacketLengthMismatch
		}
	}

	return nil
}

// Humanize generates a human-readable representation of the Header.
func (h *Header) Humanize() string {
	lines := []string{
		"  PVN: " + strconv.Itoa(int(h.PVN)),
		"  Protocol ID: " + strconv.Itoa(int(h.ProtocolID)) + " (" + protocolIDName(h.ProtocolID) + ")",
		"  Length of Length: " + strconv.Itoa(int(h.LengthOfLength)),
		"  Header Size: " + strconv.Itoa(h.Size()) + " bytes",
	}

	if h.LengthOfLength >= LoL2Octet {
		lines = append(lines, "  User Defined: "+strconv.Itoa(int(h.UserDefined)))
		lines = append(lines, "  Protocol ID Extension: "+strconv.Itoa(int(h.ExtendedProtocolID)))
	}
	if h.LengthOfLength == LoL4Octet {
		lines = append(lines, "  CCSDS Defined: "+strconv.Itoa(int(h.CCSDSDefined)))
	}

	if h.LengthOfLength != LoLNone {
		lines = append(lines, "  Packet Length: "+strconv.FormatUint(uint64(h.PacketLength), 10))
	}

	return strings.Join(lines, "\n")
}

// HeaderSize returns the header size in bytes by inspecting the first byte of
// an encoded packet. Returns -1 if the data is too short or the first byte
// does not carry the encapsulation PVN ('111').
func HeaderSize(data []byte) int {
	if len(data) < 1 {
		return -1
	}
	if data[0]>>5 != PVN {
		return -1
	}
	return 1 << (data[0] & 0x03)
}

// protocolIDName returns a human-readable name for the given Protocol ID.
func protocolIDName(pid uint8) string {
	switch pid {
	case ProtocolIDIdle:
		return "Idle"
	case ProtocolIDLTP:
		return "LTP"
	case ProtocolIDIPE:
		return "Internet Protocol Extension"
	case ProtocolIDCFDP:
		return "CFDP"
	case ProtocolIDBP:
		return "Bundle Protocol"
	case ProtocolIDExtended:
		return "Protocol ID Extension"
	case ProtocolIDMission:
		return "Mission-Specific"
	default:
		return "Reserved"
	}
}
