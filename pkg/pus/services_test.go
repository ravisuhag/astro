package pus_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
)

func TestCUCImplicitAndExplicitAgree(t *testing.T) {
	// Table 7-10: PFC 3 to 46 leave the P-field implicit; PFC 0 carries it.
	// Both must recover the same instant.
	stamp := time.Date(2026, 7, 12, 10, 30, 15, 0, time.UTC)

	implicit := pus.DefaultProfile()
	implicit.TimeFormat = pus.TimeCUC
	explicit := implicit
	explicit.TimeFormat = pus.TimeCUCExplicit

	// The explicit form is exactly one octet wider.
	if explicit.TimeSize() != implicit.TimeSize()+1 {
		t.Errorf("explicit time is %d octets, implicit %d; want a one-octet P-field difference",
			explicit.TimeSize(), implicit.TimeSize())
	}
	// The implicit form is coarse + fine, nothing more.
	if implicit.TimeSize() != implicit.CUCCoarseBytes+implicit.CUCFineBytes {
		t.Errorf("implicit time is %d octets, want coarse+fine = %d",
			implicit.TimeSize(), implicit.CUCCoarseBytes+implicit.CUCFineBytes)
	}

	for name, profile := range map[string]pus.MissionProfile{"implicit": implicit, "explicit": explicit} {
		t.Run(name, func(t *testing.T) {
			h := profile.NewTMHeader(3, 25, 1, stamp)
			encoded, err := h.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got := &pus.TMHeader{Profile: profile}
			if err := got.Decode(encoded); err != nil {
				t.Fatal(err)
			}
			if !got.Time.Equal(stamp) {
				t.Errorf("time = %s, want %s", got.Time, stamp)
			}
		})
	}
}

func TestCUCAgencyEpoch(t *testing.T) {
	// Clause 7.4.4 note 1 allows an agency-defined epoch, which changes the
	// CUC time code level and therefore the implicit P-field.
	epoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	stamp := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)

	p := pus.DefaultProfile()
	p.CUCEpoch = epoch

	h := p.NewTMHeader(3, 25, 1, stamp)
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got := &pus.TMHeader{Profile: p}
	if err := got.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if !got.Time.Equal(stamp) {
		t.Errorf("time = %s, want %s", got.Time, stamp)
	}
}

func TestRequestIDMatchesSPPPrimaryHeader(t *testing.T) {
	// Figure 8-1 lays the request ID out exactly as the first four octets of
	// a CCSDS primary header.
	id := pus.RequestID{
		PacketVersion:       0,
		PacketType:          1,
		SecondaryHeaderFlag: 1,
		APID:                0x2AB,
		SequenceFlags:       3,
		SequenceCount:       0x1234,
	}
	encoded := id.Encode()
	if len(encoded) != pus.RequestIDSize {
		t.Fatalf("encoded %d octets, want %d", len(encoded), pus.RequestIDSize)
	}

	got, err := pus.DecodeRequestID(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("got %+v, want %+v", got, id)
	}
}

func TestVerificationReportsRoundTrip(t *testing.T) {
	id := pus.RequestID{PacketType: 1, APID: 100, SequenceCount: 7}

	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			for subtype := uint8(1); subtype <= 8; subtype++ {
				r := &pus.VerificationReport{
					Profile:   profile,
					Subtype:   subtype,
					RequestID: id,
				}
				if r.HasStepID() {
					r.StepID = 5
				}
				if r.IsFailure() {
					r.FailureCode = 3
					r.FailureData = []byte{0xAA, 0xBB}
				}

				encoded, err := r.Encode()
				if err != nil {
					t.Fatalf("subtype %d: %v", subtype, err)
				}
				got, err := pus.DecodeVerificationReport(profile, subtype, encoded)
				if err != nil {
					t.Fatalf("subtype %d: %v", subtype, err)
				}
				if got.RequestID != id {
					t.Errorf("subtype %d: request ID = %+v, want %+v", subtype, got.RequestID, id)
				}
				if r.HasStepID() && got.StepID != 5 {
					t.Errorf("subtype %d: step ID = %d, want 5", subtype, got.StepID)
				}
				if r.IsFailure() {
					if got.FailureCode != 3 {
						t.Errorf("subtype %d: failure code = %d, want 3", subtype, got.FailureCode)
					}
					if !bytes.Equal(got.FailureData, []byte{0xAA, 0xBB}) {
						t.Errorf("subtype %d: failure data = %x", subtype, got.FailureData)
					}
				}
			}
		})
	}
}

func TestVerificationSubtypeClassification(t *testing.T) {
	// Clause 8.1.2: odd subtypes are successes, even are failures; only 5 and
	// 6 carry a step ID.
	for subtype := uint8(1); subtype <= 8; subtype++ {
		r := &pus.VerificationReport{Profile: pus.DefaultProfile(), Subtype: subtype}
		wantFailure := subtype%2 == 0
		if r.IsFailure() != wantFailure {
			t.Errorf("subtype %d: IsFailure = %t, want %t", subtype, r.IsFailure(), wantFailure)
		}
		wantStep := subtype == 5 || subtype == 6
		if r.HasStepID() != wantStep {
			t.Errorf("subtype %d: HasStepID = %t, want %t", subtype, r.HasStepID(), wantStep)
		}
	}
}

func TestVerificationRejectsUnknownSubtype(t *testing.T) {
	r := &pus.VerificationReport{Profile: pus.DefaultProfile(), Subtype: 9}
	if _, err := r.Encode(); !errors.Is(err, pus.ErrWrongMessageType) {
		t.Errorf("error = %v, want ErrWrongMessageType", err)
	}
}

func TestEventReportRoundTrip(t *testing.T) {
	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			for _, sev := range []pus.Severity{
				pus.SeverityInformative, pus.SeverityLow, pus.SeverityMedium, pus.SeverityHigh,
			} {
				r := &pus.EventReport{
					Profile:           profile,
					Severity:          sev,
					EventDefinitionID: 42,
					AuxiliaryData:     []byte{1, 2, 3},
				}
				encoded, err := r.Encode()
				if err != nil {
					t.Fatal(err)
				}
				got, err := pus.DecodeEventReport(profile, sev, encoded)
				if err != nil {
					t.Fatal(err)
				}
				if got.EventDefinitionID != 42 {
					t.Errorf("%s: event ID = %d, want 42", sev, got.EventDefinitionID)
				}
				if !bytes.Equal(got.AuxiliaryData, []byte{1, 2, 3}) {
					t.Errorf("%s: auxiliary data = %x", sev, got.AuxiliaryData)
				}
				if got.Key().Subtype != uint8(sev) {
					t.Errorf("%s: subtype = %d, want %d", sev, got.Key().Subtype, uint8(sev))
				}
			}
		})
	}
}

func TestEventReportRejectsInvalidSeverity(t *testing.T) {
	r := &pus.EventReport{Profile: pus.DefaultProfile(), Severity: pus.Severity(9)}
	if _, err := r.Encode(); !errors.Is(err, pus.ErrInvalidSeverity) {
		t.Errorf("error = %v, want ErrInvalidSeverity", err)
	}
}

func TestEventControlRoundTrip(t *testing.T) {
	profile := pus.DefaultProfile()
	for _, enable := range []bool{true, false} {
		r := &pus.EventControlRequest{
			Profile:            profile,
			Enable:             enable,
			EventDefinitionIDs: []uint64{1, 2, 300},
		}
		encoded, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := pus.DecodeEventControlRequest(profile, enable, encoded)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.EventDefinitionIDs) != 3 {
			t.Fatalf("got %d event IDs, want 3", len(got.EventDefinitionIDs))
		}
		wantSubtype := uint8(6)
		if enable {
			wantSubtype = 5
		}
		if got.Key().Subtype != wantSubtype {
			t.Errorf("subtype = %d, want %d", got.Key().Subtype, wantSubtype)
		}
	}
}

func TestEventControlRejectsImpossibleCount(t *testing.T) {
	// A count field claiming more entries than the remaining bytes can hold
	// must be refused, not used to size an allocation.
	profile := pus.DefaultProfile()
	data := []byte{0xFF} // count 255, then nothing
	if _, err := pus.DecodeEventControlRequest(profile, true, data); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("error = %v, want ErrDataTooShort", err)
	}
}

func TestHousekeepingStructureRoundTrip(t *testing.T) {
	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			s := &pus.HousekeepingStructure{
				Profile:            profile,
				StructureID:        7,
				CollectionInterval: 100,
				ParameterIDs:       []uint64{10, 20, 30},
				SuperCommutated: []pus.SuperCommutatedSet{
					{RepetitionNumber: 4, ParameterIDs: []uint64{40, 50}},
					{RepetitionNumber: 2, ParameterIDs: []uint64{60}},
				},
			}
			encoded, err := s.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got, err := pus.DecodeHousekeepingStructure(profile, encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got.StructureID != 7 {
				t.Errorf("structure ID = %d, want 7", got.StructureID)
			}
			if got.CollectionInterval != 100 {
				t.Errorf("interval = %d, want 100", got.CollectionInterval)
			}
			if len(got.ParameterIDs) != 3 {
				t.Fatalf("got %d parameters, want 3", len(got.ParameterIDs))
			}
			if len(got.SuperCommutated) != 2 {
				t.Fatalf("got %d super-commutated sets, want 2", len(got.SuperCommutated))
			}
			if got.SuperCommutated[0].RepetitionNumber != 4 {
				t.Errorf("first repetition = %d, want 4", got.SuperCommutated[0].RepetitionNumber)
			}
			if len(got.SuperCommutated[1].ParameterIDs) != 1 {
				t.Errorf("second set has %d parameters, want 1", len(got.SuperCommutated[1].ParameterIDs))
			}
		})
	}
}

func TestHousekeepingReportRoundTrip(t *testing.T) {
	profile := pus.DefaultProfile()
	r := &pus.HousekeepingReport{
		Profile:         profile,
		StructureID:     3,
		ParameterValues: []byte{0x01, 0x02, 0x03, 0x04},
	}
	encoded, err := r.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pus.DecodeHousekeepingReport(profile, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.StructureID != 3 {
		t.Errorf("structure ID = %d, want 3", got.StructureID)
	}
	if !bytes.Equal(got.ParameterValues, r.ParameterValues) {
		t.Errorf("values = %x, want %x", got.ParameterValues, r.ParameterValues)
	}
}

func TestHousekeepingControlRoundTrip(t *testing.T) {
	profile := pus.DefaultProfile()
	for _, subtype := range []uint8{
		pus.SubtypeDeleteHKStructure, pus.SubtypeEnableHKGeneration, pus.SubtypeDisableHKGeneration,
	} {
		r := &pus.HousekeepingControlRequest{
			Profile:      profile,
			Subtype:      subtype,
			StructureIDs: []uint64{1, 2, 3},
		}
		encoded, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := pus.DecodeHousekeepingControlRequest(profile, subtype, encoded)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.StructureIDs) != 3 {
			t.Errorf("subtype %d: got %d structure IDs, want 3", subtype, len(got.StructureIDs))
		}
	}
}

func TestST17RoundTrip(t *testing.T) {
	// TC[17,1] and TM[17,2] carry no body.
	if body, err := (pus.AreYouAliveRequest{}).Encode(); err != nil || len(body) != 0 {
		t.Errorf("TC[17,1] body = %x (err %v), want empty", body, err)
	}
	if body, err := (pus.AreYouAliveReport{}).Encode(); err != nil || len(body) != 0 {
		t.Errorf("TM[17,2] body = %x (err %v), want empty", body, err)
	}

	req := pus.OnBoardConnectionRequest{APID: 0x123}
	encoded, err := req.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pus.DecodeOnBoardConnectionRequest(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.APID != 0x123 {
		t.Errorf("APID = %#x, want 0x123", got.APID)
	}
}

func TestRegistryDispatch(t *testing.T) {
	profile := pus.DefaultProfile()
	registry, err := pus.NewDefaultRegistry(profile)
	if err != nil {
		t.Fatal(err)
	}

	// Every service this package ships must be registered.
	wantReports := []pus.MessageKey{
		{Service: 1, Subtype: 1}, {Service: 1, Subtype: 8},
		{Service: 3, Subtype: 25},
		{Service: 5, Subtype: 1}, {Service: 5, Subtype: 4},
		{Service: 17, Subtype: 2}, {Service: 17, Subtype: 4},
	}
	for _, key := range wantReports {
		if _, err := registry.DecodeReport(key, make([]byte, 16)); errors.Is(err, pus.ErrUnknownMessageType) {
			t.Errorf("report %s is not registered", key)
		}
	}

	wantRequests := []pus.MessageKey{
		{Service: 3, Subtype: 1}, {Service: 3, Subtype: 5}, {Service: 3, Subtype: 6},
		{Service: 5, Subtype: 5}, {Service: 5, Subtype: 6},
		{Service: 17, Subtype: 1}, {Service: 17, Subtype: 3},
	}
	for _, key := range wantRequests {
		if _, err := registry.DecodeRequest(key, make([]byte, 16)); errors.Is(err, pus.ErrUnknownMessageType) {
			t.Errorf("request %s is not registered", key)
		}
	}
}

func TestRegistryRejectsUnknownAndDuplicate(t *testing.T) {
	registry, err := pus.NewRegistry(pus.DefaultProfile())
	if err != nil {
		t.Fatal(err)
	}

	key := pus.MessageKey{Service: 200, Subtype: 1}
	if _, err := registry.DecodeReport(key, nil); !errors.Is(err, pus.ErrUnknownMessageType) {
		t.Errorf("error = %v, want ErrUnknownMessageType", err)
	}

	decoder := func(pus.MissionProfile, []byte) (pus.Report, error) { return nil, nil }
	if err := registry.RegisterReport(key, decoder); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterReport(key, decoder); !errors.Is(err, pus.ErrDuplicateMessageType) {
		t.Errorf("error = %v, want ErrDuplicateMessageType", err)
	}
}
