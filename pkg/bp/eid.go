package bp

import (
	"strconv"
	"strings"
)

// SchemeCode is the integer standing in for a URI scheme name on the wire.
// Codes come from the "Bundle Protocol URI Scheme Types" registry
// (RFC 9171 clause 9.6). The registry is open; this package handles the two
// schemes RFC 9171 defines and refuses the rest rather than guessing at a
// scheme-specific part it cannot parse.
type SchemeCode uint64

const (
	// SchemeDTN is the "dtn" scheme: endpoints named by text strings.
	SchemeDTN SchemeCode = 1
	// SchemeIPN is the "ipn" scheme: endpoints named by numbers.
	SchemeIPN SchemeCode = 2
)

// maxIPNComponent bounds the allocator identifier and the node number. Both
// must fit in 32 bits so the two-element encoding can pack them into one
// 64-bit Fully Qualified Node Number (RFC 9758 clause 6.3).
const maxIPNComponent = 1 << 32

// EID identifies a bundle endpoint (RFC 9171 clause 4.2.5.1). On the wire it
// is a two-item CBOR array: a scheme code, then a scheme-specific part whose
// shape the scheme decides.
//
// Which fields matter depends on Scheme. For SchemeDTN only DTNSSP is read;
// for SchemeIPN only Allocator, Node and Service are.
//
// The ipn scheme carries an allocator identifier because RFC 9758 added one.
// RFC 9171 on its own has no such field, but the two agree on the wire: the
// two-element encoding packs allocator and node into one number, and when the
// allocator is zero — the Default Allocator — the octets are exactly what
// RFC 9171 clause 4.2.5.1.2 specifies. So an EID with Allocator zero is
// readable by an implementation that never heard of RFC 9758.
type EID struct {
	Scheme SchemeCode

	// DTNSSP is the scheme-specific part of a dtn-scheme EID. The value
	// "none" is the null endpoint, which encodes as a number rather than a
	// string (RFC 9171 clause 4.2.5.1.1).
	DTNSSP string

	// Allocator is the ipn allocator identifier (RFC 9758 clause 3.2). Zero is
	// the Default Allocator and the common case.
	Allocator uint64
	// Node is the ipn node number (RFC 9758 clause 3.3).
	Node uint64
	// Service is the ipn service number. Zero may identify a node's
	// administrative endpoint (RFC 9171 clause 4.2.5.1.2).
	Service uint64
}

// dtnNoneSSP is the scheme-specific part naming the null endpoint.
const dtnNoneSSP = "none"

// NullEID returns the null endpoint, dtn:none. It has no members, and a bundle
// uses it as the source when the sender stays anonymous
// (RFC 9171 clause 4.2.5.1.1).
func NullEID() EID {
	return EID{Scheme: SchemeDTN, DTNSSP: dtnNoneSSP}
}

// DTN returns a dtn-scheme EID with the given scheme-specific part.
func DTN(ssp string) EID {
	return EID{Scheme: SchemeDTN, DTNSSP: ssp}
}

// IPN returns an ipn-scheme EID under the Default Allocator, which is what
// almost every deployment uses. Its encoding is byte-identical to the one
// RFC 9171 clause 4.2.5.1.2 defines.
func IPN(node, service uint64) EID {
	return EID{Scheme: SchemeIPN, Node: node, Service: service}
}

// IPNWithAllocator returns an ipn-scheme EID under a named allocator
// (RFC 9758 clause 3.2).
func IPNWithAllocator(allocator, node, service uint64) EID {
	return EID{Scheme: SchemeIPN, Allocator: allocator, Node: node, Service: service}
}

// IsNull reports whether this EID names the null endpoint. Three spellings do:
// dtn:none, ipn:0.0 and ipn:0.0.0 (RFC 9758 clause 5.2).
func (e EID) IsNull() bool {
	switch e.Scheme {
	case SchemeDTN:
		return e.DTNSSP == dtnNoneSSP
	case SchemeIPN:
		return e.Allocator == 0 && e.Node == 0 && e.Service == 0
	}
	return false
}

// Validate reports whether the EID can be encoded.
func (e EID) Validate() error {
	switch e.Scheme {
	case SchemeDTN:
		return nil
	case SchemeIPN:
		if e.Allocator >= maxIPNComponent {
			return ErrIPNComponentTooLarge
		}
		if e.Node >= maxIPNComponent {
			return ErrIPNComponentTooLarge
		}
		return nil
	default:
		return ErrUnknownURIScheme
	}
}

// String renders the EID in its URI text form.
func (e EID) String() string {
	switch e.Scheme {
	case SchemeDTN:
		return "dtn:" + e.DTNSSP
	case SchemeIPN:
		// The three-number form appears only when an allocator is named;
		// RFC 9758 clause 4.1 keeps the two-number spelling otherwise.
		var b strings.Builder
		b.WriteString("ipn:")
		if e.Allocator != 0 {
			b.WriteString(strconv.FormatUint(e.Allocator, 10))
			b.WriteByte('.')
		}
		b.WriteString(strconv.FormatUint(e.Node, 10))
		b.WriteByte('.')
		b.WriteString(strconv.FormatUint(e.Service, 10))
		return b.String()
	}
	return "unknown-scheme:" + strconv.FormatUint(uint64(e.Scheme), 10)
}

// appendEID writes an EID as the two-item array of RFC 9171 clause 4.2.5.1.
//
// ipn EIDs go out in the two-element scheme-specific form. RFC 9758
// clause 6.1.1 makes that the backwards-compatible one: with the Default
// Allocator it is the same octets RFC 9171 asks for, so bundles this package
// writes are readable by implementations that predate RFC 9758.
func appendEID(dst []byte, e EID) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	dst = appendArrayHeader(dst, 2)
	dst = appendUint(dst, uint64(e.Scheme))

	switch e.Scheme {
	case SchemeDTN:
		if e.DTNSSP == dtnNoneSSP {
			// The null endpoint is the one dtn SSP that is a number.
			return appendUint(dst, 0), nil
		}
		return appendTextString(dst, e.DTNSSP), nil

	case SchemeIPN:
		dst = appendArrayHeader(dst, 2)
		dst = appendUint(dst, e.Allocator<<32|e.Node) // the Fully Qualified Node Number
		return appendUint(dst, e.Service), nil
	}

	return nil, ErrUnknownURIScheme
}

// eid reads an EID.
func (d *decoder) eid() (EID, error) {
	n, indefinite, err := d.arrayHeader()
	if err != nil {
		return EID{}, err
	}
	if indefinite || n != 2 {
		return EID{}, ErrMalformedEID
	}

	scheme, err := d.uint()
	if err != nil {
		return EID{}, err
	}

	switch SchemeCode(scheme) {
	case SchemeDTN:
		return d.dtnSSP()
	case SchemeIPN:
		return d.ipnSSP()
	default:
		return EID{}, ErrUnknownURIScheme
	}
}

// dtnSSP reads the scheme-specific part of a dtn EID. It is a text string,
// except for the null endpoint, which is the number zero
// (RFC 9171 clause 4.2.5.1.1).
func (d *decoder) dtnSSP() (EID, error) {
	head, err := d.peek()
	if err != nil {
		return EID{}, err
	}

	if head>>5 == majorUint {
		v, err := d.uint()
		if err != nil {
			return EID{}, err
		}
		if v != 0 {
			return EID{}, ErrMalformedEID
		}
		return NullEID(), nil
	}

	ssp, err := d.textString()
	if err != nil {
		return EID{}, err
	}
	return DTN(ssp), nil
}

// ipnSSP reads the scheme-specific part of an ipn EID, in either the two- or
// three-element form, following the decoding rule of RFC 9758 clause 6.2.
func (d *decoder) ipnSSP() (EID, error) {
	n, indefinite, err := d.arrayHeader()
	if err != nil {
		return EID{}, err
	}
	if indefinite || (n != 2 && n != 3) {
		return EID{}, ErrMalformedEID
	}

	first, err := d.uint()
	if err != nil {
		return EID{}, err
	}
	second, err := d.uint()
	if err != nil {
		return EID{}, err
	}

	if n == 2 {
		// The first item is a Fully Qualified Node Number: the allocator in
		// the high 32 bits, the node number in the low 32.
		//
		// RFC 9758 clause 6.2 prints this mask as "2^(32-1)", which would keep
		// 31 bits. That is a slip in the pseudocode: clause 3.3.1 defines the
		// node number as the low 32 bits, and clause 6.1.1's worked example
		// (ipn:977000.100.1 packing to 0x000EE86800000064) only comes out
		// right with a 32-bit mask. This uses 32 bits.
		return EID{
			Scheme:    SchemeIPN,
			Allocator: first >> 32,
			Node:      first & 0xFFFFFFFF,
			Service:   second,
		}, nil
	}

	third, err := d.uint()
	if err != nil {
		return EID{}, err
	}
	if first >= maxIPNComponent || second >= maxIPNComponent {
		return EID{}, ErrIPNComponentTooLarge
	}
	return EID{Scheme: SchemeIPN, Allocator: first, Node: second, Service: third}, nil
}
