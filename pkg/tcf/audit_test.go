package tcf

// Conformance regression tests for the CCSDS 301.0-B-4 time codes. The
// TCF-n tags below label the individual defects each test pins down.

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// TCF-1: CUC Level 1 counts true TAI seconds since the 1958 TAI epoch.

func TestTAIUTCOffsetAtTable(t *testing.T) {
	cases := []struct {
		when time.Time
		want int64
	}{
		{time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{time.Date(1971, 12, 31, 23, 59, 59, 0, time.UTC), 0},
		{time.Date(1972, 1, 1, 0, 0, 0, 0, time.UTC), 10},
		{time.Date(1972, 6, 30, 23, 59, 59, 0, time.UTC), 10},
		{time.Date(1972, 7, 1, 0, 0, 0, 0, time.UTC), 11},
		{time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), 32},
		{time.Date(2015, 7, 1, 0, 0, 0, 0, time.UTC), 36},
		{time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC), 36},
		{time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC), 37},
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 37},
	}
	for _, tc := range cases {
		if got := TAIUTCOffsetAt(tc.when); got != tc.want {
			t.Errorf("TAIUTCOffsetAt(%s) = %d, want %d", tc.when, got, tc.want)
		}
	}
}

func TestCUCLevel1TAILeapSeconds(t *testing.T) {
	// 2017-01-01T00:00:00 UTC is 1861920000 UTC-arithmetic seconds after
	// 1958-01-01; TAI is 37 s ahead there.
	t0 := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	c, err := NewCUC(t0)
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	if want := uint64(1861920037); c.CoarseTime != want {
		t.Errorf("CoarseTime = %d, want %d (TAI seconds)", c.CoarseTime, want)
	}
	if !c.Time().Equal(t0) {
		t.Errorf("round trip = %v, want %v", c.Time(), t0)
	}

	// Just before the 2017 leap second, the offset is still 36.
	t1 := time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC)
	c1, err := NewCUC(t1)
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	if want := uint64(1861919999 + 36); c1.CoarseTime != want {
		t.Errorf("CoarseTime = %d, want %d", c1.CoarseTime, want)
	}
	if !c1.Time().Equal(t1) {
		t.Errorf("round trip = %v, want %v", c1.Time(), t1)
	}

	// Pre-1972 instants use offset 0 (documented behavior).
	t2 := time.Date(1960, 6, 1, 0, 0, 0, 0, time.UTC)
	c2, err := NewCUC(t2)
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	secs, _ := epochDelta(t2, CCSDSEpoch)
	if c2.CoarseTime != uint64(secs) {
		t.Errorf("pre-1972 CoarseTime = %d, want %d (no offset)", c2.CoarseTime, secs)
	}
	if !c2.Time().Equal(t2) {
		t.Errorf("pre-1972 round trip = %v, want %v", c2.Time(), t2)
	}
}

func TestCUCLevel1LeapSecondInstantCollapses(t *testing.T) {
	// TAI second 1861920036 since epoch is the inserted leap second
	// (UTC 23:59:60 on 2016-12-31). Go cannot represent it; it must decode
	// as the following 00:00:00 UTC, not silently shift other instants.
	c := &CUC{CoarseTime: 1861920036, CoarseBytes: 4, Epoch: CCSDSEpoch}
	want := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	if !c.Time().Equal(want) {
		t.Errorf("leap-second instant = %v, want %v", c.Time(), want)
	}
	// The neighbors decode unambiguously.
	before := &CUC{CoarseTime: 1861920035, CoarseBytes: 4, Epoch: CCSDSEpoch}
	if want := time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC); !before.Time().Equal(want) {
		t.Errorf("second before leap = %v, want %v", before.Time(), want)
	}
	after := &CUC{CoarseTime: 1861920038, CoarseBytes: 4, Epoch: CCSDSEpoch}
	if want := time.Date(2017, 1, 1, 0, 0, 1, 0, time.UTC); !after.Time().Equal(want) {
		t.Errorf("second after leap = %v, want %v", after.Time(), want)
	}
}

func TestCUCLevel2PurelyArithmetic(t *testing.T) {
	// A Level 2 epoch spanning the 2016-12-31 leap second must NOT pick up
	// a leap correction: exactly 2*86400 arithmetic seconds elapse.
	epoch := time.Date(2016, 12, 31, 0, 0, 0, 0, time.UTC)
	c, err := NewCUC(time.Date(2017, 1, 2, 0, 0, 0, 0, time.UTC), WithCUCEpoch(epoch))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	if want := uint64(2 * 86400); c.CoarseTime != want {
		t.Errorf("Level 2 CoarseTime = %d, want %d (no leap correction)", c.CoarseTime, want)
	}
}

// TCF-15: epoch comparison must ignore location and monotonic readings.

func TestCUCEpochComparisonIgnoresLocation(t *testing.T) {
	// The same instant as CCSDSEpoch, expressed in a non-UTC zone, is still
	// the CCSDS epoch: the code must classify it as Level 1.
	zone := time.FixedZone("EST", -5*3600)
	epoch := time.Date(1957, 12, 31, 19, 0, 0, 0, zone) // == 1958-01-01T00:00:00Z
	c, err := NewCUC(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), WithCUCEpoch(epoch))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	if c.PField.TimeCodeID != TimeCodeCUCLevel1 {
		t.Errorf("TimeCodeID = %d, want Level 1 for zone-shifted CCSDS epoch", c.PField.TimeCodeID)
	}
}

// TCF-5: fine octets up to the spec's 10, with >64-bit wire fidelity.

func TestCUCTenFineOctets(t *testing.T) {
	testTime := CCSDSEpoch.Add(100*time.Second + 123456789*time.Nanosecond)
	c, err := NewCUC(testTime, WithCUCFineBytes(10))
	if err != nil {
		t.Fatalf("NewCUC with 10 fine octets failed: %v", err)
	}

	encoded, err := c.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// P-field(2) + coarse(4) + fine(10)
	if len(encoded) != 16 {
		t.Fatalf("encoded length = %d, want 16", len(encoded))
	}

	decoded, err := DecodeCUC(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
	}
	if decoded.FineBytes != 10 {
		t.Errorf("FineBytes = %d, want 10", decoded.FineBytes)
	}
	reEncoded, err := decoded.Encode()
	if err != nil {
		t.Fatalf("Re-encode failed: %v", err)
	}
	if !bytes.Equal(encoded, reEncoded) {
		t.Error("10-fine-octet round trip produced different bytes")
	}
	if diff := c.Time().Sub(decoded.Time()); diff != 0 {
		t.Errorf("time round trip differs by %v", diff)
	}
}

func TestCUCFineTimeExtWireFidelity(t *testing.T) {
	// Hand-craft a T-field whose least significant fine octets (9-10) are
	// nonzero: they must survive a decode/encode round trip via FineTimeExt.
	tf := []byte{
		0x00, 0x00, 0x00, 0x2A, // coarse = 42
		0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // fine octets 1-8
		0xAB, 0xCD, // fine octets 9-10
	}
	c, err := DecodeCUCTField(tf, 4, 10, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUCTField failed: %v", err)
	}
	if c.FineTime != 0x8000000000000000 {
		t.Errorf("FineTime = 0x%016X, want 0x8000000000000000", c.FineTime)
	}
	if c.FineTimeExt != 0xABCD {
		t.Errorf("FineTimeExt = 0x%04X, want 0xABCD", c.FineTimeExt)
	}
	out, err := c.EncodeTField()
	if err != nil {
		t.Fatalf("EncodeTField failed: %v", err)
	}
	if !bytes.Equal(out, tf) {
		t.Errorf("T-field round trip: got % 02X, want % 02X", out, tf)
	}
}

func TestCUCFineBytesCap(t *testing.T) {
	if _, err := NewCUC(CCSDSEpoch.Add(time.Second), WithCUCFineBytes(11)); !errors.Is(err, ErrInvalidFineOctets) {
		t.Errorf("FineBytes=11: err = %v, want ErrInvalidFineOctets", err)
	}
}

// TCF-6: reserved and further-extension P-field bits must be rejected.

func TestPFieldFurtherExtensionBitRejected(t *testing.T) {
	// First octet: extension set, CUC Level 1. Second octet: bit 0 set
	// (a third P-field octet is not defined by CCSDS 301.0-B-4).
	var p PField
	if err := p.Decode([]byte{0x9C, 0x84}); !errors.Is(err, ErrInvalidPField) {
		t.Errorf("err = %v, want ErrInvalidPField", err)
	}
}

func TestCUCReservedExtensionBitsRejected(t *testing.T) {
	// Valid extended CUC P-field except reserved bits 6-7 of octet 2 set.
	data := []byte{0x9C, 0x21, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := DecodeCUC(data, time.Time{}); !errors.Is(err, ErrInvalidPField) {
		t.Errorf("err = %v, want ErrInvalidPField for reserved ext bits", err)
	}
}

// TCF-7: reserved CDS sub-millisecond code '11'.

func TestCDSReservedSubmsCodeRejected(t *testing.T) {
	// P-field 0x43: CDS, Level 1, 16-bit day, sub-ms code '11' (reserved).
	data := []byte{0x43, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00}
	if _, err := DecodeCDS(data, time.Time{}); !errors.Is(err, ErrReservedSubmsCode) {
		t.Errorf("err = %v, want ErrReservedSubmsCode", err)
	}
}

// TCF-8: CDS sub-millisecond range checks.

func TestCDSSubmsRangeChecked(t *testing.T) {
	// 16-bit field is microseconds 0-999: 1000 must be rejected.
	c := &CDS{DayBytes: 2, SubmsBytes: 2, Submilliseconds: 1000}
	if err := c.Validate(); !errors.Is(err, ErrInvalidSubmilliseconds) {
		t.Errorf("us=1000: err = %v, want ErrInvalidSubmilliseconds", err)
	}
	// 32-bit field is picoseconds 0-999999999.
	c = &CDS{DayBytes: 2, SubmsBytes: 4, Submilliseconds: 1000000000}
	if err := c.Validate(); !errors.Is(err, ErrInvalidSubmilliseconds) {
		t.Errorf("ps=1e9: err = %v, want ErrInvalidSubmilliseconds", err)
	}
	// On the wire too: 0x03E8 = 1000 microseconds.
	data := []byte{0x41, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x01, 0x03, 0xE8}
	if _, err := DecodeCDS(data, time.Time{}); !errors.Is(err, ErrInvalidSubmilliseconds) {
		t.Errorf("decode us=1000: err = %v, want ErrInvalidSubmilliseconds", err)
	}
	// Boundary values are accepted.
	c = &CDS{DayBytes: 2, SubmsBytes: 2, Submilliseconds: 999}
	if err := c.Validate(); err != nil {
		t.Errorf("us=999: unexpected err %v", err)
	}
}

func TestCDSExtensionFlagRejected(t *testing.T) {
	// CDS P-field is a single octet; a set extension flag is invalid.
	data := []byte{0xC0, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x01}
	if _, err := DecodeCDS(data, time.Time{}); !errors.Is(err, ErrInvalidPField) {
		t.Errorf("err = %v, want ErrInvalidPField", err)
	}
}

// TCF-10: non-BCD nibbles must be rejected on CCS decode.

func TestCCSNonBCDNibbleRejected(t *testing.T) {
	base, err := NewCCS(time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}
	encoded, err := base.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// Corrupt each T-field octet with a non-decimal nibble in turn.
	for i := 1; i < len(encoded); i++ {
		bad := append([]byte{}, encoded...)
		bad[i] = 0x1A
		if _, err := DecodeCCS(bad); !errors.Is(err, ErrInvalidBCD) {
			t.Errorf("octet %d = 0x1A: err = %v, want ErrInvalidBCD", i, err)
		}
	}
	// The reserved upper nibble of the first DOY octet must be zero.
	bad := append([]byte{}, encoded...)
	bad[3] = 0x10 // DOY hundreds octet: upper nibble must be 0
	if _, err := DecodeCCS(bad); !errors.Is(err, ErrInvalidBCD) {
		t.Errorf("DOY reserved nibble: err = %v, want ErrInvalidBCD", err)
	}
}

// TCF-11: second=60 is an explicit, flagged leap second.

func TestCCSLeapSecondFlagged(t *testing.T) {
	c := &CCS{Year: 2016, DayOfYear: 366, Hour: 23, Minute: 59, Second: 60}
	if err := c.Validate(); err != nil {
		t.Fatalf("leap second at 23:59:60 must validate, got %v", err)
	}
	if !c.IsLeapSecond() {
		t.Error("IsLeapSecond() = false, want true")
	}
	// Documented normalization: 23:59:60 -> next day 00:00:00.
	want := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	if !c.Time().Equal(want) {
		t.Errorf("Time() = %v, want %v", c.Time(), want)
	}
	// Second 60 anywhere but 23:59 is invalid.
	bad := &CCS{Year: 2016, DayOfYear: 100, Hour: 12, Minute: 0, Second: 60}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidCalendarTime) {
		t.Errorf("second=60 at 12:00: err = %v, want ErrInvalidCalendarTime", err)
	}
}

// TCF-12: year and calendar cross-checks.

func TestCCSCalendarCrossChecks(t *testing.T) {
	cases := []struct {
		name string
		ccs  CCS
		ok   bool
	}{
		{"year over 9999", CCS{Year: 10000, DayOfYear: 1}, false},
		{"feb 31", CCS{Year: 2024, MonthDay: true, Month: 2, DayOfMonth: 31}, false},
		{"feb 29 non-leap", CCS{Year: 2023, MonthDay: true, Month: 2, DayOfMonth: 29}, false},
		{"feb 29 leap", CCS{Year: 2024, MonthDay: true, Month: 2, DayOfMonth: 29}, true},
		{"apr 31", CCS{Year: 2024, MonthDay: true, Month: 4, DayOfMonth: 31}, false},
		{"doy 366 non-leap", CCS{Year: 2023, DayOfYear: 366}, false},
		{"doy 366 leap", CCS{Year: 2024, DayOfYear: 366}, true},
		{"doy 365 non-leap", CCS{Year: 2023, DayOfYear: 365}, true},
	}
	for _, tc := range cases {
		err := tc.ccs.Validate()
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}

// TCF-13: strict fixed-width ASCII decode.

func TestASCIIStrictDecodeRejects(t *testing.T) {
	a, _ := NewASCIITime(ASCIITypeA)
	badA := []string{
		"2024-1-01T00:00:00",             // month not 2 digits
		"2024-01-1T00:00:00",             // day not 2 digits
		"24-01-01T00:00:00",              // year not 4 digits
		"2024-01-01T0:00:00",             // hour not 2 digits
		"2024-01-01T00:00:00.",           // empty fraction
		"2024-01-01T00:00:00.12a",        // non-digit fraction
		"2024-01-01T00:00:00.1234567890", // >9 fraction digits
		"+024-01-01T00:00:00",            // sign accepted by Atoi, not by spec
		"2024-13-01T00:00:00",            // month 13
		"2024-02-30T00:00:00",            // Feb 30
		"2023-02-29T00:00:00",            // Feb 29 in non-leap year
		"2024-01-00T00:00:00",            // day 0
		"2024-01-01T24:00:00",            // hour 24
		"2024-01-01T00:60:00",            // minute 60
		"2024-01-01T12:00:60",            // second 60 outside 23:59
		"2024-01-01 00:00:00",            // missing T
		" 2024-01-01T00:00:00",           // leading space
	}
	for _, s := range badA {
		if _, err := a.Decode(s); !errors.Is(err, ErrInvalidASCIIFormat) {
			t.Errorf("Type A Decode(%q): err = %v, want ErrInvalidASCIIFormat", s, err)
		}
	}

	b, _ := NewASCIITime(ASCIITypeB)
	badB := []string{
		"2024-01-01T00:00:00", // Type A shape
		"2024-75T00:00:00",    // DOY not 3 digits
		"2023-366T00:00:00",   // DOY 366 in non-leap year
		"2024-000T00:00:00",   // DOY 0
		"2024-367T00:00:00",   // DOY 367
	}
	for _, s := range badB {
		if _, err := b.Decode(s); !errors.Is(err, ErrInvalidASCIIFormat) {
			t.Errorf("Type B Decode(%q): err = %v, want ErrInvalidASCIIFormat", s, err)
		}
	}
}

func TestASCIILeapSecondDecode(t *testing.T) {
	a, _ := NewASCIITime(ASCIITypeA)
	got, err := a.Decode("2016-12-31T23:59:60Z")
	if err != nil {
		t.Fatalf("Decode leap second failed: %v", err)
	}
	// Documented normalization: Go cannot represent second 60.
	want := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Decode = %v, want %v", got, want)
	}
}

// TCF-14: T-field-only (implicit P-field) APIs.

func TestCUCTFieldOnlyRoundTrip(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 12, 30, 0, 500000000, time.UTC)
	c, err := NewCUC(testTime, WithCUCFineBytes(2))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	tf, err := c.EncodeTField()
	if err != nil {
		t.Fatalf("EncodeTField failed: %v", err)
	}
	if len(tf) != 6 { // 4 coarse + 2 fine, no P-field
		t.Fatalf("T-field length = %d, want 6", len(tf))
	}
	decoded, err := DecodeCUCTField(tf, 4, 2, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUCTField failed: %v", err)
	}
	if decoded.CoarseTime != c.CoarseTime || decoded.FineTime != c.FineTime {
		t.Errorf("round trip mismatch: got (%d,%d), want (%d,%d)",
			decoded.CoarseTime, decoded.FineTime, c.CoarseTime, c.FineTime)
	}
	if !decoded.Time().Equal(c.Time()) {
		t.Errorf("Time = %v, want %v", decoded.Time(), c.Time())
	}

	// Level 2 with explicit epoch.
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	c2, err := NewCUC(epoch.Add(42*time.Second), WithCUCEpoch(epoch), WithCUCCoarseBytes(2))
	if err != nil {
		t.Fatalf("NewCUC Level 2 failed: %v", err)
	}
	tf2, err := c2.EncodeTField()
	if err != nil {
		t.Fatalf("EncodeTField failed: %v", err)
	}
	d2, err := DecodeCUCTField(tf2, 2, 0, epoch)
	if err != nil {
		t.Fatalf("DecodeCUCTField failed: %v", err)
	}
	if d2.CoarseTime != 42 {
		t.Errorf("Level 2 CoarseTime = %d, want 42", d2.CoarseTime)
	}

	if _, err := DecodeCUCTField([]byte{0x01}, 4, 0, time.Time{}); !errors.Is(err, ErrDataTooShort) {
		t.Errorf("short data: err = %v, want ErrDataTooShort", err)
	}
	if _, err := DecodeCUCTField(tf, 0, 2, time.Time{}); !errors.Is(err, ErrInvalidCoarseOctets) {
		t.Errorf("coarse=0: err = %v, want ErrInvalidCoarseOctets", err)
	}
}

func TestCDSTFieldOnlyRoundTrip(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 12, 30, 0, 123456000, time.UTC)
	c, err := NewCDS(testTime, tfNoop(), WithCDSSubmsBytes(2))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}
	tf, err := c.EncodeTField()
	if err != nil {
		t.Fatalf("EncodeTField failed: %v", err)
	}
	if len(tf) != 8 { // 2 day + 4 ms + 2 subms
		t.Fatalf("T-field length = %d, want 8", len(tf))
	}
	decoded, err := DecodeCDSTField(tf, 2, 2, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCDSTField failed: %v", err)
	}
	if decoded.Day != c.Day || decoded.Milliseconds != c.Milliseconds || decoded.Submilliseconds != c.Submilliseconds {
		t.Errorf("round trip mismatch: got (%d,%d,%d), want (%d,%d,%d)",
			decoded.Day, decoded.Milliseconds, decoded.Submilliseconds,
			c.Day, c.Milliseconds, c.Submilliseconds)
	}
	if _, err := DecodeCDSTField(tf, 4, 2, time.Time{}); !errors.Is(err, ErrInvalidDaySegment) {
		t.Errorf("dayBytes=4: err = %v, want ErrInvalidDaySegment", err)
	}
}

// tfNoop returns a no-op option so the default 16-bit day is used explicitly.
func tfNoop() CDSOption {
	return func(*CDS) error { return nil }
}

func TestCCSTFieldOnlyRoundTrip(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 12, 30, 45, 120000000, time.UTC)
	c, err := NewCCS(testTime, WithCCSSubSecBytes(1))
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}
	tf, err := c.EncodeTField()
	if err != nil {
		t.Fatalf("EncodeTField failed: %v", err)
	}
	if len(tf) != 8 { // year(2)+doy(2)+h+m+s+subsec(1)
		t.Fatalf("T-field length = %d, want 8", len(tf))
	}
	decoded, err := DecodeCCSTField(tf, false, 1)
	if err != nil {
		t.Fatalf("DecodeCCSTField failed: %v", err)
	}
	if decoded.Year != c.Year || decoded.DayOfYear != c.DayOfYear ||
		decoded.Hour != c.Hour || decoded.SubSecond != c.SubSecond {
		t.Error("CCS T-field round trip mismatch")
	}
	if _, err := DecodeCCSTField(tf[:5], false, 1); !errors.Is(err, ErrDataTooShort) {
		t.Errorf("short data: err = %v, want ErrDataTooShort", err)
	}
}

// TCF-16: fine time truncates toward zero (documented, verified).

func TestCUCFineTimeTruncates(t *testing.T) {
	// 0.9999999 s with one fine octet: 0.9999999 * 256 = 255.99997...;
	// truncation gives 255, rounding would give 256 (overflow).
	testTime := CCSDSEpoch.Add(time.Second - 100*time.Nanosecond)
	c, err := NewCUC(testTime, WithCUCFineBytes(1))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}
	if c.CoarseTime != 0 || c.FineTime != 255 {
		t.Errorf("got coarse=%d fine=%d, want coarse=0 fine=255 (truncation)", c.CoarseTime, c.FineTime)
	}
}

// TCF-17: large counts must not overflow time.Duration.

func TestCUCLargeCoarseNoDurationOverflow(t *testing.T) {
	// 2^55 seconds is ~1.1 billion years: far beyond time.Duration's
	// ~292-year range, but valid in a 7-octet coarse field.
	c := &CUC{CoarseTime: 1 << 55, CoarseBytes: 7, Epoch: CCSDSEpoch}
	got := c.Time()
	want := time.Unix(CCSDSEpoch.Unix()+(1<<55)-37, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("Time() = %v, want %v", got, want)
	}
	if got.Before(time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("Time() went backwards: duration overflow")
	}
}

func TestCDSLargeDayNoDurationOverflow(t *testing.T) {
	// Max 24-bit day count is ~45,000 years: beyond time.Duration's range.
	c := &CDS{Day: 16777215, Milliseconds: 1000, DayBytes: 3, Epoch: CCSDSEpoch}
	got := c.Time()
	want := time.Unix(CCSDSEpoch.Unix()+int64(16777215)*86400+1, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("Time() = %v, want %v", got, want)
	}
	// And back: NewCDS on that instant reproduces the day count.
	rt, err := NewCDS(got, WithCDSDayBytes(3))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}
	if rt.Day != 16777215 {
		t.Errorf("round-trip Day = %d, want 16777215", rt.Day)
	}
}
