package sle

import (
	"fmt"
	"sync"
	"time"
)

// The service state machine shared by RAF, RCF, ROCF and FCLTU.
//
// All four specs define the same three states and the same shape of table:
// CCSDS 911.1-B-5 clause 4.2.1, 911.2-B-4 clause 4.2.1, 911.5-B-4 clause 4.2.1 and
// 912.1-B-5 clause 4.2.1 each say state 1 'unbound', state 2 'ready', state 3
// 'active', and each table hangs the same association operations off them.
// What differs is the data operation in state 3 (frames, control fields,
// CLTUs) and that lives in the per-service machines in raf.go, rcf.go,
// rocf.go and fcltu.go.
//
// This file holds the part that would otherwise be written four times. It is
// not a file the plan named; it exists because four copies of a state table
// is four places for a transcription error to hide.
//
// The machines are caller-pumped, following pkg/cop's FOP-1: you hand them
// events, you pull PDUs out, and they own no goroutines, no timers and no
// sockets. Anything the spec describes with a timer (the return <n> timer of
// table note 11, the release timer of state table row 13) is the caller's to
// run, using the deadline hints on Association.

// ServiceState is the state of one service instance.
//
// The numbers are the specs' own: state 1, 2 and 3. They start at 1 rather
// than 0 so that a state printed in a log matches the state named in the
// table you are reading it against.
type ServiceState int

const (
	// ServiceUnbound is state 1: no association, nothing allocated.
	ServiceUnbound ServiceState = 1
	// ServiceReady is state 2: bound, but not moving data.
	ServiceReady ServiceState = 2
	// ServiceActive is state 3: bound and transferring.
	ServiceActive ServiceState = 3
)

// String names the state the way the specs do.
func (s ServiceState) String() string {
	switch s {
	case ServiceUnbound:
		return "unbound"
	case ServiceReady:
		return "ready"
	case ServiceActive:
		return "active"
	default:
		return fmt.Sprintf("state(%d)", int(s))
	}
}

// ServiceConfig configures one service instance.
type ServiceConfig struct {
	// Association is the bound association this instance runs over. It must
	// already exist; this package does not create one for you, because a
	// caller normally wants to configure authentication and heartbeats first.
	Association *Association

	// Kind names the service, which decides the PDU tags.
	Kind ServiceKind

	// DeliveryMode is the mode the service agreement fixed. See delivery.go
	// for what the machine does with it and what it leaves to you.
	DeliveryMode DeliveryMode

	// Version is the service version to bind at.
	Version uint16

	// ResponderPort names the provider's port identifier, used by BIND.
	ResponderPort string

	// Instance identifies the service instance, used by BIND.
	Instance ServiceInstanceIdentifier
}

// applicationIdentifier maps a service kind onto the BIND field that names it.
func (k ServiceKind) applicationIdentifier() ApplicationIdentifier {
	switch k {
	case ServiceRAF:
		return AppReturnAllFrames
	case ServiceRCF:
		return AppReturnChannelFrames
	case ServiceROCF:
		return AppReturnChannelOCF
	default:
		return AppForwardCLTU
	}
}

// operationTag finds the wire tag a service gives an operation.
func operationTag(kind ServiceKind, op OperationType) (uint32, bool) {
	for tag, candidate := range bindFamilyTags {
		if candidate == op {
			return tag, true
		}
	}
	table := returnServiceTags
	if kind == ServiceFCLTU {
		table = fcltuTags
	}
	for tag, candidate := range table {
		if candidate == op {
			return tag, true
		}
	}
	return 0, false
}

// serviceCore is the state every service machine keeps, on either side.
type serviceCore struct {
	mu     sync.Mutex
	config ServiceConfig

	state ServiceState

	// outbound holds encoded PDUs waiting for the caller to send them. The
	// machines put at most one PDU here per event; nothing accumulates unless
	// the caller stops pulling.
	outbound [][]byte
}

// newServiceCore validates a configuration and prepares the shared state.
func newServiceCore(config ServiceConfig, role Role) (*serviceCore, error) {
	if config.Association == nil {
		return nil, ErrNotBound
	}
	if config.Association.Role() != role {
		return nil, ErrWrongState
	}
	if config.DeliveryMode != 0 && !config.DeliveryMode.Valid() {
		return nil, ErrWrongState
	}
	return &serviceCore{config: config, state: ServiceUnbound}, nil
}

// State returns the service state.
func (c *serviceCore) State() ServiceState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Kind returns which service this is.
func (c *serviceCore) Kind() ServiceKind { return c.config.Kind }

// DeliveryMode returns the configured delivery mode.
func (c *serviceCore) DeliveryMode() DeliveryMode { return c.config.DeliveryMode }

// Association returns the association underneath.
func (c *serviceCore) Association() *Association { return c.config.Association }

// NextPDU takes the next PDU the caller should send, or reports false when
// there is nothing waiting.
func (c *serviceCore) NextPDU() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.outbound) == 0 {
		return nil, false
	}
	pdu := c.outbound[0]
	c.outbound = c.outbound[1:]
	return pdu, true
}

// Pending reports how many PDUs are waiting to be sent.
func (c *serviceCore) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.outbound)
}

// queue wraps an operation's content in its tag and holds it for the caller.
// The lock must already be held.
func (c *serviceCore) queue(op OperationType, content []byte) error {
	tag, ok := operationTag(c.config.Kind, op)
	if !ok {
		return ErrInvalidTag
	}
	c.outbound = append(c.outbound, AppendPDU(nil, tag, content))
	return nil
}

// abort queues a PEER-ABORT and drops to state 1, which is what every state
// table cell marked 'peer abort protocol error' does.
// The lock must already be held.
func (c *serviceCore) abort(diagnostic PeerAbortDiagnostic, now time.Time) {
	pdu := c.config.Association.Abort(diagnostic, now)
	if pdu != nil {
		tag, ok := operationTag(c.config.Kind, OpPeerAbort)
		if ok {
			// PEER-ABORT is [104] IMPLICIT PeerAbortDiagnostic: the tag
			// replaces the INTEGER's, so the PDU is a primitive element
			// holding the bare diagnostic octets, 9F 68 01 xx on the wire.
			c.outbound = append(c.outbound,
				AppendElement(nil, ClassContext, false, tag, pdu.Encode()))
		}
	}
	c.state = ServiceUnbound
}

// authenticate verifies the credentials on an inbound service PDU against
// the association's authentication level. At any level below 'all' it is a
// no-op; at 'all' a PDU with missing or wrong credentials is refused before
// the machine acts on it.
func (c *serviceCore) authenticate(creds *Credentials, now time.Time) error {
	return c.config.Association.CheckPeerCredentials(creds, now)
}

// PeerAbort ends the association from this end.
func (c *serviceCore) PeerAbort(diagnostic PeerAbortDiagnostic, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.abort(diagnostic, now)
}

// HandlePeerAbort takes an inbound PEER-ABORT: the association is over.
func (c *serviceCore) HandlePeerAbort(p *PeerAbort, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Association.HandlePeerAbort(p, now)
	c.state = ServiceUnbound
}

// ServiceUser is the user half of a service instance: the mission-control
// side, which binds, starts, receives or sends data, stops and unbinds.
//
// It is embedded by RAFUser, RCFUser, ROCFUser and FCLTUUser, which add the
// operations that differ between services. Use one of those rather than this
// directly.
type ServiceUser struct {
	*serviceCore

	// nextInvokeId is the identifier the next confirmed operation will carry.
	nextInvokeId InvokeId
	// awaiting tracks confirmed operations whose return has not arrived.
	// The specs put a timer on each of these (table 4-1 note 11); running it
	// is the caller's job, and Outstanding is how it knows what to watch.
	awaiting map[InvokeId]OperationType
}

// NewServiceUser prepares the user half of a service instance.
func NewServiceUser(config ServiceConfig) (*ServiceUser, error) {
	core, err := newServiceCore(config, RoleUser)
	if err != nil {
		return nil, err
	}
	return &ServiceUser{serviceCore: core, awaiting: make(map[InvokeId]OperationType)}, nil
}

// Outstanding returns the invoke identifiers still awaiting a return, so a
// caller running the spec's return timers knows which to watch.
func (u *ServiceUser) Outstanding() map[InvokeId]OperationType {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := make(map[InvokeId]OperationType, len(u.awaiting))
	for id, op := range u.awaiting {
		out[id] = op
	}
	return out
}

// invoke runs one confirmed user operation: check the state, allocate an
// invoke identifier, build credentials, let the caller encode the content,
// and queue the PDU. Doing it in one locked step keeps the identifier, the
// state and the queue from disagreeing.
func (u *ServiceUser) invoke(
	op OperationType,
	allowed ServiceState,
	now time.Time,
	randomNumber int32,
	build func(id InvokeId, creds *Credentials) ([]byte, error),
) (InvokeId, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.state != allowed {
		return 0, serviceStateError(u.state, allowed)
	}

	creds, err := u.config.Association.MakeCredentials(now, randomNumber)
	if err != nil {
		return 0, err
	}

	id := u.nextInvokeId
	if _, busy := u.awaiting[id]; busy {
		// nextInvokeId has wrapped (InvokeId is 16 bits) all the way back to
		// an identifier this user is still waiting on a return for: more
		// than 65536 confirmed operations are outstanding at once. Refuse
		// locally, with a diagnosable cause, rather than sending a PDU the
		// provider will bounce as 'duplicate invoke ID' for a reason the
		// caller cannot see.
		return 0, ErrInvokeIdExhausted
	}

	content, err := build(id, creds)
	if err != nil {
		return 0, err
	}
	if err := u.queue(op, content); err != nil {
		return 0, err
	}

	u.nextInvokeId++
	u.awaiting[id] = op
	u.config.Association.RecordSent(now)
	return id, nil
}

// serviceStateError names why an operation was refused.
func serviceStateError(current, allowed ServiceState) error {
	switch {
	case current == ServiceUnbound:
		return ErrNotBound
	case allowed == ServiceActive && current == ServiceReady:
		return ErrNotStarted
	case allowed == ServiceReady && current == ServiceActive:
		return ErrAlreadyStarted
	default:
		return ErrWrongState
	}
}

// settle clears an outstanding invocation, checking it is the one expected.
// The lock must already be held.
func (u *ServiceUser) settle(id InvokeId, op OperationType) error {
	got, ok := u.awaiting[id]
	if !ok {
		return ErrUnknownInvokeId
	}
	if got != op {
		return ErrUnexpectedPDU
	}
	delete(u.awaiting, id)
	return nil
}

// Bind opens the association and asks for the service instance. State 1 only,
// per CCSDS 911.1-B-5 clause 3.2.1.6.
func (u *ServiceUser) Bind(now time.Time, randomNumber int32) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.state != ServiceUnbound {
		return ErrAlreadyBound
	}

	invocation, err := u.config.Association.Bind(
		now, randomNumber,
		u.config.Kind.applicationIdentifier(),
		u.config.Version,
		u.config.ResponderPort,
		u.config.Instance,
	)
	if err != nil {
		return err
	}
	content, err := invocation.Encode()
	if err != nil {
		return err
	}
	return u.queue(OpBindInvocation, content)
}

// HandleBindReturn takes the provider's answer to BIND. A positive answer
// moves to state 2.
func (u *ServiceUser) HandleBindReturn(b *BindReturn, now time.Time) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.state != ServiceUnbound {
		u.abort(AbortProtocolError, now)
		return ErrUnexpectedPDU
	}
	if err := u.config.Association.HandleBindReturn(b, now); err != nil {
		return err
	}
	u.state = ServiceReady
	return nil
}

// Unbind ends the association. State 2 only, per clause 3.3.1.5. A user must stop
// before it may unbind.
func (u *ServiceUser) Unbind(now time.Time, randomNumber int32, reason UnbindReason) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.state != ServiceReady {
		return serviceStateError(u.state, ServiceReady)
	}
	invocation, err := u.config.Association.Unbind(now, randomNumber, reason)
	if err != nil {
		return err
	}
	content, err := invocation.Encode()
	if err != nil {
		return err
	}
	return u.queue(OpUnbindInvocation, content)
}

// HandleUnbindReturn takes the provider's answer to UNBIND, ending the
// instance at state 1.
func (u *ServiceUser) HandleUnbindReturn(r *UnbindReturn, now time.Time) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.config.Association.HandleUnbindReturn(r, now); err != nil {
		return err
	}
	u.state = ServiceUnbound
	return nil
}

// Stop ends data transfer. State 3 only, per clause 3.5.1.3.
func (u *ServiceUser) Stop(now time.Time, randomNumber int32) (InvokeId, error) {
	return u.invoke(OpStopInvocation, ServiceActive, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&StopInvocation{Credentials: creds, InvokeId: id}).Encode()
		})
}

// HandleStopReturn takes the answer to STOP. A positive answer returns the
// instance to state 2; a negative one leaves it active, which is the point of
// state table row 10's ELSE branch.
func (u *ServiceUser) HandleStopReturn(a *Acknowledgement) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if err := u.settle(a.InvokeId, OpStopInvocation); err != nil {
		return err
	}
	if a.Positive {
		u.state = ServiceReady
	}
	return nil
}

// ScheduleStatusReport asks for status reports, once or periodically, or
// turns periodic reporting off. Valid in states 2 and 3.
func (u *ServiceUser) ScheduleStatusReport(
	kind ReportRequestKind, cycle uint16, now time.Time, randomNumber int32,
) (InvokeId, error) {
	u.mu.Lock()
	state := u.state
	u.mu.Unlock()

	if state == ServiceUnbound {
		return 0, ErrNotBound
	}
	if kind == ReportPeriodically && !u.config.DeliveryMode.AllowsPeriodicStatusReport() {
		// The provider would answer 'not supported in this delivery mode';
		// saying so here saves a round trip. See delivery.go.
		return 0, ErrWrongState
	}
	return u.invoke(OpScheduleStatusReportInvocation, state, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&ScheduleStatusReportInvocation{
				Credentials:    creds,
				InvokeId:       id,
				Kind:           kind,
				ReportingCycle: cycle,
			}).Encode()
		})
}

// HandleScheduleStatusReportReturn takes the answer to a report request.
func (u *ServiceUser) HandleScheduleStatusReportReturn(r *ScheduleStatusReportReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.settle(r.InvokeId, OpScheduleStatusReportInvocation)
}

// GetParameter asks the provider for one configuration parameter, named by
// the service's ParameterName value. Valid in states 2 and 3, per clause 3.10 of
// each service specification.
func (u *ServiceUser) GetParameter(parameter int, now time.Time, randomNumber int32) (InvokeId, error) {
	u.mu.Lock()
	state := u.state
	u.mu.Unlock()

	if state == ServiceUnbound {
		return 0, ErrNotBound
	}
	return u.invoke(OpGetParameterInvocation, state, now, randomNumber,
		func(id InvokeId, creds *Credentials) ([]byte, error) {
			return (&GetParameterInvocation{
				Credentials: creds,
				InvokeId:    id,
				Parameter:   parameter,
			}).Encode()
		})
}

// HandleGetParameterReturn takes the answer to GET-PARAMETER.
func (u *ServiceUser) HandleGetParameterReturn(r *GetParameterReturn) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.settle(r.InvokeId, OpGetParameterInvocation)
}

// startAccepted moves to state 3 after a positive START return.
// The lock must already be held.
func (u *ServiceUser) startAccepted() { u.state = ServiceActive }

// ServiceProvider is the provider half: the ground-station side.
//
// It answers the operations a user drives (BIND, START, STOP, UNBIND) and
// lets the caller push data while active. One of these is one service
// instance on one association.
//
// The rest of a real provider lives beside it rather than in it. Production
// and the transfer buffer are in production.go; serving several instances and
// routing an inbound BIND between them is Complex, in complex.go. What none
// of them holds is a service agreement: the provision periods, permitted
// parameter ranges and scheduling that service management hands down are
// configuration a mission supplies. See docs/content/conformance/sle.md for
// the row-by-row picture.
type ServiceProvider struct {
	*serviceCore

	// outstandingInvokeIds holds invoke identifiers that have been accepted
	// (registerInvokeId returned true) but whose return the provider has not
	// yet queued. A second invocation under one of these is a genuine
	// duplicate: the first is still being worked.
	outstandingInvokeIds map[InvokeId]bool

	// answeredInvokeIds and answeredInvokeIdOrder together are a bounded,
	// most-recently-answered window: settleInvokeId moves an identifier
	// here when its return is queued, so a retransmission of an invocation
	// already answered still draws the 'duplicate invoke ID' diagnostic
	// instead of running twice.
	//
	// The window is capped (answeredInvokeIdWindow) rather than kept
	// forever, because an InvokeId is only 16 bits (CCSDS 911.1-B-5 clause
	// 3.1.4 and its siblings) and a confirmed operation such as FCLTU
	// TRANSFER-DATA burns one per CLTU: an ordinary long pass wraps the
	// identifier space many times over within one association. A set that
	// remembered every identifier ever answered would eventually treat every
	// legitimate new invocation as a collision with its own distant past.
	answeredInvokeIds     map[InvokeId]struct{}
	answeredInvokeIdOrder []InvokeId
}

// answeredInvokeIdWindow bounds how many recently-answered invoke
// identifiers the provider remembers, so it stays small (a few hundred
// entries at most) no matter how long the association runs or how many
// times the 16-bit identifier space has wrapped.
const answeredInvokeIdWindow = 256

// NewServiceProvider prepares the provider half of a service instance.
func NewServiceProvider(config ServiceConfig) (*ServiceProvider, error) {
	core, err := newServiceCore(config, RoleProvider)
	if err != nil {
		return nil, err
	}
	return &ServiceProvider{serviceCore: core}, nil
}

// resetInvokeIdTracking drops all outstanding and recently-answered invoke
// identifiers. A fresh association starts a fresh invoke identifier space.
// The lock must already be held.
func (p *ServiceProvider) resetInvokeIdTracking() {
	p.outstandingInvokeIds = nil
	p.answeredInvokeIds = nil
	p.answeredInvokeIdOrder = nil
}

// registerInvokeId records an inbound invocation's identifier, reporting
// false when it is still outstanding or was recently answered on this
// association — either means the invocation must not run again.
func (p *ServiceProvider) registerInvokeId(id InvokeId) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.outstandingInvokeIds == nil {
		p.outstandingInvokeIds = make(map[InvokeId]bool)
	}
	if p.outstandingInvokeIds[id] {
		return false
	}
	if _, answered := p.answeredInvokeIds[id]; answered {
		return false
	}
	p.outstandingInvokeIds[id] = true
	return true
}

// settleInvokeId retires an invoke identifier once the provider has queued
// its return: it stops being outstanding and enters the bounded
// recently-answered window. The lock must already be held.
func (p *ServiceProvider) settleInvokeId(id InvokeId) {
	delete(p.outstandingInvokeIds, id)

	if p.answeredInvokeIds == nil {
		p.answeredInvokeIds = make(map[InvokeId]struct{})
	}
	if _, ok := p.answeredInvokeIds[id]; ok {
		return
	}
	if len(p.answeredInvokeIdOrder) >= answeredInvokeIdWindow {
		oldest := p.answeredInvokeIdOrder[0]
		p.answeredInvokeIdOrder = p.answeredInvokeIdOrder[1:]
		delete(p.answeredInvokeIds, oldest)
	}
	p.answeredInvokeIds[id] = struct{}{}
	p.answeredInvokeIdOrder = append(p.answeredInvokeIdOrder, id)
}

// queueDuplicateAnswer queues an already-encoded negative return carrying
// the 'duplicate invoke ID' diagnostic and reports the error the caller
// should see. Credentials are omitted: the machine has no random number to
// build them with here, and a duplicate is a peer defect being refused.
func (p *ServiceProvider) queueDuplicateAnswer(op OperationType, content []byte, err error, now time.Time) error {
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.queue(op, content); err != nil {
		return err
	}
	p.config.Association.RecordSent(now)
	return ErrDuplicateInvokeId
}

// respond builds and queues one provider PDU, holding the lock across the
// whole step, and retires id from the outstanding set into the
// recently-answered window: this is the single settle point every confirmed
// operation's return passes through, so it is where an invoke identifier
// becomes free to recycle.
func (p *ServiceProvider) respond(
	op OperationType,
	id InvokeId,
	allowed ServiceState,
	now time.Time,
	randomNumber int32,
	build func(creds *Credentials) ([]byte, error),
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != allowed {
		return serviceStateError(p.state, allowed)
	}
	creds, err := p.config.Association.MakeCredentials(now, randomNumber)
	if err != nil {
		return err
	}
	content, err := build(creds)
	if err != nil {
		return err
	}
	if err := p.queue(op, content); err != nil {
		return err
	}
	p.config.Association.RecordSent(now)
	p.settleInvokeId(id)
	return nil
}

// HandleBindInvocation answers a BIND. State table row 5: accept and go to
// state 2, or refuse and stay at state 1.
func (p *ServiceProvider) HandleBindInvocation(b *BindInvocation, now time.Time, randomNumber int32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != ServiceUnbound {
		p.abort(AbortProtocolError, now)
		return ErrUnexpectedPDU
	}

	answer, err := p.config.Association.HandleBindInvocation(b, now, randomNumber)
	if err != nil && answer == nil {
		return err
	}
	content, encodeErr := answer.Encode()
	if encodeErr != nil {
		return encodeErr
	}
	if queueErr := p.queue(OpBindReturn, content); queueErr != nil {
		return queueErr
	}
	if answer.Positive {
		p.state = ServiceReady
		p.resetInvokeIdTracking()
	}
	return err
}

// HandleUnbindInvocation answers an UNBIND, returning to state 1.
func (p *ServiceProvider) HandleUnbindInvocation(u *UnbindInvocation, now time.Time, randomNumber int32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == ServiceUnbound {
		return ErrUnexpectedPDU
	}
	answer, err := p.config.Association.HandleUnbindInvocation(u, now, randomNumber)
	if err != nil {
		return err
	}
	content, err := answer.Encode()
	if err != nil {
		return err
	}
	if err := p.queue(OpUnbindReturn, content); err != nil {
		return err
	}
	p.state = ServiceUnbound
	p.resetInvokeIdTracking()
	return nil
}

// HandleStopInvocation answers a STOP, returning to state 2 when it accepts.
func (p *ServiceProvider) HandleStopInvocation(
	s *StopInvocation, accept bool, diagnostic Diagnostics, now time.Time, randomNumber int32,
) error {
	err := p.respond(OpStopReturn, s.InvokeId, ServiceActive, now, randomNumber,
		func(creds *Credentials) ([]byte, error) {
			return (&Acknowledgement{
				Credentials: creds,
				InvokeId:    s.InvokeId,
				Positive:    accept,
				Diagnostic:  diagnostic,
			}).Encode()
		})
	if err != nil {
		return err
	}
	if accept {
		p.mu.Lock()
		p.state = ServiceReady
		p.mu.Unlock()
	}
	return nil
}

// HandleScheduleStatusReportInvocation answers a report request.
func (p *ServiceProvider) HandleScheduleStatusReportInvocation(
	s *ScheduleStatusReportInvocation, accept bool,
	diagnostic ScheduleStatusReportDiagnostic, now time.Time, randomNumber int32,
) error {
	p.mu.Lock()
	state := p.state
	p.mu.Unlock()
	if state == ServiceUnbound {
		return ErrNotBound
	}
	return p.respond(OpScheduleStatusReportReturn, s.InvokeId, state, now, randomNumber,
		func(creds *Credentials) ([]byte, error) {
			return (&ScheduleStatusReportReturn{
				Credentials:        creds,
				InvokeId:           s.InvokeId,
				Positive:           accept,
				SpecificDiagnostic: diagnostic,
			}).Encode()
		})
}

// HandleGetParameterInvocation answers a GET-PARAMETER. Valid in states 2
// and 3, per clause 3.10.
//
// parameter is the still-encoded alternative of the service's parameter
// CHOICE (one complete BER element) or nil, which answers negatively with
// 'unknown parameter'. This package does not model the per-service parameter
// CHOICEs; the caller that has a value to report encodes it.
func (p *ServiceProvider) HandleGetParameterInvocation(
	g *GetParameterInvocation, parameter []byte, now time.Time, randomNumber int32,
) error {
	p.mu.Lock()
	state := p.state
	p.mu.Unlock()
	if state == ServiceUnbound {
		return ErrNotBound
	}
	return p.respond(OpGetParameterReturn, g.InvokeId, state, now, randomNumber,
		func(creds *Credentials) ([]byte, error) {
			answer := &GetParameterReturn{
				Credentials:        creds,
				InvokeId:           g.InvokeId,
				SpecificDiagnostic: GetParameterUnknown,
			}
			if parameter != nil {
				answer.Positive = true
				answer.Parameter = parameter
			}
			return answer.Encode()
		})
}

// SendStatusReport queues an unconfirmed STATUS-REPORT. Valid in states 2
// and 3: the channel has something to report as soon as the instance is
// bound.
func (p *ServiceProvider) SendStatusReport(
	report interface{ Encode() ([]byte, error) }, now time.Time,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == ServiceUnbound {
		return ErrNotBound
	}
	content, err := report.Encode()
	if err != nil {
		return err
	}
	if err := p.queue(OpStatusReportInvocation, content); err != nil {
		return err
	}
	p.config.Association.RecordSent(now)
	return nil
}

// sendWhileActive queues one unconfirmed PDU that is only legal in state 3.
func (p *ServiceProvider) sendWhileActive(op OperationType, content []byte, now time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != ServiceActive {
		return serviceStateError(p.state, ServiceActive)
	}
	if err := p.queue(op, content); err != nil {
		return err
	}
	p.config.Association.RecordSent(now)
	return nil
}
