package ltp

// Reaches into the Receiver's unexported report-tracking fields, so it lives
// in the ltp package itself rather than ltp_test.

import "testing"

// TestReceiverCancelsOnTooManyOutstandingReports exercises S5: a stream of
// checkpoints carrying fresh serials must not grow reportsByCheckpoint,
// awaitingAck and pending without bound. Must fail without the
// MaxOutstandingReports cap (Step 1).
func TestReceiverCancelsOnTooManyOutstandingReports(t *testing.T) {
	const maxReports = 3
	r, err := NewReceiver(ReceiverConfig{
		SessionID:             SessionID{EngineID: 1, SessionNumber: 1},
		FirstReportSerial:     1,
		MaxOutstandingReports: maxReports,
	})
	if err != nil {
		t.Fatal(err)
	}

	var lastErr error
	sent := 0
	for i := uint64(0); i < 50 && r.state != StateCancelled; i++ {
		seg := &Segment{
			Header: &Header{Type: TypeRedDataCheckpoint, SessionID: r.config.SessionID},
			Data: &DataSegment{
				ClientServiceID:  1,
				Offset:           i * 10,
				Data:             []byte("x"),
				CheckpointSerial: i + 1, // fresh every time
			},
		}
		lastErr = r.HandleSegment(seg)
		sent++
	}

	if r.state != StateCancelled {
		t.Fatalf("state = %v after %d checkpoints, want cancelled", r.state, sent)
	}
	if lastErr == nil {
		t.Error("HandleSegment returned no error on the checkpoint that tripped the cap")
	}
	if r.cancelReason == nil || *r.cancelReason != ReasonSystemCancelled {
		t.Errorf("cancel reason = %v, want ReasonSystemCancelled", r.cancelReason)
	}
	if uint64(len(r.reportsByCheckpoint)) > maxReports {
		t.Errorf("reportsByCheckpoint has %d entries, want <= %d", len(r.reportsByCheckpoint), maxReports)
	}
	if uint64(len(r.awaitingAck)) > maxReports {
		t.Errorf("awaitingAck has %d entries, want <= %d", len(r.awaitingAck), maxReports)
	}
	// pending may hold one extra entry: the cancel segment queued alongside
	// the rejection.
	if uint64(len(r.pending)) > maxReports+1 {
		t.Errorf("pending has %d entries, want <= %d", len(r.pending), maxReports+1)
	}
}
