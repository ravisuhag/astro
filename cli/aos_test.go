package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAOSRoundTripJSON(t *testing.T) {
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
