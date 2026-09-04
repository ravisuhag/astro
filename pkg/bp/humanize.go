package bp

import (
	"fmt"
	"strings"
)

// Humanize returns a human-readable summary of the bundle.
func (b *Bundle) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "BPv7 Bundle\n")
	fmt.Fprintf(&sb, "  Source .......... %s\n", b.Primary.Source)
	fmt.Fprintf(&sb, "  Destination ..... %s\n", b.Primary.Destination)
	fmt.Fprintf(&sb, "  Report-to ....... %s\n", b.Primary.ReportTo)
	fmt.Fprintf(&sb, "  Created ......... %s\n", b.Primary.Timestamp.Humanize())
	fmt.Fprintf(&sb, "  Lifetime ........ %d ms\n", b.Primary.Lifetime)
	fmt.Fprintf(&sb, "  Flags ........... %s\n", b.Primary.Flags.Humanize())
	fmt.Fprintf(&sb, "  Primary CRC ..... %s\n", b.Primary.CRCType.Humanize())

	if b.Primary.Flags.Has(FlagIsFragment) {
		fmt.Fprintf(&sb, "  Fragment ........ offset %d of %d octets\n",
			b.Primary.FragmentOffset, b.Primary.TotalADULength)
	}

	fmt.Fprintf(&sb, "  Blocks .......... %d", len(b.Blocks))
	for _, blk := range b.Blocks {
		fmt.Fprintf(&sb, "\n    %s", blk.Humanize())
	}
	return sb.String()
}

// Humanize returns a human-readable summary of the block.
func (b *CanonicalBlock) Humanize() string {
	var detail string
	switch b.Type {
	case BlockTypePayload:
		detail = fmt.Sprintf("%d octets", len(b.Data))
	case BlockTypeBundleAge:
		if age, err := b.BundleAge(); err == nil {
			detail = fmt.Sprintf("%d ms", age)
		}
	case BlockTypePreviousNode:
		if node, err := b.PreviousNode(); err == nil {
			detail = node.String()
		}
	case BlockTypeHopCount:
		if limit, count, err := b.HopCount(); err == nil {
			detail = fmt.Sprintf("%d of %d", count, limit)
		}
	}
	if detail == "" {
		detail = fmt.Sprintf("%d octets", len(b.Data))
	}

	return fmt.Sprintf("#%d %s: %s [%s]", b.Number, b.Type.Humanize(), detail, b.CRCType.Humanize())
}

// Humanize names the block type.
func (t BlockType) Humanize() string {
	switch t {
	case BlockTypePayload:
		return "Payload"
	case BlockTypePreviousNode:
		return "Previous Node"
	case BlockTypeBundleAge:
		return "Bundle Age"
	case BlockTypeHopCount:
		return "Hop Count"
	}
	if t >= firstPrivateBlockType {
		return fmt.Sprintf("private type %d", uint64(t))
	}
	return fmt.Sprintf("unassigned type %d", uint64(t))
}

// Humanize names the checksum.
func (c CRCType) Humanize() string {
	switch c {
	case CRCNone:
		return "no CRC"
	case CRC16X25:
		return "X-25 CRC-16"
	case CRC32C:
		return "CRC-32C"
	}
	return fmt.Sprintf("undefined CRC type %d", uint64(c))
}

// Humanize lists the flags that are set, or says none are.
func (f BundleControlFlags) Humanize() string {
	named := []struct {
		flag BundleControlFlags
		name string
	}{
		{FlagIsFragment, "fragment"},
		{FlagAdminRecord, "admin record"},
		{FlagMustNotFragment, "must not fragment"},
		{FlagAppAckRequested, "app ack requested"},
		{FlagStatusTimeRequested, "status time requested"},
		{FlagReportReception, "report reception"},
		{FlagReportForwarding, "report forwarding"},
		{FlagReportDelivery, "report delivery"},
		{FlagReportDeletion, "report deletion"},
	}

	var set []string
	for _, n := range named {
		if f.Has(n.flag) {
			set = append(set, n.name)
		}
	}
	if len(set) == 0 {
		return "none"
	}
	return strings.Join(set, ", ")
}

// Humanize describes the creation timestamp, spelling out what a zero time
// means rather than printing the DTN epoch as though it were a real date.
func (t CreationTimestamp) Humanize() string {
	if t.Time == DTNTimeUnknown {
		return fmt.Sprintf("time unknown, sequence %d", t.Sequence)
	}
	return fmt.Sprintf("%s, sequence %d", t.Time.Time().Format("2006-01-02T15:04:05.000Z"), t.Sequence)
}

// Humanize returns a human-readable summary of the status report.
func (r *StatusReport) Humanize() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "BPv7 Bundle Status Report\n")
	fmt.Fprintf(&sb, "  Subject ......... %s, %s\n", r.SubjectSource, r.SubjectTimestamp.Humanize())
	if r.SubjectIsFragment {
		fmt.Fprintf(&sb, "  Subject fragment  offset %d, %d octets\n",
			r.SubjectFragmentOffset, r.SubjectPayloadLength)
	}
	fmt.Fprintf(&sb, "  Reason .......... %s", r.Reason.Humanize())

	for _, s := range []struct {
		name string
		item StatusItem
	}{
		{"Received", r.Received},
		{"Forwarded", r.Forwarded},
		{"Delivered", r.Delivered},
		{"Deleted", r.Deleted},
	} {
		if !s.item.Asserted {
			continue
		}
		fmt.Fprintf(&sb, "\n  %-15s asserted", s.name)
		if s.item.Time != DTNTimeUnknown {
			fmt.Fprintf(&sb, " at %s", s.item.Time.Time().Format("2006-01-02T15:04:05.000Z"))
		}
	}
	return sb.String()
}

// Humanize names the reason code.
func (r StatusReportReason) Humanize() string {
	switch r {
	case ReasonNoAdditionalInformation:
		return "no additional information"
	case ReasonLifetimeExpired:
		return "lifetime expired"
	case ReasonForwardedUnidirectional:
		return "forwarded over a unidirectional link"
	case ReasonTransmissionCanceled:
		return "transmission canceled"
	case ReasonDepletedStorage:
		return "depleted storage"
	case ReasonDestinationUnavailable:
		return "destination endpoint ID unavailable"
	case ReasonNoKnownRoute:
		return "no known route to the destination from here"
	case ReasonNoTimelyContact:
		return "no timely contact with the next node on the route"
	case ReasonBlockUnintelligible:
		return "block unintelligible"
	case ReasonHopLimitExceeded:
		return "hop limit exceeded"
	case ReasonTrafficPared:
		return "traffic pared"
	case ReasonBlockUnsupported:
		return "block unsupported"
	}
	return fmt.Sprintf("reason code %d", uint64(r))
}
