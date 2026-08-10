package rhc_test

import (
	"bytes"
	"errors"
	"math/rand"
	"strconv"
	"testing"

	"github.com/ravisuhag/astro/pkg/rhc"
)

// coded is one compressed output vector, kept with its true bit length.
type codedVector struct {
	data   []byte
	bitLen int
}

// compressStream compresses a whole stream.
func compressStream(t *testing.T, config rhc.Config, packets [][]byte) []codedVector {
	t.Helper()

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatalf("NewCompressor() = %v", err)
	}

	out := make([]codedVector, len(packets))
	for i, packet := range packets {
		data, bitLen, err := compressor.Compress(packet)
		if err != nil {
			t.Fatalf("packet %d: Compress() = %v", i, err)
		}
		out[i] = codedVector{data: data, bitLen: bitLen}
	}
	return out
}

// TestLossRecovery is the test this package exists to pass.
//
// A stream is compressed, some outputs are thrown away, and the survivors are
// fed to a decompressor that is told about the gaps — which §2.2 says is the
// mission's job, since the standard "does not provide a mechanism for
// identifying the number of sequential output binary vectors that were lost".
//
// The claim being checked is the one that matters: every vector the
// decompressor returns is byte-identical to the original. It may refuse, and
// after a gap it must refuse until an output reaches back far enough, but it
// must never hand back something wrong.
func TestLossRecovery(t *testing.T) {
	robustnessLevels := []int{0, 1, 3, 7}
	dropRates := []float64{0.05, 0.2, 0.5}

	for _, robustness := range robustnessLevels {
		for _, dropRate := range dropRates {
			name := "robustness" + itoa(robustness) + "_drop" + itoa(int(dropRate*100))
			t.Run(name, func(t *testing.T) {
				config := rhc.Config{
					VectorLength: 128,
					Robustness:   robustness,
					// Send the whole mask and a whole input regularly, which
					// is what lets a decompressor recover from a long gap.
					SendMaskInterval:     16,
					UncompressedInterval: 16,
					NewMaskInterval:      24,
				}
				packets := housekeeping(300, config.VectorLength, 2, int64(robustness*100+int(dropRate*100)))
				stream := compressStream(t, config, packets)

				decompressor, err := rhc.NewDecompressor(config)
				if err != nil {
					t.Fatalf("NewDecompressor() = %v", err)
				}

				rng := rand.New(rand.NewSource(7))
				dropped := 0
				delivered := 0
				refused := 0
				pendingGap := 0

				for i, unit := range stream {
					// Never drop the very first output; a stream has to start.
					if i > 0 && rng.Float64() < dropRate {
						dropped++
						pendingGap++
						continue
					}

					if pendingGap > 0 {
						decompressor.NotifyLoss(pendingGap)
						pendingGap = 0
					}

					back, err := decompressor.Decompress(unit.data, unit.bitLen)
					if err != nil {
						refused++
						continue
					}

					delivered++
					if !bytes.Equal(back, packets[i]) {
						t.Fatalf("packet %d came back wrong after %d drops:\n got %08b\nwant %08b",
							i, dropped, back, packets[i])
					}
				}

				if dropped == 0 {
					t.Fatal("no outputs were dropped; the test proved nothing")
				}
				if delivered == 0 {
					t.Fatal("nothing was delivered; the decompressor never recovered")
				}
				t.Logf("dropped %d, delivered %d, refused %d", dropped, delivered, refused)
			})
		}
	}
}

// itoa is strconv.Itoa under a shorter name, for building subtest names.
func itoa(i int) string { return strconv.Itoa(i) }

// TestLossWithinRobustnessRecoversImmediately pins the guarantee of §2.1: "the
// mask can be synchronized even if the number of consecutive output binary
// vectors lost immediately before this output bit vector is equal to, or less
// than, the effective robustness level."
//
// With robustness 3 and a gap of 3, the next output must decode. The stream
// must also be one where the previous input is still tracked — so the test
// drops nothing that carries an uncompressed vector.
func TestLossWithinRobustnessRecoversImmediately(t *testing.T) {
	config := rhc.Config{
		VectorLength:         64,
		Robustness:           3,
		UncompressedInterval: 4,
		SendMaskInterval:     4,
	}
	packets := housekeeping(40, config.VectorLength, 1, 11)
	stream := compressStream(t, config, packets)

	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}

	// Decode the first ten normally.
	for i := range 10 {
		if _, err := decompressor.Decompress(stream[i].data, stream[i].bitLen); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}

	// Drop three, which the robustness level covers, then decode the next.
	decompressor.NotifyLoss(3)
	back, err := decompressor.Decompress(stream[13].data, stream[13].bitLen)
	if err != nil {
		t.Fatalf("a gap of 3 at robustness 3 should decode, got %v", err)
	}
	if !bytes.Equal(back, packets[13]) {
		t.Errorf("packet 13 came back wrong:\n got %08b\nwant %08b", back, packets[13])
	}
}

// TestLossBeyondRobustnessIsRefused is the other half: a gap the output cannot
// reach across must produce an error, not a guess.
func TestLossBeyondRobustnessIsRefused(t *testing.T) {
	config := rhc.Config{VectorLength: 64, Robustness: 1}
	packets := housekeeping(40, config.VectorLength, 3, 12)
	stream := compressStream(t, config, packets)

	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		if _, err := decompressor.Decompress(stream[i].data, stream[i].bitLen); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}

	// Drop ten, far past what robustness 1 covers.
	decompressor.NotifyLoss(10)
	if _, err := decompressor.Decompress(stream[20].data, stream[20].bitLen); err == nil {
		t.Fatal("a gap of 10 at robustness 1 was accepted; it cannot be reconstructed")
	}
	if decompressor.Synchronized() {
		t.Error("the decompressor still claims to be synchronized")
	}
}

// TestUncompressedOutputAlwaysRecovers checks the recovery lever: an output
// carrying the whole input vector restores a decompressor from any state.
func TestUncompressedOutputAlwaysRecovers(t *testing.T) {
	config := rhc.Config{VectorLength: 64, Robustness: 2}
	packets := housekeeping(30, config.VectorLength, 4, 13)

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}

	stream := make([]codedVector, len(packets))
	for i, packet := range packets {
		if i == 20 {
			// Force a whole mask and a whole input at index 20.
			compressor.ForceSendMask()
			compressor.ForceUncompressed()
		}
		data, bitLen, err := compressor.Compress(packet)
		if err != nil {
			t.Fatal(err)
		}
		stream[i] = codedVector{data: data, bitLen: bitLen}
	}

	// Decode the first few, then lose everything up to index 20.
	for i := range 5 {
		if _, err := decompressor.Decompress(stream[i].data, stream[i].bitLen); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}
	decompressor.NotifyLoss(15)

	back, err := decompressor.Decompress(stream[20].data, stream[20].bitLen)
	if err != nil {
		t.Fatalf("an uncompressed output should recover from any gap, got %v", err)
	}
	if !bytes.Equal(back, packets[20]) {
		t.Errorf("packet 20 came back wrong:\n got %08b\nwant %08b", back, packets[20])
	}
	if !decompressor.Synchronized() {
		t.Error("the decompressor is not synchronized after an uncompressed output")
	}

	// And the stream continues.
	for i := 21; i < len(packets); i++ {
		got, err := decompressor.Decompress(stream[i].data, stream[i].bitLen)
		if err != nil {
			t.Fatalf("packet %d after recovery: %v", i, err)
		}
		if !bytes.Equal(got, packets[i]) {
			t.Fatalf("packet %d came back wrong after recovery", i)
		}
	}
}

// TestFreshDecompressorRefusesUntilSynchronized pins the contract on the type:
// before anything has established state, nothing is returned.
func TestFreshDecompressorRefusesUntilSynchronized(t *testing.T) {
	config := rhc.Config{VectorLength: 64, Robustness: 0}
	packets := housekeeping(10, config.VectorLength, 2, 14)

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	// The first output at robustness 0 carries everything, per §3.3.2c and d
	// forcing both flags while t <= R_t. Skip it and the next one cannot be
	// decoded.
	if _, _, err := compressor.Compress(packets[0]); err != nil {
		t.Fatal(err)
	}
	second, secondLen, err := compressor.Compress(packets[1])
	if err != nil {
		t.Fatal(err)
	}

	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	if decompressor.Synchronized() {
		t.Error("a fresh decompressor claims to be synchronized")
	}

	_, err = decompressor.Decompress(second, secondLen)
	if err == nil {
		t.Fatal("the second output decoded without the first")
	}
	if !errors.Is(err, rhc.ErrNotSynchronized) && !errors.Is(err, rhc.ErrMaskUnavailable) {
		t.Errorf("err = %v, want ErrNotSynchronized or ErrMaskUnavailable", err)
	}
}

// TestResetRequiresResynchronization checks Reset really does clear state.
func TestResetRequiresResynchronization(t *testing.T) {
	config := rhc.Config{VectorLength: 64, Robustness: 0}
	packets := housekeeping(10, config.VectorLength, 2, 15)
	stream := compressStream(t, config, packets)

	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if _, err := decompressor.Decompress(stream[i].data, stream[i].bitLen); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}
	if !decompressor.Synchronized() {
		t.Fatal("the decompressor should be synchronized after five outputs")
	}

	decompressor.Reset()
	if decompressor.Synchronized() {
		t.Error("the decompressor is still synchronized after Reset")
	}
	if _, err := decompressor.Decompress(stream[5].data, stream[5].bitLen); err == nil {
		t.Error("an output decoded straight after Reset")
	}
}

// TestMalformedInputDoesNotPoisonState checks that a bad output vector leaves
// a working decompressor working. Parsing completes before any state changes,
// so a failed parse changes nothing.
func TestMalformedInputDoesNotPoisonState(t *testing.T) {
	config := rhc.Config{VectorLength: 64, Robustness: 2}
	packets := housekeeping(20, config.VectorLength, 2, 16)
	stream := compressStream(t, config, packets)

	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if _, err := decompressor.Decompress(stream[i].data, stream[i].bitLen); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}

	// Feed a series of corrupt vectors between good ones.
	corrupt := [][]byte{
		{},
		{0xFF},
		{0x00, 0x00},
		bytes.Repeat([]byte{0xAA}, 8),
		bytes.Repeat([]byte{0xFF}, 64),
	}
	for _, bad := range corrupt {
		_, _ = decompressor.Decompress(bad, 0)
	}

	// The stream must still decode.
	for i := 5; i < len(packets); i++ {
		got, err := decompressor.Decompress(stream[i].data, stream[i].bitLen)
		if err != nil {
			t.Fatalf("packet %d after corrupt input: %v", i, err)
		}
		if !bytes.Equal(got, packets[i]) {
			t.Fatalf("packet %d came back wrong after corrupt input", i)
		}
	}
}
