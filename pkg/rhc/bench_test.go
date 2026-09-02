package rhc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// Benchmarks for POCKET+, which runs once per housekeeping cycle.
//
// A cycle is cheap next to a frame, but it happens on a schedule for the
// whole mission, so the interesting number is what one cycle costs rather
// than throughput.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/rhc/

var sink []byte

// housekeeping is what the coder is for: a vector where almost nothing
// changes from one cycle to the next.
func benchHousekeeping(cycles, octets int) [][]byte {
	base := make([]byte, octets)
	for i := range base {
		base[i] = byte(i * 17)
	}

	out := make([][]byte, cycles)
	for i := range out {
		vector := make([]byte, octets)
		copy(vector, base)
		if i%7 == 0 {
			vector[i%octets] ^= 0x08
		}
		out[i] = vector
	}
	return out
}

func BenchmarkCompressCycle(b *testing.B) {
	const octets = 64
	vectors := benchHousekeeping(64, octets)

	compressor, err := rhc.NewCompressor(rhc.Config{VectorLength: octets * 8})
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(octets)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, _, err := compressor.Compress(vectors[i%len(vectors)])
		if err != nil {
			b.Fatal(err)
		}
		sink = out
	}
}

func BenchmarkDecompressCycle(b *testing.B) {
	const octets = 64
	vectors := benchHousekeeping(64, octets)

	compressor, err := rhc.NewCompressor(rhc.Config{VectorLength: octets * 8})
	if err != nil {
		b.Fatal(err)
	}

	type cycle struct {
		data   []byte
		bitLen int
	}
	coded := make([]cycle, 0, len(vectors))
	for _, vector := range vectors {
		out, bitLen, err := compressor.Compress(vector)
		if err != nil {
			b.Fatal(err)
		}
		coded = append(coded, cycle{data: append([]byte{}, out...), bitLen: bitLen})
	}

	// The decompressor is stateful and has to see the cycles in order, so it
	// is rebuilt whenever the benchmark laps the recorded run.
	decompressor, err := rhc.NewDecompressor(rhc.Config{VectorLength: octets * 8})
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(octets)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if i%len(coded) == 0 && i > 0 {
			decompressor.Reset()
		}
		out, err := decompressor.Decompress(coded[i%len(coded)].data, coded[i%len(coded)].bitLen)
		if err != nil {
			b.Fatal(err)
		}
		sink = out
	}
}
