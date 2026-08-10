package rhc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// encodeCount renders COUNT(a) as a bit string.
func encodeCount(t *testing.T, a int) string {
	t.Helper()
	var w rhc.BitWriter
	if err := rhc.AppendCount(&w, a); err != nil {
		t.Fatalf("AppendCount(%d) = %v", a, err)
	}
	return bitString(w.Bytes(), w.BitLen())
}

// bitString renders packed octets as a bit string.
func bitString(data []byte, bits int) string {
	out := make([]byte, 0, bits)
	for i := range bits {
		out = append(out, '0'+(data[i/8]>>(7-uint(i%8)))&1)
	}
	return string(out)
}

// TestCountTable transcribes table 5-1 of CCSDS 124.0-B-1 §5.2.2:
//
//	A = 1          '0'
//	2 <= A <= 33   '110' || BIT5(A-2)
//	A >= 34        '111' || BITE(A-2),  E = 2*floor(log2(A-2)+1) - 6
func TestCountTable(t *testing.T) {
	tests := []struct {
		a    int
		want string
	}{
		{1, "0"},
		// The short form: '110' then five bits of A-2.
		{2, "110" + "00000"},
		{3, "110" + "00001"},
		{33, "110" + "11111"},
		// The long form starts at 34, where A-2 = 32 has a minimal width of
		// six, so E = 2*6-6 = 6 and there are no leading zeros.
		{34, "111" + "100000"},
		{35, "111" + "100001"},
		{65, "111" + "111111"},
		// A-2 = 64 has a minimal width of seven, so E = 8 and one leading
		// zero appears.
		{66, "111" + "01000000"},
		// A-2 = 128, minimal width eight, E = 10, two leading zeros.
		{130, "111" + "0010000000"},
	}

	for _, test := range tests {
		if got := encodeCount(t, test.a); got != test.want {
			t.Errorf("COUNT(%d) = %s, want %s", test.a, got, test.want)
		}
	}
}

// TestCountLeadingZerosEncodeTheWidth checks the property the note under
// equation 9 relies on: the number of leading zeros in the long form grows one
// for one with the payload width, so the decoder can parse without a length
// field.
func TestCountLeadingZerosEncodeTheWidth(t *testing.T) {
	for _, a := range []int{34, 66, 130, 258, 514, 1026, 2050, 4098, 8194, 16386, 32770, 65535} {
		bits := encodeCount(t, a)
		if bits[:3] != "111" {
			t.Fatalf("COUNT(%d) is not in the long form: %s", a, bits)
		}
		payload := bits[3:]

		zeros := 0
		for zeros < len(payload) && payload[zeros] == '0' {
			zeros++
		}
		// E = 2*(zeros+6) - 6 = 2*zeros + 6.
		if want := 2*zeros + 6; len(payload) != want {
			t.Errorf("COUNT(%d) payload is %d bits with %d leading zeros, want %d",
				a, len(payload), zeros, want)
		}
	}
}

func TestCountRoundTrip(t *testing.T) {
	for a := 1; a <= rhc.MaxCount; a++ {
		var w rhc.BitWriter
		if err := rhc.AppendCount(&w, a); err != nil {
			t.Fatalf("AppendCount(%d) = %v", a, err)
		}

		r := rhc.NewBitReaderN(w.Bytes(), w.BitLen())
		got, terminator, err := rhc.ReadCount(r)
		if err != nil {
			t.Fatalf("ReadCount for %d: %v", a, err)
		}
		if terminator {
			t.Fatalf("COUNT(%d) read back as the terminator", a)
		}
		if got != a {
			t.Fatalf("COUNT round trip gave %d, want %d", got, a)
		}
		if left := r.BitsLeft(); left != 0 {
			t.Fatalf("COUNT(%d) left %d bits unread", a, left)
		}
	}
}

func TestCountRejectsOutOfRange(t *testing.T) {
	var w rhc.BitWriter
	for _, a := range []int{0, -1, rhc.MaxCount + 1} {
		if err := rhc.AppendCount(&w, a); err == nil {
			t.Errorf("AppendCount(%d) was accepted", a)
		}
	}
}

// TestTerminatorIsDistinguishable checks the prefix property the whole scheme
// rests on: '10' cannot be confused with any counter codeword.
func TestTerminatorIsDistinguishable(t *testing.T) {
	var w rhc.BitWriter
	w.WriteString("10")

	r := rhc.NewBitReaderN(w.Bytes(), w.BitLen())
	_, terminator, err := rhc.ReadCount(r)
	if err != nil {
		t.Fatalf("ReadCount = %v", err)
	}
	if !terminator {
		t.Error("'10' was not read as the terminator")
	}
}

// TestRLEFigure5_1 transcribes the worked example of figure 5-1, which walks a
// vector and names the counts it produces:
//
//	0001 000001 1 000001 00001 0000001 ... 1001 0000000000000000
//	C0=4  C1=6   C2=1  C3=6   C4=5   C5=7      C(H-1)=3
func TestRLEFigure5_1(t *testing.T) {
	// The figure's vector up to C5, then the "1001" tail and sixteen zeros.
	// The ellipsis in the figure hides an unknown middle, so this builds the
	// part that is written out and checks the counts it yields.
	v := rhc.VectorFromString("0001" + "000001" + "1" + "000001" + "00001" + "0000001")

	var w rhc.BitWriter
	if err := rhc.AppendRLE(&w, v); err != nil {
		t.Fatalf("AppendRLE() = %v", err)
	}

	// Read the counts back and compare with the figure.
	r := rhc.NewBitReaderN(w.Bytes(), w.BitLen())
	want := []int{4, 6, 1, 6, 5, 7}
	for i, expected := range want {
		got, terminator, err := rhc.ReadCount(r)
		if err != nil {
			t.Fatalf("count %d: %v", i, err)
		}
		if terminator {
			t.Fatalf("hit the terminator after %d counts, want %d", i, len(want))
		}
		if got != expected {
			t.Errorf("C%d = %d, want %d", i, got, expected)
		}
	}
	if _, terminator, err := rhc.ReadCount(r); err != nil || !terminator {
		t.Errorf("the encoding did not end with the terminator (err %v)", err)
	}
}

// TestRLETrailingZerosAreNotEncoded pins note 1 of §5.2.3: zeros after the last
// one bit are inferred from the vector length.
func TestRLETrailingZerosAreNotEncoded(t *testing.T) {
	short := rhc.VectorFromString("1")
	long := rhc.VectorFromString("1" + "0000000000000000")

	var shortW, longW rhc.BitWriter
	if err := rhc.AppendRLE(&shortW, short); err != nil {
		t.Fatal(err)
	}
	if err := rhc.AppendRLE(&longW, long); err != nil {
		t.Fatal(err)
	}

	if shortW.BitLen() != longW.BitLen() {
		t.Errorf("sixteen trailing zeros changed the encoding from %d to %d bits",
			shortW.BitLen(), longW.BitLen())
	}
}

// TestRLEOfZeroVector pins note 2 of §5.2.3: a vector with no one bits encodes
// as just the '10' terminator.
func TestRLEOfZeroVector(t *testing.T) {
	var w rhc.BitWriter
	if err := rhc.AppendRLE(&w, rhc.NewVector(64)); err != nil {
		t.Fatal(err)
	}
	if got := bitString(w.Bytes(), w.BitLen()); got != "10" {
		t.Errorf("RLE of an all-zero vector = %s, want 10", got)
	}
}

func TestRLERoundTrip(t *testing.T) {
	tests := []string{
		"0",
		"1",
		"00000000",
		"11111111",
		"10000001",
		"0001000001100000100001000000100000000000",
		"1" + "0000000000000000",
	}

	for _, bits := range tests {
		t.Run(bits, func(t *testing.T) {
			v := rhc.VectorFromString(bits)

			var w rhc.BitWriter
			if err := rhc.AppendRLE(&w, v); err != nil {
				t.Fatalf("AppendRLE() = %v", err)
			}

			r := rhc.NewBitReaderN(w.Bytes(), w.BitLen())
			back, err := rhc.ReadRLE(r, v.Len())
			if err != nil {
				t.Fatalf("ReadRLE() = %v", err)
			}
			if back.String() != v.String() {
				t.Errorf("round trip gave %s, want %s", back, v)
			}
			if left := r.BitsLeft(); left != 0 {
				t.Errorf("%d bits left unread", left)
			}
		})
	}
}

func TestRLERejectsPositionPastTheEnd(t *testing.T) {
	// A count that places a one bit beyond an eight-bit vector.
	var w rhc.BitWriter
	if err := rhc.AppendCount(&w, 100); err != nil {
		t.Fatal(err)
	}
	w.WriteString("10")

	r := rhc.NewBitReaderN(w.Bytes(), w.BitLen())
	if _, err := rhc.ReadRLE(r, 8); err == nil {
		t.Error("ReadRLE accepted a position past the end of the vector")
	}
}

func TestReadCountRejectsTruncation(t *testing.T) {
	var w rhc.BitWriter
	if err := rhc.AppendCount(&w, 1000); err != nil {
		t.Fatal(err)
	}
	full := w.BitLen()

	for n := range full {
		r := rhc.NewBitReaderN(w.Bytes(), n)
		if _, _, err := rhc.ReadCount(r); err == nil {
			t.Errorf("ReadCount accepted %d of %d bits", n, full)
		}
	}
}
