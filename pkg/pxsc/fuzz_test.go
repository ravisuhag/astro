package pxsc_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxsc"
)

func FuzzUnwrapPLTU(f *testing.F) {
	if pltu, err := pxsc.WrapPLTU([]byte("seed frame")); err == nil {
		f.Add(pltu)
	}
	f.Add([]byte{})
	f.Add(pxsc.DefaultASM())
	f.Add(append(pxsc.DefaultASM(), make([]byte, 8)...))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and anything that unwraps must re-wrap to
		// exactly the octets it came from.
		frame, err := pxsc.UnwrapPLTU(data)
		if err != nil {
			return
		}
		rewrapped, err := pxsc.WrapPLTU(frame)
		if err != nil {
			t.Fatalf("an unwrapped frame failed to re-wrap: %v", err)
		}
		if !bytes.Equal(rewrapped, data) {
			t.Fatalf("re-wrapping produced %d octets, want the original %d", len(rewrapped), len(data))
		}
	})
}

func FuzzSynchronizer(f *testing.F) {
	if pltu, err := pxsc.WrapPLTU([]byte("seed frame")); err == nil {
		f.Add(append(pxsc.IdleData(8), pltu...))
	}
	f.Add([]byte{})
	f.Add(pxsc.IdleData(64))
	f.Add(bytes.Repeat(pxsc.DefaultASM(), 20))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and every PLTU reported must actually
		// verify at the offset given.
		s := pxsc.NewSynchronizer()
		s.MaxFrameLength = 256

		for _, u := range s.Scan(data) {
			if u.Offset < 0 || u.Offset+u.Length() > len(data) {
				t.Fatalf("PLTU at offset %d length %d falls outside a %d-octet stream",
					u.Offset, u.Length(), len(data))
			}
			rewrapped, err := pxsc.WrapPLTU(u.Frame)
			if err != nil {
				t.Fatalf("a reported frame failed to re-wrap: %v", err)
			}
			if !bytes.Equal(rewrapped, data[u.Offset:u.Offset+u.Length()]) {
				t.Fatal("a reported PLTU does not match the stream at its offset")
			}
		}
	})
}

func FuzzCRC32(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("message"))
	f.Add(bytes.Repeat([]byte{0xFF}, 64))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: a message plus its own CRC always verifies, and the
		// syndrome of a valid codeword is zero.
		sum := pxsc.ComputeCRC32(data)
		codeword := append(append([]byte{}, data...),
			byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum))

		if !pxsc.VerifyCRC32(codeword) {
			t.Fatal("a codeword built from our own CRC failed verification")
		}
		if got := pxsc.ComputeCRC32(codeword); got != 0 {
			t.Fatalf("syndrome = %#08x, want 0", got)
		}
	})
}
