package ldc_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// TestNoCompressionWinsOnIncompressibleData checks the arithmetic of §3.7.3 on
// a block chosen so the answer can be worked out by hand.
//
// Eight 8-bit samples with no preprocessor, all at 255. The options cost:
//
//	no compression   3 + 8*8                 = 67 bits
//	FS (k=0)         3 + 8*(255+1)           = 2051 bits
//	k=5              3 + 8*(255>>5 + 1) + 40 = 3 + 8*8 + 40 = 107 bits
//	second extension 4 + 4 FS codewords of gamma(255,255) = huge
//
// So no compression wins, and the coded data set is 67 bits.
func TestNoCompressionWinsOnIncompressibleData(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorNone, ReferenceInterval: 1,
	}
	samples := repeat(255, 8)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}

	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("%d coded data sets, want 1", len(infos))
	}
	if infos[0].Option != ldc.OptionNoCompression {
		t.Errorf("chose %v, want no compression", infos[0].Option)
	}
	if infos[0].Bits != 67 {
		t.Errorf("coded data set is %d bits, want 67 (3 identifier + 64 samples)", infos[0].Bits)
	}
}

// TestFSWinsOnSmallValues is the opposite end. Eight 8-bit samples all zero
// would be a zero block, so use ones:
//
//	FS (k=0)         3 + 8*(1+1)  = 19 bits
//	no compression   3 + 8*8      = 67 bits
//	k=1              3 + 8*1 + 8  = 19 bits
//	second extension 4 + 4*(gamma(1,1)+1) = 4 + 4*5 = 24 bits
//
// FS and k=1 tie at 19. §3.7.4c breaks the tie towards the smaller k, so FS
// wins.
func TestFSWinsAndTieBreaksToSmallestK(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorNone, ReferenceInterval: 1,
	}
	samples := repeat(1, 8)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}

	if infos[0].Option != ldc.OptionSplitSample {
		t.Fatalf("chose %v, want a split-sample option", infos[0].Option)
	}
	if infos[0].K != 0 {
		t.Errorf("chose k=%d; §3.7.4c breaks a tie towards the smallest k, so k=0", infos[0].K)
	}
	if infos[0].Bits != 19 {
		t.Errorf("coded data set is %d bits, want 19 (3 identifier + 16 FS)", infos[0].Bits)
	}
}

// TestSecondExtensionBeatsFSOnPairs uses a block where pairing pays.
//
// Eight samples alternating 0 and 1:
//
//	FS (k=0)         3 + 4*(0+1) + 4*(1+1)   = 15 bits
//	second extension 4 + 4*(gamma(0,1)+1) = 4 + 4*3 = 16 bits
//
// FS still wins there, so this checks the coder does not reach for the
// low-entropy option when a plainer one is shorter.
func TestSecondExtensionNotChosenWhenFSIsShorter(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorNone, ReferenceInterval: 1,
	}
	samples := []uint32{0, 1, 0, 1, 0, 1, 0, 1}

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}
	if infos[0].Option != ldc.OptionSplitSample || infos[0].K != 0 {
		t.Errorf("chose %v k=%d, want FS", infos[0].Option, infos[0].K)
	}
	if infos[0].Bits != 15 {
		t.Errorf("coded data set is %d bits, want 15", infos[0].Bits)
	}
}

// TestZeroBlockAlwaysWins pins §3.7.2: an all-zero block takes the zero-block
// option whatever else would cost, and one coded data set covers the whole
// run.
func TestZeroBlockAlwaysWins(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4,
	}
	// Constant input becomes all zeros after the unit-delay predictor.
	samples := repeat(77, 32)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("%d coded data sets, want 1 covering all four blocks", len(infos))
	}
	if infos[0].Option != ldc.OptionZeroBlock {
		t.Fatalf("chose %v, want zero block", infos[0].Option)
	}
	if infos[0].ZeroRun != 4 {
		t.Errorf("the run covers %d blocks, want 4", infos[0].ZeroRun)
	}
	if !infos[0].HasReference {
		t.Error("the first block should carry a reference sample")
	}
	// Identifier 4 bits, reference 8 bits, run-of-4 codeword "0001" 4 bits.
	if infos[0].Bits != 16 {
		t.Errorf("coded data set is %d bits, want 16 (4 id + 8 reference + 4 run)", infos[0].Bits)
	}
}

// TestRemainderOfSegmentIsUsed exercises the codeword displaced between four
// and five in table 3-2. A segment is 64 blocks (§3.5.2), so a reference
// interval of at least 64 blocks of zeros reaches the end of one.
func TestRemainderOfSegmentIsUsed(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 64,
	}
	// 64 blocks of 8 samples, all the same value.
	samples := repeat(200, 64*8)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("%d coded data sets, want 1", len(infos))
	}
	if !infos[0].IsROS {
		t.Errorf("a run to the end of a 64-block segment should use the ROS codeword; got run=%d",
			infos[0].ZeroRun)
	}
	// Identifier 4, reference 8, ROS codeword "00001" 5 bits.
	if infos[0].Bits != 17 {
		t.Errorf("coded data set is %d bits, want 17 (4 id + 8 reference + 5 ROS)", infos[0].Bits)
	}

	back, err := ldc.DecompressCount(coded, p, len(samples))
	if err != nil {
		t.Fatalf("DecompressCount() = %v", err)
	}
	for i := range samples {
		if back[i] != samples[i] {
			t.Fatalf("sample %d: %d became %d", i, samples[i], back[i])
		}
	}
}

// TestZeroRunStopsAtSixtyThree checks table 3-2's last row: a longer run needs
// a second codeword, because the table stops at 63.
func TestZeroRunStopsAtSixtyThree(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 4096,
	}
	// 100 blocks of zeros, which is more than one segment of 64.
	samples := repeat(9, 100*8)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}

	// Segments are 64 blocks, so 100 blocks is a full segment then 36 more.
	if len(infos) != 2 {
		t.Fatalf("%d coded data sets, want 2 (one per segment)", len(infos))
	}
	if infos[0].ZeroRun != 64 || !infos[0].IsROS {
		t.Errorf("first set covers %d blocks (ROS %v), want 64 with ROS",
			infos[0].ZeroRun, infos[0].IsROS)
	}
	if infos[1].Block != 64 || infos[1].ZeroRun != 36 {
		t.Errorf("second set starts at block %d covering %d, want 64 and 36",
			infos[1].Block, infos[1].ZeroRun)
	}

	back, err := ldc.DecompressCount(coded, p, len(samples))
	if err != nil {
		t.Fatalf("DecompressCount() = %v", err)
	}
	for i := range samples {
		if back[i] != samples[i] {
			t.Fatalf("sample %d: %d became %d", i, samples[i], back[i])
		}
	}
}

// TestZeroRunDoesNotCrossAReferenceInterval checks §3.5.2's other boundary: a
// run is bounded by the reference interval as well as by the segment, because
// the next interval starts with an uncoded reference sample.
func TestZeroRunDoesNotCrossAReferenceInterval(t *testing.T) {
	p := ldc.Params{
		BlockSize: 8, Resolution: 8,
		Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 2,
	}
	samples := repeat(5, 8*8) // eight blocks, four reference intervals

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}
	infos, err := ldc.Analyze(coded, p, len(samples))
	if err != nil {
		t.Fatalf("Analyze() = %v", err)
	}

	if len(infos) != 4 {
		t.Fatalf("%d coded data sets, want 4, one per reference interval", len(infos))
	}
	for i, info := range infos {
		if info.ZeroRun != 2 {
			t.Errorf("set %d covers %d blocks, want 2", i, info.ZeroRun)
		}
		if !info.HasReference {
			t.Errorf("set %d should start a reference interval", i)
		}
	}
}

// TestCompressionActuallyCompresses is the sanity check the whole package
// exists for.
func TestCompressionActuallyCompresses(t *testing.T) {
	p := ldc.DefaultParams()
	samples := lowEntropy(4096, 11)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}

	raw := len(samples) // one octet per 8-bit sample
	if len(coded) >= raw {
		t.Errorf("coded %d octets from %d raw; that is not compression", len(coded), raw)
	}
	t.Logf("%d octets from %d raw, ratio %.2f", len(coded), raw, float64(raw)/float64(len(coded)))
}

// TestNoiseDoesNotExpandMuch checks the other direction: incompressible data
// costs only the option identifiers.
func TestNoiseDoesNotExpandMuch(t *testing.T) {
	p := ldc.Params{
		BlockSize: 16, Resolution: 8,
		Predictor: ldc.PredictorNone, ReferenceInterval: 64,
	}
	samples := noise(1024, 8, 12)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatalf("Compress() = %v", err)
	}

	// 64 blocks, each paying a 3-bit identifier at worst: 192 bits = 24 octets.
	raw := len(samples)
	if overhead := len(coded) - raw; overhead > 24 {
		t.Errorf("coded %d octets from %d raw, overhead %d; want no more than 24",
			len(coded), raw, overhead)
	}
}
