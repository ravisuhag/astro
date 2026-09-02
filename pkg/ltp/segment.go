// Package ltp implements the Licklider Transmission Protocol
// per RFC 5326, profiled for space links by CCSDS 734.1-B-1.
//
// LTP moves blocks of data over links where a round trip takes minutes or
// hours. TCP's handshakes are useless at those delays, so LTP takes a
// different approach: the sender pushes the whole block, then asks "what did
// you miss?" at checkpoints, and the receiver answers with reception claims.
//
// A block has two parts. The red part is delivered reliably: gaps are
// retransmitted until the receiver confirms it has everything. The green part
// is best effort, sent once and never chased. A block can be all red, all
// green, or red followed by green.
//
//	block:  [ ────── red part ────── │ ─── green part ─── ]
//	          retransmitted on loss     sent once
//
// Nearly every field is a Self-Delimiting Numeric Value, so this package
// builds on pkg/sdnv.
//
// The session machines here own no goroutines and no clock, the same contract
// as pkg/cop's FOP-1. LTP's timers (checkpoint retransmission, report
// retransmission, cancel retransmission) are the caller's to run, because on
// a light-minutes link only the mission knows what a sensible timeout is.
package ltp

import (
	"fmt"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

// Version is the LTP segment version number, per RFC 5326 clause 3.1. Only 0 is
// defined.
const Version = 0

// SegmentType is the 4-bit type code of RFC 5326 clause 3.1.2, built from the CTRL,
// EXC, Flag 1 and Flag 0 bits.
type SegmentType uint8

// Segment type codes, per table clause 3.1.2.
const (
	// TypeRedData is red data that is neither checkpoint, end of red part,
	// nor end of block.
	TypeRedData SegmentType = 0
	// TypeRedDataCheckpoint is red data that is a checkpoint only.
	TypeRedDataCheckpoint SegmentType = 1
	// TypeRedDataCheckpointEORP is red data that is a checkpoint and the end
	// of the red part, but not the end of the block.
	TypeRedDataCheckpointEORP SegmentType = 2
	// TypeRedDataCheckpointEORPEOB is red data that is a checkpoint, the end
	// of the red part, and the end of the block.
	TypeRedDataCheckpointEORPEOB SegmentType = 3

	// TypeGreenData is green data that is not the end of the block.
	TypeGreenData SegmentType = 4
	// TypeGreenDataEOB is green data that ends the block.
	TypeGreenDataEOB SegmentType = 7

	// TypeReport carries reception claims.
	TypeReport SegmentType = 8
	// TypeReportAck acknowledges a report segment.
	TypeReportAck SegmentType = 9

	// TypeCancelFromSender cancels a session, from the block sender.
	TypeCancelFromSender SegmentType = 12
	// TypeCancelAckToSender acknowledges a cancel from the block sender.
	TypeCancelAckToSender SegmentType = 13
	// TypeCancelFromReceiver cancels a session, from the block receiver.
	TypeCancelFromReceiver SegmentType = 14
	// TypeCancelAckToReceiver acknowledges a cancel from the block receiver.
	TypeCancelAckToReceiver SegmentType = 15
)

// String names the segment type.
func (t SegmentType) String() string {
	switch t {
	case TypeRedData:
		return "red data"
	case TypeRedDataCheckpoint:
		return "red data, checkpoint"
	case TypeRedDataCheckpointEORP:
		return "red data, checkpoint, end of red part"
	case TypeRedDataCheckpointEORPEOB:
		return "red data, checkpoint, end of red part, end of block"
	case TypeGreenData:
		return "green data"
	case TypeGreenDataEOB:
		return "green data, end of block"
	case TypeReport:
		return "report"
	case TypeReportAck:
		return "report acknowledgment"
	case TypeCancelFromSender:
		return "cancel from block sender"
	case TypeCancelAckToSender:
		return "cancel acknowledgment to block sender"
	case TypeCancelFromReceiver:
		return "cancel from block receiver"
	case TypeCancelAckToReceiver:
		return "cancel acknowledgment to block receiver"
	default:
		return fmt.Sprintf("undefined(%d)", uint8(t))
	}
}

// Defined reports whether the type code is one RFC 5326 clause 3.1.2 assigns a
// meaning. Codes 5, 6, 10 and 11 are listed as undefined.
func (t SegmentType) Defined() bool {
	switch t {
	case 5, 6, 10, 11:
		return false
	default:
		return t <= 15
	}
}

// IsData reports whether this is a data segment, red or green.
// Clause 3.1.1: the CTRL flag, bit 3, is clear for data.
func (t SegmentType) IsData() bool { return t&0x08 == 0 }

// IsRedData reports whether this carries red-part data. Red data has both the
// CTRL flag (bit 3) and the EXC flag (bit 2) clear.
func (t SegmentType) IsRedData() bool { return t&0x0C == 0 }

// IsGreenData reports whether this carries green-part data: CTRL clear, EXC
// set.
func (t SegmentType) IsGreenData() bool { return t&0x0C == 0x04 }

// IsCheckpoint reports whether this data segment is a checkpoint. Clause 3.1.1: any
// red-part data segment with either low flag set is a checkpoint.
func (t SegmentType) IsCheckpoint() bool {
	return t.IsRedData() && t&0x03 != 0
}

// IsEORP reports whether this segment ends the red part. Clause 3.1.3: red data
// with Flag 1 set.
func (t SegmentType) IsEORP() bool {
	return t.IsRedData() && t&0x02 != 0
}

// IsEOB reports whether this segment ends the block. Clause 3.1.3: a data segment
// with both low flags set.
func (t SegmentType) IsEOB() bool {
	return t.IsData() && t&0x03 == 0x03
}

// IsCancel reports whether this is a cancel segment, from either end.
func (t SegmentType) IsCancel() bool {
	return t == TypeCancelFromSender || t == TypeCancelFromReceiver
}

// IsCancelAck reports whether this acknowledges a cancel.
func (t SegmentType) IsCancelAck() bool {
	return t == TypeCancelAckToSender || t == TypeCancelAckToReceiver
}

// SessionID names a transmission session, per RFC 5326 clause 3.1.
//
// The engine ID identifies the sender, and the session number distinguishes
// this session from that engine's others. Together they are unique.
type SessionID struct {
	EngineID      uint64
	SessionNumber uint64
}

// String renders the session ID.
func (s SessionID) String() string {
	return fmt.Sprintf("%d:%d", s.EngineID, s.SessionNumber)
}

// Extension is one header or trailer extension TLV, per clause 3.1.4: a one-octet
// tag, an SDNV length, then the value.
type Extension struct {
	Tag   uint8
	Value []byte
}

// Extension tags from the IANA LTP Extension Tag registry (clause 3.1.4).
const (
	// ExtensionAuth is the LTP authentication extension.
	ExtensionAuth uint8 = 0x00
	// ExtensionCookie is the LTP cookie extension.
	ExtensionCookie uint8 = 0x01
)

// Encode serializes the extension TLV.
func (e Extension) Encode() []byte {
	out := make([]byte, 0, 1+sdnv.MaxEncodedSize+len(e.Value))
	out = append(out, e.Tag)
	out = sdnv.AppendEncode(out, uint64(len(e.Value)))
	return append(out, e.Value...)
}

// decodeExtension reads one extension TLV, returning it and the octets consumed.
func decodeExtension(data []byte) (Extension, int, error) {
	if len(data) < 1 {
		return Extension{}, 0, ErrDataTooShort
	}
	e := Extension{Tag: data[0]}
	offset := 1

	length, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return Extension{}, 0, ErrDataTooShort
	}
	offset += n

	if uint64(len(data)-offset) < length {
		return Extension{}, 0, ErrDataTooShort
	}
	if length > 0 {
		e.Value = make([]byte, length)
		copy(e.Value, data[offset:offset+int(length)])
	}
	return e, offset + int(length), nil
}

// Header is the part every LTP segment shares, per clause 3.1: a control octet
// carrying version and type, the session ID, an extension-counts octet, and
// the header extensions themselves.
type Header struct {
	Type      SegmentType
	SessionID SessionID

	// HeaderExtensions sit between the counts octet and the segment content.
	HeaderExtensions []Extension
	// TrailerExtensions follow the segment content. They are counted in the
	// same octet as the header ones, which is why they live here.
	TrailerExtensions []Extension
}

// Validate checks the header against clause 3.1.
func (h *Header) Validate() error {
	if !h.Type.Defined() {
		return ErrUndefinedSegmentType
	}
	if len(h.HeaderExtensions) > 15 || len(h.TrailerExtensions) > 15 {
		return ErrTooManyExtensions
	}
	return nil
}

// Encode serializes the header: control octet, session ID, extension counts,
// then the header extensions. Trailer extensions are appended by Segment.Encode
// after the content.
func (h *Header) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	var out []byte
	// Control octet: 4-bit version, 4-bit segment type flags.
	out = append(out, Version<<4|byte(h.Type&0x0F))

	out = sdnv.AppendEncode(out, h.SessionID.EngineID)
	out = sdnv.AppendEncode(out, h.SessionID.SessionNumber)

	// Extension counts: high nibble header, low nibble trailer.
	out = append(out, byte(len(h.HeaderExtensions))<<4|byte(len(h.TrailerExtensions)&0x0F))

	for _, e := range h.HeaderExtensions {
		out = append(out, e.Encode()...)
	}
	return out, nil
}

// DecodeHeader parses a segment header from the front of data, returning the
// header, the number of octets consumed, and how many trailer extensions the
// counts octet promised.
func DecodeHeader(data []byte) (*Header, int, int, error) {
	if len(data) < 1 {
		return nil, 0, 0, ErrDataTooShort
	}

	if version := data[0] >> 4; version != Version {
		return nil, 0, 0, ErrInvalidVersion
	}

	h := &Header{Type: SegmentType(data[0] & 0x0F)}
	if !h.Type.Defined() {
		return nil, 0, 0, ErrUndefinedSegmentType
	}
	offset := 1

	engine, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, 0, ErrDataTooShort
	}
	offset += n

	session, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, 0, ErrDataTooShort
	}
	offset += n
	h.SessionID = SessionID{EngineID: engine, SessionNumber: session}

	if len(data) < offset+1 {
		return nil, 0, 0, ErrDataTooShort
	}
	headerCount := int(data[offset] >> 4)
	trailerCount := int(data[offset] & 0x0F)
	offset++

	for i := 0; i < headerCount; i++ {
		e, n, err := decodeExtension(data[offset:])
		if err != nil {
			return nil, 0, 0, err
		}
		h.HeaderExtensions = append(h.HeaderExtensions, e)
		offset += n
	}

	return h, offset, trailerCount, nil
}

// Humanize returns a human-readable summary of the header.
func (h *Header) Humanize() string {
	return fmt.Sprintf("LTP Segment Header\n  Type ....... %s\n  Session .... %s\n  Extensions . %d header, %d trailer",
		h.Type, h.SessionID, len(h.HeaderExtensions), len(h.TrailerExtensions))
}
