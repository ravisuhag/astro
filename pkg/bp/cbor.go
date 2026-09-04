package bp

// CBOR, the encoding BPv7 bundles are written in (RFC 8949, required by
// RFC 9171 clause 4.1).
//
// This is not a general CBOR library and must not grow into one. It carries
// only the pieces a bundle needs: unsigned integers, byte strings, text
// strings, definite and indefinite arrays, the break stop code, and the two
// tags the RFC 9171 appendix B grammar mentions. Every branch here should be
// traceable to something in a bundle.
//
// RFC 9171 clause 4.1 requires the core deterministic encoding of RFC 8949
// clause 4.2.1, with one relaxation: indefinite-length items stay legal. The
// part that bites is the argument encoding — a value must use the shortest
// head that can hold it, so 40 is 0x18 0x28 and never 0x1A 0x00 0x00 0x00 0x28.
// The encoder here always emits the shortest head, and the decoder refuses
// anything longer. That strictness is a choice: clause 4.1 lets an
// implementation accept sloppy input and repair it, and this one does not.

// CBOR major types, the top three bits of a head byte (RFC 8949 clause 3.1).
const (
	majorUint    = 0
	majorNegInt  = 1
	majorByteStr = 2
	majorTextStr = 3
	majorArray   = 4
	majorMap     = 5
	majorTag     = 6
	majorSimple  = 7
)

// Additional-information values with a meaning of their own (RFC 8949 clause 3).
const (
	aiOneByte    = 24 // argument is the next octet
	aiTwoByte    = 25 // ... the next two
	aiFourByte   = 26 // ... the next four
	aiEightByte  = 27 // ... the next eight
	aiIndefinite = 31 // no argument; the item runs until a break
	aiMaxInline  = 23 // the largest argument a head byte carries by itself
)

// breakStop closes an indefinite-length item (RFC 8949 clause 3.2.1).
const breakStop = 0xFF

// Tags the RFC 9171 appendix B grammar allows. Neither is required, and this
// package never emits one, but a decoder meets them in the wild.
const (
	tagEmbeddedCBOR  = 24    // a byte string whose content is itself CBOR
	tagSelfDescribed = 55799 // the "this is CBOR" marker, RFC 8949 clause 3.4.6
)

// appendHead writes one item head: the major type and its argument, in the
// shortest form that holds the value.
func appendHead(dst []byte, major byte, arg uint64) []byte {
	base := major << 5
	switch {
	case arg <= aiMaxInline:
		return append(dst, base|byte(arg))
	case arg <= 0xFF:
		return append(dst, base|aiOneByte, byte(arg))
	case arg <= 0xFFFF:
		return append(dst, base|aiTwoByte, byte(arg>>8), byte(arg))
	case arg <= 0xFFFFFFFF:
		return append(dst, base|aiFourByte,
			byte(arg>>24), byte(arg>>16), byte(arg>>8), byte(arg))
	default:
		return append(dst, base|aiEightByte,
			byte(arg>>56), byte(arg>>48), byte(arg>>40), byte(arg>>32),
			byte(arg>>24), byte(arg>>16), byte(arg>>8), byte(arg))
	}
}

// appendUint writes an unsigned integer.
func appendUint(dst []byte, v uint64) []byte {
	return appendHead(dst, majorUint, v)
}

// appendByteString writes a definite-length byte string.
func appendByteString(dst, b []byte) []byte {
	dst = appendHead(dst, majorByteStr, uint64(len(b)))
	return append(dst, b...)
}

// appendTextString writes a definite-length text string.
func appendTextString(dst []byte, s string) []byte {
	dst = appendHead(dst, majorTextStr, uint64(len(s)))
	return append(dst, s...)
}

// appendArrayHeader opens a definite-length array of n items.
func appendArrayHeader(dst []byte, n uint64) []byte {
	return appendHead(dst, majorArray, n)
}

// appendIndefiniteArrayHeader opens an array that runs until a break. A bundle
// itself is one of these (RFC 9171 clause 4.1).
func appendIndefiniteArrayHeader(dst []byte) []byte {
	return append(dst, majorArray<<5|aiIndefinite)
}

// Simple values for the two booleans (RFC 8949 clause 3.3). They are major
// type 7 with an additional-information value of their own, so they need no
// argument.
const (
	simpleFalse = 20
	simpleTrue  = 21
)

// appendBool writes a boolean. Status report items are the only place a bundle
// has one (RFC 9171 clause 6.1.1).
func appendBool(dst []byte, v bool) []byte {
	if v {
		return append(dst, majorSimple<<5|simpleTrue)
	}
	return append(dst, majorSimple<<5|simpleFalse)
}

// appendBreak writes the break stop code that closes an indefinite-length item.
func appendBreak(dst []byte) []byte {
	return append(dst, breakStop)
}

// decoder walks a CBOR byte sequence. The zero value is not usable; build one
// with newDecoder. It never panics: every read bounds-checks first.
type decoder struct {
	buf []byte
	pos int
}

func newDecoder(b []byte) *decoder { return &decoder{buf: b} }

// atEnd reports whether every octet has been consumed.
func (d *decoder) atEnd() bool { return d.pos >= len(d.buf) }

// peek returns the next head byte without consuming it.
func (d *decoder) peek() (byte, error) {
	if d.atEnd() {
		return 0, ErrTruncated
	}
	return d.buf[d.pos], nil
}

// head reads one item head and returns its major type and argument. An
// indefinite-length head reports indefinite and an argument of zero.
func (d *decoder) head() (major byte, arg uint64, indefinite bool, err error) {
	if d.atEnd() {
		return 0, 0, false, ErrTruncated
	}
	b := d.buf[d.pos]
	d.pos++
	major = b >> 5
	ai := b & 0x1F

	switch {
	case ai <= aiMaxInline:
		return major, uint64(ai), false, nil
	case ai == aiIndefinite:
		// Legal for strings, arrays and maps, and for the break code itself.
		if major == majorUint || major == majorNegInt || major == majorTag {
			return 0, 0, false, ErrInvalidCBOR
		}
		return major, 0, true, nil
	case ai >= 28 && ai <= 30:
		// RFC 8949 clause 3 reserves these.
		return 0, 0, false, ErrInvalidCBOR
	}

	width := 1 << (ai - aiOneByte) // 24->1, 25->2, 26->4, 27->8
	if d.pos+width > len(d.buf) {
		return 0, 0, false, ErrTruncated
	}
	for i := 0; i < width; i++ {
		arg = arg<<8 | uint64(d.buf[d.pos+i])
	}
	d.pos += width

	// Deterministic encoding: the argument must not fit in a shorter head.
	if !shortestHead(arg, width) {
		return 0, 0, false, ErrNotDeterministic
	}
	return major, arg, false, nil
}

// shortestHead reports whether arg genuinely needs an argument this wide.
func shortestHead(arg uint64, width int) bool {
	switch width {
	case 1:
		return arg > aiMaxInline
	case 2:
		return arg > 0xFF
	case 4:
		return arg > 0xFFFF
	case 8:
		return arg > 0xFFFFFFFF
	}
	return false
}

// uint reads an unsigned integer item.
func (d *decoder) uint() (uint64, error) {
	major, arg, indefinite, err := d.head()
	if err != nil {
		return 0, err
	}
	if major != majorUint || indefinite {
		return 0, ErrWrongCBORType
	}
	return arg, nil
}

// byteString reads a definite-length byte string. The result aliases the input
// buffer; callers that keep it past the life of the input must copy.
func (d *decoder) byteString() ([]byte, error) {
	major, arg, indefinite, err := d.head()
	if err != nil {
		return nil, err
	}
	if major != majorByteStr {
		return nil, ErrWrongCBORType
	}
	if indefinite {
		// RFC 9171 clause 4.3.2 requires block-type-specific data to be a
		// definite-length byte string, and nothing else in a bundle is a byte
		// string at all.
		return nil, ErrIndefiniteByteString
	}
	if arg > uint64(len(d.buf)-d.pos) {
		return nil, ErrTruncated
	}
	start := d.pos
	d.pos += int(arg)
	return d.buf[start:d.pos], nil
}

// textString reads a definite-length text string.
func (d *decoder) textString() (string, error) {
	major, arg, indefinite, err := d.head()
	if err != nil {
		return "", err
	}
	if major != majorTextStr || indefinite {
		return "", ErrWrongCBORType
	}
	if arg > uint64(len(d.buf)-d.pos) {
		return "", ErrTruncated
	}
	start := d.pos
	d.pos += int(arg)
	return string(d.buf[start:d.pos]), nil
}

// boolean reads a boolean.
func (d *decoder) boolean() (bool, error) {
	major, arg, indefinite, err := d.head()
	if err != nil {
		return false, err
	}
	if major != majorSimple || indefinite {
		return false, ErrWrongCBORType
	}
	switch arg {
	case simpleTrue:
		return true, nil
	case simpleFalse:
		return false, nil
	}
	return false, ErrWrongCBORType
}

// arrayHeader reads an array head. For a definite-length array n is the item
// count; for an indefinite one the caller reads items until readBreak succeeds.
func (d *decoder) arrayHeader() (n uint64, indefinite bool, err error) {
	major, arg, indef, err := d.head()
	if err != nil {
		return 0, false, err
	}
	if major != majorArray {
		return 0, false, ErrWrongCBORType
	}
	return arg, indef, nil
}

// atBreak reports whether the next octet is the break stop code.
func (d *decoder) atBreak() bool {
	b, err := d.peek()
	return err == nil && b == breakStop
}

// readBreak consumes a break stop code.
func (d *decoder) readBreak() error {
	b, err := d.peek()
	if err != nil {
		return err
	}
	if b != breakStop {
		return ErrExpectedBreak
	}
	d.pos++
	return nil
}
