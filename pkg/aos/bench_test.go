package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

// Frame-rate benchmarks for AOS, the data link most high-rate missions fly.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/aos/

var sink []byte

// benchFrameLength is the CCSDS 131.0 Reed-Solomon codeblock payload at
// interleave depth 5, the most common downlink frame size.
const benchFrameLength = 1115

func benchPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func BenchmarkEncodeFrame(b *testing.B) {
	// The primary header is six octets and the FECF two.
	frame, err := aos.NewTransferFrame(42, 1, benchPayload(benchFrameLength-8), aos.WithFECF())
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(benchFrameLength)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		encoded, err := frame.Encode()
		if err != nil {
			b.Fatal(err)
		}
		sink = encoded
	}
}

func BenchmarkDecodeFrame(b *testing.B) {
	frame, err := aos.NewTransferFrame(42, 1, benchPayload(benchFrameLength-8), aos.WithFECF())
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(encoded)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := aos.DecodeTransferFrame(encoded, 0, false, true); err != nil {
			b.Fatal(err)
		}
	}
}

// The header alone, which is what a demultiplexer reads: it needs the virtual
// channel and the counts, not the data field.
func BenchmarkDecodeHeader(b *testing.B) {
	frame, err := aos.NewTransferFrame(42, 1, benchPayload(benchFrameLength-8), aos.WithFECF())
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var header aos.PrimaryHeader
		if err := header.Decode(encoded[:6]); err != nil {
			b.Fatal(err)
		}
	}
}
