package ldc

import "fmt"

// The decoder, inverting the coded data set formats of CCSDS 121.0-B-3
// section 5.2.
//
// Decoding is mechanical once the option identifier is read: each option
// knows how to read itself back. What the coded stream does not carry is how
// many samples it holds.
//
// That is not an omission in this implementation. Clause 5.3.2.2 says as much: to
// decode a stream that may include fill bits, "several pieces of information
// must be communicated to the decoder a priori", and the file header of
// Clause 7.2.2 exists to carry them, sample count included. So there are three ways
// in, in increasing order of certainty:
//
//	Decompress          whole blocks only, fill detected by inspection
//	DecompressCount     you know how many samples there are
//	DecompressFile      the header says, which is the standard's own answer

// Decompress reads a coded data set stream back into samples.
//
// It decodes until the input is exhausted, treating a trailing run of fewer
// than eight zero bits as the fill of clause 7.2.3.2. That works because fill is
// always zeros and every coded data set needs a one bit to terminate its
// first codeword, so fill can never be mistaken for another set.
//
// The eight-bit bound is deliberate, and it is a limitation: a file written
// with an output word size B above one octet (clause 7.2.1.2) may carry up to 8B-1
// bits of zero fill, and Decompress cannot tell such a tail from a truncated
// coded data set. It returns an error rather than guessing. That is the safe
// failure: only a decode that knows the sample count can skip longer fill,
// which is what DecompressCount and DecompressFile do.
//
// It also cannot recover a partial final block, because nothing in the stream
// says the block was short. Use DecompressCount or DecompressFile when the
// sample count is not a whole number of blocks.
func Decompress(data []byte, p Params) ([]uint32, error) {
	return decompress(data, p, -1)
}

// DecompressCount reads exactly count samples, which allows a partial final
// block.
func DecompressCount(data []byte, p Params, count int) ([]uint32, error) {
	if count < 0 {
		return nil, fmt.Errorf("%w: negative sample count", ErrSampleCountMismatch)
	}
	return decompress(data, p, count)
}

// decompress does the work. A count below zero means "until the input runs
// out".
func decompress(data []byte, p Params, count int) ([]uint32, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if count == 0 || len(data) == 0 {
		return nil, nil
	}

	r := NewBitReader(data)

	var mapped []uint32
	references := map[int]uint32{}

	totalBlocks := -1
	if count > 0 {
		totalBlocks = (count + p.BlockSize - 1) / p.BlockSize
	}

	for block := 0; ; {
		if totalBlocks >= 0 && block >= totalBlocks {
			break
		}
		if totalBlocks < 0 && isOnlyFillLeft(r) {
			break
		}

		// How many samples this block holds, which the last one may shorten.
		blockSamples := p.BlockSize
		if count > 0 {
			if remaining := count - block*p.BlockSize; remaining < blockSamples {
				blockSamples = remaining
			}
		}

		consumed, err := p.readBlock(r, block, blockSamples, count, totalBlocks, &mapped, references)
		if err != nil {
			return nil, err
		}
		block += consumed
	}

	if count > 0 {
		if len(mapped) < count {
			return nil, fmt.Errorf("%w: got %d of %d", ErrSampleCountMismatch, len(mapped), count)
		}
		mapped = mapped[:count]
	}

	return Unpreprocess(mapped, references, p), nil
}

// isOnlyFillLeft reports whether what remains is the zero fill of clause 7.2.3.2
// rather than another coded data set.
//
// Fill is at most seven bits and always zeros. Any real coded data set needs
// a one bit to end its first codeword, so a remainder that is short and
// entirely zero cannot be one.
func isOnlyFillLeft(r *BitReader) bool {
	left := r.BitsLeft()
	if left == 0 {
		return true
	}
	if left >= 8 {
		return false
	}

	// Peek without consuming.
	saved := *r
	value, err := r.ReadBits(left)
	*r = saved
	return err == nil && value == 0
}

// readBlock reads one coded data set and appends its samples. It returns how
// many blocks the set covered, which is more than one only for a zero-block
// run.
func (p Params) readBlock(
	r *BitReader, block, blockSamples, count, totalBlocks int,
	mapped *[]uint32, references map[int]uint32,
) (int, error) {
	option, k, err := p.readOptionID(r)
	if err != nil {
		return 0, err
	}

	start := block * p.BlockSize
	hasReference := p.isReferencePosition(start)

	codedCount := blockSamples
	if hasReference {
		reference, err := r.ReadBits(int(p.Resolution))
		if err != nil {
			return 0, err
		}
		references[start] = uint32(reference)
		// The reference occupies the block's first slot; the coder handled the
		// rest.
		*mapped = append(*mapped, 0)
		codedCount--
	}

	switch option {
	case OptionZeroBlock:
		return p.readZeroBlock(r, block, count, totalBlocks, hasReference, mapped, references)

	case OptionSecondExtension:
		paired := codedCount
		if hasReference {
			// Clause 5.2.6 inserted a zero in front, so one more symbol was coded.
			paired++
		}
		if paired%2 != 0 {
			return 0, fmt.Errorf("%w: %d samples cannot be paired", ErrDataTooShort, paired)
		}
		block, err := readSecondExtension(r, paired, p.Resolution)
		if err != nil {
			return 0, err
		}
		if hasReference {
			// Drop the inserted zero.
			block = block[1:]
		}
		*mapped = append(*mapped, block...)

	case OptionNoCompression:
		samples, err := readNoCompression(r, codedCount, p.Resolution)
		if err != nil {
			return 0, err
		}
		*mapped = append(*mapped, samples...)

	default:
		if k > p.maxK() {
			return 0, fmt.Errorf("%w: k=%d", ErrInvalidOptionID, k)
		}
		samples, err := readSplitSample(r, codedCount, k, p.Resolution)
		if err != nil {
			return 0, err
		}
		*mapped = append(*mapped, samples...)
	}

	return 1, nil
}

// readZeroBlock reads a zero-block coded data set and appends the zeros it
// stands for.
func (p Params) readZeroBlock(
	r *BitReader, block, count, totalBlocks int, hasReference bool,
	mapped *[]uint32, references map[int]uint32,
) (int, error) {
	value, err := r.ReadFS(maxZeroRun)
	if err != nil {
		return 0, err
	}

	run, isROS, err := zeroRunFromFSValue(value)
	if err != nil {
		return 0, err
	}

	if isROS {
		// The rest of this segment is zeros. Where the segment ends depends on
		// how many blocks there are, which only a bounded decode knows.
		if totalBlocks < 0 {
			return 0, fmt.Errorf(
				"%w: a remainder-of-segment codeword needs the sample count to resolve",
				ErrSampleCountMismatch)
		}
		run = p.segmentEnd(block, totalBlocks) - block
	}
	if run < 1 {
		return 0, fmt.Errorf("%w: zero run of %d blocks", ErrInvalidOptionID, run)
	}

	// The first block already had its reference slot appended by the caller.
	first := p.BlockSize
	if hasReference {
		first--
	}
	for range first {
		*mapped = append(*mapped, 0)
	}

	// Every further block in the run is all zeros too, and may itself carry a
	// reference sample position. Clause 3.5.1 counts a reference block as all zeros
	// when the J-1 samples after the reference are zero, so a run can span
	// one, but a reference sample is uncoded and cannot be inside a run's
	// single codeword, so a run never crosses a reference interval.
	for i := 1; i < run; i++ {
		for range p.BlockSize {
			*mapped = append(*mapped, 0)
		}
	}

	_ = references
	_ = count
	return run, nil
}

// BlockInfo describes one coded data set as the decoder found it.
//
// This is what makes a compressed stream inspectable: which option won each
// block, and how many bits it cost. Useful for checking a mission's parameter
// choices against real data, and for the tests that pin option selection.
type BlockInfo struct {
	// Block is the index of the first block this coded data set covers.
	Block int
	// Option is the coding option used.
	Option Option
	// K is the split-sample parameter, meaningful only for OptionSplitSample.
	K int
	// ZeroRun is how many blocks a zero-block set covers, 1 for every other
	// option.
	ZeroRun int
	// IsROS says a zero-block set used the remainder-of-segment codeword.
	IsROS bool
	// Bits is the coded length of the whole set, identifier included.
	Bits int
	// HasReference says the set carries an uncoded reference sample.
	HasReference bool
}

// Analyze walks a coded stream and reports what each coded data set contains,
// without reconstructing the samples.
func Analyze(data []byte, p Params, count int) ([]BlockInfo, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if count <= 0 || len(data) == 0 {
		return nil, nil
	}

	r := NewBitReader(data)
	totalBlocks := (count + p.BlockSize - 1) / p.BlockSize

	var infos []BlockInfo
	var mapped []uint32
	references := map[int]uint32{}

	for block := 0; block < totalBlocks; {
		before := r.Pos()

		start := block * p.BlockSize
		hasReference := p.isReferencePosition(start)

		// Peek at the option by decoding the block, then describe it.
		blockSamples := p.BlockSize
		if remaining := count - start; remaining < blockSamples {
			blockSamples = remaining
		}

		peek := *r
		option, k, err := p.readOptionID(&peek)
		if err != nil {
			return nil, err
		}

		consumed, err := p.readBlock(r, block, blockSamples, count, totalBlocks, &mapped, references)
		if err != nil {
			return nil, err
		}

		info := BlockInfo{
			Block:        block,
			Option:       option,
			K:            k,
			ZeroRun:      consumed,
			Bits:         r.Pos() - before,
			HasReference: hasReference,
		}
		if option == OptionZeroBlock {
			// A run written as ROS reaches the end of its segment.
			info.IsROS = block+consumed == p.segmentEnd(block, totalBlocks) && consumed >= 5
		}
		infos = append(infos, info)

		block += consumed
	}
	return infos, nil
}
