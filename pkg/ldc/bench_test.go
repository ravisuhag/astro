package ldc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// Benchmarks for the compressors, which on an instrument-heavy mission run
// over far more octets than the frame layer ever sees.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/ldc/

var (
	sinkBytes   []byte
	sinkSamples []uint32
)

// ramp is a slowly changing signal, which is what the unit-delay predictor is
// for and what real instrument data looks like.
func benchRamp(count int) []uint32 {
	samples := make([]uint32, count)
	for i := range samples {
		samples[i] = uint32((i * 3) % 256)
	}
	return samples
}

// noise is the opposite case: nothing to predict, so the coder falls back on
// its least favourable option. Worth measuring because it bounds the cost.
func benchNoise(count int) []uint32 {
	samples := make([]uint32, count)
	state := uint32(1)
	for i := range samples {
		state = state*1664525 + 1013904223
		samples[i] = state >> 24
	}
	return samples
}

func BenchmarkCompress(b *testing.B) {
	params := ldc.DefaultParams()

	for name, samples := range map[string][]uint32{
		"ramp":  benchRamp(4096),
		"noise": benchNoise(4096),
	} {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(samples)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				out, err := ldc.Compress(samples, params)
				if err != nil {
					b.Fatal(err)
				}
				sinkBytes = out
			}
		})
	}
}

func BenchmarkDecompress(b *testing.B) {
	params := ldc.DefaultParams()

	for name, samples := range map[string][]uint32{
		"ramp":  benchRamp(4096),
		"noise": benchNoise(4096),
	} {
		b.Run(name, func(b *testing.B) {
			coded, err := ldc.Compress(samples, params)
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(len(samples)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				out, err := ldc.DecompressCount(coded, params, len(samples))
				if err != nil {
					b.Fatal(err)
				}
				sinkSamples = out
			}
		})
	}
}
