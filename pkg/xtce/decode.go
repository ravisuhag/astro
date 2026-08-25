package xtce

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Turning a field of bits into a value.
//
// This is the "how is it written" half of a parameter type, and it is separate
// from the "what does it mean" half in extract.go. A 12-bit unsigned field is
// decoded the same way whether the parameter that carries it is an integer, an
// enumeration or a boolean; what differs is what happens to the number
// afterwards.

// decodeField reads one field and returns its raw value: the number, text or
// octets the packet carried, before any calibration.
func decodeField(reader bitReader, field Field) (any, error) {
	encoding := field.Type.Encoding()

	switch {
	case encoding == nil:
		return nil, fmt.Errorf("%w: parameter %q has no data encoding", ErrUnsupportedEncoding, field.Name)
	case encoding.Integer != nil:
		return decodeInteger(reader, field, encoding.Integer)
	case encoding.Float != nil:
		return decodeFloat(reader, field, encoding.Float)
	case encoding.String != nil:
		return decodeString(reader, field, encoding.String)
	default:
		return reader.readBytes(field.BitOffset, field.BitSize)
	}
}

// decodeInteger reads a whole-number field.
//
// It returns an int64 for the signed encodings and a uint64 for the unsigned
// ones, rather than making everything signed. A 64-bit unsigned field does not
// fit an int64, and quietly turning it negative would be worse than the
// caller having to check the type.
func decodeInteger(reader bitReader, field Field, encoding *IntegerDataEncoding) (any, error) {
	value, err := reader.read(field.BitOffset, field.BitSize)
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", field.Name, err)
	}

	value = applyBitOrder(value, field.BitSize, encoding.BitOrderOrDefault())
	value = applyByteOrder(value, field.BitSize, encoding.ByteOrderOrDefault())

	switch encoding.EncodingOrDefault() {
	case "unsigned":
		return value, nil

	case "twosComplement":
		return signExtend(value, field.BitSize), nil

	case "signMagnitude":
		// The top bit is the sign and the rest is the magnitude, so negative
		// zero exists and decodes to zero.
		if field.BitSize == 0 {
			return int64(0), nil
		}
		magnitude := value &^ (1 << (field.BitSize - 1))
		if value>>(field.BitSize-1)&1 == 1 {
			return -int64(magnitude), nil
		}
		return int64(magnitude), nil

	case "onesComplement":
		// Negative values are the bitwise complement of the magnitude, which
		// also gives two spellings of zero.
		if field.BitSize == 0 {
			return int64(0), nil
		}
		if value>>(field.BitSize-1)&1 == 1 {
			mask := uint64(1)<<field.BitSize - 1
			return -int64(^value & mask), nil
		}
		return int64(value), nil

	case "BCD":
		return decodeBCD(value, field.BitSize, false)

	case "packedBCD":
		return decodeBCD(value, field.BitSize, true)

	default:
		return nil, fmt.Errorf("%w: integer encoding %q", ErrUnsupportedEncoding, encoding.Encoding)
	}
}

// decodeBCD reads binary-coded decimal: four bits per decimal digit, most
// significant digit first.
//
// The packed form spends its lowest four bits on a sign nibble instead of a
// digit, which is what "packed" means in the schema's sense.
func decodeBCD(value uint64, width uint, packed bool) (any, error) {
	if width%4 != 0 {
		return nil, fmt.Errorf("%w: a %d-bit BCD field is not a whole number of nibbles",
			ErrUnsupportedEncoding, width)
	}

	nibbles := int(width / 4)

	sign := int64(1)
	if packed {
		if nibbles == 0 {
			return nil, fmt.Errorf("%w: a packed BCD field with no sign nibble", ErrUnsupportedEncoding)
		}
		// 0xD and 0xB mean negative; the rest mean positive.
		switch value & 0xF {
		case 0xD, 0xB:
			sign = -1
		}
		value >>= 4
		nibbles--
	}

	var result int64
	for i := nibbles - 1; i >= 0; i-- {
		digit := value >> uint(i*4) & 0xF
		if digit > 9 {
			return nil, fmt.Errorf("%w: BCD nibble %X is not a decimal digit", ErrUnsupportedEncoding, digit)
		}
		result = result*10 + int64(digit)
	}
	return sign * result, nil
}

// decodeFloat reads a real-number field.
func decodeFloat(reader bitReader, field Field, encoding *FloatDataEncoding) (any, error) {
	if kind := encoding.EncodingOrDefault(); kind != "IEEE754_1985" {
		// MILSTD_1750A and the two decimal forms. Each is a different layout
		// and none is common enough to code from the schema alone.
		return nil, fmt.Errorf("%w: float encoding %q", ErrUnsupportedEncoding, kind)
	}

	value, err := reader.read(field.BitOffset, field.BitSize)
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", field.Name, err)
	}

	value = applyBitOrder(value, field.BitSize, encoding.BitOrderOrDefault())
	value = applyByteOrder(value, field.BitSize, encoding.ByteOrderOrDefault())

	switch field.BitSize {
	case 16:
		return float64(float16(uint16(value))), nil
	case 32:
		return float64(math.Float32frombits(uint32(value))), nil
	case 64:
		return math.Float64frombits(value), nil
	default:
		// The schema also allows 40, 48, 80 and 128 bits. Those are not IEEE
		// 754 layouts and the schema does not say what they are.
		return nil, fmt.Errorf("%w: a %d-bit IEEE 754 float", ErrUnsupportedEncoding, field.BitSize)
	}
}

// float16 converts an IEEE 754 binary16 to a float32.
//
// Go has no float16, and the conversion is short enough to write out: one sign
// bit, five exponent bits, ten of mantissa, with the usual special cases for a
// zero exponent (subnormal) and an all-ones one (infinity or NaN).
func float16(bits uint16) float32 {
	sign := uint32(bits>>15) << 31
	exponent := uint32(bits >> 10 & 0x1F)
	mantissa := uint32(bits & 0x3FF)

	switch exponent {
	case 0:
		if mantissa == 0 {
			return math.Float32frombits(sign)
		}
		// Subnormal: renormalise into a float32, which has the range for it.
		exponent = 127 - 15 + 1
		for mantissa&0x400 == 0 {
			mantissa <<= 1
			exponent--
		}
		mantissa &= 0x3FF
		return math.Float32frombits(sign | exponent<<23 | mantissa<<13)

	case 0x1F:
		// Infinity or NaN, both of which keep an all-ones exponent.
		return math.Float32frombits(sign | 0xFF<<23 | mantissa<<13)

	default:
		// The bias differs between the two formats: 15 against 127.
		return math.Float32frombits(sign | (exponent-15+127)<<23 | mantissa<<13)
	}
}

// decodeString reads a text field.
func decodeString(reader bitReader, field Field, encoding *StringDataEncoding) (any, error) {
	raw, err := reader.readBytes(field.BitOffset, field.BitSize)
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", field.Name, err)
	}

	text, err := decodeText(raw, encoding.EncodingOrDefault())
	if err != nil {
		return nil, fmt.Errorf("parameter %q: %w", field.Name, err)
	}

	// A fixed-width string field is padded, and the schema's own examples pad
	// with NULs. Trailing NULs are not part of the value.
	return strings.TrimRight(text, "\x00"), nil
}

// decodeText applies the character encoding.
func decodeText(raw []byte, encoding string) (string, error) {
	switch encoding {
	case "UTF-8":
		if !utf8.Valid(raw) {
			return "", fmt.Errorf("%w: the field is not valid UTF-8", ErrUnsupportedEncoding)
		}
		return string(raw), nil

	case "UTF-16", "UTF-16BE":
		return decodeUTF16(raw, binary.BigEndian)

	case "UTF-16LE":
		return decodeUTF16(raw, binary.LittleEndian)

	default:
		return "", fmt.Errorf("%w: string encoding %q", ErrUnsupportedEncoding, encoding)
	}
}

// decodeUTF16 turns octets into text, two octets per code unit.
func decodeUTF16(raw []byte, order binary.ByteOrder) (string, error) {
	if len(raw)%2 != 0 {
		return "", fmt.Errorf("%w: a %d-octet UTF-16 field is not a whole number of code units",
			ErrUnsupportedEncoding, len(raw))
	}

	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(units)), nil
}
