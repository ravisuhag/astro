package bpsec_test

import (
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/bpsec"
)

// The property is that arbitrary octets never panic and never drive an
// allocation from an attacker-controlled length field. An Abstract Security
// Block has three such fields — the target count, the parameter count and the
// result count — and each is bounded by how many octets are actually left.
func FuzzDecodeASB(f *testing.F) {
	seeds := []string{
		// A.1.3.2: one target, two parameters, one result.
		"810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808" +
			"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1",
		// A.3.3.2: two targets, two results.
		"8200020101820282030082820105820300828182015820cac6ce8e4c5dae57988b" +
			"757e49a6dd1431dc04763541b2845098265bc817241b81820158203ed614c0d97f49" +
			"b3633627779aa18a338d212bf3c92b97759d9739cd50725596",
		// A.2.3.2: a BCB with a wrapped key.
		"8101020182028202018482014c5477656c766531323132313282020182035818" +
			"69c411276fecddc4780df42c8a2af89296fabf34d7fae7008204008181820150efa4b5ac0108e3816c5606479801bc04",
		// Heads claiming far more items than the input holds.
		"9bffffffffffffffff",
		"81011bffffffffffffffff",
	}
	for _, seed := range seeds {
		b, err := hex.DecodeString(seed)
		if err != nil {
			f.Fatalf("bad seed hex %q: %v", seed, err)
		}
		f.Add(b)
	}
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		asb, err := bpsec.DecodeASB(data)
		if err != nil {
			return
		}
		// Anything that decodes must re-encode, and must survive being
		// summarised for a dump.
		if _, err := asb.Encode(); err != nil {
			t.Fatalf("an ASB that decoded failed to encode: %v", err)
		}
		_ = asb.Humanize()
	})
}

// Verify and Decrypt both walk a security block that arrived from the network,
// so they take the same arbitrary octets the ASB decoder does.
func FuzzProcessSecurityBlock(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add(
		mustHexBytes(f, "810101018202820201828201078203008181820158403bdc69b3a34a2b5d3a8554368bd1e808"+
			"f606219d2a10a846eae3886ae4ecc83c4ee550fdfb1cc636b904e2f1a73e303dcd4b6ccece003e95e8164dcc89a156e1"),
		mustHexBytes(f, "1a2b1a2b1a2b1a2b1a2b1a2b1a2b1a2b"),
	)
	f.Add(
		mustHexBytes(f, "8101020182028202018382014c5477656c76653132313231328202018204008181"+
			"820150efa4b5ac0108e3816c5606479801bc04"),
		mustHexBytes(f, "71776572747975696f70617364666768"),
	)

	f.Fuzz(func(t *testing.T, blockData, key []byte) {
		bundle, err := bp.Decode(mustHexBytes(t, rfcOriginalBundle))
		if err != nil {
			t.Fatalf("decoding the fixed bundle: %v", err)
		}

		bib := &bp.CanonicalBlock{Type: bpsec.BlockTypeIntegrity, Number: 2, Data: blockData}
		bcb := &bp.CanonicalBlock{
			Type:   bpsec.BlockTypeConfidentiality,
			Number: 3,
			Flags:  bp.BlockFlagReplicateInEveryFragment,
			Data:   blockData,
		}
		bundle.Blocks = append([]*bp.CanonicalBlock{bib, bcb}, bundle.Blocks...)

		// Errors are the expected outcome. A panic is not.
		_ = bpsec.Verify(bundle, bib, bpsec.Keys{Key: key, KEK: key})
		_ = bpsec.Decrypt(bundle, bcb, bpsec.Keys{Key: key, KEK: key})
		_ = bpsec.Humanize(bib)
		_ = bpsec.Humanize(bcb)
	})
}

// mustHexBytes decodes hex for a fuzz seed or a fuzz body, either of which may
// be a *testing.F or a *testing.T.
func mustHexBytes(tb testing.TB, s string) []byte {
	tb.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		tb.Fatalf("bad test hex %q: %v", s, err)
	}
	return b
}
