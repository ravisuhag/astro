package sdls_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// testKey is a fixed AES-256 key. Test vectors only; never a real key.
var testKey = bytes.Repeat([]byte{0x0F}, sdls.AESKeySize)

// baselineLengths is the CCSDS 355.0-B-2 clause E1 baseline: 96-bit IV, no explicit
// sequence number, no padding, 128-bit MAC. Security Header is 14 octets.
var baselineLengths = sdls.FieldLengths{IV: sdls.GCMIVSize, SeqNum: 0, PadLen: 0, MAC: 16}

func newTestSA(t *testing.T, mode sdls.Mode) *sdls.SecurityAssociation {
	t.Helper()
	sa := &sdls.SecurityAssociation{
		SPI:          1,
		Mode:         mode,
		Key:          testKey,
		FieldLengths: baselineLengths,
		SeqWindow:    100,
	}
	if err := sa.Validate(); err != nil {
		t.Fatalf("test SA is invalid: %v", err)
	}
	return sa
}

func TestBaselineSecurityHeaderSize(t *testing.T) {
	// Clause E1.2: the baseline Security Header is 14 octets.
	if got := baselineLengths.HeaderSize(); got != 14 {
		t.Errorf("baseline header size = %d, want 14", got)
	}
}

func TestSecurityHeaderRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		fl   sdls.FieldLengths
	}{
		{"baseline: IV only", sdls.FieldLengths{IV: 12, MAC: 16}},
		{"explicit sequence number", sdls.FieldLengths{IV: 12, SeqNum: 4, MAC: 16}},
		{"pad length present", sdls.FieldLengths{IV: 12, SeqNum: 4, PadLen: 1, MAC: 16}},
		{"no optional fields", sdls.FieldLengths{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &sdls.SecurityHeader{SPI: 0x1234}
			if tt.fl.IV > 0 {
				h.IV = bytes.Repeat([]byte{0xAA}, tt.fl.IV)
			}
			if tt.fl.SeqNum > 0 {
				h.SeqNum = bytes.Repeat([]byte{0xBB}, tt.fl.SeqNum)
			}
			if tt.fl.PadLen > 0 {
				h.PadLength = bytes.Repeat([]byte{0x00}, tt.fl.PadLen)
			}

			encoded, err := h.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != tt.fl.HeaderSize() {
				t.Fatalf("encoded %d octets, want %d", len(encoded), tt.fl.HeaderSize())
			}

			decoded, consumed, err := sdls.DecodeSecurityHeader(encoded, tt.fl)
			if err != nil {
				t.Fatal(err)
			}
			if consumed != tt.fl.HeaderSize() {
				t.Errorf("consumed %d, want %d", consumed, tt.fl.HeaderSize())
			}
			if decoded.SPI != h.SPI {
				t.Errorf("SPI = %#x, want %#x", decoded.SPI, h.SPI)
			}
			if !bytes.Equal(decoded.IV, h.IV) {
				t.Errorf("IV = %x, want %x", decoded.IV, h.IV)
			}
			if !bytes.Equal(decoded.SeqNum, h.SeqNum) {
				t.Errorf("SeqNum = %x, want %x", decoded.SeqNum, h.SeqNum)
			}
		})
	}
}

func TestSecurityHeaderSPIPosition(t *testing.T) {
	// Clause 4.1.1.2.1: bits 0-15 of the Security Header are the SPI, big-endian.
	h := &sdls.SecurityHeader{SPI: 0xBEEF, IV: bytes.Repeat([]byte{1}, 12)}
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint16(encoded[:2]); got != 0xBEEF {
		t.Errorf("leading 2 octets = %#x, want 0xBEEF", got)
	}
}

func TestDecodeSecurityHeaderShortInput(t *testing.T) {
	fl := baselineLengths
	full := make([]byte, fl.HeaderSize())
	for cut := 0; cut < fl.HeaderSize(); cut++ {
		if _, _, err := sdls.DecodeSecurityHeader(full[:cut], fl); !errors.Is(err, sdls.ErrDataTooShort) {
			t.Errorf("length %d: error = %v, want ErrDataTooShort", cut, err)
		}
	}
}

func TestSecurityHeaderTooLong(t *testing.T) {
	// Clause 4.1.1.1.4: a Security Header is at most 64 octets.
	fl := sdls.FieldLengths{IV: 63, MAC: 16}
	if err := fl.Validate(); !errors.Is(err, sdls.ErrHeaderTooLong) {
		t.Errorf("error = %v, want ErrHeaderTooLong", err)
	}
}

func TestSecurityAssociationValidate(t *testing.T) {
	valid := func() *sdls.SecurityAssociation {
		return &sdls.SecurityAssociation{
			SPI: 1, Mode: sdls.AuthenticatedEncryption,
			Key: testKey, FieldLengths: baselineLengths,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*sdls.SecurityAssociation)
		wantErr error
	}{
		{"valid authenticated encryption", func(*sdls.SecurityAssociation) {}, nil},
		{"valid authentication", func(sa *sdls.SecurityAssociation) { sa.Mode = sdls.Authentication }, nil},
		// Clause 4.1.1.2.3 reserves both extremes.
		{"SPI zero reserved", func(sa *sdls.SecurityAssociation) { sa.SPI = 0 }, sdls.ErrInvalidSPI},
		{"SPI all ones reserved", func(sa *sdls.SecurityAssociation) { sa.SPI = 65535 }, sdls.ErrInvalidSPI},
		{"mode zero", func(sa *sdls.SecurityAssociation) { sa.Mode = 0 }, sdls.ErrInvalidMode},
		{"mode unknown", func(sa *sdls.SecurityAssociation) { sa.Mode = sdls.Mode(99) }, sdls.ErrInvalidMode},
		{"key too short", func(sa *sdls.SecurityAssociation) { sa.Key = testKey[:16] }, sdls.ErrInvalidKey},
		{"key nil", func(sa *sdls.SecurityAssociation) { sa.Key = nil }, sdls.ErrInvalidKey},
		{"negative IV length", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.IV = -1 }, sdls.ErrInvalidFieldLengths},
		{"IV not 12 for GCM", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.IV = 8 }, sdls.ErrInvalidFieldLengths},
		{"MAC zero with auth", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.MAC = 0 }, sdls.ErrInvalidFieldLengths},
		{"MAC below minimum", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.MAC = 8 }, sdls.ErrInvalidFieldLengths},
		{"MAC above maximum", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.MAC = 20 }, sdls.ErrInvalidFieldLengths},
		{"header over 64 octets", func(sa *sdls.SecurityAssociation) { sa.FieldLengths.SeqNum = 100 }, sdls.ErrHeaderTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := valid()
			tt.mutate(sa)
			err := sa.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyProcessRoundTrip(t *testing.T) {
	for _, mode := range []sdls.Mode{sdls.AuthenticatedEncryption, sdls.Authentication} {
		t.Run(mode.String(), func(t *testing.T) {
			for _, aadCase := range []struct {
				name        string
				frameHeader []byte
			}{
				{"with frame header", []byte{0x01, 0xA2, 0x00, 0x00, 0x18, 0x00}},
				{"without frame header", nil},
			} {
				t.Run(aadCase.name, func(t *testing.T) {
					tx := newTestSA(t, mode)
					rx := newTestSA(t, mode)

					plaintext := []byte("telemetry payload")
					protected, err := tx.ApplySecurity(aadCase.frameHeader, plaintext)
					if err != nil {
						t.Fatalf("ApplySecurity: %v", err)
					}

					header, got, err := sdls.ProcessSecurity(protected, aadCase.frameHeader, sdls.StaticLookup(rx))
					if err != nil {
						t.Fatalf("ProcessSecurity: %v", err)
					}
					if !bytes.Equal(got, plaintext) {
						t.Errorf("recovered %q, want %q", got, plaintext)
					}
					if header.SPI != tx.SPI {
						t.Errorf("SPI = %d, want %d", header.SPI, tx.SPI)
					}

					// Authentication leaves the data field readable on the wire;
					// authenticated encryption must not.
					body := protected[baselineLengths.HeaderSize() : len(protected)-baselineLengths.MAC]
					if mode == sdls.Authentication && !bytes.Equal(body, plaintext) {
						t.Error("authentication mode altered the data field")
					}
					if mode == sdls.AuthenticatedEncryption && bytes.Contains(protected, plaintext) {
						t.Error("authenticated encryption left plaintext on the wire")
					}
				})
			}
		})
	}
}

func TestApplySecurityLayoutMatchesRawGCM(t *testing.T) {
	// Prove the wire layout by decrypting with a bare stdlib GCM call:
	// Security Header || ciphertext || tag, associated data = masked prefix
	// with the IV zeroed (clause 4.2.2.6.2 h).
	sa := newTestSA(t, sdls.AuthenticatedEncryption)
	frameHeader := []byte{0x01, 0xA2, 0x00, 0x00}
	plaintext := []byte("payload")

	protected, err := sa.ApplySecurity(frameHeader, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	hdrSize := baselineLengths.HeaderSize()
	headerBytes := protected[:hdrSize]
	iv := headerBytes[sdls.SPISize : sdls.SPISize+sdls.GCMIVSize]

	// Rebuild the associated data by hand.
	prefix := append(append([]byte{}, frameHeader...), headerBytes...)
	for i := len(frameHeader) + sdls.SPISize; i < len(frameHeader)+sdls.SPISize+sdls.GCMIVSize; i++ {
		prefix[i] = 0
	}

	block, err := aes.NewCipher(testKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCMWithTagSize(block, baselineLengths.MAC)
	if err != nil {
		t.Fatal(err)
	}

	got, err := gcm.Open(nil, iv, protected[hdrSize:], prefix)
	if err != nil {
		t.Fatalf("raw GCM could not open the frame: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("raw GCM recovered %q, want %q", got, plaintext)
	}
}

func TestApplySecurityNeverRepeatsIV(t *testing.T) {
	sa := newTestSA(t, sdls.AuthenticatedEncryption)
	seen := make(map[string]bool)

	for i := 0; i < 64; i++ {
		protected, err := sa.ApplySecurity(nil, []byte("payload"))
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		iv := string(protected[sdls.SPISize : sdls.SPISize+sdls.GCMIVSize])
		if seen[iv] {
			t.Fatalf("call %d reused an IV: %x", i, iv)
		}
		seen[iv] = true
	}
}

func TestApplySecurityRejectsEncryptionMode(t *testing.T) {
	sa := &sdls.SecurityAssociation{
		SPI: 1, Mode: sdls.Encryption, Key: testKey,
		FieldLengths: sdls.FieldLengths{IV: 12},
	}
	if _, err := sa.ApplySecurity(nil, []byte("x")); !errors.Is(err, sdls.ErrUnsupportedMode) {
		t.Errorf("error = %v, want ErrUnsupportedMode", err)
	}
}
