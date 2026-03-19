package tmdl

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/crc"
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

// TMTransferFrame represents a CCSDS TM Space Data Link Protocol Transfer Frame.
type TMTransferFrame struct {
	Header             PrimaryHeader
	SecondaryHeader    SecondaryHeader
	DataField          []byte // Main telemetry data
	OperationalControl []byte // 4-byte OCF (if used)
	FrameErrorControl  uint16 // 16-bit CRC (Error Control)
}

// NewTMTransferFrame initializes a new TM Transfer Frame.
func NewTMTransferFrame(scid uint16, vcid uint8, data []byte, secondaryHeaderData []byte, ocf []byte) (*TMTransferFrame, error) {
	if len(data) > 65535 {
		return nil, ErrDataTooLarge
	}

	secondaryHeader := SecondaryHeader{
		DataField: secondaryHeaderData,
	}
	if len(secondaryHeaderData) > 0 {
		// Per CCSDS 132.0-B-3 §4.1.3.2.2: HeaderLength = (Data Field octets) - 1
		secondaryHeader.HeaderLength = uint8(len(secondaryHeaderData) - 1)
	}

	frame := &TMTransferFrame{
		Header: PrimaryHeader{
			VersionNumber:    0b00,          // Default CCSDS TM version
			SpacecraftID:     scid & 0x03FF, // Mask to 10 bits
			VirtualChannelID: vcid & 0x07,   // Mask to 3 bits
			OCFFlag:          len(ocf) > 0,  // Set OCF flag if present
			FSHFlag:          len(secondaryHeaderData) > 0,
			MCFrameCount:     0, // To be set dynamically
			VCFrameCount:     0, // To be set dynamically
			SyncFlag:         false,
			PacketOrderFlag:  false,
			SegmentLengthID:  0b11, // Default segment length ID
			FirstHeaderPtr:   0,    // Default "no packet start" pointer
		},
		SecondaryHeader:    secondaryHeader,
		DataField:          data,
		OperationalControl: ocf,
	}
	// FirstHeaderPtr defaults to 0: first packet starts at byte 0 of Data Field.
	// Per CCSDS 132.0-B-3 §4.1.2.7.3, FirstHeaderPtr is relative to the
	// Transfer Frame Data Field (after the Secondary Header), not the frame payload.
	// VCA service sets SyncFlag=true and FirstHeaderPtr=0x07FF separately.

	// Compute Frame Error Control (CRC-16)
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}
	frame.FrameErrorControl = crc.ComputeCRC16(encoded)

	return frame, nil
}

// Encode converts the TM Transfer Frame to a byte slice.
func (tf *TMTransferFrame) Encode() ([]byte, error) {
	frameData, err := tf.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}

	// Append CRC-16
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
	return append(frameData, crcBytes...), nil
}

// EncodeWithoutFEC converts the frame to bytes excluding the CRC field.
func (tf *TMTransferFrame) EncodeWithoutFEC() ([]byte, error) {
	header, err := tf.Header.Encode()
	if err != nil {
		return nil, err
	}

	var secondaryHeader []byte

	// Only encode secondary header if FSHFlag is set
	if tf.Header.FSHFlag {
		secondaryHeader, err = tf.SecondaryHeader.Encode()
		if err != nil {
			return nil, err
		}
	}

	// Assemble full frame
	frameData := append(header, secondaryHeader...)
	frameData = append(frameData, tf.DataField...)
	if tf.Header.OCFFlag {
		if len(tf.OperationalControl) != 4 {
			return nil, ErrInvalidOCFLength
		}
		frameData = append(frameData, tf.OperationalControl...)
	}

	return frameData, nil
}

// padDataField copies data into a new slice of the given capacity,
// filling any remaining bytes with 0xFF (idle fill). If data is longer
// than capacity it is truncated. The returned slice never aliases the input.
func padDataField(data []byte, capacity int) []byte {
	padded := make([]byte, capacity)
	copy(padded, data)
	for i := len(data); i < capacity; i++ {
		padded[i] = 0xFF
	}
	return padded
}

// NewIdleFrame creates an idle TM Transfer Frame with all-idle data field
// and FirstHeaderPtr set to 0x07FF per CCSDS 132.0-B-3.
func NewIdleFrame(scid uint16, vcid uint8, config ChannelConfig) (*TMTransferFrame, error) {
	capacity := config.DataFieldCapacity(0)
	if capacity <= 0 {
		return nil, ErrDataFieldTooSmall
	}
	idleData := make([]byte, capacity)
	for i := range idleData {
		idleData[i] = 0xFF
	}
	var ocf []byte
	if config.HasOCF {
		ocf = make([]byte, 4)
	}
	frame, err := NewTMTransferFrame(scid, vcid, idleData, nil, ocf)
	if err != nil {
		return nil, err
	}
	frame.Header.FirstHeaderPtr = 0x07FF
	return frame, recomputeCRC(frame)
}

// recomputeCRC re-encodes the frame (without FEC) and updates FrameErrorControl.
func recomputeCRC(frame *TMTransferFrame) error {
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return err
	}
	frame.FrameErrorControl = crc.ComputeCRC16(encoded)
	return nil
}

// IsIdleFrame reports whether the frame is an idle frame
// (SyncFlag=false with FirstHeaderPtr=0x07FF).
func IsIdleFrame(frame *TMTransferFrame) bool {
	return !frame.Header.SyncFlag && frame.Header.FirstHeaderPtr == 0x07FF
}

// DecodeTMTransferFrame parses a byte slice into a TM Transfer Frame.
func DecodeTMTransferFrame(data []byte) (*TMTransferFrame, error) {
	if len(data) < 8 {
		return nil, ErrDataTooShort
	}

	// Decode Primary Header
	var header PrimaryHeader
	if err := header.Decode(data[:6]); err != nil {
		return nil, err
	}

	// Compute and verify CRC-16
	receivedCRC := binary.BigEndian.Uint16(data[len(data)-2:])
	computedCRC := crc.ComputeCRC16(data[:len(data)-2])
	if receivedCRC != computedCRC {
		return nil, ErrCRCMismatch
	}

	// Extract Data Field
	primaryHeaderLength := 6
	dataStart := primaryHeaderLength
	dataEnd := len(data) - 2
	operationalControl := []byte{}

	// Decode Secondary Header if present, using self-describing length
	var secondaryHeader SecondaryHeader
	if header.FSHFlag {
		if dataStart >= dataEnd {
			return nil, ErrDataTooShort
		}
		if err := secondaryHeader.Decode(data[dataStart:dataEnd]); err != nil {
			return nil, err
		}
		dataStart += 1 + len(secondaryHeader.DataField)
	}

	// Extract OCF if present
	if header.OCFFlag {
		if dataEnd-dataStart < 4 {
			return nil, ErrDataTooShort
		}
		operationalControl = make([]byte, 4)
		copy(operationalControl, data[dataEnd-4:dataEnd])
		dataEnd -= 4
	}

	// Extract main Data Field (copy to avoid aliasing caller's buffer)
	dataField := make([]byte, dataEnd-dataStart)
	copy(dataField, data[dataStart:dataEnd])

	// Construct and return the TMTransferFrame object
	return &TMTransferFrame{
		Header:             header,
		SecondaryHeader:    secondaryHeader,
		DataField:          dataField,
		OperationalControl: operationalControl,
		FrameErrorControl:  receivedCRC,
	}, nil
}
