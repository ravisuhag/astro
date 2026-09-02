package cfdp

import "fmt"

// DirectiveCode identifies a file directive, per CCSDS 727.0-B-5 table 5-4.
type DirectiveCode uint8

const (
	// DirectiveEOF closes out the file data stream (§5.2.2).
	DirectiveEOF DirectiveCode = 0x04
	// DirectiveFinished reports delivery at the receiver (§5.2.3).
	DirectiveFinished DirectiveCode = 0x05
	// DirectiveACK acknowledges an EOF or Finished PDU (§5.2.4).
	DirectiveACK DirectiveCode = 0x06
	// DirectiveMetadata opens a transaction (§5.2.5).
	DirectiveMetadata DirectiveCode = 0x07
	// DirectiveNAK names the file segments still missing (§5.2.6).
	DirectiveNAK DirectiveCode = 0x08
	// DirectivePrompt asks the far end for a NAK or Keep Alive (§5.2.7).
	DirectivePrompt DirectiveCode = 0x09
	// DirectiveKeepAlive reports the receiver's progress (§5.2.8).
	DirectiveKeepAlive DirectiveCode = 0x0C
)

// String names the directive.
func (d DirectiveCode) String() string {
	switch d {
	case DirectiveEOF:
		return "EOF"
	case DirectiveFinished:
		return "Finished"
	case DirectiveACK:
		return "ACK"
	case DirectiveMetadata:
		return "Metadata"
	case DirectiveNAK:
		return "NAK"
	case DirectivePrompt:
		return "Prompt"
	case DirectiveKeepAlive:
		return "Keep Alive"
	default:
		return fmt.Sprintf("reserved(%#02x)", uint8(d))
	}
}

// Valid reports whether the code is one of the seven defined directives.
// Table 5-4 reserves 00-03 and 0D-FF, and leaves 0A-0B undefined.
func (d DirectiveCode) Valid() bool {
	switch d {
	case DirectiveEOF, DirectiveFinished, DirectiveACK, DirectiveMetadata,
		DirectiveNAK, DirectivePrompt, DirectiveKeepAlive:
		return true
	default:
		return false
	}
}

// ConditionCode reports why a transaction ended as it did, per table 5-5.
type ConditionCode uint8

const (
	CondNoError                 ConditionCode = 0x0
	CondPositiveACKLimitReached ConditionCode = 0x1
	CondKeepAliveLimitReached   ConditionCode = 0x2
	CondInvalidTransmissionMode ConditionCode = 0x3
	CondFilestoreRejection      ConditionCode = 0x4
	CondFileChecksumFailure     ConditionCode = 0x5
	CondFileSizeError           ConditionCode = 0x6
	CondNAKLimitReached         ConditionCode = 0x7
	CondInactivityDetected      ConditionCode = 0x8
	CondInvalidFileStructure    ConditionCode = 0x9
	CondCheckLimitReached       ConditionCode = 0xA
	CondUnsupportedChecksumType ConditionCode = 0xB
	CondSuspendRequestReceived  ConditionCode = 0xE
	CondCancelRequestReceived   ConditionCode = 0xF
)

// String names the condition.
func (c ConditionCode) String() string {
	switch c {
	case CondNoError:
		return "no error"
	case CondPositiveACKLimitReached:
		return "positive ACK limit reached"
	case CondKeepAliveLimitReached:
		return "keep alive limit reached"
	case CondInvalidTransmissionMode:
		return "invalid transmission mode"
	case CondFilestoreRejection:
		return "filestore rejection"
	case CondFileChecksumFailure:
		return "file checksum failure"
	case CondFileSizeError:
		return "file size error"
	case CondNAKLimitReached:
		return "NAK limit reached"
	case CondInactivityDetected:
		return "inactivity detected"
	case CondInvalidFileStructure:
		return "invalid file structure"
	case CondCheckLimitReached:
		return "check limit reached"
	case CondUnsupportedChecksumType:
		return "unsupported checksum type"
	case CondSuspendRequestReceived:
		return "suspend request received"
	case CondCancelRequestReceived:
		return "cancel request received"
	default:
		return fmt.Sprintf("reserved(%#x)", uint8(c))
	}
}

// directiveBody strips the directive code octet off a File Directive PDU data
// field and checks it is the one expected (§5.2.1.1).
func directiveBody(data []byte, want DirectiveCode) ([]byte, error) {
	if len(data) < 1 {
		return nil, ErrDataTooShort
	}
	got := DirectiveCode(data[0])
	if !got.Valid() {
		return nil, ErrInvalidDirectiveCode
	}
	if got != want {
		return nil, ErrWrongDirectiveCode
	}
	return data[1:], nil
}

// DirectiveCodeOf returns the directive code of a File Directive PDU data field.
func DirectiveCodeOf(data []byte) (DirectiveCode, error) {
	if len(data) < 1 {
		return 0, ErrDataTooShort
	}
	code := DirectiveCode(data[0])
	if !code.Valid() {
		return 0, ErrInvalidDirectiveCode
	}
	return code, nil
}

// --- EOF PDU, table 5-6 ---

// EOFPDU closes the file data stream and carries the checksum the receiver
// verifies against.
type EOFPDU struct {
	ConditionCode ConditionCode
	FileChecksum  uint32
	FileSize      uint64 // FSS
	// FaultLocation is an entity ID TLV, present only when ConditionCode is
	// not "no error".
	FaultLocation *TLV
}

// Encode serializes the EOF PDU data field, directive code included.
func (p *EOFPDU) Encode(largeFile bool) ([]byte, error) {
	out := []byte{byte(DirectiveEOF)}
	// Condition code (4 bits) then 4 spare bits, all zeros.
	out = append(out, byte(p.ConditionCode&0x0F)<<4)
	out = append(out, byte(p.FileChecksum>>24), byte(p.FileChecksum>>16),
		byte(p.FileChecksum>>8), byte(p.FileChecksum))
	out = appendFSS(out, p.FileSize, largeFile)

	if p.ConditionCode != CondNoError && p.FaultLocation != nil {
		b, err := p.FaultLocation.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// DecodeEOFPDU parses an EOF PDU data field.
func DecodeEOFPDU(data []byte, largeFile bool) (*EOFPDU, error) {
	body, err := directiveBody(data, DirectiveEOF)
	if err != nil {
		return nil, err
	}
	if len(body) < 5 {
		return nil, ErrDataTooShort
	}

	p := &EOFPDU{ConditionCode: ConditionCode(body[0] >> 4)}
	p.FileChecksum = uint32(body[1])<<24 | uint32(body[2])<<16 | uint32(body[3])<<8 | uint32(body[4])

	size, n, err := readFSS(body[5:], largeFile)
	if err != nil {
		return nil, err
	}
	p.FileSize = size
	offset := 5 + n

	if offset < len(body) {
		tlv, _, err := DecodeTLV(body[offset:])
		if err != nil {
			return nil, err
		}
		p.FaultLocation = &tlv
	}
	return p, nil
}

// Humanize returns a human-readable summary.
func (p *EOFPDU) Humanize() string {
	return fmt.Sprintf("CFDP EOF PDU\n  Condition ... %s\n  Checksum .... %#08x\n  File size ... %d",
		p.ConditionCode, p.FileChecksum, p.FileSize)
}

// --- Finished PDU, table 5-7 ---

// DeliveryCode says whether the receiver got everything (§5.2.3).
type DeliveryCode uint8

const (
	// DeliveryDataComplete means metadata, all file data and EOF arrived and
	// the checksum verified.
	DeliveryDataComplete DeliveryCode = 0
	// DeliveryDataIncomplete means something is still missing.
	DeliveryDataIncomplete DeliveryCode = 1
)

// String names the delivery code.
func (d DeliveryCode) String() string {
	if d == DeliveryDataComplete {
		return "data complete"
	}
	return "data incomplete"
}

// FileStatus reports what became of the delivered file (§5.2.3).
type FileStatus uint8

const (
	FileDiscardedDeliberately FileStatus = 0
	FileDiscardedRejection    FileStatus = 1
	FileRetainedSuccessfully  FileStatus = 2
	FileStatusUnreported      FileStatus = 3
)

// String names the file status.
func (f FileStatus) String() string {
	switch f {
	case FileDiscardedDeliberately:
		return "discarded deliberately"
	case FileDiscardedRejection:
		return "discarded due to filestore rejection"
	case FileRetainedSuccessfully:
		return "retained successfully"
	default:
		return "unreported"
	}
}

// FinishedPDU reports the outcome of a transaction at the receiver.
type FinishedPDU struct {
	ConditionCode      ConditionCode
	DeliveryCode       DeliveryCode
	FileStatus         FileStatus
	FilestoreResponses []TLV
	FaultLocation      *TLV
}

// Encode serializes the Finished PDU data field.
func (p *FinishedPDU) Encode() ([]byte, error) {
	out := []byte{byte(DirectiveFinished)}
	// Condition code (4) | spare (1) | delivery code (1) | file status (2).
	b := byte(p.ConditionCode&0x0F) << 4
	b |= byte(p.DeliveryCode&0x01) << 2
	b |= byte(p.FileStatus & 0x03)
	out = append(out, b)

	responses, err := encodeTLVs(p.FilestoreResponses)
	if err != nil {
		return nil, err
	}
	out = append(out, responses...)

	// §5.2.3: omitted for "no error" and for "unsupported checksum type".
	if p.ConditionCode != CondNoError && p.ConditionCode != CondUnsupportedChecksumType && p.FaultLocation != nil {
		fl, err := p.FaultLocation.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, fl...)
	}
	return out, nil
}

// DecodeFinishedPDU parses a Finished PDU data field.
//
// The trailing TLVs are all filestore responses except a final entity ID TLV,
// which is the fault location. Table 5-7 distinguishes them by type, not by
// position, so this splits on the type code.
func DecodeFinishedPDU(data []byte) (*FinishedPDU, error) {
	body, err := directiveBody(data, DirectiveFinished)
	if err != nil {
		return nil, err
	}
	if len(body) < 1 {
		return nil, ErrDataTooShort
	}

	p := &FinishedPDU{
		ConditionCode: ConditionCode(body[0] >> 4),
		DeliveryCode:  DeliveryCode(body[0] >> 2 & 0x01),
		FileStatus:    FileStatus(body[0] & 0x03),
	}

	tlvs, err := DecodeTLVs(body[1:])
	if err != nil {
		return nil, err
	}
	for i := range tlvs {
		if tlvs[i].Type == TLVEntityID {
			t := tlvs[i]
			p.FaultLocation = &t
			continue
		}
		p.FilestoreResponses = append(p.FilestoreResponses, tlvs[i])
	}
	return p, nil
}

// Humanize returns a human-readable summary.
func (p *FinishedPDU) Humanize() string {
	complete := "incomplete"
	if p.DeliveryCode == DeliveryDataComplete {
		complete = "complete"
	}
	return fmt.Sprintf("CFDP Finished PDU\n  Condition ... %s\n  Delivery .... data %s\n  File ........ %s",
		p.ConditionCode, complete, p.FileStatus)
}

// --- ACK PDU, table 5-8 ---

// TransactionStatus is the acknowledging entity's view of the transaction (§5.2.4).
type TransactionStatus uint8

const (
	StatusUndefined    TransactionStatus = 0
	StatusActive       TransactionStatus = 1
	StatusTerminated   TransactionStatus = 2
	StatusUnrecognized TransactionStatus = 3
)

// String names the transaction status.
func (s TransactionStatus) String() string {
	switch s {
	case StatusUndefined:
		return "undefined"
	case StatusActive:
		return "active"
	case StatusTerminated:
		return "terminated"
	default:
		return "unrecognized"
	}
}

// ACKPDU acknowledges an EOF or Finished PDU. Table 5-8 allows no others.
type ACKPDU struct {
	// AckedDirective is the directive being acknowledged: EOF or Finished.
	AckedDirective DirectiveCode
	// DirectiveSubtype is binary '0001' when acknowledging a Finished PDU and
	// '0000' for everything else (§5.2.4).
	DirectiveSubtype uint8
	// ConditionCode is the condition code of the acknowledged PDU.
	ConditionCode     ConditionCode
	TransactionStatus TransactionStatus
}

// NewACK builds an ACK for one directive, filling in the subtype table 5-8 requires.
func NewACK(acked DirectiveCode, cond ConditionCode, status TransactionStatus) (*ACKPDU, error) {
	if acked != DirectiveEOF && acked != DirectiveFinished {
		return nil, ErrWrongDirectiveCode
	}
	subtype := uint8(0)
	if acked == DirectiveFinished {
		subtype = 1
	}
	return &ACKPDU{
		AckedDirective:    acked,
		DirectiveSubtype:  subtype,
		ConditionCode:     cond,
		TransactionStatus: status,
	}, nil
}

// Encode serializes the ACK PDU data field.
func (p *ACKPDU) Encode() ([]byte, error) {
	if p.AckedDirective != DirectiveEOF && p.AckedDirective != DirectiveFinished {
		return nil, ErrWrongDirectiveCode
	}
	out := []byte{byte(DirectiveACK)}
	out = append(out, byte(p.AckedDirective&0x0F)<<4|byte(p.DirectiveSubtype&0x0F))
	// Condition code (4) | spare (2) | transaction status (2).
	out = append(out, byte(p.ConditionCode&0x0F)<<4|byte(p.TransactionStatus&0x03))
	return out, nil
}

// DecodeACKPDU parses an ACK PDU data field.
func DecodeACKPDU(data []byte) (*ACKPDU, error) {
	body, err := directiveBody(data, DirectiveACK)
	if err != nil {
		return nil, err
	}
	if len(body) < 2 {
		return nil, ErrDataTooShort
	}
	acked := DirectiveCode(body[0] >> 4)
	if acked != DirectiveEOF && acked != DirectiveFinished {
		return nil, ErrWrongDirectiveCode
	}
	return &ACKPDU{
		AckedDirective:    acked,
		DirectiveSubtype:  body[0] & 0x0F,
		ConditionCode:     ConditionCode(body[1] >> 4),
		TransactionStatus: TransactionStatus(body[1] & 0x03),
	}, nil
}

// Humanize returns a human-readable summary.
func (p *ACKPDU) Humanize() string {
	return fmt.Sprintf("CFDP ACK PDU\n  Acknowledges . %s\n  Condition .... %s\n  Status ....... %s",
		p.AckedDirective, p.ConditionCode, p.TransactionStatus)
}

// --- Metadata PDU, table 5-9 ---

// MetadataPDU opens a transaction and describes the file being sent.
type MetadataPDU struct {
	// ClosureRequested asks the receiver for a Finished PDU. Table 5-9 says
	// set it to '0' and ignore it in acknowledged mode, where a Finished PDU
	// is sent regardless.
	ClosureRequested bool
	// ChecksumType selects the algorithm, per the SANA registry. Zero is the
	// legacy modular checksum.
	ChecksumType uint8
	// FileSize is the file length in octets, or zero for unbounded size.
	FileSize uint64 // FSS
	// SourceFileName and DestinationFileName are empty when the transaction
	// carries no file (metadata-only, as used by proxy operations).
	SourceFileName      LV
	DestinationFileName LV
	// Options are filestore requests, messages to user, fault handler
	// overrides and flow labels.
	Options []TLV
}

// Encode serializes the Metadata PDU data field.
func (p *MetadataPDU) Encode(largeFile bool) ([]byte, error) {
	out := []byte{byte(DirectiveMetadata)}

	// Reserved (1) | closure requested (1) | reserved (2) | checksum type (4).
	b := byte(0)
	if p.ClosureRequested {
		b |= 1 << 6
	}
	b |= p.ChecksumType & 0x0F
	out = append(out, b)

	out = appendFSS(out, p.FileSize, largeFile)

	src, err := p.SourceFileName.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, src...)

	dst, err := p.DestinationFileName.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, dst...)

	opts, err := encodeTLVs(p.Options)
	if err != nil {
		return nil, err
	}
	return append(out, opts...), nil
}

// DecodeMetadataPDU parses a Metadata PDU data field.
func DecodeMetadataPDU(data []byte, largeFile bool) (*MetadataPDU, error) {
	body, err := directiveBody(data, DirectiveMetadata)
	if err != nil {
		return nil, err
	}
	if len(body) < 1 {
		return nil, ErrDataTooShort
	}

	p := &MetadataPDU{
		ClosureRequested: body[0]&(1<<6) != 0,
		ChecksumType:     body[0] & 0x0F,
	}

	size, n, err := readFSS(body[1:], largeFile)
	if err != nil {
		return nil, err
	}
	p.FileSize = size
	offset := 1 + n

	src, n, err := DecodeLV(body[offset:])
	if err != nil {
		return nil, err
	}
	p.SourceFileName = src
	offset += n

	dst, n, err := DecodeLV(body[offset:])
	if err != nil {
		return nil, err
	}
	p.DestinationFileName = dst
	offset += n

	if offset < len(body) {
		if p.Options, err = DecodeTLVs(body[offset:]); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Humanize returns a human-readable summary.
func (p *MetadataPDU) Humanize() string {
	return fmt.Sprintf("CFDP Metadata PDU\n  Closure ....... %t\n  Checksum type . %d\n  File size ..... %d\n  Source ........ %q\n  Destination ... %q\n  Options ....... %d",
		p.ClosureRequested, p.ChecksumType, p.FileSize,
		p.SourceFileName.String(), p.DestinationFileName.String(), len(p.Options))
}

// --- NAK PDU, tables 5-10 and 5-11 ---

// SegmentRequest names one range of file data the receiver still needs.
// A request of 0..0 asks for the metadata (table 5-11).
type SegmentRequest struct {
	StartOffset uint64 // FSS
	EndOffset   uint64 // FSS, first octet after the requested segment
}

// IsMetadataRequest reports whether this request asks for the Metadata PDU
// rather than a run of file data.
func (s SegmentRequest) IsMetadataRequest() bool {
	return s.StartOffset == 0 && s.EndOffset == 0
}

// NAKPDU lists the gaps the receiver is still missing.
type NAKPDU struct {
	StartOfScope uint64 // FSS
	EndOfScope   uint64 // FSS
	Requests     []SegmentRequest
}

// Encode serializes the NAK PDU data field.
func (p *NAKPDU) Encode(largeFile bool) ([]byte, error) {
	out := []byte{byte(DirectiveNAK)}
	out = appendFSS(out, p.StartOfScope, largeFile)
	out = appendFSS(out, p.EndOfScope, largeFile)
	for _, r := range p.Requests {
		out = appendFSS(out, r.StartOffset, largeFile)
		out = appendFSS(out, r.EndOffset, largeFile)
	}
	return out, nil
}

// DecodeNAKPDU parses a NAK PDU data field.
func DecodeNAKPDU(data []byte, largeFile bool) (*NAKPDU, error) {
	body, err := directiveBody(data, DirectiveNAK)
	if err != nil {
		return nil, err
	}

	p := &NAKPDU{}
	offset := 0

	start, n, err := readFSS(body[offset:], largeFile)
	if err != nil {
		return nil, err
	}
	p.StartOfScope = start
	offset += n

	end, n, err := readFSS(body[offset:], largeFile)
	if err != nil {
		return nil, err
	}
	p.EndOfScope = end
	offset += n

	for offset < len(body) {
		s, n, err := readFSS(body[offset:], largeFile)
		if err != nil {
			return nil, err
		}
		offset += n
		e, n, err := readFSS(body[offset:], largeFile)
		if err != nil {
			return nil, err
		}
		offset += n
		p.Requests = append(p.Requests, SegmentRequest{StartOffset: s, EndOffset: e})
	}
	return p, nil
}

// Humanize returns a human-readable summary.
func (p *NAKPDU) Humanize() string {
	return fmt.Sprintf("CFDP NAK PDU\n  Scope ....... %d to %d\n  Requests .... %d",
		p.StartOfScope, p.EndOfScope, len(p.Requests))
}

// --- Prompt PDU, table 5-12 ---

// PromptResponse selects what the far end should send back (§5.2.7).
type PromptResponse uint8

const (
	// PromptNAK asks for a NAK PDU.
	PromptNAK PromptResponse = 0
	// PromptKeepAlive asks for a Keep Alive PDU.
	PromptKeepAlive PromptResponse = 1
)

// PromptPDU asks the far end for a NAK or a Keep Alive.
type PromptPDU struct {
	Response PromptResponse
}

// Encode serializes the Prompt PDU data field.
func (p *PromptPDU) Encode() ([]byte, error) {
	// Response (1 bit) then 7 spare bits.
	return []byte{byte(DirectivePrompt), byte(p.Response&0x01) << 7}, nil
}

// DecodePromptPDU parses a Prompt PDU data field.
func DecodePromptPDU(data []byte) (*PromptPDU, error) {
	body, err := directiveBody(data, DirectivePrompt)
	if err != nil {
		return nil, err
	}
	if len(body) < 1 {
		return nil, ErrDataTooShort
	}
	return &PromptPDU{Response: PromptResponse(body[0] >> 7 & 0x01)}, nil
}

// Humanize returns a human-readable summary.
func (p *PromptPDU) Humanize() string {
	want := "NAK"
	if p.Response == PromptKeepAlive {
		want = "Keep Alive"
	}
	return "CFDP Prompt PDU\n  Response required ... " + want
}

// --- Keep Alive PDU, table 5-13 ---

// KeepAlivePDU reports how much of the file the receiver has.
type KeepAlivePDU struct {
	Progress uint64 // FSS, offset from the start of the file
}

// Encode serializes the Keep Alive PDU data field.
func (p *KeepAlivePDU) Encode(largeFile bool) ([]byte, error) {
	out := []byte{byte(DirectiveKeepAlive)}
	return appendFSS(out, p.Progress, largeFile), nil
}

// DecodeKeepAlivePDU parses a Keep Alive PDU data field.
func DecodeKeepAlivePDU(data []byte, largeFile bool) (*KeepAlivePDU, error) {
	body, err := directiveBody(data, DirectiveKeepAlive)
	if err != nil {
		return nil, err
	}
	progress, _, err := readFSS(body, largeFile)
	if err != nil {
		return nil, err
	}
	return &KeepAlivePDU{Progress: progress}, nil
}

// Humanize returns a human-readable summary.
func (p *KeepAlivePDU) Humanize() string {
	return fmt.Sprintf("CFDP Keep Alive PDU\n  Progress ... %d octets", p.Progress)
}
