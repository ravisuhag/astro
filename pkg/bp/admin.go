package bp

import (
	"fmt"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

// RecordType is the 4-bit administrative record type code of RFC 5050 §6.1.
type RecordType uint8

const (
	// RecordStatusReport reports how a bundle progressed through the network.
	RecordStatusReport RecordType = 1
	// RecordCustodySignal accepts or refuses custody of a bundle.
	RecordCustodySignal RecordType = 2
)

// String names the record type.
func (r RecordType) String() string {
	switch r {
	case RecordStatusReport:
		return "bundle status report"
	case RecordCustodySignal:
		return "custody signal"
	default:
		return fmt.Sprintf("reserved(%d)", uint8(r))
	}
}

// RecordFlagFragment is the administrative record flag saying the record
// concerns a fragment, so the fragment offset and length fields are present
// (§6.1, figure 9).
const RecordFlagFragment uint8 = 0x01

// DTNTime is a time in the representation §6.1 defines for administrative
// records: seconds since the start of the year 2000, and nanoseconds within
// that second.
//
// CCSDS 734.2-B-1 §3.4 relaxes the precision requirement: where a spacecraft
// clock cannot produce meaningful nanoseconds, the onboard precision is used
// instead, and this does not drive a requirement on the clock.
type DTNTime struct {
	Seconds     uint64
	Nanoseconds uint64
}

// appendDTNTime writes a DTN time as two SDNVs.
func appendDTNTime(dst []byte, t DTNTime) []byte {
	dst = sdnv.AppendEncode(dst, t.Seconds)
	return sdnv.AppendEncode(dst, t.Nanoseconds)
}

// readDTNTime reads a DTN time, returning it and the octets consumed.
func readDTNTime(data []byte) (DTNTime, int, error) {
	fields, n, err := sdnv.DecodeN(data, 2)
	if err != nil {
		return DTNTime{}, 0, ErrDataTooShort
	}
	return DTNTime{Seconds: fields[0], Nanoseconds: fields[1]}, n, nil
}

// StatusFlags are the status report flags of §6.1.1, figure 11. Each bit says
// what the reporting node did with the bundle.
type StatusFlags uint8

const (
	// StatusReceived means the reporting node received the bundle.
	StatusReceived StatusFlags = 0x01
	// StatusCustodyAccepted means it accepted custody.
	StatusCustodyAccepted StatusFlags = 0x02
	// StatusForwarded means it forwarded the bundle.
	StatusForwarded StatusFlags = 0x04
	// StatusDelivered means it delivered the bundle.
	StatusDelivered StatusFlags = 0x08
	// StatusDeleted means it deleted the bundle.
	StatusDeleted StatusFlags = 0x10
)

// Has reports whether every flag in want is set.
func (f StatusFlags) Has(want StatusFlags) bool { return f&want == want }

// ReasonCode explains a status report or custody signal, per §6.1.1.
//
// The list is neither exhaustive nor exclusive: other DTN specifications may
// define more.
type ReasonCode uint8

const (
	// ReasonNoInformation means no additional information.
	ReasonNoInformation ReasonCode = 0x00
	// ReasonLifetimeExpired means the bundle outlived its lifetime.
	ReasonLifetimeExpired ReasonCode = 0x01
	// ReasonForwardedUnidirectional means it was forwarded over a
	// unidirectional link.
	ReasonForwardedUnidirectional ReasonCode = 0x02
	// ReasonTransmissionCancelled means transmission was cancelled.
	ReasonTransmissionCancelled ReasonCode = 0x03
	// ReasonDepletedStorage means the node ran out of storage.
	ReasonDepletedStorage ReasonCode = 0x04
	// ReasonEndpointIDUnintelligible means the destination could not be parsed.
	ReasonEndpointIDUnintelligible ReasonCode = 0x05
	// ReasonNoRoute means no route to the destination.
	ReasonNoRoute ReasonCode = 0x06
	// ReasonNoContact means no timely contact with the next node.
	ReasonNoContact ReasonCode = 0x07
	// ReasonBlockUnintelligible means a block could not be processed.
	ReasonBlockUnintelligible ReasonCode = 0x08
)

// String names the reason.
func (r ReasonCode) String() string {
	switch r {
	case ReasonNoInformation:
		return "no additional information"
	case ReasonLifetimeExpired:
		return "lifetime expired"
	case ReasonForwardedUnidirectional:
		return "forwarded over a unidirectional link"
	case ReasonTransmissionCancelled:
		return "transmission cancelled"
	case ReasonDepletedStorage:
		return "depleted storage"
	case ReasonEndpointIDUnintelligible:
		return "destination endpoint ID unintelligible"
	case ReasonNoRoute:
		return "no known route to destination"
	case ReasonNoContact:
		return "no timely contact with next node"
	case ReasonBlockUnintelligible:
		return "block unintelligible"
	default:
		return fmt.Sprintf("reason(%d)", uint8(r))
	}
}

// StatusReport is a bundle status report, per §6.1.1, figure 10.
//
// A time field is present only when its matching status flag is set, which is
// what makes the record variable-length.
type StatusReport struct {
	Flags  StatusFlags
	Reason ReasonCode

	// IsFragment says the record concerns a fragment, so the offset and
	// length fields are present.
	IsFragment     bool
	FragmentOffset uint64
	FragmentLength uint64

	// Each time is present only when the matching flag in Flags is set.
	ReceiptTime  DTNTime
	CustodyTime  DTNTime
	ForwardTime  DTNTime
	DeliveryTime DTNTime
	DeletionTime DTNTime

	// The bundle being reported on is identified by its creation timestamp
	// and source endpoint.
	CreationTimestamp CreationTimestamp
	SourceEndpoint    EndpointID
}

// Encode serializes the status report as administrative record content.
func (s *StatusReport) Encode() ([]byte, error) {
	out := []byte{byte(s.Flags), byte(s.Reason)}

	if s.IsFragment {
		out = sdnv.AppendEncode(out, s.FragmentOffset)
		out = sdnv.AppendEncode(out, s.FragmentLength)
	}

	// Each time rides on its own flag.
	for _, pair := range []struct {
		flag StatusFlags
		time DTNTime
	}{
		{StatusReceived, s.ReceiptTime},
		{StatusCustodyAccepted, s.CustodyTime},
		{StatusForwarded, s.ForwardTime},
		{StatusDelivered, s.DeliveryTime},
		{StatusDeleted, s.DeletionTime},
	} {
		if s.Flags.Has(pair.flag) {
			out = appendDTNTime(out, pair.time)
		}
	}

	out = sdnv.AppendEncode(out, s.CreationTimestamp.Time)
	out = sdnv.AppendEncode(out, s.CreationTimestamp.SequenceNumber)

	eid := s.SourceEndpoint.String()
	out = sdnv.AppendEncode(out, uint64(len(eid)))
	return append(out, eid...), nil
}

// decodeStatusReport parses status report content.
func decodeStatusReport(data []byte, isFragment bool) (*StatusReport, error) {
	if len(data) < 2 {
		return nil, ErrDataTooShort
	}
	s := &StatusReport{
		Flags:      StatusFlags(data[0]),
		Reason:     ReasonCode(data[1]),
		IsFragment: isFragment,
	}
	offset := 2

	if isFragment {
		fields, n, err := sdnv.DecodeN(data[offset:], 2)
		if err != nil {
			return nil, ErrDataTooShort
		}
		s.FragmentOffset, s.FragmentLength = fields[0], fields[1]
		offset += n
	}

	for _, pair := range []struct {
		flag StatusFlags
		time *DTNTime
	}{
		{StatusReceived, &s.ReceiptTime},
		{StatusCustodyAccepted, &s.CustodyTime},
		{StatusForwarded, &s.ForwardTime},
		{StatusDelivered, &s.DeliveryTime},
		{StatusDeleted, &s.DeletionTime},
	} {
		if !s.Flags.Has(pair.flag) {
			continue
		}
		t, n, err := readDTNTime(data[offset:])
		if err != nil {
			return nil, err
		}
		*pair.time = t
		offset += n
	}

	fields, n, err := sdnv.DecodeN(data[offset:], 3)
	if err != nil {
		return nil, ErrDataTooShort
	}
	s.CreationTimestamp = CreationTimestamp{Time: fields[0], SequenceNumber: fields[1]}
	eidLen := fields[2]
	offset += n

	if uint64(len(data)-offset) < eidLen {
		return nil, ErrDataTooShort
	}
	eid, err := ParseEndpointID(string(data[offset : offset+int(eidLen)]))
	if err != nil {
		return nil, err
	}
	s.SourceEndpoint = eid
	return s, nil
}

// Humanize returns a human-readable summary.
func (s *StatusReport) Humanize() string {
	out := "Bundle Status Report\n  Reason ..... " + s.Reason.String()
	for _, pair := range []struct {
		flag StatusFlags
		name string
	}{
		{StatusReceived, "received"},
		{StatusCustodyAccepted, "custody accepted"},
		{StatusForwarded, "forwarded"},
		{StatusDelivered, "delivered"},
		{StatusDeleted, "deleted"},
	} {
		if s.Flags.Has(pair.flag) {
			out += "\n  Status ..... " + pair.name
		}
	}
	out += "\n  Bundle ..... " + s.SourceEndpoint.String()
	return out
}

// CustodySignal is a custody signal, per §6.1.2, figure 13. It tells the
// current custodian whether the reporting node took custody.
type CustodySignal struct {
	// Succeeded is the high bit of the status byte: custody was accepted.
	Succeeded bool
	// Reason occupies the low seven bits of the status byte.
	Reason ReasonCode

	IsFragment     bool
	FragmentOffset uint64
	FragmentLength uint64

	// SignalTime is when the signal was generated. Unlike a status report,
	// this time is always present.
	SignalTime DTNTime

	CreationTimestamp CreationTimestamp
	SourceEndpoint    EndpointID
}

// Encode serializes the custody signal as administrative record content.
func (c *CustodySignal) Encode() ([]byte, error) {
	status := byte(c.Reason) & 0x7F
	if c.Succeeded {
		status |= 0x80
	}
	out := []byte{status}

	if c.IsFragment {
		out = sdnv.AppendEncode(out, c.FragmentOffset)
		out = sdnv.AppendEncode(out, c.FragmentLength)
	}

	out = appendDTNTime(out, c.SignalTime)
	out = sdnv.AppendEncode(out, c.CreationTimestamp.Time)
	out = sdnv.AppendEncode(out, c.CreationTimestamp.SequenceNumber)

	eid := c.SourceEndpoint.String()
	out = sdnv.AppendEncode(out, uint64(len(eid)))
	return append(out, eid...), nil
}

// decodeCustodySignal parses custody signal content.
func decodeCustodySignal(data []byte, isFragment bool) (*CustodySignal, error) {
	if len(data) < 1 {
		return nil, ErrDataTooShort
	}
	c := &CustodySignal{
		Succeeded:  data[0]&0x80 != 0,
		Reason:     ReasonCode(data[0] & 0x7F),
		IsFragment: isFragment,
	}
	offset := 1

	if isFragment {
		fields, n, err := sdnv.DecodeN(data[offset:], 2)
		if err != nil {
			return nil, ErrDataTooShort
		}
		c.FragmentOffset, c.FragmentLength = fields[0], fields[1]
		offset += n
	}

	t, n, err := readDTNTime(data[offset:])
	if err != nil {
		return nil, err
	}
	c.SignalTime = t
	offset += n

	fields, n, err := sdnv.DecodeN(data[offset:], 3)
	if err != nil {
		return nil, ErrDataTooShort
	}
	c.CreationTimestamp = CreationTimestamp{Time: fields[0], SequenceNumber: fields[1]}
	eidLen := fields[2]
	offset += n

	if uint64(len(data)-offset) < eidLen {
		return nil, ErrDataTooShort
	}
	eid, err := ParseEndpointID(string(data[offset : offset+int(eidLen)]))
	if err != nil {
		return nil, err
	}
	c.SourceEndpoint = eid
	return c, nil
}

// Humanize returns a human-readable summary.
func (c *CustodySignal) Humanize() string {
	outcome := "refused"
	if c.Succeeded {
		outcome = "accepted"
	}
	return "Custody Signal\n  Custody .... " + outcome +
		"\n  Reason ..... " + c.Reason.String() +
		"\n  Bundle ..... " + c.SourceEndpoint.String()
}

// AdminRecord is the payload of a bundle whose administrative-record flag is
// set, per §6.1: a four-bit type code, four bits of flags, then type-specific
// content.
//
// Exactly one of StatusReport and CustodySignal is set.
type AdminRecord struct {
	Type  RecordType
	Flags uint8

	StatusReport  *StatusReport
	CustodySignal *CustodySignal
}

// NewStatusReportRecord wraps a status report as an administrative record.
func NewStatusReportRecord(s *StatusReport) *AdminRecord {
	r := &AdminRecord{Type: RecordStatusReport, StatusReport: s}
	if s != nil && s.IsFragment {
		r.Flags |= RecordFlagFragment
	}
	return r
}

// NewCustodySignalRecord wraps a custody signal as an administrative record.
func NewCustodySignalRecord(c *CustodySignal) *AdminRecord {
	r := &AdminRecord{Type: RecordCustodySignal, CustodySignal: c}
	if c != nil && c.IsFragment {
		r.Flags |= RecordFlagFragment
	}
	return r
}

// Encode serializes the administrative record.
func (a *AdminRecord) Encode() ([]byte, error) {
	// Type in the high nibble, flags in the low nibble.
	head := byte(a.Type&0x0F)<<4 | (a.Flags & 0x0F)

	var content []byte
	var err error
	switch a.Type {
	case RecordStatusReport:
		if a.StatusReport == nil {
			return nil, ErrInvalidRecordType
		}
		content, err = a.StatusReport.Encode()
	case RecordCustodySignal:
		if a.CustodySignal == nil {
			return nil, ErrInvalidRecordType
		}
		content, err = a.CustodySignal.Encode()
	default:
		return nil, ErrInvalidRecordType
	}
	if err != nil {
		return nil, err
	}
	return append([]byte{head}, content...), nil
}

// DecodeAdminRecord parses an administrative record from a bundle payload.
func DecodeAdminRecord(data []byte) (*AdminRecord, error) {
	if len(data) < 1 {
		return nil, ErrDataTooShort
	}
	a := &AdminRecord{
		Type:  RecordType(data[0] >> 4),
		Flags: data[0] & 0x0F,
	}
	isFragment := a.Flags&RecordFlagFragment != 0

	var err error
	switch a.Type {
	case RecordStatusReport:
		a.StatusReport, err = decodeStatusReport(data[1:], isFragment)
	case RecordCustodySignal:
		a.CustodySignal, err = decodeCustodySignal(data[1:], isFragment)
	default:
		return nil, ErrInvalidRecordType
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// AdminRecord parses the bundle's payload as an administrative record. It
// fails when the bundle's administrative-record flag is not set.
func (b *Bundle) AdminRecord() (*AdminRecord, error) {
	if b.Primary == nil || !b.Primary.IsAdminRecord() {
		return nil, ErrNotAdminRecord
	}
	payload, err := b.Payload()
	if err != nil {
		return nil, err
	}
	return DecodeAdminRecord(payload)
}

// Humanize returns a human-readable summary.
func (a *AdminRecord) Humanize() string {
	switch {
	case a.StatusReport != nil:
		return a.StatusReport.Humanize()
	case a.CustodySignal != nil:
		return a.CustodySignal.Humanize()
	default:
		return "Administrative Record (" + a.Type.String() + ")"
	}
}
