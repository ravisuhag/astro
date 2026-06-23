package sdls_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

func FuzzDecodeSecurityHeader(f *testing.F) {
	// Seed with a valid baseline header plus degenerate inputs.
	h := &sdls.SecurityHeader{SPI: 1, IV: bytes.Repeat([]byte{0xAA}, 12)}
	if encoded, err := h.Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 2))
	f.Add(make([]byte, 14))

	// A small matrix of plausible SA layouts, exercised on every input.
	layouts := []sdls.FieldLengths{
		{IV: 0, SeqNum: 0, PadLen: 0, MAC: 16},
		{IV: 12, SeqNum: 0, PadLen: 0, MAC: 16},
		{IV: 12, SeqNum: 4, PadLen: 1, MAC: 12},
		{IV: 12, SeqNum: 4, PadLen: 1, MAC: 16},
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		for _, fl := range layouts {
			header, consumed, err := sdls.DecodeSecurityHeader(data, fl)
			if err != nil {
				continue
			}
			if consumed != fl.HeaderSize() {
				t.Fatalf("consumed %d, want %d", consumed, fl.HeaderSize())
			}
			if _, err := header.Encode(); err != nil {
				t.Fatalf("re-encoding a decoded header failed: %v", err)
			}
		}
	})
}

func FuzzProcessSecurity(f *testing.F) {
	key := bytes.Repeat([]byte{0x0F}, sdls.AESKeySize)
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}

	// Seed with one genuine protected frame.
	tx := &sdls.SecurityAssociation{SPI: 1, Mode: sdls.AuthenticatedEncryption, Key: key, FieldLengths: fl}
	if protected, err := tx.ApplySecurity([]byte{0x01, 0x02}, []byte("seed payload")); err == nil {
		f.Add(protected)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 2))
	f.Add(make([]byte, 30))

	f.Fuzz(func(t *testing.T, data []byte) {
		// A fresh SA per call: ProcessSecurity mutates anti-replay state.
		known := &sdls.SecurityAssociation{
			SPI: 1, Mode: sdls.AuthenticatedEncryption, Key: key, FieldLengths: fl, SeqWindow: 1000,
		}
		anySPI := func(uint16) (*sdls.SecurityAssociation, error) { return known, nil }
		noSPI := func(uint16) (*sdls.SecurityAssociation, error) { return nil, sdls.ErrUnknownSPI }

		// Property: never panic, and never return data alongside an error.
		for _, lookup := range []sdls.SALookup{sdls.SALookup(anySPI), sdls.SALookup(noSPI), sdls.StaticLookup(known)} {
			_, plaintext, err := sdls.ProcessSecurity(data, []byte{0x01, 0x02}, lookup)
			if err != nil && plaintext != nil {
				t.Fatalf("returned %d bytes of plaintext with error %v", len(plaintext), err)
			}
		}
	})
}
