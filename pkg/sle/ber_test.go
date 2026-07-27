package sle_test

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"math"
	"testing"

	"github.com/ravisuhag/astro/pkg/sle"
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
		got := sle.AppendInteger(nil, v)

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
		encoded := sle.AppendInteger(nil, v)

		e, err := sle.NewDecoder(encoded).Next()
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
	if got := sle.AppendOctetString(nil, payload); !bytes.Equal(got, want) {
		t.Errorf("AppendOctetString = % X, want % X", got, want)
	}

	// VisibleString. encoding/asn1 does not have a dedicated type, so check
	// the tag and content directly against X.690.
	encoded := sle.AppendVisibleString(nil, "CCSDS")
	if encoded[0] != 0x1A {
		t.Errorf("VisibleString tag = %#02x, want 0x1A", encoded[0])
	}
	e, err := sle.NewDecoder(encoded).Next()
	if err != nil {
		t.Fatal(err)
	}
	if e.String() != "CCSDS" {
		t.Errorf("string = %q, want CCSDS", e.String())
	}
}

func TestLongFormLength(t *testing.T) {
	// X.690 §8.1.3: values of 128 octets or more use the long form.
	for _, size := range []int{127, 128, 255, 256, 65535, 65536} {
		content := make([]byte, size)
		encoded := sle.AppendOctetString(nil, content)

		e, err := sle.NewDecoder(encoded).Next()
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
	// the 30 the low-tag form can hold (X.690 §8.1.2.4).
	for _, tag := range []uint32{0, 1, 30, 31, 100, 127, 128, 1000, 100000} {
		encoded := sle.AppendElement(nil, sle.ClassContext, true, tag, []byte{0xAA})

		e, err := sle.NewDecoder(encoded).Next()
		if err != nil {
			t.Fatalf("tag %d: %v", tag, err)
		}
		if e.Tag != tag {
			t.Errorf("tag = %d, want %d", e.Tag, tag)
		}
		if e.Class != sle.ClassContext {
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
	content = sle.AppendInteger(content, 42)
	content = sle.AppendOctetString(content, []byte("payload"))
	encoded := sle.AppendSequence(nil, content)

	d := sle.NewDecoder(encoded)
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
	encoded := sle.AppendNull(nil)
	if !bytes.Equal(encoded, []byte{0x05, 0x00}) {
		t.Errorf("NULL = % X, want 05 00", encoded)
	}
}

func TestDecoderRejectsIndefiniteLength(t *testing.T) {
	// X.690 §8.1.3.6. Real providers emit it; guessing where the value ends
	// is worse than refusing.
	data := []byte{0x30, 0x80, 0x02, 0x01, 0x2A, 0x00, 0x00}
	if _, err := sle.NewDecoder(data).Next(); !errors.Is(err, sle.ErrIndefiniteLength) {
		t.Errorf("error = %v, want ErrIndefiniteLength", err)
	}
}

func TestDecoderRejectsOversizedLength(t *testing.T) {
	// A length field can name far more than any real PDU holds.
	data := []byte{0x04, 0x84, 0x7F, 0xFF, 0xFF, 0xFF}
	if _, err := sle.NewDecoderWithLimit(data, 1024).Next(); !errors.Is(err, sle.ErrLengthTooLarge) {
		t.Errorf("error = %v, want ErrLengthTooLarge", err)
	}
}

func TestDecoderRejectsTruncatedInput(t *testing.T) {
	full := sle.AppendOctetString(nil, []byte("some content here"))
	for cut := 0; cut < len(full); cut++ {
		if _, err := sle.NewDecoder(full[:cut]).Next(); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}

func TestDecoderRejectsReservedLengthOctet(t *testing.T) {
	// X.690 §8.1.3.5 c) reserves 0xFF.
	if _, err := sle.NewDecoder([]byte{0x04, 0xFF, 0x00}).Next(); !errors.Is(err, sle.ErrInvalidLength) {
		t.Errorf("error = %v, want ErrInvalidLength", err)
	}
}

func TestIntegerOverflowRejected(t *testing.T) {
	// Nine content octets cannot be an int64.
	data := []byte{0x02, 0x09, 0x01, 0, 0, 0, 0, 0, 0, 0, 0}
	e, err := sle.NewDecoder(data).Next()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Int64(); !errors.Is(err, sle.ErrIntegerOverflow) {
		t.Errorf("error = %v, want ErrIntegerOverflow", err)
	}
}

func TestUint64HandlesFullRange(t *testing.T) {
	// A positive value needing all 64 bits carries a leading zero octet, so
	// its content is nine octets and Int64 cannot hold it.
	data := []byte{0x02, 0x09, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	e, err := sle.NewDecoder(data).Next()
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
	encoded := sle.AppendOctetString(nil, []byte("original"))
	e, err := sle.NewDecoder(encoded).Next()
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
