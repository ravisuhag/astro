package sdls_test

import (
	"fmt"
	"testing"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/sdls"
)

// The wire vectors for this package live in vectors/sdls/. Each was
// computed two ways before being pinned — once by ApplySecurity and once
// from first principles with an independent AES-GCM or CMAC — and the
// first-principles recomputation stays in kat_test.go, because a fixture
// records the answer while that test records the derivation.
//
// Test keys only. Never real keys.

func saFrom(config vectors.Fields, spi uint64) (*sdls.SecurityAssociation, error) {
	modeName, err := config.Str("mode")
	if err != nil {
		return nil, err
	}
	var mode sdls.Mode
	switch modeName {
	case "authentication":
		mode = sdls.Authentication
	case "encryption":
		mode = sdls.Encryption
	case "authenticated_encryption":
		mode = sdls.AuthenticatedEncryption
	default:
		return nil, fmt.Errorf("unknown mode %q", modeName)
	}

	algName, err := config.Str("auth_algorithm")
	if err != nil {
		return nil, err
	}
	var alg sdls.AuthAlgorithm
	switch algName {
	case "gmac":
		alg = sdls.AuthGMAC
	case "cmac":
		alg = sdls.AuthCMAC
	default:
		return nil, fmt.Errorf("unknown auth algorithm %q", algName)
	}

	ivLen, err := config.UintOr("iv_len", 0)
	if err != nil {
		return nil, err
	}
	seqLen, err := config.UintOr("seq_len", 0)
	if err != nil {
		return nil, err
	}
	macLen, err := config.UintOr("mac_len", 0)
	if err != nil {
		return nil, err
	}
	key, err := config.Hex("key")
	if err != nil {
		return nil, err
	}

	sa := &sdls.SecurityAssociation{
		SPI:           uint16(spi),
		Mode:          mode,
		AuthAlgorithm: alg,
		Key:           key,
		FieldLengths: sdls.FieldLengths{
			IV:     int(ivLen),
			SeqNum: int(seqLen),
			MAC:    int(macLen),
		},
		SeqWindow: 10,
	}
	return sa, sa.Validate()
}

func TestProtectedFrameVectors(t *testing.T) {
	vectors.RunFile(t, "sdls/protected-frame.json", vectors.Impl{
		EncodeFn: func(f, config vectors.Fields) ([]byte, error) {
			spi, err := f.Uint("spi")
			if err != nil {
				return nil, err
			}
			// The key and header are per-vector inputs on encode, so lift
			// them into the config the SA builder reads.
			merged := vectors.Fields{}
			for k, v := range config {
				merged[k] = v
			}
			merged["key"] = f["key"]

			sa, err := saFrom(merged, spi)
			if err != nil {
				return nil, err
			}
			header, err := f.Hex("frame_header")
			if err != nil {
				return nil, err
			}
			plaintext, err := f.Hex("plaintext")
			if err != nil {
				return nil, err
			}
			return sa.ApplySecurity(header, plaintext)
		},

		DecodeFn: func(input []byte, config vectors.Fields) (vectors.Fields, error) {
			sa, err := saFrom(config, 0x0042)
			if err != nil {
				return nil, err
			}
			header, err := config.Hex("frame_header")
			if err != nil {
				return nil, err
			}
			gotHeader, data, err := sdls.ProcessSecurity(input, header, sdls.StaticLookup(sa))
			if err != nil {
				return nil, err
			}
			return vectors.Fields{
				"spi":       gotHeader.SPI,
				"plaintext": data,
			}, nil
		},
	})
}
