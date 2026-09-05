package cli

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
	"github.com/ravisuhag/astro/pkg/cfdp"
	"github.com/ravisuhag/astro/pkg/ltp"
	"github.com/ravisuhag/astro/pkg/sle"
)

// These tests build each PDU with the library that owns it and then decode it
// through the CLI, so what is exercised is a real encoding rather than a hand
// transcription of one.

// pduJSON is the shape the decode commands emit.
type pduJSON struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	Octets  int    `json:"octets"`
	Body    string `json:"body"`
	Note    string `json:"note"`
}

func decodePDUJSON(t *testing.T, out string) pduJSON {
	t.Helper()

	var described pduJSON
	if err := json.Unmarshal([]byte(out), &described); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	return described
}

// cfdpPDU wraps a data field in a CFDP header and returns the hex.
func cfdpPDU(t *testing.T, data []byte, withCRC bool) string {
	t.Helper()

	pdu := &cfdp.PDU{
		Header: &cfdp.PDUHeader{
			Direction:      cfdp.TowardReceiver,
			CRCFlag:        withCRC,
			Source:         cfdp.NewEntityID(1),
			TransactionSeq: cfdp.NewEntityID(7),
			Destination:    cfdp.NewEntityID(2),
		},
		Data: data,
	}
	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatalf("encoding the CFDP PDU: %v", err)
	}
	return hex.EncodeToString(encoded)
}

// An EOF directive is decoded down to its fields, not left as octets.
func TestCFDPDecodeEOFDirective(t *testing.T) {
	t.Parallel()
	eof := &cfdp.EOFPDU{
		ConditionCode: cfdp.CondNoError,
		FileChecksum:  0xDEADBEEF,
		FileSize:      1024,
	}
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatalf("encoding the EOF PDU: %v", err)
	}

	out, err := runCLI(t, []byte(cfdpPDU(t, body, false)), "cfdp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	described := decodePDUJSON(t, out)
	if !strings.Contains(described.Kind, "EOF") {
		t.Errorf("kind = %q, want it to name the EOF directive", described.Kind)
	}
	if described.Note != "" {
		t.Errorf("the EOF directive was not decoded: %s", described.Note)
	}
	// The file size came out of the directive body, so it proves the body was
	// really read rather than skipped.
	if !strings.Contains(described.Summary, "1024") {
		t.Errorf("summary does not carry the file size:\n%s", described.Summary)
	}
}

func TestCFDPDecodeNAKDirective(t *testing.T) {
	t.Parallel()
	nak := &cfdp.NAKPDU{
		StartOfScope: 0,
		EndOfScope:   500,
		Requests:     []cfdp.SegmentRequest{{StartOffset: 100, EndOffset: 200}},
	}
	body, err := nak.Encode(false)
	if err != nil {
		t.Fatalf("encoding the NAK PDU: %v", err)
	}

	out, err := runCLI(t, []byte(cfdpPDU(t, body, false)), "cfdp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	described := decodePDUJSON(t, out)
	if !strings.Contains(described.Kind, "NAK") {
		t.Errorf("kind = %q, want it to name the NAK directive", described.Kind)
	}
	if described.Note != "" {
		t.Errorf("the NAK directive was not decoded: %s", described.Note)
	}
}

// File data is not a directive, so it is reported as octets, and labelled as
// uninterpreted rather than left looking decoded.
func TestCFDPDecodeFileData(t *testing.T) {
	t.Parallel()
	pdu := &cfdp.PDU{
		Header: &cfdp.PDUHeader{
			IsFileData:     true,
			Direction:      cfdp.TowardReceiver,
			Source:         cfdp.NewEntityID(1),
			TransactionSeq: cfdp.NewEntityID(7),
			Destination:    cfdp.NewEntityID(2),
		},
		Data: []byte{0, 0, 0, 0, 'h', 'e', 'l', 'l', 'o'},
	}
	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatalf("encoding the file data PDU: %v", err)
	}

	out, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "cfdp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	described := decodePDUJSON(t, out)
	if !strings.Contains(described.Kind, "File Data") {
		t.Errorf("kind = %q, want it to say file data", described.Kind)
	}
	if described.Note == "" {
		t.Error("file data was shown without saying it is uninterpreted")
	}
}

// The CRC is verified when the header says one is there, so a corrupted PDU
// fails rather than decoding to something plausible. Clause 4.1.2 requires the
// receiver to discard it.
func TestCFDPDecodeRejectsABadCRC(t *testing.T) {
	t.Parallel()
	eof := &cfdp.EOFPDU{ConditionCode: cfdp.CondNoError, FileChecksum: 1, FileSize: 8}
	body, err := eof.Encode(false)
	if err != nil {
		t.Fatalf("encoding the EOF PDU: %v", err)
	}

	good := cfdpPDU(t, body, true)
	if _, err := runCLI(t, []byte(good), "cfdp", "decode", "--input", "hex"); err != nil {
		t.Fatalf("a PDU with a valid CRC failed to decode: %v", err)
	}

	// Flip the last octet of the CRC.
	corrupted := []byte(good)
	if corrupted[len(corrupted)-1] == '0' {
		corrupted[len(corrupted)-1] = '1'
	} else {
		corrupted[len(corrupted)-1] = '0'
	}

	if _, err := runCLI(t, corrupted, "cfdp", "decode", "--input", "hex"); err == nil {
		t.Error("decode accepted a PDU whose CRC does not match")
	}
}

func TestCFDPDecodeRejectsRubbish(t *testing.T) {
	t.Parallel()
	if _, err := runCLI(t, []byte("00"), "cfdp", "decode", "--input", "hex"); err == nil {
		t.Error("decode accepted one octet as a PDU")
	}
}

// LTP: each segment type has its own content, and the header says which.
func TestLTPDecodeSegments(t *testing.T) {
	t.Parallel()
	for name, segment := range map[string]*ltp.Segment{
		"red data": {
			Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 9}},
			Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("payload")},
		},
		"report": {
			Header: &ltp.Header{Type: ltp.TypeReport, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 9}},
			Report: &ltp.ReportSegment{
				ReportSerial: 3, UpperBound: 100,
				Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 50}},
			},
		},
		"report ack": {
			Header:    &ltp.Header{Type: ltp.TypeReportAck, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 9}},
			ReportAck: &ltp.ReportAckSegment{ReportSerial: 3},
		},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := segment.Encode()
			if err != nil {
				t.Fatalf("encoding the segment: %v", err)
			}

			out, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "ltp", "decode",
				"--input", "hex", "--format", "json")
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			described := decodePDUJSON(t, out)
			if described.Kind != "LTP Segment" {
				t.Errorf("kind = %q, want LTP Segment", described.Kind)
			}
			if described.Octets != len(encoded) {
				t.Errorf("octets = %d, want %d", described.Octets, len(encoded))
			}
			if described.Summary == "" {
				t.Error("no summary for the segment")
			}
		})
	}
}

// The session identifier has to survive into the summary, or the decode is
// not telling the operator which session they are looking at.
func TestLTPDecodeCarriesTheSession(t *testing.T) {
	t.Parallel()
	segment := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: ltp.SessionID{EngineID: 42, SessionNumber: 77}},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("x")},
	}
	encoded, err := segment.Encode()
	if err != nil {
		t.Fatalf("encoding the segment: %v", err)
	}

	out, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "ltp", "decode", "--input", "hex")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !strings.Contains(out, "42") || !strings.Contains(out, "77") {
		t.Errorf("the session identifier is missing from:\n%s", out)
	}
}

func TestLTPDecodeRejectsRubbish(t *testing.T) {
	t.Parallel()
	if _, err := runCLI(t, []byte("ff"), "ltp", "decode", "--input", "hex"); err == nil {
		t.Error("decode accepted one octet as a segment")
	}
}

// BP: a bundle round trips through the CLI decoder.
func TestBPDecodeBundle(t *testing.T) {
	t.Parallel()
	primary := &bp.PrimaryBlock{
		CRCType:     bp.CRC32C,
		Destination: bp.IPN(2, 1),
		Source:      bp.IPN(1, 1),
		ReportTo:    bp.IPN(1, 0),
		Timestamp:   bp.CreationTimestamp{Time: 800_000_000_000, Sequence: 42},
		Lifetime:    3_600_000,
	}

	bundle, err := bp.NewBundle(primary, []byte("telemetry"))
	if err != nil {
		t.Fatalf("building the bundle: %v", err)
	}
	encoded, err := bundle.Encode()
	if err != nil {
		t.Fatalf("encoding the bundle: %v", err)
	}

	out, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "bp", "decode",
		"--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	described := decodePDUJSON(t, out)
	if described.Kind != "BPv7 Bundle" {
		t.Errorf("kind = %q, want BPv7 Bundle", described.Kind)
	}
	if described.Octets != len(encoded) {
		t.Errorf("octets = %d, want %d", described.Octets, len(encoded))
	}
	if described.Summary == "" {
		t.Error("no summary for the bundle")
	}
}

func TestBPDecodeRejectsRubbish(t *testing.T) {
	t.Parallel()
	if _, err := runCLI(t, []byte("ffffff"), "bp", "decode", "--input", "hex"); err == nil {
		t.Error("decode accepted rubbish as a bundle")
	}
}

// SLE: the wire tag means different things in different services, so the
// service has to be given. Decoding the same octets as two services must
// give two different answers, which is the whole reason the flag exists.
func TestSLEDecodeNeedsTheService(t *testing.T) {
	t.Parallel()
	// A transfer-buffer PDU tag under RAF.
	content := []byte{0x00}
	encoded := sle.AppendPDU(nil, 8, content)
	input := []byte(hex.EncodeToString(encoded))

	raf, err := runCLI(t, input, "sle", "decode",
		"--service", "raf", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode as raf failed: %v", err)
	}
	fcltu, err := runCLI(t, input, "sle", "decode",
		"--service", "fcltu", "--input", "hex", "--format", "json")
	if err != nil {
		t.Fatalf("decode as fcltu failed: %v", err)
	}

	rafPDU := decodePDUJSON(t, raf)
	fcltuPDU := decodePDUJSON(t, fcltu)

	if !strings.Contains(rafPDU.Kind, "RAF") {
		t.Errorf("kind = %q, want it to name RAF", rafPDU.Kind)
	}
	if !strings.Contains(fcltuPDU.Kind, "FCLTU") {
		t.Errorf("kind = %q, want it to name FCLTU", fcltuPDU.Kind)
	}
	// The same tag is a different operation in each service.
	if rafPDU.Summary == fcltuPDU.Summary {
		t.Errorf("the same tag decoded identically under two services:\n%s", rafPDU.Summary)
	}
}

func TestSLEDecodeRequiresService(t *testing.T) {
	t.Parallel()
	encoded := sle.AppendPDU(nil, 8, []byte{0x00})

	if _, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "sle", "decode",
		"--input", "hex"); err == nil {
		t.Error("decode ran without --service")
	}
}

func TestSLEDecodeRejectsUnknownService(t *testing.T) {
	t.Parallel()
	encoded := sle.AppendPDU(nil, 8, []byte{0x00})

	if _, err := runCLI(t, []byte(hex.EncodeToString(encoded)), "sle", "decode",
		"--service", "telepathy", "--input", "hex"); err == nil {
		t.Error("decode accepted an unknown --service")
	}
}

func TestSLEDecodeRejectsRubbish(t *testing.T) {
	t.Parallel()
	if _, err := runCLI(t, []byte("00"), "sle", "decode",
		"--service", "raf", "--input", "hex"); err == nil {
		t.Error("decode accepted one octet as an SLE PDU")
	}
}

// Every one of these commands takes the same two flags, so an unknown output
// format has to fail the same way in all of them.
func TestPDUCommandsRejectUnknownFormat(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"cfdp": {"cfdp", "decode", "--input", "hex", "--format", "yaml"},
		"ltp":  {"ltp", "decode", "--input", "hex", "--format", "yaml"},
		"bp":   {"bp", "decode", "--input", "hex", "--format", "yaml"},
		"sle":  {"sle", "decode", "--service", "raf", "--input", "hex", "--format", "yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runCLI(t, []byte("0102030405060708"), args...); err == nil {
				t.Errorf("%s decode accepted an unknown format", name)
			}
		})
	}
}
