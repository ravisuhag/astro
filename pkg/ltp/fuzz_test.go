package ltp_test

import (
	"testing"

	"github.com/ravisuhag/astro/pkg/ltp"
)

func FuzzDecodeSegment(f *testing.F) {
	// Seed with one of every segment shape.
	seeds := []*ltp.Segment{
		{
			Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
			Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("data")},
		},
		{
			Header: &ltp.Header{Type: ltp.TypeRedDataCheckpointEORPEOB, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
			Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("data"), CheckpointSerial: 1},
		},
		{
			Header: &ltp.Header{Type: ltp.TypeReport, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
			Report: &ltp.ReportSegment{
				ReportSerial: 1, UpperBound: 100,
				Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 50}},
			},
		},
		{
			Header:    &ltp.Header{Type: ltp.TypeReportAck, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
			ReportAck: &ltp.ReportAckSegment{ReportSerial: 1},
		},
		{
			Header: &ltp.Header{Type: ltp.TypeCancelFromSender, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
			Cancel: &ltp.CancelSegment{Reason: ltp.ReasonUserCancelled},
		},
		{
			Header: &ltp.Header{Type: ltp.TypeCancelAckToSender, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
		},
	}
	for _, s := range seeds {
		if encoded, err := s.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 16))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic, and anything that decodes must re-encode.
		seg, err := ltp.DecodeSegment(data)
		if err != nil {
			return
		}
		if _, err := seg.Encode(); err != nil {
			t.Fatalf("a decoded segment failed to re-encode: %v", err)
		}
	})
}

func FuzzReceiverHandle(f *testing.F) {
	seg := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: ltp.SessionID{EngineID: 1, SessionNumber: 1}},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("seed")},
	}
	if encoded, err := seg.Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 8))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: arbitrary segments pumped into a receiver never panic and
		// never wedge it into panicking later.
		r, err := ltp.NewReceiver(ltp.ReceiverConfig{
			SessionID:         ltp.SessionID{EngineID: 1, SessionNumber: 1},
			FirstReportSerial: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		seg, err := ltp.DecodeSegment(data)
		if err != nil {
			return
		}
		_ = r.HandleSegment(seg)
		_, _, _ = r.NextSegment()
		_ = r.MissingRanges()
		_ = r.Block()
		_ = r.Complete()
	})
}
