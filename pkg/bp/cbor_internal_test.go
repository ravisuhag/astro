package bp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

// The encoder vectors below are transcribed from RFC 8949 appendix A, the
// table of worked CBOR examples. They pin the shortest-head rule that
// RFC 9171 clause 4.1 requires: 24 is 0x1818, never 0x190018.
func TestCBORAppendUint(t *testing.T) {
	tests := []struct {
		value uint64
		want  string
	}{
		{0, "00"},
		{1, "01"},
		{10, "0a"},
		{23, "17"}, // the last value a head byte carries alone
		{24, "1818"},
		{25, "1819"},
		{100, "1864"},
		{1000, "1903e8"},
		{1000000, "1a000f4240"},
		{1000000000000, "1b000000e8d4a51000"},
		{18446744073709551615, "1bffffffffffffffff"},
	}

	for _, tt := range tests {
		got := hex.EncodeToString(appendUint(nil, tt.value))
		if got != tt.want {
			t.Errorf("appendUint(%d) = %s, want %s", tt.value, got, tt.want)
		}

		// And it reads back as itself.
		v, err := newDecoder(mustHex(t, tt.want)).uint()
		if err != nil {
			t.Errorf("decoding %s: %v", tt.want, err)
			continue
		}
		if v != tt.value {
			t.Errorf("decode(%s) = %d, want %d", tt.want, v, tt.value)
		}
	}
}

func TestCBORAppendStringsAndArrays(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"empty byte string", hex.EncodeToString(appendByteString(nil, nil)), "40"},
		{"byte string 01020304", hex.EncodeToString(appendByteString(nil, []byte{1, 2, 3, 4})), "4401020304"},
		{"empty text string", hex.EncodeToString(appendTextString(nil, "")), "60"},
		{"text string a", hex.EncodeToString(appendTextString(nil, "a")), "6161"},
		{"text string IETF", hex.EncodeToString(appendTextString(nil, "IETF")), "6449455446"},
		{"empty array", hex.EncodeToString(appendArrayHeader(nil, 0)), "80"},
		{"array of 3", hex.EncodeToString(appendArrayHeader(nil, 3)), "83"},
		{"indefinite array", hex.EncodeToString(appendIndefiniteArrayHeader(nil)), "9f"},
		{"break", hex.EncodeToString(appendBreak(nil)), "ff"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, tt.got, tt.want)
		}
	}
}

// RFC 8949 appendix A gives [1, 2, 3] as 0x83010203 and the indefinite-length
// form [_ 1, 2, 3] as 0x9f010203ff. A bundle is the indefinite kind
// (RFC 9171 clause 4.1), so both paths matter.
func TestCBORArrayForms(t *testing.T) {
	d := newDecoder(mustHex(t, "83010203"))
	n, indefinite, err := d.arrayHeader()
	if err != nil || indefinite || n != 3 {
		t.Fatalf("definite array header = (%d, %v, %v), want (3, false, nil)", n, indefinite, err)
	}
	for want := uint64(1); want <= 3; want++ {
		if v, err := d.uint(); err != nil || v != want {
			t.Fatalf("item %d = (%d, %v)", want, v, err)
		}
	}
	if !d.atEnd() {
		t.Error("definite array left bytes unread")
	}

	d = newDecoder(mustHex(t, "9f010203ff"))
	n, indefinite, err = d.arrayHeader()
	if err != nil || !indefinite || n != 0 {
		t.Fatalf("indefinite array header = (%d, %v, %v), want (0, true, nil)", n, indefinite, err)
	}
	count := 0
	for !d.atBreak() {
		if _, err := d.uint(); err != nil {
			t.Fatalf("item %d: %v", count, err)
		}
		count++
	}
	if count != 3 {
		t.Errorf("read %d items before the break, want 3", count)
	}
	if err := d.readBreak(); err != nil {
		t.Errorf("readBreak: %v", err)
	}
	if !d.atEnd() {
		t.Error("indefinite array left bytes unread")
	}
}

// RFC 9171 clause 4.1 requires RFC 8949 core deterministic encoding, so a
// value written wider than it needs to be is not a bundle this package will
// read. Each input below encodes a value that fits in a shorter head.
func TestCBORRejectsNonShortestForm(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"23 in a one-byte argument", "1817"},
		{"24 in a two-byte argument", "190018"},
		{"255 in a four-byte argument", "1a000000ff"},
		{"65535 in an eight-byte argument", "1b000000000000ffff"},
	}

	for _, tt := range tests {
		_, err := newDecoder(mustHex(t, tt.input)).uint()
		if !errors.Is(err, ErrNotDeterministic) {
			t.Errorf("%s: err = %v, want ErrNotDeterministic", tt.name, err)
		}
	}
}

// Every head must bounds-check before it reads. These inputs stop partway
// through an argument or a payload.
func TestCBORTruncated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		read  func(*decoder) error
	}{
		{"empty input", "", func(d *decoder) error { _, err := d.uint(); return err }},
		{"uint head with no argument", "18", func(d *decoder) error { _, err := d.uint(); return err }},
		{"uint argument cut short", "1b00000000", func(d *decoder) error { _, err := d.uint(); return err }},
		{"byte string shorter than its length", "4401", func(d *decoder) error { _, err := d.byteString(); return err }},
		{"text string shorter than its length", "6449", func(d *decoder) error { _, err := d.textString(); return err }},
		{"array head with no items", "83", func(d *decoder) error {
			if _, _, err := d.arrayHeader(); err != nil {
				return err
			}
			_, err := d.uint()
			return err
		}},
		{"missing break", "9f0102", func(d *decoder) error {
			if _, _, err := d.arrayHeader(); err != nil {
				return err
			}
			for !d.atBreak() && !d.atEnd() {
				if _, err := d.uint(); err != nil {
					return err
				}
			}
			return d.readBreak()
		}},
	}

	for _, tt := range tests {
		err := tt.read(newDecoder(mustHex(t, tt.input)))
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("%s: err = %v, want ErrTruncated", tt.name, err)
		}
	}
}

func TestCBORRejectsMalformedHeads(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{"reserved additional information 28", "1c", ErrInvalidCBOR},
		{"reserved additional information 29", "1d", ErrInvalidCBOR},
		{"reserved additional information 30", "1e", ErrInvalidCBOR},
		{"indefinite-length integer", "1f", ErrInvalidCBOR},
		{"text string where an integer belongs", "6161", ErrWrongCBORType},
		{"array where an integer belongs", "80", ErrWrongCBORType},
	}

	for _, tt := range tests {
		_, err := newDecoder(mustHex(t, tt.input)).uint()
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

// RFC 9171 clause 4.3.2 requires block-type-specific data to be a
// definite-length byte string. 0x5f opens the indefinite kind.
func TestCBORRejectsIndefiniteByteString(t *testing.T) {
	_, err := newDecoder(mustHex(t, "5f42010243030405ff")).byteString()
	if !errors.Is(err, ErrIndefiniteByteString) {
		t.Errorf("err = %v, want ErrIndefiniteByteString", err)
	}
}

func TestCBORByteStringAliasesInput(t *testing.T) {
	in := mustHex(t, "4401020304")
	got, err := newDecoder(in).byteString()
	if err != nil {
		t.Fatalf("byteString: %v", err)
	}
	if !bytes.Equal(got, []byte{1, 2, 3, 4}) {
		t.Fatalf("got % x, want 01 02 03 04", got)
	}
	// Documented behaviour: the result points into the input, so a caller that
	// keeps it must copy. Prove it rather than leave it to the comment.
	in[1] = 0xFF
	if got[0] != 0xFF {
		t.Error("byteString copied when it is documented to alias")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad test hex %q: %v", s, err)
	}
	return b
}
