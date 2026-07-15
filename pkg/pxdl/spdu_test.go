package pxdl_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/pxdl"
)

func TestPLCWRoundTrip(t *testing.T) {
	p := &pxdl.PLCW{
		RetransmitFlag:        true,
		PCID:                  1,
		ExpeditedFrameCounter: 5,
		ReportValue:           0xA3,
	}
	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != pxdl.FixedSPDUSize {
		t.Fatalf("encoded %d octets, want %d", len(encoded), pxdl.FixedSPDUSize)
	}

	got, err := pxdl.DecodePLCW(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReportValue != 0xA3 {
		t.Errorf("report value = %#x, want 0xA3", got.ReportValue)
	}
	if !got.RetransmitFlag {
		t.Error("retransmit flag lost")
	}
	if got.PCID != 1 {
		t.Errorf("PCID = %d, want 1", got.PCID)
	}
	if got.ExpeditedFrameCounter != 5 {
		t.Errorf("expedited frame counter = %d, want 5", got.ExpeditedFrameCounter)
	}
}

func TestPLCWFormatAndTypeBits(t *testing.T) {
	// §3.2.4.3.1: format ID '1' identifies a fixed-length SPDU, and type ID
	// '0' identifies it as a PLCW.
	p := &pxdl.PLCW{ReportValue: 1}
	encoded, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if format := encoded[0] >> 7 & 0x01; format != pxdl.SPDUFormatFixed {
		t.Errorf("format bit = %d, want 1 for fixed length", format)
	}
	if typeID := encoded[0] >> 6 & 0x01; typeID != pxdl.FixedTypePLCW {
		t.Errorf("type bit = %d, want 0 for a PLCW", typeID)
	}
}

func TestPLCWRejectsVariableFormat(t *testing.T) {
	// A leading zero means a variable-length SPDU, not a PLCW.
	if _, err := pxdl.DecodePLCW([]byte{0x00, 0x00}); !errors.Is(err, pxdl.ErrInvalidSPDU) {
		t.Errorf("error = %v, want ErrInvalidSPDU", err)
	}
}

func TestVariableSPDURoundTrip(t *testing.T) {
	s := &pxdl.VariableSPDU{TypeID: 3, Data: []byte{1, 2, 3, 4}}
	encoded, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, consumed, err := pxdl.DecodeVariableSPDU(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != len(encoded) {
		t.Errorf("consumed %d, want %d", consumed, len(encoded))
	}
	if got.TypeID != 3 {
		t.Errorf("type = %d, want 3", got.TypeID)
	}
	if !bytes.Equal(got.Data, s.Data) {
		t.Errorf("data = %x, want %x", got.Data, s.Data)
	}
}

func TestVariableSPDULengthIsNotMinusOne(t *testing.T) {
	// §3.2.4.2.2 a) 3) calls this out: "Data Field Length is not a 'length
	// minus one' field." Everything else in CCSDS goes the other way, so it
	// is worth pinning.
	s := &pxdl.VariableSPDU{TypeID: 1, Data: []byte{0xAA, 0xBB, 0xCC}}
	encoded, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if length := encoded[0] & 0x0F; int(length) != len(s.Data) {
		t.Errorf("length field = %d, want %d (the actual count)", length, len(s.Data))
	}
}

func TestVariableSPDUEmptyData(t *testing.T) {
	// §3.2.4.2.2 b): 0 to 15 octets, so empty is legal.
	s := &pxdl.VariableSPDU{TypeID: 2}
	encoded, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 1 {
		t.Fatalf("encoded %d octets, want 1 for an empty data field", len(encoded))
	}
	got, _, err := pxdl.DecodeVariableSPDU(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 0 {
		t.Errorf("data = %x, want empty", got.Data)
	}
}

func TestVariableSPDUSizeLimit(t *testing.T) {
	// The 4-bit length field tops out at 15.
	if _, err := (&pxdl.VariableSPDU{Data: make([]byte, 15)}).Encode(); err != nil {
		t.Errorf("15 octets was rejected: %v", err)
	}
	if _, err := (&pxdl.VariableSPDU{Data: make([]byte, 16)}).Encode(); !errors.Is(err, pxdl.ErrSPDUDataTooLarge) {
		t.Errorf("error = %v, want ErrSPDUDataTooLarge", err)
	}
}

func TestSPDUsAreSelfDelimiting(t *testing.T) {
	// §3.2.4.1: SPDUs are self-identifying and self-delimiting, so a run of
	// mixed fixed and variable ones decodes without a count.
	spdus := []pxdl.SPDU{
		{PLCW: &pxdl.PLCW{ReportValue: 10}},
		{Variable: &pxdl.VariableSPDU{TypeID: 1, Data: []byte{1, 2}}},
		{PLCW: &pxdl.PLCW{ReportValue: 20, RetransmitFlag: true}},
		{Variable: &pxdl.VariableSPDU{TypeID: 7}},
	}

	encoded, err := pxdl.EncodeSPDUs(spdus)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pxdl.DecodeSPDUs(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("decoded %d SPDUs, want 4", len(got))
	}
	if got[0].PLCW == nil || got[0].PLCW.ReportValue != 10 {
		t.Error("first PLCW did not survive")
	}
	if got[1].Variable == nil || got[1].Variable.TypeID != 1 {
		t.Error("first variable SPDU did not survive")
	}
	if got[2].PLCW == nil || !got[2].PLCW.RetransmitFlag {
		t.Error("second PLCW did not survive")
	}
	if got[3].Variable == nil || got[3].Variable.TypeID != 7 {
		t.Error("second variable SPDU did not survive")
	}
}

func TestSupervisoryFrameCarriesSPDUs(t *testing.T) {
	spdus := []pxdl.SPDU{
		{PLCW: &pxdl.PLCW{ReportValue: 99, PCID: 1}},
	}
	body, err := pxdl.EncodeSPDUs(spdus)
	if err != nil {
		t.Fatal(err)
	}

	f, err := pxdl.NewSupervisoryFrame(42, 0, body)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := pxdl.DecodeTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := got.SPDUs()
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].PLCW == nil {
		t.Fatal("the PLCW did not survive the frame round trip")
	}
	if parsed[0].PLCW.ReportValue != 99 {
		t.Errorf("report value = %d, want 99", parsed[0].PLCW.ReportValue)
	}
}

func TestUserFrameHasNoSPDUs(t *testing.T) {
	f, err := pxdl.NewTransferFrame(1, 0, []byte("user data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.SPDUs(); !errors.Is(err, pxdl.ErrNotSupervisoryFrame) {
		t.Errorf("error = %v, want ErrNotSupervisoryFrame", err)
	}
}
