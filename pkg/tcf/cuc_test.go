package tcf

import (
	"bytes"
	"testing"
	"time"
)

func TestCUCNewAndEncodeDecode(t *testing.T) {
	// Use a known time relative to CCSDS epoch
	testTime := CCSDSEpoch.Add(1000 * time.Second)

	cuc, err := NewCUC(testTime)
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	if cuc.CoarseTime != 1000 {
		t.Errorf("expected CoarseTime=1000, got %d", cuc.CoarseTime)
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCUC(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
	}

	if decoded.CoarseTime != cuc.CoarseTime {
		t.Errorf("expected CoarseTime=%d, got %d", cuc.CoarseTime, decoded.CoarseTime)
	}
	if decoded.CoarseBytes != cuc.CoarseBytes {
		t.Errorf("expected CoarseBytes=%d, got %d", cuc.CoarseBytes, decoded.CoarseBytes)
	}
}

func TestCUCWithFineTime(t *testing.T) {
	testTime := CCSDSEpoch.Add(100*time.Second + 500*time.Millisecond)

	cuc, err := NewCUC(testTime, WithCUCFineBytes(2))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	if cuc.CoarseTime != 100 {
		t.Errorf("expected CoarseTime=100, got %d", cuc.CoarseTime)
	}
	if cuc.FineTime == 0 {
		t.Error("expected non-zero FineTime for 500ms fraction")
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCUC(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
	}

	if decoded.FineBytes != 2 {
		t.Errorf("expected FineBytes=2, got %d", decoded.FineBytes)
	}

	// Round-trip time should be close (within fine time resolution)
	original := cuc.Time()
	roundTrip := decoded.Time()
	diff := original.Sub(roundTrip)
	if diff < 0 {
		diff = -diff
	}
	// 2 fine octets gives ~15.3µs resolution
	if diff > 20*time.Microsecond {
		t.Errorf("round-trip time difference too large: %v", diff)
	}
}

func TestCUCLevel2CustomEpoch(t *testing.T) {
	customEpoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	testTime := customEpoch.Add(42 * time.Second)

	cuc, err := NewCUC(testTime, WithCUCEpoch(customEpoch), WithCUCCoarseBytes(2))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	if cuc.PField.TimeCodeID != TimeCodeCUCLevel2 {
		t.Errorf("expected Level 2 time code ID, got %d", cuc.PField.TimeCodeID)
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCUC(encoded, customEpoch)
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
	}

	if decoded.CoarseTime != 42 {
		t.Errorf("expected CoarseTime=42, got %d", decoded.CoarseTime)
	}
}

func TestCUCLevel2RequiresEpoch(t *testing.T) {
	customEpoch := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	testTime := customEpoch.Add(1 * time.Second)

	cuc, err := NewCUC(testTime, WithCUCEpoch(customEpoch))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decoding Level 2 without an epoch should fail
	_, err = DecodeCUC(encoded, time.Time{})
	if err != ErrEpochRequired {
		t.Errorf("expected ErrEpochRequired, got %v", err)
	}
}

func TestCUCOverflow(t *testing.T) {
	// Time before epoch
	beforeEpoch := CCSDSEpoch.Add(-1 * time.Second)
	_, err := NewCUC(beforeEpoch)
	if err != ErrOverflow {
		t.Errorf("expected ErrOverflow for time before epoch, got %v", err)
	}
}

func TestCUCEncodeDecodeRoundTrip(t *testing.T) {
	testTime := CCSDSEpoch.Add(86400*365*10*time.Second + 123456789*time.Nanosecond)

	cuc, err := NewCUC(testTime, WithCUCFineBytes(3))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCUC(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
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

func TestCUCHumanize(t *testing.T) {
	testTime := CCSDSEpoch.Add(1000 * time.Second)
	cuc, err := NewCUC(testTime)
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	h := cuc.Humanize()
	if len(h) == 0 {
		t.Error("Humanize returned empty string")
	}
}

func TestCUCDataTooShort(t *testing.T) {
	_, err := DecodeCUC([]byte{0x1C}, time.Time{})
	if err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestCUCInvalidTimeCodeID(t *testing.T) {
	// Build a P-field with CDS time code ID (100 = 0x04)
	data := []byte{0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := DecodeCUC(data, time.Time{})
	if err != ErrInvalidTimeCodeID {
		t.Errorf("expected ErrInvalidTimeCodeID, got %v", err)
	}
}

func TestCUCPFieldExtensionBitLayout(t *testing.T) {
	// Verify extension octet encodes additional coarse in bits 1-2
	// and additional fine in bits 3-5 per §3.2.2
	testTime := CCSDSEpoch.Add(100 * time.Second)
	cuc, err := NewCUC(testTime, WithCUCCoarseBytes(5), WithCUCFineBytes(4))
	if err != nil {
		t.Fatalf("NewCUC failed: %v", err)
	}

	if !cuc.PField.Extension {
		t.Fatal("expected extension flag set")
	}

	encoded, err := cuc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeCUC(encoded, time.Time{})
	if err != nil {
		t.Fatalf("DecodeCUC failed: %v", err)
	}

	if decoded.CoarseBytes != 5 {
		t.Errorf("expected CoarseBytes=5, got %d", decoded.CoarseBytes)
	}
	if decoded.FineBytes != 4 {
		t.Errorf("expected FineBytes=4, got %d", decoded.FineBytes)
	}
}
