package bp

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/internal/cbor"
)

// The ipn vectors are the worked examples of RFC 9758 clause 6.1.1 and 6.1.2.
// The dtn:none vector is RFC 9171 clause 4.2.5.1.1: the one dtn
// scheme-specific part encoded as a number rather than a string.
//
// The plain ipn:1.2 case is transcribed from RFC 9173 appendix A.1.1.1, which
// prints the primary block of a real bundle. That matters more than it looks:
// RFC 9173 predates RFC 9758 and writes the two-element form with no allocator,
// so it proves this encoder is byte-compatible with RFC 9171 as published.
func TestEIDEncoding(t *testing.T) {
	tests := []struct {
		name string
		eid  EID
		want string
	}{
		{"null endpoint dtn:none", NullEID(), "820100"},
		{"dtn scheme with a text SSP", DTN("//node1/service"), "8201" + "6f2f2f6e6f6465312f73657276696365"},
		{"ipn:1.2 as RFC 9173 prints it", IPN(1, 2), "8202820102"},
		{"ipn:2.1 as RFC 9173 prints it", IPN(2, 1), "8202820201"},
		{"ipn with an allocator, RFC 9758 clause 6.1.1", IPNWithAllocator(977000, 100, 1), "8202821b000ee8680000006401"},
	}

	for _, tt := range tests {
		got, err := appendEID(nil, tt.eid)
		if err != nil {
			t.Errorf("%s: appendEID: %v", tt.name, err)
			continue
		}
		if hex.EncodeToString(got) != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, hex.EncodeToString(got), tt.want)
			continue
		}

		back, err := decodeEIDFrom(cbor.NewDecoder(got))
		if err != nil {
			t.Errorf("%s: decode: %v", tt.name, err)
			continue
		}
		if back != tt.eid {
			t.Errorf("%s round trip = %+v, want %+v", tt.name, back, tt.eid)
		}
	}
}

// RFC 9758 clause 6.2 says the two encodings carry the same EID. Read both
// spellings of ipn:977000.100.1 and check they land on the same Scheme,
// Allocator, Node and Service.
//
// The two do not compare equal with == any more: decoding keeps track of
// which spelling was used (the unexported form field) so that re-encoding
// reproduces it rather than folding both into this library's preferred
// two-element form — see EID's form field and B6 in plans/009. That is a
// deliberate change from the wire-compatibility this test used to check by
// comparing whole structs, so it now compares the fields a caller can see.
func TestEIDBothIPNFormsAgree(t *testing.T) {
	two := mustHex(t, "8202821b000ee8680000006401")        // clause 6.1.1
	three := mustHex(t, "820283"+"1a000ee868"+"1864"+"01") // clause 6.1.2

	fromTwo, err := decodeEIDFrom(cbor.NewDecoder(two))
	if err != nil {
		t.Fatalf("two-element form: %v", err)
	}
	fromThree, err := decodeEIDFrom(cbor.NewDecoder(three))
	if err != nil {
		t.Fatalf("three-element form: %v", err)
	}
	if fromTwo.Scheme != fromThree.Scheme || fromTwo.Allocator != fromThree.Allocator ||
		fromTwo.Node != fromThree.Node || fromTwo.Service != fromThree.Service {
		t.Errorf("two-element gave %+v, three-element gave %+v", fromTwo, fromThree)
	}
	want := IPNWithAllocator(977000, 100, 1)
	if fromTwo != want {
		t.Errorf("decoded %+v, want %+v", fromTwo, want)
	}
	if fromThree == want {
		t.Errorf("three-element form decoded == a caller-built EID; want its form remembered as distinct")
	}
}

// B6: an EID decoded from a spelling other than this library's preferred one
// must re-encode to the exact octets it arrived with, not silently normalize
// to the preferred spelling. RFC 9171 clause 4.3.1 requires the primary block
// to survive a hop unchanged, and pkg/bpsec rebuilds the integrity-protected
// plaintext by calling Encode on whatever pkg/bp decoded (see
// TestIntegrityAcceptsIPNThreeElementSource and
// TestIntegrityAcceptsDTNNoneAsTextSource in pkg/bpsec for that end-to-end
// case).
func TestEIDRoundTripsTheDecodedForm(t *testing.T) {
	tests := []struct {
		name string
		hex  string
	}{
		{"ipn three-element form, RFC 9758 clause 6.1.2", "820283" + "1a000ee868" + "1864" + "01"},
		{"dtn null endpoint spelled as text, RFC 9171 clause 4.2.5.1.1", "8201" + "646e6f6e65"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := mustHex(t, tt.hex)
			decoded, err := decodeEIDFrom(cbor.NewDecoder(want))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			got, err := appendEID(nil, decoded)
			if err != nil {
				t.Fatalf("appendEID: %v", err)
			}
			if hex.EncodeToString(got) != hex.EncodeToString(want) {
				t.Errorf("round trip = %s, want %s", hex.EncodeToString(got), hex.EncodeToString(want))
			}
		})
	}
}

// A caller building an EID by hand -- rather than decoding one -- always gets
// this library's preferred spelling: the ipn two-element form regardless of
// whether the allocator is zero, and the dtn null endpoint as the integer 0.
// Originated bundles are unaffected by the form field decoding introduces.
func TestEIDCallerBuiltAlwaysUsesThePreferredForm(t *testing.T) {
	for _, e := range []EID{
		IPN(2, 1),
		IPNWithAllocator(0, 2, 1),
		IPNWithAllocator(977000, 100, 1), // a nonzero allocator does not switch the wire form
	} {
		got, err := appendEID(nil, e)
		if err != nil {
			t.Fatalf("%s: appendEID: %v", e, err)
		}
		// [scheme, [fully-qualified node number, service]]: the SSP array
		// header, the third octet, is 0x82 (two elements), never 0x83.
		if got[2] != 0x82 {
			t.Errorf("%s: SSP array header = 0x%02x, want 0x82 (two-element)", e, got[2])
		}
	}

	for _, e := range []EID{NullEID(), DTN("none")} {
		got, err := appendEID(nil, e)
		if err != nil {
			t.Fatalf("%s: appendEID: %v", e, err)
		}
		if hex.EncodeToString(got) != "820100" {
			t.Errorf("%s = %s, want 820100 (the null endpoint as the integer 0)", e, hex.EncodeToString(got))
		}
	}
}

// With the Default Allocator the packed node number is just the node number,
// so the octets are the ones RFC 9171 clause 4.2.5.1.2 specifies. An
// implementation that never heard of RFC 9758 still reads them.
func TestEIDDefaultAllocatorMatchesRFC9171(t *testing.T) {
	got, err := appendEID(nil, IPN(1, 2))
	if err != nil {
		t.Fatalf("appendEID: %v", err)
	}
	// [2, [1, 2]] with no allocator anywhere in the octets.
	if hex.EncodeToString(got) != "8202820102" {
		t.Fatalf("got %s, want 8202820102", hex.EncodeToString(got))
	}
}

func TestEIDIsNull(t *testing.T) {
	tests := []struct {
		name string
		eid  EID
		want bool
	}{
		{"dtn:none", NullEID(), true},
		{"ipn:0.0", IPN(0, 0), true},
		{"ipn:0.0.0", IPNWithAllocator(0, 0, 0), true},
		{"dtn with a real SSP", DTN("//node1/svc"), false},
		{"ipn:1.2", IPN(1, 2), false},
	}
	for _, tt := range tests {
		if got := tt.eid.IsNull(); got != tt.want {
			t.Errorf("%s: IsNull = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEIDString(t *testing.T) {
	tests := []struct {
		eid  EID
		want string
	}{
		{NullEID(), "dtn:none"},
		{DTN("//node1/service"), "dtn://node1/service"},
		{IPN(1, 2), "ipn:1.2"},
		{IPNWithAllocator(977000, 100, 1), "ipn:977000.100.1"},
	}
	for _, tt := range tests {
		if got := tt.eid.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestEIDRejects(t *testing.T) {
	encodeTests := []struct {
		name string
		eid  EID
		want error
	}{
		{"unknown scheme code", EID{Scheme: 99}, ErrUnknownURIScheme},
		{"allocator at 2^32", IPNWithAllocator(1<<32, 1, 1), ErrIPNComponentTooLarge},
		{"node number at 2^32", IPNWithAllocator(0, 1<<32, 1), ErrIPNComponentTooLarge},
	}
	for _, tt := range encodeTests {
		if _, err := appendEID(nil, tt.eid); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	decodeTests := []struct {
		name  string
		input string
		want  error
	}{
		{"scheme code 3 is not defined", "8203820102", ErrUnknownURIScheme},
		{"endpoint array of one item", "8102", ErrMalformedEID},
		{"endpoint array of three items", "830201820102", ErrMalformedEID},
		{"dtn SSP is a non-zero number", "820101", ErrMalformedEID},
		{"ipn SSP array of one item", "82028101", ErrMalformedEID},
		{"ipn SSP array of four items", "82028401020304", ErrMalformedEID},
	}
	for _, tt := range decodeTests {
		if _, err := decodeEIDFrom(cbor.NewDecoder(mustHex(t, tt.input))); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

// DTN time is milliseconds since 2000-01-01, not seconds and not since 1970.
// Pin it against wall-clock times, not a round trip: a wrong epoch or a wrong
// unit round-trips perfectly and is still wrong on the wire.
func TestDTNTime(t *testing.T) {
	tests := []struct {
		name string
		utc  time.Time
		dtn  DTNTime
	}{
		{"the DTN epoch itself", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"one second after the epoch", time.Date(2000, 1, 1, 0, 0, 1, 0, time.UTC), 1000},
		{"2024-01-01T00:00:00Z", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 757382400000},
	}

	for _, tt := range tests {
		got, ok := NewDTNTime(tt.utc)
		if !ok {
			t.Errorf("%s: NewDTNTime reported out of range", tt.name)
			continue
		}
		if got != tt.dtn {
			t.Errorf("%s: NewDTNTime = %d, want %d", tt.name, got, tt.dtn)
		}
		if back := tt.dtn.Time(); !back.Equal(tt.utc) {
			t.Errorf("%s: Time() = %v, want %v", tt.name, back, tt.utc)
		}
	}

	// The DTN epoch starts in 2000, so 1999 has no representation.
	if _, ok := NewDTNTime(time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC)); ok {
		t.Error("a time before the DTN epoch was accepted")
	}
}

// RFC 9173 appendix A.1.1.1 gives the creation timestamp [0, 40]: a node with
// no clock, sequence number 40.
func TestCreationTimestamp(t *testing.T) {
	ts := CreationTimestamp{Time: DTNTimeUnknown, Sequence: 40}

	got := appendCreationTimestamp(nil, ts)
	if hex.EncodeToString(got) != "82001828" {
		t.Fatalf("got %s, want 82001828", hex.EncodeToString(got))
	}

	back, err := decodeCreationTimestamp(cbor.NewDecoder(got))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back != ts {
		t.Errorf("round trip = %+v, want %+v", back, ts)
	}
}
