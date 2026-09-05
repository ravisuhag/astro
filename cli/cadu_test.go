package cli

import (
	"strconv"
	"strings"
	"testing"
)

// encodeTMFrameHex returns one CLI-encoded TM frame as hex.
func encodeTMFrameHex(t *testing.T) string {
	t.Helper()
	out, err := runCLI(t, nil, "tm", "encode", "--scid", "26", "--vcid", "1", "--data", "0102030405")
	if err != nil {
		t.Fatalf("tm encode: %v", err)
	}
	return strings.TrimSpace(out)
}

func TestCADUWrapUnwrapRoundTrip(t *testing.T) {
	t.Parallel()
	frame := encodeTMFrameHex(t)

	wrapped, err := runCLI(t, []byte(frame), "cadu", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}
	wrapped = strings.TrimSpace(wrapped)

	// The ASM (0x1ACFFC1D) is prepended, so the CADU is 4 bytes longer.
	if !strings.HasPrefix(wrapped, "1acffc1d") {
		t.Errorf("CADU %q does not start with the ASM", wrapped)
	}

	unwrapped, err := runCLI(t, []byte(wrapped), "cadu", "unwrap", "--input", "hex")
	if err != nil {
		t.Fatalf("unwrap failed: %v", err)
	}
	if got := strings.TrimSpace(unwrapped); got != frame {
		t.Errorf("round trip produced %q, want %q", got, frame)
	}
}

func TestCADURandomizeRoundTrip(t *testing.T) {
	t.Parallel()
	frame := encodeTMFrameHex(t)

	wrapped, err := runCLI(t, []byte(frame), "cadu", "wrap", "--input", "hex", "--randomize")
	if err != nil {
		t.Fatalf("wrap --randomize failed: %v", err)
	}
	wrapped = strings.TrimSpace(wrapped)

	// Randomization must actually change the frame body.
	if strings.Contains(wrapped, strings.TrimPrefix(frame, "0x")) {
		t.Error("randomized CADU still contains the plain frame bytes")
	}

	unwrapped, err := runCLI(t, []byte(wrapped), "cadu", "unwrap", "--input", "hex", "--derandomize")
	if err != nil {
		t.Fatalf("unwrap --derandomize failed: %v", err)
	}
	if got := strings.TrimSpace(unwrapped); got != frame {
		t.Errorf("randomize round trip produced %q, want %q", got, frame)
	}
}

// wrapCADU returns one wrapped CADU as hex, plus its length in bytes.
func wrapCADU(t *testing.T) (string, int) {
	t.Helper()
	frame := encodeTMFrameHex(t)
	out, err := runCLI(t, []byte(frame), "cadu", "wrap", "--input", "hex")
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	cadu := strings.TrimSpace(out)
	return cadu, len(cadu) / 2
}

func TestCADUSyncFindsAllCADUs(t *testing.T) {
	t.Parallel()
	cadu, caduLen := wrapCADU(t)

	out, err := runCLI(t, []byte(cadu+cadu), "cadu", "sync",
		"--input", "hex", "--frame-len", strconv.Itoa(caduLen))
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if !strings.Contains(out, "Found 2 CADU(s)") {
		t.Errorf("want 2 CADUs found, got:\n%s", out)
	}
	if !strings.Contains(out, "offset 0,") {
		t.Errorf("want the first CADU at offset 0, got:\n%s", out)
	}
	if !strings.Contains(out, "offset "+strconv.Itoa(caduLen)+",") {
		t.Errorf("want the second CADU at offset %d, got:\n%s", caduLen, out)
	}
}

func TestCADUSyncSkipsLeadingJunk(t *testing.T) {
	t.Parallel()
	cadu, caduLen := wrapCADU(t)

	// 4 junk bytes before the first ASM; sync must report offset 4.
	out, err := runCLI(t, []byte("deadbeef"+cadu+cadu), "cadu", "sync",
		"--input", "hex", "--frame-len", strconv.Itoa(caduLen))
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if !strings.Contains(out, "Found 2 CADU(s)") {
		t.Errorf("want 2 CADUs found after junk, got:\n%s", out)
	}
	if !strings.Contains(out, "offset 4,") {
		t.Errorf("want the first CADU at offset 4, got:\n%s", out)
	}
}

func TestCADUSyncJunkOnly(t *testing.T) {
	t.Parallel()
	_, caduLen := wrapCADU(t)

	out, err := runCLI(t, []byte("deadbeefdeadbeef"), "cadu", "sync",
		"--input", "hex", "--frame-len", strconv.Itoa(caduLen))
	if err != nil {
		t.Fatalf("sync on junk failed: %v", err)
	}
	if !strings.Contains(out, "Found 0 CADU(s)") {
		t.Errorf("want zero CADUs, got:\n%s", out)
	}
}

func TestCADUSyncEmptyInput(t *testing.T) {
	t.Parallel()
	_, caduLen := wrapCADU(t)

	out, err := runCLI(t, []byte(""), "cadu", "sync",
		"--input", "hex", "--frame-len", strconv.Itoa(caduLen))
	if err != nil {
		t.Fatalf("sync on empty input failed: %v", err)
	}
	if !strings.Contains(out, "Found 0 CADU(s)") {
		t.Errorf("want zero CADUs, got:\n%s", out)
	}
}

func TestCADUInspect(t *testing.T) {
	t.Parallel()
	cadu, _ := wrapCADU(t)

	out, err := runCLI(t, []byte(cadu), "cadu", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(out, "Attached Sync Marker") {
		t.Errorf("inspect output does not mention the sync marker:\n%s", out)
	}
	if !strings.Contains(out, "1acffc1d") {
		t.Errorf("inspect output does not show the ASM bytes:\n%s", out)
	}
}
