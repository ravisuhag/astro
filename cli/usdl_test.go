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

	// OCF presence is signaled in-band by the OCF flag: no --ocf flag is
	// needed on decode.
	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "usdl", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame usdlFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if !frame.OCFFlag {
		t.Error("ocf_flag not set")
	}
	if frame.OCF != "deadbeef" {
		t.Errorf("ocf = %q, want %q", frame.OCF, "deadbeef")
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want %q", frame.DataField, "0102030405")
	}
}

func TestUSDLRoundTripCRC16FECF(t *testing.T) {
	encoded, err := runCLI(t, nil, "usdl", "encode",
		"--scid", "42", "--data", "0102030405", "--format", "hex")
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
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
	// The USLP FECF is the 16-bit CRC (CCSDS 732.1-B-3 clause 4.1.6.2.2):
	// 2 bytes, so 4 hex characters.
	if len(frame.FECF) != 4 {
		t.Errorf("fecf = %q, want a 4-character CRC-16 value", frame.FECF)
	}
}

func TestUSDLRoundTripVCFCount(t *testing.T) {
	encoded, err := runCLI(t, nil, "usdl", "encode",
		"--scid", "42", "--data", "0102", "--vcf-len", "2", "--vcf-count", "258", "--format", "hex")
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
	if frame.VCFCountLen != 2 || frame.VCFCount != 258 {
		t.Errorf("vcf = len %d count %d, want len 2 count 258", frame.VCFCountLen, frame.VCFCount)
	}
}

func TestUSDLInspect(t *testing.T) {
	encoded, err := runCLI(t, nil, "usdl", "encode",
		"--scid", "42", "--data", "0102030405", "--format", "hex")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "usdl", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("inspect output does not mention SCID 42:\n%s", out)
	}
}
