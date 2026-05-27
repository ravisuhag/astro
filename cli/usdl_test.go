package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUSDLRoundTripWithOCF(t *testing.T) {
	encoded, err := runCLI(t, nil, "usdl", "encode",
		"--scid", "42", "--data", "0102030405", "--ocf", "deadbeef", "--format", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "usdl", "decode",
		"--ocf", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame usdlFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.OCF != "deadbeef" {
		t.Errorf("ocf = %q, want %q", frame.OCF, "deadbeef")
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want %q", frame.DataField, "0102030405")
	}
}

func TestUSDLDecodeWithoutOCFFlag(t *testing.T) {
	// Documents the legacy behavior: without --ocf the decoder cannot know the
	// trailing 4 bytes are an OCF, so they fold into the data field. Auto-
	// detection is impossible per pkg/usdl/frame.go:507, hence the flag.
	encoded, err := runCLI(t, nil, "usdl", "encode",
		"--scid", "42", "--data", "0102030405", "--ocf", "deadbeef", "--format", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "usdl", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame usdlFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.OCF != "" {
		t.Errorf("ocf = %q, want empty without --ocf", frame.OCF)
	}
}
