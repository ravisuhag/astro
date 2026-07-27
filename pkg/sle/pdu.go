package sle

import "fmt"

// PDU envelopes.
//
// Every SLE PDU travels inside a context-specific tag naming the operation,
// taken from the service's PDU CHOICE. The BIND family shares tags [100] to
// [104] across all four services; the service-specific operations take low
// numbers that differ per service.
//
// So decoding is two steps: read the outer tag to learn which operation this
// is, then decode the content with the right codec.

// OperationType names an SLE operation, independent of which service carries
// it. The wire tags differ per service; this does not.
type OperationType int

const (
	OpUnknown OperationType = iota
	OpBindInvocation
	OpBindReturn
	OpUnbindInvocation
	OpUnbindReturn
	OpPeerAbort
	OpStartInvocation
	OpStartReturn
	OpStopInvocation
	OpStopReturn
	OpScheduleStatusReportInvocation
	OpScheduleStatusReportReturn
	OpGetParameterInvocation
	OpGetParameterReturn
	OpTransferBuffer
	OpStatusReportInvocation
	OpTransferDataInvocation
	OpAsyncNotifyInvocation
	OpThrowEventInvocation
	OpThrowEventReturn
	OpTransferDataReturn
)

// String names the operation.
func (o OperationType) String() string {
	switch o {
	case OpBindInvocation:
		return "BIND invocation"
	case OpBindReturn:
		return "BIND return"
	case OpUnbindInvocation:
		return "UNBIND invocation"
	case OpUnbindReturn:
		return "UNBIND return"
	case OpPeerAbort:
		return "PEER-ABORT"
	case OpStartInvocation:
		return "START invocation"
	case OpStartReturn:
		return "START return"
	case OpStopInvocation:
		return "STOP invocation"
	case OpStopReturn:
		return "STOP return"
	case OpScheduleStatusReportInvocation:
		return "SCHEDULE-STATUS-REPORT invocation"
	case OpScheduleStatusReportReturn:
		return "SCHEDULE-STATUS-REPORT return"
	case OpGetParameterInvocation:
		return "GET-PARAMETER invocation"
	case OpGetParameterReturn:
		return "GET-PARAMETER return"
	case OpTransferBuffer:
		return "TRANSFER-BUFFER"
	case OpStatusReportInvocation:
		return "STATUS-REPORT invocation"
	case OpTransferDataInvocation:
		return "TRANSFER-DATA invocation"
	case OpAsyncNotifyInvocation:
		return "ASYNC-NOTIFY invocation"
	case OpThrowEventInvocation:
		return "THROW-EVENT invocation"
	case OpThrowEventReturn:
		return "THROW-EVENT return"
	case OpTransferDataReturn:
		return "TRANSFER-DATA return"
	default:
		return "unknown operation"
	}
}

// ServiceKind names which transfer service a PDU belongs to. The same tag
// number means different operations in different services, so decoding needs
// to know.
type ServiceKind int

const (
	// ServiceRAF is Return All Frames, CCSDS 911.1-B-5.
	ServiceRAF ServiceKind = iota
	// ServiceRCF is Return Channel Frames, CCSDS 911.2-B-4.
	ServiceRCF
	// ServiceROCF is Return Operational Control Fields, CCSDS 911.5-B-4.
	ServiceROCF
	// ServiceFCLTU is Forward CLTU, CCSDS 912.1-B-5.
	ServiceFCLTU
)

// String names the service.
func (s ServiceKind) String() string {
	switch s {
	case ServiceRAF:
		return "RAF"
	case ServiceRCF:
		return "RCF"
	case ServiceROCF:
		return "ROCF"
	default:
		return "FCLTU"
	}
}

// bindFamilyTags are the tags every service shares for the association
// operations.
var bindFamilyTags = map[uint32]OperationType{
	100: OpBindInvocation,
	101: OpBindReturn,
	102: OpUnbindInvocation,
	103: OpUnbindReturn,
	104: OpPeerAbort,
}

// returnServiceTags are the operation tags the three return services share.
// RAF, RCF and ROCF assign the same numbers to the same operations.
var returnServiceTags = map[uint32]OperationType{
	0: OpStartInvocation,
	1: OpStartReturn,
	2: OpStopInvocation,
	3: OpStopReturn,
	4: OpScheduleStatusReportInvocation,
	5: OpScheduleStatusReportReturn,
	6: OpGetParameterInvocation,
	7: OpGetParameterReturn,
	8: OpTransferBuffer,
	9: OpStatusReportInvocation,
}

// fcltuTags are the Forward CLTU operation tags, which differ from the return
// services because the service does different things.
var fcltuTags = map[uint32]OperationType{
	0:  OpStartInvocation,
	1:  OpStartReturn,
	2:  OpStopInvocation,
	3:  OpStopReturn,
	4:  OpScheduleStatusReportInvocation,
	5:  OpScheduleStatusReportReturn,
	6:  OpGetParameterInvocation,
	7:  OpGetParameterReturn,
	8:  OpThrowEventInvocation,
	9:  OpThrowEventReturn,
	10: OpTransferDataInvocation,
	11: OpTransferDataReturn,
	12: OpAsyncNotifyInvocation,
	13: OpStatusReportInvocation,
}

// PDU is one decoded SLE protocol data unit: which operation it is, and its
// still-encoded content.
type PDU struct {
	Service   ServiceKind
	Operation OperationType
	// Tag is the wire tag it arrived under.
	Tag uint32
	// Content is the operation's encoded content, ready for the matching
	// decoder.
	Content []byte
}

// AppendPDU wraps an operation's content in its service tag.
func AppendPDU(dst []byte, tag uint32, content []byte) []byte {
	return AppendElement(dst, ClassContext, true, tag, content)
}

// DecodePDU reads the outer tag of an SLE PDU and reports which operation it
// names, leaving the content for the caller to decode.
func DecodePDU(data []byte, service ServiceKind) (*PDU, error) {
	e, err := NewDecoder(data).Next()
	if err != nil {
		return nil, err
	}
	if e.Class != ClassContext {
		return nil, ErrInvalidTag
	}

	p := &PDU{Service: service, Tag: e.Tag, Content: e.Bytes}

	if op, ok := bindFamilyTags[e.Tag]; ok {
		p.Operation = op
		return p, nil
	}

	table := returnServiceTags
	if service == ServiceFCLTU {
		table = fcltuTags
	}
	op, ok := table[e.Tag]
	if !ok {
		return nil, ErrInvalidTag
	}
	p.Operation = op
	return p, nil
}

// Humanize returns a human-readable summary.
func (p *PDU) Humanize() string {
	return fmt.Sprintf("SLE PDU\n  Service ..... %s\n  Operation ... %s\n  Tag ......... [%d]\n  Content ..... %d octets",
		p.Service, p.Operation, p.Tag, len(p.Content))
}
