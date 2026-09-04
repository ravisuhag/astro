package sle

import "github.com/ravisuhag/astro/internal/ber"

// The BER codec this package uses lives in internal/ber.
//
// It moved there when pkg/csts arrived. The CSTS specification framework of
// CCSDS 921.1-B-2 is a different set of ASN.1 modules — it shares no data type
// with SLE — but it is carried by the same encoding, and one codec under both
// is better than two that can drift apart.
//
// The names stay here as aliases rather than being removed, because they were
// exported and a caller may be using them. An alias is the same type and the
// same function, so sle.NewDecoder and ber.NewDecoder are interchangeable.

// Tag classes, per X.690 clause 8.1.2.2.
const (
	ClassUniversal   = ber.ClassUniversal
	ClassApplication = ber.ClassApplication
	ClassContext     = ber.ClassContext
	ClassPrivate     = ber.ClassPrivate
)

// Constructed is the bit X.690 clause 8.1.2.5 sets on a constructed encoding.
const Constructed = ber.Constructed

// Universal tag numbers, per X.690 clause 8.
const (
	TagBoolean          = ber.TagBoolean
	TagInteger          = ber.TagInteger
	TagOctetString      = ber.TagOctetString
	TagNull             = ber.TagNull
	TagObjectIdentifier = ber.TagObjectIdentifier
	TagVisibleString    = ber.TagVisibleString
	TagSequence         = ber.TagSequence
	TagSet              = ber.TagSet
)

// DefaultMaxLength caps what one decoder will accept from a length field.
const DefaultMaxLength = ber.DefaultMaxLength

// Element is one decoded BER element.
type Element = ber.Element

// Decoder walks a BER encoding.
type Decoder = ber.Decoder

// The encoder.
var (
	AppendTag              = ber.AppendTag
	AppendLength           = ber.AppendLength
	AppendElement          = ber.AppendElement
	AppendInteger          = ber.AppendInteger
	AppendTaggedInteger    = ber.AppendTaggedInteger
	AppendOctetString      = ber.AppendOctetString
	AppendVisibleString    = ber.AppendVisibleString
	AppendNull             = ber.AppendNull
	AppendObjectIdentifier = ber.AppendObjectIdentifier
	AppendSequence         = ber.AppendSequence
	IntegerContent         = ber.IntegerContent
)

// The decoder.
var (
	NewDecoder          = ber.NewDecoder
	NewDecoderWithLimit = ber.NewDecoderWithLimit
)
