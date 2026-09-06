package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestAOSRoundTripJSON(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "aos", "encode",
		"--scid", "42", "--vcid", "3", "--vc-count", "99", "--data", "0102030405")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "aos", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame aosFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.SpacecraftID != 42 {
		t.Errorf("spacecraft_id = %d, want 42", frame.SpacecraftID)
	}
	if frame.VirtualChannelID != 3 {
		t.Errorf("virtual_channel_id = %d, want 3", frame.VirtualChannelID)
	}
	if frame.VCFrameCount != 99 {
		t.Errorf("vc_frame_count = %d, want 99", frame.VCFrameCount)
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
}

func TestAOSRoundTripWithOCF(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "aos", "encode",
		"--scid", "42", "--vcid", "1", "--data", "0102030405", "--ocf", "deadbeef")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "aos", "decode",
		"--input", "hex", "--ocf", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame aosFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.OCF != "deadbeef" {
		t.Errorf("ocf = %q, want deadbeef", frame.OCF)
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
}

func TestAOSRoundTripWithFECF(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "aos", "encode",
		"--scid", "42", "--vcid", "1", "--data", "0102030405", "--fecf")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "aos", "decode",
		"--input", "hex", "--fecf", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame aosFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.FECF == "" {
		t.Error("fecf is empty, want the 2-byte FECF")
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
}

func TestAOSRoundTripWithInsertZone(t *testing.T) {
	t.Parallel()
	// The insert zone is 4 bytes here; decode must be told its length.
	encoded, err := runCLI(t, nil, "aos", "encode",
		"--scid", "42", "--vcid", "1", "--data", "0102030405", "--insert", "aabbccdd")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "aos", "decode",
		"--input", "hex", "--insert-len", "4", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame aosFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.InsertZone != "aabbccdd" {
		t.Errorf("insert_zone = %q, want aabbccdd", frame.InsertZone)
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
}

func TestAOSInspect(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "aos", "encode",
		"--scid", "42", "--vcid", "1", "--data", "0102030405")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "aos", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("inspect output does not mention SCID 42:\n%s", out)
	}
}

// buildAOSStream encodes count frames on one virtual channel, stepping the VC
// frame count by step so a step above one leaves a gap.
func buildAOSStream(t *testing.T, scid, vcid uint8, first, step, count int) (string, int) {
	t.Helper()

	var stream strings.Builder
	frameLen := 0

	for i := 0; i < count; i++ {
		encoded, err := runCLI(t, nil, "aos", "encode",
			"--scid", strconv.Itoa(int(scid)),
			"--vcid", strconv.Itoa(int(vcid)),
			"--vc-count", strconv.Itoa(first+i*step),
			"--data", "0102030405")
		if err != nil {
			t.Fatalf("encode frame %d: %v", i, err)
		}
		frame := strings.TrimSpace(encoded)
		if frameLen == 0 {
			frameLen = len(frame) / 2
		}
		stream.WriteString(frame)
	}

	return stream.String(), frameLen
}

func TestAOSGapsContiguous(t *testing.T) {
	t.Parallel()
	stream, frameLen := buildAOSStream(t, 50, 1, 0, 1, 3)

	out, err := runCLI(t, []byte(stream), "aos", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 3 frame(s), found 0 gap(s).") {
		t.Errorf("want no gaps, got:\n%s", out)
	}
}

func TestAOSGapsSizesTheGap(t *testing.T) {
	t.Parallel()
	// Counts 0, 4, 8: three frames missing before each of the last two.
	stream, frameLen := buildAOSStream(t, 50, 1, 0, 4, 3)

	out, err := runCLI(t, []byte(stream), "aos", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "found 2 gap(s), 6 frame(s) missing.") {
		t.Errorf("want six missing frames across two gaps, got:\n%s", out)
	}
}

// AOS has no master channel frame count, so no master channel gap can be
// reported for it however the counts run.
func TestAOSGapsReportsNoMasterChannel(t *testing.T) {
	t.Parallel()
	stream, frameLen := buildAOSStream(t, 50, 1, 0, 4, 2)

	out, err := runCLI(t, []byte(stream), "aos", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if strings.Contains(out, "MC gap") {
		t.Errorf("AOS has no master channel count, got:\n%s", out)
	}
	if !strings.Contains(out, "VC gap") {
		t.Errorf("want the virtual channel gap reported, got:\n%s", out)
	}
}

// The 24-bit count wraps at 0xFFFFFF, so 0xFFFFFF -> 0 is contiguous rather
// than sixteen million frames missing.
func TestAOSGapsWrapsAt24Bits(t *testing.T) {
	t.Parallel()
	first, frameLen := buildAOSStream(t, 50, 1, 0xFFFFFF, 1, 1)
	second, _ := buildAOSStream(t, 50, 1, 0, 1, 1)

	out, err := runCLI(t, []byte(first+second), "aos", "gaps",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen))
	if err != nil {
		t.Fatalf("gaps failed: %v", err)
	}
	if !strings.Contains(out, "Scanned 2 frame(s), found 0 gap(s).") {
		t.Errorf("want the 24-bit wrap to read as contiguous, got:\n%s", out)
	}
}

func TestAOSDemuxFiltersByVCID(t *testing.T) {
	t.Parallel()
	onVC1, frameLen := buildAOSStream(t, 50, 1, 0, 1, 2)
	onVC2, _ := buildAOSStream(t, 50, 2, 0, 1, 1)

	out, err := runCLI(t, []byte(onVC1+onVC2), "aos", "demux",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen),
		"--vcid", "1", "--format", "json")
	if err != nil {
		t.Fatalf("demux failed: %v", err)
	}

	lines := nonEmptyLines(out)
	if len(lines) != 2 {
		t.Fatalf("got %d frames on VC 1, want 2:\n%s", len(lines), out)
	}
	for i, line := range lines {
		var frame aosFrameJSON
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("line %d unmarshal %q: %v", i, line, err)
		}
		if frame.VirtualChannelID != 1 {
			t.Errorf("line %d virtual_channel_id = %d, want 1", i, frame.VirtualChannelID)
		}
	}
}

func TestAOSDemuxNoMatches(t *testing.T) {
	t.Parallel()
	stream, frameLen := buildAOSStream(t, 50, 1, 0, 1, 1)

	out, err := runCLI(t, []byte(stream), "aos", "demux",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen),
		"--vcid", "9", "--format", "text")
	if err != nil {
		t.Fatalf("demux failed: %v", err)
	}
	if !strings.Contains(out, "Matched 0 of 1 frame(s) on VCID=9.") {
		t.Errorf("want a zero-match summary, got:\n%s", out)
	}
}

func TestAOSDemuxRejectsUnknownFormat(t *testing.T) {
	t.Parallel()
	stream, frameLen := buildAOSStream(t, 50, 1, 0, 1, 1)

	if _, err := runCLI(t, []byte(stream), "aos", "demux",
		"--input", "hex", "--frame-len", strconv.Itoa(frameLen),
		"--vcid", "1", "--format", "yaml"); err == nil {
		t.Error("demux accepted an unknown output format")
	}
}
