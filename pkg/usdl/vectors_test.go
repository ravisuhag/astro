package usdl_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/usdl"
)

// Golden wire vectors, hand-computed from the CCSDS 732.1-B-3 clause 4.1.2 and
// Clause 4.1.4 field layouts and checked with independent CRC implementations.

// Non-truncated frame: TFVN='1100', SCID=1234 (0x04D2), source/dest=1,
// VCID=42, MAP ID=5, EOFPH=0, frame length=18-1=17, bypass=0, protocol
// control=0, spares=00, OCF flag=0, VCF count length=2, VCF count=258.
// TFDF header: rule '000' | UPID 0, FHP=0. TFDZ=deadbeef. CRC-16 FECF.
//
//	byte 0 = 1100_0000                        = 0xC0
//	byte 1 = SCID[11:4]                       = 0x4D
//	byte 2 = SCID[3:0] | S/D | VCID[5:3]      = 0x2D
//	byte 3 = VCID[2:0] | MAPID | EOFPH        = 0x4A
//	bytes 4-5 = 0x0011, byte 6 = 0x02, bytes 7-8 = 0x0102
func TestGoldenVector_NonTruncatedCRC16(t *testing.T) {
	want, _ := hex.DecodeString("c04d2d4a0011020102000000deadbeef0e51")

	frame, err := usdl.NewTransferFrame(1234, 42, 5, []byte{0xDE, 0xAD, 0xBE, 0xEF},
		usdl.WithSourceOrDest(1),
		usdl.WithConstructionRule(usdl.RulePacketsSpanning),
		usdl.WithUPID(usdl.UPIDSpacePackets),
		usdl.WithPointer(0),
		usdl.WithVCFCount(2, 258),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got %x\nwant %x", got, want)
	}

	decoded, err := usdl.DecodeTransferFrame(want, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	h := decoded.Header
	if h.SCID != 1234 || h.SourceOrDest != 1 || h.VCID != 42 || h.MAPID != 5 ||
		h.EndOfFPH || h.FrameLength != 17 || h.VCFCountLen != 2 || h.VCFCount != 258 {
		t.Errorf("decoded header mismatch: %+v", h)
	}
	if decoded.DataFieldHeader.ConstructionRule != usdl.RulePacketsSpanning ||
		decoded.DataFieldHeader.Pointer != 0 {
		t.Errorf("decoded TFDF header mismatch: %+v", decoded.DataFieldHeader)
	}
	if !bytes.Equal(decoded.DataField, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("decoded TFDZ = %x", decoded.DataField)
	}
}

// Truncated frame (annex D): exactly 4 header octets, EOFPH=1, then a
// 1-octet TFDF header (rule '111', UPID 5) and the TFDZ. No insert zone,
// OCF, or FECF. SCID=0xAB, VCID=1, MAP ID=2.
//
//	byte 0 = 0xC0, byte 1 = 0x0A, byte 2 = 0xB0, byte 3 = 0x25
//	TFDF header = 111_00101 = 0xE5
func TestGoldenVector_Truncated(t *testing.T) {
	want, _ := hex.DecodeString("c00ab025e5112233")

	frame, err := usdl.NewTruncatedFrame(0xAB, 1, 2, []byte{0x11, 0x22, 0x33})
	if err != nil {
		t.Fatalf("NewTruncatedFrame() error = %v", err)
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got %x\nwant %x", got, want)
	}
	if len(got) != usdl.TruncatedPrimaryHeaderSize+1+3 {
		t.Errorf("truncated frame length = %d, want 8", len(got))
	}

	decoded, err := usdl.DecodeTransferFrame(want, 0, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if !decoded.Header.EndOfFPH || decoded.Header.SCID != 0xAB ||
		decoded.Header.VCID != 1 || decoded.Header.MAPID != 2 {
		t.Errorf("decoded header mismatch: %+v", decoded.Header)
	}
	if decoded.DataFieldHeader.ConstructionRule != usdl.RuleNoSegmentation ||
		decoded.DataFieldHeader.UPID != usdl.UPIDMissionSpecific1 {
		t.Errorf("decoded TFDF header mismatch: %+v", decoded.DataFieldHeader)
	}
	if !bytes.Equal(decoded.DataField, []byte{0x11, 0x22, 0x33}) {
		t.Errorf("decoded TFDZ = %x", decoded.DataField)
	}
}

// Non-truncated frame with OCF: SCID=100, VCID=1, MAP=0, no VCF count,
// rule '111' UPID 0, TFDZ=0102, OCF=aabbccdd, 16-bit FECF (clause 4.1.6.2.2:
// the FECF, when present, is the last 16 bits of the frame). The OCF flag
// (bit 52) is set: byte 6 = 0x08. Total 16 octets, frame length field 15.
func TestGoldenVector_OCFAndCRC16(t *testing.T) {
	want, _ := hex.DecodeString("c0064020000f08e00102aabbccdd778e")

	frame, err := usdl.NewTransferFrame(100, 1, 0, []byte{0x01, 0x02},
		usdl.WithConstructionRule(usdl.RuleNoSegmentation),
		usdl.WithUPID(usdl.UPIDSpacePackets),
		usdl.WithOCF([]byte{0xAA, 0xBB, 0xCC, 0xDD}),
	)
	if err != nil {
		t.Fatalf("NewTransferFrame() error = %v", err)
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\n got %x\nwant %x", got, want)
	}

	// The OCF is recovered from the in-band OCF flag, not out-of-band
	// knowledge.
	decoded, err := usdl.DecodeTransferFrame(want, usdl.FECSize16, 0)
	if err != nil {
		t.Fatalf("DecodeTransferFrame() error = %v", err)
	}
	if !decoded.Header.OCFFlag {
		t.Error("OCF flag not decoded")
	}
	if !bytes.Equal(decoded.OCF, []byte{0xAA, 0xBB, 0xCC, 0xDD}) {
		t.Errorf("decoded OCF = %x, want aabbccdd", decoded.OCF)
	}
	if !bytes.Equal(decoded.DataField, []byte{0x01, 0x02}) {
		t.Errorf("decoded TFDZ = %x, want 0102", decoded.DataField)
	}
}

// oidPNPrefix is the start of the mandatory OID PN sequence exactly as
// printed in CCSDS 732.1-B-3 annex H ("Generated data pattern in both
// cases: FF FF FF FF 6D B6 D8 61 45 1F 11 F1 97 16 72 3C BE 7E 00 B1").
const oidPNPrefix = "ffffffff6db6d861451f11f19716723cbe7e00b1"

// The OID fill generator must reproduce the annex H known-answer stream:
// a 32-cell Fibonacci LFSR with polynomial D0+D1+D2+D22+D32 seeded all
// ones (clause 4.1.4.1.10).
func TestGoldenVector_OIDPNSequence(t *testing.T) {
	want, _ := hex.DecodeString(oidPNPrefix)

	seq := usdl.NewOIDSequence()
	got := make([]byte, len(want))
	seq.Fill(got)
	if !bytes.Equal(got, want) {
		t.Fatalf("PN stream mismatch:\n got %x\nwant %x", got, want)
	}

	// Clause 4.1.6.2.2: only 16-bit FECFs exist; a 32-bit size is refused.
	if _, err := usdl.DecodeTransferFrame(want, 4, 0); err != usdl.ErrInvalidFECSize {
		t.Errorf("DecodeTransferFrame(fecSize=4) error = %v, want ErrInvalidFECSize", err)
	}
}

// The decoded frame length field must match the delivered buffer.
func TestDecode_FrameLengthCrossCheck(t *testing.T) {
	frame, err := usdl.NewTransferFrame(1, 1, 0, []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// Deliver with a trailing extra byte: length field no longer matches.
	padded := append(append([]byte{}, encoded...), 0x00)
	if _, err := usdl.DecodeTransferFrame(padded, usdl.FECSize16, 0); err != usdl.ErrFrameLengthMismatch {
		t.Errorf("expected ErrFrameLengthMismatch, got %v", err)
	}
}

// Reserved spare bits (bits 50-51) must be zero on decode.
func TestDecode_RejectsHeaderSpares(t *testing.T) {
	frame, err := usdl.NewTransferFrame(1, 1, 0, []byte{0x01},
		usdl.WithoutFECF(),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[6] |= 0x10 // set a reserved spare bit
	if _, err := usdl.DecodeTransferFrame(encoded, 0, 0); err != usdl.ErrInvalidHeaderSpare {
		t.Errorf("expected ErrInvalidHeaderSpare, got %v", err)
	}
}
