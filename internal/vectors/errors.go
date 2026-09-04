package vectors

import "sort"

// ErrorNames is the fixed error vocabulary. A reject vector names one of
// these. The set is deliberately closed: every consumer has to agree on
// what each name means, so adding one is a considered change.
//
// The names describe what a conforming implementation must refuse, not
// how any one of them phrases the refusal. Go sentinels differ per
// package (pkg/spp.ErrDataTooShort, pkg/aos.ErrFrameTooShort, and so on),
// which is idiomatic Go and deliberately not unified — so the mapping
// from a name to a sentinel lives per package, at the call site, and this
// file only fixes the vocabulary itself.
var ErrorNames = map[string]bool{
	// The octets end before a field the layout requires.
	"truncated": true,
	// A length field disagrees with the octets actually present, or with
	// the fixed length the channel agreed.
	"length_mismatch": true,
	// A check sequence over the data does not match it: a CRC-based FECF,
	// a packet error control field, a CFDP checksum.
	"crc_mismatch": true,
	// A code protecting a header does not verify: the Reed-Solomon frame
	// header error control of AOS and USLP, the BCH of a CLTU codeblock.
	// Distinct from crc_mismatch because these are not CRCs, and a corpus
	// that called them one would lie about what failed.
	"header_check_failed": true,
	// A value does not fit the bits the standard gives its field: an APID
	// above 2047, a VCID above 7.
	"field_out_of_range": true,
	// A field holds a value the standard reserves.
	"reserved_value": true,
	// A version or protocol identifier this implementation does not carry.
	"unsupported_version": true,
	// Octets remain after the structure the length field described.
	"trailing_data": true,
	// A caller-supplied output buffer is too short. Only reachable through
	// requires: ["encode_into"], which Go has no equivalent for.
	"buffer_too_small": true,
	// The octets parse as the underlying encoding but are not the form the
	// standard requires of it: a CBOR definite-length array where BPv7
	// clause 4.1 mandates the indefinite form, or an argument written wider
	// than the deterministic encoding of RFC 8949 clause 4.2.1 allows.
	// Distinct from reserved_value because no field holds a forbidden value —
	// the container itself is written the wrong way.
	"malformed_encoding": true,
}

// errorVocabulary returns the vocabulary in sorted order, for messages.
func errorVocabulary() []string {
	out := make([]string, 0, len(ErrorNames))
	for k := range ErrorNames {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
