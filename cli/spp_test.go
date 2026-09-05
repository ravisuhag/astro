package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSPPRoundTripJSON(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "spp", "encode",
		"--apid", "100", "--type", "tm", "--data", "68656c6c6f")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "spp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var pkt packetJSON
	if err := json.Unmarshal([]byte(out), &pkt); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if pkt.APID != 100 {
		t.Errorf("apid = %d, want 100", pkt.APID)
	}
	if pkt.UserData != "68656c6c6f" {
		t.Errorf("user_data = %q, want 68656c6c6f", pkt.UserData)
	}
	if pkt.TypeName != "TM" {
		t.Errorf("type_name = %q, want TM", pkt.TypeName)
	}
}

func TestSPPValidateWithCRC(t *testing.T) {
	t.Parallel()
	encoded, err := runCLI(t, nil, "spp", "encode",
		"--apid", "100", "--type", "tm", "--data", "616161", "--crc")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "spp", "validate",
		"--input", "hex", "--crc")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("output %q, want it to report the packet is valid", out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("output %q, want it to report the CRC is OK", out)
	}
}

// encodeSPP returns the hex encoding of one TM packet with the given APID.
func encodeSPP(t *testing.T, apid, data string) string {
	t.Helper()
	out, err := runCLI(t, nil, "spp", "encode", "--apid", apid, "--type", "tm", "--data", data)
	if err != nil {
		t.Fatalf("encode apid %s: %v", apid, err)
	}
	return strings.TrimSpace(out)
}

func TestSPPStreamThreePackets(t *testing.T) {
	t.Parallel()
	stream := encodeSPP(t, "100", "616161") + encodeSPP(t, "200", "626262") + encodeSPP(t, "300", "636363")

	out, err := runCLI(t, []byte(stream), "spp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	lines := nonEmptyLines(out)
	if len(lines) != 3 {
		t.Fatalf("got %d JSON lines, want 3:\n%s", len(lines), out)
	}

	wantAPID := []uint16{100, 200, 300}
	wantData := []string{"616161", "626262", "636363"}
	for i, line := range lines {
		var pkt packetJSON
		if err := json.Unmarshal([]byte(line), &pkt); err != nil {
			t.Fatalf("line %d unmarshal %q: %v", i, line, err)
		}
		if pkt.APID != wantAPID[i] {
			t.Errorf("line %d apid = %d, want %d", i, pkt.APID, wantAPID[i])
		}
		if pkt.UserData != wantData[i] {
			t.Errorf("line %d user_data = %q, want %q", i, pkt.UserData, wantData[i])
		}
	}
}

func TestSPPStreamTrailingBytes(t *testing.T) {
	t.Parallel()
	// Documents today's behavior: whole packets are emitted, leftover bytes
	// produce a warning on stderr and exit 0, not an error. If streaming ever
	// becomes incremental, this expectation is meant to be revisited.
	stream := encodeSPP(t, "100", "616161") + encodeSPP(t, "200", "626262") + "ffff"

	out, err := runCLI(t, []byte(stream), "spp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream returned an error for trailing bytes: %v", err)
	}
	if got := len(nonEmptyLines(out)); got != 2 {
		t.Errorf("got %d JSON lines, want 2 (trailing bytes skipped):\n%s", got, out)
	}
}

func TestSPPStreamTruncatedFinalPacket(t *testing.T) {
	t.Parallel()
	// A final packet cut mid-body is dropped with a stderr warning, same as
	// trailing garbage. Observed behavior, frozen deliberately.
	full := encodeSPP(t, "300", "636363")
	stream := encodeSPP(t, "100", "616161") + full[:10]

	out, err := runCLI(t, []byte(stream), "spp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream returned an error for a truncated packet: %v", err)
	}
	if got := len(nonEmptyLines(out)); got != 1 {
		t.Errorf("got %d JSON lines, want 1:\n%s", got, out)
	}
}

func TestSPPStreamEmptyInput(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, []byte(""), "spp", "stream", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("stream on empty input failed: %v", err)
	}
	if got := len(nonEmptyLines(out)); got != 0 {
		t.Errorf("got %d JSON lines, want 0:\n%s", got, out)
	}
}

func TestSPPInspect(t *testing.T) {
	t.Parallel()
	encoded := encodeSPP(t, "100", "616161")

	out, err := runCLI(t, []byte(encoded), "spp", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(out, "100") {
		t.Errorf("inspect output does not mention APID 100:\n%s", out)
	}
}

// nonEmptyLines splits output into lines, dropping blanks.
func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
