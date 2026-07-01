package pus_test

import (
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
)

// fuzzProfiles is a small matrix of tailorings, exercised on every input.
func fuzzProfiles() []pus.MissionProfile {
	narrow := pus.MissionProfile{
		TimeFormat:                   pus.TimeNone,
		StepIDBytes:                  1,
		FailureCodeBytes:             1,
		EventDefinitionIDBytes:       1,
		HousekeepingStructureIDBytes: 1,
		ParameterIDBytes:             1,
		CollectionIntervalBytes:      1,
		CountBytes:                   1,
	}
	return []pus.MissionProfile{narrow, pus.DefaultProfile()}
}

func FuzzDecodeTCHeader(f *testing.F) {
	p := pus.DefaultProfile()
	if encoded, err := p.NewTCHeader(1, 1, 0x1234, pus.AckAcceptance).Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 5))
	f.Add(make([]byte, 64))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and a header that decodes must re-encode.
		for _, profile := range fuzzProfiles() {
			h := &pus.TCHeader{Profile: profile}
			if err := h.Decode(data); err != nil {
				continue
			}
			encoded, err := h.Encode()
			if err != nil {
				t.Fatalf("a decoded header failed to re-encode: %v", err)
			}
			if len(encoded) != h.Size() {
				t.Fatalf("re-encoded %d octets, Size() says %d", len(encoded), h.Size())
			}
		}
	})
}

func FuzzDecodeTMHeader(f *testing.F) {
	p := pus.DefaultProfile()
	if encoded, err := p.NewTMHeader(3, 25, 1, time.Unix(1700000000, 0)).Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 13))
	f.Add(make([]byte, 64))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Re-encoding is not asserted here, because a
		// decoded CUC time can legitimately fail to re-encode when it falls
		// before the profile's epoch.
		for _, profile := range fuzzProfiles() {
			h := &pus.TMHeader{Profile: profile}
			if err := h.Decode(data); err != nil {
				continue
			}
			_, _ = h.Encode()
		}
	})
}

func FuzzRegistryDecode(f *testing.F) {
	f.Add(uint8(1), uint8(1), []byte{0, 0, 0, 0})
	f.Add(uint8(3), uint8(1), make([]byte, 12))
	f.Add(uint8(5), uint8(5), []byte{2, 0, 1, 0, 2})
	f.Add(uint8(17), uint8(1), []byte{})
	f.Add(uint8(200), uint8(0), []byte{1, 2, 3})

	f.Fuzz(func(t *testing.T, service, subtype uint8, data []byte) {
		// Property: arbitrary bytes through the registry never panic.
		for _, profile := range fuzzProfiles() {
			registry, err := pus.NewDefaultRegistry(profile)
			if err != nil {
				t.Fatal(err)
			}
			key := pus.MessageKey{Service: service, Subtype: subtype}

			if req, err := registry.DecodeRequest(key, data); err == nil && req != nil {
				_, _ = req.Encode()
			}
			if rep, err := registry.DecodeReport(key, data); err == nil && rep != nil {
				_, _ = rep.Encode()
			}
		}
	})
}
