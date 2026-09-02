package bp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
)

func TestFragmentAndReassemble(t *testing.T) {
	payload := make([]byte, 1000)
	for i := range payload {
		payload[i] = byte(i * 7)
	}

	original, err := bp.NewBundle(testPrimary(), payload)
	if err != nil {
		t.Fatal(err)
	}

	fragments, err := original.Fragment(256)
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 4 {
		t.Fatalf("got %d fragments, want 4", len(fragments))
	}

	for i, f := range fragments {
		if !f.Primary.IsFragment() {
			t.Errorf("fragment %d does not have the fragment flag", i)
		}
		if f.Primary.TotalADULength != uint64(len(payload)) {
			t.Errorf("fragment %d total ADU length = %d, want %d",
				i, f.Primary.TotalADULength, len(payload))
		}
	}

	rebuilt, err := bp.Reassemble(fragments)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Primary.IsFragment() {
		t.Error("the reassembled bundle still has the fragment flag")
	}

	got, err := rebuilt.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("the reassembled payload differs from the original")
	}
}

func TestReassembleOutOfOrder(t *testing.T) {
	payload := make([]byte, 500)
	for i := range payload {
		payload[i] = byte(i)
	}
	original, err := bp.NewBundle(testPrimary(), payload)
	if err != nil {
		t.Fatal(err)
	}
	fragments, err := original.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}

	// Shuffle: last, first, middle.
	shuffled := []*bp.Bundle{
		fragments[4], fragments[0], fragments[2], fragments[1], fragments[3],
	}
	rebuilt, err := bp.Reassemble(shuffled)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rebuilt.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("out-of-order reassembly produced the wrong payload")
	}
}

func TestReassembleDetectsGap(t *testing.T) {
	payload := make([]byte, 500)
	original, err := bp.NewBundle(testPrimary(), payload)
	if err != nil {
		t.Fatal(err)
	}
	fragments, err := original.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}

	// Drop the middle fragment.
	incomplete := []*bp.Bundle{fragments[0], fragments[1], fragments[3], fragments[4]}
	if _, err := bp.Reassemble(incomplete); !errors.Is(err, bp.ErrIncompleteFragments) {
		t.Errorf("error = %v, want ErrIncompleteFragments", err)
	}
}

func TestReassembleRejectsMismatchedFragments(t *testing.T) {
	payload := make([]byte, 200)

	first, err := bp.NewBundle(testPrimary(), payload)
	if err != nil {
		t.Fatal(err)
	}
	firstFrags, err := first.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}

	other := testPrimary()
	other.CreationTimestamp.SequenceNumber = 999 // a different bundle
	second, err := bp.NewBundle(other, payload)
	if err != nil {
		t.Fatal(err)
	}
	secondFrags, err := second.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}

	mixed := []*bp.Bundle{firstFrags[0], secondFrags[1]}
	if _, err := bp.Reassemble(mixed); !errors.Is(err, bp.ErrMismatchedFragments) {
		t.Errorf("error = %v, want ErrMismatchedFragments", err)
	}
}

func TestFragmentRespectsNoFragmentFlag(t *testing.T) {
	// Clause 4.2: the flag forbids it.
	primary := testPrimary()
	primary.Flags |= bp.FlagNoFragment

	b, err := bp.NewBundle(primary, make([]byte, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Fragment(100); !errors.Is(err, bp.ErrCannotFragment) {
		t.Errorf("error = %v, want ErrCannotFragment", err)
	}
}

func TestFragmentReplicatesFlaggedBlocks(t *testing.T) {
	// Clause 5.8: only blocks flagged for replication go in every fragment.
	replicated := &bp.CanonicalBlock{
		Type: 200, Flags: bp.BlockReplicate, Data: []byte("everywhere"),
	}
	once := &bp.CanonicalBlock{
		Type: 201, Data: []byte("first only"),
	}

	b, err := bp.NewBundle(testPrimary(), make([]byte, 300),
		bp.WithBlock(replicated), bp.WithBlock(once))
	if err != nil {
		t.Fatal(err)
	}

	fragments, err := b.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 3 {
		t.Fatalf("got %d fragments, want 3", len(fragments))
	}

	for i, f := range fragments {
		hasReplicated, hasOnce := false, false
		for _, block := range f.Blocks {
			switch block.Type {
			case 200:
				hasReplicated = true
			case 201:
				hasOnce = true
			}
		}
		if !hasReplicated {
			t.Errorf("fragment %d is missing the replicated block", i)
		}
		if i == 0 && !hasOnce {
			t.Error("the first fragment is missing the non-replicated block")
		}
		if i > 0 && hasOnce {
			t.Errorf("fragment %d carries a block that should ride only on the first", i)
		}
	}
}

func TestFragmentPostPayloadBlocksRideTheLastFragment(t *testing.T) {
	// Clause 5.8: blocks preceding the payload go with the first fragment; blocks
	// following it go with the last, in place, not all with the first.
	payload := make([]byte, 300)
	b := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: 200, Data: []byte("before")},
			{Type: bp.BlockTypePayload, Data: payload},
			{Type: 201, Flags: bp.BlockLast, Data: []byte("after")},
		},
	}
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}

	fragments, err := b.Fragment(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 3 {
		t.Fatalf("got %d fragments, want 3", len(fragments))
	}

	find := func(f *bp.Bundle, bt bp.BlockType) int {
		for i, block := range f.Blocks {
			if block.Type == bt {
				return i
			}
		}
		return -1
	}

	for i, f := range fragments {
		first, last := i == 0, i == len(fragments)-1
		pre, pay, post := find(f, 200), find(f, bp.BlockTypePayload), find(f, 201)

		if first && (pre < 0 || pre > pay) {
			t.Error("the first fragment must carry the pre-payload block, before the payload")
		}
		if !first && pre >= 0 {
			t.Errorf("fragment %d carries the pre-payload block; only the first should", i)
		}
		if last && (post < 0 || post < pay) {
			t.Error("the last fragment must carry the post-payload block, after the payload")
		}
		if !last && post >= 0 {
			t.Errorf("fragment %d carries the post-payload block; only the last should", i)
		}
		if !f.Blocks[len(f.Blocks)-1].IsLast() {
			t.Errorf("fragment %d's final block lacks the last-block flag", i)
		}
	}

	// Reassembly puts them back where they were.
	rebuilt, err := bp.Reassemble(fragments)
	if err != nil {
		t.Fatal(err)
	}
	pre, pay, post := find(rebuilt, 200), find(rebuilt, bp.BlockTypePayload), find(rebuilt, 201)
	if pre != 0 || pay != 1 || post != 2 {
		t.Errorf("rebuilt block order = %d,%d,%d; want the original 0,1,2", pre, pay, post)
	}
}

func TestFragmentSmallBundleIsUnchanged(t *testing.T) {
	b, err := bp.NewBundle(testPrimary(), []byte("small"))
	if err != nil {
		t.Fatal(err)
	}
	fragments, err := b.Fragment(1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 1 {
		t.Fatalf("got %d fragments, want 1", len(fragments))
	}
	if fragments[0].Primary.IsFragment() {
		t.Error("a bundle that needed no fragmenting was marked as a fragment")
	}
}

func TestECOSRoundTrip(t *testing.T) {
	// CCSDS 734.2-B-1 annex C.
	tests := []struct {
		name string
		ecos bp.ECOS
	}{
		{"plain ordinal", bp.ECOS{Ordinal: 42}},
		{"critical", bp.ECOS{Flags: bp.ECOSCritical, Ordinal: 100}},
		{"streaming", bp.ECOS{Flags: bp.ECOSStreaming, Ordinal: 1}},
		{"reliable", bp.ECOS{Flags: bp.ECOSReliable, Ordinal: 7}},
		{"with flow label", bp.ECOS{
			Flags: bp.ECOSFlowLabelPresent, Ordinal: 5, FlowLabel: 70000,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.ecos.Encode()
			if err != nil {
				t.Fatal(err)
			}
			// C2 d): block data length is 2 + N.
			if len(encoded) < 2 {
				t.Fatalf("encoded %d octets, want at least 2", len(encoded))
			}
			if tt.ecos.Flags&bp.ECOSFlowLabelPresent == 0 && len(encoded) != 2 {
				t.Errorf("encoded %d octets, want exactly 2 with no flow label", len(encoded))
			}

			got, err := bp.DecodeECOS(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got.Ordinal != tt.ecos.Ordinal {
				t.Errorf("ordinal = %d, want %d", got.Ordinal, tt.ecos.Ordinal)
			}
			if got.Flags != tt.ecos.Flags {
				t.Errorf("flags = %#02x, want %#02x", got.Flags, tt.ecos.Flags)
			}
			if got.FlowLabel != tt.ecos.FlowLabel {
				t.Errorf("flow label = %d, want %d", got.FlowLabel, tt.ecos.FlowLabel)
			}
		})
	}
}

func TestECOSBlockFlags(t *testing.T) {
	// C2 b): bit 0 of the block processing flags is set. C2 c): no EID
	// references.
	e := bp.ECOS{Ordinal: 1}
	block, err := e.Block()
	if err != nil {
		t.Fatal(err)
	}
	if !block.Flags.Has(bp.BlockReplicate) {
		t.Error("the ECOS block must be replicated in every fragment")
	}
	if block.Flags.Has(bp.BlockHasEIDRefs) {
		t.Error("the ECOS block must carry no EID references")
	}
}

func TestBundleWithECOS(t *testing.T) {
	b, err := bp.NewBundle(testPrimary(), []byte("payload"),
		bp.WithECOS(bp.ECOS{Flags: bp.ECOSCritical, Ordinal: 200}))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := bp.DecodeBundle(encoded)
	if err != nil {
		t.Fatal(err)
	}

	e, ok := got.ECOS()
	if !ok {
		t.Fatal("the ECOS block did not survive the round trip")
	}
	if e.Ordinal != 200 {
		t.Errorf("ordinal = %d, want 200", e.Ordinal)
	}
	if e.Flags&bp.ECOSCritical == 0 {
		t.Error("the critical flag was lost")
	}
}

func TestECOSMustPrecedePayload(t *testing.T) {
	// C3.1.1.
	ecos := bp.ECOS{Ordinal: 1}
	ecosBlock, err := ecos.Block()
	if err != nil {
		t.Fatal(err)
	}

	b := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: bp.BlockTypePayload, Data: []byte("payload")},
			ecosBlock,
		},
	}
	b.Blocks[1].Flags |= bp.BlockLast

	if err := b.Validate(); !errors.Is(err, bp.ErrInvalidECOS) {
		t.Errorf("error = %v, want ErrInvalidECOS", err)
	}
}

func TestECOSRulesEnforcedOnDecodedBundles(t *testing.T) {
	// Annex C, C2 b)/c) and C3.1.4 apply to any bundle, not just ones built
	// by the helper: a decoded ECOS block without the replicate flag, or
	// with ordinal 255 outside a custody signal, is refused.
	noReplicate := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: bp.BlockTypeECOS, Data: []byte{0x00, 0x01}}, // no BlockReplicate
			{Type: bp.BlockTypePayload, Flags: bp.BlockLast, Data: []byte("p")},
		},
	}
	if err := noReplicate.Validate(); !errors.Is(err, bp.ErrInvalidECOS) {
		t.Errorf("missing replicate flag: error = %v, want ErrInvalidECOS", err)
	}

	ordinal255 := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: bp.BlockTypeECOS, Flags: bp.BlockReplicate, Data: []byte{0x00, 0xFF}},
			{Type: bp.BlockTypePayload, Flags: bp.BlockLast, Data: []byte("p")},
		},
	}
	if err := ordinal255.Validate(); !errors.Is(err, bp.ErrInvalidECOS) {
		t.Errorf("ordinal 255 outside a custody signal: error = %v, want ErrInvalidECOS", err)
	}
}

func TestOnlyOneECOSBlock(t *testing.T) {
	// C3.1.2.
	_, err := bp.NewBundle(testPrimary(), []byte("payload"),
		bp.WithECOS(bp.ECOS{Ordinal: 1}),
		bp.WithECOS(bp.ECOS{Ordinal: 2}))
	if !errors.Is(err, bp.ErrInvalidECOS) {
		t.Errorf("error = %v, want ErrInvalidECOS", err)
	}
}
