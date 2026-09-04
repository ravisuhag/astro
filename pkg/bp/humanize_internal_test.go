package bp

import (
	"strings"
	"testing"
)

// Humanize output is what the CLI prints, so it is checked for the facts a
// reader needs rather than for exact formatting.
func TestBundleHumanize(t *testing.T) {
	hop, err := NewHopCountBlock(2, 32, 5)
	if err != nil {
		t.Fatalf("NewHopCountBlock: %v", err)
	}
	prev, err := NewPreviousNodeBlock(3, IPN(5, 0))
	if err != nil {
		t.Fatalf("NewPreviousNodeBlock: %v", err)
	}
	age, err := NewBundleAgeBlock(4, 300)
	if err != nil {
		t.Fatalf("NewBundleAgeBlock: %v", err)
	}

	primary := &PrimaryBlock{
		CRCType:     CRC32C,
		Destination: IPN(1, 2),
		Source:      DTN("//rover/science"),
		ReportTo:    IPN(2, 1),
		Timestamp:   CreationTimestamp{Time: 757382400000, Sequence: 3},
		Lifetime:    86400000,
		Flags:       FlagReportDelivery | FlagMustNotFragment,
	}
	b, err := NewBundle(primary, []byte("telemetry"), hop, prev, age)
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}

	out := b.Humanize()
	for _, want := range []string{
		"BPv7 Bundle",
		"dtn://rover/science",
		"ipn:1.2",
		"2024-01-01T00:00:00.000Z",
		"86400000 ms",
		"must not fragment",
		"report delivery",
		"CRC-32C",
		"Hop Count: 5 of 32",
		"Previous Node: ipn:5.0",
		"Bundle Age: 300 ms",
		"Payload: 9 octets",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q from:\n%s", want, out)
		}
	}
}

// A fragment says so, with the offset and total that let a reader place it.
func TestBundleHumanizeFragment(t *testing.T) {
	primary := &PrimaryBlock{
		Destination: IPN(1, 2), Source: IPN(2, 1), ReportTo: IPN(2, 1),
		Timestamp: CreationTimestamp{Time: 757382400000, Sequence: 1},
	}
	b, err := NewBundle(primary, []byte("abcdefghijklmnop"))
	if err != nil {
		t.Fatalf("NewBundle: %v", err)
	}
	fragments, err := b.Fragment(8)
	if err != nil {
		t.Fatalf("Fragment: %v", err)
	}

	out := fragments[1].Humanize()
	if !strings.Contains(out, "offset 8 of 16") {
		t.Errorf("missing the fragment position from:\n%s", out)
	}
}

// An unknown creation time must not print as the year 2000, which is what a
// naive conversion of zero would show.
func TestCreationTimestampHumanizeUnknownTime(t *testing.T) {
	got := CreationTimestamp{Time: DTNTimeUnknown, Sequence: 40}.Humanize()
	if !strings.Contains(got, "time unknown") {
		t.Errorf("got %q, want it to say the time is unknown", got)
	}
	if strings.Contains(got, "2000") {
		t.Errorf("got %q, which prints the epoch as though it were a real date", got)
	}
}

func TestHumanizeNames(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{BlockTypePayload.Humanize(), "Payload"},
		{BlockTypeHopCount.Humanize(), "Hop Count"},
		{BlockType(200).Humanize(), "private type 200"},
		{BlockType(50).Humanize(), "unassigned type 50"},
		{CRCNone.Humanize(), "no CRC"},
		{CRC16X25.Humanize(), "X-25 CRC-16"},
		{CRCType(9).Humanize(), "undefined CRC type 9"},
		{BundleControlFlags(0).Humanize(), "none"},
		{ReasonHopLimitExceeded.Humanize(), "hop limit exceeded"},
		{StatusReportReason(200).Humanize(), "reason code 200"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestStatusReportHumanize(t *testing.T) {
	r := &StatusReport{
		Received:              StatusItem{Asserted: true, Time: 757382400000},
		Delivered:             StatusItem{Asserted: true},
		Reason:                ReasonLifetimeExpired,
		SubjectSource:         IPN(2, 1),
		SubjectTimestamp:      CreationTimestamp{Time: 757382000000, Sequence: 2},
		SubjectIsFragment:     true,
		SubjectFragmentOffset: 2048,
		SubjectPayloadLength:  1024,
		IncludeTime:           true,
	}

	out := r.Humanize()
	for _, want := range []string{
		"BPv7 Bundle Status Report",
		"ipn:2.1",
		"lifetime expired",
		"offset 2048, 1024 octets",
		"Received",
		"2024-01-01T00:00:00.000Z",
		"Delivered",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q from:\n%s", want, out)
		}
	}

	// Statuses that were not asserted are left out entirely, rather than
	// listed as false.
	if strings.Contains(out, "Forwarded") {
		t.Errorf("an unasserted status was listed:\n%s", out)
	}
}

// A block whose data cannot be parsed as its type still describes itself,
// falling back to an octet count rather than printing nothing.
func TestCanonicalBlockHumanizeFallsBack(t *testing.T) {
	broken := &CanonicalBlock{Type: BlockTypeHopCount, Number: 2, Data: []byte{0xFF}}
	if got := broken.Humanize(); !strings.Contains(got, "1 octets") {
		t.Errorf("got %q, want an octet count fallback", got)
	}
}
