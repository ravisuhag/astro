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
	// Clause 7.4.3.1j note 1 allows an agency-defined epoch, which changes the
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
			for _, subtype := range []uint8{1, 2, 3, 4, 5, 6, 7, 8, 10} {
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
	// Clause 8.1.2: odd subtypes are successes, even are failures — TM[1,10]
	// included; only 5 and 6 carry a step ID.
	for _, subtype := range []uint8{1, 2, 3, 4, 5, 6, 7, 8, 10} {
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
	// The standard defines no TM[1,9], and nothing past TM[1,10].
	for _, subtype := range []uint8{0, 9, 11} {
		r := &pus.VerificationReport{Profile: pus.DefaultProfile(), Subtype: subtype}
		if _, err := r.Encode(); !errors.Is(err, pus.ErrWrongMessageType) {
			t.Errorf("subtype %d: error = %v, want ErrWrongMessageType", subtype, err)
		}
	}
}

func TestFailedRoutingReportRoundTrip(t *testing.T) {
	// TM[1,10] carries a request ID and a failure notice, like TM[1,2].
	profile := pus.DefaultProfile()
	r := &pus.VerificationReport{
		Profile:     profile,
		Subtype:     pus.SubtypeRoutingFailure,
		RequestID:   pus.RequestID{PacketType: 1, APID: 42, SequenceCount: 9},
		FailureCode: 7,
		FailureData: []byte{0x01, 0x02},
	}
	if !r.IsFailure() {
		t.Error("TM[1,10] must carry a failure notice")
	}
	if r.HasStepID() {
		t.Error("TM[1,10] must not carry a step ID")
	}

	encoded, err := r.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pus.DecodeVerificationReport(profile, pus.SubtypeRoutingFailure, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestID != r.RequestID {
		t.Errorf("request ID = %+v, want %+v", got.RequestID, r.RequestID)
	}
	if got.FailureCode != 7 {
		t.Errorf("failure code = %d, want 7", got.FailureCode)
	}
	if !bytes.Equal(got.FailureData, r.FailureData) {
		t.Errorf("failure data = %x, want %x", got.FailureData, r.FailureData)
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

func TestListDecodersRejectZeroWidthElements(t *testing.T) {
	// A profile may declare a zero width for an absent optional field, but a
	// message whose count says entries follow cannot be decoded against it: the
	// entries would consume no input, so a hostile count would drive an
	// unbounded allocation. Every list decoder must refuse the pair.
	t.Run("TC[5,5] event control", func(t *testing.T) {
		p := pus.DefaultProfile()
		p.EventDefinitionIDBytes = 0
		data := []byte{0xFF} // count 255 over zero-width elements
		if _, err := pus.DecodeEventControlRequest(p, true, data); !errors.Is(err, pus.ErrInvalidProfile) {
			t.Errorf("error = %v, want ErrInvalidProfile", err)
		}
	})

	t.Run("TM[5,8] disabled events list", func(t *testing.T) {
		p := pus.DefaultProfile()
		p.EventDefinitionIDBytes = 0
		data := []byte{0xFF}
		if _, err := pus.DecodeDisabledEventsReport(p, data); !errors.Is(err, pus.ErrInvalidProfile) {
			t.Errorf("error = %v, want ErrInvalidProfile", err)
		}
	})

	t.Run("TC[3,1] parameter list", func(t *testing.T) {
		p := pus.DefaultProfile()
		p.ParameterIDBytes = 0
		// structure ID (1) + interval (4) + hostile N1
		data := []byte{0x01, 0, 0, 0, 100, 0xFF}
		if _, err := pus.DecodeHousekeepingStructure(p, data); !errors.Is(err, pus.ErrInvalidProfile) {
			t.Errorf("error = %v, want ErrInvalidProfile", err)
		}
	})

	t.Run("TC[3,1] super-commutated list", func(t *testing.T) {
		p := pus.DefaultProfile()
		p.ParameterIDBytes = 0
		// structure ID (1) + interval (4) + N1=0 + NFA=1 + repetition + hostile N2
		data := []byte{0x01, 0, 0, 0, 100, 0x00, 0x01, 0x02, 0xFF}
		if _, err := pus.DecodeHousekeepingStructure(p, data); !errors.Is(err, pus.ErrInvalidProfile) {
			t.Errorf("error = %v, want ErrInvalidProfile", err)
		}
	})

	t.Run("TC[3,5] structure ID list", func(t *testing.T) {
		p := pus.DefaultProfile()
		p.HousekeepingStructureIDBytes = 0
		data := []byte{0xFF}
		if _, err := pus.DecodeHousekeepingControlRequest(p, pus.SubtypeEnableHKGeneration, data); !errors.Is(err, pus.ErrInvalidProfile) {
			t.Errorf("error = %v, want ErrInvalidProfile", err)
		}
	})
}

func TestListDecodersSurviveHugeCounts(t *testing.T) {
	// An 8-octet count of 2^64-1 must not overflow the bounds arithmetic into
	// accepting the list.
	p := pus.DefaultProfile()
	p.CountBytes = 8
	p.EventDefinitionIDBytes = 8
	data := append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, make([]byte, 64)...)
	if _, err := pus.DecodeEventControlRequest(p, true, data); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("error = %v, want ErrDataTooShort", err)
	}
}

func TestDisabledEventsRoundTrip(t *testing.T) {
	// TC[5,7] carries no body; TM[5,8] answers with the disabled list.
	if body, err := (pus.ReportDisabledEventsRequest{}).Encode(); err != nil || len(body) != 0 {
		t.Errorf("TC[5,7] body = %x (err %v), want empty", body, err)
	}
	if _, err := pus.DecodeReportDisabledEventsRequest(nil); err != nil {
		t.Errorf("decoding an empty TC[5,7]: %v", err)
	}
	if _, err := pus.DecodeReportDisabledEventsRequest([]byte{0}); !errors.Is(err, pus.ErrTrailingBytes) {
		t.Errorf("TC[5,7] with a body: error = %v, want ErrTrailingBytes", err)
	}

	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			r := &pus.DisabledEventsReport{
				Profile:            profile,
				EventDefinitionIDs: []uint64{4, 8, 15},
			}
			encoded, err := r.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got, err := pus.DecodeDisabledEventsReport(profile, encoded)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.EventDefinitionIDs) != 3 || got.EventDefinitionIDs[2] != 15 {
				t.Errorf("event IDs = %v, want [4 8 15]", got.EventDefinitionIDs)
			}
			if got.Key() != (pus.MessageKey{Service: 5, Subtype: 8}) {
				t.Errorf("key = %s, want [5,8]", got.Key())
			}
		})
	}
}

func TestFixedSizeBodiesRejectTrailingOctets(t *testing.T) {
	// The PUS acceptance checks verify a request against its type's structure.
	// A body longer than the type allows is malformed, not padded.
	profile := pus.DefaultProfile()

	t.Run("TM[1,1] success report", func(t *testing.T) {
		r := &pus.VerificationReport{Profile: profile, Subtype: pus.SubtypeAcceptSuccess}
		encoded, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pus.DecodeVerificationReport(profile, r.Subtype, append(encoded, 0xEE)); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("TC[5,5] event control", func(t *testing.T) {
		r := &pus.EventControlRequest{Profile: profile, Enable: true, EventDefinitionIDs: []uint64{1}}
		encoded, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pus.DecodeEventControlRequest(profile, true, append(encoded, 0xEE)); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("TC[3,1] structure definition", func(t *testing.T) {
		s := &pus.HousekeepingStructure{Profile: profile, StructureID: 1, ParameterIDs: []uint64{2}}
		encoded, err := s.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pus.DecodeHousekeepingStructure(profile, append(encoded, 0xEE)); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("TC[3,5] control request", func(t *testing.T) {
		r := &pus.HousekeepingControlRequest{Profile: profile, Subtype: pus.SubtypeEnableHKGeneration, StructureIDs: []uint64{1}}
		encoded, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pus.DecodeHousekeepingControlRequest(profile, r.Subtype, append(encoded, 0xEE)); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("TC[17,3] connection test", func(t *testing.T) {
		if _, err := pus.DecodeOnBoardConnectionRequest(profile, []byte{0x00, 0x64, 0xEE}); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})

	t.Run("TM[17,4] connection report", func(t *testing.T) {
		if _, err := pus.DecodeOnBoardConnectionReport(profile, []byte{0x00, 0x64, 0xEE}); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("error = %v, want ErrTrailingBytes", err)
		}
	})
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

	profile := pus.DefaultProfile()
	req := pus.OnBoardConnectionRequest{Profile: profile, APID: 0x123}
	encoded, err := req.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 2 {
		t.Errorf("TC[17,3] body is %d octets, want 2 with the default profile", len(encoded))
	}
	got, err := pus.DecodeOnBoardConnectionRequest(profile, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.APID != 0x123 {
		t.Errorf("APID = %#x, want 0x123", got.APID)
	}
}

func TestST17APIDWidthIsTailorable(t *testing.T) {
	// The ST[17] APID field is enumerated without a stated width, so the
	// profile declares it. Zero keeps the common 2-octet width.
	narrow := pus.DefaultProfile()
	narrow.APIDBytes = 1

	req := pus.OnBoardConnectionRequest{Profile: narrow, APID: 0x64}
	encoded, err := req.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 1 {
		t.Fatalf("TC[17,3] body is %d octets, want 1", len(encoded))
	}
	got, err := pus.DecodeOnBoardConnectionRequest(narrow, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.APID != 0x64 {
		t.Errorf("APID = %#x, want 0x64", got.APID)
	}

	// A value too wide for the declared field is refused on encode.
	wide := pus.OnBoardConnectionRequest{Profile: narrow, APID: 0x123}
	if _, err := wide.Encode(); !errors.Is(err, pus.ErrValueTooLarge) {
		t.Errorf("error = %v, want ErrValueTooLarge", err)
	}

	// An unset width means two octets, for both message directions.
	var zero pus.MissionProfile
	if zero.APIDSize() != 2 {
		t.Errorf("APIDSize() = %d with APIDBytes unset, want 2", zero.APIDSize())
	}
	rep := pus.OnBoardConnectionReport{Profile: pus.DefaultProfile(), APID: 0x123}
	encodedRep, err := rep.Encode()
	if err != nil {
		t.Fatal(err)
	}
	gotRep, err := pus.DecodeOnBoardConnectionReport(pus.DefaultProfile(), encodedRep)
	if err != nil {
		t.Fatal(err)
	}
	if gotRep.APID != 0x123 {
		t.Errorf("report APID = %#x, want 0x123", gotRep.APID)
	}
}

func TestWordSizeValidation(t *testing.T) {
	// A declared word size makes the spare-byte arithmetic checkable: both
	// secondary headers must be a whole number of words. Zero disables it.
	p := pus.DefaultProfile()
	if err := p.Validate(); err != nil {
		t.Fatalf("word size zero must not be checked: %v", err)
	}

	// Default TC header is 5 octets, TM header is 13: neither is even.
	p.WordSizeBytes = 2
	if err := p.Validate(); !errors.Is(err, pus.ErrHeaderNotWordAligned) {
		t.Errorf("error = %v, want ErrHeaderNotWordAligned", err)
	}

	// One spare octet on each side aligns both headers to 16-bit words.
	p.TCSpareBytes = 1
	p.TMSpareBytes = 1
	if err := p.Validate(); err != nil {
		t.Errorf("aligned profile rejected: %v", err)
	}

	p.WordSizeBytes = -1
	if err := p.Validate(); !errors.Is(err, pus.ErrInvalidProfile) {
		t.Errorf("error = %v, want ErrInvalidProfile", err)
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
		{Service: 1, Subtype: 1}, {Service: 1, Subtype: 8}, {Service: 1, Subtype: 10},
		{Service: 3, Subtype: 25},
		{Service: 5, Subtype: 1}, {Service: 5, Subtype: 4}, {Service: 5, Subtype: 8},
		{Service: 17, Subtype: 2}, {Service: 17, Subtype: 4},
	}
	for _, key := range wantReports {
		if _, err := registry.DecodeReport(key, make([]byte, 16)); errors.Is(err, pus.ErrUnknownMessageType) {
			t.Errorf("report %s is not registered", key)
		}
	}

	wantRequests := []pus.MessageKey{
		{Service: 3, Subtype: 1}, {Service: 3, Subtype: 5}, {Service: 3, Subtype: 6},
		{Service: 5, Subtype: 5}, {Service: 5, Subtype: 6}, {Service: 5, Subtype: 7},
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
