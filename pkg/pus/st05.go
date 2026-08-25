package pus

// ST[05] event reporting, per ECSS-E-ST-70-41C clause 8.5.
//
// An on-board event produces a report at one of four severities. Each report
// carries an event definition ID and, optionally, auxiliary data whose
// structure that ID implies.
const (
	ServiceEventReporting uint8 = 5

	SubtypeInformativeEvent uint8 = 1 // TM[5,1] clause 8.5.2.1
	SubtypeLowSeverity      uint8 = 2 // TM[5,2] clause 8.5.2.2
	SubtypeMediumSeverity   uint8 = 3 // TM[5,3] clause 8.5.2.3
	SubtypeHighSeverity     uint8 = 4 // TM[5,4] clause 8.5.2.4
	SubtypeEnableEvents     uint8 = 5 // TC[5,5] clause 8.5.2.5
	SubtypeDisableEvents    uint8 = 6 // TC[5,6] clause 8.5.2.6
	SubtypeReportDisabled   uint8 = 7 // TC[5,7] clause 8.5.2.7
	SubtypeDisabledList     uint8 = 8 // TM[5,8] clause 8.5.2.8
)

// Severity names the four levels of ST[05] event report.
type Severity uint8

const (
	// SeverityInformative is a normal, expected event.
	SeverityInformative Severity = Severity(SubtypeInformativeEvent)
	// SeverityLow is a low severity anomaly.
	SeverityLow Severity = Severity(SubtypeLowSeverity)
	// SeverityMedium is a medium severity anomaly.
	SeverityMedium Severity = Severity(SubtypeMediumSeverity)
	// SeverityHigh is a high severity anomaly.
	SeverityHigh Severity = Severity(SubtypeHighSeverity)
)

// String names the severity.
func (s Severity) String() string {
	switch s {
	case SeverityInformative:
		return "informative"
	case SeverityLow:
		return "low severity anomaly"
	case SeverityMedium:
		return "medium severity anomaly"
	case SeverityHigh:
		return "high severity anomaly"
	default:
		return "unknown"
	}
}

// Valid reports whether the severity is one of the four the standard defines.
func (s Severity) Valid() bool {
	return s >= SeverityInformative && s <= SeverityHigh
}

// EventReport is TM[5,1] through TM[5,4], per Figure 8-59 and its siblings.
//
// The four subtypes share one structure and differ only in severity, so one
// type covers them all.
type EventReport struct {
	Profile  MissionProfile
	Severity Severity

	// EventDefinitionID, with the application process ID, identifies the
	// event definition and therefore the shape of the auxiliary data.
	EventDefinitionID uint64

	// AuxiliaryData is deduced from the event definition. This package carries
	// it verbatim rather than interpreting it.
	AuxiliaryData []byte
}

// Key returns the message type.
func (r *EventReport) Key() MessageKey {
	return MessageKey{Service: ServiceEventReporting, Subtype: uint8(r.Severity)}
}

// Validate checks the report.
func (r *EventReport) Validate() error {
	if !r.Severity.Valid() {
		return ErrInvalidSeverity
	}
	return r.Profile.Validate()
}

// Encode serializes the source data field.
func (r *EventReport) Encode() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, r.EventDefinitionID, r.Profile.EventDefinitionIDBytes)
	if err != nil {
		return nil, err
	}
	return append(out, r.AuxiliaryData...), nil
}

// DecodeEventReport parses an ST[05] event report of the given severity.
func DecodeEventReport(profile MissionProfile, severity Severity, data []byte) (*EventReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if !severity.Valid() {
		return nil, ErrInvalidSeverity
	}

	id, err := readUint(data, profile.EventDefinitionIDBytes)
	if err != nil {
		return nil, err
	}

	r := &EventReport{Profile: profile, Severity: severity, EventDefinitionID: id}
	if len(data) > profile.EventDefinitionIDBytes {
		r.AuxiliaryData = make([]byte, len(data)-profile.EventDefinitionIDBytes)
		copy(r.AuxiliaryData, data[profile.EventDefinitionIDBytes:])
	}
	return r, nil
}

// Humanize returns a human-readable summary.
func (r *EventReport) Humanize() string {
	return "PUS TM[5," + itoa(int(r.Severity)) + "] " + r.Severity.String() +
		"\n  Event ID ....... " + itoa(int(r.EventDefinitionID)) +
		"\n  Auxiliary data . " + itoa(len(r.AuxiliaryData)) + " octets"
}

// EventControlRequest is TC[5,5] and TC[5,6]: enable or disable the report
// generation of a list of event definitions.
type EventControlRequest struct {
	Profile MissionProfile
	// Enable selects TC[5,5] when true and TC[5,6] when false.
	Enable bool
	// EventDefinitionIDs names the events to enable or disable.
	EventDefinitionIDs []uint64
}

// Key returns the message type.
func (r *EventControlRequest) Key() MessageKey {
	subtype := SubtypeDisableEvents
	if r.Enable {
		subtype = SubtypeEnableEvents
	}
	return MessageKey{Service: ServiceEventReporting, Subtype: subtype}
}

// Encode serializes the application data field: a count followed by that many
// event definition IDs.
func (r *EventControlRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}

	out, err := putUint(nil, uint64(len(r.EventDefinitionIDs)), r.Profile.CountBytes)
	if err != nil {
		return nil, err
	}
	for _, id := range r.EventDefinitionIDs {
		if out, err = putUint(out, id, r.Profile.EventDefinitionIDBytes); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeEventControlRequest parses TC[5,5] or TC[5,6].
func DecodeEventControlRequest(profile MissionProfile, enable bool, data []byte) (*EventControlRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	ids, n, err := readUintList(data, profile.CountBytes, profile.EventDefinitionIDBytes)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, ErrTrailingBytes
	}
	return &EventControlRequest{Profile: profile, Enable: enable, EventDefinitionIDs: ids}, nil
}

// Humanize returns a human-readable summary.
func (r *EventControlRequest) Humanize() string {
	verb := "disable"
	if r.Enable {
		verb = "enable"
	}
	return "PUS TC[5," + itoa(int(r.Key().Subtype)) + "] " + verb + " event report generation" +
		"\n  Events ... " + itoa(len(r.EventDefinitionIDs))
}

// ReportDisabledEventsRequest is TC[5,7]: report the list of event definitions
// whose report generation is disabled, per clause 8.5.2.7. Its application
// data field is empty; the answer is a TM[5,8] report.
type ReportDisabledEventsRequest struct{}

// Key returns the message type.
func (ReportDisabledEventsRequest) Key() MessageKey {
	return MessageKey{Service: ServiceEventReporting, Subtype: SubtypeReportDisabled}
}

// Encode returns an empty application data field.
func (ReportDisabledEventsRequest) Encode() ([]byte, error) { return nil, nil }

// DecodeReportDisabledEventsRequest parses TC[5,7], whose body is empty.
func DecodeReportDisabledEventsRequest(data []byte) (*ReportDisabledEventsRequest, error) {
	if len(data) != 0 {
		return nil, ErrTrailingBytes
	}
	return &ReportDisabledEventsRequest{}, nil
}

// Humanize returns a human-readable summary.
func (ReportDisabledEventsRequest) Humanize() string {
	return "PUS TC[5,7] report the disabled event definitions"
}

// DisabledEventsReport is TM[5,8]: the disabled event definitions list report,
// per clause 8.5.2.8. It carries a count followed by that many event
// definition IDs, the answer to TC[5,7].
type DisabledEventsReport struct {
	Profile MissionProfile
	// EventDefinitionIDs names the events whose report generation is disabled.
	EventDefinitionIDs []uint64
}

// Key returns the message type.
func (r *DisabledEventsReport) Key() MessageKey {
	return MessageKey{Service: ServiceEventReporting, Subtype: SubtypeDisabledList}
}

// Encode serializes the source data field: a count followed by that many
// event definition IDs.
func (r *DisabledEventsReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.EventDefinitionIDs)), r.Profile.CountBytes)
	if err != nil {
		return nil, err
	}
	for _, id := range r.EventDefinitionIDs {
		if out, err = putUint(out, id, r.Profile.EventDefinitionIDBytes); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeDisabledEventsReport parses TM[5,8].
func DecodeDisabledEventsReport(profile MissionProfile, data []byte) (*DisabledEventsReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	ids, n, err := readUintList(data, profile.CountBytes, profile.EventDefinitionIDBytes)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, ErrTrailingBytes
	}
	return &DisabledEventsReport{Profile: profile, EventDefinitionIDs: ids}, nil
}

// Humanize returns a human-readable summary.
func (r *DisabledEventsReport) Humanize() string {
	return "PUS TM[5,8] disabled event definitions list report" +
		"\n  Events ... " + itoa(len(r.EventDefinitionIDs))
}

// registerST05 adds the ST[05] codecs to a registry.
func registerST05(r *Registry) error {
	for _, severity := range []Severity{SeverityInformative, SeverityLow, SeverityMedium, SeverityHigh} {
		sev := severity
		key := MessageKey{Service: ServiceEventReporting, Subtype: uint8(sev)}
		err := r.RegisterReport(key, func(p MissionProfile, data []byte) (Report, error) {
			return DecodeEventReport(p, sev, data)
		})
		if err != nil {
			return err
		}
	}

	if err := r.RegisterRequest(
		MessageKey{Service: ServiceEventReporting, Subtype: SubtypeEnableEvents},
		func(p MissionProfile, data []byte) (Request, error) {
			return DecodeEventControlRequest(p, true, data)
		},
	); err != nil {
		return err
	}
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceEventReporting, Subtype: SubtypeDisableEvents},
		func(p MissionProfile, data []byte) (Request, error) {
			return DecodeEventControlRequest(p, false, data)
		},
	); err != nil {
		return err
	}
	if err := r.RegisterRequest(
		MessageKey{Service: ServiceEventReporting, Subtype: SubtypeReportDisabled},
		func(_ MissionProfile, data []byte) (Request, error) {
			return DecodeReportDisabledEventsRequest(data)
		},
	); err != nil {
		return err
	}
	return r.RegisterReport(
		MessageKey{Service: ServiceEventReporting, Subtype: SubtypeDisabledList},
		func(p MissionProfile, data []byte) (Report, error) {
			return DecodeDisabledEventsReport(p, data)
		},
	)
}
