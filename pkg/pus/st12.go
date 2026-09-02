package pus

// ST[12] on-board monitoring, per ECSS-E-ST-70-41C clauses 6.12 and 8.12.
//
// Two subservices in one service. Parameter monitoring watches individual
// on-board parameters and reports when one goes out of range. Functional
// monitoring watches groups of those checks and reports when enough of them
// fail at once. Twenty-eight message types between them, and this package
// implements the wire format of all of them.
//
// ST[12] is the first service in this package that cannot be decoded from the
// message alone.
//
// The limits, delta thresholds, expected values and masks are all typed
// "deduced" in the figures, and the notes say what they are deduced from: the
// monitored parameter identified by the monitored parameter ID field. That is
// mission configuration (an on-board parameter's type) and this package does
// not hold it. Unlike ST[03]'s parameter values or ST[08]'s function
// arguments, these fields are not at the end of the message: they sit in the
// middle of a repeated group, so without their widths there is no way to find
// where the next definition starts. Carrying them raw is not an option.
//
// So the codecs that touch them take a ParameterResolver. A registry without
// one decodes the other twenty-one message types and refuses these seven,
// rather than guessing and silently mis-splitting a definition list.

const (
	ServiceOnBoardMonitoring uint8 = 12

	SubtypeEnablePMONDefinitions   uint8 = 1  // TC[12,1] clause 8.12.2.1
	SubtypeDisablePMONDefinitions  uint8 = 2  // TC[12,2] clause 8.12.2.2
	SubtypeChangeTransitionDelay   uint8 = 3  // TC[12,3] clause 8.12.2.3
	SubtypeDeleteAllPMON           uint8 = 4  // TC[12,4] clause 8.12.2.4
	SubtypeAddPMONDefinitions      uint8 = 5  // TC[12,5] clause 8.12.2.5
	SubtypeDeletePMONDefinitions   uint8 = 6  // TC[12,6] clause 8.12.2.6
	SubtypeModifyPMONDefinitions   uint8 = 7  // TC[12,7] clause 8.12.2.7
	SubtypeReportPMONDefinitions   uint8 = 8  // TC[12,8] clause 8.12.2.8
	SubtypePMONDefinitionReport    uint8 = 9  // TM[12,9] clause 8.12.2.9
	SubtypeReportOutOfLimits       uint8 = 10 // TC[12,10] clause 8.12.2.10
	SubtypeOutOfLimitsReport       uint8 = 11 // TM[12,11] clause 8.12.2.11
	SubtypeCheckTransitionReport   uint8 = 12 // TM[12,12] clause 8.12.2.12
	SubtypeReportPMONStatus        uint8 = 13 // TC[12,13] clause 8.12.2.13
	SubtypePMONStatusReport        uint8 = 14 // TM[12,14] clause 8.12.2.14
	SubtypeEnablePMONFunction      uint8 = 15 // TC[12,15] clause 8.12.2.15
	SubtypeDisablePMONFunction     uint8 = 16 // TC[12,16] clause 8.12.2.16
	SubtypeEnableFMONFunction      uint8 = 17 // TC[12,17] clause 8.12.2.17
	SubtypeDisableFMONFunction     uint8 = 18 // TC[12,18] clause 8.12.2.18
	SubtypeEnableFMONDefinitions   uint8 = 19 // TC[12,19] clause 8.12.2.19
	SubtypeDisableFMONDefinitions  uint8 = 20 // TC[12,20] clause 8.12.2.20
	SubtypeProtectFMONDefinitions  uint8 = 21 // TC[12,21] clause 8.12.2.21
	SubtypeUnprotectFMONDefinition uint8 = 22 // TC[12,22] clause 8.12.2.22
	SubtypeAddFMONDefinitions      uint8 = 23 // TC[12,23] clause 8.12.2.23
	SubtypeDeleteFMONDefinitions   uint8 = 24 // TC[12,24] clause 8.12.2.24
	SubtypeReportFMONDefinitions   uint8 = 25 // TC[12,25] clause 8.12.2.25
	SubtypeFMONDefinitionReport    uint8 = 26 // TM[12,26] clause 8.12.2.26
	SubtypeReportFMONStatus        uint8 = 27 // TC[12,27] clause 8.12.2.27
	SubtypeFMONStatusReport        uint8 = 28 // TM[12,28] clause 8.12.2.28
)

// CheckType is the kind of check a parameter monitoring definition performs,
// per Table 8-6.
type CheckType uint64

const (
	// CheckExpectedValue compares the masked value to an expected value
	// (clause 6.12.3.2.1c). Raw value 0.
	CheckExpectedValue CheckType = 0
	// CheckLimit compares the value to a low and a high limit
	// (clause 6.12.3.2.1b). Raw value 1.
	CheckLimit CheckType = 1
	// CheckDelta compares the change between two consecutive values to a low
	// and a high threshold (clause 6.12.3.2.2c). Raw value 2.
	CheckDelta CheckType = 2
)

// String names the check type the way Table 8-6 writes it.
func (c CheckType) String() string {
	switch c {
	case CheckExpectedValue:
		return "expected-value-checking"
	case CheckLimit:
		return "limit-checking"
	case CheckDelta:
		return "delta-checking"
	default:
		return "unknown"
	}
}

// valid reports whether the check type is one of Table 8-6's three values.
func (c CheckType) valid() bool { return c <= CheckDelta }

// PMONCheckingStatus is the checking status of a parameter monitoring
// definition.
//
// Its raw values mean different things depending on the check type: clause
// 8.12.3.1b gives three tables, 8-7 for expected-value-checking, 8-8 for
// limit-checking and 8-9 for delta-checking. Values 0, 1 and 2 line up (the
// nominal status, "unchecked" and "invalid") but 3 and 4 do not. So a raw
// value on its own does not name a status, and NameFor takes the check type.
type PMONCheckingStatus uint64

const (
	// PMONNominal is raw value 0: "expected value" for an expected-value
	// check, "within limits" for a limit check, "within thresholds" for a
	// delta check.
	PMONNominal PMONCheckingStatus = 0
	// PMONUnchecked is raw value 1 in all three tables.
	PMONUnchecked PMONCheckingStatus = 1
	// PMONInvalid is raw value 2 in all three tables.
	PMONInvalid PMONCheckingStatus = 2
	// PMONUnexpectedValue is raw value 3 for expected-value-checking only
	// (Table 8-7).
	PMONUnexpectedValue PMONCheckingStatus = 3
	// PMONBelowLowLimit is raw value 3 for limit-checking (Table 8-8).
	PMONBelowLowLimit PMONCheckingStatus = 3
	// PMONAboveHighLimit is raw value 4 for limit-checking (Table 8-8).
	PMONAboveHighLimit PMONCheckingStatus = 4
	// PMONBelowLowThreshold is raw value 3 for delta-checking (Table 8-9).
	PMONBelowLowThreshold PMONCheckingStatus = 3
	// PMONAboveHighThreshold is raw value 4 for delta-checking (Table 8-9).
	PMONAboveHighThreshold PMONCheckingStatus = 4
)

// NameFor returns the engineering value this raw status has under the given
// check type, per Tables 8-7, 8-8 and 8-9.
//
// The check type is required because the tables disagree: raw 3 is
// "unexpected value" for an expected-value check, "below low limit" for a
// limit check and "below low threshold" for a delta check. Reporting the wrong
// one is a plausible-looking error in an operator display.
func (s PMONCheckingStatus) NameFor(check CheckType) string {
	switch s {
	case PMONUnchecked:
		return "unchecked"
	case PMONInvalid:
		return "invalid"
	}

	switch check {
	case CheckExpectedValue:
		switch s {
		case PMONNominal:
			return "expected value"
		case PMONUnexpectedValue:
			return "unexpected value"
		}
	case CheckLimit:
		switch s {
		case PMONNominal:
			return "within limits"
		case PMONBelowLowLimit:
			return "below low limit"
		case PMONAboveHighLimit:
			return "above high limit"
		}
	case CheckDelta:
		switch s {
		case PMONNominal:
			return "within thresholds"
		case PMONBelowLowThreshold:
			return "below low threshold"
		case PMONAboveHighThreshold:
			return "above high threshold"
		}
	}
	return "unknown"
}

// PMONStatus is whether a parameter monitoring definition is enabled, per
// Table 8-10.
type PMONStatus uint64

const (
	// PMONDisabled is raw value 0.
	PMONDisabled PMONStatus = 0
	// PMONEnabled is raw value 1.
	PMONEnabled PMONStatus = 1
)

// String names the status the way Table 8-10 writes it.
func (s PMONStatus) String() string {
	switch s {
	case PMONDisabled:
		return "disabled"
	case PMONEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

// FMONProtectionStatus is whether a functional monitoring definition is
// protected from deletion, per Table 8-11.
type FMONProtectionStatus uint64

const (
	// FMONUnprotected is raw value 0.
	FMONUnprotected FMONProtectionStatus = 0
	// FMONProtected is raw value 1.
	FMONProtected FMONProtectionStatus = 1
)

// String names the status the way Table 8-11 writes it.
func (s FMONProtectionStatus) String() string {
	switch s {
	case FMONUnprotected:
		return "unprotected"
	case FMONProtected:
		return "protected"
	default:
		return "unknown"
	}
}

// FMONStatus is whether a functional monitoring definition is enabled, per
// Table 8-12.
type FMONStatus uint64

const (
	// FMONDisabled is raw value 0.
	FMONDisabled FMONStatus = 0
	// FMONEnabled is raw value 1.
	FMONEnabled FMONStatus = 1
)

// String names the status the way Table 8-12 writes it.
func (s FMONStatus) String() string {
	switch s {
	case FMONDisabled:
		return "disabled"
	case FMONEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

// FMONCheckingStatus is the checking status of a functional monitoring
// definition, per Table 8-13.
//
// Unlike PMONCheckingStatus this one has a single table, so a raw value names
// a status on its own.
type FMONCheckingStatus uint64

const (
	// FMONUnchecked is raw value 0.
	FMONUnchecked FMONCheckingStatus = 0
	// FMONRunning is raw value 1.
	FMONRunning FMONCheckingStatus = 1
	// FMONInvalid is raw value 2.
	FMONInvalid FMONCheckingStatus = 2
	// FMONFailed is raw value 3.
	FMONFailed FMONCheckingStatus = 3
)

// String names the status the way Table 8-13 writes it.
func (s FMONCheckingStatus) String() string {
	switch s {
	case FMONUnchecked:
		return "unchecked"
	case FMONRunning:
		return "running"
	case FMONInvalid:
		return "invalid"
	case FMONFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ParameterLayout is the on-wire width of one on-board parameter's value and
// of a bit mask over it, both in octets.
//
// The two are separate because the standard never says they are equal. It says
// only that each is "specific to" or "derived from" the parameter, Figure
// 8-114 note 2, Figure 8-115 note 1 and their siblings. A mask over a value is
// normally the same width, but a mission that declares otherwise is not
// contradicting the standard, so this type does not decide for it.
type ParameterLayout struct {
	// ValueBytes is the width of the parameter's value, and so of a limit, a
	// delta threshold or an expected value compared against it.
	ValueBytes int
	// MaskBytes is the width of a bit mask applied to that value.
	MaskBytes int
}

// ParameterResolver returns the layout of the on-board parameter with this ID.
//
// It is the mission's parameter definition, and there is no way for this
// package to supply it. Without one, the seven ST[12] message types that carry
// deduced fields cannot be decoded at all (see the package comment on
// st12.go) so a registry built without a resolver refuses them rather than
// guessing.
type ParameterResolver func(parameterID uint64) (ParameterLayout, error)

// ValidityCondition is the check validity condition of Figures 8-114, 8-124,
// 8-135 and 8-138: a parameter, a mask over it, and the value the masked
// parameter must equal for the check to run.
//
// Clause 6.12.3.2.1i spells out the evaluation: bitwise-and the mask with the
// sampled value of the validity parameter, and declare the condition true when
// the result equals the expected value.
//
// The whole group is present only when the subservice supports conditional
// checking, clause 6.12.3.3c for parameter monitoring, 6.12.4.2.1c for
// functional monitoring. The profile declares each separately.
type ValidityCondition struct {
	// ParameterID names the validity parameter, whose layout gives the widths
	// of the two fields below.
	ParameterID uint64
	// Mask is the bit mask to apply to the sampled value.
	Mask []byte
	// ExpectedValue is what the masked value must equal.
	ExpectedValue []byte
}

// ExpectedValueCheck is the expected-value criteria of Figures 8-115, 8-120
// and 8-125.
type ExpectedValueCheck struct {
	// Mask is applied to the monitored parameter's sampled value.
	Mask []byte
	// Spare pads the criteria out so all three check types can share a width
	// (Figure 8-115 note 2). Present only when the profile declares it, and
	// then exactly as wide as an event definition ID.
	Spare []byte
	// ExpectedValue is what the masked value must equal.
	ExpectedValue []byte
	// EventDefinitionID is the event to raise on a new "unexpected value"
	// status. Zero means no event report (Figure 8-115 note 3).
	EventDefinitionID uint64
}

// LimitCheck is the limit criteria of Figures 8-116, 8-121 and 8-126.
type LimitCheck struct {
	// LowLimit is the value the parameter must stay at or above.
	LowLimit []byte
	// LowEventDefinitionID is the event to raise on a new "below low limit"
	// status. Zero means no event report.
	LowEventDefinitionID uint64
	// HighLimit is the value the parameter must stay at or below.
	HighLimit []byte
	// HighEventDefinitionID is the event to raise on a new "above high limit"
	// status. Zero means no event report.
	HighEventDefinitionID uint64
}

// DeltaCheck is the delta criteria of Figures 8-117, 8-122 and 8-127.
type DeltaCheck struct {
	// LowThreshold is the change the parameter must stay at or above.
	LowThreshold []byte
	// LowEventDefinitionID is the event to raise on a new "below low
	// threshold" status. Zero means no event report.
	LowEventDefinitionID uint64
	// HighThreshold is the change the parameter must stay at or below.
	HighThreshold []byte
	// HighEventDefinitionID is the event to raise on a new "above high
	// threshold" status. Zero means no event report.
	HighEventDefinitionID uint64
	// ConsecutiveDeltaValues is how many consecutive delta values the check
	// uses.
	ConsecutiveDeltaValues uint64
}

// CheckCriteria is the "check type dependent criteria" field of Figures 8-114,
// 8-119 and 8-124.
//
// Exactly one of the three pointers matches Type. Anything else is refused
// rather than encoded, because the receiving end reads the criteria according
// to the type and would misread a mismatch.
type CheckCriteria struct {
	Type CheckType

	ExpectedValue *ExpectedValueCheck
	Limit         *LimitCheck
	Delta         *DeltaCheck
}

// validate checks that the one pointer matching Type is the one that is set.
func (c CheckCriteria) validate() error {
	if !c.Type.valid() {
		return ErrInvalidCheckType
	}
	set := 0
	if c.ExpectedValue != nil {
		set++
	}
	if c.Limit != nil {
		set++
	}
	if c.Delta != nil {
		set++
	}
	if set != 1 {
		return ErrCheckCriteriaMismatch
	}

	switch c.Type {
	case CheckExpectedValue:
		if c.ExpectedValue == nil {
			return ErrCheckCriteriaMismatch
		}
	case CheckLimit:
		if c.Limit == nil {
			return ErrCheckCriteriaMismatch
		}
	case CheckDelta:
		if c.Delta == nil {
			return ErrCheckCriteriaMismatch
		}
	}
	return nil
}

// encode appends the criteria at the widths the monitored parameter's layout
// gives.
func (c CheckCriteria) encode(dst []byte, p MissionProfile, layout ParameterLayout) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	eventWidth := p.EventDefinitionIDSize()

	switch c.Type {
	case CheckExpectedValue:
		out, err := appendFixed(dst, c.ExpectedValue.Mask, layout.MaskBytes)
		if err != nil {
			return nil, err
		}
		if p.ExpectedValueSpare {
			// Figure 8-115: the spare is a bit-string "of event definition ID
			// field size".
			if out, err = appendFixed(out, c.ExpectedValue.Spare, eventWidth); err != nil {
				return nil, err
			}
		}
		if out, err = appendFixed(out, c.ExpectedValue.ExpectedValue, layout.ValueBytes); err != nil {
			return nil, err
		}
		return putUint(out, c.ExpectedValue.EventDefinitionID, eventWidth)

	case CheckLimit:
		out, err := appendFixed(dst, c.Limit.LowLimit, layout.ValueBytes)
		if err != nil {
			return nil, err
		}
		if out, err = putUint(out, c.Limit.LowEventDefinitionID, eventWidth); err != nil {
			return nil, err
		}
		if out, err = appendFixed(out, c.Limit.HighLimit, layout.ValueBytes); err != nil {
			return nil, err
		}
		return putUint(out, c.Limit.HighEventDefinitionID, eventWidth)

	default:
		out, err := appendFixed(dst, c.Delta.LowThreshold, layout.ValueBytes)
		if err != nil {
			return nil, err
		}
		if out, err = putUint(out, c.Delta.LowEventDefinitionID, eventWidth); err != nil {
			return nil, err
		}
		if out, err = appendFixed(out, c.Delta.HighThreshold, layout.ValueBytes); err != nil {
			return nil, err
		}
		if out, err = putUint(out, c.Delta.HighEventDefinitionID, eventWidth); err != nil {
			return nil, err
		}
		return putUint(out, c.Delta.ConsecutiveDeltaValues, p.DeltaValueCountSize())
	}
}

// criteriaSize returns the encoded width of the criteria for a check type.
func criteriaSize(p MissionProfile, check CheckType, layout ParameterLayout) int {
	eventWidth := p.EventDefinitionIDSize()
	switch check {
	case CheckExpectedValue:
		size := layout.MaskBytes + layout.ValueBytes + eventWidth
		if p.ExpectedValueSpare {
			size += eventWidth
		}
		return size
	case CheckLimit:
		return 2 * (layout.ValueBytes + eventWidth)
	default:
		return 2*(layout.ValueBytes+eventWidth) + p.DeltaValueCountSize()
	}
}

// decodeCheckCriteria reads the criteria for a check type from the front of
// data.
func decodeCheckCriteria(p MissionProfile, check CheckType, layout ParameterLayout, data []byte) (CheckCriteria, int, error) {
	if !check.valid() {
		return CheckCriteria{}, 0, ErrInvalidCheckType
	}
	if layout.ValueBytes < 0 || layout.MaskBytes < 0 {
		return CheckCriteria{}, 0, ErrInvalidParameterLayout
	}
	if len(data) < criteriaSize(p, check, layout) {
		return CheckCriteria{}, 0, ErrDataTooShort
	}

	eventWidth := p.EventDefinitionIDSize()
	criteria := CheckCriteria{Type: check}
	offset := 0

	take := func(width int) []byte {
		out := make([]byte, width)
		copy(out, data[offset:offset+width])
		offset += width
		return out
	}
	takeUint := func(width int) (uint64, error) {
		v, err := readUint(data[offset:], width)
		offset += width
		return v, err
	}

	switch check {
	case CheckExpectedValue:
		value := &ExpectedValueCheck{Mask: take(layout.MaskBytes)}
		if p.ExpectedValueSpare {
			value.Spare = take(eventWidth)
		}
		value.ExpectedValue = take(layout.ValueBytes)
		id, err := takeUint(eventWidth)
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		value.EventDefinitionID = id
		criteria.ExpectedValue = value

	case CheckLimit:
		limit := &LimitCheck{LowLimit: take(layout.ValueBytes)}
		low, err := takeUint(eventWidth)
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		limit.LowEventDefinitionID = low
		limit.HighLimit = take(layout.ValueBytes)
		high, err := takeUint(eventWidth)
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		limit.HighEventDefinitionID = high
		criteria.Limit = limit

	default:
		delta := &DeltaCheck{LowThreshold: take(layout.ValueBytes)}
		low, err := takeUint(eventWidth)
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		delta.LowEventDefinitionID = low
		delta.HighThreshold = take(layout.ValueBytes)
		high, err := takeUint(eventWidth)
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		delta.HighEventDefinitionID = high
		count, err := takeUint(p.DeltaValueCountSize())
		if err != nil {
			return CheckCriteria{}, 0, err
		}
		delta.ConsecutiveDeltaValues = count
		criteria.Delta = delta
	}

	return criteria, offset, nil
}

// appendFixed appends a field that must be exactly width octets.
//
// A short or long value is refused rather than padded or truncated: every one
// of these fields sits before others in a repeated group, so a wrong width
// shifts everything after it.
func appendFixed(dst, value []byte, width int) ([]byte, error) {
	if width < 0 {
		return nil, ErrInvalidParameterLayout
	}
	if len(value) != width {
		return nil, ErrFieldWidthMismatch
	}
	return append(dst, value...), nil
}

// encodeValidityCondition appends a check validity condition.
func encodeValidityCondition(dst []byte, p MissionProfile, c ValidityCondition, resolve ParameterResolver) ([]byte, error) {
	layout, err := resolveLayout(resolve, c.ParameterID)
	if err != nil {
		return nil, err
	}
	out, err := putUint(dst, c.ParameterID, p.ParameterIDSize())
	if err != nil {
		return nil, err
	}
	if out, err = appendFixed(out, c.Mask, layout.MaskBytes); err != nil {
		return nil, err
	}
	return appendFixed(out, c.ExpectedValue, layout.ValueBytes)
}

// decodeValidityCondition reads a check validity condition from the front of
// data.
func decodeValidityCondition(p MissionProfile, data []byte, resolve ParameterResolver) (ValidityCondition, int, error) {
	width := p.ParameterIDSize()
	id, err := readUint(data, width)
	if err != nil {
		return ValidityCondition{}, 0, err
	}
	layout, err := resolveLayout(resolve, id)
	if err != nil {
		return ValidityCondition{}, 0, err
	}
	offset := width
	if len(data)-offset < layout.MaskBytes+layout.ValueBytes {
		return ValidityCondition{}, 0, ErrDataTooShort
	}

	condition := ValidityCondition{ParameterID: id}
	condition.Mask = make([]byte, layout.MaskBytes)
	copy(condition.Mask, data[offset:offset+layout.MaskBytes])
	offset += layout.MaskBytes

	condition.ExpectedValue = make([]byte, layout.ValueBytes)
	copy(condition.ExpectedValue, data[offset:offset+layout.ValueBytes])
	offset += layout.ValueBytes

	return condition, offset, nil
}

// resolveLayout calls the resolver, turning an absent one into the error that
// says so rather than a nil dereference.
func resolveLayout(resolve ParameterResolver, parameterID uint64) (ParameterLayout, error) {
	if resolve == nil {
		return ParameterLayout{}, ErrNoParameterResolver
	}
	layout, err := resolve(parameterID)
	if err != nil {
		return ParameterLayout{}, err
	}
	if layout.ValueBytes < 0 || layout.MaskBytes < 0 {
		return ParameterLayout{}, ErrInvalidParameterLayout
	}
	return layout, nil
}
