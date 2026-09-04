package csts

import "github.com/ravisuhag/astro/internal/ber"

// The Association Control procedure of CCSDS 921.1-B-2 clause 4.3, whose
// operations are BIND, UNBIND and PEER-ABORT (annex F3.5).
//
// Every CSTS has this procedure (clause 4.2), and it is the only one whose
// operations are defined outside the common operations module — because the
// three of them exist to open and close the association rather than to move
// data through it.

// ServiceInstanceIdentifier names the service instance a BIND is for
// (annex F3.2, and clause 3.4.2.2.7).
//
// All three of its identifiers are OIDs registered with SANA rather than free
// text. That is a real difference from SLE, whose service instance identifier
// is a sequence of attribute-value pairs written as strings.
type ServiceInstanceIdentifier struct {
	SpacecraftID OID
	FacilityID   OID
	ServiceType  OID
	// InstanceNumber distinguishes two instances of the same service type
	// between the same two parties.
	InstanceNumber uint32
}

func appendServiceInstanceIdentifier(dst []byte, s ServiceInstanceIdentifier) ([]byte, error) {
	var content []byte
	var err error

	for _, oid := range []OID{s.SpacecraftID, s.FacilityID, s.ServiceType} {
		if content, err = ber.AppendObjectIdentifier(content, oid); err != nil {
			return nil, err
		}
	}
	content = ber.AppendInteger(content, int64(s.InstanceNumber))
	return ber.AppendSequence(dst, content), nil
}

func decodeServiceInstanceIdentifier(e *ber.Element) (ServiceInstanceIdentifier, error) {
	var s ServiceInstanceIdentifier
	if !e.IsUniversal(ber.TagSequence) {
		return s, ErrMalformedPDU
	}
	d := ber.NewDecoder(e.Bytes)

	for _, into := range []*OID{&s.SpacecraftID, &s.FacilityID, &s.ServiceType} {
		element, err := d.Next()
		if err != nil {
			return s, err
		}
		oid, err := element.ObjectIdentifier()
		if err != nil {
			return s, err
		}
		*into = OID(oid)
	}

	number, err := d.Next()
	if err != nil {
		return s, err
	}
	value, err := number.Uint64()
	if err != nil {
		return s, err
	}
	if value > 0xFFFFFFFF {
		return s, ErrIntegerRange
	}
	s.InstanceNumber = uint32(value)

	if !d.Empty() {
		return s, ErrTrailingContent
	}
	return s, nil
}

// AuthorityIdentifier length limits, from the SIZE constraint in annex F3.3.
const (
	MinAuthorityIdentifierLength = 3
	MaxAuthorityIdentifierLength = 16
)

// MaxPortIDLength is the SIZE ceiling annex F3.3 puts on a LogicalPortName.
const MaxPortIDLength = 128

// BindInvocation opens an association (clause 3.4, annex F3.5).
type BindInvocation struct {
	Header InvocationHeader
	// InitiatorIdentifier is the authority opening the association, 3 to 16
	// characters with no blanks.
	InitiatorIdentifier string
	// ResponderPortIdentifier names the responder's logical port. It is a
	// local matter between the two parties, not something registered.
	ResponderPortIdentifier string
	// ServiceType is the OID of the service being bound.
	ServiceType OID
	// VersionNumber is the service's version, typed IntPos so at least 1.
	VersionNumber   uint32
	ServiceInstance ServiceInstanceIdentifier
}

// Encode writes the invocation's content, without the framework PDU tag.
func (b *BindInvocation) encode() ([]byte, error) {
	if err := checkIdentifier(b.InitiatorIdentifier,
		MinAuthorityIdentifierLength, MaxAuthorityIdentifierLength); err != nil {
		return nil, err
	}
	if len(b.ResponderPortIdentifier) < 1 || len(b.ResponderPortIdentifier) > MaxPortIDLength {
		return nil, ErrIdentifierLength
	}
	if b.VersionNumber == 0 {
		// Annex F3.5 types VersionNumber as IntPos, which starts at 1.
		return nil, ErrInvalidVersion
	}

	content, err := appendInvocationHeader(nil, b.Header)
	if err != nil {
		return nil, err
	}
	content = ber.AppendVisibleString(content, b.InitiatorIdentifier)
	content = ber.AppendVisibleString(content, b.ResponderPortIdentifier)
	if content, err = ber.AppendObjectIdentifier(content, b.ServiceType); err != nil {
		return nil, err
	}
	content = ber.AppendInteger(content, int64(b.VersionNumber))
	if content, err = appendServiceInstanceIdentifier(content, b.ServiceInstance); err != nil {
		return nil, err
	}
	content = appendExtendedNotUsed(content)
	return content, nil
}

func decodeBindInvocation(content []byte) (*BindInvocation, error) {
	b := &BindInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if b.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}

	initiator, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.InitiatorIdentifier = initiator.String()
	if err := checkIdentifier(b.InitiatorIdentifier,
		MinAuthorityIdentifierLength, MaxAuthorityIdentifierLength); err != nil {
		return nil, err
	}

	port, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.ResponderPortIdentifier = port.String()

	serviceType, err := d.Next()
	if err != nil {
		return nil, err
	}
	oid, err := serviceType.ObjectIdentifier()
	if err != nil {
		return nil, err
	}
	b.ServiceType = OID(oid)

	version, err := d.Next()
	if err != nil {
		return nil, err
	}
	value, err := version.Uint64()
	if err != nil {
		return nil, err
	}
	if value == 0 || value > 0xFFFFFFFF {
		return nil, ErrInvalidVersion
	}
	b.VersionNumber = uint32(value)

	instance, err := d.Next()
	if err != nil {
		return nil, err
	}
	if b.ServiceInstance, err = decodeServiceInstanceIdentifier(instance); err != nil {
		return nil, err
	}
	return b, nil
}

// BindReturn answers a BIND (annex F3.5).
//
// It is the one return in the framework that adds a field to the standard
// header: the responder names itself, so the initiator can check it reached
// the authority it meant to. PEER-ABORT diagnostic 41 is
// 'unexpectedResponderId', which is what that check produces when it fails.
type BindReturn struct {
	Header ReturnHeader
	// ResponderIdentifier is the answering authority, 3 to 16 characters.
	ResponderIdentifier string
}

func (b *BindReturn) encode() ([]byte, error) {
	if err := checkIdentifier(b.ResponderIdentifier,
		MinAuthorityIdentifierLength, MaxAuthorityIdentifierLength); err != nil {
		return nil, err
	}
	content, err := appendReturnHeader(nil, b.Header)
	if err != nil {
		return nil, err
	}
	return ber.AppendVisibleString(content, b.ResponderIdentifier), nil
}

func decodeBindReturn(content []byte) (*BindReturn, error) {
	b := &BindReturn{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if b.Header, err = decodeReturnHeader(header); err != nil {
		return nil, err
	}

	responder, err := d.Next()
	if err != nil {
		return nil, err
	}
	b.ResponderIdentifier = responder.String()
	if err := checkIdentifier(b.ResponderIdentifier,
		MinAuthorityIdentifierLength, MaxAuthorityIdentifierLength); err != nil {
		return nil, err
	}
	return b, nil
}

// UnbindInvocation closes an association in an orderly way (clause 3.5).
type UnbindInvocation struct {
	Header InvocationHeader
}

func (u *UnbindInvocation) encode() ([]byte, error) {
	content, err := appendInvocationHeader(nil, u.Header)
	if err != nil {
		return nil, err
	}
	return appendExtendedNotUsed(content), nil
}

func decodeUnbindInvocation(content []byte) (*UnbindInvocation, error) {
	u := &UnbindInvocation{}
	d := ber.NewDecoder(content)

	header, err := d.Next()
	if err != nil {
		return nil, err
	}
	if u.Header, err = decodeInvocationHeader(header); err != nil {
		return nil, err
	}
	return u, nil
}

// UnbindReturn answers an UNBIND. Annex F3.5 defines it as the standard
// return header and nothing more.
type UnbindReturn struct {
	Header ReturnHeader
}

func (u *UnbindReturn) encode() ([]byte, error) {
	// UnbindReturn ::= StandardReturnHeader, so the [3] tag of annex F3.15
	// replaces the header's SEQUENCE tag rather than wrapping it.
	return appendReturnHeaderContent(nil, u.Header)
}

func decodeUnbindReturn(content []byte) (*UnbindReturn, error) {
	h, err := decodeReturnHeaderContent(content)
	if err != nil {
		return nil, err
	}
	return &UnbindReturn{Header: h}, nil
}

// PeerAbortInvocation tears an association down (clause 3.6).
//
// It is the only operation with no standard header, which clause 3.3.1.1 says
// outright. Annex F3.5 calls its ASN.1 a "dummy definition" and explains why:
// ISP1 carries a peer abort as a single octet of diagnostic, so anything more
// elaborate here could not be transmitted.
type PeerAbortInvocation struct {
	Diagnostic PeerAbortDiagnostic
}

func (p *PeerAbortInvocation) encode() ([]byte, error) {
	return ber.AppendOctetString(nil, []byte{byte(p.Diagnostic)}), nil
}

func decodePeerAbortInvocation(content []byte) (*PeerAbortInvocation, error) {
	e, err := ber.NewDecoder(content).Next()
	if err != nil {
		return nil, err
	}
	if len(e.Bytes) != 1 {
		// Annex F3.5 fixes the size at one octet.
		return nil, ErrMalformedPDU
	}
	return &PeerAbortInvocation{Diagnostic: PeerAbortDiagnostic(e.Bytes[0])}, nil
}

// checkIdentifier enforces a SIZE constraint and the IdentifierString rule of
// annex F3.3, which excludes the space character.
func checkIdentifier(s string, minLen, maxLen int) error {
	if len(s) < minLen || len(s) > maxLen {
		return ErrIdentifierLength
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			// IdentifierString is VisibleString (FROM (ALL EXCEPT " ")).
			return ErrIdentifierHasBlank
		}
	}
	return nil
}
