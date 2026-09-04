package csts

import (
	"github.com/ravisuhag/astro/internal/ber"
)

// The common types of CCSDS 921.1-B-2 annex F3.3.
//
// These are the building blocks every operation message is made of. What is
// here is what the framework's own operations need; the procedure-specific
// modules of F3.6 to F3.14 add their own on top, and a service specification
// adds more again.

// Credentials carry whatever a peer needs to authenticate an invocation.
//
// The framework does not say what is inside. Annex F3.3 notes that the
// structure depends on the algorithm and is therefore not specified, and
// clause 2.6 says an implementation using ISP1 — which is the default
// underlying protocol — uses the credentials algorithm of CCSDS 913.1-B-2.
// So these octets are the same shape pkg/sle builds, and this package carries
// them rather than interpreting them.
type Credentials struct {
	// Used says whether the CHOICE took the 'used' alternative. When it is
	// false the peer asked for no authentication and Value is empty.
	Used bool
	// Value is 8 to 256 octets when Used is set.
	Value []byte
}

// Unused returns credentials asserting that authentication is not required.
func Unused() Credentials { return Credentials{} }

// The context tags of the Credentials CHOICE.
const (
	tagCredentialsUnused uint32 = 0
	tagCredentialsUsed   uint32 = 1
)

// Credential size limits, from the SIZE constraint in annex F3.3.
const (
	MinCredentialsLength = 8
	MaxCredentialsLength = 256
)

func appendCredentials(dst []byte, c Credentials) ([]byte, error) {
	if !c.Used {
		return ber.AppendElement(dst, ber.ClassContext, false, tagCredentialsUnused, nil), nil
	}
	if len(c.Value) < MinCredentialsLength || len(c.Value) > MaxCredentialsLength {
		return nil, ErrCredentialsLength
	}
	return ber.AppendElement(dst, ber.ClassContext, false, tagCredentialsUsed, c.Value), nil
}

func decodeCredentials(e *ber.Element) (Credentials, error) {
	switch {
	case e.IsContext(tagCredentialsUnused):
		return Credentials{}, nil
	case e.IsContext(tagCredentialsUsed):
		if len(e.Bytes) < MinCredentialsLength || len(e.Bytes) > MaxCredentialsLength {
			return Credentials{}, ErrCredentialsLength
		}
		return Credentials{Used: true, Value: e.Copy()}, nil
	}
	return Credentials{}, ErrMalformedCredentials
}

// ProcedureRole says which of a service's procedures a message belongs to.
//
// A service has one prime procedure, any number of secondary ones, and always
// an Association Control procedure (clause 4.2). The role is a CHOICE rather
// than a number because those three are genuinely different things: the prime
// procedure drives the service state, a secondary procedure has a state of its
// own that the service ignores, and Association Control is neither.
type ProcedureRole int

const (
	// RolePrime is the prime procedure, whose state is the service's state.
	RolePrime ProcedureRole = iota
	// RoleSecondary is a secondary procedure, identified by its instance
	// number. Clause 4.2 lets a service have several.
	RoleSecondary
	// RoleAssociationControl is the Association Control procedure, which
	// every service has exactly one of.
	RoleAssociationControl
)

// The context tags of the procedureRole CHOICE in annex F3.3.
const (
	tagRolePrime       uint32 = 0
	tagRoleSecondary   uint32 = 1
	tagRoleAssociation uint32 = 2
)

func (r ProcedureRole) String() string {
	switch r {
	case RolePrime:
		return "prime"
	case RoleSecondary:
		return "secondary"
	case RoleAssociationControl:
		return "association control"
	}
	return "unknown"
}

// ProcedureName names the procedure instance an operation belongs to.
//
// This is the field that makes a CSTS PDU self-describing where an SLE one is
// not. The same operation — a START, a GET — means something different under
// each procedure of a service, and clause 3.3.2.5 puts the procedure in the
// message rather than leaving it to the association.
type ProcedureName struct {
	// Type is the procedure type OID, one of the OIDProcedures children for a
	// framework procedure or a service's own for anything else.
	Type OID
	Role ProcedureRole
	// Instance is the secondary procedure's instance number, meaningful only
	// when Role is RoleSecondary. Annex F3.3 types it IntPos, so it is at
	// least 1.
	Instance uint32
}

func appendProcedureName(dst []byte, p ProcedureName) ([]byte, error) {
	var content []byte
	var err error

	if content, err = ber.AppendObjectIdentifier(content, p.Type); err != nil {
		return nil, err
	}

	switch p.Role {
	case RolePrime:
		content = ber.AppendElement(content, ber.ClassContext, false, tagRolePrime, nil)
	case RoleAssociationControl:
		content = ber.AppendElement(content, ber.ClassContext, false, tagRoleAssociation, nil)
	case RoleSecondary:
		if p.Instance == 0 {
			// Annex F3.3 types the instance number IntPos, which starts at 1.
			return nil, ErrInvalidProcedureName
		}
		content = ber.AppendTaggedInteger(content, tagRoleSecondary, int64(p.Instance))
	default:
		return nil, ErrInvalidProcedureName
	}
	return ber.AppendSequence(dst, content), nil
}

func decodeProcedureName(e *ber.Element) (ProcedureName, error) {
	var p ProcedureName
	if !e.IsUniversal(ber.TagSequence) {
		return p, ErrInvalidProcedureName
	}
	d := ber.NewDecoder(e.Bytes)

	typeElement, err := d.Next()
	if err != nil {
		return p, err
	}
	oid, err := typeElement.ObjectIdentifier()
	if err != nil {
		return p, err
	}
	p.Type = OID(oid)

	role, err := d.Next()
	if err != nil {
		return p, err
	}
	switch {
	case role.IsContext(tagRolePrime):
		p.Role = RolePrime
	case role.IsContext(tagRoleAssociation):
		p.Role = RoleAssociationControl
	case role.IsContext(tagRoleSecondary):
		p.Role = RoleSecondary
		instance, err := role.Uint64()
		if err != nil {
			return p, err
		}
		if instance == 0 || instance > 0xFFFFFFFF {
			return p, ErrInvalidProcedureName
		}
		p.Instance = uint32(instance)
	default:
		return p, ErrInvalidProcedureName
	}

	if !d.Empty() {
		return p, ErrTrailingContent
	}
	return p, nil
}

// InvocationHeader is the StandardInvocationHeader of clause 3.3 and annex
// F3.3: what every operation invocation carries but the PEER-ABORT.
//
// Clause 3.3.1.1 makes PEER-ABORT the single exception, because an abort is
// not part of any procedure's dialogue and has nothing to be authenticated
// against — it carries one octet of diagnostic and nothing else.
type InvocationHeader struct {
	InvokerCredentials Credentials
	// InvokeID is an arbitrary integer the invoker picks (clause 3.3.2.4.1),
	// which the performer copies unchanged into every response.
	InvokeID  uint32
	Procedure ProcedureName
}

func appendInvocationHeader(dst []byte, h InvocationHeader) ([]byte, error) {
	var content []byte
	var err error

	if content, err = appendCredentials(content, h.InvokerCredentials); err != nil {
		return nil, err
	}
	content = ber.AppendInteger(content, int64(h.InvokeID))
	if content, err = appendProcedureName(content, h.Procedure); err != nil {
		return nil, err
	}
	return ber.AppendSequence(dst, content), nil
}

func decodeInvocationHeader(e *ber.Element) (InvocationHeader, error) {
	var h InvocationHeader
	if !e.IsUniversal(ber.TagSequence) {
		return h, ErrMalformedHeader
	}
	d := ber.NewDecoder(e.Bytes)

	credentials, err := d.Next()
	if err != nil {
		return h, err
	}
	if h.InvokerCredentials, err = decodeCredentials(credentials); err != nil {
		return h, err
	}

	invokeID, err := d.Next()
	if err != nil {
		return h, err
	}
	id, err := invokeID.Uint64()
	if err != nil {
		return h, err
	}
	if id > 0xFFFFFFFF {
		// Annex F3.3 types InvokeId as IntUnsigned, 0 to 2^32-1.
		return h, ErrIntegerRange
	}
	h.InvokeID = uint32(id)

	procedure, err := d.Next()
	if err != nil {
		return h, err
	}
	if h.Procedure, err = decodeProcedureName(procedure); err != nil {
		return h, err
	}

	if !d.Empty() {
		return h, ErrTrailingContent
	}
	return h, nil
}

// ReturnHeader is the StandardReturnHeader of clause 3.3 and annex F3.3,
// carried by every operation response and acknowledgement.
//
// StandardAcknowledgeHeader is defined as the same type, so an acknowledgement
// and a return are told apart by the PDU tag rather than by their content —
// the note under clause 3.3.1.3 says so outright.
type ReturnHeader struct {
	PerformerCredentials Credentials
	// InvokeID is the value the invoker supplied, copied unchanged
	// (clause 3.3.2.4.2).
	InvokeID uint32
	// Positive says whether the result CHOICE took the positive alternative.
	// When it is false, Diagnostic says why.
	Positive bool
	// Diagnostic is meaningful only on a negative result.
	Diagnostic Diagnostic
}

// The context tags of the result CHOICE in annex F3.3.
const (
	tagResultPositive uint32 = 0
	tagResultNegative uint32 = 1
)

// appendReturnHeader writes the header as a field of an enclosing type, with
// its own universal SEQUENCE tag.
func appendReturnHeader(dst []byte, h ReturnHeader) ([]byte, error) {
	content, err := appendReturnHeaderContent(nil, h)
	if err != nil {
		return nil, err
	}
	return ber.AppendSequence(dst, content), nil
}

// appendReturnHeaderContent writes the header's three fields without a
// SEQUENCE tag around them.
//
// Several alternatives of the framework PDU CHOICE are defined as
// 'X ::= StandardReturnHeader' rather than as a SEQUENCE containing one.
// Annex F3.15's module uses IMPLICIT TAGS, so the alternative's context tag
// replaces the SEQUENCE's universal tag instead of nesting inside it. Writing
// a SEQUENCE there as well would produce a PDU one level too deep, which
// decodes cleanly against this package's own reader and against nobody
// else's.
func appendReturnHeaderContent(dst []byte, h ReturnHeader) ([]byte, error) {
	content := dst
	var err error

	if content, err = appendCredentials(content, h.PerformerCredentials); err != nil {
		return nil, err
	}
	content = ber.AppendInteger(content, int64(h.InvokeID))

	if h.Positive {
		// The positive alternative carries an Extended, whose default is the
		// 'notUsed' NULL of clause F2.1.
		return ber.AppendElement(content, ber.ClassContext, true, tagResultPositive,
			appendExtendedNotUsed(nil)), nil
	}

	var negative []byte
	if negative, err = appendDiagnostic(negative, h.Diagnostic); err != nil {
		return nil, err
	}
	negative = appendExtendedNotUsed(negative)
	return ber.AppendElement(content, ber.ClassContext, true, tagResultNegative, negative), nil
}

// decodeReturnHeader reads a header carried as a field, with its own SEQUENCE
// tag around it.
func decodeReturnHeader(e *ber.Element) (ReturnHeader, error) {
	if !e.IsUniversal(ber.TagSequence) {
		return ReturnHeader{}, ErrMalformedHeader
	}
	return decodeReturnHeaderContent(e.Bytes)
}

// decodeReturnHeaderContent reads the header's three fields from content that
// the enclosing context tag already delimited.
func decodeReturnHeaderContent(content []byte) (ReturnHeader, error) {
	var h ReturnHeader
	d := ber.NewDecoder(content)

	credentials, err := d.Next()
	if err != nil {
		return h, err
	}
	if h.PerformerCredentials, err = decodeCredentials(credentials); err != nil {
		return h, err
	}

	invokeID, err := d.Next()
	if err != nil {
		return h, err
	}
	id, err := invokeID.Uint64()
	if err != nil {
		return h, err
	}
	if id > 0xFFFFFFFF {
		return h, ErrIntegerRange
	}
	h.InvokeID = uint32(id)

	result, err := d.Next()
	if err != nil {
		return h, err
	}
	switch {
	case result.IsContext(tagResultPositive):
		h.Positive = true
	case result.IsContext(tagResultNegative):
		inner := ber.NewDecoder(result.Bytes)
		diagnostic, err := inner.Next()
		if err != nil {
			return h, err
		}
		if h.Diagnostic, err = decodeDiagnostic(diagnostic); err != nil {
			return h, err
		}
	default:
		return h, ErrMalformedHeader
	}

	if !d.Empty() {
		return h, ErrTrailingContent
	}
	return h, nil
}

// appendExtendedNotUsed writes the Extended CHOICE's 'notUsed' alternative,
// which clause F2.1 makes the value when the extension capability is unused.
func appendExtendedNotUsed(dst []byte) []byte {
	return ber.AppendElement(dst, ber.ClassContext, false, tagExtendedNotUsed, nil)
}

// The context tags of the Extended CHOICE in annex F3.3.
const (
	tagExtendedExternal uint32 = 0
	tagExtendedNotUsed  uint32 = 1
)
