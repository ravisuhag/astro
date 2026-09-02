package ldc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// fuzzParams is the small matrix the fuzz bodies sweep. It covers both
// identifier widths that matter, both preprocessor modes, and the restricted
// option set.
var fuzzParams = []ldc.Params{
	{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4},
	{BlockSize: 16, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 64},
	{BlockSize: 8, Resolution: 16, Predictor: ldc.PredictorBypass, ReferenceInterval: 2},
	{BlockSize: 8, Resolution: 4, Restricted: true, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2},
	{BlockSize: 8, Resolution: 32, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1},
}

// FuzzDecompress throws arbitrary bytes at the decoder.
//
// The property is that it never panics and never spins: every read goes
// through BitReader, which reports exhaustion, and every FS codeword is read
// with a limit so a run of zero octets cannot be taken for an enormous
// sample. Both Decompress and DecompressFile are exercised, because they
// reach the decoder by different routes. One is told the parameters, the
// other reads them from a header it must not trust.
func FuzzDecompress(f *testing.F) {
	// Seed with real output from every parameter set.
	for _, p := range fuzzParams {
		samples := lowEntropy(64, 1)
		for i := range samples {
			if p.Resolution < 32 {
				samples[i] &= (1 << p.Resolution) - 1
			}
		}
		if coded, err := ldc.Compress(samples, p); err == nil {
			f.Add(coded)
			if len(coded) > 2 {
				f.Add(coded[:len(coded)/2]) // truncated
			}
		}
		if file, err := ldc.CompressFile(samples, p, 1); err == nil {
			f.Add(file)
		}
	}
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 64)) // all zeros: a long FS run with no terminator

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, p := range fuzzParams {
			_, _ = ldc.Decompress(data, p)
			_, _ = ldc.DecompressCount(data, p, 64)
			_, _ = ldc.Analyze(data, p, 64)
		}
		// The self-describing path, where the header itself is hostile.
		_, _ = ldc.DecompressFile(data)
		_, _ = ldc.DecodeFileHeader(data)
	})
}

// FuzzCompressRoundTrip fuzzes the samples rather than the coded bytes.
//
// The property is identity: whatever the data, compressing and decompressing
// gives it back. That is the one claim a lossless coder makes, and it is
// worth checking against inputs no hand-written test would think of.
func FuzzCompressRoundTrip(f *testing.F) {
	f.Add([]byte{0}, uint8(0))
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8}, uint8(0))
	f.Add(make([]byte, 64), uint8(1))
	f.Add([]byte{255, 0, 255, 0, 255, 0, 255, 0}, uint8(2))

	f.Fuzz(func(t *testing.T, raw []byte, selector uint8) {
		if len(raw) == 0 {
			return
		}
		p := fuzzParams[int(selector)%len(fuzzParams)]

		samples := samplesFrom(raw, p.Resolution)
		if len(samples) == 0 {
			return
		}
		// Keep the work bounded: the point is odd data, not large data.
		if len(samples) > 512 {
			samples = samples[:512]
		}

		coded, err := ldc.Compress(samples, p)
		if err != nil {
			t.Fatalf("Compress of %d valid samples failed: %v", len(samples), err)
		}

		back, err := ldc.DecompressCount(coded, p, len(samples))
		if err != nil {
			t.Fatalf("DecompressCount failed on our own output: %v", err)
		}
		if len(back) != len(samples) {
			t.Fatalf("got %d samples back, want %d", len(back), len(samples))
		}
		for i := range samples {
			if back[i] != samples[i] {
				t.Fatalf("sample %d: %d became %d", i, samples[i], back[i])
			}
		}

		// The file form must agree, and must recover the count on its own.
		file, err := ldc.CompressFile(samples, p, 1)
		if err != nil {
			t.Fatalf("CompressFile failed: %v", err)
		}
		fromFile, err := ldc.DecompressFile(file)
		if err != nil {
			t.Fatalf("DecompressFile failed on our own output: %v", err)
		}
		if len(fromFile) != len(samples) {
			t.Fatalf("the file gave %d samples, want %d", len(fromFile), len(samples))
		}
		for i := range samples {
			if fromFile[i] != samples[i] {
				t.Fatalf("file sample %d: %d became %d", i, samples[i], fromFile[i])
			}
		}
	})
}

// samplesFrom turns raw fuzz bytes into samples that fit the resolution.
func samplesFrom(raw []byte, resolution uint) []uint32 {
	width := int(resolution+7) / 8
	count := len(raw) / width
	if count == 0 {
		return nil
	}

	samples := make([]uint32, count)
	for i := range samples {
		chunk := raw[i*width : (i+1)*width]
		var v uint32
		for _, b := range chunk {
			v = v<<8 | uint32(b)
		}
		if resolution < 32 {
			v &= (1 << resolution) - 1
		}
		samples[i] = v
	}
	return samples
}
