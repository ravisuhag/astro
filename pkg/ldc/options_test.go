package ldc_test

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// bitString renders packed bytes as a bit string of the given length, so a
// test can compare against a literal transcribed from the spec.
func bitString(data []byte, bits int) string {
	out := make([]byte, 0, bits)
	for i := range bits {
		bit := (data[i/8] >> (7 - uint(i%8))) & 1
		out = append(out, '0'+bit)
	}
	return string(out)
}

// TestOptionIDTable transcribes table 5-1 of CCSDS 121.0-B-3 in full. It is
// the table an implementation is most likely to get subtly wrong, because the
// identifier width changes with resolution and the zero-block and
// second-extension codes are one bit longer than the rest.
func TestOptionIDTable(t *testing.T) {
	tests := []struct {
		name       string
		resolution uint
		restricted bool
		// want maps a description to its bits, exactly as table 5-1 prints
		// them.
		want map[string]string
	}{
		{"restricted n=1,2", 2, true, map[string]string{
			"zero":   "00",
			"second": "01",
			"nocomp": "1",
		}},
		{"restricted n=3,4", 4, true, map[string]string{
			"zero":   "000",
			"second": "001",
			"fs":     "01",
			"k=1":    "10",
			"nocomp": "11",
		}},
		{"basic n<=8", 8, false, map[string]string{
			"zero":   "0000",
			"second": "0001",
			"fs":     "001",
			"k=1":    "010",
			"k=2":    "011",
			"k=3":    "100",
			"k=4":    "101",
			"k=5":    "110",
			"nocomp": "111",
		}},
		{"basic 8<n<=16", 16, false, map[string]string{
			"zero":   "00000",
			"second": "00001",
			"fs":     "0001",
			"k=1":    "0010",
			"k=7":    "1000",
			"k=13":   "1110",
			"nocomp": "1111",
		}},
		{"basic 16<n<=32", 32, false, map[string]string{
			"zero":   "000000",
			"second": "000001",
			"fs":     "00001",
			"k=1":    "00010",
			"k=14":   "01111",
			"k=15":   "10000",
			"k=29":   "11110",
			"nocomp": "11111",
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := ldc.Params{
				BlockSize: 8, Resolution: test.resolution, Restricted: test.restricted,
				Predictor: ldc.PredictorNone, ReferenceInterval: 1,
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}

			for label, wantBits := range test.want {
				option, k := parseLabel(t, label)

				got := ldc.EncodeOptionIDForTest(p, option, k)
				if got != wantBits {
					t.Errorf("%s identifier = %s, want %s (table 5-1)", label, got, wantBits)
				}

				// And it must read back as what it says.
				gotOption, gotK, err := ldc.DecodeOptionIDForTest(p, wantBits)
				if err != nil {
					t.Errorf("%s: decoding %s: %v", label, wantBits, err)
					continue
				}
				if gotOption != option || (option == ldc.OptionSplitSample && gotK != k) {
					t.Errorf("%s decoded as %v k=%d, want %v k=%d", label, gotOption, gotK, option, k)
				}
			}
		})
	}
}

// parseLabel turns a table row name into an option and a k.
func parseLabel(t *testing.T, label string) (ldc.Option, int) {
	t.Helper()
	switch label {
	case "zero":
		return ldc.OptionZeroBlock, 0
	case "second":
		return ldc.OptionSecondExtension, 0
	case "nocomp":
		return ldc.OptionNoCompression, 0
	case "fs":
		return ldc.OptionSplitSample, 0
	default:
		var k int
		if _, err := fmt.Sscanf(label, "k=%d", &k); err != nil {
			t.Fatalf("bad label %q", label)
		}
		return ldc.OptionSplitSample, k
	}
}

// TestMaxKMatchesTable5_1 checks the derived bound against the last k the
// table prints in each column.
func TestMaxKMatchesTable5_1(t *testing.T) {
	tests := []struct {
		resolution uint
		restricted bool
		want       int
	}{
		{2, true, -1}, // no split-sample options at all
		{4, true, 1},
		{8, false, 5},
		{16, false, 13},
		{32, false, 29},
	}

	for _, test := range tests {
		p := ldc.Params{
			BlockSize: 8, Resolution: test.resolution, Restricted: test.restricted,
			Predictor: ldc.PredictorNone, ReferenceInterval: 1,
		}
		if got := ldc.MaxKForTest(p); got != test.want {
			t.Errorf("n=%d restricted=%v: max k = %d, want %d",
				test.resolution, test.restricted, got, test.want)
		}
	}
}

// TestFundamentalSequenceOption pins §3.2 and table 3-1: the FS option is the
// split-sample option with k=0, and a sample of value m is m zeros then a one.
func TestFundamentalSequenceOption(t *testing.T) {
	block := []uint32{0, 1, 2, 3}
	// 1 | 01 | 001 | 0001
	const want = "1010010001"

	got, bits := ldc.EncodeSplitSampleForTest(block, 0)
	if bitString(got, bits) != want {
		t.Errorf("FS coded %s, want %s", bitString(got, bits), want)
	}
	if bits != len(want) {
		t.Errorf("FS is %d bits, want %d", bits, len(want))
	}
	if length := ldc.SplitSampleLengthForTest(block, 0); length != len(want) {
		t.Errorf("computed length %d, want %d", length, len(want))
	}

	back, err := ldc.DecodeSplitSampleForTest(got, len(block), 0, 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range block {
		if back[i] != block[i] {
			t.Errorf("sample %d: %d -> %d", i, block[i], back[i])
		}
	}
}

// TestSplitSampleOption pins §3.3: the top n-k bits become FS codewords for
// the whole block first, and only then do the k low bits follow — §3.3.3 is
// explicit that they are not interleaved.
func TestSplitSampleOption(t *testing.T) {
	// Four 8-bit samples, k=2. Top six bits and low two bits:
	//   9 = 0b00001001 -> high 2, low 01
	//   4 = 0b00000100 -> high 1, low 00
	//   0 = 0b00000000 -> high 0, low 00
	//   7 = 0b00000111 -> high 1, low 11
	block := []uint32{9, 4, 0, 7}
	// FS codewords: 001 01 1 01, then split bits: 01 00 00 11
	const want = "001011" + "01" + "01000011"

	got, bits := ldc.EncodeSplitSampleForTest(block, 2)
	if bitString(got, bits) != want {
		t.Errorf("split-sample coded %s, want %s", bitString(got, bits), want)
	}
	if length := ldc.SplitSampleLengthForTest(block, 2); length != bits {
		t.Errorf("computed length %d but emitted %d bits", length, bits)
	}

	back, err := ldc.DecodeSplitSampleForTest(got, len(block), 2, 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range block {
		if back[i] != block[i] {
			t.Errorf("sample %d: %d -> %d", i, block[i], back[i])
		}
	}
}

// TestSecondExtensionTransform pins the equation of §3.4.1:
//
//	gamma_j = (d_{2j-1} + d_{2j})(d_{2j-1} + d_{2j} + 1)/2 + d_{2j}
func TestSecondExtensionTransform(t *testing.T) {
	tests := []struct {
		first, second uint32
		gamma         uint64
	}{
		{0, 0, 0},  // (0)(1)/2 + 0
		{0, 1, 2},  // (1)(2)/2 + 1
		{1, 0, 1},  // (1)(2)/2 + 0
		{1, 1, 4},  // (2)(3)/2 + 1
		{2, 0, 3},  // (2)(3)/2 + 0
		{0, 2, 5},  // (2)(3)/2 + 2
		{3, 2, 17}, // (5)(6)/2 + 2
	}

	for _, test := range tests {
		block := []uint32{test.first, test.second}
		got, ok := ldc.SecondExtensionSymbolsForTest(block)
		if !ok {
			t.Fatalf("(%d,%d) reported unusable", test.first, test.second)
		}
		if got[0] != test.gamma {
			t.Errorf("gamma(%d,%d) = %d, want %d", test.first, test.second, got[0], test.gamma)
		}
	}
}

func TestSecondExtensionRoundTrip(t *testing.T) {
	block := []uint32{0, 0, 1, 0, 0, 1, 2, 1}

	got, bits := ldc.EncodeSecondExtensionForTest(block)
	if length := ldc.SecondExtensionLengthForTest(block); length != bits {
		t.Errorf("computed length %d but emitted %d bits", length, bits)
	}

	back, err := ldc.DecodeSecondExtensionForTest(got, len(block), 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range block {
		if back[i] != block[i] {
			t.Errorf("sample %d: %d -> %d", i, block[i], back[i])
		}
	}
}

// TestSecondExtensionRefusesOverflow guards the trap in §3.4: at high
// resolution the transform can exceed what a uint64 holds, and the option must
// report itself unusable rather than wrap.
func TestSecondExtensionRefusesOverflow(t *testing.T) {
	block := []uint32{0xFFFFFFFF, 0xFFFFFFFF}

	if _, ok := ldc.SecondExtensionSymbolsForTest(block); !ok {
		// Good: the transform declined.
		if got := ldc.SecondExtensionLengthForTest(block); got != ldc.UnusableForTest() {
			t.Errorf("length of an unusable block = %d, want the unusable marker", got)
		}
		return
	}
	// If it did compute, the length must still be enormous enough never to win.
	if got := ldc.SecondExtensionLengthForTest(block); got < 1<<20 {
		t.Errorf("length of a huge block = %d, which is suspiciously small", got)
	}
}

// TestTriangularRootIsExact checks the inverse pairing over a wide sweep. It
// uses binary search rather than a square root because the standard is
// integer only.
func TestTriangularRootIsExact(t *testing.T) {
	for s := uint64(0); s < 2000; s++ {
		base := s * (s + 1) / 2
		for offset := uint64(0); offset <= s && offset < 4; offset++ {
			if got := ldc.TriangularRootForTest(base + offset); got != s {
				t.Fatalf("triangularRoot(%d) = %d, want %d", base+offset, got, s)
			}
		}
		// One below the base belongs to s-1.
		if s > 0 {
			if got := ldc.TriangularRootForTest(base - 1); got != s-1 {
				t.Fatalf("triangularRoot(%d) = %d, want %d", base-1, got, s-1)
			}
		}
	}
}

// TestTriangularRootAtBoundary is the regression for the n=31/32 overflow: the
// probes and the binary search used to compute s(s+1)/2 in bare uint64
// arithmetic, which wraps once s passes about 2^32, and the resolutions 31 and
// 32 are exactly where the second-extension decoder needs such values.
func TestTriangularRootAtBoundary(t *testing.T) {
	// s = 2^32: T(s) = 2^31 * (2^32 + 1) = 2^63 + 2^31, near the top of the
	// range where the old arithmetic still fit; s(s+1) itself is past 2^64.
	const s32 = uint64(1) << 32
	const t32 = uint64(1)<<63 + uint64(1)<<31

	tests := []struct {
		v    uint64
		want uint64
	}{
		{t32 - 1, s32 - 1},
		{t32, s32},
		{t32 + s32 - 1, s32}, // largest v still under T(s32+1)
		{t32 + s32 + 1, s32 + 1},
		// The very top of uint64. floor((sqrt(8v+1)-1)/2) for v = 2^64-1 is
		// 6074000999: T(6074000999) = 18446744070963499500 <= 2^64-1 and
		// T(6074001000) = 18446744077037500500 > 2^64-1.
		{^uint64(0), 6074000999},
		{18446744070963499500, 6074000999},
		{18446744070963499499, 6074000998},
	}
	for _, test := range tests {
		if got := ldc.TriangularRootForTest(test.v); got != test.want {
			t.Errorf("triangularRoot(%d) = %d, want %d", test.v, got, test.want)
		}
	}
}

// TestSecondExtensionAtMaxResolution decodes second-extension blocks at n=31
// and n=32, where the limit arithmetic used to wrap. The samples are small —
// large ones make the option unusable by design — but the decode path computes
// the limit from the resolution before reading a single bit, so a wrapped
// limit would corrupt even these.
func TestSecondExtensionAtMaxResolution(t *testing.T) {
	block := []uint32{0, 3, 1, 0, 2, 2, 5, 1}
	for _, resolution := range []uint{31, 32} {
		coded, _ := ldc.EncodeSecondExtensionForTest(block)
		back, err := ldc.DecodeSecondExtensionForTest(coded, len(block), resolution)
		if err != nil {
			t.Fatalf("n=%d: decode: %v", resolution, err)
		}
		for i := range block {
			if back[i] != block[i] {
				t.Errorf("n=%d: sample %d: %d -> %d", resolution, i, block[i], back[i])
			}
		}
	}
}

// TestNoCompressionOption pins §3.6: the block goes out unaltered.
func TestNoCompressionOption(t *testing.T) {
	block := []uint32{0, 1, 254, 255}
	const want = "00000000" + "00000001" + "11111110" + "11111111"

	got, bits := ldc.EncodeNoCompressionForTest(block, 8)
	if bitString(got, bits) != want {
		t.Errorf("no-compression coded %s, want %s", bitString(got, bits), want)
	}
	if length := ldc.NoCompressionLengthForTest(block, 8); length != 32 {
		t.Errorf("computed length %d, want 32", length)
	}

	back, err := ldc.DecodeNoCompressionForTest(got, len(block), 8)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range block {
		if back[i] != block[i] {
			t.Errorf("sample %d: %d -> %d", i, block[i], back[i])
		}
	}
}

// TestZeroBlockCodewords transcribes table 3-2 in full, including the
// remainder-of-segment codeword displaced between four and five — the one
// place the standard's coding is not a plain FS code over the count.
func TestZeroBlockCodewords(t *testing.T) {
	tests := []struct {
		count int
		isROS bool
		bits  string
	}{
		{1, false, "1"},
		{2, false, "01"},
		{3, false, "001"},
		{4, false, "0001"},
		{0, true, "00001"}, // ROS
		{5, false, "000001"},
		{6, false, "0000001"},
		{7, false, "00000001"},
		{8, false, "000000001"},
	}

	for _, test := range tests {
		got := ldc.EncodeZeroRunForTest(test.count, test.isROS)
		if got != test.bits {
			label := "ROS"
			if !test.isROS {
				label = fmt.Sprintf("%d blocks", test.count)
			}
			t.Errorf("%s coded %s, want %s (table 3-2)", label, got, test.bits)
		}
		if length := ldc.ZeroBlockLengthForTest(test.count, test.isROS); length != len(test.bits) {
			t.Errorf("count=%d isROS=%v: length %d, want %d",
				test.count, test.isROS, length, len(test.bits))
		}
	}

	// 63 is the last row of the table: 63 zeros and a one.
	got := ldc.EncodeZeroRunForTest(63, false)
	if len(got) != 64 || got[63] != '1' {
		t.Errorf("63 blocks coded %d bits ending %q, want 64 ending in a one", len(got), got[len(got)-1])
	}
}

// TestZeroRunCodewordsInvert checks the displaced numbering reads back.
func TestZeroRunCodewordsInvert(t *testing.T) {
	for count := 1; count <= 63; count++ {
		value := ldc.ZeroRunFSValueForTest(count)
		back, isROS, err := ldc.ZeroRunFromFSValueForTest(value)
		if err != nil {
			t.Fatalf("count %d: %v", count, err)
		}
		if isROS || back != count {
			t.Errorf("count %d round tripped to %d (ROS %v)", count, back, isROS)
		}
	}

	_, isROS, err := ldc.ZeroRunFromFSValueForTest(4)
	if err != nil || !isROS {
		t.Errorf("FS value 4 decoded as ROS=%v err=%v, want the ROS marker", isROS, err)
	}

	if _, _, err := ldc.ZeroRunFromFSValueForTest(64); err == nil {
		t.Error("FS value 64 was accepted; table 3-2 stops at 63")
	}
}
