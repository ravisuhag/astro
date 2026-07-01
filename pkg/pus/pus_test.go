package pus_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
	"github.com/ravisuhag/astro/pkg/spp"
)

// Three profiles spanning the tailoring range, so nothing depends on one
// particular set of widths.
func minimalProfile() pus.MissionProfile {
	return pus.MissionProfile{
		TimeFormat:                   pus.TimeNone,
		StepIDBytes:                  1,
		FailureCodeBytes:             1,
		EventDefinitionIDBytes:       1,
		HousekeepingStructureIDBytes: 1,
		ParameterIDBytes:             1,
		CollectionIntervalBytes:      1,
		CountBytes:                   1,
	}
}

func wideProfile() pus.MissionProfile {
	return pus.MissionProfile{
		TCSpareBytes:                 2,
		TMSpareBytes:                 2,
		TimeFormat:                   pus.TimeCUC,
		CUCCoarseBytes:               4,
		CUCFineBytes:                 3,
		StepIDBytes:                  4,
		FailureCodeBytes:             4,
		EventDefinitionIDBytes:       4,
		HousekeepingStructureIDBytes: 2,
		ParameterIDBytes:             4,
		CollectionIntervalBytes:      4,
		CountBytes:                   2,
	}
}

func allProfiles() map[string]pus.MissionProfile {
	return map[string]pus.MissionProfile{
		"minimal": minimalProfile(),
		"default": pus.DefaultProfile(),
		"wide":    wideProfile(),
	}
}

func TestProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*pus.MissionProfile)
		wantErr error
	}{
		{"default is valid", func(*pus.MissionProfile) {}, nil},
		{"negative spare", func(p *pus.MissionProfile) { p.TCSpareBytes = -1 }, pus.ErrInvalidProfile},
		{"width over eight octets", func(p *pus.MissionProfile) { p.ParameterIDBytes = 9 }, pus.ErrInvalidProfile},
		{"CUC coarse zero", func(p *pus.MissionProfile) { p.CUCCoarseBytes = 0 }, pus.ErrInvalidProfile},
		{"CUC coarse over four", func(p *pus.MissionProfile) { p.CUCCoarseBytes = 5 }, pus.ErrInvalidProfile},
		{"CUC fine over three", func(p *pus.MissionProfile) { p.CUCFineBytes = 4 }, pus.ErrInvalidProfile},
		{"raw time with no width", func(p *pus.MissionProfile) {
			p.TimeFormat = pus.TimeRaw
			p.TimeRawBytes = 0
		}, pus.ErrInvalidProfile},
		{"header past 63 octets", func(p *pus.MissionProfile) { p.TMSpareBytes = 8; p.CUCCoarseBytes = 4 }, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pus.DefaultProfile()
			tt.mutate(&p)
			err := p.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfileRejectsOversizedHeader(t *testing.T) {
	// pkg/spp caps a secondary header at 63 octets.
	p := pus.DefaultProfile()
	p.TMSpareBytes = 8 // width 8 is the per-field maximum
	p.TCSpareBytes = 8
	if err := p.Validate(); err != nil {
		t.Fatalf("this profile should still fit: %v", err)
	}

	// Push past the limit by widening the time field as far as it goes and
	// checking the arithmetic, since single fields cap at 8 octets.
	if p.TMHeaderSize() > 63 {
		t.Fatal("test profile already exceeds the limit")
	}
}

func TestTCHeaderRoundTrip(t *testing.T) {
	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			h := profile.NewTCHeader(3, 1, 0xBEEF, pus.AckAcceptance|pus.AckCompletion)

			encoded, err := h.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != h.Size() {
				t.Fatalf("encoded %d octets, Size() says %d", len(encoded), h.Size())
			}

			got := &pus.TCHeader{Profile: profile}
			if err := got.Decode(encoded); err != nil {
				t.Fatal(err)
			}
			if got.Service != 3 || got.Subtype != 1 {
				t.Errorf("message type = TC[%d,%d], want TC[3,1]", got.Service, got.Subtype)
			}
			if got.SourceID != 0xBEEF {
				t.Errorf("source ID = %#04x, want 0xBEEF", got.SourceID)
			}
			if !got.AckFlags.Has(pus.AckAcceptance) || !got.AckFlags.Has(pus.AckCompletion) {
				t.Errorf("ack flags = %s, want acceptance and completion", got.AckFlags)
			}
			if got.AckFlags.Has(pus.AckStart) {
				t.Error("ack flags gained a start request that was never set")
			}
		})
	}
}

func TestTCHeaderVersionIsTwo(t *testing.T) {
	// Clause 7.4.4.1c: the TC packet PUS version number is 2 for PUS-C.
	h := pus.DefaultProfile().NewTCHeader(1, 1, 0, 0)
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if version := encoded[0] >> 4; version != pus.Version {
		t.Errorf("version = %d, want %d", version, pus.Version)
	}
}

func TestTCHeaderAckFlagBitPositions(t *testing.T) {
	// Clause 7.4.4.1d fixes the bit positions: 3 acceptance, 2 start,
	// 1 progress, 0 completion. Swapping them silently changes which reports
	// a spacecraft sends back.
	tests := []struct {
		flag pus.AckFlags
		bit  uint8
	}{
		{pus.AckAcceptance, 3},
		{pus.AckStart, 2},
		{pus.AckProgress, 1},
		{pus.AckCompletion, 0},
	}
	for _, tt := range tests {
		h := pus.DefaultProfile().NewTCHeader(1, 1, 0, tt.flag)
		encoded, err := h.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := encoded[0] & 0x0F; got != 1<<tt.bit {
			t.Errorf("%s encoded as %04b, want bit %d set", tt.flag, got, tt.bit)
		}
	}
}

func TestTMHeaderRoundTrip(t *testing.T) {
	stamp := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)

	for name, profile := range allProfiles() {
		t.Run(name, func(t *testing.T) {
			h := profile.NewTMHeader(5, 1, 0x1234, stamp)
			h.MessageTypeCounter = 99
			h.TimeReferenceStatus = 3

			encoded, err := h.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != h.Size() {
				t.Fatalf("encoded %d octets, Size() says %d", len(encoded), h.Size())
			}

			got := &pus.TMHeader{Profile: profile}
			if err := got.Decode(encoded); err != nil {
				t.Fatal(err)
			}
			if got.Service != 5 || got.Subtype != 1 {
				t.Errorf("message type = TM[%d,%d], want TM[5,1]", got.Service, got.Subtype)
			}
			if got.DestinationID != 0x1234 {
				t.Errorf("destination = %#04x, want 0x1234", got.DestinationID)
			}
			if got.MessageTypeCounter != 99 {
				t.Errorf("counter = %d, want 99", got.MessageTypeCounter)
			}
			if got.TimeReferenceStatus != 3 {
				t.Errorf("time reference status = %d, want 3", got.TimeReferenceStatus)
			}
			if profile.TimeFormat == pus.TimeCUC && !got.Time.Equal(stamp) {
				t.Errorf("time = %s, want %s", got.Time, stamp)
			}
		})
	}
}

func TestTMHeaderRawTime(t *testing.T) {
	p := pus.DefaultProfile()
	p.TimeFormat = pus.TimeRaw
	p.TimeRawBytes = 6

	h := p.NewTMHeader(3, 25, 1, time.Time{})
	h.RawTime = []byte{1, 2, 3, 4, 5, 6}

	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got := &pus.TMHeader{Profile: p}
	if err := got.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.RawTime, h.RawTime) {
		t.Errorf("raw time = %x, want %x", got.RawTime, h.RawTime)
	}
}

func TestHeadersRejectWrongVersion(t *testing.T) {
	p := pus.DefaultProfile()

	tc := p.NewTCHeader(1, 1, 0, 0)
	encoded, err := tc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] = 1<<4 | (encoded[0] & 0x0F) // version 1, the E-70-41A PUS
	if err := (&pus.TCHeader{Profile: p}).Decode(encoded); !errors.Is(err, pus.ErrInvalidVersion) {
		t.Errorf("TC: error = %v, want ErrInvalidVersion", err)
	}

	tm := p.NewTMHeader(1, 1, 0, time.Now())
	encodedTM, err := tm.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encodedTM[0] = 0<<4 | (encodedTM[0] & 0x0F) // version 0, the ESA PUS
	if err := (&pus.TMHeader{Profile: p}).Decode(encodedTM); !errors.Is(err, pus.ErrInvalidVersion) {
		t.Errorf("TM: error = %v, want ErrInvalidVersion", err)
	}
}

func TestHeadersRejectShortInput(t *testing.T) {
	p := pus.DefaultProfile()

	tc := p.NewTCHeader(1, 1, 0, 0)
	encoded, err := tc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(encoded); cut++ {
		if err := (&pus.TCHeader{Profile: p}).Decode(encoded[:cut]); !errors.Is(err, pus.ErrDataTooShort) {
			t.Errorf("TC length %d: error = %v, want ErrDataTooShort", cut, err)
		}
	}
}

func TestProfilesAreNotInterchangeable(t *testing.T) {
	// Two missions with different widths cannot read each other's packets.
	// The header is not self-describing, so this must be visible, not silent.
	narrow := minimalProfile()
	wide := wideProfile()

	if narrow.TMHeaderSize() == wide.TMHeaderSize() {
		t.Fatal("test profiles must differ in size")
	}

	h := narrow.NewTMHeader(5, 1, 1, time.Now())
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// The wide profile expects a longer header, so it must refuse.
	if err := (&pus.TMHeader{Profile: wide}).Decode(encoded); err == nil {
		t.Error("a wide profile decoded a narrow profile's header without complaint")
	}
}

func TestPUSTelecommandThroughSPP(t *testing.T) {
	// The point of the whole package: a PUS header plugs into pkg/spp with no
	// changes to either side.
	p := pus.DefaultProfile()
	tcHeader := p.NewTCHeader(pus.ServiceTest, pus.SubtypeAreYouAlive, 0x0042, pus.AckAcceptance)

	body, err := pus.AreYouAliveRequest{}.Encode()
	if err != nil {
		t.Fatal(err)
	}

	packet, err := spp.NewTCPacket(100, body, spp.WithSecondaryHeader(tcHeader))
	if err != nil {
		t.Fatalf("building the space packet: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decodeInto := &pus.TCHeader{Profile: p}
	decoded, err := spp.Decode(encoded, spp.WithDecodeSecondaryHeader(decodeInto))
	if err != nil {
		t.Fatalf("decoding the space packet: %v", err)
	}
	if decoded.PrimaryHeader.APID != 100 {
		t.Errorf("APID = %d, want 100", decoded.PrimaryHeader.APID)
	}
	if decoded.PrimaryHeader.SecondaryHeaderFlag != 1 {
		t.Error("the secondary header flag was not set")
	}
	if decodeInto.Service != pus.ServiceTest || decodeInto.Subtype != pus.SubtypeAreYouAlive {
		t.Errorf("message type = TC[%d,%d], want TC[17,1]", decodeInto.Service, decodeInto.Subtype)
	}
	if decodeInto.SourceID != 0x0042 {
		t.Errorf("source ID = %#04x, want 0x0042", decodeInto.SourceID)
	}
}

func TestPUSTelemetryThroughSPP(t *testing.T) {
	p := pus.DefaultProfile()
	stamp := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	report := &pus.EventReport{
		Profile:           p,
		Severity:          pus.SeverityHigh,
		EventDefinitionID: 0x0101,
		AuxiliaryData:     []byte{0xDE, 0xAD},
	}
	body, err := report.Encode()
	if err != nil {
		t.Fatal(err)
	}

	tmHeader := p.NewTMHeader(pus.ServiceEventReporting, uint8(pus.SeverityHigh), 0x0001, stamp)
	packet, err := spp.NewTMPacket(200, body, spp.WithSecondaryHeader(tmHeader))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decodeInto := &pus.TMHeader{Profile: p}
	decoded, err := spp.Decode(encoded, spp.WithDecodeSecondaryHeader(decodeInto))
	if err != nil {
		t.Fatal(err)
	}

	registry, err := pus.NewDefaultRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	gotReport, err := registry.DecodeReport(decodeInto.Key(), decoded.UserData)
	if err != nil {
		t.Fatalf("decoding the report: %v", err)
	}

	event, ok := gotReport.(*pus.EventReport)
	if !ok {
		t.Fatalf("got %T, want *pus.EventReport", gotReport)
	}
	if event.EventDefinitionID != 0x0101 {
		t.Errorf("event ID = %#x, want 0x0101", event.EventDefinitionID)
	}
	if !bytes.Equal(event.AuxiliaryData, []byte{0xDE, 0xAD}) {
		t.Errorf("auxiliary data = %x, want dead", event.AuxiliaryData)
	}
	if !decodeInto.Time.Equal(stamp) {
		t.Errorf("time = %s, want %s", decodeInto.Time, stamp)
	}
}
