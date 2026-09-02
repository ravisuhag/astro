package ldc

import "fmt"

// The file format of CCSDS 121.0-B-3 section 7.
//
// A coded data set stream says nothing about how it was made: not the block
// size, not the resolution, not whether a predictor ran. Clause 5.3.2.2 spells the
// consequence out (that information "must be communicated to the decoder a
// priori") and the standard offers two ways to do it. Section 6 defines a
// compression identification packet for the packet case; section 7 defines
// this file header for the stored case.
//
// The header is twelve octets and carries everything a decoder needs,
// including the sample count. So CompressFile and DecompressFile are the pair
// to reach for when the two ends do not already share a configuration.

// FileHeaderSize is the fixed width of the file header, per clause 7.2.2.
const FileHeaderSize = 12

// Predictor type codes from table 7-1.
const (
	predictorCodeBypass      = 0
	predictorCodeUnitDelay   = 1
	predictorCodeApplication = 7
)

// Mapper type codes from table 7-1.
const (
	mapperCodePredictionError = 0
	mapperCodeApplication     = 3
)

// Block size codes from table 7-1.
var blockSizeCodes = map[int]uint64{8: 0, 16: 1, 32: 2, 64: 3}

// FileHeader is the twelve-octet header of table 7-1.
type FileHeader struct {
	// WordSize is B, the output word size in octets, 1 to 8. The file is
	// padded to a multiple of it (clause 7.2.1.2, clause 7.2.3.2).
	WordSize int
	// Params are the compression parameters the body was coded with.
	Params Params
	// SampleCount is N, the number of samples the body holds.
	SampleCount uint64
}

// Encode serializes the header.
func (h FileHeader) Encode() ([]byte, error) {
	if h.WordSize < 1 || h.WordSize > 8 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidWordSize, h.WordSize)
	}
	if err := h.Params.Validate(); err != nil {
		return nil, err
	}
	if h.SampleCount < 1 || h.SampleCount > 1<<48 {
		return nil, fmt.Errorf("%w: %d", ErrTooManySamples, h.SampleCount)
	}

	predictorCode, mapperCode, preprocessorPresent := h.Params.headerCodes()

	var w BitWriter
	w.WriteBits(0, 1)                            // Reserved
	w.WriteBits(uint64(h.WordSize-1), 3)         // Output Word Size, B-1
	w.WriteBits(boolBit(preprocessorPresent), 1) // Preprocessor Status
	w.WriteBits(predictorCode, 3)                // Predictor Type
	w.WriteBits(mapperCode, 2)                   // Mapper Type
	// Data Sense reads the opposite way round from the other flags: table 7-1
	// gives '0' to two's complement and '1' to positive, so the bit is the
	// negation of Signed.
	w.WriteBits(boolBit(!h.Params.Signed), 1)
	w.WriteBits(0, 8)                             // Reserved
	w.WriteBits(uint64(h.Params.Resolution-1), 5) // Input Data Resolution, n-1
	w.WriteBits(0, 1)                             // Reserved
	w.WriteBits(blockSizeCodes[h.Params.BlockSize], 2)
	w.WriteBits(boolBit(h.Params.Restricted), 1) // Restricted Code Option
	w.WriteBits(uint64(h.Params.ReferenceInterval-1), 12)
	w.WriteBits(0, 8)                // Reserved
	w.WriteBits(h.SampleCount-1, 48) // Number of Samples, N-1

	out := w.Bytes()
	if len(out) != FileHeaderSize {
		return nil, fmt.Errorf("header came to %d octets, want %d", len(out), FileHeaderSize)
	}
	return out, nil
}

// boolBit turns a flag into a bit.
func boolBit(set bool) uint64 {
	if set {
		return 1
	}
	return 0
}

// headerCodes turns the predictor setting into the three header fields that
// describe it.
func (p Params) headerCodes() (predictorCode, mapperCode uint64, preprocessorPresent bool) {
	switch p.Predictor {
	case PredictorUnitDelay:
		return predictorCodeUnitDelay, mapperCodePredictionError, true
	case PredictorBypass:
		return predictorCodeBypass, mapperCodePredictionError, true
	default:
		// Clause 7.2.2: with the preprocessor absent, the predictor field is '000'
		// and the mapper field '00', the same codes the bypass predictor uses.
		// The Preprocessor Status bit is what tells them apart.
		return predictorCodeBypass, mapperCodePredictionError, false
	}
}

// DecodeFileHeader parses a file header.
func DecodeFileHeader(data []byte) (FileHeader, error) {
	var h FileHeader
	if len(data) < FileHeaderSize {
		return h, fmt.Errorf("%w: %d of %d octets", ErrTruncatedFile, len(data), FileHeaderSize)
	}

	r := NewBitReader(data[:FileHeaderSize])
	read := func(n int) uint64 {
		v, _ := r.ReadBits(n) // bounded: the slice is exactly 12 octets
		return v
	}

	if read(1) != 0 {
		return h, fmt.Errorf("%w: the leading reserved bit", ErrReservedFieldSet)
	}
	h.WordSize = int(read(3)) + 1

	preprocessorPresent := read(1) == 1
	predictorCode := read(3)
	mapperCode := read(2)
	signed := read(1) == 0 // Data Sense: 0 is two's complement

	if read(8) != 0 {
		return h, fmt.Errorf("%w: the octet after Data Sense", ErrReservedFieldSet)
	}

	resolution := uint(read(5)) + 1

	if read(1) != 0 {
		return h, fmt.Errorf("%w: the bit before Block Size", ErrReservedFieldSet)
	}

	blockCode := read(2)
	restricted := read(1) == 1
	referenceInterval := int(read(12)) + 1

	if read(8) != 0 {
		return h, fmt.Errorf("%w: the octet before Number of Samples", ErrReservedFieldSet)
	}

	h.SampleCount = read(48) + 1

	switch {
	case mapperCode == mapperCodeApplication:
		return h, fmt.Errorf("%w: application-specific mapper", ErrUnsupportedMapper)
	case mapperCode != mapperCodePredictionError:
		return h, fmt.Errorf("%w: mapper code %d is reserved", ErrUnsupportedMapper, mapperCode)
	}

	predictor, err := decodePredictor(predictorCode, preprocessorPresent)
	if err != nil {
		return h, err
	}

	h.Params = Params{
		BlockSize:         blockSizeFromCode(blockCode),
		Resolution:        resolution,
		Signed:            signed,
		Predictor:         predictor,
		ReferenceInterval: referenceInterval,
		Restricted:        restricted,
	}
	if err := h.Params.Validate(); err != nil {
		return h, err
	}
	return h, nil
}

// decodePredictor turns the header's predictor code and preprocessor bit into
// a Predictor.
func decodePredictor(code uint64, present bool) (Predictor, error) {
	if !present {
		return PredictorNone, nil
	}
	switch code {
	case predictorCodeBypass:
		return PredictorBypass, nil
	case predictorCodeUnitDelay:
		return PredictorUnitDelay, nil
	case predictorCodeApplication:
		return 0, fmt.Errorf("%w: application-specific predictor", ErrUnsupportedPredictor)
	default:
		return 0, fmt.Errorf("%w: predictor code %d is reserved", ErrUnsupportedPredictor, code)
	}
}

// blockSizeFromCode inverts table 7-1's two-bit block size field.
func blockSizeFromCode(code uint64) int {
	return 8 << code
}

// Humanize returns a human-readable summary.
func (h FileHeader) Humanize() string {
	return fmt.Sprintf("LDC File Header\n"+
		"  Word size .......... %d octets\n"+
		"  Samples ............ %d\n"+
		"%s",
		h.WordSize, h.SampleCount, h.Params.Humanize())
}

// CompressFile codes samples into the self-describing file format of section
// 7: a twelve-octet header, the coded data sets, then zero fill to the next
// output word boundary.
//
// wordSize is B in octets, 1 to 8. Pass 1 for no padding beyond the octet.
func CompressFile(samples []uint32, p Params, wordSize int) ([]byte, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("%w: nothing to compress", ErrTooManySamples)
	}

	header := FileHeader{
		WordSize:    wordSize,
		Params:      p,
		SampleCount: uint64(len(samples)),
	}
	head, err := header.Encode()
	if err != nil {
		return nil, err
	}

	body, err := Compress(samples, p)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(head)+len(body)+wordSize)
	out = append(out, head...)
	out = append(out, body...)

	// Clause 7.2.3.2: fill with zeros to a multiple of the output word size. The
	// header is twelve octets, so the padding is measured over the whole file.
	if remainder := len(out) % wordSize; remainder != 0 {
		out = append(out, make([]byte, wordSize-remainder)...)
	}
	return out, nil
}

// DecompressFile reads a file written by CompressFile, taking every parameter
// from its header.
func DecompressFile(data []byte) ([]uint32, error) {
	header, err := DecodeFileHeader(data)
	if err != nil {
		return nil, err
	}
	if header.SampleCount > uint64(maxDecodableSamples) {
		return nil, fmt.Errorf("%w: header claims %d samples", ErrTooManySamples, header.SampleCount)
	}
	return DecompressCount(data[FileHeaderSize:], header.Params, int(header.SampleCount))
}

// maxDecodableSamples caps what a header may ask a decoder to allocate.
//
// The Number of Samples field is 48 bits, so a hostile header can claim 2^48
// samples (a terabyte of uint32) and the decoder would size a slice from it
// before reading a single coded bit. The cap is not in the standard; it is
// what stops a twelve-octet file from exhausting memory.
const maxDecodableSamples = 1 << 28

// FileBody returns the coded data sets of a file, without decoding them.
func FileBody(data []byte) ([]byte, error) {
	if len(data) < FileHeaderSize {
		return nil, fmt.Errorf("%w: %d of %d octets", ErrTruncatedFile, len(data), FileHeaderSize)
	}
	return data[FileHeaderSize:], nil
}
