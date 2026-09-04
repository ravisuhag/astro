package bpsec_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// RFC 9173 appendix A.3: two security blocks from two different nodes. A
// waypoint signs the primary block and a Bundle Age block, while the source
// encrypts the payload.
//
// This is the only worked example whose BIB targets the primary block, which
// is the case that frames the target differently from every other.
func TestSecurityBlocksFromMultipleSources(t *testing.T) {
	// A.3.1.4. The same bundle as the other examples plus a Bundle Age block
	// carrying 300 milliseconds, block number 2.
	const originalBundle = "9f88070000820282010282028202018202820201820018281a000f4240" +
		"85070200004319012c" +
		"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff"

	bundle := decodeBundle(t, originalBundle)

	// A.3.3: the BIB, added by ipn:3.0 over the primary block and the Bundle
	// Age block. Targets are listed primary first, and the results follow.
	integrity := bpsec.Integrity{
		BlockNumber: 3,
		Source:      bp.IPN(3, 0),
		Variant:     bpsec.HMACSHA256,
		Scope:       bpsec.ScopeNone,
		Key:         mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	}
	bib, err := integrity.Add(bundle, bp.PrimaryBlockNumber, 2)
	if err != nil {
		t.Fatalf("adding the BIB: %v", err)
	}

	// A.3.3.1 prints the IPPT for each target. The primary block one is the
	// case worth pinning: 00 581c 8807... wraps the whole primary block in a
	// byte string head that appears nowhere in the bundle itself.
	wantPrimaryIPPT := "00581c88070000820282010282028202018202820201820018281a000f4240"
	gotPrimaryIPPT, err := bpsec.IPPT(bundle, bpsec.ScopeNone, bp.PrimaryBlockNumber, bib)
	if err != nil {
		t.Fatalf("IPPT for the primary block: %v", err)
	}
	if !bytes.Equal(gotPrimaryIPPT, mustHex(t, wantPrimaryIPPT)) {
		t.Errorf("primary block IPPT =\n\t%x\nwant\n\t%x", gotPrimaryIPPT, mustHex(t, wantPrimaryIPPT))
	}

	wantAgeIPPT := "004319012c"
	gotAgeIPPT, err := bpsec.IPPT(bundle, bpsec.ScopeNone, 2, bib)
	if err != nil {
		t.Fatalf("IPPT for the Bundle Age block: %v", err)
	}
	if !bytes.Equal(gotAgeIPPT, mustHex(t, wantAgeIPPT)) {
		t.Errorf("Bundle Age block IPPT =\n\t%x\nwant\n\t%x", gotAgeIPPT, mustHex(t, wantAgeIPPT))
	}

	// A.3.3.2, the abstract security block: two targets, two results.
	wantBIBASB := "8200020101820282030082820105820300828182015820cac6ce8e4c5dae57988b" +
		"757e49a6dd1431dc04763541b2845098265bc817241b81820158203ed614c0d97f49" +
		"b3633627779aa18a338d212bf3c92b97759d9739cd50725596"
	if !bytes.Equal(bib.Data, mustHex(t, wantBIBASB)) {
		t.Errorf("BIB abstract security block =\n\t%x\nwant\n\t%x", bib.Data, mustHex(t, wantBIBASB))
	}

	// A.3.4: the BCB over the payload, added by ipn:2.1 and numbered 4.
	confidentiality := bpsec.Confidentiality{
		BlockNumber: 4,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES128GCM,
		Scope:       bpsec.ScopeNone,
		Key:         mustHex(t, "71776572747975696f70617364666768"),
		IV:          mustHex(t, "5477656c7665313231323132"),
	}
	bcb, err := confidentiality.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	wantBCBASB := "8101020182028202018382014c5477656c76653132313231328202018204008181" +
		"820150efa4b5ac0108e3816c5606479801bc04"
	if !bytes.Equal(bcb.Data, mustHex(t, wantBCBASB)) {
		t.Errorf("BCB abstract security block =\n\t%x\nwant\n\t%x", bcb.Data, mustHex(t, wantBCBASB))
	}

	// A.3.5, the final bundle: primary, BIB, BCB, Bundle Age, payload.
	wantBundle := "9f88070000820282010282028202018202820201820018281a000f4240" +
		"850b030000585c" + wantBIBASB +
		"850c040100" + "5834" + wantBCBASB +
		"85070200004319012c" +
		"850101000058233a09c1e63fe23a7f66a59c7303837241e070b02619fc59c5214a22f08cd70795e73e9a" +
		"ff"
	if got := encodeBundle(t, bundle); !bytes.Equal(got, mustHex(t, wantBundle)) {
		t.Errorf("final bundle =\n\t%x\nwant\n\t%x", got, mustHex(t, wantBundle))
	}

	// The BIB covers the primary block and the Bundle Age block, neither of
	// which the BCB touched, so it still verifies with the payload encrypted.
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key}); err != nil {
		t.Errorf("Verify with the payload still encrypted: %v", err)
	}

	if err := bpsec.Decrypt(bundle, bcb, bpsec.Keys{Key: confidentiality.Key}); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got := bundle.PayloadBlock().Data; !bytes.Equal(got, mustHex(t, rfcPayloadData)) {
		t.Errorf("decrypted payload = %x, want %x", got, mustHex(t, rfcPayloadData))
	}
	if err := bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key}); err != nil {
		t.Errorf("Verify after decrypting: %v", err)
	}
}
