package bp

import "github.com/ravisuhag/astro/internal/cbor"

// AdminRecordType names a kind of administrative record
// (RFC 9171 clause 6.1). Only one is defined for version 7.
type AdminRecordType uint64

// AdminRecordStatusReport is the bundle status report, the only
// administrative record RFC 9171 defines.
const AdminRecordStatusReport AdminRecordType = 1

// StatusReportReason explains why a status was asserted
// (RFC 9171 clause 6.1.1, table 1). The registry is open and other
// specifications add to it, so a code this package does not name still decodes.
type StatusReportReason uint64

const (
	ReasonNoAdditionalInformation StatusReportReason = 0
	ReasonLifetimeExpired         StatusReportReason = 1
	ReasonForwardedUnidirectional StatusReportReason = 2
	ReasonTransmissionCanceled    StatusReportReason = 3
	ReasonDepletedStorage         StatusReportReason = 4
	ReasonDestinationUnavailable  StatusReportReason = 5
	ReasonNoKnownRoute            StatusReportReason = 6
	ReasonNoTimelyContact         StatusReportReason = 7
	ReasonBlockUnintelligible     StatusReportReason = 8
	ReasonHopLimitExceeded        StatusReportReason = 9
	ReasonTrafficPared            StatusReportReason = 10
	ReasonBlockUnsupported        StatusReportReason = 11
)

// StatusItem is one of the four assertions a status report carries
// (RFC 9171 clause 6.1.1).
type StatusItem struct {
	// Asserted says whether this status happened.
	Asserted bool
	// Time is when it happened, by the reporting node's clock. It travels only
	// when Asserted is true and the subject bundle asked for status times.
	Time DTNTime
}

// StatusReport says how a bundle is progressing through the network
// (RFC 9171 clause 6.1.1). It is carried as the payload of another bundle,
// sent to the subject bundle's report-to endpoint.
//
// The four assertions come in a fixed order, and the report names its subject
// by source node ID and creation timestamp — the same pair that identifies a
// bundle everywhere else in the protocol.
type StatusReport struct {
	Received  StatusItem
	Forwarded StatusItem
	Delivered StatusItem
	Deleted   StatusItem

	Reason StatusReportReason

	// SubjectSource and SubjectTimestamp identify the bundle being reported on.
	SubjectSource    EID
	SubjectTimestamp CreationTimestamp

	// SubjectIsFragment says whether the subject bundle carried a fragment
	// offset. Clause 6.1.1 makes the last two elements present if and only if
	// it did.
	SubjectIsFragment     bool
	SubjectFragmentOffset uint64
	SubjectPayloadLength  uint64

	// IncludeTime mirrors the status-time flag on the subject bundle. Clause
	// 6.1.1 makes a status item two elements long only when its indicator is
	// true and that flag was set, so the encoder needs to know it. It is not
	// carried in the report itself; a reader recovers it from the items.
	IncludeTime bool
}

// appendStatusItem writes one assertion. Clause 6.1.1 fixes the length at two
// items only when the status is asserted and the subject asked for times, and
// at one otherwise.
func appendStatusItem(dst []byte, item StatusItem, includeTime bool) []byte {
	if item.Asserted && includeTime {
		dst = cbor.AppendArrayHeader(dst, 2)
		dst = cbor.AppendBool(dst, true)
		return cbor.AppendUint(dst, uint64(item.Time))
	}
	dst = cbor.AppendArrayHeader(dst, 1)
	return cbor.AppendBool(dst, item.Asserted)
}

// statusItem reads one assertion.
func decodeStatusItem(d *cbor.Decoder) (StatusItem, error) {
	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return StatusItem{}, err
	}
	if indefinite || n < 1 || n > 2 {
		return StatusItem{}, ErrMalformedStatusReport
	}

	asserted, err := d.Boolean()
	if err != nil {
		return StatusItem{}, err
	}
	item := StatusItem{Asserted: asserted}

	if n == 2 {
		// A time on an unasserted status is meaningless: clause 6.1.1 permits
		// the second element only when the indicator is 1.
		if !asserted {
			return StatusItem{}, ErrStatusTimeWithoutAssertion
		}
		t, err := d.Uint()
		if err != nil {
			return StatusItem{}, err
		}
		item.Time = DTNTime(t)
	}
	return item, nil
}

// Encode writes the status report as a complete administrative record: the
// two-item array of clause 6.1, carrying the report as its content.
//
// The result goes in the payload block of a bundle whose FlagAdminRecord is
// set. NewStatusReportBundle does that for you.
func (r *StatusReport) Encode() ([]byte, error) {
	if err := r.SubjectSource.Validate(); err != nil {
		return nil, err
	}

	// Clause 6.1: [record type code, record content].
	out := cbor.AppendArrayHeader(nil, 2)
	out = cbor.AppendUint(out, uint64(AdminRecordStatusReport))

	// Clause 6.1.1: six elements for a fragment, four otherwise.
	items := uint64(4)
	if r.SubjectIsFragment {
		items = 6
	}
	out = cbor.AppendArrayHeader(out, items)

	out = cbor.AppendArrayHeader(out, 4)
	for _, item := range []StatusItem{r.Received, r.Forwarded, r.Delivered, r.Deleted} {
		out = appendStatusItem(out, item, r.IncludeTime)
	}

	out = cbor.AppendUint(out, uint64(r.Reason))

	var err error
	if out, err = appendEID(out, r.SubjectSource); err != nil {
		return nil, err
	}
	out = appendCreationTimestamp(out, r.SubjectTimestamp)

	if r.SubjectIsFragment {
		out = cbor.AppendUint(out, r.SubjectFragmentOffset)
		out = cbor.AppendUint(out, r.SubjectPayloadLength)
	}
	return out, nil
}

// DecodeStatusReport reads an administrative record and returns the status
// report inside it.
func DecodeStatusReport(data []byte) (*StatusReport, error) {
	d := cbor.NewDecoder(data)

	n, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite || n != 2 {
		return nil, ErrMalformedAdminRecord
	}

	recordType, err := d.Uint()
	if err != nil {
		return nil, err
	}
	if AdminRecordType(recordType) != AdminRecordStatusReport {
		return nil, ErrUnknownAdminRecordType
	}

	items, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite || (items != 4 && items != 6) {
		return nil, ErrMalformedStatusReport
	}

	statusCount, indefinite, err := d.ArrayHeader()
	if err != nil {
		return nil, err
	}
	// Clause 6.1.1 says "at least four": later specifications may append more
	// assertions, and a reader takes the four it knows and skips the rest.
	if indefinite || statusCount < 4 {
		return nil, ErrMalformedStatusReport
	}

	r := &StatusReport{}
	for _, into := range []*StatusItem{&r.Received, &r.Forwarded, &r.Delivered, &r.Deleted} {
		item, err := decodeStatusItem(d)
		if err != nil {
			return nil, err
		}
		*into = item
		if item.Asserted && item.Time != 0 {
			r.IncludeTime = true
		}
	}
	for i := uint64(4); i < statusCount; i++ {
		if _, err := decodeStatusItem(d); err != nil {
			return nil, err
		}
	}

	reason, err := d.Uint()
	if err != nil {
		return nil, err
	}
	r.Reason = StatusReportReason(reason)

	if r.SubjectSource, err = decodeEIDFrom(d); err != nil {
		return nil, err
	}
	if r.SubjectTimestamp, err = decodeCreationTimestamp(d); err != nil {
		return nil, err
	}

	if items == 6 {
		r.SubjectIsFragment = true
		if r.SubjectFragmentOffset, err = d.Uint(); err != nil {
			return nil, err
		}
		if r.SubjectPayloadLength, err = d.Uint(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// NewStatusReportBundle wraps a status report in a bundle addressed to the
// subject bundle's report-to endpoint.
//
// The administrative record flag is set here, and clause 4.2.3 forbids such a
// bundle from asking for status reports of its own — otherwise reports would
// beget reports. Validate enforces that, so it cannot be set by accident.
func NewStatusReportBundle(primary *PrimaryBlock, report *StatusReport, extensions ...*CanonicalBlock) (*Bundle, error) {
	payload, err := report.Encode()
	if err != nil {
		return nil, err
	}
	primary.Flags |= FlagAdminRecord
	return NewBundle(primary, payload, extensions...)
}

// StatusReport reads the status report a bundle carries, or reports that it
// carries none.
func (b *Bundle) StatusReport() (*StatusReport, error) {
	if !b.Primary.Flags.Has(FlagAdminRecord) {
		return nil, ErrNotAnAdminRecord
	}
	return DecodeStatusReport(b.Payload())
}
