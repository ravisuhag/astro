// Package pxdl implements the Proximity-1 Space Data Link Protocol
// per CCSDS 211.0-B-6 (July 2020), Data Link Layer.
//
// Proximity-1 is the short-range link protocol: orbiter to lander, orbiter to
// rover, spacecraft to spacecraft. It is what the Mars relay network runs on.
//
// It differs from the long-haul data link protocols this library also ships —
// TM, TC, AOS, USLP — in ways that follow from the short range. Frames are
// small, at most 2048 octets. The header is five octets with no error control
// field, because the coding layer below handles that. And a single frame type
// carries both user data and the protocol's own supervisory traffic,
// distinguished by one bit.
//
//	U-frame:  header │ user data (packets, segments, or raw)
//	P-frame:  header │ supervisory PDUs (link control words, directives)
//
// This package implements the Version-3 Transfer Frame and its data field
// constructions. The coding and synchronization layer, CCSDS 211.2-B-3, lives
// in pkg/pxsc.
package pxdl

import (
	"encoding/binary"
	"fmt"
)

// Version is the Transfer Frame Version Number for a Version-3 frame: binary
// '10', per CCSDS 211.0-B-6 §3.2.2.2.2.
const Version = 2

// HeaderSize is the width of the Transfer Frame Header in octets (§3.2.1 a).
const HeaderSize = 5

// Frame size bounds, per §3.2.1 and §3.2.2.10.2.
const (
	// MaxDataFieldSize is the largest Transfer Frame Data field.
	MaxDataFieldSize = 2043
	// MinFrameSize is the smallest legal frame: header only.
	MinFrameSize = HeaderSize
	// MaxFrameSize is the largest frame the 11-bit length field can describe.
	MaxFrameSize = 2048
)

// QoS is the Quality of Service Indicator of §3.2.2.3.
type QoS uint8

const (
	// SequenceControlled is the reliable service. COP-P checks the frame
	// sequence number of every frame on it.
	SequenceControlled QoS = 0
	// Expedited bypasses the sequence number check. Supervisory PDUs travel
	// only on this service (§3.2.4.1).
	Expedited QoS = 1
)

// String names the service.
func (q QoS) String() string {
	if q == Expedited {
		return "expedited"
	}
	return "sequence controlled"
}

// PDUType is the PDU Type ID of §3.2.2.4: whether the data field carries user
// data or the protocol's own supervisory traffic.
type PDUType uint8

const (
	// UserData marks a U-frame, carrying user data.
	UserData PDUType = 0
	// SupervisoryData marks a P-frame, carrying SPDUs.
	SupervisoryData PDUType = 1
)

// String names the PDU type.
func (p PDUType) String() string {
	if p == SupervisoryData {
		return "supervisory (P-frame)"
	}
	return "user data (U-frame)"
}

// DFCID is the Data Field Construction ID of §3.2.2.5, saying how a U-frame's
// data field is arranged. Table 3-1 gives the four values.
type DFCID uint8

const (
	// DFCPackets means an integer number of unsegmented packets.
	DFCPackets DFCID = 0
	// DFCSegment means one complete or segmented packet, behind a segment header.
	DFCSegment DFCID = 1
	// DFCReserved is reserved for future CCSDS definition.
	DFCReserved DFCID = 2
	// DFCUserDefined means the content is defined by the mission.
	DFCUserDefined DFCID = 3
)

// String names the construction.
func (d DFCID) String() string {
	switch d {
	case DFCPackets:
		return "packets"
	case DFCSegment:
		return "segment data"
	case DFCReserved:
		return "reserved"
	default:
		return "user defined"
	}
}

// SourceOrDest is the Source-or-Destination Identifier of §3.2.2.9, saying
// whether the SCID names the sender or the receiver.
type SourceOrDest uint8

const (
	// SCIDIsDestination means the SCID field names the destination spacecraft.
	SCIDIsDestination SourceOrDest = 0
	// SCIDIsSource means the SCID field names the source spacecraft.
	SCIDIsSource SourceOrDest = 1
)

// String names the interpretation.
func (s SourceOrDest) String() string {
	if s == SCIDIsSource {
		return "source"
	}
	return "destination"
}

// Header is the Transfer Frame Header of §3.2.2, figure 3-3.
//
// Five octets, ten fields:
//
//	Octet 0:  TFVN(2) | QoS(1) | PDU type(1) | DFC ID(2) | SCID[9:8](2)
//	Octet 1:  SCID[7:0](8)
//	Octet 2:  PCID(1) | Port ID(3) | Src/Dest(1) | Frame Length[10:8](3)
//	Octet 3:  Frame Length[7:0](8)
//	Octet 4:  Frame Sequence Number(8)
type Header struct {
	// QoS selects the sequence-controlled or expedited service.
	QoS QoS
	// PDUType distinguishes a U-frame from a P-frame.
	PDUType PDUType
	// DFCID says how a U-frame's data field is arranged. §3.2.2.5.2 requires
	// zero on a P-frame.
	DFCID DFCID
	// SCID identifies the spacecraft, 10 bits.
	SCID uint16
	// PCID selects one of two physical channels, 1 bit.
	PCID uint8
	// PortID identifies the port, 3 bits.
	PortID uint8
	// SourceOrDest says whether SCID names the source or the destination.
	SourceOrDest SourceOrDest
	// FrameLength is the total frame length in octets. It travels as a count
	// one less than this value (§3.2.2.10.2).
	FrameLength uint16
	// FrameSequenceNumber counts frames per PCID and service (§3.2.2.11).
	FrameSequenceNumber uint8
}

// Validate checks the header against §3.2.2.
func (h *Header) Validate() error {
	if h.SCID > 0x03FF {
		return ErrInvalidSCID
	}
	if h.PCID > 1 {
		return ErrInvalidPCID
	}
	if h.PortID > 0x07 {
		return ErrInvalidPortID
	}
	if h.FrameLength < MinFrameSize || h.FrameLength > MaxFrameSize {
		return ErrInvalidFrameLength
	}
	// §3.2.2.5.2: in a P-frame the DFC ID is not used and is set to '00'.
	if h.PDUType == SupervisoryData && h.DFCID != 0 {
		return ErrInvalidDFCID
	}
	// §3.2.4.1: SPDUs travel only on the Expedited service.
	if h.PDUType == SupervisoryData && h.QoS != Expedited {
		return ErrInvalidQoS
	}
	return nil
}

// Encode serializes the header per figure 3-3.
func (h *Header) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	out := make([]byte, HeaderSize)

	// Octet 0: version(2) | QoS(1) | PDU type(1) | DFC ID(2) | SCID high 2 bits.
	out[0] = Version << 6
	out[0] |= byte(h.QoS&0x01) << 5
	out[0] |= byte(h.PDUType&0x01) << 4
	out[0] |= byte(h.DFCID&0x03) << 2
	out[0] |= byte(h.SCID >> 8 & 0x03)

	// Octet 1: the low 8 bits of the SCID.
	out[1] = byte(h.SCID)

	// §3.2.2.10.2: the field carries one fewer than the total octet count.
	count := h.FrameLength - 1

	// Octet 2: PCID(1) | Port ID(3) | source-or-dest(1) | length high 3 bits.
	out[2] = h.PCID & 0x01 << 7
	out[2] |= h.PortID & 0x07 << 4
	out[2] |= byte(h.SourceOrDest&0x01) << 3
	out[2] |= byte(count >> 8 & 0x07)

	// Octet 3: the low 8 bits of the length count.
	out[3] = byte(count)

	out[4] = h.FrameSequenceNumber
	return out, nil
}

// Decode parses a Transfer Frame Header from the front of data.
func (h *Header) Decode(data []byte) error {
	if len(data) < HeaderSize {
		return ErrDataTooShort
	}

	if version := data[0] >> 6; version != Version {
		return ErrInvalidVersion
	}

	h.QoS = QoS(data[0] >> 5 & 0x01)
	h.PDUType = PDUType(data[0] >> 4 & 0x01)
	h.DFCID = DFCID(data[0] >> 2 & 0x03)
	h.SCID = uint16(data[0]&0x03)<<8 | uint16(data[1])

	h.PCID = data[2] >> 7 & 0x01
	h.PortID = data[2] >> 4 & 0x07
	h.SourceOrDest = SourceOrDest(data[2] >> 3 & 0x01)

	count := uint16(data[2]&0x07)<<8 | uint16(data[3])
	h.FrameLength = count + 1

	h.FrameSequenceNumber = data[4]
	return h.Validate()
}

// Humanize returns a human-readable summary.
func (h *Header) Humanize() string {
	return fmt.Sprintf("Proximity-1 Transfer Frame Header\n"+
		"  Version ....... 3 (binary '10')\n"+
		"  QoS ........... %s\n"+
		"  PDU type ...... %s\n"+
		"  DFC ID ........ %s\n"+
		"  SCID .......... %d (%s)\n"+
		"  PCID .......... %d\n"+
		"  Port ID ....... %d\n"+
		"  Frame length .. %d octets\n"+
		"  Sequence ...... %d",
		h.QoS, h.PDUType, h.DFCID, h.SCID, h.SourceOrDest,
		h.PCID, h.PortID, h.FrameLength, h.FrameSequenceNumber)
}

// TransferFrame is a Version-3 Transfer Frame: a five-octet header and a data
// field of up to 2043 octets (§3.2.1).
//
// There is no frame error control field. Proximity-1 leaves error detection to
// the coding layer below, CCSDS 211.2-B-3.
type TransferFrame struct {
	Header    Header
	DataField []byte
}

// FrameOption configures a frame at construction.
type FrameOption func(*TransferFrame)

// WithQoS selects the quality of service.
func WithQoS(q QoS) FrameOption {
	return func(f *TransferFrame) { f.Header.QoS = q }
}

// WithDFCID sets the data field construction for a U-frame.
func WithDFCID(d DFCID) FrameOption {
	return func(f *TransferFrame) { f.Header.DFCID = d }
}

// WithPCID selects the physical channel.
func WithPCID(pcid uint8) FrameOption {
	return func(f *TransferFrame) { f.Header.PCID = pcid }
}

// WithSourceSCID marks the SCID field as naming the source spacecraft. The
// default is the destination.
func WithSourceSCID() FrameOption {
	return func(f *TransferFrame) { f.Header.SourceOrDest = SCIDIsSource }
}

// WithSequenceNumber sets the frame sequence number.
func WithSequenceNumber(n uint8) FrameOption {
	return func(f *TransferFrame) { f.Header.FrameSequenceNumber = n }
}

// NewTransferFrame builds a U-frame carrying user data.
func NewTransferFrame(scid uint16, portID uint8, data []byte, opts ...FrameOption) (*TransferFrame, error) {
	if len(data) > MaxDataFieldSize {
		return nil, ErrDataTooLarge
	}

	f := &TransferFrame{
		Header: Header{
			QoS:         SequenceControlled,
			PDUType:     UserData,
			DFCID:       DFCPackets,
			SCID:        scid,
			PortID:      portID,
			FrameLength: uint16(HeaderSize + len(data)),
		},
		DataField: data,
	}
	for _, opt := range opts {
		opt(f)
	}

	// The length follows from the payload, whatever the options did.
	f.Header.FrameLength = uint16(HeaderSize + len(data))

	if err := f.Header.Validate(); err != nil {
		return nil, err
	}
	return f, nil
}

// NewSupervisoryFrame builds a P-frame carrying supervisory PDUs.
//
// §3.2.4.1 restricts SPDUs to the Expedited service and §3.2.2.5.2 requires a
// zero DFC ID, so both are set here rather than left to the caller.
func NewSupervisoryFrame(scid uint16, portID uint8, spdus []byte, opts ...FrameOption) (*TransferFrame, error) {
	if len(spdus) > MaxDataFieldSize {
		return nil, ErrDataTooLarge
	}

	f := &TransferFrame{
		Header: Header{
			QoS:         Expedited,
			PDUType:     SupervisoryData,
			DFCID:       0,
			SCID:        scid,
			PortID:      portID,
			FrameLength: uint16(HeaderSize + len(spdus)),
		},
		DataField: spdus,
	}
	for _, opt := range opts {
		opt(f)
	}

	// Restore what the protocol fixes, in case an option changed it.
	f.Header.QoS = Expedited
	f.Header.PDUType = SupervisoryData
	f.Header.DFCID = 0
	f.Header.FrameLength = uint16(HeaderSize + len(spdus))

	if err := f.Header.Validate(); err != nil {
		return nil, err
	}
	return f, nil
}

// IsUserFrame reports whether this is a U-frame.
func (f *TransferFrame) IsUserFrame() bool { return f.Header.PDUType == UserData }

// IsSupervisoryFrame reports whether this is a P-frame.
func (f *TransferFrame) IsSupervisoryFrame() bool { return f.Header.PDUType == SupervisoryData }

// Validate checks the frame.
func (f *TransferFrame) Validate() error {
	if err := f.Header.Validate(); err != nil {
		return err
	}
	if len(f.DataField) > MaxDataFieldSize {
		return ErrDataTooLarge
	}
	if int(f.Header.FrameLength) != HeaderSize+len(f.DataField) {
		return ErrInvalidFrameLength
	}
	return nil
}

// Encode serializes the whole frame.
func (f *TransferFrame) Encode() ([]byte, error) {
	// Keep the length honest even if the caller changed the data field.
	f.Header.FrameLength = uint16(HeaderSize + len(f.DataField))

	if err := f.Validate(); err != nil {
		return nil, err
	}
	header, err := f.Header.Encode()
	if err != nil {
		return nil, err
	}
	return append(header, f.DataField...), nil
}

// DecodeTransferFrame parses a Version-3 Transfer Frame.
func DecodeTransferFrame(data []byte) (*TransferFrame, error) {
	if len(data) < HeaderSize {
		return nil, ErrDataTooShort
	}

	f := &TransferFrame{}
	if err := f.Header.Decode(data); err != nil {
		return nil, err
	}

	// The length count is validated by Header.Validate, so this cannot
	// underflow or name a position before the header.
	total := int(f.Header.FrameLength)
	if len(data) < total {
		return nil, ErrDataTooShort
	}

	f.DataField = make([]byte, total-HeaderSize)
	copy(f.DataField, data[HeaderSize:total])
	return f, nil
}

// Humanize returns a human-readable summary.
func (f *TransferFrame) Humanize() string {
	return f.Header.Humanize() +
		fmt.Sprintf("\n  Data field .... %d octets", len(f.DataField))
}

// appendUint16 is a small helper for the SPDU codecs.
func appendUint16(dst []byte, v uint16) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return append(dst, b[:]...)
}
