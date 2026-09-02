package tcsc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tcsc"
)

// Benchmarks for the uplink coding layer.
//
// A command link runs at a fraction of a downlink's rate, so these matter
// less for throughput than the TM side — but a station commanding several
// spacecraft still runs them often, and the BCH codeblock is the unit every
// uplink octet passes through.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/tcsc/

var sink []byte

func benchData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

func BenchmarkBCHEncode(b *testing.B) {
	info := benchData(tcsc.InfoBytes)

	b.SetBytes(int64(len(info)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		block, err := tcsc.BCHEncode(info)
		if err != nil {
			b.Fatal(err)
		}
		sink = block[:]
	}
}

// The error-free case, which is what a good uplink sees.
func BenchmarkBCHDecodeClean(b *testing.B) {
	block, err := tcsc.BCHEncode(benchData(tcsc.InfoBytes))
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(block)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, _, err := tcsc.BCHDecode(block); err != nil {
			b.Fatal(err)
		}
	}
}

// With one error, which the code can correct.
func BenchmarkBCHDecodeWithError(b *testing.B) {
	block, err := tcsc.BCHEncode(benchData(tcsc.InfoBytes))
	if err != nil {
		b.Fatal(err)
	}
	block[3] ^= 0x08

	b.SetBytes(int64(len(block)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, _, err := tcsc.BCHDecode(block); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWrapCLTU(b *testing.B) {
	frame := benchData(1024)

	b.SetBytes(int64(len(frame)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := tcsc.WrapCLTU(frame, nil, nil, false)
		if err != nil {
			b.Fatal(err)
		}
		sink = out
	}
}
