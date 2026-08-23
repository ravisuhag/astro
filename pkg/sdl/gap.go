package sdl

// Frame gap detection.
//
// Every data link protocol in this repository counts frames per virtual
// channel in a field that wraps: eight bits for TM and TC, sixteen for USLP,
// twenty-four for AOS. A receiver notices loss by comparing the count it got
// against the one it expected, modulo that width.
//
// The arithmetic is identical in all four and only the width differs, so it
// lives here once, parameterised by the counter type and its mask. The four
// packages keep their own FrameGapDetector — the exported shapes differ, and
// TM tracks a master channel count the others do not — but the counting is no
// longer written out four times.

// Counter is a frame count field. The protocols use widths from eight to
// twenty-four bits, carried in whichever unsigned type holds them.
type Counter interface {
	~uint8 | ~uint16 | ~uint32
}

// GapCounter tracks one wrapping frame count and reports how many frames went
// missing between one it saw and the next.
//
// The zero value is not usable; construct with NewGapCounter, which needs the
// mask.
type GapCounter[C Counter] struct {
	// mask is the field width: 0xFF for an 8-bit count, 0xFFFFFF for 24. The
	// subtraction is done modulo this, which is what makes a wrap read as a
	// gap of zero rather than a gap of nearly the whole range.
	mask C

	expected map[uint8]C
	seen     map[uint8]bool

	lastGap int
}

// NewGapCounter returns a counter for a field of the given mask, which must be
// all ones up to the field width — 0xFF for eight bits, 0xFFFF for sixteen,
// 0xFFFFFF for twenty-four.
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
	if g.seen[channel] {
		g.lastGap = int((count - g.expected[channel]) & g.mask)
	} else {
		g.seen[channel] = true
		g.lastGap = 0
	}
	g.expected[channel] = (count + 1) & g.mask
	return g.lastGap
}

// LastGap returns the gap reported by the most recent Track, across all
// channels.
func (g *GapCounter[C]) LastGap() int { return g.lastGap }

// Reset forgets every channel, so the next frame on each is treated as a
// first frame again.
func (g *GapCounter[C]) Reset() {
	clear(g.expected)
	clear(g.seen)
	g.lastGap = 0
}
