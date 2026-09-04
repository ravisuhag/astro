package bpsec_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// Octets transcribed from RFC 9173 appendix A. Every example in the appendix
// shares one original bundle: the primary block of A.1.1.1 and the 35-octet
// payload of A.1.1.2.
const (
	rfcOriginalBundle = "9f88070000820282010282028202018202820201820018281a000f42408501010" +
		"0005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff"

	rfcPrimaryBlock = "88070000820282010282028202018202820201820018281a000f4240"

	// The payload as it sits in the block-type-specific data field, without
	// the CBOR byte string head. Appendix A calls this the "Payload Data".
	rfcPayloadData = "526561647920746f2067656e657261746520612033322d62797465207061796c6f6164"

	// The same octets with their head, which is what the IPPT includes.
	rfcPayloadBTSD = "5823" + rfcPayloadData
)

// The IPPT and the AAD are the octets two implementations most easily disagree
// about, and appendix A prints both for every example. These pin them.
func TestIPPTMatchesRFC9173(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)

	tests := []struct {
		name     string
		scope    bpsec.ScopeFlags
		target   uint64
		security *bp.CanonicalBlock
		want     string
	}{
		{
			// A.1.3.1: scope flags 0x00, so the IPPT is the flags byte and the
			// target's block-type-specific data and nothing else.
			name:     "A.1 simple integrity, no additional scope",
			scope:    bpsec.ScopeNone,
			target:   bp.PayloadBlockNumber,
			security: &bp.CanonicalBlock{Type: bpsec.BlockTypeIntegrity, Number: 2},
			want:     "00" + rfcPayloadBTSD,
		},
		{
			// A.4.3.1: scope flags 0x07, BIB is block number 3 with no flags.
			// The appendix prints the pieces separately — primary block data,
			// payload header 010100, BIB header 0b0300 — and then the whole
			// IPPT, which is what this compares against.
			name:     "A.4 full scope",
			scope:    bpsec.ScopeAll,
			target:   bp.PayloadBlockNumber,
			security: &bp.CanonicalBlock{Type: bpsec.BlockTypeIntegrity, Number: 3},
			want:     "07" + rfcPrimaryBlock + "010100" + "0b0300" + rfcPayloadBTSD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bpsec.IPPT(bundle, tt.scope, tt.target, tt.security)
			if err != nil {
				t.Fatalf("IPPT: %v", err)
			}
			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Errorf("IPPT =\n\t%x\nwant\n\t%x", got, want)
			}
		})
	}
}

func TestAADMatchesRFC9173(t *testing.T) {
	// A.4 encrypts two blocks, so the BCB has two targets and the appendix
	// prints an AAD for each. Both are built here against the same bundle,
	// with the BIB present as block 3.
	bundle := decodeBundle(t, rfcOriginalBundle)
	bib := &bp.CanonicalBlock{
		Type:   bpsec.BlockTypeIntegrity,
		Number: 3,
		Data:   mustHex(t, "81010101820282020182820106820307818182015830f75fe4c37f76f046165855bd5ff72fbfd4e3a64b4695c40e2b787da005ae819f0a2e30a2e8b325527de8aefb52e73d71"),
	}
	bundle.Blocks = append([]*bp.CanonicalBlock{bib}, bundle.Blocks...)

	// The BCB is block 2 with the replicate-in-every-fragment flag set, which
	// is why its header canonicalizes to 0c0201 rather than 0c0200.
	bcb := &bp.CanonicalBlock{
		Type:   bpsec.BlockTypeConfidentiality,
		Number: 2,
		Flags:  bp.BlockFlagReplicateInEveryFragment,
	}

	tests := []struct {
		name   string
		target uint64
		want   string
	}{
		{
			name:   "A.4 payload AAD",
			target: bp.PayloadBlockNumber,
			want:   "07" + rfcPrimaryBlock + "010100" + "0c0201",
		},
		{
			name:   "A.4 BIB AAD",
			target: 3,
			want:   "07" + rfcPrimaryBlock + "0b0300" + "0c0201",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bpsec.AAD(bundle, bpsec.ScopeAll, tt.target, bcb)
			if err != nil {
				t.Fatalf("AAD: %v", err)
			}
			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Errorf("AAD =\n\t%x\nwant\n\t%x", got, want)
			}
		})
	}
}

// A.2 uses scope flags 0x00, where the AAD is just the flags byte. The
// appendix prints it as h'00', which is easy to mistake for "no AAD at all".
func TestAADWithNoAdditionalScope(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	bcb := &bp.CanonicalBlock{
		Type:   bpsec.BlockTypeConfidentiality,
		Number: 2,
		Flags:  bp.BlockFlagReplicateInEveryFragment,
	}

	got, err := bpsec.AAD(bundle, bpsec.ScopeNone, bp.PayloadBlockNumber, bcb)
	if err != nil {
		t.Fatalf("AAD: %v", err)
	}
	if want := mustHex(t, "00"); !bytes.Equal(got, want) {
		t.Errorf("AAD = %x, want %x", got, want)
	}
}

func decodeBundle(t *testing.T, hexString string) *bp.Bundle {
	t.Helper()
	b, err := bp.Decode(mustHex(t, hexString))
	if err != nil {
		t.Fatalf("decoding the bundle from RFC 9173: %v", err)
	}
	return b
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad test hex %q: %v", s, err)
	}
	return b
}
