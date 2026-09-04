// Package cbor is the deterministic CBOR subset the Bundle Protocol family
// needs (RFC 8949).
//
// This is not a general CBOR library and must not grow into one. It carries
// only the pieces a bundle and its security blocks need: unsigned integers,
// byte strings, text strings, definite and indefinite arrays, the break stop
// code, and the two tags the RFC 9171 appendix B grammar mentions. Every
// branch here should be traceable to something on the wire.
//
// RFC 9171 clause 4.1 requires the core deterministic encoding of RFC 8949
// clause 4.2.1, with one relaxation: indefinite-length items stay legal. The
// part that bites is the argument encoding — a value must use the shortest
// head that can hold it, so 40 is 0x18 0x28 and never 0x1A 0x00 0x00 0x00 0x28.
// The encoder here always emits the shortest head, and the reader refuses
// anything longer. That strictness is a choice: clause 4.1 lets an
// implementation accept sloppy input and repair it, and this one does not.
//
// It lives in internal because it is shared by pkg/bp and pkg/bpsec and
// carries no API commitment of its own.
package cbor

// Major types, the top three bits of a head byte (RFC 8949 clause 3.1).
const (
	MajorUint    = 0
	MajorNegInt  = 1
	MajorByteStr = 2
	MajorTextStr = 3
	MajorArray   = 4
	MajorMap     = 5
	MajorTag     = 6
	MajorSimple  = 7
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

// BreakStop closes an indefinite-length item (RFC 8949 clause 3.2.1).
const BreakStop = 0xFF

// Tags the RFC 9171 appendix B grammar allows. Neither is required, and
// nothing here ever emits one, but a reader meets them in the wild.
const (
	TagEmbeddedCBOR  = 24    // a byte string whose content is itself CBOR
	TagSelfDescribed = 55799 // the "this is CBOR" marker, RFC 8949 clause 3.4.6
)

// Simple values for the two booleans (RFC 8949 clause 3.3). They are major
// type 7 with an additional-information value of their own, so they need no
// argument.
const (
	simpleFalse = 20
	simpleTrue  = 21
)

// maxSkipDepth bounds how far Skip will descend. Nothing a bundle or a
// security block defines nests anywhere near this far; the limit exists so
// that hostile input cannot drive the walker into unbounded recursion.
const maxSkipDepth = 64

// AppendHead writes one item head: the major type and its argument, in the
// shortest form that holds the value.
func AppendHead(dst []byte, major byte, arg uint64) []byte {
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

// AppendUint writes an unsigned integer.
func AppendUint(dst []byte, v uint64) []byte {
	return AppendHead(dst, MajorUint, v)
}

// AppendByteString writes a definite-length byte string.
func AppendByteString(dst, b []byte) []byte {
	dst = AppendHead(dst, MajorByteStr, uint64(len(b)))
	return append(dst, b...)
}

// AppendTextString writes a definite-length text string.
func AppendTextString(dst []byte, s string) []byte {
	dst = AppendHead(dst, MajorTextStr, uint64(len(s)))
	return append(dst, s...)
}

// AppendArrayHeader opens a definite-length array of n items.
func AppendArrayHeader(dst []byte, n uint64) []byte {
	return AppendHead(dst, MajorArray, n)
}

// AppendIndefiniteArrayHeader opens an array that runs until a break. A bundle
// itself is one of these (RFC 9171 clause 4.1).
func AppendIndefiniteArrayHeader(dst []byte) []byte {
	return append(dst, MajorArray<<5|aiIndefinite)
}

// AppendBool writes a boolean. Status report items are the only place a bundle
// has one (RFC 9171 clause 6.1.1).
func AppendBool(dst []byte, v bool) []byte {
	if v {
		return append(dst, MajorSimple<<5|simpleTrue)
	}
	return append(dst, MajorSimple<<5|simpleFalse)
}

// AppendBreak writes the break stop code that closes an indefinite-length item.
func AppendBreak(dst []byte) []byte {
	return append(dst, BreakStop)
}

// Decoder walks a CBOR byte sequence. The zero value is not usable; build one
// with NewDecoder. It never panics: every read bounds-checks first.
type Decoder struct {
	buf []byte
	pos int
}

// NewDecoder returns a Decoder reading b. The slices it hands back alias b,
// so a caller keeping one past the life of b must copy it.
func NewDecoder(b []byte) *Decoder { return &Decoder{buf: b} }

// Offset reports how many octets have been consumed. Paired with Skip it gives
// the extent of an item a caller wants to hand to another decoder whole.
func (d *Decoder) Offset() int { return d.pos }

// Slice returns buf[from:to] from the input. It is meant for pulling out the
// extent Offset and Skip identified, and returns nil for a range outside the
// input rather than panicking.
func (d *Decoder) Slice(from, to int) []byte {
	if from < 0 || to > len(d.buf) || from > to {
		return nil
	}
	return d.buf[from:to]
}

// Remaining reports how many octets are left unread. A decoder can use it to
// bound a length field before allocating: no item is shorter than one octet,
// so a claimed count larger than this cannot be honest.
func (d *Decoder) Remaining() int { return len(d.buf) - d.pos }

// AtEnd reports whether every octet has been consumed.
func (d *Decoder) AtEnd() bool { return d.pos >= len(d.buf) }

// Peek returns the next head byte without consuming it.
func (d *Decoder) Peek() (byte, error) {
	if d.AtEnd() {
		return 0, ErrTruncated
	}
	return d.buf[d.pos], nil
}

// Head reads one item head and returns its major type and argument. An
// indefinite-length head reports indefinite and an argument of zero.
func (d *Decoder) Head() (major byte, arg uint64, indefinite bool, err error) {
	if d.AtEnd() {
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
		if major == MajorUint || major == MajorNegInt || major == MajorTag {
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

// Uint reads an unsigned integer item.
func (d *Decoder) Uint() (uint64, error) {
	major, arg, indefinite, err := d.Head()
	if err != nil {
		return 0, err
	}
	if major != MajorUint || indefinite {
		return 0, ErrWrongCBORType
	}
	return arg, nil
}

// ByteString reads a definite-length byte string. The result aliases the input
// buffer; callers that keep it past the life of the input must copy.
func (d *Decoder) ByteString() ([]byte, error) {
	major, arg, indefinite, err := d.Head()
	if err != nil {
		return nil, err
	}
	if major != MajorByteStr {
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

// TextString reads a definite-length text string.
func (d *Decoder) TextString() (string, error) {
	major, arg, indefinite, err := d.Head()
	if err != nil {
		return "", err
	}
	if major != MajorTextStr || indefinite {
		return "", ErrWrongCBORType
	}
	if arg > uint64(len(d.buf)-d.pos) {
		return "", ErrTruncated
	}
	start := d.pos
	d.pos += int(arg)
	return string(d.buf[start:d.pos]), nil
}

// Boolean reads a boolean.
func (d *Decoder) Boolean() (bool, error) {
	major, arg, indefinite, err := d.Head()
	if err != nil {
		return false, err
	}
	if major != MajorSimple || indefinite {
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

// ArrayHeader reads an array head. For a definite-length array n is the item
// count; for an indefinite one the caller reads items until ReadBreak succeeds.
func (d *Decoder) ArrayHeader() (n uint64, indefinite bool, err error) {
	major, arg, indef, err := d.Head()
	if err != nil {
		return 0, false, err
	}
	if major != MajorArray {
		return 0, false, ErrWrongCBORType
	}
	return arg, indef, nil
}

// AtBreak reports whether the next octet is the break stop code.
func (d *Decoder) AtBreak() bool {
	b, err := d.Peek()
	return err == nil && b == BreakStop
}

// ReadBreak consumes a break stop code.
func (d *Decoder) ReadBreak() error {
	b, err := d.Peek()
	if err != nil {
		return err
	}
	if b != BreakStop {
		return ErrExpectedBreak
	}
	d.pos++
	return nil
}

// Skip consumes exactly one item, however deeply nested, and returns the raw
// octets it covered. It exists so that a caller can lift a whole sub-item out
// of a stream and hand it to a decoder that knows the shape — pkg/bpsec reads
// the security source that way, leaving the endpoint ID rules with pkg/bp
// where they belong.
//
// The returned slice aliases the input.
func (d *Decoder) Skip() ([]byte, error) {
	start := d.pos
	if err := d.skipOne(0); err != nil {
		return nil, err
	}
	return d.buf[start:d.pos], nil
}

// skipOne consumes one item at the given nesting depth.
func (d *Decoder) skipOne(depth int) error {
	if depth > maxSkipDepth {
		return ErrNestingTooDeep
	}

	// A break where an item was expected is the caller's to handle, not ours.
	if d.AtBreak() {
		return ErrWrongCBORType
	}

	major, arg, indefinite, err := d.Head()
	if err != nil {
		return err
	}

	switch major {
	case MajorUint, MajorNegInt, MajorSimple:
		// Head carried the whole item. Major type 7 also covers the float
		// widths, whose payload the head's argument already consumed.
		return nil

	case MajorByteStr, MajorTextStr:
		if indefinite {
			// An indefinite string is a run of definite chunks of the same
			// major type, closed by a break (RFC 8949 clause 3.2.3).
			for !d.AtBreak() {
				chunkMajor, chunkArg, chunkIndef, err := d.Head()
				if err != nil {
					return err
				}
				if chunkMajor != major || chunkIndef {
					return ErrInvalidCBOR
				}
				if err := d.advance(chunkArg); err != nil {
					return err
				}
			}
			return d.ReadBreak()
		}
		return d.advance(arg)

	case MajorArray, MajorMap:
		items := arg
		if major == MajorMap {
			// A map's argument counts pairs, so it holds twice as many items.
			if items > ^uint64(0)/2 {
				return ErrTruncated
			}
			items *= 2
		}
		if indefinite {
			for !d.AtBreak() {
				if err := d.skipOne(depth + 1); err != nil {
					return err
				}
			}
			return d.ReadBreak()
		}
		// Guard before looping: a huge count on a short input would otherwise
		// spin until the reader ran off the end one item at a time.
		if items > uint64(len(d.buf)-d.pos) {
			return ErrTruncated
		}
		for i := uint64(0); i < items; i++ {
			if err := d.skipOne(depth + 1); err != nil {
				return err
			}
		}
		return nil

	case MajorTag:
		// A tag is a head followed by exactly one tagged item.
		return d.skipOne(depth + 1)
	}

	return ErrInvalidCBOR
}

// advance consumes n octets of payload.
func (d *Decoder) advance(n uint64) error {
	if n > uint64(len(d.buf)-d.pos) {
		return ErrTruncated
	}
	d.pos += int(n)
	return nil
}
