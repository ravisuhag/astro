// Package aos implements the CCSDS Advanced Orbiting Systems (AOS)
// Space Data Link Protocol per CCSDS 732.0-B-4.
//
// AOS Transfer Frames carry one of three PDU types per Virtual Channel:
// Multiplexing PDUs (M_PDU) for variable-length packets, Bitstream PDUs
// (B_PDU) for octet-aligned bitstreams, or Virtual Channel Access SDUs
// (VCA) for opaque data. Frames are fixed-length on a physical channel
// and may include an Insert Zone, Operational Control Field, and a
// 16-bit Frame Error Control Field.
package aos

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/pkg/crc"
)

// TFVN is the AOS Transfer Frame Version Number (CCSDS 732.0-B-4).
const TFVN = 1 // 0b01

// PrimaryHeaderSize is the fixed size of the AOS primary header in bytes.
const PrimaryHeaderSize = 6

// FECFSize is the size of the Frame Error Control Field in bytes.
const FECFSize = 2

// OCFSize is the size of the Operational Control Field in bytes.
const OCFSize = 4

// OIDVCID is the Virtual Channel ID reserved for Only Idle Data frames.
const OIDVCID = 63

// MaxVCFrameCount is the maximum value of the 24-bit VC Frame Count.
const MaxVCFrameCount = 0xFFFFFF

// M_PDU header constants (CCSDS 732.0-B-4 clause 4.1.4.2).
const (
	// MPDUHeaderSize is the encoded size of the M_PDU header in bytes.
	MPDUHeaderSize = 2
	// MPDUMaxFirstHeaderPointer is the maximum value of the 11-bit FHP.
	MPDUMaxFirstHeaderPointer = 0x07FF
	// FHPNoPacketStart marks a frame in which no packet starts
	// (CCSDS 732.0-B-4 clause 4.1.4.2.3.4: FHP set to 'all ones').
	FHPNoPacketStart uint16 = 0x07FF
	// FHPAllIdle marks an M_PDU that contains only idle data
	// (CCSDS 732.0-B-4 clause 4.1.4.2.3.5: FHP set to 'all ones minus one').
	FHPAllIdle uint16 = 0x07FE
)

// B_PDU header constants (CCSDS 732.0-B-4 clause 4.1.4.3).
const (
	// BPDUHeaderSize is the encoded size of the B_PDU header in bytes.
	BPDUHeaderSize = 2
	// BPDUMaxBitstreamDataPointer is the maximum value of the 14-bit BDP.
	BPDUMaxBitstreamDataPointer = 0x3FFF
	// BDPAllValid indicates the bitstream contains no end-of-data within this frame.
	BDPAllValid uint16 = 0x3FFF
	// BDPAllIdle indicates the bitstream is entirely idle fill.
	BDPAllIdle uint16 = 0x3FFE
)

// PrimaryHeader represents the AOS Transfer Frame Primary Header.
//
// Bit layout (CCSDS 732.0-B-4 clause 4.1.2):
//
//	Byte 0:  TFVN[1:0]  | SCID[7:2]
//	Byte 1:  SCID[1:0]  | VCID[5:0]
//	Byte 2:  VCFrameCount[23:16]
//	Byte 3:  VCFrameCount[15:8]
//	Byte 4:  VCFrameCount[7:0]
//	Byte 5:  Replay | VCFCUsage | reserved(2) | VCFCCycle[3:0]
type PrimaryHeader struct {
	TFVN              uint8  // 2 bits  - Transfer Frame Version Number (must be 1)
	SCID              uint8  // 8 bits  - Spacecraft Identifier
	VCID              uint8  // 6 bits  - Virtual Channel Identifier
	VCFrameCount      uint32 // 24 bits - Virtual Channel Frame Count
	ReplayFlag        bool   // 1 bit   - Replay Flag
	VCFCUsageFlag     bool   // 1 bit   - VC Frame Count Usage Flag
	VCFrameCountCycle uint8  // 4 bits  - VC Frame Count Cycle
}

// MCID returns the Master Channel Identifier (TFVN + SCID).
func (h *PrimaryHeader) MCID() uint16 {
	return uint16(h.TFVN)<<8 | uint16(h.SCID)
}

// GVCID returns the Global Virtual Channel Identifier (MCID + VCID).
func (h *PrimaryHeader) GVCID() uint32 {
	return uint32(h.MCID())<<6 | uint32(h.VCID)
}

// Encode packs the PrimaryHeader fields into a 6-byte slice.
func (h *PrimaryHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	b := make([]byte, PrimaryHeaderSize)

	// Byte 0: TFVN[1:0] | SCID[7:2]
	b[0] = (h.TFVN&0x03)<<6 | (h.SCID >> 2)
	// Byte 1: SCID[1:0] | VCID[5:0]
	b[1] = (h.SCID&0x03)<<6 | (h.VCID & 0x3F)
	// Bytes 2-4: VCFrameCount (24 bits)
	b[2] = uint8(h.VCFrameCount >> 16)
	b[3] = uint8(h.VCFrameCount >> 8)
	b[4] = uint8(h.VCFrameCount)
	// Byte 5: signaling field
	b[5] = 0
	if h.ReplayFlag {
		b[5] |= 1 << 7
	}
	if h.VCFCUsageFlag {
		b[5] |= 1 << 6
	}
	b[5] |= h.VCFrameCountCycle & 0x0F

	return b, nil
}

// Decode parses a byte slice into the PrimaryHeader.
func (h *PrimaryHeader) Decode(data []byte) error {
	if len(data) < PrimaryHeaderSize {
		return ErrDataTooShort
	}

	h.TFVN = (data[0] >> 6) & 0x03
	h.SCID = (data[0]&0x3F)<<2 | (data[1] >> 6)
	h.VCID = data[1] & 0x3F
	h.VCFrameCount = uint32(data[2])<<16 | uint32(data[3])<<8 | uint32(data[4])
	h.ReplayFlag = data[5]&(1<<7) != 0
	h.VCFCUsageFlag = data[5]&(1<<6) != 0
	// CCSDS 732.0-B-4 clause 4.1.2.6.5 (signaling field): bits 42-43 are reserved
	// spares and shall be set to '00'.
	if data[5]&0x30 != 0 {
		return ErrInvalidSignalingSpare
	}
	h.VCFrameCountCycle = data[5] & 0x0F

	return h.Validate()
}

// Validate checks that header values are within their bit-field widths
// and that TFVN is the AOS version.
func (h *PrimaryHeader) Validate() error {
	if h.TFVN != TFVN {
		return ErrInvalidVersion
	}
	if h.VCID > 0x3F {
		return ErrInvalidVCID
	}
	if h.VCFrameCount > MaxVCFrameCount {
		return ErrInvalidVCFrameCount
	}
	if h.VCFrameCountCycle > 0x0F {
		return ErrInvalidVCFrameCountCycle
	}
	return nil
}

// Humanize returns a multi-line human-readable representation of the header.
func (h *PrimaryHeader) Humanize() string {
	return strings.Join([]string{
		"  TFVN: " + strconv.Itoa(int(h.TFVN)),
		"  Spacecraft ID: " + strconv.Itoa(int(h.SCID)),
		"  Virtual Channel ID: " + strconv.Itoa(int(h.VCID)),
		"  VC Frame Count: " + strconv.FormatUint(uint64(h.VCFrameCount), 10),
		"  Replay Flag: " + strconv.FormatBool(h.ReplayFlag),
		"  VC Frame Count Usage Flag: " + strconv.FormatBool(h.VCFCUsageFlag),
		"  VC Frame Count Cycle: " + strconv.Itoa(int(h.VCFrameCountCycle)),
	}, "\n")
}

// MPDUHeader represents an M_PDU header carried at the start of a
// Transfer Frame Data Field that uses the M_PDU service.
//
// Bit layout (CCSDS 732.0-B-4 clause 4.1.4.2):
//
//	Byte 0: reserved(5) | FHP[10:8]
//	Byte 1: FHP[7:0]
type MPDUHeader struct {
	FirstHeaderPointer uint16 // 11 bits
}

// Encode packs the M_PDU header into 2 bytes.
func (h *MPDUHeader) Encode() ([]byte, error) {
	if h.FirstHeaderPointer > MPDUMaxFirstHeaderPointer {
		return nil, ErrInvalidFirstHeaderPointer
	}
	b := make([]byte, MPDUHeaderSize)
	b[0] = uint8((h.FirstHeaderPointer >> 8) & 0x07)
	b[1] = uint8(h.FirstHeaderPointer & 0xFF)
	return b, nil
}

// Decode parses an M_PDU header from the start of data.
func (h *MPDUHeader) Decode(data []byte) error {
	if len(data) < MPDUHeaderSize {
		return ErrDataTooShort
	}
	h.FirstHeaderPointer = (uint16(data[0]&0x07) << 8) | uint16(data[1])
	return nil
}

// BPDUHeader represents a B_PDU header carried at the start of a
// Transfer Frame Data Field that uses the B_PDU service.
//
// Bit layout (CCSDS 732.0-B-4 clause 4.1.4.3):
//
//	Byte 0: reserved(2) | BDP[13:8]
//	Byte 1: BDP[7:0]
type BPDUHeader struct {
	BitstreamDataPointer uint16 // 14 bits
}

// Encode packs the B_PDU header into 2 bytes.
func (h *BPDUHeader) Encode() ([]byte, error) {
	if h.BitstreamDataPointer > BPDUMaxBitstreamDataPointer {
		return nil, ErrInvalidBitstreamDataPointer
	}
	b := make([]byte, BPDUHeaderSize)
	b[0] = uint8((h.BitstreamDataPointer >> 8) & 0x3F)
	b[1] = uint8(h.BitstreamDataPointer & 0xFF)
	return b, nil
}

// Decode parses a B_PDU header from the start of data.
func (h *BPDUHeader) Decode(data []byte) error {
	if len(data) < BPDUHeaderSize {
		return ErrDataTooShort
	}
	h.BitstreamDataPointer = (uint16(data[0]&0x3F) << 8) | uint16(data[1])
	return nil
}

// TransferFrame represents an AOS Transfer Frame per CCSDS 732.0-B-4.
//
// Layout: PrimaryHeader | InsertZone? | DataField | OCF? | FECF?
//
// The DataField carries one of M_PDU, B_PDU, or VCA payload depending on
// the Virtual Channel configuration. The frame structure does not encode
// which PDU type is in use. That is determined per-VC by mission config.
type TransferFrame struct {
	Header     PrimaryHeader
	FHEC       []byte // 2 bytes when present (Frame Header Error Control)
	InsertZone []byte // optional, mission-defined fixed length
	DataField  []byte // includes M_PDU/B_PDU header when applicable
	OCF        []byte // 4 bytes when present
	FECF       []byte // 2 bytes when present
	HasFHEC    bool
	HasFECF    bool
}

// FrameOption configures optional fields on a TransferFrame.
type FrameOption func(*TransferFrame)

// WithInsertZone sets the insert zone data.
func WithInsertZone(data []byte) FrameOption {
	return func(f *TransferFrame) { f.InsertZone = data }
}

// WithOCF sets the Operational Control Field.
func WithOCF(ocf []byte) FrameOption {
	return func(f *TransferFrame) { f.OCF = ocf }
}

// WithFECF enables the Frame Error Control Field (CRC-16-CCITT).
func WithFECF() FrameOption {
	return func(f *TransferFrame) { f.HasFECF = true }
}

// WithFHEC enables the Frame Header Error Control (RS(10,6) check symbols
// over the protected header octets, CCSDS 732.0-B-4 clause 4.1.2.6).
func WithFHEC() FrameOption {
	return func(f *TransferFrame) { f.HasFHEC = true }
}

// WithVCFrameCount sets the VC frame count.
//
// A count wider than the 24-bit field is stored as given and refused by
// Encode with ErrInvalidVCFrameCount. Masking it here would substitute a
// different count silently — a value of 2^24 would go out as 0 — and a
// frame count is how a receiver detects gaps, so a substituted one reads
// as a lost frame rather than a caller mistake.
func WithVCFrameCount(count uint32) FrameOption {
	return func(f *TransferFrame) { f.Header.VCFrameCount = count }
}

// WithReplayFlag sets the Replay Flag.
func WithReplayFlag() FrameOption {
	return func(f *TransferFrame) { f.Header.ReplayFlag = true }
}

// WithVCFCUsage enables the VC Frame Count Usage Flag and sets the cycle.
//
// A cycle wider than the 4-bit field is stored as given and refused by
// Encode with ErrInvalidVCFrameCountCycle, for the same reason as
// WithVCFrameCount.
func WithVCFCUsage(cycle uint8) FrameOption {
	return func(f *TransferFrame) {
		f.Header.VCFCUsageFlag = true
		f.Header.VCFrameCountCycle = cycle
	}
}

// NewTransferFrame creates a new AOS Transfer Frame.
//
// data is the Transfer Frame Data Field as it will appear on the wire,
// including any M_PDU or B_PDU header when applicable. Use the Pack helpers
// (PackMPDUDataField, PackBPDUDataField) to build a data field with header.
// A VCID wider than the 6-bit field is stored as given and refused by
// Encode with ErrInvalidVCID. Masking it here would route the frame to a
// different virtual channel than the caller named, silently.
func NewTransferFrame(scid, vcid uint8, data []byte, opts ...FrameOption) (*TransferFrame, error) {
	frame := &TransferFrame{
		Header: PrimaryHeader{
			TFVN: TFVN,
			SCID: scid,
			VCID: vcid,
		},
		DataField: data,
	}

	for _, opt := range opts {
		opt(frame)
	}

	if frame.HasFECF {
		if err := frame.computeFECF(); err != nil {
			return nil, err
		}
	}

	return frame, nil
}

// PackMPDUDataField returns a Transfer Frame Data Field that begins with
// an M_PDU header pointing to fhp, followed by data. Use FHPNoPacketStart
// or FHPAllIdle for the special pointer values.
func PackMPDUDataField(fhp uint16, data []byte) ([]byte, error) {
	hdr := MPDUHeader{FirstHeaderPointer: fhp}
	hb, err := hdr.Encode()
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(hb)+len(data))
	out = append(out, hb...)
	out = append(out, data...)
	return out, nil
}

// PackBPDUDataField returns a Transfer Frame Data Field that begins with
// a B_PDU header carrying bdp, followed by data. Use BDPAllValid or
// BDPAllIdle for the special pointer values.
func PackBPDUDataField(bdp uint16, data []byte) ([]byte, error) {
	hdr := BPDUHeader{BitstreamDataPointer: bdp}
	hb, err := hdr.Encode()
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(hb)+len(data))
	out = append(out, hb...)
	out = append(out, data...)
	return out, nil
}

// computeFECF computes the Frame Error Control Field over the frame
// excluding the FECF itself.
func (f *TransferFrame) computeFECF() error {
	encoded, err := f.encodeWithoutFECF()
	if err != nil {
		return err
	}
	checksum := crc.ComputeCRC16(encoded)
	f.FECF = make([]byte, FECFSize)
	binary.BigEndian.PutUint16(f.FECF, checksum)
	return nil
}

// EncodeWithoutFECF serializes the frame excluding the Frame Error Control
// Field. Use it to build frames with a deliberately invalid FECF; Encode
// always writes a correct one.
func (f *TransferFrame) EncodeWithoutFECF() ([]byte, error) {
	return f.encodeWithoutFECF()
}

// encodeWithoutFECF encodes the frame excluding the FECF.
func (f *TransferFrame) encodeWithoutFECF() ([]byte, error) {
	header, err := f.Header.Encode()
	if err != nil {
		return nil, err
	}

	var buf []byte
	buf = append(buf, header...)
	if f.HasFHEC {
		fhec, err := ComputeFHEC(header)
		if err != nil {
			return nil, err
		}
		f.FHEC = fhec
		buf = append(buf, fhec...)
	}
	buf = append(buf, f.InsertZone...)
	buf = append(buf, f.DataField...)

	if len(f.OCF) > 0 {
		if len(f.OCF) != OCFSize {
			return nil, ErrInvalidOCFLength
		}
		buf = append(buf, f.OCF...)
	}

	return buf, nil
}

// Encode converts the AOS Transfer Frame to a byte slice.
//
// When the frame carries a Frame Error Control Field, it is computed from
// the frame's current contents on every call, so changes made after
// construction are always covered. Use EncodeWithoutFECF to build a frame
// with a deliberately invalid FECF.
func (f *TransferFrame) Encode() ([]byte, error) {
	buf, err := f.encodeWithoutFECF()
	if err != nil {
		return nil, err
	}
	if !f.HasFECF {
		return buf, nil
	}

	// Compute the FECF from the frame's current contents, so fields changed
	// after construction are covered, and refresh the exported field.
	if err := f.computeFECF(); err != nil {
		return nil, err
	}
	return append(buf, f.FECF...), nil
}

// DecodeTransferFrame parses a byte slice into an AOS Transfer Frame.
//
// insertZoneLen is the configured insert zone length for the physical
// channel (0 if none). hasOCF and hasFECF select the optional trailing
// fields. Frames are fixed-length per physical channel; the caller is
// responsible for delivering exactly one frame.
func DecodeTransferFrame(data []byte, insertZoneLen int, hasOCF, hasFECF bool) (*TransferFrame, error) {
	return decodeFrame(data, insertZoneLen, hasOCF, hasFECF, false)
}

// DecodeFrame parses a byte slice into an AOS Transfer Frame using the
// physical channel configuration for the optional fields, including the
// Frame Header Error Control.
//
// Deprecated: use DecodeTransferFrameWithConfig, which is the name every
// data-link package now uses. This forwarder will be removed in v1.0.
func DecodeFrame(data []byte, config ChannelConfig) (*TransferFrame, error) {
	return DecodeTransferFrameWithConfig(data, config)
}

// DecodeTransferFrameWithConfig parses a byte slice into an AOS Transfer
// Frame using the physical channel configuration for the optional fields,
// including the Frame Header Error Control.
func DecodeTransferFrameWithConfig(data []byte, config ChannelConfig) (*TransferFrame, error) {
	return decodeFrame(data, config.InsertZoneLen, config.HasOCF, config.HasFECF, config.HasFHEC)
}

func decodeFrame(data []byte, insertZoneLen int, hasOCF, hasFECF, hasFHEC bool) (*TransferFrame, error) {
	minLen := PrimaryHeaderSize + insertZoneLen
	if hasFHEC {
		minLen += FHECSize
	}
	if hasOCF {
		minLen += OCFSize
	}
	if hasFECF {
		minLen += FECFSize
	}
	if len(data) < minLen {
		return nil, ErrDataTooShort
	}

	// Verify whole-frame integrity first: a frame that fails the FECF is
	// rejected before any header field is interpreted.
	end := len(data)
	var fecf []byte
	if hasFECF {
		fecStart := end - FECFSize
		received := binary.BigEndian.Uint16(data[fecStart:end])
		computed := crc.ComputeCRC16(data[:fecStart])
		if received != computed {
			return nil, ErrCRCMismatch
		}
		fecf = make([]byte, FECFSize)
		copy(fecf, data[fecStart:end])
		end = fecStart
	}

	var header PrimaryHeader
	if err := header.Decode(data[:PrimaryHeaderSize]); err != nil {
		return nil, err
	}

	var fhec []byte
	if hasFHEC {
		fhec = make([]byte, FHECSize)
		copy(fhec, data[PrimaryHeaderSize:PrimaryHeaderSize+FHECSize])
		if !VerifyFHEC(data[:PrimaryHeaderSize], fhec) {
			return nil, ErrFHECMismatch
		}
	}

	var ocf []byte
	if hasOCF {
		ocfStart := end - OCFSize
		ocf = make([]byte, OCFSize)
		copy(ocf, data[ocfStart:end])
		end = ocfStart
	}

	pos := PrimaryHeaderSize
	if hasFHEC {
		pos += FHECSize
	}
	var insertZone []byte
	if insertZoneLen > 0 {
		insertZone = make([]byte, insertZoneLen)
		copy(insertZone, data[pos:pos+insertZoneLen])
		pos += insertZoneLen
	}

	dataField := make([]byte, end-pos)
	copy(dataField, data[pos:end])

	return &TransferFrame{
		Header:     header,
		FHEC:       fhec,
		InsertZone: insertZone,
		DataField:  dataField,
		OCF:        ocf,
		FECF:       fecf,
		HasFHEC:    hasFHEC,
		HasFECF:    hasFECF,
	}, nil
}

// IsIdleFrame reports whether the frame is an Only Idle Data frame.
// Per CCSDS 732.0-B-4 clause 4.1.2.2.4, OID frames use VCID 63.
func IsIdleFrame(frame *TransferFrame) bool {
	return frame.Header.VCID == OIDVCID
}

// DefaultIdleFill is the idle fill byte used when ChannelConfig.IdlePattern
// is empty. The idle pattern is a mission-managed parameter; override it
// via ChannelConfig.IdlePattern.
const DefaultIdleFill byte = 0xFE

// fillIdle writes the repeating idle pattern into buf.
func fillIdle(buf []byte, pattern []byte) {
	if len(pattern) == 0 {
		pattern = []byte{DefaultIdleFill}
	}
	for i := range buf {
		buf[i] = pattern[i%len(pattern)]
	}
}

// padDataField copies data into a new slice of the given capacity,
// filling remaining bytes with the channel's idle fill pattern.
func padDataField(data []byte, capacity int, pattern []byte) []byte {
	padded := make([]byte, capacity)
	copy(padded, data)
	fillIdle(padded[len(data):], pattern)
	return padded
}

// NewIdleFrame creates an OID (Only Idle Data) Transfer Frame on VCID 63
// with a data field of the given capacity filled with the channel's idle
// pattern.
func NewIdleFrame(scid uint8, config ChannelConfig) (*TransferFrame, error) {
	capacity := config.DataFieldCapacity()
	if capacity <= 0 {
		return nil, ErrDataFieldTooSmall
	}
	idleData := make([]byte, capacity)
	fillIdle(idleData, config.IdlePattern)

	opts := frameOpts(config)
	return NewTransferFrame(scid, OIDVCID, idleData, opts...)
}

// recomputeFECF re-encodes the frame and updates the FECF.
func recomputeFECF(frame *TransferFrame) error {
	if !frame.HasFECF {
		return nil
	}
	return frame.computeFECF()
}

// Humanize returns a multi-line human-readable representation of the frame.
func (f *TransferFrame) Humanize() string {
	lines := []string{
		"AOS Transfer Frame:",
		"Primary Header:",
		f.Header.Humanize(),
	}
	if len(f.FHEC) > 0 {
		lines = append(lines, "FHEC: "+hex.EncodeToString(f.FHEC))
	}
	if len(f.InsertZone) > 0 {
		lines = append(lines, "Insert Zone: "+hex.EncodeToString(f.InsertZone))
	}
	lines = append(lines, "Data Field: "+hex.EncodeToString(f.DataField))
	if len(f.OCF) > 0 {
		lines = append(lines, "OCF: "+hex.EncodeToString(f.OCF))
	}
	if len(f.FECF) > 0 {
		lines = append(lines, "FECF: "+hex.EncodeToString(f.FECF))
	}
	lines = append(lines, "Idle: "+strconv.FormatBool(IsIdleFrame(f)))
	return strings.Join(lines, "\n")
}
