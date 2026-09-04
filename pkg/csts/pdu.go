package csts

import "github.com/ravisuhag/astro/internal/ber"

// The framework protocol data unit of CCSDS 921.1-B-2 annex F3.15.
//
// CstsFrameworkPdu is a CHOICE over every message the framework defines, and
// its context tag is what says which one arrived. The numbering is deliberate
// and worth reading: the tags run in tens, so [10] and [11] are the START
// invocation and return, [20] and [21] the STOP pair, and so on. That leaves
// room between them for a future issue to add messages without renumbering
// anything, and it is why the tags are not 0 to 19.
//
// This is the part of CSTS that differs most from SLE in practice. An SLE PDU
// tag means one operation in RAF and another in FCLTU, so pkg/sle needs the
// service told to it out of band. A framework PDU tag means the same operation
// everywhere, and the procedure it belongs to is inside the message.

// OperationType is which alternative of the framework PDU CHOICE a message is.
type OperationType uint32

// The context tags of annex F3.15.
const (
	OpBindInvocation              OperationType = 0
	OpBindReturn                  OperationType = 1
	OpUnbindInvocation            OperationType = 2
	OpUnbindReturn                OperationType = 3
	OpPeerAbortInvocation         OperationType = 4
	OpStartInvocation             OperationType = 10
	OpStartReturn                 OperationType = 11
	OpStopInvocation              OperationType = 20
	OpStopReturn                  OperationType = 21
	OpExecuteDirectiveInvocation  OperationType = 30
	OpExecuteDirectiveAcknowledge OperationType = 31
	OpExecuteDirectiveReturn      OperationType = 32
	OpGetInvocation               OperationType = 40
	OpGetReturn                   OperationType = 41
	OpNotifyInvocation            OperationType = 50
	OpProcessDataInvocation       OperationType = 60
	OpProcessDataReturn           OperationType = 61
	OpForwardBuffer               OperationType = 62
	OpTransferDataInvocation      OperationType = 70
	OpReturnBuffer                OperationType = 71
)

var operationNames = map[OperationType]string{
	OpBindInvocation:              "BIND invocation",
	OpBindReturn:                  "BIND return",
	OpUnbindInvocation:            "UNBIND invocation",
	OpUnbindReturn:                "UNBIND return",
	OpPeerAbortInvocation:         "PEER-ABORT invocation",
	OpStartInvocation:             "START invocation",
	OpStartReturn:                 "START return",
	OpStopInvocation:              "STOP invocation",
	OpStopReturn:                  "STOP return",
	OpExecuteDirectiveInvocation:  "EXECUTE-DIRECTIVE invocation",
	OpExecuteDirectiveAcknowledge: "EXECUTE-DIRECTIVE acknowledgement",
	OpExecuteDirectiveReturn:      "EXECUTE-DIRECTIVE return",
	OpGetInvocation:               "GET invocation",
	OpGetReturn:                   "GET return",
	OpNotifyInvocation:            "NOTIFY invocation",
	OpProcessDataInvocation:       "PROCESS-DATA invocation",
	OpProcessDataReturn:           "PROCESS-DATA return",
	OpForwardBuffer:               "forward buffer",
	OpTransferDataInvocation:      "TRANSFER-DATA invocation",
	OpReturnBuffer:                "return buffer",
}

// String names the operation, or reports the tag for one annex F3.15 does not
// define.
func (t OperationType) String() string {
	if name, ok := operationNames[t]; ok {
		return name
	}
	return "unknown operation " + itoa(uint64(t))
}

// Known reports whether the tag is one of the twenty alternatives.
func (t OperationType) Known() bool {
	_, ok := operationNames[t]
	return ok
}

// PDU is one framework protocol data unit.
//
// Exactly one of the message fields is set, matching Type. Content holds the
// encoded alternative in every case, including the ones this package does not
// model, so nothing is lost by decoding a PDU it only partly understands.
type PDU struct {
	Type OperationType
	// Content is the encoded content of the chosen alternative, without the
	// outer tag and length.
	Content []byte

	Bind         *BindInvocation
	BindReturn   *BindReturn
	Unbind       *UnbindInvocation
	UnbindReturn *UnbindReturn
	PeerAbort    *PeerAbortInvocation

	Start        *StartInvocation
	Stop         *StopInvocation
	Get          *GetInvocation
	Notify       *NotifyInvocation
	TransferData *TransferDataInvocation
	ProcessData  *ProcessDataInvocation

	// Return holds any of the responses annex F3.4 defines as the standard
	// return header alone.
	Return *StandardReturn
}

// Encode writes the PDU.
func (p *PDU) Encode() ([]byte, error) {
	content, err := p.encodeContent()
	if err != nil {
		return nil, err
	}
	// Every alternative of the CHOICE is a SEQUENCE, so the context tag that
	// replaces its universal tag is constructed in every case — the peer
	// abort included, which is a SEQUENCE holding one OCTET STRING.
	return ber.AppendElement(nil, ber.ClassContext, true, uint32(p.Type), content), nil
}

func (p *PDU) encodeContent() ([]byte, error) {
	switch p.Type {
	case OpBindInvocation:
		if p.Bind == nil {
			return nil, ErrMissingField
		}
		return p.Bind.encode()
	case OpBindReturn:
		if p.BindReturn == nil {
			return nil, ErrMissingField
		}
		return p.BindReturn.encode()
	case OpUnbindInvocation:
		if p.Unbind == nil {
			return nil, ErrMissingField
		}
		return p.Unbind.encode()
	case OpUnbindReturn:
		if p.UnbindReturn == nil {
			return nil, ErrMissingField
		}
		return p.UnbindReturn.encode()
	case OpPeerAbortInvocation:
		if p.PeerAbort == nil {
			return nil, ErrMissingField
		}
		return p.PeerAbort.encode()
	case OpStartInvocation:
		if p.Start == nil {
			return nil, ErrMissingField
		}
		return p.Start.encode()
	case OpStopInvocation:
		if p.Stop == nil {
			return nil, ErrMissingField
		}
		return p.Stop.encode()
	case OpGetInvocation:
		if p.Get == nil {
			return nil, ErrMissingField
		}
		return p.Get.encode()
	case OpNotifyInvocation:
		if p.Notify == nil {
			return nil, ErrMissingField
		}
		return p.Notify.encode()
	case OpTransferDataInvocation:
		if p.TransferData == nil {
			return nil, ErrMissingField
		}
		return p.TransferData.encode()
	case OpProcessDataInvocation:
		if p.ProcessData == nil {
			return nil, ErrMissingField
		}
		return p.ProcessData.encode()
	case OpStartReturn, OpStopReturn, OpGetReturn, OpProcessDataReturn,
		OpExecuteDirectiveReturn, OpExecuteDirectiveAcknowledge:
		if p.Return == nil {
			return nil, ErrMissingField
		}
		return p.Return.encode()
	case OpExecuteDirectiveInvocation, OpForwardBuffer, OpReturnBuffer:
		// Modelled as raw content: the directive qualifier is a four-way
		// CHOICE over SANA-registered identifiers, and the two buffers belong
		// to the Buffered Data Delivery and Buffered Data Processing
		// procedures rather than to the common operations. See the package
		// documentation.
		//
		// The test is for nil rather than for length, because an empty
		// SEQUENCE OF is a buffer that carries nothing, which both buffer
		// types allow. A decoded PDU always has a Content slice; one built by
		// hand without it does not.
		if p.Content == nil {
			return nil, ErrMissingField
		}
		return p.Content, nil
	}
	return nil, ErrUnknownOperation
}

// Decode reads one framework PDU.
//
// The content of every alternative is kept, and the ones this package models
// are read into their own types besides. A tag annex F3.15 does not define is
// refused: the twenty alternatives are the whole CHOICE, and a twenty-first
// would be a message this implementation cannot claim to have understood.
func Decode(data []byte) (*PDU, error) {
	d := ber.NewDecoder(data)

	e, err := d.Next()
	if err != nil {
		return nil, err
	}
	if e.Class != ber.ClassContext {
		return nil, ErrMalformedPDU
	}
	// Every alternative of the CHOICE in annex F3.15 is a SEQUENCE — the two
	// buffers are SEQUENCE OF, the rest plain SEQUENCEs — so the context tag
	// that replaces the universal one is constructed in every case. A
	// primitive tag here is a PDU no conforming peer produced.
	// Found by FuzzDecode.
	if !e.Constructed {
		return nil, ErrMalformedPDU
	}
	if !d.Empty() {
		return nil, ErrTrailingContent
	}

	p := &PDU{Type: OperationType(e.Tag), Content: e.Copy()}
	if !p.Type.Known() {
		return nil, ErrUnknownOperation
	}

	switch p.Type {
	case OpBindInvocation:
		p.Bind, err = decodeBindInvocation(e.Bytes)
	case OpBindReturn:
		p.BindReturn, err = decodeBindReturn(e.Bytes)
	case OpUnbindInvocation:
		p.Unbind, err = decodeUnbindInvocation(e.Bytes)
	case OpUnbindReturn:
		p.UnbindReturn, err = decodeUnbindReturn(e.Bytes)
	case OpPeerAbortInvocation:
		p.PeerAbort, err = decodePeerAbortInvocation(e.Bytes)
	case OpStartInvocation:
		var h InvocationHeader
		if h, err = decodeHeaderOnlyInvocation(e.Bytes); err == nil {
			p.Start = &StartInvocation{Header: h}
		}
	case OpStopInvocation:
		var h InvocationHeader
		if h, err = decodeHeaderOnlyInvocation(e.Bytes); err == nil {
			p.Stop = &StopInvocation{Header: h}
		}
	case OpGetInvocation:
		p.Get, err = decodeGetInvocation(e.Bytes)
	case OpNotifyInvocation:
		p.Notify, err = decodeNotifyInvocation(e.Bytes)
	case OpTransferDataInvocation:
		p.TransferData, err = decodeTransferDataInvocation(e.Bytes)
	case OpProcessDataInvocation:
		p.ProcessData, err = decodeProcessDataInvocation(e.Bytes)
	case OpStartReturn, OpStopReturn, OpGetReturn, OpProcessDataReturn,
		OpExecuteDirectiveReturn, OpExecuteDirectiveAcknowledge:
		p.Return, err = decodeStandardReturn(e.Bytes)
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Header returns the standard invocation header of whichever operation this
// PDU carries, and whether it has one.
//
// PEER-ABORT has none, which clause 3.3.1.1 states outright, and neither does
// any response — a response carries a return header instead.
func (p *PDU) Header() (InvocationHeader, bool) {
	switch {
	case p.Bind != nil:
		return p.Bind.Header, true
	case p.Unbind != nil:
		return p.Unbind.Header, true
	case p.Start != nil:
		return p.Start.Header, true
	case p.Stop != nil:
		return p.Stop.Header, true
	case p.Get != nil:
		return p.Get.Header, true
	case p.Notify != nil:
		return p.Notify.Header, true
	case p.TransferData != nil:
		return p.TransferData.Header, true
	case p.ProcessData != nil:
		return p.ProcessData.Header, true
	}
	return InvocationHeader{}, false
}

// ReturnHeader returns the standard return header of whichever response this
// PDU carries, and whether it has one.
func (p *PDU) ReturnHeader() (ReturnHeader, bool) {
	switch {
	case p.BindReturn != nil:
		return p.BindReturn.Header, true
	case p.UnbindReturn != nil:
		return p.UnbindReturn.Header, true
	case p.Return != nil:
		return p.Return.Header, true
	}
	return ReturnHeader{}, false
}
