package bp

import (
	"strconv"
	"strings"

	"github.com/ravisuhag/astro/internal/cbor"
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

	// form records which of two equivalent wire spellings this EID was
	// decoded from, for the two cases where RFC 9171 and RFC 9758 allow more
	// than one encoding of the same value: an ipn scheme-specific part as a
	// two- or three-element array, and the dtn null endpoint as the integer 0
	// or the text string "none".
	//
	// appendEID honours it so that re-encoding a decoded EID reproduces the
	// same octets, which pkg/bpsec depends on when it re-encodes the primary
	// block to build the integrity-protected plaintext (RFC 9171 clause
	// 4.3.1 requires that block to arrive unchanged). An EID a caller builds
	// by hand leaves this at its zero value, meaning "this library's
	// preferred form", so originated bundles are unaffected.
	form eidForm
}

// eidForm names a non-preferred wire spelling decodeIPNSSP or decodeDTNSSP
// read, so appendEID can reproduce it. The zero value means the preferred
// form and is what a caller-constructed EID carries.
type eidForm uint8

const (
	// formPreferred is this library's own spelling: an ipn two-element
	// scheme-specific part, and the dtn null endpoint as the integer 0.
	formPreferred eidForm = iota
	// formIPNThreeElement is an ipn scheme-specific part written as the
	// three-element array of RFC 9758 clause 6.1.1, naming the allocator
	// explicitly even when it is the Default Allocator.
	formIPNThreeElement
	// formDTNNoneAsText is the dtn null endpoint spelled as the text string
	// "none" rather than the integer 0 (RFC 9171 clause 4.2.5.1.1 allows
	// either).
	formDTNNoneAsText
)

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
// A caller-built ipn EID (form == formPreferred) goes out in the two-element
// scheme-specific form. RFC 9758 clause 6.1.1 makes that the
// backwards-compatible one: with the Default Allocator it is the same octets
// RFC 9171 asks for, so bundles this package originates are readable by
// implementations that predate RFC 9758. An EID decoded from the
// three-element form, or from the dtn null endpoint spelled as text, keeps
// that spelling — see the form field's doc comment.
func appendEID(dst []byte, e EID) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	dst = cbor.AppendArrayHeader(dst, 2)
	dst = cbor.AppendUint(dst, uint64(e.Scheme))

	switch e.Scheme {
	case SchemeDTN:
		if e.DTNSSP == dtnNoneSSP && e.form == formPreferred {
			// The null endpoint is the one dtn SSP that is a number, unless
			// it arrived spelled as text.
			return cbor.AppendUint(dst, 0), nil
		}
		return cbor.AppendTextString(dst, e.DTNSSP), nil

	case SchemeIPN:
		if e.form == formIPNThreeElement {
			dst = cbor.AppendArrayHeader(dst, 3)
			dst = cbor.AppendUint(dst, e.Allocator)
			dst = cbor.AppendUint(dst, e.Node)
			return cbor.AppendUint(dst, e.Service), nil
		}
		dst = cbor.AppendArrayHeader(dst, 2)
		dst = cbor.AppendUint(dst, e.Allocator<<32|e.Node) // the Fully Qualified Node Number
		return cbor.AppendUint(dst, e.Service), nil
	}

	return nil, ErrUnknownURIScheme
}

// eid reads an EID.
func decodeEIDFrom(d *cbor.Decoder) (EID, error) {
	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return EID{}, err
	}
	if indefinite || n != 2 {
		return EID{}, ErrMalformedEID
	}

	scheme, err := d.Uint()
	if err != nil {
		return EID{}, err
	}

	switch SchemeCode(scheme) {
	case SchemeDTN:
		return decodeDTNSSP(d)
	case SchemeIPN:
		return decodeIPNSSP(d)
	default:
		return EID{}, ErrUnknownURIScheme
	}
}

// dtnSSP reads the scheme-specific part of a dtn EID. It is a text string,
// except for the null endpoint, which is the number zero
// (RFC 9171 clause 4.2.5.1.1).
func decodeDTNSSP(d *cbor.Decoder) (EID, error) {
	head, err := d.Peek()
	if err != nil {
		return EID{}, err
	}

	if head>>5 == cbor.MajorUint {
		v, err := d.Uint()
		if err != nil {
			return EID{}, err
		}
		if v != 0 {
			return EID{}, ErrMalformedEID
		}
		return NullEID(), nil
	}

	ssp, err := d.TextString()
	if err != nil {
		return EID{}, err
	}
	e := DTN(ssp)
	if ssp == dtnNoneSSP {
		// The null endpoint spelled as text rather than as the integer 0
		// (RFC 9171 clause 4.2.5.1.1 allows both). Remember which one this
		// bundle used so re-encoding it does not silently switch spellings.
		e.form = formDTNNoneAsText
	}
	return e, nil
}

// ipnSSP reads the scheme-specific part of an ipn EID, in either the two- or
// three-element form, following the decoding rule of RFC 9758 clause 6.2.
func decodeIPNSSP(d *cbor.Decoder) (EID, error) {
	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return EID{}, err
	}
	if indefinite || (n != 2 && n != 3) {
		return EID{}, ErrMalformedEID
	}

	first, err := d.Uint()
	if err != nil {
		return EID{}, err
	}
	second, err := d.Uint()
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

	third, err := d.Uint()
	if err != nil {
		return EID{}, err
	}
	if first >= maxIPNComponent || second >= maxIPNComponent {
		return EID{}, ErrIPNComponentTooLarge
	}
	return EID{
		Scheme:    SchemeIPN,
		Allocator: first,
		Node:      second,
		Service:   third,
		// Remember the three-element spelling so re-encoding does not fold
		// it into the two-element form.
		form: formIPNThreeElement,
	}, nil
}

// Encode writes the endpoint ID as the two-item array of RFC 9171
// clause 4.2.5.1.
func (e EID) Encode() ([]byte, error) {
	return appendEID(nil, e)
}

// DecodeEID reads an endpoint ID that stands alone, and rejects any octet
// after it.
//
// Security blocks carry a security source as an endpoint ID inside their own
// CBOR (RFC 9172 clause 3.6). pkg/bpsec lifts that item out of the stream whole
// and hands it here, so the scheme rules stay in one place rather than being
// written a second time next to the security block that quotes them.
func DecodeEID(data []byte) (EID, error) {
	d := cbor.NewDecoder(data)
	e, err := decodeEIDFrom(d)
	if err != nil {
		return EID{}, err
	}
	if !d.AtEnd() {
		return EID{}, ErrTrailingBytes
	}
	return e, nil
}
