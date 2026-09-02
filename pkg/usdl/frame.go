// Package usdl implements the CCSDS Unified Space Data Link Protocol
// (USLP) per CCSDS 732.1-B-3.
//
// USLP Transfer Frames carry a Transfer Frame Data Field whose layout is
// declared in-band by the TFDZ Construction Rules of the TFDF Header.
// Frames are either non-truncated (full primary header with frame length,
// flags, and an optional Virtual Channel Frame Count of 0-7 octets) or
// truncated (a fixed 4-octet header for short telecommands, annex D).
package usdl

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/internal/pn"
	"github.com/ravisuhag/astro/pkg/crc"
)

// TFVN is the USLP Transfer Frame Version Number (CCSDS 732.1-B-3
// Clause 4.1.2.2.2: '1100').
const TFVN = 12 // 0b1100

// TFDZ Construction Rules per CCSDS 732.1-B-3 clause 4.1.4.2.2.2.
const (
	// RulePacketsSpanning ('000'): fixed-length TFDZ of concatenated CCSDS
	// packets that may span frame boundaries; the First Header Pointer is
	// required.
	RulePacketsSpanning uint8 = 0b000
	// RuleStartOfSDU ('001'): fixed-length TFDZ holding the start of (or a
	// complete) MAPA_SDU/VCA_SDU; the Last Valid Octet Pointer is required.
	RuleStartOfSDU uint8 = 0b001
	// RuleContinuingSDU ('010'): fixed-length TFDZ continuing a
	// MAPA_SDU/VCA_SDU started earlier; the Last Valid Octet Pointer is
	// required.
	RuleContinuingSDU uint8 = 0b010
	// RuleOctetStream ('011'): variable-length TFDZ carrying a continuous
	// octet-aligned stream.
	RuleOctetStream uint8 = 0b011
	// RuleStartingSegment ('100'): variable-length TFDZ with the starting
	// segment of a segmented SDU.
	RuleStartingSegment uint8 = 0b100
	// RuleContinuingSegment ('101'): variable-length TFDZ with a continuing
	// segment.
	RuleContinuingSegment uint8 = 0b101
	// RuleLastSegment ('110'): variable-length TFDZ with the last segment.
	RuleLastSegment uint8 = 0b110
	// RuleNoSegmentation ('111'): variable-length TFDZ holding complete
	// SDUs or packets, unsegmented.
	RuleNoSegmentation uint8 = 0b111
)

// Special pointer values (CCSDS 732.1-B-3 clause 4.1.4.2.4).
const (
	// FHPNoPacketStart: for rule '000', no packet starts within this TFDZ
	// ('all ones', clause 4.1.4.2.4.4).
	FHPNoPacketStart uint16 = 0xFFFF
	// LVOPIncomplete: for rules '001'/'010', the SDU does not complete
	// within this TFDZ ('all ones', clause 4.1.4.2.4.6).
	LVOPIncomplete uint16 = 0xFFFF
)

// USLP Protocol Identifiers from the SANA UPID registry
// (https://sanaregistry.org/r/uslp_protocol_id, CCSDS 732.1-B-3 clause 4.1.4.2.3).
const (
	UPIDSpacePackets       uint8 = 0  // Space Packets or Encapsulation Packets
	UPIDCOP1Control        uint8 = 1  // COP-1 Control Commands
	UPIDCOPPControl        uint8 = 2  // COP-P Control Commands
	UPIDUserOctetStream    uint8 = 4  // User-defined Octet Stream
	UPIDMissionSpecific1   uint8 = 5  // Mission Specific Information-1 (one MAPA_SDU)
	UPIDProx1PseudoPacket1 uint8 = 6  // Proximity-1 Pseudo Packet ID 1
	UPIDProx1SPDU          uint8 = 7  // Proximity-1 SPDUs
	UPIDProx1PseudoPacket2 uint8 = 8  // Proximity-1 Pseudo Packet ID 2
	UPIDIdle               uint8 = 31 // Idle Data (OID frames)
)

// OIDVCID is the Virtual Channel ID reserved for Only Idle Data frames
// (CCSDS 732.1-B-3 clause 4.1.4.1.6: 'all ones').
const OIDVCID = 63

// Primary header sizes.
const (
	// TruncatedPrimaryHeaderSize is the fixed size of the truncated
	// primary header (annex D1.3: exactly 4 octets).
	TruncatedPrimaryHeaderSize = 4
	// PrimaryHeaderBaseSize is the size of the non-truncated primary
	// header before the variable Virtual Channel Frame Count.
	PrimaryHeaderBaseSize = 7
	// MaxVCFCountLen is the largest VCF Count field length in octets
	// (clause 4.1.2.11, table 4-2).
	MaxVCFCountLen = 7
)

// PrimaryHeader represents the USLP Transfer Frame Primary Header.
//
// Bit layout (CCSDS 732.1-B-3 clause 4.1.2):
//
//	Byte 0:  TFVN[3:0]    | SCID[15:12]
//	Byte 1:  SCID[11:4]
//	Byte 2:  SCID[3:0]    | SourceOrDest | VCID[5:3]
//	Byte 3:  VCID[2:0]    | MAPID[3:0]   | EndOfFPH
//
// EndOfFPH = 1 marks the truncated header, which is exactly these 4
// octets (annex D). A non-truncated header (EndOfFPH = 0) continues:
//
//	Bytes 4-5: Frame Length (total octets - 1)
//	Byte 6:    Bypass/SeqCtrl | ProtCtrlCmd | spares(2) | OCFFlag | VCFCountLen[2:0]
//	Bytes 7+:  VCF Count (VCFCountLen octets, big-endian)
type PrimaryHeader struct {
	TFVN          uint8  // 4 bits  - Transfer Frame Version Number (must be 12 = 0b1100)
	SCID          uint16 // 16 bits - Spacecraft Identifier
	SourceOrDest  uint8  // 1 bit   - 0=SCID is source, 1=SCID is destination
	VCID          uint8  // 6 bits  - Virtual Channel Identifier (0-63)
	MAPID         uint8  // 4 bits  - Multiplexer Access Point Identifier (0-15)
	EndOfFPH      bool   // 1 bit   - End of Frame Primary Header flag (truncated header)
	FrameLength   uint16 // 16 bits - total frame octets minus 1 (non-truncated only)
	BypassSeqCtrl bool   // 1 bit   - Bypass/Sequence Control flag (1 = expedited)
	ProtCtrlCmd   bool   // 1 bit   - Protocol Control Command flag (1 = protocol control)
	OCFFlag       bool   // 1 bit   - Operational Control Field present
	VCFCountLen   uint8  // 3 bits  - VCF Count field length in octets (0-7)
	VCFCount      uint64 // 0-56 bits - Virtual Channel Frame Count
}

// Size returns the encoded size of the primary header in bytes.
func (h *PrimaryHeader) Size() int {
	if h.EndOfFPH {
		return TruncatedPrimaryHeaderSize
	}
	return PrimaryHeaderBaseSize + int(h.VCFCountLen)
}

// MCID returns the Master Channel Identifier (TFVN + SCID).
func (h *PrimaryHeader) MCID() uint32 {
	return uint32(h.TFVN)<<16 | uint32(h.SCID)
}

// GVCID returns the Global Virtual Channel Identifier (MCID + VCID).
func (h *PrimaryHeader) GVCID() uint32 {
	return h.MCID()<<6 | uint32(h.VCID)
}

// GMAPID returns the Global MAP Identifier (GVCID + MAP ID).
func (h *PrimaryHeader) GMAPID() uint32 {
	return h.GVCID()<<4 | uint32(h.MAPID)
}

// maxVCFCount returns the largest count the configured VCF Count field
// width can carry.
func maxVCFCount(countLen uint8) uint64 {
	if countLen == 0 {
		return 0
	}
	if countLen >= 8 {
		return ^uint64(0)
	}
	return 1<<(8*countLen) - 1
}

// Encode packs the PrimaryHeader fields into a byte slice.
func (h *PrimaryHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	b := make([]byte, h.Size())

	// Byte 0: TFVN[3:0] | SCID[15:12]
	b[0] = h.TFVN<<4 | uint8(h.SCID>>12)
	// Byte 1: SCID[11:4]
	b[1] = uint8(h.SCID >> 4)
	// Byte 2: SCID[3:0] | SourceOrDest | VCID[5:3]
	b[2] = uint8(h.SCID&0x0F)<<4 | (h.SourceOrDest&0x01)<<3 | h.VCID>>3
	// Byte 3: VCID[2:0] | MAPID[3:0] | EndOfFPH
	b[3] = (h.VCID&0x07)<<5 | (h.MAPID&0x0F)<<1
	if h.EndOfFPH {
		b[3] |= 0x01
		return b, nil
	}

	// Bytes 4-5: Frame Length (total - 1)
	binary.BigEndian.PutUint16(b[4:6], h.FrameLength)
	// Byte 6: Bypass | ProtCtrlCmd | spares(00) | OCFFlag | VCFCountLen
	if h.BypassSeqCtrl {
		b[6] |= 1 << 7
	}
	if h.ProtCtrlCmd {
		b[6] |= 1 << 6
	}
	if h.OCFFlag {
		b[6] |= 1 << 3
	}
	b[6] |= h.VCFCountLen & 0x07
	// Bytes 7+: VCF Count, big-endian.
	for i := 0; i < int(h.VCFCountLen); i++ {
		b[7+i] = uint8(h.VCFCount >> (8 * (int(h.VCFCountLen) - 1 - i)))
	}

	return b, nil
}

// Decode parses a byte slice into the PrimaryHeader.
func (h *PrimaryHeader) Decode(data []byte) error {
	if len(data) < TruncatedPrimaryHeaderSize {
		return ErrDataTooShort
	}

	h.TFVN = data[0] >> 4
	h.SCID = uint16(data[0]&0x0F)<<12 | uint16(data[1])<<4 | uint16(data[2]>>4)
	h.SourceOrDest = (data[2] >> 3) & 0x01
	h.VCID = (data[2]&0x07)<<3 | data[3]>>5
	h.MAPID = (data[3] >> 1) & 0x0F
	h.EndOfFPH = data[3]&0x01 != 0
	h.FrameLength = 0
	h.BypassSeqCtrl = false
	h.ProtCtrlCmd = false
	h.OCFFlag = false
	h.VCFCountLen = 0
	h.VCFCount = 0

	if h.EndOfFPH {
		return h.Validate()
	}

	if len(data) < PrimaryHeaderBaseSize {
		return ErrDataTooShort
	}
	h.FrameLength = binary.BigEndian.Uint16(data[4:6])
	h.BypassSeqCtrl = data[6]&(1<<7) != 0
	h.ProtCtrlCmd = data[6]&(1<<6) != 0
	// Clause 4.1.2.9: bits 50-51 are reserved spares and shall be '00'.
	if data[6]&0x30 != 0 {
		return ErrInvalidHeaderSpare
	}
	h.OCFFlag = data[6]&(1<<3) != 0
	h.VCFCountLen = data[6] & 0x07

	if len(data) < PrimaryHeaderBaseSize+int(h.VCFCountLen) {
		return ErrDataTooShort
	}
	for i := 0; i < int(h.VCFCountLen); i++ {
		h.VCFCount = h.VCFCount<<8 | uint64(data[7+i])
	}

	return h.Validate()
}

// Validate checks that header values are within their bit-field widths.
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
	if h.MAPID > 0x0F {
		return ErrInvalidMAPID
	}
	if h.VCFCountLen > MaxVCFCountLen {
		return ErrInvalidVCFCountLen
	}
	if h.VCFCount > maxVCFCount(h.VCFCountLen) {
		return ErrInvalidVCFCount
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
		"  Truncated Header: " + strconv.FormatBool(h.EndOfFPH),
	}
	if !h.EndOfFPH {
		lines = append(lines,
			"  Frame Length: "+strconv.Itoa(int(h.FrameLength)+1)+" bytes",
			"  Bypass/Seq Ctrl: "+strconv.FormatBool(h.BypassSeqCtrl),
			"  Protocol Ctrl Cmd: "+strconv.FormatBool(h.ProtCtrlCmd),
			"  OCF Present: "+strconv.FormatBool(h.OCFFlag),
			"  VCF Count Length: "+strconv.Itoa(int(h.VCFCountLen)),
		)
		if h.VCFCountLen > 0 {
			lines = append(lines, "  VCF Count: "+strconv.FormatUint(h.VCFCount, 10))
		}
	}
	return strings.Join(lines, "\n")
}

// DataFieldHeader represents the USLP Transfer Frame Data Field Header
// (TFDF Header, CCSDS 732.1-B-3 clause 4.1.4.2): one mandatory octet holding
// the TFDZ Construction Rules (3 bits) and the UPID (5 bits), followed by
// a 16-bit First Header Pointer / Last Valid Octet Pointer only for the
// construction rules that require one ('000', '001', '010').
type DataFieldHeader struct {
	ConstructionRule uint8  // 3 bits - TFDZ construction rule
	UPID             uint8  // 5 bits - USLP Protocol Identifier
	Pointer          uint16 // 16 bits - FHP (rule '000') or LVOP (rules '001'/'010')
}

// HasPointer reports whether the construction rule carries the 16-bit
// pointer field (clause 4.1.4.2.4.1: rules '000', '001', and '010' only).
func (h *DataFieldHeader) HasPointer() bool {
	return h.ConstructionRule <= RuleContinuingSDU
}

// Size returns the encoded size of the TFDF header in bytes: 1, or 3 when
// the construction rule carries a pointer.
func (h *DataFieldHeader) Size() int {
	if h.HasPointer() {
		return 3
	}
	return 1
}

// Encode packs the DataFieldHeader into a byte slice.
func (h *DataFieldHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	b := make([]byte, h.Size())
	b[0] = h.ConstructionRule<<5 | h.UPID&0x1F
	if h.HasPointer() {
		binary.BigEndian.PutUint16(b[1:3], h.Pointer)
	}
	return b, nil
}

// Decode parses a TFDF header from the start of data.
func (h *DataFieldHeader) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}
	h.ConstructionRule = data[0] >> 5
	h.UPID = data[0] & 0x1F
	h.Pointer = 0
	if h.HasPointer() {
		if len(data) < 3 {
			return ErrDataTooShort
		}
		h.Pointer = binary.BigEndian.Uint16(data[1:3])
	}
	return nil
}

// Validate checks the data field header values.
func (h *DataFieldHeader) Validate() error {
	if h.ConstructionRule > 7 {
		return ErrInvalidConstructionRule
	}
	if h.UPID > 0x1F {
		return fmt.Errorf("invalid UPID: must be in range 0-31 (5 bits)")
	}
	return nil
}

// constructionRuleName maps a rule value to its spec name.
func constructionRuleName(rule uint8) string {
	switch rule {
	case RulePacketsSpanning:
		return "Packets Spanning Multiple Frames"
	case RuleStartOfSDU:
		return "Start of MAPA_SDU/VCA_SDU"
	case RuleContinuingSDU:
		return "Continuing Portion of MAPA_SDU/VCA_SDU"
	case RuleOctetStream:
		return "Octet Stream"
	case RuleStartingSegment:
		return "Starting Segment"
	case RuleContinuingSegment:
		return "Continuing Segment"
	case RuleLastSegment:
		return "Last Segment"
	case RuleNoSegmentation:
		return "No Segmentation"
	}
	return "Unknown"
}

// Humanize returns a human-readable representation of the DataFieldHeader.
func (h *DataFieldHeader) Humanize() string {
	lines := []string{
		"  Construction Rule: " + strconv.Itoa(int(h.ConstructionRule)) + " (" + constructionRuleName(h.ConstructionRule) + ")",
		"  UPID: " + strconv.Itoa(int(h.UPID)),
	}
	if h.HasPointer() {
		lines = append(lines, "  Pointer: "+fmt.Sprintf("0x%04X", h.Pointer))
	}
	return strings.Join(lines, "\n")
}

// FECSize16 is the size of the 16-bit Frame Error Control Field
// (clause 4.1.6.2.2: when present, the FECF occupies the last 16 bits of every
// Transfer Frame, USLP has no other FECF size).
const FECSize16 = 2

// OCFSize is the size of the Operational Control Field in bytes.
const OCFSize = 4

// TransferFrame represents a USLP Transfer Frame per CCSDS 732.1-B-3.
//
// Layout: PrimaryHeader | InsertZone? | TFDF Header | TFDZ | OCF? | FECF?
//
// Truncated frames (annex D) carry only the 4-octet primary header, a
// 1-octet TFDF header, and the TFDZ: no insert zone, OCF, or FECF.
type TransferFrame struct {
	Header          PrimaryHeader
	InsertZone      []byte          // optional, fixed length per physical channel
	DataFieldHeader DataFieldHeader // TFDF header
	DataField       []byte          // Transfer Frame Data Zone (TFDZ)
	OCF             []byte          // 4 bytes when present (signaled by OCFFlag)
	FECF            []byte          // Frame Error Control Field (16-bit CRC)
	HasFECF         bool            // FECF present (managed per physical channel)
}

// FrameOption configures optional fields on a TransferFrame.
type FrameOption func(*TransferFrame)

// WithInsertZone sets the insert zone data.
func WithInsertZone(data []byte) FrameOption {
	return func(f *TransferFrame) { f.InsertZone = data }
}

// WithOCF sets the Operational Control Field. Its presence is signaled by
// the OCF Flag in the primary header.
func WithOCF(ocf []byte) FrameOption {
	return func(f *TransferFrame) { f.OCF = ocf }
}

// WithoutFECF omits the Frame Error Control Field. Its presence is a
// managed parameter of the physical channel (clause 4.1.6.2.1).
func WithoutFECF() FrameOption {
	return func(f *TransferFrame) { f.HasFECF = false }
}

// WithConstructionRule sets the TFDZ construction rule.
func WithConstructionRule(rule uint8) FrameOption {
	return func(f *TransferFrame) { f.DataFieldHeader.ConstructionRule = rule }
}

// WithUPID sets the USLP Protocol Identifier.
func WithUPID(upid uint8) FrameOption {
	return func(f *TransferFrame) { f.DataFieldHeader.UPID = upid }
}

// WithPointer sets the First Header Pointer / Last Valid Octet Pointer.
// It is encoded only for construction rules '000', '001', and '010'.
func WithPointer(p uint16) FrameOption {
	return func(f *TransferFrame) { f.DataFieldHeader.Pointer = p }
}

// WithSourceOrDest sets the source-or-destination flag.
func WithSourceOrDest(flag uint8) FrameOption {
	return func(f *TransferFrame) { f.Header.SourceOrDest = flag & 0x01 }
}

// WithBypassSeqCtrl marks the frame as expedited (bypass flag set).
func WithBypassSeqCtrl() FrameOption {
	return func(f *TransferFrame) { f.Header.BypassSeqCtrl = true }
}

// WithProtCtrlCmd marks the TFDF as carrying protocol control commands.
func WithProtCtrlCmd() FrameOption {
	return func(f *TransferFrame) { f.Header.ProtCtrlCmd = true }
}

// WithVCFCount sets the Virtual Channel Frame Count and its field length
// in octets (0-7).
func WithVCFCount(countLen uint8, count uint64) FrameOption {
	return func(f *TransferFrame) {
		f.Header.VCFCountLen = countLen
		f.Header.VCFCount = count
	}
}

// NewTransferFrame creates a new non-truncated USLP Transfer Frame. The
// frame length field is computed from the frame contents, the OCF flag
// from OCF presence, and the FECF (present by default) from the encoded
// frame.
func NewTransferFrame(scid uint16, vcid, mapid uint8, data []byte, opts ...FrameOption) (*TransferFrame, error) {
	frame := &TransferFrame{
		Header: PrimaryHeader{
			TFVN:  TFVN,
			SCID:  scid,
			VCID:  vcid & 0x3F,
			MAPID: mapid & 0x0F,
		},
		DataField: data,
		HasFECF:   true,
	}

	for _, opt := range opts {
		opt(frame)
	}

	frame.Header.OCFFlag = len(frame.OCF) > 0

	totalLen := frame.computeTotalLength()
	if totalLen > 65536 {
		return nil, ErrDataTooLarge
	}
	frame.Header.FrameLength = uint16(totalLen - 1)

	if frame.HasFECF {
		if err := frame.computeFECF(); err != nil {
			return nil, err
		}
	}

	return frame, nil
}

// MaxTruncatedFrameLen is the largest total truncated frame length in
// octets (annex D1.3.2 note 3: the complete frame must fit within one
// LDPC(512, 256) codeword; annex D1.4.2.4 caps the TFDZ at 27 octets).
const MaxTruncatedFrameLen = 32

// NewTruncatedFrame creates a truncated USLP Transfer Frame (annex D):
// a 4-octet primary header, a 1-octet TFDF header with construction rule
// '111' (No Segmentation), and the TFDZ. Truncated frames carry no insert
// zone, OCF, or FECF, and are allowed only on variable-length virtual
// channels. The TFDZ must hold at least one octet (annex D1.3.2 note 2:
// minimum total length 6 octets) and at most 27 (annex D1.4.2.4: maximum
// total length 32 octets).
func NewTruncatedFrame(scid uint16, vcid, mapid uint8, data []byte, opts ...FrameOption) (*TransferFrame, error) {
	frame := &TransferFrame{
		Header: PrimaryHeader{
			TFVN:     TFVN,
			SCID:     scid,
			VCID:     vcid & 0x3F,
			MAPID:    mapid & 0x0F,
			EndOfFPH: true,
		},
		DataFieldHeader: DataFieldHeader{
			ConstructionRule: RuleNoSegmentation,
			UPID:             UPIDMissionSpecific1,
		},
		DataField: data,
		HasFECF:   false,
	}
	for _, opt := range opts {
		opt(frame)
	}
	if len(frame.InsertZone) > 0 || len(frame.OCF) > 0 || frame.HasFECF {
		return nil, ErrTruncatedFrameFields
	}
	if frame.DataFieldHeader.HasPointer() {
		// Annex D1.4.1.2: the pointer field is not present in truncated
		// TFDF headers, so pointer-carrying rules are not allowed.
		return nil, ErrTruncatedFrameFields
	}
	if len(frame.DataField) == 0 {
		// Annex D1.3.2 note 2: minimum 6 octets, the truncated headers
		// plus a single TFDZ octet.
		return nil, ErrTruncatedFrameTooShort
	}
	if TruncatedPrimaryHeaderSize+dfhSizeNoPointer+len(frame.DataField) > MaxTruncatedFrameLen {
		// Annex D1.3.2 note 3 / D1.4.2.4: at most 32 octets in total.
		return nil, ErrTruncatedFrameTooLong
	}
	return frame, nil
}

// computeTotalLength returns the total frame length in bytes.
func (f *TransferFrame) computeTotalLength() int {
	total := f.Header.Size()
	total += len(f.InsertZone)
	total += f.DataFieldHeader.Size()
	total += len(f.DataField)
	if len(f.OCF) > 0 {
		total += OCFSize
	}
	if f.HasFECF {
		total += FECSize16
	}
	return total
}

// computeFECF computes the Frame Error Control Field (clause 4.1.6.2.3: the
// 16-bit coding procedure of annex B).
func (f *TransferFrame) computeFECF() error {
	encoded, err := f.encodeWithoutFECF()
	if err != nil {
		return err
	}
	f.FECF = make([]byte, FECSize16)
	binary.BigEndian.PutUint16(f.FECF, crc.ComputeCRC16(encoded))
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
	if f.Header.EndOfFPH && (len(f.InsertZone) > 0 || len(f.OCF) > 0 || f.HasFECF) {
		return nil, ErrTruncatedFrameFields
	}
	f.Header.OCFFlag = len(f.OCF) > 0

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
		if len(f.OCF) != OCFSize {
			return nil, ErrInvalidOCFLength
		}
		buf = append(buf, f.OCF...)
	}

	return buf, nil
}

// Encode converts the USLP Transfer Frame to a byte slice.
//
// The frame length field and the Frame Error Control Field are refreshed
// from the frame's current contents on every call, so changes made after
// construction are always covered. Use EncodeWithoutFECF to build a frame
// with a deliberately invalid FECF.
func (f *TransferFrame) Encode() ([]byte, error) {
	if !f.Header.EndOfFPH {
		totalLen := f.computeTotalLength()
		if totalLen > 65536 {
			return nil, ErrDataTooLarge
		}
		f.Header.FrameLength = uint16(totalLen - 1)
	}

	buf, err := f.encodeWithoutFECF()
	if err != nil {
		return nil, err
	}
	if !f.HasFECF {
		return buf, nil
	}
	if err := f.computeFECF(); err != nil {
		return nil, err
	}
	return append(buf, f.FECF...), nil
}

// DecodeTransferFrame parses a byte slice into a USLP Transfer Frame.
//
// fecSize is the managed FECF presence for the physical channel: 0
// (absent) or FECSize16 (clause 4.1.6.2.2: the FECF, when present, is the last
// 16 bits of the frame). insertZoneLen is the managed insert zone length
// (0 if none). OCF presence is signaled in-band by the OCF Flag.
// Truncated frames (EndOfFPH set) carry no insert zone, OCF, or FECF,
// regardless of the managed parameters.
func DecodeTransferFrame(data []byte, fecSize int, insertZoneLen int) (*TransferFrame, error) {
	if fecSize != 0 && fecSize != FECSize16 {
		return nil, ErrInvalidFECSize
	}

	var header PrimaryHeader
	if err := header.Decode(data); err != nil {
		return nil, err
	}

	if header.EndOfFPH {
		return decodeTruncated(data, header)
	}

	// Clause 4.1.2.7: the frame length field counts every octet of the frame.
	// Cross-check it against the buffer before trusting any offset.
	if int(header.FrameLength)+1 != len(data) {
		return nil, ErrFrameLengthMismatch
	}

	// Verify whole-frame integrity before interpreting the contents.
	end := len(data)
	var fecf []byte
	if fecSize > 0 {
		fecStart := end - fecSize
		if fecStart < header.Size() {
			return nil, ErrDataTooShort
		}
		if binary.BigEndian.Uint16(data[fecStart:end]) != crc.ComputeCRC16(data[:fecStart]) {
			return nil, ErrCRCMismatch
		}
		fecf = make([]byte, fecSize)
		copy(fecf, data[fecStart:end])
		end = fecStart
	}

	var ocf []byte
	if header.OCFFlag {
		ocfStart := end - OCFSize
		if ocfStart < header.Size() {
			return nil, ErrDataTooShort
		}
		ocf = make([]byte, OCFSize)
		copy(ocf, data[ocfStart:end])
		end = ocfStart
	}

	pos := header.Size()
	var insertZone []byte
	if insertZoneLen > 0 {
		if pos+insertZoneLen > end {
			return nil, ErrDataTooShort
		}
		insertZone = make([]byte, insertZoneLen)
		copy(insertZone, data[pos:pos+insertZoneLen])
		pos += insertZoneLen
	}

	var dfh DataFieldHeader
	if err := dfh.Decode(data[pos:end]); err != nil {
		return nil, err
	}
	pos += dfh.Size()
	if pos > end {
		return nil, ErrDataTooShort
	}

	dataField := make([]byte, end-pos)
	copy(dataField, data[pos:end])

	return &TransferFrame{
		Header:          header,
		InsertZone:      insertZone,
		DataFieldHeader: dfh,
		DataField:       dataField,
		OCF:             ocf,
		FECF:            fecf,
		HasFECF:         fecSize > 0,
	}, nil
}

// decodeTruncated parses the remainder of a truncated Transfer Frame.
func decodeTruncated(data []byte, header PrimaryHeader) (*TransferFrame, error) {
	pos := TruncatedPrimaryHeaderSize
	var dfh DataFieldHeader
	if err := dfh.Decode(data[pos:]); err != nil {
		return nil, err
	}
	if dfh.HasPointer() {
		// Annex D1.4.1.2: truncated TFDF headers never carry the pointer.
		return nil, ErrTruncatedFrameFields
	}
	pos += dfh.Size()

	dataField := make([]byte, len(data)-pos)
	copy(dataField, data[pos:])

	return &TransferFrame{
		Header:          header,
		DataFieldHeader: dfh,
		DataField:       dataField,
	}, nil
}

// IsIdleFrame reports whether the frame is an Only Idle Data frame.
// Per CCSDS 732.1-B-3 clause 4.1.4.1.6, OID frames use VCID 63.
func IsIdleFrame(frame *TransferFrame) bool {
	return frame.Header.VCID == OIDVCID
}

// DefaultIdleFill is the idle fill byte used when ChannelConfig.IdlePattern
// is empty. It fills the unused tail of fixed-length TFDZs behind the Last
// Valid Octet Pointer and the body of Encapsulation Idle Packets; that
// idle pattern is project-specified (clause 4.1.4.3 note 1; a random pattern is
// preferred) and can be overridden via ChannelConfig.IdlePattern. The TFDZ
// of an OID frame is NOT filled with this pattern: Clause 4.1.4.1.10 mandates
// the PN sequence generated by OIDSequence.
const DefaultIdleFill byte = 0x55

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
// filling remaining bytes with the channel's idle pattern.
func padDataField(data []byte, capacity int, pattern []byte) []byte {
	padded := make([]byte, capacity)
	copy(padded, data)
	fillIdle(padded[len(data):], pattern)
	return padded
}

// OIDSequence generates the mandatory Pseudo Noise (PN) sequence that
// fills the TFDZ of OID Transfer Frames (CCSDS 732.1-B-3 clause 4.1.4.1.10,
// annex H): a 32-cell Fibonacci-form Linear Feedback Shift Register with
// polynomial D0 + D1 + D2 + D22 + D32, initialized to the 'all ones'
// state at device start-up and never restarted for subsequent frames.
// The first octets of the stream are FF FF FF FF 6D B6 D8 61 ...
// (annex H). It is safe for concurrent use.
//
// TM mandates the same generator (CCSDS 132.0-B-3 clause 4.1.4.6.2), so the
// implementation is shared with pkg/tmdl rather than copied.
type OIDSequence = pn.OIDSequence

// NewOIDSequence returns a PN generator in the 'all ones' start-up state.
// Keep one generator per physical channel for the life of the device; the
// sequence must not be restarted across OID frames (clause 4.1.4.1.10.1).
func NewOIDSequence() *OIDSequence { return pn.NewOIDSequence() }

// NewIdleFrame creates an OID (Only Idle Data) Transfer Frame per CCSDS
// 732.1-B-3 clause 4.1.4.1: VCID 63, MAP ID 0, construction rule '001' with the
// Last Valid Octet Pointer set to the last TFDZ octet, UPID 'Idle Data',
// and a TFDZ filled from the mandatory PN sequence (clause 4.1.4.1.10). OID
// frames exist only on fixed-length physical channels.
//
// fill is the channel's persistent PN generator; passing nil starts a
// fresh sequence for this frame only, which is fine for a single frame
// but violates the never-restarted rule across frames, long-lived
// senders must keep one OIDSequence (MasterChannel does). ocf supplies
// the 4-octet Operational Control Field when config.HasOCF is set; with
// HasOCF and no OCF the frame is refused rather than fabricating an
// all-zero report.
func NewIdleFrame(scid uint16, config ChannelConfig, fill *OIDSequence, ocf []byte) (*TransferFrame, error) {
	dfh := DataFieldHeader{ConstructionRule: RuleStartOfSDU}
	capacity := config.DataFieldCapacity(dfh.Size())
	if capacity <= 0 {
		return nil, ErrDataFieldTooSmall
	}
	if fill == nil {
		fill = NewOIDSequence()
	}
	idleData := make([]byte, capacity)
	fill.Fill(idleData)

	opts := []FrameOption{
		WithConstructionRule(RuleStartOfSDU),
		WithUPID(UPIDIdle),
		WithPointer(uint16(capacity - 1)),
	}
	if config.HasOCF {
		if ocf == nil {
			return nil, ErrNoOCFSupplier
		}
		if len(ocf) != OCFSize {
			return nil, ErrInvalidOCFLength
		}
		opts = append(opts, WithOCF(ocf))
	}
	if config.InsertZoneLen > 0 {
		opts = append(opts, WithInsertZone(make([]byte, config.InsertZoneLen)))
	}
	if !config.HasFECF {
		opts = append(opts, WithoutFECF())
	}
	if config.VCFCountLen > 0 {
		opts = append(opts, WithVCFCount(config.VCFCountLen, 0))
	}
	return NewTransferFrame(scid, OIDVCID, 0, idleData, opts...)
}

// recomputeFECF re-encodes the frame and updates the FECF.
func recomputeFECF(frame *TransferFrame) error {
	if !frame.HasFECF {
		return nil
	}
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
	if len(f.FECF) > 0 {
		lines = append(lines, "FECF: "+hex.EncodeToString(f.FECF))
	}
	lines = append(lines, "Idle: "+strconv.FormatBool(IsIdleFrame(f)))
	return strings.Join(lines, "\n")
}
