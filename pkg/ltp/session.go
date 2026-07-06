package ltp

import "sync"

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

// add folds a range into the set, merging it with any neighbours.
func (s *spanSet) add(start, end uint64) {
	if end <= start {
		return
	}
	merged := make([]span, 0, len(s.spans)+1)
	added := false

	for _, existing := range s.spans {
		switch {
		case existing.end < start:
			merged = append(merged, existing)
		case existing.start > end:
			if !added {
				merged = append(merged, span{start, end})
				added = true
			}
			merged = append(merged, existing)
		default:
			if existing.start < start {
				start = existing.start
			}
			if existing.end > end {
				end = existing.end
			}
		}
	}
	if !added {
		merged = append(merged, span{start, end})
	}
	s.spans = merged
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

	// FirstCheckpointSerial seeds the checkpoint counter. §3.2.1 says the
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
		// §3.2.1 forbids zero. Rather than invent randomness, insist.
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
// §3.1.1: a red segment is a checkpoint when it ends the red part, and a
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

// buildData assembles a data segment for a range.
func (s *Sender) buildData(start, end uint64, forceCheckpoint bool) (*Segment, error) {
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
		// ReportSerial stays zero unless this checkpoint answers a report;
		// §3.2.1 requires zero in that case.
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

	if s.state == StateClosed {
		return nil, false, nil
	}

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

	// Acknowledge any reports received, per §3.2.3.
	if len(s.pendingReportAcks) > 0 {
		serial := s.pendingReportAcks[0]
		s.pendingReportAcks = s.pendingReportAcks[1:]
		seg := &Segment{
			Header:    s.header(TypeReportAck),
			ReportAck: &ReportAckSegment{ReportSerial: serial},
		}
		return seg, true, nil
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

		// A retransmitted range that reaches the end of the red part is a
		// checkpoint again, so the receiver knows to report.
		seg, err := s.buildData(gap.start, end, end >= s.config.RedPartLength)
		if err != nil {
			return nil, false, err
		}
		s.state = StateWaitingReport
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

		seg, err := s.buildData(start, end, false)
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
		s.state = StateCancelled

	case TypeCancelAckToSender:
		s.state = StateCancelled
	}
	return nil
}

// handleReport folds a reception report into the session and queues whatever
// the gaps demand.
func (s *Sender) handleReport(r *ReportSegment) error {
	// §3.2.3: every report is acknowledged.
	s.pendingReportAcks = append(s.pendingReportAcks, r.ReportSerial)

	// Claim offsets are relative to the report's lower bound.
	for _, c := range r.ClaimedRanges() {
		s.acknowledged.add(c.Offset, c.Offset+c.Length)
	}

	red := s.config.RedPartLength
	if s.acknowledged.covers(red) {
		s.state = StateClosed
		s.retransmit = nil
		return nil
	}

	// Anything below the report's upper bound that is not claimed must go
	// again.
	limit := r.UpperBound
	if limit > red {
		limit = red
	}
	s.retransmit = append(s.retransmit, s.acknowledged.gaps(limit)...)
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
