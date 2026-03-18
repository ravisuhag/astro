package tcf

import (
	"bytes"
	"testing"
	"time"
)

func TestCDSNewAndEncodeDecode(t *testing.T) {
	// 10 days and 12 hours after CCSDS epoch
	testTime := CCSDSEpoch.Add(10*24*time.Hour + 12*time.Hour)

	cds, err := NewCDS(testTime)
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	if cds.Day != 10 {
		t.Errorf("expected Day=10, got %d", cds.Day)
	}

	expectedMs := uint32(12 * 3600 * 1000)
	if cds.Milliseconds != expectedMs {
		t.Errorf("expected Milliseconds=%d, got %d", expectedMs, cds.Milliseconds)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCDS(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCDS failed: %v", err)
	}

	if decoded.Day != cds.Day {
		t.Errorf("expected Day=%d, got %d", cds.Day, decoded.Day)
	}
	if decoded.Milliseconds != cds.Milliseconds {
		t.Errorf("expected Milliseconds=%d, got %d", cds.Milliseconds, decoded.Milliseconds)
	}
}

func TestCDSPFieldTimeCodeID(t *testing.T) {
	// Both Level 1 and Level 2 should use TimeCodeCDS (100)
	testTime := CCSDSEpoch.Add(1 * 24 * time.Hour)
	cds, err := NewCDS(testTime)
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}
	if cds.PField.TimeCodeID != TimeCodeCDS {
		t.Errorf("Level 1: expected TimeCodeID=%d, got %d", TimeCodeCDS, cds.PField.TimeCodeID)
	}

	customEpoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cds2, err := NewCDS(customEpoch.Add(1*24*time.Hour), WithCDSEpoch(customEpoch))
	if err != nil {
		t.Fatalf("NewCDS Level 2 failed: %v", err)
	}
	if cds2.PField.TimeCodeID != TimeCodeCDS {
		t.Errorf("Level 2: expected TimeCodeID=%d, got %d", TimeCodeCDS, cds2.PField.TimeCodeID)
	}
	// Level 2 should have epoch bit set (Detail bit 3)
	if (cds2.PField.Detail>>3)&0x01 != 1 {
		t.Error("Level 2: expected epoch bit (Detail bit 3) to be 1")
	}
}

func TestCDS24BitDay(t *testing.T) {
	// Day count > 65535 requires 24-bit day segment
	testTime := CCSDSEpoch.Add(70000 * 24 * time.Hour)

	cds, err := NewCDS(testTime, WithCDSDayBytes(3))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	if cds.Day != 70000 {
		t.Errorf("expected Day=70000, got %d", cds.Day)
	}
	if cds.DayBytes != 3 {
		t.Errorf("expected DayBytes=3, got %d", cds.DayBytes)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCDS(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCDS failed: %v", err)
	}

	if decoded.Day != 70000 {
		t.Errorf("expected Day=70000, got %d", decoded.Day)
	}
	if decoded.DayBytes != 3 {
		t.Errorf("expected DayBytes=3, got %d", decoded.DayBytes)
	}
}

func TestCDSWithMicroseconds(t *testing.T) {
	testTime := CCSDSEpoch.Add(1*24*time.Hour + 500*time.Microsecond)

	cds, err := NewCDS(testTime, WithCDSSubmsBytes(2))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	if cds.SubmsBytes != 2 {
		t.Errorf("expected SubmsBytes=2, got %d", cds.SubmsBytes)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCDS(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCDS failed: %v", err)
	}

	if decoded.SubmsBytes != 2 {
		t.Errorf("expected SubmsBytes=2, got %d", decoded.SubmsBytes)
	}

	// Round-trip time should be close
	diff := cds.Time().Sub(decoded.Time())
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Microsecond {
		t.Errorf("round-trip time difference too large: %v", diff)
	}
}

func TestCDSLevel2CustomEpoch(t *testing.T) {
	customEpoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	testTime := customEpoch.Add(5 * 24 * time.Hour)

	cds, err := NewCDS(testTime, WithCDSEpoch(customEpoch))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	if cds.PField.TimeCodeID != TimeCodeCDS {
		t.Errorf("expected TimeCodeCDS, got %d", cds.PField.TimeCodeID)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCDS(encoded, customEpoch)
	if err != nil {
		t.Fatalf("DecodeCDS failed: %v", err)
	}

	if decoded.Day != 5 {
		t.Errorf("expected Day=5, got %d", decoded.Day)
	}
}

func TestCDSLevel2RequiresEpoch(t *testing.T) {
	customEpoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	testTime := customEpoch.Add(1 * 24 * time.Hour)

	cds, err := NewCDS(testTime, WithCDSEpoch(customEpoch))
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	_, err = DecodeCDS(encoded, time.Time{})
	if err != ErrEpochRequired {
		t.Errorf("expected ErrEpochRequired, got %v", err)
	}
}

func TestCDSRoundTrip(t *testing.T) {
	testTime := CCSDSEpoch.Add(365*24*time.Hour + 12*time.Hour + 30*time.Minute + 45*time.Second + 123*time.Millisecond)

	cds, err := NewCDS(testTime)
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	encoded, err := cds.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCDS(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCDS failed: %v", err)
	}

	reEncoded, err := decoded.Encode()
	if err != nil {
		t.Fatalf("Re-encode failed: %v", err)
	}

	if !bytes.Equal(encoded, reEncoded) {
		t.Error("round-trip encode produced different bytes")
	}
}

func TestCDSOverflow(t *testing.T) {
	// Time before epoch
	_, err := NewCDS(CCSDSEpoch.Add(-1 * time.Hour))
	if err != ErrOverflow {
		t.Errorf("expected ErrOverflow, got %v", err)
	}
}

func TestCDSInvalidMilliseconds(t *testing.T) {
	c := &CDS{DayBytes: 2, Milliseconds: 86400000}
	if err := c.Validate(); err != ErrInvalidMilliseconds {
		t.Errorf("expected ErrInvalidMilliseconds, got %v", err)
	}
}

func TestCDSHumanize(t *testing.T) {
	testTime := CCSDSEpoch.Add(1 * 24 * time.Hour)
	cds, err := NewCDS(testTime)
	if err != nil {
		t.Fatalf("NewCDS failed: %v", err)
	}

	h := cds.Humanize()
	if len(h) == 0 {
		t.Error("Humanize returned empty string")
	}
}

func TestCDSDataTooShort(t *testing.T) {
	_, err := DecodeCDS([]byte{0x40}, time.Time{})
	if err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestCDSInvalidTimeCodeID(t *testing.T) {
	// P-field with CUC time code ID (001)
	data := []byte{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := DecodeCDS(data, time.Time{})
	if err != ErrInvalidTimeCodeID {
		t.Errorf("expected ErrInvalidTimeCodeID, got %v", err)
	}
}
