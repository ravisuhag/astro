package cli

import (
	"bytes"
	"embed"
	"io"
	"os"
	"testing"
)

// runCLI executes the root command with args, feeding stdin and capturing
// output.
//
// Conversion of cli/*.go to route output through the command's writer
// (cmd.OutOrStdout()) is in progress (plan 025). Commands that have been
// converted write to a buffer via cmd.SetOut; commands that have not yet
// been converted still print via fmt to the process's os.Stdout, so that is
// swapped with a pipe for the duration of the call too. The two captures
// are concatenated, which is safe because a single command exercises only
// one of the two mechanisms at a time. Once every command is converted,
// the os.Stdout swap can be deleted (plan 025 step 4).
func runCLI(t *testing.T, stdin []byte, args ...string) (string, error) {
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
	var pipeBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&pipeBuf, outR)
		close(done)
	}()

	var cmdOut, cmdErr bytes.Buffer
	cmd := New(embed.FS{})
	cmd.SetOut(&cmdOut)
	cmd.SetErr(&cmdErr)
	cmd.SetArgs(args)
	execErr := cmd.Execute()

	_ = outW.Close()
	<-done
	os.Stdin, os.Stdout = origIn, origOut
	return pipeBuf.String() + cmdOut.String(), execErr
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
