package pus

import "time"

// ST[11] time-based scheduling, per ECSS-E-ST-70-41C clauses 6.11 and 8.11.
//
// The schedule holds telecommands with a release time. The ground inserts
// them, shifts them, deletes them and asks what is in there; the spacecraft
// releases each one when its time comes. Twenty-seven message types, and this
// package implements the wire format of all of them.
//
// What it does not implement is the schedule itself. Clause 6.11 specifies a
// stateful machine (sub-schedules and groups that can be enabled and
// disabled, a release window, interlocks between activities) and running it
// is flight software's job, not a codec's. Every consistency check here is one
// a message can fail on its own.
//
// Three things about these figures needed care rather than transcription.
//
// The sub-schedule ID and group ID fields are marked optional, and what makes
// them present is not a flag in the message: clause 6.11.4.1 declares whether
// the subservice supports sub-schedules and groups at all. So the profile
// carries SupportsSubSchedules and SupportsGroups, and a decoder without them
// would mis-split every activity in a list.
//
// The two time tags of a filter have "deduced presence", deduced from the
// window type. Clause 6.11.10.3c settles which: item (b) sends the from tag
// for "from time tag" and "from time tag to time tag", item (c) sends the to
// tag for "to time tag" and "from time tag to time tag". So "to time tag"
// carries a tag in the second slot with the first slot absent, not a tag in
// the first slot.
//
// The request ID of figure 8-92 is not the request ID of figure 8-1. See
// ScheduleRequestID.
const (
	ServiceTimeBasedScheduling uint8 = 11

	SubtypeEnableScheduleExecution   uint8 = 1  // TC[11,1] clause 8.11.2.1
	SubtypeDisableScheduleExecution  uint8 = 2  // TC[11,2] clause 8.11.2.2
	SubtypeResetSchedule             uint8 = 3  // TC[11,3] clause 8.11.2.3
	SubtypeInsertActivities          uint8 = 4  // TC[11,4] clause 8.11.2.4
	SubtypeDeleteByRequestID         uint8 = 5  // TC[11,5] clause 8.11.2.5
	SubtypeDeleteByFilter            uint8 = 6  // TC[11,6] clause 8.11.2.6
	SubtypeTimeShiftByRequestID      uint8 = 7  // TC[11,7] clause 8.11.2.7
	SubtypeTimeShiftByFilter         uint8 = 8  // TC[11,8] clause 8.11.2.8
	SubtypeDetailReportByRequestID   uint8 = 9  // TC[11,9] clause 8.11.2.9
	SubtypeScheduleDetailReport      uint8 = 10 // TM[11,10] clause 8.11.2.10
	SubtypeDetailReportByFilter      uint8 = 11 // TC[11,11] clause 8.11.2.11
	SubtypeSummaryReportByRequestID  uint8 = 12 // TC[11,12] clause 8.11.2.12
	SubtypeScheduleSummaryReport     uint8 = 13 // TM[11,13] clause 8.11.2.13
	SubtypeSummaryReportByFilter     uint8 = 14 // TC[11,14] clause 8.11.2.14
	SubtypeTimeShiftAll              uint8 = 15 // TC[11,15] clause 8.11.2.15
	SubtypeDetailReportAll           uint8 = 16 // TC[11,16] clause 8.11.2.16
	SubtypeSummaryReportAll          uint8 = 17 // TC[11,17] clause 8.11.2.17
	SubtypeReportSubScheduleStatus   uint8 = 18 // TC[11,18] clause 8.11.2.18
	SubtypeSubScheduleStatusReport   uint8 = 19 // TM[11,19] clause 8.11.2.19
	SubtypeEnableSubSchedules        uint8 = 20 // TC[11,20] clause 8.11.2.20
	SubtypeDisableSubSchedules       uint8 = 21 // TC[11,21] clause 8.11.2.21
	SubtypeCreateSchedulingGroups    uint8 = 22 // TC[11,22] clause 8.11.2.22
	SubtypeDeleteSchedulingGroups    uint8 = 23 // TC[11,23] clause 8.11.2.23
	SubtypeEnableSchedulingGroups    uint8 = 24 // TC[11,24] clause 8.11.2.24
	SubtypeDisableSchedulingGroups   uint8 = 25 // TC[11,25] clause 8.11.2.25
	SubtypeReportSchedulingGroups    uint8 = 26 // TC[11,26] clause 8.11.2.26
	SubtypeSchedulingGroupStatusRept uint8 = 27 // TM[11,27] clause 8.11.2.27
)

// ScheduleStatus is a sub-schedule or scheduling group status.
//
// Tables 8-3 and 8-4 define the two separately and give them the same two
// values, so one type serves both.
type ScheduleStatus uint64

const (
	// ScheduleDisabled is raw value 0 (Tables 8-3 and 8-4).
	ScheduleDisabled ScheduleStatus = 0
	// ScheduleEnabled is raw value 1.
	ScheduleEnabled ScheduleStatus = 1
)

// String names the status the way Tables 8-3 and 8-4 write it.
func (s ScheduleStatus) String() string {
	switch s {
	case ScheduleDisabled:
		return "disabled"
	case ScheduleEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

// TimeWindowType is the type of a time-window filter, per Table 8-5.
type TimeWindowType uint64

const (
	// WindowSelectAll selects every activity; neither time tag travels.
	WindowSelectAll TimeWindowType = 0
	// WindowFromTo selects activities between and including both tags
	// (clause 6.11.10.2.2b). Both tags travel.
	WindowFromTo TimeWindowType = 1
	// WindowFrom selects activities at and after the from tag
	// (clause 6.11.10.2.2c). Only the from tag travels.
	WindowFrom TimeWindowType = 2
	// WindowTo selects activities before and at the to tag
	// (clause 6.11.10.2.2d). Only the to tag travels.
	WindowTo TimeWindowType = 3
)

// String names the type the way Table 8-5 writes it.
func (t TimeWindowType) String() string {
	switch t {
	case WindowSelectAll:
		return "select all"
	case WindowFromTo:
		return "from time tag to time tag"
	case WindowFrom:
		return "from time tag"
	case WindowTo:
		return "to time tag"
	default:
		return "unknown"
	}
}

// hasFrom reports whether this window type carries a from time tag, per
// clause 6.11.10.3c item (b).
func (t TimeWindowType) hasFrom() bool { return t == WindowFrom || t == WindowFromTo }

// hasTo reports whether this window type carries a to time tag, per
// clause 6.11.10.3c item (c).
func (t TimeWindowType) hasTo() bool { return t == WindowTo || t == WindowFromTo }

// valid reports whether the type is one of Table 8-5's four values. Clause
// 6.11.10.3d item 1 makes anything else a rejection.
func (t TimeWindowType) valid() bool { return t <= WindowTo }

// ScheduleRequestID identifies one scheduled activity, per Figure 8-92.
//
// This is not the RequestID of ST[01]. That one (Figure 8-1) is a bit-packed
// 32-bit copy of the CCSDS primary header, and its own note says it "cannot be
// used to identify the request since it does not contain the identifier of the
// source of that request". Figure 8-92 fixes exactly that: it carries a source
// ID as well, and all three of its fields are whole octets at
// mission-declared widths rather than bitfields. Encoding one with the other's
// codec produces the wrong bytes and no error, so the two are separate types.
type ScheduleRequestID struct {
	// SourceID identifies where the request came from, the field ST[01]'s
	// request ID lacks.
	SourceID uint64
	// APID is the application process the request was addressed to.
	APID uint64
	// SequenceCount is the request's packet sequence count.
	SequenceCount uint64
}

// encode appends the request ID at the profile's widths.
func (r ScheduleRequestID) encode(dst []byte, p MissionProfile) ([]byte, error) {
	out, err := putUint(dst, r.SourceID, p.ScheduleSourceIDSize())
	if err != nil {
		return nil, err
	}
	if out, err = putUint(out, r.APID, p.ScheduleAPIDSize()); err != nil {
		return nil, err
	}
	return putUint(out, r.SequenceCount, p.ScheduleSeqCountSize())
}

// decodeScheduleRequestID reads one request ID from the front of data.
func decodeScheduleRequestID(p MissionProfile, data []byte) (ScheduleRequestID, int, error) {
	if len(data) < p.ScheduleRequestIDSize() {
		return ScheduleRequestID{}, 0, ErrDataTooShort
	}
	offset := 0

	source, err := readUint(data[offset:], p.ScheduleSourceIDSize())
	if err != nil {
		return ScheduleRequestID{}, 0, err
	}
	offset += p.ScheduleSourceIDSize()

	apid, err := readUint(data[offset:], p.ScheduleAPIDSize())
	if err != nil {
		return ScheduleRequestID{}, 0, err
	}
	offset += p.ScheduleAPIDSize()

	count, err := readUint(data[offset:], p.ScheduleSeqCountSize())
	if err != nil {
		return ScheduleRequestID{}, 0, err
	}
	offset += p.ScheduleSeqCountSize()

	return ScheduleRequestID{SourceID: source, APID: apid, SequenceCount: count}, offset, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleRequestID) Humanize() string {
	return "source " + itoa(int(r.SourceID)) +
		" APID " + itoa(int(r.APID)) +
		" seq " + itoa(int(r.SequenceCount))
}

// TimeWindow is the time-window part of a filter, per Figures 8-93 and its
// siblings.
type TimeWindow struct {
	// Type selects which of the four filtering mechanisms of clause
	// 6.11.10.2.2 applies, and with it which tags travel.
	Type TimeWindowType

	// From is the "from time tag". It travels for WindowFrom and WindowFromTo.
	From time.Time
	// To is the "to time tag". It travels for WindowTo and WindowFromTo.
	To time.Time

	// RawFrom and RawTo carry the tags when the profile declares TimeRaw.
	RawFrom []byte
	RawTo   []byte
}

// encode appends the time window.
func (w TimeWindow) encode(dst []byte, p MissionProfile) ([]byte, error) {
	if !w.Type.valid() {
		return nil, ErrInvalidTimeWindow
	}
	// Clause 6.11.10.3d item 2 rejects a from tag greater than a to tag. It is
	// a check on the message alone, so it belongs here.
	if w.Type == WindowFromTo && p.TimeFormat != TimeRaw && w.From.After(w.To) {
		return nil, ErrInvalidTimeWindow
	}

	out, err := putUint(dst, uint64(w.Type), p.TimeWindowTypeSize())
	if err != nil {
		return nil, err
	}
	if w.Type.hasFrom() {
		field, err := encodeAbsoluteTime(p, w.From, w.RawFrom)
		if err != nil {
			return nil, err
		}
		out = append(out, field...)
	}
	if w.Type.hasTo() {
		field, err := encodeAbsoluteTime(p, w.To, w.RawTo)
		if err != nil {
			return nil, err
		}
		out = append(out, field...)
	}
	return out, nil
}

// decodeTimeWindow reads a time window from the front of data.
func decodeTimeWindow(p MissionProfile, data []byte) (TimeWindow, int, error) {
	width := p.TimeWindowTypeSize()
	raw, err := readUint(data, width)
	if err != nil {
		return TimeWindow{}, 0, err
	}
	window := TimeWindow{Type: TimeWindowType(raw)}
	if !window.Type.valid() {
		return TimeWindow{}, 0, ErrInvalidTimeWindow
	}
	offset := width

	if window.Type.hasFrom() {
		stamp, rawField, used, err := decodeAbsoluteTime(p, data[offset:])
		if err != nil {
			return TimeWindow{}, 0, err
		}
		window.From, window.RawFrom = stamp, rawField
		offset += used
	}
	if window.Type.hasTo() {
		stamp, rawField, used, err := decodeAbsoluteTime(p, data[offset:])
		if err != nil {
			return TimeWindow{}, 0, err
		}
		window.To, window.RawTo = stamp, rawField
		offset += used
	}

	if window.Type == WindowFromTo && p.TimeFormat != TimeRaw && window.From.After(window.To) {
		return TimeWindow{}, 0, ErrInvalidTimeWindow
	}
	return window, offset, nil
}

// Humanize returns a human-readable summary.
func (w TimeWindow) Humanize() string {
	switch w.Type {
	case WindowFromTo:
		return w.Type.String() + " " + w.From.UTC().Format(time.RFC3339) +
			" to " + w.To.UTC().Format(time.RFC3339)
	case WindowFrom:
		return w.Type.String() + " " + w.From.UTC().Format(time.RFC3339)
	case WindowTo:
		return w.Type.String() + " " + w.To.UTC().Format(time.RFC3339)
	default:
		return w.Type.String()
	}
}

// ScheduleFilter is the whole filter of Figures 8-93, 8-95, 8-98 and 8-101: a
// time window, then optionally a list of sub-schedules, then optionally a list
// of groups.
//
// The two lists are intersected with the window, not unioned with it
// (clause 6.11.10.2.5). Each list is present only when the subservice supports
// that capability, which the profile declares.
type ScheduleFilter struct {
	Window TimeWindow

	// SubScheduleIDs narrows the selection to these sub-schedules. Present
	// only when the profile sets SupportsSubSchedules. An empty list with the
	// capability supported still carries its count field, as N1 = 0.
	SubScheduleIDs []uint64

	// GroupIDs narrows the selection to these groups. Present only when the
	// profile sets SupportsGroups.
	GroupIDs []uint64
}

// encode appends the filter.
func (f ScheduleFilter) encode(dst []byte, p MissionProfile) ([]byte, error) {
	out, err := f.Window.encode(dst, p)
	if err != nil {
		return nil, err
	}
	if p.SupportsSubSchedules {
		if out, err = encodeUintList(out, f.SubScheduleIDs,
			p.ScheduleCountSize(), p.SubScheduleIDSize()); err != nil {
			return nil, err
		}
	} else if len(f.SubScheduleIDs) > 0 {
		// Encoding them would produce octets the peer cannot parse, since it
		// is not expecting the field at all.
		return nil, ErrCapabilityNotSupported
	}
	if p.SupportsGroups {
		if out, err = encodeUintList(out, f.GroupIDs,
			p.ScheduleCountSize(), p.GroupIDSize()); err != nil {
			return nil, err
		}
	} else if len(f.GroupIDs) > 0 {
		return nil, ErrCapabilityNotSupported
	}
	return out, nil
}

// decodeScheduleFilter reads a filter from the front of data.
func decodeScheduleFilter(p MissionProfile, data []byte) (ScheduleFilter, int, error) {
	window, offset, err := decodeTimeWindow(p, data)
	if err != nil {
		return ScheduleFilter{}, 0, err
	}
	filter := ScheduleFilter{Window: window}

	if p.SupportsSubSchedules {
		ids, used, err := readUintList(data[offset:], p.ScheduleCountSize(), p.SubScheduleIDSize())
		if err != nil {
			return ScheduleFilter{}, 0, err
		}
		filter.SubScheduleIDs = ids
		offset += used
	}
	if p.SupportsGroups {
		ids, used, err := readUintList(data[offset:], p.ScheduleCountSize(), p.GroupIDSize())
		if err != nil {
			return ScheduleFilter{}, 0, err
		}
		filter.GroupIDs = ids
		offset += used
	}
	return filter, offset, nil
}

// Humanize returns a human-readable summary.
func (f ScheduleFilter) Humanize() string {
	return f.Window.Humanize() +
		", " + itoa(len(f.SubScheduleIDs)) + " sub-schedule(s)" +
		", " + itoa(len(f.GroupIDs)) + " group(s)"
}

// ScheduledActivity is one entry of an insert request or a detail report: a
// telecommand and when to release it.
type ScheduledActivity struct {
	// SubScheduleID names the sub-schedule the activity belongs to. Carried
	// per activity in a detail report (Figure 8-97) but once for the whole
	// request in an insert (Figure 8-91), so this field is used only by the
	// report.
	SubScheduleID uint64

	// GroupID names the scheduling group. Present when the profile sets
	// SupportsGroups.
	GroupID uint64

	// ReleaseTime is when the request is to be released.
	ReleaseTime time.Time
	// RawReleaseTime carries the release time when the profile declares
	// TimeRaw.
	RawReleaseTime []byte

	// Request is the telecommand to release: a whole CCSDS telecommand packet,
	// primary header included. Table 7-12 PFC 1 types the field that way, and
	// its length comes from the packet's own length field rather than from
	// anything in the ST[11] message.
	Request []byte
}

// tcPacketLength returns the total length of the CCSDS packet at the front of
// data.
//
// The packet data length field sits in octets 4 and 5 of the 6-octet primary
// header and holds the data field length minus one, per CCSDS 133.0-B-2
// 4.1.3.5.3. Reading it here rather than through pkg/spp keeps pkg/pus free of
// a dependency on it: the two compose through an interface today, and neither
// imports the other.
func tcPacketLength(data []byte) (int, error) {
	const primaryHeaderSize = 6
	if len(data) < primaryHeaderSize {
		return 0, ErrDataTooShort
	}
	dataFieldLength := int(data[4])<<8 | int(data[5])
	return primaryHeaderSize + dataFieldLength + 1, nil
}

// encodeActivity appends one activity. withSubSchedule selects the detail
// report's layout, which carries a sub-schedule ID per activity.
func (a ScheduledActivity) encodeActivity(dst []byte, p MissionProfile, withSubSchedule bool) ([]byte, error) {
	out := dst
	var err error

	if withSubSchedule && p.SupportsSubSchedules {
		if out, err = putUint(out, a.SubScheduleID, p.SubScheduleIDSize()); err != nil {
			return nil, err
		}
	}
	if p.SupportsGroups {
		if out, err = putUint(out, a.GroupID, p.GroupIDSize()); err != nil {
			return nil, err
		}
	}

	field, err := encodeAbsoluteTime(p, a.ReleaseTime, a.RawReleaseTime)
	if err != nil {
		return nil, err
	}
	out = append(out, field...)

	// A request whose own length field disagrees with the octets given would
	// desynchronise every activity after it in the list.
	length, err := tcPacketLength(a.Request)
	if err != nil {
		return nil, err
	}
	if length != len(a.Request) {
		return nil, ErrPacketLengthMismatch
	}
	return append(out, a.Request...), nil
}

// decodeActivity reads one activity from the front of data.
func decodeActivity(p MissionProfile, data []byte, withSubSchedule bool) (ScheduledActivity, int, error) {
	var activity ScheduledActivity
	offset := 0

	if withSubSchedule && p.SupportsSubSchedules {
		id, err := readUint(data[offset:], p.SubScheduleIDSize())
		if err != nil {
			return ScheduledActivity{}, 0, err
		}
		activity.SubScheduleID = id
		offset += p.SubScheduleIDSize()
	}
	if p.SupportsGroups {
		id, err := readUint(data[offset:], p.GroupIDSize())
		if err != nil {
			return ScheduledActivity{}, 0, err
		}
		activity.GroupID = id
		offset += p.GroupIDSize()
	}

	stamp, raw, used, err := decodeAbsoluteTime(p, data[offset:])
	if err != nil {
		return ScheduledActivity{}, 0, err
	}
	activity.ReleaseTime, activity.RawReleaseTime = stamp, raw
	offset += used

	length, err := tcPacketLength(data[offset:])
	if err != nil {
		return ScheduledActivity{}, 0, err
	}
	if len(data)-offset < length {
		return ScheduledActivity{}, 0, ErrDataTooShort
	}
	activity.Request = data[offset : offset+length]
	offset += length

	return activity, offset, nil
}

// Humanize returns a human-readable summary.
func (a ScheduledActivity) Humanize() string {
	return "release " + a.ReleaseTime.UTC().Format(time.RFC3339) +
		", group " + itoa(int(a.GroupID)) +
		", " + itoa(len(a.Request)) + " octet request"
}

// encodeUintList appends a count-prefixed list of unsigned integers.
func encodeUintList(dst []byte, values []uint64, countWidth, elemWidth int) ([]byte, error) {
	out, err := putUint(dst, uint64(len(values)), countWidth)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		if out, err = putUint(out, v, elemWidth); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// readActivityList reads a count-prefixed list of activities.
//
// The count is untrusted and an activity is variable-length, so nothing is
// sized from the count: the list is walked until the octets run out, and a
// count that disagrees with what the octets hold is an error rather than a
// partial answer. A non-empty list must consume at least one octet per
// activity, which is what bounds the loop.
func readActivityList(p MissionProfile, data []byte, withSubSchedule bool) ([]ScheduledActivity, int, error) {
	countWidth := p.ScheduleCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, 0, err
	}
	offset := countWidth
	if count == 0 {
		return nil, offset, nil
	}

	// The shortest possible activity is one release time plus the shortest
	// possible packet, so this bounds the count before anything is allocated.
	minimum := p.TimeSize() + 7
	if p.SupportsGroups {
		minimum += p.GroupIDSize()
	}
	if withSubSchedule && p.SupportsSubSchedules {
		minimum += p.SubScheduleIDSize()
	}
	if uint64(len(data)-offset)/uint64(minimum) < count {
		return nil, 0, ErrDataTooShort
	}

	activities := make([]ScheduledActivity, 0, count)
	for i := uint64(0); i < count; i++ {
		activity, used, err := decodeActivity(p, data[offset:], withSubSchedule)
		if err != nil {
			return nil, 0, err
		}
		activities = append(activities, activity)
		offset += used
	}
	return activities, offset, nil
}

// readRequestIDList reads a count-prefixed list of request IDs.
func readRequestIDList(p MissionProfile, data []byte) ([]ScheduleRequestID, int, error) {
	countWidth := p.ScheduleCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, 0, err
	}
	offset := countWidth
	if count == 0 {
		return nil, offset, nil
	}

	width := p.ScheduleRequestIDSize()
	if uint64(len(data)-offset)/uint64(width) < count {
		return nil, 0, ErrDataTooShort
	}

	ids := make([]ScheduleRequestID, 0, count)
	for i := uint64(0); i < count; i++ {
		id, used, err := decodeScheduleRequestID(p, data[offset:])
		if err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
		offset += used
	}
	return ids, offset, nil
}

// encodeRequestIDList appends a count-prefixed list of request IDs.
func encodeRequestIDList(dst []byte, ids []ScheduleRequestID, p MissionProfile) ([]byte, error) {
	out, err := putUint(dst, uint64(len(ids)), p.ScheduleCountSize())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if out, err = id.encode(out, p); err != nil {
			return nil, err
		}
	}
	return out, nil
}
