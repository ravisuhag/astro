package tmsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tmsc"
)

// Benchmarks for the coding layer, which is the most arithmetic-heavy thing
// on a downlink.
//
// Reed-Solomon runs over every codeblock on both sides. Encoding is a
// polynomial division and is cheap; decoding has to compute syndromes and,
// when they are non-zero, run Berlekamp-Massey and Chien search, so the
// error-free and error-present cases cost very different amounts. Both are
// measured, because a ground station's throughput depends on which one it
// spends its time in.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/tmsc/

var sink []byte

func benchData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

func BenchmarkRSEncode(b *testing.B) {
	for _, tc := range []struct {
		name  string
		codec *tmsc.RSCodec
	}{
		{"255,223", tmsc.NewRS255_223()},
		{"255,239", tmsc.NewRS255_239()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data := benchData(tc.codec.DataLen())

			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				out, err := tc.codec.Encode(data)
				if err != nil {
					b.Fatal(err)
				}
				sink = out
			}
		})
	}
}

// The error-free case, which is what a good link spends its time in.
func BenchmarkRSDecodeClean(b *testing.B) {
	codec := tmsc.NewRS255_223()

	codeword, err := codec.Encode(benchData(codec.DataLen()))
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(codeword)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, _, err := codec.Decode(codeword); err != nil {
			b.Fatal(err)
		}
	}
}

// With errors to correct, which is what the code is for and what costs.
func BenchmarkRSDecodeWithErrors(b *testing.B) {
	codec := tmsc.NewRS255_223()

	for _, errors := range []int{1, 8, 16} {
		b.Run(errorName(errors), func(b *testing.B) {
			clean, err := codec.Encode(benchData(codec.DataLen()))
			if err != nil {
				b.Fatal(err)
			}

			// Corrupt distinct octets, spread out so they do not land in one
			// burst.
			corrupted := make([]byte, len(clean))
			copy(corrupted, clean)
			for e := 0; e < errors; e++ {
				corrupted[e*13%len(corrupted)] ^= 0xFF
			}

			b.SetBytes(int64(len(corrupted)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, _, err := codec.Decode(corrupted); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWrapCADU(b *testing.B) {
	frame := benchData(1115)

	b.SetBytes(int64(len(frame)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = tmsc.WrapCADU(frame, nil, false)
	}
}

// With randomisation, which walks the whole frame through the PN sequence.
func BenchmarkWrapCADURandomized(b *testing.B) {
	frame := benchData(1115)

	b.SetBytes(int64(len(frame)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink = tmsc.WrapCADU(frame, nil, true)
	}
}

func errorName(n int) string {
	switch n {
	case 1:
		return "1-error"
	case 8:
		return "8-errors"
	default:
		return "16-errors"
	}
}
