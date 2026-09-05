package pxdl

import "fmt"

// Packet segmentation, per CCSDS 211.0-B-6 clause 3.2.3.3.
//
// A packet too big for one frame is cut into segments, each behind a one-octet
// segment header. The header says where the segment sits in the packet and
// carries a pseudo packet ID tying the pieces together.

// SegmentHeaderSize is the width of a segment header in octets (clause 3.2.3.3.1).
const SegmentHeaderSize = 1

// SequenceFlags say where a segment sits relative to its packet, per table 3-4.
type SequenceFlags uint8

const (
	// SegmentContinuing is a middle segment ('00').
	SegmentContinuing SequenceFlags = 0
	// SegmentFirst starts a packet ('01').
	SegmentFirst SequenceFlags = 1
	// SegmentLast ends a packet ('10').
	SegmentLast SequenceFlags = 2
	// SegmentUnsegmented carries a whole packet ('11').
	SegmentUnsegmented SequenceFlags = 3
)

// String names the position.
func (s SequenceFlags) String() string {
	switch s {
	case SegmentFirst:
		return "first segment"
	case SegmentContinuing:
		return "continuing segment"
	case SegmentLast:
		return "last segment"
	default:
		return "unsegmented"
	}
}

// SegmentHeader is the one-octet header of a segment data unit (clause 3.2.3.3.2).
//
//	bits 0-1: sequence flags
//	bits 2-7: pseudo packet identifier
type SegmentHeader struct {
	SequenceFlags SequenceFlags
	// PseudoPacketID ties the segments of one packet together, 6 bits.
	PseudoPacketID uint8
}

// Validate checks the header's field widths.
func (s *SegmentHeader) Validate() error {
	if s.PseudoPacketID > 0x3F {
		return ErrInvalidSegment
	}
	return nil
}

// Encode serializes the segment header.
func (s *SegmentHeader) Encode() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return []byte{byte(s.SequenceFlags&0x03)<<6 | s.PseudoPacketID&0x3F}, nil
}

// Decode parses a segment header.
func (s *SegmentHeader) Decode(data []byte) error {
	if len(data) < SegmentHeaderSize {
		return ErrDataTooShort
	}
	s.SequenceFlags = SequenceFlags(data[0] >> 6 & 0x03)
	s.PseudoPacketID = data[0] & 0x3F
	return nil
}

// Humanize returns a human-readable summary.
func (s *SegmentHeader) Humanize() string {
	return fmt.Sprintf("Segment Header\n  Position ..... %s\n  Pseudo packet ID .. %d",
		s.SequenceFlags, s.PseudoPacketID)
}

// Segment is one segment data unit: a header and a slice of a packet.
type Segment struct {
	Header SegmentHeader
	Data   []byte
}

// Encode serializes the segment data unit.
func (s *Segment) Encode() ([]byte, error) {
	header, err := s.Header.Encode()
	if err != nil {
		return nil, err
	}
	return append(header, s.Data...), nil
}

// DecodeSegment parses a segment data unit from a U-frame's data field.
func DecodeSegment(data []byte) (*Segment, error) {
	s := &Segment{}
	if err := s.Header.Decode(data); err != nil {
		return nil, err
	}
	s.Data = make([]byte, len(data)-SegmentHeaderSize)
	copy(s.Data, data[SegmentHeaderSize:])
	return s, nil
}

// RoutingID identifies the stream a segment belongs to, per clause 1.5.1.2: the
// physical channel, the port, and the pseudo packet ID together.
//
// Clause 3.2.3.3.2 c) requires all segments of one packet to travel with the same
// PCID and Port ID, which is what makes this triple sufficient.
type RoutingID struct {
	PCID           uint8
	PortID         uint8
	PseudoPacketID uint8
}

// String renders the routing ID.
func (r RoutingID) String() string {
	return fmt.Sprintf("PCID %d, port %d, packet %d", r.PCID, r.PortID, r.PseudoPacketID)
}

// DefaultMaxPacketSize bounds an accumulating packet when Reassembler leaves
// MaxPacketSize at zero: 64 KiB.
//
// The standard sets no ceiling on a reassembled packet. Without one, a stream
// of "continuing" segments that never ends would grow without limit.
const DefaultMaxPacketSize = 64 << 10

// DefaultMaxPending bounds how many routing IDs may have a partial packet
// open at once when Reassembler leaves MaxPending at zero: 32.
//
// PCID (1 bit), Port ID (3 bits) and Pseudo Packet ID (6 bits) together name
// 1024 distinct routing IDs, clause 3.2.3.3.2 c)'s own key, and every bit of it is
// chosen by the peer. MaxPacketSize alone bounds one buffer; nothing bounded
// the set of them. A peer that opens every routing ID and finishes none would
// otherwise pin MaxPacketSize octets per key at once: 64 MiB at the defaults,
// proportionally more if a mission raises MaxPacketSize.
const DefaultMaxPending = 32

// Reassembler rebuilds packets from the segments arriving on a link, per
// Clause 3.2.3.3.3.
//
// Segments of different packets interleave freely as long as they differ in
// PCID or Port ID, so the reassembler keeps one buffer per routing ID.
//
// Clause 3.2.3.3.4 is strict: only complete packets are delivered. A stream that
// starts mid-packet, or grows past the limit, is discarded rather than
// half-delivered.
//
// A Reassembler is not safe for concurrent use.
type Reassembler struct {
	// MaxPacketSize bounds one accumulating packet. Zero selects
	// DefaultMaxPacketSize.
	MaxPacketSize int

	// MaxPending bounds how many routing IDs may have a partial packet open
	// at once. Zero selects DefaultMaxPending. Admitting one more than the
	// limit evicts the oldest partial still open, which is safe: clause
	// 3.2.3.3.4 delivers only complete packets, so one that never completes
	// was already lost.
	MaxPending int

	partial map[RoutingID][]byte
	// order lists the routing IDs currently in partial, oldest first, so
	// eviction knows what to drop. Kept in lockstep with partial: every
	// insertion appends here, every removal removes here.
	order []RoutingID
}

// NewReassembler returns an empty reassembler.
func NewReassembler() *Reassembler {
	return &Reassembler{partial: make(map[RoutingID][]byte)}
}

// maxSize returns the effective packet ceiling.
func (r *Reassembler) maxSize() int {
	if r.MaxPacketSize > 0 {
		return r.MaxPacketSize
	}
	return DefaultMaxPacketSize
}

// maxPending returns the effective ceiling on concurrently open partials.
func (r *Reassembler) maxPending() int {
	if r.MaxPending > 0 {
		return r.MaxPending
	}
	return DefaultMaxPending
}

// admit installs buf as the partial for id, evicting the oldest open partial
// first if id is new and the reassembler is already at maxPending.
//
// id already being open (a restart per clause 3.2.3.3.5 b, or a continuing
// segment updating its own buffer) never evicts anything: it is not a new
// entrant into the open set, just a value update.
func (r *Reassembler) admit(id RoutingID, buf []byte) {
	if _, open := r.partial[id]; !open {
		for len(r.partial) >= r.maxPending() {
			r.evictOldest()
		}
		r.order = append(r.order, id)
	}
	r.partial[id] = buf
}

// evictOldest drops the oldest partial still open. A partial that never
// completes was already lost, per clause 3.2.3.3.4, so dropping it early loses
// nothing a peer that finishes its packets would notice.
func (r *Reassembler) evictOldest() {
	if len(r.order) == 0 {
		return
	}
	oldest := r.order[0]
	r.order = r.order[1:]
	delete(r.partial, oldest)
}

// forget discards id's partial, if any, and removes it from the eviction
// queue so that queue never grows past what is actually open.
func (r *Reassembler) forget(id RoutingID) {
	if _, open := r.partial[id]; !open {
		return
	}
	delete(r.partial, id)
	for i, o := range r.order {
		if o == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Accept folds one segment into the reassembler.
//
// It returns a complete packet when this segment finishes one, and nil when
// more segments are still needed.
func (r *Reassembler) Accept(pcid, portID uint8, seg *Segment) ([]byte, error) {
	if seg == nil {
		return nil, ErrInvalidSegment
	}
	if r.partial == nil {
		r.partial = make(map[RoutingID][]byte)
	}

	id := RoutingID{PCID: pcid, PortID: portID, PseudoPacketID: seg.Header.PseudoPacketID}

	switch seg.Header.SequenceFlags {
	case SegmentUnsegmented:
		// A whole packet in one segment. Any partial buffer for this routing
		// ID was never completed, so drop it.
		r.forget(id)
		out := make([]byte, len(seg.Data))
		copy(out, seg.Data)
		return out, nil

	case SegmentFirst:
		// Clause 3.2.3.3.5 b): a new first segment abandons whatever came before.
		buf := make([]byte, len(seg.Data))
		copy(buf, seg.Data)
		if len(buf) > r.maxSize() {
			r.forget(id)
			return nil, ErrReassemblyTooLarge
		}
		r.admit(id, buf)
		return nil, nil

	case SegmentContinuing, SegmentLast:
		buf, started := r.partial[id]
		if !started {
			// Clause 3.2.3.3.5 b): the first segment for a routing ID must be a
			// start segment. Discard rather than guess.
			return nil, ErrSegmentOutOfOrder
		}
		if len(buf)+len(seg.Data) > r.maxSize() {
			r.forget(id)
			return nil, ErrReassemblyTooLarge
		}
		buf = append(buf, seg.Data...)

		if seg.Header.SequenceFlags == SegmentLast {
			r.forget(id)
			return buf, nil
		}
		r.partial[id] = buf
		return nil, nil

	default:
		return nil, ErrInvalidSegment
	}
}

// AcceptFrame folds a U-frame's segment into the reassembler, taking the PCID
// and Port ID from the frame header.
func (r *Reassembler) AcceptFrame(f *TransferFrame) ([]byte, error) {
	if f == nil {
		return nil, ErrInvalidSegment
	}
	if !f.IsUserFrame() {
		return nil, ErrNotUserFrame
	}
	if f.Header.DFCID != DFCSegment {
		return nil, ErrInvalidDFCID
	}

	seg, err := DecodeSegment(f.DataField)
	if err != nil {
		return nil, err
	}
	return r.Accept(f.Header.PCID, f.Header.PortID, seg)
}

// Pending returns how many partial packets are still accumulating.
func (r *Reassembler) Pending() int { return len(r.partial) }

// Reset discards every partial packet.
func (r *Reassembler) Reset() {
	r.partial = make(map[RoutingID][]byte)
	r.order = nil
}

// Segmentize cuts a packet into segments whose data fields are at most
// maxSegmentData octets each, tagged with the given pseudo packet ID.
//
// A packet that fits in one segment comes back as a single unsegmented one,
// which is what the '11' sequence flag is for.
func Segmentize(packet []byte, pseudoPacketID uint8, maxSegmentData int) ([]*Segment, error) {
	if maxSegmentData <= 0 {
		return nil, ErrInvalidSegment
	}
	if pseudoPacketID > 0x3F {
		return nil, ErrInvalidSegment
	}

	if len(packet) <= maxSegmentData {
		return []*Segment{{
			Header: SegmentHeader{SequenceFlags: SegmentUnsegmented, PseudoPacketID: pseudoPacketID},
			Data:   packet,
		}}, nil
	}

	var out []*Segment
	for start := 0; start < len(packet); start += maxSegmentData {
		end := start + maxSegmentData
		if end > len(packet) {
			end = len(packet)
		}

		flags := SegmentContinuing
		switch {
		case start == 0:
			flags = SegmentFirst
		case end >= len(packet):
			flags = SegmentLast
		}

		piece := make([]byte, end-start)
		copy(piece, packet[start:end])
		out = append(out, &Segment{
			Header: SegmentHeader{SequenceFlags: flags, PseudoPacketID: pseudoPacketID},
			Data:   piece,
		})
	}
	return out, nil
}
