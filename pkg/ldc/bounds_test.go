package ldc_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// zeroBlockParams is a Params value under which readZeroBlock never needs a
// reference sample, so a crafted stream of zero-block runs can be built
// without worrying about reference bits landing inside it.
var zeroBlockParams = ldc.Params{BlockSize: 64, Resolution: 8, ReferenceInterval: 1}

// zeroRunCodeword builds one zero-block coded data set for zeroBlockParams
// covering a run of count all-zero blocks (1 to 63, non-ROS).
//
// The option identifier is 4 zero bits: idWidth is 3 at this resolution
// (unrestricted, resolution <= 8), and the zero-block escape adds one more
// zero. The run itself is an FS codeword: table 3-2 maps a count above 4
// straight to itself, so it is count zero bits then a one bit.
func zeroRunCodeword(w *ldc.BitWriter, count int) {
	w.WriteBits(0, 4)
	w.WriteZeros(uint64(count))
	w.WriteOne()
}

// TestDecompressBoundsOutputWithoutCount feeds Decompress a small crafted
// stream that decodes into far more samples than its ceiling allows.
//
// Each codeword is a run of 63 all-zero blocks of 64 samples, 4032 samples
// from 68 bits of input. Two of them cross a ceiling of 5000, so the second
// codeword must be rejected instead of decoded: without the ceiling from
// Plan 013 step 1, this stream decodes cleanly into 8064 samples and the test
// fails.
func TestDecompressBoundsOutputWithoutCount(t *testing.T) {
	var w ldc.BitWriter
	zeroRunCodeword(&w, 63)
	zeroRunCodeword(&w, 63)
	data := w.Bytes()

	// Sanity check on the setup itself: this should be a handful of bytes,
	// not something sized to the sample counts involved.
	if len(data) > 32 {
		t.Fatalf("crafted input is %d bytes, expected well under 32", len(data))
	}

	_, err := ldc.Decompress(data, zeroBlockParams, ldc.WithMaxSamples(5000))
	if !errors.Is(err, ldc.ErrOutputTooLarge) {
		t.Fatalf("err = %v, want ErrOutputTooLarge", err)
	}
}

// TestDecompressBoundsOutputDefaultCeiling checks that Decompress applies a
// ceiling even when WithMaxSamples is never given, so the default guards
// callers who do not know to ask for it.
func TestDecompressBoundsOutputDefaultCeiling(t *testing.T) {
	// One codeword's worth of samples is nowhere near maxDecodableSamples
	// (1<<28), so this only checks that a single small crafted stream still
	// decodes fine under the default, and a below-limit run is not rejected.
	var w ldc.BitWriter
	zeroRunCodeword(&w, 63)
	data := w.Bytes()

	out, err := ldc.Decompress(data, zeroBlockParams)
	if err != nil {
		t.Fatalf("Decompress under the default ceiling: %v", err)
	}
	if len(out) != 63*64 {
		t.Fatalf("got %d samples, want %d", len(out), 63*64)
	}
}

// TestDecompressRoundTripUnaffectedByCeiling checks that an ordinary,
// block-aligned round trip through Decompress still works once it carries a
// ceiling.
func TestDecompressRoundTripUnaffectedByCeiling(t *testing.T) {
	p := ldc.DefaultParams()
	samples := make([]uint32, p.BlockSize*4)
	for i := range samples {
		samples[i] = uint32(i % 200)
	}

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	back, err := ldc.Decompress(coded, p)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if len(back) != len(samples) {
		t.Fatalf("got %d samples, want %d", len(back), len(samples))
	}
	for i := range samples {
		if back[i] != samples[i] {
			t.Fatalf("sample %d: got %d, want %d", i, back[i], samples[i])
		}
	}
}

// TestDecompressCountUnaffectedByCeiling checks that DecompressCount, which
// is bounded by its own explicit count, is not double-bounded by the new
// ceiling: a legitimate decode larger than a small WithMaxSamples-style limit
// must still succeed, because DecompressCount has no such option to give.
func TestDecompressCountUnaffectedByCeiling(t *testing.T) {
	p := ldc.DefaultParams()
	const count = 6000 // comfortably past the 5000-sample ceiling used above

	samples := make([]uint32, count)
	for i := range samples {
		samples[i] = uint32(i % 4) // low entropy, compresses fast and small
	}

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}

	back, err := ldc.DecompressCount(coded, p, count)
	if err != nil {
		t.Fatalf("DecompressCount: %v", err)
	}
	if len(back) != count {
		t.Fatalf("got %d samples, want %d", len(back), count)
	}
	for i := range samples {
		if back[i] != samples[i] {
			t.Fatalf("sample %d: got %d, want %d", i, back[i], samples[i])
		}
	}
}
