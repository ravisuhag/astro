package ldc

// Test shims.
//
// The coding options are unexported because they are not an API: a caller
// compresses and decompresses, and the choice of option is the coder's. But
// each option is pinned to a spec table, and those tests are worth writing
// against the option itself rather than through the whole encoder, a failure
// then names the option instead of saying "the output changed".
//
// This file exists only in tests, so the shims cost nothing at build time.

// EncodeOptionIDForTest returns an option identifier as a bit string.
func EncodeOptionIDForTest(p Params, o Option, k int) string {
	var w BitWriter
	p.writeOptionID(&w, o, k)
	return bitsOf(w.Bytes(), w.BitLen())
}

// DecodeOptionIDForTest reads an option identifier from a bit string.
func DecodeOptionIDForTest(p Params, bits string) (Option, int, error) {
	r := NewBitReader(packBits(bits))
	return p.readOptionID(r)
}

// MaxKForTest exposes the derived bound on the split-sample parameter.
func MaxKForTest(p Params) int { return p.maxK() }

// EncodeSplitSampleForTest codes one block and returns the bytes and length.
func EncodeSplitSampleForTest(block []uint32, k int) ([]byte, int) {
	var w BitWriter
	writeSplitSample(&w, block, k)
	return w.Bytes(), w.BitLen()
}

// DecodeSplitSampleForTest reads one split-sample block.
func DecodeSplitSampleForTest(data []byte, count, k int, resolution uint) ([]uint32, error) {
	return readSplitSample(NewBitReader(data), count, k, resolution)
}

// SplitSampleLengthForTest exposes the length calculation.
func SplitSampleLengthForTest(block []uint32, k int) int { return splitSampleLength(block, k) }

// SecondExtensionSymbolsForTest exposes the pairing transform, collecting the
// visited symbols so a test can compare them against the worked values.
//
// The transform itself visits rather than collects, because neither caller in
// the coder keeps the symbols and the slice was 95% of a compression's
// allocations. Gathering them here costs nothing that matters.
func SecondExtensionSymbolsForTest(block []uint32) ([]uint64, bool) {
	var symbols []uint64
	ok := eachSecondExtensionSymbol(block, func(gamma uint64) bool {
		symbols = append(symbols, gamma)
		return true
	})
	if !ok {
		return nil, false
	}
	return symbols, true
}

// EncodeSecondExtensionForTest codes one block.
func EncodeSecondExtensionForTest(block []uint32) ([]byte, int) {
	var w BitWriter
	writeSecondExtension(&w, block)
	return w.Bytes(), w.BitLen()
}

// DecodeSecondExtensionForTest reads one second-extension block.
func DecodeSecondExtensionForTest(data []byte, count int, resolution uint) ([]uint32, error) {
	return readSecondExtension(NewBitReader(data), count, resolution)
}

// SecondExtensionLengthForTest exposes the length calculation.
func SecondExtensionLengthForTest(block []uint32) int { return secondExtensionLength(block) }

// TriangularRootForTest exposes the inverse pairing helper.
func TriangularRootForTest(v uint64) uint64 { return triangularRoot(v) }

// UnusableForTest exposes the marker for an option that cannot be used.
func UnusableForTest() int { return unusable }

// EncodeNoCompressionForTest codes one block unaltered.
func EncodeNoCompressionForTest(block []uint32, resolution uint) ([]byte, int) {
	var w BitWriter
	writeNoCompression(&w, block, resolution)
	return w.Bytes(), w.BitLen()
}

// DecodeNoCompressionForTest reads one uncoded block.
func DecodeNoCompressionForTest(data []byte, count int, resolution uint) ([]uint32, error) {
	return readNoCompression(NewBitReader(data), count, resolution)
}

// NoCompressionLengthForTest exposes the length calculation.
func NoCompressionLengthForTest(block []uint32, resolution uint) int {
	return noCompressionLength(block, resolution)
}

// EncodeZeroRunForTest returns a zero-run codeword as a bit string.
func EncodeZeroRunForTest(count int, isROS bool) string {
	var w BitWriter
	value := uint64(rosCodeword)
	if !isROS {
		value = zeroRunFSValue(count)
	}
	w.WriteZeros(value)
	w.WriteOne()
	return bitsOf(w.Bytes(), w.BitLen())
}

// ZeroBlockLengthForTest exposes the length calculation.
func ZeroBlockLengthForTest(count int, isROS bool) int { return zeroBlockLength(count, isROS) }

// ZeroRunFSValueForTest exposes the displaced numbering of table 3-2.
func ZeroRunFSValueForTest(count int) uint64 { return zeroRunFSValue(count) }

// ZeroRunFromFSValueForTest inverts it.
func ZeroRunFromFSValueForTest(value uint64) (int, bool, error) {
	return zeroRunFromFSValue(value)
}

// bitsOf renders packed octets as a bit string of the given length.
func bitsOf(data []byte, bits int) string {
	out := make([]byte, 0, bits)
	for i := range bits {
		out = append(out, '0'+(data[i/8]>>(7-uint(i%8)))&1)
	}
	return string(out)
}

// packBits is the inverse of bitsOf.
func packBits(bits string) []byte {
	var w BitWriter
	for _, c := range bits {
		if c == '1' {
			w.WriteBits(1, 1)
		} else {
			w.WriteBits(0, 1)
		}
	}
	return w.Bytes()
}
