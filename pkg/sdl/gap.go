package sdl

import (
	"math/bits"
	"sync"
)

// Frame gap detection.
//
// Every data link protocol in this repository counts frames per virtual
// channel in a field that wraps: eight bits for TM and TC, twenty-four for
// AOS, and a managed width for USLP — the USLP count length is a managed
// parameter of up to 56 bits (CCSDS 732.1-B-3 4.1.2.11), not a fixed size. A
// receiver notices loss by comparing the count it got against the one it
// expected, modulo that width.
//
// The arithmetic is identical in all four and only the width differs, so it
// lives here once, parameterised by the counter type and its mask. The four
// packages keep their own FrameGapDetector — the exported shapes differ, and
// TM tracks a master channel count the others do not — but the counting is no
// longer written out four times.

// Counter is a frame count field. The protocols use widths from eight bits
// (TM, TC) through twenty-four (AOS) up to USLP's managed maximum of
// fifty-six, carried in whichever unsigned type holds them.
type Counter interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64
}

// GapCounter tracks one wrapping frame count and reports how many frames went
// missing between one it saw and the next. It is safe for concurrent use.
//
// The zero value is not usable; construct with NewGapCounter, which needs the
// mask.
type GapCounter[C Counter] struct {
	// mask is the field width: 0xFF for an 8-bit count, 0xFFFFFF for 24. The
	// subtraction is done modulo this, which is what makes a wrap read as a
	// gap of zero rather than a gap of nearly the whole range.
	mask C

	mu       sync.Mutex
	expected map[uint8]C
	seen     map[uint8]bool

	lastGap int
}

// NewGapCounter returns a counter for a field of the given mask, which must be
// all ones up to the field width — 0xFF for eight bits, 0xFFFFFF for
// twenty-four, up to 0xFFFFFFFFFFFFFF for USLP's 56-bit maximum.
func NewGapCounter[C Counter](mask C) *GapCounter[C] {
	return &GapCounter[C]{
		mask:     mask,
		expected: make(map[uint8]C),
		seen:     make(map[uint8]bool),
	}
}

// Track records a frame count seen on a channel and returns the gap before it:
// how many frames are missing between the last one tracked on that channel and
// this one.
//
// The first frame on a channel reports a gap of zero. There is nothing to
// compare it against, and reporting the distance from an assumed zero would
// invent a loss that did not happen — a receiver joining a pass in progress
// would report a gap of however far the counter had already run.
func (g *GapCounter[C]) Track(channel uint8, count C) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.track(channel, count, g.mask)
}

// TrackWithCycle records a frame count together with the cycle counter some
// protocols pair it with, and returns the gap before it. AOS is the one that
// needs this: when the VC frame count usage flag is set, the signaling field
// carries a four-bit VC frame count cycle that increments each time the
// 24-bit count wraps (CCSDS 732.0-B-4 4.1.2.5.5.3), and the pair behaves as
// one 28-bit count. cycleMask is the cycle field's width in the same
// all-ones form as the count mask: 0xF for AOS's four bits.
//
// The combined count — cycle bits above the count bits — must fit in C. Do
// not mix Track and TrackWithCycle on the same channel: the two disagree
// about the modulus, so a gap computed across the switch would be wrong.
func (g *GapCounter[C]) TrackWithCycle(channel uint8, count C, cycle uint8, cycleMask uint8) int {
	shift := bits.Len64(uint64(g.mask))
	folded := C(cycle&cycleMask)<<shift | (count & g.mask)
	mask := C(cycleMask)<<shift | g.mask
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.track(channel, folded, mask)
}

// track is the shared arithmetic; the caller holds g.mu.
func (g *GapCounter[C]) track(channel uint8, count C, mask C) int {
	if g.seen[channel] {
		g.lastGap = int((count - g.expected[channel]) & mask)
	} else {
		g.seen[channel] = true
		g.lastGap = 0
	}
	g.expected[channel] = (count + 1) & mask
	return g.lastGap
}

// LastGap returns the gap reported by the most recent Track, across all
// channels.
func (g *GapCounter[C]) LastGap() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastGap
}

// Reset forgets every channel, so the next frame on each is treated as a
// first frame again.
func (g *GapCounter[C]) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	clear(g.expected)
	clear(g.seen)
	g.lastGap = 0
}
