package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

// Frame-rate benchmarks for USLP.
//
// The frames carry a FECF, which is USLP's default: the package offers
// WithoutFECF to drop it rather than an option to add it.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/usdl/

var sink []byte

const benchFrameLength = 1115

func benchPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func BenchmarkEncodeFrame(b *testing.B) {
	frame, err := usdl.NewTransferFrame(42, 1, 0, benchPayload(benchFrameLength/2))
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
	frame, err := usdl.NewTransferFrame(42, 1, 0, benchPayload(benchFrameLength/2))
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
		if _, err := usdl.DecodeTransferFrameWithConfig(encoded, usdl.ChannelConfig{HasFECF: true}); err != nil {
			b.Fatal(err)
		}
	}
}
