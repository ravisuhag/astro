package pus_test

import (
	"bytes"
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
		FunctionIDBytes:              1,
		FunctionArgumentCountBytes:   1,
		FunctionArgumentIDBytes:      1,
		RelativeTimeCoarseBytes:      1,
		RelativeTimeFineBytes:        0,
		SubScheduleIDBytes:           1,
		GroupIDBytes:                 1,
		ScheduleCountBytes:           1,
		ScheduleStatusBytes:          1,
		TimeWindowTypeBytes:          1,
		ScheduleSourceIDBytes:        1,
		ScheduleAPIDBytes:            1,
		ScheduleSeqCountBytes:        1,
		SupportsSubSchedules:         true,
		SupportsGroups:               true,
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
	f.Add(uint8(8), uint8(1), []byte("DEPLOY\x00\x00"))
	f.Add(uint8(11), uint8(1), []byte{})
	f.Add(uint8(11), uint8(4), []byte{1, 1, 2, 0, 0, 0, 0, 0, 0, 8, 1, 0, 0xC0, 0, 0, 1, 0xFF})
	f.Add(uint8(11), uint8(6), []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add(uint8(11), uint8(15), []byte{0xFF, 0xFF, 0xF1, 0xF0})
	f.Add(uint8(11), uint8(22), []byte{2, 1, 1, 2, 0})
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

func FuzzDecodePerformFunction(f *testing.F) {
	f.Add([]byte("DEPLOY\x00\x00"))
	f.Add([]byte("SETMODE\x00\x02\x01\xBE\xEF\x07\x2A"))
	f.Add([]byte{})
	f.Add(make([]byte, 9))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: a TC[8,1] that decodes must re-encode to the same octets.
		// The only lossy-looking step is the fixed character-string, whose NUL
		// padding is dropped on decode and restored on encode; an interior NUL
		// has to survive that, and this is what proves it does.
		for _, profile := range fuzzProfiles() {
			request, err := pus.DecodePerformFunctionRequest(profile, data)
			if err != nil {
				continue
			}
			encoded, err := request.Encode()
			if err != nil {
				t.Fatalf("a decoded TC[8,1] failed to re-encode: %v", err)
			}
			if !bytes.Equal(encoded, data) {
				t.Fatalf("re-encoded % x, want % x", encoded, data)
			}
		}
	})
}

func FuzzSplitFunctionArguments(f *testing.F) {
	f.Add(uint64(2), []byte{0x01, 0xBE, 0xEF, 0x07, 0x2A}, uint8(1))
	f.Add(uint64(1)<<60, []byte{1, 2}, uint8(1))
	f.Add(uint64(0), []byte{}, uint8(0))

	f.Fuzz(func(t *testing.T, count uint64, raw []byte, valueWidth uint8) {
		// Property: an untrusted count and an arbitrary block never panic and
		// never allocate on the count's word.
		args := &pus.FunctionArguments{Count: count, Raw: raw}
		width := func(uint64) (int, error) { return int(valueWidth), nil }

		for _, profile := range fuzzProfiles() {
			got, err := args.SplitArguments(profile, width)
			if err != nil {
				continue
			}
			if uint64(len(got)) != count {
				t.Fatalf("split %d arguments but the count says %d", len(got), count)
			}
		}
	})
}

func FuzzDecodeScheduleMessages(f *testing.F) {
	f.Add(uint8(4), []byte{1, 1, 2, 0, 0, 0, 0, 0, 0, 8, 1, 0, 0xC0, 0, 0, 1, 0xFF})
	f.Add(uint8(6), []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add(uint8(10), []byte{1, 1, 2, 0, 0, 0, 0, 0, 0, 8, 1, 0, 0xC0, 0, 0, 1, 0xFF})
	f.Add(uint8(13), []byte{1, 1, 2, 0, 0, 0, 0, 0, 0, 1, 2, 3})
	f.Add(uint8(15), []byte{0xFF, 0xFF, 0xF1, 0xF0})
	f.Add(uint8(19), []byte{2, 1, 1, 2, 0})
	f.Add(uint8(0), []byte{})
	f.Add(uint8(255), make([]byte, 40))

	f.Fuzz(func(t *testing.T, subtype uint8, data []byte) {
		// Property: an ST[11] message that decodes must re-encode to the same
		// octets. Every one of these bodies is either fixed-width or
		// self-delimiting, so there is nowhere for a decoder to guess and
		// nothing it may drop.
		//
		// The exception is the absolute time field, and it is not this
		// package's. pkg/tcf truncates in both directions by design, and its
		// own TestCUCFineTimeTruncates says why: rounding to nearest can carry
		// the fine field past its width. Truncating twice loses a tick
		// whenever a tick is not a whole number of nanoseconds, which is
		// whenever 2^(8*fine) does not divide 10^9 — and since 10^9 is 2^9
		// times 5^9, that means whenever fine is 2 or 3. So a CUC field of two
		// or three fine octets can come back one tick lower than it went out,
		// and byte equality is asserted only where it cannot.
		for _, profile := range fuzzProfiles() {
			registry, err := pus.NewDefaultRegistry(profile)
			if err != nil {
				t.Fatal(err)
			}
			key := pus.MessageKey{Service: pus.ServiceTimeBasedScheduling, Subtype: subtype}
			exact := timeFieldRoundTripsExactly(profile)

			check := func(kind string, encoded []byte, err error) {
				if err != nil {
					t.Fatalf("a decoded %s[11,%d] failed to re-encode: %v", kind, subtype, err)
				}
				if len(encoded) != len(data) {
					t.Fatalf("%s[11,%d] re-encoded %d octets, want %d",
						kind, subtype, len(encoded), len(data))
				}
				if exact && !bytes.Equal(encoded, data) {
					t.Fatalf("%s[11,%d] re-encoded % x, want % x", kind, subtype, encoded, data)
				}
			}

			if request, err := registry.DecodeRequest(key, data); err == nil {
				encoded, err := request.Encode()
				check("TC", encoded, err)
			}
			if report, err := registry.DecodeReport(key, data); err == nil {
				encoded, err := report.Encode()
				check("TM", encoded, err)
			}
		}
	})
}

// timeFieldRoundTripsExactly reports whether this profile's absolute time
// field survives a decode and re-encode byte for byte.
//
// It does when there is no CUC fine field, or when one fine tick is a whole
// number of nanoseconds. See the comment in FuzzDecodeScheduleMessages.
func timeFieldRoundTripsExactly(p pus.MissionProfile) bool {
	switch p.TimeFormat {
	case pus.TimeNone, pus.TimeRaw:
		return true
	default:
		return p.CUCFineBytes <= 1
	}
}

func FuzzRelativeTime(f *testing.F) {
	f.Add([]byte{0xFF, 0xFF, 0xF1, 0xF0}, uint8(4), uint8(0))
	f.Add([]byte{0, 0, 0, 0, 0, 0, 1}, uint8(4), uint8(3))
	f.Add([]byte{0x80}, uint8(1), uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, coarse, fine uint8) {
		// Property: a relative time field that decodes re-encodes to the same
		// octets, for every width Table 7-11 allows. The sign extension is
		// where this could go wrong, so the negative half of the range matters
		// as much as the positive.
		profile := pus.DefaultProfile()
		profile.RelativeTimeCoarseBytes = int(coarse)
		profile.RelativeTimeFineBytes = int(fine)
		if profile.Validate() != nil {
			return
		}
		width := profile.RelativeTimeSize()
		if len(data) < width {
			return
		}

		request, err := pus.DecodeTimeShiftAllRequest(profile, data[:width])
		if err != nil {
			return
		}
		encoded, err := request.Encode()
		if err != nil {
			t.Fatalf("a decoded relative time failed to re-encode: %v", err)
		}
		if !bytes.Equal(encoded, data[:width]) {
			t.Fatalf("re-encoded % x, want % x", encoded, data[:width])
		}
	})
}
