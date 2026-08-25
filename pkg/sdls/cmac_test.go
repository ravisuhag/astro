package sdls_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// tcBaselineLengths is the CCSDS 355.0-B-2 §E2 telecommand baseline: no
// initialization vector, a 32-bit sequence number, no padding, a 128-bit MAC.
// §E2.2 figure E-3 makes the Security Header six octets.
var tcBaselineLengths = sdls.FieldLengths{IV: 0, SeqNum: 4, PadLen: 0, MAC: 16}

func newCMACSA(t *testing.T) *sdls.SecurityAssociation {
	t.Helper()
	sa := &sdls.SecurityAssociation{
		SPI:           7,
		Mode:          sdls.Authentication,
		AuthAlgorithm: sdls.AuthCMAC,
		Key:           testKey,
		FieldLengths:  tcBaselineLengths,
		SeqWindow:     100,
	}
	if err := sa.Validate(); err != nil {
		t.Fatalf("the §E2 baseline SA is invalid: %v", err)
	}
	return sa
}

// TestTCBaselineSecurityHeaderSize pins §E2.2 figure E-3: 16-bit SPI, no IV,
// 32-bit sequence number, no pad length — six octets in all.
func TestTCBaselineSecurityHeaderSize(t *testing.T) {
	if got := tcBaselineLengths.HeaderSize(); got != 6 {
		t.Errorf("the §E2 header is %d octets, want 6", got)
	}
}

func TestCMACRoundTrip(t *testing.T) {
	sender := newCMACSA(t)
	receiver := newCMACSA(t)

	frameHeader := []byte{0x20, 0x00, 0x00, 0x0A}
	plaintext := []byte("telecommand payload")

	protected, err := sender.ApplySecurity(frameHeader, plaintext)
	if err != nil {
		t.Fatalf("ApplySecurity() = %v", err)
	}

	// §E2: authentication without encryption, so the data field travels in
	// the clear between the header and the MAC.
	if !bytes.Contains(protected, plaintext) {
		t.Error("the data field was altered; §E2 authenticates without encrypting")
	}

	header, recovered, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(receiver))
	if err != nil {
		t.Fatalf("ProcessSecurity() = %v", err)
	}
	if !bytes.Equal(recovered, plaintext) {
		t.Errorf("recovered %q, want %q", recovered, plaintext)
	}
	if header.SPI != 7 {
		t.Errorf("SPI = %d, want 7", header.SPI)
	}
	if len(header.IV) != 0 {
		t.Errorf("IV is %d octets, want none under §E2", len(header.IV))
	}
}

// TestCMACDetectsTampering walks every octet of a protected frame and flips a
// bit, which must always be caught.
func TestCMACDetectsTampering(t *testing.T) {
	frameHeader := []byte{0x20, 0x00, 0x00, 0x0A}
	plaintext := []byte("telecommand payload")

	sender := newCMACSA(t)
	protected, err := sender.ApplySecurity(frameHeader, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	for i := range protected {
		corrupt := append([]byte(nil), protected...)
		corrupt[i] ^= 0x01

		receiver := newCMACSA(t)
		if _, _, err := sdls.ProcessSecurity(corrupt, frameHeader, sdls.StaticLookup(receiver)); err == nil {
			t.Fatalf("a bit flipped at octet %d went undetected", i)
		}
	}

	// And a change to the frame header the MAC covers.
	badHeader := append([]byte(nil), frameHeader...)
	badHeader[1] ^= 0x04
	receiver := newCMACSA(t)
	if _, _, err := sdls.ProcessSecurity(protected, badHeader, sdls.StaticLookup(receiver)); err == nil {
		t.Error("a modified frame header went undetected")
	}
}

// TestCMACDiffersFromGMAC checks the algorithm selector actually selects. Two
// SAs identical but for AuthAlgorithm must not produce interchangeable frames.
func TestCMACDiffersFromGMAC(t *testing.T) {
	frameHeader := []byte{0x20, 0x00, 0x00, 0x0A}
	plaintext := []byte("telecommand payload")

	cmacSA := newCMACSA(t)
	protected, err := cmacSA.ApplySecurity(frameHeader, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// A GMAC receiver on the §E1 layout cannot read a §E2 frame.
	gmacSA := newTestSA(t, sdls.Authentication)
	if _, _, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(gmacSA)); err == nil {
		t.Error("a GMAC receiver accepted a CMAC-protected frame")
	}
}

// TestCMACRequiresNoIV pins the §E2.2 note: CMAC needs no initialization
// vector, so an SA declaring one is misconfigured.
func TestCMACRequiresNoIV(t *testing.T) {
	sa := &sdls.SecurityAssociation{
		SPI:           7,
		Mode:          sdls.Authentication,
		AuthAlgorithm: sdls.AuthCMAC,
		Key:           testKey,
		FieldLengths:  sdls.FieldLengths{IV: sdls.GCMIVSize, SeqNum: 4, MAC: 16},
	}
	if err := sa.Validate(); !errors.Is(err, sdls.ErrInvalidFieldLengths) {
		t.Errorf("Validate() = %v, want ErrInvalidFieldLengths for a CMAC SA with an IV", err)
	}
}

// TestGMACRemainsTheDefault checks the zero value did not change behaviour:
// an SA that does not choose an algorithm still gets the §E1 baseline.
func TestGMACRemainsTheDefault(t *testing.T) {
	sa := newTestSA(t, sdls.Authentication)
	if sa.AuthAlgorithm != sdls.AuthGMAC {
		t.Errorf("the default algorithm is %v, want GMAC", sa.AuthAlgorithm)
	}
	if sa.AuthAlgorithm.String() != "GMAC" {
		t.Errorf("String() = %q", sa.AuthAlgorithm.String())
	}
	if sdls.AuthCMAC.String() != "AES-CMAC" {
		t.Errorf("AuthCMAC.String() = %q", sdls.AuthCMAC.String())
	}
}

// TestCMACTruncatedMAC checks that a CMAC SA may truncate below the GCM
// floor: SP 800-38B §6.4 permits any width, so 1 to 16 octets validate and
// round-trip, while GCM keeps its 12-octet floor (Go's crypto/cipher limit).
func TestCMACTruncatedMAC(t *testing.T) {
	for _, width := range []int{sdls.MinCMACSize, 4, 8, sdls.MaxMACSize} {
		sender := newCMACSA(t)
		sender.FieldLengths.MAC = width
		receiver := newCMACSA(t)
		receiver.FieldLengths.MAC = width

		if err := sender.Validate(); err != nil {
			t.Fatalf("MAC width %d: Validate() = %v", width, err)
		}

		frameHeader := []byte{0x20, 0x00, 0x00, 0x0A}
		protected, err := sender.ApplySecurity(frameHeader, []byte("cmd"))
		if err != nil {
			t.Fatalf("MAC width %d: ApplySecurity() = %v", width, err)
		}
		wantLen := receiver.FieldLengths.HeaderSize() + len("cmd") + width
		if len(protected) != wantLen {
			t.Errorf("MAC width %d: frame is %d octets, want %d", width, len(protected), wantLen)
		}
		if _, _, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(receiver)); err != nil {
			t.Errorf("MAC width %d: ProcessSecurity() = %v", width, err)
		}
	}

	// Out-of-range CMAC widths are still refused.
	for _, width := range []int{0, 17} {
		sa := newCMACSA(t)
		sa.FieldLengths.MAC = width
		if err := sa.Validate(); !errors.Is(err, sdls.ErrInvalidFieldLengths) {
			t.Errorf("MAC width %d: Validate() = %v, want ErrInvalidFieldLengths", width, err)
		}
	}

	// The 12-octet floor still holds for GCM and GMAC.
	for _, mode := range []sdls.Mode{sdls.Authentication, sdls.AuthenticatedEncryption} {
		sa := newTestSA(t, mode)
		sa.FieldLengths.MAC = 8
		if err := sa.Validate(); !errors.Is(err, sdls.ErrInvalidFieldLengths) {
			t.Errorf("GCM mode %v with an 8-octet MAC: Validate() = %v, want ErrInvalidFieldLengths", mode, err)
		}
	}
}

// TestCMACAntiReplay checks the sequence window still applies: replaying a
// frame that already verified must be refused.
func TestCMACAntiReplay(t *testing.T) {
	sender := newCMACSA(t)
	receiver := newCMACSA(t)
	frameHeader := []byte{0x20, 0x00, 0x00, 0x0A}

	first, err := sender.ApplySecurity(frameHeader, []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := sender.ApplySecurity(frameHeader, []byte("two"))
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := sdls.ProcessSecurity(second, frameHeader, sdls.StaticLookup(receiver)); err != nil {
		t.Fatalf("the second frame should verify: %v", err)
	}
	// The first is now older than what the receiver has accepted.
	if _, _, err := sdls.ProcessSecurity(first, frameHeader, sdls.StaticLookup(receiver)); err == nil {
		t.Error("a replayed frame was accepted")
	}
}
