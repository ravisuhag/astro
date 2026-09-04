package keywrap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

// The six vectors of RFC 3394 clause 4, transcribed from the document. Each
// section prints the key encryption key, the key data and the resulting
// ciphertext; the intermediate steps the RFC also prints are not reproduced
// here, because agreeing on the output means agreeing on all of them.
var rfc3394Vectors = []struct {
	name string
	kek  string
	key  string
	want string
}{
	{
		name: "4.1 wrap 128 bits of key data with a 128-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F",
		key:  "00112233445566778899AABBCCDDEEFF",
		want: "1FA68B0A8112B447AEF34BD8FB5A7B829D3E862371D2CFE5",
	},
	{
		name: "4.2 wrap 128 bits of key data with a 192-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F1011121314151617",
		key:  "00112233445566778899AABBCCDDEEFF",
		want: "96778B25AE6CA435F92B5B97C050AED2468AB8A17AD84E5D",
	},
	{
		name: "4.3 wrap 128 bits of key data with a 256-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F",
		key:  "00112233445566778899AABBCCDDEEFF",
		want: "64E8C3F9CE0F5BA263E9777905818A2A93C8191E7D6E8AE7",
	},
	{
		name: "4.4 wrap 192 bits of key data with a 192-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F1011121314151617",
		key:  "00112233445566778899AABBCCDDEEFF0001020304050607",
		want: "031D33264E15D33268F24EC260743EDCE1C6C7DDEE725A936BA814915C6762D2",
	},
	{
		name: "4.5 wrap 192 bits of key data with a 256-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F",
		key:  "00112233445566778899AABBCCDDEEFF0001020304050607",
		want: "A8F9BC1612C68B3FF6E6F4FBE30E71E4769C8B80A32CB8958CD5D17D6B254DA1",
	},
	{
		name: "4.6 wrap 256 bits of key data with a 256-bit KEK",
		kek:  "000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F",
		key:  "00112233445566778899AABBCCDDEEFF000102030405060708090A0B0C0D0E0F",
		want: "28C9F404C4B810F4CBCCB35CFB87F8263F5786E2D80ED326CBC7F0E71A99F43BFB988B9B7A02DD21",
	},
}

func TestWrapMatchesRFC3394(t *testing.T) {
	for _, tt := range rfc3394Vectors {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Wrap(mustHex(t, tt.kek), mustHex(t, tt.key))
			if err != nil {
				t.Fatalf("Wrap: %v", err)
			}
			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Errorf("Wrap = %X, want %X", got, want)
			}
		})
	}
}

func TestUnwrapMatchesRFC3394(t *testing.T) {
	for _, tt := range rfc3394Vectors {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unwrap(mustHex(t, tt.kek), mustHex(t, tt.want))
			if err != nil {
				t.Fatalf("Unwrap: %v", err)
			}
			if want := mustHex(t, tt.key); !bytes.Equal(got, want) {
				t.Errorf("Unwrap = %X, want %X", got, want)
			}
		})
	}
}

// A wrong key encryption key must produce an error and no key data, which is
// the whole point of the constant initial value (RFC 3394 clause 2.2.3.1).
func TestUnwrapRejectsWrongKEK(t *testing.T) {
	kek := mustHex(t, "000102030405060708090A0B0C0D0E0F")
	wrapped := mustHex(t, "1FA68B0A8112B447AEF34BD8FB5A7B829D3E862371D2CFE5")

	wrong := make([]byte, len(kek))
	copy(wrong, kek)
	wrong[0] ^= 0x01

	got, err := Unwrap(wrong, wrapped)
	if !errors.Is(err, ErrIntegrityCheck) {
		t.Fatalf("Unwrap with a wrong KEK = %v, want ErrIntegrityCheck", err)
	}
	if got != nil {
		t.Errorf("Unwrap returned %X alongside the error; clause 2.2.2 requires no key data at all", got)
	}
}

// Altering any octet of the ciphertext must fail the same way.
func TestUnwrapRejectsAlteredCiphertext(t *testing.T) {
	kek := mustHex(t, "000102030405060708090A0B0C0D0E0F")
	wrapped := mustHex(t, "1FA68B0A8112B447AEF34BD8FB5A7B829D3E862371D2CFE5")

	for i := range wrapped {
		altered := make([]byte, len(wrapped))
		copy(altered, wrapped)
		altered[i] ^= 0x01

		if _, err := Unwrap(kek, altered); !errors.Is(err, ErrIntegrityCheck) {
			t.Errorf("Unwrap with octet %d altered = %v, want ErrIntegrityCheck", i, err)
		}
	}
}

func TestWrapRejectsBadLengths(t *testing.T) {
	kek := mustHex(t, "000102030405060708090A0B0C0D0E0F")

	tests := []struct {
		name string
		key  string
		want error
	}{
		{"empty", "", ErrKeyDataLength},
		{"one block", "0011223344556677", ErrKeyDataLength},
		{"not a whole block", "00112233445566778899AABBCCDDEEFF00", ErrKeyDataLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Wrap(kek, mustHex(t, tt.key)); !errors.Is(err, tt.want) {
				t.Errorf("Wrap = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUnwrapRejectsBadLengths(t *testing.T) {
	kek := mustHex(t, "000102030405060708090A0B0C0D0E0F")

	tests := []struct {
		name    string
		wrapped string
		want    error
	}{
		{"empty", "", ErrCiphertextLength},
		{"two blocks", "00112233445566778899AABBCCDDEEFF", ErrCiphertextLength},
		{"not a whole block", "1FA68B0A8112B447AEF34BD8FB5A7B829D3E862371D2CF", ErrCiphertextLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Unwrap(kek, mustHex(t, tt.wrapped)); !errors.Is(err, tt.want) {
				t.Errorf("Unwrap = %v, want %v", err, tt.want)
			}
		})
	}
}

// A key encryption key that is not a legal AES key size must be refused by
// crypto/aes rather than silently accepted.
func TestRejectsBadKEKSize(t *testing.T) {
	keyData := mustHex(t, "00112233445566778899AABBCCDDEEFF")
	if _, err := Wrap(make([]byte, 17), keyData); err == nil {
		t.Error("Wrap accepted a 17-octet key encryption key")
	}
	wrapped := mustHex(t, "1FA68B0A8112B447AEF34BD8FB5A7B829D3E862371D2CFE5")
	if _, err := Unwrap(make([]byte, 17), wrapped); err == nil {
		t.Error("Unwrap accepted a 17-octet key encryption key")
	}
}

// Wrap must not scribble on the key data it was handed.
func TestWrapDoesNotModifyInput(t *testing.T) {
	kek := mustHex(t, "000102030405060708090A0B0C0D0E0F")
	keyData := mustHex(t, "00112233445566778899AABBCCDDEEFF")
	before := make([]byte, len(keyData))
	copy(before, keyData)

	if _, err := Wrap(kek, keyData); err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if !bytes.Equal(keyData, before) {
		t.Errorf("Wrap modified its input: %X, was %X", keyData, before)
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad test hex %q: %v", s, err)
	}
	return b
}
