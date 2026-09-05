package sdls_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/sdls"
)

// Benchmarks for the per-frame security path.
//
// SDLS runs on every protected TM, AOS, or USLP frame, the same frame rate as
// pkg/tmsc and pkg/pxsc, but until now nothing in this package measured it.
// These benchmarks do not change ApplySecurity or ProcessSecurity: the point
// here is only the number, not an optimisation.
//
// Run with:
//
//	go test -bench . -benchmem ./pkg/sdls/

var sdlsSink []byte

// benchKey is a fixed 256-bit AES key. Test fixture only, never a real key.
var benchKey = func() []byte {
	k := make([]byte, sdls.AESKeySize)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}()

// benchFrameHeader models the carrier frame octets ahead of the Security
// Header: a six-octet TM-like primary header, the same shape kat_test.go
// uses.
var benchFrameHeader = []byte{0x02, 0x3e, 0x0a, 0x0b, 0x18, 0x00}

// benchPlaintext is a 1115-octet data field, the CADU-sized frame pkg/tmsc's
// own benchmarks use for the same reason: it is representative of a filled
// TM Transfer Frame.
func benchPlaintext() []byte {
	data := make([]byte, 1115)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

// benchSAs are the three per-frame configurations worth measuring
// separately: AES-GCM authenticated encryption (clause E1/E3/E4), GMAC
// authentication-only, and the AES-CMAC telecommand baseline (clause E2).
func benchSAs() []struct {
	name string
	sa   *sdls.SecurityAssociation
} {
	return []struct {
		name string
		sa   *sdls.SecurityAssociation
	}{
		{
			name: "AuthenticatedEncryption",
			sa: &sdls.SecurityAssociation{
				SPI:           0x0042,
				Mode:          sdls.AuthenticatedEncryption,
				AuthAlgorithm: sdls.AuthGMAC,
				Key:           benchKey,
				FieldLengths:  sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16},
				SeqWindow:     10,
			},
		},
		{
			name: "Authentication/GMAC",
			sa: &sdls.SecurityAssociation{
				SPI:           0x0042,
				Mode:          sdls.Authentication,
				AuthAlgorithm: sdls.AuthGMAC,
				Key:           benchKey,
				FieldLengths:  sdls.FieldLengths{IV: sdls.GCMIVSize, MAC: 16},
				SeqWindow:     10,
			},
		},
		{
			name: "Authentication/CMAC",
			sa: &sdls.SecurityAssociation{
				SPI:           0x0042,
				Mode:          sdls.Authentication,
				AuthAlgorithm: sdls.AuthCMAC,
				Key:           benchKey,
				FieldLengths:  sdls.FieldLengths{SeqNum: 4, MAC: 16},
				SeqWindow:     10,
			},
		},
	}
}

// BenchmarkApplySecurity measures the sender side: building the Security
// Header, the masked Authentication Payload prefix, and sealing or tagging
// the data field.
func BenchmarkApplySecurity(b *testing.B) {
	plaintext := benchPlaintext()

	for _, tc := range benchSAs() {
		b.Run(tc.name, func(b *testing.B) {
			sa := tc.sa

			b.SetBytes(int64(len(plaintext)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				out, err := sa.ApplySecurity(benchFrameHeader, plaintext)
				if err != nil {
					b.Fatal(err)
				}
				sdlsSink = out
			}
		})
	}
}

// BenchmarkProcessSecurity measures the receiver side: decoding the Security
// Header, rebuilding the same masked prefix, and verifying (and for
// AuthenticatedEncryption, decrypting) the frame.
//
// Each subtest decodes one protected frame repeatedly rather than a stream of
// distinct ones. The receiving SA's anti-replay window is disabled
// (SeqWindow: 0) because a real window would reject the second decode of the
// same frame outright; that check is exercised elsewhere
// (TestProcessSecurityReplayDetection and friends), not here.
func BenchmarkProcessSecurity(b *testing.B) {
	plaintext := benchPlaintext()

	for _, tc := range benchSAs() {
		b.Run(tc.name, func(b *testing.B) {
			frame, err := tc.sa.ApplySecurity(benchFrameHeader, plaintext)
			if err != nil {
				b.Fatal(err)
			}

			rx := *tc.sa
			rx.SeqWindow = 0
			lookup := sdls.StaticLookup(&rx)

			b.SetBytes(int64(len(plaintext)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, data, err := sdls.ProcessSecurity(frame, benchFrameHeader, lookup)
				if err != nil {
					b.Fatal(err)
				}
				sdlsSink = data
			}
		})
	}
}
