package sdls_test

// Frame-level known-answer tests.
//
// Each test pins the exact wire bytes of one protected frame (Security
// Header, data field, and MAC) under a fixed key, a fixed frame header, and
// the SA's first counter value. The expected bytes are computed two ways:
// once by ApplySecurity, and once here from first principles with the
// standard library, building the Authentication Payload by hand in the
// Clause 4.2.3 order (masked frame header, then the security header with a zeroed
// IV, then the data field). Both must equal the pinned constant, so any
// change to the AAD ordering, the IV placement, or the header layout fails
// loudly rather than round-tripping quietly.

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/internal/cmac"
	"github.com/ravisuhag/astro/pkg/sdls"
)

// The fixed inputs shared by all three known-answer tests. Test vectors
// only; never real keys.
var (
	// katKey is 000102...1f: the AES-256 key.
	katKey = mustHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	// katFrameHeader is a six-octet TM-like primary header.
	katFrameHeader = mustHex("023e0a0b1800")

	// katPlaintext is the transfer frame data field to protect.
	katPlaintext = []byte("astro sdls kat")
)

const katSPI = 0x0042

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// katSA builds the SA under test with the shared fixed inputs.
func katSA(t *testing.T, mode sdls.Mode, alg sdls.AuthAlgorithm, fl sdls.FieldLengths) *sdls.SecurityAssociation {
	t.Helper()
	sa := &sdls.SecurityAssociation{
		SPI:           katSPI,
		Mode:          mode,
		AuthAlgorithm: alg,
		Key:           katKey,
		FieldLengths:  fl,
		SeqWindow:     10,
	}
	if err := sa.Validate(); err != nil {
		t.Fatalf("KAT SA invalid: %v", err)
	}
	return sa
}

// checkKAT asserts that ApplySecurity and the independent construction both
// produce exactly the pinned bytes, and that ProcessSecurity accepts them.
func checkKAT(t *testing.T, sa *sdls.SecurityAssociation, independent []byte, wantHex string) {
	t.Helper()

	want := mustHex(wantHex)

	got, err := sa.ApplySecurity(katFrameHeader, katPlaintext)
	if err != nil {
		t.Fatalf("ApplySecurity: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ApplySecurity:\n got  %x\n want %x", got, want)
	}
	if !bytes.Equal(independent, want) {
		t.Errorf("independent construction:\n got  %x\n want %x", independent, want)
	}

	rx := *sa // receiver with its own anti-replay state
	header, data, err := sdls.ProcessSecurity(want, katFrameHeader, sdls.StaticLookup(&rx))
	if err != nil {
		t.Fatalf("ProcessSecurity on the pinned frame: %v", err)
	}
	if header.SPI != katSPI {
		t.Errorf("SPI = %#x, want %#x", header.SPI, katSPI)
	}
	if !bytes.Equal(data, katPlaintext) {
		t.Errorf("recovered %q, want %q", data, katPlaintext)
	}
}

// TestKnownAnswerGCM pins the clause E1/clause E3/clause E4 baseline: AES-256-GCM
// authenticated encryption, 96-bit IV, 128-bit MAC. The first frame carries
// IV counter value 1. The associated data is the frame header followed by
// the security header with the IV zeroed.
func TestKnownAnswerGCM(t *testing.T) {
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}
	sa := katSA(t, sdls.AuthenticatedEncryption, sdls.AuthGMAC, fl)

	iv := mustHex("000000000000000000000001")
	spi := []byte{0x00, katSPI}

	// AAD per clause 4.2.3.2.2.3 a): frame header, SPI, then the IV field as zeros
	// (clause 4.2.2.6.2 h) excludes the IV from the authenticated data).
	aad := concat(katFrameHeader, spi, make([]byte, sdls.GCMIVSize))

	block, err := aes.NewCipher(katKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, iv, katPlaintext, aad)
	independent := concat(spi, iv, sealed)

	checkKAT(t, sa, independent,
		"0042"+ // SPI
			"000000000000000000000001"+ // IV, first counter value
			"74a5cb8e2bd4437a625d71528dd2"+ // ciphertext (14 octets)
			"2a780a5a36b2578da5be98c21c714557") // 16-octet GCM tag
}

// TestKnownAnswerGMAC pins the authentication-only GMAC companion of the GCM
// baseline: nothing is encrypted, and the MAC covers the whole
// Authentication Payload (masked prefix, then the data field) as
// associated data over an empty plaintext.
func TestKnownAnswerGMAC(t *testing.T) {
	fl := sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16}
	sa := katSA(t, sdls.Authentication, sdls.AuthGMAC, fl)

	iv := mustHex("000000000000000000000001")
	spi := []byte{0x00, katSPI}

	// Clause 4.2.3.2.2.2: the Authentication Payload is prefix then data field.
	aad := concat(katFrameHeader, spi, make([]byte, sdls.GCMIVSize), katPlaintext)

	block, err := aes.NewCipher(katKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	tag := gcm.Seal(nil, iv, nil, aad)
	independent := concat(spi, iv, katPlaintext, tag)

	checkKAT(t, sa, independent,
		"0042"+ // SPI
			"000000000000000000000001"+ // IV, first counter value
			hex.EncodeToString(katPlaintext)+ // data field, in the clear
			"01473b4b99175664d8ee702ed7ff135d") // 16-octet GMAC tag
}

// TestKnownAnswerCMAC pins the clause E2 telecommand baseline: AES-CMAC with a
// 256-bit key, a 32-bit sequence number, a 6-octet security header, and a
// 128-bit MAC. The first frame carries sequence number 1. There is no IV to
// zero: the MAC covers the frame header, the full security header, and the
// data field, in that order.
func TestKnownAnswerCMAC(t *testing.T) {
	fl := sdls.FieldLengths{SeqNum: 4, MAC: 16}
	sa := katSA(t, sdls.Authentication, sdls.AuthCMAC, fl)

	spi := []byte{0x00, katSPI}
	seq := mustHex("00000001")

	mac, err := cmac.New(katKey)
	if err != nil {
		t.Fatal(err)
	}
	tag := mac.Sum(concat(katFrameHeader, spi, seq, katPlaintext))
	independent := concat(spi, seq, katPlaintext, tag)

	checkKAT(t, sa, independent,
		"0042"+ // SPI
			"00000001"+ // sequence number, first counter value
			hex.EncodeToString(katPlaintext)+ // data field, in the clear
			"2d2d626240a8bdda645103c30796e642") // 16-octet CMAC tag
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
