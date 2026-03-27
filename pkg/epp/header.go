package epp

import (
	"encoding/binary"
	"strconv"
	"strings"
)

/*
Encapsulation Packet Protocol (EPP) Header:

Octet 0 (always present):
+---+---+---+---+---+---+---+---+
| Packet Version    | Protocol  |L|
| Number (0111)     | ID (3b)   |o|
|                   |           |L|
+---+---+---+---+---+---+---+---+
  7   6   5   4   3   2   1   0

Header Formats:

Format 1 — Idle (1 byte):
  PID=000, LoL=0
  +--------+
  | Octet0 |
  +--------+

Format 2 — Short (2 bytes):
  PID=001-110, LoL=0
  +--------+------------------+
  | Octet0 | Packet Length 8b |
  +--------+------------------+

Format 3 — Medium (4 bytes):
  PID=001-110, LoL=1
  +--------+--------------+---------------------+
  | Octet0 | User Defined | Packet Length 16b   |
  +--------+--------------+---------------------+

Format 4 — Extended Medium (4 bytes):
  PID=111, LoL=0
  +--------+----------+---------------------+
  | Octet0 | Ext PID  | Packet Length 16b   |
  +--------+----------+---------------------+

Format 5 — Extended Long (8 bytes):
  PID=111, LoL=1
  +--------+----------+----------------+---------------------+
  | Octet0 | Ext PID  | CCSDS Defined  | Packet Length 32b   |
  +--------+----------+----------------+---------------------+

Legend:
  - PVN = Packet Version Number (4 bits, always 0111 = 7)
  - PID = Protocol ID (3 bits)
  - LoL = Length of Length (1 bit)
  - Packet Length = total octets in the entire encapsulation packet
*/

// Packet Version Number for Encapsulation Packets per CCSDS 133.1-B-3.
const PVN = 7

// Protocol ID values per CCSDS 133.1-B-3 Section 4.1.2.
const (
	ProtocolIDIdle     uint8 = 0 // Idle packet (fill data)
	ProtocolIDReserved uint8 = 1 // Reserved
	ProtocolIDIPE      uint8 = 2 // Internet Protocol Extension
	ProtocolIDUserDef  uint8 = 6 // User-Defined Protocol
	ProtocolIDExtended uint8 = 7 // Protocol ID Extension (see next byte)
)

// Header format sizes in bytes.
const (
	HeaderSizeIdle           = 1
	HeaderSizeShort          = 2
	HeaderSizeMedium         = 4
	HeaderSizeExtendedMedium = 4
	HeaderSizeExtendedLong   = 8
)

// Maximum data zone sizes per header format.
const (
	MaxPacketLengthShort        = 255        // 8-bit packet length
	MaxPacketLengthMedium       = 65535      // 16-bit packet length
	MaxPacketLengthExtendedLong = 4294967295 // 32-bit packet length
)

// Header represents the variable-length header of an Encapsulation Packet.
type Header struct {
	PVN                uint8  // Packet Version Number (4 bits, must be 7)
	ProtocolID         uint8  // Protocol ID (3 bits, 0-7)
	LengthOfLength     uint8  // Length of Length indicator (1 bit, 0 or 1)
	UserDefined        uint8  // User-defined field (8 bits, Format 3 only)
	ExtendedProtocolID uint8  // Extended Protocol ID (8 bits, Formats 4 and 5)
	CCSDSDefined       uint16 // CCSDS-defined field (16 bits, Format 5 only)
	PacketLength       uint32 // Total packet length in octets (size depends on format)
}

// Format returns the header format (1-5) based on ProtocolID and LengthOfLength.
func (h *Header) Format() int {
	if h.ProtocolID == ProtocolIDIdle && h.LengthOfLength == 0 {
		return 1
	}
	if h.ProtocolID != ProtocolIDExtended && h.LengthOfLength == 0 {
		return 2
	}
	if h.ProtocolID != ProtocolIDExtended && h.LengthOfLength == 1 {
		return 3
	}
	if h.ProtocolID == ProtocolIDExtended && h.LengthOfLength == 0 {
		return 4
	}
	return 5
}

// Size returns the header size in bytes for this header's format.
func (h *Header) Size() int {
	switch h.Format() {
	case 1:
		return HeaderSizeIdle
	case 2:
		return HeaderSizeShort
	case 3:
		return HeaderSizeMedium
	case 4:
		return HeaderSizeExtendedMedium
	case 5:
		return HeaderSizeExtendedLong
	default:
		return 0
	}
}

// Encode serializes the Header into bytes.
func (h *Header) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	octet0 := (h.PVN << 4) | (h.ProtocolID << 1) | h.LengthOfLength

	switch h.Format() {
	case 1:
		return []byte{octet0}, nil

	case 2:
		return []byte{octet0, byte(h.PacketLength)}, nil

	case 3:
		buf := make([]byte, 4)
		buf[0] = octet0
		buf[1] = h.UserDefined
		binary.BigEndian.PutUint16(buf[2:4], uint16(h.PacketLength))
		return buf, nil

	case 4:
		buf := make([]byte, 4)
		buf[0] = octet0
		buf[1] = h.ExtendedProtocolID
		binary.BigEndian.PutUint16(buf[2:4], uint16(h.PacketLength))
		return buf, nil

	case 5:
		buf := make([]byte, 8)
		buf[0] = octet0
		buf[1] = h.ExtendedProtocolID
		binary.BigEndian.PutUint16(buf[2:4], h.CCSDSDefined)
		binary.BigEndian.PutUint32(buf[4:8], h.PacketLength)
		return buf, nil

	default:
		return nil, ErrInvalidProtocolID
	}
}

// Decode deserializes bytes into a Header. At least 1 byte must be provided;
// additional bytes are read as needed based on the detected format.
func (h *Header) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}

	h.PVN = data[0] >> 4
	h.ProtocolID = (data[0] >> 1) & 0x07
	h.LengthOfLength = data[0] & 0x01

	if h.PVN != PVN {
		return ErrInvalidPVN
	}

	switch h.Format() {
	case 1:
		h.PacketLength = 1
		return nil

	case 2:
		if len(data) < 2 {
			return ErrDataTooShort
		}
		h.PacketLength = uint32(data[1])
		return h.Validate()

	case 3:
		if len(data) < 4 {
			return ErrDataTooShort
		}
		h.UserDefined = data[1]
		h.PacketLength = uint32(binary.BigEndian.Uint16(data[2:4]))
		return h.Validate()

	case 4:
		if len(data) < 4 {
			return ErrDataTooShort
		}
		h.ExtendedProtocolID = data[1]
		h.PacketLength = uint32(binary.BigEndian.Uint16(data[2:4]))
		return h.Validate()

	case 5:
		if len(data) < 8 {
			return ErrDataTooShort
		}
		h.ExtendedProtocolID = data[1]
		h.CCSDSDefined = binary.BigEndian.Uint16(data[2:4])
		h.PacketLength = binary.BigEndian.Uint32(data[4:8])
		return h.Validate()

	default:
		return ErrInvalidProtocolID
	}
}

// Validate checks that all fields conform to CCSDS 133.1-B-3.
func (h *Header) Validate() error {
	if h.PVN != PVN {
		return ErrInvalidPVN
	}
	if h.ProtocolID > 7 {
		return ErrInvalidProtocolID
	}
	if h.LengthOfLength > 1 {
		return ErrInvalidLengthOfLength
	}
	return nil
}

// Humanize generates a human-readable representation of the Header.
func (h *Header) Humanize() string {
	lines := []string{
		"  PVN: " + strconv.Itoa(int(h.PVN)),
		"  Protocol ID: " + strconv.Itoa(int(h.ProtocolID)) + " (" + protocolIDName(h.ProtocolID) + ")",
		"  Length of Length: " + strconv.Itoa(int(h.LengthOfLength)),
		"  Format: " + strconv.Itoa(h.Format()) + " (" + strconv.Itoa(h.Size()) + " bytes)",
	}

	switch h.Format() {
	case 3:
		lines = append(lines, "  User Defined: "+strconv.Itoa(int(h.UserDefined)))
	case 4:
		lines = append(lines, "  Extended Protocol ID: "+strconv.Itoa(int(h.ExtendedProtocolID)))
	case 5:
		lines = append(lines, "  Extended Protocol ID: "+strconv.Itoa(int(h.ExtendedProtocolID)))
		lines = append(lines, "  CCSDS Defined: "+strconv.Itoa(int(h.CCSDSDefined)))
	}

	if h.Format() != 1 {
		lines = append(lines, "  Packet Length: "+strconv.FormatUint(uint64(h.PacketLength), 10))
	}

	return strings.Join(lines, "\n")
}

// HeaderSize returns the header size in bytes by inspecting the first byte of
// an encoded packet. Returns -1 if the data is too short.
func HeaderSize(data []byte) int {
	if len(data) < 1 {
		return -1
	}
	pid := (data[0] >> 1) & 0x07
	lol := data[0] & 0x01

	if pid == ProtocolIDIdle && lol == 0 {
		return HeaderSizeIdle
	}
	if pid != ProtocolIDExtended && lol == 0 {
		return HeaderSizeShort
	}
	if pid != ProtocolIDExtended && lol == 1 {
		return HeaderSizeMedium
	}
	if pid == ProtocolIDExtended && lol == 0 {
		return HeaderSizeExtendedMedium
	}
	return HeaderSizeExtendedLong
}

// protocolIDName returns a human-readable name for the given Protocol ID.
func protocolIDName(pid uint8) string {
	switch pid {
	case ProtocolIDIdle:
		return "Idle"
	case ProtocolIDIPE:
		return "Internet Protocol Extension"
	case ProtocolIDUserDef:
		return "User-Defined"
	case ProtocolIDExtended:
		return "Protocol ID Extension"
	default:
		return "Reserved"
	}
}
