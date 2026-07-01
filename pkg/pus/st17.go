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
// the application data field.
type OnBoardConnectionRequest struct {
	// APID identifies the application process to test.
	APID uint16
}

// Key returns the message type.
func (OnBoardConnectionRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardConnection}
}

// Encode serializes the application data field.
func (r OnBoardConnectionRequest) Encode() ([]byte, error) {
	return putUint(nil, uint64(r.APID), 2)
}

// DecodeOnBoardConnectionRequest parses TC[17,3].
func DecodeOnBoardConnectionRequest(data []byte) (*OnBoardConnectionRequest, error) {
	apid, err := readUint(data, 2)
	if err != nil {
		return nil, err
	}
	return &OnBoardConnectionRequest{APID: uint16(apid)}, nil
}

// Humanize returns a human-readable summary.
func (r OnBoardConnectionRequest) Humanize() string {
	return "PUS TC[17,3] on-board connection test\n  APID .... " + itoa(int(r.APID))
}

// OnBoardConnectionReport is TM[17,4], the answer to TC[17,3].
type OnBoardConnectionReport struct {
	// APID identifies the application process that was tested.
	APID uint16
}

// Key returns the message type.
func (OnBoardConnectionReport) Key() MessageKey {
	return MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardReport}
}

// Encode serializes the source data field.
func (r OnBoardConnectionReport) Encode() ([]byte, error) {
	return putUint(nil, uint64(r.APID), 2)
}

// DecodeOnBoardConnectionReport parses TM[17,4].
func DecodeOnBoardConnectionReport(data []byte) (*OnBoardConnectionReport, error) {
	apid, err := readUint(data, 2)
	if err != nil {
		return nil, err
	}
	return &OnBoardConnectionReport{APID: uint16(apid)}, nil
}

// Humanize returns a human-readable summary.
func (r OnBoardConnectionReport) Humanize() string {
	return "PUS TM[17,4] on-board connection test report\n  APID .... " + itoa(int(r.APID))
}

// registerST17 adds the ST[17] codecs to a registry.
func registerST17(r *Registry) error {
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAlive},
		func(MissionProfile, []byte) (Request, error) { return AreYouAliveRequest{}, nil },
	); err != nil {
		return err
	}
	if err := r.RegisterReport(
		MessageKey{Service: ServiceTest, Subtype: SubtypeAreYouAliveReport},
		func(MissionProfile, []byte) (Report, error) { return AreYouAliveReport{}, nil },
	); err != nil {
		return err
	}
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardConnection},
		func(_ MissionProfile, data []byte) (Request, error) {
			return DecodeOnBoardConnectionRequest(data)
		},
	); err != nil {
		return err
	}
	return r.RegisterReport(
		MessageKey{Service: ServiceTest, Subtype: SubtypeOnBoardReport},
		func(_ MissionProfile, data []byte) (Report, error) {
			return DecodeOnBoardConnectionReport(data)
		},
	)
}
