package xtce

import "testing"

// readBytes' bounds check has to be written so that offset+width cannot wrap
// past the packet's real bit length, because a width taken from a packet can
// be driven arbitrarily close to the maximum uint. This is exercised directly
// against bitReader rather than through the public API: dynamic.go's fieldWidth
// caps a dynamically-sized binary field's width against the packet it comes
// from, so no legitimate packet -- being finite -- can actually construct an
// offset/width pair that overflows here. That the public API cannot reach
// this case is itself a sign the cap works; readBytes is still checked
// in-package so the guard's overflow-safety is proven on its own.
func TestReadBytesRefusesOverflowingOffsetWidth(t *testing.T) {
	reader := bitReader{data: make([]byte, 4)} // bitLen() = 32

	// offset(10) + width wraps past 2^64 back down to 4, which is well
	// within the 32-bit packet. The old guard, "offset+width > bitLen()",
	// computed that wrapped sum and let this straight through; the
	// subtraction form used now, "width > bitLen()-offset", never computes
	// the sum at all.
	offset := uint(10)
	width := ^uint(0) - 5
	if _, err := reader.readBytes(offset, width); err == nil {
		t.Fatal("readBytes accepted an offset/width pair whose sum wraps past the packet's bit length")
	}

	// An offset past the packet on its own, with no wraparound involved.
	if _, err := reader.readBytes(1000, 1); err == nil {
		t.Fatal("readBytes accepted an offset past the packet")
	}

	// An ordinary aligned read still works.
	data := []byte{0xAB, 0xCD, 0xEF, 0x01}
	reader = bitReader{data: data}
	got, err := reader.readBytes(0, 32)
	if err != nil {
		t.Fatalf("readBytes(0, 32): %v", err)
	}
	if len(got) != 4 || got[0] != 0xAB || got[3] != 0x01 {
		t.Errorf("readBytes(0, 32) = %x, want abcdef01", got)
	}
}
