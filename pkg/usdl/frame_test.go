package usdl_test

import (
	"bytes"
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
			name:    "valid header",
			header:  usdl.PrimaryHeader{TFVN: 12, SCID: 100, VCID: 1, MAPID: 0},
			wantErr: false,
		},
		{
			name:    "invalid TFVN",
			header:  usdl.PrimaryHeader{TFVN: 0, SCID: 100, VCID: 1},
			wantErr: true,
		},
		{
			name:    "invalid VCID",
			header:  usdl.PrimaryHeader{TFVN: 12, SCID: 100, VCID: 64},
			wantErr: true,
		},
		{
			name:    "invalid MAPID (5 bits)",
			header:  usdl.PrimaryHeader{TFVN: 12, SCID: 100, VCID: 1, MAPID: 16},
			wantErr: true,
		},
		{
			name:    "max valid fields",
			header:  usdl.PrimaryHeader{TFVN: 12, SCID: 65535, VCID: 63, MAPID: 15},
			wantErr: false,
		},
		{
			name: "VCF count too wide for field",
			header: usdl.PrimaryHeader{
				TFVN: 12, SCID: 1, VCID: 1,
				VCFCountLen: 1, VCFCount: 256,
			},
			wantErr: true,
		},
		{
			name: "VCF count length out of range",
			header: usdl.PrimaryHeader{
				TFVN: 12, SCID: 1, VCID: 1, VCFCountLen: 8,
			},
			wantErr: true,
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

func TestPrimaryHeader_EncodeDecode_Truncated(t *testing.T) {
	original := usdl.PrimaryHeader{
		TFVN:         12,
		SCID:         1234,
		SourceOrDest: 1,
		VCID:         42,
		MAPID:        15,
		EndOfFPH:     true,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != usdl.TruncatedPrimaryHeaderSize {
		t.Fatalf("Encode() len = %d, want %d (truncated header is exactly 4 octets)",
			len(encoded), usdl.TruncatedPrimaryHeaderSize)
	}

	var decoded usdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, original)
	}
}

func TestPrimaryHeader_EncodeDecode_NonTruncated(t *testing.T) {
	original := usdl.PrimaryHeader{
		TFVN:          12,
		SCID:          500,
		SourceOrDest:  0,
		VCID:          7,
		MAPID:         3,
		FrameLength:   1023,
		BypassSeqCtrl: true,
		ProtCtrlCmd:   true,
		OCFFlag:       true,
		VCFCountLen:   3,
		VCFCount:      0x0A0B0C,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != usdl.PrimaryHeaderBaseSize+3 {
		t.Fatalf("Encode() len = %d, want %d", len(encoded), usdl.PrimaryHeaderBaseSize+3)
	}

	var decoded usdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", decoded, original)
	}
}

func TestPrimaryHeader_EncodeDecode_AllBitPatterns(t *testing.T) {
	original := usdl.PrimaryHeader{
		TFVN:          12,
		SCID:          0xFFFF,
		SourceOrDest:  1,
		VCID:          0x3F,
		MAPID:         0x0F,
		FrameLength:   0xFFFF,
		BypassSeqCtrl: true,
		ProtCtrlCmd:   true,
		OCFFlag:       true,
		VCFCountLen:   7,
		VCFCount:      0xFFFFFFFFFFFFFF,
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
	h := usdl.PrimaryHeader{TFVN: 12, SCID: 100, VCID: 5, MAPID: 2}
	expectedMCID := uint32(12)<<16 | 100
	if got := h.MCID(); got != expectedMCID {
		t.Errorf("MCID() = %d, want %d", got, expectedMCID)
	}
	expectedGVCID := expectedMCID<<6 | 5
	if got := h.GVCID(); got != expectedGVCID {
		t.Errorf("GVCID() = %d, want %d", got, expectedGVCID)
	}
	expectedGMAPID := expectedGVCID<<4 | 2
	if got := h.GMAPID(); got != expectedGMAPID {
		t.Errorf("GMAPID() = %d, want %d", got, expectedGMAPID)
	}
}

func TestDataFieldHeader_EncodeDecode(t *testing.T) {
	tests := []struct {
		name     string
		dfh      usdl.DataFieldHeader
		wantSize int
	}{
		{
			name: "packets spanning (pointer present)",
			dfh: usdl.DataFieldHeader{
				ConstructionRule: usdl.RulePacketsSpanning,
				UPID:             usdl.UPIDSpacePackets,
				Pointer:          42,
			},
			wantSize: 3,
		},
		{
			name: "start of SDU (pointer present)",
			dfh: usdl.DataFieldHeader{
				ConstructionRule: usdl.RuleStartOfSDU,
				UPID:             usdl.UPIDMissionSpecific1,
				Pointer:          usdl.LVOPIncomplete,
			},
			wantSize: 3,
		},
		{
			name: "octet stream (no pointer)",
			dfh: usdl.DataFieldHeader{
				ConstructionRule: usdl.RuleOctetStream,
				UPID:             usdl.UPIDUserOctetStream,
			},
			wantSize: 1,
		},
		{
			name: "no segmentation (no pointer)",
			dfh: usdl.DataFieldHeader{
				ConstructionRule: usdl.RuleNoSegmentation,
				UPID:             0x1F,
			},
			wantSize: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.dfh.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if len(encoded) != tt.wantSize {
				t.Fatalf("Encode() len = %d, want %d", len(encoded), tt.wantSize)
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

func TestConstructionRuleValues(t *testing.T) {
	// Spec values per CCSDS 732.1-B-2 §4.1.4.2.2.2 / table 4-3.
	want := map[uint8]uint8{
		usdl.RulePacketsSpanning:   0b000,
		usdl.RuleStartOfSDU:        0b001,
		usdl.RuleContinuingSDU:     0b010,
		usdl.RuleOctetStream:       0b011,
		usdl.RuleStartingSegment:   0b100,
		usdl.RuleContinuingSegment: 0b101,
		usdl.RuleLastSegment:       0b110,
		usdl.RuleNoSegmentation:    0b111,
	}
	for got, spec := range want {
		if got != spec {
			t.Errorf("construction rule constant = %03b, want %03b", got, spec)
		}
	}
}

func TestTransferFrame_EncodeDecode_CRC16(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data,
		usdl.WithConstructionRule(usdl.RulePacketsSpanning),
		usdl.WithPointer(0),
		usdl.WithVCFCount(2, 42),
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

	if decoded.Header.SCID != 100 || decoded.Header.VCID != 1 || decoded.Header.MAPID != 0 {
		t.Errorf("header mismatch: %+v", decoded.Header)
	}
	if decoded.Header.VCFCount != 42 {
		t.Errorf("VCFCount = %d, want 42", decoded.Header.VCFCount)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %x, want %x", decoded.DataField, data)
	}
}

func TestTransferFrame_EncodeDecode_CRC32(t *testing.T) {
	data := []byte{0x0A, 0x0B, 0x0C}
	frame, err := usdl.NewTransferFrame(200, 5, 10, data,
		usdl.WithCRC32(),
		usdl.WithConstructionRule(usdl.RuleOctetStream),
		usdl.WithUPID(usdl.UPIDUserOctetStream),
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
	if decoded.DataFieldHeader.UPID != usdl.UPIDUserOctetStream {
		t.Errorf("UPID = %d, want %d", decoded.DataFieldHeader.UPID, usdl.UPIDUserOctetStream)
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
	encoded[len(encoded)-3] ^= 0xFF

	_, err = usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != usdl.ErrCRCMismatch {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestTransferFrame_WithOCF_SignaledInBand(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	ocf := []byte{0xA1, 0xB2, 0xC3, 0xD4}
	frame, err := usdl.NewTransferFrame(100, 1, 0, data,
		usdl.WithOCF(ocf),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	if !frame.Header.OCFFlag {
		t.Error("OCF flag not set on construction")
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	// No out-of-band OCF knowledge needed: the flag drives the decoder.
	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if !bytes.Equal(decoded.OCF, ocf) {
		t.Errorf("OCF = %x, want %x", decoded.OCF, ocf)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %x, want %x", decoded.DataField, data)
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

	if !bytes.Equal(decoded.InsertZone, iz) {
		t.Errorf("InsertZone = %x, want %x", decoded.InsertZone, iz)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %x, want %x", decoded.DataField, data)
	}
}

func TestNewTruncatedFrame_RejectsTrailers(t *testing.T) {
	_, err := usdl.NewTruncatedFrame(1, 1, 0, []byte{0x01},
		usdl.WithOCF(make([]byte, 4)))
	if err != usdl.ErrTruncatedFrameFields {
		t.Errorf("expected ErrTruncatedFrameFields for OCF, got %v", err)
	}
	_, err = usdl.NewTruncatedFrame(1, 1, 0, []byte{0x01},
		usdl.WithConstructionRule(usdl.RulePacketsSpanning))
	if err != usdl.ErrTruncatedFrameFields {
		t.Errorf("expected ErrTruncatedFrameFields for pointer rule, got %v", err)
	}
}

func TestNewIdleFrame(t *testing.T) {
	config := usdl.ChannelConfig{
		FrameLength: 64,
		HasFECF:     true,
	}
	frame, err := usdl.NewIdleFrame(100, config)
	if err != nil {
		t.Fatalf("NewIdleFrame() error = %v", err)
	}

	if !usdl.IsIdleFrame(frame) {
		t.Error("expected idle frame")
	}
	if frame.Header.VCID != usdl.OIDVCID {
		t.Errorf("VCID = %d, want %d (OID frames use VCID 63)", frame.Header.VCID, usdl.OIDVCID)
	}
	if frame.Header.MAPID != 0 {
		t.Errorf("MAPID = %d, want 0 (§4.1.4.1.8)", frame.Header.MAPID)
	}
	if frame.DataFieldHeader.ConstructionRule != usdl.RuleStartOfSDU {
		t.Errorf("ConstructionRule = %d, want %d ('001', §4.1.4.1.9)",
			frame.DataFieldHeader.ConstructionRule, usdl.RuleStartOfSDU)
	}
	if frame.DataFieldHeader.UPID != usdl.UPIDIdle {
		t.Errorf("UPID = %d, want %d (Idle Data)", frame.DataFieldHeader.UPID, usdl.UPIDIdle)
	}
	if int(frame.DataFieldHeader.Pointer) != len(frame.DataField)-1 {
		t.Errorf("LVOP = %d, want %d (last TFDZ octet)",
			frame.DataFieldHeader.Pointer, len(frame.DataField)-1)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) != config.FrameLength {
		t.Errorf("encoded idle frame length = %d, want %d", len(encoded), config.FrameLength)
	}
	if _, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0); err != nil {
		t.Fatalf("DecodeTransferFrame(idle) error = %v", err)
	}
}

func TestNewIdleFrame_CustomPattern(t *testing.T) {
	config := usdl.ChannelConfig{
		FrameLength: 32,
		HasFECF:     true,
		IdlePattern: []byte{0xA5, 0x5A},
	}
	frame, err := usdl.NewIdleFrame(100, config)
	if err != nil {
		t.Fatalf("NewIdleFrame() error = %v", err)
	}
	for i, b := range frame.DataField {
		want := config.IdlePattern[i%2]
		if b != want {
			t.Errorf("idle fill[%d] = 0x%02X, want 0x%02X", i, b, want)
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

	if s := frame.Humanize(); s == "" {
		t.Error("Humanize() returned empty string")
	}
}

func TestUSDLFrame_EncodeRecomputesFECFAfterMutation(t *testing.T) {
	frame, err := usdl.NewTransferFrame(42, 1, 0, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}

	frame.Header.VCID = 3

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := usdl.DecodeTransferFrame(encoded, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("re-encoded frame does not decode: %v", err)
	}
	if decoded.Header.VCID != 3 {
		t.Errorf("VCID = %d, want 3", decoded.Header.VCID)
	}
}
