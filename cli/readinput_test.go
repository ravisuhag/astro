package cli

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validSPPHex is a TM Space Packet, APID 100, user data "hello".
// Produced by: astro spp encode --apid 100 --type tm --data 68656c6c6f
const validSPPHex = "0064c000000468656c6c6f"

func TestReadInputHexVariants(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		format  string
		wantErr string // substring; empty means success expected
	}{
		{name: "plain hex", stdin: validSPPHex, format: "hex"},
		{name: "surrounding whitespace", stdin: "  " + validSPPHex + "  ", format: "hex"},
		{name: "trailing newline", stdin: validSPPHex + "\n", format: "hex"},
		{name: "embedded newlines", stdin: "0064c000\n0004\n68656c6c6f", format: "hex"},
		{name: "embedded spaces", stdin: "0064 c000 0004 68656c6c6f", format: "hex"},
		{name: "carriage returns", stdin: validSPPHex + "\r\n", format: "hex"},
		{name: "0x prefix", stdin: "0x" + validSPPHex, format: "hex"},
		{name: "odd length hex", stdin: "0064c00000046", format: "hex", wantErr: "hex"},
		{name: "non-hex characters", stdin: "zzzz", format: "hex", wantErr: "hex"},
		{name: "unknown format", stdin: validSPPHex, format: "yaml", wantErr: "unknown input format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runCLI(t, []byte(tt.stdin), "spp", "decode", "--input", tt.format)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestReadInputBinary(t *testing.T) {
	raw, err := hex.DecodeString(validSPPHex)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, raw, "spp", "decode", "--input", "bin", "--format", "json")
	if err != nil {
		t.Fatalf("decode from bin failed: %v", err)
	}
	if !strings.Contains(out, `"apid": 100`) {
		t.Errorf("output %q, want it to report apid 100", out)
	}
}

func TestReadInputFromFileArg(t *testing.T) {
	dir := t.TempDir()

	hexPath := filepath.Join(dir, "packet.hex")
	if err := os.WriteFile(hexPath, []byte(validSPPHex+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, nil, "spp", "decode", hexPath, "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode from hex file failed: %v", err)
	}
	if !strings.Contains(out, `"apid": 100`) {
		t.Errorf("output %q, want it to report apid 100", out)
	}

	raw, err := hex.DecodeString(validSPPHex)
	if err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "packet.bin")
	if err := os.WriteFile(binPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, nil, "spp", "decode", binPath, "--input", "bin", "--format", "json")
	if err != nil {
		t.Fatalf("decode from bin file failed: %v", err)
	}
	if !strings.Contains(out, `"apid": 100`) {
		t.Errorf("output %q, want it to report apid 100", out)
	}
}

func TestReadInputMissingFile(t *testing.T) {
	_, err := runCLI(t, nil, "spp", "decode", filepath.Join(t.TempDir(), "nope.hex"))
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
	if !strings.Contains(err.Error(), "reading input") {
		t.Errorf("error = %q, want it to mention reading input", err)
	}
}
