package ldc

import (
	"fmt"
	"math/bits"
)

// The code options of CCSDS 121.0-B-3 section 3.
//
// The coder tries every applicable option on each block and keeps the one
// that comes out shortest. So each option here does three things: say how
// long its output would be without producing it, produce it, and read it
// back.
//
// The length-without-producing part is not an optimisation. An FS codeword
// for a sample of value m is m+1 bits, and m can be four billion at 32-bit
// resolution — an encoder that measured options by emitting them would try to
// build a half-gigabyte codeword before discarding it.

// Option names a code option.
type Option int

const (
	// OptionZeroBlock encodes a run of all-zero blocks, §3.5.
	OptionZeroBlock Option = iota
	// OptionSecondExtension pairs samples before coding them, §3.4.
	OptionSecondExtension
	// OptionSplitSample is the family of split-sample options, §3.3. The FS
	// option of §3.2 is the member with k=0.
	OptionSplitSample
	// OptionNoCompression sends the block unaltered, §3.6.
	OptionNoCompression
)

// String names the option.
func (o Option) String() string {
	switch o {
	case OptionZeroBlock:
		return "zero block"
	case OptionSecondExtension:
		return "second extension"
	case OptionSplitSample:
		return "split sample"
	default:
		return "no compression"
	}
}

// unusable marks an option that cannot encode a block, or whose output would
// be so long that it can never win. Lengths are compared as ints, and no real
// option approaches this.
const unusable = int(^uint(0) >> 2)

// writeOptionID writes the identifier for a split-sample option with the given
// k, per table 5-1.
//
// The table is one rule in five columns: a w-bit identifier holding k+1, with
// all-ones reserved for no compression and all-zeros escaping into a further
// bit that separates the zero-block and second-extension options.
func (p Params) writeOptionID(w *BitWriter, o Option, k int) {
	width := p.idWidth()

	switch o {
	case OptionZeroBlock:
		// All zeros, then a further 0.
		w.WriteBits(0, width+1)
	case OptionSecondExtension:
		// All zeros, then a further 1.
		w.WriteBits(1, width+1)
	case OptionNoCompression:
		w.WriteBits(uint64(1<<width)-1, width)
	default:
		w.WriteBits(uint64(k+1), width)
	}
}

// optionIDBits returns how many bits an option's identifier occupies.
func (p Params) optionIDBits(o Option) int {
	switch o {
	case OptionZeroBlock, OptionSecondExtension:
		return p.idWidth() + 1
	default:
		return p.idWidth()
	}
}

// readOptionID reads an option identifier and reports which option it names.
func (p Params) readOptionID(r *BitReader) (Option, int, error) {
	width := p.idWidth()

	value, err := r.ReadBits(width)
	if err != nil {
		return 0, 0, err
	}

	switch {
	case value == 0:
		// The escape: one more bit separates zero-block from second-extension.
		extra, err := r.ReadBits(1)
		if err != nil {
			return 0, 0, err
		}
		if extra == 0 {
			return OptionZeroBlock, 0, nil
		}
		return OptionSecondExtension, 0, nil

	case value == uint64(1<<width)-1:
		return OptionNoCompression, 0, nil

	default:
		k := int(value) - 1
		if k > p.maxK() {
			return 0, 0, fmt.Errorf("%w: k=%d is past the %d this resolution allows",
				ErrInvalidOptionID, k, p.maxK())
		}
		return OptionSplitSample, k, nil
	}
}

// splitSampleLength returns the coded length in bits of one block under the
// split-sample option with parameter k, excluding the option identifier and
// any reference sample.
//
// §3.3.1: the top n-k bits of each sample become an FS codeword, and the low k
// bits follow uncoded. An FS codeword for value v is v+1 bits.
func splitSampleLength(block []uint32, k int) int {
	total := 0
	for _, sample := range block {
		total += int(sample>>uint(k)) + 1
		if total >= unusable {
			return unusable
		}
	}
	return total + k*len(block)
}

// writeSplitSample emits one block under the split-sample option.
//
// §3.3.3 fixes the order: every FS codeword first, then every sample's k low
// bits. Interleaving them would be the obvious implementation and the wrong
// one.
func writeSplitSample(w *BitWriter, block []uint32, k int) {
	for _, sample := range block {
		w.WriteZeros(uint64(sample >> uint(k)))
		w.WriteOne()
	}
	if k == 0 {
		return
	}
	mask := uint32(1)<<uint(k) - 1
	for _, sample := range block {
		w.WriteBits(uint64(sample&mask), k)
	}
}

// readSplitSample reads one block coded with the split-sample option.
func readSplitSample(r *BitReader, count, k int, resolution uint) ([]uint32, error) {
	// The FS part of a sample cannot exceed what the resolution allows, which
	// bounds how long a legal codeword can be.
	limit := fsLimit(resolution, k)

	block := make([]uint32, count)
	for i := range block {
		high, err := r.ReadFS(limit)
		if err != nil {
			return nil, err
		}
		block[i] = uint32(high) << uint(k)
	}
	if k == 0 {
		return block, nil
	}
	for i := range block {
		low, err := r.ReadBits(k)
		if err != nil {
			return nil, err
		}
		block[i] |= uint32(low)
	}
	return block, nil
}

// fsLimit is the largest FS value a legal sample can produce at this
// resolution and k, which bounds how many zeros a codeword may carry.
//
// Without it, a run of zero octets in a corrupt stream would be read as a
// gigantic value and the reader would spin through it.
func fsLimit(resolution uint, k int) uint64 {
	if resolution >= 32 {
		return (uint64(1) << 32) >> uint(k)
	}
	return (uint64(1) << resolution) >> uint(k)
}

// secondExtensionSymbols transforms a block into the paired symbols of §3.4.1:
//
//	gamma_j = (d_{2j-1} + d_{2j})(d_{2j-1} + d_{2j} + 1)/2 + d_{2j}
//
// The product of two consecutive integers is always even, so the division is
// exact and the whole thing stays in integers.
//
// It reports false when the arithmetic would overflow. That is not a
// theoretical worry: at 32-bit resolution a pair of large samples gives a sum
// near 2^33 and a product near 2^66, which does not fit. §3.4.2 notes the
// option "is only designed to be a useful option when all of the transformed
// symbols are small", so an overflowing block simply cannot use it.
func secondExtensionSymbols(block []uint32) ([]uint64, bool) {
	if len(block)%2 != 0 {
		return nil, false
	}

	symbols := make([]uint64, 0, len(block)/2)
	for i := 0; i < len(block); i += 2 {
		first := uint64(block[i])
		second := uint64(block[i+1])
		sum := first + second

		// (sum)(sum+1)/2 overflows a uint64 once sum passes about 2^32. Check
		// before multiplying rather than after.
		if sum >= 1<<32 {
			return nil, false
		}
		gamma := sum*(sum+1)/2 + second
		symbols = append(symbols, gamma)
	}
	return symbols, true
}

// secondExtensionLength returns the coded length in bits, excluding the option
// identifier and any reference sample.
func secondExtensionLength(block []uint32) int {
	symbols, ok := secondExtensionSymbols(block)
	if !ok {
		return unusable
	}

	total := 0
	for _, gamma := range symbols {
		if gamma >= uint64(unusable) {
			return unusable
		}
		total += int(gamma) + 1
		if total >= unusable {
			return unusable
		}
	}
	return total
}

// writeSecondExtension emits one block under the second-extension option.
func writeSecondExtension(w *BitWriter, block []uint32) bool {
	symbols, ok := secondExtensionSymbols(block)
	if !ok {
		return false
	}
	for _, gamma := range symbols {
		w.WriteZeros(gamma)
		w.WriteOne()
	}
	return true
}

// readSecondExtension reads count samples coded with the second-extension
// option, inverting the pairing transform.
//
// Recovering the pair from gamma means finding the triangular number below
// it: gamma = T(sum) + second where T(s) = s(s+1)/2 and second <= sum. Since
// T is strictly increasing, the largest s with T(s) <= gamma is the sum.
func readSecondExtension(r *BitReader, count int, resolution uint) ([]uint32, error) {
	if count%2 != 0 {
		return nil, ErrDataTooShort
	}

	// The largest legal gamma comes from two samples at the top of the range.
	// At n = 32 that is T(2^33-2) + 2^32-1, which does not fit a uint64, so
	// the limit saturates. That still bounds ReadFS — a codeword near the
	// saturated limit would need exabytes of input to exist — and the range
	// check on first and second below rejects anything a wrapped bound would
	// have let through.
	maxSample := uint64(1)<<resolution - 1
	if resolution >= 32 {
		maxSample = uint64(1)<<32 - 1
	}
	maxSum := 2 * maxSample
	limit := ^uint64(0)
	if t, ok := triangular(maxSum); ok && t <= limit-maxSample {
		limit = t + maxSample
	}

	block := make([]uint32, 0, count)
	for range count / 2 {
		gamma, err := r.ReadFS(limit)
		if err != nil {
			return nil, err
		}

		sum := triangularRoot(gamma)
		// triangularRoot guarantees T(sum) <= gamma, so the triangular number
		// fits and the subtraction cannot wrap.
		t, _ := triangular(sum)
		second := gamma - t
		if second > sum {
			return nil, ErrDataTooShort
		}
		first := sum - second

		if first > maxSample || second > maxSample {
			return nil, ErrSampleOutOfRange
		}
		block = append(block, uint32(first), uint32(second))
	}
	return block, nil
}

// triangular returns s(s+1)/2 and whether it fits a uint64.
//
// The product s(s+1) is taken at 128 bits so that s near 2^33 — reachable at
// 32-bit resolution, where a pair sum can be up to 2^33-2 — cannot wrap.
func triangular(s uint64) (uint64, bool) {
	hi, lo := bits.Mul64(s, s+1)
	return lo>>1 | hi<<63, hi>>1 == 0
}

// triangularRoot returns the largest s for which s(s+1)/2 <= v.
//
// Done by binary search rather than by the closed form, which would need a
// square root and therefore floating point — and this standard is integer
// only. The comparisons go through triangular, so v all the way up to the
// maximum uint64 is handled without the arithmetic wrapping.
func triangularRoot(v uint64) uint64 {
	triangularLE := func(s uint64) bool {
		t, ok := triangular(s)
		return ok && t <= v
	}
	low, high := uint64(0), uint64(1)
	for triangularLE(high) && high < 1<<33 {
		high *= 2
	}
	for low < high {
		mid := (low + high + 1) / 2
		if triangularLE(mid) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return low
}

// noCompressionLength returns the coded length in bits, excluding the option
// identifier and any reference sample. §3.6.1: the block goes out unaltered.
func noCompressionLength(block []uint32, resolution uint) int {
	return len(block) * int(resolution)
}

// writeNoCompression emits one block unaltered.
func writeNoCompression(w *BitWriter, block []uint32, resolution uint) {
	for _, sample := range block {
		w.WriteBits(uint64(sample), int(resolution))
	}
}

// readNoCompression reads count uncoded samples.
func readNoCompression(r *BitReader, count int, resolution uint) ([]uint32, error) {
	block := make([]uint32, count)
	for i := range block {
		v, err := r.ReadBits(int(resolution))
		if err != nil {
			return nil, err
		}
		block[i] = uint32(v)
	}
	return block, nil
}

// Zero-block run coding, per table 3-2.
//
// The table is not quite an FS code over the run length: the
// remainder-of-segment codeword sits between four and five, so a run of five
// or more that reaches the end of a segment is written as ROS instead of its
// count. That displacement is the only awkward part of the standard's coding.
const (
	// rosCodeword is the FS value that means remainder-of-segment.
	rosCodeword = 4
	// maxZeroRun is the largest run table 3-2 gives a codeword to.
	maxZeroRun = 63
	// zeroBlockSegment is how many blocks a segment holds, per §3.5.2.
	zeroBlockSegment = 64
)

// zeroRunFSValue returns the FS value that encodes a run of count all-zero
// blocks, per table 3-2: counts 1 to 4 map to 0 to 3, and counts 5 and up map
// to themselves, because the value 4 is taken by ROS.
func zeroRunFSValue(count int) uint64 {
	if count <= 4 {
		return uint64(count - 1)
	}
	return uint64(count)
}

// zeroRunFromFSValue inverts zeroRunFSValue, and reports whether the codeword
// was the remainder-of-segment marker.
func zeroRunFromFSValue(value uint64) (count int, isROS bool, err error) {
	switch {
	case value < rosCodeword:
		return int(value) + 1, false, nil
	case value == rosCodeword:
		return 0, true, nil
	case value <= maxZeroRun:
		return int(value), false, nil
	default:
		return 0, false, fmt.Errorf("%w: zero-run codeword %d is past 63", ErrInvalidOptionID, value)
	}
}

// zeroBlockLength returns the coded length in bits of a zero-block codeword,
// excluding the option identifier and any reference sample.
func zeroBlockLength(count int, isROS bool) int {
	if isROS {
		return rosCodeword + 1
	}
	return int(zeroRunFSValue(count)) + 1
}

// isAllZeros reports whether a block of mapped samples is all zero.
//
// §3.5.1: when the block carries a reference sample, only the J-1 samples
// after it are considered, "regardless of whether the reference sample itself
// is zero" (§3.7.2). The caller passes the block already trimmed of its
// reference, so this is a plain scan.
func isAllZeros(block []uint32) bool {
	for _, sample := range block {
		if sample != 0 {
			return false
		}
	}
	return true
}
