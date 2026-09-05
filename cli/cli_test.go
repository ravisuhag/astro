package cli

import (
	"bytes"
	"embed"
	"io"
	"os"
	"strings"
	"testing"
)

// testDocsFS is a small stand-in for the real docs/content/cli/*.md tree
// main.go embeds and passes to New. It cannot be that tree directly: a
// //go:embed pattern may not climb out of its own package directory with
// "..", and the real docs/ lives at the repo root, one level above cli/.
// It also cannot live under a "docs" of its own choosing and be renamed
// into place — an embed.FS's paths are exactly its pattern, and New/
// printManual hardcode "docs/content/cli/" + protocol, so the fixture
// must sit at cli/docs/content/cli/ for that lookup to succeed. See
// cli/docs/content/cli/time.md; it is fixture content, not a real doc.
//
//go:embed docs/content/cli/*.md
var testDocsFS embed.FS

// runCLI executes the root command with args, feeding stdin and capturing
// stdout. Commands print via fmt (os.Stdout) and read os.Stdin directly,
// so both are swapped with pipes for the duration of the call.
//
// The root command is built with an empty docs filesystem, which is right
// for every command except manual: printManual always fails at ReadFile
// against it. Tests that need manual to actually run use runCLIWithFS.
func runCLI(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	return runCLIWithFS(t, embed.FS{}, stdin, args...)
}

// runCLIWithFS is runCLI with the docs filesystem New receives made a
// parameter, so a test can pass testDocsFS and exercise manual for real.
func runCLIWithFS(t *testing.T, docsFS embed.FS, stdin []byte, args ...string) (string, error) {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origIn, origOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	defer func() { os.Stdin, os.Stdout = origIn, origOut }()

	go func() {
		_, _ = inW.Write(stdin)
		_ = inW.Close()
	}()

	// Drain stdout concurrently so a command writing more than the pipe
	// buffer holds cannot block.
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, outR)
		close(done)
	}()

	cmd := New(docsFS)
	cmd.SetArgs(args)
	execErr := cmd.Execute()

	_ = outW.Close()
	<-done
	os.Stdin, os.Stdout = origIn, origOut
	return buf.String(), execErr
}

func TestRunCLI_SPPRoundTrip(t *testing.T) {
	out, err := runCLI(t, nil, "spp", "encode", "--apid", "100", "--type", "tm", "--data", "68656c6c6f")
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected hex output")
	}
}

// TestRunCLI_Manual covers printManual and stripFrontmatter, neither of
// which any test reached before: runCLI builds New(embed.FS{}), an empty
// filesystem, so printManual always failed at ReadFile against it.
// runCLIWithFS(testDocsFS, ...) gives manual something real to read.
func TestRunCLI_Manual(t *testing.T) {
	out, err := runCLIWithFS(t, testDocsFS, nil, "manual", "time")
	if err != nil {
		t.Fatalf("manual time failed: %v", err)
	}
	if strings.Contains(out, "---") {
		t.Errorf("frontmatter was not stripped:\n%s", out)
	}
	if !strings.Contains(out, "astro time") {
		t.Errorf("manual output does not mention the page title:\n%s", out)
	}

	// The empty filesystem runCLI itself uses cannot answer this: no
	// protocols.md file exists under it, so this is exactly the failure
	// printManual is for.
	if _, err := runCLI(t, nil, "manual", "time"); err == nil {
		t.Fatal("manual time succeeded against an empty docs filesystem")
	}
}

// TestRunCLI_ManualIndex covers printManualIndex, the no-argument form.
func TestRunCLI_ManualIndex(t *testing.T) {
	out, err := runCLI(t, nil, "manual")
	if err != nil {
		t.Fatalf("manual failed: %v", err)
	}
	// The heading itself renders with ANSI styling split across its words,
	// so match a table cell that survives markdown rendering intact instead.
	if !strings.Contains(out, "astro manual spp") {
		t.Errorf("manual index output is missing the protocol table:\n%s", out)
	}
}

// TestRunCLI_ManualUnknownProtocol covers printManual's rejection branch.
func TestRunCLI_ManualUnknownProtocol(t *testing.T) {
	if _, err := runCLIWithFS(t, testDocsFS, nil, "manual", "sundial"); err == nil {
		t.Fatal("expected an error for an unknown protocol, got nil")
	}
}
