package ldc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// TestBitWriterPacksMSBFirst pins the bit order to CCSDS 121.0-B-3 clause 1.5.2:
// the first bit transmitted is the most significant.
func TestBitWriterPacksMSBFirst(t *testing.T) {
	var w ldc.BitWriter
	// 101 then 1 then 0011 makes 1011 0011 = 0xB3.
	w.WriteBits(0b101, 3)
	w.WriteBits(0b1, 1)
	w.WriteBits(0b0011, 4)

	got := w.Bytes()
	want := []byte{0xB3}
	if !bytes.Equal(got, want) {
		t.Errorf("packed % 08b, want % 08b", got, want)
	}
}

// TestBitWriterFillsWithZeros covers clause 7.2.3.2: fill bits are zeros.
func TestBitWriterFillsWithZeros(t *testing.T) {
	var w ldc.BitWriter
	w.WriteBits(0b111, 3)

	if got := w.BitLen(); got != 3 {
		t.Errorf("BitLen() = %d, want 3", got)
	}
	got := w.Bytes()
	// 111 then five zero fill bits.
	if len(got) != 1 || got[0] != 0b11100000 {
		t.Errorf("filled to %08b, want 11100000", got)
	}
}

func TestBitRoundTripEveryWidth(t *testing.T) {
	// A value that exercises every bit position.
	const pattern = uint64(0x0123456789ABCDEF)

	for n := 1; n <= 64; n++ {
		var w ldc.BitWriter
		want := pattern
		if n < 64 {
			want &= (1 << uint(n)) - 1
		}
		// Write a lead-in of an odd width so the value crosses octet
		// boundaries at a different offset each time.
		w.WriteBits(0b101, 3)
		w.WriteBits(want, n)

		r := ldc.NewBitReader(w.Bytes())
		lead, err := r.ReadBits(3)
		if err != nil {
			t.Fatalf("n=%d reading the lead-in: %v", n, err)
		}
		if lead != 0b101 {
			t.Errorf("n=%d lead-in = %03b, want 101", n, lead)
		}
		got, err := r.ReadBits(n)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if got != want {
			t.Errorf("n=%d round trip gave %#x, want %#x", n, got, want)
		}
	}
}

func TestBitReaderReportsExhaustion(t *testing.T) {
	r := ldc.NewBitReader([]byte{0xFF})

	if _, err := r.ReadBits(8); err != nil {
		t.Fatalf("reading the only octet: %v", err)
	}
	if _, err := r.ReadBits(1); !errors.Is(err, ldc.ErrDataTooShort) {
		t.Errorf("reading past the end = %v, want ErrDataTooShort", err)
	}
	// An empty reader is exhausted from the start, and does not panic.
	empty := ldc.NewBitReader(nil)
	if _, err := empty.ReadBits(1); !errors.Is(err, ldc.ErrDataTooShort) {
		t.Errorf("reading an empty stream = %v, want ErrDataTooShort", err)
	}
}

func TestBitReaderZeroWidthRead(t *testing.T) {
	r := ldc.NewBitReader(nil)
	v, err := r.ReadBits(0)
	if err != nil || v != 0 {
		t.Errorf("ReadBits(0) = %d, %v; want 0, nil", v, err)
	}
}

// TestFSCodewords pins table 3-1: a preprocessed sample of value m is m zeros
// followed by a one.
func TestFSCodewords(t *testing.T) {
	tests := []struct {
		value uint64
		bits  string
	}{
		{0, "1"},
		{1, "01"},
		{2, "001"},
		{7, "00000001"},
	}

	for _, test := range tests {
		var w ldc.BitWriter
		w.WriteZeros(test.value)
		w.WriteOne()

		if got := w.BitLen(); got != len(test.bits) {
			t.Errorf("FS(%d) is %d bits, want %d", test.value, got, len(test.bits))
		}

		r := ldc.NewBitReader(w.Bytes())
		got, err := r.ReadFS(1000)
		if err != nil {
			t.Fatalf("FS(%d): %v", test.value, err)
		}
		if got != test.value {
			t.Errorf("FS round trip gave %d, want %d", got, test.value)
		}
	}
}

// TestReadFSRefusesARunawayRun checks the limit that stops a run of zero
// octets being read as an enormous sample.
func TestReadFSRefusesARunawayRun(t *testing.T) {
	r := ldc.NewBitReader(make([]byte, 64)) // 512 zero bits, no terminator
	if _, err := r.ReadFS(100); !errors.Is(err, ldc.ErrDataTooShort) {
		t.Errorf("ReadFS over a long zero run = %v, want ErrDataTooShort", err)
	}
}

func TestBitReaderAlign(t *testing.T) {
	r := ldc.NewBitReader([]byte{0xFF, 0x0F})

	if _, err := r.ReadBits(3); err != nil {
		t.Fatal(err)
	}
	r.Align()
	if got := r.Pos(); got != 8 {
		t.Errorf("after Align, position is %d, want 8", got)
	}
	got, err := r.ReadBits(8)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x0F {
		t.Errorf("read %#x after aligning, want 0x0F", got)
	}
	// Aligning when already aligned moves nothing.
	r.Align()
	if got := r.Pos(); got != 16 {
		t.Errorf("Align on a boundary moved to %d, want 16", got)
	}
}

// TestBitWriterCrossesManyOctets checks a long write lands where a hand count
// says it should.
func TestBitWriterCrossesManyOctets(t *testing.T) {
	var w ldc.BitWriter
	w.WriteBits(1, 1)      // 1
	w.WriteZeros(20)       // twenty zeros
	w.WriteBits(0b1111, 4) // 1111
	// 1 + 20 + 4 = 25 bits, so four octets with seven fill bits.
	if got := w.BitLen(); got != 25 {
		t.Fatalf("BitLen() = %d, want 25", got)
	}

	out := w.Bytes()
	if len(out) != 4 {
		t.Fatalf("packed into %d octets, want 4", len(out))
	}
	// 1000 0000 0000 0000 0000 0111 1000 0000
	want := []byte{0x80, 0x00, 0x07, 0x80}
	if !bytes.Equal(out, want) {
		t.Errorf("packed % 08b, want % 08b", out, want)
	}
}
