package cfdp

import "sync"

// segment is one contiguous run of received file data, half-open [start, end).
type segment struct {
	start, end uint64
}

// ReceiverConfig describes one incoming transaction.
type ReceiverConfig struct {
	// Source, Destination and TransactionSeq identify the transaction. They
	// come from the first PDU received.
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
}

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

	metadata  *MetadataPDU
	largeFile bool

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

	// filename is where the file is being written.
	filename string

	// pending holds PDUs waiting to go out.
	pending []*PDU

	// finishedSent and finishedAcked track the close-out handshake.
	finishedSent  bool
	finishedAcked bool

	// filestoreResponses accumulate as requests are executed.
	filestoreResponses []TLV
}

// NewReceiver prepares a transaction to receive one file.
func NewReceiver(fs Filestore, config ReceiverConfig) *Receiver {
	return &Receiver{
		config:    config,
		fs:        fs,
		state:     StateIdle,
		condition: CondNoError,
	}
}

// header builds a PDU header for a PDU heading back to the sender.
func (r *Receiver) header(dataLen int) *PDUHeader {
	return &PDUHeader{
		IsFileData:   false,
		Direction:    TowardSender,
		Acknowledged: r.config.Acknowledged,
		CRCFlag:      r.config.CRCFlag,
		LargeFile:    r.largeFile,
		DataLength:   uint16(dataLen),
		// The transaction is still named by its originator, so the source
		// entity ID stays the sender's even on the return path (§5.1 note).
		Source:         r.config.Source,
		TransactionSeq: r.config.TransactionSeq,
		Destination:    r.config.Destination,
	}
}

// recordSegment folds a new byte range into the received set, merging it with
// any neighbours so the set stays sorted and non-overlapping.
func (r *Receiver) recordSegment(start, end uint64) {
	if end <= start {
		return
	}

	merged := make([]segment, 0, len(r.received)+1)
	added := false

	for _, s := range r.received {
		switch {
		case s.end < start:
			merged = append(merged, s)
		case s.start > end:
			if !added {
				merged = append(merged, segment{start, end})
				added = true
			}
			merged = append(merged, s)
		default:
			// Overlapping or touching: widen the range being inserted.
			if s.start < start {
				start = s.start
			}
			if s.end > end {
				end = s.end
			}
		}
	}
	if !added {
		merged = append(merged, segment{start, end})
	}

	r.received = merged
	if end > r.highWater {
		r.highWater = end
	}
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

// HandlePDU feeds one arriving PDU into the transaction.
func (r *Receiver) HandlePDU(pdu *PDU) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if pdu == nil || pdu.Header == nil {
		return ErrDataTooShort
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
			r.state = StateFinished
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
	if r.metadata != nil {
		return nil // a retransmission; the first one already opened us
	}

	r.metadata = meta
	r.declaredSize = meta.FileSize

	r.filename = r.config.DestinationFileName
	if r.filename == "" {
		r.filename = meta.DestinationFileName.String()
	}

	sum, err := NewChecksum(meta.ChecksumType)
	if err != nil {
		// §5.2.3 has a condition code for exactly this, and table 5-7 says the
		// fault location is omitted for it.
		r.condition = CondUnsupportedChecksumType
		return err
	}
	r.checksum = sum

	if r.filename != "" {
		if err := r.fs.Create(r.filename); err != nil {
			r.condition = CondFilestoreRejection
			return err
		}
	}

	r.state = StateSendingData
	return nil
}

// handleFileData stores one segment and folds it into the checksum.
func (r *Receiver) handleFileData(pdu *PDU) error {
	fd, err := DecodeFileDataPDU(pdu.Data, pdu.Header.SegmentMetadataFlag, pdu.Header.LargeFile)
	if err != nil {
		return err
	}
	if len(fd.Data) == 0 {
		return nil
	}

	// §4.2: data past the declared size is a file size error.
	if r.eofSeen && r.declaredSize > 0 && fd.End() > r.declaredSize {
		r.condition = CondFileSizeError
		return ErrFileSizeError
	}

	// A duplicate segment must not be folded into the checksum twice.
	if r.alreadyHave(fd.Offset, fd.End()) {
		return nil
	}

	if r.filename != "" {
		if err := r.fs.WriteAt(r.filename, fd.Offset, fd.Data); err != nil {
			r.condition = CondFilestoreRejection
			return err
		}
	}
	if r.checksum != nil {
		r.checksum.Update(fd.Offset, fd.Data)
	}
	r.recordSegment(fd.Offset, fd.End())
	return nil
}

// alreadyHave reports whether [start, end) is fully covered already.
func (r *Receiver) alreadyHave(start, end uint64) bool {
	for _, s := range r.received {
		if s.start <= start && s.end >= end {
			return true
		}
	}
	return false
}

// handleEOF closes the data stream and decides whether the file is complete.
func (r *Receiver) handleEOF(pdu *PDU) error {
	eof, err := DecodeEOFPDU(pdu.Data, pdu.Header.LargeFile)
	if err != nil {
		return err
	}

	r.eofSeen = true
	r.eofChecksum = eof.FileChecksum
	r.declaredSize = eof.FileSize
	if eof.ConditionCode != CondNoError {
		r.condition = eof.ConditionCode
	}

	// §5.2.4: a Class 2 receiver acknowledges the EOF.
	if r.config.Acknowledged {
		ack, err := NewACK(DirectiveEOF, eof.ConditionCode, StatusActive)
		if err != nil {
			return err
		}
		body, err := ack.Encode()
		if err != nil {
			return err
		}
		r.pending = append(r.pending, &PDU{Header: r.header(len(body)), Data: body})
	}

	r.evaluateCompletion()
	return nil
}

// handlePrompt answers a Prompt PDU with what it asked for (§5.2.7).
func (r *Receiver) handlePrompt(p *PromptPDU) error {
	if p.Response == PromptKeepAlive {
		ka := &KeepAlivePDU{Progress: r.progress()}
		body, err := ka.Encode(r.largeFile)
		if err != nil {
			return err
		}
		r.pending = append(r.pending, &PDU{Header: r.header(len(body)), Data: body})
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

// queueNAK builds a NAK naming everything still missing (§5.2.6).
func (r *Receiver) queueNAK() error {
	limit := r.declaredSize
	if !r.eofSeen || limit == 0 {
		limit = r.highWater
	}

	nak := &NAKPDU{StartOfScope: 0, EndOfScope: limit}
	if r.metadata == nil {
		// A segment request of 0..0 asks for the Metadata PDU (table 5-11).
		nak.Requests = append(nak.Requests, SegmentRequest{})
	}
	nak.Requests = append(nak.Requests, r.gaps(limit)...)

	if len(nak.Requests) == 0 {
		return nil
	}

	body, err := nak.Encode(r.largeFile)
	if err != nil {
		return err
	}
	r.pending = append(r.pending, &PDU{Header: r.header(len(body)), Data: body})
	return nil
}

// evaluateCompletion checks the checksum once everything has arrived and
// queues either a Finished PDU or a NAK for what is still missing.
func (r *Receiver) evaluateCompletion() {
	if !r.eofSeen {
		return
	}

	if !r.complete() {
		if r.config.Acknowledged {
			_ = r.queueNAK()
		}
		return
	}

	delivery := DeliveryDataComplete
	status := FileRetainedSuccessfully

	// §4.2: verify the checksum before declaring the file delivered.
	if r.checksum != nil && r.condition == CondNoError {
		if r.checksum.Sum() != r.eofChecksum {
			r.condition = CondFileChecksumFailure
			delivery = DeliveryDataIncomplete
			status = FileDiscardedRejection
		}
	}
	if r.condition != CondNoError && r.condition != CondFileChecksumFailure {
		delivery = DeliveryDataIncomplete
		status = FileDiscardedRejection
	}
	if r.filename == "" {
		status = FileStatusUnreported
	}

	r.runFilestoreRequests()
	r.queueFinished(delivery, status)
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
	if r.finishedSent {
		return
	}
	// §5.2.5: a Class 1 transaction sends Finished only when asked.
	if !r.config.Acknowledged && (r.metadata == nil || !r.metadata.ClosureRequested) {
		r.state = StateFinished
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
		if tlv, err := EntityIDTLV(r.config.Destination); err == nil {
			fin.FaultLocation = &tlv
		}
	}

	body, err := fin.Encode()
	if err != nil {
		return
	}
	r.pending = append(r.pending, &PDU{Header: r.header(len(body)), Data: body})
	r.finishedSent = true

	if r.config.Acknowledged {
		r.state = StateAwaitingFinished // waiting for the ACK of our Finished
	} else {
		r.state = StateFinished
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
		r.evaluateCompletion()
	}
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
