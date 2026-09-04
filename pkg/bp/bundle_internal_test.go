package bp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/cbor"
)

// RFC 9173 appendix A.1.1.3, the whole bundle: the primary block of A.1.1.1
// and the payload block of A.1.1.2, wrapped in an indefinite-length array and
// closed by a break.
//
// This vector settles the contradiction in RFC 9171. Appendix B's CDDL writes
// the bundle as "[primary-block, *extension-block, payload-block]", which reads
// as a definite-length array; clause 4.1 says indefinite, and the appendix
// says the prose wins. The published bytes open with 0x9f and end with 0xff,
// which is the indefinite form. Clause 4.1 is right.
const rfc9173Bundle = "9f" + rfc9173PrimaryBlock + rfc9173PayloadBlock + "ff"

func TestBundleRFC9173Vector(t *testing.T) {
	raw := mustHex(t, rfc9173Bundle)

	if raw[0] != 0x9F {
		t.Fatalf("the published bundle opens with 0x%02X, not the indefinite array head 0x9F", raw[0])
	}
	if raw[len(raw)-1] != 0xFF {
		t.Fatalf("the published bundle ends with 0x%02X, not a break stop code", raw[len(raw)-1])
	}

	b, err := Decode(raw)
	if err != nil {
		t.Fatalf("decoding the published bundle: %v", err)
	}
	if got := string(b.Payload()); got != rfc9173Payload {
		t.Errorf("payload = %q, want %q", got, rfc9173Payload)
	}
	if b.Primary.Destination != IPN(1, 2) || b.Primary.Source != IPN(2, 1) {
		t.Errorf("endpoints = %v -> %v, want ipn:2.1 -> ipn:1.2", b.Primary.Source, b.Primary.Destination)
	}
	if b.Primary.Lifetime != 1000000 {
		t.Errorf("lifetime = %d, want 1000000", b.Primary.Lifetime)
	}

	// Re-encode block by block. Encode itself refuses this bundle, because its
	// creation time is unknown and it carries no Bundle Age block — see the
	// test below. The wire bytes still have to come out identical.
	out := cbor.AppendIndefiniteArrayHeader(nil)
	if out, err = appendPrimaryBlock(out, b.Primary); err != nil {
		t.Fatalf("re-encoding the primary block: %v", err)
	}
	for _, blk := range b.Blocks {
		if out, err = appendCanonicalBlock(out, blk); err != nil {
			t.Fatalf("re-encoding block %d: %v", blk.Number, err)
		}
	}
	out = cbor.AppendBreak(out)

	if !bytes.Equal(out, raw) {
		t.Errorf("re-encoded\n got %s\nwant %s", hex.EncodeToString(out), rfc9173Bundle)
	}
}

// The published example does not satisfy clause 4.4.2: its creation time is
// zero and it has no Bundle Age block. astro reads it and refuses to create
// one like it. Pin both halves of that so neither drifts.
func TestBundleAgeRuleIsCreateOnly(t *testing.T) {
	raw := mustHex(t, rfc9173Bundle)

	b, err := Decode(raw)
	if err != nil {
		t.Fatalf("a receiver must accept it: %v", err)
	}
	if err := b.Validate(); !errors.Is(err, ErrMissingBundleAgeBlock) {
		t.Errorf("Validate = %v, want ErrMissingBundleAgeBlock", err)
	}
	if _, err := b.Encode(); !errors.Is(err, ErrMissingBundleAgeBlock) {
		t.Errorf("Encode = %v, want ErrMissingBundleAgeBlock", err)
	}

	// Adding the block clause 4.4.2 asks for makes it encodable.
	age, err := NewBundleAgeBlock(2, 300)
	if err != nil {
		t.Fatalf("NewBundleAgeBlock: %v", err)
	}
	b.Blocks = append([]*CanonicalBlock{age}, b.Blocks...)
	if _, err := b.Encode(); err != nil {
		t.Errorf("with a Bundle Age block, Encode still failed: %v", err)
	}
}

func TestBundleRoundTrip(t *testing.T) {
	hop, err := NewHopCountBlock(2, 32, 0)
	if err != nil {
		t.Fatalf("NewHopCountBlock: %v", err)
	}
	prev, err := NewPreviousNodeBlock(3, IPN(5, 0))
	if err != nil {
		t.Fatalf("NewPreviousNodeBlock: %v", err)
	}

	primary := &PrimaryBlock{
		CRCType:     CRC32C,
		Destination: DTN("//receiver/inbox"),
		Source:      IPN(2, 1),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: 757382400000, Sequence: 3},
		Lifetime:    86400000,
		Flags:       FlagReportDelivery | FlagStatusTimeRequested,
	}
	original, err := NewBundle(primary, []byte("telemetry frame"), hop, prev)
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if *back.Primary != *original.Primary {
		t.Errorf("primary block = %+v, want %+v", *back.Primary, *original.Primary)
	}
	if len(back.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(back.Blocks))
	}
	if !bytes.Equal(back.Payload(), original.Payload()) {
		t.Errorf("payload = %q, want %q", back.Payload(), original.Payload())
	}
	limit, count, err := back.Blocks[0].HopCount()
	if err != nil || limit != 32 || count != 0 {
		t.Errorf("hop count = (%d, %d, %v), want (32, 0, nil)", limit, count, err)
	}
}

// Plan 025 found four packages replaying a checksum computed at construction
// time, so a mutated header shipped an invalid frame. Encode here recomputes,
// and this proves it.
func TestBundleEncodeRecomputesCRC(t *testing.T) {
	primary := &PrimaryBlock{
		CRCType:     CRC32C,
		Destination: IPN(1, 2),
		Source:      IPN(2, 1),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: 1000, Sequence: 1},
		Lifetime:    1000,
	}
	b, err := NewBundle(primary, []byte("payload"))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	first, err := b.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	b.Primary.Lifetime = 2000
	second, err := b.Encode()
	if err != nil {
		t.Fatalf("Encode after mutation: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("changing the lifetime did not change the encoding")
	}

	// The point is not that the bytes differ but that the new ones verify.
	back, err := Decode(second)
	if err != nil {
		t.Fatalf("the re-encoded bundle does not verify: %v", err)
	}
	if back.Primary.Lifetime != 2000 {
		t.Errorf("lifetime = %d, want 2000", back.Primary.Lifetime)
	}
}

func TestBundleRejects(t *testing.T) {
	primary := func() *PrimaryBlock {
		return &PrimaryBlock{
			Destination: IPN(1, 2), Source: IPN(2, 1), ReportTo: IPN(2, 1),
			Timestamp: CreationTimestamp{Time: 1000, Sequence: 1},
		}
	}

	encodeTests := []struct {
		name   string
		bundle func() *Bundle
		want   error
	}{
		{"no primary block", func() *Bundle {
			return &Bundle{Blocks: []*CanonicalBlock{NewPayloadBlock(nil)}}
		}, ErrNoPrimaryBlock},
		{"no blocks at all", func() *Bundle {
			return &Bundle{Primary: primary()}
		}, ErrNoPayloadBlock},
		// Two payload blocks trip the "last block" rule first, since the
		// earlier one is by definition not last. Either way it is refused.
		{"two payload blocks", func() *Bundle {
			return &Bundle{Primary: primary(), Blocks: []*CanonicalBlock{
				NewPayloadBlock([]byte("a")), NewPayloadBlock([]byte("b")),
			}}
		}, ErrPayloadBlockNotLast},
		{"payload block not last", func() *Bundle {
			age, _ := NewBundleAgeBlock(2, 1)
			return &Bundle{Primary: primary(), Blocks: []*CanonicalBlock{
				NewPayloadBlock([]byte("a")), age,
			}}
		}, ErrPayloadBlockNotLast},
		{"two blocks sharing a number", func() *Bundle {
			age, _ := NewBundleAgeBlock(2, 1)
			hop, _ := NewHopCountBlock(2, 10, 0)
			return &Bundle{Primary: primary(), Blocks: []*CanonicalBlock{
				age, hop, NewPayloadBlock([]byte("a")),
			}}
		}, ErrDuplicateBlockNumber},
		{"two hop count blocks", func() *Bundle {
			one, _ := NewHopCountBlock(2, 10, 0)
			two, _ := NewHopCountBlock(3, 10, 0)
			return &Bundle{Primary: primary(), Blocks: []*CanonicalBlock{
				one, two, NewPayloadBlock([]byte("a")),
			}}
		}, ErrDuplicateExtensionBlock},
	}

	for _, tt := range encodeTests {
		if _, err := tt.bundle().Encode(); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	decodeTests := []struct {
		name  string
		input string
		want  error
	}{
		{"definite-length array", "82" + rfc9173PrimaryBlock + rfc9173PayloadBlock, ErrDefiniteLengthBundle},
		{"no break at the end", "9f" + rfc9173PrimaryBlock + rfc9173PayloadBlock, ErrTruncated},
		{"bytes after the break", rfc9173Bundle + "00", ErrTrailingBytes},
		{"primary block only", "9f" + rfc9173PrimaryBlock + "ff", ErrNoPayloadBlock},
	}
	for _, tt := range decodeTests {
		if _, err := Decode(mustHex(t, tt.input)); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}
