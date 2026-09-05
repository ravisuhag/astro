package tmdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// Benchmarks for the paths a ground station runs at frame rate.
//
// A 1115-octet frame at a few megabits per second is a few hundred frames a
// second, and a station handling several spacecraft multiplies that. What
// matters is not only time but allocations: a per-frame allocation is a
// per-frame contribution to garbage collection, and the pauses land on the
// same goroutine that has to keep up with the downlink.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/tmdl/

// benchFrameLength is the CCSDS 131.0 Reed-Solomon codeblock payload for an
// interleave depth of 5, which is the most common downlink frame size.
const benchFrameLength = 1115

// primaryHeaderSize is the TM Transfer Frame Primary Header, fixed at six
// octets by CCSDS 132.0-B-3 4.1.2. The package does not export it, and a
// benchmark is not a reason to widen the API.
const primaryHeaderSize = 6

func benchConfig() tmdl.ChannelConfig {
	return tmdl.ChannelConfig{FrameLength: benchFrameLength, HasFEC: true}
}

// payload is a frame's worth of data field, allowing for the primary header
// and the frame error control field.
func payload(config tmdl.ChannelConfig) []byte {
	size := config.FrameLength - primaryHeaderSize
	if config.HasFEC {
		size -= 2
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func BenchmarkEncodeFrame(b *testing.B) {
	config := benchConfig()
	data := payload(config)

	frame, err := tmdl.NewTransferFrame(42, 1, data, nil, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(config.FrameLength))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := frame.EncodeWithConfig(config); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNewAndEncodeFrame is the whole spacecraft-side cost of one frame:
// building it and putting it on the wire.
func BenchmarkNewAndEncodeFrame(b *testing.B) {
	config := benchConfig()
	data := payload(config)

	b.SetBytes(int64(config.FrameLength))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		frame, err := tmdl.NewTransferFrame(42, 1, data, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := frame.EncodeWithConfig(config); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeFrame(b *testing.B) {
	config := benchConfig()

	frame, err := tmdl.NewTransferFrame(42, 1, payload(config), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := frame.EncodeWithConfig(config)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(encoded)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := tmdl.DecodeTransferFrame(encoded); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeHeader isolates the header, which is what a demultiplexer
// reads: it needs the virtual channel and the counts, not the data field.
func BenchmarkDecodeHeader(b *testing.B) {
	config := benchConfig()

	frame, err := tmdl.NewTransferFrame(42, 1, payload(config), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := frame.EncodeWithConfig(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var header tmdl.PrimaryHeader
		if err := header.Decode(encoded[:primaryHeaderSize]); err != nil {
			b.Fatal(err)
		}
	}
}
