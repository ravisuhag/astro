package cli

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmdl"
)

// buildTMFrame encodes one TM Transfer Frame with explicit master-channel and
// virtual-channel counters, which the CLI's encode command cannot set.
func buildTMFrame(t *testing.T, scid uint16, vcid, mcCount, vcCount uint8, data []byte) []byte {
	t.Helper()
	frame, err := tmdl.NewTMTransferFrame(scid, vcid, data, nil, nil)
	if err != nil {
		t.Fatalf("building frame: %v", err)
	}
	frame.Header.MCFrameCount = mcCount
	frame.Header.VCFrameCount = vcCount

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("encoding frame: %v", err)
	}
	return encoded
}

// tmStream concatenates frames and returns the hex stream plus the frame length.
func tmStream(t *testing.T, frames ...[]byte) (string, int) {
	t.Helper()
	if len(frames) == 0 {
		t.Fatal("no frames given")
	}
	frameLen := len(frames[0])
	var all []byte
	for i, f := range frames {
		if len(f) != frameLen {
			t.Fatalf("frame %d is %d bytes, want %d, all frames must be equal length", i, len(f), frameLen)
		}
		all = append(all, f...)
	}
	return hex.EncodeToString(all), frameLen
}

func TestTMRoundTripJSON(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "tm", "encode",
		"--scid", "42", "--vcid", "1", "--data", "68656c6c6f")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "tm", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame tmFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.SpacecraftID != 42 {
		t.Errorf("spacecraft_id = %d, want 42", frame.SpacecraftID)
	}
	if frame.VirtualChannelID != 1 {
		t.Errorf("virtual_channel_id = %d, want 1", frame.VirtualChannelID)
	}
}

func TestTMGapsContiguous(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 0, 0, 0, payload),
		buildTMFrame(t, 42, 0, 1, 1, payload),
		buildTMFrame(t, 42, 0, 2, 2, payload),
	)

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 3 frame(s), found 0 gap(s).") {
		t.Errorf("want no gaps reported, got:\n%s", out)
	}
}

func TestTMGapsDetectsSkippedFrame(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	// Counters jump 1 -> 3 on both the master and virtual channel.
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 0, 0, 0, payload),
		buildTMFrame(t, 42, 0, 1, 1, payload),
		buildTMFrame(t, 42, 0, 3, 3, payload),
	)

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 3 frame(s), found 2 gap(s), 2 frame(s) missing.") {
		t.Errorf("want 2 gaps (one MC, one VC) and 2 missing frames, got:\n%s", out)
	}
	if !strings.Contains(out, "MC gap:") {
		t.Errorf("want an MC gap line, got:\n%s", out)
	}
	if !strings.Contains(out, "VC gap:") {
		t.Errorf("want a VC gap line, got:\n%s", out)
	}
	// The gap is reported as a count of missing frames, not just as a
	// discontinuity: one frame went missing on each counter.
	if !strings.Contains(out, "1 frame(s) missing before MC=3") {
		t.Errorf("want the MC gap sized, got:\n%s", out)
	}
}

// A counter that skips far enough to wrap must report the short distance
// forward, not the long way round. sdl.GapCounter does the modular
// arithmetic; this checks the CLI actually routes through it.
func TestTMGapsCountsMissingFramesAcrossWrap(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	// 254 -> 255 -> 1 loses only the frame numbered 0.
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 0, 254, 254, payload),
		buildTMFrame(t, 42, 0, 255, 255, payload),
		buildTMFrame(t, 42, 0, 1, 1, payload),
	)

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "found 2 gap(s), 2 frame(s) missing.") {
		t.Errorf("want one missing frame on each counter across the wrap, got:\n%s", out)
	}
}

// Counts from two spacecraft are unrelated, so they must be tracked apart.
// Interleaving them used to read as a gap on every frame.
func TestTMGapsSeparatesSpacecraft(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 0, 0, 0, payload),
		buildTMFrame(t, 99, 0, 40, 40, payload),
		buildTMFrame(t, 42, 0, 1, 1, payload),
		buildTMFrame(t, 99, 0, 41, 41, payload),
	)

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 4 frame(s), found 0 gap(s).") {
		t.Errorf("want no gaps across two spacecraft, got:\n%s", out)
	}
	if !strings.Contains(out, "2 spacecraft seen") {
		t.Errorf("want the two spacecraft noted, got:\n%s", out)
	}
}

// A frame length above maxUnitHeader must work. The sizer path used for
// variable-length packets gives up after 64 octets of header probing, so
// fixed-length frames take their own reader.
func TestTMGapsLongFrames(t *testing.T) {
	t.Parallel()
	payload := make([]byte, 300)
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 0, 0, 0, payload),
		buildTMFrame(t, 42, 0, 1, 1, payload),
	)
	if frameLen <= 64 {
		t.Fatalf("frame length %d does not exercise the long-frame path", frameLen)
	}

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed on %d-octet frames: %v", frameLen, err)
	}
	if !strings.Contains(out, "Scanned 2 frame(s), found 0 gap(s).") {
		t.Errorf("want both long frames scanned, got:\n%s", out)
	}
}

func TestTMDemuxFiltersByVCID(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	// Two frames on VC 1, one on VC 2, interleaved.
	stream, frameLen := tmStream(t,
		buildTMFrame(t, 42, 1, 0, 0, payload),
		buildTMFrame(t, 42, 2, 1, 0, payload),
		buildTMFrame(t, 42, 1, 2, 1, payload),
	)

	out, err := runCLI(t, []byte(stream), "tm", "demux",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen), "--vcid", "1", "--format", "json")
	if err != nil {
		t.Fatalf("demux failed: %v", err)
	}

	lines := nonEmptyLines(out)
	if len(lines) != 2 {
		t.Fatalf("got %d frames on VC 1, want 2:\n%s", len(lines), out)
	}
	for i, line := range lines {
		var frame tmFrameJSON
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("line %d unmarshal %q: %v", i, line, err)
		}
		if frame.VirtualChannelID != 1 {
			t.Errorf("line %d virtual_channel_id = %d, want 1", i, frame.VirtualChannelID)
		}
	}
}

func TestTMDemuxNoMatches(t *testing.T) {
	t.Parallel()
	payload := []byte("frame-payload")
	stream, frameLen := tmStream(t, buildTMFrame(t, 42, 1, 0, 0, payload))

	out, err := runCLI(t, []byte(stream), "tm", "demux",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen), "--vcid", "5", "--format", "text")
	if err != nil {
		t.Fatalf("demux failed: %v", err)
	}
	if !strings.Contains(out, "Matched 0 of 1 frame(s)") {
		t.Errorf("want a zero-match summary, got:\n%s", out)
	}
}

func TestTMSplitFramesNonMultipleLength(t *testing.T) {
	t.Parallel()
	// Documents today's behavior: a partial trailing frame is dropped with a
	// stderr warning and the command still succeeds.
	payload := []byte("frame-payload")
	frame := buildTMFrame(t, 42, 0, 0, 0, payload)
	frameLen := len(frame)

	stream := hex.EncodeToString(append(append([]byte{}, frame...), frame[:5]...))

	out, err := runCLI(t, []byte(stream), "tm", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed on a non-multiple length: %v", err)
	}
	if !strings.Contains(out, "Scanned 1 frame(s)") {
		t.Errorf("want only the whole frame scanned, got:\n%s", out)
	}
}

func TestTMGapsEmptyInput(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, []byte(""), "tm", "gaps", "--input", "hex", "--frame-len", "128")
	if err != nil {
		t.Fatalf("gaps on empty input failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 0 frame(s), found 0 gap(s).") {
		t.Errorf("want an empty scan summary, got:\n%s", out)
	}
}
