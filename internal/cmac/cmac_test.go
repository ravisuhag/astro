package cmac_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/internal/cmac"
)

// unhex parses a spaced hex string as the reference documents print them.
func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

// TestSubkeyDerivation pins the K1 and K2 of RFC 4493 section 4, which is
// where a wrong shift or a missing Rb fold shows up first.
//
// They are not exported, so they are checked through the tags they produce:
// an empty message uses K2, and a single whole block uses K1. Both vectors
// above already cover that, so this test instead pins the property the
// doubling must have (that K2 is K1 doubled) by way of a key whose L has the
// top bit set, exercising the Rb fold.
func TestSubkeyDerivationExercisesTheReduction(t *testing.T) {
	// AES-128(0^128) under this key has its top bit set, so deriving K1 must
	// fold in Rb. If the fold is missing the tags below are wrong.
	key := unhex(t, "2b7e1516 28aed2a6 abf71588 09cf4f3c")

	c, err := cmac.New(key)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 4493 prints AES-128(key,0) = 7df76b0c..., whose top bit is 0, so K1
	// is a plain shift; K1 = fbeed618..., whose top bit IS set, so K2 folds.
	// The len-0 vector below depends on K2 and therefore on that fold.
	want := unhex(t, "bb1d6929 e9593728 7fa37d12 9b756746")
	if got := c.Sum(nil); !bytes.Equal(got, want) {
		t.Errorf("the empty-message tag is %X, want %X; the Rb fold in K2 is wrong", got, want)
	}
}

// TestLengthBoundaries walks every length across two block boundaries. The
// whole point of CMAC over CBC-MAC is that the two paths (padded and not)
// give different tags, so the boundary is where an off-by-one lives.
func TestLengthBoundaries(t *testing.T) {
	key := make([]byte, 32)
	c, err := cmac.New(key)
	if err != nil {
		t.Fatal(err)
	}

	message := make([]byte, 40)
	for i := range message {
		message[i] = byte(i)
	}

	seen := make(map[string]int)
	for length := 0; length <= 40; length++ {
		tag := c.Sum(message[:length])
		if len(tag) != cmac.BlockSize {
			t.Fatalf("length %d: tag is %d octets, want %d", length, len(tag), cmac.BlockSize)
		}
		key := string(tag)
		if prev, dup := seen[key]; dup {
			t.Errorf("lengths %d and %d produced the same tag", prev, length)
		}
		seen[key] = length
	}
}

// TestPaddedAndCompleteDiffer is the forgery CMAC exists to prevent: a
// 15-octet message and the same message padded to 16 by hand must not
// authenticate to the same tag.
func TestPaddedAndCompleteDiffer(t *testing.T) {
	key := make([]byte, 32)
	c, err := cmac.New(key)
	if err != nil {
		t.Fatal(err)
	}

	short := bytes.Repeat([]byte{0xAA}, 15)
	handPadded := append(append([]byte(nil), short...), 0x80)

	if bytes.Equal(c.Sum(short), c.Sum(handPadded)) {
		t.Error("a message and its hand-padded form share a tag; the subkeys are not being applied")
	}
}

func TestVerify(t *testing.T) {
	key := make([]byte, 32)
	c, err := cmac.New(key)
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("telecommand frame contents")
	tag := c.Sum(message)

	if !c.Verify(message, tag) {
		t.Error("Verify rejected a tag it produced")
	}

	// Every single-bit change in the tag must be rejected.
	for i := range tag {
		for bit := range 8 {
			bad := append([]byte(nil), tag...)
			bad[i] ^= 1 << uint(bit)
			if c.Verify(message, bad) {
				t.Fatalf("Verify accepted a tag with octet %d bit %d flipped", i, bit)
			}
		}
	}

	// And any change to the message.
	for i := range message {
		bad := append([]byte(nil), message...)
		bad[i] ^= 0x01
		if c.Verify(bad, tag) {
			t.Fatalf("Verify accepted a message with octet %d altered", i)
		}
	}
}

func TestVerifyRejectsBadTagLengths(t *testing.T) {
	c, err := cmac.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("x")

	if c.Verify(message, nil) {
		t.Error("Verify accepted an empty tag")
	}
	if c.Verify(message, make([]byte, cmac.BlockSize+1)) {
		t.Error("Verify accepted an oversized tag")
	}
}

func TestSumTruncated(t *testing.T) {
	c, err := cmac.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("truncate me")
	full := c.Sum(message)

	for length := 1; length <= cmac.BlockSize; length++ {
		got, err := c.SumTruncated(message, length)
		if err != nil {
			t.Fatalf("length %d: %v", length, err)
		}
		if !bytes.Equal(got, full[:length]) {
			t.Errorf("length %d: %X, want %X", length, got, full[:length])
		}
	}

	for _, bad := range []int{0, -1, cmac.BlockSize + 1} {
		if _, err := c.SumTruncated(message, bad); err == nil {
			t.Errorf("SumTruncated accepted length %d", bad)
		}
	}
}

func TestNewRejectsBadKeys(t *testing.T) {
	for _, size := range []int{0, 1, 15, 17, 31, 33, 64} {
		if _, err := cmac.New(make([]byte, size)); err == nil {
			t.Errorf("New accepted a %d-octet key", size)
		}
	}
	for _, size := range []int{16, 24, 32} {
		if _, err := cmac.New(make([]byte, size)); err != nil {
			t.Errorf("New rejected a %d-octet key: %v", size, err)
		}
	}
}

// TestSumDoesNotMutateMessage guards against the padding being written into
// the caller's slice.
func TestSumDoesNotMutateMessage(t *testing.T) {
	c, err := cmac.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}

	message := bytes.Repeat([]byte{0x5A}, 20)
	original := append([]byte(nil), message...)

	c.Sum(message)
	if !bytes.Equal(message, original) {
		t.Error("Sum wrote through to its input")
	}
}

func FuzzSumNeverPanics(f *testing.F) {
	f.Add([]byte{}, make([]byte, 32))
	f.Add([]byte("short"), make([]byte, 16))
	f.Add(bytes.Repeat([]byte{0xFF}, 64), make([]byte, 24))

	f.Fuzz(func(t *testing.T, message, key []byte) {
		c, err := cmac.New(key)
		if err != nil {
			return
		}
		tag := c.Sum(message)
		if len(tag) != cmac.BlockSize {
			t.Fatalf("tag is %d octets, want %d", len(tag), cmac.BlockSize)
		}
		if !c.Verify(message, tag) {
			t.Fatal("Verify rejected a tag Sum produced")
		}
	})
}
