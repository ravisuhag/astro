package pus

// AckFlags are the four acknowledgement request bits of a TC secondary
// header, per ECSS-E-ST-70-41C clause 7.4.4.1d.
//
// Each bit asks the destination application process for one verification
// report from service ST[01]. The bit positions are fixed by the standard:
// bit 3 acceptance, bit 2 start, bit 1 progress, bit 0 completion.
type AckFlags uint8

const (
	// AckCompletion asks for a report on successful completion of execution
	// (bit 0, clause 7.4.4.1d.4).
	AckCompletion AckFlags = 1 << 0
	// AckProgress asks for reports on successful progress of execution
	// (bit 1, clause 7.4.4.1d.3).
	AckProgress AckFlags = 1 << 1
	// AckStart asks for a report on successful start of execution
	// (bit 2, clause 7.4.4.1d.2).
	AckStart AckFlags = 1 << 2
	// AckAcceptance asks for a report on successful acceptance
	// (bit 3, clause 7.4.4.1d.1).
	AckAcceptance AckFlags = 1 << 3
)

// Has reports whether every flag in want is set.
func (a AckFlags) Has(want AckFlags) bool { return a&want == want }

// String lists the acknowledgements requested.
func (a AckFlags) String() string {
	if a == 0 {
		return "none"
	}
	out := ""
	for _, f := range []struct {
		flag AckFlags
		name string
	}{
		{AckAcceptance, "acceptance"},
		{AckStart, "start"},
		{AckProgress, "progress"},
		{AckCompletion, "completion"},
	} {
		if a.Has(f.flag) {
			if out != "" {
				out += ", "
			}
			out += f.name
		}
	}
	return out
}

// TCHeader is the telecommand packet secondary header of Figure 7-9.
//
// It implements spp.SecondaryHeader, so a PUS telecommand is built by handing
// one to spp.WithSecondaryHeader.
type TCHeader struct {
	// Profile pins the mission-tailorable widths. A TCHeader without one
	// cannot encode, which is deliberate.
	Profile MissionProfile

	// AckFlags asks for verification reports (clause 7.4.4.1d).
	AckFlags AckFlags
	// Service and Subtype are the message type ID (clause 7.4.4.1e).
	Service uint8
	Subtype uint8
	// SourceID identifies the issuing entity (clause 7.4.4.1f), 16 bits.
	SourceID uint16
	// Spare pads the header to the mission's word size. Its length must match
	// the profile's TCSpareBytes.
	Spare []byte
}

// NewTCHeader builds a telecommand secondary header under a profile.
func (p MissionProfile) NewTCHeader(service, subtype uint8, sourceID uint16, ack AckFlags) *TCHeader {
	h := &TCHeader{
		Profile:  p,
		AckFlags: ack,
		Service:  service,
		Subtype:  subtype,
		SourceID: sourceID,
	}
	if p.TCSpareBytes > 0 {
		h.Spare = make([]byte, p.TCSpareBytes)
	}
	return h
}

// Size returns the fixed encoded width, as spp.SecondaryHeader requires.
func (h *TCHeader) Size() int { return h.Profile.TCHeaderSize() }

// Validate checks the header against clause 7.4.4.1.
func (h *TCHeader) Validate() error {
	if err := h.Profile.Validate(); err != nil {
		return err
	}
	if h.AckFlags > 0x0F {
		return ErrValueTooLarge
	}
	if len(h.Spare) != h.Profile.TCSpareBytes {
		return ErrInvalidProfile
	}
	return nil
}

// Encode serializes the header per Figure 7-9.
func (h *TCHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	out := make([]byte, 0, h.Size())
	// Octet 0: PUS version (4 bits) | acknowledgement flags (4 bits).
	out = append(out, Version<<4|byte(h.AckFlags&0x0F))
	out = append(out, h.Service, h.Subtype)

	var err error
	if out, err = putUint(out, uint64(h.SourceID), SourceIDSize); err != nil {
		return nil, err
	}
	return append(out, h.Spare...), nil
}

// Decode parses the header. The receiving header must already carry the
// profile, since the wire format is not self-describing.
func (h *TCHeader) Decode(data []byte) error {
	if err := h.Profile.Validate(); err != nil {
		return err
	}
	if len(data) < h.Size() {
		return ErrDataTooShort
	}

	if version := data[0] >> 4; version != Version {
		return ErrInvalidVersion
	}
	h.AckFlags = AckFlags(data[0] & 0x0F)
	h.Service = data[1]
	h.Subtype = data[2]

	offset := 3
	src, err := readUint(data[offset:], SourceIDSize)
	if err != nil {
		return err
	}
	h.SourceID = uint16(src)
	offset += SourceIDSize

	if h.Profile.TCSpareBytes > 0 {
		h.Spare = make([]byte, h.Profile.TCSpareBytes)
		copy(h.Spare, data[offset:offset+h.Profile.TCSpareBytes])
	} else {
		h.Spare = nil
	}
	return nil
}

// Key returns the message type this header names.
func (h *TCHeader) Key() MessageKey { return MessageKey{Service: h.Service, Subtype: h.Subtype} }

// Humanize returns a human-readable summary.
func (h *TCHeader) Humanize() string {
	return "PUS TC Secondary Header" +
		"\n  Version ....... " + itoa(Version) +
		"\n  Ack flags ..... " + h.AckFlags.String() +
		"\n  Message type .. TC[" + itoa(int(h.Service)) + "," + itoa(int(h.Subtype)) + "]" +
		"\n  Source ID ..... " + itoa(int(h.SourceID))
}
