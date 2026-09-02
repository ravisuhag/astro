package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// testProtocol is a stand-in for a real protocol, so the loop can be driven
// with exact counter values rather than through an encoder. One frame is four
// octets: vcid, count, cycle, and a usage flag.
func testProtocol(vcMask, mcMask uint64, cycleMask uint8) frameProtocol {
	return frameProtocol{
		vcMask:    vcMask,
		mcMask:    mcMask,
		cycleMask: cycleMask,
		ident: func(raw []byte) (frameIdent, error) {
			if len(raw) < 4 {
				return frameIdent{}, fmt.Errorf("short frame")
			}
			if raw[0] == 0xFF {
				return frameIdent{}, fmt.Errorf("unreadable frame")
			}
			return frameIdent{
				scid:     1,
				vcid:     raw[0],
				vcCount:  uint64(raw[1]),
				cycle:    raw[2],
				hasCycle: raw[3] != 0,
				hasMC:    mcMask != 0,
				mcCount:  uint64(raw[1]),
			}, nil
		},
	}
}

func scan(t *testing.T, proto frameProtocol, frames ...[]byte) (string, string) {
	t.Helper()

	var out, errOut bytes.Buffer
	scanner := newGapScanner(proto, &out, &errOut)

	var stream []byte
	for _, f := range frames {
		stream = append(stream, f...)
	}
	if err := streamFixed(bytes.NewReader(stream), 4, scanner.track, nil); err != nil {
		t.Fatalf("streamFixed: %v", err)
	}
	scanner.summary()

	return out.String(), errOut.String()
}

func frame(vcid, count, cycle byte, usage bool) []byte {
	flag := byte(0)
	if usage {
		flag = 1
	}
	return []byte{vcid, count, cycle, flag}
}

// A frame that will not decode is reported and skipped, and the scan carries
// on. One corrupt frame in a long capture must not end the run.
func TestGapScannerSkipsUndecodableFrames(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0)
	out, errOut := scan(t, proto,
		frame(0, 0, 0, false),
		[]byte{0xFF, 0, 0, 0},
		frame(0, 1, 0, false),
	)

	if !strings.Contains(out, "Scanned 2 frame(s), found 0 gap(s).") {
		t.Errorf("want two frames scanned, got:\n%s", out)
	}
	if !strings.Contains(out, "1 frame(s) could not be decoded") {
		t.Errorf("want the skip noted in the summary, got:\n%s", out)
	}
	if !strings.Contains(errOut, "decode error") {
		t.Errorf("want a warning on stderr, got:\n%s", errOut)
	}
}

// The first frame on a channel has nothing to be compared against, so it must
// not report a gap. A receiver joining a pass in progress would otherwise
// report the whole counter as missing.
func TestGapScannerFirstFrameIsNotAGap(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0)
	out, _ := scan(t, proto, frame(0, 200, 0, false))

	if !strings.Contains(out, "Scanned 1 frame(s), found 0 gap(s).") {
		t.Errorf("want no gap on the first frame, got:\n%s", out)
	}
}

// Each virtual channel counts separately, so interleaving two channels is not
// a gap on either.
func TestGapScannerTracksChannelsApart(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0)
	out, _ := scan(t, proto,
		frame(1, 0, 0, false),
		frame(2, 90, 0, false),
		frame(1, 1, 0, false),
		frame(2, 91, 0, false),
	)

	if !strings.Contains(out, "Scanned 4 frame(s), found 0 gap(s).") {
		t.Errorf("want no gaps across two channels, got:\n%s", out)
	}
}

// The gap is a count of missing frames, measured modulo the field width.
func TestGapScannerSizesTheGap(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0)
	out, _ := scan(t, proto,
		frame(1, 10, 0, false),
		frame(1, 15, 0, false),
	)

	if !strings.Contains(out, "4 frame(s) missing before VC=15") {
		t.Errorf("want four missing frames, got:\n%s", out)
	}
	if !strings.Contains(out, "found 1 gap(s), 4 frame(s) missing.") {
		t.Errorf("want the summary to total the missing frames, got:\n%s", out)
	}
}

// With the cycle in use the count and cycle behave as one wider counter, so a
// wrap of the count with the cycle stepping is contiguous rather than a gap.
func TestGapScannerFoldsTheCycleOnWrap(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0x0F)
	out, _ := scan(t, proto,
		frame(1, 254, 0, true),
		frame(1, 255, 0, true),
		frame(1, 0, 1, true),
		frame(1, 1, 1, true),
	)

	if !strings.Contains(out, "Scanned 4 frame(s), found 0 gap(s).") {
		t.Errorf("want the cycle step to read as contiguous, got:\n%s", out)
	}
}

// Without the cycle folded in, the same wrap would still be contiguous — but
// a count that repeats after the cycle steps must read as a full lap missing,
// which is what proves the cycle is actually in the arithmetic.
func TestGapScannerCycleWidensTheCounter(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0x0F)
	out, _ := scan(t, proto,
		frame(1, 5, 0, true),
		frame(1, 6, 1, true),
	)

	// Folded, the frames are 0x005 and 0x106. The count expected next was
	// 0x006, so 0x106 is 256 further on: a whole lap of the count went by.
	if !strings.Contains(out, "256 frame(s) missing before VC=6") {
		t.Errorf("want the cycle counted in the gap, got:\n%s", out)
	}
}

// sdl.GapCounter documents that Track and TrackWithCycle must not be mixed on
// one channel, because they disagree about where the counter wraps. A stream
// whose usage flag flips mid-pass is reported and the counter restarted,
// rather than a gap being measured against the wrong modulus.
func TestGapScannerReportsCycleUsageChange(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0x0F)
	out, errOut := scan(t, proto,
		frame(1, 10, 0, false),
		frame(1, 11, 0, true),
		frame(1, 12, 0, true),
	)

	if !strings.Contains(errOut, "changed frame count cycle usage") {
		t.Errorf("want the change reported, got:\n%s", errOut)
	}
	// The frame at the change is not counted against either modulus, and the
	// one after it is a first frame again.
	if !strings.Contains(out, "found 0 gap(s).") {
		t.Errorf("want no gap invented across the change, got:\n%s", out)
	}
}

// A protocol with no master channel count must not report master channel
// gaps. AOS is the case: its header has no such field.
func TestGapScannerSkipsMasterChannelWhenAbsent(t *testing.T) {
	proto := testProtocol(0xFF, 0, 0)
	out, _ := scan(t, proto,
		frame(1, 10, 0, false),
		frame(1, 20, 0, false),
	)

	if strings.Contains(out, "MC gap") {
		t.Errorf("want no master channel gap for a protocol without one, got:\n%s", out)
	}
	if !strings.Contains(out, "VC gap") {
		t.Errorf("want the virtual channel gap still reported, got:\n%s", out)
	}
}
