package pus_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/pus"
)

// tcPacket builds a minimal valid CCSDS telecommand packet with a data field
// of the given length, so the activity lists have something real to carry.
func tcPacket(apid uint16, dataLen int) []byte {
	if dataLen < 1 {
		dataLen = 1
	}
	out := make([]byte, 6+dataLen)
	// Version 0, type 1 (TC), no secondary header, the APID.
	first := uint16(1)<<12 | (apid & 0x07FF)
	out[0] = byte(first >> 8)
	out[1] = byte(first)
	// Sequence flags 3 (unsegmented), count 0.
	out[2] = 0xC0
	out[3] = 0x00
	// Packet data length: the data field length minus one.
	out[4] = byte((dataLen - 1) >> 8)
	out[5] = byte(dataLen - 1)
	for i := range out[6:] {
		out[6+i] = byte(i + 1)
	}
	return out
}

// schedProfile is a profile with both scheduling capabilities on and widths
// that make the field boundaries easy to read in a hex dump.
func schedProfile() pus.MissionProfile {
	p := pus.DefaultProfile()
	p.SubScheduleIDBytes = 1
	p.GroupIDBytes = 1
	p.ScheduleCountBytes = 1
	p.ScheduleStatusBytes = 1
	p.TimeWindowTypeBytes = 1
	p.ScheduleSourceIDBytes = 2
	p.ScheduleAPIDBytes = 2
	p.ScheduleSeqCountBytes = 2
	p.SupportsSubSchedules = true
	p.SupportsGroups = true
	return p
}

// TestST11RegistersEveryMessageType checks all twenty-seven are wired up: the
// standard defines exactly that many, so a missing one is a hole.
func TestST11RegistersEveryMessageType(t *testing.T) {
	r, err := pus.NewDefaultRegistry(schedProfile())
	if err != nil {
		t.Fatal(err)
	}

	// The requests: every subtype except the four report subtypes.
	wantRequests := []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 14, 15, 16, 17, 18, 20, 21, 22, 23, 24, 25, 26}
	wantReports := []uint8{10, 13, 19, 27}

	have := map[uint8]bool{}
	for _, key := range r.KnownRequests() {
		if key.Service == pus.ServiceTimeBasedScheduling {
			have[key.Subtype] = true
		}
	}
	for _, subtype := range wantRequests {
		if !have[subtype] {
			t.Errorf("TC[11,%d] is not registered", subtype)
		}
	}
	if len(have) != len(wantRequests) {
		t.Errorf("registered %d ST[11] requests, want %d", len(have), len(wantRequests))
	}

	have = map[uint8]bool{}
	for _, key := range r.KnownReports() {
		if key.Service == pus.ServiceTimeBasedScheduling {
			have[key.Subtype] = true
		}
	}
	for _, subtype := range wantReports {
		if !have[subtype] {
			t.Errorf("TM[11,%d] is not registered", subtype)
		}
	}
	if len(have) != len(wantReports) {
		t.Errorf("registered %d ST[11] reports, want %d", len(have), len(wantReports))
	}
}

// TestScheduleControlRequestsCarryNoBody checks the seven requests whose
// application data field "shall be omitted".
func TestScheduleControlRequestsCarryNoBody(t *testing.T) {
	r, err := pus.NewDefaultRegistry(schedProfile())
	if err != nil {
		t.Fatal(err)
	}

	for _, subtype := range []uint8{1, 2, 3, 16, 17, 18, 26} {
		request := pus.ScheduleControlRequest{Subtype: subtype}
		encoded, err := request.Encode()
		if err != nil {
			t.Fatalf("TC[11,%d]: %v", subtype, err)
		}
		if len(encoded) != 0 {
			t.Errorf("TC[11,%d] encoded %d octets, want none", subtype, len(encoded))
		}

		key := pus.MessageKey{Service: pus.ServiceTimeBasedScheduling, Subtype: subtype}
		if _, err := r.DecodeRequest(key, nil); err != nil {
			t.Errorf("TC[11,%d] empty body: %v", subtype, err)
		}
		// A body where none belongs is a rejection, not something to ignore.
		if _, err := r.DecodeRequest(key, []byte{0x00}); !errors.Is(err, pus.ErrTrailingBytes) {
			t.Errorf("TC[11,%d] with a body: err = %v, want ErrTrailingBytes", subtype, err)
		}
	}

	// A subtype that is not one of the seven is refused rather than encoded
	// into a message no receiver expects.
	if _, err := (pus.ScheduleControlRequest{Subtype: 4}).Encode(); !errors.Is(err, pus.ErrWrongMessageType) {
		t.Errorf("err = %v, want ErrWrongMessageType", err)
	}
}

// TestST11RequestIDIsNotST01RequestID is the trap this test exists to catch.
//
// Both figures call their field "request ID" and they are different fields.
// Figure 8-1's is a bit-packed 32-bit copy of the CCSDS primary header;
// figure 8-92's carries a source ID as well and uses whole octets at
// mission-declared widths. Encoding one with the other's codec is silent.
func TestST11RequestIDIsNotST01RequestID(t *testing.T) {
	p := schedProfile()

	// The ST[01] form: four octets, fields packed into bits.
	st01 := pus.RequestID{APID: 0x2A, SequenceCount: 0x1234, SequenceFlags: 3}
	if got := len(st01.Encode()); got != pus.RequestIDSize {
		t.Fatalf("ST[01] request ID is %d octets, want %d", got, pus.RequestIDSize)
	}

	// The ST[11] form: six octets at this profile's widths, and a source ID
	// that the ST[01] form has nowhere to put.
	if p.ScheduleRequestIDSize() != 6 {
		t.Fatalf("ST[11] request ID is %d octets, want 6 at this profile", p.ScheduleRequestIDSize())
	}
	if p.ScheduleRequestIDSize() == pus.RequestIDSize {
		t.Error("the two request ID widths coincide, so this test proves nothing")
	}

	list := pus.ScheduleRequestIDListRequest{
		Profile: p,
		Subtype: pus.SubtypeDeleteByRequestID,
		RequestIDs: []pus.ScheduleRequestID{
			{SourceID: 0x0102, APID: 0x002A, SequenceCount: 0x1234},
		},
	}
	encoded, err := list.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// N = 1, then source ID, APID and sequence count, each big-endian.
	want := []byte{0x01, 0x01, 0x02, 0x00, 0x2A, 0x12, 0x34}
	if !bytes.Equal(encoded, want) {
		t.Errorf("encoded % x, want % x", encoded, want)
	}
}

// TestTimeWindowToTagGoesInTheSecondSlot pins the reading clause 6.11.10.3c
// gives: item (b) sends the from tag, item (c) sends the to tag. So a "to time
// tag" window carries its tag as the *to* tag, and the from slot is absent
// entirely. Putting the single tag in the first slot would encode a
// "from time tag" window's bytes under a "to time tag" type.
func TestTimeWindowToTagGoesInTheSecondSlot(t *testing.T) {
	p := schedProfile()
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	windows := []struct {
		name string
		in   pus.TimeWindow
		tags int
	}{
		{"select all", pus.TimeWindow{Type: pus.WindowSelectAll}, 0},
		{"from", pus.TimeWindow{Type: pus.WindowFrom, From: from}, 1},
		{"to", pus.TimeWindow{Type: pus.WindowTo, To: to}, 1},
		{"from-to", pus.TimeWindow{Type: pus.WindowFromTo, From: from, To: to}, 2},
	}

	for _, w := range windows {
		t.Run(w.name, func(t *testing.T) {
			// No sub-schedules or groups, so the body is the window alone.
			noLists := p
			noLists.SupportsSubSchedules = false
			noLists.SupportsGroups = false

			request := pus.ScheduleFilterRequest{
				Profile: noLists,
				Subtype: pus.SubtypeDeleteByFilter,
				Filter:  pus.ScheduleFilter{Window: w.in},
			}
			encoded, err := request.Encode()
			if err != nil {
				t.Fatal(err)
			}
			wantLen := noLists.TimeWindowTypeSize() + w.tags*noLists.TimeSize()
			if len(encoded) != wantLen {
				t.Fatalf("encoded %d octets, want %d for %d tag(s)", len(encoded), wantLen, w.tags)
			}

			got, err := pus.DecodeScheduleFilterRequest(noLists, pus.SubtypeDeleteByFilter, encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got.Filter.Window.Type != w.in.Type {
				t.Errorf("type = %v, want %v", got.Filter.Window.Type, w.in.Type)
			}
			// The tag that travelled comes back in the slot it went out in.
			if w.in.Type.String() == pus.WindowTo.String() {
				if !got.Filter.Window.To.Equal(to) {
					t.Errorf("to = %s, want %s", got.Filter.Window.To, to)
				}
				if !got.Filter.Window.From.IsZero() {
					t.Errorf("from = %s, want the zero time: no from tag travels for %q",
						got.Filter.Window.From, w.in.Type)
				}
			}
			if w.in.Type == pus.WindowFrom {
				if !got.Filter.Window.From.Equal(from) {
					t.Errorf("from = %s, want %s", got.Filter.Window.From, from)
				}
				if !got.Filter.Window.To.IsZero() {
					t.Errorf("to = %s, want the zero time", got.Filter.Window.To)
				}
			}
		})
	}
}

// TestTimeWindowRefusesFromAfterTo checks clause 6.11.10.3d item 2, which is
// a check on the message alone and so belongs in the codec.
func TestTimeWindowRefusesFromAfterTo(t *testing.T) {
	p := schedProfile()
	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	request := pus.ScheduleFilterRequest{
		Profile: p,
		Subtype: pus.SubtypeDeleteByFilter,
		Filter: pus.ScheduleFilter{
			Window: pus.TimeWindow{Type: pus.WindowFromTo, From: from, To: to},
		},
	}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrInvalidTimeWindow) {
		t.Errorf("err = %v, want ErrInvalidTimeWindow", err)
	}
}

// TestTimeWindowRefusesAnUnknownType checks Table 8-5 is a closed set, which
// clause 6.11.10.3d item 1 makes a rejection.
func TestTimeWindowRefusesAnUnknownType(t *testing.T) {
	p := schedProfile()
	p.SupportsSubSchedules = false
	p.SupportsGroups = false

	request := pus.ScheduleFilterRequest{
		Profile: p,
		Subtype: pus.SubtypeDeleteByFilter,
		Filter:  pus.ScheduleFilter{Window: pus.TimeWindow{Type: 4}},
	}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrInvalidTimeWindow) {
		t.Errorf("encode: err = %v, want ErrInvalidTimeWindow", err)
	}

	if _, err := pus.DecodeScheduleFilterRequest(p, pus.SubtypeDeleteByFilter, []byte{0x04}); !errors.Is(err, pus.ErrInvalidTimeWindow) {
		t.Errorf("decode: err = %v, want ErrInvalidTimeWindow", err)
	}
}

// TestInsertActivitiesRoundTrip checks the layout of Figure 8-91: the
// sub-schedule ID comes once, before the count.
func TestInsertActivitiesRoundTrip(t *testing.T) {
	p := schedProfile()
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	request := pus.InsertActivitiesRequest{
		Profile:       p,
		SubScheduleID: 3,
		Activities: []pus.ScheduledActivity{
			{GroupID: 7, ReleaseTime: release, Request: tcPacket(0x2A, 4)},
			{GroupID: 8, ReleaseTime: release.Add(time.Minute), Request: tcPacket(0x2B, 1)},
		},
	}
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// The sub-schedule ID is the first octet, and the count the second: one
	// sub-schedule for the whole request, not one per activity.
	if encoded[0] != 3 {
		t.Errorf("first octet = %d, want the sub-schedule ID 3", encoded[0])
	}
	if encoded[1] != 2 {
		t.Errorf("second octet = %d, want the count 2", encoded[1])
	}

	got, err := pus.DecodeInsertActivitiesRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubScheduleID != 3 {
		t.Errorf("sub-schedule = %d, want 3", got.SubScheduleID)
	}
	if len(got.Activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(got.Activities))
	}
	if got.Activities[0].GroupID != 7 || got.Activities[1].GroupID != 8 {
		t.Errorf("group IDs = %d, %d; want 7, 8",
			got.Activities[0].GroupID, got.Activities[1].GroupID)
	}
	if !got.Activities[0].ReleaseTime.Equal(release) {
		t.Errorf("release = %s, want %s", got.Activities[0].ReleaseTime, release)
	}
	if !bytes.Equal(got.Activities[0].Request, tcPacket(0x2A, 4)) {
		t.Errorf("request 0 = % x, want % x", got.Activities[0].Request, tcPacket(0x2A, 4))
	}
	if !bytes.Equal(got.Activities[1].Request, tcPacket(0x2B, 1)) {
		t.Errorf("request 1 = % x, want % x", got.Activities[1].Request, tcPacket(0x2B, 1))
	}
}

// TestDetailReportCarriesSubScheduleIDPerActivity checks the difference
// between Figures 8-91 and 8-97: the insert request names one sub-schedule,
// the detail report names one per activity, so a report can span
// sub-schedules.
func TestDetailReportCarriesSubScheduleIDPerActivity(t *testing.T) {
	p := schedProfile()
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	report := pus.ScheduleDetailReport{
		Profile: p,
		Activities: []pus.ScheduledActivity{
			{SubScheduleID: 1, GroupID: 7, ReleaseTime: release, Request: tcPacket(0x2A, 2)},
			{SubScheduleID: 2, GroupID: 8, ReleaseTime: release, Request: tcPacket(0x2B, 2)},
		},
	}
	encoded, err := report.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// The count comes first here, so the repeated group starts at octet 1.
	if encoded[0] != 2 {
		t.Errorf("first octet = %d, want the count 2", encoded[0])
	}
	if encoded[1] != 1 {
		t.Errorf("second octet = %d, want the first activity's sub-schedule ID", encoded[1])
	}

	got, err := pus.DecodeScheduleDetailReport(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(got.Activities))
	}
	if got.Activities[0].SubScheduleID != 1 || got.Activities[1].SubScheduleID != 2 {
		t.Errorf("sub-schedules = %d, %d; want 1, 2. The report spans them",
			got.Activities[0].SubScheduleID, got.Activities[1].SubScheduleID)
	}
}

// TestSummaryReportRoundTrip checks Figure 8-100, which names each activity by
// its request ID rather than carrying the whole telecommand.
func TestSummaryReportRoundTrip(t *testing.T) {
	p := schedProfile()
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	report := pus.ScheduleSummaryReport{
		Profile: p,
		Activities: []pus.SummaryActivity{
			{
				SubScheduleID: 1, GroupID: 7, ReleaseTime: release,
				RequestID: pus.ScheduleRequestID{SourceID: 5, APID: 0x2A, SequenceCount: 100},
			},
			{
				SubScheduleID: 2, GroupID: 8, ReleaseTime: release.Add(time.Hour),
				RequestID: pus.ScheduleRequestID{SourceID: 6, APID: 0x2B, SequenceCount: 101},
			},
		},
	}
	encoded, err := report.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := pus.DecodeScheduleSummaryReport(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Activities) != 2 {
		t.Fatalf("got %d activities, want 2", len(got.Activities))
	}
	for i, want := range report.Activities {
		have := got.Activities[i]
		if have.SubScheduleID != want.SubScheduleID || have.GroupID != want.GroupID {
			t.Errorf("activity %d IDs = %d/%d, want %d/%d",
				i, have.SubScheduleID, have.GroupID, want.SubScheduleID, want.GroupID)
		}
		if !have.ReleaseTime.Equal(want.ReleaseTime) {
			t.Errorf("activity %d release = %s, want %s", i, have.ReleaseTime, want.ReleaseTime)
		}
		if have.RequestID != want.RequestID {
			t.Errorf("activity %d request ID = %+v, want %+v", i, have.RequestID, want.RequestID)
		}
	}
}

// TestIDListZeroCountMeansAll checks clauses 8.11.2.20c, 21c, 23c, 24c and
// 25c: an N of 0 is not an empty request, it is every sub-schedule or group.
func TestIDListZeroCountMeansAll(t *testing.T) {
	p := schedProfile()

	for _, subtype := range []uint8{20, 21, 23, 24, 25} {
		all := pus.ScheduleIDListRequest{Profile: p, Subtype: subtype}
		if !all.IsAll() {
			t.Errorf("TC[11,%d] with no IDs: IsAll() = false, want true", subtype)
		}
		encoded, err := all.Encode()
		if err != nil {
			t.Fatalf("TC[11,%d]: %v", subtype, err)
		}
		if len(encoded) != 1 || encoded[0] != 0 {
			t.Errorf("TC[11,%d] encoded % x, want a single zero count", subtype, encoded)
		}

		some := pus.ScheduleIDListRequest{Profile: p, Subtype: subtype, IDs: []uint64{1, 2}}
		if some.IsAll() {
			t.Errorf("TC[11,%d] with two IDs: IsAll() = true, want false", subtype)
		}
		encoded, err = some.Encode()
		if err != nil {
			t.Fatal(err)
		}
		want := []byte{0x02, 0x01, 0x02}
		if !bytes.Equal(encoded, want) {
			t.Errorf("TC[11,%d] encoded % x, want % x", subtype, encoded, want)
		}
	}
}

// TestIDListUsesTheRightWidth checks that the sub-schedule requests size their
// IDs with SubScheduleIDBytes and the group requests with GroupIDBytes. The
// two are separately declared, so a profile where they differ catches a
// codec that reaches for the wrong one.
func TestIDListUsesTheRightWidth(t *testing.T) {
	p := schedProfile()
	p.SubScheduleIDBytes = 1
	p.GroupIDBytes = 4

	subSchedules := pus.ScheduleIDListRequest{
		Profile: p, Subtype: pus.SubtypeEnableSubSchedules, IDs: []uint64{9},
	}
	encoded, err := subSchedules.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x01, 0x09}; !bytes.Equal(encoded, want) {
		t.Errorf("sub-schedule list = % x, want % x", encoded, want)
	}

	groups := pus.ScheduleIDListRequest{
		Profile: p, Subtype: pus.SubtypeEnableSchedulingGroups, IDs: []uint64{9},
	}
	encoded, err = groups.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x01, 0x00, 0x00, 0x00, 0x09}; !bytes.Equal(encoded, want) {
		t.Errorf("group list = % x, want % x", encoded, want)
	}
}

// TestStatusListsRoundTrip covers TC[11,22], TM[11,19] and TM[11,27], the
// three ID-and-status lists.
func TestStatusListsRoundTrip(t *testing.T) {
	p := schedProfile()
	entries := []pus.ScheduleStatusEntry{
		{ID: 1, Status: pus.ScheduleEnabled},
		{ID: 2, Status: pus.ScheduleDisabled},
	}

	create := pus.CreateSchedulingGroupsRequest{Profile: p, Groups: entries}
	encoded, err := create.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x02, 0x01, 0x01, 0x02, 0x00}; !bytes.Equal(encoded, want) {
		t.Errorf("TC[11,22] = % x, want % x", encoded, want)
	}
	gotCreate, err := pus.DecodeCreateSchedulingGroupsRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotCreate.Groups) != 2 || gotCreate.Groups[0].Status != pus.ScheduleEnabled {
		t.Errorf("TC[11,22] decoded %+v", gotCreate.Groups)
	}

	subs := pus.SubScheduleStatusReport{Profile: p, SubSchedules: entries}
	encoded, err = subs.Encode()
	if err != nil {
		t.Fatal(err)
	}
	gotSubs, err := pus.DecodeSubScheduleStatusReport(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotSubs.SubSchedules) != 2 {
		t.Errorf("TM[11,19] decoded %d entries, want 2", len(gotSubs.SubSchedules))
	}

	groups := pus.SchedulingGroupStatusReport{Profile: p, Groups: entries}
	encoded, err = groups.Encode()
	if err != nil {
		t.Fatal(err)
	}
	gotGroups, err := pus.DecodeSchedulingGroupStatusReport(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotGroups.Groups) != 2 {
		t.Errorf("TM[11,27] decoded %d entries, want 2", len(gotGroups.Groups))
	}
}

// TestRelativeTimeNegativeOffset checks clause 7.3.11b's note: a negative
// offset is the two's complement of the positive one, over the whole
// coarse-and-fine field.
func TestRelativeTimeNegativeOffset(t *testing.T) {
	p := schedProfile()
	p.RelativeTimeCoarseBytes = 4
	p.RelativeTimeFineBytes = 0

	forward, err := pus.NewRelativeTime(p, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := pus.NewRelativeTime(p, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if forward.Ticks != 3600 {
		t.Errorf("forward ticks = %d, want 3600", forward.Ticks)
	}
	if backward.Ticks != -3600 {
		t.Errorf("backward ticks = %d, want -3600", backward.Ticks)
	}

	shiftBack := pus.TimeShiftAllRequest{Profile: p, Offset: backward}
	encoded, err := shiftBack.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// -3600 as a 32-bit two's complement is FFFFF1F0.
	if want := []byte{0xFF, 0xFF, 0xF1, 0xF0}; !bytes.Equal(encoded, want) {
		t.Errorf("encoded % x, want % x", encoded, want)
	}

	got, err := pus.DecodeTimeShiftAllRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Offset.Ticks != -3600 {
		t.Errorf("decoded ticks = %d, want -3600: the field must sign-extend", got.Offset.Ticks)
	}
	if got.Offset.Duration() != -time.Hour {
		t.Errorf("duration = %s, want -1h", got.Offset.Duration())
	}
}

// TestRelativeTimeFineFieldRoundTripsExactly is why RelativeTime stores ticks
// rather than a time.Duration. A fine field of three octets resolves about
// 60 ns, and the nearest whole nanosecond is a different number: a Duration
// would lose the low bits and re-encode to different octets.
func TestRelativeTimeFineFieldRoundTripsExactly(t *testing.T) {
	p := schedProfile()
	p.RelativeTimeCoarseBytes = 4
	p.RelativeTimeFineBytes = 3

	// One fine tick: 2^-24 of a second, about 59.6 ns.
	offset := pus.RelativeTime{Ticks: 1, FineBytes: 3}
	request := pus.TimeShiftAllRequest{Profile: p, Offset: offset}
	encoded, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0, 0, 0, 0, 0, 0, 1}; !bytes.Equal(encoded, want) {
		t.Errorf("encoded % x, want % x", encoded, want)
	}

	got, err := pus.DecodeTimeShiftAllRequest(p, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Offset.Ticks != 1 {
		t.Errorf("ticks = %d, want 1", got.Offset.Ticks)
	}

	// And here is the loss a time.Duration would have caused. One fine tick is
	// 59.6 ns, which truncates to 59 ns; converting 59 ns back to ticks gives
	// zero, not one. Storing the Duration would have turned this value into a
	// different one and re-encoded different octets.
	viaDuration, err := pus.NewRelativeTime(p, got.Offset.Duration())
	if err != nil {
		t.Fatal(err)
	}
	if viaDuration.Ticks == got.Offset.Ticks {
		t.Errorf("the round trip through a Duration kept %d ticks; if that is now "+
			"lossless, RelativeTime could hold a Duration instead", viaDuration.Ticks)
	}
	if viaDuration.Ticks != 0 {
		t.Errorf("via Duration = %d ticks, want 0, 59 ns is less than one fine tick",
			viaDuration.Ticks)
	}
}

// TestRelativeTimeRefusesAnOverlongValue checks the field width is enforced
// rather than silently wrapped.
func TestRelativeTimeRefusesAnOverlongValue(t *testing.T) {
	p := schedProfile()
	p.RelativeTimeCoarseBytes = 1
	p.RelativeTimeFineBytes = 0

	// A one-octet two's complement field holds -128 to 127.
	if _, err := pus.NewRelativeTime(p, 128*time.Second); !errors.Is(err, pus.ErrValueTooLarge) {
		t.Errorf("err = %v, want ErrValueTooLarge", err)
	}
	ok, err := pus.NewRelativeTime(p, 127*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok.Ticks != 127 {
		t.Errorf("ticks = %d, want 127", ok.Ticks)
	}
}

// TestCapabilityFlagsChangeTheWireFormat checks the reason SupportsSubSchedules
// and SupportsGroups are in the profile: they decide field presence, and a
// decoder without them mis-splits the body.
func TestCapabilityFlagsChangeTheWireFormat(t *testing.T) {
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	packet := tcPacket(0x2A, 2)

	both := schedProfile()
	neither := both
	neither.SupportsSubSchedules = false
	neither.SupportsGroups = false

	activity := pus.ScheduledActivity{SubScheduleID: 1, GroupID: 7, ReleaseTime: release, Request: packet}

	withBoth, err := (pus.InsertActivitiesRequest{
		Profile: both, SubScheduleID: 1, Activities: []pus.ScheduledActivity{activity},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	withNeither, err := (pus.InsertActivitiesRequest{
		Profile: neither, Activities: []pus.ScheduledActivity{activity},
	}).Encode()
	if err != nil {
		t.Fatal(err)
	}

	// One sub-schedule ID for the request plus one group ID for the activity.
	wantDiff := both.SubScheduleIDSize() + both.GroupIDSize()
	if len(withBoth)-len(withNeither) != wantDiff {
		t.Fatalf("the two profiles differ by %d octets, want %d",
			len(withBoth)-len(withNeither), wantDiff)
	}

	// Decoding one profile's octets under the other is where the silent
	// corruption would happen, so the wrong profile must not produce a clean
	// parse of the same bytes.
	if got, err := pus.DecodeInsertActivitiesRequest(neither, withBoth); err == nil {
		t.Errorf("the capability-free profile parsed the capability-bearing octets: %+v", got)
	}

	// A caller that sets IDs the profile says are not supported is refused
	// rather than encoding a field the peer will not look for.
	filter := pus.ScheduleFilterRequest{
		Profile: neither,
		Subtype: pus.SubtypeDeleteByFilter,
		Filter: pus.ScheduleFilter{
			Window:         pus.TimeWindow{Type: pus.WindowSelectAll},
			SubScheduleIDs: []uint64{1},
		},
	}
	if _, err := filter.Encode(); !errors.Is(err, pus.ErrCapabilityNotSupported) {
		t.Errorf("err = %v, want ErrCapabilityNotSupported", err)
	}
}

// TestActivityRefusesAMismatchedPacketLength checks the embedded packet's own
// length field is trusted for the split and verified on the way out. A
// disagreement would desynchronise every activity after it.
func TestActivityRefusesAMismatchedPacketLength(t *testing.T) {
	p := schedProfile()
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	packet := tcPacket(0x2A, 4)
	// Claim a longer data field than the octets supplied.
	packet[5] = 0xFF

	request := pus.InsertActivitiesRequest{
		Profile: p,
		Activities: []pus.ScheduledActivity{
			{ReleaseTime: release, Request: packet},
		},
	}
	if _, err := request.Encode(); !errors.Is(err, pus.ErrPacketLengthMismatch) {
		t.Errorf("err = %v, want ErrPacketLengthMismatch", err)
	}
}

// TestScheduleListsRefuseAHostileCount checks an untrusted N cannot drive an
// allocation. Every list here is length-bounded before anything is made.
func TestScheduleListsRefuseAHostileCount(t *testing.T) {
	p := schedProfile()
	p.ScheduleCountBytes = 4

	// A count of 0xFFFFFFFF with no elements behind it.
	hostile := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	if _, err := pus.DecodeScheduleDetailReport(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("detail report: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeScheduleSummaryReport(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("summary report: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeSubScheduleStatusReport(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("sub-schedule status: err = %v, want ErrDataTooShort", err)
	}
	if _, err := pus.DecodeCreateSchedulingGroupsRequest(p, hostile); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("create groups: err = %v, want ErrDataTooShort", err)
	}
	// The insert request has a sub-schedule ID before the count.
	if _, err := pus.DecodeInsertActivitiesRequest(p, append([]byte{0x01}, hostile...)); !errors.Is(err, pus.ErrDataTooShort) {
		t.Errorf("insert: err = %v, want ErrDataTooShort", err)
	}
}

// TestST11ThroughTheRegistry round-trips one message of each shape through the
// registry, which is the path a caller actually takes.
func TestST11ThroughTheRegistry(t *testing.T) {
	p := schedProfile()
	r, err := pus.NewDefaultRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	release := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	offset, err := pus.NewRelativeTime(p, -90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ids := []pus.ScheduleRequestID{{SourceID: 1, APID: 2, SequenceCount: 3}}
	filter := pus.ScheduleFilter{
		Window:         pus.TimeWindow{Type: pus.WindowFrom, From: release},
		SubScheduleIDs: []uint64{1},
		GroupIDs:       []uint64{2, 3},
	}

	requests := []pus.Request{
		pus.ScheduleControlRequest{Subtype: pus.SubtypeResetSchedule},
		pus.InsertActivitiesRequest{Profile: p, SubScheduleID: 1, Activities: []pus.ScheduledActivity{
			{GroupID: 2, ReleaseTime: release, Request: tcPacket(0x2A, 3)},
		}},
		pus.ScheduleRequestIDListRequest{Profile: p, Subtype: pus.SubtypeDeleteByRequestID, RequestIDs: ids},
		pus.ScheduleFilterRequest{Profile: p, Subtype: pus.SubtypeDetailReportByFilter, Filter: filter},
		pus.TimeShiftByRequestIDRequest{Profile: p, Offset: offset, RequestIDs: ids},
		pus.TimeShiftByFilterRequest{Profile: p, Offset: offset, Filter: filter},
		pus.TimeShiftAllRequest{Profile: p, Offset: offset},
		pus.ScheduleIDListRequest{Profile: p, Subtype: pus.SubtypeDisableSubSchedules, IDs: []uint64{4}},
		pus.CreateSchedulingGroupsRequest{Profile: p, Groups: []pus.ScheduleStatusEntry{
			{ID: 1, Status: pus.ScheduleEnabled},
		}},
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

	reports := []pus.Report{
		&pus.ScheduleDetailReport{Profile: p, Activities: []pus.ScheduledActivity{
			{SubScheduleID: 1, GroupID: 2, ReleaseTime: release, Request: tcPacket(0x2A, 3)},
		}},
		&pus.ScheduleSummaryReport{Profile: p, Activities: []pus.SummaryActivity{
			{SubScheduleID: 1, GroupID: 2, ReleaseTime: release, RequestID: ids[0]},
		}},
		&pus.SubScheduleStatusReport{Profile: p, SubSchedules: []pus.ScheduleStatusEntry{
			{ID: 1, Status: pus.ScheduleDisabled},
		}},
		&pus.SchedulingGroupStatusReport{Profile: p, Groups: []pus.ScheduleStatusEntry{
			{ID: 2, Status: pus.ScheduleEnabled},
		}},
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

// TestScheduleEnumerationsMatchTheTables checks Tables 8-3, 8-4 and 8-5.
func TestScheduleEnumerationsMatchTheTables(t *testing.T) {
	// Tables 8-3 and 8-4: disabled 0, enabled 1.
	if pus.ScheduleDisabled != 0 || pus.ScheduleEnabled != 1 {
		t.Errorf("status raw values are %d and %d, want 0 and 1",
			pus.ScheduleDisabled, pus.ScheduleEnabled)
	}
	if pus.ScheduleDisabled.String() != "disabled" || pus.ScheduleEnabled.String() != "enabled" {
		t.Errorf("status names are %q and %q", pus.ScheduleDisabled, pus.ScheduleEnabled)
	}

	// Table 8-5.
	wantWindows := map[pus.TimeWindowType]string{
		0: "select all",
		1: "from time tag to time tag",
		2: "from time tag",
		3: "to time tag",
	}
	for raw, name := range wantWindows {
		if raw.String() != name {
			t.Errorf("window type %d is %q, want %q", uint64(raw), raw.String(), name)
		}
	}
}
