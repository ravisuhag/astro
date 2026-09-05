package ltp_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/ravisuhag/astro/pkg/ltp"
)

// testBlock returns a block with recognizable contents.
func testBlock(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 3)
	}
	return b
}

// pump runs sender and receiver against each other over encoded segments,
// optionally dropping the nth segment the sender emits.
func pump(t *testing.T, s *ltp.Sender, r *ltp.Receiver, dropIndex int) {
	t.Helper()

	sent := 0
	for round := 0; round < 200; round++ {
		progressed := false

		for {
			seg, ok, err := s.NextSegment()
			if err != nil {
				t.Fatalf("sender: %v", err)
			}
			if !ok {
				break
			}
			progressed = true

			index := sent
			sent++
			if index == dropIndex {
				continue // lost in transit
			}

			raw, err := seg.Encode()
			if err != nil {
				t.Fatalf("encoding: %v", err)
			}
			decoded, err := ltp.DecodeSegment(raw)
			if err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if err := r.HandleSegment(decoded); err != nil {
				t.Fatalf("receiver rejected a segment: %v", err)
			}
		}

		for {
			seg, ok, err := r.NextSegment()
			if err != nil {
				t.Fatalf("receiver: %v", err)
			}
			if !ok {
				break
			}
			progressed = true

			raw, err := seg.Encode()
			if err != nil {
				t.Fatalf("encoding: %v", err)
			}
			decoded, err := ltp.DecodeSegment(raw)
			if err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if err := s.HandleSegment(decoded); err != nil {
				t.Fatalf("sender rejected a segment: %v", err)
			}
		}

		if !progressed {
			return
		}
	}
	t.Fatal("sessions did not settle within 200 rounds")
}

func TestAllRedBlockDelivery(t *testing.T) {
	block := testBlock(500)

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		ClientServiceID:       1,
		SegmentSize:           64,
		RedPartLength:         uint64(len(block)), // all red
		FirstCheckpointSerial: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 200,
	})
	if err != nil {
		t.Fatal(err)
	}

	pump(t, sender, receiver, -1)

	if !receiver.RedPartComplete() {
		t.Fatalf("red part incomplete; missing %+v", receiver.MissingRanges())
	}
	if !bytes.Equal(receiver.RedPart(), block) {
		t.Error("delivered red part differs from the source block")
	}
	if !sender.RedPartAcknowledged() {
		t.Error("sender does not consider the red part acknowledged")
	}
}

func TestAllGreenBlockDelivery(t *testing.T) {
	block := testBlock(300)

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		ClientServiceID:       1,
		SegmentSize:           64,
		RedPartLength:         0, // all green
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	pump(t, sender, receiver, -1)

	// Green data is best effort: no reports, no acknowledgment, but with a
	// clean link everything still arrives.
	if !receiver.Complete() {
		t.Error("green block did not arrive complete over a clean link")
	}
	if !bytes.Equal(receiver.Block(), block) {
		t.Error("delivered green block differs from the source")
	}
}

func TestMixedRedGreenBlock(t *testing.T) {
	block := testBlock(400)
	redLen := uint64(200)

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		ClientServiceID:       1,
		SegmentSize:           64,
		RedPartLength:         redLen,
		FirstCheckpointSerial: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	pump(t, sender, receiver, -1)

	if !receiver.RedPartComplete() {
		t.Fatalf("red part incomplete; missing %+v", receiver.MissingRanges())
	}
	if !bytes.Equal(receiver.RedPart(), block[:redLen]) {
		t.Error("red part differs from the source")
	}
	if !receiver.Complete() {
		t.Error("block incomplete over a clean link")
	}
	if !bytes.Equal(receiver.Block(), block) {
		t.Error("whole block differs from the source")
	}
}

func TestRedPartRecoversLostSegment(t *testing.T) {
	// The point of the red part: a lost segment must be retransmitted until
	// the receiver has it.
	block := testBlock(500)

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		ClientServiceID:       1,
		SegmentSize:           64,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 200,
	})
	if err != nil {
		t.Fatal(err)
	}

	pump(t, sender, receiver, 2) // drop the third segment

	if !receiver.RedPartComplete() {
		t.Fatalf("recovery failed; still missing %+v", receiver.MissingRanges())
	}
	if !bytes.Equal(receiver.RedPart(), block) {
		t.Error("recovered red part differs from the source")
	}
}

func TestReceiverReportsGapsAfterLoss(t *testing.T) {
	block := testBlock(300)

	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		ClientServiceID:       1,
		SegmentSize:           100,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Deliver segments one and three, drop the middle one.
	index := 0
	for {
		seg, ok, err := sender.NextSegment()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if index == 1 {
			index++
			continue
		}
		index++

		raw, _ := seg.Encode()
		decoded, err := ltp.DecodeSegment(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := receiver.HandleSegment(decoded); err != nil {
			t.Fatal(err)
		}
	}

	missing := receiver.MissingRanges()
	if len(missing) == 0 {
		t.Fatal("receiver reported no gap after a segment was dropped")
	}
	if missing[0].Offset != 100 || missing[0].Length != 100 {
		t.Errorf("gap = offset %d length %d, want offset 100 length 100",
			missing[0].Offset, missing[0].Length)
	}
}

func TestMiscoloredBlockCancelled(t *testing.T) {
	// Clause 3.2.4: green data below a red offset is MISCOLORED.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	red := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: testSession()},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 100, Data: []byte("red")},
	}
	if err := receiver.HandleSegment(red); err != nil {
		t.Fatal(err)
	}

	// Green data below the red part's end.
	green := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeGreenData, SessionID: testSession()},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("green")},
	}
	if err := receiver.HandleSegment(green); !errors.Is(err, ltp.ErrRedGreenOrder) {
		t.Errorf("error = %v, want ErrRedGreenOrder", err)
	}

	// The receiver must cancel with the MISCOLORED reason.
	seg, ok, err := receiver.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a cancel segment")
	}
	if seg.Header.Type != ltp.TypeCancelFromReceiver {
		t.Fatalf("type = %s, want cancel from receiver", seg.Header.Type)
	}
	if seg.Cancel.Reason != ltp.ReasonMiscolored {
		t.Errorf("reason = %s, want miscolored", seg.Cancel.Reason)
	}
}

func TestSenderCancel(t *testing.T) {
	block := testBlock(100)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           64,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sender.Cancel(ltp.ReasonUserCancelled); err != nil {
		t.Fatal(err)
	}

	seg, ok, err := sender.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a cancel segment")
	}
	if seg.Header.Type != ltp.TypeCancelFromSender {
		t.Errorf("type = %s, want cancel from sender", seg.Header.Type)
	}
	if seg.Cancel.Reason != ltp.ReasonUserCancelled {
		t.Errorf("reason = %s, want user cancelled", seg.Cancel.Reason)
	}
	if !sender.Done() {
		t.Error("sender is not done after cancelling")
	}

	// Only one cancel goes out.
	if _, ok, _ := sender.NextSegment(); ok {
		t.Error("a second segment was emitted after the cancel")
	}
}

func TestReceiverAcknowledgesCancel(t *testing.T) {
	// Clause 3.2.5: a cancel is acknowledged.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	cancel := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeCancelFromSender, SessionID: testSession()},
		Cancel: &ltp.CancelSegment{Reason: ltp.ReasonSystemCancelled},
	}
	if err := receiver.HandleSegment(cancel); err != nil {
		t.Fatal(err)
	}

	seg, ok, err := receiver.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a cancel acknowledgment")
	}
	if seg.Header.Type != ltp.TypeCancelAckToSender {
		t.Errorf("type = %s, want cancel acknowledgment to sender", seg.Header.Type)
	}
	if !receiver.Done() {
		t.Error("receiver is not done after a cancel")
	}
}

func TestSessionsIgnoreOtherSessionsTraffic(t *testing.T) {
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	other := &ltp.Segment{
		Header: &ltp.Header{
			Type:      ltp.TypeRedData,
			SessionID: ltp.SessionID{EngineID: 99, SessionNumber: 99},
		},
		Data: &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: []byte("not ours")},
	}
	if err := receiver.HandleSegment(other); err != nil {
		t.Fatalf("another session's segment produced an error: %v", err)
	}
	if len(receiver.Block()) != 0 {
		t.Error("another session's data was stored")
	}
}

func TestSerialNumbersMustNotBeZero(t *testing.T) {
	// Clause 3.2.1 and clause 3.2.2 both forbid zero, and this package refuses rather
	// than inventing randomness of its own.
	if _, err := ltp.NewSender(nil, ltp.SenderConfig{FirstCheckpointSerial: 0}); !errors.Is(err, ltp.ErrInvalidSerialNumber) {
		t.Errorf("sender: error = %v, want ErrInvalidSerialNumber", err)
	}
	if _, err := ltp.NewReceiver(ltp.ReceiverConfig{FirstReportSerial: 0}); !errors.Is(err, ltp.ErrInvalidSerialNumber) {
		t.Errorf("receiver: error = %v, want ErrInvalidSerialNumber", err)
	}
}

func TestResendCheckpointRequeuesGaps(t *testing.T) {
	block := testBlock(200)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           100,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Drain everything, so the sender is waiting for a report.
	for {
		if _, ok, _ := sender.NextSegment(); !ok {
			break
		}
	}
	if sender.State() != ltp.StateWaitingReport {
		t.Fatalf("state = %s, want waiting for report", sender.State())
	}

	// The caller's timer fires: nothing came back.
	sender.ResendCheckpoint()

	if _, ok, err := sender.NextSegment(); err != nil || !ok {
		t.Errorf("ResendCheckpoint queued nothing (ok=%t, err=%v)", ok, err)
	}
}

func TestFinalReportIsAcknowledged(t *testing.T) {
	// LTP-1, clause 6.13/clause 6.14: the report that completes the red part closes the
	// session, but its acknowledgment must still go out. A conformant peer
	// retransmits that report forever without it.
	block := testBlock(100)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           64,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 7,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Drain the data, then answer with a report claiming everything.
	for {
		if _, ok, _ := sender.NextSegment(); !ok {
			break
		}
	}
	report := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()},
		Report: &ltp.ReportSegment{
			ReportSerial: 31, CheckpointSerial: 7, UpperBound: 100,
			Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 100}},
		},
	}
	if err := sender.HandleSegment(report); err != nil {
		t.Fatal(err)
	}
	if sender.State() != ltp.StateClosed {
		t.Fatalf("state = %s, want closed after a full claim", sender.State())
	}

	seg, ok, err := sender.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the closed sender never emitted the final report acknowledgment")
	}
	if seg.Header.Type != ltp.TypeReportAck {
		t.Fatalf("type = %s, want report acknowledgment", seg.Header.Type)
	}
	if seg.ReportAck.ReportSerial != 31 {
		t.Errorf("acknowledged serial = %d, want 31", seg.ReportAck.ReportSerial)
	}
}

func TestInteriorGapRetransmissionEndsWithCheckpoint(t *testing.T) {
	// LTP-2, clause 6.9: the last segment of a retransmission cycle is a checkpoint
	// wherever it sits. An interior gap must not leave the sender wedged in
	// StateWaitingReport with nothing prompting the next report.
	// LTP-3, clause 3.2.1: that checkpoint carries the prompting report's serial.
	block := testBlock(300)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           100,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, ok, _ := sender.NextSegment(); !ok {
			break
		}
	}

	// The middle segment went missing: claims cover [0,100) and [200,300).
	report := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()},
		Report: &ltp.ReportSegment{
			ReportSerial: 55, CheckpointSerial: 40, UpperBound: 300,
			Claims: []ltp.ReceptionClaim{
				{Offset: 0, Length: 100},
				{Offset: 200, Length: 100},
			},
		},
	}
	if err := sender.HandleSegment(report); err != nil {
		t.Fatal(err)
	}

	// First out is the report acknowledgment, then the retransmission.
	seg, ok, err := sender.NextSegment()
	if err != nil || !ok || seg.Header.Type != ltp.TypeReportAck {
		t.Fatalf("expected a report ack first, got %v (ok=%t, err=%v)", seg, ok, err)
	}

	seg, ok, err = sender.NextSegment()
	if err != nil || !ok {
		t.Fatalf("no retransmission emitted (ok=%t, err=%v)", ok, err)
	}
	if !seg.Header.Type.IsCheckpoint() {
		t.Fatalf("type = %s; the cycle's last segment must be a checkpoint", seg.Header.Type)
	}
	if seg.Data.Offset != 100 || len(seg.Data.Data) != 100 {
		t.Errorf("retransmitted offset %d length %d, want the 100..200 gap",
			seg.Data.Offset, len(seg.Data.Data))
	}
	if seg.Data.ReportSerial != 55 {
		t.Errorf("checkpoint report serial = %d, want the prompting report's 55",
			seg.Data.ReportSerial)
	}
	if sender.State() != ltp.StateWaitingReport {
		t.Errorf("state = %s, want waiting for report", sender.State())
	}
}

func TestSenderAcknowledgesCancelFromReceiver(t *testing.T) {
	// LTP-4, clause 6.17: a cancel from the receiver is acknowledged, or the
	// receiver's cancel timer retransmits it forever.
	block := testBlock(100)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           64,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	cancel := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeCancelFromReceiver, SessionID: testSession()},
		Cancel: &ltp.CancelSegment{Reason: ltp.ReasonUserCancelled},
	}
	if err := sender.HandleSegment(cancel); err != nil {
		t.Fatal(err)
	}

	seg, ok, err := sender.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a cancel acknowledgment to the receiver")
	}
	if seg.Header.Type != ltp.TypeCancelAckToReceiver {
		t.Fatalf("type = %s, want cancel acknowledgment to block receiver", seg.Header.Type)
	}
	if !sender.Done() {
		t.Error("sender is not done after the receiver cancelled")
	}
	// No data follows the teardown.
	if _, ok, _ := sender.NextSegment(); ok {
		t.Error("a segment was emitted after the session was cancelled")
	}
}

func TestRetransmittedCheckpointGetsSameReport(t *testing.T) {
	// LTP-6, clause 6.11: a checkpoint the receiver has already answered is
	// answered with the same report, not a fresh serial.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 9,
	})
	if err != nil {
		t.Fatal(err)
	}

	cp := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedDataCheckpointEORPEOB, SessionID: testSession()},
		Data: &ltp.DataSegment{
			ClientServiceID: 1, Offset: 0, Data: testBlock(50), CheckpointSerial: 3,
		},
	}
	if err := receiver.HandleSegment(cp); err != nil {
		t.Fatal(err)
	}
	first, ok, _ := receiver.NextSegment()
	if !ok || first.Header.Type != ltp.TypeReport {
		t.Fatal("expected a report for the checkpoint")
	}

	// The same checkpoint arrives again (the sender's timer fired).
	if err := receiver.HandleSegment(cp); err != nil {
		t.Fatal(err)
	}
	second, ok, _ := receiver.NextSegment()
	if !ok || second.Header.Type != ltp.TypeReport {
		t.Fatal("expected the report to be resent")
	}
	if second.Report.ReportSerial != first.Report.ReportSerial {
		t.Errorf("resent report serial = %d, want the original %d",
			second.Report.ReportSerial, first.Report.ReportSerial)
	}
}

func TestReceiverClosureGatedOnReportAck(t *testing.T) {
	// LTP-8, clause 6.11/clause 6.16: the receiver keeps the session open until its
	// report is acknowledged, then closes. A green EOB closes an all-green
	// session directly.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	cp := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedDataCheckpointEORPEOB, SessionID: testSession()},
		Data: &ltp.DataSegment{
			ClientServiceID: 1, Offset: 0, Data: testBlock(50), CheckpointSerial: 3,
		},
	}
	if err := receiver.HandleSegment(cp); err != nil {
		t.Fatal(err)
	}
	if receiver.State() != ltp.StateActive {
		t.Fatalf("state = %s before the report ack, want active", receiver.State())
	}

	ack := &ltp.Segment{
		Header:    &ltp.Header{Type: ltp.TypeReportAck, SessionID: testSession()},
		ReportAck: &ltp.ReportAckSegment{ReportSerial: 9},
	}
	if err := receiver.HandleSegment(ack); err != nil {
		t.Fatal(err)
	}
	if receiver.State() != ltp.StateClosed {
		t.Errorf("state = %s after the report ack, want closed", receiver.State())
	}
}

func TestAllGreenSessionClosesOnGreenEOB(t *testing.T) {
	// LTP-8, clause 6.16: an all-green session involves no reports, so the green
	// EOB is its only close signal.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	eob := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeGreenDataEOB, SessionID: testSession()},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 0, Data: testBlock(40)},
	}
	if err := receiver.HandleSegment(eob); err != nil {
		t.Fatal(err)
	}
	if receiver.State() != ltp.StateClosed {
		t.Errorf("state = %s after the green EOB, want closed", receiver.State())
	}
}

func TestReceiverRefusesOversizedOffset(t *testing.T) {
	// A data segment offset is an SDNV and can name a position near 2^64.
	// Sizing a buffer from it would exhaust memory, so the receiver refuses.
	// Found by FuzzReceiverHandle.
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1, MaxBlockSize: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}

	huge := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: testSession()},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: 1 << 62, Data: []byte("x")},
	}
	if err := receiver.HandleSegment(huge); !errors.Is(err, ltp.ErrBlockTooLarge) {
		t.Errorf("error = %v, want ErrBlockTooLarge", err)
	}
	if len(receiver.Block()) != 0 {
		t.Errorf("allocated %d octets for a bogus offset", len(receiver.Block()))
	}

	// The session cancels rather than limping on.
	seg, ok, err := receiver.NextSegment()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || seg.Header.Type != ltp.TypeCancelFromReceiver {
		t.Error("expected a cancel from the receiver")
	}
}

func TestReceiverDefaultBlockSizeCap(t *testing.T) {
	receiver, err := ltp.NewReceiver(ltp.ReceiverConfig{
		SessionID: testSession(), FirstReportSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Just past the 64 MiB default.
	seg := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeRedData, SessionID: testSession()},
		Data:   &ltp.DataSegment{ClientServiceID: 1, Offset: ltp.DefaultMaxBlockSize, Data: []byte("x")},
	}
	if err := receiver.HandleSegment(seg); !errors.Is(err, ltp.ErrBlockTooLarge) {
		t.Errorf("error = %v, want ErrBlockTooLarge", err)
	}
}

func TestSenderRejectsReportClaimingCoverageBeyondTheBlock(t *testing.T) {
	// B10: a corrupt or spoofed report claiming full coverage must not make
	// the sender close the session and discard the retransmit queue it still
	// owes. Must fail without the bounds check (Step 2).
	block := testBlock(300)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           100,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, ok, _ := sender.NextSegment(); !ok {
			break
		}
	}

	// A genuine report leaves an interior gap outstanding: [100, 200).
	good := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()},
		Report: &ltp.ReportSegment{
			ReportSerial: 10, CheckpointSerial: 1, UpperBound: 300,
			Claims: []ltp.ReceptionClaim{
				{Offset: 0, Length: 100},
				{Offset: 200, Length: 100},
			},
		},
	}
	if err := sender.HandleSegment(good); err != nil {
		t.Fatalf("genuine report: %v", err)
	}
	if sender.State() != ltp.StateActive {
		t.Fatalf("state = %s after the genuine report, want active", sender.State())
	}

	// A bogus report claims coverage past the end of the block entirely.
	bogus := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()},
		Report: &ltp.ReportSegment{
			ReportSerial: 11, UpperBound: 1000,
			Claims: []ltp.ReceptionClaim{{Offset: 0, Length: 1000}},
		},
	}
	if err := sender.HandleSegment(bogus); !errors.Is(err, ltp.ErrReportOutOfRange) {
		t.Errorf("error = %v, want ErrReportOutOfRange", err)
	}
	if sender.State() == ltp.StateClosed {
		t.Error("the bogus report closed the session")
	}

	// Both reports are still acknowledged, per clause 3.2.3 - a conformant
	// peer must not be left retransmitting either forever just because the
	// sender distrusted its content.
	for _, wantSerial := range []uint64{10, 11} {
		seg, ok, err := sender.NextSegment()
		if err != nil || !ok || seg.Header.Type != ltp.TypeReportAck {
			t.Fatalf("expected a report ack for %d, got %v (ok=%t, err=%v)", wantSerial, seg, ok, err)
		}
		if seg.ReportAck.ReportSerial != wantSerial {
			t.Errorf("acknowledged serial = %d, want %d", seg.ReportAck.ReportSerial, wantSerial)
		}
	}

	// The gap the genuine report exposed still goes out: the bogus report
	// must not have discarded it.
	seg, ok, err := sender.NextSegment()
	if err != nil || !ok {
		t.Fatalf("retransmit queue was discarded (ok=%t, err=%v)", ok, err)
	}
	if seg.Data == nil || seg.Data.Offset != 100 || len(seg.Data.Data) != 100 {
		t.Fatalf("retransmitted segment = %+v, want offset 100 length 100", seg.Data)
	}
}

func TestSenderRejectsOverflowingClaimWithoutPanic(t *testing.T) {
	// A claim's absolute offset is LowerBound + the claim's own offset, both
	// wire-chosen SDNVs that can each reach 2^64. A conformant peer's report
	// always passes ReportSegment.Validate first, which makes this
	// combination impossible over the wire - but HandleSegment does not
	// re-validate, so a report built by hand (as this test does) can still
	// reach the sender with numbers Validate would have refused. Rejecting it
	// cleanly, without panicking on the wraparound, is what's under test.
	block := testBlock(100)
	sender, err := ltp.NewSender(block, ltp.SenderConfig{
		SessionID:             testSession(),
		SegmentSize:           100,
		RedPartLength:         uint64(len(block)),
		FirstCheckpointSerial: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, ok, _ := sender.NextSegment(); !ok {
			break
		}
	}

	report := &ltp.Segment{
		Header: &ltp.Header{Type: ltp.TypeReport, SessionID: testSession()},
		Report: &ltp.ReportSegment{
			ReportSerial: 7,
			UpperBound:   100, // within the red part on its own
			LowerBound:   math.MaxUint64 - 2,
			Claims:       []ltp.ReceptionClaim{{Offset: 5, Length: 1}},
		},
	}

	err = sender.HandleSegment(report) // must not panic
	if !errors.Is(err, ltp.ErrReportOutOfRange) {
		t.Errorf("error = %v, want ErrReportOutOfRange", err)
	}
	if sender.State() == ltp.StateClosed {
		t.Error("an overflowing claim closed the session")
	}
}
