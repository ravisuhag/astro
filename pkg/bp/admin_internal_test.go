package bp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

// A minimal status report: delivery asserted, nothing else, no times, subject
// not a fragment. Built by hand from clauses 6.1 and 6.1.1 so the shape is
// pinned rather than round-tripped.
//
//	[1, [                       / admin record type 1        /
//	  [[false],[false],[true],[false]],  / the four assertions        /
//	  0,                                 / reason: no extra info      /
//	  [2, [2, 1]],                       / subject source ipn:2.1     /
//	  [0, 40]                            / subject creation timestamp /
//	]]
const statusReportDelivered = "82" + "01" +
	"84" +
	"84" + "81f4" + "81f4" + "81f5" + "81f4" +
	"00" +
	"8202820201" +
	"82001828"

func TestStatusReportEncoding(t *testing.T) {
	r := &StatusReport{
		Delivered:        StatusItem{Asserted: true},
		Reason:           ReasonNoAdditionalInformation,
		SubjectSource:    IPN(2, 1),
		SubjectTimestamp: CreationTimestamp{Time: DTNTimeUnknown, Sequence: 40},
	}

	got, err := r.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if hex.EncodeToString(got) != statusReportDelivered {
		t.Fatalf("\n got %s\nwant %s", hex.EncodeToString(got), statusReportDelivered)
	}

	back, err := DecodeStatusReport(got)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if *back != *r {
		t.Errorf("round trip = %+v, want %+v", *back, *r)
	}
}

// Clause 6.1.1: a status item is two elements only when the status is asserted
// AND the subject bundle asked for status times. Every other combination is
// one element, so an unasserted status never carries a time.
func TestStatusItemLength(t *testing.T) {
	tests := []struct {
		name        string
		item        StatusItem
		includeTime bool
		want        string
	}{
		{"not asserted, times not wanted", StatusItem{}, false, "81f4"},
		{"not asserted, times wanted", StatusItem{}, true, "81f4"},
		{"asserted, times not wanted", StatusItem{Asserted: true, Time: 1000}, false, "81f5"},
		{"asserted, times wanted", StatusItem{Asserted: true, Time: 1000}, true, "82f51903e8"},
	}

	for _, tt := range tests {
		got := hex.EncodeToString(appendStatusItem(nil, tt.item, tt.includeTime))
		if got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, got, tt.want)
		}
	}
}

// B8: DTNTimeUnknown (zero) is a legitimate time from a clockless node, so a
// status item's Time cannot tell "asserted with a time" apart from "asserted
// with no time wanted" -- only the arity can. IncludeTime must survive a
// round trip even when every asserted status carries time zero.
func TestStatusReportIncludeTimeSurvivesAllZeroTimes(t *testing.T) {
	r := &StatusReport{
		Received:         StatusItem{Asserted: true, Time: DTNTimeUnknown},
		Forwarded:        StatusItem{Asserted: true, Time: DTNTimeUnknown},
		Delivered:        StatusItem{Asserted: true, Time: DTNTimeUnknown},
		Deleted:          StatusItem{Asserted: true, Time: DTNTimeUnknown},
		Reason:           ReasonNoAdditionalInformation,
		SubjectSource:    IPN(2, 1),
		SubjectTimestamp: CreationTimestamp{Time: DTNTimeUnknown, Sequence: 40},
		IncludeTime:      true,
	}

	encoded, err := r.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Byte 3 opens the four-assertions array; each status item follows as
	// [true, 0] -- three octets, 0x82 0xf5 0x00 -- rather than collapsing to
	// the one-octet form [true] a value-based IncludeTime would produce.
	for i := 0; i < 4; i++ {
		item := encoded[4+i*3 : 4+i*3+3]
		if !bytes.Equal(item, []byte{0x82, 0xf5, 0x00}) {
			t.Fatalf("status item %d = % x, want 82 f5 00", i, item)
		}
	}

	back, err := DecodeStatusReport(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !back.IncludeTime {
		t.Error("IncludeTime was lost when every asserted time was zero")
	}
	if *back != *r {
		t.Errorf("round trip = %+v, want %+v", *back, *r)
	}
}

func TestStatusReportWithTimesAndFragment(t *testing.T) {
	r := &StatusReport{
		Received:              StatusItem{Asserted: true, Time: 757382400000},
		Forwarded:             StatusItem{Asserted: true, Time: 757382401000},
		Deleted:               StatusItem{Asserted: true, Time: 757382402000},
		Reason:                ReasonLifetimeExpired,
		SubjectSource:         DTN("//sender/app"),
		SubjectTimestamp:      CreationTimestamp{Time: 757382000000, Sequence: 2},
		SubjectIsFragment:     true,
		SubjectFragmentOffset: 2048,
		SubjectPayloadLength:  1024,
		IncludeTime:           true,
	}

	encoded, err := r.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := DecodeStatusReport(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if *back != *r {
		t.Errorf("round trip = %+v\nwant        %+v", *back, *r)
	}

	// Six elements for a fragment, four otherwise. The subject array head is
	// the third octet: 0x82 0x01 then the report array.
	if encoded[2] != 0x86 {
		t.Errorf("report array head = 0x%02X, want 0x86 for a fragment subject", encoded[2])
	}
}

// A status report travels as the payload of a bundle flagged as an
// administrative record, and clause 4.2.3 forbids that bundle from requesting
// reports about itself.
func TestStatusReportBundle(t *testing.T) {
	report := &StatusReport{
		Delivered:        StatusItem{Asserted: true},
		SubjectSource:    IPN(2, 1),
		SubjectTimestamp: CreationTimestamp{Time: 757382400000, Sequence: 40},
	}
	primary := &PrimaryBlock{
		CRCType:     CRC32C,
		Destination: IPN(2, 1), // the subject's report-to endpoint
		Source:      IPN(1, 0),
		ReportTo:    IPN(1, 0),
		Timestamp:   CreationTimestamp{Time: 757382500000, Sequence: 1},
		Lifetime:    3600000,
	}

	b, err := NewStatusReportBundle(primary, report)
	if err != nil {
		t.Fatalf("NewStatusReportBundle: %v", err)
	}
	if !b.Primary.Flags.Has(FlagAdminRecord) {
		t.Error("the administrative record flag was not set")
	}

	encoded, err := b.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := back.StatusReport()
	if err != nil {
		t.Fatalf("StatusReport: %v", err)
	}
	if *got != *report {
		t.Errorf("report = %+v, want %+v", *got, *report)
	}

	// Asking for reports about a report is refused, by clause 4.2.3.
	primary.Flags |= FlagReportDelivery
	if _, err := b.Encode(); !errors.Is(err, ErrAdminRecordWantsReports) {
		t.Errorf("err = %v, want ErrAdminRecordWantsReports", err)
	}
}

func TestStatusReportRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  error
	}{
		{"record array of one item", "8101", ErrMalformedAdminRecord},
		{"unknown record type", "8202" + "84" + "84" + "81f4" + "81f4" + "81f4" + "81f4" + "00" + "8202820201" + "82001828", ErrUnknownAdminRecordType},
		{"report array of five elements", "8201" + "85" + "84" + "81f4" + "81f4" + "81f5" + "81f4" + "00" + "8202820201" + "82001828" + "00", ErrMalformedStatusReport},
		{"only three assertions", "8201" + "84" + "83" + "81f4" + "81f4" + "81f5" + "00" + "8202820201" + "82001828", ErrMalformedStatusReport},
		{"a time on an unasserted status", "8201" + "84" + "84" + "82f41903e8" + "81f4" + "81f4" + "81f4" + "00" + "8202820201" + "82001828", ErrStatusTimeWithoutAssertion},
	}

	for _, tt := range tests {
		if _, err := DecodeStatusReport(mustHex(t, tt.input)); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}

	// Reading a report from a bundle that is not carrying one.
	primary := &PrimaryBlock{
		Destination: IPN(1, 2), Source: IPN(2, 1), ReportTo: IPN(2, 1),
		Timestamp: CreationTimestamp{Time: 1000, Sequence: 1},
	}
	b, err := NewBundle(primary, []byte("ordinary payload"))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	if _, err := b.StatusReport(); !errors.Is(err, ErrNotAnAdminRecord) {
		t.Errorf("err = %v, want ErrNotAnAdminRecord", err)
	}
}

// B9: a bundle with no primary block must report ErrNoPrimaryBlock, matching
// every sibling method, rather than panicking on a nil dereference.
func TestStatusReportNilPrimaryBlock(t *testing.T) {
	if _, err := (&Bundle{}).StatusReport(); !errors.Is(err, ErrNoPrimaryBlock) {
		t.Errorf("err = %v, want ErrNoPrimaryBlock", err)
	}
}
