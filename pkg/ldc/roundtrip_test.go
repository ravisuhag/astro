package ldc_test

import (
	"math/rand"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

func TestCompressRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		p       ldc.Params
		samples []uint32
	}{
		{"constant data, 8-bit", ldc.Params{
			BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4,
		}, repeat(100, 64)},

		{"ramp, 8-bit", ldc.Params{
			BlockSize: 16, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
		}, ramp(64, 8)},

		{"noise, 8-bit", ldc.Params{
			BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4,
		}, noise(64, 8, 1)},

		{"noise, 12-bit bypass", ldc.Params{
			BlockSize: 32, Resolution: 12, Predictor: ldc.PredictorBypass, ReferenceInterval: 2,
		}, noise(128, 12, 2)},

		{"noise, 16-bit no preprocessor", ldc.Params{
			BlockSize: 8, Resolution: 16, Predictor: ldc.PredictorNone, ReferenceInterval: 8,
		}, noise(64, 16, 3)},

		{"noise, 32-bit", ldc.Params{
			BlockSize: 8, Resolution: 32, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1,
		}, noise(32, 32, 4)},

		{"low entropy, 8-bit", ldc.Params{
			BlockSize: 16, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4,
		}, lowEntropy(128, 5)},

		{"restricted set, 4-bit", ldc.Params{
			BlockSize: 8, Resolution: 4, Restricted: true,
			Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
		}, noise(64, 4, 6)},

		{"restricted set, 2-bit", ldc.Params{
			BlockSize: 8, Resolution: 2, Restricted: true,
			Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
		}, noise(64, 2, 7)},

		{"signed 8-bit", ldc.Params{
			BlockSize: 8, Resolution: 8, Signed: true,
			Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4,
		}, noise(64, 8, 8)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			coded, err := ldc.Compress(test.samples, test.p)
			if err != nil {
				t.Fatalf("Compress() = %v", err)
			}

			back, err := ldc.DecompressCount(coded, test.p, len(test.samples))
			if err != nil {
				t.Fatalf("DecompressCount() = %v", err)
			}
			if len(back) != len(test.samples) {
				t.Fatalf("got %d samples, want %d", len(back), len(test.samples))
			}
			for i := range test.samples {
				if back[i] != test.samples[i] {
					t.Fatalf("sample %d: %d became %d", i, test.samples[i], back[i])
				}
			}

			// The unbounded form must agree, since these are whole blocks.
			greedy, err := ldc.Decompress(coded, test.p)
			if err != nil {
				t.Fatalf("Decompress() = %v", err)
			}
			if len(greedy) != len(test.samples) {
				t.Errorf("greedy decode gave %d samples, want %d", len(greedy), len(test.samples))
			}
		})
	}
}

// repeat builds n copies of one value: the case the zero-block option exists
// for, since a unit-delay predictor turns constant data into all zeros.
func repeat(value uint32, n int) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = value
	}
	return out
}

// ramp counts up, wrapping at the resolution.
func ramp(n int, resolution uint) []uint32 {
	out := make([]uint32, n)
	limit := uint32(1) << resolution
	for i := range out {
		out[i] = uint32(i) % limit
	}
	return out
}

// noise builds uniformly random samples, which is the case no option can
// compress and no-compression should win.
func noise(n int, resolution uint, seed int64) []uint32 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]uint32, n)
	for i := range out {
		if resolution >= 32 {
			out[i] = rng.Uint32()
			continue
		}
		out[i] = uint32(rng.Intn(1 << resolution))
	}
	return out
}

// lowEntropy builds mostly-constant data with occasional steps, which is what
// the second-extension option is for.
func lowEntropy(n int, seed int64) []uint32 {
	rng := rand.New(rand.NewSource(seed))
	out := make([]uint32, n)
	value := uint32(128)
	for i := range out {
		if rng.Intn(8) == 0 {
			value = uint32(rng.Intn(256))
		}
		out[i] = value
	}
	return out
}
