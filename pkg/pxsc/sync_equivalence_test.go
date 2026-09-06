package pxsc_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxsc"
)

// noisyBlockWithFalseMarkers returns n octets of pseudo-random noise seeded
// with false ASMs every 173 octets, each followed by a header octet forced
// to look like a real Version-3 frame. That drives every false marker into
// Synchronizer.tryAt's fallback loop — the incremental-CRC scan this change
// rewrites — rather than being skipped outright.
func noisyBlockWithFalseMarkers(random *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(random.Intn(256))
	}
	for i := 0; i+pxsc.ASMSize+4 < len(b); i += 173 {
		copy(b[i:], pxsc.DefaultASM())
		b[i+pxsc.ASMSize] = (b[i+pxsc.ASMSize] & 0x3F) | 0x80
	}
	return b
}

// TestSynchronizerRecoversKnownFramesFromANoisyStream is the equivalence
// proof for the incremental-CRC fallback in Synchronizer.tryAt: a stream
// stuffed with false markers (so the brute-force fallback runs constantly)
// still yields exactly the frames planted in it, at the exact offsets they
// were planted, plus a corrupted real PLTU that must still be rejected.
//
// This same stream and expectation were checked against the pre-fix
// full-recompute implementation (by temporarily reverting crc32.go and
// sync.go with git stash) and produced byte-for-byte identical output:
// same 4 PLTUs, same offsets, same CRCs, same frame bytes. That comparison
// is not repeatable from this file alone since the pre-fix code is gone
// from the tree, which is why the known-offsets form is what is committed.
func TestSynchronizerRecoversKnownFramesFromANoisyStream(t *testing.T) {
	random := rand.New(rand.NewSource(99)) //nolint:gosec // reproducible test channel, not cryptography

	payloads := []string{"first known frame", "second known frame", "third known frame", "fourth known frame"}

	var stream []byte
	var wantFrames [][]byte
	var wantOffsets []int
	for i, payload := range payloads {
		stream = append(stream, noisyBlockWithFalseMarkers(random, 400)...)
		frame := buildFrame(t, uint16(100+i), payload)
		pltu, err := pxsc.WrapPLTU(frame)
		if err != nil {
			t.Fatal(err)
		}
		wantOffsets = append(wantOffsets, len(stream))
		stream = append(stream, pltu...)
		wantFrames = append(wantFrames, frame)
	}

	// A real PLTU with a broken CRC: real ASM, real header (so the implied
	// length matches), wrong check value. It must still be rejected, with
	// the fallback loop running across its own frame boundary and finding
	// nothing, exactly as the pre-fix version would.
	broken := buildFrame(t, 999, "a frame whose CRC will be broken")
	brokenPLTU, err := pxsc.WrapPLTU(broken)
	if err != nil {
		t.Fatal(err)
	}
	brokenPLTU[len(brokenPLTU)-1] ^= 0xFF
	stream = append(stream, brokenPLTU...)
	stream = append(stream, noisyBlockWithFalseMarkers(random, 400)...)

	s := pxsc.NewSynchronizer()
	got := s.Scan(stream)

	if len(got) != len(wantFrames) {
		t.Fatalf("recovered %d PLTUs, want %d (a corrupted PLTU may have been delivered)", len(got), len(wantFrames))
	}
	for i, u := range got {
		if !bytes.Equal(u.Frame, wantFrames[i]) {
			t.Errorf("frame %d = %X, want %X", i, u.Frame, wantFrames[i])
		}
		if u.Offset != wantOffsets[i] {
			t.Errorf("frame %d recovered at offset %d, want %d", i, u.Offset, wantOffsets[i])
		}
		if bytes.Equal(u.Frame, broken) {
			t.Error("the corrupted PLTU was delivered")
		}
	}
}
