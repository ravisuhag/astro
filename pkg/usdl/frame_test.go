package usdl_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

func TestPrimaryHeader_Validate(t *testing.T) {
	tests := []struct {
		name    string
		header  usdl.PrimaryHeader
		wantErr bool
	}{
		{
			name: "valid header",
			header: usdl.PrimaryHeader{
				TFVN:  12,
				SCID:  100,
				VCID:  1,
				MAPID: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid TFVN",
			header: usdl.PrimaryHeader{
				TFVN:  0,
				SCID:  100,
				VCID:  1,
				MAPID: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid VCID",
			header: usdl.PrimaryHeader{
				TFVN:  12,
				SCID:  100,
				VCID:  64,
				MAPID: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid MAPID",
			header: usdl.PrimaryHeader{
				TFVN:  12,
				SCID:  100,
				VCID:  1,
				MAPID: 64,
			},
			wantErr: true,
		},
		{
			name: "max valid SCID",
			header: usdl.PrimaryHeader{
				TFVN:  12,
				SCID:  65535,
				VCID:  63,
				MAPID: 63,
			},
			wantErr: false,
		},
		{
			name: "source/dest flag = 1",
			header: usdl.PrimaryHeader{
				TFVN:         12,
				SCID:         100,
				SourceOrDest: 1,
				VCID:         1,
				MAPID:        0,
			},
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

func TestPrimaryHeader_EncodeDecode_FixedLength(t *testing.T) {
	original := usdl.PrimaryHeader{
		TFVN:         12,
		SCID:         1234,
		SourceOrDest: 1,
		VCID:         42,
		MAPID:        15,
		EndOfFPH:     true, // fixed-length: no frame length field
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != usdl.PrimaryHeaderFixedSize {
		t.Fatalf("Encode() len = %d, want %d", len(encoded), usdl.PrimaryHeaderFixedSize)
	}

	var decoded usdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if decoded.TFVN != original.TFVN {
		t.Errorf("TFVN = %d, want %d", decoded.TFVN, original.TFVN)
	}
	if decoded.SCID != original.SCID {
		t.Errorf("SCID = %d, want %d", decoded.SCID, original.SCID)
	}
	if decoded.SourceOrDest != original.SourceOrDest {
		t.Errorf("SourceOrDest = %d, want %d", decoded.SourceOrDest, original.SourceOrDest)
	}
	if decoded.VCID != original.VCID {
		t.Errorf("VCID = %d, want %d", decoded.VCID, original.VCID)
	}
	if decoded.MAPID != original.MAPID {
		t.Errorf("MAPID = %d, want %d", decoded.MAPID, original.MAPID)
	}
	if decoded.EndOfFPH != original.EndOfFPH {
		t.Errorf("EndOfFPH = %v, want %v", decoded.EndOfFPH, original.EndOfFPH)
	}
}

func TestPrimaryHeader_EncodeDecode_VariableLength(t *testing.T) {
	original := usdl.PrimaryHeader{
		TFVN:         12,
		SCID:         500,
		SourceOrDest: 0,
		VCID:         7,
		MAPID:        3,
		EndOfFPH:     false, // variable-length: includes frame length
		FrameLength:  1023,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != usdl.PrimaryHeaderVariableSize {
		t.Fatalf("Encode() len = %d, want %d", len(encoded), usdl.PrimaryHeaderVariableSize)
	}

	var decoded usdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if decoded.SCID != original.SCID {
		t.Errorf("SCID = %d, want %d", decoded.SCID, original.SCID)
	}
	if decoded.VCID != original.VCID {
		t.Errorf("VCID = %d, want %d", decoded.VCID, original.VCID)
	}
	if decoded.MAPID != original.MAPID {
		t.Errorf("MAPID = %d, want %d", decoded.MAPID, original.MAPID)
	}
	if decoded.FrameLength != original.FrameLength {
		t.Errorf("FrameLength = %d, want %d", decoded.FrameLength, original.FrameLength)
	}
}

func TestPrimaryHeader_EncodeDecode_AllBitPatterns(t *testing.T) {
	// Test with all fields at maximum values
	original := usdl.PrimaryHeader{
		TFVN:         12,
		SCID:         0xFFFF,
		SourceOrDest: 1,
		VCID:         0x3F,
		MAPID:        0x3F,
		EndOfFPH:     false,
		FrameLength:  0xFFFF,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var decoded usdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, original)
	}
}

func TestPrimaryHeader_MCID_GVCID(t *testing.T) {
	h := usdl.PrimaryHeader{
		TFVN: 12,
		SCID: 100,
		VCID: 5,
	}
	mcid := h.MCID()
	expectedMCID := uint32(12)<<16 | 100
	if mcid != expectedMCID {
		t.Errorf("MCID() = %d, want %d", mcid, expectedMCID)
	}

	gvcid := h.GVCID()
	expectedGVCID := expectedMCID<<6 | 5
	if gvcid != expectedGVCID {
		t.Errorf("GVCID() = %d, want %d", gvcid, expectedGVCID)
	}
}

func TestDataFieldHeader_EncodeDecode(t *testing.T) {
	tests := []struct {
		name string
		dfh  usdl.DataFieldHeader
	}{
		{
			name: "packet spanning",
			dfh: usdl.DataFieldHeader{
				ConstructionRule:  usdl.RulePacketSpanning,
				UPID:              5,
				FirstHeaderOffset: 42,
				SequenceNumber:    1000,
			},
		},
		{
			name: "idle",
			dfh: usdl.DataFieldHeader{
				ConstructionRule:  usdl.RuleIdle,
				UPID:              0,
				FirstHeaderOffset: 0xFFFE,
				SequenceNumber:    0,
			},
		},
		{
			name: "max values",
			dfh: usdl.DataFieldHeader{
				ConstructionRule:  7,
				UPID:              0x1F,
				FirstHeaderOffset: 0xFFFF,
				SequenceNumber:    0xFFFF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.dfh.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if len(encoded) != usdl.DataFieldHeaderSize {
				t.Fatalf("Encode() len = %d, want %d", len(encoded), usdl.DataFieldHeaderSize)
			}

			var decoded usdl.DataFieldHeader
			if err := decoded.Decode(encoded); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			if decoded != tt.dfh {
				t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, tt.dfh)
			}
		})
	}
}

func TestTransferFrame_EncodeDecode_CRC16(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data,
		usdl.WithConstructionRule(usdl.RulePacketSpanning),
		usdl.WithSequenceNumber(42),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}

	if decoded.Header.SCID != 100 {
		t.Errorf("SCID = %d, want 100", decoded.Header.SCID)
	}
	if decoded.Header.VCID != 1 {
		t.Errorf("VCID = %d, want 1", decoded.Header.VCID)
	}
	if decoded.Header.MAPID != 0 {
		t.Errorf("MAPID = %d, want 0", decoded.Header.MAPID)
	}
	if decoded.DataFieldHeader.SequenceNumber != 42 {
		t.Errorf("SequenceNumber = %d, want 42", decoded.DataFieldHeader.SequenceNumber)
	}
	if len(decoded.DataField) != len(data) {
		t.Errorf("DataField len = %d, want %d", len(decoded.DataField), len(data))
	}
	for i, b := range decoded.DataField {
		if b != data[i] {
			t.Errorf("DataField[%d] = 0x%02X, want 0x%02X", i, b, data[i])
		}
	}
}

func TestTransferFrame_EncodeDecode_CRC32(t *testing.T) {
	data := []byte{0x0A, 0x0B, 0x0C}
	frame, err := usdl.NewTransferFrame(200, 5, 10, data,
		usdl.WithCRC32(),
		usdl.WithConstructionRule(usdl.RuleOctetStream),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize32, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}

	if decoded.Header.SCID != 200 {
		t.Errorf("SCID = %d, want 200", decoded.Header.SCID)
	}
	if decoded.DataFieldHeader.ConstructionRule != usdl.RuleOctetStream {
		t.Errorf("ConstructionRule = %d, want %d", decoded.DataFieldHeader.ConstructionRule, usdl.RuleOctetStream)
	}
}

func TestTransferFrame_CRCMismatch(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// Corrupt a data byte
	encoded[len(encoded)/2] ^= 0xFF

	_, err = usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != usdl.ErrCRCMismatch {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestTransferFrame_WithOCF(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	ocf := []byte{0xA1, 0xB2, 0xC3, 0xD4}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data,
		usdl.WithOCF(ocf),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := usdl.DecodeTransferFrameWithOCF(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrameWithOCF() error = %v", err)
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
	data := []byte{0x01, 0x02}
	iz := []byte{0xAA, 0xBB, 0xCC}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data,
		usdl.WithInsertZone(iz),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 3)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}

	if len(decoded.InsertZone) != 3 {
		t.Fatalf("InsertZone len = %d, want 3", len(decoded.InsertZone))
	}
	for i, b := range decoded.InsertZone {
		if b != iz[i] {
			t.Errorf("InsertZone[%d] = 0x%02X, want 0x%02X", i, b, iz[i])
		}
	}
}

func TestNewIdleFrame(t *testing.T) {
	config := usdl.ChannelConfig{
		FrameLength: 64,
		HasFECF:     true,
	}
	frame, err := usdl.NewIdleFrame(100, 1, config)
	if err != nil {
		t.Fatalf("NewIdleFrame() error = %v", err)
	}

	if !usdl.IsIdleFrame(frame) {
		t.Error("expected idle frame")
	}
	if frame.DataFieldHeader.ConstructionRule != usdl.RuleIdle {
		t.Errorf("ConstructionRule = %d, want %d", frame.DataFieldHeader.ConstructionRule, usdl.RuleIdle)
	}
	if !frame.Header.EndOfFPH {
		t.Error("expected EndOfFPH=true for idle frame on fixed-length channel")
	}

	// Verify encoded frame matches configured FrameLength
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != config.FrameLength {
		t.Errorf("encoded idle frame length = %d, want %d", len(encoded), config.FrameLength)
	}

	// Verify CRC is valid
	_, err = usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame(idle) error = %v", err)
	}

	// Verify all data bytes are 0xFF
	for i, b := range frame.DataField {
		if b != 0xFF {
			t.Errorf("DataField[%d] = 0x%02X, want 0xFF", i, b)
			break
		}
	}
}

func TestTransferFrame_Humanize(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}

	s := frame.Humanize()
	if s == "" {
		t.Error("Humanize() returned empty string")
	}
}
