package ltp_test

import (
	"bytes"
	"errors"
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
	// §3.2.4: green data below a red offset is MISCOLORED.
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
	// §3.2.5: a cancel is acknowledged.
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
	// §3.2.1 and §3.2.2 both forbid zero, and this package refuses rather
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
