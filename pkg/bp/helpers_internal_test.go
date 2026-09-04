package bp

import (
	"encoding/hex"
	"testing"
)

// mustHex decodes a hex string from a standard or an RFC, or fails the test.
// The vectors in this package are transcribed as hex because that is how the
// documents print them.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}
