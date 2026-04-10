package usdl

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/crc"
)

// USLP Transfer Frame Version Number (CCSDS 732.1-B-2).
const TFVN = 12 // 0b1100

// TFDZ Construction Rules per CCSDS 732.1-B-2 §4.1.4.2.2.
const (
	RulePacketSpanning uint8 = 0 // Packets may span frames (MAPP)
	RuleVCASDU         uint8 = 1 // Fixed-length VCA SDUs (MAPA)
	RuleOctetStream    uint8 = 2 // Octet stream service (MAPO)
	RuleIdle           uint8 = 7 // Idle data
)

// Special FirstHeaderOffset values per CCSDS 732.1-B-2 §4.1.4.2.3.
const (
	FHONoPacketStart uint16 = 0xFFFF // No packet starts in this frame
	FHOAllIdle       uint16 = 0xFFFE // Entire TFDZ is idle fill
)

// PrimaryHeader represents the USLP Transfer Frame Primary Header.
//
// Bit layout (CCSDS 732.1-B-2 §4.1.2):
//
//	Byte 0:    TFVN[3:0] | SCID[15:12]
//	Byte 1:    SCID[11:4]
//	Byte 2:    SCID[3:0] | SourceOrDest | VCID[5:3]
//	Byte 3:    VCID[2:0] | MAPID[5:1]
//	Byte 4:    MAPID[0] | EndOfFPH | reserved(6 bits, must be 000000)
//
// For variable-length frames (when FrameLength is used):
//
//	Byte 5:    FrameLength[15:8]
//	Byte 6:    FrameLength[7:0]
type PrimaryHeader struct {
	TFVN         uint8  // 4 bits  - Transfer Frame Version Number (must be 12 = 0b1100)
	SCID         uint16 // 16 bits - Spacecraft Identifier (0-65535)
	SourceOrDest uint8  // 1 bit   - 0=source, 1=destination
	VCID         uint8  // 6 bits  - Virtual Channel Identifier (0-63)
	MAPID        uint8  // 6 bits  - Multiplexer Access Point Identifier (0-63)
	EndOfFPH     bool   // 1 bit   - End of Frame Primary Header flag
	FrameLength  uint16 // 16 bits - Total frame octets minus 1 (variable-length only)
}

// PrimaryHeaderFixedSize is the size of the fixed part of the primary header.
const PrimaryHeaderFixedSize = 5

// PrimaryHeaderVariableSize is the total size with the frame length field.
const PrimaryHeaderVariableSize = 7

// Size returns the encoded size of the primary header in bytes.
func (h *PrimaryHeader) Size() int {
	if h.EndOfFPH {
		return PrimaryHeaderFixedSize
	}
	return PrimaryHeaderVariableSize
}

// MCID returns the Master Channel Identifier (TFVN + SCID).
func (h *PrimaryHeader) MCID() uint32 {
	return uint32(h.TFVN)<<16 | uint32(h.SCID)
}

// GVCID returns the Global Virtual Channel Identifier (MCID + VCID).
func (h *PrimaryHeader) GVCID() uint32 {
	return h.MCID()<<6 | uint32(h.VCID)
}

// Encode packs the PrimaryHeader fields into a byte slice.
func (h *PrimaryHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	size := h.Size()
	b := make([]byte, size)

	// Byte 0: TFVN[3:0] | SCID[15:12]
	b[0] = (h.TFVN << 4) | uint8(h.SCID>>12)

	// Byte 1: SCID[11:4]
	b[1] = uint8(h.SCID >> 4)

	// Byte 2: SCID[3:0] | SourceOrDest | VCID[5:3]
	b[2] = uint8(h.SCID&0x0F)<<4 | (h.SourceOrDest & 0x01) << 3 | (h.VCID >> 3)

	// Byte 3: VCID[2:0] | MAPID[5:1]
	b[3] = (h.VCID&0x07)<<5 | (h.MAPID >> 1)

	// Byte 4: MAPID[0] | EndOfFPH | 000000
	b[4] = (h.MAPID & 0x01) << 7
	if h.EndOfFPH {
		b[4] |= 0x40
	}

	// Bytes 5-6: Frame Length (variable-length frames only)
	if size == PrimaryHeaderVariableSize {
		binary.BigEndian.PutUint16(b[5:7], h.FrameLength)
	}

	return b, nil
}

// Decode parses a byte slice into the PrimaryHeader.
func (h *PrimaryHeader) Decode(data []byte) error {
	if len(data) < PrimaryHeaderFixedSize {
		return ErrDataTooShort
	}

	h.TFVN = (data[0] >> 4) & 0x0F
	h.SCID = uint16(data[0]&0x0F)<<12 | uint16(data[1])<<4 | uint16(data[2]>>4)
	h.SourceOrDest = (data[2] >> 3) & 0x01
	h.VCID = (data[2]&0x07)<<3 | (data[3] >> 5)
	h.MAPID = (data[3]&0x1F)<<1 | (data[4] >> 7)
	h.EndOfFPH = (data[4] & 0x40) != 0

	if !h.EndOfFPH {
		if len(data) < PrimaryHeaderVariableSize {
			return ErrDataTooShort
		}
		h.FrameLength = binary.BigEndian.Uint16(data[5:7])
	}

	return h.Validate()
}

// Validate checks if the header values are within valid ranges.
func (h *PrimaryHeader) Validate() error {
	if h.TFVN != TFVN {
		return ErrInvalidVersion
	}
	if h.SourceOrDest > 1 {
		return fmt.Errorf("invalid source/dest flag: must be 0 or 1")
	}
	if h.VCID > 0x3F {
		return ErrInvalidVCID
	}
	if h.MAPID > 0x3F {
		return ErrInvalidMAPID
	}
	return nil
}

// Humanize returns a human-readable representation of the PrimaryHeader.
func (h *PrimaryHeader) Humanize() string {
	srcDst := "Source"
	if h.SourceOrDest == 1 {
		srcDst = "Destination"
	}
	lines := []string{
		"  TFVN: " + strconv.Itoa(int(h.TFVN)),
		"  Spacecraft ID: " + strconv.Itoa(int(h.SCID)),
		"  Source/Dest: " + srcDst,
		"  Virtual Channel ID: " + strconv.Itoa(int(h.VCID)),
		"  MAP ID: " + strconv.Itoa(int(h.MAPID)),
		"  End of Frame PH: " + strconv.FormatBool(h.EndOfFPH),
	}
	if !h.EndOfFPH {
		lines = append(lines, "  Frame Length: "+strconv.Itoa(int(h.FrameLength+1))+" bytes")
	}
	return strings.Join(lines, "\n")
}

// DataFieldHeader represents the USLP Transfer Frame Data Field Header (TFDFH).
//
// Per CCSDS 732.1-B-2 §4.1.4.2, the TFDFH contains:
//   - Construction Rule (3 bits)
//   - USLP Protocol ID (UPID) — variable length, protocol dependent
//   - First Header Offset / Last Valid Octet Offset (16 bits)
//   - Frame Sequence Number (16 bits)
type DataFieldHeader struct {
	ConstructionRule  uint8  // 3 bits  - TFDZ construction rule
	UPID              uint8  // 5 bits  - USLP Protocol Identifier
	FirstHeaderOffset uint16 // 16 bits - offset to first header in TFDZ
	SequenceNumber    uint16 // 16 bits - frame sequence number
}

// DataFieldHeaderSize is the encoded size of the TFDFH in bytes.
const DataFieldHeaderSize = 5

// Encode packs the DataFieldHeader into a byte slice.
func (h *DataFieldHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	b := make([]byte, DataFieldHeaderSize)

	// Byte 0: ConstructionRule[2:0] | UPID[4:0]
	b[0] = (h.ConstructionRule << 5) | (h.UPID & 0x1F)

	// Bytes 1-2: FirstHeaderOffset
	binary.BigEndian.PutUint16(b[1:3], h.FirstHeaderOffset)

	// Bytes 3-4: SequenceNumber
	binary.BigEndian.PutUint16(b[3:5], h.SequenceNumber)

	return b, nil
}

// Decode parses a byte slice into the DataFieldHeader.
func (h *DataFieldHeader) Decode(data []byte) error {
	if len(data) < DataFieldHeaderSize {
		return ErrDataTooShort
	}

	h.ConstructionRule = (data[0] >> 5) & 0x07
	h.UPID = data[0] & 0x1F
	h.FirstHeaderOffset = binary.BigEndian.Uint16(data[1:3])
	h.SequenceNumber = binary.BigEndian.Uint16(data[3:5])

	return h.Validate()
}

// Validate checks if the data field header values are within valid ranges.
func (h *DataFieldHeader) Validate() error {
	if h.ConstructionRule > 7 {
		return ErrInvalidConstructionRule
	}
	if h.UPID > 0x1F {
		return fmt.Errorf("invalid UPID: must be in range 0-31 (5 bits)")
	}
	return nil
}

// Humanize returns a human-readable representation of the DataFieldHeader.
func (h *DataFieldHeader) Humanize() string {
	ruleStr := "Unknown"
	switch h.ConstructionRule {
	case RulePacketSpanning:
		ruleStr = "Packet Spanning (MAPP)"
	case RuleVCASDU:
		ruleStr = "VCA SDU (MAPA)"
	case RuleOctetStream:
		ruleStr = "Octet Stream (MAPO)"
	case RuleIdle:
		ruleStr = "Idle"
	}
	return strings.Join([]string{
		"  Construction Rule: " + ruleStr,
		"  UPID: " + strconv.Itoa(int(h.UPID)),
		"  First Header Offset: " + fmt.Sprintf("0x%04X", h.FirstHeaderOffset),
		"  Sequence Number: " + strconv.Itoa(int(h.SequenceNumber)),
	}, "\n")
}

// FECSize16 is the size of a 16-bit Frame Error Control field.
const FECSize16 = 2

// FECSize32 is the size of a 32-bit Frame Error Control field.
const FECSize32 = 4

// TransferFrame represents a USLP Transfer Frame per CCSDS 732.1-B-2.
type TransferFrame struct {
	Header          PrimaryHeader
	InsertZone      []byte          // Optional insert zone
	DataFieldHeader DataFieldHeader // TFDFH
	DataField       []byte          // Transfer Frame Data Zone (TFDZ)
	OCF             []byte          // 4-byte Operational Control Field (optional)
	FECF            []byte          // Frame Error Control Field (2 or 4 bytes)
	UseCRC32        bool            // true=CRC-32, false=CRC-16
}

// FrameOption configures optional fields on a TransferFrame.
type FrameOption func(*TransferFrame)

// WithInsertZone sets the insert zone data.
func WithInsertZone(data []byte) FrameOption {
	return func(f *TransferFrame) {
		f.InsertZone = data
	}
}

// WithOCF sets the Operational Control Field.
func WithOCF(ocf []byte) FrameOption {
	return func(f *TransferFrame) {
		f.OCF = ocf
	}
}

// WithCRC32 enables CRC-32 instead of the default CRC-16.
func WithCRC32() FrameOption {
	return func(f *TransferFrame) {
		f.UseCRC32 = true
	}
}

// WithConstructionRule sets the TFDZ construction rule.
func WithConstructionRule(rule uint8) FrameOption {
	return func(f *TransferFrame) {
		f.DataFieldHeader.ConstructionRule = rule
	}
}

// WithUPID sets the USLP Protocol Identifier.
func WithUPID(upid uint8) FrameOption {
	return func(f *TransferFrame) {
		f.DataFieldHeader.UPID = upid
	}
}

// WithSequenceNumber sets the frame sequence number.
func WithSequenceNumber(seq uint16) FrameOption {
	return func(f *TransferFrame) {
		f.DataFieldHeader.SequenceNumber = seq
	}
}

// WithFirstHeaderOffset sets the first header offset in the TFDZ.
func WithFirstHeaderOffset(offset uint16) FrameOption {
	return func(f *TransferFrame) {
		f.DataFieldHeader.FirstHeaderOffset = offset
	}
}

// WithSourceOrDest sets the source/destination flag.
func WithSourceOrDest(flag uint8) FrameOption {
	return func(f *TransferFrame) {
		f.Header.SourceOrDest = flag & 0x01
	}
}

// WithEndOfFPH sets the End of Frame Primary Header flag.
// When true, the frame uses the shorter 5-byte header (fixed-length mode).
func WithEndOfFPH() FrameOption {
	return func(f *TransferFrame) {
		f.Header.EndOfFPH = true
	}
}

// NewTransferFrame creates a new USLP Transfer Frame.
// For variable-length frames, set EndOfFPH=false (default).
// For fixed-length frames, the caller should set the frame length via ChannelConfig.
func NewTransferFrame(scid uint16, vcid, mapid uint8, data []byte, opts ...FrameOption) (*TransferFrame, error) {
	frame := &TransferFrame{
		Header: PrimaryHeader{
			TFVN:  TFVN,
			SCID:  scid,
			VCID:  vcid & 0x3F,
			MAPID: mapid & 0x3F,
		},
		DataField: data,
	}

	for _, opt := range opts {
		opt(frame)
	}

	// Compute total frame length and set header
	totalLen := frame.computeTotalLength()
	if totalLen > 65536 {
		return nil, ErrDataTooLarge
	}
	frame.Header.FrameLength = uint16(totalLen - 1)

	// Compute FECF
	if err := frame.computeFECF(); err != nil {
		return nil, err
	}

	return frame, nil
}

// computeTotalLength returns the total frame length in bytes.
func (f *TransferFrame) computeTotalLength() int {
	total := f.Header.Size()
	total += len(f.InsertZone)
	total += DataFieldHeaderSize
	total += len(f.DataField)
	if len(f.OCF) > 0 {
		total += 4
	}
	if f.UseCRC32 {
		total += FECSize32
	} else {
		total += FECSize16
	}
	return total
}

// computeFECF computes the Frame Error Control Field.
func (f *TransferFrame) computeFECF() error {
	encoded, err := f.encodeWithoutFECF()
	if err != nil {
		return err
	}
	if f.UseCRC32 {
		checksum := crc.ComputeCRC32(encoded)
		f.FECF = make([]byte, 4)
		binary.BigEndian.PutUint32(f.FECF, checksum)
	} else {
		checksum := crc.ComputeCRC16(encoded)
		f.FECF = make([]byte, 2)
		binary.BigEndian.PutUint16(f.FECF, checksum)
	}
	return nil
}

// encodeWithoutFECF encodes the frame excluding the FECF.
func (f *TransferFrame) encodeWithoutFECF() ([]byte, error) {
	header, err := f.Header.Encode()
	if err != nil {
		return nil, err
	}

	dfh, err := f.DataFieldHeader.Encode()
	if err != nil {
		return nil, err
	}

	var buf []byte
	buf = append(buf, header...)
	buf = append(buf, f.InsertZone...)
	buf = append(buf, dfh...)
	buf = append(buf, f.DataField...)

	if len(f.OCF) > 0 {
		if len(f.OCF) != 4 {
			return nil, ErrInvalidOCFLength
		}
		buf = append(buf, f.OCF...)
	}

	return buf, nil
}

// Encode converts the USLP Transfer Frame to a byte slice.
func (f *TransferFrame) Encode() ([]byte, error) {
	buf, err := f.encodeWithoutFECF()
	if err != nil {
		return nil, err
	}
	return append(buf, f.FECF...), nil
}

// DecodeTransferFrame parses a byte slice into a USLP Transfer Frame.
// fecSize must be 2 (CRC-16) or 4 (CRC-32). insertZoneLen specifies
// the expected insert zone length (0 if none).
func DecodeTransferFrame(data []byte, fecSize int, insertZoneLen int) (*TransferFrame, error) {
	if len(data) < PrimaryHeaderFixedSize {
		return nil, ErrDataTooShort
	}

	// Decode primary header
	var header PrimaryHeader
	if err := header.Decode(data); err != nil {
		return nil, err
	}

	headerSize := header.Size()
	useCRC32 := fecSize == FECSize32

	// Verify minimum length
	minLen := headerSize + insertZoneLen + DataFieldHeaderSize + fecSize
	if len(data) < minLen {
		return nil, ErrDataTooShort
	}

	// Verify FECF
	fecStart := len(data) - fecSize
	if useCRC32 {
		received := binary.BigEndian.Uint32(data[fecStart:])
		computed := crc.ComputeCRC32(data[:fecStart])
		if received != computed {
			return nil, ErrCRCMismatch
		}
	} else {
		received := binary.BigEndian.Uint16(data[fecStart:])
		computed := crc.ComputeCRC16(data[:fecStart])
		if received != computed {
			return nil, ErrCRCMismatch
		}
	}

	pos := headerSize

	// Extract insert zone
	var insertZone []byte
	if insertZoneLen > 0 {
		insertZone = make([]byte, insertZoneLen)
		copy(insertZone, data[pos:pos+insertZoneLen])
		pos += insertZoneLen
	}

	// Decode data field header
	var dfh DataFieldHeader
	if err := dfh.Decode(data[pos:]); err != nil {
		return nil, err
	}
	pos += DataFieldHeaderSize

	// Determine data field boundaries
	dataEnd := fecStart
	var ocf []byte

	// OCF detection: if there are 4 bytes between data and FECF that would otherwise
	// be unaccounted for, they could be OCF. The caller must know the channel config.
	// For now, we don't auto-detect OCF — callers use DecodeTransferFrameWithOCF.

	dataField := make([]byte, dataEnd-pos)
	copy(dataField, data[pos:dataEnd])

	fecfBytes := make([]byte, fecSize)
	copy(fecfBytes, data[fecStart:])

	return &TransferFrame{
		Header:          header,
		InsertZone:      insertZone,
		DataFieldHeader: dfh,
		DataField:       dataField,
		OCF:             ocf,
		FECF:            fecfBytes,
		UseCRC32:        useCRC32,
	}, nil
}

// DecodeTransferFrameWithOCF decodes a frame that includes a 4-byte OCF.
func DecodeTransferFrameWithOCF(data []byte, fecSize int, insertZoneLen int) (*TransferFrame, error) {
	frame, err := DecodeTransferFrame(data, fecSize, insertZoneLen)
	if err != nil {
		return nil, err
	}

	// Extract OCF from the end of the data field
	if len(frame.DataField) < 4 {
		return nil, ErrDataTooShort
	}
	ocfStart := len(frame.DataField) - 4
	frame.OCF = make([]byte, 4)
	copy(frame.OCF, frame.DataField[ocfStart:])
	frame.DataField = frame.DataField[:ocfStart]

	return frame, nil
}

// IsIdleFrame reports whether the frame is an idle frame.
func IsIdleFrame(frame *TransferFrame) bool {
	return frame.DataFieldHeader.ConstructionRule == RuleIdle
}

// NewIdleFrame creates an idle USLP Transfer Frame with all-idle data field.
func NewIdleFrame(scid uint16, vcid uint8, config ChannelConfig) (*TransferFrame, error) {
	capacity := config.DataFieldCapacity(0)
	if capacity <= 0 {
		return nil, ErrDataFieldTooSmall
	}
	idleData := make([]byte, capacity)
	for i := range idleData {
		idleData[i] = 0xFF
	}
	opts := []FrameOption{
		WithConstructionRule(RuleIdle),
		WithFirstHeaderOffset(FHOAllIdle),
		WithEndOfFPH(),
	}
	if config.HasOCF {
		opts = append(opts, WithOCF(make([]byte, 4)))
	}
	if config.UseCRC32 {
		opts = append(opts, WithCRC32())
	}
	return NewTransferFrame(scid, vcid, 0, idleData, opts...)
}

// padDataField copies data into a new slice of the given capacity,
// filling remaining bytes with 0xFF (idle fill).
func padDataField(data []byte, capacity int) []byte {
	padded := make([]byte, capacity)
	copy(padded, data)
	for i := len(data); i < capacity; i++ {
		padded[i] = 0xFF
	}
	return padded
}

// recomputeFECF re-encodes the frame and updates the FECF.
func recomputeFECF(frame *TransferFrame) error {
	return frame.computeFECF()
}

// Humanize returns a human-readable representation of the TransferFrame.
func (f *TransferFrame) Humanize() string {
	lines := []string{
		"USLP Transfer Frame:",
		"Primary Header:",
		f.Header.Humanize(),
	}
	if len(f.InsertZone) > 0 {
		lines = append(lines, "Insert Zone: "+hex.EncodeToString(f.InsertZone))
	}
	lines = append(lines,
		"Data Field Header:",
		f.DataFieldHeader.Humanize(),
		"Data Field: "+hex.EncodeToString(f.DataField),
	)
	if len(f.OCF) > 0 {
		lines = append(lines, "OCF: "+hex.EncodeToString(f.OCF))
	}
	lines = append(lines, "FECF: "+hex.EncodeToString(f.FECF))
	lines = append(lines, "Idle: "+strconv.FormatBool(IsIdleFrame(f)))
	return strings.Join(lines, "\n")
}
