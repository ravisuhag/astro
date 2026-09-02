package pxsc_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxsc"
)

func TestASMIsThreeOctets(t *testing.T) {
	// Clause 3.2.3: 24 bits, pattern FAF320. Note this is three octets, not the
	// four that TM and AOS use.
	asm := pxsc.DefaultASM()
	if len(asm) != 3 {
		t.Fatalf("ASM is %d octets, want 3", len(asm))
	}
	if !bytes.Equal(asm, []byte{0xFA, 0xF3, 0x20}) {
		t.Errorf("ASM = %X, want FAF320", asm)
	}
}

func TestCRC32PolynomialDiffersFromTheCommonOnes(t *testing.T) {
	// Annex C, C1.3: G(X) = X^32 + X^23 + X^21 + X^11 + X^2 + 1.
	// Reusing IEEE CRC-32 or CRC-32C here would produce a checksum that looks
	// fine and rejects every frame.
	if pxsc.CRC32Polynomial != 0x00A00805 {
		t.Errorf("polynomial = %#08x, want 0x00A00805", pxsc.CRC32Polynomial)
	}
	if pxsc.CRC32Polynomial == 0x04C11DB7 {
		t.Error("the polynomial is IEEE CRC-32, which is the wrong one")
	}
	if pxsc.CRC32Polynomial == 0x1EDC6F41 {
		t.Error("the polynomial is CRC-32C, which is the wrong one")
	}
}

func TestCRC32StartsFromZero(t *testing.T) {
	// Annex C's encoder note: the shift register is preset to all zeros,
	// unlike the 16-bit CCSDS CRC which starts at all ones.
	//
	// An empty message therefore has a zero checksum. With an all-ones
	// preset it would not.
	if got := pxsc.ComputeCRC32(nil); got != 0 {
		t.Errorf("CRC of an empty message = %#08x, want 0", got)
	}
}

func TestCRC32SyndromeOfAValidCodewordIsZero(t *testing.T) {
	// Annex C, C2: the decoder computes a syndrome and an error is detected
	// if and only if it is non-zero.
	message := []byte("proximity transfer frame")
	sum := pxsc.ComputeCRC32(message)

	codeword := append([]byte{}, message...)
	codeword = append(codeword, byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum))

	if !pxsc.VerifyCRC32(codeword) {
		t.Error("a correctly built codeword failed verification")
	}
	if got := pxsc.ComputeCRC32(codeword); got != 0 {
		t.Errorf("syndrome = %#08x, want 0", got)
	}
}

func TestCRC32DetectsEverySingleBitError(t *testing.T) {
	message := []byte("proximity transfer frame")
	sum := pxsc.ComputeCRC32(message)
	codeword := append(append([]byte{}, message...),
		byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum))

	for i := range codeword {
		for bit := 0; bit < 8; bit++ {
			corrupt := append([]byte{}, codeword...)
			corrupt[i] ^= 1 << uint(bit)
			if pxsc.VerifyCRC32(corrupt) {
				t.Fatalf("a flip at octet %d bit %d went undetected", i, bit)
			}
		}
	}
}

func TestPLTURoundTrip(t *testing.T) {
	frame := []byte("a Version-3 transfer frame")

	pltu, err := pxsc.WrapPLTU(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(pltu) != len(frame)+pxsc.PLTUOverhead {
		t.Fatalf("PLTU is %d octets, want %d", len(pltu), len(frame)+pxsc.PLTUOverhead)
	}
	if !bytes.HasPrefix(pltu, pxsc.DefaultASM()) {
		t.Error("the PLTU does not start with the sync marker")
	}

	got, err := pxsc.UnwrapPLTU(pltu)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, frame) {
		t.Errorf("recovered %q, want %q", got, frame)
	}
}

func TestCRCExcludesTheASM(t *testing.T) {
	// Annex C, C1.2 note 2: "The ASM is NOT used for computing the CRC-32."
	frame := []byte("frame contents")
	pltu, err := pxsc.WrapPLTU(frame)
	if err != nil {
		t.Fatal(err)
	}

	// The CRC in the PLTU must equal the CRC over the frame alone.
	want := pxsc.ComputeCRC32(frame)
	tail := pltu[len(pltu)-pxsc.CRC32Size:]
	got := uint32(tail[0])<<24 | uint32(tail[1])<<16 | uint32(tail[2])<<8 | uint32(tail[3])

	if got != want {
		t.Errorf("attached CRC = %#08x, want %#08x (over the frame, not the ASM)", got, want)
	}
}

func TestUnwrapRejectsBadASM(t *testing.T) {
	pltu, err := pxsc.WrapPLTU([]byte("frame"))
	if err != nil {
		t.Fatal(err)
	}
	pltu[1] ^= 0xFF

	if _, err := pxsc.UnwrapPLTU(pltu); !errors.Is(err, pxsc.ErrInvalidASM) {
		t.Errorf("error = %v, want ErrInvalidASM", err)
	}
}

func TestUnwrapRejectsCorruptFrame(t *testing.T) {
	pltu, err := pxsc.WrapPLTU([]byte("frame contents here"))
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a frame octet, leaving the ASM intact.
	pltu[pxsc.ASMSize+2] ^= 0x01

	if _, err := pxsc.UnwrapPLTU(pltu); !errors.Is(err, pxsc.ErrCRCMismatch) {
		t.Errorf("error = %v, want ErrCRCMismatch", err)
	}
}

func TestUnwrapRejectsShortInput(t *testing.T) {
	pltu, err := pxsc.WrapPLTU([]byte("frame"))
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(pltu); cut++ {
		if _, err := pxsc.UnwrapPLTU(pltu[:cut]); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}

func TestWrapRejectsEmptyFrame(t *testing.T) {
	if _, err := pxsc.WrapPLTU(nil); !errors.Is(err, pxsc.ErrEmptyFrame) {
		t.Errorf("error = %v, want ErrEmptyFrame", err)
	}
}

func TestIdlePattern(t *testing.T) {
	// Clause 3.3.2.2: the PN sequence 352EF853, repeated as needed.
	idle := pxsc.IdleData(12)
	want := []byte{0x35, 0x2E, 0xF8, 0x53, 0x35, 0x2E, 0xF8, 0x53, 0x35, 0x2E, 0xF8, 0x53}
	if !bytes.Equal(idle, want) {
		t.Errorf("idle data = %X, want %X", idle, want)
	}

	// Clause 3.3.2.4: a partial repetition is fine; it just stops where it stops.
	partial := pxsc.IdleData(6)
	if !bytes.Equal(partial, want[:6]) {
		t.Errorf("partial idle = %X, want %X", partial, want[:6])
	}

	if !pxsc.IsIdleData(idle) {
		t.Error("IsIdleData rejected genuine idle data")
	}
	if pxsc.IsIdleData([]byte{0x00, 0x00}) {
		t.Error("IsIdleData accepted something that is not the pattern")
	}
}

func TestIdleSequencesShareThePattern(t *testing.T) {
	// Clause 3.3.3.2 and clause 3.3.5.2.1: the acquisition and tail sequences carry the
	// same data as any other idle. Only when they are sent differs.
	n := 16
	if !bytes.Equal(pxsc.AcquisitionSequence(n), pxsc.IdleData(n)) {
		t.Error("the acquisition sequence differs from idle data")
	}
	if !bytes.Equal(pxsc.TailSequence(n), pxsc.IdleData(n)) {
		t.Error("the tail sequence differs from idle data")
	}
	if !bytes.Equal(pxsc.IdleSequence(n), pxsc.IdleData(n)) {
		t.Error("the idle sequence differs from idle data")
	}
}
