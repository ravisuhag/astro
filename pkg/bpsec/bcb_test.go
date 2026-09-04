package bpsec_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// RFC 9173 appendix A.2: a BCB over the payload block with no additional
// scope, carrying its content encryption key wrapped under a key encryption
// key.
func TestConfidentialityMatchesRFC9173Example2(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)

	confidentiality := bpsec.Confidentiality{
		BlockNumber: 2,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES128GCM,
		Scope:       bpsec.ScopeNone,
		Key:         mustHex(t, "71776572747975696f70617364666768"),
		KEK:         mustHex(t, "6162636465666768696a6b6c6d6e6f70"),
		IV:          mustHex(t, "5477656c7665313231323132"),
	}

	bcb, err := confidentiality.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A.2.3.1 prints the wrapped key the RFC expects. Reproducing it proves
	// the key wrap and the key agree with the document, not just with us.
	wantWrapped := "69c411276fecddc4780df42c8a2af89296fabf34d7fae700"
	// A.2.3.1 also prints the ciphertext that replaces the payload.
	wantCiphertext := "3a09c1e63fe23a7f66a59c7303837241e070b02619fc59c5214a22f08cd70795e73e9a"
	if got := bundle.PayloadBlock().Data; !bytes.Equal(got, mustHex(t, wantCiphertext)) {
		t.Errorf("payload ciphertext =\n\t%x\nwant\n\t%x", got, mustHex(t, wantCiphertext))
	}

	// A.2.3.2, the abstract security block, which carries the wrapped key and
	// the authentication tag.
	wantASB := "8101020182028202018482014c5477656c766531323132313282020182035818" +
		wantWrapped + "8204008181820150efa4b5ac0108e3816c5606479801bc04"
	if !bytes.Equal(bcb.Data, mustHex(t, wantASB)) {
		t.Errorf("abstract security block =\n\t%x\nwant\n\t%x", bcb.Data, mustHex(t, wantASB))
	}

	// A.2.3.3, the complete BCB.
	gotBlock, err := bcb.Encode()
	if err != nil {
		t.Fatalf("encoding the BCB: %v", err)
	}
	if want := mustHex(t, "850c020100"+"5850"+wantASB); !bytes.Equal(gotBlock, want) {
		t.Errorf("BCB block =\n\t%x\nwant\n\t%x", gotBlock, want)
	}

	// A.2.4, the final bundle.
	wantBundle := "9f88070000820282010282028202018202820201820018281a000f4240" +
		"850c0201005850" + wantASB +
		"85010100005823" + wantCiphertext + "ff"
	if got := encodeBundle(t, bundle); !bytes.Equal(got, mustHex(t, wantBundle)) {
		t.Errorf("final bundle =\n\t%x\nwant\n\t%x", got, mustHex(t, wantBundle))
	}

	// Round trip: the receiver holds only the key encryption key.
	if err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{KEK: confidentiality.KEK}); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got := bundle.PayloadBlock().Data; !bytes.Equal(got, mustHex(t, rfcPayloadData)) {
		t.Errorf("decrypted payload = %x, want %x", got, mustHex(t, rfcPayloadData))
	}
	if bundle.Block(2) != nil {
		t.Error("the BCB is still in the bundle after Decrypt; clause 5.1.1 requires its removal")
	}
}

// RFC 9173 appendix A.4: a BIB over the payload, then a BCB encrypting both
// the payload and that BIB, with every scope flag set. This is the case where
// a security block is itself a security target.
func TestConfidentialityMatchesRFC9173Example4(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)

	integrity := bpsec.Integrity{
		BlockNumber: 3,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA384,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	}
	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BIB: %v", err)
	}

	confidentiality := bpsec.Confidentiality{
		BlockNumber: 2,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES256GCM,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "71776572747975696f7061736466676871776572747975696f70617364666768"),
		IV:          mustHex(t, "5477656c7665313231323132"),
	}

	// The BIB comes first in the target list, which is why its authentication
	// tag comes first in the security results.
	bcb, err := confidentiality.Add(bundle, 3, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	wantBIBCiphertext := "438ed6208eb1c1ffb94d952175167df0902902064a2983910c4fb2340790bf42" +
		"0a7d1921d5bf7c4721e02ab87a93ab1e0b75cf62e4948727c8b5dae46ed2af05439b88029191"
	if got := bib.Data; !bytes.Equal(got, mustHex(t, wantBIBCiphertext)) {
		t.Errorf("BIB ciphertext =\n\t%x\nwant\n\t%x", got, mustHex(t, wantBIBCiphertext))
	}

	wantPayloadCiphertext := "90eab6457593379298a8724e16e61f837488e127212b59ac91f8a86287b7d07630a122"
	if got := bundle.PayloadBlock().Data; !bytes.Equal(got, mustHex(t, wantPayloadCiphertext)) {
		t.Errorf("payload ciphertext =\n\t%x\nwant\n\t%x", got, mustHex(t, wantPayloadCiphertext))
	}

	// A.4.4.2, the abstract security block: two targets, two tags.
	wantASB := "820301020182028202018382014c5477656c7665313231323132820203820407" +
		"8281820150220ffc45c8a901999ecc60991dd78b2981820150d2c51cb2481792dae8b21d848cede99b"
	if !bytes.Equal(bcb.Data, mustHex(t, wantASB)) {
		t.Errorf("abstract security block =\n\t%x\nwant\n\t%x", bcb.Data, mustHex(t, wantASB))
	}

	// A.4.5, the final bundle, with both the BIB and the payload encrypted.
	wantBundle := "9f88070000820282010282028202018202820201820018281a000f4240" +
		"850b0300005846" + wantBIBCiphertext +
		"850c0201005849" + wantASB +
		"85010100005823" + wantPayloadCiphertext + "ff"
	if got := encodeBundle(t, bundle); !bytes.Equal(got, mustHex(t, wantBundle)) {
		t.Errorf("final bundle =\n\t%x\nwant\n\t%x", got, mustHex(t, wantBundle))
	}

	// Unwind it: decrypt, then check the integrity the BIB was protecting.
	if err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{Key: confidentiality.Key}); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key}); err != nil {
		t.Errorf("Verify after decrypting: %v", err)
	}
	if got := bundle.PayloadBlock().Data; !bytes.Equal(got, mustHex(t, rfcPayloadData)) {
		t.Errorf("decrypted payload = %x, want %x", got, mustHex(t, rfcPayloadData))
	}
}

// Anything the AAD covers must break decryption, and must break it as an
// authentication failure rather than producing wrong plaintext.
func TestDecryptDetectsChanges(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(b *bp.Bundle, bcb *bp.CanonicalBlock)
	}{
		{"ciphertext", func(b *bp.Bundle, _ *bp.CanonicalBlock) { b.PayloadBlock().Data[0] ^= 0x01 }},
		{"primary block", func(b *bp.Bundle, _ *bp.CanonicalBlock) { b.Primary.Lifetime++ }},
		{"target header", func(b *bp.Bundle, _ *bp.CanonicalBlock) {
			b.PayloadBlock().Flags |= bp.BlockFlagReportIfUnprocessable
		}},
		{"BCB header", func(_ *bp.Bundle, bcb *bp.CanonicalBlock) {
			bcb.Flags |= bp.BlockFlagReportIfUnprocessable
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle, bcb, key := encryptedBundle(t)
			tt.break_(bundle, bcb)

			before := append([]byte(nil), bundle.PayloadBlock().Data...)
			if err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{Key: key}); !errors.Is(err, bpsec.ErrDecryptionFailed) {
				t.Fatalf("Decrypt after changing the %s = %v, want ErrDecryptionFailed", tt.name, err)
			}
			// A failed decryption must leave the bundle as it found it: a node
			// that cannot decrypt a payload has to discard the bundle, and
			// that needs the bundle it received.
			if !bytes.Equal(bundle.PayloadBlock().Data, before) {
				t.Error("a failed Decrypt modified the bundle")
			}
			if bundle.Block(bcb.Number) == nil {
				t.Error("a failed Decrypt removed the BCB")
			}
		})
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	bundle, bcb, key := encryptedBundle(t)

	wrong := append([]byte(nil), key...)
	wrong[0] ^= 0x01

	if err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{Key: wrong}); !errors.Is(err, bpsec.ErrDecryptionFailed) {
		t.Errorf("Decrypt with a wrong key = %v, want ErrDecryptionFailed", err)
	}
}

// RFC 9172 clause 3.8 fixes two of a BCB's own flags, and RFC 9173 bounds the
// initialisation vector. All three are refused before anything is encrypted.
func TestConfidentialityRejectsBadConfiguration(t *testing.T) {
	key := mustHex(t, "71776572747975696f70617364666768")
	iv := mustHex(t, "5477656c7665313231323132")

	tests := []struct {
		name string
		edit func(c *bpsec.Confidentiality)
		want error
	}{
		{
			name: "payload target without the fragment replication flag",
			edit: func(c *bpsec.Confidentiality) { c.Flags = 0 },
			want: bpsec.ErrBCBFragmentFlag,
		},
		{
			name: "discard flag set",
			edit: func(c *bpsec.Confidentiality) {
				c.Flags |= bp.BlockFlagDiscardBlockIfUnprocessable
			},
			want: bpsec.ErrBCBRemovableFlag,
		},
		{
			name: "initialisation vector too short",
			edit: func(c *bpsec.Confidentiality) { c.IV = mustHex(t, "5477656c76653132")[:7] },
			want: bpsec.ErrIVLength,
		},
		{
			name: "initialisation vector too long",
			edit: func(c *bpsec.Confidentiality) {
				c.IV = mustHex(t, "5477656c76653132313231325477656c7665")
			},
			want: bpsec.ErrIVLength,
		},
		{
			name: "key length does not match the variant",
			edit: func(c *bpsec.Confidentiality) { c.Variant = bpsec.AES256GCM },
			want: bpsec.ErrKeyLength,
		},
		{
			name: "unknown AES variant",
			edit: func(c *bpsec.Confidentiality) { c.Variant = 2 },
			want: bpsec.ErrUnknownAESVariant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := decodeBundle(t, rfcOriginalBundle)
			c := bpsec.Confidentiality{
				BlockNumber: 2,
				Flags:       bp.BlockFlagReplicateInEveryFragment,
				Source:      bp.IPN(2, 1),
				Variant:     bpsec.AES128GCM,
				Scope:       bpsec.ScopeAll,
				Key:         key,
				IV:          iv,
			}
			tt.edit(&c)

			_, err := c.Add(bundle, bp.PayloadBlockNumber)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Add = %v, want %v", err, tt.want)
			}
			// Nothing was encrypted, so the payload is still readable.
			if !bytes.Equal(bundle.PayloadBlock().Data, mustHex(t, rfcPayloadData)) {
				t.Error("a rejected Add still encrypted the payload")
			}
		})
	}
}

// RFC 9172 clause 3.8 forbids a BCB from targeting the primary block.
func TestConfidentialityRefusesThePrimaryBlock(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	c := bpsec.Confidentiality{
		BlockNumber: 2,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES128GCM,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "71776572747975696f70617364666768"),
		IV:          mustHex(t, "5477656c7665313231323132"),
	}

	if _, err := c.Add(bundle, bp.PrimaryBlockNumber); !errors.Is(err, bpsec.ErrConfidentialityTargetsPrimary) {
		t.Errorf("Add targeting the primary block = %v, want ErrConfidentialityTargetsPrimary", err)
	}
}

// encryptedBundle returns the RFC 9173 bundle with its payload encrypted, plus
// the BCB and the key.
func encryptedBundle(t *testing.T) (*bp.Bundle, *bp.CanonicalBlock, []byte) {
	t.Helper()

	bundle := decodeBundle(t, rfcOriginalBundle)
	key := mustHex(t, "71776572747975696f70617364666768")

	c := bpsec.Confidentiality{
		BlockNumber: 2,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES128GCM,
		Scope:       bpsec.ScopeAll,
		Key:         key,
		IV:          mustHex(t, "5477656c7665313231323132"),
	}
	bcb, err := c.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return bundle, bcb, key
}
