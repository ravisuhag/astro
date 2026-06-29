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
