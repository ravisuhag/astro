package crc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/crc"
)

// Benchmarks at frame rate.
//
// Every frame and every packet on both the send and receive paths runs one of
// these over its whole length, so the checksum is on the hottest path in the
// library. 1115 octets is the CCSDS 131.0 Reed-Solomon codeblock payload at
// interleave depth 5, the most common downlink frame size.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/crc/

var benchSizes = []struct {
	name string
	size int
}{
	{"packet-64", 64},
	{"frame-1115", 1115},
	{"frame-8192", 8192},
}

// Sinks for the benchmark results.
//
// A checksum is a pure function of an invariant buffer, so a loop that throws
// the answer away can be hoisted out entirely and the benchmark then measures
// nothing. Accumulating into a package-level variable keeps the call inside
// the loop where it belongs.
var (
	sink16 uint16
	sink32 uint32
)

func benchData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

func BenchmarkComputeCRC16(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			data := benchData(tc.size)

			b.SetBytes(int64(tc.size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sink16 ^= crc.ComputeCRC16(data)
			}
		})
	}
}

func BenchmarkComputeCRC32(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			data := benchData(tc.size)

			b.SetBytes(int64(tc.size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sink32 ^= crc.ComputeCRC32(data)
			}
		})
	}
}

// The bit-by-bit reference, benchmarked beside the table so the comparison is
// made in one process rather than across two runs on a machine whose clock
// speed and load are not constant.
func BenchmarkBitwiseCRC16(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			data := benchData(tc.size)

			b.SetBytes(int64(tc.size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sink16 ^= bitwiseCRC16(data)
			}
		})
	}
}

func BenchmarkBitwiseCRC32(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			data := benchData(tc.size)

			b.SetBytes(int64(tc.size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				sink32 ^= bitwiseCRC32(data)
			}
		})
	}
}
