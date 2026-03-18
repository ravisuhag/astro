package tcf

import (
	"bytes"
	"testing"
	"time"
)

func TestCCSNewDayOfYearVariant(t *testing.T) {
	testTime := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)

	ccs, err := NewCCS(testTime)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	if ccs.Year != 2024 {
		t.Errorf("expected Year=2024, got %d", ccs.Year)
	}
	if ccs.DayOfYear != uint16(testTime.YearDay()) {
		t.Errorf("expected DayOfYear=%d, got %d", testTime.YearDay(), ccs.DayOfYear)
	}
	if ccs.Hour != 14 {
		t.Errorf("expected Hour=14, got %d", ccs.Hour)
	}
	if ccs.Minute != 30 {
		t.Errorf("expected Minute=30, got %d", ccs.Minute)
	}
	if ccs.Second != 45 {
		t.Errorf("expected Second=45, got %d", ccs.Second)
	}
	if ccs.MonthDay {
		t.Error("expected Day-of-Year variant")
	}
}

func TestCCSMonthDayVariant(t *testing.T) {
	testTime := time.Date(2024, 7, 20, 8, 15, 30, 0, time.UTC)

	ccs, err := NewCCS(testTime, WithCCSMonthDay())
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	if !ccs.MonthDay {
		t.Error("expected Month-Day variant")
	}
	if ccs.Month != 7 {
		t.Errorf("expected Month=7, got %d", ccs.Month)
	}
	if ccs.DayOfMonth != 20 {
		t.Errorf("expected DayOfMonth=20, got %d", ccs.DayOfMonth)
	}
}

func TestCCSBCDEncoding(t *testing.T) {
	// Verify BCD encoding: year 2024 should encode as 0x20 0x24
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	ccs, err := NewCCS(testTime)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	encoded, err := ccs.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// P-field is 1 byte, then T-field starts at index 1
	// Year should be BCD: 2024 → 0x20 0x24
	if encoded[1] != 0x20 || encoded[2] != 0x24 {
		t.Errorf("expected BCD year 0x20 0x24, got 0x%02X 0x%02X", encoded[1], encoded[2])
	}
}

func TestCCSBCDDayOfYear(t *testing.T) {
	// Day 75 (March 15 in 2024 leap year) should encode as BCD in DOY format
	testTime := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	ccs, err := NewCCS(testTime)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	if ccs.DayOfYear != 75 {
		t.Fatalf("expected DayOfYear=75, got %d", ccs.DayOfYear)
	}

	encoded, err := ccs.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// DOY 75: upper 4 bits zero, then BCD 075 → byte[0]=0x00, byte[1]=0x75
	doyBytes := encoded[3:5] // after 1-byte P-field and 2-byte year
	if doyBytes[0] != 0x00 || doyBytes[1] != 0x75 {
		t.Errorf("expected BCD DOY 0x00 0x75, got 0x%02X 0x%02X", doyBytes[0], doyBytes[1])
	}
}

func TestCCSBCDHour(t *testing.T) {
	// Hour 14 should encode as BCD 0x14, not binary 0x0E
	testTime := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)

	ccs, err := NewCCS(testTime)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	encoded, err := ccs.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// P(1) + Year(2) + DOY(2) + Hour(1) = index 5
	hourByte := encoded[5]
	if hourByte != 0x14 {
		t.Errorf("expected BCD hour 0x14, got 0x%02X", hourByte)
	}
}

func TestCCSWithSubSeconds(t *testing.T) {
	// 0.123456 seconds → sub-second octets: 12, 34, 56 (BCD pairs)
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 123456000, time.UTC)

	ccs, err := NewCCS(testTime, WithCCSSubSecBytes(3))
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	if ccs.SubSecBytes != 3 {
		t.Errorf("expected SubSecBytes=3, got %d", ccs.SubSecBytes)
	}
	if ccs.SubSecond[0] != 12 {
		t.Errorf("expected SubSecond[0]=12, got %d", ccs.SubSecond[0])
	}
	if ccs.SubSecond[1] != 34 {
		t.Errorf("expected SubSecond[1]=34, got %d", ccs.SubSecond[1])
	}
	if ccs.SubSecond[2] != 56 {
		t.Errorf("expected SubSecond[2]=56, got %d", ccs.SubSecond[2])
	}
}

func TestCCSEncodeDecodeRoundTrip(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 12, 30, 45, 500000000, time.UTC)

	ccs, err := NewCCS(testTime, WithCCSSubSecBytes(1))
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	encoded, err := ccs.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCCS(encoded)
	if err != nil {
		t.Fatalf("DecodeCCS failed: %v", err)
	}

	if decoded.Year != ccs.Year {
		t.Errorf("expected Year=%d, got %d", ccs.Year, decoded.Year)
	}
	if decoded.DayOfYear != ccs.DayOfYear {
		t.Errorf("expected DayOfYear=%d, got %d", ccs.DayOfYear, decoded.DayOfYear)
	}
	if decoded.Hour != ccs.Hour {
		t.Errorf("expected Hour=%d, got %d", ccs.Hour, decoded.Hour)
	}

	// Re-encode and compare bytes
	reEncoded, err := decoded.Encode()
	if err != nil {
		t.Fatalf("Re-encode failed: %v", err)
	}

	if !bytes.Equal(encoded, reEncoded) {
		t.Error("round-trip encode produced different bytes")
	}
}

func TestCCSMonthDayEncodeDecodeRoundTrip(t *testing.T) {
	testTime := time.Date(2024, 12, 25, 18, 0, 0, 0, time.UTC)

	ccs, err := NewCCS(testTime, WithCCSMonthDay())
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	encoded, err := ccs.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCCS(encoded)
	if err != nil {
		t.Fatalf("DecodeCCS failed: %v", err)
	}

	if !decoded.MonthDay {
		t.Error("expected Month-Day variant after decode")
	}
	if decoded.Month != 12 {
		t.Errorf("expected Month=12, got %d", decoded.Month)
	}
	if decoded.DayOfMonth != 25 {
		t.Errorf("expected DayOfMonth=25, got %d", decoded.DayOfMonth)
	}
}

func TestCCSCalendarVariationFlag(t *testing.T) {
	// Per §3.4.2: bit 4 = 0 means Month/Day, bit 4 = 1 means Day-of-Year
	testTime := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)

	// Day-of-Year variant: bit 4 should be 1
	doy, _ := NewCCS(testTime)
	if (doy.PField.Detail>>3)&0x01 != 1 {
		t.Error("Day-of-Year: expected bit 4 = 1")
	}

	// Month/Day variant: bit 4 should be 0
	md, _ := NewCCS(testTime, WithCCSMonthDay())
	if (md.PField.Detail>>3)&0x01 != 0 {
		t.Error("Month/Day: expected bit 4 = 0")
	}
}

func TestCCSTimeConversion(t *testing.T) {
	original := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)

	ccs, err := NewCCS(original)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	result := ccs.Time()
	if !result.Equal(original) {
		t.Errorf("expected %v, got %v", original, result)
	}
}

func TestCCSMonthDayTimeConversion(t *testing.T) {
	original := time.Date(2024, 7, 20, 8, 15, 30, 0, time.UTC)

	ccs, err := NewCCS(original, WithCCSMonthDay())
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	result := ccs.Time()
	if !result.Equal(original) {
		t.Errorf("expected %v, got %v", original, result)
	}
}

func TestCCSValidation(t *testing.T) {
	tests := []struct {
		name string
		ccs  CCS
	}{
		{"invalid hour", CCS{DayOfYear: 1, Hour: 24, SubSecBytes: 0}},
		{"invalid minute", CCS{DayOfYear: 1, Minute: 60, SubSecBytes: 0}},
		{"invalid second", CCS{DayOfYear: 1, Second: 61, SubSecBytes: 0}},
		{"invalid month", CCS{MonthDay: true, Month: 13, DayOfMonth: 1, SubSecBytes: 0}},
		{"invalid day", CCS{MonthDay: true, Month: 1, DayOfMonth: 0, SubSecBytes: 0}},
		{"invalid doy", CCS{DayOfYear: 367, SubSecBytes: 0}},
		{"invalid subsec bytes", CCS{DayOfYear: 1, SubSecBytes: 7}},
		{"invalid bcd digit", CCS{DayOfYear: 1, SubSecBytes: 1, SubSecond: [6]uint8{100}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ccs.Validate(); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCCSHumanize(t *testing.T) {
	testTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	ccs, err := NewCCS(testTime)
	if err != nil {
		t.Fatalf("NewCCS failed: %v", err)
	}

	h := ccs.Humanize()
	if len(h) == 0 {
		t.Error("Humanize returned empty string")
	}
}

func TestCCSDataTooShort(t *testing.T) {
	_, err := DecodeCCS([]byte{0x50})
	if err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestCCSInvalidTimeCodeID(t *testing.T) {
	// P-field with CUC time code ID (001)
	data := []byte{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := DecodeCCS(data)
	if err != ErrInvalidTimeCodeID {
		t.Errorf("expected ErrInvalidTimeCodeID, got %v", err)
	}
}
