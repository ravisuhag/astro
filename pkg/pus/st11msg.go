package pus

import "time"

// The twenty-seven ST[11] message types.
//
// Several of the figures are the same figure twice: 8-104 and 8-105 differ
// only in which subtype carries them, and so do 8-107, 8-108 and 8-109. Where
// the standard repeats a structure this file carries one type that pins the
// subtype, rather than pretending the messages differ.

// scheduleControlSubtypes are the seven ST[11] requests whose application
// data field "shall be omitted".
var scheduleControlSubtypes = []uint8{
	SubtypeEnableScheduleExecution,
	SubtypeDisableScheduleExecution,
	SubtypeResetSchedule,
	SubtypeDetailReportAll,
	SubtypeSummaryReportAll,
	SubtypeReportSubScheduleStatus,
	SubtypeReportSchedulingGroups,
}

// scheduleControlNames labels the empty-bodied requests for Humanize.
var scheduleControlNames = map[uint8]string{
	SubtypeEnableScheduleExecution:  "enable the time-based schedule execution function",
	SubtypeDisableScheduleExecution: "disable the time-based schedule execution function",
	SubtypeResetSchedule:            "reset the time-based schedule",
	SubtypeDetailReportAll:          "detail-report all time-based scheduled activities",
	SubtypeSummaryReportAll:         "summary-report all time-based scheduled activities",
	SubtypeReportSubScheduleStatus:  "report the status of each time-based sub-schedule",
	SubtypeReportSchedulingGroups:   "report the status of each time-based scheduling group",
}

// ScheduleControlRequest is any of the seven ST[11] requests that carry no
// body: TC[11,1], TC[11,2], TC[11,3], TC[11,16], TC[11,17], TC[11,18] and
// TC[11,26].
//
// They are one type because they are one structure. The subtype is what
// distinguishes them, so it is a field rather than seven near-identical
// declarations.
type ScheduleControlRequest struct {
	// Subtype is one of the seven; anything else is refused on encode.
	Subtype uint8
}

// Key returns the message type.
func (r ScheduleControlRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: r.Subtype}
}

// Encode returns an empty application data field.
func (r ScheduleControlRequest) Encode() ([]byte, error) {
	if _, ok := scheduleControlNames[r.Subtype]; !ok {
		return nil, ErrWrongMessageType
	}
	return nil, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleControlRequest) Humanize() string {
	name, ok := scheduleControlNames[r.Subtype]
	if !ok {
		name = "unknown"
	}
	return "PUS TC[11," + itoa(int(r.Subtype)) + "] " + name
}

// InsertActivitiesRequest is TC[11,4], per Figure 8-91.
//
// The sub-schedule ID comes once, before the count, so one request inserts
// into one sub-schedule. The detail report of TM[11,10] puts it inside the
// repeated group instead, because a report can span sub-schedules.
type InsertActivitiesRequest struct {
	Profile MissionProfile

	// SubScheduleID names the sub-schedule to insert into. Present only when
	// the profile sets SupportsSubSchedules.
	SubScheduleID uint64

	// Activities are the telecommands to schedule, each with its release time.
	Activities []ScheduledActivity
}

// Key returns the message type.
func (InsertActivitiesRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeInsertActivities}
}

// Encode serializes the application data field.
func (r InsertActivitiesRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}

	var out []byte
	var err error
	if r.Profile.SupportsSubSchedules {
		if out, err = putUint(out, r.SubScheduleID, r.Profile.SubScheduleIDSize()); err != nil {
			return nil, err
		}
	}
	if out, err = putUint(out, uint64(len(r.Activities)), r.Profile.ScheduleCountSize()); err != nil {
		return nil, err
	}
	for _, activity := range r.Activities {
		if out, err = activity.encodeActivity(out, r.Profile, false); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeInsertActivitiesRequest parses TC[11,4].
func DecodeInsertActivitiesRequest(profile MissionProfile, data []byte) (*InsertActivitiesRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	request := &InsertActivitiesRequest{Profile: profile}
	offset := 0
	if profile.SupportsSubSchedules {
		id, err := readUint(data, profile.SubScheduleIDSize())
		if err != nil {
			return nil, err
		}
		request.SubScheduleID = id
		offset += profile.SubScheduleIDSize()
	}

	activities, used, err := readActivityList(profile, data[offset:], false)
	if err != nil {
		return nil, err
	}
	request.Activities = activities
	offset += used

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return request, nil
}

// Humanize returns a human-readable summary.
func (r InsertActivitiesRequest) Humanize() string {
	out := "PUS TC[11,4] insert activities into the time-based schedule" +
		"\n  Sub-schedule .. " + itoa(int(r.SubScheduleID)) +
		"\n  Activities .... " + itoa(len(r.Activities))
	for _, activity := range r.Activities {
		out += "\n    " + activity.Humanize()
	}
	return out
}

// requestIDListNames labels the three request-ID-list requests.
var requestIDListNames = map[uint8]string{
	SubtypeDeleteByRequestID:        "delete time-based scheduled activities identified by request identifier",
	SubtypeDetailReportByRequestID:  "detail-report time-based scheduled activities identified by request identifier",
	SubtypeSummaryReportByRequestID: "summary-report time-based scheduled activities identified by request identifier",
}

// ScheduleRequestIDListRequest is TC[11,5], TC[11,9] or TC[11,12], per
// Figures 8-92, 8-96 and 8-99 — one structure printed three times.
type ScheduleRequestIDListRequest struct {
	Profile MissionProfile
	// Subtype is one of the three.
	Subtype uint8
	// RequestIDs names the activities the request concerns.
	RequestIDs []ScheduleRequestID
}

// Key returns the message type.
func (r ScheduleRequestIDListRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: r.Subtype}
}

// Encode serializes the application data field.
func (r ScheduleRequestIDListRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	if _, ok := requestIDListNames[r.Subtype]; !ok {
		return nil, ErrWrongMessageType
	}
	return encodeRequestIDList(nil, r.RequestIDs, r.Profile)
}

// DecodeScheduleRequestIDListRequest parses TC[11,5], TC[11,9] or TC[11,12].
//
// The subtype has to be supplied because the three share one structure: the
// octets alone do not say which of the three they are.
func DecodeScheduleRequestIDListRequest(profile MissionProfile, subtype uint8, data []byte) (*ScheduleRequestIDListRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	ids, used, err := readRequestIDList(profile, data)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &ScheduleRequestIDListRequest{Profile: profile, Subtype: subtype, RequestIDs: ids}, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleRequestIDListRequest) Humanize() string {
	name, ok := requestIDListNames[r.Subtype]
	if !ok {
		name = "unknown"
	}
	out := "PUS TC[11," + itoa(int(r.Subtype)) + "] " + name +
		"\n  Activities .... " + itoa(len(r.RequestIDs))
	for _, id := range r.RequestIDs {
		out += "\n    " + id.Humanize()
	}
	return out
}

// filterNames labels the three filter requests.
var filterNames = map[uint8]string{
	SubtypeDeleteByFilter:        "delete the time-based scheduled activities identified by a filter",
	SubtypeDetailReportByFilter:  "detail-report the time-based scheduled activities identified by a filter",
	SubtypeSummaryReportByFilter: "summary-report the time-based scheduled activities identified by a filter",
}

// ScheduleFilterRequest is TC[11,6], TC[11,11] or TC[11,14], per Figures
// 8-93, 8-98 and 8-101 — again one structure printed three times.
type ScheduleFilterRequest struct {
	Profile MissionProfile
	// Subtype is one of the three.
	Subtype uint8
	// Filter selects the activities the request concerns.
	Filter ScheduleFilter
}

// Key returns the message type.
func (r ScheduleFilterRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: r.Subtype}
}

// Encode serializes the application data field.
func (r ScheduleFilterRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	if _, ok := filterNames[r.Subtype]; !ok {
		return nil, ErrWrongMessageType
	}
	return r.Filter.encode(nil, r.Profile)
}

// DecodeScheduleFilterRequest parses TC[11,6], TC[11,11] or TC[11,14]. The
// subtype has to be supplied for the same reason as above.
func DecodeScheduleFilterRequest(profile MissionProfile, subtype uint8, data []byte) (*ScheduleFilterRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	filter, used, err := decodeScheduleFilter(profile, data)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &ScheduleFilterRequest{Profile: profile, Subtype: subtype, Filter: filter}, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleFilterRequest) Humanize() string {
	name, ok := filterNames[r.Subtype]
	if !ok {
		name = "unknown"
	}
	return "PUS TC[11," + itoa(int(r.Subtype)) + "] " + name +
		"\n  Filter ........ " + r.Filter.Humanize()
}

// TimeShiftByRequestIDRequest is TC[11,7], per Figure 8-94: an offset, then
// the activities to shift.
type TimeShiftByRequestIDRequest struct {
	Profile MissionProfile
	// Offset is how far to shift, positive or negative (clause 7.3.11b).
	Offset RelativeTime
	// RequestIDs names the activities to shift.
	RequestIDs []ScheduleRequestID
}

// Key returns the message type.
func (TimeShiftByRequestIDRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeTimeShiftByRequestID}
}

// Encode serializes the application data field.
func (r TimeShiftByRequestIDRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := encodeRelativeTime(r.Profile, r.Offset)
	if err != nil {
		return nil, err
	}
	return encodeRequestIDList(out, r.RequestIDs, r.Profile)
}

// DecodeTimeShiftByRequestIDRequest parses TC[11,7].
func DecodeTimeShiftByRequestIDRequest(profile MissionProfile, data []byte) (*TimeShiftByRequestIDRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	offsetValue, used, err := decodeRelativeTime(profile, data)
	if err != nil {
		return nil, err
	}
	ids, consumed, err := readRequestIDList(profile, data[used:])
	if err != nil {
		return nil, err
	}
	if used+consumed != len(data) {
		return nil, ErrTrailingBytes
	}
	return &TimeShiftByRequestIDRequest{Profile: profile, Offset: offsetValue, RequestIDs: ids}, nil
}

// Humanize returns a human-readable summary.
func (r TimeShiftByRequestIDRequest) Humanize() string {
	return "PUS TC[11,7] time-shift scheduled activities identified by request identifier" +
		"\n  Offset ........ " + r.Offset.Duration().String() +
		"\n  Activities .... " + itoa(len(r.RequestIDs))
}

// TimeShiftByFilterRequest is TC[11,8], per Figure 8-95.
type TimeShiftByFilterRequest struct {
	Profile MissionProfile
	// Offset is how far to shift.
	Offset RelativeTime
	// Filter selects the activities to shift.
	Filter ScheduleFilter
}

// Key returns the message type.
func (TimeShiftByFilterRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeTimeShiftByFilter}
}

// Encode serializes the application data field.
func (r TimeShiftByFilterRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := encodeRelativeTime(r.Profile, r.Offset)
	if err != nil {
		return nil, err
	}
	return r.Filter.encode(out, r.Profile)
}

// DecodeTimeShiftByFilterRequest parses TC[11,8].
func DecodeTimeShiftByFilterRequest(profile MissionProfile, data []byte) (*TimeShiftByFilterRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	offsetValue, used, err := decodeRelativeTime(profile, data)
	if err != nil {
		return nil, err
	}
	filter, consumed, err := decodeScheduleFilter(profile, data[used:])
	if err != nil {
		return nil, err
	}
	if used+consumed != len(data) {
		return nil, ErrTrailingBytes
	}
	return &TimeShiftByFilterRequest{Profile: profile, Offset: offsetValue, Filter: filter}, nil
}

// Humanize returns a human-readable summary.
func (r TimeShiftByFilterRequest) Humanize() string {
	return "PUS TC[11,8] time-shift the scheduled activities identified by a filter" +
		"\n  Offset ........ " + r.Offset.Duration().String() +
		"\n  Filter ........ " + r.Filter.Humanize()
}

// TimeShiftAllRequest is TC[11,15], per Figure 8-102: an offset and nothing
// else.
type TimeShiftAllRequest struct {
	Profile MissionProfile
	// Offset is how far to shift every scheduled activity.
	Offset RelativeTime
}

// Key returns the message type.
func (TimeShiftAllRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeTimeShiftAll}
}

// Encode serializes the application data field.
func (r TimeShiftAllRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	return encodeRelativeTime(r.Profile, r.Offset)
}

// DecodeTimeShiftAllRequest parses TC[11,15].
func DecodeTimeShiftAllRequest(profile MissionProfile, data []byte) (*TimeShiftAllRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	offsetValue, used, err := decodeRelativeTime(profile, data)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &TimeShiftAllRequest{Profile: profile, Offset: offsetValue}, nil
}

// Humanize returns a human-readable summary.
func (r TimeShiftAllRequest) Humanize() string {
	return "PUS TC[11,15] time-shift all scheduled activities" +
		"\n  Offset ........ " + r.Offset.Duration().String()
}

// ScheduleDetailReport is TM[11,10], per Figure 8-97.
//
// Unlike the insert request, each reported activity carries its own
// sub-schedule ID, so one report can span sub-schedules.
type ScheduleDetailReport struct {
	Profile MissionProfile
	// Activities are the scheduled activities being reported, in full.
	Activities []ScheduledActivity
}

// Key returns the message type.
func (ScheduleDetailReport) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeScheduleDetailReport}
}

// Encode serializes the source data field.
func (r ScheduleDetailReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Activities)), r.Profile.ScheduleCountSize())
	if err != nil {
		return nil, err
	}
	for _, activity := range r.Activities {
		if out, err = activity.encodeActivity(out, r.Profile, true); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeScheduleDetailReport parses TM[11,10].
func DecodeScheduleDetailReport(profile MissionProfile, data []byte) (*ScheduleDetailReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	activities, used, err := readActivityList(profile, data, true)
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	return &ScheduleDetailReport{Profile: profile, Activities: activities}, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleDetailReport) Humanize() string {
	out := "PUS TM[11,10] time-based schedule detail report" +
		"\n  Activities .... " + itoa(len(r.Activities))
	for _, activity := range r.Activities {
		out += "\n    sub-schedule " + itoa(int(activity.SubScheduleID)) + ", " + activity.Humanize()
	}
	return out
}

// SummaryActivity is one entry of TM[11,13], per Figure 8-100: an activity
// named by its request ID rather than carried in full.
type SummaryActivity struct {
	// SubScheduleID names the sub-schedule. Present when the profile sets
	// SupportsSubSchedules.
	SubScheduleID uint64
	// GroupID names the scheduling group. Present when the profile sets
	// SupportsGroups.
	GroupID uint64
	// ReleaseTime is when the request is to be released.
	ReleaseTime time.Time
	// RawReleaseTime carries the release time when the profile declares
	// TimeRaw.
	RawReleaseTime []byte
	// RequestID names the activity. This is the summary: where the detail
	// report carries the whole telecommand, this carries only its identity.
	RequestID ScheduleRequestID
}

// ScheduleSummaryReport is TM[11,13], per Figure 8-100.
type ScheduleSummaryReport struct {
	Profile MissionProfile
	// Activities are the scheduled activities being summarised.
	Activities []SummaryActivity
}

// Key returns the message type.
func (ScheduleSummaryReport) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeScheduleSummaryReport}
}

// summaryActivitySize is the encoded width of one summary entry. Every field
// is fixed-width, so unlike a detail report this one can be sized up front.
func summaryActivitySize(p MissionProfile) int {
	size := p.TimeSize() + p.ScheduleRequestIDSize()
	if p.SupportsSubSchedules {
		size += p.SubScheduleIDSize()
	}
	if p.SupportsGroups {
		size += p.GroupIDSize()
	}
	return size
}

// Encode serializes the source data field.
func (r ScheduleSummaryReport) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(r.Activities)), r.Profile.ScheduleCountSize())
	if err != nil {
		return nil, err
	}
	for _, activity := range r.Activities {
		if r.Profile.SupportsSubSchedules {
			if out, err = putUint(out, activity.SubScheduleID, r.Profile.SubScheduleIDSize()); err != nil {
				return nil, err
			}
		}
		if r.Profile.SupportsGroups {
			if out, err = putUint(out, activity.GroupID, r.Profile.GroupIDSize()); err != nil {
				return nil, err
			}
		}
		field, err := encodeAbsoluteTime(r.Profile, activity.ReleaseTime, activity.RawReleaseTime)
		if err != nil {
			return nil, err
		}
		out = append(out, field...)
		if out, err = activity.RequestID.encode(out, r.Profile); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeScheduleSummaryReport parses TM[11,13].
func DecodeScheduleSummaryReport(profile MissionProfile, data []byte) (*ScheduleSummaryReport, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	countWidth := profile.ScheduleCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, err
	}
	offset := countWidth

	report := &ScheduleSummaryReport{Profile: profile}
	if count > 0 {
		width := summaryActivitySize(profile)
		if width == 0 {
			return nil, ErrInvalidProfile
		}
		if uint64(len(data)-offset)/uint64(width) < count {
			return nil, ErrDataTooShort
		}
		report.Activities = make([]SummaryActivity, 0, count)
	}

	for i := uint64(0); i < count; i++ {
		var activity SummaryActivity
		if profile.SupportsSubSchedules {
			id, err := readUint(data[offset:], profile.SubScheduleIDSize())
			if err != nil {
				return nil, err
			}
			activity.SubScheduleID = id
			offset += profile.SubScheduleIDSize()
		}
		if profile.SupportsGroups {
			id, err := readUint(data[offset:], profile.GroupIDSize())
			if err != nil {
				return nil, err
			}
			activity.GroupID = id
			offset += profile.GroupIDSize()
		}
		stamp, raw, used, err := decodeAbsoluteTime(profile, data[offset:])
		if err != nil {
			return nil, err
		}
		activity.ReleaseTime, activity.RawReleaseTime = stamp, raw
		offset += used

		id, used, err := decodeScheduleRequestID(profile, data[offset:])
		if err != nil {
			return nil, err
		}
		activity.RequestID = id
		offset += used

		report.Activities = append(report.Activities, activity)
	}

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return report, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleSummaryReport) Humanize() string {
	out := "PUS TM[11,13] time-based schedule summary report" +
		"\n  Activities .... " + itoa(len(r.Activities))
	for _, activity := range r.Activities {
		out += "\n    release " + activity.ReleaseTime.UTC().Format(time.RFC3339) +
			", " + activity.RequestID.Humanize()
	}
	return out
}

// idListNames labels the five plain ID-list requests.
var idListNames = map[uint8]string{
	SubtypeEnableSubSchedules:      "enable time-based sub-schedules",
	SubtypeDisableSubSchedules:     "disable time-based sub-schedules",
	SubtypeDeleteSchedulingGroups:  "delete time-based scheduling groups",
	SubtypeEnableSchedulingGroups:  "enable time-based scheduling groups",
	SubtypeDisableSchedulingGroups: "disable time-based scheduling groups",
}

// ScheduleIDListRequest is TC[11,20], TC[11,21], TC[11,23], TC[11,24] or
// TC[11,25], per Figures 8-104, 8-105, 8-107, 8-108 and 8-109 — one structure
// printed five times, over sub-schedule IDs for the first two and group IDs
// for the last three.
//
// An empty list is not "nothing": clauses 8.11.2.20c, 8.11.2.21c, 8.11.2.23c,
// 8.11.2.24c and 8.11.2.25c all say that N set to 0 means all of them. So a
// zero-length ID list enables, disables or deletes everything, and IsAll says
// so rather than leaving the caller to remember.
type ScheduleIDListRequest struct {
	Profile MissionProfile
	// Subtype is one of the five.
	Subtype uint8
	// IDs are the sub-schedules or groups the request names. Empty means all.
	IDs []uint64
}

// Key returns the message type.
func (r ScheduleIDListRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: r.Subtype}
}

// IsAll reports whether this request applies to every sub-schedule or group,
// which is what an N of 0 means.
func (r ScheduleIDListRequest) IsAll() bool { return len(r.IDs) == 0 }

// idWidth returns the element width for this subtype: sub-schedule IDs for
// TC[11,20] and TC[11,21], group IDs for the other three.
func (r ScheduleIDListRequest) idWidth() int {
	switch r.Subtype {
	case SubtypeEnableSubSchedules, SubtypeDisableSubSchedules:
		return r.Profile.SubScheduleIDSize()
	default:
		return r.Profile.GroupIDSize()
	}
}

// Encode serializes the application data field.
func (r ScheduleIDListRequest) Encode() ([]byte, error) {
	if err := r.Profile.Validate(); err != nil {
		return nil, err
	}
	if _, ok := idListNames[r.Subtype]; !ok {
		return nil, ErrWrongMessageType
	}
	return encodeUintList(nil, r.IDs, r.Profile.ScheduleCountSize(), r.idWidth())
}

// DecodeScheduleIDListRequest parses TC[11,20], TC[11,21], TC[11,23],
// TC[11,24] or TC[11,25]. The subtype decides whether the IDs are
// sub-schedule IDs or group IDs, so it has to be supplied.
func DecodeScheduleIDListRequest(profile MissionProfile, subtype uint8, data []byte) (*ScheduleIDListRequest, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	request := &ScheduleIDListRequest{Profile: profile, Subtype: subtype}
	ids, used, err := readUintList(data, profile.ScheduleCountSize(), request.idWidth())
	if err != nil {
		return nil, err
	}
	if used != len(data) {
		return nil, ErrTrailingBytes
	}
	request.IDs = ids
	return request, nil
}

// Humanize returns a human-readable summary.
func (r ScheduleIDListRequest) Humanize() string {
	name, ok := idListNames[r.Subtype]
	if !ok {
		name = "unknown"
	}
	out := "PUS TC[11," + itoa(int(r.Subtype)) + "] " + name
	if r.IsAll() {
		return out + "\n  Scope ......... all (N = 0)"
	}
	return out + "\n  Scope ......... " + itoa(len(r.IDs)) + " named"
}

// ScheduleStatusEntry pairs an identifier with its status.
type ScheduleStatusEntry struct {
	// ID is a sub-schedule ID or a group ID, depending on the message.
	ID uint64
	// Status is the enabled or disabled state, per Tables 8-3 and 8-4.
	Status ScheduleStatus
}

// CreateSchedulingGroupsRequest is TC[11,22], per Figure 8-106: groups to
// create, each with the status it starts in.
type CreateSchedulingGroupsRequest struct {
	Profile MissionProfile
	// Groups are the groups to create and their initial statuses.
	Groups []ScheduleStatusEntry
}

// Key returns the message type.
func (CreateSchedulingGroupsRequest) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeCreateSchedulingGroups}
}

// Encode serializes the application data field.
func (r CreateSchedulingGroupsRequest) Encode() ([]byte, error) {
	return encodeStatusList(r.Profile, r.Groups, r.Profile.GroupIDSize())
}

// DecodeCreateSchedulingGroupsRequest parses TC[11,22].
func DecodeCreateSchedulingGroupsRequest(profile MissionProfile, data []byte) (*CreateSchedulingGroupsRequest, error) {
	entries, err := decodeStatusList(profile, data, profile.GroupIDSize())
	if err != nil {
		return nil, err
	}
	return &CreateSchedulingGroupsRequest{Profile: profile, Groups: entries}, nil
}

// Humanize returns a human-readable summary.
func (r CreateSchedulingGroupsRequest) Humanize() string {
	return "PUS TC[11,22] create time-based scheduling groups" + humanizeStatusList(r.Groups, "Group")
}

// SubScheduleStatusReport is TM[11,19], per Figure 8-103.
type SubScheduleStatusReport struct {
	Profile MissionProfile
	// SubSchedules are the sub-schedules and their statuses.
	SubSchedules []ScheduleStatusEntry
}

// Key returns the message type.
func (SubScheduleStatusReport) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeSubScheduleStatusReport}
}

// Encode serializes the source data field.
func (r SubScheduleStatusReport) Encode() ([]byte, error) {
	return encodeStatusList(r.Profile, r.SubSchedules, r.Profile.SubScheduleIDSize())
}

// DecodeSubScheduleStatusReport parses TM[11,19].
func DecodeSubScheduleStatusReport(profile MissionProfile, data []byte) (*SubScheduleStatusReport, error) {
	entries, err := decodeStatusList(profile, data, profile.SubScheduleIDSize())
	if err != nil {
		return nil, err
	}
	return &SubScheduleStatusReport{Profile: profile, SubSchedules: entries}, nil
}

// Humanize returns a human-readable summary.
func (r SubScheduleStatusReport) Humanize() string {
	return "PUS TM[11,19] time-based sub-schedule status report" +
		humanizeStatusList(r.SubSchedules, "Sub-schedule")
}

// SchedulingGroupStatusReport is TM[11,27], per Figure 8-110.
type SchedulingGroupStatusReport struct {
	Profile MissionProfile
	// Groups are the scheduling groups and their statuses.
	Groups []ScheduleStatusEntry
}

// Key returns the message type.
func (SchedulingGroupStatusReport) Key() MessageKey {
	return MessageKey{Service: ServiceTimeBasedScheduling, Subtype: SubtypeSchedulingGroupStatusRept}
}

// Encode serializes the source data field.
func (r SchedulingGroupStatusReport) Encode() ([]byte, error) {
	return encodeStatusList(r.Profile, r.Groups, r.Profile.GroupIDSize())
}

// DecodeSchedulingGroupStatusReport parses TM[11,27].
func DecodeSchedulingGroupStatusReport(profile MissionProfile, data []byte) (*SchedulingGroupStatusReport, error) {
	entries, err := decodeStatusList(profile, data, profile.GroupIDSize())
	if err != nil {
		return nil, err
	}
	return &SchedulingGroupStatusReport{Profile: profile, Groups: entries}, nil
}

// Humanize returns a human-readable summary.
func (r SchedulingGroupStatusReport) Humanize() string {
	return "PUS TM[11,27] time-based scheduling group status report" +
		humanizeStatusList(r.Groups, "Group")
}

// encodeStatusList serializes a count-prefixed list of ID and status pairs,
// the shape shared by TC[11,22], TM[11,19] and TM[11,27].
func encodeStatusList(p MissionProfile, entries []ScheduleStatusEntry, idWidth int) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out, err := putUint(nil, uint64(len(entries)), p.ScheduleCountSize())
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if out, err = putUint(out, entry.ID, idWidth); err != nil {
			return nil, err
		}
		if out, err = putUint(out, uint64(entry.Status), p.ScheduleStatusSize()); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// decodeStatusList parses a count-prefixed list of ID and status pairs.
func decodeStatusList(p MissionProfile, data []byte, idWidth int) ([]ScheduleStatusEntry, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	countWidth := p.ScheduleCountSize()
	count, err := readUint(data, countWidth)
	if err != nil {
		return nil, err
	}
	offset := countWidth

	var entries []ScheduleStatusEntry
	if count > 0 {
		width := idWidth + p.ScheduleStatusSize()
		if uint64(len(data)-offset)/uint64(width) < count {
			return nil, ErrDataTooShort
		}
		entries = make([]ScheduleStatusEntry, 0, count)
	}

	for i := uint64(0); i < count; i++ {
		id, err := readUint(data[offset:], idWidth)
		if err != nil {
			return nil, err
		}
		offset += idWidth

		status, err := readUint(data[offset:], p.ScheduleStatusSize())
		if err != nil {
			return nil, err
		}
		offset += p.ScheduleStatusSize()

		entries = append(entries, ScheduleStatusEntry{ID: id, Status: ScheduleStatus(status)})
	}

	if offset != len(data) {
		return nil, ErrTrailingBytes
	}
	return entries, nil
}

// humanizeStatusList renders an ID and status list.
func humanizeStatusList(entries []ScheduleStatusEntry, label string) string {
	out := "\n  Entries ....... " + itoa(len(entries))
	for _, entry := range entries {
		out += "\n    " + label + " " + itoa(int(entry.ID)) + ": " + entry.Status.String()
	}
	return out
}

// registerST11 adds the ST[11] codecs to a registry.
func registerST11(r *Registry) error {
	for _, subtype := range scheduleControlSubtypes {
		key := MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype}
		want := subtype
		if err := r.RegisterRequest(key, func(_ MissionProfile, data []byte) (Request, error) {
			if len(data) != 0 {
				return nil, ErrTrailingBytes
			}
			return ScheduleControlRequest{Subtype: want}, nil
		}); err != nil {
			return err
		}
	}

	for subtype := range requestIDListNames {
		want := subtype
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype},
			func(p MissionProfile, data []byte) (Request, error) {
				return DecodeScheduleRequestIDListRequest(p, want, data)
			},
		); err != nil {
			return err
		}
	}

	for subtype := range filterNames {
		want := subtype
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype},
			func(p MissionProfile, data []byte) (Request, error) {
				return DecodeScheduleFilterRequest(p, want, data)
			},
		); err != nil {
			return err
		}
	}

	for subtype := range idListNames {
		want := subtype
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype},
			func(p MissionProfile, data []byte) (Request, error) {
				return DecodeScheduleIDListRequest(p, want, data)
			},
		); err != nil {
			return err
		}
	}

	requests := map[uint8]RequestDecoder{
		SubtypeInsertActivities: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeInsertActivitiesRequest(p, data)
		},
		SubtypeTimeShiftByRequestID: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeTimeShiftByRequestIDRequest(p, data)
		},
		SubtypeTimeShiftByFilter: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeTimeShiftByFilterRequest(p, data)
		},
		SubtypeTimeShiftAll: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeTimeShiftAllRequest(p, data)
		},
		SubtypeCreateSchedulingGroups: func(p MissionProfile, data []byte) (Request, error) {
			return DecodeCreateSchedulingGroupsRequest(p, data)
		},
	}
	for subtype, decoder := range requests {
		if err := r.RegisterRequest(
			MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype}, decoder,
		); err != nil {
			return err
		}
	}

	reports := map[uint8]ReportDecoder{
		SubtypeScheduleDetailReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeScheduleDetailReport(p, data)
		},
		SubtypeScheduleSummaryReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeScheduleSummaryReport(p, data)
		},
		SubtypeSubScheduleStatusReport: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeSubScheduleStatusReport(p, data)
		},
		SubtypeSchedulingGroupStatusRept: func(p MissionProfile, data []byte) (Report, error) {
			return DecodeSchedulingGroupStatusReport(p, data)
		},
	}
	for subtype, decoder := range reports {
		if err := r.RegisterReport(
			MessageKey{Service: ServiceTimeBasedScheduling, Subtype: subtype}, decoder,
		); err != nil {
			return err
		}
	}
	return nil
}
