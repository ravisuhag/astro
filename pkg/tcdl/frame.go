package tcdl

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/crc"
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
//
//	Byte 0: [VN:2][Bypass:1][CtrlCmd:1][Rsvd:2][SCID_hi:2]
//	Byte 1: [SCID_lo:8]
//	Byte 2: [VCID:6][FrameLen_hi:2]
//	Byte 3: [FrameLen_lo:8]
//	Byte 4: [FrameSeqNum:8]

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
	// Per CCSDS 232.0-B-4 4.1.2.3, Bypass=0 with Control Command=1 is an
	// invalid frame type. Valid types: AD (0,0), BD (1,0), BC (1,1).
	if h.BypassFlag == 0 && h.ControlCommandFlag == 1 {
		return ErrInvalidFrameType
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

const (
	// PrimaryHeaderSize is the size of the TC primary header in bytes.
	PrimaryHeaderSize = 5
	// MaxFrameLength is the maximum total TC frame length in bytes.
	MaxFrameLength = 1024
	// FECSize is the size of the Frame Error Control field in bytes.
	FECSize = 2
)

// TCTransferFrame represents a CCSDS TC Space Data Link Protocol Transfer Frame.
type TCTransferFrame struct {
	Header            PrimaryHeader
	SegmentHeader     *SegmentHeader // optional, present when MAP sublayer is used
	DataField         []byte         // Frame Data Field
	FrameErrorControl uint16         // 16-bit CRC-16-CCITT
}

// checkSegmentHeaderAllowed verifies that a control command frame carries no
// Segment Header (4.1.3.2.2.1.3).
func (tf *TCTransferFrame) checkSegmentHeaderAllowed() error {
	// The data field of a Type-BC frame is the whole COP-1 control command,
	// and a FARM reads it as such. A segment header octet in front of it
	// shifts the command, so Unlock and Set V(R) are no longer recognised
	// and the directive is dropped without an error at either end. The
	// options refuse this combination, but SegmentHeader is exported and can
	// be assigned past them, so the check belongs here too.
	if tf.Header.ControlCommandFlag == 1 && tf.SegmentHeader != nil {
		return ErrSegmentHeaderOnControlCommand
	}
	return nil
}

// FrameOption configures optional fields on a TCTransferFrame.
type FrameOption func(*TCTransferFrame)

// WithBypass sets the Bypass Flag to 1 (Type-B expedited frame).
func WithBypass() FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.BypassFlag = 1
	}
}

// WithControlCommand sets the Control Command Flag to 1, producing a
// Type-BC frame. Per CCSDS 232.0-B-4 4.1.2.3 a control command frame is
// always a bypass frame (Bypass=0 with Control Command=1 is invalid), so
// the Bypass Flag is set to 1 as well.
func WithControlCommand() FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.BypassFlag = 1
		f.Header.ControlCommandFlag = 1
	}
}

// WithSegmentHeader attaches a segment header to the frame. Combining it
// with WithControlCommand is refused: CCSDS 232.0-B-4 4.1.3.2.2.1.3 bars a
// segment header from a frame carrying a control command.
func WithSegmentHeader(sh SegmentHeader) FrameOption {
	return func(f *TCTransferFrame) {
		f.SegmentHeader = &sh
	}
}

// WithSequenceNumber sets the Frame Sequence Number (N(S) for COP-1).
func WithSequenceNumber(n uint8) FrameOption {
	return func(f *TCTransferFrame) {
		f.Header.FrameSequenceNum = n
	}
}

// NewTCTransferFrame creates a new TC Transfer Frame.
//
// Deprecated: use NewTransferFrame, which is the name every data-link
// package now uses. This forwarder will be removed in v1.0.
func NewTCTransferFrame(scid uint16, vcid uint8, data []byte, opts ...FrameOption) (*TCTransferFrame, error) {
	return NewTransferFrame(scid, vcid, data, opts...)
}

// NewTransferFrame creates a new TC Transfer Frame.
// The frame length is automatically computed. CRC is auto-calculated.
// A VCID wider than the 6-bit field is stored as given and refused on
// encode rather than masked: masking would route the frame to a different
// virtual channel than the caller named.
func NewTransferFrame(scid uint16, vcid uint8, data []byte, opts ...FrameOption) (*TCTransferFrame, error) {
	frame := &TCTransferFrame{
		Header: PrimaryHeader{
			VersionNumber:    0,
			SpacecraftID:     scid,
			VirtualChannelID: vcid,
		},
		DataField: data,
	}

	for _, opt := range opts {
		opt(frame)
	}

	if err := frame.checkSegmentHeaderAllowed(); err != nil {
		return nil, err
	}

	// Per CCSDS 232.0-B-4 4.1.2.7, the Frame Sequence Number is all zeros
	// on Type-B (bypass) frames; only Type-A frames carry N(S).
	if frame.Header.BypassFlag == 1 {
		frame.Header.FrameSequenceNum = 0
	}

	// Compute total frame length
	totalLen := PrimaryHeaderSize + len(data) + FECSize
	if frame.SegmentHeader != nil {
		totalLen++
	}
	if totalLen > MaxFrameLength {
		return nil, ErrDataTooLarge
	}
	frame.Header.FrameLength = uint16(totalLen - 1)

	// Compute CRC
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}
	frame.FrameErrorControl = crc.ComputeCRC16(encoded)

	return frame, nil
}

// Encode converts the TC Transfer Frame to a byte slice including CRC.
//
// The Frame Length field and the Frame Error Control Field are both
// refreshed from the frame's current contents on every call, so header or
// data changes made after construction (DataField and SegmentHeader are
// exported) cannot produce a declared length that disagrees with what is
// actually on the wire: decodeTCFrame trusts the header's Frame Length to
// find the CRC, so a stale value there gets the frame rejected at the
// other end. Use EncodeWithoutFEC to build a frame with a deliberately
// invalid CRC.
func (tf *TCTransferFrame) Encode() ([]byte, error) {
	totalLen := PrimaryHeaderSize + len(tf.DataField) + FECSize
	if tf.SegmentHeader != nil {
		totalLen++
	}
	if totalLen > MaxFrameLength {
		return nil, ErrDataTooLarge
	}
	tf.Header.FrameLength = uint16(totalLen - 1)

	frameData, err := tf.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}

	// Compute the CRC from the frame's current contents, so header fields
	// changed after construction are covered, and refresh the exported field.
	tf.FrameErrorControl = crc.ComputeCRC16(frameData)

	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
	return append(frameData, crcBytes...), nil
}

// EncodeWithoutFEC converts the frame to bytes excluding the CRC field.
func (tf *TCTransferFrame) EncodeWithoutFEC() ([]byte, error) {
	if err := tf.checkSegmentHeaderAllowed(); err != nil {
		return nil, err
	}

	header, err := tf.Header.Encode()
	if err != nil {
		return nil, err
	}

	frameData := make([]byte, 0, PrimaryHeaderSize+1+len(tf.DataField))
	frameData = append(frameData, header...)

	if tf.SegmentHeader != nil {
		sh, err := tf.SegmentHeader.Encode()
		if err != nil {
			return nil, err
		}
		frameData = append(frameData, sh...)
	}

	frameData = append(frameData, tf.DataField...)
	return frameData, nil
}

// DecodeTCTransferFrame parses a byte slice into a TC Transfer Frame.
//
// Deprecated: use DecodeTransferFrame, which is the name every data-link
// package now uses. This forwarder will be removed in v1.0.
func DecodeTCTransferFrame(data []byte) (*TCTransferFrame, error) {
	return DecodeTransferFrame(data)
}

// DecodeTransferFrame parses a byte slice into a TC Transfer Frame.
// Verifies CRC integrity. The segment header, if present, remains in
// DataField because the primary header has no flag to indicate its
// presence. Use DecodeTCTransferFrameWithSegmentHeader when the MAP
// sublayer is known to be in use.
func DecodeTransferFrame(data []byte) (*TCTransferFrame, error) {
	return decodeTCFrame(data, false)
}

// DecodeTCTransferFrameWithSegmentHeader parses a byte slice into a TC
// Transfer Frame, treating the first byte of the data field as a MAP
// Segment Header. Use this when the MAP sublayer is known to be in use.
// Type-BC frames are excepted: they carry no segment header
// (CCSDS 232.0-B-4 4.1.3.2.2.1.3) and their data field is returned whole.
func DecodeTCTransferFrameWithSegmentHeader(data []byte) (*TCTransferFrame, error) {
	return decodeTCFrame(data, true)
}

func decodeTCFrame(data []byte, hasSegmentHeader bool) (*TCTransferFrame, error) {
	if len(data) < PrimaryHeaderSize+FECSize {
		return nil, ErrDataTooShort
	}

	// Decode primary header
	var header PrimaryHeader
	if err := header.Decode(data[:PrimaryHeaderSize]); err != nil {
		return nil, err
	}

	// Verify frame length matches data. Octets beyond the declared frame
	// length are deliberately ignored: the synchronization sublayer may
	// deliver CLTU fill octets after the frame (CCSDS 232.0-B-4 4.1.2.7.2),
	// and the frame ends where its Frame Length field says it does.
	expectedLen := int(header.FrameLength) + 1
	if expectedLen < PrimaryHeaderSize+FECSize {
		return nil, ErrInvalidFrameLength
	}
	if len(data) < expectedLen {
		return nil, ErrDataTooShort
	}

	// Verify CRC
	receivedCRC := binary.BigEndian.Uint16(data[expectedLen-FECSize : expectedLen])
	computedCRC := crc.ComputeCRC16(data[:expectedLen-FECSize])
	if receivedCRC != computedCRC {
		return nil, ErrCRCMismatch
	}

	// Extract data field (between header and CRC)
	dataStart := PrimaryHeaderSize
	dataEnd := expectedLen - FECSize

	frame := &TCTransferFrame{
		Header:            header,
		FrameErrorControl: receivedCRC,
	}

	// Parse segment header if expected. A Type-BC frame never carries one
	// (4.1.3.2.2.1.3), even on a virtual channel that otherwise uses the MAP
	// sublayer, so its data field is handed back whole. Taking a segment
	// header off it would shift the COP-1 control command and leave the
	// caller with an Unlock or Set V(R) that no longer parses.
	if hasSegmentHeader && header.ControlCommandFlag == 0 {
		if dataStart >= dataEnd {
			return nil, ErrDataTooShort
		}
		var sh SegmentHeader
		if err := sh.Decode(data[dataStart : dataStart+1]); err != nil {
			return nil, err
		}
		frame.SegmentHeader = &sh
		dataStart++
	}

	dataField := make([]byte, dataEnd-dataStart)
	copy(dataField, data[dataStart:dataEnd])
	frame.DataField = dataField

	return frame, nil
}

// IsControlFrame reports whether the frame is a control command frame.
func IsControlFrame(frame *TCTransferFrame) bool {
	return frame.Header.ControlCommandFlag == 1
}

// IsBypass reports whether the frame is a Type-B (bypass/expedited) frame.
func IsBypass(frame *TCTransferFrame) bool {
	return frame.Header.BypassFlag == 1
}
