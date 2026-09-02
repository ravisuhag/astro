package tcdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tcdl"
)

// Benchmarks for TC transfer frames.
//
// A command link runs at a fraction of a downlink's rate, so these matter
// less for throughput than the TM side. They are here so the frame protocols
// are measured consistently rather than three of four.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/tcdl/

var sink []byte

func benchPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func BenchmarkEncodeFrame(b *testing.B) {
	frame, err := tcdl.NewTCTransferFrame(42, 1, benchPayload(1017))
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
		out, err := frame.Encode()
		if err != nil {
			b.Fatal(err)
		}
		sink = out
	}
}

func BenchmarkDecodeFrame(b *testing.B) {
	frame, err := tcdl.NewTCTransferFrame(42, 1, benchPayload(1017))
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
		if _, err := tcdl.DecodeTCTransferFrame(encoded); err != nil {
			b.Fatal(err)
		}
	}
}
