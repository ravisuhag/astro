package ldc

// The preprocessor, per CCSDS 121.0-B-3 section 4.
//
// The entropy coder wants small non-negative integers. Raw telemetry is
// neither: it is correlated, and it may be signed. The preprocessor fixes
// both, in two steps that undo exactly.
//
//	prediction   subtract what the previous sample suggests this one will be,
//	             leaving a residual near zero
//	mapping      fold that signed residual onto the non-negative integers,
//	             so that small residuals of either sign become small values
//
// Both steps are integer arithmetic and both are reversible, which is what
// makes the whole standard lossless. §4.2.3 also allows a bypass predictor
// that predicts zero, keeping the mapper for data that is already
// decorrelated but may be signed.

// Predictor says how the preprocessor predicts each sample.
type Predictor int

const (
	// PredictorNone means no preprocessor at all: samples go to the entropy
	// coder as they are. §4.1 allows this when the data is already suitable.
	PredictorNone Predictor = iota
	// PredictorUnitDelay predicts each sample from the one before, per §4.2.5.
	// This is the only predictor the standard specifies.
	PredictorUnitDelay
	// PredictorBypass predicts zero and keeps the mapper, per §4.2.3.
	PredictorBypass
)

// String names the predictor.
func (p Predictor) String() string {
	switch p {
	case PredictorUnitDelay:
		return "unit delay"
	case PredictorBypass:
		return "bypass"
	default:
		return "none"
	}
}

// NeedsReferenceSamples reports whether the decoder needs uncoded reference
// samples to invert the preprocessing.
//
// §4.2.6: reference samples are required "when, and only when, a Unit-Delay
// Predictor or other higher-order predictor that bases its predictions on
// previous sample values is used. Otherwise, reference samples shall not be
// employed." So the bypass predictor, which looks at nothing, needs none.
func (p Predictor) NeedsReferenceSamples() bool {
	return p == PredictorUnitDelay
}

// sampleRange returns the smallest and largest values a sample may take, per
// §4.4.
//
// Signed samples are two's complement in the range [-2^(n-1), 2^(n-1)-1];
// unsigned run from 0 to 2^n-1. The values are int64 because the mapper
// subtracts them and an n-bit difference needs n+1 bits.
func sampleRange(resolution uint, signed bool) (minimum, maximum int64) {
	if signed {
		return -(1 << (resolution - 1)), (1 << (resolution - 1)) - 1
	}
	return 0, (1 << resolution) - 1
}

// mapError folds a signed prediction error onto a non-negative integer, per
// the equation of §4.4:
//
//	        2*d              for 0 <= d <= theta
//	delta = 2*|d| - 1        for -theta <= d < 0
//	        theta + |d|      otherwise
//
//	theta = min(predicted - xmin, xmax - predicted)
//
// theta is how far the prediction sits from the nearer end of the sample
// range. Within that distance the error can go either way, so the mapping
// interleaves positives and negatives. Past it only one sign is possible, so
// the mapping runs straight on without interleaving — which is what keeps the
// result inside n bits instead of spilling into n+1.
func mapError(delta, predicted, minimum, maximum int64) uint32 {
	theta := predicted - minimum
	if other := maximum - predicted; other < theta {
		theta = other
	}

	switch {
	case delta >= 0 && delta <= theta:
		return uint32(2 * delta)
	case delta < 0 && delta >= -theta:
		return uint32(-2*delta - 1)
	default:
		if delta < 0 {
			return uint32(theta - delta)
		}
		return uint32(theta + delta)
	}
}

// unmapError is the exact inverse of mapError.
//
// Which branch produced a value is recoverable from the value itself: below
// 2*theta the mapping interleaved, so even values came from positives and odd
// from negatives. At or above 2*theta it ran straight on, and the sign is
// whichever direction had room left.
func unmapError(mapped uint32, predicted, minimum, maximum int64) int64 {
	theta := predicted - minimum
	if other := maximum - predicted; other < theta {
		theta = other
	}

	value := int64(mapped)
	if value <= 2*theta {
		if value%2 == 0 {
			return value / 2
		}
		return -(value + 1) / 2
	}

	// Past the interleaved region. Only the side with more room can still
	// produce errors, so the sign follows from which end theta came from.
	if predicted-minimum < maximum-predicted {
		// Room below is what ran out, so the error must be positive.
		return value - theta
	}
	return theta - value
}

// Preprocess turns input samples into the non-negative values the entropy
// coder takes.
//
// blockSize and referenceInterval decide where reference samples fall: the
// first sample of every reference interval is a reference, and §4.2.5 gives
// it a predicted value equal to itself, so its prediction error is zero. The
// returned slice has one entry per input sample; the reference positions hold
// zero and the caller emits the raw sample instead.
func Preprocess(samples []uint32, p Params) []uint32 {
	mapped := make([]uint32, len(samples))
	if p.Predictor == PredictorNone {
		copy(mapped, samples)
		return mapped
	}

	minimum, maximum := sampleRange(p.Resolution, p.Signed)
	samplesPerInterval := p.BlockSize * p.ReferenceInterval

	var previous int64
	for i, raw := range samples {
		value := p.signedValue(raw)

		var predicted int64
		switch {
		case p.Predictor == PredictorBypass:
			// §4.2.3: predict zero, keep the mapper.
			predicted = 0
		case i%samplesPerInterval == 0:
			// §4.2.5: the first sample of a reference interval predicts
			// itself, so the error is zero. The raw sample travels uncoded in
			// the CDS, and the decoder starts from it.
			predicted = value
		default:
			predicted = previous
		}

		mapped[i] = mapError(value-predicted, predicted, minimum, maximum)
		previous = value
	}
	return mapped
}

// Unpreprocess inverts Preprocess.
//
// references supplies the raw value at each reference position, which the
// decoder read uncoded from the stream. Without them a unit-delay chain has
// no starting point.
func Unpreprocess(mapped []uint32, references map[int]uint32, p Params) []uint32 {
	out := make([]uint32, len(mapped))
	if p.Predictor == PredictorNone {
		copy(out, mapped)
		return out
	}

	minimum, maximum := sampleRange(p.Resolution, p.Signed)
	samplesPerInterval := p.BlockSize * p.ReferenceInterval

	var previous int64
	for i, m := range mapped {
		isReference := p.Predictor == PredictorUnitDelay && i%samplesPerInterval == 0

		if isReference {
			// The raw sample was carried uncoded; nothing to invert.
			raw := references[i]
			out[i] = raw
			previous = p.signedValue(raw)
			continue
		}

		var predicted int64
		if p.Predictor != PredictorBypass {
			predicted = previous
		}

		delta := unmapError(m, predicted, minimum, maximum)
		value := predicted + delta
		out[i] = p.rawValue(value)
		previous = value
	}
	return out
}

// signedValue reinterprets a stored sample as the signed integer it stands
// for, when the data sense says the samples are two's complement.
func (p Params) signedValue(raw uint32) int64 {
	if !p.Signed {
		return int64(raw)
	}
	// Sign extend from the configured resolution.
	shift := uint(64 - p.Resolution)
	return int64(uint64(raw)<<shift) >> shift
}

// rawValue is the inverse of signedValue: it packs a signed sample back into
// the low bits of a uint32.
func (p Params) rawValue(value int64) uint32 {
	if p.Resolution == 32 {
		return uint32(value)
	}
	return uint32(uint64(value) & ((1 << p.Resolution) - 1))
}

// isReferencePosition reports whether sample index i carries a reference
// sample.
func (p Params) isReferencePosition(i int) bool {
	if !p.Predictor.NeedsReferenceSamples() {
		return false
	}
	return i%(p.BlockSize*p.ReferenceInterval) == 0
}
