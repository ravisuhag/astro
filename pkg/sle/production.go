package sle

import (
	"fmt"
	"sync"
	"time"
)

// Production: the transfer buffer and the production status.
//
// A return-service provider does two things beyond answering operations, and
// service.go holds neither. It runs production — the antenna, the receiver,
// the frame synchroniser — and reports what that is doing. And it batches what
// production yields into a transfer buffer rather than sending one PDU per
// frame, which is what lets RAF carry a line-rate downlink over a TCP
// connection.
//
// Both are specified precisely, and both are specified in ways that are easy
// to get subtly wrong:
//
//   - The release timer starts when a record goes into an *empty* buffer
//     (§3.1.9.1.4), not on every insert. Restarting it per record would hold
//     data indefinitely on a busy channel.
//   - On backpressure the whole buffer is discarded, not the one record that
//     would not fit (§3.1.9.1.9).
//   - When the resulting 'data discarded due to excessive backlog'
//     notification goes in, the buffer's size is incremented by one until the
//     next release (§3.1.9.1.10). Without that, a channel configured with a
//     buffer size of one would send nothing but backlog notifications.
//   - Production starts 'halted', not 'running' (§B2.3), because production
//     is not yet configured for the service instance.
//   - halted goes to running and running goes to interrupted, but halted does
//     not go straight to interrupted (table B-1).
//
// Clauses cited are CCSDS 911.1-B-5. RCF, ROCF and FCLTU carry the same
// mechanism in their own sections; the buffer here is RAF-shaped because that
// is the type this package's transfer buffer uses.
//
// What is still not here is a service agreement: the provision periods and
// permitted parameter ranges service management hands down. Enforcing one
// means modelling the agreement, and that is configuration a mission supplies
// rather than protocol.

// ProductionConfig is what service management sets for one service instance
// (§3.1.9.1.5 and §3.1.9.1.6).
type ProductionConfig struct {
	// BufferSize is transfer-buffer-size: how many transfer-data and
	// sync-notify records the buffer holds before it must be released
	// (§3.1.9.1.6). It must be at least one.
	BufferSize int

	// LatencyLimit is how long the release timer runs from the moment a
	// record enters an empty buffer (§3.1.9.1.5). Zero means no timer, which
	// is the offline and complete-online case: §3.1.9.1 requires the timer
	// only for timely online delivery, though the buffer itself is used in
	// every mode.
	LatencyLimit time.Duration
}

// Validate checks the configuration.
func (c ProductionConfig) Validate() error {
	if c.BufferSize < 1 {
		return fmt.Errorf("%w: transfer buffer size must be at least one, got %d",
			ErrInvalidProductionConfig, c.BufferSize)
	}
	if c.LatencyLimit < 0 {
		return fmt.Errorf("%w: latency limit cannot be negative", ErrInvalidProductionConfig)
	}
	return nil
}

// Production runs the transfer buffer and the production status for one
// service instance.
//
// It owns no clock. Every method that could involve time takes the time, and
// the release timer is checked by the caller through Due and Expired, which
// is the same bargain the rest of this library makes: a library that sleeps
// is a library you cannot test.
//
// It is safe for concurrent use, because production and the association that
// drains it are naturally different goroutines.
type Production struct {
	config ProductionConfig

	mu sync.Mutex

	status ProductionStatus

	// buffer holds the records waiting to be released, in insertion order
	// (§3.1.9.1.8).
	buffer RAFTransferBuffer

	// timerStarted is when the release timer began, and timerRunning whether
	// it is running at all. The timer starts on insertion into an empty
	// buffer (§3.1.9.1.4).
	timerStarted time.Time
	timerRunning bool

	// extraCapacity is the temporary increment of §3.1.9.1.10: one while a
	// backlog notification is in the buffer, zero otherwise.
	extraCapacity int

	// released counts the buffers handed out, and discarded the ones thrown
	// away for backpressure, so a caller can see the channel misbehaving.
	released  int
	discarded int
}

// NewProduction prepares production for one service instance.
//
// The initial status is halted, per §B2.3: production is not yet configured
// for the instance. A caller that has configured it calls SetRunning.
func NewProduction(config ProductionConfig) (*Production, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Production{
		config: config,
		status: ProductionHalted,
	}, nil
}

// Status reports the current production status.
func (p *Production) Status() ProductionStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// Pending reports how many records are waiting in the transfer buffer.
func (p *Production) Pending() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.buffer)
}

// Capacity reports the buffer's current size, including the temporary
// increment of §3.1.9.1.10 when one is in effect.
func (p *Production) Capacity() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config.BufferSize + p.extraCapacity
}

// Counters reports how many buffers have been released and how many
// discarded for backpressure.
func (p *Production) Counters() (released, discarded int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.released, p.discarded
}

// --- production status, table B-1 ---

// SetRunning moves production to running.
//
// Table B-1 allows it from halted (management has configured production) and
// from interrupted (the provider has detected the fault is corrected). It
// returns the notification to deliver, which the caller inserts into the
// buffer; §3.1.9.1.3 puts a status-change notification in sequence with the
// frames, not out of band.
//
// Calling it when production is already running changes nothing and yields no
// notification: table B-1 lists notifications against transitions, and a
// non-transition is not one.
func (p *Production) SetRunning() (*SyncNotifyInvocation, bool) {
	return p.transition(ProductionRunning)
}

// SetInterrupted moves production to interrupted, which table B-1 allows only
// from running: it is the provider detecting a production fault.
//
// From halted it is refused. Halted means production is not configured, so
// there is nothing running to be interrupted, and reporting one would tell a
// user the channel had failed rather than that it was never started.
func (p *Production) SetInterrupted() (*SyncNotifyInvocation, bool) {
	return p.transition(ProductionInterrupted)
}

// SetHalted moves production to halted, which table B-1 allows from any
// state: it is direct management action.
func (p *Production) SetHalted() (*SyncNotifyInvocation, bool) {
	return p.transition(ProductionHalted)
}

// transition applies one status change if table B-1 permits it.
func (p *Production) transition(to ProductionStatus) (*SyncNotifyInvocation, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status == to {
		return nil, false
	}
	if !permittedTransition(p.status, to) {
		return nil, false
	}

	p.status = to
	return &SyncNotifyInvocation{
		Kind:             NotifyProductionStatusChange,
		ProductionStatus: to,
	}, true
}

// permittedTransition reports whether table B-1 defines a transition.
//
// The table's rows are: halted to running, running to interrupted,
// interrupted to running, and anything to halted. Halted to interrupted is
// absent, and figure B-1 shows no edge for it either.
func permittedTransition(from, to ProductionStatus) bool {
	switch to {
	case ProductionHalted:
		// "[Any] -> Halted", by direct management action.
		return true
	case ProductionRunning:
		return from == ProductionHalted || from == ProductionInterrupted
	case ProductionInterrupted:
		return from == ProductionRunning
	default:
		return false
	}
}

// --- the transfer buffer, §3.1.9.1 ---

// Insert puts one annotated frame into the transfer buffer.
//
// It reports whether the buffer is now due for release, which happens when
// the buffer is full (§3.1.9.1.7a). A caller inserting frames checks the
// return and calls Release when it is true; it also has to check Expired
// between inserts, because the release timer can fire while nothing is
// arriving.
//
// Frames are refused unless production is running: a provider that is halted
// or interrupted has no data to deliver, and buffering one anyway would
// deliver it late and out of sequence when production resumed.
func (p *Production) Insert(frame *RAFTransferDataInvocation, now time.Time) (due bool, err error) {
	if frame == nil {
		return false, fmt.Errorf("%w: no frame to insert", ErrInvalidProductionConfig)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.status != ProductionRunning {
		return false, fmt.Errorf("%w: production is %s", ErrProductionNotRunning, p.status)
	}
	return p.insertLocked(TransferBufferEntry{Frame: frame}, now), nil
}

// InsertNotification puts one synchronous notification into the transfer
// buffer.
//
// Notifications are accepted whatever production is doing, because the
// notifications are how a user learns production stopped. §3.1.9.1.3 requires
// a status-change notification to sit in sequence between the frames acquired
// before the event and those after it, which is only possible if it goes
// through the same buffer.
//
// An 'end of data' notification releases the buffer at once (§3.1.9.1.7c),
// which is why this reports due as well.
func (p *Production) InsertNotification(notification *SyncNotifyInvocation, now time.Time) (due bool, err error) {
	if notification == nil {
		return false, fmt.Errorf("%w: no notification to insert", ErrInvalidProductionConfig)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	due = p.insertLocked(TransferBufferEntry{Notification: notification}, now)

	// §3.1.9.1.7c: an 'end of data' record releases the buffer immediately,
	// whether or not it is full.
	if notification.Kind == NotifyEndOfData {
		due = true
	}
	return due, nil
}

// insertLocked appends a record and starts the release timer if the buffer
// was empty. The lock must already be held.
func (p *Production) insertLocked(entry TransferBufferEntry, now time.Time) bool {
	// §3.1.9.1.4: the timer starts when a record enters an empty buffer, so
	// it measures how long the oldest record has waited rather than the
	// newest.
	if len(p.buffer) == 0 && p.config.LatencyLimit > 0 {
		p.timerStarted = now
		p.timerRunning = true
	}

	p.buffer = append(p.buffer, entry)

	return len(p.buffer) >= p.config.BufferSize+p.extraCapacity
}

// Due reports whether the buffer should be released now: it is full, or the
// release timer has expired.
func (p *Production) Due(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dueLocked(now)
}

func (p *Production) dueLocked(now time.Time) bool {
	if len(p.buffer) == 0 {
		return false
	}
	if len(p.buffer) >= p.config.BufferSize+p.extraCapacity {
		return true
	}
	return p.expiredLocked(now)
}

// Expired reports whether the release timer has run out (§3.1.9.1.5).
//
// It is false when no timer is running, which is the case for an empty buffer
// and for a configuration with no latency limit.
func (p *Production) Expired(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.expiredLocked(now)
}

func (p *Production) expiredLocked(now time.Time) bool {
	if !p.timerRunning || p.config.LatencyLimit <= 0 {
		return false
	}
	return !now.Before(p.timerStarted.Add(p.config.LatencyLimit))
}

// Deadline reports when the release timer expires, and whether one is
// running.
//
// A caller driving this from a select loop uses it to size its wait rather
// than polling Due.
func (p *Production) Deadline() (time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.timerRunning || p.config.LatencyLimit <= 0 {
		return time.Time{}, false
	}
	return p.timerStarted.Add(p.config.LatencyLimit), true
}

// Release takes the buffer's contents to hand to the communications service.
//
// It returns the records in insertion order (§3.1.9.1.8) and clears the
// buffer, stopping the release timer. An empty buffer yields nil, which is
// not an error: a caller may release on a schedule and find nothing waiting.
//
// The temporary size increment of §3.1.9.1.10 is given back here, since the
// clause ties it to the buffer being passed to the communications service.
func (p *Production) Release() RAFTransferBuffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.releaseLocked()
}

func (p *Production) releaseLocked() RAFTransferBuffer {
	if len(p.buffer) == 0 {
		return nil
	}

	buffer := p.buffer
	p.buffer = nil
	p.timerRunning = false

	// §3.1.9.1.10: the increment lasts until the contents are passed on.
	p.extraCapacity = 0
	p.released++

	return buffer
}

// Backpressure handles the communications service refusing a released buffer
// (§3.1.9.1.9).
//
// The whole buffer is discarded rather than the one record that would not
// fit, a 'data discarded due to excessive backlog' notification is inserted,
// and the release timer is restarted. Per §3.1.9.1.10 the buffer's size is
// incremented by one while that notification waits, so a channel configured
// with a buffer size of one still carries some telemetry rather than nothing
// but notifications.
//
// It returns whether the buffer is due again, which it is when the size is
// one: the notification alone then fills it.
func (p *Production) Backpressure(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Discard everything, including anything inserted since the release.
	p.buffer = nil
	p.timerRunning = false
	p.discarded++

	// The increment goes on before the insert, so the notification does not
	// immediately fill a size-one buffer and push out the frames that follow
	// it. It is one whatever happens: a second backpressure event before the
	// next release does not stack.
	p.extraCapacity = 1

	p.insertLocked(TransferBufferEntry{
		Notification: &SyncNotifyInvocation{Kind: NotifyExcessiveDataBacklog},
	}, now)

	return len(p.buffer) >= p.config.BufferSize+p.extraCapacity
}

// Stop builds the buffer for immediate delivery on an accepted STOP
// invocation (§3.1.9.1.11).
//
// The clause requires the provider to build and pass the buffer at once
// rather than waiting for the timer, so a user that stops a service gets what
// production had already recovered.
func (p *Production) Stop() RAFTransferBuffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.releaseLocked()
}

// Abort clears the transfer buffer without delivering it (§3.1.9.1.12).
//
// An aborted association has nowhere to deliver to, so the contents go rather
// than waiting for a connection that is not coming back. The counters are
// kept: they describe the instance, not the association.
func (p *Production) Abort() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer = nil
	p.timerRunning = false
	p.extraCapacity = 0
}

// Humanize returns a human-readable summary.
func (p *Production) Humanize() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	timer := "not running"
	if p.timerRunning {
		timer = fmt.Sprintf("running, limit %s", p.config.LatencyLimit)
	}

	return fmt.Sprintf("SLE Production\n"+
		"  Status ......... %s\n"+
		"  Buffer ......... %d of %d record(s)\n"+
		"  Release timer .. %s\n"+
		"  Released ....... %d buffer(s)\n"+
		"  Discarded ...... %d buffer(s)",
		p.status,
		len(p.buffer), p.config.BufferSize+p.extraCapacity,
		timer, p.released, p.discarded)
}
