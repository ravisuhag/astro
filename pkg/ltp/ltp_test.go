package ltp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/ltp"
)

func testSession() ltp.SessionID {
	return ltp.SessionID{EngineID: 42, SessionNumber: 7}
}

func TestSegmentTypeClassification(t *testing.T) {
	// RFC 5326 §3.1.2 and §3.1.3. Getting these predicates wrong changes
	// which segments a receiver reports on.
	tests := []struct {
		code                                    ltp.SegmentType
		data, red, green, checkpoint, eorp, eob bool
	}{
		{0, true, true, false, false, false, false},
		{1, true, true, false, true, false, false},
		{2, true, true, false, true, true, false},
		{3, true, true, false, true, true, true},
		{4, true, false, true, false, false, false},
		{7, true, false, true, false, false, true},
		{8, false, false, false, false, false, false},
		{9, false, false, false, false, false, false},
		{12, false, false, false, false, false, false},
		{15, false, false, false, false, false, false},
	}

	for _, tt := range tests {
		if got := tt.code.IsData(); got != tt.data {
			t.Errorf("code %d: IsData = %t, want %t", tt.code, got, tt.data)
		}
		if got := tt.code.IsRedData(); got != tt.red {
			t.Errorf("code %d: IsRedData = %t, want %t", tt.code, got, tt.red)
		}
		if got := tt.code.IsGreenData(); got != tt.green {
			t.Errorf("code %d: IsGreenData = %t, want %t", tt.code, got, tt.green)
		}
		if got := tt.code.IsCheckpoint(); got != tt.checkpoint {
			t.Errorf("code %d: IsCheckpoint = %t, want %t", tt.code, got, tt.checkpoint)
		}
		if got := tt.code.IsEORP(); got != tt.eorp {
			t.Errorf("code %d: IsEORP = %t, want %t", tt.code, got, tt.eorp)
		}
		if got := tt.code.IsEOB(); got != tt.eob {
			t.Errorf("code %d: IsEOB = %t, want %t", tt.code, got, tt.eob)
		}
	}
}

func TestUndefinedSegmentTypes(t *testing.T) {
	// §3.1.2 marks codes 5, 6, 10 and 11 undefined.
	for _, code := range []ltp.SegmentType{5, 6, 10, 11} {
		if code.Defined() {
			t.Errorf("code %d should be undefined", code)
		}
	}
	for _, code := range []ltp.SegmentType{0, 1, 2, 3, 4, 7, 8, 9, 12, 13, 14, 15} {
		if !code.Defined() {
			t.Errorf("code %d should be defined", code)
		}
	}
}

func TestDataSegmentRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		typ  ltp.SegmentType
	}{
		{"plain red data", ltp.TypeRedData},
		{"red checkpoint", ltp.TypeRedDataCheckpoint},
		{"red checkpoint EORP", ltp.TypeRedDataCheckpointEORP},
		{"red checkpoint EORP EOB", ltp.TypeRedDataCheckpointEORPEOB},
		{"green data", ltp.TypeGreenData},
		{"green data EOB", ltp.TypeGreenDataEOB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ltp.DataSegment{
				ClientServiceID: 3,
				Offset:          1024,
				Data:            []byte("payload octets"),
			}
			if tt.typ.IsCheckpoint() {
				d.CheckpointSerial = 5
				d.ReportSerial = 2
			}

			seg := &ltp.Segment{Header: &ltp.Header{Type: tt.typ, SessionID: testSession()}, Data: d}
			encoded, err := seg.Encode()
			if err != nil {
				t.Fatal(err)
			}

			got, err := ltp.DecodeSegment(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got.Header.Type != tt.typ {
				t.Errorf("type = %s, want %s", got.Header.Type, tt.typ)
			}
			if got.Header.SessionID != testSession() {
				t.Errorf("session = %s, want %s", got.Header.SessionID, testSession())
			}
			if got.Data.ClientServiceID != 3 {
				t.Errorf("client service = %d, want 3", got.Data.ClientServiceID)
			}
			if got.Data.Offset != 1024 {
				t.Errorf("offset = %d, want 1024", got.Data.Offset)
			}
			if !bytes.Equal(got.Data.Data, d.Data) {
				t.Errorf("data = %q, want %q", got.Data.Data, d.Data)
			}
			if tt.typ.IsCheckpoint() {
				if got.Data.CheckpointSerial != 5 {
					t.Errorf("checkpoint serial = %d, want 5", got.Data.CheckpointSerial)
				}
				if got.Data.ReportSerial != 2 {
					t.Errorf("report serial = %d, want 2", got.Data.ReportSerial)
				}
			}
		})
	}
}

func TestNonCheckpointCarriesNoSerials(t *testing.T) {
	// §3.2.1: "Data segments that are not checkpoints MUST NOT have these two
	// fields in the header and MUST continue on directly with the client
	// service data." A plain segment must therefore be shorter.
	d := &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("abc")}

	plain := &ltp.Segment{Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: testSession()}, Data: d}
	plainBytes, err := plain.Encode()
	if err != nil {
		t.Fatal(err)
	}

	checkpointData := *d
	checkpointData.CheckpointSerial = 1
	cp := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedDataCheckpoint, SessionID: testSession()},
		Data:   &checkpointData,
	}
	cpBytes, err := cp.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if len(plainBytes) >= len(cpBytes) {
		t.Errorf("plain segment is %d octets, checkpoint %d; the checkpoint must be longer",
			len(plainBytes), len(cpBytes))
	}
}

func TestCheckpointSerialMustNotBeZero(t *testing.T) {
	// §3.2.1.
	d := &ltp.DataSegment{ClientServiceID: 1, Data: []byte("x"), CheckpointSerial: 0}
	if _, err := d.Encode(true); !errors.Is(err, ltp.ErrInvalidSerialNumber) {
		t.Errorf("error = %v, want ErrInvalidSerialNumber", err)
	}
}

func TestReportSegmentRoundTrip(t *testing.T) {
	r := &ltp.ReportSegment{
		ReportSerial:     10,
		CheckpointSerial: 3,
		UpperBound:       1000,
		LowerBound:       100,
		Claims: []ltp.ReceptionClaim{
			{Offset: 0, Length: 200},
			{Offset: 300, Length: 100},
		},
	}
	seg := &ltp.Segment{Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()}, Report: r}

	encoded, err := seg.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ltp.DecodeSegment(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Report.ReportSerial != 10 || got.Report.CheckpointSerial != 3 {
		t.Errorf("serials = %d/%d, want 10/3", got.Report.ReportSerial, got.Report.CheckpointSerial)
	}
	if got.Report.UpperBound != 1000 || got.Report.LowerBound != 100 {
		t.Errorf("bounds = %d..%d, want 100..1000", got.Report.LowerBound, got.Report.UpperBound)
	}
	if len(got.Report.Claims) != 2 {
		t.Fatalf("got %d claims, want 2", len(got.Report.Claims))
	}
}

func TestClaimOffsetsAreRelativeToLowerBound(t *testing.T) {
	// §3.2.2: "The offset within the entire block can be calculated by
	// summing this offset with the lower bound of the RS." Treating claim
	// offsets as absolute silently corrupts every gap calculation.
	r := &ltp.ReportSegment{
		ReportSerial: 1,
		UpperBound:   1000,
		LowerBound:   400,
		Claims:       []ltp.ReceptionClaim{{Offset: 0, Length: 100}},
	}
	ranges := r.ClaimedRanges()
	if len(ranges) != 1 {
		t.Fatalf("got %d ranges, want 1", len(ranges))
	}
	if ranges[0].Offset != 400 {
		t.Errorf("absolute offset = %d, want 400 (lower bound + claim offset)", ranges[0].Offset)
	}
}

func TestReportValidationRules(t *testing.T) {
	// §3.2.2's rules on serials, bounds, and claim lengths.
	tests := []struct {
		name   string
		report ltp.ReportSegment
	}{
		{"zero report serial", ltp.ReportSegment{ReportSerial: 0, UpperBound: 10}},
		{"upper below lower", ltp.ReportSegment{ReportSerial: 1, UpperBound: 5, LowerBound: 10}},
		{"zero-length claim", ltp.ReportSegment{
			ReportSerial: 1, UpperBound: 100,
			Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 0}},
		}},
		{"claim longer than the scope", ltp.ReportSegment{
			ReportSerial: 1, UpperBound: 100,
			Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 200}},
		}},
		{"claim reaching past the upper bound", ltp.ReportSegment{
			ReportSerial: 1, UpperBound: 100,
			Claims: []ltp.ReceptionClaim{{Offset: 50, Length: 60}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.report.Validate(); err == nil {
				t.Error("expected validation to fail")
			}
		})
	}
}

func TestCancelSegmentRoundTrip(t *testing.T) {
	for _, reason := range []ltp.CancelReason{
		ltp.ReasonUserCancelled, ltp.ReasonUnreachable, ltp.ReasonRetransmitLimit,
		ltp.ReasonMiscolored, ltp.ReasonSystemCancelled, ltp.ReasonRetransmitCyclesExceeded,
	} {
		for _, typ := range []ltp.SegmentType{ltp.TypeCancelFromSender, ltp.TypeCancelFromReceiver} {
			seg := &ltp.Segment{
				Header: &ltp.Header{Type: typ, SessionID: testSession()},
				Cancel: &ltp.CancelSegment{Reason: reason},
			}
			encoded, err := seg.Encode()
			if err != nil {
				t.Fatal(err)
			}
			got, err := ltp.DecodeSegment(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got.Cancel.Reason != reason {
				t.Errorf("reason = %s, want %s", got.Cancel.Reason, reason)
			}
		}
	}
}

func TestCancelRejectsReservedReason(t *testing.T) {
	// §3.2.4 reserves 06 to FF.
	c := &ltp.CancelSegment{Reason: ltp.CancelReason(6)}
	if _, err := c.Encode(); !errors.Is(err, ltp.ErrInvalidReasonCode) {
		t.Errorf("error = %v, want ErrInvalidReasonCode", err)
	}
}

func TestCancelAckHasNoContent(t *testing.T) {
	// §3.2.5: "The Cancel-acknowledgments (CAx) have no content."
	for _, typ := range []ltp.SegmentType{ltp.TypeCancelAckToSender, ltp.TypeCancelAckToReceiver} {
		seg := &ltp.Segment{Header: &ltp.Header{Type: typ, SessionID: testSession()}}
		encoded, err := seg.Encode()
		if err != nil {
			t.Fatal(err)
		}
		got, err := ltp.DecodeSegment(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if got.Header.Type != typ {
			t.Errorf("type = %s, want %s", got.Header.Type, typ)
		}
	}
}

func TestExtensionsRoundTrip(t *testing.T) {
	// §3.1.4: up to 15 header and 15 trailer extensions, counted in one octet.
	seg := &ltp.Segment{
		Header: &ltp.Header{
			Type:      ltp.TypeReportAck,
			SessionID: testSession(),
			HeaderExtensions: []ltp.Extension{
				{Tag: ltp.ExtensionCookie, Value: []byte{1, 2, 3}},
				{Tag: 0x42, Value: nil},
			},
			TrailerExtensions: []ltp.Extension{
				{Tag: ltp.ExtensionAuth, Value: []byte{9, 9}},
			},
		},
		ReportAck: &ltp.ReportAckSegment{ReportSerial: 4},
	}

	encoded, err := seg.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ltp.DecodeSegment(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Header.HeaderExtensions) != 2 {
		t.Fatalf("got %d header extensions, want 2", len(got.Header.HeaderExtensions))
	}
	if len(got.Header.TrailerExtensions) != 1 {
		t.Fatalf("got %d trailer extensions, want 1", len(got.Header.TrailerExtensions))
	}
	if got.Header.HeaderExtensions[0].Tag != ltp.ExtensionCookie {
		t.Errorf("first tag = %#x, want cookie", got.Header.HeaderExtensions[0].Tag)
	}
	if !bytes.Equal(got.Header.TrailerExtensions[0].Value, []byte{9, 9}) {
		t.Errorf("trailer value = %x, want 0909", got.Header.TrailerExtensions[0].Value)
	}
}

func TestTooManyExtensionsRejected(t *testing.T) {
	h := &ltp.Header{Type: ltp.TypeReportAck, SessionID: testSession()}
	for i := 0; i < 16; i++ {
		h.HeaderExtensions = append(h.HeaderExtensions, ltp.Extension{Tag: 0x50})
	}
	if _, err := h.Encode(); !errors.Is(err, ltp.ErrTooManyExtensions) {
		t.Errorf("error = %v, want ErrTooManyExtensions", err)
	}
}

func TestDecodeRejectsWrongVersion(t *testing.T) {
	seg := &ltp.Segment{
		Header:    &ltp.Header{Type: ltp.TypeReportAck, SessionID: testSession()},
		ReportAck: &ltp.ReportAckSegment{ReportSerial: 1},
	}
	encoded, err := seg.Encode()
	if err != nil {
		t.Fatal(err)
	}
	encoded[0] = 1<<4 | (encoded[0] & 0x0F)

	if _, err := ltp.DecodeSegment(encoded); !errors.Is(err, ltp.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestDecodeRejectsUndefinedType(t *testing.T) {
	// Type code 5 is undefined per §3.1.2.
	data := []byte{0x05, 0x2A, 0x07, 0x00}
	if _, err := ltp.DecodeSegment(data); !errors.Is(err, ltp.ErrUndefinedSegmentType) {
		t.Errorf("error = %v, want ErrUndefinedSegmentType", err)
	}
}

func TestDecodeRejectsShortInput(t *testing.T) {
	seg := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedDataCheckpointEORPEOB, SessionID: testSession()},
		Data: &ltp.DataSegment{
			ClientServiceID: 1, Offset: 0, Data: []byte("payload"), CheckpointSerial: 1,
		},
	}
	encoded, err := seg.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for cut := 0; cut < len(encoded); cut++ {
		if _, err := ltp.DecodeSegment(encoded[:cut]); err == nil {
			t.Errorf("length %d: expected an error, got nil", cut)
		}
	}
}
