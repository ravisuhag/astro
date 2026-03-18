package tcf

import "testing"

func TestPFieldEncodeDecodeNoExtension(t *testing.T) {
	p := PField{
		Extension:  false,
		TimeCodeID: TimeCodeCUCLevel1,
		Detail:     0x0F,
	}

	data, err := p.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 byte, got %d", len(data))
	}

	var decoded PField
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Extension != false {
		t.Errorf("expected Extension=false, got true")
	}
	if decoded.TimeCodeID != TimeCodeCUCLevel1 {
		t.Errorf("expected TimeCodeID=%d, got %d", TimeCodeCUCLevel1, decoded.TimeCodeID)
	}
	if decoded.Detail != 0x0F {
		t.Errorf("expected Detail=0x0F, got 0x%02X", decoded.Detail)
	}
}

func TestPFieldEncodeDecodeWithExtension(t *testing.T) {
	p := PField{
		Extension:  true,
		TimeCodeID: TimeCodeCUCLevel1,
		Detail:     0x0C,
		ExtDetail:  0x09,
	}

	data, err := p.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 bytes, got %d", len(data))
	}

	var decoded PField
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Extension != true {
		t.Errorf("expected Extension=true, got false")
	}
	if decoded.TimeCodeID != TimeCodeCUCLevel1 {
		t.Errorf("expected TimeCodeID=%d, got %d", TimeCodeCUCLevel1, decoded.TimeCodeID)
	}
	if decoded.Detail != 0x0C {
		t.Errorf("expected Detail=0x0C, got 0x%02X", decoded.Detail)
	}
	if decoded.ExtDetail != 0x09 {
		t.Errorf("expected ExtDetail=0x09, got 0x%02X", decoded.ExtDetail)
	}
}

func TestPFieldSize(t *testing.T) {
	p1 := PField{Extension: false}
	if p1.Size() != 1 {
		t.Errorf("expected size 1, got %d", p1.Size())
	}

	p2 := PField{Extension: true}
	if p2.Size() != 2 {
		t.Errorf("expected size 2, got %d", p2.Size())
	}
}

func TestPFieldDecodeDataTooShort(t *testing.T) {
	var p PField
	if err := p.Decode(nil); err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}

func TestPFieldDecodeExtensionDataTooShort(t *testing.T) {
	// Extension flag set but only 1 byte provided
	var p PField
	if err := p.Decode([]byte{0x80}); err != ErrDataTooShort {
		t.Errorf("expected ErrDataTooShort, got %v", err)
	}
}
