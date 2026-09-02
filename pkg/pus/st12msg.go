package pus

import "time"

// The twenty-eight ST[12] message types.
//
// Twenty-one of them are plain: an ID list, a status list, a single integer or
// nothing at all. Seven carry fields sized by the mission's parameter
// definitions and take a ParameterResolver; see st12.go.

// monitorControlNames labels the eight ST[12] requests whose application data
// field "shall be omitted".
var monitorControlNames = map[uint8]string{
	SubtypeDeleteAllPMON:       "delete all parameter monitoring definitions",
	SubtypeReportOutOfLimits:   "report the out-of-limits",
	SubtypeReportPMONStatus:    "report the status of each parameter monitoring definition",
	SubtypeEnablePMONFunction:  "enable the parameter monitoring function",
	SubtypeDisablePMONFunction: "disable the parameter monitoring function",
	SubtypeEnableFMONFunction:  "enable the functional monitoring function",
	SubtypeDisableFMONFunction: "disable the functional monitoring function",
	SubtypeReportFMONStatus:    "report the status of each functional monitoring definition",
}

// MonitorControlRequest is any of the eight ST[12] requests that carry no
// body.
//
// TC[12,4] is worth singling out: it deletes every parameter monitoring
// definition, and clause 6.12.3.9.2 gives it no arguments at all. There is no
// selective form of it, TC[12,6] is that.
type MonitorControlRequest struct {
	// Subtype is one of the eight; anything else is refused on encode.
	Subtype uint8
}

// Key returns the message type.
func (r MonitorControlRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: r.Subtype}
}

// Encode returns an empty application data field.
func (r MonitorControlRequest) Encode() ([]byte, error) {
	if _, ok := monitorControlNames[r.Subtype]; !ok {
		return nil, ErrWrongMessageType
	}
	return nil, nil
}

// Humanize returns a human-readable summary.
func (r MonitorControlRequest) Humanize() string {
	name, ok := monitorControlNames[r.Subtype]
	if !ok {
		name = "unknown"
	}
	return "PUS TC[12," + itoa(int(r.Subtype)) + "] " + name
}

// pmonIDListNames labels the four requests that carry a list of PMON IDs.
var pmonIDListNames = map[uint8]string{
	SubtypeEnablePMONDefinitions:  "enable parameter monitoring definitions",
	SubtypeDisablePMONDefinitions: "disable parameter monitoring definitions",
	SubtypeDeletePMONDefinitions:  "delete parameter monitoring definitions",
	SubtypeReportPMONDefinitions:  "report parameter monitoring definitions",
}

// fmonIDListNames labels the six requests that carry a list of FMON IDs.
var fmonIDListNames = map[uint8]string{
	SubtypeEnableFMONDefinitions:   "enable functional monitoring definitions",
	SubtypeDisableFMONDefinitions:  "disable functional monitoring definitions",
	SubtypeProtectFMONDefinitions:  "protect functional monitoring definitions",
	SubtypeUnprotectFMONDefinition: "unprotect functional monitoring definitions",
	SubtypeDeleteFMONDefinitions:   "delete functional monitoring definitions",
	SubtypeReportFMONDefinitions:   "report functional monitoring definitions",
}

// MonitorIDListRequest is any of the ten ST[12] requests whose body is a
// count and a list of identifiers, per Figures 8-111, 8-112, 8-118, 8-123,
// 8-131 to 8-134, 8-136 and 8-137.
//
// Four carry parameter monitoring IDs and six carry functional monitoring
// IDs; the subtype says which, and the two are sized separately.
//
// Two of the ten give an empty list a meaning: clauses 8.12.2.8c and
// 8.12.2.25c say that setting N to 0 reports all definitions. The other eight
// say nothing about it, so IsAll only reports true for those two. An empty
// enable request is an empty enable request, not a request to enable
// everything.
type MonitorIDListRequest struct {
	Profile MissionProfile
	// Subtype is one of the ten.
	Subtype uint8
	// IDs are the definitions the request names.
	IDs []uint64
}

// Key returns the message type.
func (r MonitorIDListRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: r.Subtype}
}

// idWidth returns the element width for this subtype.
func (r MonitorIDListRequest) idWidth() int {
	if _, ok := pmonIDListNames[r.Subtype]; ok {
		return r.Profile.PMONIDSize()
	}
	return r.Profile.FMONIDSize()
}

// IsAll reports whether an empty list means "all definitions" for this
// subtype, which only TC[12,8] and TC[12,25] say it does.
func (r MonitorIDListRequest) IsAll() bool {
	if len(r.IDs) != 0 {
		return false
	}
	return r.Subtype == SubtypeReportPMONDefinitions || r.Subtype == SubtypeReportFMONDefinitions
}

// Encode serializes the application data field.
func (r MonitorIDListRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	if !monitorIDListSubtype(r.Subtype) {
		return nil, ErrWrongMessageType
	}
	return encodeUintList(nil, r.IDs, r.Profile.MonitorCountSize(), r.idWidth())
}

// DecodeMonitorIDListRequest parses one of the ten ID-list requests. The
// subtype has to be supplied because the octets do not say which of the ten
// they are, and because it decides whether the IDs are PMON or FMON IDs.
func DecodeMonitorIDListRequest(profile MissionProfile, subtype uint8, data []byte) (*MonitorIDListRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if !monitorIDListSubtype(subtype) {
		return nil, ErrWrongMessageType
	}
	request := &MonitorIDListRequest{Profile: profile, Subtype: subtype}
	ids, used, err := readUintList(data, profile.MonitorCountSize(), request.idWidth())
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	request.IDs = ids
	return request, nil
}

// monitorIDListSubtype reports whether this subtype is one of the ten.
func monitorIDListSubtype(subtype uint8) bool {
	_, pmon := pmonIDListNames[subtype]
	_, fmon := fmonIDListNames[subtype]
	return pmon || fmon
}

// Humanize returns a human-readable summary.
func (r MonitorIDListRequest) Humanize() string {
	name, ok := pmonIDListNames[r.Subtype]
	if !ok {
		name = fmonIDListNames[r.Subtype]
	}
	if name == "" {
		name = "unknown"
	}
	out := "PUS TC[12," + itoa(int(r.Subtype)) + "] " + name
	if r.IsAll() {
		return out + "\n  Scope ......... all (N = 0)"
	}
	return out + "\n  Definitions ... " + itoa(len(r.IDs))
}

// ChangeTransitionDelayRequest is TC[12,3], per Figure 8-113: one unsigned
// integer and nothing else.
type ChangeTransitionDelayRequest struct {
	Profile MissionProfile
	// MaxReportingDelay is the new maximum transition reporting delay.
	MaxReportingDelay uint64
}

// Key returns the message type.
func (ChangeTransitionDelayRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeChangeTransitionDelay}
}

// Encode serializes the application data field.
func (r ChangeTransitionDelayRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	return putUint(nil, r.MaxReportingDelay, r.Profile.TransitionDelaySize())
}

// DecodeChangeTransitionDelayRequest parses TC[12,3].
func DecodeChangeTransitionDelayRequest(profile MissionProfile, data []byte) (*ChangeTransitionDelayRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	width := profile.TransitionDelaySize()
	delay, err := readUint(data, width)
	if err != nil {
		return nil, err
	}
	if len(data) != width {
		return nil, ErrTrailingBytes
	}
	return &ChangeTransitionDelayRequest{Profile: profile, MaxReportingDelay: delay}, nil
}

// Humanize returns a human-readable summary.
func (r ChangeTransitionDelayRequest) Humanize() string {
	return "PUS TC[12,3] change the maximum transition reporting delay" +
		"\n  Delay ......... " + itoa(int(r.MaxReportingDelay))
}

// PMONStatusEntry pairs a parameter monitoring definition with its status.
type PMONStatusEntry struct {
	// ID names the definition.
	ID uint64
	// Status is whether it is enabled, per Table 8-10.
	Status PMONStatus
}

// PMONStatusReport is TM[12,14], per Figure 8-130.
type PMONStatusReport struct {
	Profile MissionProfile
	// Definitions are the parameter monitoring definitions and their statuses.
	Definitions []PMONStatusEntry
}

// Key returns the message type.
func (PMONStatusReport) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypePMONStatusReport}
}

// Encode serializes the source data field.
func (r PMONStatusReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Definitions)), r.Profile.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, entry := range r.Definitions {
		if out, err = putUint(out, entry.ID, r.Profile.PMONIDSize()); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(entry.Status), r.Profile.PMONStatusSize()); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodePMONStatusReport parses TM[12,14].
func DecodePMONStatusReport(profile MissionProfile, data []byte) (*PMONStatusReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	countWidth := profile.MonitorCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, err
	}
	offset := countWidth

	report := &PMONStatusReport{Profile: profile}
	entryWidth := profile.PMONIDSize() + profile.PMONStatusSize()
	if count > 0 {
		if uint64(len(data)-offset)/uint64(entryWidth) < count {
			return nil, ErrDataTooShort
		}
		report.Definitions = make([]PMONStatusEntry, 0, count)
	}

	for i := uint64(0); i < count; i++ {
		id, err := readUint(data[offset:], profile.PMONIDSize())
		if err != nil {
			return nil, err
		}
		offset += profile.PMONIDSize()
		status, err := readUint(data[offset:], profile.PMONStatusSize())
		if err != nil {
			return nil, err
		}
		offset += profile.PMONStatusSize()
		report.Definitions = append(report.Definitions,
			PMONStatusEntry{ID: id, Status: PMONStatus(status)})
	}

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return report, nil
}

// Humanize returns a human-readable summary.
func (r PMONStatusReport) Humanize() string {
	out := "PUS TM[12,14] parameter monitoring definition status report" +
		"\n  Definitions ... " + itoa(len(r.Definitions))
	for _, entry := range r.Definitions {
		out += "\n    PMON " + itoa(int(entry.ID)) + ": " + entry.Status.String()
	}
	return out
}

// FMONStatusEntry is one entry of TM[12,28], per Figure 8-139.
type FMONStatusEntry struct {
	// ID names the functional monitoring definition.
	ID uint64
	// ProtectionStatus is whether the definition is protected from deletion,
	// per Table 8-11. Present only when the profile sets
	// SupportsFMONProtection.
	ProtectionStatus FMONProtectionStatus
	// Status is whether the definition is enabled, per Table 8-12.
	Status FMONStatus
	// CheckingStatus is the outcome of its last evaluation, per Table 8-13.
	CheckingStatus FMONCheckingStatus
}

// FMONStatusReport is TM[12,28], per Figure 8-139.
type FMONStatusReport struct {
	Profile MissionProfile
	// Definitions are the functional monitoring definitions and their
	// statuses.
	Definitions []FMONStatusEntry
}

// Key returns the message type.
func (FMONStatusReport) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeFMONStatusReport}
}

// fmonStatusEntrySize is the encoded width of one TM[12,28] entry.
func fmonStatusEntrySize(p MissionProfile) int {
	size := p.FMONIDSize() + p.FMONStatusSize() + p.FMONCheckingStatusSize()
	if p.SupportsFMONProtection {
		size += p.FMONProtectionStatusSize()
	}
	return size
}

// Encode serializes the source data field.
func (r FMONStatusReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Definitions)), r.Profile.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, entry := range r.Definitions {
		if out, err = putUint(out, entry.ID, r.Profile.FMONIDSize()); err != nil {
			return nil, err
		}
		if r.Profile.SupportsFMONProtection {
			if out, err = putUint(out, uint64(entry.ProtectionStatus),
				r.Profile.FMONProtectionStatusSize()); err != nil {
				return nil, err
			}
		}
		if out, err = putUint(out, uint64(entry.Status), r.Profile.FMONStatusSize()); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(entry.CheckingStatus),
			r.Profile.FMONCheckingStatusSize()); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeFMONStatusReport parses TM[12,28].
func DecodeFMONStatusReport(profile MissionProfile, data []byte) (*FMONStatusReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	countWidth := profile.MonitorCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, err
	}
	offset := countWidth

	report := &FMONStatusReport{Profile: profile}
	if count > 0 {
		if uint64(len(data)-offset)/uint64(fmonStatusEntrySize(profile)) < count {
			return nil, ErrDataTooShort
		}
		report.Definitions = make([]FMONStatusEntry, 0, count)
	}

	for i := uint64(0); i < count; i++ {
		var entry FMONStatusEntry
		read := func(width int) (uint64, error) {
			v, err := readUint(data[offset:], width)
			offset += width
			return v, err
		}

		id, err := read(profile.FMONIDSize())
		if err != nil {
			return nil, err
		}
		entry.ID = id

		if profile.SupportsFMONProtection {
			protection, err := read(profile.FMONProtectionStatusSize())
			if err != nil {
				return nil, err
			}
			entry.ProtectionStatus = FMONProtectionStatus(protection)
		}

		status, err := read(profile.FMONStatusSize())
		if err != nil {
			return nil, err
		}
		entry.Status = FMONStatus(status)

		checking, err := read(profile.FMONCheckingStatusSize())
		if err != nil {
			return nil, err
		}
		entry.CheckingStatus = FMONCheckingStatus(checking)

		report.Definitions = append(report.Definitions, entry)
	}

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return report, nil
}

// Humanize returns a human-readable summary.
func (r FMONStatusReport) Humanize() string {
	out := "PUS TM[12,28] functional monitoring definition status report" +
		"\n  Definitions ... " + itoa(len(r.Definitions))
	for _, entry := range r.Definitions {
		out += "\n    FMON " + itoa(int(entry.ID)) + ": " + entry.Status.String() +
			", " + entry.CheckingStatus.String()
		if r.Profile.SupportsFMONProtection {
			out += ", " + entry.ProtectionStatus.String()
		}
	}
	return out
}

// PMONDefinition is one parameter monitoring definition, per clause 6.12.3.3g
// and Figures 8-114, 8-119 and 8-124.
//
// One type serves the add request, the modify request and the report, because
// they differ only in which fields they carry, and the differences are worth
// naming. The add request carries the check validity condition and the
// monitoring interval; the modify request carries neither, because clause
// 6.12.3.9.4 modifies a check rather than replacing a definition. The report
// carries both plus the PMON status.
type PMONDefinition struct {
	// ID names the definition.
	ID uint64

	// MonitoredParameterID names the on-board parameter being watched. Its
	// layout gives the widths of every deduced field in Criteria.
	MonitoredParameterID uint64

	// Validity is the check validity condition. Carried by the add request
	// and the report, and only when the profile sets
	// SupportsConditionalChecking.
	Validity ValidityCondition

	// MonitoringInterval is how long between two evaluations, in on-board
	// parameter minimum sampling interval units (clause 6.12.3.3f). Carried
	// by the add request and the report, and only when the profile sets
	// PerDefinitionMonitoringInterval.
	MonitoringInterval uint64

	// Status is whether the definition is enabled. Carried by the report
	// only.
	Status PMONStatus

	// RepetitionNumber is how many consecutive consistent checks establish a
	// new checking status (clause 6.12.3.3j item 1). Always carried.
	RepetitionNumber uint64

	// Criteria is the check itself.
	Criteria CheckCriteria
}

// definitionShape says which of the three PMON definition layouts to use.
type definitionShape int

const (
	// shapeAdd is Figure 8-114: validity condition and monitoring interval,
	// no status.
	shapeAdd definitionShape = iota
	// shapeModify is Figure 8-119: neither validity condition nor monitoring
	// interval nor status.
	shapeModify
	// shapeReport is Figure 8-124: validity condition, monitoring interval
	// and status.
	shapeReport
)

// encodePMONDefinition appends one definition in the given shape.
func encodePMONDefinition(dst []byte, p MissionProfile, d PMONDefinition, shape definitionShape, resolve ParameterResolver) ([]byte, error) {
	// The criteria are checked before anything is written. A mismatch between
	// the check type and the alternative that is set is a mistake in the
	// caller's own value, and reporting it as a field-width problem three
	// fields later would send them looking in the wrong place.
	if err := d.Criteria.validate(); err != nil {
		return nil, err
	}

	layout, err := resolveLayout(resolve, d.MonitoredParameterID)
	if err != nil {
		return nil, err
	}

	out, err := putUint(dst, d.ID, p.PMONIDSize())
	if err != nil {
		return nil, err
	}
	if out, err = putUint(out, d.MonitoredParameterID, p.ParameterIDSize()); err != nil {
		return nil, err
	}

	if shape != shapeModify && p.SupportsConditionalChecking {
		if out, err = encodeValidityCondition(out, p, d.Validity, resolve); err != nil {
			return nil, err
		}
	}
	if shape != shapeModify && p.PerDefinitionMonitoringInterval {
		if out, err = putUint(out, d.MonitoringInterval, p.MonitoringIntervalSize()); err != nil {
			return nil, err
		}
	}
	if shape == shapeReport {
		if out, err = putUint(out, uint64(d.Status), p.PMONStatusSize()); err != nil {
			return nil, err
		}
	}

	if out, err = putUint(out, d.RepetitionNumber, p.RepetitionNumberSize()); err != nil {
		return nil, err
	}
	if out, err = putUint(out, uint64(d.Criteria.Type), p.CheckTypeSize()); err != nil {
		return nil, err
	}
	return d.Criteria.encode(out, p, layout)
}

// decodePMONDefinition reads one definition in the given shape.
func decodePMONDefinition(p MissionProfile, data []byte, shape definitionShape, resolve ParameterResolver) (PMONDefinition, int, error) {
	var definition PMONDefinition
	offset := 0

	read := func(width int) (uint64, error) {
		v, err := readUint(data[offset:], width)
		offset += width
		return v, err
	}

	id, err := read(p.PMONIDSize())
	if err != nil {
		return PMONDefinition{}, 0, err
	}
	definition.ID = id

	parameterID, err := read(p.ParameterIDSize())
	if err != nil {
		return PMONDefinition{}, 0, err
	}
	definition.MonitoredParameterID = parameterID

	layout, err := resolveLayout(resolve, parameterID)
	if err != nil {
		return PMONDefinition{}, 0, err
	}

	if shape != shapeModify && p.SupportsConditionalChecking {
		condition, used, err := decodeValidityCondition(p, data[offset:], resolve)
		if err != nil {
			return PMONDefinition{}, 0, err
		}
		definition.Validity = condition
		offset += used
	}
	if shape != shapeModify && p.PerDefinitionMonitoringInterval {
		interval, err := read(p.MonitoringIntervalSize())
		if err != nil {
			return PMONDefinition{}, 0, err
		}
		definition.MonitoringInterval = interval
	}
	if shape == shapeReport {
		status, err := read(p.PMONStatusSize())
		if err != nil {
			return PMONDefinition{}, 0, err
		}
		definition.Status = PMONStatus(status)
	}

	repetition, err := read(p.RepetitionNumberSize())
	if err != nil {
		return PMONDefinition{}, 0, err
	}
	definition.RepetitionNumber = repetition

	rawCheck, err := read(p.CheckTypeSize())
	if err != nil {
		return PMONDefinition{}, 0, err
	}
	check := CheckType(rawCheck)
	if !check.valid() {
		return PMONDefinition{}, 0, ErrInvalidCheckType
	}

	criteria, used, err := decodeCheckCriteria(p, check, layout, data[offset:])
	if err != nil {
		return PMONDefinition{}, 0, err
	}
	definition.Criteria = criteria
	offset += used

	return definition, offset, nil
}

// minPMONDefinitionSize is the smallest a definition of this shape can be,
// used to bound an untrusted count before anything is allocated. A definition
// whose deduced fields are all zero-width is still at least this long.
func minPMONDefinitionSize(p MissionProfile, shape definitionShape) int {
	size := p.PMONIDSize() + p.ParameterIDSize() +
		p.RepetitionNumberSize() + p.CheckTypeSize() +
		// The narrowest criteria: two event definition IDs, for a limit check
		// over a zero-width parameter.
		2*p.EventDefinitionIDSize()
	if shape != shapeModify && p.SupportsConditionalChecking {
		size += p.ParameterIDSize()
	}
	if shape != shapeModify && p.PerDefinitionMonitoringInterval {
		size += p.MonitoringIntervalSize()
	}
	if shape == shapeReport {
		size += p.PMONStatusSize()
	}
	return size
}

// readPMONDefinitions reads a count-prefixed list of definitions.
//
// A definition is variable-length, so nothing is sized from the count: the
// list is walked until the octets run out, with the count bounded first
// against the shortest a definition can be.
func readPMONDefinitions(p MissionProfile, data []byte, shape definitionShape, resolve ParameterResolver) ([]PMONDefinition, int, error) {
	countWidth := p.MonitorCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, 0, err
	}
	offset := countWidth
	if count == 0 {
		return nil, offset, nil
	}

	minimum := minPMONDefinitionSize(p, shape)
	if minimum <= 0 {
		return nil, 0, ErrInvalidProfile
	}
	if uint64(len(data)-offset)/uint64(minimum) < count {
		return nil, 0, ErrDataTooShort
	}

	definitions := make([]PMONDefinition, 0, count)
	for i := uint64(0); i < count; i++ {
		definition, used, err := decodePMONDefinition(p, data[offset:], shape, resolve)
		if err != nil {
			return nil, 0, err
		}
		definitions = append(definitions, definition)
		offset += used
	}
	return definitions, offset, nil
}

// AddPMONDefinitionsRequest is TC[12,5], per Figure 8-114.
type AddPMONDefinitionsRequest struct {
	Profile MissionProfile
	// Resolve gives the widths of the deduced fields. Required.
	Resolve ParameterResolver
	// Definitions are the parameter monitoring definitions to add.
	Definitions []PMONDefinition
}

// Key returns the message type.
func (AddPMONDefinitionsRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeAddPMONDefinitions}
}

// Encode serializes the application data field.
func (r AddPMONDefinitionsRequest) Encode() ([]byte, error) {
	return encodeDefinitionList(r.Profile, r.Definitions, shapeAdd, r.Resolve)
}

// DecodeAddPMONDefinitionsRequest parses TC[12,5].
func DecodeAddPMONDefinitionsRequest(profile MissionProfile, resolve ParameterResolver, data []byte) (*AddPMONDefinitionsRequest, error) {
	definitions, err := decodeDefinitionList(profile, data, shapeAdd, resolve)
	if err != nil {
		return nil, err
	}
	return &AddPMONDefinitionsRequest{Profile: profile, Resolve: resolve, Definitions: definitions}, nil
}

// Humanize returns a human-readable summary.
func (r AddPMONDefinitionsRequest) Humanize() string {
	return "PUS TC[12,5] add parameter monitoring definitions" +
		humanizePMONDefinitions(r.Definitions)
}

// ModifyPMONDefinitionsRequest is TC[12,7], per Figure 8-119.
//
// It carries neither a check validity condition nor a monitoring interval:
// clause 6.12.3.9.4 modifies the check in an existing definition rather than
// replacing the definition. Clause 8.12.2.7c adds that the check type must
// match the definition's original one, which only the flight software can
// verify. It holds the original.
type ModifyPMONDefinitionsRequest struct {
	Profile MissionProfile
	// Resolve gives the widths of the deduced fields. Required.
	Resolve ParameterResolver
	// Definitions are the modifications to apply.
	Definitions []PMONDefinition
}

// Key returns the message type.
func (ModifyPMONDefinitionsRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeModifyPMONDefinitions}
}

// Encode serializes the application data field.
func (r ModifyPMONDefinitionsRequest) Encode() ([]byte, error) {
	return encodeDefinitionList(r.Profile, r.Definitions, shapeModify, r.Resolve)
}

// DecodeModifyPMONDefinitionsRequest parses TC[12,7].
func DecodeModifyPMONDefinitionsRequest(profile MissionProfile, resolve ParameterResolver, data []byte) (*ModifyPMONDefinitionsRequest, error) {
	definitions, err := decodeDefinitionList(profile, data, shapeModify, resolve)
	if err != nil {
		return nil, err
	}
	return &ModifyPMONDefinitionsRequest{Profile: profile, Resolve: resolve, Definitions: definitions}, nil
}

// Humanize returns a human-readable summary.
func (r ModifyPMONDefinitionsRequest) Humanize() string {
	return "PUS TC[12,7] modify parameter monitoring definitions" +
		humanizePMONDefinitions(r.Definitions)
}

// PMONDefinitionReport is TM[12,9], per Figure 8-124.
//
// Clause 6.12.3.10i item 1 puts the current maximum transition reporting delay
// at the front, but only for a subservice that supports changing it. The
// profile declares that with SupportsTransitionDelayChange.
type PMONDefinitionReport struct {
	Profile MissionProfile
	// Resolve gives the widths of the deduced fields. Required.
	Resolve ParameterResolver

	// MaxReportingDelay is the current maximum transition reporting delay.
	// Present only when the profile sets SupportsTransitionDelayChange.
	MaxReportingDelay uint64

	// Definitions are the parameter monitoring definitions being reported.
	Definitions []PMONDefinition
}

// Key returns the message type.
func (PMONDefinitionReport) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypePMONDefinitionReport}
}

// Encode serializes the source data field.
func (r PMONDefinitionReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	var out []byte
	var err error
	if r.Profile.SupportsTransitionDelayChange {
		if out, err = putUint(out, r.MaxReportingDelay, r.Profile.TransitionDelaySize()); err != nil {
			return nil, err
		}
	}
	if out, err = putUint(out, uint64(len(r.Definitions)), r.Profile.MonitorCountSize()); err != nil {
		return nil, err
	}
	for _, definition := range r.Definitions {
		if out, err = encodePMONDefinition(out, r.Profile, definition, shapeReport, r.Resolve); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodePMONDefinitionReport parses TM[12,9].
func DecodePMONDefinitionReport(profile MissionProfile, resolve ParameterResolver, data []byte) (*PMONDefinitionReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	report := &PMONDefinitionReport{Profile: profile, Resolve: resolve}
	offset := 0
	if profile.SupportsTransitionDelayChange {
		width := profile.TransitionDelaySize()
		delay, err := readUint(data, width)
		if err != nil {
			return nil, err
		}
		report.MaxReportingDelay = delay
		offset += width
	}

	definitions, used, err := readPMONDefinitions(profile, data[offset:], shapeReport, resolve)
	if err != nil {
		return nil, err
	}
	report.Definitions = definitions
	offset += used

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return report, nil
}

// Humanize returns a human-readable summary.
func (r PMONDefinitionReport) Humanize() string {
	out := "PUS TM[12,9] parameter monitoring definition report"
	if r.Profile.SupportsTransitionDelayChange {
		out += "\n  Max delay ..... " + itoa(int(r.MaxReportingDelay))
	}
	return out + humanizePMONDefinitions(r.Definitions)
}

// encodeDefinitionList serializes a count-prefixed definition list.
func encodeDefinitionList(p MissionProfile, definitions []PMONDefinition, shape definitionShape, resolve ParameterResolver) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(definitions)), p.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, definition := range definitions {
		if out, err = encodePMONDefinition(out, p, definition, shape, resolve); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// decodeDefinitionList parses a count-prefixed definition list that makes up
// a whole message body.
func decodeDefinitionList(p MissionProfile, data []byte, shape definitionShape, resolve ParameterResolver) ([]PMONDefinition, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	definitions, used, err := readPMONDefinitions(p, data, shape, resolve)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return definitions, nil
}

// humanizePMONDefinitions renders a definition list.
func humanizePMONDefinitions(definitions []PMONDefinition) string {
	out := "\n  Definitions ... " + itoa(len(definitions))
	for _, definition := range definitions {
		out += "\n    PMON " + itoa(int(definition.ID)) +
			" on parameter " + itoa(int(definition.MonitoredParameterID)) +
			": " + definition.Criteria.Type.String()
	}
	return out
}

// CheckTransition is one entry of TM[12,11] and TM[12,12], per Figures 8-128
// and 8-129: a parameter monitoring definition whose checking status changed,
// and what it changed from and to.
type CheckTransition struct {
	// PMONID names the definition.
	PMONID uint64
	// MonitoredParameterID names the parameter, and its layout gives the
	// widths of the three deduced fields below.
	MonitoredParameterID uint64
	// CheckType is the kind of check, and it decides whether the mask is
	// present.
	CheckType CheckType

	// ExpectedValueCheckMask is present only for expected-value-checking
	// (Figure 8-128 note 1). This is the one "deduced presence" in ST[12]
	// that another field in the same entry decides.
	ExpectedValueCheckMask []byte

	// ParameterValue is the sampled value that caused the transition.
	ParameterValue []byte
	// LimitCrossed is the limit or threshold it crossed.
	LimitCrossed []byte

	// PreviousCheckingStatus and CurrentCheckingStatus are read against
	// CheckType, since Tables 8-7, 8-8 and 8-9 give the same raw values
	// different meanings.
	PreviousCheckingStatus PMONCheckingStatus
	CurrentCheckingStatus  PMONCheckingStatus

	// TransitionTime is when the change happened.
	TransitionTime time.Time
	// RawTransitionTime carries the time when the profile declares TimeRaw.
	RawTransitionTime []byte
}

// CheckTransitionReport is TM[12,11] or TM[12,12], per Figures 8-128 and
// 8-129. One structure printed twice.
//
// The two differ in what they report, not how. TM[12,11] answers TC[12,10]
// with every definition currently out of limits; TM[12,12] is sent
// unprompted when a check transitions. Clause 6.12.3.12c limits which
// transitions appear in TM[12,11], and that is a matter for whatever fills the
// report, not for the codec.
type CheckTransitionReport struct {
	Profile MissionProfile
	// Resolve gives the widths of the deduced fields. Required.
	Resolve ParameterResolver
	// Subtype is SubtypeOutOfLimitsReport or SubtypeCheckTransitionReport.
	Subtype uint8
	// Transitions are the entries being reported.
	Transitions []CheckTransition
}

// Key returns the message type.
func (r CheckTransitionReport) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: r.Subtype}
}

// Encode serializes the source data field.
func (r CheckTransitionReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	if r.Subtype != SubtypeOutOfLimitsReport && r.Subtype != SubtypeCheckTransitionReport {
		return nil, ErrWrongMessageType
	}

	out, err := putUint(nil, uint64(len(r.Transitions)), r.Profile.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, transition := range r.Transitions {
		if !transition.CheckType.valid() {
			return nil, ErrInvalidCheckType
		}
		layout, err := resolveLayout(r.Resolve, transition.MonitoredParameterID)
		if err != nil {
			return nil, err
		}

		if out, err = putUint(out, transition.PMONID, r.Profile.PMONIDSize()); err != nil {
			return nil, err
		}
		if out, err = putUint(out, transition.MonitoredParameterID, r.Profile.ParameterIDSize()); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(transition.CheckType), r.Profile.CheckTypeSize()); err != nil {
			return nil, err
		}
		if transition.CheckType == CheckExpectedValue {
			if out, err = appendFixed(out, transition.ExpectedValueCheckMask, layout.MaskBytes); err != nil {
				return nil, err
			}
		}
		if out, err = appendFixed(out, transition.ParameterValue, layout.ValueBytes); err != nil {
			return nil, err
		}
		if out, err = appendFixed(out, transition.LimitCrossed, layout.ValueBytes); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(transition.PreviousCheckingStatus),
			r.Profile.PMONCheckingStatusSize()); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(transition.CurrentCheckingStatus),
			r.Profile.PMONCheckingStatusSize()); err != nil {
			return nil, err
		}
		field, err := encodeAbsoluteTime(r.Profile, transition.TransitionTime, transition.RawTransitionTime)
		if err != nil {
			return nil, err
		}
		out = append(out, field...)
	}
	return out, nil
}

// DecodeCheckTransitionReport parses TM[12,11] or TM[12,12]. The subtype has
// to be supplied because the two share one structure.
func DecodeCheckTransitionReport(profile MissionProfile, resolve ParameterResolver, subtype uint8, data []byte) (*CheckTransitionReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if subtype != SubtypeOutOfLimitsReport && subtype != SubtypeCheckTransitionReport {
		return nil, ErrWrongMessageType
	}

	countWidth := profile.MonitorCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, err
	}
	offset := countWidth

	report := &CheckTransitionReport{Profile: profile, Resolve: resolve, Subtype: subtype}
	if count > 0 {
		// The shortest entry: the fixed fields with zero-width deduced ones.
		minimum := profile.PMONIDSize() + profile.ParameterIDSize() +
			profile.CheckTypeSize() + 2*profile.PMONCheckingStatusSize() +
			profile.TimeSize()
		if minimum <= 0 {
			return nil, ErrInvalidProfile
		}
		if uint64(len(data)-offset)/uint64(minimum) < count {
			return nil, ErrDataTooShort
		}
		report.Transitions = make([]CheckTransition, 0, count)
	}

	for i := uint64(0); i < count; i++ {
		var transition CheckTransition
		read := func(width int) (uint64, error) {
			v, err := readUint(data[offset:], width)
			offset += width
			return v, err
		}

		id, err := read(profile.PMONIDSize())
		if err != nil {
			return nil, err
		}
		transition.PMONID = id

		parameterID, err := read(profile.ParameterIDSize())
		if err != nil {
			return nil, err
		}
		transition.MonitoredParameterID = parameterID

		layout, err := resolveLayout(resolve, parameterID)
		if err != nil {
			return nil, err
		}

		rawCheck, err := read(profile.CheckTypeSize())
		if err != nil {
			return nil, err
		}
		transition.CheckType = CheckType(rawCheck)
		if !transition.CheckType.valid() {
			return nil, ErrInvalidCheckType
		}

		need := 2*layout.ValueBytes + 2*profile.PMONCheckingStatusSize() + profile.TimeSize()
		if transition.CheckType == CheckExpectedValue {
			need += layout.MaskBytes
		}
		if len(data)-offset < need {
			return nil, ErrDataTooShort
		}

		take := func(width int) []byte {
			out := make([]byte, width)
			copy(out, data[offset:offset+width])
			offset += width
			return out
		}
		if transition.CheckType == CheckExpectedValue {
			transition.ExpectedValueCheckMask = take(layout.MaskBytes)
		}
		transition.ParameterValue = take(layout.ValueBytes)
		transition.LimitCrossed = take(layout.ValueBytes)

		previous, err := read(profile.PMONCheckingStatusSize())
		if err != nil {
			return nil, err
		}
		transition.PreviousCheckingStatus = PMONCheckingStatus(previous)

		current, err := read(profile.PMONCheckingStatusSize())
		if err != nil {
			return nil, err
		}
		transition.CurrentCheckingStatus = PMONCheckingStatus(current)

		stamp, raw, used, err := decodeAbsoluteTime(profile, data[offset:])
		if err != nil {
			return nil, err
		}
		transition.TransitionTime, transition.RawTransitionTime = stamp, raw
		offset += used

		report.Transitions = append(report.Transitions, transition)
	}

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return report, nil
}

// Humanize returns a human-readable summary.
func (r CheckTransitionReport) Humanize() string {
	name := "check transition report"
	if r.Subtype == SubtypeOutOfLimitsReport {
		name = "out-of-limits report"
	}
	out := "PUS TM[12," + itoa(int(r.Subtype)) + "] " + name +
		"\n  Transitions ... " + itoa(len(r.Transitions))
	for _, transition := range r.Transitions {
		out += "\n    PMON " + itoa(int(transition.PMONID)) + ": " +
			transition.PreviousCheckingStatus.NameFor(transition.CheckType) +
			" -> " + transition.CurrentCheckingStatus.NameFor(transition.CheckType) +
			" at " + transition.TransitionTime.UTC().Format(time.RFC3339)
	}
	return out
}

// FMONDefinition is one functional monitoring definition, per clause
// 6.12.4.2.1f and Figures 8-135 and 8-138.
type FMONDefinition struct {
	// ID names the definition.
	ID uint64

	// Validity is the check validity condition. Present only when the profile
	// sets SupportsFMONConditionalChecking (clause 6.12.4.2.1f item 2).
	Validity ValidityCondition

	// ProtectionStatus is whether the definition is protected from deletion.
	// Carried by the report only, and only when the profile sets
	// SupportsFMONProtection.
	ProtectionStatus FMONProtectionStatus

	// Status is whether the definition is enabled. Carried by the report
	// only.
	Status FMONStatus

	// EventDefinitionID is the event to raise when the check fails.
	EventDefinitionID uint64

	// MinPMONFailing is how many of the definition's parameter monitoring
	// definitions must be violated at once for the functional check to fail.
	// Present only when the profile sets SupportsMinPMONFailingNumber. A
	// value of 1 is a logical OR of the checks; a value equal to the number
	// of checks is a logical AND (clause 6.12.4.2.1d notes 2 and 3).
	MinPMONFailing uint64

	// PMONIDs are the parameter monitoring definitions this one watches.
	// Clause 6.12.4.2.1f item 5 requires at least one.
	PMONIDs []uint64
}

// encodeFMONDefinition appends one definition. withStatus selects the report's
// layout, which carries the protection and enable statuses.
func encodeFMONDefinition(dst []byte, p MissionProfile, d FMONDefinition, withStatus bool, resolve ParameterResolver) ([]byte, error) {
	out, err := putUint(dst, d.ID, p.FMONIDSize())
	if err != nil {
		return nil, err
	}
	if p.SupportsFMONConditionalChecking {
		if out, err = encodeValidityCondition(out, p, d.Validity, resolve); err != nil {
			return nil, err
		}
	}
	if withStatus {
		if p.SupportsFMONProtection {
			if out, err = putUint(out, uint64(d.ProtectionStatus),
				p.FMONProtectionStatusSize()); err != nil {
				return nil, err
			}
		}
		if out, err = putUint(out, uint64(d.Status), p.FMONStatusSize()); err != nil {
			return nil, err
		}
	}
	if out, err = putUint(out, d.EventDefinitionID, p.EventDefinitionIDSize()); err != nil {
		return nil, err
	}
	if p.SupportsMinPMONFailingNumber {
		if out, err = putUint(out, d.MinPMONFailing, p.MinPMONFailingSize()); err != nil {
			return nil, err
		}
	}
	return encodeUintList(out, d.PMONIDs, p.MonitorCountSize(), p.PMONIDSize())
}

// decodeFMONDefinition reads one definition.
func decodeFMONDefinition(p MissionProfile, data []byte, withStatus bool, resolve ParameterResolver) (FMONDefinition, int, error) {
	var definition FMONDefinition
	offset := 0

	read := func(width int) (uint64, error) {
		v, err := readUint(data[offset:], width)
		offset += width
		return v, err
	}

	id, err := read(p.FMONIDSize())
	if err != nil {
		return FMONDefinition{}, 0, err
	}
	definition.ID = id

	if p.SupportsFMONConditionalChecking {
		condition, used, err := decodeValidityCondition(p, data[offset:], resolve)
		if err != nil {
			return FMONDefinition{}, 0, err
		}
		definition.Validity = condition
		offset += used
	}
	if withStatus {
		if p.SupportsFMONProtection {
			protection, err := read(p.FMONProtectionStatusSize())
			if err != nil {
				return FMONDefinition{}, 0, err
			}
			definition.ProtectionStatus = FMONProtectionStatus(protection)
		}
		status, err := read(p.FMONStatusSize())
		if err != nil {
			return FMONDefinition{}, 0, err
		}
		definition.Status = FMONStatus(status)
	}

	event, err := read(p.EventDefinitionIDSize())
	if err != nil {
		return FMONDefinition{}, 0, err
	}
	definition.EventDefinitionID = event

	if p.SupportsMinPMONFailingNumber {
		minimum, err := read(p.MinPMONFailingSize())
		if err != nil {
			return FMONDefinition{}, 0, err
		}
		definition.MinPMONFailing = minimum
	}

	ids, used, err := readUintList(data[offset:], p.MonitorCountSize(), p.PMONIDSize())
	if err != nil {
		return FMONDefinition{}, 0, err
	}
	definition.PMONIDs = ids
	offset += used

	return definition, offset, nil
}

// minFMONDefinitionSize bounds an untrusted count.
func minFMONDefinitionSize(p MissionProfile, withStatus bool) int {
	size := p.FMONIDSize() + p.EventDefinitionIDSize() + p.MonitorCountSize()
	if p.SupportsFMONConditionalChecking {
		size += p.ParameterIDSize()
	}
	if withStatus {
		size += p.FMONStatusSize()
		if p.SupportsFMONProtection {
			size += p.FMONProtectionStatusSize()
		}
	}
	if p.SupportsMinPMONFailingNumber {
		size += p.MinPMONFailingSize()
	}
	return size
}

// readFMONDefinitions reads a count-prefixed list of FMON definitions.
func readFMONDefinitions(p MissionProfile, data []byte, withStatus bool, resolve ParameterResolver) ([]FMONDefinition, int, error) {
	countWidth := p.MonitorCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, 0, err
	}
	offset := countWidth
	if count == 0 {
		return nil, offset, nil
	}

	minimum := minFMONDefinitionSize(p, withStatus)
	if minimum <= 0 {
		return nil, 0, ErrInvalidProfile
	}
	if uint64(len(data)-offset)/uint64(minimum) < count {
		return nil, 0, ErrDataTooShort
	}

	definitions := make([]FMONDefinition, 0, count)
	for i := uint64(0); i < count; i++ {
		definition, used, err := decodeFMONDefinition(p, data[offset:], withStatus, resolve)
		if err != nil {
			return nil, 0, err
		}
		definitions = append(definitions, definition)
		offset += used
	}
	return definitions, offset, nil
}

// AddFMONDefinitionsRequest is TC[12,23], per Figure 8-135.
type AddFMONDefinitionsRequest struct {
	Profile MissionProfile
	// Resolve gives the widths of the validity condition's deduced fields.
	// Required only when the profile sets SupportsFMONConditionalChecking,
	// because that is the only part of this message that needs it.
	Resolve ParameterResolver
	// Definitions are the functional monitoring definitions to add.
	Definitions []FMONDefinition
}

// Key returns the message type.
func (AddFMONDefinitionsRequest) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeAddFMONDefinitions}
}

// Encode serializes the application data field.
func (r AddFMONDefinitionsRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Definitions)), r.Profile.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, definition := range r.Definitions {
		if out, err = encodeFMONDefinition(out, r.Profile, definition, false, r.Resolve); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeAddFMONDefinitionsRequest parses TC[12,23].
func DecodeAddFMONDefinitionsRequest(profile MissionProfile, resolve ParameterResolver, data []byte) (*AddFMONDefinitionsRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	definitions, used, err := readFMONDefinitions(profile, data, false, resolve)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &AddFMONDefinitionsRequest{Profile: profile, Resolve: resolve, Definitions: definitions}, nil
}

// Humanize returns a human-readable summary.
func (r AddFMONDefinitionsRequest) Humanize() string {
	return "PUS TC[12,23] add functional monitoring definitions" +
		humanizeFMONDefinitions(r.Definitions)
}

// FMONDefinitionReport is TM[12,26], per Figure 8-138.
type FMONDefinitionReport struct {
	Profile MissionProfile
	// Resolve gives the widths of the validity condition's deduced fields.
	Resolve ParameterResolver
	// Definitions are the functional monitoring definitions being reported.
	Definitions []FMONDefinition
}

// Key returns the message type.
func (FMONDefinitionReport) Key() MessageKey {
	return MessageKey{Service: ServiceOnBoardMonitoring, Subtype: SubtypeFMONDefinitionReport}
}

// Encode serializes the source data field.
func (r FMONDefinitionReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Definitions)), r.Profile.MonitorCountSize())
	if err != nil {
		return nil, err
	}
	for _, definition := range r.Definitions {
		if out, err = encodeFMONDefinition(out, r.Profile, definition, true, r.Resolve); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeFMONDefinitionReport parses TM[12,26].
func DecodeFMONDefinitionReport(profile MissionProfile, resolve ParameterResolver, data []byte) (*FMONDefinitionReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	definitions, used, err := readFMONDefinitions(profile, data, true, resolve)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &FMONDefinitionReport{Profile: profile, Resolve: resolve, Definitions: definitions}, nil
}

// Humanize returns a human-readable summary.
func (r FMONDefinitionReport) Humanize() string {
	return "PUS TM[12,26] functional monitoring definition report" +
		humanizeFMONDefinitions(r.Definitions)
}

// humanizeFMONDefinitions renders an FMON definition list.
func humanizeFMONDefinitions(definitions []FMONDefinition) string {
	out := "\n  Definitions ... " + itoa(len(definitions))
	for _, definition := range definitions {
		out += "\n    FMON " + itoa(int(definition.ID)) +
			" over " + itoa(len(definition.PMONIDs)) + " check(s)"
	}
	return out
}

// registerST12 adds the ST[12] codecs to a registry.
//
// The seven decoders that need parameter layouts read r.parameters at decode
// time, so a registry built without WithParameterResolver still registers them
// and they return ErrNoParameterResolver.
func registerST12(r *Registry) error {
	for subtype := range monitorControlNames {
		want := subtype
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceOnBoardMonitoring, Subtype: subtype},
			func(_ MissionProfile, data []byte) (Request, error) {
				if len(data) != 0 {
					return nil, ErrTrailingBytes
				}
				return MonitorControlRequest{Subtype: want}, nil
			},
		); err != nil {
			return err
		}
	}

	for _, names := range []map[uint8]string{pmonIDListNames, fmonIDListNames} {
		for subtype := range names {
			want := subtype
			if err := r.RegisterRequest(
				MessageKey{Service: ServiceOnBoardMonitoring, Subtype: subtype},
				func(p MissionProfile, data []byte) (Request, error) {
					return DecodeMonitorIDListRequest(p, want, data)
				},
			); err != nil {
				return err
			}
		}
	}

	requests := map[uint8]RequestDecoder{
		SubtypeChangeTransitionDelay: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeChangeTransitionDelayRequest(p, data)
		},
		SubtypeAddPMONDefinitions: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeAddPMONDefinitionsRequest(p, r.parameters, data)
		},
		SubtypeModifyPMONDefinitions: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeModifyPMONDefinitionsRequest(p, r.parameters, data)
		},
		SubtypeAddFMONDefinitions: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeAddFMONDefinitionsRequest(p, r.parameters, data)
		},
	}
	for subtype, decoder := range requests {
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceOnBoardMonitoring, Subtype: subtype}, decoder,
		); err != nil {
			return err
		}
	}

	reports := map[uint8]ReportDecoder{
		SubtypePMONDefinitionReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodePMONDefinitionReport(p, r.parameters, data)
		},
		SubtypeOutOfLimitsReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeCheckTransitionReport(p, r.parameters, SubtypeOutOfLimitsReport, data)
		},
		SubtypeCheckTransitionReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeCheckTransitionReport(p, r.parameters, SubtypeCheckTransitionReport, data)
		},
		SubtypePMONStatusReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodePMONStatusReport(p, data)
		},
		SubtypeFMONDefinitionReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeFMONDefinitionReport(p, r.parameters, data)
		},
		SubtypeFMONStatusReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeFMONStatusReport(p, data)
		},
	}
	for subtype, decoder := range reports {
		if err := r.RegisterReport(
			MessageKey{Service: ServiceOnBoardMonitoring, Subtype: subtype}, decoder,
		); err != nil {
			return err
		}
	}
	return nil
}
