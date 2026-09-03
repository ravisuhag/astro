package sdnv_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

func TestContinuationBits(t *testing.T) {
	// Every octet but the last must have its top bit set.
	encoded := sdnv.Encode(0x4234)
	for i, b := range encoded {
		last := i == len(encoded)-1
		if last && b&0x80 != 0 {
			t.Errorf("octet %d is last but has the continuation bit set", i)
		}
		if !last && b&0x80 == 0 {
			t.Errorf("octet %d is not last but has no continuation bit", i)
		}
	}
}

func TestRoundTripBoundaries(t *testing.T) {
	values := []uint64{
		0, 1, 126, 127, 128, 129,
		16383, 16384,
		1 << 20, 1 << 21,
		math.MaxUint32,
		1 << 56,
		math.MaxUint64,
	}
	for _, v := range values {
		encoded := sdnv.Encode(v)
		if len(encoded) > sdnv.MaxEncodedSize {
			t.Errorf("Encode(%d) produced %d octets, over the %d maximum", v, len(encoded), sdnv.MaxEncodedSize)
		}
		got, n, err := sdnv.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode of %d: %v", v, err)
		}
		if got != v {
			t.Errorf("round trip of %d gave %d", v, got)
		}
		if n != len(encoded) {
			t.Errorf("round trip of %d consumed %d, want %d", v, n, len(encoded))
		}
	}
}

func TestDecodeRejectsTruncatedValue(t *testing.T) {
	// Every octet has the continuation bit set, so the value never ends.
	for _, data := range [][]byte{{0x80}, {0x81, 0x80}, {0xFF, 0xFF, 0xFF}} {
		if _, _, err := sdnv.Decode(data); !errors.Is(err, sdnv.ErrDataTooShort) {
			t.Errorf("Decode(%x): error = %v, want ErrDataTooShort", data, err)
		}
	}
}

func TestDecodeRejectsEmptyInput(t *testing.T) {
	if _, _, err := sdnv.Decode(nil); !errors.Is(err, sdnv.ErrDataTooShort) {
		t.Errorf("error = %v, want ErrDataTooShort", err)
	}
}

func TestDecodeRejectsOverflow(t *testing.T) {
	// Eleven continuation octets cannot describe a 64-bit value.
	tooLong := bytes.Repeat([]byte{0xFF}, 11)
	tooLong = append(tooLong, 0x00)
	if _, _, err := sdnv.Decode(tooLong); !errors.Is(err, sdnv.ErrOverflow) {
		t.Errorf("error = %v, want ErrOverflow", err)
	}

	// A ten-octet value whose high bits exceed 64 must also be refused.
	huge := []byte{0x82, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00}
	if _, _, err := sdnv.Decode(huge); !errors.Is(err, sdnv.ErrOverflow) {
		t.Errorf("error = %v, want ErrOverflow for a value past 64 bits", err)
	}
}

func TestDecodeRejectsOverlongPaddedEncoding(t *testing.T) {
	// RFC 6256 clause 3.2: leading 0x80 octets pad a value without changing it.
	// Eleven octets can still describe a small value, but this package caps
	// what it reads at ten and refuses with ErrTooLong, not ErrOverflow.
	padded := append(bytes.Repeat([]byte{0x80}, 10), 0x01)
	if _, _, err := sdnv.Decode(padded); !errors.Is(err, sdnv.ErrTooLong) {
		t.Errorf("Decode: error = %v, want ErrTooLong", err)
	}
	if _, err := sdnv.DecodeFrom(bytes.NewReader(padded)); !errors.Is(err, sdnv.ErrTooLong) {
		t.Errorf("DecodeFrom: error = %v, want ErrTooLong", err)
	}

	// A ten-octet padded encoding is within the cap and decodes.
	ok := append(bytes.Repeat([]byte{0x80}, 9), 0x01)
	v, n, err := sdnv.Decode(ok)
	if err != nil || v != 1 || n != 10 {
		t.Errorf("Decode(10-octet padded 1) = %d, %d, %v; want 1, 10, nil", v, n, err)
	}
}

func TestAppendEncodeMatchesEncode(t *testing.T) {
	dst := []byte{0xAA, 0xBB}
	dst = sdnv.AppendEncode(dst, 0x4234)
	want := append([]byte{0xAA, 0xBB}, sdnv.Encode(0x4234)...)
	if !bytes.Equal(dst, want) {
		t.Errorf("AppendEncode = %x, want %x", dst, want)
	}
}

func TestDecodeN(t *testing.T) {
	var data []byte
	want := []uint64{1, 300, 70000}
	for _, v := range want {
		data = sdnv.AppendEncode(data, v)
	}

	got, consumed, err := sdnv.DecodeN(data, len(want))
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(data) {
		t.Errorf("consumed %d, want %d", consumed, len(data))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("value %d = %d, want %d", i, got[i], want[i])
		}
	}

	// Asking for more values than the data holds must fail.
	if _, _, err := sdnv.DecodeN(data, len(want)+1); err == nil {
		t.Error("expected an error when asking for more values than are present")
	}
}

func TestDecodeFrom(t *testing.T) {
	data := sdnv.Encode(0x4234)
	r := bytes.NewReader(data)

	v, err := sdnv.DecodeFrom(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0x4234 {
		t.Errorf("value = %#x, want 0x4234", v)
	}
	if r.Len() != 0 {
		t.Errorf("%d octets left unread; DecodeFrom should stop at the value's end", r.Len())
	}
}

func TestDecodeFromTruncated(t *testing.T) {
	r := bytes.NewReader([]byte{0x81})
	if _, err := sdnv.DecodeFrom(r); !errors.Is(err, sdnv.ErrDataTooShort) {
		t.Errorf("error = %v, want ErrDataTooShort", err)
	}
}
