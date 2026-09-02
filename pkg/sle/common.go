package sle

import "fmt"

// Types shared by every SLE transfer service, from the common types module of
// CCSDS 911.1-B-5 annex A2.1.

// InvokeId correlates a confirmed operation's invocation with its return.
// IntUnsignedShort: INTEGER (0 .. 65535).
type InvokeId uint16

// Diagnostics is the common diagnostic set every service shares.
type Diagnostics int

const (
	// DiagDuplicateInvokeId means the invoke identifier is already in use.
	DiagDuplicateInvokeId Diagnostics = 100
	// DiagOtherReason covers everything else.
	DiagOtherReason Diagnostics = 127
)

// String names the diagnostic.
func (d Diagnostics) String() string {
	switch d {
	case DiagDuplicateInvokeId:
		return "duplicate invoke identifier"
	case DiagOtherReason:
		return "other reason"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(d))
	}
}

// MaxSpaceLinkDataUnit is the largest SpaceLinkDataUnit, per the common types
// module: OCTET STRING (SIZE (1 .. 65536)).
const MaxSpaceLinkDataUnit = 65536

// ConditionalTime is the CHOICE of the common types module: a time that may be
// absent.
//
//	ConditionalTime ::= CHOICE
//	{ undefined [0] NULL
//	, known     [1] Time
//	}
//
// RAF START uses it for the start and stop of a requested time range, where
// "undefined" means "from now" or "until further notice".
type ConditionalTime struct {
	// Known reports whether a time is present.
	Known bool
	Time  Time
}

// AppendConditionalTime writes a ConditionalTime CHOICE.
func AppendConditionalTime(dst []byte, c ConditionalTime) []byte {
	if !c.Known {
		return AppendElement(dst, ClassContext, false, 0, nil)
	}
	// The known alternative wraps a Time, which is itself a CHOICE.
	return AppendElement(dst, ClassContext, true, 1, AppendTimeChoice(nil, c.Time))
}

// DecodeConditionalTime reads a ConditionalTime CHOICE.
func DecodeConditionalTime(e *Element) (ConditionalTime, error) {
	switch {
	case e.IsContext(0):
		return ConditionalTime{}, nil
	case e.IsContext(1):
		inner, err := NewDecoder(e.Bytes).Next()
		if err != nil {
			return ConditionalTime{}, err
		}
		t, err := DecodeTimeChoice(inner)
		if err != nil {
			return ConditionalTime{}, err
		}
		return ConditionalTime{Known: true, Time: t}, nil
	default:
		return ConditionalTime{}, ErrInvalidTag
	}
}

// LockStatus reports whether one stage of the receive chain is locked, from
// the RAF structures module.
type LockStatus int

const (
	LockInLock    LockStatus = 0
	LockOutOfLock LockStatus = 1
	LockNotInUse  LockStatus = 2
	LockUnknown   LockStatus = 3
)

// String names the lock status.
func (l LockStatus) String() string {
	switch l {
	case LockInLock:
		return "in lock"
	case LockOutOfLock:
		return "out of lock"
	case LockNotInUse:
		return "not in use"
	default:
		return "unknown"
	}
}

// ProductionStatus is the state of the return physical channel, from the RAF
// structures module and annex B.
type ProductionStatus int

const (
	// ProductionRunning means the channel is producing data.
	ProductionRunning ProductionStatus = 0
	// ProductionInterrupted means production stopped unexpectedly and may resume.
	ProductionInterrupted ProductionStatus = 1
	// ProductionHalted means production ended and will not resume.
	ProductionHalted ProductionStatus = 2
)

// String names the production status.
func (p ProductionStatus) String() string {
	switch p {
	case ProductionRunning:
		return "running"
	case ProductionInterrupted:
		return "interrupted"
	case ProductionHalted:
		return "halted"
	default:
		return fmt.Sprintf("status(%d)", int(p))
	}
}

// AntennaId names the antenna a frame arrived on.
//
//	AntennaId ::= CHOICE
//	{ globalForm [0] OBJECT IDENTIFIER
//	, localForm  [1] OCTET STRING (SIZE (1 .. 16))
//	}
type AntennaId struct {
	// Global holds an object identifier when the global form is used.
	Global []byte
	// Local holds a local name, 1 to 16 octets, when the local form is used.
	Local []byte
}

// AppendAntennaId writes an AntennaId CHOICE.
func AppendAntennaId(dst []byte, a AntennaId) []byte {
	if len(a.Global) > 0 {
		return AppendElement(dst, ClassContext, false, 0, a.Global)
	}
	return AppendElement(dst, ClassContext, false, 1, a.Local)
}

// DecodeAntennaId reads an AntennaId CHOICE.
func DecodeAntennaId(e *Element) (AntennaId, error) {
	switch {
	case e.IsContext(0):
		return AntennaId{Global: e.Copy()}, nil
	case e.IsContext(1):
		return AntennaId{Local: e.Copy()}, nil
	default:
		return AntennaId{}, ErrInvalidTag
	}
}

// String renders the antenna identifier.
func (a AntennaId) String() string {
	if len(a.Local) > 0 {
		return string(a.Local)
	}
	return fmt.Sprintf("%x", a.Global)
}

// StopInvocation is the SleStopInvocation of the common PDUs module: a
// confirmed operation ending data transfer.
type StopInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
}

// Encode serializes the STOP invocation's content.
func (s *StopInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	return AppendInteger(content, int64(s.InvokeId)), nil
}

// DecodeStopInvocation parses a STOP invocation's content.
func DecodeStopInvocation(data []byte) (*StopInvocation, error) {
	d := NewDecoder(data)
	s := &StopInvocation{}

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
	return s, nil
}

// Acknowledgement is the SleAcknowledgement of the common PDUs module: the
// answer to STOP and to other operations that need only success or failure.
type Acknowledgement struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the operation succeeded.
	Positive bool
	// Diagnostic explains a failure.
	Diagnostic Diagnostics
}

// Encode serializes the acknowledgement's content.
func (a *Acknowledgement) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, a.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(a.InvokeId))

	// result CHOICE { positiveResult [0] NULL, negativeResult [1] Diagnostics }
	if a.Positive {
		content = AppendElement(content, ClassContext, false, 0, nil)
	} else {
		content = AppendTaggedInteger(content, 1, int64(a.Diagnostic))
	}
	return content, nil
}

// DecodeAcknowledgement parses an acknowledgement's content.
func DecodeAcknowledgement(data []byte) (*Acknowledgement, error) {
	d := NewDecoder(data)
	a := &Acknowledgement{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if a.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
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
	a.InvokeId = InvokeId(id)

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case result.IsContext(0):
		a.Positive = true
	case result.IsContext(1):
		v, err := result.Int64()
		if err != nil {
			return nil, err
		}
		a.Diagnostic = Diagnostics(v)
	default:
		return nil, ErrInvalidTag
	}
	return a, nil
}

// Humanize returns a human-readable summary.
func (a *Acknowledgement) Humanize() string {
	if a.Positive {
		return fmt.Sprintf("SLE Acknowledgement\n  Invoke ID ... %d\n  Result ...... positive", a.InvokeId)
	}
	return fmt.Sprintf("SLE Acknowledgement\n  Invoke ID ... %d\n  Result ...... negative: %s",
		a.InvokeId, a.Diagnostic)
}

// GVCID identifies a global virtual channel: a spacecraft, a transfer frame
// version, and a virtual channel within it.
//
// RCF and ROCF filter by it. The frame packages in this repository carry the
// same fields on their headers but have no shared type for the triple, so it
// lives here.
type GVCID struct {
	// SpacecraftID is the spacecraft identifier.
	SpacecraftID uint16
	// VersionNumber is the transfer frame version: 0 for TM, 1 for AOS.
	VersionNumber uint8
	// VirtualChannelID is the virtual channel, or absent for a master channel.
	VirtualChannelID uint8
	// MasterChannel reports whether this names a master channel rather than a
	// virtual one, in which case VirtualChannelID is meaningless.
	MasterChannel bool
}

// String renders the global virtual channel identifier.
func (g GVCID) String() string {
	if g.MasterChannel {
		return fmt.Sprintf("SCID %d, version %d, master channel", g.SpacecraftID, g.VersionNumber)
	}
	return fmt.Sprintf("SCID %d, version %d, VC %d", g.SpacecraftID, g.VersionNumber, g.VirtualChannelID)
}

// Transfer frame version numbers as a GVCID carries them, from the GvcId
// SEQUENCE of CCSDS 911.2-B-4 annex A.
//
// Note USLP is 12, not 4. The value is the four-bit Transfer Frame Version
// Number as it appears on the wire ('1100' binary) rather than the "Version
// 4" the protocol is called.
const (
	// FrameVersionTM is a TM Transfer Frame, called Version 1.
	FrameVersionTM uint8 = 0
	// FrameVersionAOS is an AOS Transfer Frame, called Version 2.
	FrameVersionAOS uint8 = 1
	// FrameVersionUSLP is a USLP Transfer Frame, called Version 4.
	FrameVersionUSLP uint8 = 12
)

// Validate checks a GVCID against the ranges of the GvcId SEQUENCE.
func (g GVCID) Validate() error {
	// VcId ::= INTEGER (0 .. 63), whatever the frame version.
	if !g.MasterChannel && g.VirtualChannelID > 63 {
		return ErrInvalidIdentifier
	}
	switch g.VersionNumber {
	case FrameVersionTM:
		// TM spacecraft identifiers are 10 bits.
		if g.SpacecraftID > 1023 {
			return ErrInvalidIdentifier
		}
	case FrameVersionAOS:
		// AOS spacecraft identifiers are 8 bits.
		if g.SpacecraftID > 255 {
			return ErrInvalidIdentifier
		}
	case FrameVersionUSLP:
		// USLP spacecraft identifiers are 16 bits, so any value fits.
	default:
		return ErrInvalidIdentifier
	}
	return nil
}

// AppendGVCID writes a GvcId SEQUENCE.
func AppendGVCID(dst []byte, g GVCID) ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	var content []byte
	content = AppendInteger(content, int64(g.SpacecraftID))
	content = AppendInteger(content, int64(g.VersionNumber))

	// vcId CHOICE { masterChannel [0] NULL, virtualChannel [1] VcId }
	if g.MasterChannel {
		content = AppendElement(content, ClassContext, false, 0, nil)
	} else {
		content = AppendTaggedInteger(content, 1, int64(g.VirtualChannelID))
	}
	return AppendSequence(dst, content), nil
}

// DecodeGVCID reads a GvcId SEQUENCE from an element.
func DecodeGVCID(e *Element) (GVCID, error) {
	d := NewDecoder(e.Bytes)
	var g GVCID

	scidElem, err := d.Next()
	if err != nil {
		return g, err
	}
	scid, err := scidElem.Uint64()
	if err != nil {
		return g, err
	}
	if scid > 65535 {
		return g, ErrIntegerOverflow
	}
	g.SpacecraftID = uint16(scid)

	versionElem, err := d.Next()
	if err != nil {
		return g, err
	}
	version, err := versionElem.Uint64()
	if err != nil {
		return g, err
	}
	g.VersionNumber = uint8(version)

	vcElem, err := d.Next()
	if err != nil {
		return g, err
	}
	switch {
	case vcElem.IsContext(0):
		g.MasterChannel = true
	case vcElem.IsContext(1):
		vc, err := vcElem.Uint64()
		if err != nil {
			return g, err
		}
		g.VirtualChannelID = uint8(vc)
	default:
		return g, ErrInvalidTag
	}

	return g, g.Validate()
}

// NotificationKind says which alternative a sync notification took, from the
// Notification CHOICE of CCSDS 911.1-B-5 annex A, which 911.2-B-4 and
// 911.5-B-4 repeat unchanged.
type NotificationKind int

const (
	// NotifyLossFrameSync reports that frame synchronization was lost, and
	// carries the lock status of each stage.
	NotifyLossFrameSync NotificationKind = 0
	// NotifyProductionStatusChange reports a change in the channel's state.
	NotifyProductionStatusChange NotificationKind = 1
	// NotifyExcessiveDataBacklog reports the provider cannot keep up.
	NotifyExcessiveDataBacklog NotificationKind = 2
	// NotifyEndOfData reports the requested range is exhausted.
	NotifyEndOfData NotificationKind = 3
)

// String names the notification.
func (n NotificationKind) String() string {
	switch n {
	case NotifyLossFrameSync:
		return "loss of frame synchronization"
	case NotifyProductionStatusChange:
		return "production status change"
	case NotifyExcessiveDataBacklog:
		return "excessive data backlog"
	default:
		return "end of data"
	}
}

// LockStatusReport is the lock state of each stage of the receive chain at a
// moment, carried by a loss-of-frame-sync notification.
type LockStatusReport struct {
	Time                 Time
	CarrierLockStatus    LockStatus
	SubcarrierLockStatus LockStatus
	SymbolSyncLockStatus LockStatus
}

// SyncNotifyInvocation is the sync notification a return service sends: the
// provider telling the user something happened to the channel.
//
// RAF, RCF and ROCF each define this SEQUENCE in their own ASN.1 module (
// RafSyncNotifyInvocation, RcfSyncNotifyInvocation, RocfSyncNotifyInvocation)
// and all three are the same two fields wrapping the same Notification
// CHOICE. One Go type covers all three.
type SyncNotifyInvocation struct {
	Credentials *Credentials
	Kind        NotificationKind
	// LockStatus is set when Kind is NotifyLossFrameSync.
	LockStatus *LockStatusReport
	// ProductionStatus is set when Kind is NotifyProductionStatusChange.
	ProductionStatus ProductionStatus
}

// Encode serializes the SYNC-NOTIFY invocation's content.
func (n *SyncNotifyInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, n.Credentials)
	if err != nil {
		return nil, err
	}

	switch n.Kind {
	case NotifyLossFrameSync:
		if n.LockStatus == nil {
			return nil, ErrDataTooShort
		}
		// lossFrameSync [0] IMPLICIT LockStatusReport: the module is
		// DEFINITIONS IMPLICIT TAGS, so [0] replaces the report's SEQUENCE
		// tag and the fields sit directly under it, no inner SEQUENCE.
		var report []byte
		report = AppendTimeChoice(report, n.LockStatus.Time)
		report = AppendInteger(report, int64(n.LockStatus.CarrierLockStatus))
		report = AppendInteger(report, int64(n.LockStatus.SubcarrierLockStatus))
		report = AppendInteger(report, int64(n.LockStatus.SymbolSyncLockStatus))
		content = AppendElement(content, ClassContext, true, 0, report)

	case NotifyProductionStatusChange:
		content = AppendTaggedInteger(content, 1, int64(n.ProductionStatus))

	case NotifyExcessiveDataBacklog:
		content = AppendElement(content, ClassContext, false, 2, nil)

	case NotifyEndOfData:
		content = AppendElement(content, ClassContext, false, 3, nil)

	default:
		return nil, ErrInvalidTag
	}
	return content, nil
}

// DecodeSyncNotifyInvocation parses a SYNC-NOTIFY invocation's content.
func DecodeSyncNotifyInvocation(data []byte) (*SyncNotifyInvocation, error) {
	d := NewDecoder(data)
	n := &SyncNotifyInvocation{}

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
	n.Kind = NotificationKind(notify.Tag)

	switch {
	case notify.IsContext(0):
		// The [0] tag replaces the LockStatusReport SEQUENCE (implicit
		// tagging), so its fields are directly under it.
		inner := NewDecoder(notify.Bytes)

		report := &LockStatusReport{}
		timeElem, err := inner.Next()
		if err != nil {
			return nil, err
		}
		if report.Time, err = DecodeTimeChoice(timeElem); err != nil {
			return nil, err
		}
		for _, target := range []*LockStatus{
			&report.CarrierLockStatus, &report.SubcarrierLockStatus, &report.SymbolSyncLockStatus,
		} {
			e, err := inner.Next()
			if err != nil {
				return nil, err
			}
			v, err := e.Int64()
			if err != nil {
				return nil, err
			}
			*target = LockStatus(v)
		}
		n.LockStatus = report

	case notify.IsContext(1):
		v, err := notify.Int64()
		if err != nil {
			return nil, err
		}
		n.ProductionStatus = ProductionStatus(v)

	case notify.IsContext(2), notify.IsContext(3):

	default:
		return nil, ErrInvalidTag
	}
	return n, nil
}

// Humanize returns a human-readable summary.
func (n *SyncNotifyInvocation) Humanize() string {
	out := "SYNC-NOTIFY\n  Notification ... " + n.Kind.String()
	if n.Kind == NotifyProductionStatusChange {
		out += "\n  Production ..... " + n.ProductionStatus.String()
	}
	return out
}

// ReportRequestKind says what a SCHEDULE-STATUS-REPORT is asking for, from the
// ReportRequestType CHOICE of the common PDUs module.
type ReportRequestKind int

const (
	// ReportImmediately asks for one report now.
	ReportImmediately ReportRequestKind = 0
	// ReportPeriodically asks for a report every so many seconds.
	ReportPeriodically ReportRequestKind = 1
	// ReportStop turns periodic reporting off.
	ReportStop ReportRequestKind = 2
)

// String names the request.
func (r ReportRequestKind) String() string {
	switch r {
	case ReportImmediately:
		return "immediately"
	case ReportPeriodically:
		return "periodically"
	case ReportStop:
		return "stop"
	default:
		return fmt.Sprintf("request(%d)", int(r))
	}
}

// MinReportingCycle and MaxReportingCycle bound a periodic reporting cycle:
// ReportingCycle ::= INTEGER (2 .. 600), in seconds.
const (
	MinReportingCycle = 2
	MaxReportingCycle = 600
)

// ScheduleStatusReportDiagnostic explains a refused SCHEDULE-STATUS-REPORT,
// from the specific alternative of DiagnosticScheduleStatusReport.
type ScheduleStatusReportDiagnostic int

const (
	// ScheduleNotSupportedInThisDeliveryMode means the mode has no reports.
	ScheduleNotSupportedInThisDeliveryMode ScheduleStatusReportDiagnostic = 0
	// ScheduleAlreadyStopped answers a stop when nothing was running.
	ScheduleAlreadyStopped ScheduleStatusReportDiagnostic = 1
	// ScheduleInvalidReportingCycle means the cycle was outside 2 to 600.
	ScheduleInvalidReportingCycle ScheduleStatusReportDiagnostic = 2
)

// String names the diagnostic.
func (s ScheduleStatusReportDiagnostic) String() string {
	switch s {
	case ScheduleNotSupportedInThisDeliveryMode:
		return "not supported in this delivery mode"
	case ScheduleAlreadyStopped:
		return "already stopped"
	case ScheduleInvalidReportingCycle:
		return "invalid reporting cycle"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(s))
	}
}

// ScheduleStatusReportInvocation is the SleScheduleStatusReportInvocation of
// the common PDUs module. All four services use it unchanged.
type ScheduleStatusReportInvocation struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Kind says whether to report once, periodically, or to stop.
	Kind ReportRequestKind
	// ReportingCycle is the period in seconds, used only when Kind is
	// ReportPeriodically.
	ReportingCycle uint16
}

// Encode serializes the SCHEDULE-STATUS-REPORT invocation's content.
func (s *ScheduleStatusReportInvocation) Encode() ([]byte, error) {
	content, err := AppendCredentialsChoice(nil, s.Credentials)
	if err != nil {
		return nil, err
	}
	content = AppendInteger(content, int64(s.InvokeId))

	switch s.Kind {
	case ReportImmediately, ReportStop:
		return AppendElement(content, ClassContext, false, uint32(s.Kind), nil), nil
	case ReportPeriodically:
		if s.ReportingCycle < MinReportingCycle || s.ReportingCycle > MaxReportingCycle {
			return nil, ErrInvalidReportingCycle
		}
		return AppendTaggedInteger(content, 1, int64(s.ReportingCycle)), nil
	default:
		return nil, ErrInvalidTag
	}
}

// DecodeScheduleStatusReportInvocation parses the invocation's content.
func DecodeScheduleStatusReportInvocation(data []byte) (*ScheduleStatusReportInvocation, error) {
	d := NewDecoder(data)
	s := &ScheduleStatusReportInvocation{}

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

	request, err := d.Next()
	if err != nil {
		return nil, err
	}
	switch {
	case request.IsContext(0):
		s.Kind = ReportImmediately
	case request.IsContext(2):
		s.Kind = ReportStop
	case request.IsContext(1):
		s.Kind = ReportPeriodically
		cycle, err := request.Uint64()
		if err != nil {
			return nil, err
		}
		if cycle < MinReportingCycle || cycle > MaxReportingCycle {
			return nil, ErrInvalidReportingCycle
		}
		s.ReportingCycle = uint16(cycle)
	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *ScheduleStatusReportInvocation) Humanize() string {
	if s.Kind == ReportPeriodically {
		return fmt.Sprintf("SLE SCHEDULE-STATUS-REPORT\n  Invoke ID ... %d\n  Request ..... every %d s",
			s.InvokeId, s.ReportingCycle)
	}
	return fmt.Sprintf("SLE SCHEDULE-STATUS-REPORT\n  Invoke ID ... %d\n  Request ..... %s",
		s.InvokeId, s.Kind)
}

// ScheduleStatusReportReturn is the SleScheduleStatusReportReturn of the
// common PDUs module.
type ScheduleStatusReportReturn struct {
	Credentials *Credentials
	InvokeId    InvokeId
	// Positive reports whether the provider accepted.
	Positive bool
	// CommonDiagnostic is set when a refusal used the common alternative.
	CommonDiagnostic Diagnostics
	// SpecificDiagnostic is set when it used the specific one.
	SpecificDiagnostic ScheduleStatusReportDiagnostic
	// UsedCommon says which alternative a refusal took.
	UsedCommon bool
}

// Encode serializes the SCHEDULE-STATUS-REPORT return's content.
func (s *ScheduleStatusReportReturn) Encode() ([]byte, error) {
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

// DecodeScheduleStatusReportReturn parses the return's content.
func DecodeScheduleStatusReportReturn(data []byte) (*ScheduleStatusReportReturn, error) {
	d := NewDecoder(data)
	s := &ScheduleStatusReportReturn{}

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
			s.SpecificDiagnostic = ScheduleStatusReportDiagnostic(v)
		}
	default:
		return nil, ErrInvalidTag
	}
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *ScheduleStatusReportReturn) Humanize() string {
	if s.Positive {
		return fmt.Sprintf("SLE SCHEDULE-STATUS-REPORT Return\n  Invoke ID ... %d\n  Result ...... accepted", s.InvokeId)
	}
	reason := s.SpecificDiagnostic.String()
	if s.UsedCommon {
		reason = s.CommonDiagnostic.String()
	}
	return fmt.Sprintf("SLE SCHEDULE-STATUS-REPORT Return\n  Invoke ID ... %d\n  Result ...... refused: %s",
		s.InvokeId, reason)
}
