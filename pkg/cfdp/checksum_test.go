package cfdp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// annexFFile is the 15-octet file of CCSDS 727.0-B-5 Annex F.
var annexFFile = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
}

// annexFExpected is the sum the annex works out by hand:
// 00010203 + 04050607 + 08090a0b + 0c0d0e00.
const annexFExpected = uint32(0x00010203 + 0x04050607 + 0x08090a0b + 0x0c0d0e00)

func TestModularChecksumAnnexFSequential(t *testing.T) {
	// Annex F sends the file in three segments of 6, 6 and 3 octets.
	c, err := cfdp.NewChecksum(cfdp.ChecksumModular)
	if err != nil {
		t.Fatal(err)
	}
	c.Update(0, annexFFile[0:6])
	c.Update(6, annexFFile[6:12])
	c.Update(12, annexFFile[12:15])

	if got := c.Sum(); got != annexFExpected {
		t.Errorf("checksum = %#08x, want %#08x", got, annexFExpected)
	}
}

func TestModularChecksumAnnexFOutOfOrder(t *testing.T) {
	// The annex repeats the calculation with segments arriving 0, 12, 6 and
	// shows the result is the same.
	c, err := cfdp.NewChecksum(cfdp.ChecksumModular)
	if err != nil {
		t.Fatal(err)
	}
	c.Update(0, annexFFile[0:6])
	c.Update(12, annexFFile[12:15])
	c.Update(6, annexFFile[6:12])

	if got := c.Sum(); got != annexFExpected {
		t.Errorf("out-of-order checksum = %#08x, want %#08x", got, annexFExpected)
	}
}

func TestModularChecksumWholeFileMatchesSegmented(t *testing.T) {
	whole, err := cfdp.NewChecksum(cfdp.ChecksumModular)
	if err != nil {
		t.Fatal(err)
	}
	whole.Update(0, annexFFile)

	if got := whole.Sum(); got != annexFExpected {
		t.Errorf("whole-file checksum = %#08x, want %#08x", got, annexFExpected)
	}
}

func TestNullChecksumIsAlwaysZero(t *testing.T) {
	// §4.2.2.4.
	c, err := cfdp.NewChecksum(cfdp.ChecksumNull)
	if err != nil {
		t.Fatal(err)
	}
	c.Update(0, annexFFile)
	if got := c.Sum(); got != 0 {
		t.Errorf("null checksum = %#08x, want 0", got)
	}
}

func TestCRC32ChecksumsOrderIndependent(t *testing.T) {
	for _, kind := range []uint8{cfdp.ChecksumCRC32C, cfdp.ChecksumCRC32} {
		inOrder, err := cfdp.NewChecksum(kind)
		if err != nil {
			t.Fatal(err)
		}
		inOrder.Update(0, annexFFile[0:6])
		inOrder.Update(6, annexFFile[6:12])
		inOrder.Update(12, annexFFile[12:15])

		shuffled, err := cfdp.NewChecksum(kind)
		if err != nil {
			t.Fatal(err)
		}
		shuffled.Update(12, annexFFile[12:15])
		shuffled.Update(0, annexFFile[0:6])
		shuffled.Update(6, annexFFile[6:12])

		if inOrder.Sum() != shuffled.Sum() {
			t.Errorf("type %d: in-order %#08x != shuffled %#08x", kind, inOrder.Sum(), shuffled.Sum())
		}
		if inOrder.Sum() == 0 {
			t.Errorf("type %d: checksum is zero, which suggests nothing was folded in", kind)
		}
	}
}

func TestUnsupportedChecksumType(t *testing.T) {
	if _, err := cfdp.NewChecksum(9); err == nil {
		t.Error("expected an error for an unregistered checksum type")
	}
}
