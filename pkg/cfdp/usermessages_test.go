package cfdp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// Every Reserved CFDP Message opens with the four ASCII characters "cfdp"
// and its type octet (clause 6.1.2, table 6-1). That header is what tells a
// receiver a Message to User is a protocol message rather than an
// application one.
func TestUserMessageHeader(t *testing.T) {
	message := cfdp.UserMessage{
		Type:    cfdp.MsgProxyPutCancel,
		Content: nil,
	}

	encoded := message.Encode()
	if len(encoded) != 5 {
		t.Fatalf("a contentless message is %d octets, want 5", len(encoded))
	}
	if string(encoded[:4]) != "cfdp" {
		t.Errorf("the identifier is %q, want cfdp", encoded[:4])
	}
	if encoded[4] != byte(cfdp.MsgProxyPutCancel) {
		t.Errorf("the type octet is 0x%02X, want 0x09", encoded[4])
	}

	back, err := cfdp.DecodeUserMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeUserMessage: %v", err)
	}
	if back.Type != cfdp.MsgProxyPutCancel {
		t.Errorf("decoded type %s, want Proxy Put Cancel", back.Type)
	}
	if len(back.Content) != 0 {
		t.Errorf("Proxy Put Cancel decoded %d octets of content, want none, clause 6.2.6.2 gives it none",
			len(back.Content))
	}
}

// An application message is not a malformed protocol message. Telling them
// apart is the identifier's whole job, so this has to be a distinguishable
// outcome rather than a decode failure.
func TestApplicationMessageIsNotAUserMessage(t *testing.T) {
	for name, data := range map[string][]byte{
		"plain text":   []byte("hello from the payload"),
		"too short":    []byte("cfd"),
		"wrong magic":  []byte("CFDP\x00"),
		"nearly right": []byte("cfdq\x00"),
		"empty":        nil,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := cfdp.DecodeUserMessage(data); !errors.Is(err, cfdp.ErrNotUserMessage) {
				t.Errorf("err = %v, want ErrNotUserMessage", err)
			}
		})
	}
}

// UserMessagesFrom sifts a metadata TLV run: protocol messages come out,
// application messages and other TLV types are left alone.
func TestUserMessagesFrom(t *testing.T) {
	cancel := cfdp.ProxyPutCancel()

	tlvs := []cfdp.TLV{
		{Type: cfdp.TLVMessageToUser, Value: []byte("an application message")},
		cancel.EncodeTLV(),
		{Type: cfdp.TLVFlowLabel, Value: []byte{1, 2, 3}},
		cfdp.UserMessage{Type: cfdp.MsgProxyClosureRequest, Content: []byte{1}}.EncodeTLV(),
	}

	messages := cfdp.UserMessagesFrom(tlvs)
	if len(messages) != 2 {
		t.Fatalf("found %d user messages, want 2", len(messages))
	}
	if messages[0].Type != cfdp.MsgProxyPutCancel {
		t.Errorf("first message is %s, want Proxy Put Cancel", messages[0].Type)
	}
	if messages[1].Type != cfdp.MsgProxyClosureRequest {
		t.Errorf("second message is %s, want Proxy Closure Request", messages[1].Type)
	}
}

// The message type numbering has a gap inside the proxy range: 0x0A is the
// Originating Transaction ID, common to every operation, so proxy runs 0x00
// to 0x09 and resumes at 0x0B. Getting that wrong would put Proxy Closure
// Request on the common message's number.
func TestMessageTypeNumbering(t *testing.T) {
	for value, want := range map[uint8]cfdp.UserMessageType{
		0x09: cfdp.MsgProxyPutCancel,
		0x0A: cfdp.MsgOriginatingTransactionID,
		0x0B: cfdp.MsgProxyClosureRequest,
		0x10: cfdp.MsgDirectoryListingRequest,
		0x20: cfdp.MsgRemoteStatusReportRequest,
		0x30: cfdp.MsgRemoteSuspendRequest,
		0x38: cfdp.MsgRemoteResumeRequest,
	} {
		if cfdp.UserMessageType(value) != want {
			t.Errorf("0x%02X is %s, want %s", value, cfdp.UserMessageType(value), want)
		}
	}

	// The gaps between operation groups are genuinely undefined.
	for _, value := range []uint8{0x0C, 0x12, 0x22, 0x32, 0x3A, 0xFF} {
		if cfdp.UserMessageType(value).Valid() {
			t.Errorf("0x%02X reports itself as defined", value)
		}
	}
}

// A transaction ID's two 3-bit length fields hold the width less one, so a
// one-octet value encodes as zero (table 6-2). Getting that off by one would
// shift every field after it.
func TestTransactionIDRoundTrip(t *testing.T) {
	for name, id := range map[string]cfdp.TransactionID{
		"one octet each":  {Source: cfdp.NewEntityID(1), Sequence: cfdp.NewEntityID(2)},
		"wide entity":     {Source: cfdp.NewEntityID(0x0102030405), Sequence: cfdp.NewEntityID(7)},
		"wide sequence":   {Source: cfdp.NewEntityID(3), Sequence: cfdp.NewEntityID(0xFFFFFFFF)},
		"both eight wide": {Source: cfdp.NewEntityID(0x0102030405060708), Sequence: cfdp.NewEntityID(0x1122334455667788)},
	} {
		t.Run(name, func(t *testing.T) {
			message, err := cfdp.OriginatingTransactionID{Transaction: id}.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			back, err := cfdp.DecodeOriginatingTransactionID(message.Content)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if back.Transaction.Source.Value != id.Source.Value {
				t.Errorf("source = %d, want %d", back.Transaction.Source.Value, id.Source.Value)
			}
			if back.Transaction.Sequence.Value != id.Sequence.Value {
				t.Errorf("sequence = %d, want %d", back.Transaction.Sequence.Value, id.Sequence.Value)
			}
		})
	}
}

// A one-octet transaction ID encodes its lengths as zero, which is the case
// worth pinning outright.
func TestTransactionIDLengthEncoding(t *testing.T) {
	id := cfdp.TransactionID{Source: cfdp.NewEntityID(1), Sequence: cfdp.NewEntityID(2)}

	encoded, err := id.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) != 3 {
		t.Fatalf("got %d octets, want 3: one length octet and two values", len(encoded))
	}
	if encoded[0] != 0x00 {
		t.Errorf("the length octet is 0x%02X, want 0x00. The widths are stored less one", encoded[0])
	}

	// Three octets each: lengths of two in both nibbles.
	wide := cfdp.TransactionID{
		Source:   cfdp.EntityID{Value: 1, Width: 3},
		Sequence: cfdp.EntityID{Value: 2, Width: 3},
	}
	encoded, err = wide.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if encoded[0] != 0x22 {
		t.Errorf("the length octet is 0x%02X, want 0x22 for two three-octet values", encoded[0])
	}
}

// The reserved bits either side of the length fields must be zero. A sender
// setting one is using something this issue has not defined.
func TestTransactionIDRejectsReservedBits(t *testing.T) {
	for name, first := range map[string]byte{
		"entity reserved bit":   0x80,
		"sequence reserved bit": 0x08,
		"both":                  0x88,
	} {
		t.Run(name, func(t *testing.T) {
			content := []byte{first, 0x01, 0x02}
			if _, err := cfdp.DecodeOriginatingTransactionID(content); !errors.Is(err, cfdp.ErrReservedBitsSet) {
				t.Errorf("err = %v, want ErrReservedBitsSet", err)
			}
		})
	}
}

func TestProxyPutRequestRoundTrip(t *testing.T) {
	original := cfdp.ProxyPutRequest{
		Destination:         cfdp.NewEntityID(42),
		SourceFileName:      "/remote/science.dat",
		DestinationFileName: "/local/science.dat",
	}

	message, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if message.Type != cfdp.MsgProxyPutRequest {
		t.Errorf("type = %s, want Proxy Put Request", message.Type)
	}

	back, err := cfdp.DecodeProxyPutRequest(message.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if back.Destination.Value != 42 {
		t.Errorf("beneficiary = %d, want 42", back.Destination.Value)
	}
	if back.SourceFileName != original.SourceFileName {
		t.Errorf("source = %q, want %q", back.SourceFileName, original.SourceFileName)
	}
	if back.DestinationFileName != original.DestinationFileName {
		t.Errorf("destination = %q, want %q", back.DestinationFileName, original.DestinationFileName)
	}
}

// Table 6-4 expresses an omitted file name as a zero-length LV rather than
// an absent field, so both names have to survive being empty.
func TestProxyPutRequestOmittedFileNames(t *testing.T) {
	original := cfdp.ProxyPutRequest{Destination: cfdp.NewEntityID(7)}

	message, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := cfdp.DecodeProxyPutRequest(message.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if back.SourceFileName != "" || back.DestinationFileName != "" {
		t.Errorf("omitted names came back as %q and %q",
			back.SourceFileName, back.DestinationFileName)
	}
}

func TestProxyPutResponseRoundTrip(t *testing.T) {
	original := cfdp.ProxyPutResponse{
		Condition: cfdp.CondNoError,
		Delivery:  cfdp.DeliveryDataComplete,
		File:      cfdp.FileRetainedSuccessfully,
	}

	message, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := cfdp.DecodeProxyPutResponse(message.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if back.Condition != original.Condition || back.Delivery != original.Delivery ||
		back.File != original.File {
		t.Errorf("got %+v, want %+v", back, original)
	}

	// The spare bit sits between the delivery code and the condition code, so
	// a packing error would show up as a shifted field.
	incomplete := cfdp.ProxyPutResponse{
		Condition: cfdp.CondFileChecksumFailure,
		Delivery:  cfdp.DeliveryDataIncomplete,
		File:      cfdp.FileDiscardedRejection,
	}
	message, err = incomplete.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err = cfdp.DecodeProxyPutResponse(message.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if back.Condition != incomplete.Condition || back.Delivery != incomplete.Delivery ||
		back.File != incomplete.File {
		t.Errorf("got %+v, want %+v", back, incomplete)
	}
}

// The two response codes in section 6 have OPPOSITE polarity, which is the
// single most likely thing to get wrong here. Table 6-16 encodes a
// successful directory listing as '0'; table 6-19 encodes a successful status
// report as '1'.
func TestResponseCodePolaritiesAreOpposite(t *testing.T) {
	listing, err := cfdp.DirectoryListingResponse{
		Successful:        true,
		DirectoryName:     "/data",
		DirectoryFileName: "/local/listing.txt",
	}.Encode()
	if err != nil {
		t.Fatalf("Encode listing: %v", err)
	}
	// Table 6-16: successful is '0' in the top bit.
	if listing.Content[0]&0x80 != 0 {
		t.Errorf("a successful directory listing encoded its flag set; table 6-16 says '0'")
	}

	report, err := cfdp.RemoteStatusReportResponse{
		Successful:  true,
		Status:      cfdp.StatusActive,
		Transaction: cfdp.TransactionID{Source: cfdp.NewEntityID(1), Sequence: cfdp.NewEntityID(2)},
	}.Encode()
	if err != nil {
		t.Fatalf("Encode report: %v", err)
	}
	// Table 6-19: successful is '1' in the low bit.
	if report.Content[0]&0x01 == 0 {
		t.Errorf("a successful status report encoded its flag clear; table 6-19 says '1'")
	}
}

func TestDirectoryListingRoundTrip(t *testing.T) {
	request := cfdp.DirectoryListingRequest{
		DirectoryName:     "/science",
		DirectoryFileName: "/local/listing.txt",
	}
	message, err := request.Encode()
	if err != nil {
		t.Fatalf("Encode request: %v", err)
	}
	backRequest, err := cfdp.DecodeDirectoryListingRequest(message.Content)
	if err != nil {
		t.Fatalf("Decode request: %v", err)
	}
	if *backRequest != request {
		t.Errorf("request round trip: got %+v, want %+v", *backRequest, request)
	}

	for _, successful := range []bool{true, false} {
		response := cfdp.DirectoryListingResponse{
			Successful:        successful,
			DirectoryName:     "/science",
			DirectoryFileName: "/local/listing.txt",
		}
		message, err := response.Encode()
		if err != nil {
			t.Fatalf("Encode response: %v", err)
		}
		back, err := cfdp.DecodeDirectoryListingResponse(message.Content)
		if err != nil {
			t.Fatalf("Decode response: %v", err)
		}
		if *back != response {
			t.Errorf("response round trip: got %+v, want %+v", *back, response)
		}
	}
}

func TestRemoteStatusReportRoundTrip(t *testing.T) {
	id := cfdp.TransactionID{Source: cfdp.NewEntityID(5), Sequence: cfdp.NewEntityID(99)}

	request := cfdp.RemoteStatusReportRequest{Transaction: id, ReportFileName: "/local/report.txt"}
	message, err := request.Encode()
	if err != nil {
		t.Fatalf("Encode request: %v", err)
	}
	backRequest, err := cfdp.DecodeRemoteStatusReportRequest(message.Content)
	if err != nil {
		t.Fatalf("Decode request: %v", err)
	}
	if backRequest.ReportFileName != request.ReportFileName {
		t.Errorf("report file = %q, want %q", backRequest.ReportFileName, request.ReportFileName)
	}
	if backRequest.Transaction.Source.Value != 5 || backRequest.Transaction.Sequence.Value != 99 {
		t.Errorf("transaction = %s, want entity 5 sequence 99", backRequest.Transaction.Humanize())
	}

	for _, successful := range []bool{true, false} {
		response := cfdp.RemoteStatusReportResponse{
			Successful:  successful,
			Status:      cfdp.StatusTerminated,
			Transaction: id,
		}
		message, err := response.Encode()
		if err != nil {
			t.Fatalf("Encode response: %v", err)
		}
		back, err := cfdp.DecodeRemoteStatusReportResponse(message.Content)
		if err != nil {
			t.Fatalf("Decode response: %v", err)
		}
		if back.Successful != successful {
			t.Errorf("successful = %v, want %v", back.Successful, successful)
		}
		if back.Status != cfdp.StatusTerminated {
			t.Errorf("status = %s, want terminated", back.Status)
		}
		if back.Transaction.Source.Value != 5 {
			t.Errorf("transaction source = %d, want 5", back.Transaction.Source.Value)
		}
	}
}

// Suspend and resume are symmetric: both requests carry only a transaction
// ID, and both responses carry the suspension indicator, the status and the
// transaction ID. Table 6-22 does include the transaction ID, which is easy
// to miss because the field list runs onto a second page.
func TestSuspendAndResumeRoundTrip(t *testing.T) {
	id := cfdp.TransactionID{Source: cfdp.NewEntityID(11), Sequence: cfdp.NewEntityID(22)}

	suspendRequest, err := cfdp.RemoteSuspendRequest{Transaction: id}.Encode()
	if err != nil {
		t.Fatalf("Encode suspend request: %v", err)
	}
	if _, err := cfdp.DecodeRemoteSuspendRequest(suspendRequest.Content); err != nil {
		t.Fatalf("Decode suspend request: %v", err)
	}

	resumeRequest, err := cfdp.RemoteResumeRequest{Transaction: id}.Encode()
	if err != nil {
		t.Fatalf("Encode resume request: %v", err)
	}
	if _, err := cfdp.DecodeRemoteResumeRequest(resumeRequest.Content); err != nil {
		t.Fatalf("Decode resume request: %v", err)
	}

	for _, suspended := range []bool{true, false} {
		body := cfdp.SuspensionResponse{
			Suspended:   suspended,
			Status:      cfdp.StatusActive,
			Transaction: id,
		}

		suspendResponse, err := cfdp.RemoteSuspendResponse{SuspensionResponse: body}.Encode()
		if err != nil {
			t.Fatalf("Encode suspend response: %v", err)
		}
		backSuspend, err := cfdp.DecodeRemoteSuspendResponse(suspendResponse.Content)
		if err != nil {
			t.Fatalf("Decode suspend response: %v", err)
		}
		if backSuspend.Suspended != suspended || backSuspend.Status != cfdp.StatusActive {
			t.Errorf("suspend response: got %+v", backSuspend.SuspensionResponse)
		}
		if backSuspend.Transaction.Source.Value != 11 {
			t.Error("the suspend response lost its transaction ID, which table 6-22 includes")
		}

		resumeResponse, err := cfdp.RemoteResumeResponse{SuspensionResponse: body}.Encode()
		if err != nil {
			t.Fatalf("Encode resume response: %v", err)
		}
		backResume, err := cfdp.DecodeRemoteResumeResponse(resumeResponse.Content)
		if err != nil {
			t.Fatalf("Decode resume response: %v", err)
		}
		if backResume.Suspended != suspended {
			t.Errorf("resume response suspended = %v, want %v", backResume.Suspended, suspended)
		}

		// The two responses differ only in their message type, which is what
		// makes sharing the body correct.
		if !bytes.Equal(suspendResponse.Content, resumeResponse.Content) {
			t.Error("the suspend and resume response bodies differ; tables 6-22 and 6-25 are the same")
		}
		if suspendResponse.Type == resumeResponse.Type {
			t.Error("the two responses share a message type")
		}
	}
}

// The single-flag messages: seven spare bits and one flag. Two of the three
// read inverted from the wire, because the standard encodes the useful state
// as '0'.
func TestSingleFlagProxyMessages(t *testing.T) {
	// Table 5-1: '0' is acknowledged.
	acked := cfdp.ProxyTransmissionMode{Acknowledged: true}.Encode()
	if acked.Content[0] != 0 {
		t.Errorf("acknowledged encoded as 0x%02X, want 0x00", acked.Content[0])
	}
	back, err := cfdp.DecodeProxyTransmissionMode(acked.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !back.Acknowledged {
		t.Error("acknowledged did not survive the round trip")
	}

	// Table 6-10: '0' means record boundaries respected.
	respected := cfdp.ProxySegmentationControl{RecordBoundariesRespected: true}.Encode()
	if respected.Content[0] != 0 {
		t.Errorf("boundaries respected encoded as 0x%02X, want 0x00", respected.Content[0])
	}

	// Table 6-11: '1' means closure requested, the plain direction.
	closure := cfdp.ProxyClosureRequest{ClosureRequested: true}.Encode()
	if closure.Content[0] != 1 {
		t.Errorf("closure requested encoded as 0x%02X, want 0x01", closure.Content[0])
	}
	backClosure, err := cfdp.DecodeProxyClosureRequest(closure.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !backClosure.ClosureRequested {
		t.Error("closure requested did not survive the round trip")
	}
}

func TestSingleFlagMessagesRejectSpareBits(t *testing.T) {
	if _, err := cfdp.DecodeProxyClosureRequest([]byte{0x02}); !errors.Is(err, cfdp.ErrReservedBitsSet) {
		t.Errorf("err = %v, want ErrReservedBitsSet", err)
	}
	if _, err := cfdp.DecodeProxyTransmissionMode([]byte{0xFE}); !errors.Is(err, cfdp.ErrReservedBitsSet) {
		t.Errorf("err = %v, want ErrReservedBitsSet", err)
	}
	if _, err := cfdp.DecodeProxyClosureRequest([]byte{0x01, 0x00}); err == nil {
		t.Error("a two-octet single-flag message was accepted")
	}
}

func TestProxyFilestoreRoundTrip(t *testing.T) {
	request := cfdp.ProxyFilestoreRequest{
		Request: cfdp.FilestoreRequest{
			Action:        cfdp.ActionCreateFile,
			FirstFileName: cfdp.LV{Value: []byte("/tmp/new.dat")},
		},
	}
	message, err := request.Encode()
	if err != nil {
		t.Fatalf("Encode request: %v", err)
	}
	back, err := cfdp.DecodeProxyFilestoreRequest(message.Content)
	if err != nil {
		t.Fatalf("Decode request: %v", err)
	}
	if back.Request.Action != cfdp.ActionCreateFile {
		t.Errorf("action = %s, want create file", back.Request.Action)
	}
	if back.Request.FirstFileName.String() != "/tmp/new.dat" {
		t.Errorf("file name = %q, want /tmp/new.dat", back.Request.FirstFileName.String())
	}

	response := cfdp.ProxyFilestoreResponse{
		Response: cfdp.FilestoreResponse{
			Action:        cfdp.ActionCreateFile,
			StatusCode:    0,
			FirstFileName: cfdp.LV{Value: []byte("/tmp/new.dat")},
		},
	}
	message, err = response.Encode()
	if err != nil {
		t.Fatalf("Encode response: %v", err)
	}
	backResponse, err := cfdp.DecodeProxyFilestoreResponse(message.Content)
	if err != nil {
		t.Fatalf("Decode response: %v", err)
	}
	if backResponse.Response.Action != cfdp.ActionCreateFile {
		t.Errorf("action = %s, want create file", backResponse.Response.Action)
	}
}

// Tables 6-6 and 6-13 put an eight-bit length in front of the value, doing
// the job the TLV's own length would. A length that disagrees with what
// follows is a broken message.
func TestProxyFilestoreRejectsBadLength(t *testing.T) {
	if _, err := cfdp.DecodeProxyFilestoreRequest([]byte{0x10, 0x01}); !errors.Is(err, cfdp.ErrDataTooShort) {
		t.Errorf("err = %v, want ErrDataTooShort for a length past the end", err)
	}
	if _, err := cfdp.DecodeProxyFilestoreRequest([]byte{0x01, 0x01, 0x02}); !errors.Is(err, cfdp.ErrDataLengthMismatch) {
		t.Errorf("err = %v, want ErrDataLengthMismatch for trailing octets", err)
	}
}

func TestProxyLVMessages(t *testing.T) {
	text, err := cfdp.ProxyMessageToUser{Text: []byte("for the beneficiary")}.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	backText, err := cfdp.DecodeProxyMessageToUser(text.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if string(backText.Text) != "for the beneficiary" {
		t.Errorf("text = %q", backText.Text)
	}

	label, err := cfdp.ProxyFlowLabel{Label: []byte{0xAA, 0xBB}}.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	backLabel, err := cfdp.DecodeProxyFlowLabel(label.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !bytes.Equal(backLabel.Label, []byte{0xAA, 0xBB}) {
		t.Errorf("label = %x", backLabel.Label)
	}
}

func TestProxyFaultHandlerOverrideRoundTrip(t *testing.T) {
	original := cfdp.ProxyFaultHandlerOverride{
		Condition: cfdp.CondFileChecksumFailure,
		Handler:   cfdp.FaultHandlerSuspend,
	}

	message := original.Encode()
	back, err := cfdp.DecodeProxyFaultHandlerOverride(message.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if *back != original {
		t.Errorf("got %+v, want %+v", *back, original)
	}
}

// A message with more length-value fields than its table defines is carrying
// something this issue does not describe, and reading past what is known
// would be a guess either way.
func TestLVMessagesRejectTrailingFields(t *testing.T) {
	request := cfdp.DirectoryListingRequest{DirectoryName: "/a", DirectoryFileName: "/b"}
	message, err := request.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	extended := append(append([]byte{}, message.Content...), 0x01, 0xFF)
	if _, err := cfdp.DecodeDirectoryListingRequest(extended); !errors.Is(err, cfdp.ErrDataLengthMismatch) {
		t.Errorf("err = %v, want ErrDataLengthMismatch", err)
	}
}

// Truncated messages are reported rather than read past the end of.
func TestUserMessagesRejectTruncation(t *testing.T) {
	for name, decode := range map[string]func([]byte) error{
		"proxy put request": func(b []byte) error {
			_, err := cfdp.DecodeProxyPutRequest(b)
			return err
		},
		"proxy put response": func(b []byte) error {
			_, err := cfdp.DecodeProxyPutResponse(b)
			return err
		},
		"directory listing response": func(b []byte) error {
			_, err := cfdp.DecodeDirectoryListingResponse(b)
			return err
		},
		"status report response": func(b []byte) error {
			_, err := cfdp.DecodeRemoteStatusReportResponse(b)
			return err
		},
		"suspend response": func(b []byte) error {
			_, err := cfdp.DecodeRemoteSuspendResponse(b)
			return err
		},
		"originating transaction id": func(b []byte) error {
			_, err := cfdp.DecodeOriginatingTransactionID(b)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode(nil); err == nil {
				t.Error("an empty message was accepted")
			}
		})
	}
}

// A whole proxy put, as a user would build it: the mandatory request, the
// mandatory originating transaction ID, and the optional messages, all in one
// transaction's metadata.
func TestCompleteProxyPutMetadata(t *testing.T) {
	id := cfdp.TransactionID{Source: cfdp.NewEntityID(1), Sequence: cfdp.NewEntityID(100)}

	put, err := cfdp.ProxyPutRequest{
		Destination:         cfdp.NewEntityID(3),
		SourceFileName:      "/remote/a.dat",
		DestinationFileName: "/local/a.dat",
	}.Encode()
	if err != nil {
		t.Fatalf("Encode put: %v", err)
	}
	origin, err := cfdp.OriginatingTransactionID{Transaction: id}.Encode()
	if err != nil {
		t.Fatalf("Encode origin: %v", err)
	}

	tlvs := []cfdp.TLV{
		put.EncodeTLV(),
		origin.EncodeTLV(),
		cfdp.ProxyTransmissionMode{Acknowledged: true}.Encode().EncodeTLV(),
		cfdp.ProxyClosureRequest{ClosureRequested: true}.Encode().EncodeTLV(),
	}

	// Through a real metadata encode and decode, so the TLV framing is
	// exercised rather than assumed.
	metadata := &cfdp.MetadataPDU{
		SourceFileName:      cfdp.LV{Value: []byte("/dev/null")},
		DestinationFileName: cfdp.LV{Value: []byte("/dev/null")},
		Options:             tlvs,
	}
	encoded, err := metadata.Encode(false)
	if err != nil {
		t.Fatalf("Encode metadata: %v", err)
	}
	decoded, err := cfdp.DecodeMetadataPDU(encoded, false)
	if err != nil {
		t.Fatalf("Decode metadata: %v", err)
	}

	messages := cfdp.UserMessagesFrom(decoded.Options)
	if len(messages) != 4 {
		t.Fatalf("found %d user messages in the metadata, want 4", len(messages))
	}

	want := []cfdp.UserMessageType{
		cfdp.MsgProxyPutRequest,
		cfdp.MsgOriginatingTransactionID,
		cfdp.MsgProxyTransmissionMode,
		cfdp.MsgProxyClosureRequest,
	}
	for i, kind := range want {
		if messages[i].Type != kind {
			t.Errorf("message %d is %s, want %s", i, messages[i].Type, kind)
		}
	}

	// And the mandatory two still say what they said.
	backPut, err := cfdp.DecodeProxyPutRequest(messages[0].Content)
	if err != nil {
		t.Fatalf("Decode put: %v", err)
	}
	if backPut.SourceFileName != "/remote/a.dat" {
		t.Errorf("source file = %q", backPut.SourceFileName)
	}

	backOrigin, err := cfdp.DecodeOriginatingTransactionID(messages[1].Content)
	if err != nil {
		t.Fatalf("Decode origin: %v", err)
	}
	if backOrigin.Transaction.Sequence.Value != 100 {
		t.Errorf("originating sequence = %d, want 100", backOrigin.Transaction.Sequence.Value)
	}
}
