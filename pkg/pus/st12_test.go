package pus_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
)

// monProfile is a monitoring profile with every capability on and narrow
// widths, so the field boundaries are readable in a hex dump.
func monProfile() pus.MissionProfile {
	p := pus.DefaultProfile()
	p.TimeFormat = pus.TimeNone
	p.ParameterIDBytes = 2
	p.EventDefinitionIDBytes = 2
	p.PMONIDBytes = 2
	p.FMONIDBytes = 2
	p.MonitorCountBytes = 1
	p.CheckTypeBytes = 1
	p.PMONStatusBytes = 1
	p.PMONCheckingStatusBytes = 1
	p.FMONStatusBytes = 1
	p.FMONProtectionStatusBytes = 1
	p.FMONCheckingStatusBytes = 1
	p.MonitoringIntervalBytes = 4
	p.RepetitionNumberBytes = 1
	p.TransitionDelayBytes = 2
	p.MinPMONFailingBytes = 1
	p.DeltaValueCountBytes = 1
	p.SupportsConditionalChecking = true
	p.PerDefinitionMonitoringInterval = true
	p.SupportsTransitionDelayChange = true
	p.SupportsFMONConditionalChecking = true
	p.SupportsMinPMONFailingNumber = true
	p.SupportsFMONProtection = true
	return p
}

// twoOctetParameters is a mission parameter table where every parameter is two
// octets wide with a two-octet mask.
func twoOctetParameters(uint64) (pus.ParameterLayout, error) {
	return pus.ParameterLayout{ValueBytes: 2, MaskBytes: 2}, nil
}

// TestST12RegistersEveryMessageType checks all twenty-eight are wired up.
func TestST12RegistersEveryMessageType(t *testing.T) {
	r, err := pus.NewDefaultRegistry(monProfile(), pus.WithParameterResolver(twoOctetParameters))
	if err != nil {
		t.Fatal(err)
	}

	wantRequests := []uint8{1, 2, 3, 4, 5, 6, 7, 8, 10, 13, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 27}
	wantReports := []uint8{9, 11, 12, 14, 26, 28}

	have := map[uint8]bool{}
	for _, key := range r.KnownRequests() {
		if key.Service == pus.ServiceOnBoardMonitoring {
			have[key.Subtype] = true
		}
	}
	for _, subtype := range wantRequests {
		if !have[subtype] {
			t.Errorf("TC[12,%d] is not registered", subtype)
		}
	}
	if len(have) != len(wantRequests) {
		t.Errorf("registered %d ST[12] requests, want %d", len(have), len(wantRequests))
	}

	have = map[uint8]bool{}
	for _, key := range r.KnownReports() {
		if key.Service == pus.ServiceOnBoardMonitoring {
			have[key.Subtype] = true
		}
	}
	for _, subtype := range wantReports {
		if !have[subtype] {
			t.Errorf("TM[12,%d] is not registered", subtype)
		}
	}
	if len(have) != len(wantReports) {
		t.Errorf("registered %d ST[12] reports, want %d", len(have), len(wantReports))
	}

	// Twenty-two requests plus six reports is the twenty-eight the standard
	// defines.
	if len(wantRequests)+len(wantReports) != 28 {
		t.Fatalf("this test covers %d types, want 28", len(wantRequests)+len(wantReports))
	}
}

// TestMonitorControlRequestsCarryNoBody covers the eight empty-bodied
// requests.
func TestMonitorControlRequestsCarryNoBody(t *testing.T) {
	r, err := pus.NewDefaultRegistry(monProfile())
	if err != nil {
		t.Fatal(err)
	}

	for _, subtype := range []uint8{4, 10, 13, 15, 16, 17, 18, 27} {
		request := pus.MonitorControlRequest{Subtype: subtype}
		encoded, err := request.Encode()
		if err != nil {
			t.Fatalf("TC[12,%d]: %v", subtype, err)
		}
		if len(encoded) != 0 {
			t.Errorf("TC[12,%d] encoded %d octets, want none", subtype, len(encoded))
		}

		key := pus.MessageKey{Service: pus.ServiceOnBoardMonitoring, Subtype: subtype}
		if _, err := r.DecodeRequest(key, nil); err != nil {
			t.Errorf("TC[12,%d] empty body: %v", subtype, err)
		}
		if _, err := r.DecodeRequest(key, []byte{0}); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("TC[12,%d] with a body: err = %v, want ErrTrailingBytes", subtype, err)
		}
	}

	if _, err := (pus.MonitorControlRequest{Subtype: 5}).Encode(); !errors.Is(err, pus.ErrWrongMessageType) {
		t.Errorf("err = %v, want ErrWrongMessageType", err)
	}
}

// TestMonitorIDListUsesTheRightWidth checks the PMON and FMON ID widths are
// separately declared and separately used.
func TestMonitorIDListUsesTheRightWidth(t *testing.T) {
	p := monProfile()
	p.PMONIDBytes = 1
	p.FMONIDBytes = 4

	pmon := pus.MonitorIDListRequest{
		Profile: p, Subtype: pus.SubtypeEnablePMONDefinitions, IDs: []uint64{9},
	}
	encoded, err := pmon.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x01, 0x09}; !bytes.Equal(encoded, want) {
		t.Errorf("PMON list = % x, want % x", encoded, want)
	}

	fmon := pus.MonitorIDListRequest{
		Profile: p, Subtype: pus.SubtypeEnableFMONDefinitions, IDs: []uint64{9},
	}
	encoded, err = fmon.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x01, 0x00, 0x00, 0x00, 0x09}; !bytes.Equal(encoded, want) {
		t.Errorf("FMON list = % x, want % x", encoded, want)
	}
}

// TestMonitorIDListIsAllOnlyForTheTwoReportRequests is the distinction worth
// pinning: clauses 8.12.2.8c and 8.12.2.25c give N = 0 the meaning "all", and
// the other eight ID-list requests say nothing about it. Treating an empty
// enable request as "enable everything" would be an invention.
func TestMonitorIDListIsAllOnlyForTheTwoReportRequests(t *testing.T) {
	p := monProfile()

	all := []uint8{pus.SubtypeReportPMONDefinitions, pus.SubtypeReportFMONDefinitions}
	notAll := []uint8{1, 2, 6, 19, 20, 21, 22, 24}

	for _, subtype := range all {
		request := pus.MonitorIDListRequest{Profile: p, Subtype: subtype}
		if !request.IsAll() {
			t.Errorf("TC[12,%d] with no IDs: IsAll() = false, want true", subtype)
		}
	}
	for _, subtype := range notAll {
		request := pus.MonitorIDListRequest{Profile: p, Subtype: subtype}
		if request.IsAll() {
			t.Errorf("TC[12,%d] with no IDs: IsAll() = true, but no clause says N = 0 means all", subtype)
		}
	}

	// A non-empty list is never "all", whatever the subtype.
	for _, subtype := range append(all, notAll...) {
		request := pus.MonitorIDListRequest{Profile: p, Subtype: subtype, IDs: []uint64{1}}
		if request.IsAll() {
			t.Errorf("TC[12,%d] with one ID: IsAll() = true", subtype)
		}
	}
}

// TestPMONCheckingStatusNameDependsOnTheCheckType is the trap in ST[12]'s
// enumerations. Clause 8.12.3.1b gives three tables, and raw value 3 means
// something different in each. A display that ignored the check type would be
// wrong two thirds of the time and look right.
func TestPMONCheckingStatusNameDependsOnTheCheckType(t *testing.T) {
	cases := []struct {
		raw   pus.PMONCheckingStatus
		check pus.CheckType
		want  string
	}{
		// Table 8-7, expected-value-checking.
		{0, pus.CheckExpectedValue, "expected value"},
		{1, pus.CheckExpectedValue, "unchecked"},
		{2, pus.CheckExpectedValue, "invalid"},
		{3, pus.CheckExpectedValue, "unexpected value"},
		// Table 8-8, limit-checking.
		{0, pus.CheckLimit, "within limits"},
		{1, pus.CheckLimit, "unchecked"},
		{2, pus.CheckLimit, "invalid"},
		{3, pus.CheckLimit, "below low limit"},
		{4, pus.CheckLimit, "above high limit"},
		// Table 8-9, delta-checking.
		{0, pus.CheckDelta, "within thresholds"},
		{1, pus.CheckDelta, "unchecked"},
		{2, pus.CheckDelta, "invalid"},
		{3, pus.CheckDelta, "below low threshold"},
		{4, pus.CheckDelta, "above high threshold"},
	}

	for _, c := range cases {
		if got := c.raw.NameFor(c.check); got != c.want {
			t.Errorf("raw %d under %v = %q, want %q", uint64(c.raw), c.check, got, c.want)
		}
	}

	// The same raw value, three different meanings.
	three := pus.PMONCheckingStatus(3)
	names := map[string]bool{
		three.NameFor(pus.CheckExpectedValue): true,
		three.NameFor(pus.CheckLimit):         true,
		three.NameFor(pus.CheckDelta):         true,
	}
	if len(names) != 3 {
		t.Errorf("raw 3 gives %d distinct names across the three tables, want 3", len(names))
	}

	// Table 8-7 has no raw 4: an expected-value check cannot be above a high
	// limit.
	if got := pus.PMONCheckingStatus(4).NameFor(pus.CheckExpectedValue); got != "unknown" {
		t.Errorf("raw 4 under expected-value-checking = %q, want unknown; Table 8-7 stops at 3", got)
	}
}

// TestCheckTypeEnumerationMatchesTable86 checks Table 8-6.
func TestCheckTypeEnumerationMatchesTable86(t *testing.T) {
	want := map[pus.CheckType]string{
		0: "expected-value-checking",
		1: "limit-checking",
		2: "delta-checking",
	}
	for raw, name := range want {
		if raw.String() != name {
			t.Errorf("check type %d is %q, want %q", uint64(raw), raw.String(), name)
		}
	}
	if got := pus.CheckType(3).String(); got != "unknown" {
		t.Errorf("check type 3 is %q, want unknown", got)
	}
}

// TestMonitoringStatusEnumerations checks Tables 8-10 to 8-13.
func TestMonitoringStatusEnumerations(t *testing.T) {
	if pus.PMONDisabled != 0 || pus.PMONEnabled != 1 ||
		pus.PMONDisabled.String() != "disabled" || pus.PMONEnabled.String() != "enabled" {
		t.Error("Table 8-10 PMON status is wrong")
	}
	if pus.FMONUnprotected != 0 || pus.FMONProtected != 1 ||
		pus.FMONUnprotected.String() != "unprotected" || pus.FMONProtected.String() != "protected" {
		t.Error("Table 8-11 FMON protection status is wrong")
	}
	if pus.FMONDisabled != 0 || pus.FMONEnabled != 1 {
		t.Error("Table 8-12 FMON status is wrong")
	}
	want := map[pus.FMONCheckingStatus]string{
		0: "unchecked", 1: "running", 2: "invalid", 3: "failed",
	}
	for raw, name := range want {
		if raw.String() != name {
			t.Errorf("Table 8-13 raw %d is %q, want %q", uint64(raw), raw.String(), name)
		}
	}
}

// TestSevenMessageTypesNeedAParameterResolver is the design decision this test
// exists to hold. Seven of the twenty-eight carry fields whose widths come
// from the mission's parameter definitions, and those fields sit in the middle
// of a repeated group. A registry without a resolver must refuse them, not
// guess.
func TestSevenMessageTypesNeedAParameterResolver(t *testing.T) {
	p := monProfile()
	withoutResolver, err := pus.NewDefaultRegistry(p)
	if err != nil {
		t.Fatal(err)
	}

	// A body with a count of one and enough octets behind it that a short-read
	// error cannot mask the resolver error. TM[12,9] carries the transition
	// reporting delay ahead of its count, so its count sits two octets later.
	body := make([]byte, 96)
	body[0] = 1
	delayed := make([]byte, 96)
	delayed[2] = 1

	needing := []uint8{
		pus.SubtypeAddPMONDefinitions,
		pus.SubtypeModifyPMONDefinitions,
		pus.SubtypeAddFMONDefinitions,
	}
	for _, subtype := range needing {
		key := pus.MessageKey{Service: pus.ServiceOnBoardMonitoring, Subtype: subtype}
		if _, err := withoutResolver.DecodeRequest(key, body); !errors.Is(err, pus.ErrNoParameterResolver) {
			t.Errorf("TC[12,%d]: err = %v, want ErrNoParameterResolver", subtype, err)
		}
	}

	needingReports := map[uint8][]byte{
		pus.SubtypePMONDefinitionReport:  delayed,
		pus.SubtypeOutOfLimitsReport:     body,
		pus.SubtypeCheckTransitionReport: body,
		pus.SubtypeFMONDefinitionReport:  body,
	}
	for subtype, input := range needingReports {
		key := pus.MessageKey{Service: pus.ServiceOnBoardMonitoring, Subtype: subtype}
		if _, err := withoutResolver.DecodeReport(key, input); !errors.Is(err, pus.ErrNoParameterResolver) {
			t.Errorf("TM[12,%d]: err = %v, want ErrNoParameterResolver", subtype, err)
		}
	}

	if len(needing)+len(needingReports) != 7 {
		t.Fatalf("this test covers %d types, want the 7 that need a resolver",
			len(needing)+len(needingReports))
	}

	// The other twenty-one decode without one. TC[12,1] is an ID list.
	key := pus.MessageKey{Service: pus.ServiceOnBoardMonitoring, Subtype: pus.SubtypeEnablePMONDefinitions}
	if _, err := withoutResolver.DecodeRequest(key, []byte{0x01, 0x00, 0x05}); err != nil {
		t.Errorf("TC[12,1] without a resolver: %v", err)
	}
}

// TestAddPMONDefinitionsRoundTripsEachCheckType covers Figures 8-114 through
// 8-117: the three criteria shapes.
func TestAddPMONDefinitionsRoundTripsEachCheckType(t *testing.T) {
	p := monProfile()

	criteria := []pus.CheckCriteria{
		{
			Type: pus.CheckExpectedValue,
			ExpectedValue: &pus.ExpectedValueCheck{
				Mask:              []byte{0xFF, 0x00},
				ExpectedValue:     []byte{0x12, 0x34},
				EventDefinitionID: 7,
			},
		},
		{
			Type: pus.CheckLimit,
			Limit: &pus.LimitCheck{
				LowLimit:              []byte{0x00, 0x10},
				LowEventDefinitionID:  8,
				HighLimit:             []byte{0x00, 0xF0},
				HighEventDefinitionID: 9,
			},
		},
		{
			Type: pus.CheckDelta,
			Delta: &pus.DeltaCheck{
				LowThreshold:           []byte{0xFF, 0xF0},
				LowEventDefinitionID:   10,
				HighThreshold:          []byte{0x00, 0x10},
				HighEventDefinitionID:  11,
				ConsecutiveDeltaValues: 3,
			},
		},
	}

	for _, c := range criteria {
		t.Run(c.Type.String(), func(t *testing.T) {
			request := pus.AddPMONDefinitionsRequest{
				Profile: p,
				Resolve: twoOctetParameters,
				Definitions: []pus.PMONDefinition{{
					ID:                   5,
					MonitoredParameterID: 0x0100,
					Validity: pus.ValidityCondition{
						ParameterID:   0x0200,
						Mask:          []byte{0x0F, 0xF0},
						ExpectedValue: []byte{0x00, 0x01},
					},
					MonitoringInterval: 1000,
					RepetitionNumber:   4,
					Criteria:           c,
				}},
			}
			encoded, err := request.Encode()
			if err != nil {
				t.Fatal(err)
			}

			got, err := pus.DecodeAddPMONDefinitionsRequest(p, twoOctetParameters, encoded)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Definitions) != 1 {
				t.Fatalf("got %d definitions, want 1", len(got.Definitions))
			}
			d := got.Definitions[0]
			if d.ID != 5 || d.MonitoredParameterID != 0x0100 {
				t.Errorf("IDs = %d/%d, want 5/256", d.ID, d.MonitoredParameterID)
			}
			if d.MonitoringInterval != 1000 || d.RepetitionNumber != 4 {
				t.Errorf("interval/repetition = %d/%d, want 1000/4",
					d.MonitoringInterval, d.RepetitionNumber)
			}
			if d.Validity.ParameterID != 0x0200 {
				t.Errorf("validity parameter = %d, want 512", d.Validity.ParameterID)
			}
			if d.Criteria.Type != c.Type {
				t.Errorf("check type = %v, want %v", d.Criteria.Type, c.Type)
			}

			again, err := got.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, encoded) {
				t.Errorf("re-encoded % x, want % x", again, encoded)
			}
		})
	}
}

// TestCheckCriteriaMustMatchItsType checks a criteria whose set alternative
// disagrees with its type is refused. The receiving end reads the criteria
// according to the type, so a mismatch would be misread rather than noticed.
func TestCheckCriteriaMustMatchItsType(t *testing.T) {
	p := monProfile()

	bad := []pus.CheckCriteria{
		// Says limit, carries expected-value.
		{Type: pus.CheckLimit, ExpectedValue: &pus.ExpectedValueCheck{}},
		// Says expected-value, carries nothing.
		{Type: pus.CheckExpectedValue},
		// Carries two alternatives.
		{
			Type:          pus.CheckExpectedValue,
			ExpectedValue: &pus.ExpectedValueCheck{},
			Limit:         &pus.LimitCheck{},
		},
	}

	for i, c := range bad {
		request := pus.AddPMONDefinitionsRequest{
			Profile:     p,
			Resolve:     twoOctetParameters,
			Definitions: []pus.PMONDefinition{{Criteria: c}},
		}
		if _, err := request.Encode(); !errors.Is(err, pus.ErrCheckCriteriaMismatch) {
			t.Errorf("case %d: err = %v, want ErrCheckCriteriaMismatch", i, err)
		}
	}

	// A check type outside Table 8-6 is its own error.
	request := pus.AddPMONDefinitionsRequest{
		Profile:     p,
		Resolve:     twoOctetParameters,
		Definitions: []pus.PMONDefinition{{Criteria: pus.CheckCriteria{Type: 7}}},
	}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrInvalidCheckType) {
		t.Errorf("err = %v, want ErrInvalidCheckType", err)
	}
}

// TestModifyCarriesNeitherValidityNorInterval checks the difference between
// Figures 8-114 and 8-119. Clause 6.12.3.9.4 modifies a check rather than
// replacing a definition, so the modify request drops both fields, and it
// drops them even when the profile declares the capabilities.
func TestModifyCarriesNeitherValidityNorInterval(t *testing.T) {
	p := monProfile()
	definition := pus.PMONDefinition{
		ID:                   5,
		MonitoredParameterID: 0x0100,
		Validity: pus.ValidityCondition{
			ParameterID:   0x0200,
			Mask:          []byte{0x0F, 0xF0},
			ExpectedValue: []byte{0x00, 0x01},
		},
		MonitoringInterval: 1000,
		RepetitionNumber:   4,
		Criteria: pus.CheckCriteria{
			Type: pus.CheckLimit,
			Limit: &pus.LimitCheck{
				LowLimit:  []byte{0, 1},
				HighLimit: []byte{0, 2},
			},
		},
	}

	add, err := (pus.AddPMONDefinitionsRequest{
		Profile: p, Resolve: twoOctetParameters,
		Definitions: []pus.PMONDefinition{definition},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	modify, err := (pus.ModifyPMONDefinitionsRequest{
		Profile: p, Resolve: twoOctetParameters,
		Definitions: []pus.PMONDefinition{definition},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}

	// The add request is longer by the validity condition and the interval.
	validityWidth := p.ParameterIDSize() + 2 + 2 // parameter ID, mask, expected value
	wantDiff := validityWidth + p.MonitoringIntervalSize()
	if len(add)-len(modify) != wantDiff {
		t.Fatalf("add is %d octets and modify is %d, want a %d-octet difference",
			len(add), len(modify), wantDiff)
	}

	got, err := pus.DecodeModifyPMONDefinitionsRequest(p, twoOctetParameters, modify)
	if err != nil {
		t.Fatal(err)
	}
	d := got.Definitions[0]
	if d.MonitoringInterval != 0 {
		t.Errorf("modify decoded an interval of %d; the field does not travel", d.MonitoringInterval)
	}
	if d.Validity.ParameterID != 0 {
		t.Errorf("modify decoded a validity condition; the field does not travel")
	}
	if d.RepetitionNumber != 4 {
		t.Errorf("repetition = %d, want 4: clause 6.12.3.3j item 1 always carries it", d.RepetitionNumber)
	}
}

// TestPMONDefinitionReportCarriesTheDelayAndStatus checks Figure 8-124: the
// report leads with the maximum transition reporting delay when the subservice
// can change it (clause 6.12.3.10i item 1), and each definition carries a
// status the add request has no field for.
func TestPMONDefinitionReportCarriesTheDelayAndStatus(t *testing.T) {
	p := monProfile()
	definition := pus.PMONDefinition{
		ID:                   5,
		MonitoredParameterID: 0x0100,
		Validity: pus.ValidityCondition{
			ParameterID:   0x0200,
			Mask:          []byte{0x0F, 0xF0},
			ExpectedValue: []byte{0x00, 0x01},
		},
		MonitoringInterval: 1000,
		Status:             pus.PMONEnabled,
		RepetitionNumber:   4,
		Criteria: pus.CheckCriteria{
			Type: pus.CheckLimit,
			Limit: &pus.LimitCheck{
				LowLimit:  []byte{0, 1},
				HighLimit: []byte{0, 2},
			},
		},
	}

	report := pus.PMONDefinitionReport{
		Profile:           p,
		Resolve:           twoOctetParameters,
		MaxReportingDelay: 0x1234,
		Definitions:       []pus.PMONDefinition{definition},
	}
	encoded, err := report.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// The delay is first, ahead of the count.
	if want := []byte{0x12, 0x34, 0x01}; !bytes.Equal(encoded[:3], want) {
		t.Errorf("first three octets = % x, want % x (delay then count)", encoded[:3], want)
	}

	got, err := pus.DecodePMONDefinitionReport(p, twoOctetParameters, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxReportingDelay != 0x1234 {
		t.Errorf("delay = %d, want 0x1234", got.MaxReportingDelay)
	}
	if got.Definitions[0].Status != pus.PMONEnabled {
		t.Errorf("status = %v, want enabled", got.Definitions[0].Status)
	}

	// Without the capability the delay field is gone entirely.
	noDelay := p
	noDelay.SupportsTransitionDelayChange = false
	shorter, err := (pus.PMONDefinitionReport{
		Profile: noDelay, Resolve: twoOctetParameters,
		Definitions: []pus.PMONDefinition{definition},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded)-len(shorter) != p.TransitionDelaySize() {
		t.Errorf("the delay accounts for %d octets, want %d",
			len(encoded)-len(shorter), p.TransitionDelaySize())
	}
}

// TestCheckTransitionMaskPresenceFollowsTheCheckType checks Figure 8-128
// note 1: the expected value check mask travels only for
// expected-value-checking. This is the one deduced presence in ST[12] that
// another field in the same entry decides, so it can be tested on the wire.
func TestCheckTransitionMaskPresenceFollowsTheCheckType(t *testing.T) {
	p := monProfile()
	stamp := time.Time{}

	base := pus.CheckTransition{
		PMONID:                 5,
		MonitoredParameterID:   0x0100,
		ParameterValue:         []byte{0x12, 0x34},
		LimitCrossed:           []byte{0x00, 0xF0},
		PreviousCheckingStatus: pus.PMONNominal,
		CurrentCheckingStatus:  3,
		TransitionTime:         stamp,
	}

	expected := base
	expected.CheckType = pus.CheckExpectedValue
	expected.ExpectedValueCheckMask = []byte{0xFF, 0x00}

	limit := base
	limit.CheckType = pus.CheckLimit

	withMask, err := (pus.CheckTransitionReport{
		Profile: p, Resolve: twoOctetParameters,
		Subtype: pus.SubtypeCheckTransitionReport, Transitions: []pus.CheckTransition{expected},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	withoutMask, err := (pus.CheckTransitionReport{
		Profile: p, Resolve: twoOctetParameters,
		Subtype: pus.SubtypeCheckTransitionReport, Transitions: []pus.CheckTransition{limit},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(withMask)-len(withoutMask) != 2 {
		t.Fatalf("the mask accounts for %d octets, want 2", len(withMask)-len(withoutMask))
	}

	got, err := pus.DecodeCheckTransitionReport(p, twoOctetParameters,
		pus.SubtypeCheckTransitionReport, withMask)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Transitions[0].ExpectedValueCheckMask, []byte{0xFF, 0x00}) {
		t.Errorf("mask = % x, want ff 00", got.Transitions[0].ExpectedValueCheckMask)
	}

	got, err = pus.DecodeCheckTransitionReport(p, twoOctetParameters,
		pus.SubtypeCheckTransitionReport, withoutMask)
	if err != nil {
		t.Fatal(err)
	}
	if got.Transitions[0].ExpectedValueCheckMask != nil {
		t.Errorf("mask = % x, want nil for a limit check",
			got.Transitions[0].ExpectedValueCheckMask)
	}
}

// TestOutOfLimitsAndCheckTransitionShareOneStructure checks Figures 8-128 and
// 8-129 are the same structure, and that the subtype is what tells them apart.
func TestOutOfLimitsAndCheckTransitionShareOneStructure(t *testing.T) {
	p := monProfile()
	transition := pus.CheckTransition{
		PMONID:               5,
		MonitoredParameterID: 0x0100,
		CheckType:            pus.CheckLimit,
		ParameterValue:       []byte{0x12, 0x34},
		LimitCrossed:         []byte{0x00, 0xF0},
	}

	for _, subtype := range []uint8{pus.SubtypeOutOfLimitsReport, pus.SubtypeCheckTransitionReport} {
		report := pus.CheckTransitionReport{
			Profile: p, Resolve: twoOctetParameters,
			Subtype: subtype, Transitions: []pus.CheckTransition{transition},
		}
		encoded, err := report.Encode()
		if err != nil {
			t.Fatalf("TM[12,%d]: %v", subtype, err)
		}
		got, err := pus.DecodeCheckTransitionReport(p, twoOctetParameters, subtype, encoded)
		if err != nil {
			t.Fatal(err)
		}
		if got.Subtype != subtype {
			t.Errorf("subtype = %d, want %d", got.Subtype, subtype)
		}
		if got.Key().Subtype != subtype {
			t.Errorf("Key() subtype = %d, want %d", got.Key().Subtype, subtype)
		}
	}

	// A third subtype is refused rather than encoded under a type nobody
	// decodes this way.
	bad := pus.CheckTransitionReport{Profile: p, Resolve: twoOctetParameters, Subtype: 9}
	if _, err := bad.Encode(); !errors.Is(err, pus.ErrWrongMessageType) {
		t.Errorf("err = %v, want ErrWrongMessageType", err)
	}
}

// TestFMONDefinitionsRoundTrip covers Figures 8-135 and 8-138, including the
// nested PMON ID list each definition carries.
func TestFMONDefinitionsRoundTrip(t *testing.T) {
	p := monProfile()
	definition := pus.FMONDefinition{
		ID: 3,
		Validity: pus.ValidityCondition{
			ParameterID:   0x0200,
			Mask:          []byte{0x0F, 0xF0},
			ExpectedValue: []byte{0x00, 0x01},
		},
		EventDefinitionID: 42,
		MinPMONFailing:    2,
		PMONIDs:           []uint64{10, 11, 12},
	}

	add := pus.AddFMONDefinitionsRequest{
		Profile: p, Resolve: twoOctetParameters,
		Definitions: []pus.FMONDefinition{definition},
	}
	encoded, err := add.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pus.DecodeAddFMONDefinitionsRequest(p, twoOctetParameters, encoded)
	if err != nil {
		t.Fatal(err)
	}
	d := got.Definitions[0]
	if d.ID != 3 || d.EventDefinitionID != 42 || d.MinPMONFailing != 2 {
		t.Errorf("definition = %+v", d)
	}
	if len(d.PMONIDs) != 3 || d.PMONIDs[2] != 12 {
		t.Errorf("PMON IDs = %v, want [10 11 12]", d.PMONIDs)
	}

	// The report adds the two statuses.
	withStatus := definition
	withStatus.ProtectionStatus = pus.FMONProtected
	withStatus.Status = pus.FMONEnabled

	report := pus.FMONDefinitionReport{
		Profile: p, Resolve: twoOctetParameters,
		Definitions: []pus.FMONDefinition{withStatus},
	}
	reported, err := report.Encode()
	if err != nil {
		t.Fatal(err)
	}
	wantDiff := p.FMONProtectionStatusSize() + p.FMONStatusSize()
	if len(reported)-len(encoded) != wantDiff {
		t.Fatalf("the report is %d octets longer than the add, want %d",
			len(reported)-len(encoded), wantDiff)
	}

	gotReport, err := pus.DecodeFMONDefinitionReport(p, twoOctetParameters, reported)
	if err != nil {
		t.Fatal(err)
	}
	r := gotReport.Definitions[0]
	if r.ProtectionStatus != pus.FMONProtected || r.Status != pus.FMONEnabled {
		t.Errorf("statuses = %v/%v, want protected/enabled", r.ProtectionStatus, r.Status)
	}
}

// TestFMONCapabilityFlagsChangeTheWireFormat checks the three declarations
// that decide FMON field presence, all of clause 6.12.4.
func TestFMONCapabilityFlagsChangeTheWireFormat(t *testing.T) {
	full := monProfile()
	bare := full
	bare.SupportsFMONConditionalChecking = false
	bare.SupportsMinPMONFailingNumber = false
	bare.SupportsFMONProtection = false

	definition := pus.FMONDefinition{
		ID: 3,
		Validity: pus.ValidityCondition{
			ParameterID:   0x0200,
			Mask:          []byte{0x0F, 0xF0},
			ExpectedValue: []byte{0x00, 0x01},
		},
		EventDefinitionID: 42,
		MinPMONFailing:    2,
		PMONIDs:           []uint64{10},
	}

	long, err := (pus.FMONDefinitionReport{
		Profile: full, Resolve: twoOctetParameters,
		Definitions: []pus.FMONDefinition{definition},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	short, err := (pus.FMONDefinitionReport{
		Profile: bare, Resolve: twoOctetParameters,
		Definitions: []pus.FMONDefinition{definition},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}

	validityWidth := full.ParameterIDSize() + 2 + 2
	wantDiff := validityWidth + full.MinPMONFailingSize() + full.FMONProtectionStatusSize()
	if len(long)-len(short) != wantDiff {
		t.Fatalf("the three capabilities account for %d octets, want %d",
			len(long)-len(short), wantDiff)
	}

	// With none of them declared, the report needs no resolver at all: the
	// validity condition was the only part that used one.
	if _, err := pus.DecodeFMONDefinitionReport(bare, nil, short); err != nil {
		t.Errorf("a capability-free FMON report should not need a resolver: %v", err)
	}
}

// TestFieldWidthMustMatchTheParameterLayout checks a value of the wrong length
// is refused rather than padded. These fields sit before others in a repeated
// group, so a wrong width shifts everything after it.
func TestFieldWidthMustMatchTheParameterLayout(t *testing.T) {
	p := monProfile()

	request := pus.AddPMONDefinitionsRequest{
		Profile: p,
		Resolve: twoOctetParameters,
		Definitions: []pus.PMONDefinition{{
			MonitoredParameterID: 0x0100,
			Validity: pus.ValidityCondition{
				ParameterID:   0x0200,
				Mask:          []byte{0x0F, 0xF0},
				ExpectedValue: []byte{0x00, 0x01},
			},
			Criteria: pus.CheckCriteria{
				Type: pus.CheckLimit,
				Limit: &pus.LimitCheck{
					// Three octets where the layout says two.
					LowLimit:  []byte{0, 1, 2},
					HighLimit: []byte{0, 2},
				},
			},
		}},
	}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrFieldWidthMismatch) {
		t.Errorf("err = %v, want ErrFieldWidthMismatch", err)
	}
}

// TestExpectedValueSpareIsDeclared checks clause 8.12.2.5d: the spare in the
// expected-value criteria is a per-subservice declaration, and it is exactly
// as wide as an event definition ID (Figure 8-115).
func TestExpectedValueSpareIsDeclared(t *testing.T) {
	without := monProfile()
	with := without
	with.ExpectedValueSpare = true

	build := func(p pus.MissionProfile, spare []byte) pus.AddPMONDefinitionsRequest {
		return pus.AddPMONDefinitionsRequest{
			Profile: p,
			Resolve: twoOctetParameters,
			Definitions: []pus.PMONDefinition{{
				MonitoredParameterID: 0x0100,
				Validity: pus.ValidityCondition{
					ParameterID:   0x0200,
					Mask:          []byte{0, 0},
					ExpectedValue: []byte{0, 0},
				},
				Criteria: pus.CheckCriteria{
					Type: pus.CheckExpectedValue,
					ExpectedValue: &pus.ExpectedValueCheck{
						Mask:          []byte{0xFF, 0x00},
						Spare:         spare,
						ExpectedValue: []byte{0x12, 0x34},
					},
				},
			}},
		}
	}

	short, err := build(without, nil).Encode()
	if err != nil {
		t.Fatal(err)
	}
	long, err := build(with, []byte{0x00, 0x00}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(long)-len(short) != with.EventDefinitionIDSize() {
		t.Fatalf("the spare accounts for %d octets, want %d (an event definition ID)",
			len(long)-len(short), with.EventDefinitionIDSize())
	}

	got, err := pus.DecodeAddPMONDefinitionsRequest(with, twoOctetParameters, long)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Definitions[0].Criteria.ExpectedValue.Spare) != with.EventDefinitionIDSize() {
		t.Errorf("decoded spare is %d octets, want %d",
			len(got.Definitions[0].Criteria.ExpectedValue.Spare), with.EventDefinitionIDSize())
	}
}

// TestST12ListsRefuseAHostileCount checks an untrusted N cannot drive an
// allocation in any of the list-bearing messages.
func TestST12ListsRefuseAHostileCount(t *testing.T) {
	p := monProfile()
	p.MonitorCountBytes = 4
	hostile := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	if _, err := pus.DecodePMONStatusReport(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("PMON status: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeFMONStatusReport(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("FMON status: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeAddPMONDefinitionsRequest(p, twoOctetParameters, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("add PMON: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeAddFMONDefinitionsRequest(p, twoOctetParameters, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("add FMON: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeCheckTransitionReport(p, twoOctetParameters,
		pus.SubtypeCheckTransitionReport, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("transitions: err = %v, want ErrDataTooShort", err)
	}
	// The definition report has the delay ahead of the count.
	delayed := append([]byte{0x00, 0x00}, hostile...)
	if _, err := pus.DecodePMONDefinitionReport(p, twoOctetParameters, delayed); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("definition report: err = %v, want ErrDataTooShort", err)
	}
}

// TestST12ThroughTheRegistry round-trips one message of each shape.
func TestST12ThroughTheRegistry(t *testing.T) {
	p := monProfile()
	r, err := pus.NewDefaultRegistry(p, pus.WithParameterResolver(twoOctetParameters))
	if err != nil {
		t.Fatal(err)
	}
	if r.ParameterResolver() == nil {
		t.Fatal("the registry did not keep the resolver")
	}

	validity := pus.ValidityCondition{
		ParameterID:   0x0200,
		Mask:          []byte{0x0F, 0xF0},
		ExpectedValue: []byte{0x00, 0x01},
	}
	criteria := pus.CheckCriteria{
		Type:  pus.CheckLimit,
		Limit: &pus.LimitCheck{LowLimit: []byte{0, 1}, HighLimit: []byte{0, 2}},
	}
	pmon := pus.PMONDefinition{
		ID: 5, MonitoredParameterID: 0x0100, Validity: validity,
		MonitoringInterval: 100, RepetitionNumber: 3, Criteria: criteria,
	}
	fmon := pus.FMONDefinition{
		ID: 3, Validity: validity, EventDefinitionID: 42,
		MinPMONFailing: 2, PMONIDs: []uint64{10},
	}

	requests := []pus.Request{
		pus.MonitorControlRequest{Subtype: pus.SubtypeDeleteAllPMON},
		pus.MonitorIDListRequest{Profile: p, Subtype: pus.SubtypeEnablePMONDefinitions, IDs: []uint64{1, 2}},
		pus.MonitorIDListRequest{Profile: p, Subtype: pus.SubtypeProtectFMONDefinitions, IDs: []uint64{3}},
		pus.ChangeTransitionDelayRequest{Profile: p, MaxReportingDelay: 500},
		pus.AddPMONDefinitionsRequest{Profile: p, Resolve: twoOctetParameters, Definitions: []pus.PMONDefinition{pmon}},
		pus.ModifyPMONDefinitionsRequest{Profile: p, Resolve: twoOctetParameters, Definitions: []pus.PMONDefinition{pmon}},
		pus.AddFMONDefinitionsRequest{Profile: p, Resolve: twoOctetParameters, Definitions: []pus.FMONDefinition{fmon}},
	}

	for _, request := range requests {
		encoded, err := request.Encode()
		if err != nil {
			t.Fatalf("%v: encode: %v", request.Key(), err)
		}
		decoded, err := r.DecodeRequest(request.Key(), encoded)
		if err != nil {
			t.Fatalf("%v: decode: %v", request.Key(), err)
		}
		again, err := decoded.Encode()
		if err != nil {
			t.Fatalf("%v: re-encode: %v", request.Key(), err)
		}
		if !bytes.Equal(again, encoded) {
			t.Errorf("%v: re-encoded % x, want % x", request.Key(), again, encoded)
		}
	}

	pmonReported := pmon
	pmonReported.Status = pus.PMONEnabled
	fmonReported := fmon
	fmonReported.Status = pus.FMONEnabled
	fmonReported.ProtectionStatus = pus.FMONProtected

	reports := []pus.Report{
		&pus.PMONDefinitionReport{
			Profile: p, Resolve: twoOctetParameters, MaxReportingDelay: 7,
			Definitions: []pus.PMONDefinition{pmonReported},
		},
		&pus.CheckTransitionReport{
			Profile: p, Resolve: twoOctetParameters, Subtype: pus.SubtypeOutOfLimitsReport,
			Transitions: []pus.CheckTransition{{
				PMONID: 5, MonitoredParameterID: 0x0100, CheckType: pus.CheckLimit,
				ParameterValue: []byte{1, 2}, LimitCrossed: []byte{3, 4},
			}},
		},
		&pus.CheckTransitionReport{
			Profile: p, Resolve: twoOctetParameters, Subtype: pus.SubtypeCheckTransitionReport,
			Transitions: []pus.CheckTransition{{
				PMONID: 6, MonitoredParameterID: 0x0100, CheckType: pus.CheckExpectedValue,
				ExpectedValueCheckMask: []byte{0xFF, 0},
				ParameterValue:         []byte{1, 2}, LimitCrossed: []byte{3, 4},
			}},
		},
		&pus.PMONStatusReport{Profile: p, Definitions: []pus.PMONStatusEntry{{ID: 1, Status: pus.PMONEnabled}}},
		&pus.FMONDefinitionReport{Profile: p, Resolve: twoOctetParameters, Definitions: []pus.FMONDefinition{fmonReported}},
		&pus.FMONStatusReport{Profile: p, Definitions: []pus.FMONStatusEntry{{
			ID: 1, ProtectionStatus: pus.FMONProtected, Status: pus.FMONEnabled,
			CheckingStatus: pus.FMONRunning,
		}}},
	}

	for _, report := range reports {
		encoded, err := report.Encode()
		if err != nil {
			t.Fatalf("%v: encode: %v", report.Key(), err)
		}
		decoded, err := r.DecodeReport(report.Key(), encoded)
		if err != nil {
			t.Fatalf("%v: decode: %v", report.Key(), err)
		}
		again, err := decoded.Encode()
		if err != nil {
			t.Fatalf("%v: re-encode: %v", report.Key(), err)
		}
		if !bytes.Equal(again, encoded) {
			t.Errorf("%v: re-encoded % x, want % x", report.Key(), again, encoded)
		}
	}
}
