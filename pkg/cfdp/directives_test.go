package cfdp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

func TestEOFPDURoundTrip(t *testing.T) {
	for _, largeFile := range []bool{false, true} {
		eof := &cfdp.EOFPDU{
			ConditionCode: cfdp.CondNoError,
			FileChecksum:  0xDEADBEEF,
			FileSize:      123456,
		}
		encoded, err := eof.Encode(largeFile)
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfdp.DecodeEOFPDU(encoded, largeFile)
		if err != nil {
			t.Fatal(err)
		}
		if got.FileChecksum != eof.FileChecksum {
			t.Errorf("checksum = %#08x, want %#08x", got.FileChecksum, eof.FileChecksum)
		}
		if got.FileSize != eof.FileSize {
			t.Errorf("file size = %d, want %d", got.FileSize, eof.FileSize)
		}
		if got.FaultLocation != nil {
			t.Error("fault location must be omitted for condition 'no error'")
		}
	}
}

func TestEOFPDUCarriesFaultLocation(t *testing.T) {
	// §5.2.2: present whenever the condition code is not 'no error'.
	loc, err := cfdp.EntityIDTLV(cfdp.EntityID{Value: 9, Width: 1})
	if err != nil {
		t.Fatal(err)
	}
	eof := &cfdp.EOFPDU{
		ConditionCode: cfdp.CondCancelRequestReceived,
		FileChecksum:  1,
		FileSize:      2,
		FaultLocation: &loc,
	}
	encoded, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfdp.DecodeEOFPDU(encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.FaultLocation == nil {
		t.Fatal("fault location missing")
	}
	id, err := got.FaultLocation.AsEntityID()
	if err != nil {
		t.Fatal(err)
	}
	if id.Value != 9 {
		t.Errorf("fault location entity = %d, want 9", id.Value)
	}
}

func TestFinishedPDURoundTrip(t *testing.T) {
	fin := &cfdp.FinishedPDU{
		ConditionCode: cfdp.CondNoError,
		DeliveryCode:  cfdp.DeliveryDataComplete,
		FileStatus:    cfdp.FileRetainedSuccessfully,
	}
	encoded, err := fin.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfdp.DecodeFinishedPDU(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConditionCode != fin.ConditionCode ||
		got.DeliveryCode != fin.DeliveryCode ||
		got.FileStatus != fin.FileStatus {
		t.Errorf("got %+v, want %+v", got, fin)
	}
}

func TestFinishedPDUSeparatesResponsesFromFaultLocation(t *testing.T) {
	resp := cfdp.FilestoreResponse{
		Action:        cfdp.ActionCreateFile,
		StatusCode:    cfdp.StatusSuccessful,
		FirstFileName: cfdp.LV{Value: []byte("a.txt")},
	}
	respTLV, err := resp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := cfdp.EntityIDTLV(cfdp.EntityID{Value: 3, Width: 1})
	if err != nil {
		t.Fatal(err)
	}

	fin := &cfdp.FinishedPDU{
		ConditionCode:      cfdp.CondFilestoreRejection,
		DeliveryCode:       cfdp.DeliveryDataIncomplete,
		FileStatus:         cfdp.FileDiscardedRejection,
		FilestoreResponses: []cfdp.TLV{respTLV},
		FaultLocation:      &loc,
	}
	encoded, err := fin.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfdp.DecodeFinishedPDU(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FilestoreResponses) != 1 {
		t.Fatalf("got %d filestore responses, want 1", len(got.FilestoreResponses))
	}
	if got.FaultLocation == nil {
		t.Fatal("fault location missing")
	}
}

func TestACKPDUSubtypeRules(t *testing.T) {
	// §5.2.4: subtype '0001' for a Finished PDU, '0000' otherwise.
	tests := []struct {
		acked       cfdp.DirectiveCode
		wantSubtype uint8
	}{
		{cfdp.DirectiveEOF, 0},
		{cfdp.DirectiveFinished, 1},
	}
	for _, tt := range tests {
		ack, err := cfdp.NewACK(tt.acked, cfdp.CondNoError, cfdp.StatusActive)
		if err != nil {
			t.Fatal(err)
		}
		if ack.DirectiveSubtype != tt.wantSubtype {
			t.Errorf("%s: subtype = %d, want %d", tt.acked, ack.DirectiveSubtype, tt.wantSubtype)
		}
		encoded, err := ack.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfdp.DecodeACKPDU(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if got.AckedDirective != tt.acked || got.DirectiveSubtype != tt.wantSubtype {
			t.Errorf("round trip gave %+v", got)
		}
	}
}

func TestACKPDURejectsUnacknowledgeableDirectives(t *testing.T) {
	// Table 5-8: only EOF and Finished are ever acknowledged.
	for _, code := range []cfdp.DirectiveCode{cfdp.DirectiveMetadata, cfdp.DirectiveNAK, cfdp.DirectivePrompt} {
		if _, err := cfdp.NewACK(code, cfdp.CondNoError, cfdp.StatusActive); !errors.Is(err, cfdp.ErrWrongDirectiveCode) {
			t.Errorf("%s: error = %v, want ErrWrongDirectiveCode", code, err)
		}
	}
}

func TestMetadataPDURoundTrip(t *testing.T) {
	req := cfdp.FilestoreRequest{
		Action:        cfdp.ActionDeleteFile,
		FirstFileName: cfdp.LV{Value: []byte("old.dat")},
	}
	reqTLV, err := req.Encode()
	if err != nil {
		t.Fatal(err)
	}

	meta := &cfdp.MetadataPDU{
		ClosureRequested:    true,
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            4096,
		SourceFileName:      cfdp.LV{Value: []byte("src/a.dat")},
		DestinationFileName: cfdp.LV{Value: []byte("dst/b.dat")},
		Options:             []cfdp.TLV{reqTLV},
	}
	encoded, err := meta.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfdp.DecodeMetadataPDU(encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ClosureRequested {
		t.Error("closure requested lost")
	}
	if got.FileSize != meta.FileSize {
		t.Errorf("file size = %d, want %d", got.FileSize, meta.FileSize)
	}
	if got.SourceFileName.String() != "src/a.dat" {
		t.Errorf("source = %q", got.SourceFileName.String())
	}
	if got.DestinationFileName.String() != "dst/b.dat" {
		t.Errorf("destination = %q", got.DestinationFileName.String())
	}
	if len(got.Options) != 1 {
		t.Fatalf("got %d options, want 1", len(got.Options))
	}
}

func TestMetadataPDUEmptyFilenames(t *testing.T) {
	// §5.2.5: a transaction with no associated file carries zero-length LVs.
	meta := &cfdp.MetadataPDU{ChecksumType: cfdp.ChecksumNull}
	encoded, err := meta.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cfdp.DecodeMetadataPDU(encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SourceFileName.IsEmpty() || !got.DestinationFileName.IsEmpty() {
		t.Error("expected both filenames to be empty")
	}
}

func TestNAKPDURoundTrip(t *testing.T) {
	nak := &cfdp.NAKPDU{
		StartOfScope: 0,
		EndOfScope:   1000,
		Requests: []cfdp.SegmentRequest{
			{StartOffset: 0, EndOffset: 0}, // metadata request
			{StartOffset: 100, EndOffset: 200},
			{StartOffset: 500, EndOffset: 700},
		},
	}
	for _, largeFile := range []bool{false, true} {
		encoded, err := nak.Encode(largeFile)
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfdp.DecodeNAKPDU(encoded, largeFile)
		if err != nil {
			t.Fatal(err)
		}
		if got.EndOfScope != nak.EndOfScope {
			t.Errorf("end of scope = %d, want %d", got.EndOfScope, nak.EndOfScope)
		}
		if len(got.Requests) != len(nak.Requests) {
			t.Fatalf("got %d requests, want %d", len(got.Requests), len(nak.Requests))
		}
		if !got.Requests[0].IsMetadataRequest() {
			t.Error("first request should be recognized as a metadata request")
		}
		for i, r := range got.Requests {
			if r != nak.Requests[i] {
				t.Errorf("request %d = %+v, want %+v", i, r, nak.Requests[i])
			}
		}
	}
}

func TestPromptAndKeepAliveRoundTrip(t *testing.T) {
	for _, resp := range []cfdp.PromptResponse{cfdp.PromptNAK, cfdp.PromptKeepAlive} {
		p := &cfdp.PromptPDU{Response: resp}
		encoded, err := p.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfdp.DecodePromptPDU(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if got.Response != resp {
			t.Errorf("response = %d, want %d", got.Response, resp)
		}
	}

	ka := &cfdp.KeepAlivePDU{Progress: 8192}
	encoded, err := ka.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	gotKA, err := cfdp.DecodeKeepAlivePDU(encoded, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotKA.Progress != ka.Progress {
		t.Errorf("progress = %d, want %d", gotKA.Progress, ka.Progress)
	}
}

func TestFileDataPDURoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		withMetadata bool
		largeFile    bool
	}{
		{"plain", false, false},
		{"large file offsets", false, true},
		{"with segment metadata", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &cfdp.FileDataPDU{Offset: 4096, Data: []byte("file contents here")}
			if tt.withMetadata {
				fd.RecordContinuation = cfdp.RecordStartAndEnd
				fd.SegmentMetadata = []byte{0x01, 0x02}
			}

			encoded, err := fd.Encode(tt.withMetadata, tt.largeFile)
			if err != nil {
				t.Fatal(err)
			}
			got, err := cfdp.DecodeFileDataPDU(encoded, tt.withMetadata, tt.largeFile)
			if err != nil {
				t.Fatal(err)
			}
			if got.Offset != fd.Offset {
				t.Errorf("offset = %d, want %d", got.Offset, fd.Offset)
			}
			if !bytes.Equal(got.Data, fd.Data) {
				t.Errorf("data = %q, want %q", got.Data, fd.Data)
			}
			if tt.withMetadata {
				if got.RecordContinuation != fd.RecordContinuation {
					t.Errorf("continuation = %v, want %v", got.RecordContinuation, fd.RecordContinuation)
				}
				if !bytes.Equal(got.SegmentMetadata, fd.SegmentMetadata) {
					t.Errorf("segment metadata = %x, want %x", got.SegmentMetadata, fd.SegmentMetadata)
				}
			}
		})
	}
}

func TestFileDataPDURejectsOversizedSegmentMetadata(t *testing.T) {
	// The length field is 6 bits, so 63 octets is the ceiling.
	fd := &cfdp.FileDataPDU{SegmentMetadata: bytes.Repeat([]byte{0}, 64)}
	if _, err := fd.Encode(true, false); !errors.Is(err, cfdp.ErrSegmentTooLarge) {
		t.Errorf("error = %v, want ErrSegmentTooLarge", err)
	}
}

func TestDirectiveCodeValidity(t *testing.T) {
	valid := []cfdp.DirectiveCode{0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0C}
	for _, c := range valid {
		if !c.Valid() {
			t.Errorf("%#02x should be valid", uint8(c))
		}
	}
	// Table 5-4 reserves 00-03 and 0D-FF, and leaves 0A-0B undefined.
	for _, c := range []cfdp.DirectiveCode{0x00, 0x03, 0x0A, 0x0B, 0x0D, 0xFF} {
		if c.Valid() {
			t.Errorf("%#02x should be invalid", uint8(c))
		}
	}
}

func TestWrongDirectiveCodeRejected(t *testing.T) {
	eof := &cfdp.EOFPDU{FileChecksum: 1}
	encoded, err := eof.Encode(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfdp.DecodeFinishedPDU(encoded); !errors.Is(err, cfdp.ErrWrongDirectiveCode) {
		t.Errorf("error = %v, want ErrWrongDirectiveCode", err)
	}
}

func TestFilestoreRequestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		req  cfdp.FilestoreRequest
	}{
		{"one filename", cfdp.FilestoreRequest{
			Action:        cfdp.ActionDeleteFile,
			FirstFileName: cfdp.LV{Value: []byte("gone.dat")},
		}},
		{"two filenames", cfdp.FilestoreRequest{
			Action:         cfdp.ActionRenameFile,
			FirstFileName:  cfdp.LV{Value: []byte("from.dat")},
			SecondFileName: cfdp.LV{Value: []byte("to.dat")},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlv, err := tt.req.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got, err := cfdp.DecodeFilestoreRequest(tlv)
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != tt.req.Action {
				t.Errorf("action = %v, want %v", got.Action, tt.req.Action)
			}
			if got.FirstFileName.String() != tt.req.FirstFileName.String() {
				t.Errorf("first name = %q", got.FirstFileName.String())
			}
			if tt.req.Action.NeedsSecondFileName() &&
				got.SecondFileName.String() != tt.req.SecondFileName.String() {
				t.Errorf("second name = %q", got.SecondFileName.String())
			}
		})
	}
}
