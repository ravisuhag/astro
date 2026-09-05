package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// encodeEPP returns the hex encoding of one Encapsulation Packet.
func encodeEPP(t *testing.T, pid, data string) string {
	t.Helper()
	out, err := runCLI(t, nil, "epp", "encode", "--pid", pid, "--data", data)
	if err != nil {
		t.Fatalf("encode pid %s: %v", pid, err)
	}
	return strings.TrimSpace(out)
}

func TestEPPRoundTripJSON(t *testing.T) {
	t.Parallel()
	encoded := encodeEPP(t, "2", "616161")

	out, err := runCLI(t, []byte(encoded), "epp", "decode", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var pkt eppPacketJSON
	if err := json.Unmarshal([]byte(out), &pkt); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if pkt.ProtocolID != 2 {
		t.Errorf("protocol_id = %d, want 2", pkt.ProtocolID)
	}
	if pkt.DataZone != "616161" {
		t.Errorf("data_zone = %q, want 616161", pkt.DataZone)
	}
	if pkt.IsIdle {
		t.Error("is_idle = true, want false")
	}
}

func TestEPPValidate(t *testing.T) {
	t.Parallel()
	encoded := encodeEPP(t, "2", "616161")

	out, err := runCLI(t, []byte(encoded), "epp", "validate", "--input", "hex")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("output %q, want it to report the packet is valid", out)
	}
}

func TestEPPStreamThreePackets(t *testing.T) {
	t.Parallel()
	stream := encodeEPP(t, "2", "616161") + encodeEPP(t, "6", "626262") + encodeEPP(t, "2", "636363")

	out, err := runCLI(t, []byte(stream), "epp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	lines := nonEmptyLines(out)
	if len(lines) != 3 {
		t.Fatalf("got %d JSON lines, want 3:\n%s", len(lines), out)
	}

	wantPID := []uint8{2, 6, 2}
	wantData := []string{"616161", "626262", "636363"}
	for i, line := range lines {
		var pkt eppPacketJSON
		if err := json.Unmarshal([]byte(line), &pkt); err != nil {
			t.Fatalf("line %d unmarshal %q: %v", i, line, err)
		}
		if pkt.ProtocolID != wantPID[i] {
			t.Errorf("line %d protocol_id = %d, want %d", i, pkt.ProtocolID, wantPID[i])
		}
		if pkt.DataZone != wantData[i] {
			t.Errorf("line %d data_zone = %q, want %q", i, pkt.DataZone, wantData[i])
		}
	}
}

func TestEPPStreamEmptyInput(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, []byte(""), "epp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream on empty input failed: %v", err)
	}
	if got := len(nonEmptyLines(out)); got != 0 {
		t.Errorf("got %d JSON lines, want 0:\n%s", got, out)
	}
}

func TestEPPIdlePacket(t *testing.T) {
	t.Parallel()
	// pid 0 is the idle protocol ID; encoding with no --data yields an idle packet.
	out, err := runCLI(t, nil, "epp", "encode", "--pid", "0", "--format", "hex")
	if err != nil {
		t.Fatalf("encode idle failed: %v", err)
	}

	decoded, err := runCLI(t, []byte(strings.TrimSpace(out)), "epp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode idle failed: %v", err)
	}

	var pkt eppPacketJSON
	if err := json.Unmarshal([]byte(decoded), &pkt); err != nil {
		t.Fatalf("unmarshal %q: %v", decoded, err)
	}
	if !pkt.IsIdle {
		t.Errorf("is_idle = false, want true for pid 0")
	}
}

func TestEPPInspect(t *testing.T) {
	t.Parallel()
	encoded := encodeEPP(t, "2", "616161")

	out, err := runCLI(t, []byte(encoded), "epp", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	// The hex dump prints bytes space-separated, not as a run of hex digits.
	if !strings.Contains(out, "61 61 61") {
		t.Errorf("inspect output does not show the data zone:\n%s", out)
	}
}
