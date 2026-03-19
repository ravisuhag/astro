package tcdl

import (
	"strconv"
	"strings"
)

// PrimaryHeader represents the CCSDS TC Transfer Frame Primary Header (5 bytes).
// Per CCSDS 232.0-B-4 Section 4.1.2.
type PrimaryHeader struct {
	VersionNumber      uint8  // 2 bits  - Transfer Frame Version Number (must be 00)
	BypassFlag         uint8  // 1 bit   - 0=Type-A (sequence-controlled), 1=Type-B (expedited)
	ControlCommandFlag uint8  // 1 bit   - 0=data transfer, 1=control command
	Reserved           uint8  // 2 bits  - spare, must be 00
	SpacecraftID       uint16 // 10 bits - Spacecraft Identifier (0-1023)
	VirtualChannelID   uint8  // 6 bits  - Virtual Channel Identifier (0-63)
	FrameLength        uint16 // 10 bits - total frame octets minus 1 (0-1023)
	FrameSequenceNum   uint8  // 8 bits  - per-VC sequence number N(S) for COP-1
}

// Byte layout:
//   Byte 0: [VN:2][Bypass:1][CtrlCmd:1][Rsvd:2][SCID_hi:2]
//   Byte 1: [SCID_lo:8]
//   Byte 2: [VCID:6][FrameLen_hi:2]
//   Byte 3: [FrameLen_lo:8]
//   Byte 4: [FrameSeqNum:8]

// MCID returns the Master Channel Identifier (TFVN + SCID).
func (h *PrimaryHeader) MCID() uint16 {
	return uint16(h.VersionNumber)<<10 | h.SpacecraftID
}

// GVCID returns the Global Virtual Channel Identifier (MCID + VCID).
func (h *PrimaryHeader) GVCID() uint32 {
	return uint32(h.MCID())<<6 | uint32(h.VirtualChannelID)
}

// Encode packs the PrimaryHeader fields into a 5-byte slice.
func (h *PrimaryHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	b := make([]byte, 5)
	b[0] = (h.VersionNumber << 6) | (h.BypassFlag << 5) | (h.ControlCommandFlag << 4) |
		(h.Reserved << 2) | uint8(h.SpacecraftID>>8)
	b[1] = uint8(h.SpacecraftID & 0xFF)
	b[2] = (h.VirtualChannelID << 2) | uint8(h.FrameLength>>8)
	b[3] = uint8(h.FrameLength & 0xFF)
	b[4] = h.FrameSequenceNum

	return b, nil
}

// Decode parses a 5-byte slice into the PrimaryHeader.
func (h *PrimaryHeader) Decode(data []byte) error {
	if len(data) < 5 {
		return ErrDataTooShort
	}

	h.VersionNumber = (data[0] >> 6) & 0x03
	h.BypassFlag = (data[0] >> 5) & 0x01
	h.ControlCommandFlag = (data[0] >> 4) & 0x01
	h.Reserved = (data[0] >> 2) & 0x03
	h.SpacecraftID = uint16(data[0]&0x03)<<8 | uint16(data[1])
	h.VirtualChannelID = (data[2] >> 2) & 0x3F
	h.FrameLength = uint16(data[2]&0x03)<<8 | uint16(data[3])
	h.FrameSequenceNum = data[4]

	return h.Validate()
}

// Validate checks if the header values are within valid ranges.
func (h *PrimaryHeader) Validate() error {
	if h.VersionNumber != 0 {
		return ErrInvalidVersion
	}
	if h.Reserved != 0 {
		return ErrInvalidReservedBits
	}
	if h.SpacecraftID > 0x03FF {
		return ErrInvalidSpacecraftID
	}
	if h.VirtualChannelID > 0x3F {
		return ErrInvalidVCID
	}
	if h.FrameLength > 1023 {
		return ErrInvalidFrameLength
	}
	return nil
}

// Humanize returns a human-readable representation of the PrimaryHeader.
func (h *PrimaryHeader) Humanize() string {
	bypassStr := "Type-A (sequence-controlled)"
	if h.BypassFlag == 1 {
		bypassStr = "Type-B (expedited)"
	}
	ctrlStr := "Data"
	if h.ControlCommandFlag == 1 {
		ctrlStr = "Control Command"
	}
	return strings.Join([]string{
		"  Version Number: " + strconv.Itoa(int(h.VersionNumber)),
		"  Bypass Flag: " + bypassStr,
		"  Control Command: " + ctrlStr,
		"  Spacecraft ID: " + strconv.Itoa(int(h.SpacecraftID)),
		"  Virtual Channel ID: " + strconv.Itoa(int(h.VirtualChannelID)),
		"  Frame Length: " + strconv.Itoa(int(h.FrameLength+1)) + " bytes",
		"  Frame Sequence Number: " + strconv.Itoa(int(h.FrameSequenceNum)),
	}, "\n")
}

// SegmentHeader represents the TC Segment Header (1 byte).
// Per CCSDS 232.0-B-4 Section 4.1.4.1.
type SegmentHeader struct {
	SequenceFlags uint8 // 2 bits - 11=unsegmented, 01=first, 00=continuation, 10=last
	MAPID         uint8 // 6 bits - Multiplexer Access Point Identifier (0-63)
}

// Segment sequence flag constants.
const (
	SegContinuation uint8 = 0b00
	SegFirst        uint8 = 0b01
	SegLast         uint8 = 0b10
	SegUnsegmented  uint8 = 0b11
)

// Encode packs the SegmentHeader into a 1-byte slice.
func (sh *SegmentHeader) Encode() ([]byte, error) {
	if err := sh.Validate(); err != nil {
		return nil, err
	}
	return []byte{(sh.SequenceFlags << 6) | (sh.MAPID & 0x3F)}, nil
}

// Decode parses a 1-byte slice into the SegmentHeader.
func (sh *SegmentHeader) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}
	sh.SequenceFlags = (data[0] >> 6) & 0x03
	sh.MAPID = data[0] & 0x3F
	return sh.Validate()
}

// Validate checks if the segment header values are within valid ranges.
func (sh *SegmentHeader) Validate() error {
	if sh.SequenceFlags > 3 {
		return ErrInvalidSequenceFlags
	}
	if sh.MAPID > 0x3F {
		return ErrInvalidMAPID
	}
	return nil
}

// Humanize returns a human-readable representation of the SegmentHeader.
func (sh *SegmentHeader) Humanize() string {
	flagStr := "Continuation"
	switch sh.SequenceFlags {
	case SegFirst:
		flagStr = "First"
	case SegLast:
		flagStr = "Last"
	case SegUnsegmented:
		flagStr = "Unsegmented"
	}
	return strings.Join([]string{
		"  Sequence Flags: " + flagStr,
		"  MAP ID: " + strconv.Itoa(int(sh.MAPID)),
	}, "\n")
}
