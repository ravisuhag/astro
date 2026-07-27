package sle

import (
	"math/big"
)

// A BER subset, sized to what SLE protocol data units actually contain.
//
// Go's encoding/asn1 cannot do this job. It implements DER and rejects the
// context-specific CHOICE tagging that every SLE module relies on, so the
// alternative to hand-rolling was a third-party dependency this repository
// does not take.
//
// What is supported: the universal types SLE uses (BOOLEAN, INTEGER, NULL,
// OCTET STRING, VisibleString, SEQUENCE, SEQUENCE OF), context-specific tags
// in both primitive and constructed form, multi-octet tag numbers, and the
// definite-length form in both short and long variants.
//
// What is not: the indefinite-length form. Real providers do emit it, and this
// decoder returns ErrIndefiniteLength rather than guessing where a value ends.

// Tag classes, per X.690 §8.1.2.2.
const (
	// ClassUniversal is the class of the built-in ASN.1 types.
	ClassUniversal uint8 = 0x00
	// ClassApplication is the application class.
	ClassApplication uint8 = 0x40
	// ClassContext is the context-specific class, which SLE uses for every
	// CHOICE alternative and every tagged field.
	ClassContext uint8 = 0x80
	// ClassPrivate is the private class.
	ClassPrivate uint8 = 0xC0
)

// Constructed is the bit marking a constructed rather than primitive encoding
// (X.690 §8.1.2.5).
const Constructed uint8 = 0x20

// Universal tag numbers SLE uses.
const (
	TagBoolean       uint8 = 1
	TagInteger       uint8 = 2
	TagBitString     uint8 = 3
	TagOctetString   uint8 = 4
	TagNull          uint8 = 5
	TagEnumerated    uint8 = 10
	TagSequence      uint8 = 16
	TagSet           uint8 = 17
	TagVisibleString uint8 = 26
)

// DefaultMaxLength bounds a decoded BER value when no limit is given: 16 MiB.
//
// A BER length field can name a value far larger than any real SLE PDU. Sizing
// an allocation from one is a trivial denial of service, so the decoder always
// works against a ceiling.
const DefaultMaxLength = 16 << 20

// Element is one decoded BER tag-length-value.
type Element struct {
	// Class is the tag class: universal, application, context or private.
	Class uint8
	// Constructed reports whether the content is itself a run of elements.
	Constructed bool
	// Tag is the tag number.
	Tag uint32
	// Bytes is the content, with the tag and length already stripped.
	Bytes []byte
}

// IsContext reports whether this is a context-specific tag with the given number.
func (e *Element) IsContext(tag uint32) bool {
	return e.Class == ClassContext && e.Tag == tag
}

// IsUniversal reports whether this is a universal tag with the given number.
func (e *Element) IsUniversal(tag uint8) bool {
	return e.Class == ClassUniversal && e.Tag == uint32(tag)
}

// --- encoding ---

// AppendTag writes a BER identifier octet, or several when the tag number
// needs the high-tag-number form (X.690 §8.1.2.4).
func AppendTag(dst []byte, class uint8, constructed bool, tag uint32) []byte {
	first := class & 0xC0
	if constructed {
		first |= Constructed
	}

	if tag < 31 {
		return append(dst, first|byte(tag))
	}

	// High tag number form: the low five bits are all ones, then base-128
	// digits with the continuation bit set on all but the last.
	dst = append(dst, first|0x1F)

	var digits [5]byte
	n := 0
	for v := tag; ; v >>= 7 {
		digits[n] = byte(v & 0x7F)
		n++
		if v < 128 {
			break
		}
	}
	for i := n - 1; i >= 0; i-- {
		b := digits[i]
		if i != 0 {
			b |= 0x80
		}
		dst = append(dst, b)
	}
	return dst
}

// AppendLength writes a definite-form length, short or long as needed
// (X.690 §8.1.3).
func AppendLength(dst []byte, length int) []byte {
	if length < 128 {
		return append(dst, byte(length))
	}

	// Long form: an octet count with the high bit set, then the length
	// big-endian in as few octets as it needs.
	var buf [8]byte
	n := 0
	for v := length; v > 0; v >>= 8 {
		buf[n] = byte(v)
		n++
	}
	dst = append(dst, 0x80|byte(n))
	for i := n - 1; i >= 0; i-- {
		dst = append(dst, buf[i])
	}
	return dst
}

// AppendElement writes a complete tag-length-value.
func AppendElement(dst []byte, class uint8, constructed bool, tag uint32, content []byte) []byte {
	dst = AppendTag(dst, class, constructed, tag)
	dst = AppendLength(dst, len(content))
	return append(dst, content...)
}

// AppendInteger writes a universal INTEGER.
//
// BER integers are two's complement and minimally encoded: a leading zero
// octet appears only when it is needed to keep a positive value from looking
// negative (X.690 §8.3).
func AppendInteger(dst []byte, v int64) []byte {
	return AppendElement(dst, ClassUniversal, false, uint32(TagInteger), encodeIntegerContent(v))
}

// AppendTaggedInteger writes an INTEGER under a context-specific tag.
func AppendTaggedInteger(dst []byte, tag uint32, v int64) []byte {
	return AppendElement(dst, ClassContext, false, tag, encodeIntegerContent(v))
}

// encodeIntegerContent produces the minimal two's complement content octets.
func encodeIntegerContent(v int64) []byte {
	if v == 0 {
		return []byte{0}
	}

	var buf []byte
	n := v
	for {
		buf = append([]byte{byte(n)}, buf...)
		n >>= 8
		if n == 0 && buf[0]&0x80 == 0 {
			break
		}
		if n == -1 && buf[0]&0x80 != 0 {
			break
		}
	}
	return buf
}

// AppendOctetString writes a universal OCTET STRING.
func AppendOctetString(dst []byte, v []byte) []byte {
	return AppendElement(dst, ClassUniversal, false, uint32(TagOctetString), v)
}

// AppendVisibleString writes a universal VisibleString.
func AppendVisibleString(dst []byte, v string) []byte {
	return AppendElement(dst, ClassUniversal, false, uint32(TagVisibleString), []byte(v))
}

// AppendNull writes a universal NULL, which has no content.
func AppendNull(dst []byte) []byte {
	return AppendElement(dst, ClassUniversal, false, uint32(TagNull), nil)
}

// AppendSequence writes a universal SEQUENCE around already-encoded content.
func AppendSequence(dst []byte, content []byte) []byte {
	return AppendElement(dst, ClassUniversal, true, uint32(TagSequence), content)
}

// --- decoding ---

// Decoder walks a BER-encoded buffer.
//
// It is a value type over a slice, so a Decoder over a SEQUENCE's content is
// just another Decoder. That is how nesting works here: no reader interface,
// no allocation per level.
type Decoder struct {
	data   []byte
	offset int
	// maxLength bounds any single value. Zero selects DefaultMaxLength.
	maxLength int
}

// NewDecoder returns a decoder over data.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{data: data, maxLength: DefaultMaxLength}
}

// NewDecoderWithLimit returns a decoder with an explicit value ceiling.
func NewDecoderWithLimit(data []byte, maxLength int) *Decoder {
	if maxLength <= 0 {
		maxLength = DefaultMaxLength
	}
	return &Decoder{data: data, maxLength: maxLength}
}

// Empty reports whether the decoder has consumed everything.
func (d *Decoder) Empty() bool { return d.offset >= len(d.data) }

// Remaining returns how many octets are left.
func (d *Decoder) Remaining() int { return len(d.data) - d.offset }

// Next reads the next element.
func (d *Decoder) Next() (*Element, error) {
	if d.Empty() {
		return nil, ErrDataTooShort
	}

	first := d.data[d.offset]
	e := &Element{
		Class:       first & 0xC0,
		Constructed: first&Constructed != 0,
	}
	d.offset++

	// Tag number: the low five bits, or the high-tag-number form.
	if first&0x1F == 0x1F {
		var tag uint32
		for i := 0; ; i++ {
			if d.Empty() {
				return nil, ErrDataTooShort
			}
			// Five base-128 digits already exceed a uint32.
			if i >= 5 {
				return nil, ErrInvalidTag
			}
			b := d.data[d.offset]
			d.offset++

			if tag > (1<<32-1)>>7 {
				return nil, ErrInvalidTag
			}
			tag = tag<<7 | uint32(b&0x7F)
			if b&0x80 == 0 {
				break
			}
		}
		e.Tag = tag
	} else {
		e.Tag = uint32(first & 0x1F)
	}

	length, err := d.readLength()
	if err != nil {
		return nil, err
	}
	if length > d.Remaining() {
		return nil, ErrDataTooShort
	}

	e.Bytes = d.data[d.offset : d.offset+length]
	d.offset += length
	return e, nil
}

// readLength reads a definite-form length.
func (d *Decoder) readLength() (int, error) {
	if d.Empty() {
		return 0, ErrDataTooShort
	}
	first := d.data[d.offset]
	d.offset++

	if first < 0x80 {
		return int(first), nil
	}
	if first == 0x80 {
		// X.690 §8.1.3.6. Real providers emit it; this decoder will not guess
		// where the value ends.
		return 0, ErrIndefiniteLength
	}
	if first == 0xFF {
		// Reserved by §8.1.3.5 c).
		return 0, ErrInvalidLength
	}

	count := int(first & 0x7F)
	if count > 8 {
		return 0, ErrLengthTooLarge
	}
	if count > d.Remaining() {
		return 0, ErrDataTooShort
	}

	length := 0
	for i := 0; i < count; i++ {
		length = length<<8 | int(d.data[d.offset])
		d.offset++
		if length > d.maxLength {
			return 0, ErrLengthTooLarge
		}
	}
	if length < 0 {
		return 0, ErrInvalidLength
	}
	return length, nil
}

// Nested returns a decoder over a constructed element's content, carrying the
// same length ceiling.
func (d *Decoder) Nested(e *Element) *Decoder {
	return &Decoder{data: e.Bytes, maxLength: d.maxLength}
}

// --- value readers ---

// Int64 reads an element's content as a two's complement INTEGER.
func (e *Element) Int64() (int64, error) {
	if len(e.Bytes) == 0 {
		return 0, ErrDataTooShort
	}
	if len(e.Bytes) > 8 {
		// Anything wider than 8 octets cannot be an int64, though it is a
		// perfectly legal BER integer.
		return 0, ErrIntegerOverflow
	}

	v := int64(0)
	if e.Bytes[0]&0x80 != 0 {
		v = -1 // sign extend
	}
	for _, b := range e.Bytes {
		v = v<<8 | int64(b)
	}
	return v, nil
}

// Uint64 reads an element's content as a non-negative INTEGER.
func (e *Element) Uint64() (uint64, error) {
	v, err := e.Int64()
	if err != nil {
		// Fall back to big.Int for the one case an int64 cannot hold: a
		// positive value needing all 64 bits plus a leading zero octet.
		if len(e.Bytes) == 9 && e.Bytes[0] == 0 {
			return new(big.Int).SetBytes(e.Bytes[1:]).Uint64(), nil
		}
		return 0, err
	}
	if v < 0 {
		return 0, ErrIntegerOverflow
	}
	return uint64(v), nil
}

// String reads an element's content as text.
func (e *Element) String() string { return string(e.Bytes) }

// Bool reads an element's content as a BOOLEAN. X.690 §8.2.2: any non-zero
// octet is true.
func (e *Element) Bool() (bool, error) {
	if len(e.Bytes) != 1 {
		return false, ErrInvalidLength
	}
	return e.Bytes[0] != 0, nil
}

// Copy returns a copy of the element's content, so it does not alias the
// buffer the decoder was built over.
func (e *Element) Copy() []byte {
	out := make([]byte, len(e.Bytes))
	copy(out, e.Bytes)
	return out
}
