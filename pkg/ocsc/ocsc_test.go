package ocsc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/ocsc"
)

func TestPNSequenceMatchesTheSpecVector(t *testing.T) {
	// §3.5.2.1 note publishes the first 40 digits:
	// 1111 1111 0100 1000 0000 1110 1100 0000 1001 1010
	want := []uint8{
		1, 1, 1, 1, 1, 1, 1, 1,
		0, 1, 0, 0, 1, 0, 0, 0,
		0, 0, 0, 0, 1, 1, 1, 0,
		1, 1, 0, 0, 0, 0, 0, 0,
		1, 0, 0, 1, 1, 0, 1, 0,
	}

	got := ocsc.PNSequence(len(want))
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("digit %d = %d, want %d\n got: %v\nwant: %v", i, got[i], want[i], got, want)
		}
	}
}

func TestPNSequencePeriod(t *testing.T) {
	// §3.5.3.1: the sequence repeats after 255 digits.
	for i := 0; i < 100; i++ {
		if ocsc.PNBit(i) != ocsc.PNBit(i+ocsc.PNPeriod) {
			t.Fatalf("digit %d differs from digit %d; the period is not %d",
				i, i+ocsc.PNPeriod, ocsc.PNPeriod)
		}
	}
}

func TestRandomizeIsItsOwnInverse(t *testing.T) {
	block := ocsc.BitStringFromBytes([]byte("optical downlink payload"))

	randomized := ocsc.Randomize(block)
	if randomized.Equal(block) {
		t.Error("randomizing changed nothing")
	}

	recovered := ocsc.Derandomize(randomized)
	if !recovered.Equal(block) {
		t.Error("derandomizing did not recover the original block")
	}
}

func TestRandomizeRestartsPerBlock(t *testing.T) {
	// §3.5.3.1: the sequence begins at the first digit of each block, so two
	// identical blocks randomize identically.
	a := ocsc.BitStringFromBytes([]byte("same contents"))
	b := ocsc.BitStringFromBytes([]byte("same contents"))

	if !ocsc.Randomize(a).Equal(ocsc.Randomize(b)) {
		t.Error("two identical blocks randomized differently")
	}
}

func TestBlockSizeArithmetic(t *testing.T) {
	// Table 3-1, and §3.6.1.1 plus §3.7: k-hat is k + 32 + 2.
	tests := []struct {
		rate CodeRate
		k    int
		kHat int
	}{
		{ocsc.RateOneThird, 5006, 5040},
		{ocsc.RateOneHalf, 7526, 7560},
		{ocsc.RateTwoThirds, 10046, 10080},
	}
	for _, tt := range tests {
		if got := tt.rate.InformationBlockSize(); got != tt.k {
			t.Errorf("rate %s: k = %d, want %d", tt.rate, got, tt.k)
		}
		if got := tt.rate.EncoderInputSize(); got != tt.kHat {
			t.Errorf("rate %s: k-hat = %d, want %d", tt.rate, got, tt.kHat)
		}
		if tt.k+ocsc.CRCBits+ocsc.TerminationBits != tt.kHat {
			t.Errorf("rate %s: k + 32 + 2 does not equal k-hat", tt.rate)
		}
	}
}

// CodeRate is aliased so the table above reads naturally.
type CodeRate = ocsc.CodeRate

func TestBlockSizesAreNotByteAligned(t *testing.T) {
	// This is why the whole package works in bits. If any of these were a
	// multiple of eight, an octet-oriented implementation would have been
	// fine, and a future reader might wonder why it is not.
	for _, rate := range []ocsc.CodeRate{ocsc.RateOneThird, ocsc.RateOneHalf, ocsc.RateTwoThirds} {
		if rate.InformationBlockSize()%8 == 0 {
			t.Errorf("rate %s: k = %d is byte aligned, contrary to table 3-1",
				rate, rate.InformationBlockSize())
		}
	}
}

func TestCRC32PolynomialIsTheOpticalOne(t *testing.T) {
	// §3.6.2.2: h(X) = X^32 + X^29 + X^18 + X^14 + X^3 + 1.
	// This is the fourth distinct CRC-32 in this library. None is
	// interchangeable with another.
	if ocsc.CRC32Polynomial != 0x20044009 {
		t.Errorf("polynomial = %#08x, want 0x20044009", ocsc.CRC32Polynomial)
	}
	for _, other := range []struct {
		name  string
		value uint32
	}{
		{"IEEE CRC-32", 0x04C11DB7},
		{"CRC-32C", 0x1EDC6F41},
		{"Proximity-1", 0x00A00805},
	} {
		if ocsc.CRC32Polynomial == other.value {
			t.Errorf("the polynomial is %s, which is the wrong one", other.name)
		}
	}
}

func TestCRCRoundTrip(t *testing.T) {
	block := ocsc.BitStringFromBytes([]byte("a pseudo-randomized information block"))

	withCRC := ocsc.AttachCRC(block)
	if withCRC.Len() != block.Len()+ocsc.CRCBits {
		t.Fatalf("attached block is %d digits, want %d", withCRC.Len(), block.Len()+ocsc.CRCBits)
	}

	body, ok := ocsc.VerifyCRC(withCRC)
	if !ok {
		t.Fatal("a block built with our own CRC failed verification")
	}
	if !body.Equal(block) {
		t.Error("the recovered body differs from the original block")
	}
}

func TestCRCOnNonByteAlignedBlock(t *testing.T) {
	// The real block lengths are not multiples of eight, so the partial-tail
	// path has to work.
	block := ocsc.NewBitString(5006) // rate 1/3
	for i := 0; i < block.Len(); i += 3 {
		block.SetBit(i, 1)
	}

	withCRC := ocsc.AttachCRC(block)
	body, ok := ocsc.VerifyCRC(withCRC)
	if !ok {
		t.Fatal("verification failed on a block that is not byte aligned")
	}
	if !body.Equal(block) {
		t.Error("the recovered body differs")
	}
}

func TestCRCDetectsSingleBitErrors(t *testing.T) {
	block := ocsc.BitStringFromBytes([]byte("detect me"))
	withCRC := ocsc.AttachCRC(block)

	for i := 0; i < withCRC.Len(); i++ {
		corrupt := withCRC.Slice(0, withCRC.Len())
		corrupt.SetBit(i, corrupt.Bit(i)^1)

		if _, ok := ocsc.VerifyCRC(corrupt); ok {
			t.Fatalf("a flip at digit %d went undetected", i)
		}
	}
}

func TestASMAttachAndStrip(t *testing.T) {
	// §3.3.2: the marker is 1ACFFC1D, the same one TM uses for a CADU.
	if !bytes.Equal(ocsc.DefaultASM(), []byte{0x1A, 0xCF, 0xFC, 0x1D}) {
		t.Errorf("ASM = %X, want 1ACFFC1D", ocsc.DefaultASM())
	}

	frame := []byte("a CCSDS transfer frame")
	smtf, err := ocsc.AttachASM(frame)
	if err != nil {
		t.Fatal(err)
	}
	if smtf.Len() != ocsc.ASMBits+len(frame)*8 {
		t.Fatalf("SMTF is %d digits, want %d", smtf.Len(), ocsc.ASMBits+len(frame)*8)
	}

	got, err := ocsc.StripASM(smtf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frame) {
		t.Errorf("recovered %q, want %q", got, frame)
	}
}

func TestStripASMRejectsBadMarker(t *testing.T) {
	smtf, err := ocsc.AttachASM([]byte("frame"))
	if err != nil {
		t.Fatal(err)
	}
	smtf.SetBit(5, smtf.Bit(5)^1)

	if _, err := ocsc.StripASM(smtf); !errors.Is(err, ocsc.ErrInvalidASM) {
		t.Errorf("error = %v, want ErrInvalidASM", err)
	}
}

func TestSlicerZeroFills(t *testing.T) {
	// §3.4.2.1.1: the output is zero-filled to a multiple of k.
	rate := ocsc.RateOneThird
	k := rate.InformationBlockSize()

	// One and a bit blocks' worth of input.
	stream := ocsc.NewBitString(k + 100)
	for i := 0; i < stream.Len(); i++ {
		stream.SetBit(i, 1)
	}

	blocks, err := ocsc.Slice(stream, rate)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	for i, b := range blocks {
		if b.Len() != k {
			t.Errorf("block %d is %d digits, want %d", i, b.Len(), k)
		}
	}

	// The fill must be zeros.
	for i := 100; i < k; i++ {
		if blocks[1].Bit(i) != 0 {
			t.Fatalf("digit %d of the padded block is not zero", i)
		}
	}
}

func TestSlicerRejectsInvalidRate(t *testing.T) {
	if _, err := ocsc.Slice(ocsc.NewBitString(10), ocsc.CodeRate(9)); !errors.Is(err, ocsc.ErrInvalidCodeRate) {
		t.Errorf("error = %v, want ErrInvalidCodeRate", err)
	}
}

func TestTerminationBits(t *testing.T) {
	// §3.7: two zeros.
	block := ocsc.BitStringFromBytes([]byte{0xFF, 0xFF})

	terminated := ocsc.AttachTermination(block)
	if terminated.Len() != block.Len()+ocsc.TerminationBits {
		t.Fatalf("terminated block is %d digits, want %d",
			terminated.Len(), block.Len()+ocsc.TerminationBits)
	}
	for i := block.Len(); i < terminated.Len(); i++ {
		if terminated.Bit(i) != 0 {
			t.Errorf("termination digit %d is not zero", i)
		}
	}

	stripped, err := ocsc.StripTermination(terminated)
	if err != nil {
		t.Fatal(err)
	}
	if !stripped.Equal(block) {
		t.Error("stripping the termination did not recover the block")
	}
}

func TestStripTerminationRejectsNonZero(t *testing.T) {
	block := ocsc.BitStringFromBytes([]byte{0xFF})
	terminated := ocsc.AttachTermination(block)
	terminated.SetBit(terminated.Len()-1, 1)

	if _, err := ocsc.StripTermination(terminated); !errors.Is(err, ocsc.ErrInvalidTermination) {
		t.Errorf("error = %v, want ErrInvalidTermination", err)
	}
}

func TestFullConditioningChain(t *testing.T) {
	// The whole send side, then the whole receive side.
	// Frames are a fixed length per mission phase, which is what lets the
	// receiver tell real data from the slicer's zero fill.
	const frameLength = 24
	frames := [][]byte{
		[]byte("first transfer frame 001"),
		[]byte("second transfer frame 02"),
		[]byte("third transfer frame 003"),
	}
	for i, f := range frames {
		if len(f) != frameLength {
			t.Fatalf("test frame %d is %d octets, want %d", i, len(f), frameLength)
		}
	}

	for _, rate := range []ocsc.CodeRate{ocsc.RateOneThird, ocsc.RateOneHalf, ocsc.RateTwoThirds} {
		t.Run(rate.String(), func(t *testing.T) {
			blocks, err := ocsc.Condition(frames, rate)
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) == 0 {
				t.Fatal("conditioning produced no blocks")
			}
			for i, b := range blocks {
				if b.Len() != rate.EncoderInputSize() {
					t.Fatalf("block %d is %d digits, want k-hat = %d",
						i, b.Len(), rate.EncoderInputSize())
				}
			}

			recovered, bad, err := ocsc.Recover(blocks, rate, frameLength)
			if err != nil {
				t.Fatal(err)
			}
			if len(bad) != 0 {
				t.Fatalf("blocks %v failed their CRC on a clean round trip", bad)
			}
			if len(recovered) != len(frames) {
				t.Fatalf("recovered %d frames, want %d", len(recovered), len(frames))
			}
			for i := range frames {
				if !bytes.Equal(recovered[i], frames[i]) {
					t.Errorf("frame %d = %q, want %q", i, recovered[i], frames[i])
				}
			}
		})
	}
}

func TestRecoverReportsCorruptBlocks(t *testing.T) {
	// §2: frames from an incorrectly decoded codeword are marked invalid
	// rather than silently dropped.
	frames := [][]byte{[]byte("a frame that will be damaged in transit")}

	blocks, err := ocsc.Condition(frames, ocsc.RateOneThird)
	if err != nil {
		t.Fatal(err)
	}
	blocks[0].SetBit(100, blocks[0].Bit(100)^1)

	_, bad, err := ocsc.Recover(blocks, ocsc.RateOneThird, len(frames[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 1 || bad[0] != 0 {
		t.Errorf("bad blocks = %v, want [0]", bad)
	}
}

func TestRecoverRejectsWrongBlockLength(t *testing.T) {
	blocks := []*ocsc.BitString{ocsc.NewBitString(100)}
	if _, _, err := ocsc.Recover(blocks, ocsc.RateOneThird, 0); !errors.Is(err, ocsc.ErrInvalidBlockLength) {
		t.Errorf("error = %v, want ErrInvalidBlockLength", err)
	}
}

func TestBitStringBasics(t *testing.T) {
	b := ocsc.NewBitString(0)
	for _, v := range []uint8{1, 0, 1, 1, 0, 0, 1, 0, 1} {
		b.Append(v)
	}
	if b.Len() != 9 {
		t.Fatalf("length = %d, want 9", b.Len())
	}
	if b.Bit(0) != 1 || b.Bit(1) != 0 || b.Bit(8) != 1 {
		t.Error("bits did not survive appending")
	}

	// A partial final octet must not leak bits past the length.
	if got := b.Slice(0, 9); !got.Equal(b) {
		t.Error("slicing the whole string did not reproduce it")
	}
	if mid := b.Slice(2, 5); mid.Len() != 3 {
		t.Errorf("slice length = %d, want 3", mid.Len())
	}
}

func TestBitStringFromBitsClearsTail(t *testing.T) {
	// Two strings of the same length must compare equal even when the caller
	// left junk in the final octet.
	a := ocsc.BitStringFromBits([]byte{0xFF, 0xFF}, 12)
	b := ocsc.BitStringFromBits([]byte{0xFF, 0xF0}, 12)
	if !a.Equal(b) {
		t.Error("bits past the length were not cleared")
	}
}
