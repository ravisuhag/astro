package ber_test

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"math"
	"testing"

	"github.com/ravisuhag/astro/internal/ber"
)

func TestIntegerEncodingMatchesStdlibDER(t *testing.T) {
	// For plain universal INTEGERs, BER and DER agree. encoding/asn1 is a
	// trustworthy oracle for exactly this subset, which pins the minimal
	// two's complement encoding without hand-writing every vector.
	values := []int64{
		0, 1, -1, 127, 128, -128, -129, 255, 256, -256,
		32767, 32768, -32768, -32769,
		math.MaxInt32, math.MinInt32,
		math.MaxInt64, math.MinInt64,
	}

	for _, v := range values {
		want, err := asn1.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		got := ber.AppendInteger(nil, v)

		if !bytes.Equal(got, want) {
			t.Errorf("AppendInteger(%d) = % X, want % X", v, got, want)
		}
	}
}

func TestIntegerRoundTrip(t *testing.T) {
	values := []int64{
		0, 1, -1, 127, 128, -128, 255, 65535, -65536,
		math.MaxInt64, math.MinInt64,
	}
	for _, v := range values {
		encoded := ber.AppendInteger(nil, v)

		e, err := ber.NewDecoder(encoded).Next()
		if err != nil {
			t.Fatalf("decoding %d: %v", v, err)
		}
		if !e.IsUniversal(2) {
			t.Errorf("%d: tag = %d class %#x, want universal INTEGER", v, e.Tag, e.Class)
		}
		got, err := e.Int64()
		if err != nil {
			t.Fatalf("%d: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip of %d gave %d", v, got)
		}
	}
}

func TestOctetStringAndVisibleStringMatchStdlib(t *testing.T) {
	// OCTET STRING.
	payload := []byte{0x01, 0x02, 0xFF}
	want, err := asn1.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := ber.AppendOctetString(nil, payload); !bytes.Equal(got, want) {
		t.Errorf("AppendOctetString = % X, want % X", got, want)
	}

	// VisibleString. encoding/asn1 does not have a dedicated type, so check
	// the tag and content directly against X.690.
	encoded := ber.AppendVisibleString(nil, "CCSDS")
	if encoded[0] != 0x1A {
		t.Errorf("VisibleString tag = %#02x, want 0x1A", encoded[0])
	}
	e, err := ber.NewDecoder(encoded).Next()
	if err != nil {
		t.Fatal(err)
	}
	if e.String() != "CCSDS" {
		t.Errorf("string = %q, want CCSDS", e.String())
	}
}

func TestLongFormLength(t *testing.T) {
	// X.690 clause 8.1.3: values of 128 octets or more use the long form.
	for _, size := range []int{127, 128, 255, 256, 65535, 65536} {
		content := make([]byte, size)
		encoded := ber.AppendOctetString(nil, content)

		e, err := ber.NewDecoder(encoded).Next()
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if len(e.Bytes) != size {
			t.Errorf("size %d: decoded %d octets", size, len(e.Bytes))
		}

		// The short form covers exactly 0 to 127.
		lengthOctet := encoded[1]
		if size < 128 && lengthOctet&0x80 != 0 {
			t.Errorf("size %d used the long form", size)
		}
		if size >= 128 && lengthOctet&0x80 == 0 {
			t.Errorf("size %d used the short form", size)
		}
	}
}

func TestHighTagNumberForm(t *testing.T) {
	// SLE uses context tags like [100] for rafBindInvocation, which is past
	// the 30 the low-tag form can hold (X.690 clause 8.1.2.4).
	for _, tag := range []uint32{0, 1, 30, 31, 100, 127, 128, 1000, 100000} {
		encoded := ber.AppendElement(nil, ber.ClassContext, true, tag, []byte{0xAA})

		e, err := ber.NewDecoder(encoded).Next()
		if err != nil {
			t.Fatalf("tag %d: %v", tag, err)
		}
		if e.Tag != tag {
			t.Errorf("tag = %d, want %d", e.Tag, tag)
		}
		if e.Class != ber.ClassContext {
			t.Errorf("tag %d: class = %#x, want context", tag, e.Class)
		}
		if !e.Constructed {
			t.Errorf("tag %d: not marked constructed", tag)
		}

		// The low-tag form only reaches 30.
		if tag < 31 && encoded[0]&0x1F == 0x1F {
			t.Errorf("tag %d used the high-tag form unnecessarily", tag)
		}
		if tag >= 31 && encoded[0]&0x1F != 0x1F {
			t.Errorf("tag %d did not use the high-tag form", tag)
		}
	}
}

func TestSequenceNesting(t *testing.T) {
	// A SEQUENCE of an INTEGER and an OCTET STRING, decoded by nesting.
	var content []byte
	content = ber.AppendInteger(content, 42)
	content = ber.AppendOctetString(content, []byte("payload"))
	encoded := ber.AppendSequence(nil, content)

	d := ber.NewDecoder(encoded)
	seq, err := d.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !seq.IsUniversal(16) || !seq.Constructed {
		t.Fatalf("outer element is not a constructed SEQUENCE")
	}

	inner := d.Nested(seq)

	first, err := inner.Next()
	if err != nil {
		t.Fatal(err)
	}
	n, err := first.Int64()
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Errorf("first field = %d, want 42", n)
	}

	second, err := inner.Next()
	if err != nil {
		t.Fatal(err)
	}
	if second.String() != "payload" {
		t.Errorf("second field = %q, want payload", second.String())
	}
	if !inner.Empty() {
		t.Errorf("%d octets left in the sequence", inner.Remaining())
	}
}

func TestNullEncoding(t *testing.T) {
	encoded := ber.AppendNull(nil)
	if !bytes.Equal(encoded, []byte{0x05, 0x00}) {
		t.Errorf("NULL = % X, want 05 00", encoded)
	}
}

func TestDecoderAcceptsIndefiniteLength(t *testing.T) {
	// X.690 clause 8.1.3.6. Real providers emit it, so the decoder scans for the
	// end-of-contents octets rather than refusing.
	data := []byte{0x30, 0x80, 0x02, 0x01, 0x2A, 0x00, 0x00}
	seq, err := ber.NewDecoder(data).Next()
	if err != nil {
		t.Fatalf("Next() = %v", err)
	}
	if !seq.IsUniversal(16) || !seq.Constructed {
		t.Fatal("outer element is not a constructed SEQUENCE")
	}
	inner, err := ber.NewDecoder(seq.Bytes).Next()
	if err != nil {
		t.Fatalf("nested Next() = %v", err)
	}
	if v, err := inner.Int64(); err != nil || v != 42 {
		t.Errorf("nested integer = %d, %v, want 42", v, err)
	}

	// Nested indefinite lengths resolve too.
	nested := []byte{0x30, 0x80, 0x30, 0x80, 0x02, 0x01, 0x07, 0x00, 0x00, 0x00, 0x00}
	outer, err := ber.NewDecoder(nested).Next()
	if err != nil {
		t.Fatalf("nested indefinite Next() = %v", err)
	}
	if len(outer.Bytes) != 7 {
		t.Errorf("outer content = %d octets, want 7", len(outer.Bytes))
	}
}

func TestDecoderRejectsPrimitiveIndefiniteLength(t *testing.T) {
	// Clause 8.1.3.2: only a constructed encoding may use the indefinite form.
	data := []byte{0x04, 0x80, 0x00, 0x00}
	if _, err := ber.NewDecoder(data).Next(); !errors.Is(err, ber.ErrIndefiniteLength) {
		t.Errorf("error = %v, want ErrIndefiniteLength", err)
	}
}

func TestDecoderRejectsOversizedLength(t *testing.T) {
	// A length field can name far more than any real PDU holds.
	data := []byte{0x04, 0x84, 0x7F, 0xFF, 0xFF, 0xFF}
	if _, err := ber.NewDecoderWithLimit(data, 1024).Next(); !errors.Is(err, ber.ErrLengthTooLarge) {
		t.Errorf("error = %v, want ErrLengthTooLarge", err)
	}
}

func TestDecoderRejectsTruncatedInput(t *testing.T) {
	full := ber.AppendOctetString(nil, []byte("some content here"))
	for cut := 0; cut < len(full); cut++ {
		if _, err := ber.NewDecoder(full[:cut]).Next(); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}

func TestDecoderRejectsReservedLengthOctet(t *testing.T) {
	// X.690 clause 8.1.3.5 c) reserves 0xFF.
	if _, err := ber.NewDecoder([]byte{0x04, 0xFF, 0x00}).Next(); !errors.Is(err, ber.ErrInvalidLength) {
		t.Errorf("error = %v, want ErrInvalidLength", err)
	}
}

func TestIntegerOverflowRejected(t *testing.T) {
	// Nine content octets cannot be an int64.
	data := []byte{0x02, 0x09, 0x01, 0, 0, 0, 0, 0, 0, 0, 0}
	e, err := ber.NewDecoder(data).Next()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Int64(); !errors.Is(err, ber.ErrIntegerOverflow) {
		t.Errorf("error = %v, want ErrIntegerOverflow", err)
	}
}

func TestUint64HandlesFullRange(t *testing.T) {
	// A positive value needing all 64 bits carries a leading zero octet, so
	// its content is nine octets and Int64 cannot hold it.
	data := []byte{0x02, 0x09, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	e, err := ber.NewDecoder(data).Next()
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	if got != math.MaxUint64 {
		t.Errorf("value = %d, want %d", got, uint64(math.MaxUint64))
	}
}

func TestElementCopyDoesNotAlias(t *testing.T) {
	encoded := ber.AppendOctetString(nil, []byte("original"))
	e, err := ber.NewDecoder(encoded).Next()
	if err != nil {
		t.Fatal(err)
	}
	copied := e.Copy()

	for i := range encoded {
		encoded[i] = 0xFF
	}
	if string(copied) != "original" {
		t.Errorf("Copy aliased the decoder's buffer: %q", copied)
	}
}

// Copy and Raw serve two different callers and must keep disagreeing: Copy is
// for a caller that re-wraps the value with its own Append*, Raw is for a
// caller that stores the element opaquely and appends it back verbatim. A
// future simplification that merges the two would silently reintroduce the
// pkg/csts decode/encode asymmetry documented on Element.Raw.
func TestElementRawReturnsCompleteEncoding(t *testing.T) {
	// 04 01 01 — a universal OCTET STRING, content 0x01. This is the element
	// annex F3.4's ListOfParametersEvents CHOICE stands in for in the GET
	// invocation vector.
	original := ber.AppendOctetString(nil, []byte{0x01})
	e, err := ber.NewDecoder(original).Next()
	if err != nil {
		t.Fatal(err)
	}

	if raw := e.Raw(); !bytes.Equal(raw, original) {
		t.Errorf("Raw() = % X, want % X (the tag and length back)", raw, original)
	}
	if content := e.Copy(); !bytes.Equal(content, []byte{0x01}) {
		t.Errorf("Copy() = % X, want 01 (content only)", content)
	}

	// A context-specific, constructed, high-tag-number element too, so Raw
	// is checked against more than the simplest case.
	inner := ber.AppendInteger(nil, 9)
	original = ber.AppendElement(nil, ber.ClassContext, true, 40, inner)
	e, err = ber.NewDecoder(original).Next()
	if err != nil {
		t.Fatal(err)
	}
	if raw := e.Raw(); !bytes.Equal(raw, original) {
		t.Errorf("Raw() = % X, want % X", raw, original)
	}
	if content := e.Copy(); !bytes.Equal(content, inner) {
		t.Errorf("Copy() = % X, want % X (content only)", content, inner)
	}
}
