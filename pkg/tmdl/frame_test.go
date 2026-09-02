package tmdl_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	ccsdscrc "github.com/ravisuhag/astro/pkg/crc"
	"github.com/ravisuhag/astro/pkg/tmdl"
)

// --- Primary Header Tests ---

func TestPrimaryHeader_EncodeDecode(t *testing.T) {
	header := tmdl.PrimaryHeader{
		VersionNumber:    0b00,
		SpacecraftID:     933,
		VirtualChannelID: 2,
		OCFFlag:          true,
		MCFrameCount:     15,
		VCFrameCount:     8,
		FSHFlag:          false,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  0b11,
		FirstHeaderPtr:   1024,
	}

	encoded, err := header.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify the header length
	if len(encoded) != 6 {
		t.Fatalf("Expected header length 6, got %d", len(encoded))
	}

	// Decode the encoded header
	var decoded tmdl.PrimaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify fields match
	if header != decoded {
		t.Errorf("Header mismatch:\n  expected: %+v\n  got:      %+v", header, decoded)
	}
}

func TestPrimaryHeader_Validate(t *testing.T) {
	tests := []struct {
		name    string
		header  tmdl.PrimaryHeader
		wantErr bool
	}{
		{
			name: "valid header",
			header: tmdl.PrimaryHeader{
				VersionNumber:    0,
				SpacecraftID:     100,
				VirtualChannelID: 3,
				SegmentLengthID:  0b11,
			},
		},
		{
			name: "invalid version",
			header: tmdl.PrimaryHeader{
				VersionNumber:   1,
				SegmentLengthID: 0b11,
			},
			wantErr: true,
		},
		{
			name: "invalid SCID",
			header: tmdl.PrimaryHeader{
				SpacecraftID:    0x0400,
				SegmentLengthID: 0b11,
			},
			wantErr: true,
		},
		{
			name: "invalid VCID",
			header: tmdl.PrimaryHeader{
				VirtualChannelID: 8,
				SegmentLengthID:  0b11,
			},
			wantErr: true,
		},
		{
			name: "packet order without sync",
			header: tmdl.PrimaryHeader{
				PacketOrderFlag: true,
				SyncFlag:        false,
				SegmentLengthID: 0b11,
			},
			wantErr: true,
		},
		{
			name: "segment length without sync",
			header: tmdl.PrimaryHeader{
				SyncFlag:        false,
				SegmentLengthID: 0b01,
			},
			wantErr: true,
		},
		{
			// With the Synchronization Flag set, the First Header Pointer,
			// Packet Order Flag, and Segment Length ID are undefined by
			// CCSDS 132.0-B-3 (notes under clause 4.1.2.7.4 to clause 4.1.2.7.6) and are
			// handed to the VCA service user as the VCA Status Fields of
			// Clause 3.4.2.3, so any value is legal.
			name: "VCA status fields with sync flag",
			header: tmdl.PrimaryHeader{
				SyncFlag:        true,
				PacketOrderFlag: true,
				SegmentLengthID: 0b01,
				FirstHeaderPtr:  0x0000,
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

func TestPrimaryHeader_EncodeDecode_ValidHeader(t *testing.T) {
	// Build a valid header and encode it
	h := tmdl.PrimaryHeader{
		VersionNumber:    0,
		SpacecraftID:     0x3A5,
		VirtualChannelID: 5,
		OCFFlag:          true,
		MCFrameCount:     0xAB,
		VCFrameCount:     0xCD,
		FSHFlag:          true,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  0b11,
		FirstHeaderPtr:   0x456,
	}
	data, err := h.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var h2 tmdl.PrimaryHeader
	if err := h2.Decode(data); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if h != h2 {
		t.Errorf("Round-trip mismatch:\n  before: %+v\n  after:  %+v", h, h2)
	}

	// Verify known bit layout by checking hex encoding matches expectations
	got := hex.EncodeToString(data)
	t.Logf("Encoded header: %s", got)
}

// --- Secondary Header Tests ---

func TestSecondaryHeader_EncodeDecode(t *testing.T) {
	sh := tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  3,
		DataField:     []byte{0xAA, 0xBB, 0xCC},
	}

	encoded, err := sh.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	var decoded tmdl.SecondaryHeader
	if err := decoded.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.VersionNumber != sh.VersionNumber ||
		decoded.HeaderLength != sh.HeaderLength ||
		!bytes.Equal(decoded.DataField, sh.DataField) {
		t.Errorf("SecondaryHeader mismatch:\n  expected: %+v\n  got:      %+v", sh, decoded)
	}
}

func TestSecondaryHeader_MaxLength63(t *testing.T) {
	// 63 data octets plus the identification octet is the 64-octet maximum of
	// CCSDS 132.0-B-3 clause 4.1.3.2 and ECSS-E-ST-50-03C 5.3.1c.
	sh := tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  63,
		DataField:     make([]byte, 63),
	}
	encoded, err := sh.Encode()
	if err != nil {
		t.Errorf("Expected no error for max length 63, got %v", err)
	}
	if len(encoded) != tmdl.MaxSecondaryHeaderSize {
		t.Errorf("encoded %d octets, want %d", len(encoded), tmdl.MaxSecondaryHeaderSize)
	}
}

// TestSecondaryHeader_RejectsOversize covers ECSS-E-ST-50-03C 5.3.1c: the
// whole secondary header, identification octet included, may not exceed 64
// octets. A 64-octet data field would make 65.
func TestSecondaryHeader_RejectsOversize(t *testing.T) {
	sh := tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  64,
		DataField:     make([]byte, 64),
	}
	if _, err := sh.Encode(); err == nil {
		t.Error("Encode() accepted a 65-octet secondary header")
	}
}

func TestSecondaryHeader_MinLength1(t *testing.T) {
	sh := tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  1,
		DataField:     []byte{0xFF},
	}
	_, err := sh.Encode()
	if err != nil {
		t.Errorf("Expected no error for min length 1, got %v", err)
	}
}

func TestSCIDBoundaries(t *testing.T) {
	for _, scid := range []uint16{0, 1, 0x03FF} {
		h := tmdl.PrimaryHeader{SpacecraftID: scid, SegmentLengthID: 0b11}
		if err := h.Validate(); err != nil {
			t.Errorf("SCID %d should be valid: %v", scid, err)
		}
	}
}

func TestVCIDBoundaries(t *testing.T) {
	for _, vcid := range []uint8{0, 1, 7} {
		h := tmdl.PrimaryHeader{VirtualChannelID: vcid, SegmentLengthID: 0b11}
		if err := h.Validate(); err != nil {
			t.Errorf("VCID %d should be valid: %v", vcid, err)
		}
	}
}

func TestInvalidSCID(t *testing.T) {
	h := tmdl.PrimaryHeader{SpacecraftID: 0x0400, SegmentLengthID: 0b11}
	if err := h.Validate(); !errors.Is(err, tmdl.ErrInvalidSpacecraftID) {
		t.Errorf("Expected ErrInvalidSpacecraftID, got %v", err)
	}
}

func TestInvalidVCID(t *testing.T) {
	h := tmdl.PrimaryHeader{VirtualChannelID: 8, SegmentLengthID: 0b11}
	if err := h.Validate(); !errors.Is(err, tmdl.ErrInvalidVCID) {
		t.Errorf("Expected ErrInvalidVCID, got %v", err)
	}
}

func TestSecondaryHeader_Validate(t *testing.T) {
	tests := []struct {
		name    string
		sh      tmdl.SecondaryHeader
		wantErr bool
	}{
		{
			name: "valid",
			sh:   tmdl.SecondaryHeader{VersionNumber: 0, HeaderLength: 2, DataField: []byte{0xAA, 0xBB}},
		},
		{
			name:    "invalid version",
			sh:      tmdl.SecondaryHeader{VersionNumber: 1, HeaderLength: 1, DataField: []byte{0xAA}},
			wantErr: true,
		},
		{
			name:    "length mismatch",
			sh:      tmdl.SecondaryHeader{VersionNumber: 0, HeaderLength: 5, DataField: []byte{0xAA}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.sh.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Transfer Frame Tests ---

func TestHeaderEncoding(t *testing.T) {
	header := tmdl.PrimaryHeader{
		VersionNumber:    0b00,
		SpacecraftID:     933,
		VirtualChannelID: 2,
		OCFFlag:          true,
		FSHFlag:          false,
		MCFrameCount:     15,
		VCFrameCount:     8,
		SyncFlag:         false,
		PacketOrderFlag:  false,
		SegmentLengthID:  0b11,
		FirstHeaderPtr:   1024,
	}

	// MCID = TFVN (2 bits) << 10 | SCID (10 bits)
	expectedMCID := uint16(header.VersionNumber)<<10 | header.SpacecraftID
	// GVCID = MCID (12 bits) << 3 | VCID (3 bits)
	expectedGVCID := expectedMCID<<3 | uint16(header.VirtualChannelID)

	if header.MCID() != expectedMCID {
		t.Errorf("Expected MCID: %d, Got: %d", expectedMCID, header.MCID())
	}

	if header.GVCID() != expectedGVCID {
		t.Errorf("Expected GVCID: %d, Got: %d", expectedGVCID, header.GVCID())
	}
}

func TestNewTMTransferFrame(t *testing.T) {
	data := make([]byte, 65535)
	secondaryHeader := []byte{0xAA, 0xBB}
	ocf := []byte{0x00, 0x00, 0x00, 0xFF}

	frame, err := tmdl.NewTMTransferFrame(933, 2, data, secondaryHeader, ocf)
	if err != nil {
		t.Fatalf("Failed to create TM Transfer Frame: %v", err)
	}

	if frame.Header.SpacecraftID != 933 {
		t.Errorf("Expected SCID 933, Got: %d", frame.Header.SpacecraftID)
	}
}

func TestNewTMTransferFrame_SecondaryHeaderRoundTrip(t *testing.T) {
	data := []byte("payload")
	secondaryHeaderData := []byte{0xAA, 0xBB, 0xCC}

	frame, err := tmdl.NewTMTransferFrame(933, 1, data, secondaryHeaderData, nil)
	if err != nil {
		t.Fatalf("Failed to create frame: %v", err)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.SecondaryHeader.DataField, secondaryHeaderData) {
		t.Errorf("Secondary header data mismatch: expected %x, got %x",
			secondaryHeaderData, decoded.SecondaryHeader.DataField)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("Data field mismatch: expected %x, got %x", data, decoded.DataField)
	}
}

func TestFrameEncoding(t *testing.T) {
	data := []byte("Telemetry Data")
	frame, _ := tmdl.NewTMTransferFrame(1285, 3, data, nil, nil)
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	expectedLength := 6 + len(data) + 2 // PrimaryHeader + Data + CRC
	if len(encodedFrame) != expectedLength {
		t.Errorf("Encoded frame length mismatch: expected %d, got %d", expectedLength, len(encodedFrame))
	}
}

func TestFrameDecoding(t *testing.T) {
	encodedFrame := []byte{
		0x3A, 0x5A, 0x00, 0x00, 0x18, 0x00, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
	}

	crc := ccsdscrc.ComputeCRC16(encodedFrame)
	crcBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(crcBytes, crc)
	encodedFrame = append(encodedFrame, crcBytes...)

	frame, err := tmdl.DecodeTMTransferFrame(encodedFrame)
	if err != nil {
		t.Fatalf("Failed to decode TM Transfer Frame: %v", err)
	}

	expectedData := []byte("Telemetry Data")
	if !bytes.Equal(frame.DataField, expectedData) {
		t.Errorf("Expected data: %s, got: %s", expectedData, frame.DataField)
	}
}

func TestCRCValidation(t *testing.T) {
	data := []byte("Test Data")
	expectedCRC := ccsdscrc.ComputeCRC16(data)

	modifiedData := append(data, 0x01)
	modifiedCRC := ccsdscrc.ComputeCRC16(modifiedData)

	if expectedCRC == modifiedCRC {
		t.Errorf("CRC did not detect error; expected change but got identical CRC")
	}
}

func TestFrameRoundTrip(t *testing.T) {
	data := []byte("Round Trip Test  ")
	frame, _ := tmdl.NewTMTransferFrame(933, 5, data, nil, nil)
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decodedFrame, err := tmdl.DecodeTMTransferFrame(encodedFrame)
	if err != nil {
		t.Fatalf("Failed to decode frame in round-trip test: %v", err)
	}

	if frame.Header != decodedFrame.Header {
		t.Errorf("Header mismatch: expected %+v, got %+v", frame.Header, decodedFrame.Header)
	}
	if frame.FrameErrorControl != decodedFrame.FrameErrorControl {
		t.Errorf("Frame error control mismatch: expected %x, got %x",
			frame.FrameErrorControl, decodedFrame.FrameErrorControl)
	}
	if !bytes.Equal(frame.DataField, decodedFrame.DataField) {
		t.Errorf("Data field mismatch: expected %x, got %x", frame.DataField, decodedFrame.DataField)
	}
}

func TestMalformedFrame(t *testing.T) {
	corruptFrame := []byte{0x68, 0x05}
	_, err := tmdl.DecodeTMTransferFrame(corruptFrame)
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("Expected ErrDataTooShort, got %v", err)
	}

	validFrame := []byte{
		0x68, 0x05, 0x03, 0x00, 0x16, // Header
		'T', 'e', 'l', 'e', 'm', 'e', 't', 'r', 'y', ' ', 'D', 'a', 't', 'a', // Data
		0xA1, 0xB2, // Mocked CRC
	}
	validFrame[len(validFrame)-1] ^= 0xFF

	_, err = tmdl.DecodeTMTransferFrame(validFrame)
	if err == nil {
		t.Error("Expected CRC error but decoding succeeded")
	}
}

// TestSecondaryHeaderLength_CCSDS pins the length field to the value that goes
// on the wire, not to a round trip.
//
// CCSDS 132.0-B-3 clause 4.1.3.2.2.3 and ECSS-E-ST-50-03C 5.3.2.3c both define the
// field as the TOTAL secondary header length in octets minus one, where the
// total counts the identification octet. Three data octets make a four-octet
// header, so the field reads 3.
//
// This test previously asserted 2 and so locked in an off-by-one. Encoder and
// decoder agreed with each other, which is exactly why a round-trip test could
// never have caught it, the same shape as the PN randomizer defect in
// pkg/tmsc. Assert the octet, not the symmetry.
func TestSecondaryHeaderLength_CCSDS(t *testing.T) {
	shData := []byte{0xAA, 0xBB, 0xCC}
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("data"), shData, nil)
	if err != nil {
		t.Fatal(err)
	}

	if frame.SecondaryHeader.HeaderLength != 3 {
		t.Errorf("HeaderLength = %d, want 3 (total octets minus one)", frame.SecondaryHeader.HeaderLength)
	}
	if got := frame.SecondaryHeader.TotalLength(); got != 4 {
		t.Errorf("TotalLength() = %d, want 4", got)
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if wireLen := encoded[6] & 0x3F; wireLen != 3 {
		t.Errorf("wire length field = %d, want 3", wireLen)
	}
	// The data field must start at octet 6+4 = 10, which is what a conforming
	// receiver computes from the field.
	if got := 6 + 1 + int(encoded[6]&0x3F); got != 10 {
		t.Errorf("a receiver would start the data field at octet %d, want 10", got)
	}
}

// TestSetDataFieldDerivesLength covers the helper that keeps the two in step.
func TestSetDataFieldDerivesLength(t *testing.T) {
	var sh tmdl.SecondaryHeader
	if err := sh.SetDataField([]byte{1, 2, 3, 4, 5}); err != nil {
		t.Fatalf("SetDataField() = %v", err)
	}
	if sh.HeaderLength != 5 {
		t.Errorf("HeaderLength = %d, want 5", sh.HeaderLength)
	}
	if err := sh.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
	if err := sh.SetDataField(make([]byte, 64)); err == nil {
		t.Error("SetDataField accepted a data field that overflows the 64-octet header")
	}
}

func TestMinimumFrameSize(t *testing.T) {
	// 7 bytes: too short (need 6 header + 2 CRC minimum)
	_, err := tmdl.DecodeTMTransferFrame([]byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x00, 0x00})
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("7-byte input: got %v, want ErrDataTooShort", err)
	}

	// 8 bytes with valid CRC: smallest valid frame
	header := []byte{0x00, 0x00, 0x00, 0x00, 0x18, 0x00}
	crc := ccsdscrc.ComputeCRC16(header)
	frame := make([]byte, 8)
	copy(frame, header)
	binary.BigEndian.PutUint16(frame[6:], crc)
	if _, err := tmdl.DecodeTMTransferFrame(frame); err != nil {
		t.Errorf("8-byte frame: got %v, want nil", err)
	}
}

func TestSecondaryHeaderDecodeSelfDescribing(t *testing.T) {
	data := []byte("payload")
	shData := []byte{0x01, 0x02}

	frame, err := tmdl.NewTMTransferFrame(933, 1, data, shData, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded.SecondaryHeader.DataField, shData) {
		t.Errorf("SecondaryHeader.DataField = %x, want %x", decoded.SecondaryHeader.DataField, shData)
	}
	if !bytes.Equal(decoded.DataField, data) {
		t.Errorf("DataField = %x, want %x", decoded.DataField, data)
	}
}

func TestOCFInsufficientData(t *testing.T) {
	header := tmdl.PrimaryHeader{
		SpacecraftID:     100,
		VirtualChannelID: 1,
		OCFFlag:          true,
		SegmentLengthID:  0b11,
	}
	hBytes, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// 2 bytes of data field, not enough for the 4-byte OCF the flag promises
	withoutCRC := append(hBytes, 0x01, 0x02)
	crc := ccsdscrc.ComputeCRC16(withoutCRC)
	frame := binary.BigEndian.AppendUint16(withoutCRC, crc)

	_, err = tmdl.DecodeTMTransferFrame(frame)
	if !errors.Is(err, tmdl.ErrDataTooShort) {
		t.Errorf("got %v, want ErrDataTooShort", err)
	}
}

func TestSecondaryHeaderValidateConsistency(t *testing.T) {
	sh := &tmdl.SecondaryHeader{
		VersionNumber: 0,
		HeaderLength:  10,
		DataField:     []byte{0x01, 0x02, 0x03},
	}
	if err := sh.Validate(); err == nil {
		t.Error("Validate() = nil, want error for HeaderLength/DataField mismatch")
	}
}

func TestFirstHeaderPtr_WithSecondaryHeader(t *testing.T) {
	shData := []byte{0xAA, 0xBB, 0xCC}
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("payload"), shData, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Per CCSDS 132.0-B-3 clause 4.1.2.7.3, FirstHeaderPtr is relative to the
	// Transfer Frame Data Field (after secondary header), not the frame payload.
	// When the first packet starts at byte 0 of the Data Field, FirstHeaderPtr = 0.
	if frame.Header.FirstHeaderPtr != 0 {
		t.Errorf("FirstHeaderPtr = %d, want 0 (first packet at start of Data Field)", frame.Header.FirstHeaderPtr)
	}
}

func TestDecodedFrameDoesNotAliasInput(t *testing.T) {
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("original"), nil, []byte{0x01, 0x02, 0x03, 0x04})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatal(err)
	}

	// Zero out the encoded buffer, decoded fields should be unaffected
	for i := range encoded {
		encoded[i] = 0
	}

	if !bytes.Equal(decoded.DataField, []byte("original")) {
		t.Errorf("DataField corrupted after input modification: got %q", decoded.DataField)
	}
	if !bytes.Equal(decoded.OperationalControl, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("OperationalControl corrupted after input modification: got %x", decoded.OperationalControl)
	}
}

func TestNewIdleFrame(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	capacity := config.DataFieldCapacity(0)

	frame, err := tmdl.NewIdleFrame(933, 1, config)
	if err != nil {
		t.Fatal(err)
	}

	// CCSDS 132.0-B-3 clause 4.1.2.7.6.5: a data field of only idle data takes the
	// OID pointer 0x7FE, not the 0x7FF that means "no packet starts here".
	if frame.Header.FirstHeaderPtr != tmdl.FHPOnlyIdleData {
		t.Errorf("FirstHeaderPtr = 0x%04X, want 0x%04X (OID)",
			frame.Header.FirstHeaderPtr, tmdl.FHPOnlyIdleData)
	}
	if !tmdl.IsIdleFrame(frame) {
		t.Error("IsIdleFrame() = false for a frame built by NewIdleFrame")
	}
	if len(frame.DataField) != capacity {
		t.Errorf("DataField len = %d, want %d", len(frame.DataField), capacity)
	}

	// Clause 4.1.4.6.2: the data field carries the mandatory PN sequence, whose
	// first octets the note under clause 4.1.4.6.2.2 gives as FF FF FF FF 6D B6 D8
	// 61 45 1F. Constant fill is what this replaced. It defeats the
	// randomization the sequence exists to provide.
	want := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x6D, 0xB6, 0xD8, 0x61, 0x45, 0x1F}
	if n := min(len(want), len(frame.DataField)); !bytes.Equal(frame.DataField[:n], want[:n]) {
		t.Errorf("DataField[:%d] = % X, want % X", n, frame.DataField[:n], want[:n])
	}

	// Verify CRC is valid via round-trip
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmdl.DecodeTMTransferFrame(encoded); err != nil {
		t.Errorf("Idle frame CRC invalid: %v", err)
	}
}

func TestNewIdleFrame_WithOCF(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasOCF: true, HasFEC: true}
	frame, err := tmdl.NewIdleFrame(933, 1, config)
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Header.OCFFlag {
		t.Error("Expected OCFFlag=true")
	}
	if len(frame.OperationalControl) != 4 {
		t.Errorf("OCF len = %d, want 4", len(frame.OperationalControl))
	}

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	// Total: 6 header + capacity data + 4 OCF + 2 CRC = 28
	if len(encoded) != 28 {
		t.Errorf("Encoded len = %d, want 28", len(encoded))
	}
}

func TestIsIdleFrame(t *testing.T) {
	config := tmdl.ChannelConfig{FrameLength: 28, HasFEC: true}
	idle, err := tmdl.NewIdleFrame(933, 1, config)
	if err != nil {
		t.Fatal(err)
	}
	if !tmdl.IsIdleFrame(idle) {
		t.Error("Expected IsIdleFrame=true for idle frame")
	}

	// VCP frame should not be idle
	vcpFrame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	if tmdl.IsIdleFrame(vcpFrame) {
		t.Error("Expected IsIdleFrame=false for VCP frame")
	}

	// VCA frame (SyncFlag=true, FHP=0x07FF) should not be idle
	vcaFrame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("data"), nil, nil)
	vcaFrame.Header.SyncFlag = true
	vcaFrame.Header.FirstHeaderPtr = 0x07FF
	if tmdl.IsIdleFrame(vcaFrame) {
		t.Error("Expected IsIdleFrame=false for VCA frame (SyncFlag=true)")
	}
}

func TestCRCMismatchRejection(t *testing.T) {
	frame, _ := tmdl.NewTMTransferFrame(933, 1, []byte("test data"), nil, nil)
	encoded, _ := frame.Encode()

	encoded[8] ^= 0x01

	_, err := tmdl.DecodeTMTransferFrame(encoded)
	if err == nil {
		t.Error("Expected CRC mismatch error")
	}
}

func TestFrameFlagCombinations(t *testing.T) {
	tests := []struct {
		name    string
		hasOCF  bool
		hasFSH  bool
		shData  []byte
		ocfData []byte
	}{
		{
			name: "no optional fields",
		},
		{
			name:    "OCF only",
			hasOCF:  true,
			ocfData: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:   "FSH only",
			hasFSH: true,
			shData: []byte{0xAA, 0xBB},
		},
		{
			name:    "OCF + FSH",
			hasOCF:  true,
			hasFSH:  true,
			shData:  []byte{0xAA, 0xBB, 0xCC},
			ocfData: []byte{0x01, 0x02, 0x03, 0x04},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("test payload")
			frame, err := tmdl.NewTMTransferFrame(933, 1, data, tt.shData, tt.ocfData)
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := frame.Encode()
			if err != nil {
				t.Fatal(err)
			}

			decoded, err := tmdl.DecodeTMTransferFrame(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if !bytes.Equal(decoded.DataField, data) {
				t.Errorf("DataField mismatch: got %q, want %q", decoded.DataField, data)
			}

			if tt.hasOCF {
				if !bytes.Equal(decoded.OperationalControl, tt.ocfData) {
					t.Errorf("OCF mismatch: got %x, want %x", decoded.OperationalControl, tt.ocfData)
				}
			}

			if tt.hasFSH {
				if !decoded.Header.FSHFlag {
					t.Fatal("Expected FSHFlag=true")
				}
				if !bytes.Equal(decoded.SecondaryHeader.DataField, tt.shData) {
					t.Errorf("SH data mismatch: got %x, want %x", decoded.SecondaryHeader.DataField, tt.shData)
				}
			}
		})
	}
}

func TestUninitializedFrame(t *testing.T) {
	// Zero-value frame should fail validation since SegmentLengthID must be 0b11 when SyncFlag is 0
	frame := &tmdl.TMTransferFrame{}
	_, err := frame.Encode()
	if !errors.Is(err, tmdl.ErrInvalidSegmentLengthID) {
		t.Errorf("Expected ErrInvalidSegmentLengthID for zero-value frame, got %v", err)
	}

	// Frame with valid defaults should encode successfully
	frame.Header.SegmentLengthID = 0b11
	encodedFrame, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(encodedFrame) == 0 {
		t.Error("Encoded frame should not be zero-length")
	}
}

func TestTMFrame_EncodeRecomputesCRCAfterMutation(t *testing.T) {
	frame, err := tmdl.NewTMTransferFrame(42, 1, []byte("payload"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Stamping counters after construction is the normal caller workflow.
	frame.Header.MCFrameCount = 9
	frame.Header.VCFrameCount = 4

	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tmdl.DecodeTMTransferFrame(encoded)
	if err != nil {
		t.Fatalf("re-encoded frame does not decode: %v", err)
	}
	if decoded.Header.MCFrameCount != 9 {
		t.Errorf("MCFrameCount = %d, want 9", decoded.Header.MCFrameCount)
	}
	if decoded.Header.VCFrameCount != 4 {
		t.Errorf("VCFrameCount = %d, want 4", decoded.Header.VCFrameCount)
	}
}

// TestFECFOptionalUnderReedSolomon covers ECSS-E-ST-50-03C 5.6.1b and its
// NOTE: the Frame Error Control Field is mandatory when the frame is not
// Reed-Solomon encoded, and optional when it travels inside a code block,
// which already protects it. Clause 5.6.1c then requires the choice to hold for the
// whole physical channel.
//
// Before this, Encode always appended the field and DecodeTMTransferFrame
// always demanded it, so a Reed-Solomon mission that omitted it could not use
// the package at all.
func TestFECFOptionalUnderReedSolomon(t *testing.T) {
	// FrameLength is left unset here: these frames are shorter than any
	// realistic fixed length, and the codec now rejects a mismatch.
	withFEC := tmdl.ChannelConfig{HasFEC: true}
	noFEC := tmdl.ChannelConfig{HasFEC: false}

	frame, err := tmdl.NewTMTransferFrame(933, 2, []byte("payload"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	withBytes, err := frame.EncodeWithConfig(withFEC)
	if err != nil {
		t.Fatalf("EncodeWithConfig(HasFEC) = %v", err)
	}
	withoutBytes, err := frame.EncodeWithConfig(noFEC)
	if err != nil {
		t.Fatalf("EncodeWithConfig(no FEC) = %v", err)
	}

	if len(withBytes)-len(withoutBytes) != 2 {
		t.Errorf("with FECF is %d octets and without is %d; the difference should be exactly 2",
			len(withBytes), len(withoutBytes))
	}

	// Each decodes under its own configuration.
	back, err := tmdl.DecodeTMTransferFrameWithConfig(withoutBytes, noFEC)
	if err != nil {
		t.Fatalf("decoding a frame with no FECF = %v", err)
	}
	if string(back.DataField) != "payload" {
		t.Errorf("data field = %q, want %q", back.DataField, "payload")
	}

	back, err = tmdl.DecodeTMTransferFrameWithConfig(withBytes, withFEC)
	if err != nil {
		t.Fatalf("decoding a frame with a FECF = %v", err)
	}
	if string(back.DataField) != "payload" {
		t.Errorf("data field = %q, want %q", back.DataField, "payload")
	}

	// The default entry points keep the field, which is the mandatory case.
	defaultBytes, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(defaultBytes) != len(withBytes) {
		t.Error("Encode() no longer appends the error control field")
	}
}

// TestFECFMismatchIsDetected checks the configuration is not merely ignored:
// reading a no-FECF frame as though it had one must not silently succeed.
func TestFECFMismatchIsDetected(t *testing.T) {
	noFEC := tmdl.ChannelConfig{HasFEC: false}

	frame, err := tmdl.NewTMTransferFrame(933, 2, []byte("payload"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := frame.EncodeWithConfig(noFEC)
	if err != nil {
		t.Fatal(err)
	}

	// The last two octets are payload, not a checksum, so verifying them
	// should fail rather than quietly truncating the data field.
	if _, err := tmdl.DecodeTMTransferFrame(raw); err == nil {
		t.Error("a frame with no FECF decoded cleanly as though it had one")
	}
}

// TestChannelConfigValidate covers the ECSS-E-ST-50-03C 5.1b frame ceiling.
func TestChannelConfigValidate(t *testing.T) {
	if err := (tmdl.ChannelConfig{FrameLength: 1115}).Validate(); err != nil {
		t.Errorf("a 1115-octet frame was rejected: %v", err)
	}
	if err := (tmdl.ChannelConfig{FrameLength: tmdl.MaxFrameLength}).Validate(); err != nil {
		t.Errorf("the maximum frame length was rejected: %v", err)
	}
	if err := (tmdl.ChannelConfig{FrameLength: tmdl.MaxFrameLength + 1}).Validate(); !errors.Is(err, tmdl.ErrFrameTooLong) {
		t.Errorf("a 2049-octet frame gave %v, want ErrFrameTooLong", err)
	}
	if err := (tmdl.ChannelConfig{FrameLength: 4}).Validate(); err == nil {
		t.Error("a 4-octet frame was accepted; it cannot hold a primary header")
	}
}

// --- SyncFlag=1 First Header Pointer (CCSDS 132.0-B-3 clause 4.1.2.7.6.2) ---

// TestDecodeLenientFHPWithSyncFlag checks that a First Header Pointer carried
// under a set Synchronization Flag survives a decode/encode round trip
// untouched. The field is undefined for such frames and clause 3.4.2.3 gives those
// bits to the VCA service user as status, so neither side may rewrite them.
func TestDecodeLenientFHPWithSyncFlag(t *testing.T) {
	// Build a valid VCA-style frame (SyncFlag=1, FHP=0x7FF) and encode it.
	frame, err := tmdl.NewTMTransferFrame(933, 1, []byte("vca-sdu"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	frame.Header.SyncFlag = true
	frame.Header.FirstHeaderPtr = 0x07FF
	encoded, err := frame.Encode()
	if err != nil {
		t.Fatal(err)
	}

	// Patch the FHP to an arbitrary value (0x123) a foreign conformant
	// sender might emit, and refresh the FECF over the patched content.
	patched := make([]byte, len(encoded))
	copy(patched, encoded)
	patched[4] = (patched[4] &^ 0x07) | 0x01
	patched[5] = 0x23
	binary.BigEndian.PutUint16(patched[len(patched)-2:],
		ccsdscrc.ComputeCRC16(patched[:len(patched)-2]))

	back, err := tmdl.DecodeTMTransferFrame(patched)
	if err != nil {
		t.Fatalf("Decode rejected SyncFlag=1 frame with FHP=0x123: %v", err)
	}
	if !back.Header.SyncFlag || back.Header.FirstHeaderPtr != 0x123 {
		t.Errorf("Decoded SyncFlag=%v FHP=0x%03X, want true/0x123",
			back.Header.SyncFlag, back.Header.FirstHeaderPtr)
	}

	// Re-encoding must keep the value: with the Synchronization Flag set the
	// pointer is a VCA Status Field belonging to the service user (clause 3.4.2.3),
	// so a relay that rewrote it to all ones would destroy user data.
	if err := back.Header.Validate(); err != nil {
		t.Errorf("Validate rejected a VCA status field value: %v", err)
	}
	reencoded, err := back.Encode()
	if err != nil {
		t.Fatalf("Encode rejected a VCA status field value: %v", err)
	}
	if !bytes.Equal(reencoded, patched) {
		t.Errorf("Re-encoded frame differs from the received one:\n got % X\nwant % X",
			reencoded, patched)
	}
}
