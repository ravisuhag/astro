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

// Service instance attribute object identifiers, from the
// SLE-SERVICE-INSTANCE-ID module of CCSDS 911.1-B-5 annex A. Every attribute
// name lives under {iso(1) identified-organization(3)
// standards-producing-organization(112) ccsds(4) sle-transfer-services(3)
// service-instance-id(1) attributes(2)}.
var serviceInstanceAttributeOIDs = map[string][]uint32{
	"sagr":    {1, 3, 112, 4, 3, 1, 2, 52},
	"spack":   {1, 3, 112, 4, 3, 1, 2, 53},
	"rsl-fg":  {1, 3, 112, 4, 3, 1, 2, 38},
	"fsl-fg":  {1, 3, 112, 4, 3, 1, 2, 14},
	"raf":     {1, 3, 112, 4, 3, 1, 2, 22},
	"rcf":     {1, 3, 112, 4, 3, 1, 2, 46},
	"rocf":    {1, 3, 112, 4, 3, 1, 2, 49},
	"cltu":    {1, 3, 112, 4, 3, 1, 2, 7},
	"antenna": {1, 3, 112, 4, 3, 1, 2, 3},
}

// serviceInstanceAttributeNames maps the dotted form back to the operator
// name, built once from the table above.
var serviceInstanceAttributeNames = func() map[string]string {
	out := make(map[string]string, len(serviceInstanceAttributeOIDs))
	for name, oid := range serviceInstanceAttributeOIDs {
		out[dottedOID(oid)] = name
	}
	return out
}()

// dottedOID renders arcs as the dotted string operators write.
func dottedOID(oid []uint32) string {
	var b strings.Builder
	for i, arc := range oid {
		if i > 0 {
			b.WriteByte('.')
		}
		fmt.Fprintf(&b, "%d", arc)
	}
	return b.String()
}

// parseDottedOID turns a dotted string back into arcs, reporting false when
// the string is not one.
func parseDottedOID(s string) ([]uint32, bool) {
	if s == "" {
		return nil, false
	}
	var out []uint32
	for _, part := range strings.Split(s, ".") {
		if part == "" {
			return nil, false
		}
		var arc uint64
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, false
			}
			arc = arc*10 + uint64(r-'0')
			if arc > 1<<32-1 {
				return nil, false
			}
		}
		out = append(out, uint32(arc))
	}
	return out, len(out) >= 2
}

// ServiceInstanceAttribute is one name-value pair of a service instance
// identifier.
type ServiceInstanceAttribute struct {
	// Identifier names the attribute. On the wire it is an OBJECT IDENTIFIER
	// from the SLE-SERVICE-INSTANCE-ID module; here it is the operator name (
	// "sagr", "spack", "rsl-fg", "fsl-fg", "raf", "rcf", "rocf", "cltu" or
	// "antenna") or a dotted OID string for an identifier this package does
	// not know by name.
	Identifier string
	// Value is the attribute value.
	Value string
	// Legacy reports that the identifier arrived as a VisibleString rather
	// than the OBJECT IDENTIFIER the module requires. Some older peers, and
	// earlier versions of this package, encoded it that way; the decoder
	// accepts the form and flags it here. This package always encodes OIDs.
	Legacy bool
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

	attrs, err := appendServiceInstanceIdentifier(nil, b.ServiceInstanceIdentifier)
	if err != nil {
		return nil, err
	}
	content = AppendSequence(content, attrs)

	return content, nil
}

// appendServiceInstanceIdentifier writes the attributes of a service instance
// identifier: each one a SET OF one SEQUENCE holding an OBJECT IDENTIFIER and
// a VisibleString, per the SLE-SERVICE-INSTANCE-ID module.
func appendServiceInstanceIdentifier(dst []byte, s ServiceInstanceIdentifier) ([]byte, error) {
	for _, a := range s {
		oid, ok := serviceInstanceAttributeOIDs[a.Identifier]
		if !ok {
			// Not a name this package knows; a dotted OID is still fine.
			if oid, ok = parseDottedOID(a.Identifier); !ok {
				return nil, ErrInvalidIdentifier
			}
		}
		pair, err := AppendObjectIdentifier(nil, oid)
		if err != nil {
			return nil, err
		}
		pair = AppendVisibleString(pair, a.Value)
		inner := AppendSequence(nil, pair)
		dst = AppendElement(dst, ClassUniversal, true, uint32(TagSet), inner)
	}
	return dst, nil
}

// decodeServiceInstanceAttribute reads one attribute pair. The identifier
// should be an OBJECT IDENTIFIER; the legacy VisibleString form is accepted
// and flagged.
func decodeServiceInstanceAttribute(pair *Decoder) (ServiceInstanceAttribute, error) {
	var a ServiceInstanceAttribute

	id, err := pair.Next()
	if err != nil {
		return a, err
	}
	switch {
	case id.IsUniversal(TagObjectIdentifier):
		oid, err := id.ObjectIdentifier()
		if err != nil {
			return a, err
		}
		dotted := dottedOID(oid)
		if name, ok := serviceInstanceAttributeNames[dotted]; ok {
			a.Identifier = name
		} else {
			a.Identifier = dotted
		}
	case id.IsUniversal(TagVisibleString):
		a.Identifier = id.String()
		a.Legacy = true
	default:
		return a, ErrInvalidTag
	}

	value, err := pair.Next()
	if err != nil {
		return a, err
	}
	a.Value = value.String()
	return a, nil
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
		attribute, err := decodeServiceInstanceAttribute(inner.Nested(seq))
		if err != nil {
			return nil, err
		}
		b.ServiceInstanceIdentifier = append(b.ServiceInstanceIdentifier, attribute)
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

// Encode serializes the PEER-ABORT's content: the bare two's complement
// octets of the diagnostic.
//
// The SLE modules are DEFINITIONS IMPLICIT TAGS, and the PDU CHOICE makes the
// operation [104] IMPLICIT SlePeerAbort, so [104] replaces the INTEGER tag
// rather than wrapping it. On the wire the whole PDU is a primitive
// context-specific element, 9F 68 01 xx, with no nested INTEGER TLV.
func (p *PeerAbort) Encode() []byte {
	return encodeIntegerContent(int64(p.Diagnostic))
}

// UrgentData returns the diagnostic as the single octet CCSDS 913.1-B-2 clause 3.4
// maps onto TCP urgent data. The aborting end sends this octet out of band
// (MSG_OOB) before closing the connection, so the peer can tell an abort from
// a failure; this package owns no socket, so sending it is the caller's job.
func (p *PeerAbort) UrgentData() byte {
	return byte(p.Diagnostic)
}

// DecodePeerAbort parses a PEER-ABORT's content: the bare diagnostic octets
// found under the primitive [104] tag.
//
// The legacy shape this package once emitted (a complete INTEGER TLV nested
// under a constructed [104]) is still recognized, so an old peer's abort is
// read rather than mistaken for a huge diagnostic.
func DecodePeerAbort(data []byte) (*PeerAbort, error) {
	if len(data) >= 3 && data[0] == TagInteger && int(data[1]) == len(data)-2 {
		e, err := NewDecoder(data).Next()
		if err == nil {
			if v, err := e.Int64(); err == nil {
				return &PeerAbort{Diagnostic: PeerAbortDiagnostic(v)}, nil
			}
		}
	}

	if len(data) == 0 {
		return nil, ErrDataTooShort
	}
	v, err := (&Element{Bytes: data}).Int64()
	if err != nil {
		return nil, err
	}
	return &PeerAbort{Diagnostic: PeerAbortDiagnostic(v)}, nil
}

// Humanize returns a human-readable summary.
func (p *PeerAbort) Humanize() string {
	return "SLE PEER-ABORT\n  Diagnostic ... " + p.Diagnostic.String()
}
