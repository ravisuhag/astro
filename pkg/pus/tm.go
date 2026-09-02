package pus

import (
	"time"

	"github.com/ravisuhag/astro/pkg/tcf"
)

// TMHeader is the telemetry packet secondary header of Figure 7-7.
//
// It implements spp.SecondaryHeader, so a PUS report is built by handing one
// to spp.WithSecondaryHeader.
type TMHeader struct {
	// Profile pins the mission-tailorable widths and the time format.
	Profile MissionProfile

	// TimeReferenceStatus reports the status of the on-board time reference
	// used to time tag this packet (clause 7.4.3.1d). An application process
	// that cannot report it sets zero (clause 7.4.3.1e).
	TimeReferenceStatus uint8

	// Service and Subtype are the message type ID (clause 7.4.3.1f).
	Service uint8
	Subtype uint8

	// MessageTypeCounter counts messages of this type per destination
	// (clause 7.4.3.1g), 16 bits. Zero when the capability is absent
	// (clause 7.4.3.1h).
	MessageTypeCounter uint16

	// DestinationID is the application process user identifier of the
	// addressed process (clause 7.4.3.1i), 16 bits.
	DestinationID uint16

	// Time is the time tag of the report (clause 7.4.3.1k). Used when the
	// profile selects TimeCUC.
	Time time.Time

	// RawTime carries the time field verbatim when the profile selects
	// TimeRaw. Its length must match the profile's TimeRawBytes.
	RawTime []byte

	// Spare pads the header to the mission's word size. Its length must match
	// the profile's TMSpareBytes.
	Spare []byte
}

// NewTMHeader builds a telemetry secondary header under a profile.
func (p MissionProfile) NewTMHeader(service, subtype uint8, destinationID uint16, t time.Time) *TMHeader {
	h := &TMHeader{
		Profile:       p,
		Service:       service,
		Subtype:       subtype,
		DestinationID: destinationID,
		Time:          t,
	}
	if p.TMSpareBytes > 0 {
		h.Spare = make([]byte, p.TMSpareBytes)
	}
	if p.TimeFormat == TimeRaw && p.TimeRawBytes > 0 {
		h.RawTime = make([]byte, p.TimeRawBytes)
	}
	return h
}

// Size returns the fixed encoded width, as spp.SecondaryHeader requires.
func (h *TMHeader) Size() int { return h.Profile.TMHeaderSize() }

// Validate checks the header against clause 7.4.3.1.
func (h *TMHeader) Validate() error {
	if err := h.Profile.Validate(); err != nil {
		return err
	}
	if h.TimeReferenceStatus > 0x0F {
		return ErrValueTooLarge
	}
	if len(h.Spare) != h.Profile.TMSpareBytes {
		return ErrInvalidProfile
	}
	if h.Profile.TimeFormat == TimeRaw && len(h.RawTime) != h.Profile.TimeRawBytes {
		return ErrInvalidProfile
	}
	return nil
}

// cucOptions builds the pkg/tcf options this profile's CUC field needs.
func (p MissionProfile) cucOptions() []tcf.CUCOption {
	opts := []tcf.CUCOption{
		tcf.WithCUCCoarseBytes(uint8(p.CUCCoarseBytes)),
		tcf.WithCUCFineBytes(uint8(p.CUCFineBytes)),
	}
	if !p.CUCEpoch.IsZero() {
		opts = append(opts, tcf.WithCUCEpoch(p.CUCEpoch))
	}
	return opts
}

// epoch returns the CUC epoch this profile decodes against.
func (p MissionProfile) epoch() time.Time {
	if p.CUCEpoch.IsZero() {
		// pkg/tcf's default: the CCSDS 1958 epoch.
		return time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return p.CUCEpoch
}

// encodeTime serializes the absolute time field per the profile.
//
// The same field appears in ST[11] message bodies, so the codec is shared;
// see timefield.go.
func (h *TMHeader) encodeTime() ([]byte, error) {
	return encodeAbsoluteTime(h.Profile, h.Time, h.RawTime)
}

// Encode serializes the header per Figure 7-7.
func (h *TMHeader) Encode() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	out := make([]byte, 0, h.Size())
	// Octet 0: PUS version (4 bits) | spacecraft time reference status (4 bits).
	out = append(out, Version<<4|(h.TimeReferenceStatus&0x0F))
	out = append(out, h.Service, h.Subtype)

	var err error
	if out, err = putUint(out, uint64(h.MessageTypeCounter), MessageTypeCounterSize); err != nil {
		return nil, err
	}
	if out, err = putUint(out, uint64(h.DestinationID), DestinationIDSize); err != nil {
		return nil, err
	}

	timeField, err := h.encodeTime()
	if err != nil {
		return nil, err
	}
	if len(timeField) != h.Profile.TimeSize() {
		// A CUC encoder that disagrees with the declared width would silently
		// shift every field after it.
		return nil, ErrInvalidProfile
	}
	out = append(out, timeField...)

	return append(out, h.Spare...), nil
}

// Decode parses the header. The receiving header must already carry the
// profile, since the wire format is not self-describing.
func (h *TMHeader) Decode(data []byte) error {
	if err := h.Profile.Validate(); err != nil {
		return err
	}
	if len(data) < h.Size() {
		return ErrDataTooShort
	}

	if version := data[0] >> 4; version != Version {
		return ErrInvalidVersion
	}
	h.TimeReferenceStatus = data[0] & 0x0F
	h.Service = data[1]
	h.Subtype = data[2]

	offset := 3
	counter, err := readUint(data[offset:], MessageTypeCounterSize)
	if err != nil {
		return err
	}
	h.MessageTypeCounter = uint16(counter)
	offset += MessageTypeCounterSize

	dest, err := readUint(data[offset:], DestinationIDSize)
	if err != nil {
		return err
	}
	h.DestinationID = uint16(dest)
	offset += DestinationIDSize

	timeSize := h.Profile.TimeSize()
	stamp, raw, _, err := decodeAbsoluteTime(h.Profile, data[offset:offset+timeSize])
	if err != nil {
		return err
	}
	h.Time, h.RawTime = stamp, raw
	offset += timeSize

	if h.Profile.TMSpareBytes > 0 {
		h.Spare = make([]byte, h.Profile.TMSpareBytes)
		copy(h.Spare, data[offset:offset+h.Profile.TMSpareBytes])
	} else {
		h.Spare = nil
	}
	return nil
}

// Key returns the message type this header names.
func (h *TMHeader) Key() MessageKey { return MessageKey{Service: h.Service, Subtype: h.Subtype} }

// Humanize returns a human-readable summary.
func (h *TMHeader) Humanize() string {
	return "PUS TM Secondary Header" +
		"\n  Version ....... " + itoa(Version) +
		"\n  Time status ... " + itoa(int(h.TimeReferenceStatus)) +
		"\n  Message type .. TM[" + itoa(int(h.Service)) + "," + itoa(int(h.Subtype)) + "]" +
		"\n  Counter ....... " + itoa(int(h.MessageTypeCounter)) +
		"\n  Destination ... " + itoa(int(h.DestinationID)) +
		"\n  Time .......... " + h.Time.UTC().Format(time.RFC3339Nano)
}

// implicitCUCPField rebuilds the P-field octet that a PFC implies, so a
// T-field-only time value can be handed to pkg/tcf for decoding.
//
// It mirrors how pkg/tcf builds the field for the widths PUS permits: 1 to 4
// coarse octets and 0 to 3 fine, which always fit one octet with no extension.
// The time code ID selects Level 1 for the CCSDS 1958 epoch and Level 2 for an
// agency-defined one, per CCSDS 301.0-B-4 and clause 7.4.3.1j note 1.
func (p MissionProfile) implicitCUCPField() byte {
	id := tcf.TimeCodeCUCLevel1
	if !p.CUCEpoch.IsZero() && !p.CUCEpoch.Equal(tcf.CCSDSEpoch) {
		id = tcf.TimeCodeCUCLevel2
	}
	detail := byte((p.CUCCoarseBytes-1)<<2) | byte(p.CUCFineBytes)
	// Extension flag clear, time code ID in bits 4-6, detail in bits 0-3.
	return id<<4 | detail&0x0F
}
