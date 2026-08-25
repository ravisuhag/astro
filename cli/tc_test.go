package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTCRoundTripJSON(t *testing.T) {
	encoded, err := runCLI(t, nil, "tc", "encode",
		"--scid", "42", "--vcid", "3", "--seq-num", "7", "--data", "0102030405")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "tc", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	var frame tcFrameJSON
	if err := json.Unmarshal([]byte(out), &frame); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if frame.SpacecraftID != 42 {
		t.Errorf("spacecraft_id = %d, want 42", frame.SpacecraftID)
	}
	if frame.VirtualChannelID != 3 {
		t.Errorf("virtual_channel_id = %d, want 3", frame.VirtualChannelID)
	}
	if frame.FrameSequenceNum != 7 {
		t.Errorf("frame_sequence_num = %d, want 7", frame.FrameSequenceNum)
	}
	if frame.DataField != "0102030405" {
		t.Errorf("data_field = %q, want 0102030405", frame.DataField)
	}
}

func TestTCBypassAndControlFlags(t *testing.T) {
	tests := []struct {
		name        string
		flags       []string
		wantBypass  bool
		wantControl bool
	}{
		{name: "plain", flags: nil},
		{name: "bypass", flags: []string{"--bypass"}, wantBypass: true},
		// Per CCSDS 232.0-B-4 4.1.2.3 a control (BC) frame is always a
		// bypass frame: Bypass=0 with Control Command=1 is an invalid type.
		{name: "control", flags: []string{"--control"}, wantBypass: true, wantControl: true},
		{name: "bypass and control", flags: []string{"--bypass", "--control"}, wantBypass: true, wantControl: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"tc", "encode", "--scid", "42", "--vcid", "1", "--data", "0102"}, tt.flags...)
			encoded, err := runCLI(t, nil, args...)
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			out, err := runCLI(t, []byte(strings.TrimSpace(encoded)), "tc", "decode",
				"--input", "hex", "--format", "json")
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			var frame tcFrameJSON
			if err := json.Unmarshal([]byte(out), &frame); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if frame.IsBypass != tt.wantBypass {
				t.Errorf("is_bypass = %v, want %v", frame.IsBypass, tt.wantBypass)
			}
			if frame.IsControl != tt.wantControl {
				t.Errorf("is_control = %v, want %v", frame.IsControl, tt.wantControl)
			}
		})
	}
}

func TestTCInspect(t *testing.T) {
	encoded := encodeTCFrameHex(t)

	out, err := runCLI(t, []byte(encoded), "tc", "inspect", "--input", "hex")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if !strings.Contains(out, "26") {
		t.Errorf("inspect output does not mention SCID 26:\n%s", out)
	}
}
