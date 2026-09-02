package cfdp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

func testHeader() *cfdp.PDUHeader {
	return &cfdp.PDUHeader{
		Direction:      cfdp.TowardReceiver,
		Acknowledged:   true,
		Source:         cfdp.EntityID{Value: 1, Width: 1},
		TransactionSeq: cfdp.EntityID{Value: 42, Width: 2},
		Destination:    cfdp.EntityID{Value: 2, Width: 1},
	}
}

func TestPDUHeaderRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*cfdp.PDUHeader)
	}{
		{"file directive, acknowledged", func(*cfdp.PDUHeader) {}},
		{"file data", func(h *cfdp.PDUHeader) { h.IsFileData = true }},
		{"unacknowledged", func(h *cfdp.PDUHeader) { h.Acknowledged = false }},
		{"toward sender", func(h *cfdp.PDUHeader) { h.Direction = cfdp.TowardSender }},
		{"CRC flag", func(h *cfdp.PDUHeader) { h.CRCFlag = true }},
		{"large file", func(h *cfdp.PDUHeader) { h.LargeFile = true }},
		{"segmentation control", func(h *cfdp.PDUHeader) { h.SegmentationControl = true }},
		{"segment metadata", func(h *cfdp.PDUHeader) { h.SegmentMetadataFlag = true }},
		{"wide entity IDs", func(h *cfdp.PDUHeader) {
			h.Source = cfdp.EntityID{Value: 0x0102030405060708, Width: 8}
			h.Destination = cfdp.EntityID{Value: 0x0807060504030201, Width: 8}
		}},
		{"wide sequence number", func(h *cfdp.PDUHeader) {
			h.TransactionSeq = cfdp.EntityID{Value: 0xFFFFFFFF, Width: 4}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHeader()
			tt.mutate(h)
			h.DataLength = 7

			encoded, err := h.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != h.Size() {
				t.Fatalf("encoded %d octets, Size() says %d", len(encoded), h.Size())
			}

			got, consumed, err := cfdp.DecodePDUHeader(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if consumed != len(encoded) {
				t.Errorf("consumed %d, want %d", consumed, len(encoded))
			}
			if got.IsFileData != h.IsFileData {
				t.Errorf("IsFileData = %t, want %t", got.IsFileData, h.IsFileData)
			}
			if got.Acknowledged != h.Acknowledged {
				t.Errorf("Acknowledged = %t, want %t", got.Acknowledged, h.Acknowledged)
			}
			if got.Direction != h.Direction {
				t.Errorf("Direction = %v, want %v", got.Direction, h.Direction)
			}
			if got.CRCFlag != h.CRCFlag {
				t.Errorf("CRCFlag = %t, want %t", got.CRCFlag, h.CRCFlag)
			}
			if got.LargeFile != h.LargeFile {
				t.Errorf("LargeFile = %t, want %t", got.LargeFile, h.LargeFile)
			}
			if got.SegmentationControl != h.SegmentationControl {
				t.Errorf("SegmentationControl = %t, want %t", got.SegmentationControl, h.SegmentationControl)
			}
			if got.SegmentMetadataFlag != h.SegmentMetadataFlag {
				t.Errorf("SegmentMetadataFlag = %t, want %t", got.SegmentMetadataFlag, h.SegmentMetadataFlag)
			}
			if got.Source.Value != h.Source.Value || got.Source.Width != h.Source.Width {
				t.Errorf("Source = %+v, want %+v", got.Source, h.Source)
			}
			if got.TransactionSeq.Value != h.TransactionSeq.Value {
				t.Errorf("TransactionSeq = %d, want %d", got.TransactionSeq.Value, h.TransactionSeq.Value)
			}
			if got.Destination.Value != h.Destination.Value {
				t.Errorf("Destination = %d, want %d", got.Destination.Value, h.Destination.Value)
			}
		})
	}
}

func TestPDUHeaderVersionBits(t *testing.T) {
	// Table 5-1: version is binary '001' in the top 3 bits.
	h := testHeader()
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := encoded[0] >> 5; got != cfdp.Version {
		t.Errorf("version bits = %03b, want 001", got)
	}
}

func TestPDUHeaderTransmissionModeIsInverted(t *testing.T) {
	// Table 5-1: '0' means acknowledged, '1' means unacknowledged. Getting
	// this backwards silently swaps Class 1 and Class 2.
	ack := testHeader()
	ack.Acknowledged = true
	encodedAck, err := ack.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encodedAck[0]&(1<<2) != 0 {
		t.Error("acknowledged mode set the transmission mode bit; table 5-1 wants it clear")
	}

	unack := testHeader()
	unack.Acknowledged = false
	encodedUnack, err := unack.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if encodedUnack[0]&(1<<2) == 0 {
		t.Error("unacknowledged mode left the transmission mode bit clear; table 5-1 wants it set")
	}
}

func TestPDUHeaderRejectsWrongVersion(t *testing.T) {
	h := testHeader()
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] = encoded[0]&0x1F | (0 << 5) // version 000

	if _, _, err := cfdp.DecodePDUHeader(encoded); !errors.Is(err, cfdp.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestPDUHeaderShortInput(t *testing.T) {
	h := testHeader()
	encoded, err := h.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(encoded); cut++ {
		if _, _, err := cfdp.DecodePDUHeader(encoded[:cut]); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}

func TestEntityIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      cfdp.EntityID
		wantErr error
	}{
		{"one octet", cfdp.EntityID{Value: 255, Width: 1}, nil},
		{"eight octets", cfdp.EntityID{Value: 1 << 60, Width: 8}, nil},
		{"width zero", cfdp.EntityID{Value: 1, Width: 0}, cfdp.ErrInvalidEntityIDWidth},
		{"width nine", cfdp.EntityID{Value: 1, Width: 9}, cfdp.ErrInvalidEntityIDWidth},
		{"value overflows width", cfdp.EntityID{Value: 256, Width: 1}, cfdp.ErrEntityIDOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewEntityIDPicksMinimalWidth(t *testing.T) {
	tests := []struct {
		value uint64
		want  int
	}{
		{0, 1}, {255, 1}, {256, 2}, {65535, 2}, {65536, 3},
	}
	for _, tt := range tests {
		if got := cfdp.NewEntityID(tt.value); got.Width != tt.want {
			t.Errorf("NewEntityID(%d).Width = %d, want %d", tt.value, got.Width, tt.want)
		}
	}
}

func TestPDUCRCRoundTrip(t *testing.T) {
	h := testHeader()
	h.CRCFlag = true
	pdu := &cfdp.PDU{Header: h, Data: []byte{0x04, 0x00, 0xDE, 0xAD, 0xBE, 0xEF, 0, 0, 0, 8}}

	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatal(err)
	}

	got, err := cfdp.DecodePDU(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(got.Data, pdu.Data) {
		t.Errorf("data = %x, want %x", got.Data, pdu.Data)
	}
	// Clause 4.1.3.2: the CRC length counts toward the data field length.
	if int(got.Header.DataLength) != len(pdu.Data)+cfdp.CRCSize {
		t.Errorf("DataLength = %d, want %d", got.Header.DataLength, len(pdu.Data)+cfdp.CRCSize)
	}
}

func TestPDUCRCDetectsCorruption(t *testing.T) {
	h := testHeader()
	h.CRCFlag = true
	pdu := &cfdp.PDU{Header: h, Data: []byte{0x04, 0x00, 0xDE, 0xAD}}

	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// Every byte position must be covered by the CRC.
	for i := 0; i < len(encoded)-cfdp.CRCSize; i++ {
		corrupt := append([]byte{}, encoded...)
		corrupt[i] ^= 0x01
		if _, err := cfdp.DecodePDU(corrupt); err == nil {
			t.Errorf("corrupting octet %d went undetected", i)
		}
	}
}

func TestLVRoundTrip(t *testing.T) {
	tests := [][]byte{nil, []byte("a"), []byte("some/path/file.dat"), bytes.Repeat([]byte{0xAA}, 255)}
	for _, value := range tests {
		lv := cfdp.LV{Value: value}
		encoded, err := lv.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, n, err := cfdp.DecodeLV(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(encoded) {
			t.Errorf("consumed %d, want %d", n, len(encoded))
		}
		if !bytes.Equal(got.Value, value) {
			t.Errorf("value = %x, want %x", got.Value, value)
		}
	}
}

func TestLVTooLong(t *testing.T) {
	lv := cfdp.LV{Value: bytes.Repeat([]byte{0}, 256)}
	if _, err := lv.Encode(); !errors.Is(err, cfdp.ErrValueTooLong) {
		t.Errorf("error = %v, want ErrValueTooLong", err)
	}
}

func TestTLVRoundTrip(t *testing.T) {
	tlv := cfdp.TLV{Type: cfdp.TLVMessageToUser, Value: []byte("hello")}
	encoded, err := tlv.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, n, err := cfdp.DecodeTLV(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d, want %d", n, len(encoded))
	}
	if got.Type != tlv.Type || !bytes.Equal(got.Value, tlv.Value) {
		t.Errorf("got %+v, want %+v", got, tlv)
	}
}

func TestDecodeTLVsRejectsTrailingPartial(t *testing.T) {
	tlv := cfdp.TLV{Type: cfdp.TLVEntityID, Value: []byte{1}}
	encoded, err := tlv.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// One extra octet cannot be a whole TLV.
	if _, err := cfdp.DecodeTLVs(append(encoded, 0x06)); err == nil {
		t.Error("expected an error for a trailing partial TLV")
	}
}
