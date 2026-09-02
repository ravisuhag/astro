// Package bp implements the Bundle Protocol version 6
// per RFC 5050, profiled for space missions by CCSDS 734.2-B-1.
//
// Bundle Protocol is the network layer of Delay-Tolerant Networking. It moves
// application data units (bundles) hop by hop across links that are never
// all up at once, storing them at intermediate nodes rather than holding an
// end-to-end session open.
//
// # Version 6, not version 7
//
// CCSDS 734.2-B-1 profiles RFC 5050, which is Bundle Protocol version 6. It is
// NOT BPv7 (RFC 9171): BPv7 encodes bundles in CBOR and is wire-incompatible.
// This package implements what CCSDS specifies. BPv7 would be a separate
// package.
//
// The CCSDS profile adds two things on top of RFC 5050: the IPN naming scheme
// with Compressed Bundle Header Encoding (RFC 6260), and a mandatory Extended
// Class of Service block (annex C).
//
// A bundle is a primary block followed by one or more canonical blocks, the
// last of which is normally the payload:
//
//	[ primary block │ extension blocks... │ payload block ]
//
// Nearly every field is a Self-Delimiting Numeric Value, so this package
// builds on pkg/sdnv.
package bp

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is the bundle protocol version this package implements, per
// RFC 5050 clause 4.5.1.
const Version = 6

// IPNScheme is the naming scheme CCSDS 734.2-B-1 clause 3.2.1 requires, defined by
// RFC 6260 clause 2.1.
const IPNScheme = "ipn"

// DTNScheme is the scheme of the null endpoint "dtn:none".
const DTNScheme = "dtn"

// EndpointID names a bundle endpoint: a scheme and a scheme-specific part.
//
// On the wire an endpoint is a pair of offsets into the primary block's
// dictionary, which is how the same scheme string is shared between the
// destination, source, report-to and custodian without repeating it.
type EndpointID struct {
	Scheme string
	SSP    string
}

// NullEndpoint is "dtn:none", the endpoint that names nobody. RFC 5050 clause 4.4
// uses it for a bundle with no identifiable source.
var NullEndpoint = EndpointID{Scheme: DTNScheme, SSP: "none"}

// IsNull reports whether this is the null endpoint.
func (e EndpointID) IsNull() bool {
	return e.Scheme == DTNScheme && e.SSP == "none"
}

// String renders the endpoint as a URI.
func (e EndpointID) String() string {
	if e.Scheme == "" && e.SSP == "" {
		return "dtn:none"
	}
	return e.Scheme + ":" + e.SSP
}

// IPNEndpoint builds an endpoint in the IPN scheme that CCSDS mandates.
//
// The scheme-specific part is a node number and a service number separated by
// a period, per CCSDS 734.2-B-1 clause 3.2.1. Node numbers run 1 to 2^64-1 and are
// assigned by SANA; service numbers run 0 to 2^64-1.
func IPNEndpoint(node, service uint64) EndpointID {
	return EndpointID{
		Scheme: IPNScheme,
		SSP:    strconv.FormatUint(node, 10) + "." + strconv.FormatUint(service, 10),
	}
}

// IPNParts splits an IPN endpoint into its node and service numbers.
func (e EndpointID) IPNParts() (node, service uint64, err error) {
	if e.Scheme != IPNScheme {
		return 0, 0, ErrInvalidEndpointID
	}
	dot := strings.IndexByte(e.SSP, '.')
	if dot < 0 {
		return 0, 0, ErrInvalidEndpointID
	}
	node, err = strconv.ParseUint(e.SSP[:dot], 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidEndpointID
	}
	service, err = strconv.ParseUint(e.SSP[dot+1:], 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidEndpointID
	}
	// Clause 3.2.1: a node number is at least 1.
	if node == 0 {
		return 0, 0, ErrInvalidEndpointID
	}
	return node, service, nil
}

// ParseEndpointID parses a URI of the form "scheme:ssp".
func ParseEndpointID(uri string) (EndpointID, error) {
	colon := strings.IndexByte(uri, ':')
	if colon <= 0 || colon == len(uri)-1 {
		return EndpointID{}, ErrInvalidEndpointID
	}
	return EndpointID{Scheme: uri[:colon], SSP: uri[colon+1:]}, nil
}

// cbheParts returns the (node, service) pair RFC 6260 clause 2.1 assigns this
// endpoint in a CBHE-encoded primary block: an ipn endpoint contributes its
// node and service numbers, and the null endpoint dtn:none travels as (0, 0).
// ok is false when the endpoint fits neither form, which makes the whole
// bundle ineligible for CBHE.
func (e EndpointID) cbheParts() (node, service uint64, ok bool) {
	if e.IsNull() {
		return 0, 0, true
	}
	node, service, err := e.IPNParts()
	if err != nil {
		return 0, 0, false
	}
	return node, service, true
}

// cbheEndpoint rebuilds an endpoint from a CBHE (node, service) pair, per
// RFC 6260 clause 2.2: (0, 0) is the null endpoint, and node 0 with a nonzero
// service number names nothing.
func cbheEndpoint(node, service uint64) (EndpointID, error) {
	if node == 0 {
		if service != 0 {
			return EndpointID{}, ErrInvalidEndpointID
		}
		return NullEndpoint, nil
	}
	return IPNEndpoint(node, service), nil
}

// dictionary builds the primary block's dictionary and the offsets into it.
//
// RFC 5050 clause 4.4: the dictionary is a byte array of null-terminated strings,
// and each endpoint is a pair of offsets naming its scheme and its
// scheme-specific part. Identical strings are stored once, which is the whole
// point of the arrangement.
type dictionary struct {
	buf     []byte
	offsets map[string]uint64
}

func newDictionary() *dictionary {
	return &dictionary{offsets: make(map[string]uint64)}
}

// intern adds a string if it is not already present and returns its offset.
func (d *dictionary) intern(s string) uint64 {
	if offset, ok := d.offsets[s]; ok {
		return offset
	}
	offset := uint64(len(d.buf))
	d.offsets[s] = offset
	d.buf = append(d.buf, s...)
	d.buf = append(d.buf, 0)
	return offset
}

// add interns both halves of an endpoint and returns their offsets.
func (d *dictionary) add(e EndpointID) (scheme, ssp uint64) {
	return d.intern(e.Scheme), d.intern(e.SSP)
}

// lookupString reads the null-terminated string at an offset.
func lookupString(dict []byte, offset uint64) (string, error) {
	if offset > uint64(len(dict)) {
		return "", ErrDictionaryOffset
	}
	rest := dict[offset:]
	end := 0
	for end < len(rest) && rest[end] != 0 {
		end++
	}
	if end == len(rest) {
		// No terminator: the dictionary is malformed.
		return "", ErrDictionaryOffset
	}
	return string(rest[:end]), nil
}

// lookupEndpoint reads an endpoint from a pair of dictionary offsets.
func lookupEndpoint(dict []byte, schemeOffset, sspOffset uint64) (EndpointID, error) {
	scheme, err := lookupString(dict, schemeOffset)
	if err != nil {
		return EndpointID{}, err
	}
	ssp, err := lookupString(dict, sspOffset)
	if err != nil {
		return EndpointID{}, err
	}
	return EndpointID{Scheme: scheme, SSP: ssp}, nil
}

// Humanize returns a human-readable description of the endpoint.
func (e EndpointID) Humanize() string {
	if e.Scheme != IPNScheme {
		return e.String()
	}
	node, service, err := e.IPNParts()
	if err != nil {
		return e.String()
	}
	return fmt.Sprintf("%s (node %d, service %d)", e.String(), node, service)
}
