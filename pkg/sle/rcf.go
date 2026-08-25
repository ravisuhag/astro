package sle

import (
	"fmt"
	"time"
)

// Return Channel Frames, per CCSDS 911.2-B-4.
//
// RCF is RAF with a filter. Where RAF delivers every frame from a physical
// channel, RCF delivers only those on one master or virtual channel, named by
// a global virtual channel identifier.
//
// The operation set and its tags are the same as RAF's — the return services
// share them — so this file adds only what differs: the START invocation
// carries a GVCID instead of a frame quality, and TRANSFER-DATA has no
// quality field, because RCF delivers only frames that passed error control.

// RCFStartDiagnostic explains a refused START, from the DiagnosticRcfStart
// CHOICE of the RCF structures module.
type RCFStartDiagnostic int

const (
	RCFStartOutOfService     RCFStartDiagnostic = 0
	RCFStartUnableToComply   RCFStartDiagnostic = 1
	RCFStartInvalidStartTime RCFStartDiagnostic = 2
	RCFStartInvalidStopTime  RCFStartDiagnostic = 3
	RCFStartMissingTimeValue RCFStartDiagnostic = 4
	RCFStartInvalidGVCID     RCFStartDiagnostic = 5
)

// String names the diagnostic.
func (r RCFStartDiagnostic) String() string {
	switch r {
	case RCFStartOutOfService:
		return "out of service"
	case RCFStartUnableToComply:
		return "unable to comply"
	case RCFStartInvalidStartTime:
		return "invalid start time"
	case RCFStartInvalidStopTime:
		return "invalid stop time"
	case RCFStartMissingTimeValue:
		return "missing time value"
	case RCFStartInvalidGVCID:
		return "invalid global virtual channel identifier"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(r))
	}
}

// RCFStartInvocation is the RcfStartInvocation of the RCF incoming PDUs
// module: like RAF's, but naming a channel rather than a frame quality.
type RCFStartInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	StartTime   ConditionalTime
	StopTime    ConditionalTime
	// RequestedGVCID names the master or virtual channel to deliver.
	RequestedGVCID GVCID
}

// Encode serializes the START invocation's content.
func (s *RCFStartInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))
	content = AppendConditionalTime(content, s.StartTime)
	content = AppendConditionalTime(content, s.StopTime)
	return AppendGVCID(content, s.RequestedGVCID)
}

// DecodeRCFStartInvocation parses a START invocation's content.
func DecodeRCFStartInvocation(data []byte) (*RCFStartInvocation, error) {
	d := NewDecoder(data)
	s := &RCFStartInvocation{}

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

	gvcidElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.RequestedGVCID, err = DecodeGVCID(gvcidElem); err != nil {
		return nil, err
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RCFStartInvocation) Humanize() string {
	return fmt.Sprintf("RCF START Invocation\n  Invoke ID ... %d\n  Channel ..... %s",
		s.InvokeId, s.RequestedGVCID)
}

// RCFTransferDataInvocation is the RcfTransferDataInvocation of the RCF
// outgoing PDUs module.
//
// It differs from RAF's by having no delivered-frame-quality field: RCF
// delivers only frames that passed error control, so there is nothing to
// report.
type RCFTransferDataInvocation struct {
	Credentials        *Credentials
	EarthReceiveTime   Time
	AntennaId          AntennaId
	DataLinkContinuity int32
	PrivateAnnotation  []byte
	Data               []byte
}

// Encode serializes the TRANSFER-DATA invocation's content.
func (t *RCFTransferDataInvocation) Encode() ([]byte, error) {
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

	if len(t.PrivateAnnotation) == 0 {
		content = AppendElement(content, ClassContext, false, 0, nil)
	} else {
		content = AppendElement(content, ClassContext, false, 1, t.PrivateAnnotation)
	}
	return AppendOctetString(content, t.Data), nil
}

// DecodeRCFTransferDataInvocation parses a TRANSFER-DATA invocation's content.
func DecodeRCFTransferDataInvocation(data []byte) (*RCFTransferDataInvocation, error) {
	d := NewDecoder(data)
	t := &RCFTransferDataInvocation{}

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
func (t *RCFTransferDataInvocation) Humanize() string {
	return fmt.Sprintf("RCF TRANSFER-DATA\n  Received .... %s\n  Antenna ..... %s\n  Frame ....... %d octets",
		t.EarthReceiveTime.Humanize(), t.AntennaId, len(t.Data))
}

// RCFStartReturn is the RcfStartReturn of annex A2.7.
//
// Unlike FCLTU's, its positive result is an empty NULL: there is nothing to
// tell the user beyond "yes".
type RCFStartReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider accepted.
	Positive bool
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the RCF-specific one.
	SpecificDiagnostic RCFStartDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the START return's content.
func (s *RCFStartReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))

	if s.Positive {
		return AppendElement(content, ClassContext, false, 0, nil), nil
	}
	diagnostic := appendCommonOrSpecific(s.UsedCommon, s.CommonDiagnostic, int64(s.SpecificDiagnostic))
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeRCFStartReturn parses a START return's content.
func DecodeRCFStartReturn(data []byte) (*RCFStartReturn, error) {
	d := NewDecoder(data)
	s := &RCFStartReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if s.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		s.Positive = true
	case result.IsContext(1):
		usedCommon, v, err := decodeCommonOrSpecific(result)
		if err != nil {
			return nil, err
		}
		s.UsedCommon = usedCommon
		if usedCommon {
			s.CommonDiagnostic = Diagnostics(v)
		} else {
			s.SpecificDiagnostic = RCFStartDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RCFStartReturn) Humanize() string {
	if s.Positive {
		return fmt.Sprintf("RCF START Return\n  Invoke ID ... %d\n  Result ...... accepted", s.InvokeId)
	}
	reason := s.SpecificDiagnostic.String()
	if s.UsedCommon {
		reason = s.CommonDiagnostic.String()
	}
	return fmt.Sprintf("RCF START Return\n  Invoke ID ... %d\n  Result ...... refused: %s", s.InvokeId, reason)
}

// RCFStatusReportInvocation is the RcfStatusReportInvocation of annex A2.7.
//
// It is RAF's status report with one field missing. RAF counts frames twice —
// delivered, and of those, error free — because RAF can deliver bad frames.
// RCF only ever delivers good ones, so the error-free count would say nothing
// and the spec leaves it out.
type RCFStatusReportInvocation struct {
	Credentials *Credentials
	// DeliveredFrameNumber counts frames delivered on the requested channel.
	DeliveredFrameNumber uint32

	FrameSyncLockStatus  LockStatus
	SymbolSyncLockStatus LockStatus
	SubcarrierLockStatus LockStatus
	CarrierLockStatus    LockStatus
	ProductionStatus     ProductionStatus
}

// Encode serializes the STATUS-REPORT invocation's content.
func (s *RCFStatusReportInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.DeliveredFrameNumber))
	content = AppendInteger(content, int64(s.FrameSyncLockStatus))
	content = AppendInteger(content, int64(s.SymbolSyncLockStatus))
	content = AppendInteger(content, int64(s.SubcarrierLockStatus))
	content = AppendInteger(content, int64(s.CarrierLockStatus))
	return AppendInteger(content, int64(s.ProductionStatus)), nil
}

// DecodeRCFStatusReportInvocation parses a STATUS-REPORT invocation's content.
func DecodeRCFStatusReportInvocation(data []byte) (*RCFStatusReportInvocation, error) {
	d := NewDecoder(data)
	s := &RCFStatusReportInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	if s.DeliveredFrameNumber, err = decodeUint32(d); err != nil {
		return nil, err
	}

	locks := []*LockStatus{
		&s.FrameSyncLockStatus, &s.SymbolSyncLockStatus,
		&s.SubcarrierLockStatus, &s.CarrierLockStatus,
	}
	for _, target := range locks {
		v, err := decodeInt(d)
		if err != nil {
			return nil, err
		}
		*target = LockStatus(v)
	}

	production, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	s.ProductionStatus = ProductionStatus(production)
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *RCFStatusReportInvocation) Humanize() string {
	return fmt.Sprintf("RCF STATUS-REPORT\n"+
		"  Delivered ....... %d frames\n"+
		"  Frame sync ...... %s\n"+
		"  Carrier lock .... %s\n"+
		"  Production ...... %s",
		s.DeliveredFrameNumber, s.FrameSyncLockStatus,
		s.CarrierLockStatus, s.ProductionStatus)
}

// RCFTransferBufferEntry is one element of an RcfTransferBuffer.
//
//	FrameOrNotification ::= CHOICE
//	{ annotatedFrame   [0] RcfTransferDataInvocation
//	, syncNotification [1] RcfSyncNotifyInvocation
//	}
type RCFTransferBufferEntry struct {
	Frame        *RCFTransferDataInvocation
	Notification *SyncNotifyInvocation
}

// RCFTransferBuffer is the RcfTransferBuffer of annex A2.7: a SEQUENCE OF
// frames and notifications, delivered together for the same reason RAF
// buffers.
type RCFTransferBuffer []RCFTransferBufferEntry

// Encode serializes the transfer buffer's content.
func (b RCFTransferBuffer) Encode() ([]byte, error) {
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

// DecodeRCFTransferBuffer parses a transfer buffer's content.
func DecodeRCFTransferBuffer(data []byte) (RCFTransferBuffer, error) {
	d := NewDecoder(data)
	var buffer RCFTransferBuffer

	for !d.Empty() {
		e, err := d.Next()
		if err != nil {
			return nil, err
		}
		switch {
		case e.IsContext(0):
			frame, err := DecodeRCFTransferDataInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, RCFTransferBufferEntry{Frame: frame})
		case e.IsContext(1):
			notification, err := DecodeSyncNotifyInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, RCFTransferBufferEntry{Notification: notification})
		default:
			return nil, ErrInvalidTag
		}
	}
	return buffer, nil
}

// Frames returns just the frames in the buffer.
func (b RCFTransferBuffer) Frames() []*RCFTransferDataInvocation {
	out := make([]*RCFTransferDataInvocation, 0, len(b))
	for _, entry := range b {
		if entry.Frame != nil {
			out = append(out, entry.Frame)
		}
	}
	return out
}

// Humanize returns a human-readable summary.
func (b RCFTransferBuffer) Humanize() string {
	frames, notifications := 0, 0
	for _, entry := range b {
		if entry.Frame != nil {
			frames++
		} else {
			notifications++
		}
	}
	return fmt.Sprintf("RCF TRANSFER-BUFFER\n  Frames ......... %d\n  Notifications .. %d",
		frames, notifications)
}

// RCFUser is the user half of an RCF service instance.
//
// It is RAF's machine with one difference in START: the user names a channel
// instead of a frame quality, and gets only that channel's frames.
type RCFUser struct {
	*ServiceUser
}

// NewRCFUser prepares the user half of an RCF instance.
func NewRCFUser(config ServiceConfig) (*RCFUser, error) {
	config.Kind = ServiceRCF
	user, err := NewServiceUser(config)
	if err != nil {
		return nil, err
	}
	return &RCFUser{ServiceUser: user}, nil
}

// Start asks the provider to begin delivering one channel's frames. State 2
// only.
func (u *RCFUser) Start(
	now time.Time, randomNumber int32, start, stop ConditionalTime, channel GVCID,
) (InvokeId, error) {
	if err := channel.Validate(); err != nil {
		return 0, err
	}
	return u.invoke(OpStartInvocation, ServiceReady, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&RCFStartInvocation{
				Credentials:    creds,
				InvokeId:       id,
				StartTime:      start,
				StopTime:       stop,
				RequestedGVCID: channel,
			}).Encode()
		})
}

// HandleStartReturn takes the answer to START, moving to state 3 when
// positive.
func (u *RCFUser) HandleStartReturn(r *RCFStartReturn) error {
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

// RCFUserEvent is one decoded PDU arriving at the user.
type RCFUserEvent struct {
	Operation OperationType

	BindReturn                 *BindReturn
	UnbindReturn               *UnbindReturn
	StartReturn                *RCFStartReturn
	StopReturn                 *Acknowledgement
	ScheduleStatusReportReturn *ScheduleStatusReportReturn
	GetParameterReturn         *GetParameterReturn
	TransferBuffer             RCFTransferBuffer
	StatusReport               *RCFStatusReportInvocation
	PeerAbort                  *PeerAbort
}

// HandlePDU decodes one PDU from the provider and advances the machine.
func (u *RCFUser) HandlePDU(data []byte, now time.Time) (*RCFUserEvent, error) {
	pdu, err := DecodePDU(data, ServiceRCF)
	if err != nil {
		return nil, err
	}
	u.Association().RecordReceived(now)

	event := &RCFUserEvent{Operation: pdu.Operation}

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
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.UnbindReturn = r
		return event, u.HandleUnbindReturn(r, now)

	case OpStartReturn:
		r, err := DecodeRCFStartReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.StartReturn = r
		return event, u.HandleStartReturn(r)

	case OpStopReturn:
		r, err := DecodeAcknowledgement(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.StopReturn = r
		return event, u.HandleStopReturn(r)

	case OpScheduleStatusReportReturn:
		r, err := DecodeScheduleStatusReportReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.ScheduleStatusReportReturn = r
		return event, u.HandleScheduleStatusReportReturn(r)

	case OpGetParameterReturn:
		r, err := DecodeGetParameterReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.GetParameterReturn = r
		return event, u.HandleGetParameterReturn(r)

	case OpTransferBuffer:
		if u.State() != ServiceActive {
			u.PeerAbort(AbortProtocolError, now)
			return event, ErrUnexpectedPDU
		}
		buffer, err := DecodeRCFTransferBuffer(pdu.Content)
		if err != nil {
			return nil, err
		}
		for _, entry := range buffer {
			creds := (*Credentials)(nil)
			switch {
			case entry.Frame != nil:
				creds = entry.Frame.Credentials
			case entry.Notification != nil:
				creds = entry.Notification.Credentials
			}
			if err := u.authenticate(creds, now); err != nil {
				return nil, err
			}
		}
		event.TransferBuffer = buffer
		return event, nil

	case OpStatusReportInvocation:
		// Table 4-1: a STATUS-REPORT is legal only on a bound association.
		if u.State() == ServiceUnbound {
			u.PeerAbort(AbortProtocolError, now)
			return event, ErrUnexpectedPDU
		}
		report, err := DecodeRCFStatusReportInvocation(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(report.Credentials, now); err != nil {
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

// RCFProvider is the provider half of an RCF instance. Partial, like the rest.
type RCFProvider struct {
	*ServiceProvider
}

// NewRCFProvider prepares the provider half of an RCF instance.
func NewRCFProvider(config ServiceConfig) (*RCFProvider, error) {
	config.Kind = ServiceRCF
	provider, err := NewServiceProvider(config)
	if err != nil {
		return nil, err
	}
	return &RCFProvider{ServiceProvider: provider}, nil
}

// HandleStartInvocation answers a START, moving to state 3 when it accepts.
func (p *RCFProvider) HandleStartInvocation(
	s *RCFStartInvocation, answer *RCFStartReturn, now time.Time, randomNumber int32,
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
// only.
func (p *RCFProvider) SendTransferBuffer(buffer RCFTransferBuffer, now time.Time) error {
	content, err := buffer.Encode()
	if err != nil {
		return err
	}
	return p.sendWhileActive(OpTransferBuffer, content, now)
}
