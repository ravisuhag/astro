package tmdl

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/internal/pn"
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
	header[5] = uint8(h.FirstHeaderPtr & 0xFF)         // Bottom 8 bits

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
//
// With the Synchronization Flag set, the Packet Order Flag, Segment Length
// Identifier, and First Header Pointer are undefined by CCSDS 132.0-B-3
// (notes under §4.1.2.7.4 through §4.1.2.7.6), and §3.4.2.3 hands those bits
// to the VCA service user as the VCA Status Fields — so any value passes
// here. With the flag clear, the Packet Order Flag must be '0' and the
// Segment Length Identifier '11'.
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

// MaxSecondaryHeaderSize is the largest Transfer Frame Secondary Header,
// counting the identification octet: CCSDS 132.0-B-3 §4.1.3.2 and
// ECSS-E-ST-50-03C 5.3.1c both cap it at 64 octets.
const MaxSecondaryHeaderSize = 64

// SecondaryHeader represents the Transfer Frame Secondary Header as per CCSDS 132.0-B-3.
type SecondaryHeader struct {
	VersionNumber uint8 // 2 bits (0-1) - Always `00` for Version 1
	// HeaderLength is the field of bits 2-7. CCSDS 132.0-B-3 §4.1.3.2.2.3 and
	// ECSS-E-ST-50-03C 5.3.2.3c define it as the TOTAL secondary header length
	// in octets minus one — the total being this identification octet plus the
	// data field. So for an N-octet data field the value is N, not N-1.
	HeaderLength uint8
	DataField    []byte // Transfer Frame Secondary Header Data
}

// SetDataField installs the data field and derives HeaderLength from it, which
// is the safe way to build a secondary header by hand.
func (sh *SecondaryHeader) SetDataField(data []byte) error {
	if len(data) < 1 || len(data) > MaxSecondaryHeaderSize-1 {
		return ErrInvalidHeaderLength
	}
	sh.DataField = data
	sh.HeaderLength = uint8(len(data))
	return nil
}

// TotalLength returns the encoded size of the secondary header in octets.
func (sh *SecondaryHeader) TotalLength() int { return 1 + len(sh.DataField) }

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

	// §4.1.3.2.2.3: the field is the total length minus one, and the total
	// includes the identification octet just read. So the data field is
	// HeaderLength octets, not HeaderLength+1.
	dataFieldLen := int(sh.HeaderLength)
	if dataFieldLen < 1 {
		return ErrInvalidHeaderLength
	}
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
	// §4.1.3.2.2.3: the field carries the total length minus one, so it must
	// equal the data field length exactly.
	if len(sh.DataField) > 0 && sh.HeaderLength != uint8(len(sh.DataField)) {
		return ErrInvalidHeaderLength
	}
	// ECSS-E-ST-50-03C 5.3.1c caps the whole secondary header at 64 octets.
	if sh.TotalLength() > MaxSecondaryHeaderSize {
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
		// §4.1.3.2.2.3: the field is the total secondary header length minus
		// one, and the total counts the identification octet, so it equals the
		// data field length.
		secondaryHeader.HeaderLength = uint8(len(secondaryHeaderData))
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
//
// The Frame Error Control Field is computed from the frame's current
// contents on every call, so header or data changes made after construction
// are always covered. Use EncodeWithoutFEC to build a frame with a
// deliberately invalid CRC.
func (tf *TMTransferFrame) Encode() ([]byte, error) {
	return tf.EncodeWithConfig(ChannelConfig{HasFEC: true})
}

// EncodeWithConfig converts the frame to bytes, appending the Frame Error
// Control Field only when the channel carries one.
//
// CCSDS 132.0-B-3 §4.1.6 and ECSS-E-ST-50-03C 5.6.1b make the field mandatory
// when the frame is not Reed-Solomon encoded, and optional when it travels
// inside a code block — the code block already protects it. §5.6.1c then
// requires the choice to hold for the whole physical channel, which is why it
// belongs to ChannelConfig rather than to a single frame.
//
// When config.FrameLength is set, the encoded frame must come out exactly
// that long — CCSDS 132.0-B-3 §2.1.3 fixes the frame length per physical
// channel — and any other size returns ErrFrameLengthMismatch.
func (tf *TMTransferFrame) EncodeWithConfig(config ChannelConfig) ([]byte, error) {
	frameData, err := tf.EncodeWithoutFEC()
	if err != nil {
		return nil, err
	}

	if config.HasFEC {
		// Compute the CRC from the frame's current contents, so header fields
		// changed after construction are covered, and refresh the exported field.
		tf.FrameErrorControl = crc.ComputeCRC16(frameData)

		crcBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(crcBytes, tf.FrameErrorControl)
		frameData = append(frameData, crcBytes...)
	}

	if config.FrameLength > 0 && len(frameData) != config.FrameLength {
		return nil, ErrFrameLengthMismatch
	}
	return frameData, nil
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

// FirstHeaderPtr values with a meaning of their own, from CCSDS 132.0-B-3
// §4.1.2.7.6 and ECSS-E-ST-50-03C 5.2.7.6f and g.
//
// The two are not interchangeable. A frame whose data field simply continues a
// packet started earlier says NoPacketStart; a frame that is nothing but fill
// says OnlyIdleData. Telling them apart is the whole reason there are two
// codes: the first still carries payload, the second can be dropped.
const (
	// FHPNoPacketStart means no packet begins in this data field.
	FHPNoPacketStart uint16 = 0x07FF
	// FHPOnlyIdleData marks an OID frame: the data field is entirely idle.
	FHPOnlyIdleData uint16 = 0x07FE
)

// IdleFrameVCID is the fallback virtual channel for idle frames when the
// caller knows no better. CCSDS 132.0-B-3 §4.1.4.6.3 requires the VCID of an
// OID frame to be one of the VCIDs used for transferring packets, so
// MasterChannel picks a registered packet VCID instead; this constant is used
// only when no virtual channel is registered at all, where no conformant
// choice exists.
const IdleFrameVCID uint8 = 7

// OIDSequence generates the mandatory Pseudo Noise (PN) sequence that fills
// the data field of OID Transfer Frames (CCSDS 132.0-B-3 §4.1.4.6.2, annex D):
// a 32-cell Fibonacci-form Linear Feedback Shift Register with polynomial
// D0 + D1 + D2 + D22 + D32, initialized to the 'all ones' state at device
// start-up and never restarted for subsequent frames. The first octets of the
// stream are FF FF FF FF 6D B6 D8 61 45 1F. It is safe for concurrent use.
//
// USLP mandates the same generator (CCSDS 732.1-B-3 §4.1.4.1.10), so the
// implementation is shared with pkg/usdl rather than copied.
type OIDSequence = pn.OIDSequence

// NewOIDSequence returns a PN generator in the 'all ones' start-up state.
// Keep one generator per channel for the life of the device; §4.1.4.6.2.1
// forbids restarting the sequence across OID frames.
func NewOIDSequence() *OIDSequence { return pn.NewOIDSequence() }

// NewIdleFrame creates an idle (OID) TM Transfer Frame: a PN-filled data
// field with the First Header Pointer set to FHPOnlyIdleData, per CCSDS
// 132.0-B-3 §4.1.2.7.6.5 and §4.1.4.6.
//
// The frame's MC and VC counts are zero and its PN sequence starts fresh. Use
// NewIdleFrameWithCounter so idle frames continue the master channel sequence
// and draw from the channel's persistent PN generator.
func NewIdleFrame(scid uint16, vcid uint8, config ChannelConfig) (*TMTransferFrame, error) {
	return NewIdleFrameWithCounter(scid, vcid, config, nil, nil)
}

// NewIdleFrameWithCounter creates an idle (OID) TM Transfer Frame, stamps its
// MC and VC frame counts from the given counter, and fills its data field
// from the given PN generator.
//
// Pass the same FrameCounter the channel's services use: CCSDS 132.0-B-3
// §4.1.2.5 counts every frame of the master channel, idle frames included, so
// an unstamped idle frame breaks the MC sequence at any conformant receiver.
// A nil counter leaves both counts zero.
//
// Pass the channel's persistent OIDSequence: §4.1.4.6.2 mandates the PN fill
// and forbids restarting the generator between frames. A nil fill starts a
// fresh sequence for this frame only, which is fine for a single frame but
// repeats the same octets on every frame of a long-lived sender —
// MasterChannel keeps one generator for exactly this reason.
//
// When the channel carries a secondary header (config.FSHDataLength > 0) the
// idle frame includes a zero-filled one: §4.1.2.7.2.3 keeps the Secondary
// Header Flag static across the channel, and the OID notes under §4.1.4.6
// expect the header to stay usable on idle frames. MasterChannel overwrites
// it from the MC_FSH supplier when one is installed. The same applies to the
// Operational Control Field under config.HasOCF.
func NewIdleFrameWithCounter(scid uint16, vcid uint8, config ChannelConfig, counter *FrameCounter, fill *OIDSequence) (*TMTransferFrame, error) {
	capacity := config.DataFieldCapacity(config.FSHDataLength)
	if capacity <= 0 {
		return nil, ErrDataFieldTooSmall
	}
	if fill == nil {
		fill = NewOIDSequence()
	}
	idleData := make([]byte, capacity)
	fill.Fill(idleData)
	var fsh []byte
	if config.FSHDataLength > 0 {
		fsh = make([]byte, config.FSHDataLength)
	}
	var ocf []byte
	if config.HasOCF {
		ocf = make([]byte, 4)
	}
	frame, err := NewTMTransferFrame(scid, vcid, idleData, fsh, ocf)
	if err != nil {
		return nil, err
	}
	frame.Header.FirstHeaderPtr = FHPOnlyIdleData
	return frame, stampFrame(frame, counter, vcid)
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

// IsIdleFrame reports whether the frame is an OID frame: a data field holding
// nothing but idle data, per ECSS-E-ST-50-03C 5.2.7.6g.
//
// FHPNoPacketStart is deliberately not accepted here. A frame carrying the
// continuation of a packet started in an earlier frame also has no packet
// header in it, and discarding that as idle would lose real payload.
func IsIdleFrame(frame *TMTransferFrame) bool {
	return !frame.Header.SyncFlag && frame.Header.FirstHeaderPtr == FHPOnlyIdleData
}

// DecodeTMTransferFrame parses a byte slice into a TM Transfer Frame, treating
// the last two octets as a Frame Error Control Field and verifying them.
//
// Use DecodeTMTransferFrameWithConfig for a channel that carries no such
// field, which §5.6.1b permits under Reed-Solomon coding.
func DecodeTMTransferFrame(data []byte) (*TMTransferFrame, error) {
	return DecodeTMTransferFrameWithConfig(data, ChannelConfig{HasFEC: true})
}

// DecodeTMTransferFrameWithConfig parses a frame, verifying the Frame Error
// Control Field only when the channel carries one.
//
// When config.FrameLength is set, the input must be exactly that long —
// frames on a physical channel are fixed-length per CCSDS 132.0-B-3 §2.1.3 —
// and any other size returns ErrFrameLengthMismatch.
func DecodeTMTransferFrameWithConfig(data []byte, config ChannelConfig) (*TMTransferFrame, error) {
	// A frame needs its six-octet primary header, and two more for the error
	// control field when the channel carries one.
	minimum := 6
	if config.HasFEC {
		minimum = 8
	}
	if len(data) < minimum {
		return nil, ErrDataTooShort
	}
	if config.FrameLength > 0 && len(data) != config.FrameLength {
		return nil, ErrFrameLengthMismatch
	}

	// Decode Primary Header
	var header PrimaryHeader
	if err := header.Decode(data[:6]); err != nil {
		return nil, err
	}

	dataEnd := len(data)
	var receivedCRC uint16
	if config.HasFEC {
		// §5.6.3: verify the field over everything preceding it.
		dataEnd = len(data) - 2
		receivedCRC = binary.BigEndian.Uint16(data[dataEnd:])
		if computed := crc.ComputeCRC16(data[:dataEnd]); receivedCRC != computed {
			return nil, ErrCRCMismatch
		}
	}

	// Extract Data Field
	primaryHeaderLength := 6
	dataStart := primaryHeaderLength
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
