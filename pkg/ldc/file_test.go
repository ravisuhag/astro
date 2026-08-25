package ldc_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/ldc"
)

// TestFileHeaderLayout pins table 7-1 field by field, by building a header
// whose every field has a distinctive value and reading the octets back by
// hand.
func TestFileHeaderLayout(t *testing.T) {
	header := ldc.FileHeader{
		WordSize: 4,
		Params: ldc.Params{
			BlockSize:         32,
			Resolution:        12,
			Signed:            false,
			Predictor:         ldc.PredictorUnitDelay,
			ReferenceInterval: 256,
			Restricted:        false,
		},
		SampleCount: 1000,
	}

	got, err := header.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	if len(got) != ldc.FileHeaderSize {
		t.Fatalf("header is %d octets, want %d", len(got), ldc.FileHeaderSize)
	}

	// Octet 0: reserved 0, word size B-1 = 3 (011), preprocessor 1,
	//          predictor 001, so 0 011 1 001 = 0x39.
	if got[0] != 0x39 {
		t.Errorf("octet 0 = %08b, want 00111001", got[0])
	}
	// Octet 1: mapper 00, data sense 1 (positive), reserved 00000 of the
	//          eight-bit reserved field, so 00 1 00000 = 0x20.
	if got[1] != 0x20 {
		t.Errorf("octet 1 = %08b, want 00100000", got[1])
	}

	back, err := ldc.DecodeFileHeader(got)
	if err != nil {
		t.Fatalf("DecodeFileHeader() = %v", err)
	}
	if back.WordSize != header.WordSize {
		t.Errorf("word size = %d, want %d", back.WordSize, header.WordSize)
	}
	if back.SampleCount != header.SampleCount {
		t.Errorf("sample count = %d, want %d", back.SampleCount, header.SampleCount)
	}
	if back.Params != header.Params {
		t.Errorf("params = %+v, want %+v", back.Params, header.Params)
	}
}

// TestDataSenseIsInverted guards the one field of table 7-1 that reads the
// opposite way from the rest: '0' means two's complement, '1' means positive.
func TestDataSenseIsInverted(t *testing.T) {
	signed := ldc.FileHeader{
		WordSize:    1,
		Params:      ldc.Params{BlockSize: 8, Resolution: 8, Signed: true, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1},
		SampleCount: 8,
	}
	unsigned := signed
	unsigned.Params.Signed = false

	signedBytes, err := signed.Encode()
	if err != nil {
		t.Fatalf("Encode(signed) = %v", err)
	}
	unsignedBytes, err := unsigned.Encode()
	if err != nil {
		t.Fatalf("Encode(unsigned) = %v", err)
	}

	// The Data Sense bit is bit 2 of octet 1 (after the two mapper bits).
	signedBit := (signedBytes[1] >> 5) & 1
	unsignedBit := (unsignedBytes[1] >> 5) & 1

	if signedBit != 0 {
		t.Errorf("two's complement wrote Data Sense = %d, want 0", signedBit)
	}
	if unsignedBit != 1 {
		t.Errorf("positive wrote Data Sense = %d, want 1", unsignedBit)
	}

	// And both must read back as what they were.
	for _, header := range []ldc.FileHeader{signed, unsigned} {
		encoded, _ := header.Encode()
		back, err := ldc.DecodeFileHeader(encoded)
		if err != nil {
			t.Fatalf("DecodeFileHeader() = %v", err)
		}
		if back.Params.Signed != header.Params.Signed {
			t.Errorf("Signed round tripped %v to %v", header.Params.Signed, back.Params.Signed)
		}
	}
}

func TestFileHeaderRoundTripAcrossParams(t *testing.T) {
	blockSizes := []int{8, 16, 32, 64}
	predictors := []ldc.Predictor{ldc.PredictorNone, ldc.PredictorUnitDelay, ldc.PredictorBypass}

	for _, blockSize := range blockSizes {
		for _, predictor := range predictors {
			for _, resolution := range []uint{1, 4, 8, 16, 32} {
				for _, wordSize := range []int{1, 8} {
					p := ldc.Params{
						BlockSize:         blockSize,
						Resolution:        resolution,
						Predictor:         predictor,
						ReferenceInterval: 4096,
					}
					if p.Validate() != nil {
						continue
					}
					header := ldc.FileHeader{WordSize: wordSize, Params: p, SampleCount: 12345}

					encoded, err := header.Encode()
					if err != nil {
						t.Fatalf("Encode(%+v) = %v", header, err)
					}
					back, err := ldc.DecodeFileHeader(encoded)
					if err != nil {
						t.Fatalf("DecodeFileHeader(%+v) = %v", header, err)
					}
					if back != header {
						t.Errorf("round trip changed %+v to %+v", header, back)
					}
				}
			}
		}
	}
}

func TestFileHeaderRejectsBadValues(t *testing.T) {
	valid := ldc.Params{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 1}

	tests := []struct {
		name   string
		header ldc.FileHeader
		want   error
	}{
		{"word size zero", ldc.FileHeader{WordSize: 0, Params: valid, SampleCount: 1}, ldc.ErrInvalidWordSize},
		{"word size nine", ldc.FileHeader{WordSize: 9, Params: valid, SampleCount: 1}, ldc.ErrInvalidWordSize},
		{"no samples", ldc.FileHeader{WordSize: 1, Params: valid, SampleCount: 0}, ldc.ErrTooManySamples},
		{"bad block size", ldc.FileHeader{
			WordSize:    1,
			Params:      ldc.Params{BlockSize: 12, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 1},
			SampleCount: 1,
		}, ldc.ErrInvalidBlockSize},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.header.Encode(); !errors.Is(err, test.want) {
				t.Errorf("Encode() = %v, want %v", err, test.want)
			}
		})
	}
}

func TestDecodeFileHeaderRejectsTruncation(t *testing.T) {
	header := ldc.FileHeader{
		WordSize:    1,
		Params:      ldc.Params{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 1},
		SampleCount: 8,
	}
	encoded, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	for n := range len(encoded) {
		if _, err := ldc.DecodeFileHeader(encoded[:n]); !errors.Is(err, ldc.ErrTruncatedFile) {
			t.Errorf("DecodeFileHeader on %d octets = %v, want ErrTruncatedFile", n, err)
		}
	}
}

// TestDecodeFileHeaderRejectsSetReservedBits pins table 7-1's three reserved
// fields, which it requires to be zero.
func TestDecodeFileHeaderRejectsSetReservedBits(t *testing.T) {
	header := ldc.FileHeader{
		WordSize:    1,
		Params:      ldc.Params{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 1},
		SampleCount: 8,
	}
	base, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		octet int
		mask  byte
	}{
		{"leading reserved bit", 0, 0x80},
		{"reserved octet after Data Sense", 1, 0x01},
		{"reserved octet before the sample count", 5, 0x01},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			corrupt := make([]byte, len(base))
			copy(corrupt, base)
			corrupt[test.octet] |= test.mask

			if _, err := ldc.DecodeFileHeader(corrupt); !errors.Is(err, ldc.ErrReservedFieldSet) {
				t.Errorf("DecodeFileHeader = %v, want ErrReservedFieldSet", err)
			}
		})
	}
}

// TestDecodeFileHeaderRejectsApplicationSpecific covers the codes table 7-1
// defines but this package does not implement.
func TestDecodeFileHeaderRejectsApplicationSpecific(t *testing.T) {
	header := ldc.FileHeader{
		WordSize:    1,
		Params:      ldc.Params{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorUnitDelay, ReferenceInterval: 1},
		SampleCount: 8,
	}
	base, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// Predictor Type '111' is the application-specific predictor. It sits in
	// bits 5 to 7 of octet 0.
	appPredictor := make([]byte, len(base))
	copy(appPredictor, base)
	appPredictor[0] |= 0x07
	if _, err := ldc.DecodeFileHeader(appPredictor); !errors.Is(err, ldc.ErrUnsupportedPredictor) {
		t.Errorf("application-specific predictor = %v, want ErrUnsupportedPredictor", err)
	}

	// Mapper Type '11' is the application-specific mapper, in the top two
	// bits of octet 1.
	appMapper := make([]byte, len(base))
	copy(appMapper, base)
	appMapper[1] |= 0xC0
	if _, err := ldc.DecodeFileHeader(appMapper); !errors.Is(err, ldc.ErrUnsupportedMapper) {
		t.Errorf("application-specific mapper = %v, want ErrUnsupportedMapper", err)
	}
}

func TestFileRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		p        ldc.Params
		wordSize int
		samples  []uint32
	}{
		{"whole blocks", ldc.DefaultParams(), 1, lowEntropy(256, 21)},
		{"partial final block", ldc.DefaultParams(), 1, lowEntropy(250, 22)},
		{"word size 4", ldc.DefaultParams(), 4, lowEntropy(100, 23)},
		{"word size 8", ldc.DefaultParams(), 8, lowEntropy(37, 24)},
		{"one sample", ldc.DefaultParams(), 1, []uint32{42}},
		{"all zeros", ldc.DefaultParams(), 1, make([]uint32, 512)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, err := ldc.CompressFile(test.samples, test.p, test.wordSize)
			if err != nil {
				t.Fatalf("CompressFile() = %v", err)
			}

			// §7.2.3.2: the file is a multiple of the output word size.
			if len(file)%test.wordSize != 0 {
				t.Errorf("file is %d octets, not a multiple of the %d-octet word",
					len(file), test.wordSize)
			}

			back, err := ldc.DecompressFile(file)
			if err != nil {
				t.Fatalf("DecompressFile() = %v", err)
			}
			if len(back) != len(test.samples) {
				t.Fatalf("got %d samples, want %d", len(back), len(test.samples))
			}
			for i := range test.samples {
				if back[i] != test.samples[i] {
					t.Fatalf("sample %d: %d became %d", i, test.samples[i], back[i])
				}
			}
		})
	}
}

// TestDecompressRefusesLongWordFill pins the documented limitation of the
// unbounded Decompress: it treats only a trailing run of fewer than eight zero
// bits as the fill of §7.2.3.2, so the up-to-8B-1 bits of fill a B>1 file can
// carry make it fail — with an error, never with wrong samples. A decode that
// knows the count skips the same tail fine.
func TestDecompressRefusesLongWordFill(t *testing.T) {
	p := ldc.DefaultParams()
	samples := lowEntropy(256, 25)

	coded, err := ldc.Compress(samples, p)
	if err != nil {
		t.Fatal(err)
	}

	// Two octets of zeros: the word fill of a B=3 file whose body ended one
	// octet into a word.
	padded := append(append([]byte{}, coded...), 0, 0)

	if _, err := ldc.Decompress(padded, p); err == nil {
		t.Error("Decompress read 16 bits of word fill as a coded data set and did not error")
	}

	back, err := ldc.DecompressCount(padded, p, len(samples))
	if err != nil {
		t.Fatalf("DecompressCount over the same fill: %v", err)
	}
	for i := range samples {
		if back[i] != samples[i] {
			t.Fatalf("sample %d: %d became %d", i, samples[i], back[i])
		}
	}
}

// TestDecompressFileRefusesAnAbsurdSampleCount guards the 48-bit Number of
// Samples field: a twelve-octet header must not be able to make the decoder
// allocate a terabyte.
func TestDecompressFileRefusesAnAbsurdSampleCount(t *testing.T) {
	header := ldc.FileHeader{
		WordSize:    1,
		Params:      ldc.Params{BlockSize: 8, Resolution: 8, Predictor: ldc.PredictorNone, ReferenceInterval: 1},
		SampleCount: 1 << 40,
	}
	encoded, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ldc.DecompressFile(encoded); !errors.Is(err, ldc.ErrTooManySamples) {
		t.Errorf("DecompressFile = %v, want ErrTooManySamples", err)
	}
}
