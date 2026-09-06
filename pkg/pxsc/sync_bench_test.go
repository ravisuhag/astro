package pxsc_test

import (
	"math/rand"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxsc"
)

// degradedStream returns an n-octet stream hostile to the synchronizer.
//
// It is filled with pseudo-random noise, then every markerSpacing octets
// gets a false ASM whose following header octet is forced to look like a
// real Version-3 frame (marker bits '10'). That means tryAt's quick
// implied-length probe actually runs — and fails its CRC, since everything
// after the marker is noise — before the fallback loop takes over. This is
// the audit finding S6 cliff: a single failed sync brute-forces every
// candidate frame length, each with a full CRC-32 over the candidate body.
func degradedStream(n, markerSpacing int) []byte {
	data := make([]byte, n)
	random := rand.New(rand.NewSource(42)) //nolint:gosec // reproducible benchmark input, not cryptography
	for i := range data {
		data[i] = byte(random.Intn(256))
	}
	for i := 0; i+pxsc.ASMSize+4 < len(data); i += markerSpacing {
		copy(data[i:], pxsc.DefaultASM())
		// Force Transfer Frame Version Number '10' so impliedFrameLength
		// returns a plausible length and the quick probe is actually
		// exercised, not skipped outright.
		data[i+pxsc.ASMSize] = (data[i+pxsc.ASMSize] & 0x3F) | 0x80
	}
	return data
}

// BenchmarkScanDegraded measures the fallback path: every seeded marker's
// implied length fails its CRC, so the synchronizer must try every
// candidate frame length before concluding the marker is a false match and
// stepping past it. This is the path audit finding S6 flags as O(maxLen)
// per marker (or O(maxLen^2) overall before the incremental-CRC fix).
func BenchmarkScanDegraded(b *testing.B) {
	data := degradedStream(32*1024, 691)
	s := pxsc.NewSynchronizer()

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		s.Scan(data)
	}
}

// BenchmarkScanClean measures the fast path: every PLTU's implied frame
// length is correct the first time, so the fallback loop this change
// touches never runs. The fix must not regress this.
func BenchmarkScanClean(b *testing.B) {
	var stream []byte
	for i := 0; i < 50; i++ {
		frame := buildFrame(b, uint16(i), "a clean stream payload with no false markers")
		pltu, err := pxsc.WrapPLTU(frame)
		if err != nil {
			b.Fatal(err)
		}
		stream = append(stream, pxsc.IdleData(16)...)
		stream = append(stream, pltu...)
	}

	s := pxsc.NewSynchronizer()
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		s.Scan(stream)
	}
}
