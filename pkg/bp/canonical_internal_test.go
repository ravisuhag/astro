package bp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/internal/cbor"
)

// The payload for RFC 9173's worked examples: a 35-octet ASCII string, carried
// raw. The appendix is explicit that it is not wrapped in a CBOR text string
// inside the byte string — payload data is opaque, unlike extension data.
const rfc9173Payload = "Ready to generate a 32-byte payload"

// RFC 9173 appendix A.1.1.2.
const rfc9173PayloadBlock = "85010100005823526561647920746f2067656e657261746520612033322d62797465207061796c6f6164"

// RFC 9173 appendix A.3.1.2: a Bundle Age block carrying 300 milliseconds.
// It is the vector that proves extension data is two layers deep — the
// block-type-specific field is a three-octet byte string 0x43, and inside it
// sits the CBOR integer 0x19 0x012C.
const rfc9173BundleAgeBlock = "85070200004319012c"

func TestCanonicalBlockRFC9173Vectors(t *testing.T) {
	tests := []struct {
		name  string
		build func() (*CanonicalBlock, error)
		want  string
	}{
		{
			"payload block",
			func() (*CanonicalBlock, error) { return NewPayloadBlock([]byte(rfc9173Payload)), nil },
			rfc9173PayloadBlock,
		},
		{
			"bundle age block, 300 ms",
			func() (*CanonicalBlock, error) { return NewBundleAgeBlock(2, 300) },
			rfc9173BundleAgeBlock,
		},
	}

	for _, tt := range tests {
		b, err := tt.build()
		if err != nil {
			t.Errorf("%s: build: %v", tt.name, err)
			continue
		}
		got, err := appendCanonicalBlock(nil, b)
		if err != nil {
			t.Errorf("%s: encode: %v", tt.name, err)
			continue
		}
		if hex.EncodeToString(got) != tt.want {
			t.Errorf("%s\n got %s\nwant %s", tt.name, hex.EncodeToString(got), tt.want)
			continue
		}

		back, err := decodeCanonicalBlock(cbor.NewDecoder(mustHex(t, tt.want)))
		if err != nil {
			t.Errorf("%s: decode: %v", tt.name, err)
			continue
		}
		if back.Type != b.Type || back.Number != b.Number || !bytes.Equal(back.Data, b.Data) {
			t.Errorf("%s round trip = %+v, want %+v", tt.name, back, b)
		}
	}
}

// Extension data is CBOR inside a byte string. The accessors peel both layers,
// so a caller never has to know that.
func TestExtensionBlockAccessors(t *testing.T) {
	age, err := NewBundleAgeBlock(2, 300)
	if err != nil {
		t.Fatalf("NewBundleAgeBlock: %v", err)
	}
	if got, err := age.BundleAge(); err != nil || got != 300 {
		t.Errorf("BundleAge = (%d, %v), want (300, nil)", got, err)
	}

	prev, err := NewPreviousNodeBlock(3, IPN(2, 1))
	if err != nil {
		t.Fatalf("NewPreviousNodeBlock: %v", err)
	}
	if got, err := prev.PreviousNode(); err != nil || got != IPN(2, 1) {
		t.Errorf("PreviousNode = (%v, %v), want (ipn:2.1, nil)", got, err)
	}

	hop, err := NewHopCountBlock(4, 32, 5)
	if err != nil {
		t.Fatalf("NewHopCountBlock: %v", err)
	}
	limit, count, err := hop.HopCount()
	if err != nil || limit != 32 || count != 5 {
		t.Errorf("HopCount = (%d, %d, %v), want (32, 5, nil)", limit, count, err)
	}

	// An accessor called on the wrong block type says so rather than
	// misreading the bytes.
	if _, err := age.PreviousNode(); !errors.Is(err, ErrWrongBlockType) {
		t.Errorf("PreviousNode on a Bundle Age block: err = %v, want ErrWrongBlockType", err)
	}
}

// Clause 4.4 says a node must forward blocks it cannot parse, honouring their
// flags. So an unknown type keeps its bytes exactly.
func TestCanonicalBlockUnknownTypeRoundTrips(t *testing.T) {
	original := &CanonicalBlock{
		Type:    200, // clause 9.1 leaves 192-255 to private use
		Number:  7,
		Flags:   BlockFlagDiscardBlockIfUnprocessable | BlockFlagReportIfUnprocessable,
		CRCType: CRC32C,
		Data:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	encoded, err := appendCanonicalBlock(nil, original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	back, err := decodeCanonicalBlock(cbor.NewDecoder(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if back.Type != original.Type || back.Number != original.Number ||
		back.Flags != original.Flags || back.CRCType != original.CRCType ||
		!bytes.Equal(back.Data, original.Data) {
		t.Errorf("unknown block changed in transit: %+v, want %+v", back, original)
	}
}

// A decoded block must not point into the buffer it was read from, or a caller
// who reuses that buffer silently corrupts the block.
func TestCanonicalBlockOwnsItsData(t *testing.T) {
	input := mustHex(t, rfc9173BundleAgeBlock)
	b, err := decodeCanonicalBlock(cbor.NewDecoder(input))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	before := append([]byte(nil), b.Data...)

	for i := range input {
		input[i] = 0
	}
	if !bytes.Equal(b.Data, before) {
		t.Error("the decoded block aliased its input buffer")
	}
}

func TestCanonicalBlockRejects(t *testing.T) {
	buildTests := []struct {
		name  string
		build func() (*CanonicalBlock, error)
		want  error
	}{
		{"bundle age numbered 1", func() (*CanonicalBlock, error) { return NewBundleAgeBlock(1, 300) }, ErrReservedBlockNumber},
		{"bundle age numbered 0", func() (*CanonicalBlock, error) { return NewBundleAgeBlock(0, 300) }, ErrReservedBlockNumber},
		{"hop limit of 0", func() (*CanonicalBlock, error) { return NewHopCountBlock(2, 0, 0) }, ErrHopLimitOutOfRange},
		{"hop limit of 256", func() (*CanonicalBlock, error) { return NewHopCountBlock(2, 256, 0) }, ErrHopLimitOutOfRange},
	}
	for _, tt := range buildTests {
		if _, err := tt.build(); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	encodeTests := []struct {
		name  string
		block CanonicalBlock
		want  error
	}{
		{"payload block numbered 2", CanonicalBlock{Type: BlockTypePayload, Number: 2}, ErrPayloadBlockNumber},
		{"reserved block type 0", CanonicalBlock{Type: 0, Number: 2}, ErrReservedBlockType},
		{"undefined CRC type", CanonicalBlock{Type: BlockTypePayload, Number: 1, CRCType: 9}, ErrInvalidCRCType},
	}
	for _, tt := range encodeTests {
		if _, err := appendCanonicalBlock(nil, &tt.block); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	decodeTests := []struct {
		name  string
		input string
		want  error
	}{
		{"array of four items", "8401010000", ErrMalformedCanonicalBlock},
		{"six items with no CRC type", "86010100004000", ErrCanonicalBlockLengthMismatch},
		{"indefinite-length array", "9f0101000040ff", ErrMalformedCanonicalBlock},
	}
	for _, tt := range decodeTests {
		if _, err := decodeCanonicalBlock(cbor.NewDecoder(mustHex(t, tt.input))); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}
