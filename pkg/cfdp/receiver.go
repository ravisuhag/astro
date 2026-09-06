package cfdp

import (
	"slices"
	"sort"
	"sync"
)

// segment is one contiguous run of received file data, half-open [start, end).
type segment struct {
	start, end uint64
}

// ReceiverConfig describes one incoming transaction.
type ReceiverConfig struct {
	// Source, Destination and TransactionSeq identify the transaction. They
	// come from the first PDU received. Inbound PDUs whose source entity ID or
	// transaction sequence number differ are ignored (clause 5.1), so one Receiver
	// never applies a foreign transaction's PDUs.
	Source         EntityID
	Destination    EntityID
	TransactionSeq EntityID

	// Acknowledged selects Class 2 behavior: NAKs, Finished, and its ACK.
	Acknowledged bool

	// CRCFlag adds a CRC to every outgoing PDU.
	CRCFlag bool

	// DestinationFileName overrides the name in the Metadata PDU. Leave it
	// empty to use what the sender asked for.
	DestinationFileName string

	// FaultHandlers overrides the default disposition for the given fault
	// conditions at this entity. Table 4-1 defaults every condition to a
	// Notice of Cancellation; fault handler override TLVs arriving in the
	// Metadata PDU take precedence over both (clause 4.8).
	FaultHandlers map[ConditionCode]FaultHandler

	// MaxFileSize bounds the highest file offset this receiver will accept,
	// counting the end of each File Data PDU (offset plus data length).
	// The standard puts no ceiling on offsets, so without one a single
	// crafted PDU naming a huge offset would make the receiver allocate
	// that much memory. Zero means DefaultMaxFileSize. Set it to what the
	// mission actually transfers.
	MaxFileSize uint64

	// MaxSegments bounds how many distinct, non-adjacent byte ranges this
	// receiver will track in its received-data set at once. Two ranges that
	// touch or overlap merge into one and never count against this; only a
	// genuinely new, separate range does. Nothing in the standard bounds how
	// many such ranges a peer can force the receiver to hold -- file data
	// arriving with a lost octet between every segment never merges -- so
	// without a cap recordSegment's work and the eventual NAK would both
	// grow without bound. Zero means DefaultMaxSegments.
	MaxSegments int
}

// DefaultMaxFileSize bounds a transfer when ReceiverConfig leaves MaxFileSize
// at zero: 64 MiB, matching ltp.DefaultMaxBlockSize.
const DefaultMaxFileSize = 64 << 20

// DefaultMaxSegments bounds a transfer when ReceiverConfig leaves MaxSegments
// at zero: 65536, comfortably above what an ordinary lossy transfer
// fragments into while keeping the received-data set cheap to search.
const DefaultMaxSegments = 65536

// Receiver drives one incoming CFDP transaction.
//
// Like Sender it owns no goroutines and no clock: the caller feeds it PDUs
// with HandlePDU and asks NextPDU what to send back.
//
// Usage:
//  1. Create with NewReceiver
//  2. Call HandlePDU for every PDU that arrives
//  3. Call NextPDU and transmit whatever it returns
//  4. Check Done or State to see when the transaction has completed
//
// A Receiver is safe for concurrent use.
type Receiver struct {
	mu     sync.Mutex
	config ReceiverConfig
	fs     Filestore

	state     TransactionState
	suspended bool
	cancelled bool

	metadata  *MetadataPDU
	largeFile bool

	// early buffers File Data PDUs that arrive before the Metadata PDU makes
	// them writable. They replay, in arrival order, once Metadata arrives.
	early []*FileDataPDU

	// earlyBytes is the running total of len(fd.Data) held in early, kept
	// incrementally so a peer that never sends Metadata cannot make this
	// buffer grow without bound: it is checked against MaxFileSize the same
	// way a single segment's offset is.
	earlyBytes uint64

	// received tracks the byte ranges delivered so far, kept sorted and merged
	// so gap detection is a linear scan.
	received []segment

	// highWater is the greatest end offset seen, used before EOF tells us the
	// real size.
	highWater uint64

	checksum     Checksum
	declaredSize uint64
	eofChecksum  uint32
	eofSeen      bool
	condition    ConditionCode

	// faultOverrides holds the dispositions the Metadata PDU's fault handler
	// override TLVs installed (clause 5.4.4).
	faultOverrides map[ConditionCode]FaultHandler

	// faultLocation names the entity where the fault occurred, when that is
	// not this one, a sender-side cancel, for instance.
	faultLocation *EntityID

	// filename is where the file is being written.
	filename string

	// pending holds PDUs waiting to go out.
	pending []*PDU

	// finishedSent and finishedAcked track the close-out handshake.
	// lastDelivery and lastStatus remember what the Finished PDU said, so a
	// lost one can be rebuilt by ResendFinished.
	finishedSent  bool
	finishedAcked bool
	lastDelivery  DeliveryCode
	lastStatus    FileStatus

	// filestoreResponses accumulate as requests are executed.
	filestoreResponses []TLV
}

// NewReceiver prepares a transaction to receive one file.
func NewReceiver(fs Filestore, config ReceiverConfig) *Receiver {
	if config.MaxFileSize == 0 {
		config.MaxFileSize = DefaultMaxFileSize
	}
	if config.MaxSegments == 0 {
		config.MaxSegments = DefaultMaxSegments
	}
	return &Receiver{
		config:    config,
		fs:        fs,
		state:     StateIdle,
		condition: CondNoError,
	}
}

// header builds a PDU header for a PDU heading back to the sender. The data
// field length is not set here: PDU.Encode computes and checks the real value
// from the data actually supplied, so filling it in here would be redundant
// and, since it truncates to 16 bits, potentially lossy and misleading.
func (r *Receiver) header() *PDUHeader {
	return &PDUHeader{
		IsFileData:   false,
		Direction:    TowardSender,
		Acknowledged: r.config.Acknowledged,
		CRCFlag:      r.config.CRCFlag,
		LargeFile:    r.largeFile,
		// The transaction is still named by its originator, so the source
		// entity ID stays the sender's even on the return path (clause 5.1 note).
		Source:         r.config.Source,
		TransactionSeq: r.config.TransactionSeq,
		Destination:    r.config.Destination,
	}
}

// matchesTransaction reports whether a PDU belongs to this transaction:
// same source entity ID and transaction sequence number (clause 5.1). Values are
// compared numerically, since clause 5.1.7 note 3 zero-pads differing widths. An
// unconfigured Receiver (zero-width IDs) accepts everything, for callers that
// demultiplex upstream.
func (r *Receiver) matchesTransaction(h *PDUHeader) bool {
	if r.config.Source.Width == 0 && r.config.TransactionSeq.Width == 0 {
		return true
	}
	return h.Source.Value == r.config.Source.Value &&
		h.TransactionSeq.Value == r.config.TransactionSeq.Value
}

// recordSegment folds a new byte range into the received set, merging it with
// any neighbours that touch or overlap it. r.received stays sorted by start
// (and, since entries never touch or overlap each other, by end too), so a
// binary search finds the run of neighbours to merge in O(log n), and
// slices.Delete/slices.Insert splice the result in without rebuilding the
// whole slice the way the previous implementation did on every call.
//
// A range that touches or overlaps something already held only ever shrinks
// or holds steady the number of distinct entries. Only a genuinely new,
// separate range grows it -- that is the case MaxSegments bounds, since
// nothing in the standard stops a peer sending only non-adjacent ranges
// (S10) and making this list grow without limit.
func (r *Receiver) recordSegment(start, end uint64) error {
	if end <= start {
		return nil
	}

	// lo is the first existing segment that could touch or overlap
	// [start, end) from the left: the first whose end reaches at least as
	// far as start.
	lo := sort.Search(len(r.received), func(i int) bool {
		return r.received[i].end >= start
	})
	// hi is the first segment past that run: the first, at or after lo,
	// whose start is beyond end.
	hi := lo + sort.Search(len(r.received)-lo, func(i int) bool {
		return r.received[lo+i].start > end
	})

	for _, s := range r.received[lo:hi] {
		if s.start < start {
			start = s.start
		}
		if s.end > end {
			end = s.end
		}
	}

	if hi == lo && len(r.received) >= r.config.MaxSegments {
		return r.fault(CondFilestoreRejection)
	}

	r.received = slices.Delete(r.received, lo, hi)
	r.received = slices.Insert(r.received, lo, segment{start, end})

	if end > r.highWater {
		r.highWater = end
	}
	return nil
}

// missingWithin returns the sub-ranges of [start, end) not yet received, so a
// retransmission that overlaps data already delivered touches the file and
// the checksum only where it brings something new (clause 4.2.1).
func (r *Receiver) missingWithin(start, end uint64) []segment {
	if end <= start {
		return nil
	}
	var out []segment
	cursor := start
	for _, s := range r.received {
		if s.end <= cursor {
			continue
		}
		if s.start >= end {
			break
		}
		if s.start > cursor {
			out = append(out, segment{cursor, s.start})
		}
		cursor = s.end
		if cursor >= end {
			return out
		}
	}
	if cursor < end {
		out = append(out, segment{cursor, end})
	}
	return out
}

// gaps returns the ranges still missing below limit.
func (r *Receiver) gaps(limit uint64) []SegmentRequest {
	var out []SegmentRequest
	var cursor uint64

	for _, s := range r.received {
		if s.start > cursor {
			end := s.start
			if end > limit {
				end = limit
			}
			if end > cursor {
				out = append(out, SegmentRequest{StartOffset: cursor, EndOffset: end})
			}
		}
		if s.end > cursor {
			cursor = s.end
		}
		if cursor >= limit {
			return out
		}
	}
	if cursor < limit {
		out = append(out, SegmentRequest{StartOffset: cursor, EndOffset: limit})
	}
	return out
}

// complete reports whether every octet up to the declared file size has arrived.
func (r *Receiver) complete() bool {
	if !r.eofSeen {
		return false
	}
	if r.declaredSize == 0 {
		return true
	}
	return len(r.received) == 1 && r.received[0].start == 0 && r.received[0].end >= r.declaredSize
}

// handlerFor returns the disposition for a fault condition: a Metadata PDU
// override first, then the configured one, then the table 4-1 default.
func (r *Receiver) handlerFor(cond ConditionCode) FaultHandler {
	if h, ok := r.faultOverrides[cond]; ok {
		return h
	}
	if h, ok := r.config.FaultHandlers[cond]; ok {
		return h
	}
	return DefaultFaultHandler(cond)
}

// fault applies the configured handler for a fault condition (clause 4.8). It
// returns nil when the handler ignores the fault (processing continues) and
// the condition's sentinel error otherwise.
func (r *Receiver) fault(cond ConditionCode) error {
	switch r.handlerFor(cond) {
	case FaultHandlerIgnore:
		return nil
	case FaultHandlerSuspend:
		r.condition = cond
		r.suspended = true
	case FaultHandlerAbandon:
		// Clause 4.11.4: abandonment ends the transaction with no further protocol
		// activity, not even the PDUs already queued.
		r.condition = cond
		r.cancelled = true
		r.pending = nil
		r.state = StateCancelled
	default:
		r.cancelReceive(cond)
	}
	return faultError(cond)
}

// cancelReceive runs the receiver's Notice of Cancellation (clause 4.11.1.2): stop
// NAKing, discard the partial file, and close out with a Finished PDU
// carrying the fault's condition code.
func (r *Receiver) cancelReceive(cond ConditionCode) {
	if r.cancelled || r.state == StateFinished || r.state == StateCancelled {
		return
	}
	r.condition = cond
	r.cancelled = true
	r.early = nil
	r.earlyBytes = 0

	// A NAK already queued must not go out after the cancel.
	kept := r.pending[:0]
	for _, p := range r.pending {
		if code, err := DirectiveCodeOf(p.Data); err == nil && code == DirectiveNAK {
			continue
		}
		kept = append(kept, p)
	}
	r.pending = kept

	status := FileStatusUnreported
	if r.filename != "" && r.fs.Exists(r.filename) {
		_ = r.fs.Delete(r.filename)
		status = FileDiscardedDeliberately
	}
	r.queueFinished(DeliveryDataIncomplete, status)
}

// terminalState is where the transaction ends up once nothing more is owed.
func (r *Receiver) terminalState() TransactionState {
	if r.cancelled {
		return StateCancelled
	}
	return StateFinished
}

// HandlePDU feeds one arriving PDU into the transaction.
func (r *Receiver) HandlePDU(pdu *PDU) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if pdu == nil || pdu.Header == nil {
		return ErrDataTooShort
	}
	// Clause 5.1: a PDU for another transaction is not ours to act on.
	if !r.matchesTransaction(pdu.Header) {
		return nil
	}
	if r.state == StateCancelled {
		return nil
	}
	r.largeFile = pdu.Header.LargeFile

	if pdu.Header.IsFileData {
		return r.handleFileData(pdu)
	}

	code, err := DirectiveCodeOf(pdu.Data)
	if err != nil {
		return err
	}

	switch code {
	case DirectiveMetadata:
		return r.handleMetadata(pdu)
	case DirectiveEOF:
		return r.handleEOF(pdu)
	case DirectiveACK:
		ack, err := DecodeACKPDU(pdu.Data)
		if err != nil {
			return err
		}
		if ack.AckedDirective == DirectiveFinished {
			r.finishedAcked = true
			r.state = r.terminalState()
		}
	case DirectivePrompt:
		prompt, err := DecodePromptPDU(pdu.Data)
		if err != nil {
			return err
		}
		return r.handlePrompt(prompt)
	case DirectiveNAK, DirectiveFinished:
		// Never sent to a receiver.
	}
	return nil
}

// handleMetadata opens the transaction.
func (r *Receiver) handleMetadata(pdu *PDU) error {
	meta, err := DecodeMetadataPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		return err
	}
	if r.metadata != nil || r.cancelled {
		return nil // a retransmission; the first one already opened us
	}

	r.metadata = meta
	if !r.eofSeen {
		// The EOF PDU's file size is authoritative once it has arrived.
		r.declaredSize = meta.FileSize
	}

	// Clause 5.4.4: fault handler overrides apply from the moment they arrive, so
	// they are installed before anything below can raise a fault.
	for _, opt := range meta.Options {
		if opt.Type != TLVFaultHandlerOverride {
			continue
		}
		cond, handler, err := DecodeFaultHandlerOverride(opt)
		if err != nil {
			continue
		}
		if r.faultOverrides == nil {
			r.faultOverrides = make(map[ConditionCode]FaultHandler)
		}
		r.faultOverrides[cond] = handler
	}

	r.filename = r.config.DestinationFileName
	if r.filename == "" {
		r.filename = meta.DestinationFileName.String()
	}

	sum, err := NewChecksum(meta.ChecksumType)
	if err != nil {
		// Clause 4.2.2: fall back to the null checksum and raise the fault. When
		// the handler ignores it, the transfer proceeds unverified and the
		// Finished PDU still reports the condition, as table 5-7 anticipates.
		r.checksum, _ = NewChecksum(ChecksumNull)
		if ferr := r.fault(CondUnsupportedChecksumType); ferr != nil {
			return ferr
		}
		r.condition = CondUnsupportedChecksumType
	} else {
		r.checksum = sum
	}

	if r.filename != "" {
		if err := r.fs.Create(r.filename); err != nil {
			if ferr := r.fault(CondFilestoreRejection); ferr != nil {
				return ferr
			}
			return err
		}
	}

	r.state = StateSendingData

	// Replay the file data that arrived before the metadata made it writable.
	early := r.early
	r.early = nil
	r.earlyBytes = 0
	for _, fd := range early {
		if r.cancelled {
			break
		}
		if err := r.storeFileData(fd); err != nil {
			return err
		}
	}

	// The metadata may have been the last missing piece.
	if r.eofSeen && !r.cancelled {
		r.evaluateCompletion()
	}
	return nil
}

// handleFileData stores one segment, or buffers it when the Metadata PDU has
// not yet arrived to make it writable.
func (r *Receiver) handleFileData(pdu *PDU) error {
	fd, err := DecodeFileDataPDU(pdu.Data, pdu.Header.SegmentMetadataFlag, pdu.Header.LargeFile)
	if err != nil {
		return err
	}
	if len(fd.Data) == 0 || r.cancelled {
		return nil
	}

	// Clause 4.6.1: without metadata there is no filename and no checksum
	// algorithm, so the data cannot be delivered yet. It must not be counted
	// as received either. That would silently drop it from the file. Buffer
	// it and replay once metadata arrives.
	if r.metadata == nil {
		// No clause bounds a file offset, or how much a peer may send before
		// Metadata ever arrives; without a cap here, a single huge offset
		// would be buffered rather than written, and a peer withholding
		// Metadata forever could buffer file data without limit. Apply the
		// same MaxFileSize ceiling here that storeFileData applies once
		// Metadata is in hand.
		if fd.End() < fd.Offset || fd.End() > r.config.MaxFileSize ||
			r.earlyBytes+uint64(len(fd.Data)) > r.config.MaxFileSize {
			if err := r.fault(CondFileSizeError); err != nil {
				return err
			}
			return nil
		}
		r.early = append(r.early, fd)
		r.earlyBytes += uint64(len(fd.Data))
		return nil
	}

	if err := r.storeFileData(fd); err != nil {
		return err
	}

	// Clause 4.6.4: the transaction may just have become complete. The last
	// NAK-recovered segment must trigger the Finished PDU, not wait for
	// another EOF.
	if r.eofSeen && r.complete() {
		r.evaluateCompletion()
	}
	return nil
}

// storeFileData writes one segment, folding into the file and the checksum
// only the sub-ranges not already received, so overlapping retransmissions
// never count twice (clause 4.2.1).
func (r *Receiver) storeFileData(fd *FileDataPDU) error {
	// No clause bounds a file offset; this ceiling is what keeps one crafted
	// PDU from allocating the declared offset in memory. fd.End() wraps
	// around for an offset near 2^64, so an end that comes out before the
	// offset it was computed from is caught the same way as one past the cap.
	if fd.End() < fd.Offset || fd.End() > r.config.MaxFileSize {
		if err := r.fault(CondFileSizeError); err != nil {
			return err
		}
		return nil
	}

	// Clause 4.6.1.2: file data past the size the EOF PDU declared is a file size
	// error.
	if r.eofSeen && fd.End() > r.declaredSize {
		if err := r.fault(CondFileSizeError); err != nil {
			return err
		}
	}

	for _, m := range r.missingWithin(fd.Offset, fd.End()) {
		chunk := fd.Data[m.start-fd.Offset : m.end-fd.Offset]
		if r.filename != "" {
			if err := r.fs.WriteAt(r.filename, m.start, chunk); err != nil {
				if ferr := r.fault(CondFilestoreRejection); ferr != nil {
					return ferr
				}
				return err
			}
		}
		if r.checksum != nil {
			r.checksum.Update(m.start, chunk)
		}
		if err := r.recordSegment(m.start, m.end); err != nil {
			return err
		}
	}
	return nil
}

// handleEOF closes the data stream and decides whether the file is complete.
func (r *Receiver) handleEOF(pdu *PDU) error {
	if r.cancelled {
		return nil
	}
	eof, err := DecodeEOFPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		return err
	}

	r.eofSeen = true
	r.eofChecksum = eof.FileChecksum
	r.declaredSize = eof.FileSize

	// Clause 5.2.4: a Class 2 receiver acknowledges the EOF, cancelled or not.
	if r.config.Acknowledged {
		ack, err := NewACK(DirectiveEOF, eof.ConditionCode, StatusActive)
		if err != nil {
			return err
		}
		body, err := ack.Encode()
		if err != nil {
			return err
		}
		r.pending = append(r.pending, &PDU{Header: r.header(), Data: body})
	}

	// Clause 4.11.2: an EOF with a fault condition code is an EOF (cancel). The
	// transaction ends here, Finished (delivery incomplete) goes back and no
	// more NAKs go out.
	if eof.ConditionCode != CondNoError {
		loc := r.config.Source // the fault happened at the sender
		r.faultLocation = &loc
		r.cancelReceive(eof.ConditionCode)
		return nil
	}

	// Data already received past the declared size is a file size error even
	// though it arrived before the EOF (table 5-5, code '0110').
	if r.highWater > r.declaredSize {
		if err := r.fault(CondFileSizeError); err != nil {
			return err
		}
	}

	r.evaluateCompletion()
	return nil
}

// handlePrompt answers a Prompt PDU with what it asked for (clause 5.2.7).
func (r *Receiver) handlePrompt(p *PromptPDU) error {
	if p.Response == PromptKeepAlive {
		ka := &KeepAlivePDU{Progress: r.progress()}
		body, err := ka.Encode(r.largeFile)
		if err != nil {
			return err
		}
		r.pending = append(r.pending, &PDU{Header: r.header(), Data: body})
		return nil
	}
	return r.queueNAK()
}

// progress returns the contiguous prefix of the file received so far.
func (r *Receiver) progress() uint64 {
	if len(r.received) > 0 && r.received[0].start == 0 {
		return r.received[0].end
	}
	return 0
}

// queueNAK builds a NAK naming everything still missing (clause 5.2.6). A cancelled
// transaction NAKs nothing.
func (r *Receiver) queueNAK() error {
	if r.cancelled {
		return nil
	}

	limit := r.declaredSize
	if !r.eofSeen || limit == 0 {
		limit = r.highWater
	}

	var requests []SegmentRequest
	if r.metadata == nil {
		// A segment request of 0..0 asks for the Metadata PDU (table 5-11).
		requests = append(requests, SegmentRequest{})
	}
	requests = append(requests, r.gaps(limit)...)

	if len(requests) == 0 {
		return nil
	}

	// Each request costs 8 octets (16 in large-file mode; clause 5.1.10), and
	// PDU.Encode refuses a data field over 0xFFFF octets. At roughly 8,000
	// gaps (4,000 large-file) one NAK carrying every gap would not encode --
	// and a NAK is the only way this receiver has to say what is missing, so
	// without a split the transfer would stall behind a PDU it can never
	// send. Clause 5.2.6 provides for exactly this: break the requests into
	// as many NAK PDUs as necessary, each scoped to the range it covers.
	maxPerNAK := maxNAKRequests(r.largeFile, r.config.CRCFlag)

	start := uint64(0)
	for len(requests) > 0 {
		n := min(len(requests), maxPerNAK)
		batch := requests[:n]
		requests = requests[n:]

		// The scope of this batch runs up to where the next batch's first
		// request begins -- or, for the last batch, to the overall limit --
		// so consecutive batches' scopes partition [0, limit) with no gap
		// left undeclared.
		end := limit
		if len(requests) > 0 {
			end = requests[0].StartOffset
		}

		nak := &NAKPDU{StartOfScope: start, EndOfScope: end, Requests: batch}
		body, err := nak.Encode(r.largeFile)
		if err != nil {
			return err
		}
		r.pending = append(r.pending, &PDU{Header: r.header(), Data: body})
		start = end
	}
	return nil
}

// maxNAKRequests returns how many segment requests fit in one NAK PDU's data
// field without PDU.Encode's 0xFFFF ceiling refusing it: the directive code,
// the two scope FSS fields, an optional trailing CRC, and as many
// FSS-pair requests as remain.
func maxNAKRequests(largeFile, crcFlag bool) int {
	fssWidth := 4
	if largeFile {
		fssWidth = 8
	}
	overhead := 1 + 2*fssWidth // directive code + start-of-scope + end-of-scope
	if crcFlag {
		overhead += CRCSize
	}
	reqSize := 2 * fssWidth
	return (0xFFFF - overhead) / reqSize
}

// evaluateCompletion checks the checksum once everything has arrived and
// queues either a Finished PDU or a NAK for what is still missing.
func (r *Receiver) evaluateCompletion() {
	if !r.eofSeen || r.cancelled {
		return
	}

	if !r.complete() {
		if r.config.Acknowledged {
			_ = r.queueNAK()
		}
		return
	}

	// Clause 4.2: verify the checksum before declaring the file delivered. A
	// condition already recorded (an ignored unsupported checksum type, a
	// suspend that was resumed) means the sum is not comparable.
	if r.checksum != nil && r.condition == CondNoError && r.checksum.Sum() != r.eofChecksum {
		if err := r.fault(CondFileChecksumFailure); err != nil {
			return
		}
		// The fault handler said ignore: deliver the file anyway.
	}

	status := FileRetainedSuccessfully
	if r.filename == "" {
		status = FileStatusUnreported
	}

	r.runFilestoreRequests()
	r.queueFinished(DeliveryDataComplete, status)
}

// runFilestoreRequests executes the requests the Metadata PDU carried. Table
// 5-7 requires one response TLV per request.
func (r *Receiver) runFilestoreRequests() {
	if r.metadata == nil || len(r.filestoreResponses) > 0 {
		return
	}
	for _, opt := range r.metadata.Options {
		if opt.Type != TLVFilestoreRequest {
			continue
		}
		req, err := DecodeFilestoreRequest(opt)
		if err != nil {
			continue
		}
		resp := ExecuteFilestoreRequest(r.fs, req)
		tlv, err := resp.Encode()
		if err != nil {
			continue
		}
		r.filestoreResponses = append(r.filestoreResponses, tlv)
	}
}

// queueFinished builds the Finished PDU that closes the transaction.
func (r *Receiver) queueFinished(delivery DeliveryCode, status FileStatus) {
	r.lastDelivery, r.lastStatus = delivery, status
	if r.finishedSent {
		return
	}
	// Clause 5.2.5: a Class 1 transaction sends Finished only when asked.
	if !r.config.Acknowledged && (r.metadata == nil || !r.metadata.ClosureRequested) {
		r.state = r.terminalState()
		return
	}

	fin := &FinishedPDU{
		ConditionCode:      r.condition,
		DeliveryCode:       delivery,
		FileStatus:         status,
		FilestoreResponses: r.filestoreResponses,
	}
	// Table 5-7 omits the fault location for "no error" and for an
	// unsupported checksum type.
	if r.condition != CondNoError && r.condition != CondUnsupportedChecksumType {
		loc := r.config.Destination
		if r.faultLocation != nil {
			loc = *r.faultLocation
		}
		if tlv, err := EntityIDTLV(loc); err == nil {
			fin.FaultLocation = &tlv
		}
	}

	body, err := fin.Encode()
	if err != nil {
		return
	}
	r.pending = append(r.pending, &PDU{Header: r.header(), Data: body})
	r.finishedSent = true

	if r.config.Acknowledged {
		r.state = StateAwaitingFinished // waiting for the ACK of our Finished
	} else {
		r.state = r.terminalState()
	}
}

// NextPDU returns the next PDU to send back, or ok == false when nothing is
// pending. A suspended transaction emits nothing.
func (r *Receiver) NextPDU() (*PDU, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.suspended || len(r.pending) == 0 {
		return nil, false, nil
	}
	pdu := r.pending[0]
	r.pending = r.pending[1:]
	return pdu, true, nil
}

// RequestNAK queues a NAK for whatever is still missing. The caller drives
// this from its own timer, since this library owns no clock.
func (r *Receiver) RequestNAK() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.config.Acknowledged {
		return ErrInvalidTransmissionMode
	}
	return r.queueNAK()
}

// ResendFinished re-queues the Finished PDU when its ACK does not arrive.
func (r *Receiver) ResendFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finishedSent && !r.finishedAcked {
		r.finishedSent = false
		r.queueFinished(r.lastDelivery, r.lastStatus)
	}
}

// ExpireCheckLimit reports that the caller's transaction check timer has
// expired for the last time (clause 4.6.3.3). The caller drives this from its own
// clock, like RequestNAK and ResendEOF. Under the table 4-1 default the
// transaction cancels with condition code "check limit reached", which forces
// the Finished PDU a Class 1 closure-requested transaction still owes.
func (r *Receiver) ExpireCheckLimit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.fault(CondCheckLimitReached)
}

// DeclareFault raises a fault the caller's own timers detected (a NAK limit,
// a keep-alive limit, a positive-ACK limit, or inactivity (table 5-5)) and
// applies the fault handler configured for it, defaulting to the table 4-1
// disposition. The library owns no clock, so counting those limits is the
// caller's job.
func (r *Receiver) DeclareFault(cond ConditionCode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.fault(cond)
}

// Cancel abandons the receive (clause 4.11.1): the partial file is discarded and,
// when the class calls for one, a Finished PDU with condition code "cancel
// request received" closes the transaction out.
func (r *Receiver) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancelReceive(CondCancelRequestReceived)
}

// Suspend stops the transaction emitting PDUs until Resume.
func (r *Receiver) Suspend() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suspended = true
}

// Resume lets a suspended transaction emit again.
func (r *Receiver) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suspended = false
}

// Suspended reports whether the transaction is suspended.
func (r *Receiver) Suspended() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.suspended
}

// State returns the current transaction state.
func (r *Receiver) State() TransactionState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Done reports whether the transaction has completed.
func (r *Receiver) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == StateFinished || r.state == StateCancelled
}

// Complete reports whether every octet of the file has arrived and the EOF
// PDU has been seen.
func (r *Receiver) Complete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.complete()
}

// ConditionCode returns why the transaction ended as it did.
func (r *Receiver) ConditionCode() ConditionCode {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.condition
}

// FileName returns the destination filename in use.
func (r *Receiver) FileName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.filename
}

// Metadata returns the Metadata PDU that opened the transaction, if it has
// arrived.
func (r *Receiver) Metadata() *MetadataPDU {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.metadata
}

// MissingSegments returns the byte ranges still outstanding.
func (r *Receiver) MissingSegments() []SegmentRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	limit := r.declaredSize
	if !r.eofSeen || limit == 0 {
		limit = r.highWater
	}
	return r.gaps(limit)
}
