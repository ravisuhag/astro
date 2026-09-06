package ltp

import (
	"fmt"
	"math"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

// DataSegment is the content of a data segment, per RFC 5326 clause 3.2.1.
//
// Checkpoint segments carry two extra serial numbers. Non-checkpoint segments
// must not: the spec is explicit that they "MUST continue on directly with the
// client service data".
type DataSegment struct {
	// ClientServiceID names the upper-level service to deliver to. It works
	// like a TCP port number.
	ClientServiceID uint64
	// Offset is where this data belongs in the block, in octets from the start.
	Offset uint64
	// Data is the client service data itself. Its length travels as an SDNV.
	Data []byte

	// CheckpointSerial identifies this checkpoint among the sender's. Present
	// only on checkpoint segments, and never zero.
	CheckpointSerial uint64
	// ReportSerial is the serial of the report that prompted this checkpoint,
	// or zero when the checkpoint was not prompted by one.
	ReportSerial uint64
}

// End returns the block offset just past this segment's last octet.
func (d *DataSegment) End() uint64 { return d.Offset + uint64(len(d.Data)) }

// Encode serializes the data segment content. isCheckpoint must match the
// segment type in the header, since the wire format is not self-describing.
func (d *DataSegment) Encode(isCheckpoint bool) ([]byte, error) {
	if isCheckpoint && d.CheckpointSerial == 0 {
		// Clause 3.2.1: "The checkpoint serial number MUST NOT be zero."
		return nil, ErrInvalidSerialNumber
	}

	var out []byte
	out = sdnv.AppendEncode(out, d.ClientServiceID)
	out = sdnv.AppendEncode(out, d.Offset)
	out = sdnv.AppendEncode(out, uint64(len(d.Data)))

	if isCheckpoint {
		out = sdnv.AppendEncode(out, d.CheckpointSerial)
		out = sdnv.AppendEncode(out, d.ReportSerial)
	}
	return append(out, d.Data...), nil
}

// DecodeDataSegment parses data segment content, returning the segment and the
// octets consumed.
func DecodeDataSegment(data []byte, isCheckpoint bool) (*DataSegment, int, error) {
	d := &DataSegment{}
	offset := 0

	fields, n, err := sdnv.DecodeN(data, 3)
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	d.ClientServiceID, d.Offset = fields[0], fields[1]
	length := fields[2]
	offset += n

	if isCheckpoint {
		serials, n, err := sdnv.DecodeN(data[offset:], 2)
		if err != nil {
			return nil, 0, ErrDataTooShort
		}
		d.CheckpointSerial, d.ReportSerial = serials[0], serials[1]
		offset += n

		if d.CheckpointSerial == 0 {
			return nil, 0, ErrInvalidSerialNumber
		}
	}

	if uint64(len(data)-offset) < length {
		return nil, 0, ErrDataTooShort
	}
	if length > 0 {
		d.Data = make([]byte, length)
		copy(d.Data, data[offset:offset+int(length)])
	}
	return d, offset + int(length), nil
}

// Humanize returns a human-readable summary.
func (d *DataSegment) Humanize() string {
	return fmt.Sprintf("LTP Data Segment\n  Client service .. %d\n  Offset .......... %d\n  Length .......... %d",
		d.ClientServiceID, d.Offset, len(d.Data))
}

// ReceptionClaim is one run of successfully received data, per clause 3.2.2.
//
// The offset is measured from the report's lower bound, NOT from the start of
// the block. Add the lower bound to get a block offset.
type ReceptionClaim struct {
	Offset uint64
	Length uint64
}

// ReportSegment is the content of a report segment, per clause 3.2.2. It tells the
// sender which parts of the block arrived.
type ReportSegment struct {
	// ReportSerial identifies this report among the receiver's. Never zero.
	ReportSerial uint64
	// CheckpointSerial is the checkpoint that prompted this report, or zero
	// when the report is asynchronous.
	CheckpointSerial uint64
	// UpperBound is the size of the block prefix the claims pertain to.
	UpperBound uint64
	// LowerBound is the size of the interior prefix the claims do NOT pertain
	// to. Claim offsets are relative to it.
	LowerBound uint64
	// Claims are the received runs, in ascending offset order.
	Claims []ReceptionClaim
}

// Validate checks the report against the rules of clause 3.2.2.
func (r *ReportSegment) Validate() error {
	if r.ReportSerial == 0 {
		// Clause 3.2.2: "The report serial number MUST NOT be zero."
		return ErrInvalidSerialNumber
	}
	if r.UpperBound < r.LowerBound {
		return ErrInvalidBounds
	}
	// Clause 3.2.2: a report carries at least one reception claim.
	if len(r.Claims) == 0 {
		return ErrInvalidClaim
	}
	span := r.UpperBound - r.LowerBound
	var next uint64 // earliest offset the next claim may start at
	for i, c := range r.Claims {
		// Clause 3.2.2: a claim's length is never less than 1, and never more than
		// the gap between the bounds.
		if c.Length < 1 || c.Length > span {
			return ErrInvalidClaim
		}
		if c.Offset > span-c.Length {
			return ErrInvalidClaim
		}
		// Claims come in ascending offset order and never overlap. Two
		// adjacent claims would describe one contiguous run, so each must
		// start strictly past the end of the one before it.
		if i > 0 && c.Offset < next {
			return ErrInvalidClaim
		}
		next = c.Offset + c.Length
	}
	return nil
}

// Encode serializes the report segment content.
func (r *ReportSegment) Encode() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}

	var out []byte
	out = sdnv.AppendEncode(out, r.ReportSerial)
	out = sdnv.AppendEncode(out, r.CheckpointSerial)
	out = sdnv.AppendEncode(out, r.UpperBound)
	out = sdnv.AppendEncode(out, r.LowerBound)
	out = sdnv.AppendEncode(out, uint64(len(r.Claims)))

	for _, c := range r.Claims {
		out = sdnv.AppendEncode(out, c.Offset)
		out = sdnv.AppendEncode(out, c.Length)
	}
	return out, nil
}

// DecodeReportSegment parses report segment content.
func DecodeReportSegment(data []byte) (*ReportSegment, int, error) {
	fields, offset, err := sdnv.DecodeN(data, 5)
	if err != nil {
		return nil, 0, ErrDataTooShort
	}

	r := &ReportSegment{
		ReportSerial:     fields[0],
		CheckpointSerial: fields[1],
		UpperBound:       fields[2],
		LowerBound:       fields[3],
	}
	count := fields[4]

	// A claim needs at least two octets, so refuse a count the remaining
	// bytes cannot possibly hold before allocating for it.
	if count > uint64(len(data)-offset)/2 {
		return nil, 0, ErrDataTooShort
	}

	for i := uint64(0); i < count; i++ {
		pair, n, err := sdnv.DecodeN(data[offset:], 2)
		if err != nil {
			return nil, 0, ErrDataTooShort
		}
		r.Claims = append(r.Claims, ReceptionClaim{Offset: pair[0], Length: pair[1]})
		offset += n
	}

	if err := r.Validate(); err != nil {
		return nil, 0, err
	}
	return r, offset, nil
}

// ClaimedRanges returns the claims as absolute block offsets, having added the
// lower bound that clause 3.2.2 measures them from.
//
// LowerBound and a claim's Offset are both wire-chosen SDNVs that can each
// reach 2^64, so their sum is saturated at math.MaxUint64 rather than left to
// wrap. A caller checking the result against a known bound must see an
// out-of-range value, not a small one a wraparound slipped past it.
func (r *ReportSegment) ClaimedRanges() []ReceptionClaim {
	out := make([]ReceptionClaim, 0, len(r.Claims))
	for _, c := range r.Claims {
		offset := r.LowerBound + c.Offset
		if offset < r.LowerBound {
			offset = math.MaxUint64
		}
		out = append(out, ReceptionClaim{Offset: offset, Length: c.Length})
	}
	return out
}

// Humanize returns a human-readable summary.
func (r *ReportSegment) Humanize() string {
	return fmt.Sprintf("LTP Report Segment\n  Report serial ..... %d\n  Checkpoint serial . %d\n  Scope ............. %d to %d\n  Claims ............ %d",
		r.ReportSerial, r.CheckpointSerial, r.LowerBound, r.UpperBound, len(r.Claims))
}

// ReportAckSegment is the content of a report-acknowledgment segment, per
// Clause 3.2.3: just the serial number of the report being acknowledged.
type ReportAckSegment struct {
	ReportSerial uint64
}

// Encode serializes the report-ack content.
func (r *ReportAckSegment) Encode() ([]byte, error) {
	if r.ReportSerial == 0 {
		return nil, ErrInvalidSerialNumber
	}
	return sdnv.Encode(r.ReportSerial), nil
}

// DecodeReportAckSegment parses report-ack content.
func DecodeReportAckSegment(data []byte) (*ReportAckSegment, int, error) {
	serial, n, err := sdnv.Decode(data)
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	if serial == 0 {
		return nil, 0, ErrInvalidSerialNumber
	}
	return &ReportAckSegment{ReportSerial: serial}, n, nil
}

// Humanize returns a human-readable summary.
func (r *ReportAckSegment) Humanize() string {
	return fmt.Sprintf("LTP Report Acknowledgment\n  Report serial ... %d", r.ReportSerial)
}

// CancelReason is the one-octet reason code of clause 3.2.4.
type CancelReason uint8

const (
	// ReasonUserCancelled means the client service cancelled the session.
	ReasonUserCancelled CancelReason = 0x00
	// ReasonUnreachable means the client service could not be reached.
	ReasonUnreachable CancelReason = 0x01
	// ReasonRetransmitLimit means the retransmission limit was exceeded.
	ReasonRetransmitLimit CancelReason = 0x02
	// ReasonMiscolored means red data arrived above a green offset, or green
	// data below a red one.
	ReasonMiscolored CancelReason = 0x03
	// ReasonSystemCancelled means a system error ended the session.
	ReasonSystemCancelled CancelReason = 0x04
	// ReasonRetransmitCyclesExceeded means the retransmission-cycles limit
	// was exceeded.
	ReasonRetransmitCyclesExceeded CancelReason = 0x05
)

// String names the reason.
func (c CancelReason) String() string {
	switch c {
	case ReasonUserCancelled:
		return "client service cancelled the session"
	case ReasonUnreachable:
		return "unreachable client service"
	case ReasonRetransmitLimit:
		return "retransmission limit exceeded"
	case ReasonMiscolored:
		return "miscolored block"
	case ReasonSystemCancelled:
		return "system error"
	case ReasonRetransmitCyclesExceeded:
		return "retransmission cycles limit exceeded"
	default:
		return fmt.Sprintf("reserved(%#02x)", uint8(c))
	}
}

// Valid reports whether the reason code is one clause 3.2.4 defines. Codes 06 to FF
// are reserved.
func (c CancelReason) Valid() bool { return c <= ReasonRetransmitCyclesExceeded }

// CancelSegment is the content of a cancel segment, per clause 3.2.4: a single
// reason-code octet.
type CancelSegment struct {
	Reason CancelReason
}

// Encode serializes the cancel content.
func (c *CancelSegment) Encode() ([]byte, error) {
	if !c.Reason.Valid() {
		return nil, ErrInvalidReasonCode
	}
	return []byte{byte(c.Reason)}, nil
}

// DecodeCancelSegment parses cancel content.
func DecodeCancelSegment(data []byte) (*CancelSegment, int, error) {
	if len(data) < 1 {
		return nil, 0, ErrDataTooShort
	}
	reason := CancelReason(data[0])
	if !reason.Valid() {
		return nil, 0, ErrInvalidReasonCode
	}
	return &CancelSegment{Reason: reason}, 1, nil
}

// Humanize returns a human-readable summary.
func (c *CancelSegment) Humanize() string {
	return "LTP Cancel Segment\n  Reason ... " + c.Reason.String()
}
