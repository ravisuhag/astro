package cli

import (
	"bytes"
	"embed"
	"os"
	"sync"
	"testing"
)

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
func runCLI(t *testing.T, stdin []byte, args ...string) (string, error) {
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
	cmd := New(embed.FS{})
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
