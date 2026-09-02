package rhc_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// fuzzConfigs is the matrix the fuzz bodies sweep.
var fuzzConfigs = []rhc.Config{
	{VectorLength: 64, Robustness: 0},
	{VectorLength: 64, Robustness: 3, NewMaskInterval: 4},
	{VectorLength: 7, Robustness: 1},
	{VectorLength: 129, Robustness: 7, SendMaskInterval: 3, UncompressedInterval: 5},
}

// FuzzDecompressPacket throws arbitrary bytes at the decompressor.
//
// Two properties. It never panics. Every read goes through BitReader, which
// reports exhaustion. And it never poisons itself: after any amount of
// rubbish, a decompressor that was working still works, which is what the
// parse-then-commit split in Decompress exists to guarantee.
func FuzzDecompressPacket(f *testing.F) {
	// Seed with real output.
	for _, config := range fuzzConfigs {
		packets := housekeeping(8, config.VectorLength, 2, 3)
		compressor, err := rhc.NewCompressor(config)
		if err != nil {
			continue
		}
		for _, packet := range packets {
			data, _, err := compressor.Compress(packet)
			if err != nil {
				continue
			}
			f.Add(data)
			if len(data) > 1 {
				f.Add(data[:len(data)/2]) // truncated
			}
		}
	}
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF})
	f.Add(make([]byte, 64))
	f.Add(bytes.Repeat([]byte{0xFF}, 64))

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, config := range fuzzConfigs {
			// A fresh decompressor, which has no state to lose.
			fresh, err := rhc.NewDecompressor(config)
			if err != nil {
				t.Fatalf("NewDecompressor() = %v", err)
			}
			_, _ = fresh.Decompress(data, 0)
			_, _ = fresh.Decompress(data, len(data)*8)

			// A decompressor part way through a stream, which has state that
			// must survive.
			packets := housekeeping(6, config.VectorLength, 2, 4)
			compressor, err := rhc.NewCompressor(config)
			if err != nil {
				t.Fatalf("NewCompressor() = %v", err)
			}
			midStream, err := rhc.NewDecompressor(config)
			if err != nil {
				t.Fatalf("NewDecompressor() = %v", err)
			}

			var stream []codedVector
			for _, packet := range packets {
				coded, bitLen, err := compressor.Compress(packet)
				if err != nil {
					t.Fatalf("Compress() = %v", err)
				}
				stream = append(stream, codedVector{data: coded, bitLen: bitLen})
			}
			for i := range 3 {
				if _, err := midStream.Decompress(stream[i].data, stream[i].bitLen); err != nil {
					t.Fatalf("packet %d: %v", i, err)
				}
			}

			// Rubbish in the middle.
			_, rubbishErr := midStream.Decompress(data, 0)

			// What may be claimed next depends on whether the rubbish was
			// rejected.
			//
			// Rejected: nothing was committed, so the stream must carry on
			// exactly as before. That is what the parse-then-commit split in
			// Decompress buys, and it is worth asserting hard.
			//
			// Accepted: the bytes parsed as a well-formed output vector, and
			// the decompressor took them for one. It has no way not to. Clause 2.2
			// is explicit that the standard "does not incorporate sync
			// markers or other mechanisms to flag the header of the next
			// output binary vector", and leaves detection to the mission. So
			// the only property left is the one that always holds: nothing
			// panics.
			if rubbishErr == nil {
				for i := 3; i < len(stream); i++ {
					_, _ = midStream.Decompress(stream[i].data, stream[i].bitLen)
				}
				continue
			}

			for i := 3; i < len(stream); i++ {
				got, err := midStream.Decompress(stream[i].data, stream[i].bitLen)
				if err != nil {
					// Refusing is allowed: a rejected output still counts as
					// a lost one, which widens the gap.
					continue
				}
				if !bytes.Equal(got, packets[i]) {
					t.Fatalf("packet %d came back wrong after rejected rubbish: got %08b want %08b",
						i, got, packets[i])
				}
			}
		}
	})
}

// FuzzCompressRoundTrip fuzzes the input vectors rather than the coded bytes.
//
// The property is identity, which is the one claim a lossless compressor
// makes.
func FuzzCompressRoundTrip(f *testing.F) {
	f.Add([]byte{0}, uint8(0))
	f.Add(bytes.Repeat([]byte{0xAA}, 32), uint8(1))
	f.Add(make([]byte, 64), uint8(2))
	f.Add([]byte{0xFF, 0x00, 0xFF, 0x00}, uint8(3))

	f.Fuzz(func(t *testing.T, raw []byte, selector uint8) {
		if len(raw) == 0 {
			return
		}
		config := fuzzConfigs[int(selector)%len(fuzzConfigs)]
		octets := (config.VectorLength + 7) / 8

		// Slice the fuzz bytes into fixed-length vectors, masking off any
		// bits past the vector length so the input is legal.
		count := len(raw) / octets
		if count == 0 || count > 64 {
			if count == 0 {
				return
			}
			count = 64
		}
		packets := make([][]byte, count)
		for i := range packets {
			packet := make([]byte, octets)
			copy(packet, raw[i*octets:(i+1)*octets])
			if excess := octets*8 - config.VectorLength; excess > 0 {
				packet[octets-1] &^= byte(1<<excess - 1)
			}
			packets[i] = packet
		}

		compressor, err := rhc.NewCompressor(config)
		if err != nil {
			t.Fatalf("NewCompressor() = %v", err)
		}
		decompressor, err := rhc.NewDecompressor(config)
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
				t.Fatalf("packet %d: Decompress of our own output = %v", i, err)
			}
			if !bytes.Equal(back, packet) {
				t.Fatalf("packet %d: %08b became %08b", i, packet, back)
			}
		}
	})
}
