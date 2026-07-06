package ltp

import "sync"

// ReceiverConfig describes one incoming LTP session.
type ReceiverConfig struct {
	// SessionID names the session, taken from the first segment received.
	SessionID SessionID

	// FirstReportSerial seeds the report counter. §3.2.2 says the first
	// serial must be chosen randomly for security, and must never be zero.
	// The caller picks it.
	FirstReportSerial uint64

	// MaxBlockSize caps how large a block this session will assemble, in
	// octets. Zero selects DefaultMaxBlockSize.
	//
	// This cap is not in RFC 5326: the protocol puts no ceiling on a block,
	// and a data segment's offset is an SDNV reaching 2^64. Without a limit,
	// one corrupt or hostile segment claiming a huge offset would make the
	// receiver try to allocate that much memory. Set it to what the mission
	// actually sends.
	MaxBlockSize uint64
}

// DefaultMaxBlockSize bounds a received block when ReceiverConfig leaves
// MaxBlockSize at zero: 64 MiB.
const DefaultMaxBlockSize = 64 << 20

// Receiver drives one incoming LTP session.
//
// Like Sender it owns no goroutines and no clock: the caller feeds it segments
// with HandleSegment and asks NextSegment what to send back.
//
// A Receiver is safe for concurrent use.
type Receiver struct {
	mu     sync.Mutex
	config ReceiverConfig

	state SessionState

	// received tracks the block ranges that have arrived.
	received spanSet
	// data holds the block as it assembles.
	data []byte

	// redPartLength is learned from the segment that ends the red part.
	redPartLength uint64
	redPartKnown  bool

	// blockLength is learned from the segment that ends the block.
	blockLength uint64
	blockKnown  bool

	// lowestGreenOffset and highestRedOffset detect a miscolored block.
	lowestGreenOffset uint64
	greenSeen         bool
	highestRedEnd     uint64

	nextReportSerial uint64
	pending          []*Segment

	// awaitingAck holds report serials still unacknowledged.
	awaitingAck map[uint64]bool

	cancelReason *CancelReason

	// maxBlockSize caps the assembled block.
	maxBlockSize uint64
}

// NewReceiver prepares a session to receive one block.
func NewReceiver(config ReceiverConfig) (*Receiver, error) {
	if config.FirstReportSerial == 0 {
		return nil, ErrInvalidSerialNumber
	}
	maxBlock := config.MaxBlockSize
	if maxBlock == 0 {
		maxBlock = DefaultMaxBlockSize
	}
	return &Receiver{
		config:           config,
		state:            StateActive,
		nextReportSerial: config.FirstReportSerial,
		awaitingAck:      make(map[uint64]bool),
		maxBlockSize:     maxBlock,
	}, nil
}

// header builds a segment header for a segment heading back to the sender.
func (r *Receiver) header(t SegmentType) *Header {
	return &Header{Type: t, SessionID: r.config.SessionID}
}

// HandleSegment feeds one arriving segment into the session.
func (r *Receiver) HandleSegment(seg *Segment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if seg == nil || seg.Header == nil {
		return ErrDataTooShort
	}
	if seg.Header.SessionID != r.config.SessionID {
		return nil
	}

	t := seg.Header.Type
	switch {
	case t.IsData():
		if seg.Data == nil {
			return ErrWrongSegmentType
		}
		return r.handleData(t, seg.Data)

	case t == TypeReportAck:
		if seg.ReportAck != nil {
			delete(r.awaitingAck, seg.ReportAck.ReportSerial)
		}

	case t == TypeCancelFromSender:
		r.state = StateCancelled
		// §3.2.5: acknowledge the cancel.
		r.pending = append(r.pending, &Segment{Header: r.header(TypeCancelAckToSender)})

	case t == TypeCancelAckToReceiver:
		r.state = StateCancelled
	}
	return nil
}

// handleData stores one data segment and reacts to its flags.
func (r *Receiver) handleData(t SegmentType, d *DataSegment) error {
	start, end := d.Offset, d.End()

	// A segment's offset is an SDNV, so it can name a position far beyond any
	// real block. Refuse before sizing a buffer from it: without this, one
	// corrupt segment would make the receiver try to allocate petabytes.
	if end < start || end > r.maxBlockSize {
		r.cancelWithReason(ReasonSystemCancelled)
		return ErrBlockTooLarge
	}

	// §3.2.4 MISCOLORED: red data must not sit above any green offset, and
	// green data must not sit below any red offset.
	if t.IsRedData() {
		if r.greenSeen && start >= r.lowestGreenOffset {
			r.cancelWithReason(ReasonMiscolored)
			return ErrRedGreenOrder
		}
		if end > r.highestRedEnd {
			r.highestRedEnd = end
		}
	}
	if t.IsGreenData() {
		if start < r.highestRedEnd {
			r.cancelWithReason(ReasonMiscolored)
			return ErrRedGreenOrder
		}
		if !r.greenSeen || start < r.lowestGreenOffset {
			r.lowestGreenOffset = start
			r.greenSeen = true
		}
	}

	// Grow the buffer and copy the payload into place.
	if uint64(len(r.data)) < end {
		grown := make([]byte, end)
		copy(grown, r.data)
		r.data = grown
	}
	copy(r.data[start:end], d.Data)
	r.received.add(start, end)

	// §3.1.3: these flags tell us the shape of the block.
	if t.IsEORP() {
		r.redPartLength = end
		r.redPartKnown = true
	}
	if t.IsEOB() {
		r.blockLength = end
		r.blockKnown = true
	}

	// §6.13: a checkpoint prompts a report.
	if t.IsCheckpoint() {
		r.queueReport(d.CheckpointSerial)
	}
	return nil
}

// queueReport builds a report covering the red part and queues it.
func (r *Receiver) queueReport(checkpointSerial uint64) {
	upper := r.redPartLength
	if !r.redPartKnown {
		upper = r.received.contiguousFrom()
	}

	report := &ReportSegment{
		ReportSerial:     r.nextReportSerial,
		CheckpointSerial: checkpointSerial,
		UpperBound:       upper,
		LowerBound:       0,
	}

	// Claim offsets are measured from the lower bound, which is zero here.
	for _, s := range r.received.spans {
		start, end := s.start, s.end
		if start >= upper {
			break
		}
		if end > upper {
			end = upper
		}
		if end <= start {
			continue
		}
		report.Claims = append(report.Claims, ReceptionClaim{
			Offset: start - report.LowerBound,
			Length: end - start,
		})
	}

	if err := report.Validate(); err != nil {
		// A report with nothing worth claiming is not sent.
		return
	}

	r.awaitingAck[report.ReportSerial] = true
	r.nextReportSerial++
	r.pending = append(r.pending, &Segment{Header: r.header(TypeReport), Report: report})

	if r.redPartKnown && r.received.covers(r.redPartLength) {
		r.state = StateClosed
	}
}

// cancelWithReason queues a cancel from the receiver.
func (r *Receiver) cancelWithReason(reason CancelReason) {
	if r.cancelReason != nil {
		return
	}
	r.cancelReason = &reason
	r.state = StateCancelled
	r.pending = append(r.pending, &Segment{
		Header: r.header(TypeCancelFromReceiver),
		Cancel: &CancelSegment{Reason: reason},
	})
}

// NextSegment returns the next segment to send back, or ok == false when
// nothing is pending.
func (r *Receiver) NextSegment() (*Segment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.pending) == 0 {
		return nil, false, nil
	}
	seg := r.pending[0]
	r.pending = r.pending[1:]
	return seg, true, nil
}

// RequestReport queues an asynchronous report, one not prompted by a
// checkpoint. §3.2.2 gives it a checkpoint serial of zero. The caller drives
// this from its own timer.
func (r *Receiver) RequestReport() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queueReport(0)
}

// Cancel abandons the session from the receiver's end.
func (r *Receiver) Cancel(reason CancelReason) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !reason.Valid() {
		return ErrInvalidReasonCode
	}
	r.cancelWithReason(reason)
	return nil
}

// Block returns the data received so far. It is complete only when Complete
// reports true.
func (r *Receiver) Block() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, len(r.data))
	copy(out, r.data)
	return out
}

// RedPart returns the reliably delivered prefix of the block.
func (r *Receiver) RedPart() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.redPartKnown || uint64(len(r.data)) < r.redPartLength {
		return nil
	}
	out := make([]byte, r.redPartLength)
	copy(out, r.data[:r.redPartLength])
	return out
}

// RedPartComplete reports whether every octet of the red part has arrived.
func (r *Receiver) RedPartComplete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.redPartKnown && r.received.covers(r.redPartLength)
}

// Complete reports whether the whole block has arrived, red and green.
func (r *Receiver) Complete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.blockKnown && r.received.covers(r.blockLength)
}

// MissingRanges returns the red-part ranges still outstanding, as block
// offsets.
func (r *Receiver) MissingRanges() []ReceptionClaim {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.redPartLength
	if !r.redPartKnown {
		limit = r.received.contiguousFrom()
	}
	var out []ReceptionClaim
	for _, gap := range r.received.gaps(limit) {
		out = append(out, ReceptionClaim{Offset: gap.start, Length: gap.end - gap.start})
	}
	return out
}

// State returns the session state.
func (r *Receiver) State() SessionState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Done reports whether the session has closed or been cancelled.
func (r *Receiver) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == StateClosed || r.state == StateCancelled
}
