package bpsec_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// RFC 9173 appendix A.1: a BIB over the payload block, with no additional
// scope and a 512-bit keyed hash.
func TestIntegrityMatchesRFC9173Example1(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)

	integrity := bpsec.Integrity{
		BlockNumber: 2,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA512,
		Scope:       bpsec.ScopeNone,
		Key:         mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	}

	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A.1.3.2, the abstract security block.
	wantASB := "810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808" +
		"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1"
	if want := mustHex(t, wantASB); !bytes.Equal(bib.Data, want) {
		t.Errorf("abstract security block =\n\t%x\nwant\n\t%x", bib.Data, want)
	}

	// A.1.3.3, the complete BIB.
	wantBlock := "850b0200005856" + wantASB
	gotBlock, err := bib.Encode()
	if err != nil {
		t.Fatalf("encoding the BIB: %v", err)
	}
	if want := mustHex(t, wantBlock); !bytes.Equal(gotBlock, want) {
		t.Errorf("BIB block =\n\t%x\nwant\n\t%x", gotBlock, want)
	}

	// A.1.4, the final bundle. The BIB sits between the primary block and the
	// payload.
	wantBundle := "9f88070000820282010282028202018202820201820018281a000f4240850b0200005856" + wantASB +
		"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff"
	if got := encodeBundle(t, bundle); !bytes.Equal(got, mustHex(t, wantBundle)) {
		t.Errorf("final bundle =\n\t%x\nwant\n\t%x", got, mustHex(t, wantBundle))
	}

	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key}); err != nil {
		t.Errorf("Verify on the bundle just signed: %v", err)
	}
}

// RFC 9173 appendix A.4: a BIB over the payload with every scope flag set, and
// a 384-bit keyed hash. This is the case that exercises the whole IPPT.
func TestIntegrityMatchesRFC9173Example4(t *testing.T) {
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
		t.Fatalf("Add: %v", err)
	}

	wantASB := "81010101820282020182820106820307818182015830f75fe4c37f76f046165855bd5ff72fbf" +
		"d4e3a64b4695c40e2b787da005ae819f0a2e30a2e8b325527de8aefb52e73d71"
	if want := mustHex(t, wantASB); !bytes.Equal(bib.Data, want) {
		t.Errorf("abstract security block =\n\t%x\nwant\n\t%x", bib.Data, want)
	}

	gotBlock, err := bib.Encode()
	if err != nil {
		t.Fatalf("encoding the BIB: %v", err)
	}
	// A.4.3.3, the complete BIB.
	if want := mustHex(t, "850b0300005846"+wantASB); !bytes.Equal(gotBlock, want) {
		t.Errorf("BIB block =\n\t%x\nwant\n\t%x", gotBlock, want)
	}

	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key}); err != nil {
		t.Errorf("Verify on the bundle just signed: %v", err)
	}
}

// Altering anything the scope covers must break verification. With ScopeAll
// that includes the primary block and both block headers, none of which is
// inside the target's data.
func TestVerifyDetectsChanges(t *testing.T) {
	key := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b")

	tests := []struct {
		name   string
		break_ func(b *bp.Bundle, bib *bp.CanonicalBlock)
	}{
		{
			name:   "payload contents",
			break_: func(b *bp.Bundle, _ *bp.CanonicalBlock) { b.PayloadBlock().Data[0] ^= 0x01 },
		},
		{
			name:   "primary block lifetime",
			break_: func(b *bp.Bundle, _ *bp.CanonicalBlock) { b.Primary.Lifetime++ },
		},
		{
			name:   "target block flags",
			break_: func(b *bp.Bundle, _ *bp.CanonicalBlock) { b.PayloadBlock().Flags |= bp.BlockFlagReportIfUnprocessable },
		},
		{
			name:   "BIB block flags",
			break_: func(_ *bp.Bundle, bib *bp.CanonicalBlock) { bib.Flags |= bp.BlockFlagReportIfUnprocessable },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := decodeBundle(t, rfcOriginalBundle)
			integrity := bpsec.Integrity{
				BlockNumber: 2,
				Source:      bp.IPN(2, 1),
				Variant:     bpsec.HMACSHA384,
				Scope:       bpsec.ScopeAll,
				Key:         key,
			}
			bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key}); err != nil {
				t.Fatalf("Verify before the change: %v", err)
			}

			tt.break_(bundle, bib)

			if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key}); !errors.Is(err, bpsec.ErrIntegrityCheckFailed) {
				t.Errorf("Verify after changing the %s = %v, want ErrIntegrityCheckFailed", tt.name, err)
			}
		})
	}
}

// With no additional scope, the same change to a block header must NOT break
// verification: the caller asked for the target's contents only.
func TestVerifyIgnoresWhatScopeExcludes(t *testing.T) {
	key := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b")
	bundle := decodeBundle(t, rfcOriginalBundle)

	integrity := bpsec.Integrity{
		BlockNumber: 2,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA512,
		Scope:       bpsec.ScopeNone,
		Key:         key,
	}
	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	bundle.Primary.Lifetime++
	bundle.PayloadBlock().Flags |= bp.BlockFlagReportIfUnprocessable

	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key}); err != nil {
		t.Errorf("Verify with ScopeNone after changing headers = %v, want success", err)
	}
}

// A wrong key must fail, and must fail as an integrity error rather than
// anything vaguer.
func TestVerifyRejectsWrongKey(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	integrity := bpsec.Integrity{
		BlockNumber: 2,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA384,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	}
	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	wrong := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2c")
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: wrong}); !errors.Is(err, bpsec.ErrIntegrityCheckFailed) {
		t.Errorf("Verify with a wrong key = %v, want ErrIntegrityCheckFailed", err)
	}
}

// A BIB may carry its own key, wrapped under a key encryption key.
func TestIntegrityWithWrappedKey(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	key := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b")
	kek := mustHex(t, "6162636465666768696a6b6c6d6e6f70")

	integrity := bpsec.Integrity{
		BlockNumber: 2,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA384,
		Scope:       bpsec.ScopeAll,
		Key:         key,
		KEK:         kek,
	}
	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// The verifier is given only the key encryption key, never the HMAC key.
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{KEK: kek}); err != nil {
		t.Errorf("Verify with the KEK alone: %v", err)
	}
	// Without it there is nothing to unwrap with.
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{}); !errors.Is(err, bpsec.ErrNoKey) {
		t.Errorf("Verify with no keys = %v, want ErrNoKey", err)
	}
}

// B6 regression: a primary block whose source is spelled in the ipn
// three-element form of RFC 9758 clause 6.1.2, rather than the two-element
// form this library prefers, must still verify. Before pkg/bp remembered
// which spelling a bundle arrived with, Verify recomputed the IPPT over a
// primary block Encode re-derived in the preferred form -- different octets
// than the ones the BIB's keyed hash was actually taken over -- and the check
// failed as if the bundle had been tampered with.
//
// The bundle: primary block [7,0,0, destination ipn:1.2, source ipn:2.1
// spelled with an explicit (zero) allocator in the three-element form,
// report-to ipn:2.1, timestamp [0,40], lifetime 1000000], a BIB (block 2,
// HMAC-SHA-384, every scope flag set, key 1a2b...) over the payload, and the
// payload of RFC 9173 appendix A.1.1.2. Generated once with this package
// after the fix and pinned here; it must decode, verify, and round-trip its
// primary block byte for byte.
func TestIntegrityAcceptsIPNThreeElementSource(t *testing.T) {
	const primaryHex = "8807000082028201028202830002018202820201820018281a000f4240"
	bundle := decodeBundle(t, "9f"+primaryHex+
		"850b0200005846810101018202820201828201068203078181820158308419fb10571c387747dbaa5c40e573b852"+
		"911e8c547b8da2add5488939d8bab68dcf973678d2c227eac99f53027c400d"+
		"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff")

	if got := bundle.Primary.Source.String(); got != "ipn:2.1" {
		t.Fatalf("source = %s, want ipn:2.1", got)
	}

	bib := bundle.Block(2)
	if bib == nil {
		t.Fatal("no BIB block at number 2")
	}
	key := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b")
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key}); err != nil {
		t.Errorf("Verify: %v", err)
	}

	gotPrimary, err := bundle.Primary.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := mustHex(t, primaryHex); !bytes.Equal(gotPrimary, want) {
		t.Errorf("primary block round trip =\n\t%x\nwant\n\t%x", gotPrimary, want)
	}
}

// B6 regression, the dtn side: a primary block whose source is dtn:none
// spelled as the text string "none" rather than the integer 0 must still
// verify, for the same reason as TestIntegrityAcceptsIPNThreeElementSource.
//
// The bundle: primary block [7, must-not-fragment, 0, destination ipn:1.2,
// source dtn:none spelled as text, report-to ipn:2.1, timestamp [0,40],
// lifetime 1000000] -- must-not-fragment is set because clause 4.2.5.1.1
// requires it of an anonymous bundle -- a BIB over the payload with the same
// parameters as above, and the same RFC 9173 payload. Generated once with
// this package after the fix and pinned here.
func TestIntegrityAcceptsDTNNoneAsTextSource(t *testing.T) {
	const primaryHex = "8807040082028201028201646e6f6e658202820201820018281a000f4240"
	bundle := decodeBundle(t, "9f"+primaryHex+
		"850b020000584681010101820282020182820106820307818182015830016e3f708af2170db83bdcf69f2cce994"+
		"d9fb6a9e55ea023ed40e6058a5cd31d5968329e84eea0b0e541df2fea73f7e9"+
		"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff")

	if got := bundle.Primary.Source.String(); got != "dtn:none" {
		t.Fatalf("source = %s, want dtn:none", got)
	}

	bib := bundle.Block(2)
	if bib == nil {
		t.Fatal("no BIB block at number 2")
	}
	key := mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b")
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: key}); err != nil {
		t.Errorf("Verify: %v", err)
	}

	gotPrimary, err := bundle.Primary.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := mustHex(t, primaryHex); !bytes.Equal(gotPrimary, want) {
		t.Errorf("primary block round trip =\n\t%x\nwant\n\t%x", gotPrimary, want)
	}
}

// encodeBundle writes a bundle block by block.
//
// Bundle.Encode is not usable here. The example bundles in RFC 9173 have a
// creation time of zero and no Bundle Age block, which RFC 9171 clause 4.4.2
// forbids a node to create; pkg/bp enforces that on Encode and deliberately
// does not on Decode. Assembling the blocks directly reproduces the RFC's
// octets without asking pkg/bp to bless a bundle the RFC itself calls an
// example rather than a template.
func encodeBundle(t *testing.T, b *bp.Bundle) []byte {
	t.Helper()

	out := []byte{0x9f} // the indefinite-length array head of RFC 9171 clause 4.1
	primary, err := b.Primary.Encode()
	if err != nil {
		t.Fatalf("encoding the primary block: %v", err)
	}
	out = append(out, primary...)

	for _, blk := range b.Blocks {
		encoded, err := blk.Encode()
		if err != nil {
			t.Fatalf("encoding block %d: %v", blk.Number, err)
		}
		out = append(out, encoded...)
	}
	return append(out, 0xff) // the break stop code
}
