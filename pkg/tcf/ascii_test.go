package tcf

import (
	"testing"
	"time"
)

func TestASCIITypeAEncodeDecode(t *testing.T) {
	a, err := NewASCIITime(ASCIITypeA)
	if err != nil {
		t.Fatalf("NewASCIITime failed: %v", err)
	}

	testTime := time.Date(2024, 3, 15, 14, 30, 45, 123000000, time.UTC)

	encoded, err := a.Encode(testTime)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expected := "2024-03-15T14:30:45.123Z"
	if encoded != expected {
		t.Errorf("expected %q, got %q", expected, encoded)
	}

	decoded, err := a.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !decoded.Equal(testTime) {
		t.Errorf("expected %v, got %v", testTime, decoded)
	}
}

func TestASCIITypeBEncodeDecode(t *testing.T) {
	a, err := NewASCIITime(ASCIITypeB)
	if err != nil {
		t.Fatalf("NewASCIITime failed: %v", err)
	}

	testTime := time.Date(2024, 3, 15, 14, 30, 45, 123000000, time.UTC)

	encoded, err := a.Encode(testTime)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// March 15 in a leap year (2024) = day 75
	expected := "2024-075T14:30:45.123Z"
	if encoded != expected {
		t.Errorf("expected %q, got %q", expected, encoded)
	}

	decoded, err := a.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !decoded.Equal(testTime) {
		t.Errorf("expected %v, got %v", testTime, decoded)
	}
}

func TestASCIINoPrecision(t *testing.T) {
	a, err := NewASCIITime(ASCIITypeA, WithASCIIPrecision(0))
	if err != nil {
		t.Fatalf("NewASCIITime failed: %v", err)
	}

	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	encoded, err := a.Encode(testTime)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expected := "2024-01-01T00:00:00Z"
	if encoded != expected {
		t.Errorf("expected %q, got %q", expected, encoded)
	}

	decoded, err := a.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !decoded.Equal(testTime) {
		t.Errorf("expected %v, got %v", testTime, decoded)
	}
}

func TestASCIIHighPrecision(t *testing.T) {
	a, err := NewASCIITime(ASCIITypeA, WithASCIIPrecision(6))
	if err != nil {
		t.Fatalf("NewASCIITime failed: %v", err)
	}

	testTime := time.Date(2024, 6, 15, 12, 0, 0, 123456000, time.UTC)

	encoded, err := a.Encode(testTime)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expected := "2024-06-15T12:00:00.123456Z"
	if encoded != expected {
		t.Errorf("expected %q, got %q", expected, encoded)
	}

	decoded, err := a.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Should be within nanosecond precision
	diff := decoded.Sub(testTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Microsecond {
		t.Errorf("round-trip time difference too large: %v", diff)
	}
}

func TestASCIIOptionalZ(t *testing.T) {
	// Per §3.5.1.1: Z terminator is optional
	a, _ := NewASCIITime(ASCIITypeA, WithASCIIPrecision(0))

	// With Z
	decoded1, err := a.Decode("2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Decode with Z failed: %v", err)
	}

	// Without Z
	decoded2, err := a.Decode("2024-01-01T00:00:00")
	if err != nil {
		t.Fatalf("Decode without Z failed: %v", err)
	}

	if !decoded1.Equal(decoded2) {
		t.Errorf("results with and without Z differ: %v vs %v", decoded1, decoded2)
	}
}

func TestASCIIInvalidType(t *testing.T) {
	_, err := NewASCIITime("C")
	if err != ErrInvalidASCIIFormat {
		t.Errorf("expected ErrInvalidASCIIFormat, got %v", err)
	}
}

func TestASCIIDecodeInvalidFormat(t *testing.T) {
	a, _ := NewASCIITime(ASCIITypeA)

	tests := []string{
		"",
		"not-a-time",
		"2024-01-01 00:00:00Z", // Missing T
		"2024-01-01T00:00Z",    // Missing seconds
	}

	for _, s := range tests {
		if _, err := a.Decode(s); err != ErrInvalidASCIIFormat {
			t.Errorf("Decode(%q): expected ErrInvalidASCIIFormat, got %v", s, err)
		}
	}
}

func TestASCIITypeBDecodeInvalidFormat(t *testing.T) {
	a, _ := NewASCIITime(ASCIITypeB)

	// Type B expects YYYY-DDD, not YYYY-MM-DD
	if _, err := a.Decode("2024-01-01T00:00:00Z"); err != ErrInvalidASCIIFormat {
		t.Errorf("expected ErrInvalidASCIIFormat for Type B with Type A format, got %v", err)
	}
}

func TestASCIIInvalidPrecision(t *testing.T) {
	_, err := NewASCIITime(ASCIITypeA, WithASCIIPrecision(10))
	if err != ErrInvalidCalendarTime {
		t.Errorf("expected ErrInvalidCalendarTime, got %v", err)
	}

	_, err = NewASCIITime(ASCIITypeA, WithASCIIPrecision(-1))
	if err != ErrInvalidCalendarTime {
		t.Errorf("expected ErrInvalidCalendarTime, got %v", err)
	}
}
