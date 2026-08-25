package pus

// ST[17] test, per ECSS-E-ST-70-41C clause 8.17.
//
// The simplest service in the standard: a liveness check. The ground asks
// whether an application process is alive and it answers. Neither message
// carries a body.
const (
	ServiceTest uint8 = 17

	SubtypeAreYouAlive       uint8 = 1 // TC[17,1] clause 8.17.2.1
	SubtypeAreYouAliveReport uint8 = 2 // TM[17,2] clause 8.17.2.2
	SubtypeOnBoardConnection uint8 = 3 // TC[17,3] clause 8.17.2.3
	SubtypeOnBoardReport     uint8 = 4 // TM[17,4] clause 8.17.2.4
)

// AreYouAliveRequest is TC[17,1]. Its application data field is empty.
type AreYouAliveRequest struct{}

// Key returns the message type.
func (AreYouAliveRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAlive}
}

// Encode returns an empty application data field.
func (AreYouAliveRequest) Encode() ([]byte, error) { return nil, nil }

// Humanize returns a human-readable summary.
func (AreYouAliveRequest) Humanize() string { return "PUS TC[17,1] are-you-alive connection test" }

// AreYouAliveReport is TM[17,2]. Its source data field is empty.
type AreYouAliveReport struct{}

// Key returns the message type.
func (AreYouAliveReport) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAliveReport}
}

// Encode returns an empty source data field.
func (AreYouAliveReport) Encode() ([]byte, error) { return nil, nil }

// Humanize returns a human-readable summary.
func (AreYouAliveReport) Humanize() string {
	return "PUS TM[17,2] are-you-alive connection test report"
}

// OnBoardConnectionRequest is TC[17,3]: a connection test addressed to another
// application process on board. The APID of the process under test travels in
// the application data field, at the width the profile's APIDBytes declares
// (two octets when unset).
type OnBoardConnectionRequest struct {
	Profile MissionProfile
	// APID identifies the application process to test.
	APID uint16
}

// Key returns the message type.
func (OnBoardConnectionRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardConnection}
}

// Encode serializes the application data field.
func (r OnBoardConnectionRequest) Encode() ([]byte, error) {
	return putUint(nil, uint64(r.APID), r.Profile.APIDSize())
}

// DecodeOnBoardConnectionRequest parses TC[17,3].
func DecodeOnBoardConnectionRequest(profile MissionProfile, data []byte) (*OnBoardConnectionRequest, error) {
	apid, err := decodeTestAPID(profile, data)
	if err != nil {
		return nil, err
	}
	return &OnBoardConnectionRequest{Profile: profile, APID: apid}, nil
}

// Humanize returns a human-readable summary.
func (r OnBoardConnectionRequest) Humanize() string {
	return "PUS TC[17,3] on-board connection test\n  APID .... " + itoa(int(r.APID))
}

// OnBoardConnectionReport is TM[17,4], the answer to TC[17,3]. Its APID field
// uses the same profile-declared width as the request.
type OnBoardConnectionReport struct {
	Profile MissionProfile
	// APID identifies the application process that was tested.
	APID uint16
}

// Key returns the message type.
func (OnBoardConnectionReport) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardReport}
}

// Encode serializes the source data field.
func (r OnBoardConnectionReport) Encode() ([]byte, error) {
	return putUint(nil, uint64(r.APID), r.Profile.APIDSize())
}

// DecodeOnBoardConnectionReport parses TM[17,4].
func DecodeOnBoardConnectionReport(profile MissionProfile, data []byte) (*OnBoardConnectionReport, error) {
	apid, err := decodeTestAPID(profile, data)
	if err != nil {
		return nil, err
	}
	return &OnBoardConnectionReport{Profile: profile, APID: apid}, nil
}

// decodeTestAPID reads the single APID field that makes up a TC[17,3] or
// TM[17,4] body. The body is fixed-size, so trailing octets are refused, and
// an APID never exceeds the 11 bits CCSDS gives it, so a value past 16 bits
// cannot be one.
func decodeTestAPID(profile MissionProfile, data []byte) (uint16, error) {
	if err := profile.Validate(); err != nil {
		return 0, err
	}
	width := profile.APIDSize()
	apid, err := readUint(data, width)
	if err != nil {
		return 0, err
	}
	if len(data) != width {
		return 0, ErrTrailingBytes
	}
	if apid > 0xFFFF {
		return 0, ErrValueTooLarge
	}
	return uint16(apid), nil
}

// Humanize returns a human-readable summary.
func (r OnBoardConnectionReport) Humanize() string {
	return "PUS TM[17,4] on-board connection test report\n  APID .... " + itoa(int(r.APID))
}

// registerST17 adds the ST[17] codecs to a registry.
func registerST17(r *Registry) error {
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAlive},
		func(_ MissionProfile, data []byte) (Request, error) {
			if len(data) != 0 {
				return nil, ErrTrailingBytes
			}
			return AreYouAliveRequest{}, nil
		},
	); err != nil {
		return err
	}
	if err := r.RegisterReport(
		MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAliveReport},
		func(_ MissionProfile, data []byte) (Report, error) {
			if len(data) != 0 {
				return nil, ErrTrailingBytes
			}
			return AreYouAliveReport{}, nil
		},
	); err != nil {
		return err
	}
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardConnection},
		func(p MissionProfile, data []byte) (Request, error) {
			return DecodeOnBoardConnectionRequest(p, data)
		},
	); err != nil {
		return err
	}
	return r.RegisterReport(
		MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardReport},
		func(p MissionProfile, data []byte) (Report, error) {
			return DecodeOnBoardConnectionReport(p, data)
		},
	)
}
