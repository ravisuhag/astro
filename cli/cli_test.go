package cli

import (
	"bytes"
	"embed"
	"os"
	"strings"
	"sync"
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

// stdinMu serializes the tests below on the one resource plan 025 left
// global: os.Stdin. Every cli/*.go command now writes through
// cmd.OutOrStdout()/cmd.ErrOrStderr() rather than the process's stdout, so
// runCLI no longer needs to touch os.Stdout at all — output is captured
// straight off the buffers passed to cmd.SetOut/SetErr, and tests can run
// under t.Parallel().
//
// Input is a different story: readInput, openInput and openListing all read
// os.Stdin directly (cobra's cmd.InOrStdin() is not wired up), and changing
// that is out of scope for plan 025, which is about output only. So a test
// that feeds stdin still has to swap the package-global os.Stdin, and two
// such tests running concurrently would otherwise race on it — or worse,
// read each other's pipe. The mutex below makes that swap-and-read safe
// without touching the input path: tests still declare t.Parallel(), and the
// ones that do not use stdin genuinely overlap; the ones that do serialize on
// this lock rather than corrupting each other.
var stdinMu sync.Mutex

// runCLI executes the root command with args, feeding stdin and capturing
// output.
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

	stdinMu.Lock()
	defer stdinMu.Unlock()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origIn := os.Stdin
	os.Stdin = inR
	defer func() { os.Stdin = origIn }()

	go func() {
		_, _ = inW.Write(stdin)
		_ = inW.Close()
	}()

	var cmdOut, cmdErr bytes.Buffer
	cmd := New(docsFS)
	cmd.SetOut(&cmdOut)
	cmd.SetErr(&cmdErr)
	cmd.SetArgs(args)
	execErr := cmd.Execute()

	return cmdOut.String(), execErr
}

func TestRunCLI_SPPRoundTrip(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	if _, err := runCLIWithFS(t, testDocsFS, nil, "manual", "sundial"); err == nil {
		t.Fatal("expected an error for an unknown protocol, got nil")
	}
}
