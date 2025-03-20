package tmdl

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

// Header represents the CCSDS TM Transfer Frame Primary Header.
type PrimaryHeader struct {
	VersionNumber    uint8  // 2 bits (0-1)   - Transfer Frame Version Number (01 for TM)
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

// GetMCID returns the Master Channel Identifier (MCID) for the TM Transfer Frame.
func (h *PrimaryHeader) GetMCID() uint16 {
	// MCID = TFVN + SCID
	return uint16(h.VersionNumber)<<8 | h.SpacecraftID
}

// GetGVCID returns Global Virtual Channel Identifier.
func (h *PrimaryHeader) GetGVCID() uint16 {
	// GVCID = MCID + VCID = TFVN + SCID + VCID
	return h.GetMCID() + uint16(h.VirtualChannelID)
}

// Encode packs the Header fields into a byte slice.
func (h *PrimaryHeader) Encode() []byte {
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

	// Pack First Header Pointer (11 bits) into the last parts of byte 5 and into a new byte
	header[4] |= uint8((h.FirstHeaderPtr >> 8) & 0x07) // Top 3 bits of FirstHeaderPtr
	header[5] = uint8(h.FirstHeaderPtr & 0xFF)         // Bottom 8 bits of FirstHeaderPtr

	return header
}

// Decode parses a byte slice into a Header struct.
func (h *PrimaryHeader) Decode(data []byte) (*PrimaryHeader, error) {
	if len(data) < 6 {
		return nil, errors.New("data too short to decode primary header")
	}

	// Extract fields
	header := &PrimaryHeader{
		VersionNumber:    (data[0] >> 6) & 0x03,
		SpacecraftID:     (uint16(data[0]&0x3F) << 4) | uint16(data[1]>>4),
		VirtualChannelID: (data[1] >> 1) & 0x07,
		OCFFlag:          (data[1] & 1) != 0,
		MCFrameCount:     data[2],
		VCFrameCount:     data[3],
		FSHFlag:          (data[4] & (1 << 7)) != 0,
		SyncFlag:         (data[4] & (1 << 6)) != 0,
		PacketOrderFlag:  (data[4] & (1 << 5)) != 0,
		SegmentLengthID:  (data[4] >> 3) & 0x03,
		FirstHeaderPtr:   (uint16(data[4]&0x07) << 8) | uint16(data[5]),
	}

	if err := header.Validate(); err != nil {
		return nil, err
	}

	return header, nil
}

// Validate checks if the header values are within valid ranges.
func (h *PrimaryHeader) Validate() error {
	if h.VersionNumber > 0b11 {
		return errors.New("invalid VersionNumber: must be in range 0-3 (2 bits)")
	}
	if h.VersionNumber != 0 {
		return errors.New("invalid VersionNumber: must be 0 for TM Transfer Frame")
	}
	if h.SpacecraftID > 0x03FF {
		return errors.New("invalid SpacecraftID: must be in range 0-1023 (10 bits)")
	}
	if h.VirtualChannelID > 0x07 {
		return errors.New("invalid VirtualChannelID: must be in range 0-7 (3 bits)")
	}
	if !h.SyncFlag && h.PacketOrderFlag {
		return errors.New("invalid PacketOrderFlag: must be 0 when SyncFlag is 0")
	}
	if !h.SyncFlag && h.SegmentLengthID != 0b11 {
		return errors.New("invalid SegmentLengthID: must be 3 (0b11) when SyncFlag is 0")
	}
	if h.SegmentLengthID > 0x03 {
		return errors.New("invalid SegmentLengthID: must be in range 0-3 (2 bits)")
	}
	if h.SyncFlag && h.FirstHeaderPtr != 0xFFFF {
		return errors.New("invalid FirstHeaderPtr: must be 0xFFFF when SyncFlag is 1")
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
	HeaderLength  uint8  // 6 bits (2-7) - Length of Secondary Header (excluding this field)
	DataField     []byte // Transfer Frame Secondary Header Data
}

// Encode serializes the SecondaryHeader into a byte slice.
func (sh *SecondaryHeader) Encode() ([]byte, error) {
	if err := sh.Validate(); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	// Encode VersionNumber and HeaderLength
	firstByte := (sh.VersionNumber << 6) | (sh.HeaderLength & 0x3F)
	if err := buf.WriteByte(firstByte); err != nil {
		return nil, err
	}

	// Encode DataField
	if _, err := buf.Write(sh.DataField); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode deserializes a byte slice into a SecondaryHeader.
func (sh *SecondaryHeader) Decode(data []byte) error {
	if len(data) < 1 {
		return errors.New("data too short to decode secondary header")
	}

	// Decode VersionNumber and HeaderLength
	firstByte := data[0]
	sh.VersionNumber = firstByte >> 6
	sh.HeaderLength = firstByte & 0x3F

	// Decode DataField
	sh.DataField = data[1:]

	return sh.Validate()
}

// Validate checks if the header values are within valid ranges.
func (sh *SecondaryHeader) Validate() error {
	if sh.VersionNumber != 0 {
		return errors.New("invalid VersionNumber: must be 0 for Version 1")
	}
	if sh.HeaderLength > 0x3F {
		return errors.New("invalid HeaderLength: must be in range 0-63 (6 bits)")
	}
	return nil
}

// Humanize generates a human-readable representation of the SecondaryHeader.
func (sh *SecondaryHeader) Humanize() string {
	return strings.Join([]string{
		"  Version Number: " + strconv.Itoa(int(sh.VersionNumber)),
		"  Header Length: " + strconv.Itoa(int(sh.HeaderLength)),
		"  Data Field: " + string(sh.DataField),
	}, "\n")
}
