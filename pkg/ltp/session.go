package ltp

import (
	"slices"
	"sync"
)

// SessionState is where a transmission session has got to.
type SessionState int

const (
	// StateActive means the session is transmitting or receiving.
	StateActive SessionState = iota
	// StateWaitingReport means the sender has sent a checkpoint and is
	// waiting for the report that answers it.
	StateWaitingReport
	// StateClosed means the red part is fully acknowledged and the block is
	// complete.
	StateClosed
	// StateCancelled means the session was cancelled at one end or the other.
	StateCancelled
)

// String names the state.
func (s SessionState) String() string {
	switch s {
	case StateActive:
		return "active"
	case StateWaitingReport:
		return "waiting for report"
	case StateClosed:
		return "closed"
	default:
		return "cancelled"
	}
}

// span is a half-open range of block offsets, [start, end).
type span struct {
	start, end uint64
}

// spanSet keeps a sorted, merged set of ranges. Both session machines use it:
// the sender to track what a report has acknowledged, the receiver to track
// what has arrived.
type spanSet struct {
	spans []span
}

// add folds a range into the set, merging it with any neighbours it touches
// or overlaps. The set stays sorted, merged and non-overlapping.
func (s *spanSet) add(start, end uint64) {
	if end <= start {
		return
	}

	// Spans are sorted and non-overlapping, so an existing span's end is
	// monotonically increasing across the slice. lo is the first index whose
	// span cannot lie entirely below the new range: the first with an end at
	// or past start. Every earlier span stays untouched.
	lo, _ := slices.BinarySearchFunc(s.spans, start, func(sp span, start uint64) int {
		if sp.end < start {
			return -1
		}
		return 1
	})

	// Absorb every span from lo onward that touches or overlaps [start, end),
	// growing the new range to cover them.
	hi := lo
	for hi < len(s.spans) && s.spans[hi].start <= end {
		if s.spans[hi].start < start {
			start = s.spans[hi].start
		}
		if s.spans[hi].end > end {
			end = s.spans[hi].end
		}
		hi++
	}

	if lo == hi {
		s.spans = slices.Insert(s.spans, lo, span{start, end})
		return
	}
	s.spans[lo] = span{start, end}
	if hi-lo > 1 {
		s.spans = slices.Delete(s.spans, lo+1, hi)
	}
}

// gaps returns the ranges missing below limit.
func (s *spanSet) gaps(limit uint64) []span {
	var out []span
	var cursor uint64

	for _, existing := range s.spans {
		if existing.start > cursor {
			end := existing.start
			if end > limit {
				end = limit
			}
			if end > cursor {
				out = append(out, span{cursor, end})
			}
		}
		if existing.end > cursor {
			cursor = existing.end
		}
		if cursor >= limit {
			return out
		}
	}
	if cursor < limit {
		out = append(out, span{cursor, limit})
	}
	return out
}

// covers reports whether the set holds every octet below limit.
func (s *spanSet) covers(limit uint64) bool {
	if limit == 0 {
		return true
	}
	return len(s.spans) == 1 && s.spans[0].start == 0 && s.spans[0].end >= limit
}

// contiguousFrom returns how far the set extends without a gap from zero.
func (s *spanSet) contiguousFrom() uint64 {
	if len(s.spans) > 0 && s.spans[0].start == 0 {
		return s.spans[0].end
	}
	return 0
}

// SenderConfig describes one outgoing LTP session.
type SenderConfig struct {
	// SessionID names the session. The engine ID is this sender's.
	SessionID SessionID

	// ClientServiceID names the service at the far end.
	ClientServiceID uint64

	// SegmentSize is the largest client service data payload per segment.
	SegmentSize int

	// RedPartLength is how many leading octets of the block are red, and so
	// delivered reliably. Zero makes the whole block green; a value equal to
	// the block length makes it all red.
	RedPartLength uint64

	// FirstCheckpointSerial seeds the checkpoint counter. Clause 3.2.1 says the
	// first serial must be chosen randomly for security, and must never be
	// zero. The caller picks it, because this package has no randomness
	// policy of its own.
	FirstCheckpointSerial uint64
}

// DefaultSegmentSize is the payload size used when SenderConfig leaves
// SegmentSize at zero.
const DefaultSegmentSize = 1024

// Sender drives one outgoing LTP session.
//
// It owns no goroutines and no clock. The caller pumps it: NextSegment returns
// what to transmit, HandleSegment feeds inbound segments back in, and the
// caller's own scheduler decides when to retransmit a checkpoint. On a link
// where a round trip takes an hour, only the mission can pick that timeout.
//
// Usage:
//  1. Create with NewSender
//  2. Call NextSegment and transmit what it returns, until nothing is pending
//  3. Call HandleSegment when a segment arrives on the return link
//  4. Check Done or State for completion
//
// A Sender is safe for concurrent use.
type Sender struct {
	mu     sync.Mutex
	config SenderConfig

	block      []byte
	state      SessionState
	nextOffset uint64

	// checkpointSerial is the serial of the checkpoint currently outstanding.
	checkpointSerial uint64
	// nextCheckpoint is the serial the next checkpoint will carry.
	nextCheckpoint uint64

	// acknowledged tracks the red-part ranges a report has claimed.
	acknowledged spanSet

	// retransmit holds ranges a report showed missing.
	retransmit []span

	// pendingReportAcks holds report serials awaiting acknowledgment.
	pendingReportAcks []uint64

	// respondingTo is the serial of the report whose gaps are being
	// retransmitted; clause 3.2.1 requires the checkpoint that closes the cycle to
	// carry it. Zero when no report prompted the transmission.
	respondingTo uint64

	// cancelAckPending is set when a cancel from the receiver still needs
	// its acknowledgment, per clause 6.17.
	cancelAckPending bool

	// cancelReason is set once the session is cancelled.
	cancelReason *CancelReason
	cancelSent   bool
}

// NewSender prepares a session to send one block.
func NewSender(block []byte, config SenderConfig) (*Sender, error) {
	if config.SegmentSize <= 0 {
		config.SegmentSize = DefaultSegmentSize
	}
	if config.RedPartLength > uint64(len(block)) {
		config.RedPartLength = uint64(len(block))
	}
	if config.FirstCheckpointSerial == 0 {
		// Clause 3.2.1 forbids zero. Rather than invent randomness, insist.
		return nil, ErrInvalidSerialNumber
	}

	return &Sender{
		config:         config,
		block:          block,
		state:          StateActive,
		nextCheckpoint: config.FirstCheckpointSerial,
	}, nil
}

// header builds a segment header for an outgoing segment.
func (s *Sender) header(t SegmentType) *Header {
	return &Header{Type: t, SessionID: s.config.SessionID}
}

// dataTypeFor picks the segment type for a range, applying the flag rules of
// Clause 3.1.1: a red segment is a checkpoint when it ends the red part, and a
// segment is end-of-block when it reaches the end of the block.
func (s *Sender) dataTypeFor(start, end uint64, forceCheckpoint bool) SegmentType {
	blockLen := uint64(len(s.block))
	red := s.config.RedPartLength

	if start >= red {
		// Green part.
		if end >= blockLen {
			return TypeGreenDataEOB
		}
		return TypeGreenData
	}

	endOfRed := end >= red
	endOfBlock := end >= blockLen

	switch {
	case endOfRed && endOfBlock:
		return TypeRedDataCheckpointEORPEOB
	case endOfRed:
		return TypeRedDataCheckpointEORP
	case forceCheckpoint:
		return TypeRedDataCheckpoint
	default:
		return TypeRedData
	}
}

// buildData assembles a data segment for a range. reportSerial is the serial
// of the report that prompted this transmission, or zero when none did;
// Clause 3.2.1 puts it on any checkpoint the range produces.
func (s *Sender) buildData(start, end uint64, forceCheckpoint bool, reportSerial uint64) (*Segment, error) {
	t := s.dataTypeFor(start, end, forceCheckpoint)

	d := &DataSegment{
		ClientServiceID: s.config.ClientServiceID,
		Offset:          start,
		Data:            s.block[start:end],
	}

	if t.IsCheckpoint() {
		d.CheckpointSerial = s.nextCheckpoint
		s.checkpointSerial = s.nextCheckpoint
		s.nextCheckpoint++
		// Clause 3.2.1: the report serial of the prompting report, or zero when
		// the checkpoint was not prompted by one.
		d.ReportSerial = reportSerial
	}

	seg := &Segment{Header: s.header(t), Data: d}
	if err := seg.Validate(); err != nil {
		return nil, err
	}
	return seg, nil
}

// NextSegment returns the next segment to transmit, or ok == false when
// nothing is pending. A false does not mean the session is finished; check
// Done for that.
func (s *Sender) NextSegment() (*Segment, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A cancellation goes out before anything else, once.
	if s.cancelReason != nil {
		if s.cancelSent {
			return nil, false, nil
		}
		s.cancelSent = true
		seg := &Segment{
			Header: s.header(TypeCancelFromSender),
			Cancel: &CancelSegment{Reason: *s.cancelReason},
		}
		return seg, true, nil
	}

	// Clause 6.17: a cancel from the receiver is acknowledged, even though the
	// session is over as far as this end is concerned. Without the ack the
	// receiver's cancel timer retransmits forever.
	if s.cancelAckPending {
		s.cancelAckPending = false
		return &Segment{Header: s.header(TypeCancelAckToReceiver)}, true, nil
	}

	// Acknowledge any reports received, per clause 3.2.3. This runs before the
	// closed-state check on purpose: the report that completed the red part
	// arrives an instant before the session closes, and clause 6.13 still requires
	// its RA. A conformant peer retransmits the report until it gets one.
	if len(s.pendingReportAcks) > 0 {
		serial := s.pendingReportAcks[0]
		s.pendingReportAcks = s.pendingReportAcks[1:]
		seg := &Segment{
			Header:    s.header(TypeReportAck),
			ReportAck: &ReportAckSegment{ReportSerial: serial},
		}
		return seg, true, nil
	}

	if s.state == StateClosed || s.state == StateCancelled {
		return nil, false, nil
	}

	// Retransmissions come before fresh data so gaps close promptly.
	if len(s.retransmit) > 0 {
		gap := s.retransmit[0]
		s.retransmit = s.retransmit[1:]

		end := gap.start + uint64(s.config.SegmentSize)
		if end > gap.end {
			end = gap.end
		}
		if end < gap.end {
			// Split: keep the remainder queued.
			s.retransmit = append([]span{{end, gap.end}}, s.retransmit...)
		}

		// Clause 6.9: the last segment of a retransmission cycle is a checkpoint
		// wherever it sits in the block, so the receiver reports again and
		// the loop can converge. Reaching the end of the red part is a
		// checkpoint for the same reason.
		last := len(s.retransmit) == 0
		seg, err := s.buildData(gap.start, end, last || end >= s.config.RedPartLength, s.respondingTo)
		if err != nil {
			return nil, false, err
		}
		if seg.Header.Type.IsCheckpoint() {
			s.state = StateWaitingReport
		}
		return seg, true, nil
	}

	// Fresh data.
	blockLen := uint64(len(s.block))
	if s.nextOffset < blockLen {
		start := s.nextOffset
		end := start + uint64(s.config.SegmentSize)
		if end > blockLen {
			end = blockLen
		}
		// Never let one segment straddle the red/green boundary: the type
		// code has to describe the whole payload.
		red := s.config.RedPartLength
		if start < red && end > red {
			end = red
		}
		s.nextOffset = end

		seg, err := s.buildData(start, end, false, 0)
		if err != nil {
			return nil, false, err
		}
		if seg.Header.Type.IsCheckpoint() {
			s.state = StateWaitingReport
		}
		return seg, true, nil
	}

	// Everything sent. A block with no red part needs no acknowledgment.
	if s.config.RedPartLength == 0 {
		s.state = StateClosed
	}
	return nil, false, nil
}

// HandleSegment feeds a segment arriving on the return link into the session.
//
// A sender expects reports, cancels from the receiver, and acknowledgments of
// its own cancels. Anything else is ignored, since a shared link may carry
// segments for other sessions.
func (s *Sender) HandleSegment(seg *Segment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if seg == nil || seg.Header == nil {
		return ErrDataTooShort
	}
	if seg.Header.SessionID != s.config.SessionID {
		return nil // another session's traffic
	}

	switch seg.Header.Type {
	case TypeReport:
		if seg.Report == nil {
			return ErrWrongSegmentType
		}
		return s.handleReport(seg.Report)

	case TypeCancelFromReceiver:
		// Clause 6.17: queue the acknowledgment before tearing the session down.
		s.cancelAckPending = true
		s.state = StateCancelled

	case TypeCancelAckToSender:
		s.state = StateCancelled
	}
	return nil
}

// handleReport folds a reception report into the session and queues whatever
// the gaps demand.
func (s *Sender) handleReport(r *ReportSegment) error {
	// Clause 3.2.3: every report is acknowledged, even one rejected below -
	// refusing the ack would just make a conformant peer retransmit the same
	// report forever.
	s.pendingReportAcks = append(s.pendingReportAcks, r.ReportSerial)

	// LTP has no authentication of its own (RFC 5326 defers that to a
	// security extension), but the sender knows the true length of the red
	// part it is sending, and a report cannot claim more of it than exists.
	// Check every number in the report against that bound before folding
	// anything in: past this point s.acknowledged is trusted to decide
	// whether the block is done and whether the retransmit queue gets
	// discarded, so a spoofed or corrupt report claiming full coverage must
	// not reach it.
	red := s.config.RedPartLength
	if r.UpperBound > red {
		return ErrReportOutOfRange
	}
	claims := r.ClaimedRanges()
	for _, c := range claims {
		end := c.Offset + c.Length
		if c.Length == 0 || end < c.Offset || end > red {
			// end < c.Offset catches both an overflowing sum and a claim
			// ClaimedRanges saturated to math.MaxUint64 because LowerBound
			// plus its own offset had already overflowed.
			return ErrReportOutOfRange
		}
	}

	for _, c := range claims {
		s.acknowledged.add(c.Offset, c.Offset+c.Length)
	}

	if s.acknowledged.covers(red) {
		s.state = StateClosed
		s.retransmit = nil
		s.respondingTo = 0
		return nil
	}

	// Anything below the report's upper bound that is not claimed must go
	// again. UpperBound is already checked to be no more than red, above.
	s.retransmit = append(s.retransmit, s.acknowledged.gaps(r.UpperBound)...)
	// Clause 3.2.1: the checkpoint closing this retransmission cycle carries the
	// serial of the report that prompted it.
	s.respondingTo = r.ReportSerial
	s.state = StateActive
	return nil
}

// Cancel abandons the session. The next segment out is a cancel carrying this
// reason.
func (s *Sender) Cancel(reason CancelReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !reason.Valid() {
		return ErrInvalidReasonCode
	}
	s.cancelReason = &reason
	s.cancelSent = false
	s.state = StateCancelled
	return nil
}

// ResendCheckpoint re-queues the outstanding red-part gaps. The caller invokes
// this from its own timer when a report does not come back.
func (s *Sender) ResendCheckpoint() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateWaitingReport {
		return
	}
	red := s.config.RedPartLength
	s.retransmit = nil
	s.retransmit = append(s.retransmit, s.acknowledged.gaps(red)...)
}

// State returns the session state.
func (s *Sender) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Done reports whether the session has closed or been cancelled.
func (s *Sender) Done() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == StateClosed || s.state == StateCancelled
}

// RedPartAcknowledged reports whether every octet of the red part has been
// claimed by a report.
func (s *Sender) RedPartAcknowledged() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acknowledged.covers(s.config.RedPartLength)
}
