package bpsec_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// The block interaction rules of RFC 9172 clauses 3.2, 3.7, 3.8 and 3.9 are
// about which security operations may coexist in a bundle. They produce no
// octets, so no vector expresses them; these are what checks them.

func newIntegrity(t *testing.T, number uint64) bpsec.Integrity {
	t.Helper()
	return bpsec.Integrity{
		BlockNumber: number,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.HMACSHA384,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	}
}

func newConfidentiality(t *testing.T, number uint64) bpsec.Confidentiality {
	t.Helper()
	return bpsec.Confidentiality{
		BlockNumber: number,
		Flags:       bp.BlockFlagReplicateInEveryFragment,
		Source:      bp.IPN(2, 1),
		Variant:     bpsec.AES128GCM,
		Scope:       bpsec.ScopeAll,
		Key:         mustHex(t, "71776572747975696f70617364666768"),
		IV:          mustHex(t, "5477656c7665313231323132"),
	}
}

// Clause 3.2: the same security service must not be applied twice to one
// target.
func TestSecurityOperationsAreUnique(t *testing.T) {
	t.Run("two BIBs on one target", func(t *testing.T) {
		bundle := decodeBundle(t, rfcOriginalBundle)
		if _, err := newIntegrity(t, 2).Add(bundle, bp.PayloadBlockNumber); err != nil {
			t.Fatalf("first BIB: %v", err)
		}
		_, err := newIntegrity(t, 3).Add(bundle, bp.PayloadBlockNumber)
		if !errors.Is(err, bpsec.ErrDuplicateSecurityOperation) {
			t.Errorf("second BIB = %v, want ErrDuplicateSecurityOperation", err)
		}
	})

	t.Run("two BCBs on one target", func(t *testing.T) {
		bundle := decodeBundle(t, rfcOriginalBundle)
		if _, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber); err != nil {
			t.Fatalf("first BCB: %v", err)
		}
		_, err := newConfidentiality(t, 3).Add(bundle, bp.PayloadBlockNumber)
		if !errors.Is(err, bpsec.ErrDuplicateSecurityOperation) {
			t.Errorf("second BCB = %v, want ErrDuplicateSecurityOperation", err)
		}
	})
}

// Clause 3.9: for a given target, integrity comes before confidentiality.
func TestIntegrityMustComeBeforeConfidentiality(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	if _, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber); err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	_, err := newIntegrity(t, 3).Add(bundle, bp.PayloadBlockNumber)
	if !errors.Is(err, bpsec.ErrIntegrityAfterConfidentiality) {
		t.Errorf("BIB after BCB = %v, want ErrIntegrityAfterConfidentiality", err)
	}
}

// Clause 3.9: a BIB must not be checked once its target has been encrypted,
// because the octets in the block are no longer the ones that were hashed.
func TestVerifyRefusesAnEncryptedTarget(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)

	integrity := newIntegrity(t, 3)
	bib, err := integrity.Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BIB: %v", err)
	}
	if _, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber); err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	err = bpsec.Verify(bundle, bib, bpsec.Keys{Key: integrity.Key})
	if !errors.Is(err, bpsec.ErrIntegrityAfterConfidentiality) {
		t.Errorf("Verify over an encrypted target = %v, want ErrIntegrityAfterConfidentiality", err)
	}
}

// Clause 3.7: a BIB must not target a security block.
func TestIntegrityRefusesASecurityBlockTarget(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	bcb, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	_, err = newIntegrity(t, 3).Add(bundle, bcb.Number)
	if !errors.Is(err, bpsec.ErrIntegrityTargetsSecurityBlock) {
		t.Errorf("BIB over a BCB = %v, want ErrIntegrityTargetsSecurityBlock", err)
	}
}

// Clause 3.8: a BCB must not target another BCB, and may target a BIB only
// when the two share a security target.
func TestConfidentialityTargetRules(t *testing.T) {
	t.Run("a BCB over another BCB", func(t *testing.T) {
		bundle := decodeBundle(t, rfcOriginalBundle)
		first, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber)
		if err != nil {
			t.Fatalf("first BCB: %v", err)
		}
		_, err = newConfidentiality(t, 3).Add(bundle, first.Number)
		if !errors.Is(err, bpsec.ErrConfidentialityTargetsBCB) {
			t.Errorf("BCB over a BCB = %v, want ErrConfidentialityTargetsBCB", err)
		}
	})

	t.Run("a BCB over an unrelated BIB", func(t *testing.T) {
		// The bundle carries a Bundle Age block so the BIB has something to
		// protect that the BCB is not encrypting.
		const aged = "9f88070000820282010282028202018202820201820018281a000f4240" +
			"85070200004319012c" +
			"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff"

		bundle := decodeBundle(t, aged)
		bib, err := newIntegrity(t, 3).Add(bundle, 2)
		if err != nil {
			t.Fatalf("adding the BIB: %v", err)
		}

		// The BCB encrypts the payload and the BIB, but the BIB protects only
		// the Bundle Age block, so they share nothing.
		_, err = newConfidentiality(t, 4).Add(bundle, bp.PayloadBlockNumber, bib.Number)
		if !errors.Is(err, bpsec.ErrBCBTargetsUnsharedBIB) {
			t.Errorf("BCB over an unrelated BIB = %v, want ErrBCBTargetsUnsharedBIB", err)
		}
	})
}

// Block numbering comes from RFC 9171 clause 4.1, and a security block has to
// respect it like any other extension block.
func TestSecurityBlockNumbering(t *testing.T) {
	tests := []struct {
		name   string
		number uint64
		want   error
	}{
		{"the primary block's number", 0, bpsec.ErrReservedBlockNumber},
		{"the payload block's number", 1, bpsec.ErrReservedBlockNumber},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := decodeBundle(t, rfcOriginalBundle)
			_, err := newIntegrity(t, tt.number).Add(bundle, bp.PayloadBlockNumber)
			if !errors.Is(err, tt.want) {
				t.Errorf("Add = %v, want %v", err, tt.want)
			}
		})
	}

	t.Run("a number already in use", func(t *testing.T) {
		bundle := decodeBundle(t, rfcOriginalBundle)
		if _, err := newIntegrity(t, 2).Add(bundle, bp.PayloadBlockNumber); err != nil {
			t.Fatalf("first BIB: %v", err)
		}
		_, err := newConfidentiality(t, 2).Add(bundle, bp.PayloadBlockNumber)
		if !errors.Is(err, bpsec.ErrBlockNumberInUse) {
			t.Errorf("Add = %v, want ErrBlockNumberInUse", err)
		}
	})
}

// Clause 3.6: every target must name a block the bundle actually has.
func TestTargetMustExist(t *testing.T) {
	bundle := decodeBundle(t, rfcOriginalBundle)
	_, err := newIntegrity(t, 2).Add(bundle, 42)
	if !errors.Is(err, bpsec.ErrTargetNotInBundle) {
		t.Errorf("Add targeting a block that is not there = %v, want ErrTargetNotInBundle", err)
	}
}

// A BIB whose target has been encrypted is skipped when the package looks for
// existing integrity operations, rather than being treated as malformed. This
// is the arrangement RFC 9173 appendix A.4 prints.
func TestAnEncryptedBIBDoesNotBlockFurtherWork(t *testing.T) {
	const aged = "9f88070000820282010282028202018202820201820018281a000f4240" +
		"85070200004319012c" +
		"85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164ff"

	bundle := decodeBundle(t, aged)

	// A BIB over the payload, then a BCB encrypting both it and the payload.
	bib, err := newIntegrity(t, 3).Add(bundle, bp.PayloadBlockNumber)
	if err != nil {
		t.Fatalf("adding the BIB: %v", err)
	}
	if _, err := newConfidentiality(t, 4).Add(bundle, bp.PayloadBlockNumber, bib.Number); err != nil {
		t.Fatalf("adding the BCB: %v", err)
	}

	// The BIB is ciphertext now. Adding a second BIB over the Bundle Age block
	// must still work: nothing can read the encrypted BIB, but nothing needs
	// to, because clause 3.9 forbids checking it anyway.
	if _, err := newIntegrity(t, 5).Add(bundle, 2); err != nil {
		t.Errorf("adding a BIB beside an encrypted one: %v", err)
	}
}
