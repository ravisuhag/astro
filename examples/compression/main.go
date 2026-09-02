// Example: Compressing data before you downlink it
//
// A downlink is the scarcest thing a mission has. Two CCSDS standards shrink
// what goes over it, and they solve completely different problems:
//
//	LDC (CCSDS 121.0), pkg/ldc
//	  Science data. Decorrelate neighbouring samples, then entropy code the
//	  residuals. Lossless, integer arithmetic throughout.
//
//	RHC (CCSDS 124.0), pkg/rhc
//	  Housekeeping. Most bits are the same as the last packet, so send only
//	  the ones that changed. Built for a lossy link, with a caveat.
//
// Both produce ordinary octets, so they sit above the packet layer and know
// nothing about frames.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/ravisuhag/astro/pkg/ldc"
	"github.com/ravisuhag/astro/pkg/rhc"
)

func main() {
	scienceData()
	whichOptionWon()
	housekeeping()
	housekeepingUnderLoss()
}

// scienceData runs LDC over a synthetic instrument readout: 12-bit samples
// from a detector scanning a smooth scene, which is what makes them
// compressible. Neighbouring samples are close, so their differences are
// small, and small numbers cost few bits.
func scienceData() {
	fmt.Println("--- LDC: lossless science data ---")
	fmt.Println()

	samples := detectorScan(4096)

	params := ldc.DefaultParams()
	params.Resolution = 12                    // 12-bit ADC
	params.BlockSize = 16                     // J, a common choice
	params.Predictor = ldc.PredictorUnitDelay // subtract the previous sample
	params.ReferenceInterval = 128            // r, how often an uncoded sample goes in
	if err := params.Validate(); err != nil {
		log.Fatalf("compression parameters: %v", err)
	}

	// CompressFile writes the section 7 file format, whose header carries the
	// parameters. Compress alone is the barer call, for a mission that already
	// shares the configuration out of band.
	compressed, err := ldc.CompressFile(samples, params, 2)
	if err != nil {
		log.Fatalf("compressing: %v", err)
	}

	raw := len(samples) * 2 // 12-bit samples in 16-bit words, as they arrive
	fmt.Printf("  samples ......... %d at %d bits\n", len(samples), params.Resolution)
	fmt.Printf("  raw ............. %d octets\n", raw)
	fmt.Printf("  compressed ...... %d octets\n", len(compressed))
	fmt.Printf("  ratio ........... %.2f:1\n", float64(raw)/float64(len(compressed)))
	fmt.Printf("  bits per sample . %.2f\n", float64(len(compressed)*8)/float64(len(samples)))

	back, err := ldc.DecompressFile(compressed)
	if err != nil {
		log.Fatalf("decompressing: %v", err)
	}
	fmt.Printf("  lossless ........ %t\n", equal(samples, back))
	fmt.Println()

	// The predictor is doing most of the work. Without it the entropy coder
	// sees the raw sample values, which are large and nearly uniform.
	bare := params
	bare.Predictor = ldc.PredictorNone
	withoutPredictor, err := ldc.CompressFile(samples, bare, 2)
	if err != nil {
		log.Fatalf("compressing without the predictor: %v", err)
	}
	fmt.Printf("  without the predictor ... %d octets, %.2f:1\n",
		len(withoutPredictor), float64(raw)/float64(len(withoutPredictor)))
	fmt.Println("  Decorrelation is the whole game. The entropy coder can only")
	fmt.Println("  exploit a skew that something else created.")
	fmt.Println()
}

// whichOptionWon inspects the coded stream. The coder prices every option it
// has against each block and writes the cheapest, and Analyze reports what it
// chose. That is how you check a parameter choice against real data instead of
// guessing.
func whichOptionWon() {
	fmt.Println("--- LDC: which code option won each block ---")
	fmt.Println()

	// Three regimes in one scan: quiet, noisy, and completely flat.
	samples := make([]uint32, 0, 768)
	samples = append(samples, quiet(256)...)
	samples = append(samples, noisy(256)...)
	samples = append(samples, flat(256)...)

	params := ldc.DefaultParams()
	params.Resolution = 12
	params.BlockSize = 16
	params.ReferenceInterval = 128

	compressed, err := ldc.Compress(samples, params)
	if err != nil {
		log.Fatalf("compressing: %v", err)
	}

	blocks, err := ldc.Analyze(compressed, params, len(samples))
	if err != nil {
		log.Fatalf("analyzing: %v", err)
	}

	// Count how often each option was chosen. Keep the tally in a slice
	// rather than a map so the output is the same on every run.
	type tally struct {
		option string
		count  int
	}
	var counts []tally
	for _, block := range blocks {
		name := block.Option.String()
		found := false
		for i := range counts {
			if counts[i].option == name {
				counts[i].count++
				found = true
				break
			}
		}
		if !found {
			counts = append(counts, tally{option: name, count: 1})
		}
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].count > counts[j].count })

	fmt.Printf("  %d coded data sets over %d samples\n", len(blocks), len(samples))
	fmt.Println()
	for _, block := range blocks[:4] {
		fmt.Printf("  block %3d  %-22s k=%d  %3d bits\n",
			block.Block, block.Option, block.K, block.Bits)
	}
	fmt.Println("  ...")
	for _, block := range blocks[len(blocks)-3:] {
		fmt.Printf("  block %3d  %-22s k=%d  %3d bits  run=%d\n",
			block.Block, block.Option, block.K, block.Bits, block.ZeroRun)
	}
	fmt.Println()
	fmt.Println("  Option totals:")
	for _, t := range counts {
		fmt.Printf("    %-22s %d\n", t.option, t.count)
	}
	fmt.Println()
}

// housekeeping runs RHC over a stream of housekeeping packets. The point is
// that they barely change: a voltage wobbles in its low bits, a mode word does
// not move for hours, and most of the packet is identical to the last one.
func housekeeping() {
	fmt.Println("--- RHC: housekeeping that barely changes ---")
	fmt.Println()

	const packetOctets = 64

	config := rhc.Config{
		VectorLength: packetOctets * 8,

		// How many consecutive outputs may be lost before this one and still
		// leave the mask recoverable. Higher costs bits on every output.
		Robustness: 3,

		// The three intervals are policy, not protocol. The standard makes
		// each a per-cycle decision and says nothing about when to make it.
		NewMaskInterval:      32, // let positions go back to being predictable
		SendMaskInterval:     16, // ship the whole mask, for a lost receiver
		UncompressedInterval: 16, // ship the whole packet, the only real reset
	}
	if err := config.Validate(); err != nil {
		log.Fatalf("RHC config: %v", err)
	}

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		log.Fatalf("building the compressor: %v", err)
	}
	decompressor, err := rhc.NewDecompressor(config)
	if err != nil {
		log.Fatalf("building the decompressor: %v", err)
	}

	var rawBits, codedBits int
	lossless := true

	for cycle := range 64 {
		packet := housekeepingPacket(cycle, packetOctets)

		coded, bitLen, err := compressor.Compress(packet)
		if err != nil {
			log.Fatalf("compressing cycle %d: %v", cycle, err)
		}

		back, err := decompressor.Decompress(coded, bitLen)
		if err != nil {
			log.Fatalf("decompressing cycle %d: %v", cycle, err)
		}
		if !bytes.Equal(back, packet) {
			lossless = false
		}

		rawBits += packetOctets * 8
		codedBits += bitLen

		switch cycle {
		case 0:
			fmt.Printf("  cycle %2d ... %4d bits  (nothing is predictable yet)\n", cycle, bitLen)
		case 1, 2, 3:
			fmt.Printf("  cycle %2d ... %4d bits  (still learning what moves)\n", cycle, bitLen)
		case 4, 5:
			fmt.Printf("  cycle %2d ... %4d bits  (the mask has settled)\n", cycle, bitLen)
		case 16:
			fmt.Printf("  cycle %2d ... %4d bits  (the uncompressed interval came round)\n", cycle, bitLen)
		case 17:
			fmt.Printf("  cycle %2d ... %4d bits\n", cycle, bitLen)
		}
	}

	fmt.Println("  ...")
	fmt.Printf("  raw ............. %d bits over 64 packets\n", rawBits)
	fmt.Printf("  coded ........... %d bits\n", codedBits)
	fmt.Printf("  ratio ........... %.2f:1\n", float64(rawBits)/float64(codedBits))
	fmt.Printf("  lossless ........ %t\n", lossless)
	fmt.Println()
}

// housekeepingUnderLoss is the part worth reading twice. RHC is built for a
// lossy link, but the decompressor cannot tell that anything was lost. Clause
// 2.2 says so outright and points at packet sequence counters as the answer,
// which means noticing the gap is the caller's job.
func housekeepingUnderLoss() {
	fmt.Println("--- RHC: what a lost output actually costs ---")
	fmt.Println()

	const packetOctets = 32

	// Robustness 1 tolerates one lost output. The link is about to lose four
	// in a row, so the stream's own recovery cannot reach back across it.
	config := rhc.Config{
		VectorLength:         packetOctets * 8,
		Robustness:           1,
		NewMaskInterval:      32,
		SendMaskInterval:     16,
		UncompressedInterval: 16,
	}

	compressor, err := rhc.NewCompressor(config)
	if err != nil {
		log.Fatalf("building the compressor: %v", err)
	}
	told, err := rhc.NewDecompressor(config) // told about the gap
	if err != nil {
		log.Fatalf("building a decompressor: %v", err)
	}
	notTold, err := rhc.NewDecompressor(config) // not told
	if err != nil {
		log.Fatalf("building a decompressor: %v", err)
	}

	// Drop four consecutive outputs, more than Robustness allows for.
	const dropFrom, dropCount = 4, 4

	// Two different failures, and only one of them is safe: an output the
	// decompressor refuses, and an output it accepts and gets wrong.
	var toldRefused, toldSilentlyWrong int
	var notToldRefused, notToldSilentlyWrong int
	delivered := 0
	for cycle := range 24 {
		packet := housekeepingPacket(cycle, packetOctets)

		coded, bitLen, err := compressor.Compress(packet)
		if err != nil {
			log.Fatalf("compressing cycle %d: %v", cycle, err)
		}

		if cycle >= dropFrom && cycle < dropFrom+dropCount {
			continue
		}
		if cycle == dropFrom+dropCount {
			// The caller spotted a sequence counter gap and said so. Nothing
			// in the coded stream would have told it.
			told.NotifyLoss(dropCount)
		}
		delivered++

		switch back, err := told.Decompress(coded, bitLen); {
		case err != nil:
			toldRefused++
		case !bytes.Equal(back, packet):
			toldSilentlyWrong++
		}

		switch back, err := notTold.Decompress(coded, bitLen); {
		case err != nil:
			notToldRefused++
		case !bytes.Equal(back, packet):
			notToldSilentlyWrong++
		}
	}

	fmt.Printf("  robustness %d, %d consecutive outputs dropped, %d delivered\n",
		config.Robustness, dropCount, delivered)
	fmt.Println()
	fmt.Printf("  %-24s %-10s %s\n", "", "refused", "silently wrong")
	fmt.Printf("  %-24s %7d    %11d\n", "NotifyLoss called", toldRefused, toldSilentlyWrong)
	fmt.Printf("  %-24s %7d    %11d\n", "NotifyLoss not called", notToldRefused, notToldSilentlyWrong)
	fmt.Println()
	fmt.Println("  Both lose the same packets. Only one of them says so.")
	fmt.Println("  A decompressor that is not told about a gap hands back wrong")
	fmt.Println("  octets and reports no error, and there are no sync markers")
	fmt.Println("  either: framing and gap detection are the mission's job.")
	fmt.Println()
}

// detectorScan is a smooth scene: a slow ramp with a little noise, which is
// what a detector sweeping across something looks like.
func detectorScan(n int) []uint32 {
	samples := make([]uint32, n)
	for i := range samples {
		base := 2048 + 900*math.Sin(float64(i)/512)
		noise := float64((i*2654435761)%17) - 8 // deterministic, small
		samples[i] = clamp12(base + noise)
	}
	return samples
}

// quiet, noisy and flat are three statistical regimes, so the coder has a
// reason to switch options partway through the stream.
func quiet(n int) []uint32 {
	samples := make([]uint32, n)
	for i := range samples {
		samples[i] = clamp12(1000 + float64((i*7)%3))
	}
	return samples
}

func noisy(n int) []uint32 {
	samples := make([]uint32, n)
	for i := range samples {
		samples[i] = clamp12(float64((i * 2654435761) % 4096))
	}
	return samples
}

func flat(n int) []uint32 {
	samples := make([]uint32, n)
	for i := range samples {
		samples[i] = 2048
	}
	return samples
}

// housekeepingPacket is a plausible 64-octet housekeeping packet whose values
// hardly move: a counter, a voltage that wobbles in its low bits, a
// temperature that drifts, a mode word that never changes, and a lot of zeros.
func housekeepingPacket(cycle, size int) []byte {
	packet := make([]byte, size)

	binary.BigEndian.PutUint32(packet[0:], uint32(cycle)) // mission elapsed time
	binary.BigEndian.PutUint16(packet[4:], uint16(28100+(cycle*7)%9))
	binary.BigEndian.PutUint16(packet[6:], uint16(2250+cycle/8))
	packet[8] = 1 // mode: nominal, for the whole run
	packet[9] = 0x5A
	// The rest stays zero, which is what makes housekeeping compressible.
	return packet
}

func clamp12(v float64) uint32 {
	switch {
	case v < 0:
		return 0
	case v > 4095:
		return 4095
	default:
		return uint32(v)
	}
}

func equal(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
