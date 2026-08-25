package sle

import (
	"fmt"
	"time"
)

// Forward CLTU, per CCSDS 912.1-B-5.
//
// FCLTU runs the other way from the return services: the user hands the
// provider Communications Link Transmission Units, and the provider radiates
// them at the spacecraft. That reversal changes the shape of the protocol.
//
// A return service streams whatever arrives, so its data flows in one
// direction and the user mostly listens. FCLTU is a queue with
// acknowledgement. Every CLTU carries an identification number, the user must
// send them in ascending order, and the provider answers each one saying
// whether it was accepted and how much buffer is left. Radiation happens
// later, so a second message — ASYNC-NOTIFY — reports what became of a CLTU
// once the antenna got to it.
//
// The operations, from the PDU CHOICEs of annex A2.4 and A2.5:
//
//	user → provider    BIND, UNBIND, START, STOP, SCHEDULE-STATUS-REPORT,
//	                   GET-PARAMETER, THROW-EVENT, TRANSFER-DATA, PEER-ABORT
//	provider → user    the returns for those, plus ASYNC-NOTIFY and
//	                   STATUS-REPORT

// FCLTU PDU tags, from the CltuUserToProviderPdu and CltuProviderToUserPdu
// CHOICEs of annex A2.4 and A2.5.
//
// These are not the return services' numbers. Tags [8] and [9] are
// THROW-EVENT here, where RAF uses them for TRANSFER-BUFFER and
// STATUS-REPORT, and FCLTU's STATUS-REPORT sits up at [13]. Decoding an FCLTU
// PDU with a return-service table would silently name the wrong operation,
// which is why DecodePDU takes a ServiceKind.
const (
	TagFCLTUBindInvocation                 uint32 = 100
	TagFCLTUBindReturn                     uint32 = 101
	TagFCLTUUnbindInvocation               uint32 = 102
	TagFCLTUUnbindReturn                   uint32 = 103
	TagFCLTUPeerAbortInvocation            uint32 = 104
	TagFCLTUStartInvocation                uint32 = 0
	TagFCLTUStartReturn                    uint32 = 1
	TagFCLTUStopInvocation                 uint32 = 2
	TagFCLTUStopReturn                     uint32 = 3
	TagFCLTUScheduleStatusReportInvocation uint32 = 4
	TagFCLTUScheduleStatusReportReturn     uint32 = 5
	TagFCLTUGetParameterInvocation         uint32 = 6
	TagFCLTUGetParameterReturn             uint32 = 7
	TagFCLTUThrowEventInvocation           uint32 = 8
	TagFCLTUThrowEventReturn               uint32 = 9
	TagFCLTUTransferDataInvocation         uint32 = 10
	TagFCLTUTransferDataReturn             uint32 = 11
	TagFCLTUAsyncNotifyInvocation          uint32 = 12
	TagFCLTUStatusReportInvocation         uint32 = 13
)

// CltuIdentification numbers a CLTU within a service instance. The user
// assigns it and must keep it ascending; the provider quotes it back.
// IntUnsignedLong: INTEGER (0 .. 4294967295).
type CltuIdentification uint32

// EventInvocationId numbers a THROW-EVENT invocation, also ascending.
type EventInvocationId uint32

// MaxEventQualifier is the largest eventQualifier a THROW-EVENT may carry:
// OCTET STRING (SIZE (1 .. 1024)).
const MaxEventQualifier = 1024

// FCLTUProductionStatus is the state of the forward physical channel, from the
// ProductionStatus INTEGER of the CLTU structures module.
//
// This is a different type from the return services' ProductionStatus, not a
// wider spelling of it. FCLTU has four states where the return services have
// three, and the numbers do not line up: 1 is "configured" here and "halted"
// there. Sharing one Go type between them would mislabel every status report.
type FCLTUProductionStatus int

const (
	// FCLTUProductionOperational means the channel is configured and radiating.
	FCLTUProductionOperational FCLTUProductionStatus = 0
	// FCLTUProductionConfigured means the channel is ready but not radiating.
	FCLTUProductionConfigured FCLTUProductionStatus = 1
	// FCLTUProductionInterrupted means radiation stopped unexpectedly.
	FCLTUProductionInterrupted FCLTUProductionStatus = 2
	// FCLTUProductionHalted means the channel ended and will not resume.
	FCLTUProductionHalted FCLTUProductionStatus = 3
)

// String names the production status.
func (p FCLTUProductionStatus) String() string {
	switch p {
	case FCLTUProductionOperational:
		return "operational"
	case FCLTUProductionConfigured:
		return "configured"
	case FCLTUProductionInterrupted:
		return "interrupted"
	case FCLTUProductionHalted:
		return "halted"
	default:
		return fmt.Sprintf("status(%d)", int(p))
	}
}

// UplinkStatus reports what the uplink carrier is doing, from the UplinkStatus
// INTEGER of the CLTU structures module.
type UplinkStatus int

const (
	UplinkStatusNotAvailable UplinkStatus = 0
	UplinkNoRfAvailable      UplinkStatus = 1
	UplinkNoBitLock          UplinkStatus = 2
	UplinkNominal            UplinkStatus = 3
)

// String names the uplink status.
func (u UplinkStatus) String() string {
	switch u {
	case UplinkStatusNotAvailable:
		return "not available"
	case UplinkNoRfAvailable:
		return "no RF available"
	case UplinkNoBitLock:
		return "no bit lock"
	case UplinkNominal:
		return "nominal"
	default:
		return fmt.Sprintf("status(%d)", int(u))
	}
}

// CltuStatus says what became of a CLTU, from the CltuStatus subtype of
// ForwardDuStatus.
//
// The values are ForwardDuStatus's, and FCLTU uses only five of its seven:
// 3 is "acknowledged" and 6 is "unsupported transmission mode", both of which
// belong to the Forward Space Packet service. So the numbering has a hole in
// it, and 3 is not a value this service ever sends.
type CltuStatus int

const (
	// CltuRadiated means the CLTU went out.
	CltuRadiated CltuStatus = 0
	// CltuExpired means its transmission window passed before it could.
	CltuExpired CltuStatus = 1
	// CltuInterrupted means radiation began and was cut short.
	CltuInterrupted CltuStatus = 2
	// CltuProductionStarted means radiation started.
	CltuProductionStarted CltuStatus = 4
	// CltuProductionNotStarted means radiation did not start.
	CltuProductionNotStarted CltuStatus = 5
)

// String names the status.
func (c CltuStatus) String() string {
	switch c {
	case CltuRadiated:
		return "radiated"
	case CltuExpired:
		return "expired"
	case CltuInterrupted:
		return "interrupted"
	case CltuProductionStarted:
		return "radiation started"
	case CltuProductionNotStarted:
		return "radiation not started"
	default:
		return fmt.Sprintf("status(%d)", int(c))
	}
}

// Valid reports whether the status is one FCLTU defines.
func (c CltuStatus) Valid() bool {
	switch c {
	case CltuRadiated, CltuExpired, CltuInterrupted,
		CltuProductionStarted, CltuProductionNotStarted:
		return true
	default:
		return false
	}
}

// SlduStatusNotification says whether the user wants an ASYNC-NOTIFY once the
// CLTU has been dealt with.
type SlduStatusNotification int

const (
	// ProduceNotification asks for the notification.
	ProduceNotification SlduStatusNotification = 0
	// DoNotProduceNotification declines it.
	DoNotProduceNotification SlduStatusNotification = 1
)

// String names the choice.
func (s SlduStatusNotification) String() string {
	if s == ProduceNotification {
		return "produce notification"
	}
	return "do not produce notification"
}

// FCLTUStartDiagnostic explains a refused START, from the specific alternative
// of DiagnosticCltuStart.
type FCLTUStartDiagnostic int

const (
	FCLTUStartOutOfService          FCLTUStartDiagnostic = 0
	FCLTUStartUnableToComply        FCLTUStartDiagnostic = 1
	FCLTUStartProductionTimeExpired FCLTUStartDiagnostic = 2
	FCLTUStartInvalidCltuId         FCLTUStartDiagnostic = 3
)

// String names the diagnostic.
func (f FCLTUStartDiagnostic) String() string {
	switch f {
	case FCLTUStartOutOfService:
		return "out of service"
	case FCLTUStartUnableToComply:
		return "unable to comply"
	case FCLTUStartProductionTimeExpired:
		return "production time expired"
	case FCLTUStartInvalidCltuId:
		return "invalid CLTU identification"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(f))
	}
}

// FCLTUTransferDataDiagnostic explains a refused CLTU, from the specific
// alternative of DiagnosticCltuTransferData.
type FCLTUTransferDataDiagnostic int

const (
	FCLTUDataUnableToProcess       FCLTUTransferDataDiagnostic = 0
	FCLTUDataUnableToStore         FCLTUTransferDataDiagnostic = 1
	FCLTUDataOutOfSequence         FCLTUTransferDataDiagnostic = 2
	FCLTUDataInconsistentTimeRange FCLTUTransferDataDiagnostic = 3
	FCLTUDataInvalidTime           FCLTUTransferDataDiagnostic = 4
	FCLTUDataLateSldu              FCLTUTransferDataDiagnostic = 5
	FCLTUDataInvalidDelayTime      FCLTUTransferDataDiagnostic = 6
	FCLTUDataCltuError             FCLTUTransferDataDiagnostic = 7
)

// String names the diagnostic.
func (f FCLTUTransferDataDiagnostic) String() string {
	switch f {
	case FCLTUDataUnableToProcess:
		return "unable to process"
	case FCLTUDataUnableToStore:
		return "unable to store"
	case FCLTUDataOutOfSequence:
		return "out of sequence"
	case FCLTUDataInconsistentTimeRange:
		return "inconsistent time range"
	case FCLTUDataInvalidTime:
		return "invalid time"
	case FCLTUDataLateSldu:
		return "late SLDU"
	case FCLTUDataInvalidDelayTime:
		return "invalid delay time"
	case FCLTUDataCltuError:
		return "CLTU error"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(f))
	}
}

// FCLTUThrowEventDiagnostic explains a refused THROW-EVENT, from the specific
// alternative of DiagnosticCltuThrowEvent.
type FCLTUThrowEventDiagnostic int

const (
	FCLTUEventOperationNotSupported FCLTUThrowEventDiagnostic = 0
	FCLTUEventIdOutOfSequence       FCLTUThrowEventDiagnostic = 1
	FCLTUEventNoSuchEvent           FCLTUThrowEventDiagnostic = 2
)

// String names the diagnostic.
func (f FCLTUThrowEventDiagnostic) String() string {
	switch f {
	case FCLTUEventOperationNotSupported:
		return "operation not supported"
	case FCLTUEventIdOutOfSequence:
		return "event invocation identifier out of sequence"
	case FCLTUEventNoSuchEvent:
		return "no such event"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(f))
	}
}

// decodeInvokeId reads the next element as an InvokeId, rejecting values past
// the IntUnsignedShort range.
func decodeInvokeId(d *Decoder) (InvokeId, error) {
	e, err := d.Next()
	if err != nil {
		return 0, err
	}
	v, err := e.Uint64()
	if err != nil {
		return 0, err
	}
	if v > 65535 {
		return 0, ErrIntegerOverflow
	}
	return InvokeId(v), nil
}

// decodeUint32 reads the next element as an IntUnsignedLong.
func decodeUint32(d *Decoder) (uint32, error) {
	e, err := d.Next()
	if err != nil {
		return 0, err
	}
	v, err := e.Uint64()
	if err != nil {
		return 0, err
	}
	if v > 4294967295 {
		return 0, ErrIntegerOverflow
	}
	return uint32(v), nil
}

// decodeInt reads the next element as a plain INTEGER.
func decodeInt(d *Decoder) (int64, error) {
	e, err := d.Next()
	if err != nil {
		return 0, err
	}
	return e.Int64()
}

// appendCommonOrSpecific writes a diagnostic CHOICE of the shape every FCLTU
// negative result uses: common [0] Diagnostics, specific [1] INTEGER.
func appendCommonOrSpecific(usedCommon bool, common Diagnostics, specific int64) []byte {
	if usedCommon {
		return AppendTaggedInteger(nil, 0, int64(common))
	}
	return AppendTaggedInteger(nil, 1, specific)
}

// decodeCommonOrSpecific reads that CHOICE, returning which alternative it
// took and its value.
func decodeCommonOrSpecific(e *Element) (usedCommon bool, value int64, err error) {
	inner, err := NewDecoder(e.Bytes).Next()
	if err != nil {
		return false, 0, err
	}
	v, err := inner.Int64()
	if err != nil {
		return false, 0, err
	}
	switch {
	case inner.IsContext(0):
		return true, v, nil
	case inner.IsContext(1):
		return false, v, nil
	default:
		return false, 0, ErrInvalidTag
	}
}

// FCLTUStartInvocation is the CltuStartInvocation of annex A2.4. It opens the
// CLTU stream and fixes the number the first CLTU will carry.
type FCLTUStartInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// FirstCltuIdentification is the number the user will put on its first
	// CLTU. Everything after it must count up from here.
	FirstCltuIdentification CltuIdentification
}

// Encode serializes the START invocation's content.
func (s *FCLTUStartInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))
	return AppendInteger(content, int64(s.FirstCltuIdentification)), nil
}

// DecodeFCLTUStartInvocation parses a START invocation's content.
func DecodeFCLTUStartInvocation(data []byte) (*FCLTUStartInvocation, error) {
	d := NewDecoder(data)
	s := &FCLTUStartInvocation{}

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
	first, err := decodeUint32(d)
	if err != nil {
		return nil, err
	}
	s.FirstCltuIdentification = CltuIdentification(first)
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *FCLTUStartInvocation) Humanize() string {
	return fmt.Sprintf("FCLTU START Invocation\n  Invoke ID ..... %d\n  First CLTU .... %d",
		s.InvokeId, s.FirstCltuIdentification)
}

// FCLTUStartReturn is the CltuStartReturn of annex A2.5.
//
// Its positive result is not the empty NULL the return services use: it
// carries the window the provider has reserved for radiation.
type FCLTUStartReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider accepted.
	Positive bool
	// StartRadiationTime is when the provider can begin radiating. Set when
	// Positive.
	StartRadiationTime Time
	// StopRadiationTime bounds the window, or is undefined for "until further
	// notice". Set when Positive.
	StopRadiationTime ConditionalTime
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the FCLTU-specific one.
	SpecificDiagnostic FCLTUStartDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the START return's content.
func (s *FCLTUStartReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))

	if s.Positive {
		// positiveResult [0] SEQUENCE { startRadiationTime, stopRadiationTime }
		var window []byte
		window = AppendTimeChoice(window, s.StartRadiationTime)
		window = AppendConditionalTime(window, s.StopRadiationTime)
		return AppendElement(content, ClassContext, true, 0, window), nil
	}

	diagnostic := appendCommonOrSpecific(s.UsedCommon, s.CommonDiagnostic, int64(s.SpecificDiagnostic))
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeFCLTUStartReturn parses a START return's content.
func DecodeFCLTUStartReturn(data []byte) (*FCLTUStartReturn, error) {
	d := NewDecoder(data)
	s := &FCLTUStartReturn{}

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
		inner := NewDecoder(result.Bytes)

		startElem, err := inner.Next()
		if err != nil {
			return nil, err
		}
		if s.StartRadiationTime, err = DecodeTimeChoice(startElem); err != nil {
			return nil, err
		}

		stopElem, err := inner.Next()
		if err != nil {
			return nil, err
		}
		if s.StopRadiationTime, err = DecodeConditionalTime(stopElem); err != nil {
			return nil, err
		}

	case result.IsContext(1):
		usedCommon, v, err := decodeCommonOrSpecific(result)
		if err != nil {
			return nil, err
		}
		s.UsedCommon = usedCommon
		if usedCommon {
			s.CommonDiagnostic = Diagnostics(v)
		} else {
			s.SpecificDiagnostic = FCLTUStartDiagnostic(v)
		}

	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *FCLTUStartReturn) Humanize() string {
	if s.Positive {
		window := "until further notice"
		if s.StopRadiationTime.Known {
			window = "to " + s.StopRadiationTime.Time.Humanize()
		}
		return fmt.Sprintf("FCLTU START Return\n  Invoke ID ..... %d\n  Result ........ accepted\n  Radiation ..... from %s %s",
			s.InvokeId, s.StartRadiationTime.Humanize(), window)
	}
	reason := s.SpecificDiagnostic.String()
	if s.UsedCommon {
		reason = s.CommonDiagnostic.String()
	}
	return fmt.Sprintf("FCLTU START Return\n  Invoke ID ..... %d\n  Result ........ refused: %s",
		s.InvokeId, reason)
}

// FCLTUTransferDataInvocation is the CltuTransferDataInvocation of annex A2.4:
// one CLTU handed to the provider for radiation.
type FCLTUTransferDataInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// CltuIdentification numbers this CLTU. It must be one more than the last.
	CltuIdentification CltuIdentification
	// EarliestTransmissionTime and LatestTransmissionTime bound when the CLTU
	// may be radiated. Either may be undefined, leaving that end open.
	EarliestTransmissionTime ConditionalTime
	LatestTransmissionTime   ConditionalTime
	// DelayTime is how long, in microseconds, the provider must wait after the
	// previous CLTU before radiating this one.
	DelayTime uint32
	// RadiationNotification says whether the user wants an ASYNC-NOTIFY when
	// this CLTU is dealt with.
	RadiationNotification SlduStatusNotification
	// Data is the CLTU itself, 1 to 65536 octets: the acquisition sequence,
	// start sequence, codeblocks and tail sequence that pkg/tcsc builds.
	Data []byte
}

// Encode serializes the TRANSFER-DATA invocation's content.
func (t *FCLTUTransferDataInvocation) Encode() ([]byte, error) {
	if len(t.Data) == 0 || len(t.Data) > MaxSpaceLinkDataUnit {
		return nil, ErrDataTooShort
	}

	content, err := AppendCredentialsChoice(nil, t.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(t.InvokeId))
	content = AppendInteger(content, int64(t.CltuIdentification))
	content = AppendConditionalTime(content, t.EarliestTransmissionTime)
	content = AppendConditionalTime(content, t.LatestTransmissionTime)
	content = AppendInteger(content, int64(t.DelayTime))
	content = AppendInteger(content, int64(t.RadiationNotification))
	return AppendOctetString(content, t.Data), nil
}

// DecodeFCLTUTransferDataInvocation parses a TRANSFER-DATA invocation's
// content.
func DecodeFCLTUTransferDataInvocation(data []byte) (*FCLTUTransferDataInvocation, error) {
	d := NewDecoder(data)
	t := &FCLTUTransferDataInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if t.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	id, err := decodeUint32(d)
	if err != nil {
		return nil, err
	}
	t.CltuIdentification = CltuIdentification(id)

	earliestElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.EarliestTransmissionTime, err = DecodeConditionalTime(earliestElem); err != nil {
		return nil, err
	}

	latestElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.LatestTransmissionTime, err = DecodeConditionalTime(latestElem); err != nil {
		return nil, err
	}

	if t.DelayTime, err = decodeUint32(d); err != nil {
		return nil, err
	}

	notification, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	t.RadiationNotification = SlduStatusNotification(notification)

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
func (t *FCLTUTransferDataInvocation) Humanize() string {
	window := "any time"
	switch {
	case t.EarliestTransmissionTime.Known && t.LatestTransmissionTime.Known:
		window = t.EarliestTransmissionTime.Time.Humanize() + " to " + t.LatestTransmissionTime.Time.Humanize()
	case t.EarliestTransmissionTime.Known:
		window = "from " + t.EarliestTransmissionTime.Time.Humanize()
	case t.LatestTransmissionTime.Known:
		window = "until " + t.LatestTransmissionTime.Time.Humanize()
	}
	return fmt.Sprintf("FCLTU TRANSFER-DATA\n"+
		"  Invoke ID ....... %d\n"+
		"  CLTU ID ......... %d\n"+
		"  Window .......... %s\n"+
		"  Delay ........... %d us\n"+
		"  Notification .... %s\n"+
		"  CLTU ............ %d octets",
		t.InvokeId, t.CltuIdentification, window,
		t.DelayTime, t.RadiationNotification, len(t.Data))
}

// FCLTUTransferDataReturn is the CltuTransferDataReturn of annex A2.5: the
// provider saying whether it took the CLTU, and how much room is left.
type FCLTUTransferDataReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// CltuIdentification echoes the CLTU this answers. On a refusal it is the
	// number the provider expected next, which tells the user where to resume.
	CltuIdentification CltuIdentification
	// CltuBufferAvailable is the free buffer in octets. This is the service's
	// flow control: a user that ignores it will be refused for lack of store.
	CltuBufferAvailable uint32
	// Positive reports whether the CLTU was accepted.
	Positive bool
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the FCLTU-specific one.
	SpecificDiagnostic FCLTUTransferDataDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the TRANSFER-DATA return's content.
func (t *FCLTUTransferDataReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, t.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(t.InvokeId))
	content = AppendInteger(content, int64(t.CltuIdentification))
	content = AppendInteger(content, int64(t.CltuBufferAvailable))

	if t.Positive {
		return AppendElement(content, ClassContext, false, 0, nil), nil
	}
	diagnostic := appendCommonOrSpecific(t.UsedCommon, t.CommonDiagnostic, int64(t.SpecificDiagnostic))
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeFCLTUTransferDataReturn parses a TRANSFER-DATA return's content.
func DecodeFCLTUTransferDataReturn(data []byte) (*FCLTUTransferDataReturn, error) {
	d := NewDecoder(data)
	t := &FCLTUTransferDataReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if t.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if t.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	id, err := decodeUint32(d)
	if err != nil {
		return nil, err
	}
	t.CltuIdentification = CltuIdentification(id)

	if t.CltuBufferAvailable, err = decodeUint32(d); err != nil {
		return nil, err
	}

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		t.Positive = true
	case result.IsContext(1):
		usedCommon, v, err := decodeCommonOrSpecific(result)
		if err != nil {
			return nil, err
		}
		t.UsedCommon = usedCommon
		if usedCommon {
			t.CommonDiagnostic = Diagnostics(v)
		} else {
			t.SpecificDiagnostic = FCLTUTransferDataDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return t, nil
}

// Humanize returns a human-readable summary.
func (t *FCLTUTransferDataReturn) Humanize() string {
	result := "accepted"
	if !t.Positive {
		reason := t.SpecificDiagnostic.String()
		if t.UsedCommon {
			reason = t.CommonDiagnostic.String()
		}
		result = "refused: " + reason
	}
	return fmt.Sprintf("FCLTU TRANSFER-DATA Return\n"+
		"  Invoke ID ....... %d\n"+
		"  CLTU ID ......... %d\n"+
		"  Buffer free ..... %d octets\n"+
		"  Result .......... %s",
		t.InvokeId, t.CltuIdentification, t.CltuBufferAvailable, result)
}

// FCLTUThrowEventInvocation is the CltuThrowEventInvocation of annex A2.4.
//
// THROW-EVENT is the odd operation of this service. It does not carry data to
// the spacecraft; it asks the provider's own equipment to do something the
// service agreement defines — switch an antenna, change a modulation setting,
// start a ranging measurement. The library cannot know what the events mean,
// so it carries the identifier and qualifier through untouched.
type FCLTUThrowEventInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// EventInvocationIdentification numbers this invocation, ascending like
	// the CLTU numbers.
	EventInvocationIdentification EventInvocationId
	// EventIdentifier names the event, 1 to 65535, defined by the service
	// agreement.
	EventIdentifier uint16
	// EventQualifier is the event's argument, 1 to 1024 octets, also defined
	// by the service agreement.
	EventQualifier []byte
}

// Encode serializes the THROW-EVENT invocation's content.
func (e *FCLTUThrowEventInvocation) Encode() ([]byte, error) {
	// eventIdentifier is IntPosShort: INTEGER (1 .. 65535). Zero is not a
	// legal event.
	if e.EventIdentifier == 0 {
		return nil, ErrInvalidIdentifier
	}
	if len(e.EventQualifier) == 0 || len(e.EventQualifier) > MaxEventQualifier {
		return nil, ErrInvalidLength
	}

	content, err := AppendCredentialsChoice(nil, e.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(e.InvokeId))
	content = AppendInteger(content, int64(e.EventInvocationIdentification))
	content = AppendInteger(content, int64(e.EventIdentifier))
	return AppendOctetString(content, e.EventQualifier), nil
}

// DecodeFCLTUThrowEventInvocation parses a THROW-EVENT invocation's content.
func DecodeFCLTUThrowEventInvocation(data []byte) (*FCLTUThrowEventInvocation, error) {
	d := NewDecoder(data)
	e := &FCLTUThrowEventInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if e.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if e.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	eventInvocation, err := decodeUint32(d)
	if err != nil {
		return nil, err
	}
	e.EventInvocationIdentification = EventInvocationId(eventInvocation)

	identifierElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	identifier, err := identifierElem.Uint64()
	if err != nil {
		return nil, err
	}
	if identifier == 0 || identifier > 65535 {
		return nil, ErrIntegerOverflow
	}
	e.EventIdentifier = uint16(identifier)

	qualifierElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if len(qualifierElem.Bytes) == 0 || len(qualifierElem.Bytes) > MaxEventQualifier {
		return nil, ErrInvalidLength
	}
	e.EventQualifier = qualifierElem.Copy()
	return e, nil
}

// Humanize returns a human-readable summary.
func (e *FCLTUThrowEventInvocation) Humanize() string {
	return fmt.Sprintf("FCLTU THROW-EVENT\n"+
		"  Invoke ID ....... %d\n"+
		"  Invocation ID ... %d\n"+
		"  Event ........... %d\n"+
		"  Qualifier ....... %d octets",
		e.InvokeId, e.EventInvocationIdentification, e.EventIdentifier, len(e.EventQualifier))
}

// FCLTUThrowEventReturn is the CltuThrowEventReturn of annex A2.5.
type FCLTUThrowEventReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// EventInvocationIdentification echoes the invocation this answers.
	EventInvocationIdentification EventInvocationId
	// Positive reports whether the provider accepted the event.
	Positive bool
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the FCLTU-specific one.
	SpecificDiagnostic FCLTUThrowEventDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the THROW-EVENT return's content.
func (e *FCLTUThrowEventReturn) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, e.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(e.InvokeId))
	content = AppendInteger(content, int64(e.EventInvocationIdentification))

	if e.Positive {
		return AppendElement(content, ClassContext, false, 0, nil), nil
	}
	diagnostic := appendCommonOrSpecific(e.UsedCommon, e.CommonDiagnostic, int64(e.SpecificDiagnostic))
	return AppendElement(content, ClassContext, true, 1, diagnostic), nil
}

// DecodeFCLTUThrowEventReturn parses a THROW-EVENT return's content.
func DecodeFCLTUThrowEventReturn(data []byte) (*FCLTUThrowEventReturn, error) {
	d := NewDecoder(data)
	e := &FCLTUThrowEventReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if e.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}
	if e.InvokeId, err = decodeInvokeId(d); err != nil {
		return nil, err
	}

	eventInvocation, err := decodeUint32(d)
	if err != nil {
		return nil, err
	}
	e.EventInvocationIdentification = EventInvocationId(eventInvocation)

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		e.Positive = true
	case result.IsContext(1):
		usedCommon, v, err := decodeCommonOrSpecific(result)
		if err != nil {
			return nil, err
		}
		e.UsedCommon = usedCommon
		if usedCommon {
			e.CommonDiagnostic = Diagnostics(v)
		} else {
			e.SpecificDiagnostic = FCLTUThrowEventDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return e, nil
}

// Humanize returns a human-readable summary.
func (e *FCLTUThrowEventReturn) Humanize() string {
	result := "accepted"
	if !e.Positive {
		reason := e.SpecificDiagnostic.String()
		if e.UsedCommon {
			reason = e.CommonDiagnostic.String()
		}
		result = "refused: " + reason
	}
	return fmt.Sprintf("FCLTU THROW-EVENT Return\n  Invoke ID ....... %d\n  Invocation ID ... %d\n  Result .......... %s",
		e.InvokeId, e.EventInvocationIdentification, result)
}

// CltuLastProcessed says what the provider last took off its queue.
//
//	CltuLastProcessed ::= CHOICE
//	{ noCltuProcessed [0] NULL
//	, cltuProcessed   [1] SEQUENCE
//	   { cltuIdentification, radiationStartTime, cltuStatus }
//	}
type CltuLastProcessed struct {
	// Processed reports whether any CLTU has been processed yet.
	Processed          bool
	CltuIdentification CltuIdentification
	// RadiationStartTime is when radiation began, undefined if it has not.
	RadiationStartTime ConditionalTime
	Status             CltuStatus
}

// AppendCltuLastProcessed writes a CltuLastProcessed CHOICE.
//
// The cltuProcessed alternative is [1] IMPLICIT SEQUENCE, and the module is
// IMPLICIT TAGS: the tag replaces the SEQUENCE's, so the fields sit directly
// under [1] with no inner SEQUENCE.
func AppendCltuLastProcessed(dst []byte, c CltuLastProcessed) []byte {
	if !c.Processed {
		return AppendElement(dst, ClassContext, false, 0, nil)
	}
	var inner []byte
	inner = AppendInteger(inner, int64(c.CltuIdentification))
	inner = AppendConditionalTime(inner, c.RadiationStartTime)
	inner = AppendInteger(inner, int64(c.Status))
	return AppendElement(dst, ClassContext, true, 1, inner)
}

// DecodeCltuLastProcessed reads a CltuLastProcessed CHOICE.
func DecodeCltuLastProcessed(e *Element) (CltuLastProcessed, error) {
	var c CltuLastProcessed
	switch {
	case e.IsContext(0):
		return c, nil
	case e.IsContext(1):
		// [1] replaces the SEQUENCE tag (implicit tagging), so the fields
		// are directly under it.
		d := NewDecoder(e.Bytes)

		id, err := decodeUint32(d)
		if err != nil {
			return c, err
		}
		c.CltuIdentification = CltuIdentification(id)

		timeElem, err := d.Next()
		if err != nil {
			return c, err
		}
		if c.RadiationStartTime, err = DecodeConditionalTime(timeElem); err != nil {
			return c, err
		}

		status, err := decodeInt(d)
		if err != nil {
			return c, err
		}
		c.Status = CltuStatus(status)
		c.Processed = true
		return c, nil
	default:
		return c, ErrInvalidTag
	}
}

// String renders what was last processed.
func (c CltuLastProcessed) String() string {
	if !c.Processed {
		return "none"
	}
	return fmt.Sprintf("CLTU %d, %s", c.CltuIdentification, c.Status)
}

// CltuLastOk says which CLTU was last radiated without trouble.
//
//	CltuLastOk ::= CHOICE
//	{ noCltuOk [0] NULL
//	, cltuOk   [1] SEQUENCE { cltuIdentification, radiationStopTime }
//	}
//
// Note that radiationStopTime here is a plain Time, not a ConditionalTime: a
// CLTU that finished radiating has a stop time by definition.
type CltuLastOk struct {
	// Ok reports whether any CLTU has been radiated successfully.
	Ok                 bool
	CltuIdentification CltuIdentification
	RadiationStopTime  Time
}

// AppendCltuLastOk writes a CltuLastOk CHOICE. As with CltuLastProcessed,
// the cltuOk alternative's [1] replaces its SEQUENCE tag (implicit tagging),
// so the fields sit directly under it.
func AppendCltuLastOk(dst []byte, c CltuLastOk) []byte {
	if !c.Ok {
		return AppendElement(dst, ClassContext, false, 0, nil)
	}
	var inner []byte
	inner = AppendInteger(inner, int64(c.CltuIdentification))
	inner = AppendTimeChoice(inner, c.RadiationStopTime)
	return AppendElement(dst, ClassContext, true, 1, inner)
}

// DecodeCltuLastOk reads a CltuLastOk CHOICE.
func DecodeCltuLastOk(e *Element) (CltuLastOk, error) {
	var c CltuLastOk
	switch {
	case e.IsContext(0):
		return c, nil
	case e.IsContext(1):
		d := NewDecoder(e.Bytes)

		id, err := decodeUint32(d)
		if err != nil {
			return c, err
		}
		c.CltuIdentification = CltuIdentification(id)

		timeElem, err := d.Next()
		if err != nil {
			return c, err
		}
		if c.RadiationStopTime, err = DecodeTimeChoice(timeElem); err != nil {
			return c, err
		}
		c.Ok = true
		return c, nil
	default:
		return c, ErrInvalidTag
	}
}

// String renders the last good CLTU.
func (c CltuLastOk) String() string {
	if !c.Ok {
		return "none"
	}
	return fmt.Sprintf("CLTU %d at %s", c.CltuIdentification, c.RadiationStopTime.Humanize())
}

// CltuNotificationKind says which alternative an ASYNC-NOTIFY took, from the
// CltuNotification CHOICE of the CLTU structures module.
type CltuNotificationKind int

const (
	// NotifyCltuRadiated reports one CLTU went out.
	NotifyCltuRadiated CltuNotificationKind = 0
	// NotifySlduExpired reports a CLTU's window passed unused.
	NotifySlduExpired CltuNotificationKind = 1
	// NotifyProductionInterrupted reports radiation stopped unexpectedly.
	NotifyProductionInterrupted CltuNotificationKind = 2
	// NotifyProductionHalted reports the channel ended.
	NotifyProductionHalted CltuNotificationKind = 3
	// NotifyProductionOperational reports the channel started radiating.
	NotifyProductionOperational CltuNotificationKind = 4
	// NotifyBufferEmpty reports the provider has run out of CLTUs to send.
	NotifyBufferEmpty CltuNotificationKind = 5
	// NotifyActionListCompleted reports a thrown event's actions all ran.
	NotifyActionListCompleted CltuNotificationKind = 6
	// NotifyActionListNotCompleted reports they did not.
	NotifyActionListNotCompleted CltuNotificationKind = 7
	// NotifyEventConditionEvFalse reports a thrown event's condition was false.
	NotifyEventConditionEvFalse CltuNotificationKind = 8
)

// String names the notification.
func (n CltuNotificationKind) String() string {
	switch n {
	case NotifyCltuRadiated:
		return "CLTU radiated"
	case NotifySlduExpired:
		return "SLDU expired"
	case NotifyProductionInterrupted:
		return "production interrupted"
	case NotifyProductionHalted:
		return "production halted"
	case NotifyProductionOperational:
		return "production operational"
	case NotifyBufferEmpty:
		return "buffer empty"
	case NotifyActionListCompleted:
		return "action list completed"
	case NotifyActionListNotCompleted:
		return "action list not completed"
	case NotifyEventConditionEvFalse:
		return "event condition false"
	default:
		return fmt.Sprintf("notification(%d)", int(n))
	}
}

// CarriesEventId reports whether this notification carries an
// EventInvocationId rather than a NULL. The last three alternatives do.
func (n CltuNotificationKind) CarriesEventId() bool {
	switch n {
	case NotifyActionListCompleted, NotifyActionListNotCompleted, NotifyEventConditionEvFalse:
		return true
	default:
		return false
	}
}

// FCLTUAsyncNotifyInvocation is the CltuAsyncNotifyInvocation of annex A2.5:
// the provider reporting, after the fact, what happened to a CLTU or to the
// channel.
//
// This is what makes the service asynchronous. TRANSFER-DATA's return only
// says the CLTU was queued. Whether it actually reached the antenna arrives
// here, possibly much later.
type FCLTUAsyncNotifyInvocation struct {
	Credentials *Credentials
	// Kind says which notification this is.
	Kind CltuNotificationKind
	// EventInvocationId is set when Kind.CarriesEventId reports true.
	EventInvocationId EventInvocationId
	// LastProcessed and LastOk report the queue's position, sent with every
	// notification whatever its kind.
	LastProcessed    CltuLastProcessed
	LastOk           CltuLastOk
	ProductionStatus FCLTUProductionStatus
	UplinkStatus     UplinkStatus
}

// Encode serializes the ASYNC-NOTIFY invocation's content.
func (n *FCLTUAsyncNotifyInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, n.Credentials)
	if err != nil {
		return nil, err
	}

	tag := uint32(n.Kind)
	switch {
	case n.Kind >= NotifyCltuRadiated && n.Kind <= NotifyBufferEmpty:
		content = AppendElement(content, ClassContext, false, tag, nil)
	case n.Kind.CarriesEventId():
		content = AppendTaggedInteger(content, tag, int64(n.EventInvocationId))
	default:
		return nil, ErrInvalidTag
	}

	content = AppendCltuLastProcessed(content, n.LastProcessed)
	content = AppendCltuLastOk(content, n.LastOk)
	content = AppendInteger(content, int64(n.ProductionStatus))
	return AppendInteger(content, int64(n.UplinkStatus)), nil
}

// DecodeFCLTUAsyncNotifyInvocation parses an ASYNC-NOTIFY invocation's content.
func DecodeFCLTUAsyncNotifyInvocation(data []byte) (*FCLTUAsyncNotifyInvocation, error) {
	d := NewDecoder(data)
	n := &FCLTUAsyncNotifyInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if n.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	notify, err := d.Next()
	if err != nil {
		return nil, err
	}
	if notify.Class != ClassContext || notify.Tag > uint32(NotifyEventConditionEvFalse) {
		return nil, ErrInvalidTag
	}
	n.Kind = CltuNotificationKind(notify.Tag)
	if n.Kind.CarriesEventId() {
		v, err := notify.Uint64()
		if err != nil {
			return nil, err
		}
		if v > 4294967295 {
			return nil, ErrIntegerOverflow
		}
		n.EventInvocationId = EventInvocationId(v)
	}

	processedElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if n.LastProcessed, err = DecodeCltuLastProcessed(processedElem); err != nil {
		return nil, err
	}

	okElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if n.LastOk, err = DecodeCltuLastOk(okElem); err != nil {
		return nil, err
	}

	production, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	n.ProductionStatus = FCLTUProductionStatus(production)

	uplink, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	n.UplinkStatus = UplinkStatus(uplink)
	return n, nil
}

// Humanize returns a human-readable summary.
func (n *FCLTUAsyncNotifyInvocation) Humanize() string {
	out := "FCLTU ASYNC-NOTIFY\n  Notification .... " + n.Kind.String()
	if n.Kind.CarriesEventId() {
		out += fmt.Sprintf("\n  Invocation ID ... %d", n.EventInvocationId)
	}
	return out + fmt.Sprintf("\n  Last processed .. %s\n  Last good ....... %s\n  Production ...... %s\n  Uplink .......... %s",
		n.LastProcessed, n.LastOk, n.ProductionStatus, n.UplinkStatus)
}

// FCLTUStatusReportInvocation is the CltuStatusReportInvocation of annex A2.5:
// the periodic summary of the forward channel.
type FCLTUStatusReportInvocation struct {
	Credentials      *Credentials
	LastProcessed    CltuLastProcessed
	LastOk           CltuLastOk
	ProductionStatus FCLTUProductionStatus
	UplinkStatus     UplinkStatus
	// NumberOfCltusReceived counts CLTUs the provider accepted.
	NumberOfCltusReceived uint32
	// NumberOfCltusProcessed counts those it took off the queue.
	NumberOfCltusProcessed uint32
	// NumberOfCltusRadiated counts those that actually went out. The gap
	// between these three is where CLTUs are being dropped.
	NumberOfCltusRadiated uint32
	// CltuBufferAvailable is the free buffer in octets.
	CltuBufferAvailable uint32
}

// Encode serializes the STATUS-REPORT invocation's content.
func (s *FCLTUStatusReportInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendCltuLastProcessed(content, s.LastProcessed)
	content = AppendCltuLastOk(content, s.LastOk)
	content = AppendInteger(content, int64(s.ProductionStatus))
	content = AppendInteger(content, int64(s.UplinkStatus))
	content = AppendInteger(content, int64(s.NumberOfCltusReceived))
	content = AppendInteger(content, int64(s.NumberOfCltusProcessed))
	content = AppendInteger(content, int64(s.NumberOfCltusRadiated))
	return AppendInteger(content, int64(s.CltuBufferAvailable)), nil
}

// DecodeFCLTUStatusReportInvocation parses a STATUS-REPORT invocation's
// content.
func DecodeFCLTUStatusReportInvocation(data []byte) (*FCLTUStatusReportInvocation, error) {
	d := NewDecoder(data)
	s := &FCLTUStatusReportInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	processedElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.LastProcessed, err = DecodeCltuLastProcessed(processedElem); err != nil {
		return nil, err
	}

	okElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if s.LastOk, err = DecodeCltuLastOk(okElem); err != nil {
		return nil, err
	}

	production, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	s.ProductionStatus = FCLTUProductionStatus(production)

	uplink, err := decodeInt(d)
	if err != nil {
		return nil, err
	}
	s.UplinkStatus = UplinkStatus(uplink)

	counts := []*uint32{
		&s.NumberOfCltusReceived, &s.NumberOfCltusProcessed,
		&s.NumberOfCltusRadiated, &s.CltuBufferAvailable,
	}
	for _, target := range counts {
		v, err := decodeUint32(d)
		if err != nil {
			return nil, err
		}
		*target = v
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *FCLTUStatusReportInvocation) Humanize() string {
	return fmt.Sprintf("FCLTU STATUS-REPORT\n"+
		"  Received ........ %d CLTUs\n"+
		"  Processed ....... %d CLTUs\n"+
		"  Radiated ........ %d CLTUs\n"+
		"  Buffer free ..... %d octets\n"+
		"  Last processed .. %s\n"+
		"  Last good ....... %s\n"+
		"  Production ...... %s\n"+
		"  Uplink .......... %s",
		s.NumberOfCltusReceived, s.NumberOfCltusProcessed, s.NumberOfCltusRadiated,
		s.CltuBufferAvailable, s.LastProcessed, s.LastOk,
		s.ProductionStatus, s.UplinkStatus)
}

// FCLTUUser is the user half of a Forward CLTU instance.
//
// It carries one piece of state the return services have no equivalent for:
// the next CLTU identification. CCSDS 912.1-B-5 §3.6.2.5.1 makes the number
// the user's to keep — it starts at the START invocation's
// firstCltuIdentification and goes up by one for every CLTU the provider
// accepts. Get it wrong and the provider answers 'out of sequence' and
// discards the CLTU, so the machine keeps the count rather than trusting the
// caller to.
//
// The number advances as each CLTU is sent, not as each return arrives —
// §3.1.6 expects the user to pipeline CLTUs without waiting for returns, and
// each one it sends must carry the next number. A refusal is where the count
// corrects itself: §3.6.2.5.2b makes the provider quote the number it
// expected, so a user that fell out of step resynchronises from the refusal.
//
// THROW-EVENT identifications work the same way (§3.9): the machine numbers
// each invocation, and a refusal quotes the identification the provider
// expected, which resynchronises the count.
type FCLTUUser struct {
	*ServiceUser

	// nextCltuID is the identification the next CLTU will carry.
	nextCltuID CltuIdentification
	// cltuIDKnown is false until a START has been accepted.
	cltuIDKnown bool
	// inFlight maps an invoke identifier to the CLTU number it carried, so a
	// return can be matched to its CLTU.
	inFlight map[InvokeId]CltuIdentification
	// nextEventID numbers the next THROW-EVENT invocation (§3.9.2.4).
	nextEventID EventInvocationId
}

// NewFCLTUUser prepares the user half of an FCLTU instance.
func NewFCLTUUser(config ServiceConfig) (*FCLTUUser, error) {
	config.Kind = ServiceFCLTU
	user, err := NewServiceUser(config)
	if err != nil {
		return nil, err
	}
	return &FCLTUUser{
		ServiceUser: user,
		inFlight:    make(map[InvokeId]CltuIdentification),
	}, nil
}

// NextCltuIdentification reports the number the next CLTU will carry, and
// whether a START has fixed it yet.
func (u *FCLTUUser) NextCltuIdentification() (CltuIdentification, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.nextCltuID, u.cltuIDKnown
}

// Start opens the CLTU stream, fixing the first CLTU's number. State 2 only,
// per §3.4.1.4.
func (u *FCLTUUser) Start(
	now time.Time, randomNumber int32, first CltuIdentification,
) (InvokeId, error) {
	id, err := u.invoke(OpStartInvocation, ServiceReady, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&FCLTUStartInvocation{
				Credentials:             creds,
				InvokeId:                id,
				FirstCltuIdentification: first,
			}).Encode()
		})
	if err != nil {
		return 0, err
	}
	u.mu.Lock()
	u.nextCltuID = first
	u.mu.Unlock()
	return id, nil
}

// HandleStartReturn takes the answer to START. A positive answer moves to
// state 3 and arms the CLTU counter at the number the START asked for.
func (u *FCLTUUser) HandleStartReturn(r *FCLTUStartReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.settle(r.InvokeId, OpStartInvocation); err != nil {
		return err
	}
	if r.Positive {
		u.startAccepted()
		u.cltuIDKnown = true
	}
	return nil
}

// TransferData queues one CLTU for radiation. State 3 only, per §3.6.1.
//
// The CLTU's identification is taken from the machine's counter, not from the
// caller: §3.6.2.5.1 defines it as a sequence the user must keep unbroken.
// The data is a CLTU as pkg/tcsc.WrapCLTU builds one — acquisition sequence,
// start sequence, codeblocks and tail sequence.
func (u *FCLTUUser) TransferData(
	now time.Time, randomNumber int32,
	cltu []byte, earliest, latest ConditionalTime,
	delay uint32, notification SlduStatusNotification,
) (InvokeId, CltuIdentification, error) {
	u.mu.Lock()
	if !u.cltuIDKnown {
		u.mu.Unlock()
		return 0, 0, ErrNotStarted
	}
	cltuID := u.nextCltuID
	u.mu.Unlock()

	invokeID, err := u.invoke(OpTransferDataInvocation, ServiceActive, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&FCLTUTransferDataInvocation{
				Credentials:              creds,
				InvokeId:                 id,
				CltuIdentification:       cltuID,
				EarliestTransmissionTime: earliest,
				LatestTransmissionTime:   latest,
				DelayTime:                delay,
				RadiationNotification:    notification,
				Data:                     cltu,
			}).Encode()
		})
	if err != nil {
		return 0, 0, err
	}

	u.mu.Lock()
	u.inFlight[invokeID] = cltuID
	// §3.1.6: the count advances as the CLTU is sent, so the next one can go
	// out before this one's return arrives (pipelining).
	u.nextCltuID = cltuID + 1
	u.mu.Unlock()
	return invokeID, cltuID, nil
}

// HandleTransferDataReturn takes the answer to one CLTU.
//
// On acceptance the counter has already moved — it advanced when the CLTU
// was sent — so the return changes nothing. On refusal the counter is set to
// the number the provider says it expects, which §3.6.2.5.2b guarantees is
// in the return: a user that lost its place recovers without another START.
func (u *FCLTUUser) HandleTransferDataReturn(r *FCLTUTransferDataReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.settle(r.InvokeId, OpTransferDataInvocation); err != nil {
		return err
	}
	delete(u.inFlight, r.InvokeId)

	if !r.Positive {
		u.nextCltuID = r.CltuIdentification
		return ErrCltuOutOfSequence
	}
	return nil
}

// ThrowEvent asks the provider's equipment to do something the service
// agreement defines. Valid in states 2 and 3, per §3.9.1.
//
// The event invocation identification comes from the machine's counter, not
// from the caller: §3.9.2.4 makes it a sequence the user must keep, exactly
// like the CLTU numbers. The identification used is returned, and it
// advances as the invocation is sent.
func (u *FCLTUUser) ThrowEvent(
	now time.Time, randomNumber int32, event uint16, qualifier []byte,
) (InvokeId, EventInvocationId, error) {
	u.mu.Lock()
	state := u.state
	eventID := u.nextEventID
	u.mu.Unlock()

	if state == ServiceUnbound {
		return 0, 0, ErrNotBound
	}
	invokeID, err := u.invoke(OpThrowEventInvocation, state, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&FCLTUThrowEventInvocation{
				Credentials:                   creds,
				InvokeId:                      id,
				EventInvocationIdentification: eventID,
				EventIdentifier:               event,
				EventQualifier:                qualifier,
			}).Encode()
		})
	if err != nil {
		return 0, 0, err
	}
	u.mu.Lock()
	u.nextEventID = eventID + 1
	u.mu.Unlock()
	return invokeID, eventID, nil
}

// NextEventInvocationId reports the identification the next THROW-EVENT will
// carry.
func (u *FCLTUUser) NextEventInvocationId() EventInvocationId {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.nextEventID
}

// HandleThrowEventReturn takes the answer to a THROW-EVENT.
//
// A positive answer means the provider accepted the request, not that the
// event happened: whether the actions ran arrives later, in an ASYNC-NOTIFY
// carrying actionListCompleted or actionListNotCompleted.
//
// A refusal echoes the event invocation identification the provider
// expected (§3.9.2.5), so the machine resynchronises its counter from it,
// the same recovery the CLTU numbers get.
func (u *FCLTUUser) HandleThrowEventReturn(r *FCLTUThrowEventReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.settle(r.InvokeId, OpThrowEventInvocation); err != nil {
		return err
	}
	if !r.Positive {
		u.nextEventID = r.EventInvocationIdentification
	}
	return nil
}

// FCLTUUserEvent is one decoded PDU arriving at the user.
type FCLTUUserEvent struct {
	Operation OperationType

	BindReturn                 *BindReturn
	UnbindReturn               *UnbindReturn
	StartReturn                *FCLTUStartReturn
	StopReturn                 *Acknowledgement
	ScheduleStatusReportReturn *ScheduleStatusReportReturn
	GetParameterReturn         *GetParameterReturn
	TransferDataReturn         *FCLTUTransferDataReturn
	ThrowEventReturn           *FCLTUThrowEventReturn
	AsyncNotify                *FCLTUAsyncNotifyInvocation
	StatusReport               *FCLTUStatusReportInvocation
	PeerAbort                  *PeerAbort
}

// HandlePDU decodes one PDU from the provider and advances the machine.
//
// A refused CLTU comes back as an event with TransferDataReturn set and
// ErrCltuOutOfSequence as the error: the PDU is valid and worth reading, and
// the error says the queue is no longer in step.
func (u *FCLTUUser) HandlePDU(data []byte, now time.Time) (*FCLTUUserEvent, error) {
	pdu, err := DecodePDU(data, ServiceFCLTU)
	if err != nil {
		return nil, err
	}
	u.Association().RecordReceived(now)

	event := &FCLTUUserEvent{Operation: pdu.Operation}

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
		r, err := DecodeFCLTUStartReturn(pdu.Content)
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

	case OpTransferDataReturn:
		r, err := DecodeFCLTUTransferDataReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.TransferDataReturn = r
		return event, u.HandleTransferDataReturn(r)

	case OpThrowEventReturn:
		r, err := DecodeFCLTUThrowEventReturn(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(r.Credentials, now); err != nil {
			return nil, err
		}
		event.ThrowEventReturn = r
		return event, u.HandleThrowEventReturn(r)

	case OpAsyncNotifyInvocation:
		if u.State() == ServiceUnbound {
			u.PeerAbort(AbortProtocolError, now)
			return event, ErrUnexpectedPDU
		}
		n, err := DecodeFCLTUAsyncNotifyInvocation(pdu.Content)
		if err != nil {
			return nil, err
		}
		if err := u.authenticate(n.Credentials, now); err != nil {
			return nil, err
		}
		event.AsyncNotify = n
		return event, nil

	case OpStatusReportInvocation:
		// Table 4-1: a STATUS-REPORT is legal only on a bound association.
		if u.State() == ServiceUnbound {
			u.PeerAbort(AbortProtocolError, now)
			return event, ErrUnexpectedPDU
		}
		report, err := DecodeFCLTUStatusReportInvocation(pdu.Content)
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

// FCLTUProvider is the provider half of an FCLTU instance. Partial.
//
// It does track the expected CLTU number, because that is what makes the
// out-of-sequence check testable, but it does not radiate anything: the
// caller decides what happens to a CLTU and reports it back through
// SendAsyncNotify.
type FCLTUProvider struct {
	*ServiceProvider

	// expectedCltuID is the number the next CLTU must carry.
	expectedCltuID CltuIdentification
	// cltuIDKnown is false until a START has been accepted.
	cltuIDKnown bool
}

// NewFCLTUProvider prepares the provider half of an FCLTU instance.
func NewFCLTUProvider(config ServiceConfig) (*FCLTUProvider, error) {
	config.Kind = ServiceFCLTU
	provider, err := NewServiceProvider(config)
	if err != nil {
		return nil, err
	}
	return &FCLTUProvider{ServiceProvider: provider}, nil
}

// ExpectedCltuIdentification reports the number the next CLTU must carry.
func (p *FCLTUProvider) ExpectedCltuIdentification() (CltuIdentification, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.expectedCltuID, p.cltuIDKnown
}

// HandleStartInvocation answers a START. Accepting moves to state 3 and arms
// the expected CLTU number from the invocation.
func (p *FCLTUProvider) HandleStartInvocation(
	s *FCLTUStartInvocation, answer *FCLTUStartReturn, now time.Time, randomNumber int32,
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
		p.expectedCltuID = s.FirstCltuIdentification
		p.cltuIDKnown = true
		p.mu.Unlock()
	}
	return nil
}

// HandleTransferDataInvocation answers one CLTU.
//
// It applies the sequence rule of §3.6.2.5 itself, because that rule is the
// protocol rather than a policy: a CLTU whose number is not the expected one
// is refused with 'out of sequence', and the return carries the number the
// provider still wants. Everything else — whether there is buffer space,
// whether the time window is sane — is the caller's decision, passed in
// through accept.
//
// bufferAvailable is the free buffer to report, in octets.
func (p *FCLTUProvider) HandleTransferDataInvocation(
	t *FCLTUTransferDataInvocation, accept bool, diagnostic FCLTUTransferDataDiagnostic,
	bufferAvailable uint32, now time.Time, randomNumber int32,
) error {
	p.mu.Lock()
	if p.state != ServiceActive {
		state := p.state
		p.mu.Unlock()
		return serviceStateError(state, ServiceActive)
	}
	inSequence := p.cltuIDKnown && t.CltuIdentification == p.expectedCltuID
	expected := p.expectedCltuID
	p.mu.Unlock()

	if !inSequence {
		accept = false
		diagnostic = FCLTUDataOutOfSequence
	}

	// §3.6.2.5.2: accepted means one greater than the CLTU just taken;
	// refused means the number still expected.
	reported := expected
	if accept {
		reported = t.CltuIdentification + 1
	}

	err := p.respond(OpTransferDataReturn, ServiceActive, now, randomNumber,
		func(creds *Credentials) ([]byte, error) {
			return (&FCLTUTransferDataReturn{
				Credentials:         creds,
				InvokeId:            t.InvokeId,
				CltuIdentification:  reported,
				CltuBufferAvailable: bufferAvailable,
				Positive:            accept,
				SpecificDiagnostic:  diagnostic,
			}).Encode()
		})
	if err != nil {
		return err
	}

	if accept {
		p.mu.Lock()
		p.expectedCltuID = reported
		p.mu.Unlock()
		return nil
	}
	if !inSequence {
		return ErrCltuOutOfSequence
	}
	return nil
}

// SendAsyncNotify queues a notification about a CLTU or the channel. Valid in
// states 2 and 3: a production status change is worth reporting whether or
// not CLTUs are flowing.
func (p *FCLTUProvider) SendAsyncNotify(n *FCLTUAsyncNotifyInvocation, now time.Time) error {
	content, err := n.Encode()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == ServiceUnbound {
		return ErrNotBound
	}
	if err := p.queue(OpAsyncNotifyInvocation, content); err != nil {
		return err
	}
	p.config.Association.RecordSent(now)
	return nil
}

// FCLTUProviderEvent is one decoded PDU arriving at the provider.
type FCLTUProviderEvent struct {
	Operation OperationType

	BindInvocation                 *BindInvocation
	UnbindInvocation               *UnbindInvocation
	StartInvocation                *FCLTUStartInvocation
	StopInvocation                 *StopInvocation
	ScheduleStatusReportInvocation *ScheduleStatusReportInvocation
	GetParameterInvocation         *GetParameterInvocation
	TransferDataInvocation         *FCLTUTransferDataInvocation
	ThrowEventInvocation           *FCLTUThrowEventInvocation
	PeerAbort                      *PeerAbort
}

// HandlePDU decodes one PDU from the user.
func (p *FCLTUProvider) HandlePDU(data []byte, now time.Time) (*FCLTUProviderEvent, error) {
	pdu, err := DecodePDU(data, ServiceFCLTU)
	if err != nil {
		return nil, err
	}
	p.Association().RecordReceived(now)

	event := &FCLTUProviderEvent{Operation: pdu.Operation}

	var creds *Credentials
	switch pdu.Operation {
	case OpBindInvocation:
		// BIND credentials are verified by Association.HandleBindInvocation,
		// which owns the bind-level policy.
		event.BindInvocation, err = DecodeBindInvocation(pdu.Content)
	case OpUnbindInvocation:
		if event.UnbindInvocation, err = DecodeUnbindInvocation(pdu.Content); err == nil {
			creds = event.UnbindInvocation.Credentials
		}
	case OpStartInvocation:
		if event.StartInvocation, err = DecodeFCLTUStartInvocation(pdu.Content); err == nil {
			creds = event.StartInvocation.Credentials
		}
	case OpStopInvocation:
		if event.StopInvocation, err = DecodeStopInvocation(pdu.Content); err == nil {
			creds = event.StopInvocation.Credentials
		}
	case OpScheduleStatusReportInvocation:
		if event.ScheduleStatusReportInvocation, err = DecodeScheduleStatusReportInvocation(pdu.Content); err == nil {
			creds = event.ScheduleStatusReportInvocation.Credentials
		}
	case OpGetParameterInvocation:
		if event.GetParameterInvocation, err = DecodeGetParameterInvocation(pdu.Content); err == nil {
			creds = event.GetParameterInvocation.Credentials
		}
	case OpTransferDataInvocation:
		if event.TransferDataInvocation, err = DecodeFCLTUTransferDataInvocation(pdu.Content); err == nil {
			creds = event.TransferDataInvocation.Credentials
		}
	case OpThrowEventInvocation:
		if event.ThrowEventInvocation, err = DecodeFCLTUThrowEventInvocation(pdu.Content); err == nil {
			creds = event.ThrowEventInvocation.Credentials
		}
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
	if pdu.Operation != OpBindInvocation {
		if err := p.authenticate(creds, now); err != nil {
			return nil, err
		}
	}
	if err := p.checkDuplicateInvokeId(event, now); err != nil {
		return event, err
	}
	return event, nil
}

// checkDuplicateInvokeId refuses a confirmed invocation whose invoke
// identifier has already been used on this association, queueing the
// negative return the specs require for it.
func (p *FCLTUProvider) checkDuplicateInvokeId(event *FCLTUProviderEvent, now time.Time) error {
	switch event.Operation {
	case OpStartInvocation:
		if !p.registerInvokeId(event.StartInvocation.InvokeId) {
			content, err := (&FCLTUStartReturn{
				InvokeId:         event.StartInvocation.InvokeId,
				UsedCommon:       true,
				CommonDiagnostic: DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpStartReturn, content, err, now)
		}
	case OpStopInvocation:
		if !p.registerInvokeId(event.StopInvocation.InvokeId) {
			content, err := (&Acknowledgement{
				InvokeId:   event.StopInvocation.InvokeId,
				Diagnostic: DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpStopReturn, content, err, now)
		}
	case OpScheduleStatusReportInvocation:
		if !p.registerInvokeId(event.ScheduleStatusReportInvocation.InvokeId) {
			content, err := (&ScheduleStatusReportReturn{
				InvokeId:         event.ScheduleStatusReportInvocation.InvokeId,
				UsedCommon:       true,
				CommonDiagnostic: DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpScheduleStatusReportReturn, content, err, now)
		}
	case OpGetParameterInvocation:
		if !p.registerInvokeId(event.GetParameterInvocation.InvokeId) {
			content, err := (&GetParameterReturn{
				InvokeId:         event.GetParameterInvocation.InvokeId,
				UsedCommon:       true,
				CommonDiagnostic: DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpGetParameterReturn, content, err, now)
		}
	case OpTransferDataInvocation:
		if !p.registerInvokeId(event.TransferDataInvocation.InvokeId) {
			expected, _ := p.ExpectedCltuIdentification()
			content, err := (&FCLTUTransferDataReturn{
				InvokeId:           event.TransferDataInvocation.InvokeId,
				CltuIdentification: expected,
				UsedCommon:         true,
				CommonDiagnostic:   DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpTransferDataReturn, content, err, now)
		}
	case OpThrowEventInvocation:
		if !p.registerInvokeId(event.ThrowEventInvocation.InvokeId) {
			content, err := (&FCLTUThrowEventReturn{
				InvokeId:                      event.ThrowEventInvocation.InvokeId,
				EventInvocationIdentification: event.ThrowEventInvocation.EventInvocationIdentification,
				UsedCommon:                    true,
				CommonDiagnostic:              DiagDuplicateInvokeId,
			}).Encode()
			return p.queueDuplicateAnswer(OpThrowEventReturn, content, err, now)
		}
	}
	return nil
}
