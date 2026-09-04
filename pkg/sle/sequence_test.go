package sle_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ravisuhag/astro/internal/vectors"
	"github.com/ravisuhag/astro/pkg/sle"
)

// The sequence vectors in vectors/sle/association.json drive a user and a
// provider through the association procedures of clause 3.2.
//
// Operation octets are pinned in the other sle vector files. What these pin is
// the handshake: which side may send what, which side settles first, and when
// silence means the peer has gone. A BIND return means one thing after a BIND
// and is a protocol error before one, and that difference is the machine.
func TestAssociationSequenceVectors(t *testing.T) {
	vectors.RunFile(t, "sle/association.json", vectors.Impl{MachineFn: newAssociationPair})
}

// pair is the two ends of one association plus the clock they share.
//
// Time is a field rather than a call to time.Now, which is what makes the
// heartbeat testable: every Association method takes the current time from the
// caller, so a vector can step across a deadline exactly.
type pair struct {
	user     *sle.Association
	provider *sle.Association
	now      time.Time

	// The last PDU each side produced, waiting for the other to handle it.
	bindInvocation   *sle.BindInvocation
	bindReturn       *sle.BindReturn
	unbindInvocation *sle.UnbindInvocation
	unbindReturn     *sle.UnbindReturn
	abort            *sle.PeerAbort
}

const (
	userIdentifier     = "MISSION-USER"
	providerIdentifier = "GROUND-PROVIDER"
)

func newAssociationPair(init, config vectors.Fields) (vectors.Machine, error) {
	interval, err := config.Uint("heartbeat_interval")
	if err != nil {
		return nil, err
	}
	deadFactor, err := config.Uint("dead_factor")
	if err != nil {
		return nil, err
	}

	user, err := sle.NewAssociation(sle.AssociationConfig{
		Role:              sle.RoleUser,
		LocalIdentifier:   userIdentifier,
		PeerIdentifier:    providerIdentifier,
		HeartbeatInterval: uint16(interval),
		DeadFactor:        uint16(deadFactor),
	})
	if err != nil {
		return nil, err
	}
	provider, err := sle.NewAssociation(sle.AssociationConfig{
		Role:              sle.RoleProvider,
		LocalIdentifier:   providerIdentifier,
		PeerIdentifier:    userIdentifier,
		HeartbeatInterval: uint16(interval),
		DeadFactor:        uint16(deadFactor),
	})
	if err != nil {
		return nil, err
	}

	// A fixed start time. Nothing here reads a real clock, so the vector's
	// ticks are the only thing that moves it.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &pair{user: user, provider: provider, now: start}, nil
}

// instance is the service instance both ends name in the handshake.
func instance() sle.ServiceInstanceIdentifier {
	return sle.ServiceInstanceIdentifier{
		{Identifier: "sagr", Value: "1"},
		{Identifier: "spack", Value: "1"},
		{Identifier: "rsl-fg", Value: "1"},
		{Identifier: "raf", Value: "onlt1"},
	}
}

func (p *pair) Step(call string, fields vectors.Fields) ([]byte, vectors.Fields, error) {
	var err error

	switch call {
	case "bind":
		p.bindInvocation, err = p.user.Bind(p.now, 1, sle.AppReturnAllFrames, 5, "PORT", instance())

	case "provider_bind":
		// Clause 3.2 gives the invocation to the user side only.
		_, err = p.provider.Bind(p.now, 1, sle.AppReturnAllFrames, 5, "PORT", instance())

	case "accept_bind":
		p.bindReturn, err = p.provider.HandleBindInvocation(p.bindInvocation, p.now, 2)

	case "receive_bind_return":
		err = p.user.HandleBindReturn(p.bindReturn, p.now)

	case "unbind":
		p.unbindInvocation, err = p.user.Unbind(p.now, 3, sle.UnbindEnd)

	case "accept_unbind":
		p.unbindReturn, err = p.provider.HandleUnbindInvocation(p.unbindInvocation, p.now, 4)

	case "receive_unbind_return":
		err = p.user.HandleUnbindReturn(p.unbindReturn, p.now)

	case "provider_abort":
		p.abort = p.provider.Abort(sle.AbortOperationalRequirement, p.now)

	case "receive_abort":
		p.user.HandlePeerAbort(p.abort, p.now)

	case "record_received":
		// A PDU arrived, which is proof the peer is alive.
		p.user.RecordReceived(p.now)

	case "tick":
		seconds, terr := fields.Uint("seconds")
		if terr != nil {
			return nil, nil, terr
		}
		p.now = p.now.Add(time.Duration(seconds) * time.Second)

	default:
		return nil, nil, fmt.Errorf("unknown SLE call %q", call)
	}

	if err != nil {
		return nil, nil, err
	}
	return nil, p.state(), nil
}

func (p *pair) state() vectors.Fields {
	return vectors.Fields{
		"user_state":     p.user.State().String(),
		"provider_state": p.provider.State().String(),
		"user_bound":     p.user.Bound(),
		"provider_bound": p.provider.Bound(),
		"heartbeat_due":  p.user.HeartbeatDue(p.now),
		"peer_dead":      p.user.PeerDead(p.now),
	}
}
