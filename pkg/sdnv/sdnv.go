// Package sdnv encodes and decodes Self-Delimiting Numeric Values.
//
// An SDNV packs an unsigned integer into as few octets as it needs. The top
// bit of each octet says whether another follows: set means "more to come",
// clear means "this is the last one". The value is the 7 low bits of each
// octet concatenated, most significant first.
//
//	   0x7F  ->  01111111                    one octet
//	  0x80   ->  10000001 00000000           two octets
//	0x4234   ->  10000100 11000100 00110100  three octets
//
// The scheme comes from ASN.1 Object Identifier encoding. Two protocols in
// this library use it for nearly every variable-length field: LTP
// (RFC 5326 §1.6 item 20) and Bundle Protocol version 6 (RFC 5050 §4.1).
// It lives in its own package so neither has to carry a private copy, the
// same way pkg/crc serves the checksum users.
package sdnv

import (
	"errors"
	"io"
)

// MaxEncodedSize is the widest SDNV this package produces or accepts: ten
// octets, which is what a full 64-bit value needs at 7 bits per octet.
const MaxEncodedSize = 10

var (
	// ErrDataTooShort indicates the input ended mid-value, with every octet
	// so far having its continuation bit set.
	ErrDataTooShort = errors.New("sdnv: data ended before the value did")

	// ErrOverflow indicates a value too large for a uint64.
	ErrOverflow = errors.New("sdnv: value does not fit in 64 bits")
)

// EncodedSize returns how many octets Encode will produce for v.
func EncodedSize(v uint64) int {
	n := 1
	for v >>= 7; v > 0; v >>= 7 {
		n++
	}
	return n
}

// Encode returns the SDNV encoding of v.
func Encode(v uint64) []byte {
	return AppendEncode(nil, v)
}

// AppendEncode appends the SDNV encoding of v to dst and returns the extended
// slice. Use it to build a segment without an allocation per field.
func AppendEncode(dst []byte, v uint64) []byte {
	size := EncodedSize(v)
	start := len(dst)
	dst = append(dst, make([]byte, size)...)

	// Fill backwards: the last octet has its continuation bit clear.
	for i := size - 1; i >= 0; i-- {
		b := byte(v & 0x7F)
		if i != size-1 {
			b |= 0x80
		}
		dst[start+i] = b
		v >>= 7
	}
	return dst
}

// Decode reads one SDNV from the front of data, returning the value and the
// number of octets consumed.
//
// A value wider than 64 bits is an error rather than a silent wrap. So is an
// input that runs out while the continuation bit is still set.
func Decode(data []byte) (uint64, int, error) {
	var v uint64
	for i := 0; i < len(data); i++ {
		if i >= MaxEncodedSize {
			return 0, 0, ErrOverflow
		}
		b := data[i]

		// Adding 7 more bits must not push the value past 64.
		if v > (^uint64(0))>>7 {
			return 0, 0, ErrOverflow
		}
		v = v<<7 | uint64(b&0x7F)

		if b&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, ErrDataTooShort
}

// DecodeFrom reads one SDNV from r, one octet at a time. It reads no further
// than the end of the value.
func DecodeFrom(r io.ByteReader) (uint64, error) {
	var v uint64
	for i := 0; ; i++ {
		if i >= MaxEncodedSize {
			return 0, ErrOverflow
		}
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && i > 0 {
				return 0, ErrDataTooShort
			}
			return 0, err
		}

		if v > (^uint64(0))>>7 {
			return 0, ErrOverflow
		}
		v = v<<7 | uint64(b&0x7F)

		if b&0x80 == 0 {
			return v, nil
		}
	}
}

// DecodeN reads count consecutive SDNVs from the front of data, returning the
// values and the total octets consumed. It is the common case when a segment
// carries a run of adjacent SDNV fields.
func DecodeN(data []byte, count int) ([]uint64, int, error) {
	if count < 0 {
		return nil, 0, ErrDataTooShort
	}
	out := make([]uint64, 0, count)
	offset := 0
	for i := 0; i < count; i++ {
		v, n, err := Decode(data[offset:])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
		offset += n
	}
	return out, offset, nil
}
