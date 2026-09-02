package bp

import (
	"fmt"

	"github.com/ravisuhag/astro/pkg/sdnv"
)

// BundleFlags are the bundle processing control flags of RFC 5050 clause 4.2.
//
// The field is an SDNV, so it has no fixed width. Bits 0 to 6 are handling
// requests, 7 to 13 carry the class of service, and 14 to 20 request status
// reports.
type BundleFlags uint64

const (
	// FlagFragment marks a bundle that is a fragment (bit 0).
	FlagFragment BundleFlags = 1 << 0
	// FlagAdminRecord marks an administrative record payload (bit 1).
	FlagAdminRecord BundleFlags = 1 << 1
	// FlagNoFragment forbids fragmenting this bundle (bit 2).
	FlagNoFragment BundleFlags = 1 << 2
	// FlagCustodyRequested asks the next node to take custody (bit 3).
	FlagCustodyRequested BundleFlags = 1 << 3
	// FlagSingleton marks a destination that is a singleton endpoint (bit 4).
	FlagSingleton BundleFlags = 1 << 4
	// FlagAppAck asks for application-level acknowledgement (bit 5).
	FlagAppAck BundleFlags = 1 << 5

	// FlagReportReception asks for a bundle-reception status report (bit 14).
	FlagReportReception BundleFlags = 1 << 14
	// FlagReportCustody asks for a custody-acceptance report (bit 15).
	FlagReportCustody BundleFlags = 1 << 15
	// FlagReportForwarding asks for a forwarding report (bit 16).
	FlagReportForwarding BundleFlags = 1 << 16
	// FlagReportDelivery asks for a delivery report (bit 17).
	FlagReportDelivery BundleFlags = 1 << 17
	// FlagReportDeletion asks for a deletion report (bit 18).
	FlagReportDeletion BundleFlags = 1 << 18
)

// priorityShift is where the two-bit class-of-service field starts: bits 7
// and 8, with bit 8 the most significant (clause 4.2).
const priorityShift = 7

// Priority is the bundle's class of service, per clause 4.2.
type Priority uint8

const (
	// PriorityBulk is the lowest class.
	PriorityBulk Priority = 0
	// PriorityNormal is the middle class.
	PriorityNormal Priority = 1
	// PriorityExpedited is the highest class RFC 5050 defines. Value 3 is
	// reserved.
	PriorityExpedited Priority = 2
)

// String names the priority.
func (p Priority) String() string {
	switch p {
	case PriorityBulk:
		return "bulk"
	case PriorityNormal:
		return "normal"
	case PriorityExpedited:
		return "expedited"
	default:
		return "reserved"
	}
}

// Priority extracts the class of service from the flags.
func (f BundleFlags) Priority() Priority {
	return Priority(f >> priorityShift & 0x03)
}

// WithPriority returns the flags with the class of service replaced.
func (f BundleFlags) WithPriority(p Priority) BundleFlags {
	return f&^(0x03<<priorityShift) | BundleFlags(p&0x03)<<priorityShift
}

// Has reports whether every flag in want is set.
func (f BundleFlags) Has(want BundleFlags) bool { return f&want == want }

// CreationTimestamp identifies a bundle together with its source endpoint,
// per clause 4.5.1. The time is seconds since the year 2000, and the sequence
// number distinguishes bundles created within the same second.
type CreationTimestamp struct {
	Time           uint64
	SequenceNumber uint64
}

// PrimaryBlock is the primary bundle block of RFC 5050 clause 4.5.1.
//
// The four endpoints travel as offsets into a shared dictionary rather than
// as strings, so a bundle whose source and report-to are the same endpoint
// pays for that string once.
type PrimaryBlock struct {
	Flags BundleFlags

	Destination EndpointID
	Source      EndpointID
	ReportTo    EndpointID
	Custodian   EndpointID

	CreationTimestamp CreationTimestamp

	// Lifetime is how long the bundle stays useful, in seconds from its
	// creation timestamp.
	Lifetime uint64

	// FragmentOffset and TotalADULength are present only when FlagFragment
	// is set (clause 4.5.1).
	FragmentOffset uint64
	TotalADULength uint64
}

// IsFragment reports whether this block describes a fragment.
func (p *PrimaryBlock) IsFragment() bool { return p.Flags.Has(FlagFragment) }

// IsAdminRecord reports whether the payload is an administrative record.
func (p *PrimaryBlock) IsAdminRecord() bool { return p.Flags.Has(FlagAdminRecord) }

// Validate checks the block against clause 4.2 and clause 4.5.1.
func (p *PrimaryBlock) Validate() error {
	// Clause 4.2: an administrative record requests neither custody transfer nor
	// any status report.
	if p.IsAdminRecord() {
		reportFlags := FlagReportReception | FlagReportCustody | FlagReportForwarding |
			FlagReportDelivery | FlagReportDeletion
		if p.Flags&FlagCustodyRequested != 0 || p.Flags&reportFlags != 0 {
			return ErrAdminRecordFlags
		}
	}

	// Clause 4.2: class of service 3 is reserved.
	if p.Flags.Priority() > PriorityExpedited {
		return ErrInvalidPriority
	}

	// A bundle cannot both be a fragment and forbid fragmentation.
	if p.Flags.Has(FlagFragment) && p.Flags.Has(FlagNoFragment) {
		return ErrFragmentFlags
	}

	// Clause 4.2: an anonymous bundle (source dtn:none) is not uniquely
	// identifiable, so custody transfer must not be requested and the
	// "must not be fragmented" flag must be set.
	if p.Source.IsNull() {
		if p.Flags.Has(FlagCustodyRequested) || !p.Flags.Has(FlagNoFragment) {
			return ErrAnonymousSource
		}
	}
	return nil
}

// Encode serializes the primary block.
func (p *PrimaryBlock) Encode() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// RFC 6260 clause 2.1: when every endpoint is either ipn or dtn:none, the
	// dictionary is omitted entirely (its length encodes as zero) and each
	// endpoint's node and service numbers ride in the offset fields
	// themselves. That is Compressed Bundle Header Encoding, and CCSDS
	// 734.2-B-1 clause 3.2 mandates it. dtn:none travels as the pair (0, 0).
	endpoints := [4]EndpointID{p.Destination, p.Source, p.ReportTo, p.Custodian}
	var pairs [8]uint64
	cbhe := true
	for i, e := range endpoints {
		node, service, ok := e.cbheParts()
		if !ok {
			cbhe = false
			break
		}
		pairs[i*2], pairs[i*2+1] = node, service
	}

	var dictBuf []byte
	if !cbhe {
		// RFC 5050 clause 4.4: the general form, a dictionary of null-terminated
		// strings with each endpoint as a pair of offsets into it. Build the
		// dictionary first: the offsets depend on it.
		dict := newDictionary()
		for i, e := range endpoints {
			pairs[i*2], pairs[i*2+1] = dict.add(e)
		}
		dictBuf = dict.buf
	}

	// Everything after the block-length field, which is what that field measures.
	var body []byte
	for _, v := range []uint64{
		pairs[0], pairs[1], pairs[2], pairs[3],
		pairs[4], pairs[5], pairs[6], pairs[7],
		p.CreationTimestamp.Time, p.CreationTimestamp.SequenceNumber,
		p.Lifetime, uint64(len(dictBuf)),
	} {
		body = sdnv.AppendEncode(body, v)
	}
	body = append(body, dictBuf...)

	if p.IsFragment() {
		body = sdnv.AppendEncode(body, p.FragmentOffset)
		body = sdnv.AppendEncode(body, p.TotalADULength)
	}

	out := []byte{Version}
	out = sdnv.AppendEncode(out, uint64(p.Flags))
	out = sdnv.AppendEncode(out, uint64(len(body)))
	return append(out, body...), nil
}

// DecodePrimaryBlock parses a primary block from the front of data, returning
// the block and the octets consumed.
func DecodePrimaryBlock(data []byte) (*PrimaryBlock, int, error) {
	if len(data) < 1 {
		return nil, 0, ErrDataTooShort
	}
	if data[0] != Version {
		return nil, 0, ErrInvalidVersion
	}
	offset := 1

	flags, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	offset += n

	blockLen, n, err := sdnv.Decode(data[offset:])
	if err != nil {
		return nil, 0, ErrDataTooShort
	}
	offset += n

	if uint64(len(data)-offset) < blockLen {
		return nil, 0, ErrDataTooShort
	}
	body := data[offset : offset+int(blockLen)]
	end := offset + int(blockLen)

	// Twelve SDNVs: eight endpoint offsets, two timestamp fields, lifetime,
	// and the dictionary length.
	fields, consumed, err := sdnv.DecodeN(body, 12)
	if err != nil {
		return nil, 0, ErrDataTooShort
	}

	dictLen := fields[11]
	if uint64(len(body)-consumed) < dictLen {
		return nil, 0, ErrDataTooShort
	}
	dict := body[consumed : consumed+int(dictLen)]
	bodyOffset := consumed + int(dictLen)

	p := &PrimaryBlock{
		Flags: BundleFlags(flags),
		CreationTimestamp: CreationTimestamp{
			Time:           fields[8],
			SequenceNumber: fields[9],
		},
		Lifetime: fields[10],
	}

	for i, target := range []*EndpointID{&p.Destination, &p.Source, &p.ReportTo, &p.Custodian} {
		var eid EndpointID
		if dictLen == 0 {
			// RFC 6260 clause 2.2: a zero-length dictionary marks CBHE. The
			// offset fields hold ipn node and service numbers directly,
			// with (0, 0) standing for dtn:none.
			eid, err = cbheEndpoint(fields[i*2], fields[i*2+1])
		} else {
			eid, err = lookupEndpoint(dict, fields[i*2], fields[i*2+1])
		}
		if err != nil {
			return nil, 0, err
		}
		*target = eid
	}

	if p.IsFragment() {
		frag, _, err := sdnv.DecodeN(body[bodyOffset:], 2)
		if err != nil {
			return nil, 0, ErrDataTooShort
		}
		p.FragmentOffset, p.TotalADULength = frag[0], frag[1]
	}

	if err := p.Validate(); err != nil {
		return nil, 0, err
	}
	return p, end, nil
}

// Humanize returns a human-readable summary.
func (p *PrimaryBlock) Humanize() string {
	out := fmt.Sprintf("Bundle Primary Block\n"+
		"  Version ....... %d\n"+
		"  Priority ...... %s\n"+
		"  Destination ... %s\n"+
		"  Source ........ %s\n"+
		"  Report to ..... %s\n"+
		"  Custodian ..... %s\n"+
		"  Created ....... %d.%d\n"+
		"  Lifetime ...... %d s",
		Version, p.Flags.Priority(), p.Destination, p.Source, p.ReportTo,
		p.Custodian, p.CreationTimestamp.Time, p.CreationTimestamp.SequenceNumber, p.Lifetime)
	if p.IsFragment() {
		out += fmt.Sprintf("\n  Fragment ...... offset %d of %d", p.FragmentOffset, p.TotalADULength)
	}
	if p.IsAdminRecord() {
		out += "\n  Payload ....... administrative record"
	}
	return out
}
