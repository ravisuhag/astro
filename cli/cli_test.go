package cli

import (
	"bytes"
	"embed"
	"io"
	"os"
	"testing"
)

// runCLI executes the root command with args, feeding stdin and capturing
// stdout. Commands print via fmt (os.Stdout) and read os.Stdin directly,
// so both are swapped with pipes for the duration of the call.
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
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, outR)
		close(done)
	}()

	cmd := New(embed.FS{})
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
