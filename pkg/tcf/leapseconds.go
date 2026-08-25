package tcf

import "time"

// Leap-second handling for CUC Level 1 (CCSDS 1958 TAI epoch).
//
// TAI (International Atomic Time) is a continuous time scale with no leap
// seconds. UTC tracks Earth rotation by occasionally inserting a leap second,
// so TAI-UTC grows in whole-second steps. Since 1972-01-01 the offset is an
// integer number of seconds, starting at 10 s and reaching 37 s on
// 2017-01-01. These historical offsets are static facts and are embedded
// below. No leap second has been announced since; when the IERS announces a
// new one, append an entry to this table.
//
// Dates before 1972-01-01 are treated with a TAI-UTC offset of zero. Between
// 1958 and 1972 UTC used fractional "rubber-second" adjustments that cannot
// be represented as an integer offset; this package does not model them. See
// the package documentation for details.

// leapEntry records one step of the TAI-UTC offset.
type leapEntry struct {
	year, month int   // UTC date (first day of month) the offset took effect
	offset      int64 // TAI-UTC in whole seconds from that instant
}

// leapTable lists every integer TAI-UTC offset since 1972 (IERS Bulletin C).
// Entries are in ascending order of effective date.
var leapTable = [...]leapEntry{
	{1972, 1, 10},
	{1972, 7, 11},
	{1973, 1, 12},
	{1974, 1, 13},
	{1975, 1, 14},
	{1976, 1, 15},
	{1977, 1, 16},
	{1978, 1, 17},
	{1979, 1, 18},
	{1980, 1, 19},
	{1981, 7, 20},
	{1982, 7, 21},
	{1983, 7, 22},
	{1985, 7, 23},
	{1988, 1, 24},
	{1990, 1, 25},
	{1991, 1, 26},
	{1992, 7, 27},
	{1993, 7, 28},
	{1994, 7, 29},
	{1996, 1, 30},
	{1997, 7, 31},
	{1999, 1, 32},
	{2006, 1, 33},
	{2009, 1, 34},
	{2012, 7, 35},
	{2015, 7, 36},
	{2017, 1, 37},
}

// leapUnix holds the Unix seconds of each leap table effective date.
var leapUnix = func() [len(leapTable)]int64 {
	var u [len(leapTable)]int64
	for i, e := range leapTable {
		u[i] = time.Date(e.year, time.Month(e.month), 1, 0, 0, 0, 0, time.UTC).Unix()
	}
	return u
}()

// TAIUTCOffsetAt returns the TAI-UTC offset, in whole seconds, in effect at
// the given instant. The instant is interpreted on the UTC time scale.
//
// For instants before 1972-01-01T00:00:00Z the function returns 0: the
// pre-1972 fractional UTC adjustments are not modeled, and times in that era
// are treated as if TAI and UTC coincided.
//
// The embedded table is complete through the leap second of 2017-01-01
// (offset 37 s), which is still current. Append to the table when the IERS
// announces a new leap second.
func TAIUTCOffsetAt(t time.Time) int64 {
	u := t.Unix()
	var off int64
	for i := range leapTable {
		if u < leapUnix[i] {
			break
		}
		off = leapTable[i].offset
	}
	return off
}

// taiOffsetAtTAISeconds returns the TAI-UTC offset in effect for an elapsed
// count of TAI seconds since the CCSDS epoch (1958-01-01T00:00:00 TAI).
// Used when converting a decoded CUC Level 1 coarse count back to UTC.
func taiOffsetAtTAISeconds(taiSecs uint64) int64 {
	epochUnix := CCSDSEpoch.Unix()
	var off int64
	for i := range leapTable {
		// The boundary on the TAI scale: UTC effective instant plus the
		// offset that applies from that instant.
		boundary := leapUnix[i] - epochUnix + leapTable[i].offset
		if boundary < 0 || taiSecs < uint64(boundary) {
			break
		}
		off = leapTable[i].offset
	}
	return off
}
