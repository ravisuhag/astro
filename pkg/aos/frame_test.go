package aos_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/aos"
)

func TestPrimaryHeader_Validate(t *testing.T) {
	tests := []struct {
		name    string
		header  aos.PrimaryHeader
		wantErr bool
	}{
		{
			name:    "valid",
			header:  aos.PrimaryHeader{TFVN: 1, SCID: 100, VCID: 1},
			wantErr: false,
		},
		{
			name:    "invalid TFVN (TM)",
			header:  aos.PrimaryHeader{TFVN: 0, SCID: 100, VCID: 1},
			wantErr: true,
		},
		{
			name:    "invalid TFVN (USLP)",
			header:  aos.PrimaryHeader{TFVN: 12, SCID: 100, VCID: 1},
			wantErr: true,
		},
		{
			name:    "VCID out of range",
			header:  aos.PrimaryHeader{TFVN: 1, SCID: 100, VCID: 64},
			wantErr: true,
		},
		{
			name:    "VC frame count out of range",
			header:  aos.PrimaryHeader{TFVN: 1, VCFrameCount: 0x01000000},
			wantErr: true,
		},
		{
			name:    "VC frame count cycle out of range",
			header:  aos.PrimaryHeader{TFVN: 1, VCFrameCountCycle: 16},
			wantErr: true,
		},
		{
			name:    "max valid",
			header:  aos.PrimaryHeader{TFVN: 1, SCID: 0xFF, VCID: 0x3F, VCFrameCount: 0xFFFFFF, VCFrameCountCycle: 0x0F},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.header.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrimaryHeader_RoundTrip(t *testing.T) {
	original := aos.PrimaryHeader{
		TFVN:              1,
		SCID:              200,
		VCID:              42,
		VCFrameCount:      0x123456,
		ReplayFlag:        true,
		VCFCUsageFlag:     true,
		VCFrameCountCycle: 7,
	}
	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != aos.PrimaryHeaderSize {
		t.Fatalf("encoded len = %d, want %d", len(encoded), aos.PrimaryHeaderSize)
	}

	var decoded aos.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", decoded, original)
	}
}

func TestPrimaryHeader_AllBitPatterns(t *testing.T) {
	original := aos.PrimaryHeader{
		TFVN:              1,
		SCID:              0xFF,
		VCID:              0x3F,
		VCFrameCount:      0xFFFFFF,
		ReplayFlag:        true,
		VCFCUsageFlag:     true,
		VCFrameCountCycle: 0x0F,
	}
	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var decoded aos.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", decoded, original)
	}
}

func TestPrimaryHeader_MCID_GVCID(t *testing.T) {
	h := aos.PrimaryHeader{TFVN: 1, SCID: 50, VCID: 7}
	wantMCID := uint16(1)<<8 | 50
	if got := h.MCID(); got != wantMCID {
		t.Errorf("MCID() = %d, want %d", got, wantMCID)
	}
	wantGVCID := uint32(wantMCID)<<6 | 7
	if got := h.GVCID(); got != wantGVCID {
		t.Errorf("GVCID() = %d, want %d", got, wantGVCID)
	}
}

func TestMPDUHeader_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		fhp  uint16
	}{
		{"zero", 0},
		{"middle", 0x0123},
		{"FHPNoPacketStart", aos.FHPNoPacketStart},
		{"FHPAllIdle", aos.FHPAllIdle},
		{"max valid", aos.MPDUMaxFirstHeaderPointer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := aos.MPDUHeader{FirstHeaderPointer: tt.fhp}
			encoded, err := hdr.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if len(encoded) != aos.MPDUHeaderSize {
				t.Fatalf("encoded len = %d, want %d", len(encoded), aos.MPDUHeaderSize)
			}
			var decoded aos.MPDUHeader
			if err := decoded.Decode(encoded); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if decoded.FirstHeaderPointer != tt.fhp {
				t.Errorf("FHP = 0x%X, want 0x%X", decoded.FirstHeaderPointer, tt.fhp)
			}
		})
	}
}

func TestMPDUHeader_OutOfRange(t *testing.T) {
	hdr := aos.MPDUHeader{FirstHeaderPointer: 0x0800}
	if _, err := hdr.Encode(); err == nil {
		t.Error("expected error for FHP out of range")
	}
}

func TestBPDUHeader_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bdp  uint16
	}{
		{"zero", 0},
		{"middle", 0x0FFF},
		{"BDPAllIdle", aos.BDPAllIdle},
		{"BDPAllValid", aos.BDPAllValid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := aos.BPDUHeader{BitstreamDataPointer: tt.bdp}
			encoded, err := hdr.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if len(encoded) != aos.BPDUHeaderSize {
				t.Fatalf("encoded len = %d, want %d", len(encoded), aos.BPDUHeaderSize)
			}
			var decoded aos.BPDUHeader
			if err := decoded.Decode(encoded); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if decoded.BitstreamDataPointer != tt.bdp {
				t.Errorf("BDP = 0x%X, want 0x%X", decoded.BitstreamDataPointer, tt.bdp)
			}
		})
	}
}

func TestBPDUHeader_OutOfRange(t *testing.T) {
	hdr := aos.BPDUHeader{BitstreamDataPointer: 0x4000}
	if _, err := hdr.Encode(); err == nil {
		t.Error("expected error for BDP out of range")
	}
}

func TestTransferFrame_RoundTrip_NoOptions(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	frame, err := aos.NewTransferFrame(50, 1, data)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := aos.DecodeTransferFrame(encoded, 0, false, false)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if decoded.Header.SCID != 50 {
		t.Errorf("SCID = %d, want 50", decoded.Header.SCID)
	}
	if decoded.Header.VCID != 1 {
		t.Errorf("VCID = %d, want 1", decoded.Header.VCID)
	}
	if string(decoded.DataField) != string(data) {
		t.Errorf("DataField = %v, want %v", decoded.DataField, data)
	}
}

func TestTransferFrame_WithFECF(t *testing.T) {
	data := []byte{0xAA, 0xBB, 0xCC}
	frame, err := aos.NewTransferFrame(99, 2, data, aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := aos.DecodeTransferFrame(encoded, 0, false, true)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if string(decoded.DataField) != string(data) {
		t.Errorf("DataField mismatch")
	}
}

func TestTransferFrame_FECF_CRCMismatch(t *testing.T) {
	frame, err := aos.NewTransferFrame(99, 2, []byte{0x01, 0x02, 0x03}, aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	encoded[len(encoded)/2] ^= 0xFF
	if _, err := aos.DecodeTransferFrame(encoded, 0, false, true); err != aos.ErrCRCMismatch {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestTransferFrame_WithOCF(t *testing.T) {
	ocf := []byte{0x11, 0x22, 0x33, 0x44}
	frame, err := aos.NewTransferFrame(1, 0, []byte{0x99}, aos.WithOCF(ocf), aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := aos.DecodeTransferFrame(encoded, 0, true, true)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if len(decoded.OCF) != 4 {
		t.Fatalf("OCF len = %d, want 4", len(decoded.OCF))
	}
	for i, b := range decoded.OCF {
		if b != ocf[i] {
			t.Errorf("OCF[%d] = 0x%02X, want 0x%02X", i, b, ocf[i])
		}
	}
}

func TestTransferFrame_WithInsertZone(t *testing.T) {
	iz := []byte{0xDE, 0xAD, 0xC0, 0xDE}
	frame, err := aos.NewTransferFrame(1, 0, []byte{0x99}, aos.WithInsertZone(iz), aos.WithFECF())
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := aos.DecodeTransferFrame(encoded, len(iz), false, true)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if len(decoded.InsertZone) != len(iz) {
		t.Fatalf("InsertZone len = %d, want %d", len(decoded.InsertZone), len(iz))
	}
	for i, b := range decoded.InsertZone {
		if b != iz[i] {
			t.Errorf("InsertZone[%d] = 0x%02X, want 0x%02X", i, b, iz[i])
		}
	}
}

func TestPackMPDUDataField(t *testing.T) {
	pkt := []byte{0x01, 0x02, 0x03}
	df, err := aos.PackMPDUDataField(0, pkt)
	if err != nil {
		t.Fatalf("PackMPDUDataField() error = %v", err)
	}
	if len(df) != aos.MPDUHeaderSize+len(pkt) {
		t.Errorf("df len = %d, want %d", len(df), aos.MPDUHeaderSize+len(pkt))
	}
	var hdr aos.MPDUHeader
	if err := hdr.Decode(df); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if hdr.FirstHeaderPointer != 0 {
		t.Errorf("FHP = %d, want 0", hdr.FirstHeaderPointer)
	}
}

func TestPackBPDUDataField(t *testing.T) {
	bits := []byte{0xFF, 0x00, 0xAA}
	df, err := aos.PackBPDUDataField(aos.BDPAllValid, bits)
	if err != nil {
		t.Fatalf("PackBPDUDataField() error = %v", err)
	}
	if len(df) != aos.BPDUHeaderSize+len(bits) {
		t.Errorf("df len = %d, want %d", len(df), aos.BPDUHeaderSize+len(bits))
	}
}

func TestNewIdleFrame(t *testing.T) {
	config := aos.ChannelConfig{FrameLength: 64, HasFECF: true}
	frame, err := aos.NewIdleFrame(10, config)
	if err != nil {
		t.Fatalf("NewIdleFrame() error = %v", err)
	}
	if !aos.IsIdleFrame(frame) {
		t.Errorf("expected idle frame")
	}
	if frame.Header.VCID != aos.OIDVCID {
		t.Errorf("VCID = %d, want %d", frame.Header.VCID, aos.OIDVCID)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != config.FrameLength {
		t.Errorf("encoded len = %d, want %d", len(encoded), config.FrameLength)
	}
	if _, err := aos.DecodeTransferFrame(encoded, 0, false, true); err != nil {
		t.Errorf("decode idle frame: %v", err)
	}
}

func TestTransferFrame_Humanize(t *testing.T) {
	frame, err := aos.NewTransferFrame(1, 0, []byte{0x01})
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	if s := frame.Humanize(); s == "" {
		t.Error("Humanize() returned empty string")
	}
}
