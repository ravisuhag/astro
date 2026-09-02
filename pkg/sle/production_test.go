package sle_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

// epoch is a fixed instant, so nothing here depends on the wall clock.
var epoch = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func frame(data string) *sle.RAFTransferDataInvocation {
	return &sle.RAFTransferDataInvocation{Data: []byte(data)}
}

func newProduction(t *testing.T, size int, latency time.Duration) *sle.Production {
	t.Helper()

	production, err := sle.NewProduction(sle.ProductionConfig{
		BufferSize:   size,
		LatencyLimit: latency,
	})
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}
	return production
}

// running moves a fresh Production to running, which is where frames can be
// inserted.
func running(t *testing.T, size int, latency time.Duration) *sle.Production {
	t.Helper()

	production := newProduction(t, size, latency)
	if _, ok := production.SetRunning(); !ok {
		t.Fatal("SetRunning was refused from the initial halted state")
	}
	return production
}

// §B2.3: production starts halted, because it is not yet configured for the
// service instance. Starting it running would tell a user the channel was
// live before anyone had pointed the antenna.
func TestProductionStartsHalted(t *testing.T) {
	production := newProduction(t, 4, time.Second)

	if got := production.Status(); got != sle.ProductionHalted {
		t.Errorf("initial status = %s, want halted", got)
	}
}

// Table B-1's rows, each with the notification it carries.
func TestProductionStatusTransitions(t *testing.T) {
	production := newProduction(t, 4, time.Second)

	// Halted -> Running: management configured production.
	notification, ok := production.SetRunning()
	if !ok {
		t.Fatal("halted to running was refused")
	}
	if notification.Kind != sle.NotifyProductionStatusChange {
		t.Errorf("notification kind = %s, want production status change", notification.Kind)
	}
	if notification.ProductionStatus != sle.ProductionRunning {
		t.Errorf("notification reports %s, want running", notification.ProductionStatus)
	}

	// Running -> Interrupted: the provider detected a production fault.
	if notification, ok = production.SetInterrupted(); !ok {
		t.Fatal("running to interrupted was refused")
	}
	if notification.ProductionStatus != sle.ProductionInterrupted {
		t.Errorf("notification reports %s, want interrupted", notification.ProductionStatus)
	}

	// Interrupted -> Running: the fault was corrected.
	if _, ok = production.SetRunning(); !ok {
		t.Fatal("interrupted to running was refused")
	}

	// [Any] -> Halted: direct management action.
	if notification, ok = production.SetHalted(); !ok {
		t.Fatal("running to halted was refused")
	}
	if notification.ProductionStatus != sle.ProductionHalted {
		t.Errorf("notification reports %s, want halted", notification.ProductionStatus)
	}

	// And from interrupted too, which is the other half of "[Any]".
	if _, ok = production.SetRunning(); !ok {
		t.Fatal("halted to running was refused the second time")
	}
	if _, ok = production.SetInterrupted(); !ok {
		t.Fatal("running to interrupted was refused")
	}
	if _, ok = production.SetHalted(); !ok {
		t.Fatal("interrupted to halted was refused")
	}
}

// Halted to interrupted is not in table B-1 and has no edge in figure B-1.
// Halted means production is not configured, so there is nothing running to
// be interrupted, and reporting one would tell a user the channel had failed
// rather than that it was never started.
func TestProductionRefusesHaltedToInterrupted(t *testing.T) {
	production := newProduction(t, 4, time.Second)

	if _, ok := production.SetInterrupted(); ok {
		t.Error("halted to interrupted was allowed; table B-1 has no such row")
	}
	if got := production.Status(); got != sle.ProductionHalted {
		t.Errorf("status = %s after the refused transition, want halted", got)
	}
}

// Table B-1 lists notifications against transitions. Setting a status that is
// already current is not a transition, so it yields nothing rather than a
// notification a user would read as a change.
func TestProductionNonTransitionYieldsNoNotification(t *testing.T) {
	production := running(t, 4, time.Second)

	if notification, ok := production.SetRunning(); ok || notification != nil {
		t.Error("running to running produced a notification")
	}
}

// §3.1.9.1.7a: the buffer is released when it is full.
func TestBufferDueWhenFull(t *testing.T) {
	production := running(t, 3, 0)

	for i, data := range []string{"one", "two"} {
		due, err := production.Insert(frame(data), epoch)
		if err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
		if due {
			t.Fatalf("the buffer reported due after %d of 3 records", i+1)
		}
	}

	due, err := production.Insert(frame("three"), epoch)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !due {
		t.Error("the buffer did not report due when full")
	}

	buffer := production.Release()
	if len(buffer) != 3 {
		t.Fatalf("released %d records, want 3", len(buffer))
	}
	if production.Pending() != 0 {
		t.Error("the buffer was not cleared by Release")
	}
}

// §3.1.9.1.8: the records come out in the order they went in.
func TestBufferPreservesOrder(t *testing.T) {
	production := running(t, 4, 0)

	if _, err := production.Insert(frame("first"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := production.InsertNotification(
		&sle.SyncNotifyInvocation{Kind: sle.NotifyLossFrameSync}, epoch); err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}
	if _, err := production.Insert(frame("second"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	buffer := production.Release()
	if len(buffer) != 3 {
		t.Fatalf("released %d records, want 3", len(buffer))
	}
	if buffer[0].Frame == nil || string(buffer[0].Frame.Data) != "first" {
		t.Error("the first record is not the first frame")
	}
	if buffer[1].Notification == nil {
		t.Error("the second record is not the notification")
	}
	if buffer[2].Frame == nil || string(buffer[2].Frame.Data) != "second" {
		t.Error("the third record is not the second frame")
	}
}

// §3.1.9.1.4: the release timer starts when a record enters an *empty*
// buffer, so it measures how long the oldest record has waited. Restarting it
// on every insert would hold data indefinitely on a busy channel.
func TestReleaseTimerStartsOnEmptyBufferOnly(t *testing.T) {
	production := running(t, 10, 100*time.Millisecond)

	// The first record starts the timer.
	if _, err := production.Insert(frame("one"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	deadline, running := production.Deadline()
	if !running {
		t.Fatal("the release timer did not start on the first insert")
	}
	if want := epoch.Add(100 * time.Millisecond); !deadline.Equal(want) {
		t.Errorf("deadline = %s, want %s", deadline, want)
	}

	// A later record does not push the deadline out.
	if _, err := production.Insert(frame("two"), epoch.Add(50*time.Millisecond)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	deadline, _ = production.Deadline()
	if want := epoch.Add(100 * time.Millisecond); !deadline.Equal(want) {
		t.Errorf("the second insert moved the deadline to %s, want it left at %s", deadline, want)
	}

	// So the buffer is due at the original deadline, not 50ms later.
	if !production.Due(epoch.Add(100 * time.Millisecond)) {
		t.Error("the buffer was not due at the original deadline")
	}
}

// §3.1.9.1.7b: the timer expiring releases the buffer even when it is far
// from full.
func TestBufferDueWhenTimerExpires(t *testing.T) {
	production := running(t, 100, time.Second)

	if _, err := production.Insert(frame("lonely"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if production.Due(epoch.Add(999 * time.Millisecond)) {
		t.Error("the buffer was due before the latency limit elapsed")
	}
	if !production.Due(epoch.Add(time.Second)) {
		t.Error("the buffer was not due when the latency limit elapsed")
	}
}

// An empty buffer has no timer, so it is never due and Release yields nothing.
func TestEmptyBufferIsNeverDue(t *testing.T) {
	production := running(t, 4, time.Second)

	if production.Due(epoch.Add(time.Hour)) {
		t.Error("an empty buffer reported due")
	}
	if _, running := production.Deadline(); running {
		t.Error("an empty buffer has a running release timer")
	}
	if buffer := production.Release(); buffer != nil {
		t.Errorf("Release on an empty buffer gave %d records", len(buffer))
	}
}

// A configuration with no latency limit runs no timer, which is the offline
// and complete-online case: §3.1.9.1 requires the timer only for timely
// online delivery, though the buffer is used in every mode.
func TestNoLatencyLimitMeansNoTimer(t *testing.T) {
	production := running(t, 4, 0)

	if _, err := production.Insert(frame("one"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, running := production.Deadline(); running {
		t.Error("a zero latency limit started a release timer")
	}
	if production.Due(epoch.Add(time.Hour)) {
		t.Error("a buffer with no timer became due through time alone")
	}
	if production.Expired(epoch.Add(time.Hour)) {
		t.Error("a buffer with no timer reported an expired timer")
	}
}

// §3.1.9.1.7c: an 'end of data' record releases the buffer at once, whether
// or not it is full.
func TestEndOfDataReleasesImmediately(t *testing.T) {
	production := running(t, 100, time.Hour)

	if _, err := production.Insert(frame("last"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	due, err := production.InsertNotification(
		&sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}, epoch)
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}
	if !due {
		t.Error("an end-of-data notification did not make the buffer due")
	}

	// Another notification kind does not, so this is the end-of-data rule
	// rather than notifications generally.
	production = running(t, 100, time.Hour)
	due, err = production.InsertNotification(
		&sle.SyncNotifyInvocation{Kind: sle.NotifyLossFrameSync}, epoch)
	if err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}
	if due {
		t.Error("a loss-of-frame-sync notification made the buffer due")
	}
}

// §3.1.9.1.9: on backpressure the whole buffer goes, not the one record that
// would not fit, and a backlog notification takes its place.
func TestBackpressureDiscardsTheWholeBuffer(t *testing.T) {
	production := running(t, 10, time.Second)

	for _, data := range []string{"one", "two", "three"} {
		if _, err := production.Insert(frame(data), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	production.Backpressure(epoch.Add(10 * time.Millisecond))

	if got := production.Pending(); got != 1 {
		t.Fatalf("%d records after backpressure, want just the notification", got)
	}

	buffer := production.Release()
	if len(buffer) != 1 {
		t.Fatalf("released %d records, want 1", len(buffer))
	}
	if buffer[0].Notification == nil {
		t.Fatal("the surviving record is not a notification")
	}
	if got := buffer[0].Notification.Kind; got != sle.NotifyExcessiveDataBacklog {
		t.Errorf("notification kind = %s, want excessive data backlog", got)
	}

	_, discarded := production.Counters()
	if discarded != 1 {
		t.Errorf("discarded count = %d, want 1", discarded)
	}
}

// §3.1.9.1.9 also restarts the release timer, measured from the moment of the
// backpressure rather than from the discarded buffer's first record.
func TestBackpressureRestartsTheReleaseTimer(t *testing.T) {
	production := running(t, 10, 100*time.Millisecond)

	if _, err := production.Insert(frame("one"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	at := epoch.Add(80 * time.Millisecond)
	production.Backpressure(at)

	deadline, running := production.Deadline()
	if !running {
		t.Fatal("the release timer is not running after backpressure")
	}
	if want := at.Add(100 * time.Millisecond); !deadline.Equal(want) {
		t.Errorf("deadline = %s, want %s — the timer restarts from the backpressure", deadline, want)
	}
}

// §3.1.9.1.10: while the backlog notification waits, the buffer's size is one
// larger. The NOTE says why: without it, a channel configured with a buffer
// size of one would carry nothing but backlog notifications.
func TestBackpressureIncrementsBufferSize(t *testing.T) {
	production := running(t, 1, 0)

	// A size-one buffer is due after one frame.
	due, err := production.Insert(frame("one"), epoch)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !due {
		t.Fatal("a size-one buffer was not due after one record")
	}
	if got := production.Capacity(); got != 1 {
		t.Errorf("capacity = %d, want 1 before any backpressure", got)
	}

	// Backpressure discards it and inserts the notification. The increment
	// means the notification alone does not fill the buffer, so a frame can
	// still follow it.
	production.Backpressure(epoch)

	if got := production.Capacity(); got != 2 {
		t.Errorf("capacity = %d after backpressure, want 2 — §3.1.9.1.10 increments it", got)
	}

	due, err = production.Insert(frame("two"), epoch)
	if err != nil {
		t.Fatalf("Insert after backpressure: %v", err)
	}
	if !due {
		t.Error("the incremented buffer was not due with a notification and a frame")
	}

	buffer := production.Release()
	if len(buffer) != 2 {
		t.Fatalf("released %d records, want the notification and one frame", len(buffer))
	}
	if buffer[0].Notification == nil || buffer[1].Frame == nil {
		t.Error("the released buffer is not the notification followed by the frame")
	}

	// §3.1.9.1.10: the increment lasts until the contents are passed on.
	if got := production.Capacity(); got != 1 {
		t.Errorf("capacity = %d after release, want the configured 1 back", got)
	}
}

// A second backpressure before the next release does not stack the
// increment: §3.1.9.1.10 makes it one, singular.
func TestBackpressureIncrementDoesNotStack(t *testing.T) {
	production := running(t, 2, 0)

	production.Backpressure(epoch)
	production.Backpressure(epoch)

	if got := production.Capacity(); got != 3 {
		t.Errorf("capacity = %d after two backpressure events, want 3", got)
	}
}

// §3.1.9.1.11: an accepted STOP builds and passes the buffer at once, so a
// user that stops a service gets what production had already recovered.
func TestStopReleasesImmediately(t *testing.T) {
	production := running(t, 100, time.Hour)

	for _, data := range []string{"one", "two"} {
		if _, err := production.Insert(frame(data), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	buffer := production.Stop()
	if len(buffer) != 2 {
		t.Fatalf("Stop released %d records, want 2 without waiting for the timer", len(buffer))
	}
	if production.Pending() != 0 {
		t.Error("Stop left records in the buffer")
	}
}

// §3.1.9.1.12: an aborted association has nowhere to deliver to, so the
// buffer is cleared rather than held for a connection that is not coming
// back.
func TestAbortClearsWithoutDelivering(t *testing.T) {
	production := running(t, 100, time.Hour)

	for _, data := range []string{"one", "two"} {
		if _, err := production.Insert(frame(data), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	production.Abort()

	if production.Pending() != 0 {
		t.Error("Abort left records in the buffer")
	}
	if _, running := production.Deadline(); running {
		t.Error("Abort left the release timer running")
	}

	released, _ := production.Counters()
	if released != 0 {
		t.Errorf("released count = %d, want 0 — an abort delivers nothing", released)
	}
}

// A provider that is halted or interrupted has no data to deliver. Buffering
// a frame anyway would deliver it late and out of sequence once production
// resumed.
func TestFramesRefusedUnlessRunning(t *testing.T) {
	production := newProduction(t, 4, 0)

	if _, err := production.Insert(frame("one"), epoch); !errors.Is(err, sle.ErrProductionNotRunning) {
		t.Errorf("halted: err = %v, want ErrProductionNotRunning", err)
	}

	if _, ok := production.SetRunning(); !ok {
		t.Fatal("SetRunning was refused")
	}
	if _, ok := production.SetInterrupted(); !ok {
		t.Fatal("SetInterrupted was refused")
	}
	if _, err := production.Insert(frame("one"), epoch); !errors.Is(err, sle.ErrProductionNotRunning) {
		t.Errorf("interrupted: err = %v, want ErrProductionNotRunning", err)
	}
}

// Notifications are accepted whatever production is doing, because they are
// how a user learns production stopped. §3.1.9.1.3 needs a status-change
// notification to sit in sequence between the frames before the event and
// those after, which only works if it goes through the same buffer.
func TestNotificationsAcceptedInEveryState(t *testing.T) {
	production := newProduction(t, 10, 0)

	// Halted, before anything has run.
	if _, err := production.InsertNotification(
		&sle.SyncNotifyInvocation{Kind: sle.NotifyProductionStatusChange}, epoch); err != nil {
		t.Errorf("halted: InsertNotification: %v", err)
	}

	notification, ok := production.SetRunning()
	if !ok {
		t.Fatal("SetRunning was refused")
	}
	// The status-change notification goes through the buffer, in sequence.
	if _, err := production.InsertNotification(notification, epoch); err != nil {
		t.Errorf("running: InsertNotification: %v", err)
	}

	if production.Pending() != 2 {
		t.Errorf("%d records buffered, want 2", production.Pending())
	}
}

// A status change lands between the frames before it and those after, which
// is what §3.1.9.1.3 requires and the reason it is not sent out of band.
func TestStatusChangeSitsInSequence(t *testing.T) {
	production := running(t, 100, 0)

	if _, err := production.Insert(frame("before"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	notification, ok := production.SetInterrupted()
	if !ok {
		t.Fatal("SetInterrupted was refused")
	}
	if _, err := production.InsertNotification(notification, epoch); err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}

	// Production has to come back before a frame can follow.
	if _, ok := production.SetRunning(); !ok {
		t.Fatal("SetRunning was refused")
	}
	if _, err := production.Insert(frame("after"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	buffer := production.Release()
	if len(buffer) != 3 {
		t.Fatalf("released %d records, want 3", len(buffer))
	}
	if buffer[0].Frame == nil || string(buffer[0].Frame.Data) != "before" {
		t.Error("the frame acquired before the event is not first")
	}
	if buffer[1].Notification == nil ||
		buffer[1].Notification.ProductionStatus != sle.ProductionInterrupted {
		t.Error("the status change is not between the two frames")
	}
	if buffer[2].Frame == nil || string(buffer[2].Frame.Data) != "after" {
		t.Error("the frame acquired after the event is not last")
	}
}

// A released buffer encodes, which is what makes it deliverable: the whole
// point of the mechanism is one PDU carrying many records.
func TestReleasedBufferEncodes(t *testing.T) {
	production := running(t, 4, 0)

	for _, data := range []string{"one", "two"} {
		if _, err := production.Insert(frame(data), epoch); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	if _, err := production.InsertNotification(
		&sle.SyncNotifyInvocation{Kind: sle.NotifyEndOfData}, epoch); err != nil {
		t.Fatalf("InsertNotification: %v", err)
	}

	buffer := production.Release()
	encoded, err := buffer.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	back, err := sle.DecodeRAFTransferBuffer(encoded)
	if err != nil {
		t.Fatalf("DecodeRAFTransferBuffer: %v", err)
	}
	if len(back) != 3 {
		t.Fatalf("decoded %d records, want 3", len(back))
	}
	if back[0].Frame == nil || string(back[0].Frame.Data) != "one" {
		t.Error("the first frame did not survive the round trip")
	}
	if back[2].Notification == nil || back[2].Notification.Kind != sle.NotifyEndOfData {
		t.Error("the end-of-data notification did not survive the round trip")
	}
}

func TestProductionConfigValidation(t *testing.T) {
	for name, config := range map[string]sle.ProductionConfig{
		"zero buffer size":     {BufferSize: 0},
		"negative buffer size": {BufferSize: -1},
		"negative latency":     {BufferSize: 4, LatencyLimit: -time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			if err := config.Validate(); !errors.Is(err, sle.ErrInvalidProductionConfig) {
				t.Errorf("Validate() = %v, want ErrInvalidProductionConfig", err)
			}
			if _, err := sle.NewProduction(config); !errors.Is(err, sle.ErrInvalidProductionConfig) {
				t.Errorf("NewProduction() = %v, want ErrInvalidProductionConfig", err)
			}
		})
	}
}

func TestProductionRejectsNilRecords(t *testing.T) {
	production := running(t, 4, 0)

	if _, err := production.Insert(nil, epoch); err == nil {
		t.Error("a nil frame was accepted")
	}
	if _, err := production.InsertNotification(nil, epoch); err == nil {
		t.Error("a nil notification was accepted")
	}
}

func TestProductionHumanize(t *testing.T) {
	production := running(t, 4, time.Second)

	if _, err := production.Insert(frame("one"), epoch); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if got := production.Humanize(); got == "" {
		t.Error("Humanize returned nothing")
	}
}
