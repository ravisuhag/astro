package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

var sdlsTestKey = bytes.Repeat([]byte{0xAA}, sdls.AESKeySize)

// writeKey puts key in a private file and returns its path.
func writeKey(t *testing.T, key []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sa.key")
	if err := os.WriteFile(path, key, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSDLSApplyMatchesTheLibrary checks the flags build the SA they name: the
// command's output must be exactly what the library produces for the same SA,
// counter and frame header.
func TestSDLSApplyMatchesTheLibrary(t *testing.T) {
	t.Parallel()
	keyFile := writeKey(t, sdlsTestKey)

	out, err := runCLI(t, []byte("68656c6c6f"), "sdls", "apply",
		"--key-file", keyFile, "--spi", "7",
		"--last-iv", "000000000000000000000004", "--frame-header", "0a0b0c0d0e0f")
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	sa := &sdls.SecurityAssociation{
		SPI:          7,
		Mode:         sdls.AuthenticatedEncryption,
		Key:          sdlsTestKey,
		FieldLengths: sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: sdls.MaxMACSize},
	}
	last, _ := hex.DecodeString("000000000000000000000004")
	if err := sa.SetIVCounter(last); err != nil {
		t.Fatal(err)
	}
	want, err := sa.ApplySecurity([]byte{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != hex.EncodeToString(want) {
		t.Errorf("apply = %s, want %x", got, want)
	}
}

func TestSDLSApplyProcessRoundTrip(t *testing.T) {
	t.Parallel()
	keyFile := writeKey(t, []byte(hex.EncodeToString(sdlsTestKey)+"\n"))

	for _, tc := range []struct {
		name  string
		apply []string
		sa    []string
	}{
		{"aead", []string{"--last-iv", "000000000000000000000000"}, nil},
		{"gmac", []string{"--last-iv", "000000000000000000000000"}, []string{"--mode", "auth"}},
		{"cmac", []string{"--last-seq", "00000041"},
			[]string{"--mode", "auth", "--auth-alg", "cmac", "--iv", "0", "--seq", "4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			common := append([]string{"--key-file", keyFile, "--spi", "9", "--frame-header", "1234"}, tc.sa...)

			protected, err := runCLI(t, []byte("c0ffee"),
				append(append([]string{"sdls", "apply"}, common...), tc.apply...)...)
			if err != nil {
				t.Fatalf("apply failed: %v", err)
			}
			out, err := runCLI(t, []byte(protected), append([]string{"sdls", "process"}, common...)...)
			if err != nil {
				t.Fatalf("process failed: %v", err)
			}
			if got := strings.TrimSpace(out); got != "c0ffee" {
				t.Errorf("process = %s, want c0ffee", got)
			}

			// A different frame header must fail authentication.
			tampered := append([]string{"sdls", "process"}, common...)
			tampered = append(tampered, "--frame-header", "1235")
			if _, err := runCLI(t, []byte(protected), tampered...); err == nil {
				t.Error("process accepted a frame with a changed header")
			}
		})
	}
}

// TestSDLSApplyRequiresTheLastCounter guards the IV-reuse trap: with no
// counter given, every run would send IV 1 under the same key.
func TestSDLSApplyRequiresTheLastCounter(t *testing.T) {
	t.Parallel()
	keyFile := writeKey(t, sdlsTestKey)

	_, err := runCLI(t, []byte("00"), "sdls", "apply", "--key-file", keyFile, "--spi", "7")
	if err == nil || !strings.Contains(err.Error(), "--last-iv is required") {
		t.Errorf("apply without --last-iv: got %v, want a --last-iv error", err)
	}

	_, err = runCLI(t, []byte("00"), "sdls", "apply", "--key-file", keyFile, "--spi", "7",
		"--mode", "auth", "--auth-alg", "cmac", "--iv", "0", "--seq", "4")
	if err == nil || !strings.Contains(err.Error(), "--last-seq is required") {
		t.Errorf("apply without --last-seq: got %v, want a --last-seq error", err)
	}

	_, err = runCLI(t, []byte("00"), "sdls", "apply", "--key-file", keyFile, "--spi", "7",
		"--last-iv", "0000")
	if !errors.Is(err, sdls.ErrInvalidIVCounter) {
		t.Errorf("apply with a short --last-iv: got %v, want ErrInvalidIVCounter", err)
	}
}

func TestSDLSRejectsABadKeyFile(t *testing.T) {
	t.Parallel()
	short := writeKey(t, []byte("not a key"))
	_, err := runCLI(t, []byte("00"), "sdls", "process", "--key-file", short, "--spi", "7")
	if err == nil || !strings.Contains(err.Error(), "key file") {
		t.Errorf("process with a bad key file: got %v, want a key file error", err)
	}

	_, err = runCLI(t, []byte("00"), "sdls", "process", "--spi", "7")
	if err == nil || !strings.Contains(err.Error(), "key-file") {
		t.Errorf("process without --key-file: got %v, want a required-flag error", err)
	}
}
