package tmdl

import (
	"encoding/hex"
	"strconv"
	"strings"
)

// PrimaryHeader represents the CCSDS TM Transfer Frame Primary Header.
type PrimaryHeader struct {
	VersionNumber    uint8  // 2 bits (0-1)   - Transfer Frame Version Number (00 for TM)
	SpacecraftID     uint16 // 10 bits (2-11) - Spacecraft Identifier
	VirtualChannelID uint8  // 3 bits (12-14) - Virtual Channel Identifier
	OCFFlag          bool   // 1 bit (15)     - Operational Control Field Flag
	MCFrameCount     uint8  // 8 bits (16-23) - Master Channel Frame Count
	VCFrameCount     uint8  // 8 bits (24-31) - Virtual Channel Frame Count
	FSHFlag          bool   // 1 bit (32)     - Frame Secondary Header Flag
	SyncFlag         bool   // 1 bit (33)     - Synchronization Flag
	PacketOrderFlag  bool   // 1 bit (34)     - Packet Order Flag
	SegmentLengthID  uint8  // 2 bits (35-36) - Segment Length Identifier
	FirstHeaderPtr   uint16 // 11 bits (37-47) - First Header Pointer
}

// MCID returns the Master Channel Identifier (MCID) for the TM Transfer Frame.
func (h *PrimaryHeader) MCID() uint16 {
	// MCID = TFVN (2 bits) + SCID (10 bits)
	return uint16(h.VersionNumber)<<10 | h.SpacecraftID
}

// GVCID returns the Global Virtual Channel Identifier.
func (h *PrimaryHeader) GVCID() uint16 {
	// GVCID = MCID (12 bits) + VCID (3 bits)
	return h.MCID()<<3 | uint16(h.VirtualChannelID)
}

// Encode packs the PrimaryHeader fields into a byte slice.
func (h *PrimaryHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	header := make([]byte, 6)

	// Pack Version Number, Spacecraft ID, and Virtual Channel ID
	header[0] = (h.VersionNumber << 6) | uint8(h.SpacecraftID>>4)
	header[1] = (uint8(h.SpacecraftID&0x0F) << 4) | (h.VirtualChannelID << 1)
	if h.OCFFlag {
		header[1] |= 1
	}

	// Master Channel Frame Count
	header[2] = h.MCFrameCount

	// Virtual Channel Frame Count
	header[3] = h.VCFrameCount

	// Flags and Segment Length ID
	header[4] = 0
	if h.FSHFlag {
		header[4] |= 1 << 7
	}
	if h.SyncFlag {
		header[4] |= 1 << 6
	}
	if h.PacketOrderFlag {
		header[4] |= 1 << 5
	}
	header[4] |= (h.SegmentLengthID & 0x03) << 3

	// Pack First Header Pointer (11 bits)
	header[4] |= uint8((h.FirstHeaderPtr >> 8) & 0x07) // Top 3 bits
	header[5] = uint8(h.FirstHeaderPtr & 0xFF)          // Bottom 8 bits

	return header, nil
}

// Decode parses a byte slice into the PrimaryHeader.
func (h *PrimaryHeader) Decode(data []byte) error {
	if len(data) < 6 {
		return ErrDataTooShort
	}

	h.VersionNumber = (data[0] >> 6) & 0x03
	h.SpacecraftID = (uint16(data[0]&0x3F) << 4) | uint16(data[1]>>4)
	h.VirtualChannelID = (data[1] >> 1) & 0x07
	h.OCFFlag = (data[1] & 1) != 0
	h.MCFrameCount = data[2]
	h.VCFrameCount = data[3]
	h.FSHFlag = (data[4] & (1 << 7)) != 0
	h.SyncFlag = (data[4] & (1 << 6)) != 0
	h.PacketOrderFlag = (data[4] & (1 << 5)) != 0
	h.SegmentLengthID = (data[4] >> 3) & 0x03
	h.FirstHeaderPtr = (uint16(data[4]&0x07) << 8) | uint16(data[5])

	return h.Validate()
}

// Validate checks if the header values are within valid ranges.
func (h *PrimaryHeader) Validate() error {
	if h.VersionNumber != 0 {
		return ErrInvalidVersion
	}
	if h.SpacecraftID > 0x03FF {
		return ErrInvalidSpacecraftID
	}
	if h.VirtualChannelID > 0x07 {
		return ErrInvalidVCID
	}
	if !h.SyncFlag && h.PacketOrderFlag {
		return ErrInvalidPacketOrderFlag
	}
	if !h.SyncFlag && h.SegmentLengthID != 0b11 {
		return ErrInvalidSegmentLengthID
	}
	if h.FirstHeaderPtr > 0x07FF {
		return ErrInvalidFirstHeaderPtr
	}
	if h.SyncFlag && h.FirstHeaderPtr != 0x07FF {
		return ErrInvalidFirstHeaderPtr
	}
	return nil
}

// Humanize generates a human-readable representation of the PrimaryHeader.
func (h *PrimaryHeader) Humanize() string {
	return strings.Join([]string{
		"  Version Number: " + strconv.Itoa(int(h.VersionNumber)),
		"  Spacecraft ID: " + strconv.Itoa(int(h.SpacecraftID)),
		"  Virtual Channel ID: " + strconv.Itoa(int(h.VirtualChannelID)),
		"  OCF Flag: " + strconv.FormatBool(h.OCFFlag),
		"  Master Channel Frame Count: " + strconv.Itoa(int(h.MCFrameCount)),
		"  Virtual Channel Frame Count: " + strconv.Itoa(int(h.VCFrameCount)),
		"  Frame Secondary Header Flag: " + strconv.FormatBool(h.FSHFlag),
		"  Synchronization Flag: " + strconv.FormatBool(h.SyncFlag),
		"  Packet Order Flag: " + strconv.FormatBool(h.PacketOrderFlag),
		"  Segment Length ID: " + strconv.Itoa(int(h.SegmentLengthID)),
		"  First Header Pointer: " + strconv.Itoa(int(h.FirstHeaderPtr)),
	}, "\n")
}

// SecondaryHeader represents the Transfer Frame Secondary Header as per CCSDS 132.0-B-3.
type SecondaryHeader struct {
	VersionNumber uint8  // 2 bits (0-1) - Always `00` for Version 1
	HeaderLength  uint8  // 6 bits (2-7) - Length of Secondary Header Data Field
	DataField     []byte // Transfer Frame Secondary Header Data
}

// Encode serializes the SecondaryHeader into a byte slice.
func (sh *SecondaryHeader) Encode() ([]byte, error) {
	if err := sh.Validate(); err != nil {
		return nil, err
	}

	data := make([]byte, 1+len(sh.DataField))
	data[0] = (sh.VersionNumber << 6) | (sh.HeaderLength & 0x3F)
	copy(data[1:], sh.DataField)

	return data, nil
}

// Decode deserializes a byte slice into the SecondaryHeader.
func (sh *SecondaryHeader) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}

	sh.VersionNumber = data[0] >> 6
	sh.HeaderLength = data[0] & 0x3F

	// Per CCSDS 132.0-B-3 §4.1.3.2.2: HeaderLength = (Data Field octets) - 1
	dataFieldLen := int(sh.HeaderLength) + 1
	expectedLen := 1 + dataFieldLen
	if len(data) < expectedLen {
		return ErrDataTooShort
	}
	sh.DataField = make([]byte, dataFieldLen)
	copy(sh.DataField, data[1:expectedLen])

	return sh.Validate()
}

// Validate checks if the header values are within valid ranges.
func (sh *SecondaryHeader) Validate() error {
	if sh.VersionNumber != 0 {
		return ErrInvalidSecondaryHeaderVersion
	}
	if sh.HeaderLength > 0x3F {
		return ErrInvalidHeaderLength
	}
	// Per CCSDS: HeaderLength = len(DataField) - 1
	if len(sh.DataField) > 0 && sh.HeaderLength != uint8(len(sh.DataField)-1) {
		return ErrInvalidHeaderLength
	}
	return nil
}

// Humanize generates a human-readable representation of the SecondaryHeader.
func (sh *SecondaryHeader) Humanize() string {
	return strings.Join([]string{
		"  Version Number: " + strconv.Itoa(int(sh.VersionNumber)),
		"  Header Length: " + strconv.Itoa(int(sh.HeaderLength)),
		"  Data Field: " + hex.EncodeToString(sh.DataField),
	}, "\n")
}
