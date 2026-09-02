package cfdp

import "fmt"

// Part 2: the User Operations, from CCSDS 727.0-B-5 section 6.
//
// Part 1 of the standard moves a file when told to. Part 2 is how one CFDP
// user asks another to do something on its behalf: fetch a file from a third
// entity, list a directory, report on a transaction, suspend or resume one.
//
// None of it is a new PDU. Every operation travels as a Reserved CFDP Message
// inside a Message to User TLV in an ordinary transaction's metadata (§6.1.1),
// so the wire format underneath is what Part 1 already built. What Part 2 adds
// is the content of those messages.
//
// A Reserved CFDP Message is the four ASCII characters "cfdp", a one-octet
// message type, then content whose shape the type decides (§6.1.2, table 6-1).
// That header is what tells a receiver a Message to User is a protocol
// message rather than something for the application.
//
// What is here is the message formats: building them and reading them back.
// What is not is the user behaviour around them — which primitive to call on
// receipt, how to queue concurrent suspension orders (§6.5.4.1.2) — because
// the standard makes that the CFDP user's job and says so: "the manner in
// which this is accomplished is an implementation matter".

// MessageMagic is the message identifier every Reserved CFDP Message opens
// with: the ASCII characters "cfdp" (§6.1.2, table 6-1).
//
// It is what distinguishes a protocol message from an application one, since
// both travel in a Message to User TLV.
var MessageMagic = [4]byte{'c', 'f', 'd', 'p'}

// MessageMagicSize is the width of the message identifier in octets.
const MessageMagicSize = 4

// UserMessageType identifies a Reserved CFDP Message, per the message type
// tables of section 6.
type UserMessageType uint8

// The message types, from tables 6-3, 6-14, 6-17, 6-20 and 6-23.
//
// The numbering is grouped by operation with gaps between groups, and one gap
// inside the proxy group: 0x0A is the Originating Transaction ID, which is
// common to every operation rather than belonging to proxy, so the proxy
// group runs 0x00 to 0x09 and then resumes at 0x0B.
const (
	// Proxy operations, table 6-3.
	MsgProxyPutRequest           UserMessageType = 0x00
	MsgProxyMessageToUser        UserMessageType = 0x01
	MsgProxyFilestoreRequest     UserMessageType = 0x02
	MsgProxyFaultHandlerOverride UserMessageType = 0x03
	MsgProxyTransmissionMode     UserMessageType = 0x04
	MsgProxyFlowLabel            UserMessageType = 0x05
	MsgProxySegmentationControl  UserMessageType = 0x06
	MsgProxyPutResponse          UserMessageType = 0x07
	MsgProxyFilestoreResponse    UserMessageType = 0x08
	MsgProxyPutCancel            UserMessageType = 0x09

	// MsgOriginatingTransactionID is common to all User operations (§6.1.5),
	// which is why it sits inside the proxy range rather than after it.
	MsgOriginatingTransactionID UserMessageType = 0x0A

	MsgProxyClosureRequest UserMessageType = 0x0B

	// Directory operations, table 6-14.
	MsgDirectoryListingRequest  UserMessageType = 0x10
	MsgDirectoryListingResponse UserMessageType = 0x11

	// Remote status report operations, table 6-17.
	MsgRemoteStatusReportRequest  UserMessageType = 0x20
	MsgRemoteStatusReportResponse UserMessageType = 0x21

	// Remote suspend operations, table 6-20.
	MsgRemoteSuspendRequest  UserMessageType = 0x30
	MsgRemoteSuspendResponse UserMessageType = 0x31

	// Remote resume operations, table 6-23.
	MsgRemoteResumeRequest  UserMessageType = 0x38
	MsgRemoteResumeResponse UserMessageType = 0x39
)

// userMessageNames is how each type reads.
var userMessageNames = map[UserMessageType]string{
	MsgProxyPutRequest:            "Proxy Put Request",
	MsgProxyMessageToUser:         "Proxy Message to User",
	MsgProxyFilestoreRequest:      "Proxy Filestore Request",
	MsgProxyFaultHandlerOverride:  "Proxy Fault Handler Override",
	MsgProxyTransmissionMode:      "Proxy Transmission Mode",
	MsgProxyFlowLabel:             "Proxy Flow Label",
	MsgProxySegmentationControl:   "Proxy Segmentation Control",
	MsgProxyPutResponse:           "Proxy Put Response",
	MsgProxyFilestoreResponse:     "Proxy Filestore Response",
	MsgProxyPutCancel:             "Proxy Put Cancel",
	MsgOriginatingTransactionID:   "Originating Transaction ID",
	MsgProxyClosureRequest:        "Proxy Closure Request",
	MsgDirectoryListingRequest:    "Directory Listing Request",
	MsgDirectoryListingResponse:   "Directory Listing Response",
	MsgRemoteStatusReportRequest:  "Remote Status Report Request",
	MsgRemoteStatusReportResponse: "Remote Status Report Response",
	MsgRemoteSuspendRequest:       "Remote Suspend Request",
	MsgRemoteSuspendResponse:      "Remote Suspend Response",
	MsgRemoteResumeRequest:        "Remote Resume Request",
	MsgRemoteResumeResponse:       "Remote Resume Response",
}

// String names the message type.
func (t UserMessageType) String() string {
	if name, ok := userMessageNames[t]; ok {
		return name
	}
	return fmt.Sprintf("unknown user message type 0x%02X", uint8(t))
}

// Valid reports whether this is a message type section 6 defines.
func (t UserMessageType) Valid() bool {
	_, ok := userMessageNames[t]
	return ok
}

// UserMessage is one Reserved CFDP Message: its type and its content, with
// the "cfdp" identifier already checked off the front.
type UserMessage struct {
	Type UserMessageType
	// Content is the message body, whose shape the type decides. It is empty
	// for Proxy Put Cancel, which §6.2.6.2 says has no content.
	Content []byte
}

// Encode serializes the message with its identifier and type octet.
func (m UserMessage) Encode() []byte {
	out := make([]byte, 0, MessageMagicSize+1+len(m.Content))
	out = append(out, MessageMagic[:]...)
	out = append(out, byte(m.Type))
	return append(out, m.Content...)
}

// EncodeTLV wraps the message in the Message to User TLV that carries it.
func (m UserMessage) EncodeTLV() TLV {
	return TLV{Type: TLVMessageToUser, Value: m.Encode()}
}

// DecodeUserMessage reads a Reserved CFDP Message from a Message to User
// TLV's value.
//
// A Message to User that does not open with "cfdp" is an application message
// and not this package's business, so it comes back as ErrNotUserMessage
// rather than as a malformed protocol message. A receiver walks the metadata
// TLVs and uses that to tell the two apart.
func DecodeUserMessage(data []byte) (*UserMessage, error) {
	if len(data) < MessageMagicSize+1 {
		return nil, ErrNotUserMessage
	}
	if string(data[:MessageMagicSize]) != string(MessageMagic[:]) {
		return nil, ErrNotUserMessage
	}

	return &UserMessage{
		Type:    UserMessageType(data[MessageMagicSize]),
		Content: data[MessageMagicSize+1:],
	}, nil
}

// UserMessagesFrom picks the Reserved CFDP Messages out of a run of metadata
// TLVs, leaving application messages alone.
func UserMessagesFrom(tlvs []TLV) []*UserMessage {
	var messages []*UserMessage
	for _, tlv := range tlvs {
		if tlv.Type != TLVMessageToUser {
			continue
		}
		message, err := DecodeUserMessage(tlv.Value)
		if err != nil {
			// An application message. Not an error, just not ours.
			continue
		}
		messages = append(messages, message)
	}
	return messages
}

// TransactionID identifies one transaction by the entity that started it and
// the sequence number that entity gave it.
//
// It appears on its own as the Originating Transaction ID message (table 6-2)
// and again inside the request and response messages of the remote
// operations, always in the same encoding: a length nibble pair, then the two
// values.
type TransactionID struct {
	Source   EntityID
	Sequence EntityID
}

// Encode writes the length octet and the two values (table 6-2).
//
// The two 3-bit length fields hold the width less one, so a one-octet value
// encodes as zero. Each is preceded by a reserved bit that the standard
// requires to be zero.
func (t TransactionID) Encode() ([]byte, error) {
	if err := t.Source.Validate(); err != nil {
		return nil, fmt.Errorf("source entity ID: %w", err)
	}
	if err := t.Sequence.Validate(); err != nil {
		return nil, fmt.Errorf("transaction sequence number: %w", err)
	}

	// Reserved bit, 3-bit entity ID length, reserved bit, 3-bit sequence
	// number length.
	lengths := byte(t.Source.Width-1)<<4 | byte(t.Sequence.Width-1)

	out := []byte{lengths}

	source, err := t.Source.Encode()
	if err != nil {
		return nil, err
	}
	out = append(out, source...)

	sequence, err := t.Sequence.Encode()
	if err != nil {
		return nil, err
	}
	return append(out, sequence...), nil
}

// decodeTransactionID reads a transaction ID and reports how many octets it
// took.
func decodeTransactionID(data []byte) (TransactionID, int, error) {
	if len(data) < 1 {
		return TransactionID{}, 0, ErrDataTooShort
	}

	// The reserved bits must be zero. A sender that sets them is using a
	// field this issue of the standard has not defined, and reading the
	// lengths out from under it would be a guess.
	if data[0]&0x88 != 0 {
		return TransactionID{}, 0, ErrReservedBitsSet
	}

	sourceWidth := int(data[0]>>4&0x07) + 1
	sequenceWidth := int(data[0]&0x07) + 1

	offset := 1
	if len(data) < offset+sourceWidth+sequenceWidth {
		return TransactionID{}, 0, ErrDataTooShort
	}

	source, err := decodeEntityID(data[offset:offset+sourceWidth], sourceWidth)
	if err != nil {
		return TransactionID{}, 0, err
	}
	offset += sourceWidth

	sequence, err := decodeEntityID(data[offset:offset+sequenceWidth], sequenceWidth)
	if err != nil {
		return TransactionID{}, 0, err
	}
	offset += sequenceWidth

	return TransactionID{Source: source, Sequence: sequence}, offset, nil
}

// Humanize returns a human-readable summary.
func (t TransactionID) Humanize() string {
	return fmt.Sprintf("entity %d, sequence %d", t.Source.Value, t.Sequence.Value)
}

// --- Originating Transaction ID, §6.1.5 and table 6-2 ---

// OriginatingTransactionID is the message every User operation carries
// alongside its own, naming the transaction the operation refers to.
type OriginatingTransactionID struct {
	Transaction TransactionID
}

// Encode builds the message.
func (m OriginatingTransactionID) Encode() (UserMessage, error) {
	content, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgOriginatingTransactionID, Content: content}, nil
}

// DecodeOriginatingTransactionID reads the message content.
func DecodeOriginatingTransactionID(content []byte) (*OriginatingTransactionID, error) {
	transaction, _, err := decodeTransactionID(content)
	if err != nil {
		return nil, err
	}
	return &OriginatingTransactionID{Transaction: transaction}, nil
}

// Humanize returns a human-readable summary.
func (m *OriginatingTransactionID) Humanize() string {
	return "  Originating transaction: " + m.Transaction.Humanize()
}

// --- Proxy Put Request, table 6-4 ---

// ProxyPutRequest asks a remote user to send a file to a third entity.
//
// The remote user is the respondent, and the entity named here is the
// beneficiary. When the beneficiary is the originator the operation works as
// a Get (§6.2, note).
type ProxyPutRequest struct {
	// Destination is the beneficiary's entity ID.
	Destination EntityID
	// SourceFileName and DestinationFileName are empty when omitted, which
	// table 6-4 expresses as a zero-length LV rather than an absent field.
	SourceFileName      string
	DestinationFileName string
}

// Encode builds the message.
func (m ProxyPutRequest) Encode() (UserMessage, error) {
	if err := m.Destination.Validate(); err != nil {
		return UserMessage{}, fmt.Errorf("destination entity ID: %w", err)
	}

	// The destination is an LV here, not the bare width-prefixed integer the
	// transaction ID uses.
	id, err := m.Destination.Encode()
	if err != nil {
		return UserMessage{}, err
	}

	content, err := appendLVs(nil, id, []byte(m.SourceFileName), []byte(m.DestinationFileName))
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgProxyPutRequest, Content: content}, nil
}

// DecodeProxyPutRequest reads the message content.
func DecodeProxyPutRequest(content []byte) (*ProxyPutRequest, error) {
	values, err := decodeLVs(content, 3)
	if err != nil {
		return nil, err
	}

	if len(values[0]) == 0 {
		return nil, fmt.Errorf("%w: the destination entity ID is empty", ErrDataTooShort)
	}
	destination, err := decodeEntityID(values[0], len(values[0]))
	if err != nil {
		return nil, err
	}

	return &ProxyPutRequest{
		Destination:         destination,
		SourceFileName:      string(values[1]),
		DestinationFileName: string(values[2]),
	}, nil
}

// Humanize returns a human-readable summary.
func (m *ProxyPutRequest) Humanize() string {
	source, destination := m.SourceFileName, m.DestinationFileName
	if source == "" {
		source = "(omitted)"
	}
	if destination == "" {
		destination = "(omitted)"
	}
	return fmt.Sprintf("  Beneficiary ....... %d\n  Source file ....... %s\n  Destination file .. %s",
		m.Destination.Value, source, destination)
}

// --- Proxy Put Response, table 6-12 ---

// ProxyPutResponse reports the outcome of a proxy put back to the originator.
type ProxyPutResponse struct {
	Condition ConditionCode
	Delivery  DeliveryCode
	File      FileStatus
}

// Encode builds the message.
//
// The layout is condition code (4 bits), one spare bit, delivery code
// (1 bit), file status (2 bits) — the same packing the Finished PDU uses,
// which is why the codes are the shared types.
func (m ProxyPutResponse) Encode() (UserMessage, error) {
	octet := byte(m.Condition&0x0F)<<4 |
		byte(m.Delivery&0x01)<<2 |
		byte(m.File&0x03)

	return UserMessage{Type: MsgProxyPutResponse, Content: []byte{octet}}, nil
}

// DecodeProxyPutResponse reads the message content.
func DecodeProxyPutResponse(content []byte) (*ProxyPutResponse, error) {
	if len(content) < 1 {
		return nil, ErrDataTooShort
	}
	return &ProxyPutResponse{
		Condition: ConditionCode(content[0] >> 4),
		Delivery:  DeliveryCode(content[0] >> 2 & 0x01),
		File:      FileStatus(content[0] & 0x03),
	}, nil
}

// Humanize returns a human-readable summary.
func (m *ProxyPutResponse) Humanize() string {
	return fmt.Sprintf("  Condition ... %s\n  Delivery .... %s\n  File ........ %s",
		m.Condition, m.Delivery, m.File)
}

// --- Proxy Filestore Request and Response, tables 6-6 and 6-13 ---

// ProxyFilestoreRequest carries one filestore request for the respondent to
// perform as part of the proxy operation.
//
// Its content is a length octet and then a filestore request TLV's value, not
// a whole TLV: table 6-6 says the field "is a single CFDP filestore request
// as defined in table 5-15", and the length octet in front of it does the job
// the TLV's own length would.
type ProxyFilestoreRequest struct {
	Request FilestoreRequest
}

// Encode builds the message.
func (m ProxyFilestoreRequest) Encode() (UserMessage, error) {
	tlv, err := m.Request.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	if len(tlv.Value) > 0xFF {
		return UserMessage{}, fmt.Errorf("%w: the filestore request is %d octets",
			ErrDataLengthMismatch, len(tlv.Value))
	}

	content := append([]byte{byte(len(tlv.Value))}, tlv.Value...)
	return UserMessage{Type: MsgProxyFilestoreRequest, Content: content}, nil
}

// DecodeProxyFilestoreRequest reads the message content.
func DecodeProxyFilestoreRequest(content []byte) (*ProxyFilestoreRequest, error) {
	value, err := decodeLengthPrefixed(content)
	if err != nil {
		return nil, err
	}

	request, err := DecodeFilestoreRequest(TLV{Type: TLVFilestoreRequest, Value: value})
	if err != nil {
		return nil, err
	}
	return &ProxyFilestoreRequest{Request: *request}, nil
}

// Humanize returns a human-readable summary.
func (m *ProxyFilestoreRequest) Humanize() string {
	if !m.Request.SecondFileName.IsEmpty() {
		return fmt.Sprintf("  %s %s %s",
			m.Request.Action, m.Request.FirstFileName, m.Request.SecondFileName)
	}
	return fmt.Sprintf("  %s %s", m.Request.Action, m.Request.FirstFileName)
}

// ProxyFilestoreResponse reports the outcome of one proxy filestore request.
type ProxyFilestoreResponse struct {
	Response FilestoreResponse
}

// Encode builds the message.
func (m ProxyFilestoreResponse) Encode() (UserMessage, error) {
	tlv, err := m.Response.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	if len(tlv.Value) > 0xFF {
		return UserMessage{}, fmt.Errorf("%w: the filestore response is %d octets",
			ErrDataLengthMismatch, len(tlv.Value))
	}

	content := append([]byte{byte(len(tlv.Value))}, tlv.Value...)
	return UserMessage{Type: MsgProxyFilestoreResponse, Content: content}, nil
}

// DecodeProxyFilestoreResponse reads the message content.
func DecodeProxyFilestoreResponse(content []byte) (*ProxyFilestoreResponse, error) {
	value, err := decodeLengthPrefixed(content)
	if err != nil {
		return nil, err
	}

	response, err := DecodeFilestoreResponse(TLV{Type: TLVFilestoreResponse, Value: value})
	if err != nil {
		return nil, err
	}
	return &ProxyFilestoreResponse{Response: *response}, nil
}

// Humanize returns a human-readable summary.
func (m *ProxyFilestoreResponse) Humanize() string {
	return fmt.Sprintf("  %s on %s: status %d",
		m.Response.Action, m.Response.FirstFileName, m.Response.StatusCode)
}

// --- the single-flag proxy messages, tables 6-8, 6-10 and 6-11 ---

// ProxyTransmissionMode selects acknowledged or unacknowledged transmission
// for the proxied transaction (table 6-8).
type ProxyTransmissionMode struct {
	Acknowledged bool
}

// Encode builds the message: seven spare bits then the mode.
func (m ProxyTransmissionMode) Encode() UserMessage {
	// Table 5-1: '0' is acknowledged, '1' is unacknowledged. The flag here
	// reads the useful way round, so it is inverted on the wire.
	var octet byte
	if !m.Acknowledged {
		octet = 1
	}
	return UserMessage{Type: MsgProxyTransmissionMode, Content: []byte{octet}}
}

// DecodeProxyTransmissionMode reads the message content.
func DecodeProxyTransmissionMode(content []byte) (*ProxyTransmissionMode, error) {
	flag, err := decodeSpareFlag(content, "transmission mode")
	if err != nil {
		return nil, err
	}
	return &ProxyTransmissionMode{Acknowledged: !flag}, nil
}

// ProxySegmentationControl says whether record boundaries are respected
// (table 6-10).
type ProxySegmentationControl struct {
	// RecordBoundariesRespected is '0' on the wire when true, per table 6-10.
	RecordBoundariesRespected bool
}

// Encode builds the message.
func (m ProxySegmentationControl) Encode() UserMessage {
	var octet byte
	if !m.RecordBoundariesRespected {
		octet = 1
	}
	return UserMessage{Type: MsgProxySegmentationControl, Content: []byte{octet}}
}

// DecodeProxySegmentationControl reads the message content.
func DecodeProxySegmentationControl(content []byte) (*ProxySegmentationControl, error) {
	flag, err := decodeSpareFlag(content, "segmentation control")
	if err != nil {
		return nil, err
	}
	return &ProxySegmentationControl{RecordBoundariesRespected: !flag}, nil
}

// ProxyClosureRequest asks for transaction closure on the proxied transaction
// (table 6-11).
type ProxyClosureRequest struct {
	ClosureRequested bool
}

// Encode builds the message.
func (m ProxyClosureRequest) Encode() UserMessage {
	var octet byte
	if m.ClosureRequested {
		octet = 1
	}
	return UserMessage{Type: MsgProxyClosureRequest, Content: []byte{octet}}
}

// DecodeProxyClosureRequest reads the message content.
func DecodeProxyClosureRequest(content []byte) (*ProxyClosureRequest, error) {
	flag, err := decodeSpareFlag(content, "closure requested")
	if err != nil {
		return nil, err
	}
	return &ProxyClosureRequest{ClosureRequested: flag}, nil
}

// --- the LV-only proxy messages, tables 6-5, 6-7 and 6-9 ---

// ProxyMessageToUser carries a message for the beneficiary's user, to be
// placed in the proxied transaction's metadata (table 6-5).
type ProxyMessageToUser struct {
	Text []byte
}

// Encode builds the message.
func (m ProxyMessageToUser) Encode() (UserMessage, error) {
	content, err := appendLVs(nil, m.Text)
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgProxyMessageToUser, Content: content}, nil
}

// DecodeProxyMessageToUser reads the message content.
func DecodeProxyMessageToUser(content []byte) (*ProxyMessageToUser, error) {
	values, err := decodeLVs(content, 1)
	if err != nil {
		return nil, err
	}
	return &ProxyMessageToUser{Text: values[0]}, nil
}

// ProxyFlowLabel carries a mission-defined flow label for the proxied
// transaction (table 6-9).
type ProxyFlowLabel struct {
	Label []byte
}

// Encode builds the message.
func (m ProxyFlowLabel) Encode() (UserMessage, error) {
	content, err := appendLVs(nil, m.Label)
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgProxyFlowLabel, Content: content}, nil
}

// DecodeProxyFlowLabel reads the message content.
func DecodeProxyFlowLabel(content []byte) (*ProxyFlowLabel, error) {
	values, err := decodeLVs(content, 1)
	if err != nil {
		return nil, err
	}
	return &ProxyFlowLabel{Label: values[0]}, nil
}

// ProxyFaultHandlerOverride changes a fault handler for the proxied
// transaction (table 6-7).
//
// Its content is one octet holding the same condition code and handler code
// pairing §5.4.4 defines for the fault handler override TLV.
type ProxyFaultHandlerOverride struct {
	Condition ConditionCode
	Handler   FaultHandler
}

// Encode builds the message.
func (m ProxyFaultHandlerOverride) Encode() UserMessage {
	octet := byte(m.Condition&0x0F)<<4 | byte(m.Handler&0x0F)
	return UserMessage{Type: MsgProxyFaultHandlerOverride, Content: []byte{octet}}
}

// DecodeProxyFaultHandlerOverride reads the message content.
func DecodeProxyFaultHandlerOverride(content []byte) (*ProxyFaultHandlerOverride, error) {
	if len(content) < 1 {
		return nil, ErrDataTooShort
	}
	return &ProxyFaultHandlerOverride{
		Condition: ConditionCode(content[0] >> 4),
		Handler:   FaultHandler(content[0] & 0x0F),
	}, nil
}

// --- Directory operations, tables 6-15 and 6-16 ---

// DirectoryListingRequest asks a remote user for a directory listing, to be
// written to a named file on the requester's own filestore (table 6-15).
type DirectoryListingRequest struct {
	DirectoryName string
	// DirectoryFileName is where the responder should put the listing, on the
	// filestore local to the requesting user.
	DirectoryFileName string
}

// Encode builds the message.
func (m DirectoryListingRequest) Encode() (UserMessage, error) {
	content, err := appendLVs(nil, []byte(m.DirectoryName), []byte(m.DirectoryFileName))
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgDirectoryListingRequest, Content: content}, nil
}

// DecodeDirectoryListingRequest reads the message content.
func DecodeDirectoryListingRequest(content []byte) (*DirectoryListingRequest, error) {
	values, err := decodeLVs(content, 2)
	if err != nil {
		return nil, err
	}
	return &DirectoryListingRequest{
		DirectoryName:     string(values[0]),
		DirectoryFileName: string(values[1]),
	}, nil
}

// Humanize returns a human-readable summary.
func (m *DirectoryListingRequest) Humanize() string {
	return fmt.Sprintf("  Directory ..... %s\n  Listing file .. %s",
		m.DirectoryName, m.DirectoryFileName)
}

// DirectoryListingResponse reports whether the listing could be produced
// (table 6-16).
type DirectoryListingResponse struct {
	// Successful is the listing response code. Table 6-16 encodes success as
	// '0' — the opposite polarity from the Remote Status Report Response,
	// where table 6-19 encodes success as '1'. The flag here reads the same
	// way in both, and each encoder writes what its own table says.
	Successful bool

	DirectoryName     string
	DirectoryFileName string
}

// Encode builds the message: the response code, seven spare bits, then the
// two names.
func (m DirectoryListingResponse) Encode() (UserMessage, error) {
	// '0' is successful here.
	var octet byte
	if !m.Successful {
		octet = 0x80
	}

	content := []byte{octet}
	content, err := appendLVs(content, []byte(m.DirectoryName), []byte(m.DirectoryFileName))
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgDirectoryListingResponse, Content: content}, nil
}

// DecodeDirectoryListingResponse reads the message content.
func DecodeDirectoryListingResponse(content []byte) (*DirectoryListingResponse, error) {
	if len(content) < 1 {
		return nil, ErrDataTooShort
	}
	if content[0]&0x7F != 0 {
		return nil, ErrReservedBitsSet
	}

	values, err := decodeLVs(content[1:], 2)
	if err != nil {
		return nil, err
	}

	return &DirectoryListingResponse{
		Successful:        content[0]&0x80 == 0,
		DirectoryName:     string(values[0]),
		DirectoryFileName: string(values[1]),
	}, nil
}

// Humanize returns a human-readable summary.
func (m *DirectoryListingResponse) Humanize() string {
	outcome := "unsuccessful"
	if m.Successful {
		outcome = "successful"
	}
	return fmt.Sprintf("  Outcome ....... %s\n  Directory ..... %s\n  Listing file .. %s",
		outcome, m.DirectoryName, m.DirectoryFileName)
}

// --- Remote status report operations, tables 6-18 and 6-19 ---

// RemoteStatusReportRequest asks a remote user for a status report on one
// transaction, written to a named file on the requester's filestore
// (table 6-18).
type RemoteStatusReportRequest struct {
	Transaction TransactionID
	// ReportFileName is where the responder should put the report.
	ReportFileName string
}

// Encode builds the message.
func (m RemoteStatusReportRequest) Encode() (UserMessage, error) {
	content, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	content, err = appendLVs(content, []byte(m.ReportFileName))
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgRemoteStatusReportRequest, Content: content}, nil
}

// DecodeRemoteStatusReportRequest reads the message content.
func DecodeRemoteStatusReportRequest(content []byte) (*RemoteStatusReportRequest, error) {
	transaction, used, err := decodeTransactionID(content)
	if err != nil {
		return nil, err
	}

	values, err := decodeLVs(content[used:], 1)
	if err != nil {
		return nil, err
	}

	return &RemoteStatusReportRequest{
		Transaction:    transaction,
		ReportFileName: string(values[0]),
	}, nil
}

// Humanize returns a human-readable summary.
func (m *RemoteStatusReportRequest) Humanize() string {
	return fmt.Sprintf("  Transaction .. %s\n  Report file .. %s",
		m.Transaction.Humanize(), m.ReportFileName)
}

// RemoteStatusReportResponse reports whether the status report could be
// produced (table 6-19).
type RemoteStatusReportResponse struct {
	Status TransactionStatus
	// Successful is the report response code. Table 6-19 encodes success as
	// '1', the opposite of the Directory Listing Response's '0'.
	Successful bool

	Transaction TransactionID
}

// Encode builds the message.
//
// The first octet packs transaction status (2 bits), five spare bits, then
// the response code in the low bit — an unusual order, with the flag last
// rather than first as in table 6-16.
func (m RemoteStatusReportResponse) Encode() (UserMessage, error) {
	octet := byte(m.Status&0x03) << 6
	if m.Successful {
		octet |= 0x01
	}

	content := []byte{octet}
	transaction, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{
		Type:    MsgRemoteStatusReportResponse,
		Content: append(content, transaction...),
	}, nil
}

// DecodeRemoteStatusReportResponse reads the message content.
func DecodeRemoteStatusReportResponse(content []byte) (*RemoteStatusReportResponse, error) {
	if len(content) < 1 {
		return nil, ErrDataTooShort
	}
	if content[0]&0x3E != 0 {
		return nil, ErrReservedBitsSet
	}

	transaction, _, err := decodeTransactionID(content[1:])
	if err != nil {
		return nil, err
	}

	return &RemoteStatusReportResponse{
		Status:      TransactionStatus(content[0] >> 6),
		Successful:  content[0]&0x01 != 0,
		Transaction: transaction,
	}, nil
}

// Humanize returns a human-readable summary.
func (m *RemoteStatusReportResponse) Humanize() string {
	outcome := "unsuccessful"
	if m.Successful {
		outcome = "successful"
	}
	return fmt.Sprintf("  Outcome ...... %s\n  Status ....... %s\n  Transaction .. %s",
		outcome, m.Status, m.Transaction.Humanize())
}

// --- Remote suspend and resume, tables 6-21, 6-22, 6-24 and 6-25 ---

// RemoteSuspendRequest asks a remote user to suspend one transaction
// (table 6-21).
//
// §6.5.3.1.2 requires the carrying transaction to be Acknowledged.
type RemoteSuspendRequest struct {
	Transaction TransactionID
}

// Encode builds the message.
func (m RemoteSuspendRequest) Encode() (UserMessage, error) {
	content, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgRemoteSuspendRequest, Content: content}, nil
}

// DecodeRemoteSuspendRequest reads the message content.
func DecodeRemoteSuspendRequest(content []byte) (*RemoteSuspendRequest, error) {
	transaction, _, err := decodeTransactionID(content)
	if err != nil {
		return nil, err
	}
	return &RemoteSuspendRequest{Transaction: transaction}, nil
}

// RemoteResumeRequest asks a remote user to resume one transaction
// (table 6-24). §6.6.3.1.2 requires the carrying transaction to be
// Acknowledged.
type RemoteResumeRequest struct {
	Transaction TransactionID
}

// Encode builds the message.
func (m RemoteResumeRequest) Encode() (UserMessage, error) {
	content, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: MsgRemoteResumeRequest, Content: content}, nil
}

// DecodeRemoteResumeRequest reads the message content.
func DecodeRemoteResumeRequest(content []byte) (*RemoteResumeRequest, error) {
	transaction, _, err := decodeTransactionID(content)
	if err != nil {
		return nil, err
	}
	return &RemoteResumeRequest{Transaction: transaction}, nil
}

// SuspensionResponse is the body both the suspend and the resume response
// share: whether the transaction is now suspended, its status, and which
// transaction it was (tables 6-22 and 6-25).
//
// §6.6.4.2 notes that a successful resume may not change the suspension
// status at all, because several motivations for suspending can be valid at
// once — so the indicator reports the state, not the outcome of the request.
type SuspensionResponse struct {
	Suspended bool
	Status    TransactionStatus

	Transaction TransactionID
}

// encode builds the shared body.
func (m SuspensionResponse) encode(kind UserMessageType) (UserMessage, error) {
	// Suspension indicator (1 bit), transaction status (2 bits), five spare.
	var octet byte
	if m.Suspended {
		octet |= 0x80
	}
	octet |= byte(m.Status&0x03) << 5

	content := []byte{octet}
	transaction, err := m.Transaction.Encode()
	if err != nil {
		return UserMessage{}, err
	}
	return UserMessage{Type: kind, Content: append(content, transaction...)}, nil
}

// decodeSuspensionResponse reads the shared body.
func decodeSuspensionResponse(content []byte) (*SuspensionResponse, error) {
	if len(content) < 1 {
		return nil, ErrDataTooShort
	}
	if content[0]&0x1F != 0 {
		return nil, ErrReservedBitsSet
	}

	transaction, _, err := decodeTransactionID(content[1:])
	if err != nil {
		return nil, err
	}

	return &SuspensionResponse{
		Suspended:   content[0]&0x80 != 0,
		Status:      TransactionStatus(content[0] >> 5 & 0x03),
		Transaction: transaction,
	}, nil
}

// Humanize returns a human-readable summary.
func (m *SuspensionResponse) Humanize() string {
	state := "not suspended"
	if m.Suspended {
		state = "suspended"
	}
	return fmt.Sprintf("  State ........ %s\n  Status ....... %s\n  Transaction .. %s",
		state, m.Status, m.Transaction.Humanize())
}

// RemoteSuspendResponse reports the suspension state after a suspend request
// (table 6-22).
type RemoteSuspendResponse struct {
	SuspensionResponse
}

// Encode builds the message.
func (m RemoteSuspendResponse) Encode() (UserMessage, error) {
	return m.encode(MsgRemoteSuspendResponse)
}

// DecodeRemoteSuspendResponse reads the message content.
func DecodeRemoteSuspendResponse(content []byte) (*RemoteSuspendResponse, error) {
	body, err := decodeSuspensionResponse(content)
	if err != nil {
		return nil, err
	}
	return &RemoteSuspendResponse{SuspensionResponse: *body}, nil
}

// RemoteResumeResponse reports the suspension state after a resume request
// (table 6-25).
type RemoteResumeResponse struct {
	SuspensionResponse
}

// Encode builds the message.
func (m RemoteResumeResponse) Encode() (UserMessage, error) {
	return m.encode(MsgRemoteResumeResponse)
}

// DecodeRemoteResumeResponse reads the message content.
func DecodeRemoteResumeResponse(content []byte) (*RemoteResumeResponse, error) {
	body, err := decodeSuspensionResponse(content)
	if err != nil {
		return nil, err
	}
	return &RemoteResumeResponse{SuspensionResponse: *body}, nil
}

// ProxyPutCancel asks the respondent to cancel the proxied transaction.
//
// §6.2.6.2: "A Proxy Put Cancel message is mandatory. It has no content."
func ProxyPutCancel() UserMessage {
	return UserMessage{Type: MsgProxyPutCancel}
}

// --- shared helpers ---

// appendLVs writes a run of length-value fields.
func appendLVs(dst []byte, values ...[]byte) ([]byte, error) {
	for _, value := range values {
		encoded, err := LV{Value: value}.Encode()
		if err != nil {
			return nil, err
		}
		dst = append(dst, encoded...)
	}
	return dst, nil
}

// decodeLVs reads exactly count length-value fields.
//
// A message with fewer than its table says is truncated; one with more is
// carrying something this issue of the standard does not define, and reading
// past what is known would be a guess either way.
func decodeLVs(data []byte, count int) ([][]byte, error) {
	values := make([][]byte, 0, count)

	offset := 0
	for range count {
		lv, used, err := DecodeLV(data[offset:])
		if err != nil {
			return nil, err
		}
		values = append(values, lv.Value)
		offset += used
	}

	if offset != len(data) {
		return nil, fmt.Errorf("%w: %d octets left after %d length-value fields",
			ErrDataLengthMismatch, len(data)-offset, count)
	}
	return values, nil
}

// decodeLengthPrefixed reads an eight-bit length and the value it names,
// which is the shape tables 6-6 and 6-13 use.
func decodeLengthPrefixed(data []byte) ([]byte, error) {
	if len(data) < 1 {
		return nil, ErrDataTooShort
	}
	length := int(data[0])
	if len(data) < 1+length {
		return nil, ErrDataTooShort
	}
	if len(data) != 1+length {
		return nil, fmt.Errorf("%w: the length octet says %d but %d octets follow",
			ErrDataLengthMismatch, length, len(data)-1)
	}
	return data[1 : 1+length], nil
}

// decodeSpareFlag reads the one-octet messages that are seven spare bits and
// a flag in the low bit.
func decodeSpareFlag(content []byte, what string) (bool, error) {
	if len(content) < 1 {
		return false, ErrDataTooShort
	}
	if len(content) != 1 {
		return false, fmt.Errorf("%w: %s should be one octet, got %d",
			ErrDataLengthMismatch, what, len(content))
	}
	if content[0]&0xFE != 0 {
		return false, ErrReservedBitsSet
	}
	return content[0]&0x01 != 0, nil
}
