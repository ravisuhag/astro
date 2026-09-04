package cbor

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

// The inputs below are transcribed from RFC 8949 appendix A, the table of
// worked CBOR examples, so what Skip must consume is fixed by the document
// rather than by this package's own encoder.
func TestSkipConsumesExactlyOneItem(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unsigned inline", "17"},
		{"unsigned one-byte argument", "1818"},
		{"unsigned eight-byte argument", "1b000000e8d4a51000"},
		{"negative integer", "3903e7"},
		{"empty byte string", "40"},
		{"byte string", "4401020304"},
		{"text string", "6449455446"},
		{"empty array", "80"},
		{"flat array", "83010203"},
		{"nested arrays", "8301820203820405"},
		{"array of 25 items", "98190102030405060708090a0b0c0d0e0f101112131415161718181819"},
		{"empty map", "a0"},
		{"map", "a201020304"},
		{"map with array values", "a26161016162820203"},
		{"indefinite array", "9f018202039f0405ffff"},
		{"indefinite map", "bf61610161629f0203ffff"},
		{"indefinite byte string", "5f42010243030405ff"},
		{"indefinite text string", "7f657374726561646d696e67ff"},
		{"tagged item", "c11a514b67b0"},
		{"false", "f4"},
		{"true", "f5"},
		{"null", "f6"},
		{"half float", "f93c00"},
		{"double float", "fbc010666666666666"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A trailing sentinel proves Skip stopped where the item did
			// rather than swallowing whatever came next.
			in := mustHex(t, tt.input+"01")
			d := NewDecoder(in)

			got, err := d.Skip()
			if err != nil {
				t.Fatalf("Skip: %v", err)
			}
			want := mustHex(t, tt.input)
			if !bytes.Equal(got, want) {
				t.Errorf("Skip returned %x, want %x", got, want)
			}

			v, err := d.Uint()
			if err != nil {
				t.Fatalf("reading the sentinel after Skip: %v", err)
			}
			if v != 1 {
				t.Errorf("sentinel after Skip is %d, want 1", v)
			}
			if !d.AtEnd() {
				t.Errorf("decoder is not at the end after the sentinel")
			}
		})
	}
}

// Skip aliases its input, the way every other reader here does.
func TestSkipAliasesInput(t *testing.T) {
	in := mustHex(t, "4401020304")
	d := NewDecoder(in)

	got, err := d.Skip()
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	in[1] = 0xFF
	if got[1] != 0xFF {
		t.Errorf("Skip copied its input; it is documented to alias it")
	}
}

func TestSkipRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{"empty input", "", ErrTruncated},
		{"byte string past the end", "44010203", ErrTruncated},
		{"array short one item", "8301", ErrTruncated},
		{"array head claims more items than octets remain", "9bffffffffffffffff", ErrTruncated},
		{"map head claims more pairs than octets remain", "bbffffffffffffff00", ErrTruncated},
		{"indefinite array with no break", "9f0102", ErrTruncated},
		{"tag with nothing tagged", "c1", ErrTruncated},
		{"reserved additional information", "1c", ErrInvalidCBOR},
		{"indefinite-length unsigned integer", "1f", ErrInvalidCBOR},
		{"indefinite string chunk of the wrong type", "5f6161ff", ErrInvalidCBOR},
		{"break where an item was expected", "ff", ErrWrongCBORType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDecoder(mustHex(t, tt.input)).Skip()
			if !errors.Is(err, tt.want) {
				t.Errorf("Skip(%q) = %v, want %v", tt.input, err, tt.want)
			}
		})
	}
}

// Nesting is bounded so that hostile input cannot drive the walker into
// unbounded recursion. An array head is one octet, so a run of them is the
// cheapest way to nest deeply.
func TestSkipRejectsDeepNesting(t *testing.T) {
	deep := bytes.Repeat([]byte{0x81}, maxSkipDepth+2)
	deep = append(deep, 0x01)

	if _, err := NewDecoder(deep).Skip(); !errors.Is(err, ErrNestingTooDeep) {
		t.Errorf("Skip on %d nested arrays = %v, want ErrNestingTooDeep", maxSkipDepth+2, err)
	}

	// One below the limit still reads, so the bound is not accidentally tight.
	shallow := bytes.Repeat([]byte{0x81}, maxSkipDepth-1)
	shallow = append(shallow, 0x01)
	if _, err := NewDecoder(shallow).Skip(); err != nil {
		t.Errorf("Skip on %d nested arrays = %v, want success", maxSkipDepth-1, err)
	}
}

func FuzzSkip(f *testing.F) {
	for _, seed := range []string{
		"17", "8301820203820405", "9f018202039f0405ffff", "bf61610161629f0203ffff",
		"5f42010243030405ff", "c11a514b67b0", "fbc010666666666666",
	} {
		b, err := hex.DecodeString(seed)
		if err != nil {
			f.Fatalf("bad seed hex %q: %v", seed, err)
		}
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		d := NewDecoder(data)
		got, err := d.Skip()
		if err != nil {
			return
		}
		// A successful Skip reports exactly the octets it consumed, and
		// consumes at least one.
		if len(got) == 0 {
			t.Fatalf("Skip succeeded without consuming anything")
		}
		if d.Offset() != len(got) {
			t.Fatalf("Skip consumed %d octets but returned %d", d.Offset(), len(got))
		}
		if !bytes.Equal(got, data[:d.Offset()]) {
			t.Fatalf("Skip returned octets it did not consume")
		}
	})
}
