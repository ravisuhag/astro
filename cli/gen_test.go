package cli

import (
	"strings"
	"testing"
)

func TestGenNegativeSize(t *testing.T) {
	// Used to panic with a raw Go stack trace from make([]byte, -1).
	_, err := runCLI(t, nil, "spp", "gen", "--size=-1")
	if err == nil {
		t.Fatal("expected an error for negative --size, got nil")
	}
	if !strings.Contains(err.Error(), "size must be >= 0") {
		t.Errorf("error = %q, want it to mention size must be >= 0", err)
	}
}

func TestGenOversize(t *testing.T) {
	_, err := runCLI(t, nil, "spp", "gen", "--size", "20000000")
	if err == nil {
		t.Fatal("expected an error for oversized --size, got nil")
	}
	if !strings.Contains(err.Error(), "size must be <=") {
		t.Errorf("error = %q, want it to mention the size ceiling", err)
	}
}

func TestGenNegativeCount(t *testing.T) {
	_, err := runCLI(t, nil, "spp", "gen", "--count=-1")
	if err == nil {
		t.Fatal("expected an error for negative --count, got nil")
	}
	if !strings.Contains(err.Error(), "count must be >= 0") {
		t.Errorf("error = %q, want it to mention count must be >= 0", err)
	}
}

func TestGenTypeCaseInsensitive(t *testing.T) {
	// spp encode accepted --type TM while spp gen rejected it.
	out, err := runCLI(t, nil, "spp", "gen", "--type", "TM", "--count", "1", "--format", "hex")
	if err != nil {
		t.Fatalf("gen --type TM failed: %v", err)
	}
	if len(strings.TrimSpace(out)) == 0 {
		t.Fatal("expected hex output")
	}
}
