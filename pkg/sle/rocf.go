package sle

import (
	"fmt"
	"time"
)

// Return Operational Control Fields, per CCSDS 911.5-B-4.
//
// ROCF delivers only the Operational Control Field out of each telemetry
// frame, not the whole frame. In practice that field carries a CLCW (the
// telecommand acknowledgement, which pkg/cop decodes) so ROCF is how a
// control centre learns whether its commands got through, without paying for
// the whole downlink.
//
// A START names a channel like RCF's does, and adds two more filters: which
// kind of control word to deliver, and whether to send every one or only
// those that changed.

// ControlWordKind selects which control words to deliver, from the
// ControlWordType CHOICE of the ROCF structures module.
type ControlWordKind int

const (
	// ControlWordAll delivers every operational control field.
	ControlWordAll ControlWordKind = 0
	// ControlWordCLCW delivers only CLCWs, for one telecommand virtual
	// channel or for all of them.
	ControlWordCLCW ControlWordKind = 1
	// ControlWordNotCLCW delivers only fields that are not CLCWs.
	ControlWordNotCLCW ControlWordKind = 2
)

// String names the kind.
func (c ControlWordKind) String() string {
	switch c {
	case ControlWordAll:
		return "all control words"
	case ControlWordCLCW:
		return "CLCW"
	default:
		return "not CLCW"
	}
}

// ControlWordType is the ROCF filter on which control words to deliver.
//
//	ControlWordType ::= CHOICE
//	{ allControlWords [0] NULL
//	, clcw            [1] TcVcid
//	, notClcw         [2] NULL
//	}
//
// The clcw alternative carries a TcVcid, itself a CHOICE of "no telecommand
// virtual channel" and a specific one, so a caller can ask for CLCWs from one
// TC virtual channel, or from any.
type ControlWordType struct {
	Kind ControlWordKind
	// TCVirtualChannel names one telecommand virtual channel when Kind is
	// ControlWordCLCW and HasTCVirtualChannel is set.
	TCVirtualChannel    uint8
	HasTCVirtualChannel bool
}

// AppendControlWordType writes a ControlWordType CHOICE.
func AppendControlWordType(dst []byte, c ControlWordType) []byte {
	switch c.Kind {
	case ControlWordCLCW:
		// TcVcid CHOICE { noTcVC [0] NULL, tcVcid [1] VcId }
		var inner []byte
		if c.HasTCVirtualChannel {
			inner = AppendTaggedInteger(nil, 1, int64(c.TCVirtualChannel))
		} else {
			inner = AppendElement(nil, ClassContext, false, 0, nil)
		}
		return AppendElement(dst, ClassContext, true, 1, inner)
	case ControlWordNotCLCW:
		return AppendElement(dst, ClassContext, false, 2, nil)
	default:
		return AppendElement(dst, ClassContext, false, 0, nil)
	}
}

// DecodeControlWordType reads a ControlWordType CHOICE.
func DecodeControlWordType(e *Element) (ControlWordType, error) {
	switch {
	case e.IsContext(0):
		return ControlWordType{Kind: ControlWordAll}, nil
	case e.IsContext(2):
		return ControlWordType{Kind: ControlWordNotCLCW}, nil
	case e.IsContext(1):
		c := ControlWordType{Kind: ControlWordCLCW}
		inner, err := NewDecoder(e.Bytes).Next()
		if err != nil {
			return c, err
		}
		switch {
		case inner.IsContext(0):
		case inner.IsContext(1):
			vc, err := inner.Uint64()
			if err != nil {
				return c, err
			}
			// VcId ::= INTEGER (0 .. 63).
			if vc > 63 {
				return c, ErrInvalidIdentifier
			}
			c.TCVirtualChannel = uint8(vc)
			c.HasTCVirtualChannel = true
		default:
			return c, ErrInvalidTag
		}
		return c, nil
	default:
		return ControlWordType{}, ErrInvalidTag
	}
}

// String renders the filter.
func (c ControlWordType) String() string {
	if c.Kind == ControlWordCLCW && c.HasTCVirtualChannel {
		return fmt.Sprintf("CLCW from TC virtual channel %d", c.TCVirtualChannel)
	}
	return c.Kind.String()
}

// UpdateMode says whether to deliver every control field or only changes,
// from the RequestedUpdateMode INTEGER of the ROCF structures module.
type UpdateMode int

const (
	// UpdateContinuous delivers every operational control field.
	UpdateContinuous UpdateMode = 0
	// UpdateChangeBased delivers only fields differing from the last one sent.
	//
	// A CLCW usually repeats unchanged for many frames, so this cuts the
	// downlink cost of watching one dramatically.
	UpdateChangeBased UpdateMode = 1
)

// String names the update mode.
func (u UpdateMode) String() string {
	if u == UpdateChangeBased {
		return "change based"
	}
	return "continuous"
}

// ROCFStartDiagnostic explains a refused START, from the DiagnosticRocfStart
// CHOICE of the ROCF structures module.
type ROCFStartDiagnostic int

const (
	ROCFStartOutOfService           ROCFStartDiagnostic = 0
	ROCFStartUnableToComply         ROCFStartDiagnostic = 1
	ROCFStartInvalidStartTime       ROCFStartDiagnostic = 2
	ROCFStartInvalidStopTime        ROCFStartDiagnostic = 3
	ROCFStartMissingTimeValue       ROCFStartDiagnostic = 4
	ROCFStartInvalidGVCID           ROCFStartDiagnostic = 5
	ROCFStartInvalidControlWordType ROCFStartDiagnostic = 6
	ROCFStartInvalidTcVcid          ROCFStartDiagnostic = 7
	ROCFStartInvalidUpdateMode      ROCFStartDiagnostic = 8
)

// String names the diagnostic.
func (r ROCFStartDiagnostic) String() string {
	switch r {
	case ROCFStartOutOfService:
		return "out of service"
	case ROCFStartUnableToComply:
		return "unable to comply"
	case ROCFStartInvalidStartTime:
		return "invalid start time"
	case ROCFStartInvalidStopTime:
		return "invalid stop time"
	case ROCFStartMissingTimeValue:
		return "missing time value"
	case ROCFStartInvalidGVCID:
		return "invalid global virtual channel identifier"
	case ROCFStartInvalidControlWordType:
		return "invalid control word type"
	case ROCFStartInvalidTcVcid:
		return "invalid telecommand virtual channel"
	case ROCFStartInvalidUpdateMode:
		return "invalid update mode"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(r))
	}
}

// ROCFStartInvocation is the RocfStartInvocation of the ROCF incoming PDUs
// module.
type ROCFStartInvocation struct {
	Credentials    *Credentials
	InvokeId       InvokeId
	StartTime      ConditionalTime
	StopTime       ConditionalTime
	RequestedGVCID GVCID
	// ControlWordType filters which control words to deliver.
	ControlWordType ControlWordType
	// UpdateMode says whether to deliver every one or only changes.
	UpdateMode UpdateMode
}

// Encode serializes the START invocation's content.
func (s *ROCFStartInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))
	content = AppendConditionalTime(content, s.StartTime)
	content = AppendConditionalTime(content, s.StopTime)

	if content, err = AppendGVCID(content, s.RequestedGVCID); err != nil {
		return nil, err
	}
	content = AppendControlWordType(content, s.ControlWordType)
	return AppendInteger(content, int64(s.UpdateMode)), nil
}

// DecodeROCFStartInvocation parses a START invocation's content.
func DecodeROCFStartInvocation(data []byte) (*ROCFStartInvocation, error) {
	d := NewDecoder(data)
	s := &ROCFStartInvocation{}

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

	cwElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.ControlWordType, err = DecodeControlWordType(cwElem); err != nil {
		return nil, err
	}

	modeElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	mode, err := modeElem.Int64()
	if err != nil {
		return nil, err
	}
	s.UpdateMode = UpdateMode(mode)
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *ROCFStartInvocation) Humanize() string {
	return fmt.Sprintf("ROCF START Invocation\n"+
		"  Invoke ID ..... %d\n"+
		"  Channel ....... %s\n"+
		"  Control word .. %s\n"+
		"  Update mode ... %s",
		s.InvokeId, s.RequestedGVCID, s.ControlWordType, s.UpdateMode)
}

// ROCFTransferDataInvocation is the RocfTransferDataInvocation of the ROCF
// outgoing PDUs module.
//
// Its Data is the operational control field itself (four octets, usually a
// CLCW that pkg/cop can decode) rather than a whole frame.
type ROCFTransferDataInvocation struct {
	Credentials        *Credentials
	EarthReceiveTime   Time
	AntennaId          AntennaId
	DataLinkContinuity int32
	PrivateAnnotation  []byte
	// Data is the operational control field.
	Data []byte
}

// Encode serializes the TRANSFER-DATA invocation's content.
func (t *ROCFTransferDataInvocation) Encode() ([]byte, error) {
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

// DecodeROCFTransferDataInvocation parses a TRANSFER-DATA invocation's content.
//
// The layout matches RCF's exactly, so this shares its decoder and relabels
// the result.
func DecodeROCFTransferDataInvocation(data []byte) (*ROCFTransferDataInvocation, error) {
	rcf, err := DecodeRCFTransferDataInvocation(data)
	if err != nil {
		return nil, err
	}
	return &ROCFTransferDataInvocation{
		Credentials:        rcf.Credentials,
		EarthReceiveTime:   rcf.EarthReceiveTime,
		AntennaId:          rcf.AntennaId,
		DataLinkContinuity: rcf.DataLinkContinuity,
		PrivateAnnotation:  rcf.PrivateAnnotation,
		Data:               rcf.Data,
	}, nil
}

// Humanize returns a human-readable summary.
func (t *ROCFTransferDataInvocation) Humanize() string {
	return fmt.Sprintf("ROCF TRANSFER-DATA\n  Received .... %s\n  Antenna ..... %s\n  OCF ......... %d octets",
		t.EarthReceiveTime.Humanize(), t.AntennaId, len(t.Data))
}

// ROCFStatusReportInvocation is the RocfStatusReportInvocation of annex A2.7.
//
// Its two counters are not the return services' usual pair. ROCF counts the
// frames it looked at and the OCFs it sent on, and those differ: a frame may
// carry no operational control field, or carry one the filter rejected. The
// gap between the two numbers is how much of the channel was not of interest.
type ROCFStatusReportInvocation struct {
	Credentials *Credentials
	// ProcessedFrameNumber counts frames the provider examined.
	ProcessedFrameNumber uint32
	// DeliveredOCFsNumber counts control fields actually delivered.
	DeliveredOCFsNumber uint32

	FrameSyncLockStatus  LockStatus
	SymbolSyncLockStatus LockStatus
	SubcarrierLockStatus LockStatus
	CarrierLockStatus    LockStatus
	ProductionStatus     ProductionStatus
}

// Encode serializes the STATUS-REPORT invocation's content.
func (s *ROCFStatusReportInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.ProcessedFrameNumber))
	content = AppendInteger(content, int64(s.DeliveredOCFsNumber))
	content = AppendInteger(content, int64(s.FrameSyncLockStatus))
	content = AppendInteger(content, int64(s.SymbolSyncLockStatus))
	content = AppendInteger(content, int64(s.SubcarrierLockStatus))
	content = AppendInteger(content, int64(s.CarrierLockStatus))
	return AppendInteger(content, int64(s.ProductionStatus)), nil
}

// DecodeROCFStatusReportInvocation parses a STATUS-REPORT invocation's content.
func DecodeROCFStatusReportInvocation(data []byte) (*ROCFStatusReportInvocation, error) {
	d := NewDecoder(data)
	s := &ROCFStatusReportInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	counts := []*uint32{&s.ProcessedFrameNumber, &s.DeliveredOCFsNumber}
	for _, target := range counts {
		v, err := decodeUint32(d)
		if err != nil {
			return nil, err
		}
		*target = v
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
func (s *ROCFStatusReportInvocation) Humanize() string {
	return fmt.Sprintf("ROCF STATUS-REPORT\n"+
		"  Processed ....... %d frames\n"+
		"  Delivered ....... %d OCFs\n"+
		"  Frame sync ...... %s\n"+
		"  Carrier lock .... %s\n"+
		"  Production ...... %s",
		s.ProcessedFrameNumber, s.DeliveredOCFsNumber,
		s.FrameSyncLockStatus, s.CarrierLockStatus, s.ProductionStatus)
}

// ROCFTransferBufferEntry is one element of a RocfTransferBuffer.
//
//	OcfOrNotification ::= CHOICE
//	{ annotatedOcf     [0] RocfTransferDataInvocation
//	, syncNotification [1] RocfSyncNotifyInvocation
//	}
type ROCFTransferBufferEntry struct {
	OCF          *ROCFTransferDataInvocation
	Notification *SyncNotifyInvocation
}

// ROCFTransferBuffer is the RocfTransferBuffer of annex A2.7.
//
// Buffering matters more here than in RAF. An OCF is four octets, so one PDU
// per control field would spend far more on framing than on data.
type ROCFTransferBuffer []ROCFTransferBufferEntry

// Encode serializes the transfer buffer's content.
func (b ROCFTransferBuffer) Encode() ([]byte, error) {
	var content []byte
	for _, entry := range b {
		switch {
		case entry.OCF != nil:
			inner, err := entry.OCF.Encode()
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

// DecodeROCFTransferBuffer parses a transfer buffer's content.
func DecodeROCFTransferBuffer(data []byte) (ROCFTransferBuffer, error) {
	d := NewDecoder(data)
	var buffer ROCFTransferBuffer

	for !d.Empty() {
		e, err := d.Next()
		if err != nil {
			return nil, err
		}
		switch {
		case e.IsContext(0):
			ocf, err := DecodeROCFTransferDataInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, ROCFTransferBufferEntry{OCF: ocf})
		case e.IsContext(1):
			notification, err := DecodeSyncNotifyInvocation(e.Bytes)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, ROCFTransferBufferEntry{Notification: notification})
		default:
			return nil, ErrInvalidTag
		}
	}
	return buffer, nil
}

// OCFs returns just the control fields in the buffer.
func (b ROCFTransferBuffer) OCFs() []*ROCFTransferDataInvocation {
	out := make([]*ROCFTransferDataInvocation, 0, len(b))
	for _, entry := range b {
		if entry.OCF != nil {
			out = append(out, entry.OCF)
		}
	}
	return out
}

// Humanize returns a human-readable summary.
func (b ROCFTransferBuffer) Humanize() string {
	ocfs, notifications := 0, 0
	for _, entry := range b {
		if entry.OCF != nil {
			ocfs++
		} else {
			notifications++
		}
	}
	return fmt.Sprintf("ROCF TRANSFER-BUFFER\n  OCFs ........... %d\n  Notifications .. %d",
		ocfs, notifications)
}

// ROCFStartReturn is the RocfStartReturn of annex A2.7.
type ROCFStartReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider accepted.
	Positive bool
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the ROCF-specific one.
	SpecificDiagnostic ROCFStartDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the START return's content.
func (s *ROCFStartReturn) Encode() ([]byte, error) {
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

// DecodeROCFStartReturn parses a START return's content.
func DecodeROCFStartReturn(data []byte) (*ROCFStartReturn, error) {
	d := NewDecoder(data)
	s := &ROCFStartReturn{}

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
			s.SpecificDiagnostic = ROCFStartDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *ROCFStartReturn) Humanize() string {
	if s.Positive {
		return fmt.Sprintf("ROCF START Return\n  Invoke ID ... %d\n  Result ...... accepted", s.InvokeId)
	}
	reason := s.SpecificDiagnostic.String()
	if s.UsedCommon {
		reason = s.CommonDiagnostic.String()
	}
	return fmt.Sprintf("ROCF START Return\n  Invoke ID ... %d\n  Result ...... refused: %s", s.InvokeId, reason)
}

// ROCFUser is the user half of an ROCF service instance.
//
// ROCF is the narrowest of the return services: four octets per frame, and
// only from the frames that carry a control field of the requested kind. A
// mission running FOP-1 on the ground uses it to close the loop, feeding each
// delivered field to pkg/cop's CLCW decoder.
type ROCFUser struct {
	*ServiceUser
}

// NewROCFUser prepares the user half of an ROCF instance.
func NewROCFUser(config ServiceConfig) (*ROCFUser, error) {
	config.Kind = ServiceROCF
	user, err := NewServiceUser(config)
	if err != nil {
		return nil, err
	}
	return &ROCFUser{ServiceUser: user}, nil
}

// Start asks the provider to begin delivering control fields. State 2 only.
func (u *ROCFUser) Start(
	now time.Time, randomNumber int32, start, stop ConditionalTime,
	channel GVCID, control ControlWordType, mode UpdateMode,
) (InvokeId, error) {
	if err := channel.Validate(); err != nil {
		return 0, err
	}
	// VcId ::= INTEGER (0 .. 63), for the TC virtual channel too.
	if control.HasTCVirtualChannel && control.TCVirtualChannel > 63 {
		return 0, ErrInvalidIdentifier
	}
	return u.invoke(OpStartInvocation, ServiceReady, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&ROCFStartInvocation{
				Credentials:     creds,
				InvokeId:        id,
				StartTime:       start,
				StopTime:        stop,
				RequestedGVCID:  channel,
				ControlWordType: control,
				UpdateMode:      mode,
			}).Encode()
		})
}

// HandleStartReturn takes the answer to START, moving to state 3 when
// positive.
func (u *ROCFUser) HandleStartReturn(r *ROCFStartReturn) error {
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

// ROCFUserEvent is one decoded PDU arriving at the user.
type ROCFUserEvent struct {
	Operation OperationType

	BindReturn                 *BindReturn
	UnbindReturn               *UnbindReturn
	StartReturn                *ROCFStartReturn
	StopReturn                 *Acknowledgement
	ScheduleStatusReportReturn *ScheduleStatusReportReturn
	GetParameterReturn         *GetParameterReturn
	TransferBuffer             ROCFTransferBuffer
	StatusReport               *ROCFStatusReportInvocation
	PeerAbort                  *PeerAbort
}

// HandlePDU decodes one PDU from the provider and advances the machine.
func (u *ROCFUser) HandlePDU(data []byte, now time.Time) (*ROCFUserEvent, error) {
	pdu, err := DecodePDU(data, ServiceROCF)
	if err != nil {
		return nil, err
	}
	u.Association().RecordReceived(now)

	event := &ROCFUserEvent{Operation: pdu.Operation}

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
		r, err := DecodeROCFStartReturn(pdu.Content)
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
		buffer, err := DecodeROCFTransferBuffer(pdu.Content)
		if err != nil {
			return nil, err
		}
		for _, entry := range buffer {
			creds := (*Credentials)(nil)
			switch {
			case entry.OCF != nil:
				creds = entry.OCF.Credentials
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
		report, err := DecodeROCFStatusReportInvocation(pdu.Content)
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

// ROCFProvider is the provider half of an ROCF instance. Partial.
type ROCFProvider struct {
	*ServiceProvider
}

// NewROCFProvider prepares the provider half of an ROCF instance.
func NewROCFProvider(config ServiceConfig) (*ROCFProvider, error) {
	config.Kind = ServiceROCF
	provider, err := NewServiceProvider(config)
	if err != nil {
		return nil, err
	}
	return &ROCFProvider{ServiceProvider: provider}, nil
}

// HandleStartInvocation answers a START, moving to state 3 when it accepts.
func (p *ROCFProvider) HandleStartInvocation(
	s *ROCFStartInvocation, answer *ROCFStartReturn, now time.Time, randomNumber int32,
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

// SendTransferBuffer queues a buffer of control fields and notifications.
// State 3 only.
func (p *ROCFProvider) SendTransferBuffer(buffer ROCFTransferBuffer, now time.Time) error {
	content, err := buffer.Encode()
	if err != nil {
		return err
	}
	return p.sendWhileActive(OpTransferBuffer, content, now)
}
