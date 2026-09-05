package cfdp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/cfdp"
)

// seedPDU returns one valid encoded PDU of the given directive body.
func seedPDU(t *testing.F, body []byte, isFileData, crc bool) []byte {
	t.Helper()
	pdu := &cfdp.PDU{
		Header: &cfdp.PDUHeader{
			IsFileData:     isFileData,
			Direction:      cfdp.TowardReceiver,
			Acknowledged:   true,
			CRCFlag:        crc,
			Source:         cfdp.EntityID{Value: 1, Width: 1},
			TransactionSeq: cfdp.EntityID{Value: 1, Width: 1},
			Destination:    cfdp.EntityID{Value: 2, Width: 1},
		},
		Data: body,
	}
	encoded, err := pdu.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func FuzzDecodePDU(f *testing.F) {
	// Seed with one of every PDU kind so the fuzzer starts from real structure.
	meta := &cfdp.MetadataPDU{
		ChecksumType:        cfdp.ChecksumModular,
		FileSize:            16,
		SourceFileName:      cfdp.LV{Value: []byte("a")},
		DestinationFileName: cfdp.LV{Value: []byte("b")},
	}
	if body, err := meta.Encode(false); err == nil {
		f.Add(seedPDU(f, body, false, false))
		f.Add(seedPDU(f, body, false, true))
	}

	eof := &cfdp.EOFPDU{FileChecksum: 0x1234, FileSize: 16}
	if body, err := eof.Encode(false); err == nil {
		f.Add(seedPDU(f, body, false, false))
	}

	fd := &cfdp.FileDataPDU{Offset: 0, Data: []byte("data")}
	if body, err := fd.Encode(false, false); err == nil {
		f.Add(seedPDU(f, body, true, false))
	}

	nak := &cfdp.NAKPDU{EndOfScope: 100, Requests: []cfdp.SegmentRequest{{StartOffset: 0, EndOffset: 10}}}
	if body, err := nak.Encode(false); err == nil {
		f.Add(seedPDU(f, body, false, false))
	}

	f.Add([]byte{})
	f.Add(make([]byte, 4))
	f.Add(make([]byte, 32))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		pdu, err := cfdp.DecodePDU(data)
		if err != nil {
			return
		}

		// A PDU that decodes must re-encode without panicking.
		if _, err := pdu.Encode(); err != nil {
			return
		}

		if pdu.Header.IsFileData {
			_, _ = cfdp.DecodeFileDataPDU(pdu.Data, pdu.Header.SegmentMetadataFlag, pdu.Header.LargeFile)
			return
		}

		code, err := cfdp.DirectiveCodeOf(pdu.Data)
		if err != nil {
			return
		}
		large := pdu.Header.LargeFile
		switch code {
		case cfdp.DirectiveEOF:
			_, _ = cfdp.DecodeEOFPDU(pdu.Data, large)
		case cfdp.DirectiveFinished:
			_, _ = cfdp.DecodeFinishedPDU(pdu.Data)
		case cfdp.DirectiveACK:
			_, _ = cfdp.DecodeACKPDU(pdu.Data)
		case cfdp.DirectiveMetadata:
			_, _ = cfdp.DecodeMetadataPDU(pdu.Data, large)
		case cfdp.DirectiveNAK:
			_, _ = cfdp.DecodeNAKPDU(pdu.Data, large)
		case cfdp.DirectivePrompt:
			_, _ = cfdp.DecodePromptPDU(pdu.Data)
		case cfdp.DirectiveKeepAlive:
			_, _ = cfdp.DecodeKeepAlivePDU(pdu.Data, large)
		}
	})
}

func FuzzDecodeTLV(f *testing.F) {
	if tlv, err := (cfdp.TLV{Type: cfdp.TLVEntityID, Value: []byte{1, 2}}).Encode(); err == nil {
		f.Add(tlv)
	}
	req := cfdp.FilestoreRequest{
		Action:         cfdp.ActionRenameFile,
		FirstFileName:  cfdp.LV{Value: []byte("a")},
		SecondFileName: cfdp.LV{Value: []byte("b")},
	}
	if tlv, err := req.Encode(); err == nil {
		if encoded, err := tlv.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x00, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic on any of the TLV entry points.
		tlvs, err := cfdp.DecodeTLVs(data)
		if err != nil {
			return
		}
		for _, tlv := range tlvs {
			switch tlv.Type {
			case cfdp.TLVFilestoreRequest:
				_, _ = cfdp.DecodeFilestoreRequest(tlv)
			case cfdp.TLVFilestoreResponse:
				_, _ = cfdp.DecodeFilestoreResponse(tlv)
			case cfdp.TLVEntityID:
				_, _ = tlv.AsEntityID()
			}
		}
		_, _, _ = cfdp.DecodeLV(data)
	})
}

func FuzzReceiverHandle(f *testing.F) {
	fd := &cfdp.FileDataPDU{Offset: 0, Data: []byte("seed")}
	if body, err := fd.Encode(false, false); err == nil {
		f.Add(seedPDU(f, body, true, false))
	}

	// A File Data PDU naming a huge offset, the shape that once made
	// MemoryFilestore.WriteAt try to allocate the declared offset in memory
	// (see MaxFileSize on ReceiverConfig). This harness only feeds one PDU
	// per iteration, with a fresh Receiver each time, so it cannot exercise
	// the metadata-then-huge-offset sequence the real vulnerability needs;
	// TestFileDataOffsetPastMaxFileSizeFaults and
	// TestFileDataOffsetOverflowNoPanic in limits_test.go cover that path.
	huge := &cfdp.FileDataPDU{Offset: 1 << 62, Data: []byte("seed")}
	if body, err := huge.Encode(false, true); err == nil {
		pdu := &cfdp.PDU{
			Header: &cfdp.PDUHeader{
				IsFileData:     true,
				Direction:      cfdp.TowardReceiver,
				Acknowledged:   true,
				LargeFile:      true,
				Source:         cfdp.EntityID{Value: 1, Width: 1},
				TransactionSeq: cfdp.EntityID{Value: 1, Width: 1},
				Destination:    cfdp.EntityID{Value: 2, Width: 1},
			},
			Data: body,
		}
		if encoded, err := pdu.Encode(); err == nil {
			f.Add(encoded)
		}
	}

	f.Add([]byte{})
	f.Add(make([]byte, 8))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: arbitrary bytes pumped into a receiver never panic, and
		// never leave the state machine wedged in a way that panics later.
		fs := cfdp.NewMemoryFilestore()
		rx := cfdp.NewReceiver(fs, cfdp.ReceiverConfig{
			Source:         cfdp.EntityID{Value: 1, Width: 1},
			Destination:    cfdp.EntityID{Value: 2, Width: 1},
			TransactionSeq: cfdp.EntityID{Value: 1, Width: 1},
			Acknowledged:   true,
		})

		pdu, err := cfdp.DecodePDU(data)
		if err != nil {
			return
		}
		_ = rx.HandlePDU(pdu)
		_, _, _ = rx.NextPDU()
		_ = rx.MissingSegments()
		_ = rx.Complete()
	})
}

// FuzzDecodeUserMessage throws arbitrary bytes at every Part 2 message
// decoder.
//
// These are the newest wire decoders in the package and they all read
// length-prefixed and width-prefixed fields out of untrusted input, which is
// exactly the shape that reads past the end of a buffer when a length is not
// checked against what remains.
func FuzzDecodeUserMessage(f *testing.F) {
	// Seeds: one well-formed message of each shape, so the fuzzer starts from
	// something that parses rather than from noise.
	id := cfdp.TransactionID{Source: cfdp.NewEntityID(1), Sequence: cfdp.NewEntityID(2)}

	origin, _ := cfdp.OriginatingTransactionID{Transaction: id}.Encode()
	put, _ := cfdp.ProxyPutRequest{
		Destination:         cfdp.NewEntityID(3),
		SourceFileName:      "/a",
		DestinationFileName: "/b",
	}.Encode()
	response, _ := cfdp.ProxyPutResponse{}.Encode()
	listing, _ := cfdp.DirectoryListingResponse{DirectoryName: "/d", DirectoryFileName: "/f"}.Encode()
	report, _ := cfdp.RemoteStatusReportResponse{Transaction: id}.Encode()
	suspend, _ := cfdp.RemoteSuspendResponse{
		SuspensionResponse: cfdp.SuspensionResponse{Transaction: id},
	}.Encode()

	for _, message := range []cfdp.UserMessage{
		origin, put, response, listing, report, suspend,
		cfdp.ProxyPutCancel(),
		cfdp.ProxyClosureRequest{ClosureRequested: true}.Encode(),
	} {
		f.Add(message.Encode())
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: no input makes any decoder panic. Every one is reached,
		// whatever the type octet says, because a decoder must be safe
		// against content that was never meant for it.
		message, err := cfdp.DecodeUserMessage(data)
		if err != nil {
			return
		}

		_ = message.Type.String()
		_ = message.Type.Valid()

		_, _ = cfdp.DecodeOriginatingTransactionID(message.Content)
		_, _ = cfdp.DecodeProxyPutRequest(message.Content)
		_, _ = cfdp.DecodeProxyPutResponse(message.Content)
		_, _ = cfdp.DecodeProxyMessageToUser(message.Content)
		_, _ = cfdp.DecodeProxyFilestoreRequest(message.Content)
		_, _ = cfdp.DecodeProxyFilestoreResponse(message.Content)
		_, _ = cfdp.DecodeProxyFaultHandlerOverride(message.Content)
		_, _ = cfdp.DecodeProxyTransmissionMode(message.Content)
		_, _ = cfdp.DecodeProxySegmentationControl(message.Content)
		_, _ = cfdp.DecodeProxyFlowLabel(message.Content)
		_, _ = cfdp.DecodeProxyClosureRequest(message.Content)
		_, _ = cfdp.DecodeDirectoryListingRequest(message.Content)
		_, _ = cfdp.DecodeDirectoryListingResponse(message.Content)
		_, _ = cfdp.DecodeRemoteStatusReportRequest(message.Content)
		_, _ = cfdp.DecodeRemoteStatusReportResponse(message.Content)
		_, _ = cfdp.DecodeRemoteSuspendRequest(message.Content)
		_, _ = cfdp.DecodeRemoteSuspendResponse(message.Content)
		_, _ = cfdp.DecodeRemoteResumeRequest(message.Content)
		_, _ = cfdp.DecodeRemoteResumeResponse(message.Content)
	})
}
