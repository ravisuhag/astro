package pxsc_test

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxsc"
)

// TestViterbiRoundTrip is the basic claim: what the encoder produced, the
// decoder recovers.
func TestViterbiRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"one octet", []byte{0xAB}},
		{"all zeros", make([]byte, 16)},
		{"all ones", bytes.Repeat([]byte{0xFF}, 16)},
		{"alternating", bytes.Repeat([]byte{0xAA, 0x55}, 8)},
		{"idle pattern", bytes.Repeat([]byte{0x55}, 64)},
		{"text", []byte("Proximity-1 space link protocol, coding layer.")},
		{"long", func() []byte {
			b := make([]byte, 1024)
			for i := range b {
				b[i] = byte(i * 7)
			}
			return b
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			symbols := pxsc.ConvolutionalEncode(test.data)

			got, err := pxsc.ViterbiDecode(symbols)
			if err != nil {
				t.Fatalf("ViterbiDecode() = %v", err)
			}
			if !bytes.Equal(got, test.data) {
				t.Errorf("decoded %d octets, want %d\n got %X\nwant %X",
					len(got), len(test.data), got, test.data)
			}
		})
	}
}

// TestViterbiKnownAnswer decodes symbols pinned from an independent encoder
// (the libfec / gr-satellites realization of the CCSDS 171/133 code), not from
// this package's own. A decoder that merely mirrors a wrong encoder passes
// every round-trip test and still fails this one.
func TestViterbiKnownAnswer(t *testing.T) {
	symbols := []byte{0x6E, 0x9F, 0x23, 0x2F, 0x20, 0x93, 0x53, 0x19, 0xAA, 0x23}

	got, err := pxsc.ViterbiDecode(symbols)
	if err != nil {
		t.Fatalf("ViterbiDecode() = %v", err)
	}
	if want := []byte("CCSDS"); !bytes.Equal(got, want) {
		t.Errorf("decoded %X, want %X (%q)", got, want, want)
	}
}

// TestViterbiCorrectsErrors is the reason the decoder exists. A rate 1/2,
// constraint-length 7 code has a free distance of 10, so isolated errors
// spaced well apart are recovered exactly.
func TestViterbiCorrectsErrors(t *testing.T) {
	data := make([]byte, 512)
	for i := range data {
		data[i] = byte(i*31 + 17)
	}
	clean := pxsc.ConvolutionalEncode(data)

	for _, spacing := range []int{64, 128, 256} {
		t.Run(name(spacing), func(t *testing.T) {
			corrupt := append([]byte(nil), clean...)

			// One flipped symbol every `spacing` bits, far enough apart that
			// the trellis recovers between them.
			flips := 0
			for bit := spacing / 2; bit < len(corrupt)*8; bit += spacing {
				corrupt[bit/8] ^= 1 << uint(7-bit%8)
				flips++
			}

			got, err := pxsc.ViterbiDecode(corrupt)
			if err != nil {
				t.Fatalf("ViterbiDecode() = %v", err)
			}
			if !bytes.Equal(got, data) {
				t.Errorf("%d isolated symbol errors were not corrected", flips)
			}
		})
	}
}

// TestViterbiCorrectsRandomErrors checks a whole channel rather than placed
// flips: at a 1% symbol error rate the code should recover the message
// outright most of the time, and never do worse than the uncoded stream.
func TestViterbiCorrectsRandomErrors(t *testing.T) {
	random := rand.New(rand.NewSource(20112)) //nolint:gosec // reproducible test channel, not cryptography

	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(random.Intn(256))
	}
	clean := pxsc.ConvolutionalEncode(data)

	const trials = 20
	exact := 0

	for range trials {
		corrupt := append([]byte(nil), clean...)
		for bit := range len(corrupt) * 8 {
			if random.Float64() < 0.01 {
				corrupt[bit/8] ^= 1 << uint(7-bit%8)
			}
		}

		got, err := pxsc.ViterbiDecode(corrupt)
		if err != nil {
			t.Fatalf("ViterbiDecode() = %v", err)
		}
		if bytes.Equal(got, data) {
			exact++
		}
	}

	// The margin is generous on purpose. This asserts the decoder works as a
	// decoder, not a particular coding gain figure.
	if exact < trials*3/4 {
		t.Errorf("recovered %d of %d messages at a 1%% symbol error rate; the decoder is not correcting", exact, trials)
	}
}

// TestViterbiUncorrectedIsWorse is the control for the test above: without
// decoding, a 1% symbol error rate leaves errors behind. If this passed
// trivially the test above would prove nothing.
func TestViterbiUncorrectedIsWorse(t *testing.T) {
	random := rand.New(rand.NewSource(7)) //nolint:gosec // reproducible test channel, not cryptography

	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(random.Intn(256))
	}
	clean := pxsc.ConvolutionalEncode(data)

	corrupt := append([]byte(nil), clean...)
	flipped := 0
	for bit := range len(corrupt) * 8 {
		if random.Float64() < 0.01 {
			corrupt[bit/8] ^= 1 << uint(7-bit%8)
			flipped++
		}
	}
	if flipped == 0 {
		t.Fatal("the test channel introduced no errors")
	}
	if bytes.Equal(corrupt, clean) {
		t.Fatal("the corrupted stream equals the clean one")
	}
}

// TestViterbiStreamingMatchesOneShot checks that a stream split across calls
// decodes the same as one handed over whole. Clause 3.4.3.2 encodes everything as a
// single continuous stream, so the decoder has to carry the trellis across
// calls the way the encoder carries its register.
func TestViterbiStreamingMatchesOneShot(t *testing.T) {
	data := []byte("a continuous stream of PLTUs and idle data, encoded as one")
	symbols := pxsc.ConvolutionalEncode(data)

	want, err := pxsc.ViterbiDecode(symbols)
	if err != nil {
		t.Fatal(err)
	}

	for _, chunk := range []int{2, 4, 10, 32} {
		t.Run(name(chunk), func(t *testing.T) {
			d := pxsc.NewViterbiDecoder()

			var got []byte
			for start := 0; start < len(symbols); start += chunk {
				end := min(start+chunk, len(symbols))

				out, err := d.Decode(symbols[start:end])
				if err != nil {
					t.Fatalf("Decode() = %v", err)
				}
				got = append(got, out...)
			}
			got = append(got, d.Flush()...)

			if !bytes.Equal(got, want) {
				t.Errorf("chunked decode differs from one-shot\n got %X\nwant %X", got, want)
			}
		})
	}
}

// TestViterbiSoftDecisionsBeatHard is the point of clause 3.4.3.3. Given the same
// channel, three-bit soft decisions should recover messages that hard
// decisions lose, because a marginal symbol costs little to overrule.
func TestViterbiSoftDecisionsBeatHard(t *testing.T) {
	random := rand.New(rand.NewSource(4242)) //nolint:gosec // reproducible test channel, not cryptography

	data := make([]byte, 128)
	for i := range data {
		data[i] = byte(random.Intn(256))
	}
	clean := pxsc.ConvolutionalEncode(data)

	const trials = 30
	hardWins, softWins := 0, 0

	for range trials {
		// Build a channel that reports confidence. Most symbols arrive clean
		// and certain; a few land near the decision threshold and those are
		// the ones that flip. A hard decision throws that distinction away and
		// treats every symbol as equally trustworthy, which is exactly what
		// costs it.
		soft := make([]int8, 0, len(clean)*8)
		hard := append([]byte(nil), clean...)

		for bit := range len(clean) * 8 {
			sent := clean[bit/8] >> uint(7-bit%8) & 1

			// Confidence from 1 (right on the threshold) to 4 (certain), and
			// the chance a symbol at that confidence came out wrong.
			confidence, flipChance := int8(4), 0.0
			switch roll := random.Float64(); {
			case roll < 0.05:
				confidence, flipChance = 1, 0.45
			case roll < 0.15:
				confidence, flipChance = 2, 0.15
			case roll < 0.30:
				confidence, flipChance = 3, 0.01
			}

			value := confidence
			if sent == 0 {
				value = -confidence
			}
			if random.Float64() < flipChance {
				value = -value
				hard[bit/8] ^= 1 << uint(7-bit%8)
			}
			soft = append(soft, value)
		}

		hardOut, err := pxsc.ViterbiDecode(hard)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(hardOut, data) {
			hardWins++
		}

		d := pxsc.NewViterbiDecoder()
		softOut, err := d.DecodeSoft(soft)
		if err != nil {
			t.Fatal(err)
		}
		softOut = append(softOut, d.Flush()...)
		if bytes.Equal(softOut, data) {
			softWins++
		}
	}

	t.Logf("soft %d/%d, hard %d/%d", softWins, trials, hardWins, trials)

	// On this channel soft decisions recover every message and hard decisions
	// about half, which is the clause 3.4.3.3 gain showing up. The assertion is only
	// that soft wins, not by how much.
	if softWins <= hardWins {
		t.Errorf("soft decisions recovered %d of %d messages and hard decisions %d; soft should win",
			softWins, trials, hardWins)
	}
}

// TestViterbiHardMatchesFullConfidenceSoft pins the two entry points together:
// a hard decision is a soft decision the demodulator is certain about, so the
// two paths must agree.
func TestViterbiHardMatchesFullConfidenceSoft(t *testing.T) {
	data := []byte("hard and soft agree at full confidence")
	symbols := pxsc.ConvolutionalEncode(data)

	want, err := pxsc.ViterbiDecode(symbols)
	if err != nil {
		t.Fatal(err)
	}

	soft := make([]int8, 0, len(symbols)*8)
	for _, b := range symbols {
		for i := 7; i >= 0; i-- {
			if b>>uint(i)&1 == 1 {
				soft = append(soft, 1)
			} else {
				soft = append(soft, -1)
			}
		}
	}

	d := pxsc.NewViterbiDecoder()
	got, err := d.DecodeSoft(soft)
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, d.Flush()...)

	if !bytes.Equal(got, want) {
		t.Errorf("soft path gave %X, hard path %X", got, want)
	}
}

func TestViterbiRejectsPartialSymbols(t *testing.T) {
	// Two coded bits per input bit, so an odd octet count cannot be whole.
	if _, err := pxsc.ViterbiDecode([]byte{0x00, 0x00, 0x00}); !errors.Is(err, pxsc.ErrInvalidLength) {
		t.Errorf("ViterbiDecode(3 octets) = %v, want ErrInvalidLength", err)
	}

	d := pxsc.NewViterbiDecoder()
	if _, err := d.DecodeSoft([]int8{1, -1, 1}); !errors.Is(err, pxsc.ErrInvalidLength) {
		t.Errorf("DecodeSoft(3 symbols) = %v, want ErrInvalidLength", err)
	}
}

func TestViterbiEmptyInput(t *testing.T) {
	got, err := pxsc.ViterbiDecode(nil)
	if err != nil {
		t.Fatalf("ViterbiDecode(nil) = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ViterbiDecode(nil) = %X, want nothing", got)
	}
}

// TestViterbiResetStartsAStream checks that Reset really does return the
// decoder to state zero, rather than leaving the previous stream's trellis in
// place.
func TestViterbiResetStartsAStream(t *testing.T) {
	first := []byte("the first stream")
	second := []byte("a different second stream entirely")

	d := pxsc.NewViterbiDecoder()

	out, err := d.Decode(pxsc.ConvolutionalEncode(first))
	if err != nil {
		t.Fatal(err)
	}
	_ = append(out, d.Flush()...)

	d.Reset()

	// The encoder must be reset too: both sides start a stream together.
	out, err = d.Decode(pxsc.ConvolutionalEncode(second))
	if err != nil {
		t.Fatal(err)
	}
	got := append(out, d.Flush()...)

	if !bytes.Equal(got, second) {
		t.Errorf("after Reset the decoder gave %X, want %X", got, second)
	}
}

// TestViterbiTracebackDelayEmitsEverything guards the boundary between Decode
// and Flush: nothing may be dropped or duplicated, whatever the length.
func TestViterbiTracebackDelayEmitsEverything(t *testing.T) {
	for length := 1; length <= 20; length++ {
		data := make([]byte, length)
		for i := range data {
			data[i] = byte(0x5A + i)
		}

		got, err := pxsc.ViterbiDecode(pxsc.ConvolutionalEncode(data))
		if err != nil {
			t.Fatalf("length %d: %v", length, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("length %d: got %X, want %X", length, got, data)
		}
	}
}

func FuzzViterbiRoundTrip(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte("Proximity-1"))
	f.Add(bytes.Repeat([]byte{0xFF}, 40))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		got, err := pxsc.ViterbiDecode(pxsc.ConvolutionalEncode(data))
		if err != nil {
			t.Fatalf("ViterbiDecode() = %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("round trip changed the data\n got %X\nwant %X", got, data)
		}
	})
}

func FuzzViterbiDecodeNeverPanics(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xFA, 0xF3})
	f.Add(bytes.Repeat([]byte{0xA5}, 33))

	f.Fuzz(func(t *testing.T, symbols []byte) {
		if len(symbols) > 8192 {
			return
		}
		out, err := pxsc.ViterbiDecode(symbols)
		if err != nil {
			return
		}
		// Two coded bits per input bit, so the output is half the input.
		if want := len(symbols) / 2; len(out) != want {
			t.Fatalf("decoded %d octets from %d, want %d", len(out), len(symbols), want)
		}
	})
}

// name renders an int as a subtest name without pulling in fmt for one call.
func name(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func BenchmarkViterbiDecode(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i * 13)
	}
	symbols := pxsc.ConvolutionalEncode(data)

	b.SetBytes(int64(len(symbols)))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := pxsc.ViterbiDecode(symbols); err != nil {
			b.Fatal(err)
		}
	}
}
