package pus

import "encoding/binary"

// ST[01] request verification, per ECSS-E-ST-70-41C clause 8.1.
//
// Every report in this service carries a request ID naming the telecommand it
// concerns, and the failure reports add a failure notice.
const (
	ServiceRequestVerification uint8 = 1

	SubtypeAcceptSuccess   uint8 = 1  // TM[1,1] clause 8.1.2.1
	SubtypeAcceptFailure   uint8 = 2  // TM[1,2] clause 8.1.2.2
	SubtypeStartSuccess    uint8 = 3  // TM[1,3] clause 8.1.2.3
	SubtypeStartFailure    uint8 = 4  // TM[1,4] clause 8.1.2.4
	SubtypeProgressSuccess uint8 = 5  // TM[1,5] clause 8.1.2.5
	SubtypeProgressFailure uint8 = 6  // TM[1,6] clause 8.1.2.6
	SubtypeCompleteSuccess uint8 = 7  // TM[1,7] clause 8.1.2.7
	SubtypeCompleteFailure uint8 = 8  // TM[1,8] clause 8.1.2.8
	SubtypeRoutingFailure  uint8 = 10 // TM[1,10] clause 8.1.2.10
)

// RequestIDSize is the encoded width of a request ID, in octets. Figure 8-1
// lays it out as packet version number (3) + packet type (1) + secondary
// header flag (1) + APID (11) + sequence flags (2) + sequence count (14),
// which is 32 bits — exactly the first four octets of a CCSDS primary header.
const RequestIDSize = 4

// RequestID identifies the telecommand a verification report concerns
// (Figure 8-1).
//
// It does not name the source of the request. As the standard's note points
// out, that comes from the destination ID of the report's own secondary
// header.
type RequestID struct {
	PacketVersion       uint8  // 3 bits
	PacketType          uint8  // 1 bit
	SecondaryHeaderFlag uint8  // 1 bit
	APID                uint16 // 11 bits
	SequenceFlags       uint8  // 2 bits
	SequenceCount       uint16 // 14 bits
}

// Encode serializes the request ID into four octets.
func (r RequestID) Encode() []byte {
	out := make([]byte, RequestIDSize)
	first := uint16(r.PacketVersion&0x07) << 13
	first |= uint16(r.PacketType&0x01) << 12
	first |= uint16(r.SecondaryHeaderFlag&0x01) << 11
	first |= r.APID & 0x07FF
	binary.BigEndian.PutUint16(out[0:2], first)

	second := uint16(r.SequenceFlags&0x03) << 14
	second |= r.SequenceCount & 0x3FFF
	binary.BigEndian.PutUint16(out[2:4], second)
	return out
}

// DecodeRequestID parses a request ID from the front of data.
func DecodeRequestID(data []byte) (RequestID, error) {
	if len(data) < RequestIDSize {
		return RequestID{}, ErrDataTooShort
	}
	first := binary.BigEndian.Uint16(data[0:2])
	second := binary.BigEndian.Uint16(data[2:4])
	return RequestID{
		PacketVersion:       uint8(first >> 13 & 0x07),
		PacketType:          uint8(first >> 12 & 0x01),
		SecondaryHeaderFlag: uint8(first >> 11 & 0x01),
		APID:                first & 0x07FF,
		SequenceFlags:       uint8(second >> 14 & 0x03),
		SequenceCount:       second & 0x3FFF,
	}, nil
}

// VerificationReport is any of the nine ST[01] reports: TM[1,1] to TM[1,8],
// plus the TM[1,10] failed routing verification report.
//
// Which fields carry meaning depends on the subtype: the progress reports
// (subtypes 5 and 6) add a step ID, and the failure reports (even subtypes,
// including TM[1,10]) add a failure notice.
type VerificationReport struct {
	Profile MissionProfile
	Subtype uint8

	// RequestID names the telecommand being reported on.
	RequestID RequestID

	// StepID is present on TM[1,5] and TM[1,6] only. Its width comes from the
	// profile, since Figures 8-5 and 8-6 mark it enumerated without one.
	StepID uint64

	// FailureCode and FailureData form the failure notice on the even
	// subtypes. FailureData is deduced from the code and carried verbatim.
	FailureCode uint64
	FailureData []byte
}

// IsFailure reports whether this subtype carries a failure notice. The even
// subtypes are the failures, per clause 8.1.2 — including TM[1,10], whose
// body is a request ID and a failure notice, like TM[1,2].
func (r *VerificationReport) IsFailure() bool { return r.Subtype%2 == 0 }

// HasStepID reports whether this subtype carries a step ID.
func (r *VerificationReport) HasStepID() bool {
	return r.Subtype == SubtypeProgressSuccess || r.Subtype == SubtypeProgressFailure
}

// Key returns the message type.
func (r *VerificationReport) Key() MessageKey {
	return MessageKey{Service: ServiceRequestVerification, Subtype: r.Subtype}
}

// Validate checks the report against clause 8.1.2. The valid subtypes are 1
// to 8 and 10; the standard defines no TM[1,9].
func (r *VerificationReport) Validate() error {
	switch {
	case r.Subtype >= SubtypeAcceptSuccess && r.Subtype <= SubtypeCompleteFailure:
	case r.Subtype == SubtypeRoutingFailure:
	default:
		return ErrWrongMessageType
	}
	return r.Profile.Validate()
}

// Encode serializes the source data field.
func (r *VerificationReport) Encode() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}

	out := r.RequestID.Encode()

	if r.HasStepID() {
		var err error
		if out, err = putUint(out, r.StepID, r.Profile.StepIDBytes); err != nil {
			return nil, err
		}
	}

	if r.IsFailure() {
		var err error
		if out, err = putUint(out, r.FailureCode, r.Profile.FailureCodeBytes); err != nil {
			return nil, err
		}
		out = append(out, r.FailureData...)
	}
	return out, nil
}

// DecodeVerificationReport parses an ST[01] report of the given subtype.
func DecodeVerificationReport(profile MissionProfile, subtype uint8, data []byte) (*VerificationReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	r := &VerificationReport{Profile: profile, Subtype: subtype}
	if err := r.Validate(); err != nil {
		return nil, err
	}

	id, err := DecodeRequestID(data)
	if err != nil {
		return nil, err
	}
	r.RequestID = id
	offset := RequestIDSize

	if r.HasStepID() {
		step, err := readUint(data[offset:], profile.StepIDBytes)
		if err != nil {
			return nil, err
		}
		r.StepID = step
		offset += profile.StepIDBytes
	}

	if r.IsFailure() {
		code, err := readUint(data[offset:], profile.FailureCodeBytes)
		if err != nil {
			return nil, err
		}
		r.FailureCode = code
		offset += profile.FailureCodeBytes

		if offset < len(data) {
			r.FailureData = make([]byte, len(data)-offset)
			copy(r.FailureData, data[offset:])
		}
	} else if offset != len(data) {
		// The success reports are fixed-size: a request ID, plus a step ID on
		// TM[1,5]. Octets beyond that are a malformed body, not padding.
		return nil, ErrTrailingBytes
	}
	return r, nil
}

// Humanize returns a human-readable summary.
func (r *VerificationReport) Humanize() string {
	out := "PUS TM[1," + itoa(int(r.Subtype)) + "] verification report" +
		"\n  Request APID .. " + itoa(int(r.RequestID.APID)) +
		"\n  Sequence ...... " + itoa(int(r.RequestID.SequenceCount))
	if r.HasStepID() {
		out += "\n  Step ID ....... " + itoa(int(r.StepID))
	}
	if r.IsFailure() {
		out += "\n  Failure code .. " + itoa(int(r.FailureCode))
	}
	return out
}

// registerST01 adds every ST[01] report decoder to a registry.
func registerST01(r *Registry) error {
	subtypes := []uint8{
		SubtypeAcceptSuccess, SubtypeAcceptFailure,
		SubtypeStartSuccess, SubtypeStartFailure,
		SubtypeProgressSuccess, SubtypeProgressFailure,
		SubtypeCompleteSuccess, SubtypeCompleteFailure,
		SubtypeRoutingFailure,
	}
	for _, subtype := range subtypes {
		sub := subtype // captured per iteration
		key := MessageKey{Service: ServiceRequestVerification, Subtype: sub}
		err := r.RegisterReport(key, func(p MissionProfile, data []byte) (Report, error) {
			return DecodeVerificationReport(p, sub, data)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
