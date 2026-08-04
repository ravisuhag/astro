package ldc

import "fmt"

// The encoder: option selection and coded data set assembly, per
// CCSDS 121.0-B-3 sections 3.7 and 5.2.
//
// Compression walks the mapped samples one block at a time. For each block it
// prices every option that applies, takes the cheapest, and writes a coded
// data set: an option identifier, an uncoded reference sample where the
// standard requires one, and the option's own output.
//
// Two things complicate the walk, and both come from the zero-block option.
// It is the only option that spans blocks, so a run of all-zero blocks is
// found before anything is priced. And its runs are bounded by segments of 64
// blocks inside each reference interval (§3.5.2), so the walk has to know
// where those boundaries fall.

// blockPlan is one block's coding decision.
type blockPlan struct {
	option Option
	k      int
	// zeroRun is how many blocks a zero-block plan covers.
	zeroRun int
	// isROS says the run reaches the end of its segment and is written with
	// the remainder-of-segment codeword.
	isROS bool
	// bits is the coded length including the identifier and any reference
	// sample.
	bits int
}

// Compress codes samples into a coded data set stream.
//
// The output is the concatenation of coded data sets with zero fill to the
// next octet, which is the file body of §7.2.3. It does not carry the
// parameters: Decompress needs the same Params, and CompressFile is the
// self-describing form.
func Compress(samples []uint32, p Params) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, nil
	}
	if err := checkSampleRange(samples, p); err != nil {
		return nil, err
	}

	mapped := Preprocess(samples, p)
	var w BitWriter

	blocks := (len(mapped) + p.BlockSize - 1) / p.BlockSize
	for block := 0; block < blocks; {
		plan := p.planBlock(mapped, block, blocks)
		p.writeBlock(&w, samples, mapped, block, plan)

		if plan.option == OptionZeroBlock {
			block += plan.zeroRun
			continue
		}
		block++
	}

	return w.Bytes(), nil
}

// checkSampleRange refuses samples that do not fit the configured resolution.
//
// Silently truncating them would produce a stream that decodes to different
// data, which is the one thing a lossless coder must never do.
func checkSampleRange(samples []uint32, p Params) error {
	if p.Resolution >= 32 {
		return nil
	}
	limit := uint32(1) << p.Resolution
	for i, sample := range samples {
		if sample >= limit {
			return fmt.Errorf("%w: sample %d is %d, which needs more than %d bits",
				ErrSampleOutOfRange, i, sample, p.Resolution)
		}
	}
	return nil
}

// blockSamples returns the mapped samples of one block, and separately the
// samples the entropy coder actually codes.
//
// They differ when the block carries a reference sample: §4.2.6 sends that
// sample uncoded at the front of the coded data set, so the coder handles the
// remaining J-1.
func (p Params) blockSamples(mapped []uint32, block int) (coded []uint32, hasReference bool) {
	start := block * p.BlockSize
	end := start + p.BlockSize
	if end > len(mapped) {
		end = len(mapped)
	}

	hasReference = p.isReferencePosition(start)
	if hasReference {
		return mapped[start+1 : end], true
	}
	return mapped[start:end], false
}

// segmentEnd returns the block index one past the end of the segment holding
// the given block.
//
// §3.5.2: the r blocks of a reference interval are partitioned into segments
// of 64 blocks, the last possibly shorter. A zero run may not cross either
// boundary.
func (p Params) segmentEnd(block, totalBlocks int) int {
	intervalStart := (block / p.ReferenceInterval) * p.ReferenceInterval
	withinInterval := block - intervalStart
	segmentStart := intervalStart + (withinInterval/zeroBlockSegment)*zeroBlockSegment

	end := segmentStart + zeroBlockSegment
	if intervalEnd := intervalStart + p.ReferenceInterval; end > intervalEnd {
		end = intervalEnd
	}
	if end > totalBlocks {
		end = totalBlocks
	}
	return end
}

// planBlock decides how to code the block starting at the given index.
func (p Params) planBlock(mapped []uint32, block, totalBlocks int) blockPlan {
	coded, hasReference := p.blockSamples(mapped, block)

	referenceBits := 0
	if hasReference {
		referenceBits = int(p.Resolution)
	}

	// §3.7.2: a run of all-zero blocks always takes the zero-block option,
	// whatever any other option would cost.
	if isAllZeros(coded) {
		run, isROS := p.zeroRun(mapped, block, totalBlocks)
		return blockPlan{
			option:  OptionZeroBlock,
			zeroRun: run,
			isROS:   isROS,
			bits:    p.optionIDBits(OptionZeroBlock) + referenceBits + zeroBlockLength(run, isROS),
		}
	}

	// §3.7.3: otherwise price every single-block option and take the shortest,
	// counting the identifier bits.
	best := blockPlan{
		option: OptionNoCompression,
		bits:   p.optionIDBits(OptionNoCompression) + referenceBits + noCompressionLength(coded, p.Resolution),
	}

	// §3.7.4 fixes the order to try ties in: no compression wins a tie, then
	// second extension, then the smallest k. Working from that order and
	// keeping a strict improvement gives exactly that preference.
	if secondBits := p.secondExtensionBits(coded, hasReference); secondBits < best.bits {
		best = blockPlan{option: OptionSecondExtension, bits: secondBits + referenceBits}
	}

	for k := 0; k <= p.maxK(); k++ {
		length := splitSampleLength(coded, k)
		if length >= unusable {
			continue
		}
		bits := p.optionIDBits(OptionSplitSample) + referenceBits + length
		if bits < best.bits {
			best = blockPlan{option: OptionSplitSample, k: k, bits: bits}
		}
	}

	return best
}

// secondExtensionBits prices the second-extension option, or reports it
// unusable.
//
// §5.2.6: when the block carries a reference sample, a zero is inserted in
// front of the J-1 coded samples so the transform still sees J of them. That
// inserted zero is why the option stays available on reference blocks at all —
// an odd count could not be paired.
func (p Params) secondExtensionBits(coded []uint32, hasReference bool) int {
	samples := coded
	if hasReference {
		samples = append([]uint32{0}, coded...)
	}
	if len(samples)%2 != 0 {
		// A partial block at the end of the input cannot be paired.
		return unusable
	}

	length := secondExtensionLength(samples)
	if length >= unusable {
		return unusable
	}
	return p.optionIDBits(OptionSecondExtension) + length
}

// zeroRun counts how many consecutive all-zero blocks start at the given
// block, and whether the run reaches the end of its segment.
//
// §3.5.3: a run that consumes the remainder of a segment is written with the
// remainder-of-segment codeword. The note there records that not using ROS
// still produces a decodable stream, but ROS is shorter for runs of five or
// more, so this uses it wherever the standard allows.
func (p Params) zeroRun(mapped []uint32, block, totalBlocks int) (run int, isROS bool) {
	limit := p.segmentEnd(block, totalBlocks)

	// Count the zeros first, without capping. Whether the cap applies depends
	// on how far the run reaches, which is not known until it stops.
	for current := block; current < limit; current++ {
		coded, _ := p.blockSamples(mapped, current)
		if !isAllZeros(coded) {
			break
		}
		run++
	}

	// ROS says "the rest of this segment is zeros", and §3.5.3 restricts it to
	// runs of five or more. It is checked before the length cap, not after:
	// a segment is 64 blocks and table 3-2 counts only to 63, so a wholly
	// zero segment can be written no other way.
	if block+run == limit && run >= 5 {
		return run, true
	}

	// Table 3-2 stops at 63, so a longer run that does not reach the end of
	// its segment needs a second codeword.
	if run > maxZeroRun {
		run = maxZeroRun
	}
	return run, false
}

// writeBlock emits one coded data set.
func (p Params) writeBlock(w *BitWriter, samples, mapped []uint32, block int, plan blockPlan) {
	coded, hasReference := p.blockSamples(mapped, block)

	p.writeOptionID(w, plan.option, plan.k)

	if hasReference {
		// §5.2.2: the uncoded reference sample follows the identifier.
		w.WriteBits(uint64(samples[block*p.BlockSize]), int(p.Resolution))
	}

	switch plan.option {
	case OptionZeroBlock:
		value := uint64(rosCodeword)
		if !plan.isROS {
			value = zeroRunFSValue(plan.zeroRun)
		}
		w.WriteZeros(value)
		w.WriteOne()

	case OptionSecondExtension:
		paired := coded
		if hasReference {
			paired = append([]uint32{0}, coded...)
		}
		writeSecondExtension(w, paired)

	case OptionNoCompression:
		writeNoCompression(w, coded, p.Resolution)

	default:
		writeSplitSample(w, coded, plan.k)
	}
}
