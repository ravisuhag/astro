package cli

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// ramp is a stream of 8-bit samples that changes slowly, which is what
// unit-delay prediction is for.
func ramp(count int) []byte {
	data := make([]byte, count)
	for i := range data {
		data[i] = byte((i * 3) % 256)
	}
	return data
}

// The property that matters: what goes in comes out, exactly. Lossless means
// lossless, so anything short of byte-identical is a bug.
func TestLDCRoundTripIsExact(t *testing.T) {
	original := ramp(512)

	coded, err := runCLI(t, []byte(hex.EncodeToString(original)), "ldc", "compress",
		"--input", "hex", "--format", "hex", "--resolution", "8")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	recovered, err := runCLI(t, []byte(strings.TrimSpace(coded)), "ldc", "decompress",
		"--input", "hex", "--format", "hex")
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	got, err := hex.DecodeString(strings.TrimSpace(recovered))
	if err != nil {
		t.Fatalf("decoding the recovered hex: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("round trip changed the data: %d octets in, %d out", len(original), len(got))
	}
}

// A slowly changing ramp should actually get smaller, or the coder is not
// doing its job.
func TestLDCCompressesARamp(t *testing.T) {
	original := ramp(512)

	out, err := runCLI(t, []byte(hex.EncodeToString(original)), "ldc", "compress",
		"--input", "hex", "--format", "text", "--resolution", "8")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}
	if !strings.Contains(out, "512 sample(s)") {
		t.Errorf("compress did not report the sample count:\n%s", out)
	}

	coded, err := hex.DecodeString(strings.TrimSpace(lastLine(out)))
	if err != nil {
		t.Fatalf("decoding the coded hex: %v", err)
	}
	if len(coded) >= len(original) {
		t.Errorf("coded output is %d octets, no smaller than the %d it started at",
			len(coded), len(original))
	}
}

// The file carries its own parameters, which is what lets decompress take no
// flags at all.
func TestLDCInspectReadsTheHeader(t *testing.T) {
	coded, err := runCLI(t, []byte(hex.EncodeToString(ramp(128))), "ldc", "compress",
		"--input", "hex", "--format", "hex",
		"--resolution", "8", "--block-size", "32", "--reference-interval", "64")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(coded)), "ldc", "inspect",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	var header struct {
		SampleCount       uint64 `json:"sample_count"`
		BlockSize         int    `json:"block_size"`
		Resolution        uint   `json:"resolution"`
		Predictor         string `json:"predictor"`
		ReferenceInterval int    `json:"reference_interval"`
	}
	if err := json.Unmarshal([]byte(out), &header); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if header.SampleCount != 128 {
		t.Errorf("sample_count = %d, want 128", header.SampleCount)
	}
	if header.BlockSize != 32 {
		t.Errorf("block_size = %d, want the 32 that was asked for", header.BlockSize)
	}
	if header.ReferenceInterval != 64 {
		t.Errorf("reference_interval = %d, want 64", header.ReferenceInterval)
	}
	if header.Predictor != "unit-delay" {
		t.Errorf("predictor = %q, want unit-delay", header.Predictor)
	}
}

// A 12-bit sample travels in two octets, so a round trip has to preserve the
// width as well as the values.
func TestLDCWiderSamples(t *testing.T) {
	// Two-octet samples, each below 4096.
	var original []byte
	for i := 0; i < 256; i++ {
		value := (i * 7) % 4096
		original = append(original, byte(value>>8), byte(value))
	}

	coded, err := runCLI(t, []byte(hex.EncodeToString(original)), "ldc", "compress",
		"--input", "hex", "--format", "hex", "--resolution", "12")
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	recovered, err := runCLI(t, []byte(strings.TrimSpace(coded)), "ldc", "decompress",
		"--input", "hex", "--format", "hex")
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	got, err := hex.DecodeString(strings.TrimSpace(recovered))
	if err != nil {
		t.Fatalf("decoding the recovered hex: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("12-bit round trip changed the data: %d octets in, %d out",
			len(original), len(got))
	}
}

// A sample that does not fit the stated resolution is a mismatch between the
// flag and the data. Coding it would truncate silently, so it is refused.
func TestLDCRejectsSamplesTooWideForTheResolution(t *testing.T) {
	// 0x0FFF does not fit 8 bits, and at --resolution 8 each octet is a
	// sample, so use a value above 255 in a two-octet reading instead.
	if _, err := runCLI(t, []byte("0fff0001"), "ldc", "compress",
		"--input", "hex", "--format", "hex", "--resolution", "10"); err == nil {
		t.Error("compress accepted a sample too wide for --resolution 10")
	}
}

// Input that is not a whole number of samples means the resolution and the
// data disagree, which is worth saying rather than dropping a trailing octet.
func TestLDCRejectsPartialSample(t *testing.T) {
	if _, err := runCLI(t, []byte("010203"), "ldc", "compress",
		"--input", "hex", "--format", "hex", "--resolution", "16"); err == nil {
		t.Error("compress accepted three octets as 16-bit samples")
	}
}

func TestLDCRejectsBadParameters(t *testing.T) {
	for name, args := range map[string][]string{
		"block size not in the standard's set": {
			"ldc", "compress", "--input", "hex", "--resolution", "8", "--block-size", "10"},
		"resolution out of range": {
			"ldc", "compress", "--input", "hex", "--resolution", "40"},
		"reference interval out of range": {
			"ldc", "compress", "--input", "hex", "--resolution", "8", "--reference-interval", "9000"},
		"unknown predictor": {
			"ldc", "compress", "--input", "hex", "--resolution", "8", "--predictor", "crystal-ball"},
		"restricted above four bits": {
			"ldc", "compress", "--input", "hex", "--resolution", "8", "--restricted"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runCLI(t, []byte("0102030405060708"), args...); err == nil {
				t.Errorf("%s was accepted", name)
			}
		})
	}
}

func TestLDCEmptyInput(t *testing.T) {
	if _, err := runCLI(t, []byte(""), "ldc", "compress",
		"--input", "hex", "--resolution", "8"); err == nil {
		t.Error("compress accepted empty input")
	}
}

func TestLDCDecompressRejectsRubbish(t *testing.T) {
	if _, err := runCLI(t, []byte("deadbeef"), "ldc", "decompress", "--input", "hex"); err == nil {
		t.Error("decompress accepted a file with no valid header")
	}
}

// lastLine returns the final non-empty line of some output.
func lastLine(out string) string {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}
