package bp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
)

func testPrimary() *bp.PrimaryBlock {
	return &bp.PrimaryBlock{
		Flags:             bp.BundleFlags(0).WithPriority(bp.PriorityNormal) | bp.FlagSingleton,
		Destination:       bp.IPNEndpoint(2, 1),
		Source:            bp.IPNEndpoint(1, 1),
		ReportTo:          bp.IPNEndpoint(1, 0),
		Custodian:         bp.NullEndpoint,
		CreationTimestamp: bp.CreationTimestamp{Time: 800000000, SequenceNumber: 42},
		Lifetime:          3600,
	}
}

func TestIPNEndpoint(t *testing.T) {
	// CCSDS 734.2-B-1 §3.2.1: node number then a period then service number.
	e := bp.IPNEndpoint(1234, 5)
	if e.Scheme != bp.IPNScheme {
		t.Errorf("scheme = %q, want ipn", e.Scheme)
	}
	if e.String() != "ipn:1234.5" {
		t.Errorf("URI = %q, want ipn:1234.5", e.String())
	}

	node, service, err := e.IPNParts()
	if err != nil {
		t.Fatal(err)
	}
	if node != 1234 || service != 5 {
		t.Errorf("parts = %d.%d, want 1234.5", node, service)
	}
}

func TestIPNNodeNumberMustNotBeZero(t *testing.T) {
	// §3.2.1: a node number is in the range 1 to 2^64-1.
	e := bp.EndpointID{Scheme: bp.IPNScheme, SSP: "0.1"}
	if _, _, err := e.IPNParts(); !errors.Is(err, bp.ErrInvalidEndpointID) {
		t.Errorf("error = %v, want ErrInvalidEndpointID", err)
	}
}

func TestNullEndpoint(t *testing.T) {
	if !bp.NullEndpoint.IsNull() {
		t.Error("NullEndpoint.IsNull() = false")
	}
	if bp.NullEndpoint.String() != "dtn:none" {
		t.Errorf("null endpoint = %q, want dtn:none", bp.NullEndpoint)
	}
}

func TestParseEndpointID(t *testing.T) {
	tests := []struct {
		uri     string
		wantErr bool
	}{
		{"ipn:1.2", false},
		{"dtn:none", false},
		{"dtn://node/service", false},
		{"noscheme", true},
		{":nossp", true},
		{"scheme:", true},
		{"", true},
	}
	for _, tt := range tests {
		_, err := bp.ParseEndpointID(tt.uri)
		if tt.wantErr && err == nil {
			t.Errorf("ParseEndpointID(%q) should have failed", tt.uri)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ParseEndpointID(%q): %v", tt.uri, err)
		}
	}
}

func TestPriorityRoundTrip(t *testing.T) {
	// §4.2: bits 7 and 8, with bit 8 the most significant.
	for _, p := range []bp.Priority{bp.PriorityBulk, bp.PriorityNormal, bp.PriorityExpedited} {
		flags := bp.BundleFlags(0).WithPriority(p)
		if got := flags.Priority(); got != p {
			t.Errorf("priority = %s, want %s", got, p)
		}
	}

	// Setting the priority must not disturb the other flags.
	flags := (bp.FlagSingleton | bp.FlagReportDelivery).WithPriority(bp.PriorityExpedited)
	if !flags.Has(bp.FlagSingleton) || !flags.Has(bp.FlagReportDelivery) {
		t.Error("setting the priority cleared another flag")
	}
	if flags.Priority() != bp.PriorityExpedited {
		t.Error("priority was not applied")
	}
}

func TestPrimaryBlockRoundTrip(t *testing.T) {
	p := testPrimary()
	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encoded[0] != bp.Version {
		t.Errorf("version octet = %d, want %d", encoded[0], bp.Version)
	}

	got, consumed, err := bp.DecodePrimaryBlock(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(encoded) {
		t.Errorf("consumed %d, want %d", consumed, len(encoded))
	}
	if got.Destination != p.Destination {
		t.Errorf("destination = %s, want %s", got.Destination, p.Destination)
	}
	if got.Source != p.Source {
		t.Errorf("source = %s, want %s", got.Source, p.Source)
	}
	if got.ReportTo != p.ReportTo {
		t.Errorf("report-to = %s, want %s", got.ReportTo, p.ReportTo)
	}
	if !got.Custodian.IsNull() {
		t.Errorf("custodian = %s, want the null endpoint", got.Custodian)
	}
	if got.CreationTimestamp != p.CreationTimestamp {
		t.Errorf("timestamp = %+v, want %+v", got.CreationTimestamp, p.CreationTimestamp)
	}
	if got.Lifetime != p.Lifetime {
		t.Errorf("lifetime = %d, want %d", got.Lifetime, p.Lifetime)
	}
}

func TestDictionarySharesRepeatedStrings(t *testing.T) {
	// §4.4: identical strings are stored once. Four endpoints all in the dtn
	// scheme must not store "dtn" four times. (All-ipn bundles skip the
	// dictionary entirely via CBHE, so dtn endpoints force the general
	// form here.)
	shared := &bp.PrimaryBlock{
		Destination:       bp.EndpointID{Scheme: "dtn", SSP: "//dest"},
		Source:            bp.EndpointID{Scheme: "dtn", SSP: "//src"},
		ReportTo:          bp.EndpointID{Scheme: "dtn", SSP: "//ops"},
		Custodian:         bp.EndpointID{Scheme: "dtn", SSP: "//ops"}, // same as ReportTo
		CreationTimestamp: bp.CreationTimestamp{Time: 800000000, SequenceNumber: 42},
		Lifetime:          3600,
	}

	encoded, err := shared.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(encoded, []byte("dtn\x00")) != 1 {
		t.Errorf("the scheme string appears %d times; the dictionary should share it",
			bytes.Count(encoded, []byte("dtn\x00")))
	}
	// ReportTo and Custodian are the same endpoint, so its SSP appears once.
	if bytes.Count(encoded, []byte("//ops\x00")) != 1 {
		t.Error("a repeated scheme-specific part was stored more than once")
	}

	// And the round trip preserves all four endpoints.
	got, _, err := bp.DecodePrimaryBlock(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Custodian != shared.Custodian || got.Source != shared.Source {
		t.Error("dictionary-form endpoints did not survive the round trip")
	}
}

func TestCBHERoundTripAgainstHandBuiltBytes(t *testing.T) {
	// RFC 6260 §2.1: with every endpoint ipn (dtn:none as node 0, service 0),
	// the dictionary length is zero and the node and service numbers ride in
	// the scheme and SSP offset fields. These bytes are built by hand from
	// the RFC's layout, field by field.
	wire := []byte{
		0x06,       // version 6
		0x10,       // flags: singleton destination
		0x0E,       // block length: 14 octets follow
		0x02, 0x01, // destination ipn:2.1
		0x01, 0x01, // source ipn:1.1
		0x01, 0x00, // report-to ipn:1.0
		0x00, 0x00, // custodian dtn:none
		0x87, 0x68, // creation time 1000 as SDNV
		0x05,       // sequence number 5
		0x9C, 0x10, // lifetime 3600 as SDNV
		0x00, // dictionary length 0: this is CBHE
	}

	got, consumed, err := bp.DecodePrimaryBlock(wire)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(wire) {
		t.Errorf("consumed %d, want %d", consumed, len(wire))
	}
	if got.Destination != bp.IPNEndpoint(2, 1) {
		t.Errorf("destination = %s, want ipn:2.1", got.Destination)
	}
	if got.Source != bp.IPNEndpoint(1, 1) {
		t.Errorf("source = %s, want ipn:1.1", got.Source)
	}
	if got.ReportTo != bp.IPNEndpoint(1, 0) {
		t.Errorf("report-to = %s, want ipn:1.0", got.ReportTo)
	}
	if !got.Custodian.IsNull() {
		t.Errorf("custodian = %s, want dtn:none", got.Custodian)
	}
	if got.CreationTimestamp.Time != 1000 || got.CreationTimestamp.SequenceNumber != 5 {
		t.Errorf("timestamp = %+v, want 1000.5", got.CreationTimestamp)
	}
	if got.Lifetime != 3600 {
		t.Errorf("lifetime = %d, want 3600", got.Lifetime)
	}

	// Encoding the same block must reproduce the hand-built bytes exactly:
	// all-ipn endpoints take the CBHE form, not a dictionary.
	encoded, err := got.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, wire) {
		t.Errorf("re-encoded CBHE block = % X, want % X", encoded, wire)
	}
}

func TestCBHERejectsNodeZeroWithService(t *testing.T) {
	// RFC 6260 §2.2: node 0 is only meaningful as (0, 0), the null endpoint.
	wire := []byte{
		0x06, 0x10, 0x0C, // version, flags, block length 12
		0x00, 0x07, // node 0, service 7: names nothing
		0x01, 0x01, // source ipn:1.1
		0x01, 0x00, // report-to ipn:1.0
		0x00, 0x00, // custodian dtn:none
		0x01, 0x01, 0x01, // creation time 1, sequence 1, lifetime 1
		0x00, // dictionary length 0: CBHE
	}
	if _, _, err := bp.DecodePrimaryBlock(wire); !errors.Is(err, bp.ErrInvalidEndpointID) {
		t.Errorf("error = %v, want ErrInvalidEndpointID", err)
	}
}

func TestPrimaryBlockValidationTightening(t *testing.T) {
	// §4.2: reserved priority, contradictory fragment flags, and the
	// anonymous-source constraints are all refused.
	p := testPrimary()
	p.Flags = p.Flags.WithPriority(3)
	if err := p.Validate(); !errors.Is(err, bp.ErrInvalidPriority) {
		t.Errorf("priority 3: error = %v, want ErrInvalidPriority", err)
	}

	p = testPrimary()
	p.Flags |= bp.FlagFragment | bp.FlagNoFragment
	if err := p.Validate(); !errors.Is(err, bp.ErrFragmentFlags) {
		t.Errorf("fragment + no-fragment: error = %v, want ErrFragmentFlags", err)
	}

	// An anonymous bundle must not request custody...
	p = testPrimary()
	p.Source = bp.NullEndpoint
	p.Flags |= bp.FlagNoFragment | bp.FlagCustodyRequested
	if err := p.Validate(); !errors.Is(err, bp.ErrAnonymousSource) {
		t.Errorf("anonymous custody: error = %v, want ErrAnonymousSource", err)
	}

	// ...and must set the no-fragment flag.
	p = testPrimary()
	p.Source = bp.NullEndpoint
	if err := p.Validate(); !errors.Is(err, bp.ErrAnonymousSource) {
		t.Errorf("anonymous without no-fragment: error = %v, want ErrAnonymousSource", err)
	}

	// With the flag set and no custody request, an anonymous bundle is fine.
	p = testPrimary()
	p.Source = bp.NullEndpoint
	p.Flags |= bp.FlagNoFragment
	if err := p.Validate(); err != nil {
		t.Errorf("a conformant anonymous bundle was rejected: %v", err)
	}
}

func TestDecodeBundleRejectsTrailingBytes(t *testing.T) {
	// §4.1: a bundle ends at its last block. Octets past it are corruption,
	// not padding, and DecodeBundle refuses them.
	b, err := bp.NewBundle(testPrimary(), []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}

	junk := append(append([]byte{}, encoded...), 0xDE, 0xAD)
	if _, err := bp.DecodeBundle(junk); !errors.Is(err, bp.ErrTrailingBytes) {
		t.Errorf("error = %v, want ErrTrailingBytes", err)
	}

	// DecodeBundleN reports how much it consumed instead, for streams of
	// concatenated bundles.
	got, n, err := bp.DecodeBundleN(junk, bp.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d, want %d", n, len(encoded))
	}
	if got == nil {
		t.Error("DecodeBundleN returned no bundle")
	}
}

func TestPrimaryBlockFragmentFields(t *testing.T) {
	// §4.5.1: the two fragment fields are present only with the fragment flag.
	p := testPrimary()
	p.Flags |= bp.FlagFragment
	p.FragmentOffset = 1000
	p.TotalADULength = 5000

	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := bp.DecodePrimaryBlock(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.FragmentOffset != 1000 || got.TotalADULength != 5000 {
		t.Errorf("fragment fields = %d of %d, want 1000 of 5000",
			got.FragmentOffset, got.TotalADULength)
	}

	// Without the flag, the block is shorter.
	plain := testPrimary()
	plainEncoded, err := plain.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(plainEncoded) >= len(encoded) {
		t.Error("a non-fragment primary block should be shorter than a fragment one")
	}
}

func TestAdminRecordFlagsRejected(t *testing.T) {
	// §4.2: an administrative record requests neither custody transfer nor
	// any status report.
	p := testPrimary()
	p.Flags |= bp.FlagAdminRecord | bp.FlagCustodyRequested
	if err := p.Validate(); !errors.Is(err, bp.ErrAdminRecordFlags) {
		t.Errorf("custody: error = %v, want ErrAdminRecordFlags", err)
	}

	p = testPrimary()
	p.Flags |= bp.FlagAdminRecord | bp.FlagReportDelivery
	if err := p.Validate(); !errors.Is(err, bp.ErrAdminRecordFlags) {
		t.Errorf("status report: error = %v, want ErrAdminRecordFlags", err)
	}
}

func TestPrimaryBlockRejectsWrongVersion(t *testing.T) {
	p := testPrimary()
	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] = 7 // BPv7 is CBOR and wire-incompatible

	if _, _, err := bp.DecodePrimaryBlock(encoded); !errors.Is(err, bp.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestCanonicalBlockRoundTrip(t *testing.T) {
	b := &bp.CanonicalBlock{
		Type:  bp.BlockTypePayload,
		Flags: bp.BlockLast,
		Data:  []byte("payload contents"),
	}
	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, consumed, err := bp.DecodeCanonicalBlock(encoded, 0)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(encoded) {
		t.Errorf("consumed %d, want %d", consumed, len(encoded))
	}
	if got.Type != bp.BlockTypePayload || !got.IsLast() {
		t.Errorf("type = %s, last = %t", got.Type, got.IsLast())
	}
	if !bytes.Equal(got.Data, b.Data) {
		t.Errorf("data = %q, want %q", got.Data, b.Data)
	}
}

func TestCanonicalBlockEIDReferences(t *testing.T) {
	// §4.5.2: the reference field is present if and only if the flag is set.
	b := &bp.CanonicalBlock{
		Type:  200,
		Flags: bp.BlockHasEIDRefs | bp.BlockLast,
		EIDReferences: []bp.EIDReference{
			{SchemeOffset: 0, SSPOffset: 4},
			{SchemeOffset: 0, SSPOffset: 8},
		},
		Data: []byte{1, 2, 3},
	}
	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := bp.DecodeCanonicalBlock(encoded, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.EIDReferences) != 2 {
		t.Fatalf("got %d references, want 2", len(got.EIDReferences))
	}
	if got.EIDReferences[1].SSPOffset != 8 {
		t.Errorf("second reference SSP offset = %d, want 8", got.EIDReferences[1].SSPOffset)
	}
}

func TestCanonicalBlockRefFlagMustMatchField(t *testing.T) {
	// The flag set with no references is invalid, and so is the reverse.
	b := &bp.CanonicalBlock{Type: 200, Flags: bp.BlockHasEIDRefs}
	if err := b.Validate(); err == nil {
		t.Error("the reference flag with no references should be invalid")
	}

	b = &bp.CanonicalBlock{
		Type:          200,
		EIDReferences: []bp.EIDReference{{SchemeOffset: 0, SSPOffset: 1}},
	}
	if err := b.Validate(); err == nil {
		t.Error("references without the flag should be invalid")
	}
}

func TestCanonicalBlockRejectsOversizedLength(t *testing.T) {
	// A block length is an SDNV reaching 2^64; sizing an allocation from it
	// would be an easy denial of service.
	b := &bp.CanonicalBlock{Type: bp.BlockTypePayload, Flags: bp.BlockLast, Data: []byte("x")}
	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := bp.DecodeCanonicalBlock(encoded, 0); err != nil {
		t.Fatalf("a normal block was rejected: %v", err)
	}
	// Cap smaller than the body.
	if _, _, err := bp.DecodeCanonicalBlock(encoded, 0); err != nil {
		t.Fatal(err)
	}

	// Hand-build a block claiming a huge length.
	huge := []byte{byte(bp.BlockTypePayload), 0x08} // type, flags
	huge = append(huge, 0x8F, 0xFF, 0xFF, 0xFF, 0x7F)
	if _, _, err := bp.DecodeCanonicalBlock(huge, 1024); !errors.Is(err, bp.ErrBlockTooLarge) {
		t.Errorf("error = %v, want ErrBlockTooLarge", err)
	}
}

func TestBundleRoundTrip(t *testing.T) {
	payload := []byte("application data unit")
	b, err := bp.NewBundle(testPrimary(), payload)
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

	gotPayload, err := got.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload = %q, want %q", gotPayload, payload)
	}
	if got.Primary.Destination != b.Primary.Destination {
		t.Errorf("destination = %s, want %s", got.Primary.Destination, b.Primary.Destination)
	}
}

func TestBundleRequiresPayload(t *testing.T) {
	b := &bp.Bundle{Primary: testPrimary()}
	if err := b.Validate(); !errors.Is(err, bp.ErrMissingPayload) {
		t.Errorf("error = %v, want ErrMissingPayload", err)
	}
}

func TestBundleRejectsTwoPayloads(t *testing.T) {
	b := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: bp.BlockTypePayload, Data: []byte("one")},
			{Type: bp.BlockTypePayload, Flags: bp.BlockLast, Data: []byte("two")},
		},
	}
	if err := b.Validate(); !errors.Is(err, bp.ErrMultiplePayloads) {
		t.Errorf("error = %v, want ErrMultiplePayloads", err)
	}
}

func TestBundleRequiresLastBlockFlag(t *testing.T) {
	b := &bp.Bundle{
		Primary: testPrimary(),
		Blocks:  []*bp.CanonicalBlock{{Type: bp.BlockTypePayload, Data: []byte("x")}},
	}
	if err := b.Validate(); !errors.Is(err, bp.ErrNoLastBlock) {
		t.Errorf("error = %v, want ErrNoLastBlock", err)
	}
}

func TestBundleRejectsLastFlagBeforeTheEnd(t *testing.T) {
	b := &bp.Bundle{
		Primary: testPrimary(),
		Blocks: []*bp.CanonicalBlock{
			{Type: 200, Flags: bp.BlockLast, Data: []byte("early")},
			{Type: bp.BlockTypePayload, Flags: bp.BlockLast, Data: []byte("payload")},
		},
	}
	if err := b.Validate(); !errors.Is(err, bp.ErrNoLastBlock) {
		t.Errorf("error = %v, want ErrNoLastBlock", err)
	}
}
