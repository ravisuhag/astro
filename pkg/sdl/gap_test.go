package sdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// TestGapCounterFirstFrameIsNotAGap pins the rule that matters most to a
// receiver joining a pass already in progress: the first frame on a channel
// has nothing to compare against and must report zero, not the distance from
// an assumed count of zero.
func TestGapCounterFirstFrameIsNotAGap(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	if gap := g.Track(0, 200); gap != 0 {
		t.Errorf("first frame at count 200 reported a gap of %d, want 0", gap)
	}
	if gap := g.Track(0, 201); gap != 0 {
		t.Errorf("the next in sequence reported a gap of %d, want 0", gap)
	}
}

func TestGapCounterCountsMissingFrames(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	g.Track(0, 10)
	if gap := g.Track(0, 11); gap != 0 {
		t.Errorf("consecutive frames gave %d, want 0", gap)
	}
	if gap := g.Track(0, 15); gap != 3 {
		t.Errorf("12, 13 and 14 missing gave %d, want 3", gap)
	}
	if gap := g.LastGap(); gap != 3 {
		t.Errorf("LastGap() = %d, want 3", gap)
	}
}

// TestGapCounterWrapsCleanly is why the subtraction is masked. A count that
// rolls over from 255 to 0 is the next frame, not 255 lost ones.
func TestGapCounterWrapsCleanly(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	g.Track(0, 254)
	if gap := g.Track(0, 255); gap != 0 {
		t.Errorf("254 to 255 gave %d, want 0", gap)
	}
	if gap := g.Track(0, 0); gap != 0 {
		t.Errorf("255 to 0 gave %d, want 0 — the counter wrapped", gap)
	}
	if gap := g.Track(0, 2); gap != 1 {
		t.Errorf("0 to 2 gave %d, want 1", gap)
	}
}

// TestGapCounterPerChannel checks the counters are independent: traffic on one
// virtual channel must not look like loss on another.
func TestGapCounterPerChannel(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	g.Track(1, 100)
	g.Track(2, 50)

	if gap := g.Track(1, 101); gap != 0 {
		t.Errorf("channel 1 continuing gave %d, want 0", gap)
	}
	if gap := g.Track(2, 51); gap != 0 {
		t.Errorf("channel 2 continuing gave %d, want 0", gap)
	}
	if gap := g.Track(1, 105); gap != 3 {
		t.Errorf("channel 1 skipping gave %d, want 3", gap)
	}
	// Channel 2 is unaffected by channel 1's loss.
	if gap := g.Track(2, 52); gap != 0 {
		t.Errorf("channel 2 after channel 1 lost frames gave %d, want 0", gap)
	}
}

// TestGapCounterWidths covers the three field widths the protocols use, each
// wrapping at its own boundary.
func TestGapCounterWidths(t *testing.T) {
	t.Run("8-bit, TM and TC", func(t *testing.T) {
		g := sdl.NewGapCounter[uint8](0xFF)
		g.Track(0, 0xFE)
		if gap := g.Track(0, 0x01); gap != 2 {
			t.Errorf("gap = %d, want 2 across the 8-bit wrap", gap)
		}
	})

	t.Run("16-bit, USLP", func(t *testing.T) {
		g := sdl.NewGapCounter[uint16](0xFFFF)
		g.Track(0, 0xFFFE)
		if gap := g.Track(0, 0x0001); gap != 2 {
			t.Errorf("gap = %d, want 2 across the 16-bit wrap", gap)
		}
		// A value that would wrap an 8-bit counter must not wrap this one.
		g2 := sdl.NewGapCounter[uint16](0xFFFF)
		g2.Track(0, 250)
		if gap := g2.Track(0, 260); gap != 9 {
			t.Errorf("gap = %d, want 9; a 16-bit counter does not wrap at 255", gap)
		}
	})

	t.Run("24-bit, AOS", func(t *testing.T) {
		g := sdl.NewGapCounter[uint32](0xFFFFFF)
		g.Track(0, 0xFFFFFE)
		if gap := g.Track(0, 0x000001); gap != 2 {
			t.Errorf("gap = %d, want 2 across the 24-bit wrap", gap)
		}
		g2 := sdl.NewGapCounter[uint32](0xFFFFFF)
		g2.Track(0, 0xFFFF)
		if gap := g2.Track(0, 0x10005); gap != 5 {
			t.Errorf("gap = %d, want 5; a 24-bit counter does not wrap at 65535", gap)
		}
	})

	t.Run("56-bit, USLP managed maximum", func(t *testing.T) {
		// The USLP count length is managed, up to 56 bits (CCSDS 732.1-B-3
		// 4.1.2.11); the widest configuration needs a uint64 counter.
		g := sdl.NewGapCounter[uint64](0xFFFFFFFFFFFFFF)
		g.Track(0, 0xFFFFFFFFFFFFFE)
		if gap := g.Track(0, 0x00000000000001); gap != 2 {
			t.Errorf("gap = %d, want 2 across the 56-bit wrap", gap)
		}
		g2 := sdl.NewGapCounter[uint64](0xFFFFFFFFFFFFFF)
		g2.Track(0, 0xFFFFFFFF)
		if gap := g2.Track(0, 0x100000003); gap != 3 {
			t.Errorf("gap = %d, want 3; a 56-bit counter does not wrap at 2^32", gap)
		}
	})
}

// TestGapCounterWithCycle covers the AOS pairing of the 24-bit VC frame count
// with the four-bit frame count cycle from the signaling field (CCSDS
// 732.0-B-4 4.1.2.5.5): together they behave as one 28-bit count, so a wrap
// of the 24-bit field accompanied by a cycle increment is not a gap.
func TestGapCounterWithCycle(t *testing.T) {
	g := sdl.NewGapCounter[uint32](0xFFFFFF)

	if gap := g.TrackWithCycle(0, 0xFFFFFE, 0, 0xF); gap != 0 {
		t.Fatalf("first frame gave %d, want 0", gap)
	}
	if gap := g.TrackWithCycle(0, 0xFFFFFF, 0, 0xF); gap != 0 {
		t.Errorf("consecutive frames gave %d, want 0", gap)
	}
	// The 24-bit count wraps and the cycle increments: still consecutive.
	if gap := g.TrackWithCycle(0, 0x000000, 1, 0xF); gap != 0 {
		t.Errorf("count wrap with cycle increment gave %d, want 0", gap)
	}
	// Losing a whole cycle's worth of frames is visible, where the bare
	// 24-bit arithmetic would have folded it away as zero.
	if gap := g.TrackWithCycle(0, 0x000001, 2, 0xF); gap != 0xFFFFFF+1 {
		t.Errorf("one full cycle lost gave %d, want %d", gap, 0xFFFFFF+1)
	}
}

// TestGapCounterWithCycleWraps checks the far edge of the 28-bit combined
// count: cycle 15 rolling over to cycle 0 is a wrap, not a loss.
func TestGapCounterWithCycleWraps(t *testing.T) {
	g := sdl.NewGapCounter[uint32](0xFFFFFF)

	g.TrackWithCycle(0, 0xFFFFFF, 0xF, 0xF)
	if gap := g.TrackWithCycle(0, 0x000000, 0x0, 0xF); gap != 0 {
		t.Errorf("28-bit wrap gave %d, want 0", gap)
	}
}

// TestGapCounterMaximumGap checks the far edge: a count one behind the
// expected reads as the largest possible gap, not as a negative.
func TestGapCounterMaximumGap(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	g.Track(0, 10)
	// Expected 11; receiving 10 again is 255 frames later modulo 256, which
	// is what the standard's arithmetic says. It is indistinguishable from a
	// duplicate, and the detector does not try to guess which.
	if gap := g.Track(0, 10); gap != 255 {
		t.Errorf("a repeated count gave %d, want 255", gap)
	}
}

func TestGapCounterReset(t *testing.T) {
	g := sdl.NewGapCounter[uint8](0xFF)

	g.Track(0, 10)
	g.Track(0, 20)
	if g.LastGap() == 0 {
		t.Fatal("expected a gap before reset")
	}

	g.Reset()
	if g.LastGap() != 0 {
		t.Errorf("LastGap() = %d after Reset, want 0", g.LastGap())
	}
	if gap := g.Track(0, 200); gap != 0 {
		t.Errorf("after Reset the next frame gave %d, want 0 as a first frame", gap)
	}
}
