package ldc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// TestPreprocessGreenBookExample transcribes the worked table of
// CCSDS 120.0-G-4 §3.3.3, which runs a unit-delay predictor and the mapper
// over 8-bit samples in the range 0 to 255:
//
//	Sample  Predictor   Delta   theta   delta
//	  101       —         —       —       —     (reference)
//	  101      101        0      101      0
//	  100      101       -1      101      1
//	  101      100        1      100      2
//	   99      101       -2      101      3
//	  101       99        2       99      4
//	  223      101      122      101    223
//	  100      223     -123       32    155
//
// The last two rows are the ones worth having: they fall past theta, where
// the mapping stops interleaving signs and runs straight on. An implementation
// that got only the interleaved branch right would pass every other row.
func TestPreprocessGreenBookExample(t *testing.T) {
	samples := []uint32{101, 101, 100, 101, 99, 101, 223, 100}
	// The first sample is a reference, so its mapped value is zero.
	want := []uint32{0, 0, 1, 2, 3, 4, 223, 155}

	p := ldc.Params{
		BlockSize:         8,
		Resolution:        8,
		Predictor:         ldc.PredictorUnitDelay,
		ReferenceInterval: 1,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}

	got := ldc.Preprocess(samples, p)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample %d (value %d): mapped to %d, want %d",
				i, samples[i], got[i], want[i])
		}
	}
}

// TestPreprocessRoundTrip checks the preprocessor inverts exactly, which is
// what makes the whole standard lossless.
func TestPreprocessRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		p       ldc.Params
		samples []uint32
	}{
		{"8-bit unsigned, unit delay", ldc.Params{
			BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
		}, []uint32{0, 255, 128, 127, 1, 254, 3, 200, 200, 0, 0, 255, 17, 42, 99, 100}},

		{"8-bit signed, unit delay", ldc.Params{
			BlockSize: 8, Resolution: 8, Signed: true,
			Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
		}, []uint32{0x80, 0x7F, 0x00, 0xFF, 0x01, 0x80, 0x7F, 0x40}},

		{"12-bit unsigned, bypass", ldc.Params{
			BlockSize: 8, Resolution: 12, Predictor: ldc.PredictorBypass, ReferenceInterval: 4,
		}, []uint32{0, 4095, 2048, 1, 4094, 100, 200, 300}},

		{"16-bit unsigned, no preprocessor", ldc.Params{
			BlockSize: 8, Resolution: 16, Predictor: ldc.PredictorNone, ReferenceInterval: 1,
		}, []uint32{0, 65535, 32768, 1, 2, 3, 4, 5}},

		{"32-bit unsigned, unit delay", ldc.Params{
			BlockSize: 8, Resolution: 32, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1,
		}, []uint32{0, 4294967295, 2147483648, 1, 4294967294, 7, 8, 9}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.p.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}

			mapped := ldc.Preprocess(test.samples, test.p)

			// Every mapped value must fit the resolution, which is the
			// mapper's whole purpose: an (n+1)-bit signed error folded into
			// n bits.
			for i, m := range mapped {
				if test.p.Resolution < 32 && uint64(m) >= 1<<test.p.Resolution {
					t.Errorf("mapped sample %d is %d, which does not fit %d bits",
						i, m, test.p.Resolution)
				}
			}

			references := map[int]uint32{}
			for i := range test.samples {
				if test.p.Predictor == ldc.PredictorUnitDelay &&
					i%(test.p.BlockSize*test.p.ReferenceInterval) == 0 {
					references[i] = test.samples[i]
				}
			}

			back := ldc.Unpreprocess(mapped, references, test.p)
			for i := range test.samples {
				if back[i] != test.samples[i] {
					t.Errorf("sample %d: %d -> %d -> %d", i, test.samples[i], mapped[i], back[i])
				}
			}
		})
	}
}

// TestMapperIsABijection checks exhaustively at low resolution that the mapper
// loses nothing: every input value comes back, and no two map to one.
func TestMapperIsABijection(t *testing.T) {
	for _, resolution := range []uint{1, 2, 3, 4, 6, 8} {
		for _, signed := range []bool{false, true} {
			p := ldc.Params{
				BlockSize: 8, Resolution: resolution, Signed: signed,
				Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1024,
			}
			if err := p.Validate(); err != nil {
				continue
			}

			count := 1 << resolution
			// Sweep every predictor value by feeding a pair of samples, and
			// every sample value against it.
			for predictor := range count {
				// Every sample predicts from the one before it, so to hold
				// the predictor fixed while sweeping the value, alternate
				// the two.
				pairs := []uint32{uint32(predictor)}
				for value := range count {
					pairs = append(pairs, uint32(value), uint32(predictor))
				}

				mapped := ldc.Preprocess(pairs, p)
				references := map[int]uint32{0: pairs[0]}
				back := ldc.Unpreprocess(mapped, references, p)

				for i := range pairs {
					if back[i] != pairs[i] {
						t.Fatalf("n=%d signed=%v predictor=%d: sample %d, %d -> %d -> %d",
							resolution, signed, predictor, i, pairs[i], mapped[i], back[i])
					}
				}
			}
		}
	}
}

// TestReferenceSampleHasZeroError pins §4.2.5: the first sample of a reference
// interval predicts itself, so its prediction error is zero.
func TestReferenceSampleHasZeroError(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1,
	}
	samples := make([]uint32, 24)
	for i := range samples {
		samples[i] = uint32(i * 7 % 256)
	}

	mapped := ldc.Preprocess(samples, p)
	// A reference falls every BlockSize*ReferenceInterval = 8 samples.
	for _, i := range []int{0, 8, 16} {
		if mapped[i] != 0 {
			t.Errorf("reference sample at %d mapped to %d, want 0", i, mapped[i])
		}
	}
}

// TestBypassPredictorNeedsNoReferences pins §4.2.6: reference samples are
// employed only with a predictor that looks at previous samples.
func TestBypassPredictorNeedsNoReferences(t *testing.T) {
	if ldc.PredictorBypass.NeedsReferenceSamples() {
		t.Error("the bypass predictor asked for reference samples")
	}
	if ldc.PredictorNone.NeedsReferenceSamples() {
		t.Error("an absent preprocessor asked for reference samples")
	}
	if !ldc.PredictorUnitDelay.NeedsReferenceSamples() {
		t.Error("the unit-delay predictor did not ask for reference samples")
	}
}
