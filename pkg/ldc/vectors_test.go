package ldc_test

// Official CCSDS 121.0-B-2 test vectors, published by the CCSDS SLS Data
// Compression working group (cwe.ccsds.org) and mirrored in libaec's
// data/121B2TestData. Each .dat file holds uncompressed samples stored
// LSB-first in the smallest byte width that fits the resolution; each .rz
// file is the expected coded bit stream for one parameter set.
//
// Every vector here must encode byte-identically and decode back to the
// exact samples. The ExtendedParameters set is not vendored: its streams
// use per-reference-interval byte alignment, which this package does not
// implement (see docs/pics/ldc-pics.md).

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

func vectorParams(n uint, refInterval int, restricted bool) ldc.Params {
	return ldc.Params{
		BlockSize:         16,
		Resolution:        n,
		Predictor:         ldc.PredictorUnitDelay,
		ReferenceInterval: refInterval,
		Restricted:        restricted,
	}
}

func readVectorSamples(t *testing.T, path string, n uint) []uint32 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bps := 1
	if n > 8 {
		bps = 2
	}
	if n > 16 {
		bps = 4
	}
	samples := make([]uint32, len(raw)/bps)
	for i := range samples {
		var v uint32
		for b := range bps {
			v |= uint32(raw[i*bps+b]) << (8 * b)
		}
		samples[i] = v
	}
	return samples
}

func runVector(t *testing.T, datPath, rzPath string, p ldc.Params) {
	t.Helper()
	samples := readVectorSamples(t, datPath, p.Resolution)
	want, err := os.ReadFile(rzPath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("encoded stream differs from official vector %s (got %d bytes, want %d)",
			filepath.Base(rzPath), len(got), len(want))
	}

	dec, err := ldc.DecompressCount(want, p, len(samples))
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	for i := range samples {
		if dec[i] != samples[i] {
			t.Fatalf("decoded sample %d = %d, want %d", i, dec[i], samples[i])
		}
	}
}

func TestVectors_AllOptions(t *testing.T) {
	dir := filepath.Join("testdata", "121B2TestData", "AllOptions")

	for n := uint(1); n <= 4; n++ {
		dat := filepath.Join(dir, fmt.Sprintf("test_p256n%02d.dat", n))
		t.Run(fmt.Sprintf("n%02d_basic", n), func(t *testing.T) {
			runVector(t, dat, filepath.Join(dir, fmt.Sprintf("test_p256n%02d-basic.rz", n)),
				vectorParams(n, 16, false))
		})
		t.Run(fmt.Sprintf("n%02d_restricted", n), func(t *testing.T) {
			runVector(t, dat, filepath.Join(dir, fmt.Sprintf("test_p256n%02d-restricted.rz", n)),
				vectorParams(n, 16, true))
		})
	}
	for n := uint(5); n <= 16; n++ {
		t.Run(fmt.Sprintf("n%02d", n), func(t *testing.T) {
			runVector(t,
				filepath.Join(dir, fmt.Sprintf("test_p256n%02d.dat", n)),
				filepath.Join(dir, fmt.Sprintf("test_p256n%02d.rz", n)),
				vectorParams(n, 16, false))
		})
	}
	for n := uint(17); n <= 32; n++ {
		t.Run(fmt.Sprintf("n%02d", n), func(t *testing.T) {
			runVector(t,
				filepath.Join(dir, fmt.Sprintf("test_p512n%02d.dat", n)),
				filepath.Join(dir, fmt.Sprintf("test_p512n%02d.rz", n)),
				vectorParams(n, 32, false))
		})
	}
}

func TestVectors_LowEntropyOptions(t *testing.T) {
	dir := filepath.Join("testdata", "121B2TestData", "LowEntropyOptions")

	for set := 1; set <= 3; set++ {
		dat := filepath.Join(dir, fmt.Sprintf("Lowset%d_8bit.dat", set))
		for n := uint(1); n <= 4; n++ {
			t.Run(fmt.Sprintf("set%d_n%02d_basic", set, n), func(t *testing.T) {
				runVector(t, dat,
					filepath.Join(dir, fmt.Sprintf("Lowset%d_8bit.n%02d-basic.rz", set, n)),
					vectorParams(n, 64, false))
			})
			t.Run(fmt.Sprintf("set%d_n%02d_restricted", set, n), func(t *testing.T) {
				runVector(t, dat,
					filepath.Join(dir, fmt.Sprintf("Lowset%d_8bit.n%02d-restricted.rz", set, n)),
					vectorParams(n, 64, true))
			})
		}
		for n := uint(5); n <= 8; n++ {
			t.Run(fmt.Sprintf("set%d_n%02d", set, n), func(t *testing.T) {
				runVector(t, dat,
					filepath.Join(dir, fmt.Sprintf("Lowset%d_8bit.n%02d.rz", set, n)),
					vectorParams(n, 64, false))
			})
		}
	}
}
