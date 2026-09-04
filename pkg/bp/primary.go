package bp

// Version is the Bundle Protocol version this package speaks
// (RFC 9171 clause 4.3.1). Version 6 is a different protocol, not an earlier
// revision of this one, and nothing here reads it.
const Version = 7

// BundleControlFlags describes the bundle as a whole
// (RFC 9171 clause 4.2.3). Bit numbering runs from the low-order bit, which is
// the reverse of the usual convention; the clause calls that out because more
// flags may be defined later.
type BundleControlFlags uint64

const (
	// FlagIsFragment marks a bundle carrying part of an application data unit.
	FlagIsFragment BundleControlFlags = 1 << 0
	// FlagAdminRecord marks a payload that is an administrative record.
	FlagAdminRecord BundleControlFlags = 1 << 1
	// FlagMustNotFragment forbids fragmenting this bundle.
	FlagMustNotFragment BundleControlFlags = 1 << 2
	// FlagAppAckRequested asks the receiving application to acknowledge.
	FlagAppAckRequested BundleControlFlags = 1 << 5
	// FlagStatusTimeRequested asks for a timestamp in every status report.
	FlagStatusTimeRequested BundleControlFlags = 1 << 6
	// FlagReportReception asks for a report when the bundle is received.
	FlagReportReception BundleControlFlags = 1 << 14
	// FlagReportForwarding asks for a report when the bundle is forwarded.
	FlagReportForwarding BundleControlFlags = 1 << 16
	// FlagReportDelivery asks for a report when the bundle is delivered.
	FlagReportDelivery BundleControlFlags = 1 << 17
	// FlagReportDeletion asks for a report when the bundle is deleted.
	FlagReportDeletion BundleControlFlags = 1 << 18
)

// statusReportFlags is every "please report" flag at once. Two rules in
// clause 4.2.3 turn all of them off together.
const statusReportFlags = FlagReportReception | FlagReportForwarding |
	FlagReportDelivery | FlagReportDeletion

// Has reports whether every flag in mask is set.
func (f BundleControlFlags) Has(mask BundleControlFlags) bool {
	return f&mask == mask
}

// PrimaryBlock carries what a node needs to forward a bundle
// (RFC 9171 clause 4.3.1). It is immutable once the bundle is created: every
// field must reach the destination with the octets it left with.
type PrimaryBlock struct {
	Flags       BundleControlFlags
	CRCType     CRCType
	Destination EID
	// Source is the node the bundle started at, or the null endpoint when the
	// sender stays anonymous.
	Source   EID
	ReportTo EID
	// Timestamp, with Source and the fragment fields, identifies the bundle.
	Timestamp CreationTimestamp
	// Lifetime is milliseconds past the creation time, after which the bundle
	// is no longer useful and may be deleted.
	Lifetime uint64

	// FragmentOffset and TotalADULength are present if and only if
	// FlagIsFragment is set.
	FragmentOffset uint64
	TotalADULength uint64
}

// Validate checks the rules clause 4.2.3 states about flag combinations. They
// are cross-field rules, so no single field being sensible catches them.
func (p *PrimaryBlock) Validate() error {
	if !p.CRCType.valid() {
		return ErrInvalidCRCType
	}

	// An administrative record cannot ask for status reports about itself,
	// which would let reports beget reports.
	if p.Flags.Has(FlagAdminRecord) && p.Flags&statusReportFlags != 0 {
		return ErrAdminRecordWantsReports
	}

	// An anonymous bundle has no identity, so everything built on identity has
	// to be switched off: it cannot be fragmented and cannot be reported on.
	if p.Source.IsNull() {
		if !p.Flags.Has(FlagMustNotFragment) {
			return ErrAnonymousBundleFragmentable
		}
		if p.Flags&statusReportFlags != 0 {
			return ErrAnonymousBundleWantsReports
		}
	}

	for _, e := range []EID{p.Destination, p.Source, p.ReportTo} {
		if err := e.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// appendPrimaryBlock writes the primary block, checksum included.
func appendPrimaryBlock(dst []byte, p *PrimaryBlock) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Clause 4.3.1 fixes the array length by what is present: 8 plain, 9 with
	// a checksum, 10 for a fragment, 11 for a fragment with a checksum.
	items := uint64(8)
	if p.Flags.Has(FlagIsFragment) {
		items += 2
	}
	if p.CRCType != CRCNone {
		items++
	}

	start := len(dst)
	dst = appendArrayHeader(dst, items)
	dst = appendUint(dst, Version)
	dst = appendUint(dst, uint64(p.Flags))
	dst = appendUint(dst, uint64(p.CRCType))

	var err error
	for _, e := range []EID{p.Destination, p.Source, p.ReportTo} {
		if dst, err = appendEID(dst, e); err != nil {
			return nil, err
		}
	}

	dst = appendCreationTimestamp(dst, p.Timestamp)
	dst = appendUint(dst, p.Lifetime)

	if p.Flags.Has(FlagIsFragment) {
		dst = appendUint(dst, p.FragmentOffset)
		dst = appendUint(dst, p.TotalADULength)
	}

	if p.CRCType != CRCNone {
		dst = appendZeroCRC(dst, p.CRCType)
		fillCRC(dst[start:], p.CRCType)
	}
	return dst, nil
}

// primaryBlock reads a primary block and verifies its checksum.
func (d *decoder) primaryBlock() (*PrimaryBlock, error) {
	start := d.pos

	items, indefinite, err := d.arrayHeader()
	if err != nil {
		return nil, err
	}
	if indefinite || items < 8 || items > 11 {
		return nil, ErrMalformedPrimaryBlock
	}

	version, err := d.uint()
	if err != nil {
		return nil, err
	}
	if version != Version {
		return nil, ErrUnsupportedVersion
	}

	flags, err := d.uint()
	if err != nil {
		return nil, err
	}
	crcCode, err := d.uint()
	if err != nil {
		return nil, err
	}
	crcType := CRCType(crcCode)
	if !crcType.valid() {
		return nil, ErrInvalidCRCType
	}

	p := &PrimaryBlock{Flags: BundleControlFlags(flags), CRCType: crcType}

	for _, dest := range []*EID{&p.Destination, &p.Source, &p.ReportTo} {
		e, err := d.eid()
		if err != nil {
			return nil, err
		}
		*dest = e
	}

	if p.Timestamp, err = d.creationTimestamp(); err != nil {
		return nil, err
	}
	if p.Lifetime, err = d.uint(); err != nil {
		return nil, err
	}

	// The array length and the fragment flag have to tell the same story.
	wantItems := uint64(8)
	if p.Flags.Has(FlagIsFragment) {
		wantItems += 2
	}
	if crcType != CRCNone {
		wantItems++
	}
	if items != wantItems {
		return nil, ErrPrimaryBlockLengthMismatch
	}

	if p.Flags.Has(FlagIsFragment) {
		if p.FragmentOffset, err = d.uint(); err != nil {
			return nil, err
		}
		if p.TotalADULength, err = d.uint(); err != nil {
			return nil, err
		}
	}

	if crcType != CRCNone {
		got, err := d.byteString()
		if err != nil {
			return nil, err
		}
		if err := checkCRC(d.buf[start:d.pos], crcType, got); err != nil {
			return nil, err
		}
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}
