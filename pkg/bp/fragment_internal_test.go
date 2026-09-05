package bp

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func fragmentableBundle(t *testing.T, payload string, extensions ...*CanonicalBlock) *Bundle {
	t.Helper()
	primary := &PrimaryBlock{
		CRCType:     CRC32C,
		Destination: IPN(1, 2),
		Source:      IPN(2, 1),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: 757382400000, Sequence: 9},
		Lifetime:    3600000,
	}
	b, err := NewBundle(primary, []byte(payload), extensions...)
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	return b
}

// Clause 5.8: the concatenation of every fragment's payload must be identical
// to the original's, and each fragment must carry the offset and total length.
func TestFragmentAndReassemble(t *testing.T) {
	const payload = "the quick brown fox jumps over the lazy dog"
	original := fragmentableBundle(t, payload)

	for _, size := range []int{1, 7, 10, 41, 42} {
		fragments, err := original.Fragment(size)
		if err != nil {
			t.Fatalf("size %d: Fragment: %v", size, err)
		}

		var joined []byte
		for i, f := range fragments {
			if !f.Primary.Flags.Has(FlagIsFragment) {
				t.Errorf("size %d: fragment %d is not flagged as one", size, i)
			}
			if f.Primary.TotalADULength != uint64(len(payload)) {
				t.Errorf("size %d: fragment %d total length = %d, want %d",
					size, i, f.Primary.TotalADULength, len(payload))
			}
			if got := f.Primary.FragmentOffset; got != uint64(len(joined)) {
				t.Errorf("size %d: fragment %d offset = %d, want %d", size, i, got, len(joined))
			}
			if len(f.Payload()) > size {
				t.Errorf("size %d: fragment %d carries %d octets", size, i, len(f.Payload()))
			}
			joined = append(joined, f.Payload()...)

			// Every fragment must survive a trip over the wire on its own.
			encoded, err := f.Encode()
			if err != nil {
				t.Fatalf("size %d: encoding fragment %d: %v", size, i, err)
			}
			if _, err := Decode(encoded); err != nil {
				t.Fatalf("size %d: decoding fragment %d: %v", size, i, err)
			}
		}
		if string(joined) != payload {
			t.Errorf("size %d: fragments joined to %q, want %q", size, joined, payload)
		}

		back, err := Reassemble(fragments)
		if err != nil {
			t.Fatalf("size %d: Reassemble: %v", size, err)
		}
		if string(back.Payload()) != payload {
			t.Errorf("size %d: reassembled %q, want %q", size, back.Payload(), payload)
		}
		if back.Primary.Flags.Has(FlagIsFragment) {
			t.Errorf("size %d: the reassembled bundle is still flagged as a fragment", size)
		}
	}
}

// Clause 5.8 has two replication rules: a block with the replicate flag goes
// in every fragment, and the offset-zero fragment carries all the rest.
func TestFragmentReplicatesExtensionBlocks(t *testing.T) {
	everywhere, err := NewHopCountBlock(2, 32, 0)
	if err != nil {
		t.Fatalf("NewHopCountBlock: %v", err)
	}
	everywhere.Flags = BlockFlagReplicateInEveryFragment

	firstOnly, err := NewPreviousNodeBlock(3, IPN(5, 0))
	if err != nil {
		t.Fatalf("NewPreviousNodeBlock: %v", err)
	}

	original := fragmentableBundle(t, "0123456789abcdef", everywhere, firstOnly)
	fragments, err := original.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}
	if len(fragments) != 4 {
		t.Fatalf("got %d fragments, want 4", len(fragments))
	}

	for i, f := range fragments {
		if f.blockOfType(BlockTypeHopCount) == nil {
			t.Errorf("fragment %d lost the block flagged for every fragment", i)
		}
		hasPrev := f.blockOfType(BlockTypePreviousNode) != nil
		if i == 0 && !hasPrev {
			t.Error("the offset-zero fragment did not carry the other extension blocks")
		}
		if i != 0 && hasPrev {
			t.Errorf("fragment %d carried a block only the offset-zero one should have", i)
		}
	}
}

// Clause 5.8 allows separate fragmentation episodes to produce overlapping
// slices of the same payload, so clause 5.9 works in material extents rather
// than assuming a clean partition.
func TestReassembleAcceptsOverlappingFragments(t *testing.T) {
	const payload = "abcdefghijklmnop"
	original := fragmentableBundle(t, payload)

	coarse, err := original.Fragment(8)
	if err != nil {
		t.Fatalf("Fragment(8): %v", err)
	}
	fine, err := original.Fragment(3)
	if err != nil {
		t.Fatalf("Fragment(3): %v", err)
	}

	// A mixed bag from two episodes, out of order and overlapping.
	mixed := []*Bundle{fine[4], coarse[0], fine[1], coarse[1], fine[0]}
	back, err := Reassemble(mixed)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if string(back.Payload()) != payload {
		t.Errorf("reassembled %q, want %q", back.Payload(), payload)
	}
}

func TestReassembleRejects(t *testing.T) {
	original := fragmentableBundle(t, "abcdefghijklmnop")
	fragments, err := original.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}

	if _, err := Reassemble(nil); !errors.Is(err, ErrNoFragments) {
		t.Errorf("empty set: err = %v, want ErrNoFragments", err)
	}

	// A gap in the middle.
	if _, err := Reassemble([]*Bundle{fragments[0], fragments[3]}); !errors.Is(err, ErrIncompleteReassembly) {
		t.Errorf("gap: err = %v, want ErrIncompleteReassembly", err)
	}

	// Nothing covering offset zero.
	if _, err := Reassemble(fragments[1:]); !errors.Is(err, ErrIncompleteReassembly) {
		t.Errorf("no offset zero: err = %v, want ErrIncompleteReassembly", err)
	}

	// A whole bundle is not a fragment.
	if _, err := Reassemble([]*Bundle{original}); !errors.Is(err, ErrNotAFragment) {
		t.Errorf("whole bundle: err = %v, want ErrNotAFragment", err)
	}

	// Fragments of two different originals. Clause 5.9 keys on the source node
	// ID and creation timestamp together, so a different timestamp is a
	// different bundle even from the same source.
	other := fragmentableBundle(t, "ABCDEFGHIJKLMNOP")
	other.Primary.Timestamp.Sequence = 99
	otherFragments, err := other.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}
	mixed := []*Bundle{fragments[0], otherFragments[1], fragments[2], fragments[3]}
	if _, err := Reassemble(mixed); !errors.Is(err, ErrFragmentsDoNotMatch) {
		t.Errorf("mixed originals: err = %v, want ErrFragmentsDoNotMatch", err)
	}
}

// A declared total past MaxReassembledADU must be rejected before Reassemble
// tries to allocate anything that large.
func TestReassembleRejectsHugeTotal(t *testing.T) {
	original := fragmentableBundle(t, "abcdefghijklmnop")
	fragments, err := original.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}

	fragments[0].Primary.TotalADULength = 1 << 48
	if _, err := Reassemble(fragments[:1]); !errors.Is(err, ErrADUTooLarge) {
		t.Errorf("huge total: err = %v, want ErrADUTooLarge", err)
	}
}

// A fragment offset near the top of uint64 must not wrap the range check's
// addition into passing; it must be rejected outright rather than panicking
// on the later slice into adu.
func TestReassembleRejectsOverflowingOffset(t *testing.T) {
	original := fragmentableBundle(t, "abcdefghijklmnop")
	fragments, err := original.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}

	fragments[0].Primary.FragmentOffset = math.MaxUint64 - 4
	fragments[0].PayloadBlock().Data = make([]byte, 16)
	if _, err := Reassemble(fragments[:1]); !errors.Is(err, ErrFragmentPastEnd) {
		t.Errorf("overflowing offset: err = %v, want ErrFragmentPastEnd", err)
	}
}

func TestFragmentRejects(t *testing.T) {
	original := fragmentableBundle(t, "abcdefghij")

	if _, err := original.Fragment(0); !errors.Is(err, ErrFragmentSizeTooSmall) {
		t.Errorf("size 0: err = %v, want ErrFragmentSizeTooSmall", err)
	}

	original.Primary.Flags |= FlagMustNotFragment
	if _, err := original.Fragment(4); !errors.Is(err, ErrMustNotFragment) {
		t.Errorf("must-not-fragment: err = %v, want ErrMustNotFragment", err)
	}
}

// A payload that already fits comes back as itself, not as a one-item
// fragment set with the flag needlessly set.
func TestFragmentNoOpWhenItFits(t *testing.T) {
	original := fragmentableBundle(t, "short")
	fragments, err := original.Fragment(1024)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}
	if len(fragments) != 1 || fragments[0] != original {
		t.Fatalf("got %d fragments, want the original back unchanged", len(fragments))
	}
	if fragments[0].Primary.Flags.Has(FlagIsFragment) {
		t.Error("a bundle that fits was flagged as a fragment")
	}
}

// Fragmenting a fragment stays at one level: offsets are relative to the
// original application data unit, not to the piece being split (clause 5.8).
func TestFragmentOfAFragmentKeepsAbsoluteOffsets(t *testing.T) {
	const payload = "abcdefghijklmnop"
	original := fragmentableBundle(t, payload)

	halves, err := original.Fragment(8)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}
	quarters, err := halves[1].Fragment(4)
	if err != nil {
		t.Fatalf("Fragment of a fragment: %v", err)
	}

	if got := quarters[0].Primary.FragmentOffset; got != 8 {
		t.Errorf("first sub-fragment offset = %d, want 8", got)
	}
	if got := quarters[1].Primary.FragmentOffset; got != 12 {
		t.Errorf("second sub-fragment offset = %d, want 12", got)
	}
	for i, q := range quarters {
		if q.Primary.TotalADULength != uint64(len(payload)) {
			t.Errorf("sub-fragment %d total = %d, want %d", i, q.Primary.TotalADULength, len(payload))
		}
	}

	back, err := Reassemble([]*Bundle{halves[0], quarters[0], quarters[1]})
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if string(back.Payload()) != payload {
		t.Errorf("reassembled %q, want %q", back.Payload(), payload)
	}
}

// The payload of a fragment must not alias the original's buffer.
func TestFragmentCopiesPayload(t *testing.T) {
	payload := []byte("abcdefghijklmnop")
	original := fragmentableBundle(t, string(payload))
	fragments, err := original.Fragment(4)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}

	before := append([]byte(nil), fragments[0].Payload()...)
	for i := range original.PayloadBlock().Data {
		original.PayloadBlock().Data[i] = 0
	}
	if !bytes.Equal(fragments[0].Payload(), before) {
		t.Error("a fragment's payload aliased the original bundle's buffer")
	}
}
