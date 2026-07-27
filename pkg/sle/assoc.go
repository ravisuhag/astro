package sle

import (
	"errors"
	"sync"
	"time"
)

// The association state machine.
//
// An SLE association runs over one TCP connection (CCSDS 913.1-B-2 §3.3.1) and
// moves through a small sequence of states: a context message opens it, a BIND
// exchange establishes it, PDUs flow, and an UNBIND or a PEER-ABORT ends it.
//
// This machine owns no goroutines and no timers, the same contract as
// pkg/cop's FOP-1. ISP1 has a heartbeat interval and a dead factor; rather
// than run a clock, this reports when a heartbeat is due and when the peer has
// gone silent, given a time the caller supplies. Your scheduler acts on it.

// AssociationState is where an association has got to.
type AssociationState int

const (
	// StateUnbound means the connection is open but no BIND has succeeded.
	StateUnbound AssociationState = iota
	// StateBindPending means a BIND invocation is out and its return has not
	// come back.
	StateBindPending
	// StateBound means the association is established and PDUs may flow.
	StateBound
	// StateUnbindPending means an UNBIND invocation is out.
	StateUnbindPending
	// StateClosed means the association has ended, by UNBIND or abort.
	StateClosed
)

// String names the state.
func (s AssociationState) String() string {
	switch s {
	case StateUnbound:
		return "unbound"
	case StateBindPending:
		return "bind pending"
	case StateBound:
		return "bound"
	case StateUnbindPending:
		return "unbind pending"
	default:
		return "closed"
	}
}

// Role says which end of the association this is.
type Role uint8

const (
	// RoleUser is the service user, the side that sends BIND. Usually a
	// mission control centre.
	RoleUser Role = iota
	// RoleProvider is the service provider, the side that answers. Usually a
	// ground station.
	RoleProvider
)

// String names the role.
func (r Role) String() string {
	if r == RoleProvider {
		return "provider"
	}
	return "user"
}

// AssociationConfig describes one association.
type AssociationConfig struct {
	// Role is which end this is.
	Role Role

	// LocalIdentifier is this end's authority identifier, 3 to 16 characters.
	LocalIdentifier string
	// PeerIdentifier is the far end's, checked on receipt.
	PeerIdentifier string

	// HeartbeatInterval is how many seconds between heartbeats. Zero disables
	// the heartbeat (§3.3.3).
	HeartbeatInterval uint16
	// DeadFactor is how many intervals of silence mean the peer has gone.
	DeadFactor uint16

	// UserName and Password authenticate this end. Leave Password nil for an
	// unauthenticated association.
	UserName string
	Password []byte

	// PeerPassword verifies the far end's credentials.
	PeerPassword []byte

	// AcceptableDelay is how far a peer's credential time may be from now
	// before it is rejected (§3.1.2.2.1). Zero disables the check.
	AcceptableDelay time.Duration
}

// Association tracks one SLE association.
//
// It is safe for concurrent use, though the caller normally drives it from a
// single loop.
type Association struct {
	mu     sync.Mutex
	config AssociationConfig

	state AssociationState

	// contextSent and contextReceived track the TML handshake.
	contextSent     bool
	contextReceived bool
	// negotiated holds the parameters from the peer's context message, which
	// govern the heartbeat once received.
	negotiated ContextMessage

	// lastReceived is when anything last arrived, used for liveness.
	lastReceived time.Time
	// lastSent is when anything last went out, used to decide when a
	// heartbeat is due.
	lastSent time.Time

	// serviceType and version are agreed by the BIND exchange.
	serviceType ApplicationIdentifier
	version     uint16

	// abortDiagnostic records why the association ended, if it aborted.
	abortDiagnostic *PeerAbortDiagnostic
}

// NewAssociation prepares an association.
func NewAssociation(config AssociationConfig) (*Association, error) {
	if err := validateIdentifier(config.LocalIdentifier); err != nil {
		return nil, err
	}
	return &Association{config: config, state: StateUnbound}, nil
}

// State returns the current state.
func (a *Association) State() AssociationState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// Bound reports whether the association is established.
func (a *Association) Bound() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state == StateBound
}

// ContextMessage builds the TML context message that opens the connection.
//
// §3.3.2.2 makes this the first message on a connection, before any SLE PDU.
func (a *Association) ContextMessage(now time.Time) *Message {
	a.mu.Lock()
	defer a.mu.Unlock()

	c := &ContextMessage{
		HeartbeatInterval: a.config.HeartbeatInterval,
		DeadFactor:        a.config.DeadFactor,
	}
	a.contextSent = true
	a.lastSent = now
	return c.Message()
}

// HandleContextMessage records the peer's context message.
//
// The peer's heartbeat interval and dead factor govern what this end must
// send and how long it waits, so they replace the configured values.
func (a *Association) HandleContextMessage(body []byte, now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	c, err := DecodeContextMessage(body)
	if err != nil {
		return err
	}
	a.negotiated = *c
	a.contextReceived = true
	a.lastReceived = now
	return nil
}

// Bind builds a BIND invocation and moves to bind-pending.
//
// Only the service user sends BIND. randomNumber seeds the credentials; the
// caller supplies it rather than this package choosing a randomness source.
func (a *Association) Bind(
	now time.Time,
	randomNumber int32,
	serviceType ApplicationIdentifier,
	version uint16,
	responderPort string,
	instance ServiceInstanceIdentifier,
) (*BindInvocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config.Role != RoleUser {
		return nil, ErrWrongState
	}
	switch a.state {
	case StateBound:
		return nil, ErrAlreadyBound
	case StateUnbound:
	default:
		return nil, ErrWrongState
	}

	invocation := &BindInvocation{
		InitiatorIdentifier:       a.config.LocalIdentifier,
		ResponderPortIdentifier:   responderPort,
		ServiceType:               serviceType,
		VersionNumber:             version,
		ServiceInstanceIdentifier: instance,
	}

	if len(a.config.Password) > 0 {
		creds, err := GenerateCredentials(now, randomNumber, a.config.UserName, a.config.Password)
		if err != nil {
			return nil, err
		}
		invocation.Credentials = creds
	}

	if err := invocation.Validate(); err != nil {
		return nil, err
	}

	a.state = StateBindPending
	a.serviceType = serviceType
	a.version = version
	a.lastSent = now
	return invocation, nil
}

// HandleBindInvocation processes an inbound BIND at the provider, returning
// the answer to send.
func (a *Association) HandleBindInvocation(b *BindInvocation, now time.Time, randomNumber int32) (*BindReturn, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config.Role != RoleProvider {
		return nil, ErrWrongState
	}
	a.lastReceived = now

	reply := &BindReturn{ResponderIdentifier: a.config.LocalIdentifier}
	addCredentials := func() error {
		if len(a.config.Password) == 0 {
			return nil
		}
		creds, err := GenerateCredentials(now, randomNumber, a.config.UserName, a.config.Password)
		if err != nil {
			return err
		}
		reply.Credentials = creds
		return nil
	}

	// An association already bound refuses a second BIND.
	if a.state == StateBound {
		reply.Diagnostic = BindAlreadyBound
		if err := addCredentials(); err != nil {
			return nil, err
		}
		return reply, nil
	}

	// The initiator must be who we expect.
	if a.config.PeerIdentifier != "" && b.InitiatorIdentifier != a.config.PeerIdentifier {
		reply.Diagnostic = BindNotAccessibleToThisInitiator
		if err := addCredentials(); err != nil {
			return nil, err
		}
		return reply, nil
	}

	// Authenticate when this association is configured for it.
	if len(a.config.PeerPassword) > 0 {
		if b.Credentials == nil {
			reply.Diagnostic = BindAccessDenied
			if err := addCredentials(); err != nil {
				return nil, err
			}
			return reply, nil
		}
		verifyErr := b.Credentials.Verify(now, a.config.AcceptableDelay, b.InitiatorIdentifier, a.config.PeerPassword)
		if verifyErr != nil {
			// A stale clock and a bad digest are different failures, and the
			// standard has a diagnostic for each.
			reply.Diagnostic = BindAccessDenied
			if errors.Is(verifyErr, ErrCredentialsExpired) {
				reply.Diagnostic = BindInvalidTime
			}
			if err := addCredentials(); err != nil {
				return nil, err
			}
			return reply, nil
		}
	}

	reply.Positive = true
	reply.VersionNumber = b.VersionNumber
	if err := addCredentials(); err != nil {
		return nil, err
	}

	a.state = StateBound
	a.serviceType = b.ServiceType
	a.version = b.VersionNumber
	a.lastSent = now
	return reply, nil
}

// HandleBindReturn processes the provider's answer at the user.
func (a *Association) HandleBindReturn(b *BindReturn, now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateBindPending {
		return ErrWrongState
	}
	a.lastReceived = now

	// The responder must be who we expect.
	if a.config.PeerIdentifier != "" && b.ResponderIdentifier != a.config.PeerIdentifier {
		a.state = StateClosed
		d := AbortUnexpectedResponderID
		a.abortDiagnostic = &d
		return ErrBindRejected
	}

	if len(a.config.PeerPassword) > 0 {
		if b.Credentials == nil {
			a.state = StateClosed
			return ErrAuthenticationFailed
		}
		if err := b.Credentials.Verify(now, a.config.AcceptableDelay, b.ResponderIdentifier, a.config.PeerPassword); err != nil {
			a.state = StateClosed
			return err
		}
	}

	if !b.Positive {
		a.state = StateUnbound
		return ErrBindRejected
	}

	a.state = StateBound
	a.version = b.VersionNumber
	return nil
}

// Unbind builds an UNBIND invocation and moves to unbind-pending.
func (a *Association) Unbind(now time.Time, randomNumber int32, reason UnbindReason) (*UnbindInvocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateBound {
		return nil, ErrNotBound
	}

	invocation := &UnbindInvocation{Reason: reason}
	if len(a.config.Password) > 0 {
		creds, err := GenerateCredentials(now, randomNumber, a.config.UserName, a.config.Password)
		if err != nil {
			return nil, err
		}
		invocation.Credentials = creds
	}

	a.state = StateUnbindPending
	a.lastSent = now
	return invocation, nil
}

// HandleUnbindInvocation processes an inbound UNBIND, returning the answer.
//
// An UNBIND cannot be refused: the return type of annex A2.2 has only a
// positive alternative.
func (a *Association) HandleUnbindInvocation(u *UnbindInvocation, now time.Time, randomNumber int32) (*UnbindReturn, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastReceived = now

	reply := &UnbindReturn{}
	if len(a.config.Password) > 0 {
		creds, err := GenerateCredentials(now, randomNumber, a.config.UserName, a.config.Password)
		if err != nil {
			return nil, err
		}
		reply.Credentials = creds
	}

	a.state = StateClosed
	a.lastSent = now
	return reply, nil
}

// HandleUnbindReturn processes the answer to our UNBIND.
func (a *Association) HandleUnbindReturn(u *UnbindReturn, now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateUnbindPending {
		return ErrWrongState
	}
	a.lastReceived = now
	a.state = StateClosed
	return nil
}

// Abort ends the association abruptly, returning the PEER-ABORT to send.
func (a *Association) Abort(diagnostic PeerAbortDiagnostic, now time.Time) *PeerAbort {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = StateClosed
	a.abortDiagnostic = &diagnostic
	a.lastSent = now
	return &PeerAbort{Diagnostic: diagnostic}
}

// HandlePeerAbort processes an inbound PEER-ABORT.
func (a *Association) HandlePeerAbort(p *PeerAbort, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastReceived = now
	a.state = StateClosed
	d := p.Diagnostic
	a.abortDiagnostic = &d
}

// AbortDiagnostic returns why the association aborted, or nil if it did not.
func (a *Association) AbortDiagnostic() *PeerAbortDiagnostic {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.abortDiagnostic
}

// RecordSent notes that something went out, which resets the heartbeat timer.
//
// §3.3.3: a heartbeat is only needed on an idle connection, so any traffic
// serves the same purpose.
func (a *Association) RecordSent(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastSent = now
}

// RecordReceived notes that something arrived, which proves the peer is alive.
func (a *Association) RecordReceived(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastReceived = now
}

// heartbeatInterval returns the interval in force: the peer's if its context
// message has arrived, otherwise our own.
func (a *Association) heartbeatInterval() time.Duration {
	interval := a.config.HeartbeatInterval
	if a.contextReceived {
		interval = a.negotiated.HeartbeatInterval
	}
	return time.Duration(interval) * time.Second
}

// HeartbeatDue reports whether a heartbeat should go out now.
//
// This package runs no clock. Call it from your loop with the current time.
func (a *Association) HeartbeatDue(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	interval := a.heartbeatInterval()
	if interval == 0 || a.state == StateClosed {
		return false
	}
	if a.lastSent.IsZero() {
		return true
	}
	return now.Sub(a.lastSent) >= interval
}

// PeerDead reports whether the peer has been silent for longer than the
// heartbeat interval times the dead factor.
//
// §3.3.3 makes this the signal to abort the association.
func (a *Association) PeerDead(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	interval := a.heartbeatInterval()
	factor := a.config.DeadFactor
	if a.contextReceived {
		factor = a.negotiated.DeadFactor
	}
	if interval == 0 || factor == 0 || a.state == StateClosed {
		return false
	}
	if a.lastReceived.IsZero() {
		return false
	}
	return now.Sub(a.lastReceived) > interval*time.Duration(factor)
}

// NextHeartbeat returns when the next heartbeat is due, so a caller can size
// a select timeout rather than poll.
func (a *Association) NextHeartbeat() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()

	interval := a.heartbeatInterval()
	if interval == 0 {
		return time.Time{}
	}
	return a.lastSent.Add(interval)
}

// ServiceType returns the service the BIND agreed on.
func (a *Association) ServiceType() ApplicationIdentifier {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.serviceType
}

// Version returns the version the BIND agreed on.
func (a *Association) Version() uint16 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.version
}

// MakeCredentials builds credentials for an outgoing service PDU, or returns
// nil when the association is unauthenticated.
//
// The BIND family generates its own credentials inside this file. The service
// operations in raf.go, rcf.go, rocf.go and fcltu.go are built by the state
// machines in service.go, which call this so every PDU on an authenticated
// association carries the same identity the BIND did.
func (a *Association) MakeCredentials(now time.Time, randomNumber int32) (*Credentials, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.config.Password) == 0 {
		return nil, nil
	}
	return GenerateCredentials(now, randomNumber, a.config.UserName, a.config.Password)
}

// CheckPeerCredentials verifies credentials on an incoming service PDU.
//
// It returns nil when the association is not checking the peer, which is the
// unauthenticated case. When it is, missing credentials are as bad as wrong
// ones: an association configured for authentication must not accept a PDU
// that simply left the field out.
func (a *Association) CheckPeerCredentials(c *Credentials, now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.config.PeerPassword) == 0 {
		return nil
	}
	if c == nil {
		return ErrInvalidCredentials
	}
	return c.Verify(now, a.config.AcceptableDelay, a.config.PeerIdentifier, a.config.PeerPassword)
}

// Role reports which end of the association this is.
func (a *Association) Role() Role {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.Role
}
