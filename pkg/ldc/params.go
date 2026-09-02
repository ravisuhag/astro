// Package ldc implements CCSDS Lossless Data Compression per CCSDS 121.0-B-3,
// the Rice adaptive entropy coder.
//
// This is the most widely used CCSDS compression standard, and the shape of it
// is simple: decorrelate, then entropy code.
//
//	samples ──► preprocessor ──► adaptive entropy coder ──► coded data sets
//	              §4              §3                         §5
//
// The preprocessor subtracts a prediction from each sample and folds the
// signed residual onto the non-negative integers. The entropy coder then takes
// the residuals in blocks, prices every code option it has against the block,
// and writes the cheapest with an identifier saying which it chose. Nothing
// here is approximate: it is integer arithmetic throughout, and every step
// inverts exactly.
//
// # Using it
//
//	p := ldc.DefaultParams()
//	p.Resolution = 12
//
//	file, err := ldc.CompressFile(samples, p, 1)
//	back, err := ldc.DecompressFile(file)
//
// CompressFile writes the file format of section 7, whose header carries the
// parameters and the sample count. Compress and Decompress are the barer pair
// for callers who already share a configuration — a mission putting coded data
// sets straight into space packets, say, where §5.3 leaves the framing to the
// packetizer.
//
// # Choosing parameters
//
// Resolution must match the data. Everything else is a trade:
//
//	BlockSize          smaller blocks adapt faster to changing statistics and
//	                   pay more identifier bits; 16 is a common choice
//	Predictor          unit delay for correlated data, bypass for data already
//	                   decorrelated but signed, none for neither
//	ReferenceInterval  how often an uncoded sample is inserted, bounding how
//	                   far a bit error can propagate
//
// # What is here and what is not
//
// All five code options are implemented, encoder and decoder: fundamental
// sequence, the split-sample family, second extension, zero block, and no
// compression. So are both predictors the standard specifies, the file format,
// and the restricted option set for four-bit and narrower samples.
//
// Not here: the compression identification packet of section 6, insertion into
// space packets (§5.3, which the caller does), and the application-specific
// predictor and mapper the standard names but does not define. See
// docs/content/protocols/ldc/conformance.md.
package ldc

import "fmt"

// Params holds the compression parameters CCSDS 121.0-B-3 leaves to the user.
//
// None of these travel with a coded data set, so a decoder must be told them.
// That is why the standard defines a file header (§7.2.2) and an optional
// compression identification packet (§6): both exist to carry this struct's
// worth of information alongside the data.
type Params struct {
	// BlockSize is J, the number of samples the coder treats as one block.
	// §3.1.6 allows 8, 16, 32 and 64.
	BlockSize int

	// Resolution is n, the number of bits per input sample, 1 to 32 (§3.1.6).
	Resolution uint

	// Signed says whether samples are two's complement. §4.4 gives the sample
	// range either as [-2^(n-1), 2^(n-1)-1] or [0, 2^n-1], and the mapper
	// needs to know which.
	//
	// Table 7-1's Data Sense field requires this to be false when the
	// preprocessor is absent or bypassed.
	Signed bool

	// Predictor selects the preprocessing, per section 4.
	Predictor Predictor

	// ReferenceInterval is r, the number of blocks between reference samples,
	// 1 to 4096 (§4.3).
	//
	// It matters even without reference samples: §3.5.2 uses it to bound the
	// segments the zero-block option counts within.
	ReferenceInterval int

	// Restricted selects the restricted set of code options. §5.2.1.1 allows
	// it only when the resolution is 4 bits or fewer, where it buys shorter
	// option identifiers at the cost of dropping most of the split-sample
	// options.
	Restricted bool
}

// DefaultParams returns a workable starting point: 8-bit unsigned samples,
// blocks of 16, unit-delay prediction, a reference every 256 blocks.
func DefaultParams() Params {
	return Params{
		BlockSize:         16,
		Resolution:        8,
		Predictor:         PredictorUnitDelay,
		ReferenceInterval: 256,
	}
}

// Validate checks each field against the values the standard allows.
func (p Params) Validate() error {
	switch p.BlockSize {
	case 8, 16, 32, 64:
	default:
		return fmt.Errorf("%w: got %d", ErrInvalidBlockSize, p.BlockSize)
	}

	if p.Resolution < 1 || p.Resolution > 32 {
		return fmt.Errorf("%w: got %d", ErrInvalidResolution, p.Resolution)
	}

	if p.ReferenceInterval < 1 || p.ReferenceInterval > 4096 {
		return fmt.Errorf("%w: got %d", ErrInvalidReferenceInterval, p.ReferenceInterval)
	}

	if p.Restricted && p.Resolution > 4 {
		return fmt.Errorf("%w: resolution is %d", ErrRestrictedNotAllowed, p.Resolution)
	}

	if p.Signed && p.Predictor != PredictorUnitDelay {
		// Table 7-1's Data Sense field: positive is "mandatory if
		// preprocessor is bypassed or preprocessor absent".
		return fmt.Errorf("%w: signed samples need the unit-delay predictor", ErrUnsupportedPredictor)
	}

	switch p.Predictor {
	case PredictorNone, PredictorUnitDelay, PredictorBypass:
	default:
		return fmt.Errorf("%w: %d", ErrUnsupportedPredictor, int(p.Predictor))
	}

	return nil
}

// idWidth returns the width in bits of the base option identifier, from the
// resolution columns of table 5-1.
//
// The table looks like five unrelated columns of codes, but it is one rule:
// an identifier of w bits, where an all-ones value means no compression, a
// value of k+1 means the split-sample option k, and a leading zero run
// escapes into one extra bit that distinguishes the zero-block option from
// the second-extension option.
func (p Params) idWidth() int {
	if p.Restricted {
		// §5.2.1.1, the two restricted columns.
		if p.Resolution <= 2 {
			return 1
		}
		return 2
	}
	switch {
	case p.Resolution <= 8:
		return 3
	case p.Resolution <= 16:
		return 4
	default:
		return 5
	}
}

// maxK returns the largest split-sample parameter table 5-1 gives an
// identifier to.
//
// The identifier for option k is k+1, and all-ones is taken by no
// compression, so the largest usable value is 2^w - 2, meaning k goes up to
// 2^w - 3. That yields 5, 13 and 29 for the three basic columns and 1 for the
// restricted n=3,4 column — exactly the table. At w=1 there is no room for
// any k, nor for the FS option, which is again what the table shows.
func (p Params) maxK() int {
	return (1 << p.idWidth()) - 3
}

// Humanize returns a human-readable summary.
func (p Params) Humanize() string {
	set := "basic"
	if p.Restricted {
		set = "restricted"
	}
	sense := "unsigned"
	if p.Signed {
		sense = "signed"
	}
	return fmt.Sprintf("LDC Parameters\n"+
		"  Block size ......... %d samples\n"+
		"  Resolution ......... %d bits, %s\n"+
		"  Predictor .......... %s\n"+
		"  Reference every .... %d blocks\n"+
		"  Code option set .... %s (%d-bit identifiers, k up to %d)",
		p.BlockSize, p.Resolution, sense, p.Predictor,
		p.ReferenceInterval, set, p.idWidth(), p.maxK())
}
