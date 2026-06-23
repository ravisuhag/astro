package sdls_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// protectedFrame builds one protected data field plus the frame header the
// sender authenticated.
func protectedFrame(t *testing.T, mode sdls.Mode) (protected, frameHeader []byte, rx *sdls.SecurityAssociation) {
	t.Helper()
	tx := newTestSA(t, mode)
	rx = newTestSA(t, mode)
	frameHeader = []byte{0x01, 0xA2, 0x00, 0x00}

	var err error
	protected, err = tx.ApplySecurity(frameHeader, []byte("telemetry payload"))
	if err != nil {
		t.Fatalf("ApplySecurity: %v", err)
	}
	return protected, frameHeader, rx
}

func TestProcessSecurityRejectsTamperedInput(t *testing.T) {
	hdrSize := baselineLengths.HeaderSize()

	tests := []struct {
		name    string
		corrupt func(protected, frameHeader []byte) (newProtected, newHeader []byte)
		wantErr error
	}{
		{
			name: "flipped ciphertext byte",
			corrupt: func(p, h []byte) ([]byte, []byte) {
				out := append([]byte{}, p...)
				out[hdrSize] ^= 0x01
				return out, h
			},
			wantErr: sdls.ErrAuthenticationFailed,
		},
		{
			name: "flipped MAC byte",
			corrupt: func(p, h []byte) ([]byte, []byte) {
				out := append([]byte{}, p...)
				out[len(out)-1] ^= 0x01
				return out, h
			},
			wantErr: sdls.ErrAuthenticationFailed,
		},
		{
			name: "flipped frame header byte",
			corrupt: func(p, h []byte) ([]byte, []byte) {
				hdr := append([]byte{}, h...)
				hdr[0] ^= 0x01
				return p, hdr
			},
			wantErr: sdls.ErrAuthenticationFailed,
		},
		{
			name: "truncated MAC",
			corrupt: func(p, h []byte) ([]byte, []byte) {
				return p[:len(p)-1], h
			},
			wantErr: sdls.ErrAuthenticationFailed,
		},
		{
			name: "sequence number field altered",
			corrupt: func(p, h []byte) ([]byte, []byte) {
				out := append([]byte{}, p...)
				// The IV doubles as the sequence counter in the baseline;
				// changing it must break the MAC via the counter check.
				out[sdls.SPISize] ^= 0xFF
				return out, h
			},
			wantErr: sdls.ErrAuthenticationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protected, frameHeader, rx := protectedFrame(t, sdls.AuthenticatedEncryption)
			corrupted, header := tt.corrupt(protected, frameHeader)

			_, data, err := sdls.ProcessSecurity(corrupted, header, sdls.StaticLookup(rx))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if data != nil {
				t.Errorf("returned %d bytes of data on failure; must return nil", len(data))
			}
		})
	}
}

func TestProcessSecurityWrongKey(t *testing.T) {
	protected, frameHeader, rx := protectedFrame(t, sdls.AuthenticatedEncryption)
	rx.Key = bytes.Repeat([]byte{0xAB}, sdls.AESKeySize)

	_, data, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(rx))
	if !errors.Is(err, sdls.ErrAuthenticationFailed) {
		t.Errorf("error = %v, want ErrAuthenticationFailed", err)
	}
	if data != nil {
		t.Error("returned data with the wrong key")
	}
}

func TestProcessSecurityUnknownSPI(t *testing.T) {
	protected, frameHeader, rx := protectedFrame(t, sdls.AuthenticatedEncryption)
	rx.SPI = 7 // registered under a different index

	_, _, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(rx))
	if !errors.Is(err, sdls.ErrUnknownSPI) {
		t.Errorf("error = %v, want ErrUnknownSPI", err)
	}
}

func TestProcessSecurityNilLookup(t *testing.T) {
	protected, frameHeader, _ := protectedFrame(t, sdls.AuthenticatedEncryption)
	if _, _, err := sdls.ProcessSecurity(protected, frameHeader, nil); !errors.Is(err, sdls.ErrUnknownSPI) {
		t.Errorf("error = %v, want ErrUnknownSPI", err)
	}
}

func TestProcessSecurityShortInputAtEveryBoundary(t *testing.T) {
	protected, frameHeader, rx := protectedFrame(t, sdls.AuthenticatedEncryption)
	lookup := sdls.StaticLookup(rx)

	for cut := 0; cut < baselineLengths.HeaderSize()+baselineLengths.MAC; cut++ {
		_, data, err := sdls.ProcessSecurity(protected[:cut], frameHeader, lookup)
		if err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
		if data != nil {
			t.Errorf("length %d: returned data on failure", cut)
		}
	}
}

func TestProcessSecurityReplayDetection(t *testing.T) {
	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x01, 0xA2}
	lookup := sdls.StaticLookup(rx)

	first, err := tx.ApplySecurity(frameHeader, []byte("frame one"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sdls.ProcessSecurity(first, frameHeader, lookup); err != nil {
		t.Fatalf("first frame rejected: %v", err)
	}

	// §2.3.2.3.2: a counter at or below the stored one is discarded.
	_, data, err := sdls.ProcessSecurity(first, frameHeader, lookup)
	if !errors.Is(err, sdls.ErrReplayDetected) {
		t.Errorf("replayed frame: error = %v, want ErrReplayDetected", err)
	}
	if data != nil {
		t.Error("replayed frame returned data")
	}

	// A fresh frame with a higher counter still gets through.
	second, err := tx.ApplySecurity(frameHeader, []byte("frame two"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sdls.ProcessSecurity(second, frameHeader, lookup); err != nil {
		t.Errorf("second frame rejected: %v", err)
	}
}

func TestProcessSecurityBeyondSequenceWindow(t *testing.T) {
	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.SeqWindow = 4 // §2.3.2.3.3
	frameHeader := []byte{0x01}
	lookup := sdls.StaticLookup(rx)

	first, err := tx.ApplySecurity(frameHeader, []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sdls.ProcessSecurity(first, frameHeader, lookup); err != nil {
		t.Fatalf("first frame rejected: %v", err)
	}

	// Burn well past the window on the sender side only.
	var far []byte
	for i := 0; i < 20; i++ {
		far, err = tx.ApplySecurity(frameHeader, []byte("far ahead"))
		if err != nil {
			t.Fatal(err)
		}
	}

	_, data, err := sdls.ProcessSecurity(far, frameHeader, lookup)
	if !errors.Is(err, sdls.ErrReplayDetected) {
		t.Errorf("far-ahead frame: error = %v, want ErrReplayDetected", err)
	}
	if data != nil {
		t.Error("far-ahead frame returned data")
	}
}

func TestForgedFrameDoesNotAdvanceReplayWindow(t *testing.T) {
	// A failed MAC must leave the receiver's window untouched, or an attacker
	// could lock out the real sender (§4.2.4.4).
	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x01}
	lookup := sdls.StaticLookup(rx)

	genuine, err := tx.ApplySecurity(frameHeader, []byte("genuine"))
	if err != nil {
		t.Fatal(err)
	}

	forged := append([]byte{}, genuine...)
	forged[len(forged)-1] ^= 0xFF
	if _, _, err := sdls.ProcessSecurity(forged, frameHeader, lookup); !errors.Is(err, sdls.ErrAuthenticationFailed) {
		t.Fatalf("forged frame: error = %v, want ErrAuthenticationFailed", err)
	}

	// The genuine frame with the same counter must still be accepted.
	if _, _, err := sdls.ProcessSecurity(genuine, frameHeader, lookup); err != nil {
		t.Errorf("genuine frame rejected after a forgery: %v", err)
	}
}

func TestAuthenticationModeDetectsDataTampering(t *testing.T) {
	// Authentication leaves the data readable, so prove it is still covered.
	protected, frameHeader, rx := protectedFrame(t, sdls.Authentication)

	tampered := append([]byte{}, protected...)
	tampered[baselineLengths.HeaderSize()] ^= 0x01

	_, data, err := sdls.ProcessSecurity(tampered, frameHeader, sdls.StaticLookup(rx))
	if !errors.Is(err, sdls.ErrAuthenticationFailed) {
		t.Errorf("error = %v, want ErrAuthenticationFailed", err)
	}
	if data != nil {
		t.Error("returned data for a tampered authenticated frame")
	}
}

func TestMaskTooShort(t *testing.T) {
	sa := newTestSA(t, sdls.AuthenticatedEncryption)
	sa.AuthMask = []byte{0xFF} // far shorter than header + security header

	if _, err := sa.ApplySecurity([]byte{0x01, 0x02, 0x03, 0x04}, []byte("x")); !errors.Is(err, sdls.ErrMaskTooShort) {
		t.Errorf("error = %v, want ErrMaskTooShort", err)
	}
}

func TestAuthMaskExcludesMaskedHeaderBits(t *testing.T) {
	// A mask of zeros over the frame header means those octets are not
	// authenticated, so changing them must not break verification
	// (§4.2.2.6.2 j allows exactly this for unspecified fields).
	frameHeader := []byte{0x11, 0x22, 0x33, 0x44}
	mask := make([]byte, len(frameHeader)+baselineLengths.HeaderSize())
	for i := len(frameHeader); i < len(mask); i++ {
		mask[i] = 0xFF // security header stays authenticated
	}

	tx := newTestSA(t, sdls.AuthenticatedEncryption)
	tx.AuthMask = mask
	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.AuthMask = mask

	protected, err := tx.ApplySecurity(frameHeader, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	changed := []byte{0x99, 0x88, 0x77, 0x66}
	if _, _, err := sdls.ProcessSecurity(protected, changed, sdls.StaticLookup(rx)); err != nil {
		t.Errorf("masked-out header change broke verification: %v", err)
	}
}

func TestIVIsExcludedFromMACRegardlessOfMask(t *testing.T) {
	// §4.2.2.6.2 h) excludes the IV from the authenticated data. Verify the
	// implementation enforces that even when the mask is all ones: the frame
	// must still verify, which it only can if both sides zero the IV.
	sa := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x01, 0x02}
	mask := bytes.Repeat([]byte{0xFF}, len(frameHeader)+baselineLengths.HeaderSize())
	sa.AuthMask = mask

	rx := newTestSA(t, sdls.AuthenticatedEncryption)
	rx.AuthMask = mask

	protected, err := sa.ApplySecurity(frameHeader, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sdls.ProcessSecurity(protected, frameHeader, sdls.StaticLookup(rx)); err != nil {
		t.Errorf("round trip failed with an all-ones mask: %v", err)
	}
}
