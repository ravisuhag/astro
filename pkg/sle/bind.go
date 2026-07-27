package sle

import (
	"fmt"
	"strings"
)

// The BIND, UNBIND and PEER-ABORT operations, from the common PDU module of
// CCSDS 911.1-B-5 annex A2.2. Every SLE service shares them.

// ApplicationIdentifier names the transfer service a BIND is asking for.
// Values from the ApplicationIdentifier INTEGER of annex A2.2.
type ApplicationIdentifier int

const (
	AppReturnAllFrames     ApplicationIdentifier = 0
	AppReturnInsert        ApplicationIdentifier = 1
	AppReturnChannelFrames ApplicationIdentifier = 2
	AppReturnChannelFSH    ApplicationIdentifier = 3
	AppReturnChannelOCF    ApplicationIdentifier = 4
	AppReturnBitstream     ApplicationIdentifier = 5
	AppReturnSpacePacket   ApplicationIdentifier = 6
	AppForwardAOSSpacePkt  ApplicationIdentifier = 7
	AppForwardAOSVCA       ApplicationIdentifier = 8
	AppForwardBitstream    ApplicationIdentifier = 9
	AppForwardProtoVCDU    ApplicationIdentifier = 10
	AppForwardInsert       ApplicationIdentifier = 11
	AppForwardCVCDU        ApplicationIdentifier = 12
	AppForwardTCSpacePkt   ApplicationIdentifier = 13
	AppForwardTCVCA        ApplicationIdentifier = 14
	AppForwardTCFrame      ApplicationIdentifier = 15
	AppForwardCLTU         ApplicationIdentifier = 16
)

// String names the service.
func (a ApplicationIdentifier) String() string {
	switch a {
	case AppReturnAllFrames:
		return "return all frames"
	case AppReturnInsert:
		return "return insert"
	case AppReturnChannelFrames:
		return "return channel frames"
	case AppReturnChannelFSH:
		return "return channel frame secondary header"
	case AppReturnChannelOCF:
		return "return channel operational control field"
	case AppReturnBitstream:
		return "return bitstream"
	case AppReturnSpacePacket:
		return "return space packet"
	case AppForwardCLTU:
		return "forward CLTU"
	default:
		return fmt.Sprintf("application(%d)", int(a))
	}
}

// BindDiagnostic explains a refused BIND, from the BindDiagnostic INTEGER of
// annex A2.2.
type BindDiagnostic int

const (
	BindAccessDenied                 BindDiagnostic = 0
	BindServiceTypeNotSupported      BindDiagnostic = 1
	BindVersionNotSupported          BindDiagnostic = 2
	BindNoSuchServiceInstance        BindDiagnostic = 3
	BindAlreadyBound                 BindDiagnostic = 4
	BindNotAccessibleToThisInitiator BindDiagnostic = 5
	BindInconsistentServiceType      BindDiagnostic = 6
	BindInvalidTime                  BindDiagnostic = 7
	BindOutOfService                 BindDiagnostic = 8
	BindOtherReason                  BindDiagnostic = 127
)

// String names the diagnostic.
func (b BindDiagnostic) String() string {
	switch b {
	case BindAccessDenied:
		return "access denied"
	case BindServiceTypeNotSupported:
		return "service type not supported"
	case BindVersionNotSupported:
		return "version not supported"
	case BindNoSuchServiceInstance:
		return "no such service instance"
	case BindAlreadyBound:
		return "already bound"
	case BindNotAccessibleToThisInitiator:
		return "service instance not accessible to this initiator"
	case BindInconsistentServiceType:
		return "inconsistent service type"
	case BindInvalidTime:
		return "invalid time"
	case BindOutOfService:
		return "out of service"
	case BindOtherReason:
		return "other reason"
	default:
		return fmt.Sprintf("diagnostic(%d)", int(b))
	}
}

// UnbindReason says why an association is ending, from the UnbindReason
// INTEGER of annex A2.2.
type UnbindReason int

const (
	// UnbindEnd means the service provision period has ended.
	UnbindEnd UnbindReason = 0
	// UnbindSuspend means the association is being suspended and may resume.
	UnbindSuspend UnbindReason = 1
	// UnbindVersionNotSupported means the peer cannot speak this version.
	UnbindVersionNotSupported UnbindReason = 2
	// UnbindOther covers everything else.
	UnbindOther UnbindReason = 127
)

// String names the reason.
func (u UnbindReason) String() string {
	switch u {
	case UnbindEnd:
		return "end"
	case UnbindSuspend:
		return "suspend"
	case UnbindVersionNotSupported:
		return "version not supported"
	case UnbindOther:
		return "other"
	default:
		return fmt.Sprintf("reason(%d)", int(u))
	}
}

// PeerAbortDiagnostic explains an abrupt end to an association, from the
// PeerAbortDiagnostic INTEGER of annex A2.2. Values 128 to 255 are reserved
// for the communications technology in use.
type PeerAbortDiagnostic int

const (
	AbortAccessDenied                PeerAbortDiagnostic = 0
	AbortUnexpectedResponderID       PeerAbortDiagnostic = 1
	AbortOperationalRequirement      PeerAbortDiagnostic = 2
	AbortProtocolError               PeerAbortDiagnostic = 3
	AbortCommunicationsFailure       PeerAbortDiagnostic = 4
	AbortEncodingError               PeerAbortDiagnostic = 5
	AbortReturnTimeout               PeerAbortDiagnostic = 6
	AbortEndOfServiceProvisionPeriod PeerAbortDiagnostic = 7
	AbortUnsolicitedInvokeID         PeerAbortDiagnostic = 8
	AbortOtherReason                 PeerAbortDiagnostic = 127
)

// String names the diagnostic.
func (p PeerAbortDiagnostic) String() string {
	switch p {
	case AbortAccessDenied:
		return "access denied"
	case AbortUnexpectedResponderID:
		return "unexpected responder identifier"
	case AbortOperationalRequirement:
		return "operational requirement"
	case AbortProtocolError:
		return "protocol error"
	case AbortCommunicationsFailure:
		return "communications failure"
	case AbortEncodingError:
		return "encoding error"
	case AbortReturnTimeout:
		return "return timeout"
	case AbortEndOfServiceProvisionPeriod:
		return "end of service provision period"
	case AbortUnsolicitedInvokeID:
		return "unsolicited invoke identifier"
	case AbortOtherReason:
		return "other reason"
	default:
		if p >= 128 {
			return fmt.Sprintf("technology specific (%d)", int(p))
		}
		return fmt.Sprintf("diagnostic(%d)", int(p))
	}
}

// validateIdentifier checks an AuthorityIdentifier: an IdentifierString of 3
// to 16 characters, and IdentifierString excludes the space (annex A2.2).
func validateIdentifier(s string) error {
	if len(s) < 3 || len(s) > 16 {
		return ErrInvalidIdentifier
	}
	if strings.ContainsRune(s, ' ') {
		return ErrInvalidIdentifier
	}
	return nil
}

// ServiceInstanceAttribute is one name-value pair of a service instance
// identifier.
type ServiceInstanceAttribute struct {
	// Identifier is the attribute's object identifier, as a dotted string,
	// for example the one naming an RAF service instance.
	Identifier string
	// Value is the attribute value.
	Value string
}

// ServiceInstanceIdentifier names a service instance: a sequence of attributes
// that together identify one configured service at the provider.
type ServiceInstanceIdentifier []ServiceInstanceAttribute

// String renders the identifier the way SLE operators write it, as
// name=value pairs.
func (s ServiceInstanceIdentifier) String() string {
	parts := make([]string, 0, len(s))
	for _, a := range s {
		parts = append(parts, a.Identifier+"="+a.Value)
	}
	return strings.Join(parts, ".")
}

// BindInvocation is the SleBindInvocation of annex A2.2. The user sends it to
// open an association.
type BindInvocation struct {
	// Credentials authenticate the initiator, or nil for an unauthenticated
	// association.
	Credentials *Credentials
	// InitiatorIdentifier names the sending entity, 3 to 16 characters.
	InitiatorIdentifier string
	// ResponderPortIdentifier names the provider's logical port.
	ResponderPortIdentifier string
	// ServiceType is the transfer service being requested.
	ServiceType ApplicationIdentifier
	// VersionNumber is the service version, 1 to 65535.
	VersionNumber uint16
	// ServiceInstanceIdentifier names the service instance.
	ServiceInstanceIdentifier ServiceInstanceIdentifier
}

// Validate checks the invocation against annex A2.2.
func (b *BindInvocation) Validate() error {
	if err := validateIdentifier(b.InitiatorIdentifier); err != nil {
		return err
	}
	// LogicalPortName is an IdentifierString of 1 to 128 characters.
	if len(b.ResponderPortIdentifier) < 1 || len(b.ResponderPortIdentifier) > 128 {
		return ErrInvalidIdentifier
	}
	// VersionNumber is IntPosShort: INTEGER (1 .. 65535).
	if b.VersionNumber < 1 {
		return ErrInvalidVersionNumber
	}
	return nil
}

// Encode serializes the BIND invocation content, without the outer service
// tag that each service module adds.
func (b *BindInvocation) Encode() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}

	var content []byte
	var err error
	if content, err = AppendCredentialsChoice(content, b.Credentials); err != nil {
		return nil, err
	}
	content = AppendVisibleString(content, b.InitiatorIdentifier)
	content = AppendVisibleString(content, b.ResponderPortIdentifier)
	content = AppendInteger(content, int64(b.ServiceType))
	content = AppendInteger(content, int64(b.VersionNumber))

	var attrs []byte
	for _, a := range b.ServiceInstanceIdentifier {
		var pair []byte
		pair = AppendVisibleString(pair, a.Identifier)
		pair = AppendVisibleString(pair, a.Value)
		// Each attribute is a SET OF one SEQUENCE, per the service instance
		// identifier module.
		inner := AppendSequence(nil, pair)
		attrs = AppendElement(attrs, ClassUniversal, true, uint32(TagSet), inner)
	}
	content = AppendSequence(content, attrs)

	return content, nil
}

// DecodeBindInvocation parses a BIND invocation's content.
func DecodeBindInvocation(data []byte) (*BindInvocation, error) {
	d := NewDecoder(data)
	b := &BindInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if b.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	initiator, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.InitiatorIdentifier = initiator.String()

	port, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.ResponderPortIdentifier = port.String()

	serviceType, err := d.Next()
	if err != nil {
		return nil, err
	}
	st, err := serviceType.Int64()
	if err != nil {
		return nil, err
	}
	b.ServiceType = ApplicationIdentifier(st)

	version, err := d.Next()
	if err != nil {
		return nil, err
	}
	v, err := version.Int64()
	if err != nil {
		return nil, err
	}
	if v < 1 || v > 65535 {
		return nil, ErrInvalidVersionNumber
	}
	b.VersionNumber = uint16(v)

	siElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	attrs := d.Nested(siElem)
	for !attrs.Empty() {
		set, err := attrs.Next()
		if err != nil {
			return nil, err
		}
		inner := attrs.Nested(set)
		seq, err := inner.Next()
		if err != nil {
			return nil, err
		}
		pair := inner.Nested(seq)

		id, err := pair.Next()
		if err != nil {
			return nil, err
		}
		value, err := pair.Next()
		if err != nil {
			return nil, err
		}
		b.ServiceInstanceIdentifier = append(b.ServiceInstanceIdentifier,
			ServiceInstanceAttribute{Identifier: id.String(), Value: value.String()})
	}

	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}

// Humanize returns a human-readable summary.
func (b *BindInvocation) Humanize() string {
	return fmt.Sprintf("SLE BIND Invocation\n"+
		"  Initiator ......... %s\n"+
		"  Responder port .... %s\n"+
		"  Service type ...... %s\n"+
		"  Version ........... %d\n"+
		"  Service instance .. %s",
		b.InitiatorIdentifier, b.ResponderPortIdentifier,
		b.ServiceType, b.VersionNumber, b.ServiceInstanceIdentifier)
}

// BindReturn is the SleBindReturn of annex A2.2: the provider's answer.
type BindReturn struct {
	Credentials *Credentials
	// ResponderIdentifier names the answering entity.
	ResponderIdentifier string
	// Positive reports whether the bind succeeded.
	Positive bool
	// VersionNumber is the agreed version, meaningful when Positive.
	VersionNumber uint16
	// Diagnostic explains a refusal, meaningful when not Positive.
	Diagnostic BindDiagnostic
}

// Encode serializes the BIND return's content.
func (b *BindReturn) Encode() ([]byte, error) {
	if err := validateIdentifier(b.ResponderIdentifier); err != nil {
		return nil, err
	}

	var content []byte
	var err error
	if content, err = AppendCredentialsChoice(content, b.Credentials); err != nil {
		return nil, err
	}
	content = AppendVisibleString(content, b.ResponderIdentifier)

	// result CHOICE { positive [0] VersionNumber, negative [1] BindDiagnostic }
	if b.Positive {
		if b.VersionNumber < 1 {
			return nil, ErrInvalidVersionNumber
		}
		content = AppendTaggedInteger(content, 0, int64(b.VersionNumber))
	} else {
		content = AppendTaggedInteger(content, 1, int64(b.Diagnostic))
	}
	return content, nil
}

// DecodeBindReturn parses a BIND return's content.
func DecodeBindReturn(data []byte) (*BindReturn, error) {
	d := NewDecoder(data)
	b := &BindReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if b.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	responder, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.ResponderIdentifier = responder.String()

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	value, err := result.Int64()
	if err != nil {
		return nil, err
	}

	switch {
	case result.IsContext(0):
		b.Positive = true
		if value < 1 || value > 65535 {
			return nil, ErrInvalidVersionNumber
		}
		b.VersionNumber = uint16(value)
	case result.IsContext(1):
		b.Positive = false
		b.Diagnostic = BindDiagnostic(value)
	default:
		return nil, ErrInvalidTag
	}
	return b, nil
}

// Humanize returns a human-readable summary.
func (b *BindReturn) Humanize() string {
	outcome := "refused: " + b.Diagnostic.String()
	if b.Positive {
		outcome = fmt.Sprintf("accepted, version %d", b.VersionNumber)
	}
	return "SLE BIND Return\n  Responder ... " + b.ResponderIdentifier +
		"\n  Result ...... " + outcome
}

// UnbindInvocation is the SleUnbindInvocation of annex A2.2.
type UnbindInvocation struct {
	Credentials *Credentials
	Reason      UnbindReason
}

// Encode serializes the UNBIND invocation's content.
func (u *UnbindInvocation) Encode() ([]byte, error) {
	var content []byte
	var err error
	if content, err = AppendCredentialsChoice(content, u.Credentials); err != nil {
		return nil, err
	}
	return AppendInteger(content, int64(u.Reason)), nil
}

// DecodeUnbindInvocation parses an UNBIND invocation's content.
func DecodeUnbindInvocation(data []byte) (*UnbindInvocation, error) {
	d := NewDecoder(data)
	u := &UnbindInvocation{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if u.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	reason, err := d.Next()
	if err != nil {
		return nil, err
	}
	r, err := reason.Int64()
	if err != nil {
		return nil, err
	}
	u.Reason = UnbindReason(r)
	return u, nil
}

// Humanize returns a human-readable summary.
func (u *UnbindInvocation) Humanize() string {
	return "SLE UNBIND Invocation\n  Reason ... " + u.Reason.String()
}

// UnbindReturn is the SleUnbindReturn of annex A2.2. Its result CHOICE has
// only a positive alternative: an UNBIND cannot be refused.
type UnbindReturn struct {
	Credentials *Credentials
}

// Encode serializes the UNBIND return's content.
func (u *UnbindReturn) Encode() ([]byte, error) {
	var content []byte
	var err error
	if content, err = AppendCredentialsChoice(content, u.Credentials); err != nil {
		return nil, err
	}
	// result CHOICE { positive [0] NULL }
	return AppendElement(content, ClassContext, false, 0, nil), nil
}

// DecodeUnbindReturn parses an UNBIND return's content.
func DecodeUnbindReturn(data []byte) (*UnbindReturn, error) {
	d := NewDecoder(data)
	u := &UnbindReturn{}

	credElem, err := d.Next()
	if err != nil {
		return nil, err
	}
	if u.Credentials, err = DecodeCredentialsChoice(credElem); err != nil {
		return nil, err
	}

	result, err := d.Next()
	if err != nil {
		return nil, err
	}
	if !result.IsContext(0) {
		return nil, ErrInvalidTag
	}
	return u, nil
}

// PeerAbort is the SlePeerAbort of annex A2.2: a bare diagnostic, with no
// credentials and no surrounding sequence.
type PeerAbort struct {
	Diagnostic PeerAbortDiagnostic
}

// Encode serializes the PEER-ABORT.
func (p *PeerAbort) Encode() []byte {
	return AppendInteger(nil, int64(p.Diagnostic))
}

// DecodePeerAbort parses a PEER-ABORT.
func DecodePeerAbort(data []byte) (*PeerAbort, error) {
	e, err := NewDecoder(data).Next()
	if err != nil {
		return nil, err
	}
	v, err := e.Int64()
	if err != nil {
		return nil, err
	}
	return &PeerAbort{Diagnostic: PeerAbortDiagnostic(v)}, nil
}

// Humanize returns a human-readable summary.
func (p *PeerAbort) Humanize() string {
	return "SLE PEER-ABORT\n  Diagnostic ... " + p.Diagnostic.String()
}
