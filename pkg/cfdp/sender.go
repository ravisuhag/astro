package cfdp

import (
	"sort"
	"sync"
)

// TransactionState is where a transaction has got to.
type TransactionState int

const (
	// StateIdle means the transaction has not started.
	StateIdle TransactionState = iota
	// StateSendingMetadata means the Metadata PDU is next out.
	StateSendingMetadata
	// StateSendingData means file data is flowing.
	StateSendingData
	// StateSendingEOF means the EOF PDU is next out.
	StateSendingEOF
	// StateAwaitingEOFAck means the sender is waiting for the ACK of its EOF.
	StateAwaitingEOFAck
	// StateAwaitingFinished means the sender is waiting for a Finished PDU.
	StateAwaitingFinished
	// StateFinished means the transaction is complete.
	StateFinished
	// StateCancelled means the transaction was abandoned.
	StateCancelled
)

// String names the state.
func (s TransactionState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateSendingMetadata:
		return "sending metadata"
	case StateSendingData:
		return "sending data"
	case StateSendingEOF:
		return "sending EOF"
	case StateAwaitingEOFAck:
		return "awaiting EOF ACK"
	case StateAwaitingFinished:
		return "awaiting Finished"
	case StateFinished:
		return "finished"
	default:
		return "cancelled"
	}
}

// SenderConfig describes one outgoing transaction.
type SenderConfig struct {
	// Source, Destination and TransactionSeq identify the transaction.
	Source         EntityID
	Destination    EntityID
	TransactionSeq EntityID

	// Acknowledged selects Class 2 (acknowledged) rather than Class 1.
	Acknowledged bool

	// SegmentSize is the largest file data payload per PDU, in octets.
	SegmentSize int

	// SourceFileName and DestinationFileName name the file at each end.
	SourceFileName      string
	DestinationFileName string

	// ChecksumType selects the file checksum algorithm. Zero is the modular
	// checksum every implementation must provide.
	ChecksumType uint8

	// ClosureRequested asks for a Finished PDU in unacknowledged mode. It is
	// ignored in acknowledged mode, where one always comes back.
	ClosureRequested bool

	// CRCFlag adds a CRC to every outgoing PDU.
	CRCFlag bool

	// FilestoreRequests travel in the Metadata PDU for the receiver to run.
	FilestoreRequests []FilestoreRequest

	// MessagesToUser are opaque application messages for the Metadata PDU.
	MessagesToUser [][]byte

	// FaultHandlers overrides the default disposition for the given fault
	// conditions at this entity. Table 4-1 defaults every condition to a
	// Notice of Cancellation (clause 4.8).
	FaultHandlers map[ConditionCode]FaultHandler

	// FaultHandlerOverrides travel as TLVs in the Metadata PDU (clause 5.4.4) and
	// change the receiver's disposition for the named conditions.
	FaultHandlerOverrides map[ConditionCode]FaultHandler
}

// DefaultSegmentSize is the file data payload size used when a SenderConfig
// leaves SegmentSize at zero.
const DefaultSegmentSize = 1024

// Sender drives one outgoing CFDP transaction.
//
// It owns no goroutines and no clock. The caller pumps it: NextPDU returns the
// next PDU to transmit, HandlePDU feeds inbound PDUs back in, and the caller's
// own scheduler decides when to retransmit. This is the same contract as
// pkg/cop's FOP-1.
//
// Usage:
//  1. Create with NewSender
//  2. Call NextPDU repeatedly and transmit what it returns, until it reports
//     that nothing is pending
//  3. Call HandlePDU when a PDU arrives on the return link
//  4. Check Done or State to see when the transaction has completed
//
// A Sender is safe for concurrent use.
type Sender struct {
	mu     sync.Mutex
	config SenderConfig
	fs     Filestore

	state     TransactionState
	suspended bool
	cancelled bool

	fileData  []byte
	fileSize  uint64
	checksum  uint32
	largeFile bool

	// progress is how far the fresh data stream had got when the transaction
	// was cancelled; the EOF (cancel) reports it instead of the full file size
	// (clause 4.11.2.2).
	progress uint64

	// nextOffset is where the next fresh File Data PDU starts.
	nextOffset uint64

	// resend holds segments a NAK asked for again, served before fresh data.
	resend []SegmentRequest

	// metadataResend records a NAK asking for the Metadata PDU (offsets 0..0).
	metadataResend bool

	// eofSent and eofAcked track the close-out handshake.
	eofSent  bool
	eofAcked bool

	// finished holds the Finished PDU once it arrives.
	finished *FinishedPDU

	// condition records why the transaction ended.
	condition ConditionCode
}

// NewSender prepares a transaction to send one file. It reads the source file
// up front and computes its checksum, so the file must be readable now.
//
// A transaction with an empty source filename carries metadata only, which is
// how proxy operations and pure filestore requests travel (clause 5.2.5).
func NewSender(fs Filestore, config SenderConfig) (*Sender, error) {
	if config.SegmentSize <= 0 {
		config.SegmentSize = DefaultSegmentSize
	}

	s := &Sender{
		config:    config,
		fs:        fs,
		state:     StateSendingMetadata,
		condition: CondNoError,
	}

	if config.SourceFileName != "" {
		data, err := fs.Read(config.SourceFileName)
		if err != nil {
			return nil, err
		}
		s.fileData = data
		s.fileSize = uint64(len(data))

		sum, err := NewChecksum(config.ChecksumType)
		if err != nil {
			return nil, err
		}
		sum.Update(0, data)
		s.checksum = sum.Sum()
	} else {
		// No file: the checksum of nothing, per the configured algorithm.
		sum, err := NewChecksum(config.ChecksumType)
		if err != nil {
			return nil, err
		}
		s.checksum = sum.Sum()
	}

	// Clause 5.1.10: files too big for a 32-bit size must be flagged large.
	s.largeFile = s.fileSize > 0xFFFFFFFF

	return s, nil
}

// header builds a PDU header for an outgoing PDU.
func (s *Sender) header(isFileData bool, dataLen int) *PDUHeader {
	return &PDUHeader{
		IsFileData:     isFileData,
		Direction:      TowardReceiver,
		Acknowledged:   s.config.Acknowledged,
		CRCFlag:        s.config.CRCFlag,
		LargeFile:      s.largeFile,
		DataLength:     uint16(dataLen),
		Source:         s.config.Source,
		TransactionSeq: s.config.TransactionSeq,
		Destination:    s.config.Destination,
	}
}

// metadataPDU builds the Metadata PDU that opens the transaction.
func (s *Sender) metadataPDU() (*PDU, error) {
	meta := &MetadataPDU{
		// Table 5-9: the closure request bit is transmitted as '0' in
		// acknowledged mode, where a Finished PDU comes back regardless.
		ClosureRequested:    s.config.ClosureRequested && !s.config.Acknowledged,
		ChecksumType:        s.config.ChecksumType,
		FileSize:            s.fileSize,
		SourceFileName:      LV{Value: []byte(s.config.SourceFileName)},
		DestinationFileName: LV{Value: []byte(s.config.DestinationFileName)},
	}

	for _, req := range s.config.FilestoreRequests {
		tlv, err := req.Encode()
		if err != nil {
			return nil, err
		}
		meta.Options = append(meta.Options, tlv)
	}
	for _, msg := range s.config.MessagesToUser {
		meta.Options = append(meta.Options, TLV{Type: TLVMessageToUser, Value: msg})
	}

	// Fault handler overrides ride in the Metadata PDU (clause 5.4.4), in condition
	// code order so the encoding is deterministic.
	if len(s.config.FaultHandlerOverrides) > 0 {
		conds := make([]ConditionCode, 0, len(s.config.FaultHandlerOverrides))
		for cond := range s.config.FaultHandlerOverrides {
			conds = append(conds, cond)
		}
		sort.Slice(conds, func(i, j int) bool { return conds[i] < conds[j] })
		for _, cond := range conds {
			tlv, err := FaultHandlerOverrideTLV(cond, s.config.FaultHandlerOverrides[cond])
			if err != nil {
				return nil, err
			}
			meta.Options = append(meta.Options, tlv)
		}
	}

	body, err := meta.Encode(s.largeFile)
	if err != nil {
		return nil, err
	}
	return &PDU{Header: s.header(false, len(body)), Data: body}, nil
}

// fileDataPDU builds a File Data PDU for the given range.
func (s *Sender) fileDataPDU(start, end uint64) (*PDU, error) {
	if end > s.fileSize {
		end = s.fileSize
	}
	if start > end {
		start = end
	}

	fd := &FileDataPDU{Offset: start, Data: s.fileData[start:end]}
	body, err := fd.Encode(false, s.largeFile)
	if err != nil {
		return nil, err
	}
	return &PDU{Header: s.header(true, len(body)), Data: body}, nil
}

// eofPDU builds the EOF PDU that closes the file data stream.
func (s *Sender) eofPDU() (*PDU, error) {
	size := s.fileSize
	sum := s.checksum
	if s.cancelled {
		// Clause 4.11.2.2: the EOF (cancel) carries the transaction's progress (
		// how much file data went out) and the checksum of only that much.
		size = s.progress
		if c, err := NewChecksum(s.config.ChecksumType); err == nil {
			c.Update(0, s.fileData[:size])
			sum = c.Sum()
		}
	}

	eof := &EOFPDU{
		ConditionCode: s.condition,
		FileChecksum:  sum,
		FileSize:      size,
	}
	if s.condition != CondNoError {
		tlv, err := EntityIDTLV(s.config.Source)
		if err != nil {
			return nil, err
		}
		eof.FaultLocation = &tlv
	}

	body, err := eof.Encode(s.largeFile)
	if err != nil {
		return nil, err
	}
	return &PDU{Header: s.header(false, len(body)), Data: body}, nil
}

// NextPDU returns the next PDU to transmit, or ok == false when nothing is
// pending right now. A false does not mean the transaction is over: check
// Done for that.
//
// The order is metadata, then any segments a NAK asked for again, then fresh
// file data, then EOF. A suspended transaction emits nothing.
func (s *Sender) NextPDU() (*PDU, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.suspended || s.state == StateFinished || s.state == StateCancelled {
		return nil, false, nil
	}

	// A NAK can ask for the Metadata PDU again (a segment request of 0..0).
	if s.metadataResend {
		s.metadataResend = false
		pdu, err := s.metadataPDU()
		return pdu, err == nil, err
	}

	if s.state == StateSendingMetadata {
		pdu, err := s.metadataPDU()
		if err != nil {
			return nil, false, err
		}
		s.state = StateSendingData
		return pdu, true, nil
	}

	// Retransmissions come before fresh data so gaps close promptly.
	if len(s.resend) > 0 {
		req := s.resend[0]
		s.resend = s.resend[1:]
		pdu, err := s.fileDataPDU(req.StartOffset, req.EndOffset)
		return pdu, err == nil, err
	}

	if s.state == StateSendingData {
		if s.nextOffset < s.fileSize {
			start := s.nextOffset
			end := start + uint64(s.config.SegmentSize)
			if end > s.fileSize {
				end = s.fileSize
			}
			s.nextOffset = end
			pdu, err := s.fileDataPDU(start, end)
			return pdu, err == nil, err
		}
		s.state = StateSendingEOF
	}

	if s.state == StateSendingEOF {
		pdu, err := s.eofPDU()
		if err != nil {
			return nil, false, err
		}
		s.eofSent = true

		switch {
		case s.config.Acknowledged:
			s.state = StateAwaitingEOFAck
		case s.config.ClosureRequested:
			s.state = StateAwaitingFinished
		default:
			// Class 1 with no closure: nothing comes back, so we are done.
			s.state = s.terminalState()
		}
		return pdu, true, nil
	}

	return nil, false, nil
}

// ResendEOF re-queues the EOF PDU. The caller invokes this from its own timer
// when an EOF ACK does not arrive; clause 4.2 leaves the timing to the
// implementation and this library owns no clock.
func (s *Sender) ResendEOF() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateAwaitingEOFAck || s.state == StateAwaitingFinished {
		s.state = StateSendingEOF
	}
}

// HandlePDU feeds a PDU arriving on the return link into the transaction.
//
// The sender expects NAK, ACK of its EOF, Finished, Keep Alive and Prompt.
// Anything else is ignored rather than treated as an error, since a shared
// link may carry PDUs for other transactions.
func (s *Sender) HandlePDU(pdu *PDU) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pdu == nil || pdu.Header == nil {
		return ErrDataTooShort
	}
	// Clause 5.1: a PDU for another transaction is not ours to act on.
	if !s.matchesTransaction(pdu.Header) {
		return nil
	}
	if pdu.Header.IsFileData {
		return nil // the sender has no use for inbound file data
	}

	code, err := DirectiveCodeOf(pdu.Data)
	if err != nil {
		return err
	}

	switch code {
	case DirectiveNAK:
		nak, err := DecodeNAKPDU(pdu.Data, pdu.Header.LargeFile)
		if err != nil {
			return err
		}
		for _, req := range nak.Requests {
			if req.IsMetadataRequest() {
				s.metadataResend = true
				continue
			}
			s.resend = append(s.resend, req)
		}
		// Fresh data may already be exhausted; NextPDU serves the resend queue
		// regardless of state, so nothing else is needed here.

	case DirectiveACK:
		ack, err := DecodeACKPDU(pdu.Data)
		if err != nil {
			return err
		}
		if ack.AckedDirective == DirectiveEOF && s.eofSent {
			s.eofAcked = true
			if s.state == StateAwaitingEOFAck {
				s.state = StateAwaitingFinished
			}
		}

	case DirectiveFinished:
		fin, err := DecodeFinishedPDU(pdu.Data)
		if err != nil {
			return err
		}
		s.finished = fin
		// Clause 4.11.1: a Finished with a fault condition is the receiver
		// cancelling the transaction; stop sending file data.
		if fin.ConditionCode != CondNoError {
			s.condition = fin.ConditionCode
			s.cancelled = true
			s.resend = nil
			s.metadataResend = false
		}
		s.state = s.terminalState()

	case DirectiveKeepAlive, DirectivePrompt:
		// Informational for this implementation; clause 4.6 leaves the reaction to
		// flow-control policy the caller owns.

	default:
		// Metadata, EOF and File Data are never sent to a sender.
	}
	return nil
}

// AckFinished builds the ACK a Class 2 sender owes for a Finished PDU
// (clause 5.2.4). Returns ok == false when no Finished PDU has arrived.
func (s *Sender) AckFinished() (*PDU, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished == nil || !s.config.Acknowledged {
		return nil, false, nil
	}

	ack, err := NewACK(DirectiveFinished, s.finished.ConditionCode, StatusTerminated)
	if err != nil {
		return nil, false, err
	}
	body, err := ack.Encode()
	if err != nil {
		return nil, false, err
	}
	return &PDU{Header: s.header(false, len(body)), Data: body}, true, nil
}

// Suspend stops the transaction emitting PDUs until Resume. Because the caller
// owns the clock, a suspended transaction simply goes quiet.
func (s *Sender) Suspend() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suspended = true
}

// Resume lets a suspended transaction emit again.
func (s *Sender) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suspended = false
}

// Suspended reports whether the transaction is suspended.
func (s *Sender) Suspended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.suspended
}

// Cancel cancels the transaction (clause 4.11.1). The next PDU out is an EOF
// carrying the cancel condition code of table 5-5 and the transaction's
// progress so far.
func (s *Sender) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelWith(CondCancelRequestReceived)
}

// DeclareFault raises a fault the caller's own timers detected (the positive
// ACK limit of clause 4.7, a keep-alive limit, or inactivity (table 5-5)) and
// applies the fault handler configured for it, defaulting to the table 4-1
// disposition (cancel). The library owns no clock, so counting those limits
// is the caller's job.
func (s *Sender) DeclareFault(cond ConditionCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fault(cond)
}

// handlerFor returns the disposition for a fault condition: the configured
// one, or the table 4-1 default.
func (s *Sender) handlerFor(cond ConditionCode) FaultHandler {
	if h, ok := s.config.FaultHandlers[cond]; ok {
		return h
	}
	return DefaultFaultHandler(cond)
}

// fault applies the configured handler for a fault condition (clause 4.8).
func (s *Sender) fault(cond ConditionCode) {
	switch s.handlerFor(cond) {
	case FaultHandlerIgnore:
	case FaultHandlerSuspend:
		s.suspended = true
	case FaultHandlerAbandon:
		// Clause 4.11.4: abandonment ends the transaction with no further protocol
		// activity, no EOF (cancel) goes out.
		s.condition = cond
		s.cancelled = true
		s.state = StateCancelled
	default:
		s.cancelWith(cond)
	}
}

// cancelWith runs the sender's Notice of Cancellation (clause 4.11.2): retransmission
// stops and an EOF (cancel) carrying the fault's condition code goes out.
func (s *Sender) cancelWith(cond ConditionCode) {
	if s.state == StateFinished || s.state == StateCancelled {
		return
	}
	s.condition = cond
	s.cancelled = true
	s.progress = s.nextOffset
	s.state = StateSendingEOF
	s.resend = nil
	s.metadataResend = false
}

// terminalState is where the transaction ends up once nothing more is owed.
func (s *Sender) terminalState() TransactionState {
	if s.cancelled {
		return StateCancelled
	}
	return StateFinished
}

// matchesTransaction reports whether a PDU belongs to this transaction: same
// source entity ID and transaction sequence number (clause 5.1). Values compare
// numerically, since clause 5.1.7 note 3 zero-pads differing widths.
func (s *Sender) matchesTransaction(h *PDUHeader) bool {
	if s.config.Source.Width == 0 && s.config.TransactionSeq.Width == 0 {
		return true
	}
	return h.Source.Value == s.config.Source.Value &&
		h.TransactionSeq.Value == s.config.TransactionSeq.Value
}

// State returns the current transaction state.
func (s *Sender) State() TransactionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Done reports whether the transaction has completed or been cancelled.
func (s *Sender) Done() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == StateFinished || s.state == StateCancelled
}

// Finished returns the Finished PDU the receiver sent, if any.
func (s *Sender) Finished() *FinishedPDU {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

// Checksum returns the checksum computed over the source file.
func (s *Sender) Checksum() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checksum
}
