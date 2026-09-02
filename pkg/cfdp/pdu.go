// Package cfdp implements the CCSDS File Delivery Protocol
// per CCSDS 727.0-B-5 (July 2020).
//
// CFDP moves files over space links. A transaction sends one file from a
// source entity to a destination entity as a run of Protocol Data Units: a
// Metadata PDU describing the file, a stream of File Data PDUs carrying its
// contents, and an EOF PDU closing it out. In acknowledged mode the receiver
// answers with NAK PDUs naming the gaps it still needs, and the exchange ends
// with a Finished PDU and its ACK.
//
// CFDP PDUs are ordinary payload bytes. They ride inside Space Packets or
// Encapsulation Packets, so this package composes with pkg/spp and pkg/epp
// from the outside and changes neither.
//
// The transaction machines here own no goroutines and no clock. The caller
// pumps them, the same shape as pkg/cop's FOP-1: hand PDUs in, ask what to
// send next, drive timeouts from your own scheduler. That keeps the library
// testable and leaves scheduling policy where it belongs.
package cfdp

import (
	"encoding/binary"
	"fmt"

	"github.com/ravisuhag/astro/pkg/crc"
)

// Version is the PDU version of CCSDS 727.0-B-5 table 5-1: binary '001',
// the second version of the protocol.
const Version = 1

// FixedHeaderSize is the width of the PDU header before the variable-width
// entity IDs and transaction sequence number.
const FixedHeaderSize = 4

// CRCSize is the width of the optional PDU CRC, in octets (clause 4.1.3).
const CRCSize = 2

// MaxIDWidth is the widest entity ID or sequence number the 3-bit length
// fields of table 5-1 can describe: the encoded value is the width less one.
const MaxIDWidth = 8

// Direction indicates which way a PDU travels, per table 5-1. It exists so
// intermediate nodes can forward PDUs without parsing them.
type Direction uint8

const (
	// TowardReceiver marks a PDU heading to the file receiver ('0').
	TowardReceiver Direction = 0
	// TowardSender marks a PDU heading back to the file sender ('1').
	TowardSender Direction = 1
)

// String names the direction.
func (d Direction) String() string {
	if d == TowardSender {
		return "toward file sender"
	}
	return "toward file receiver"
}

// EntityID is a CFDP entity identifier or transaction sequence number: an
// unsigned integer whose octet width travels in the PDU header (clause 5.1.4).
//
// Width carries semantic weight on the wire but none in comparisons: Clause 5.1.7
// note 3 says two IDs of different widths compare by zero-padding the shorter.
type EntityID struct {
	Value uint64
	Width int // octets on the wire, 1 to 8
}

// NewEntityID returns an EntityID just wide enough to hold value.
func NewEntityID(value uint64) EntityID {
	width := 1
	for v := value >> 8; v > 0; v >>= 8 {
		width++
	}
	return EntityID{Value: value, Width: width}
}

// Validate checks the width against the 3-bit length field's range and
// confirms the value fits inside it.
func (e EntityID) Validate() error {
	if e.Width < 1 || e.Width > MaxIDWidth {
		return ErrInvalidEntityIDWidth
	}
	if e.Width < MaxIDWidth {
		if e.Value >= uint64(1)<<(8*e.Width) {
			return ErrEntityIDOverflow
		}
	}
	return nil
}

// Encode writes the value big-endian at its declared width.
func (e EntityID) Encode() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	out := make([]byte, e.Width)
	for i := e.Width - 1; i >= 0; i-- {
		out[i] = byte(e.Value >> (8 * (e.Width - 1 - i)))
	}
	return out, nil
}

// decodeEntityID reads a big-endian unsigned integer of the given width.
func decodeEntityID(data []byte, width int) (EntityID, error) {
	if width < 1 || width > MaxIDWidth {
		return EntityID{}, ErrInvalidEntityIDWidth
	}
	if len(data) < width {
		return EntityID{}, ErrDataTooShort
	}
	var v uint64
	for i := 0; i < width; i++ {
		v = v<<8 | uint64(data[i])
	}
	return EntityID{Value: v, Width: width}, nil
}

// PDUHeader is the fixed PDU header of CCSDS 727.0-B-5 table 5-1.
//
// The first octet packs version, PDU type, direction, transmission mode, the
// CRC flag and the large-file flag. Then comes the 16-bit data field length.
// The fourth octet packs segmentation control, the entity ID width, the
// segment metadata flag and the sequence number width. The three variable
// width fields follow in the order source, sequence number, destination.
type PDUHeader struct {
	// IsFileData distinguishes a File Data PDU ('1') from a File Directive ('0').
	IsFileData bool

	// Direction is which way the PDU travels.
	Direction Direction

	// Acknowledged selects acknowledged mode. Note the wire encoding is
	// inverted: table 5-1 gives '0' for acknowledged and '1' for
	// unacknowledged, so this field is the logical sense, not the bit.
	Acknowledged bool

	// CRCFlag marks a PDU carrying a trailing CRC (clause 4.1).
	CRCFlag bool

	// LargeFile widens every File-Size Sensitive field from 32 to 64 bits
	// (clause 5.1.10).
	LargeFile bool

	// DataLength is the octet length of the PDU data field. When CRCFlag is
	// set this includes the two CRC octets (clause 4.1.3.2).
	DataLength uint16

	// SegmentationControl records whether record boundaries survive
	// segmentation. Always '0' and ignored for File Directive PDUs.
	SegmentationControl bool

	// SegmentMetadataFlag marks a File Data PDU carrying segment metadata.
	// Always '0' and ignored for File Directive PDUs.
	SegmentMetadataFlag bool

	// Source, TransactionSeq and Destination identify the transaction. All
	// three entity IDs share one width; the sequence number has its own.
	Source         EntityID
	TransactionSeq EntityID
	Destination    EntityID
}

// Validate checks the header against table 5-1.
func (h *PDUHeader) Validate() error {
	if err := h.Source.Validate(); err != nil {
		return err
	}
	if err := h.Destination.Validate(); err != nil {
		return err
	}
	if err := h.TransactionSeq.Validate(); err != nil {
		return err
	}
	// Clause 5.1: one entity ID length field covers every entity ID in the header.
	if h.Source.Width != h.Destination.Width {
		return ErrInvalidEntityIDWidth
	}
	return nil
}

// Size returns the encoded width of the header in octets.
func (h *PDUHeader) Size() int {
	return FixedHeaderSize + h.Source.Width + h.TransactionSeq.Width + h.Destination.Width
}

// Encode serializes the PDU header per table 5-1.
func (h *PDUHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	out := make([]byte, FixedHeaderSize, h.Size())

	// Octet 0: version(3) | PDU type(1) | direction(1) | mode(1) | CRC(1) | large file(1).
	out[0] = Version << 5
	if h.IsFileData {
		out[0] |= 1 << 4
	}
	out[0] |= byte(h.Direction&0x01) << 3
	if !h.Acknowledged {
		// '1' means unacknowledged, per table 5-1.
		out[0] |= 1 << 2
	}
	if h.CRCFlag {
		out[0] |= 1 << 1
	}
	if h.LargeFile {
		out[0] |= 1
	}

	// Octets 1-2: PDU data field length.
	binary.BigEndian.PutUint16(out[1:3], h.DataLength)

	// Octet 3: segmentation control(1) | entity ID width-1 (3) |
	//          segment metadata flag(1) | sequence number width-1 (3).
	if h.SegmentationControl {
		out[3] |= 1 << 7
	}
	out[3] |= byte(h.Source.Width-1) << 4
	if h.SegmentMetadataFlag {
		out[3] |= 1 << 3
	}
	out[3] |= byte(h.TransactionSeq.Width - 1)

	for _, id := range []EntityID{h.Source, h.TransactionSeq, h.Destination} {
		b, err := id.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// DecodePDUHeader parses a PDU header from the front of data and returns the
// header along with the number of octets consumed.
func DecodePDUHeader(data []byte) (*PDUHeader, int, error) {
	if len(data) < FixedHeaderSize {
		return nil, 0, ErrDataTooShort
	}

	version := data[0] >> 5
	if version != Version {
		return nil, 0, ErrInvalidVersion
	}

	h := &PDUHeader{
		IsFileData: data[0]&(1<<4) != 0,
		Direction:  Direction(data[0] >> 3 & 0x01),
		// Table 5-1 inverts the sense: the bit is set for unacknowledged.
		Acknowledged:        data[0]&(1<<2) == 0,
		CRCFlag:             data[0]&(1<<1) != 0,
		LargeFile:           data[0]&1 != 0,
		DataLength:          binary.BigEndian.Uint16(data[1:3]),
		SegmentationControl: data[3]&(1<<7) != 0,
		SegmentMetadataFlag: data[3]&(1<<3) != 0,
	}

	idWidth := int(data[3]>>4&0x07) + 1
	seqWidth := int(data[3]&0x07) + 1

	offset := FixedHeaderSize
	var err error
	if h.Source, err = decodeEntityID(data[offset:], idWidth); err != nil {
		return nil, 0, err
	}
	offset += idWidth
	if h.TransactionSeq, err = decodeEntityID(data[offset:], seqWidth); err != nil {
		return nil, 0, err
	}
	offset += seqWidth
	if h.Destination, err = decodeEntityID(data[offset:], idWidth); err != nil {
		return nil, 0, err
	}
	offset += idWidth

	return h, offset, nil
}

// Humanize returns a human-readable summary of the PDU header.
func (h *PDUHeader) Humanize() string {
	kind := "File Directive"
	if h.IsFileData {
		kind = "File Data"
	}
	mode := "unacknowledged"
	if h.Acknowledged {
		mode = "acknowledged"
	}
	return fmt.Sprintf("CFDP PDU Header\n"+
		"  Type ............ %s\n"+
		"  Direction ....... %s\n"+
		"  Mode ............ %s\n"+
		"  CRC present ..... %t\n"+
		"  Large file ...... %t\n"+
		"  Data length ..... %d\n"+
		"  Source .......... %d\n"+
		"  Transaction ..... %d\n"+
		"  Destination ..... %d",
		kind, h.Direction, mode, h.CRCFlag, h.LargeFile, h.DataLength,
		h.Source.Value, h.TransactionSeq.Value, h.Destination.Value)
}

// PDU is one complete Protocol Data Unit: a header plus its data field.
type PDU struct {
	Header *PDUHeader
	// Data is the PDU data field with any trailing CRC already removed.
	Data []byte
}

// Encode serializes the PDU, setting the data field length and appending the
// CRC when the header asks for one.
//
// Per clause 4.1.3.2 the CRC occupies the final octets of the data field, its length
// counts toward the data field length, and it covers everything from the first
// octet of the header to the last octet before the CRC itself.
func (p *PDU) Encode() ([]byte, error) {
	if p.Header == nil {
		return nil, ErrDataTooShort
	}

	dataLen := len(p.Data)
	if p.Header.CRCFlag {
		dataLen += CRCSize
	}
	if dataLen > 0xFFFF {
		return nil, ErrDataLengthMismatch
	}
	p.Header.DataLength = uint16(dataLen)

	header, err := p.Header.Encode()
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(header)+dataLen)
	out = append(out, header...)
	out = append(out, p.Data...)

	if p.Header.CRCFlag {
		// Clause 4.1.3.1: the standard CCSDS Telecommand CRC-16.
		sum := crc.ComputeCRC16(out)
		out = append(out, byte(sum>>8), byte(sum))
	}
	return out, nil
}

// DecodePDU parses one complete PDU, verifying the CRC when the header says
// one is present. A CRC failure returns ErrCRCMismatch and the caller must
// discard the PDU, per clause 4.1.2.
func DecodePDU(data []byte) (*PDU, error) {
	header, consumed, err := DecodePDUHeader(data)
	if err != nil {
		return nil, err
	}

	end := consumed + int(header.DataLength)
	if end > len(data) {
		return nil, ErrDataTooShort
	}
	field := data[consumed:end]

	if header.CRCFlag {
		if len(field) < CRCSize {
			return nil, ErrDataTooShort
		}
		split := len(field) - CRCSize
		received := binary.BigEndian.Uint16(field[split:])
		computed := crc.ComputeCRC16(data[:consumed+split])
		if received != computed {
			return nil, ErrCRCMismatch
		}
		field = field[:split]
	}

	body := make([]byte, len(field))
	copy(body, field)
	return &PDU{Header: header, Data: body}, nil
}

// readFSS reads a File-Size Sensitive integer: 32 bits normally, 64 when the
// large-file flag is set (clause 5.1.10). It returns the value and octets consumed.
func readFSS(data []byte, largeFile bool) (uint64, int, error) {
	width := 4
	if largeFile {
		width = 8
	}
	if len(data) < width {
		return 0, 0, ErrDataTooShort
	}
	if largeFile {
		return binary.BigEndian.Uint64(data[:8]), 8, nil
	}
	return uint64(binary.BigEndian.Uint32(data[:4])), 4, nil
}

// appendFSS writes a File-Size Sensitive integer at the width the large-file
// flag selects.
func appendFSS(dst []byte, v uint64, largeFile bool) []byte {
	if largeFile {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], v)
		return append(dst, b[:]...)
	}
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	return append(dst, b[:]...)
}
