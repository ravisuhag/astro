package cli

import (
	"strings"
	"testing"
)

// encodeTCFrameHex returns one CLI-encoded TC frame as hex.
func encodeTCFrameHex(t *testing.T) string {
	t.Helper()
	out, err := runCLI(t, nil, "tc", "encode", "--scid", "26", "--vcid", "1", "--data", "0102030405")
	if err != nil {
		t.Fatalf("tc encode: %v", err)
	}
	return strings.TrimSpace(out)
}

func TestCLTUWrapUnwrapRoundTrip(t *testing.T) {
	t.Parallel()
	frame := encodeTCFrameHex(t)

	wrapped, err := runCLI(t, []byte(frame), "cltu", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}
	wrapped = strings.TrimSpace(wrapped)

	// The CLTU start sequence is 0xEB90.
	if !strings.HasPrefix(wrapped, "eb90") {
		t.Errorf("CLTU %q does not start with the start sequence", wrapped)
	}

	unwrapped, err := runCLI(t, []byte(wrapped), "cltu", "unwrap", "--input", "hex")
	if err != nil {
		t.Fatalf("unwrap failed: %v", err)
	}
	got := strings.TrimSpace(unwrapped)

	// CLTU codeblocks are padded with 0x55 fill, and the receiver cannot know
	// the original length, so unwrap returns the frame plus any fill bytes.
	if !strings.HasPrefix(got, frame) {
		t.Errorf("unwrapped %q does not start with the original frame %q", got, frame)
	}
	if rest := strings.TrimPrefix(got, frame); strings.Trim(rest, "5") != "" {
		t.Errorf("unwrapped trailer %q is not 0x55 fill", rest)
	}
}

func TestCLTUInspect(t *testing.T) {
	t.Parallel()
	frame := encodeTCFrameHex(t)

	wrapped, err := runCLI(t, []byte(frame), "cltu", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(wrapped)), "cltu", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "cltu") {
		t.Errorf("inspect output does not mention the CLTU:\n%s", out)
	}
}
