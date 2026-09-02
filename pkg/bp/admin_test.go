package bp_test

import (
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
)

func TestStatusReportRoundTrip(t *testing.T) {
	// Clause 6.1.1: a time field rides only on its matching status flag.
	s := &bp.StatusReport{
		Flags:             bp.StatusReceived | bp.StatusDelivered,
		Reason:            bp.ReasonNoInformation,
		ReceiptTime:       bp.DTNTime{Seconds: 800000000, Nanoseconds: 500},
		DeliveryTime:      bp.DTNTime{Seconds: 800000100, Nanoseconds: 0},
		CreationTimestamp: bp.CreationTimestamp{Time: 799999999, SequenceNumber: 3},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}

	record := bp.NewStatusReportRecord(s)
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := bp.DecodeAdminRecord(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != bp.RecordStatusReport {
		t.Fatalf("type = %s, want status report", got.Type)
	}
	r := got.StatusReport
	if r.Flags != s.Flags {
		t.Errorf("flags = %#02x, want %#02x", r.Flags, s.Flags)
	}
	if r.ReceiptTime != s.ReceiptTime {
		t.Errorf("receipt time = %+v, want %+v", r.ReceiptTime, s.ReceiptTime)
	}
	if r.DeliveryTime != s.DeliveryTime {
		t.Errorf("delivery time = %+v, want %+v", r.DeliveryTime, s.DeliveryTime)
	}
	// Times whose flags are clear must stay zero.
	if r.CustodyTime != (bp.DTNTime{}) {
		t.Errorf("custody time = %+v, want zero for an unset flag", r.CustodyTime)
	}
	if r.SourceEndpoint != s.SourceEndpoint {
		t.Errorf("source = %s, want %s", r.SourceEndpoint, s.SourceEndpoint)
	}
}

func TestStatusReportOnlyEncodesFlaggedTimes(t *testing.T) {
	// A report with one flag must be shorter than one with five.
	base := bp.StatusReport{
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}

	one := base
	one.Flags = bp.StatusReceived
	oneEncoded, err := one.Encode()
	if err != nil {
		t.Fatal(err)
	}

	all := base
	all.Flags = bp.StatusReceived | bp.StatusCustodyAccepted | bp.StatusForwarded |
		bp.StatusDelivered | bp.StatusDeleted
	allEncoded, err := all.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if len(oneEncoded) >= len(allEncoded) {
		t.Errorf("one flag encoded %d octets, five encoded %d; the first must be shorter",
			len(oneEncoded), len(allEncoded))
	}
}

func TestStatusReportForFragment(t *testing.T) {
	// Clause 6.1, figure 9: the fragment flag brings the offset and length fields.
	s := &bp.StatusReport{
		Flags:             bp.StatusReceived,
		IsFragment:        true,
		FragmentOffset:    500,
		FragmentLength:    250,
		ReceiptTime:       bp.DTNTime{Seconds: 1},
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	record := bp.NewStatusReportRecord(s)
	if record.Flags&bp.RecordFlagFragment == 0 {
		t.Error("the record flag was not set for a fragment report")
	}

	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := bp.DecodeAdminRecord(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusReport.FragmentOffset != 500 || got.StatusReport.FragmentLength != 250 {
		t.Errorf("fragment = offset %d length %d, want 500/250",
			got.StatusReport.FragmentOffset, got.StatusReport.FragmentLength)
	}
}

func TestCustodySignalRoundTrip(t *testing.T) {
	for _, succeeded := range []bool{true, false} {
		c := &bp.CustodySignal{
			Succeeded:         succeeded,
			Reason:            bp.ReasonDepletedStorage,
			SignalTime:        bp.DTNTime{Seconds: 800000000, Nanoseconds: 1},
			CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 2},
			SourceEndpoint:    bp.IPNEndpoint(3, 1),
		}
		record := bp.NewCustodySignalRecord(c)
		encoded, err := record.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := bp.DecodeAdminRecord(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if got.Type != bp.RecordCustodySignal {
			t.Fatalf("type = %s, want custody signal", got.Type)
		}
		if got.CustodySignal.Succeeded != succeeded {
			t.Errorf("succeeded = %t, want %t", got.CustodySignal.Succeeded, succeeded)
		}
		if got.CustodySignal.Reason != bp.ReasonDepletedStorage {
			t.Errorf("reason = %s, want depleted storage", got.CustodySignal.Reason)
		}
	}
}

func TestAdminRecordTypeAndFlagsNibbles(t *testing.T) {
	// Clause 6.1: a four-bit type code then four bits of flags.
	c := &bp.CustodySignal{
		SignalTime:        bp.DTNTime{Seconds: 1},
		CreationTimestamp: bp.CreationTimestamp{Time: 1, SequenceNumber: 1},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	encoded, err := bp.NewCustodySignalRecord(c).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := bp.RecordType(encoded[0] >> 4); got != bp.RecordCustodySignal {
		t.Errorf("type nibble = %d, want %d", got, bp.RecordCustodySignal)
	}
	if encoded[0]&0x0F != 0 {
		t.Errorf("flags nibble = %#x, want 0 for a non-fragment record", encoded[0]&0x0F)
	}
}

func TestAdminRecordRejectsUnknownType(t *testing.T) {
	if _, err := bp.DecodeAdminRecord([]byte{0xF0, 0, 0}); !errors.Is(err, bp.ErrInvalidRecordType) {
		t.Errorf("error = %v, want ErrInvalidRecordType", err)
	}
}

func TestBundleAdminRecordRequiresFlag(t *testing.T) {
	// Reading a payload as an administrative record only makes sense when
	// the bundle says it is one.
	b, err := bp.NewBundle(testPrimary(), []byte{0x10, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.AdminRecord(); !errors.Is(err, bp.ErrNotAdminRecord) {
		t.Errorf("error = %v, want ErrNotAdminRecord", err)
	}
}

func TestBundleCarryingStatusReport(t *testing.T) {
	s := &bp.StatusReport{
		Flags:             bp.StatusDelivered,
		Reason:            bp.ReasonNoInformation,
		DeliveryTime:      bp.DTNTime{Seconds: 800000000},
		CreationTimestamp: bp.CreationTimestamp{Time: 799999999, SequenceNumber: 7},
		SourceEndpoint:    bp.IPNEndpoint(1, 1),
	}
	payload, err := bp.NewStatusReportRecord(s).Encode()
	if err != nil {
		t.Fatal(err)
	}

	primary := testPrimary()
	primary.Flags |= bp.FlagAdminRecord

	b, err := bp.NewBundle(primary, payload)
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

	record, err := got.AdminRecord()
	if err != nil {
		t.Fatal(err)
	}
	if record.StatusReport == nil {
		t.Fatal("no status report in the record")
	}
	if !record.StatusReport.Flags.Has(bp.StatusDelivered) {
		t.Error("the delivered flag was lost")
	}
}
