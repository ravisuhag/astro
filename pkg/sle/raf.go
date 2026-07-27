package sle

import (
	"fmt"
	"time"
)

// Return All Frames, per CCSDS 911.1-B-5.
//
// RAF is the simplest return service and the one most missions start with: the
// provider delivers every telemetry frame it recovers from a physical channel,
// good and bad alike, and the user decides what to do with them.
//
// The operations, from the PDU CHOICEs of annex A2.6 and A2.7:
//
//	user → provider    BIND, UNBIND, START, STOP, SCHEDULE-STATUS-REPORT,
//	                   GET-PARAMETER, PEER-ABORT
//	provider → user    the returns for those, plus TRANSFER-BUFFER carrying
//	                   frames and notifications, and STATUS-REPORT

// RAF PDU tags, from the RafUsertoProviderPdu and RafProviderToUserPdu
// CHOICEs of annex A2.6 and A2.7.
//
// The BIND family shares tags [100] to [104] across every service; the
// service-specific operations take the low numbers.
const (
	TagRAFBindInvocation                 uint32 = 100
	TagRAFBindReturn                     uint32 = 101
	TagRAFUnbindInvocation               uint32 = 102
	TagRAFUnbindReturn                   uint32 = 103
	TagRAFPeerAbortInvocation            uint32 = 104
	TagRAFStartInvocation                uint32 = 0
	TagRAFStartReturn                    uint32 = 1
	TagRAFStopInvocation                 uint32 = 2
	TagRAFStopReturn                     uint32 = 3
	TagRAFScheduleStatusReportInvocation uint32 = 4
	TagRAFScheduleStatusReportReturn     uint32 = 5
	TagRAFGetParameterInvocation         uint32 = 6
	TagRAFGetParameterReturn             uint32 = 7
	TagRAFTransferBuffer                 uint32 = 8
	TagRAFStatusReportInvocation         uint32 = 9
)

// RequestedFrameQuality says which frames a START asks for, from the
// parReqFrameQuality values of the RAF structures module.
type RequestedFrameQuality int

const (
	// FrameQualityGoodOnly asks for frames that passed error control.
	FrameQualityGoodOnly RequestedFrameQuality = 0
	// FrameQualityErredOnly asks for frames that failed it.
	FrameQualityErredOnly RequestedFrameQuality = 1
	// FrameQualityAll asks for everything.
	FrameQualityAll RequestedFrameQuality = 2
)

// String names the requested quality.
func (r RequestedFrameQuality) String() string {
	switch r {
	case FrameQualityGoodOnly:
		return "good frames only"
	case FrameQualityErredOnly:
		return "erred frames only"
	case FrameQualityAll:
		return "all frames"
	default:
		return fmt.Sprintf("quality(%d)", int(r))
	}
}

// FrameQuality reports what a delivered frame turned out to be, from the
// FrameQuality INTEGER of the RAF structures module.
type FrameQuality int

const (
	// FrameGood passed error control.
	FrameGood FrameQuality = 0
	// FrameErred failed it.
	FrameErred FrameQuality = 1
	// FrameUndetermined could not be assessed.
	FrameUndetermined FrameQuality = 2
)

// String names the frame quality.
func (f FrameQuality) String() string {
	switch f {
	case FrameGood:
		return "good"
	case FrameErred:
		return "erred"
	default:
		return "undetermined"
	}
}

// RAFStartDiagnostic explains a refused START, from the DiagnosticRafStart
// CHOICE of the RAF structures module.
//
// The CHOICE has a common alternative carrying the shared Diagnostics and a
// specific one carrying these.
type RAFStartDiagnostic int

const (
	RAFStartOutOfService     RAFStartDiagnostic = 0
	RAFStartUnableToComply   RAFStartDiagnostic = 1
	RAFStartInvalidStartTime RAFStartDiagnostic = 2
	RAFStartInvalidStopTime  RAFStartDiagnostic = 3
	RAFStartMissingTimeValue RAFStartDiagnostic = 4
)

// String names the diagnostic.
func (r RAFStartDiagnostic) String() string {
	switch r {
	case RAFStartOutOfService:
		return "out of service"
	case RAFStartUnableToComply:
		return "unable to comply"
	case RAFStartInvalidStartTime:
		return "invalid start time"
	case RAFStartInvalidStopTime:
		return "invalid stop time"
	case RAFStartMissingTimeValue:
		return "missing time value"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(r))
	}
}

// RAFStartInvocation is the RafStartInvocation of annex A2.6. It asks the
// provider to begin delivering frames.
type RAFStartInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// StartTime and StopTime bound the requested range. Either may be
	// undefined, meaning "from now" and "until further notice".
	StartTime ConditionalTime
	StopTime  ConditionalTime
	// RequestedFrameQuality selects which frames to deliver.
	RequestedFrameQuality RequestedFrameQuality
}

// Encode serializes the START invocation's content.
func (s *RAFStartInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))
	content = AppendConditionalTime(content, s.StartTime)
	content = AppendConditionalTime(content, s.StopTime)
	content = AppendInteger(content, int64(s.RequestedFrameQuality))
	return content, nil
}

// DecodeRAFStartInvocation parses a START invocation's content.
func DecodeRAFStartInvocation(data []byte) (*RAFStartInvocation, error) {
	d := NewDecoder(data)
	s := &RAFStartInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	idElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	id, err := idElem.Uint64()
	if err != nil {
		return nil, err
	}
	if id > 65535 {
		return nil, ErrIntegerOverflow
	}
	s.InvokeId = InvokeId(id)

	startElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.StartTime, err = DecodeConditionalTime(startElem); err != nil {
		return nil, err
	}

	stopElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.StopTime, err = DecodeConditionalTime(stopElem); err != nil {
		return nil, err
	}

	qualityElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	q, err := qualityElem.Int64()
	if err != nil {
		return nil, err
	}
	s.RequestedFrameQuality = RequestedFrameQuality(q)
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RAFStartInvocation) Humanize() string {
	window := "from now until further notice"
	if s.StartTime.Known && s.StopTime.Known {
		window = s.StartTime.Time.Humanize() + " to " + s.StopTime.Time.Humanize()
	} else if s.StartTime.Known {
		window = "from " + s.StartTime.Time.Humanize()
	} else if s.StopTime.Known {
		window = "until " + s.StopTime.Time.Humanize()
	}
	return fmt.Sprintf("RAF START Invocation\n  Invoke ID ... %d\n  Window ...... %s\n  Quality ..... %s",
		s.InvokeId, window, s.RequestedFrameQuality)
}

// RAFStartReturn is the RafStartReturn of annex A2.7.
type RAFStartReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider accepted.
	Positive bool
	// CommonDiagnostic is set when the refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the RAF-specific one.
	SpecificDiagnostic RAFStartDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the START return's content.
func (s *RAFStartReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))

	if s.Positive {
		// result CHOICE { positiveResult [0] NULL, ... }
		return AppendElement(content, ClassContext, false, 0, nil), nil
	}

	// negativeResult [1] DiagnosticRafStart, itself a CHOICE of
	// common [0] Diagnostics and specific [1] INTEGER.
	var diagnostic []byte
	if s.UsedCommon {
		diagnostic = AppendTaggedInteger(nil, 0, int64(s.CommonDiagnostic))
	} else {
		diagnostic = AppendTaggedInteger(nil, 1, int64(s.SpecificDiagnostic))
	}
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeRAFStartReturn parses a START return's content.
func DecodeRAFStartReturn(data []byte) (*RAFStartReturn, error) {
	d := NewDecoder(data)
	s := &RAFStartReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	idElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	id, err := idElem.Uint64()
	if err != nil {
		return nil, err
	}
	if id > 65535 {
		return nil, ErrIntegerOverflow
	}
	s.InvokeId = InvokeId(id)

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		s.Positive = true
	case result.IsContext(1):
		inner, err := NewDecoder(result.Bytes).Next()
		if err != nil {
			return nil, err
		}
		v, err := inner.Int64()
		if err != nil {
			return nil, err
		}
		switch {
		case inner.IsContext(0):
			s.UsedCommon = true
			s.CommonDiagnostic = Diagnostics(v)
		case inner.IsContext(1):
			s.SpecificDiagnostic = RAFStartDiagnostic(v)
		default:
			return nil, ErrInvalidTag
		}
	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RAFStartReturn) Humanize() string {
	if s.Positive {
		return fmt.Sprintf("RAF START Return\n  Invoke ID ... %d\n  Result ...... accepted", s.InvokeId)
	}
	reason := s.SpecificDiagnostic.String()
	if s.UsedCommon {
		reason = s.CommonDiagnostic.String()
	}
	return fmt.Sprintf("RAF START Return\n  Invoke ID ... %d\n  Result ...... refused: %s", s.InvokeId, reason)
}

// RAFTransferDataInvocation is the RafTransferDataInvocation of annex A2.7:
// one telemetry frame with the metadata describing how it arrived.
type RAFTransferDataInvocation struct {
	Credentials *Credentials
	// EarthReceiveTime is when the frame reached the ground.
	EarthReceiveTime Time
	// AntennaId names the antenna that received it.
	AntennaId AntennaId
	// DataLinkContinuity counts frames lost since the last delivery, or -1
	// when the provider cannot tell. INTEGER (-1 .. 16777215).
	DataLinkContinuity int32
	// DeliveredFrameQuality is what the frame turned out to be.
	DeliveredFrameQuality FrameQuality
	// PrivateAnnotation is an optional provider-defined field, 1 to 128 octets.
	PrivateAnnotation []byte
	// Data is the frame itself: a CADU's frame content, as pkg/tmsc.UnwrapCADU
	// produces.
	Data []byte
}

// Encode serializes the TRANSFER-DATA invocation's content.
func (t *RAFTransferDataInvocation) Encode() ([]byte, error) {
	if len(t.Data) == 0 || len(t.Data) > MaxSpaceLinkDataUnit {
		return nil, ErrDataTooShort
	}
	if len(t.PrivateAnnotation) > 128 {
		return nil, ErrInvalidLength
	}

	content, err := AppendCredentialsChoice(nil, t.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendTimeChoice(content, t.EarthReceiveTime)
	content = AppendAntennaId(content, t.AntennaId)
	content = AppendInteger(content, int64(t.DataLinkContinuity))
	content = AppendInteger(content, int64(t.DeliveredFrameQuality))

	// privateAnnotation CHOICE { null [0] NULL, notNull [1] OCTET STRING }
	if len(t.PrivateAnnotation) == 0 {
		content = AppendElement(content, ClassContext, false, 0, nil)
	} else {
		content = AppendElement(content, ClassContext, false, 1, t.PrivateAnnotation)
	}

	return AppendOctetString(content, t.Data), nil
}

// DecodeRAFTransferDataInvocation parses a TRANSFER-DATA invocation's content.
func DecodeRAFTransferDataInvocation(data []byte) (*RAFTransferDataInvocation, error) {
	d := NewDecoder(data)
	t := &RAFTransferDataInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	timeElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.EarthReceiveTime, err = DecodeTimeChoice(timeElem); err != nil {
		return nil, err
	}

	antennaElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.AntennaId, err = DecodeAntennaId(antennaElem); err != nil {
		return nil, err
	}

	continuityElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	continuity, err := continuityElem.Int64()
	if err != nil {
		return nil, err
	}
	if continuity < -1 || continuity > 16777215 {
		return nil, ErrIntegerOverflow
	}
	t.DataLinkContinuity = int32(continuity)

	qualityElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	q, err := qualityElem.Int64()
	if err != nil {
		return nil, err
	}
	t.DeliveredFrameQuality = FrameQuality(q)

	annotationElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case annotationElem.IsContext(0):
	case annotationElem.IsContext(1):
		t.PrivateAnnotation = annotationElem.Copy()
	default:
		return nil, ErrInvalidTag
	}

	dataElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if len(dataElem.Bytes) == 0 || len(dataElem.Bytes) > MaxSpaceLinkDataUnit {
		return nil, ErrDataTooShort
	}
	t.Data = dataElem.Copy()
	return t, nil
}

// Humanize returns a human-readable summary.
func (t *RAFTransferDataInvocation) Humanize() string {
	return fmt.Sprintf("RAF TRANSFER-DATA\n"+
		"  Received .... %s\n"+
		"  Antenna ..... %s\n"+
		"  Continuity .. %d\n"+
		"  Quality ..... %s\n"+
		"  Frame ....... %d octets",
		t.EarthReceiveTime.Humanize(), t.AntennaId,
		t.DataLinkContinuity, t.DeliveredFrameQuality, len(t.Data))
}

// RAFStatusReportInvocation is the RafStatusReportInvocation of annex A2.7:
// a periodic summary of how the channel is doing.
type RAFStatusReportInvocation struct {
	Credentials *Credentials
	// ErrorFreeFrameNumber counts frames delivered that passed error control.
	ErrorFreeFrameNumber uint32
	// DeliveredFrameNumber counts every frame delivered.
	DeliveredFrameNumber uint32

	FrameSyncLockStatus  LockStatus
	SymbolSyncLockStatus LockStatus
	SubcarrierLockStatus LockStatus
	CarrierLockStatus    LockStatus
	ProductionStatus     ProductionStatus
}

// Encode serializes the STATUS-REPORT invocation's content.
func (s *RAFStatusReportInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.ErrorFreeFrameNumber))
	content = AppendInteger(content, int64(s.DeliveredFrameNumber))
	content = AppendInteger(content, int64(s.FrameSyncLockStatus))
	content = AppendInteger(content, int64(s.SymbolSyncLockStatus))
	content = AppendInteger(content, int64(s.SubcarrierLockStatus))
	content = AppendInteger(content, int64(s.CarrierLockStatus))
	content = AppendInteger(content, int64(s.ProductionStatus))
	return content, nil
}

// DecodeRAFStatusReportInvocation parses a STATUS-REPORT invocation's content.
func DecodeRAFStatusReportInvocation(data []byte) (*RAFStatusReportInvocation, error) {
	d := NewDecoder(data)
	s := &RAFStatusReportInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	counts := []*uint32{&s.ErrorFreeFrameNumber, &s.DeliveredFrameNumber}
	for _, target := range counts {
		e, err := d.Next()
		if err != nil {
			return nil, err
		}
		v, err := e.Uint64()
		if err != nil {
			return nil, err
		}
		if v > 4294967295 {
			return nil, ErrIntegerOverflow
		}
		*target = uint32(v)
	}

	locks := []*LockStatus{
		&s.FrameSyncLockStatus, &s.SymbolSyncLockStatus,
		&s.SubcarrierLockStatus, &s.CarrierLockStatus,
	}
	for _, target := range locks {
		e, err := d.Next()
		if err != nil {
			return nil, err
		}
		v, err := e.Int64()
		if err != nil {
			return nil, err
		}
		*target = LockStatus(v)
	}

	prodElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	p, err := prodElem.Int64()
	if err != nil {
		return nil, err
	}
	s.ProductionStatus = ProductionStatus(p)
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RAFStatusReportInvocation) Humanize() string {
	return fmt.Sprintf("RAF STATUS-REPORT\n"+
		"  Delivered ....... %d frames\n"+
		"  Error free ...... %d frames\n"+
		"  Frame sync ...... %s\n"+
		"  Carrier lock .... %s\n"+
		"  Production ...... %s",
		s.DeliveredFrameNumber, s.ErrorFreeFrameNumber,
		s.FrameSyncLockStatus, s.CarrierLockStatus, s.ProductionStatus)
}

// TransferBufferEntry is one element of a RafTransferBuffer: either a frame or
// a notification.
//
//	FrameOrNotification ::= CHOICE
//	{ annotatedFrame   [0] RafTransferDataInvocation
//	, syncNotification [1] RafSyncNotifyInvocation
//	}
type TransferBufferEntry struct {
	Frame        *RAFTransferDataInvocation
	Notification *SyncNotifyInvocation
}

// RAFTransferBuffer is the RafTransferBuffer of annex A2.7: a SEQUENCE OF
// frames and notifications, delivered together.
//
// Buffering is why RAF scales. A provider recovering frames at line rate does
// not send one PDU per frame; it fills a buffer and ships it, so the TCP
// connection carries a few large messages rather than thousands of small ones.
type RAFTransferBuffer []TransferBufferEntry

// Encode serializes the transfer buffer's content.
func (b RAFTransferBuffer) Encode() ([]byte, error) {
	var content []byte
	for _, entry := range b {
		switch {
		case entry.Frame != nil:
			inner, err := entry.Frame.Encode()
			if err != nil {
				return nil, err
			}
			content = AppendElement(content, ClassContext, true, 0, inner)
		case entry.Notification != nil:
			inner, err := entry.Notification.Encode()
			if err != nil {
				return nil, err
			}
			content = AppendElement(content, ClassContext, true, 1, inner)
		default:
			return nil, ErrDataTooShort
		}
	}
	return content, nil
}

// DecodeRAFTransferBuffer parses a transfer buffer's content.
func DecodeRAFTransferBuffer(data []byte) (RAFTransferBuffer, error) {
	d := NewDecoder(data)
	var buffer RAFTransferBuffer

	for !d.Empty() {
		e, err := d.Next()
		if err != nil {
			return nil, err
		}
		switch {
		case e.IsContext(0):
			frame, err := DecodeRAFTransferDataInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, TransferBufferEntry{Frame: frame})
		case e.IsContext(1):
			notification, err := DecodeSyncNotifyInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, TransferBufferEntry{Notification: notification})
		default:
			return nil, ErrInvalidTag
		}
	}
	return buffer, nil
}

// Frames returns just the frames in the buffer, which is what a user normally
// wants.
func (b RAFTransferBuffer) Frames() []*RAFTransferDataInvocation {
	out := make([]*RAFTransferDataInvocation, 0, len(b))
	for _, entry := range b {
		if entry.Frame != nil {
			out = append(out, entry.Frame)
		}
	}
	return out
}

// Humanize returns a human-readable summary.
func (b RAFTransferBuffer) Humanize() string {
	frames, notifications := 0, 0
	for _, entry := range b {
		if entry.Frame != nil {
			frames++
		} else {
			notifications++
		}
	}
	return fmt.Sprintf("RAF TRANSFER-BUFFER\n  Frames ......... %d\n  Notifications .. %d",
		frames, notifications)
}

// RAFUser is the user half of a RAF service instance.
//
// The lifecycle is the one the state table of CCSDS 911.1-B-5 §4.2.2 walks:
// Bind, wait for the return, Start, then pull transfer buffers until you have
// what you came for, Stop, Unbind. Each call queues a PDU; NextPDU hands it
// to you to write, and HandlePDU takes what comes back.
type RAFUser struct {
	*ServiceUser
}

// NewRAFUser prepares the user half of a RAF instance. The configuration's
// Kind is set for you.
func NewRAFUser(config ServiceConfig) (*RAFUser, error) {
	config.Kind = ServiceRAF
	user, err := NewServiceUser(config)
	if err != nil {
		return nil, err
	}
	return &RAFUser{ServiceUser: user}, nil
}

// Start asks the provider to begin delivering frames. State 2 only, per
// §3.4.1.7.
func (u *RAFUser) Start(
	now time.Time, randomNumber int32,
	start, stop ConditionalTime, quality RequestedFrameQuality,
) (InvokeId, error) {
	// The time range is not validated against the delivery mode here. Online
	// delivery reads a live channel, so a past range will simply return
	// nothing; offline reads a store, so a past range is the normal case. The
	// provider answers with 'invalid start time' when it disagrees, and
	// guessing on its behalf would refuse ranges some providers accept.
	return u.invoke(OpStartInvocation, ServiceReady, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&RAFStartInvocation{
				Credentials:           creds,
				InvokeId:              id,
				StartTime:             start,
				StopTime:              stop,
				RequestedFrameQuality: quality,
			}).Encode()
		})
}

// HandleStartReturn takes the answer to START. A positive answer moves to
// state 3.
func (u *RAFUser) HandleStartReturn(r *RAFStartReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.settle(r.InvokeId, OpStartInvocation); err != nil {
		return err
	}
	if r.Positive {
		u.startAccepted()
	}
	return nil
}

// RAFUserEvent is one decoded PDU arriving at the user. Exactly one field is
// set, and Operation says which.
type RAFUserEvent struct {
	Operation OperationType

	BindReturn                 *BindReturn
	UnbindReturn               *UnbindReturn
	StartReturn                *RAFStartReturn
	StopReturn                 *Acknowledgement
	ScheduleStatusReportReturn *ScheduleStatusReportReturn
	TransferBuffer             RAFTransferBuffer
	StatusReport               *RAFStatusReportInvocation
	PeerAbort                  *PeerAbort
}

// HandlePDU decodes one PDU from the provider, advances the state machine and
// returns what arrived.
//
// A PDU the state does not allow is answered with a PEER-ABORT for protocol
// error, queued for sending, and reported as ErrUnexpectedPDU — which is what
// every 'peer abort protocol error' cell of the state table says to do.
func (u *RAFUser) HandlePDU(data []byte, now time.Time) (*RAFUserEvent, error) {
	pdu, err := DecodePDU(data, ServiceRAF)
	if err != nil {
		return nil, err
	}
	u.Association().RecordReceived(now)

	event := &RAFUserEvent{Operation: pdu.Operation}

	switch pdu.Operation {
	case OpBindReturn:
		r, err := DecodeBindReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.BindReturn = r
		return event, u.HandleBindReturn(r, now)

	case OpUnbindReturn:
		r, err := DecodeUnbindReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.UnbindReturn = r
		return event, u.HandleUnbindReturn(r, now)

	case OpStartReturn:
		r, err := DecodeRAFStartReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.StartReturn = r
		return event, u.HandleStartReturn(r)

	case OpStopReturn:
		r, err := DecodeAcknowledgement(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.StopReturn = r
		return event, u.HandleStopReturn(r)

	case OpScheduleStatusReportReturn:
		r, err := DecodeScheduleStatusReportReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.ScheduleStatusReportReturn = r
		return event, u.HandleScheduleStatusReportReturn(r)

	case OpTransferBuffer:
		if u.State() != ServiceActive {
			u.PeerAbort(AbortProtocolError, now)
			return event, ErrUnexpectedPDU
		}
		buffer, err := DecodeRAFTransferBuffer(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.TransferBuffer = buffer
		return event, nil

	case OpStatusReportInvocation:
		report, err := DecodeRAFStatusReportInvocation(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.StatusReport = report
		return event, nil

	case OpPeerAbort:
		abort, err := DecodePeerAbort(pdu.Content)
		if err != nil {
			return nil, err
		}
		event.PeerAbort = abort
		u.HandlePeerAbort(abort, now)
		return event, nil

	default:
		u.PeerAbort(AbortProtocolError, now)
		return event, ErrUnexpectedPDU
	}
}

// RAFProvider is the provider half of a RAF instance. It is partial; see
// ServiceProvider for what that means.
type RAFProvider struct {
	*ServiceProvider
}

// NewRAFProvider prepares the provider half of a RAF instance.
func NewRAFProvider(config ServiceConfig) (*RAFProvider, error) {
	config.Kind = ServiceRAF
	provider, err := NewServiceProvider(config)
	if err != nil {
		return nil, err
	}
	return &RAFProvider{ServiceProvider: provider}, nil
}

// HandleStartInvocation answers a START. Accepting moves to state 3, which is
// state table row 9's positive branch.
func (p *RAFProvider) HandleStartInvocation(
	s *RAFStartInvocation, answer *RAFStartReturn, now time.Time, randomNumber int32,
) error {
	err := p.respond(OpStartReturn, ServiceReady, now, randomNumber,
		func(creds *Credentials) ([]byte, error) {
			answer.Credentials = creds
			answer.InvokeId = s.InvokeId
			return answer.Encode()
		})
	if err != nil {
		return err
	}
	if answer.Positive {
		p.mu.Lock()
		p.state = ServiceActive
		p.mu.Unlock()
	}
	return nil
}

// SendTransferBuffer queues a buffer of frames and notifications. State 3
// only, per §3.6.1.3.
func (p *RAFProvider) SendTransferBuffer(buffer RAFTransferBuffer, now time.Time) error {
	content, err := buffer.Encode()
	if err != nil {
		return err
	}
	return p.sendWhileActive(OpTransferBuffer, content, now)
}

// RAFProviderEvent is one decoded PDU arriving at the provider.
type RAFProviderEvent struct {
	Operation OperationType

	BindInvocation                 *BindInvocation
	UnbindInvocation               *UnbindInvocation
	StartInvocation                *RAFStartInvocation
	StopInvocation                 *StopInvocation
	ScheduleStatusReportInvocation *ScheduleStatusReportInvocation
	PeerAbort                      *PeerAbort
}

// HandlePDU decodes one PDU from the user. Unlike the user's HandlePDU this
// only decodes and reports: the answer is the caller's to compose, because
// only the caller knows whether the provider can comply.
func (p *RAFProvider) HandlePDU(data []byte, now time.Time) (*RAFProviderEvent, error) {
	pdu, err := DecodePDU(data, ServiceRAF)
	if err != nil {
		return nil, err
	}
	p.Association().RecordReceived(now)

	event := &RAFProviderEvent{Operation: pdu.Operation}

	switch pdu.Operation {
	case OpBindInvocation:
		event.BindInvocation, err = DecodeBindInvocation(pdu.Content)
	case OpUnbindInvocation:
		event.UnbindInvocation, err = DecodeUnbindInvocation(pdu.Content)
	case OpStartInvocation:
		event.StartInvocation, err = DecodeRAFStartInvocation(pdu.Content)
	case OpStopInvocation:
		event.StopInvocation, err = DecodeStopInvocation(pdu.Content)
	case OpScheduleStatusReportInvocation:
		event.ScheduleStatusReportInvocation, err = DecodeScheduleStatusReportInvocation(pdu.Content)
	case OpPeerAbort:
		abort, abortErr := DecodePeerAbort(pdu.Content)
		if abortErr != nil {
			return nil, abortErr
		}
		event.PeerAbort = abort
		p.HandlePeerAbort(abort, now)
		return event, nil
	default:
		p.PeerAbort(AbortProtocolError, now)
		return event, ErrUnexpectedPDU
	}
	if err != nil {
		return nil, err
	}
	return event, nil
}
