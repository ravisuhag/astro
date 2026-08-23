package cli

import (
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	out, err := runCLI(t, nil, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	for _, want := range []string{"astro ", "go:", "platform:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	// A binary built from a checkout still reports where it came from, even
	// with no link-time stamping.
	if strings.Contains(out, "astro \n") {
		t.Errorf("version is empty:\n%s", out)
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := runCLI(t, nil, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out, "astro version") {
		t.Errorf("--version output = %q", out)
	}
	// cobra prefixes the program name itself, so the value must not repeat it.
	if strings.Contains(out, "version astro") {
		t.Errorf("the program name is doubled: %q", out)
	}
}

func TestVersionTakesNoArguments(t *testing.T) {
	if _, err := runCLI(t, nil, "version", "extra"); err == nil {
		t.Error("version accepted a positional argument")
	}
}
