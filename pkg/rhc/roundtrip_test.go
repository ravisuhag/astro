package rhc_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// housekeeping builds a stream of slowly changing fixed-length vectors, which
// is the data shape CCSDS 124.0-B-1 exists for.
func housekeeping(count, lengthBits int, flipsPerStep int, seed int64) [][]byte {
	rng := rand.New(rand.NewSource(seed))
	octets := (lengthBits + 7) / 8

	current := make([]byte, octets)
	for i := range current {
		current[i] = byte(rng.Intn(256))
	}
	// Clear any bits past the vector length so the input is legal.
	if excess := octets*8 - lengthBits; excess > 0 {
		current[octets-1] &^= byte(1<<excess - 1)
	}

	out := make([][]byte, count)
	for i := range out {
		for range flipsPerStep {
			bit := rng.Intn(lengthBits)
			current[bit/8] ^= 1 << (7 - uint(bit%8))
		}
		packet := make([]byte, octets)
		copy(packet, current)
		out[i] = packet
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		config rhc.Config
		flips  int
		count  int
	}{
		{"64 bits, robustness 0", rhc.Config{VectorLength: 64, Robustness: 0}, 2, 50},
		{"64 bits, robustness 3", rhc.Config{VectorLength: 64, Robustness: 3}, 2, 50},
		{"64 bits, robustness 7", rhc.Config{VectorLength: 64, Robustness: 7}, 2, 50},
		{"one bit", rhc.Config{VectorLength: 1, Robustness: 1}, 1, 20},
		{"seven bits, not octet aligned", rhc.Config{VectorLength: 7, Robustness: 2}, 1, 30},
		{"1000 bits", rhc.Config{VectorLength: 1000, Robustness: 3}, 5, 40},
		{"new mask every 8", rhc.Config{
			VectorLength: 128, Robustness: 2, NewMaskInterval: 8,
		}, 3, 60},
		{"send mask every 5", rhc.Config{
			VectorLength: 128, Robustness: 2, SendMaskInterval: 5,
		}, 3, 40},
		{"uncompressed every 10", rhc.Config{
			VectorLength: 128, Robustness: 2, UncompressedInterval: 10,
		}, 3, 40},
		{"all knobs", rhc.Config{
			VectorLength: 96, Robustness: 4,
			NewMaskInterval: 7, SendMaskInterval: 11, UncompressedInterval: 13,
		}, 4, 80},
		{"unchanging stream", rhc.Config{VectorLength: 64, Robustness: 2}, 0, 30},
		{"heavy change", rhc.Config{VectorLength: 64, Robustness: 2}, 30, 30},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			packets := housekeeping(test.count, test.config.VectorLength, test.flips, 1)

			compressor, err := rhc.NewCompressor(test.config)
			if err != nil {
				t.Fatalf("NewCompressor() = %v", err)
			}
			decompressor, err := rhc.NewDecompressor(test.config)
			if err != nil {
				t.Fatalf("NewDecompressor() = %v", err)
			}

			for i, packet := range packets {
				coded, bitLen, err := compressor.Compress(packet)
				if err != nil {
					t.Fatalf("packet %d: Compress() = %v", i, err)
				}

				back, err := decompressor.Decompress(coded, bitLen)
				if err != nil {
					t.Fatalf("packet %d: Decompress() = %v", i, err)
				}
				if !bytes.Equal(back, packet) {
					t.Fatalf("packet %d: %08b came back as %08b", i, packet, back)
				}
			}
		})
	}
}

// TestCompressionActuallyCompresses is the point of the package.
func TestCompressionActuallyCompresses(t *testing.T) {
	config := rhc.Config{VectorLength: 512, Robustness: 3, NewMaskInterval: 32}
	packets := housekeeping(200, config.VectorLength, 2, 2)

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatal(err)
	}

	rawBits := 0
	codedBits := 0
	for _, packet := range packets {
		_, bitLen, err := compressor.Compress(packet)
		if err != nil {
			t.Fatal(err)
		}
		rawBits += config.VectorLength
		codedBits += bitLen
	}

	if codedBits >= rawBits {
		t.Errorf("coded %d bits from %d raw; that is not compression", codedBits, rawBits)
	}
	t.Logf("%d bits from %d raw, ratio %.2f", codedBits, rawBits,
		float64(rawBits)/float64(codedBits))
}
