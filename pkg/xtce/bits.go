package xtce

import "fmt"

// Reading fields out of a packet.
//
// XTCE places a field by a bit offset and a bit width, so nothing is
// guaranteed to sit on an octet boundary. On top of that a field can be
// written in either bit order and either byte order, and the two interact in a
// way that is easy to get wrong. This file keeps that in one place.

// bitReader reads fields of arbitrary width at arbitrary offsets.
type bitReader struct {
	data []byte
}

// bitLen is the number of bits the packet holds.
func (r bitReader) bitLen() uint { return uint(len(r.data)) * 8 }

// read returns width bits starting at offset, as an unsigned value.
//
// The bits are taken most significant first, which is the schema's default bit
// order and the only order in which "the field starts at bit 12" means
// anything definite. Other orders are applied on top of this by the caller.
func (r bitReader) read(offset, width uint) (uint64, error) {
	if width > 64 {
		return 0, fmt.Errorf("%w: a %d-bit field does not fit a 64-bit value", ErrUnsupportedEncoding, width)
	}
	if end := offset + width; end > r.bitLen() {
		return 0, fmt.Errorf("%w: bits %d to %d of a %d-bit packet",
			ErrPacketTooShort, offset, end, r.bitLen())
	}

	var value uint64
	for i := range width {
		bit := offset + i
		value <<= 1
		value |= uint64(r.data[bit/8]>>(7-bit%8)) & 1
	}
	return value, nil
}

// readBytes returns width bits starting at offset, as octets.
//
// The result is left-aligned when the width is not a multiple of eight: the
// first bit read becomes the top bit of the first octet, and the last octet is
// padded with zeros. That is how a binary field of, say, twelve bits has to be
// handed back, since an octet is the smallest thing a []byte can hold.
func (r bitReader) readBytes(offset, width uint) ([]byte, error) {
	if end := offset + width; end > r.bitLen() {
		return nil, fmt.Errorf("%w: bits %d to %d of a %d-bit packet",
			ErrPacketTooShort, offset, end, r.bitLen())
	}

	out := make([]byte, (width+7)/8)

	// The aligned case is a copy, and binary fields are usually aligned.
	if offset%8 == 0 {
		copy(out, r.data[offset/8:])
		if trailing := width % 8; trailing != 0 {
			out[len(out)-1] &= ^byte(0) << (8 - trailing)
		}
		return out, nil
	}

	for i := range width {
		bit := offset + i
		if r.data[bit/8]>>(7-bit%8)&1 == 1 {
			out[i/8] |= 1 << (7 - i%8)
		}
	}
	return out, nil
}

// applyBitOrder reverses a field's bits when the encoding asks for
// leastSignificantBitFirst.
//
// The schema's other bit order says the first bit encountered is the least
// significant one, so reading the field most significant first and then
// reversing gives the same answer.
func applyBitOrder(value uint64, width uint, order string) uint64 {
	if order != "leastSignificantBitFirst" {
		return value
	}

	var reversed uint64
	for range width {
		reversed = reversed<<1 | value&1
		value >>= 1
	}
	return reversed
}

// applyByteOrder reorders a field's octets when the encoding asks for
// leastSignificantByteFirst.
//
// Byte order only means something for a field that is a whole number of
// octets. A 12-bit field has no octets to swap, and the schema does not say
// what swapping one would mean, so the value is returned untouched rather than
// mangled by a guess.
func applyByteOrder(value uint64, width uint, order string) uint64 {
	if order != "leastSignificantByteFirst" || width%8 != 0 || width == 8 {
		return value
	}

	octets := width / 8

	var swapped uint64
	for range octets {
		swapped = swapped<<8 | value&0xFF
		value >>= 8
	}
	return swapped
}

// signExtend turns a width-bit two's complement value into a signed one.
func signExtend(value uint64, width uint) int64 {
	if width == 0 || width >= 64 {
		return int64(value)
	}
	if value>>(width-1)&1 == 0 {
		return int64(value)
	}
	// Set every bit above the field.
	return int64(value | ^uint64(0)<<width)
}
