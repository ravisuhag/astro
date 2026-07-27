package sle_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

var testEpoch = time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

func TestTimeRoundTrip(t *testing.T) {
	// The RAF module's TimeCCSDS: 2 octets days, 4 octets milliseconds,
	// 2 octets microseconds, with the P-field implicit.
	stamp := time.Date(2026, 7, 12, 10, 30, 15, 250_000_000, time.UTC)

	sleTime, err := sle.NewTime(stamp)
	if err != nil {
		t.Fatal(err)
	}
	encoded := sleTime.Encode()
	if len(encoded) != sle.TimeCCSDSSize {
		t.Fatalf("encoded %d octets, want 8", len(encoded))
	}

	got, err := sle.DecodeTime(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != sleTime {
		t.Errorf("round trip gave %+v, want %+v", got, sleTime)
	}
	// Microsecond resolution is all the format carries.
	if diff := got.Time().Sub(stamp); diff > time.Microsecond || diff < -time.Microsecond {
		t.Errorf("recovered %s, want %s", got.Time(), stamp)
	}
}

func TestTimeEpochIs1958(t *testing.T) {
	// Day zero is 1958-01-01.
	zero, err := sle.NewTime(sle.CCSDSEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if zero.Days != 0 || zero.Milliseconds != 0 {
		t.Errorf("the epoch encoded as %+v, want all zeros", zero)
	}

	oneDay, err := sle.NewTime(sle.CCSDSEpoch.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if oneDay.Days != 1 {
		t.Errorf("one day after the epoch = day %d, want 1", oneDay.Days)
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	creds, err := sle.GenerateCredentials(testEpoch, 12345, "MISSION-USER", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	// §3.1.2.3 requires SHA-256, which is 32 octets.
	if len(creds.Protected) != sle.DigestSizeSHA256 {
		t.Fatalf("digest is %d octets, want %d for SHA-256",
			len(creds.Protected), sle.DigestSizeSHA256)
	}

	encoded, err := creds.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sle.DecodeCredentials(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.RandomNumber != 12345 {
		t.Errorf("random number = %d, want 12345", got.RandomNumber)
	}
	if !bytes.Equal(got.Protected, creds.Protected) {
		t.Error("the digest did not survive the round trip")
	}
}

func TestCredentialsVerify(t *testing.T) {
	const user = "MISSION-USER"
	password := []byte("correct horse battery staple")

	creds, err := sle.GenerateCredentials(testEpoch, 999, user, password)
	if err != nil {
		t.Fatal(err)
	}

	if err := creds.Verify(testEpoch, time.Minute, user, password); err != nil {
		t.Errorf("genuine credentials failed to verify: %v", err)
	}
	if err := creds.Verify(testEpoch, time.Minute, user, []byte("wrong")); !errors.Is(err, sle.ErrAuthenticationFailed) {
		t.Errorf("wrong password: error = %v, want ErrAuthenticationFailed", err)
	}
	if err := creds.Verify(testEpoch, time.Minute, "OTHER-USER", password); !errors.Is(err, sle.ErrAuthenticationFailed) {
		t.Errorf("wrong user: error = %v, want ErrAuthenticationFailed", err)
	}
}

func TestCredentialsRejectStaleTime(t *testing.T) {
	// §3.1.2.2.1: credentials too far from now fail authentication. This is
	// what stops a captured BIND being replayed later.
	const user = "MISSION-USER"
	password := []byte("secret")

	creds, err := sle.GenerateCredentials(testEpoch, 1, user, password)
	if err != nil {
		t.Fatal(err)
	}

	later := testEpoch.Add(10 * time.Minute)
	if err := creds.Verify(later, time.Minute, user, password); !errors.Is(err, sle.ErrCredentialsExpired) {
		t.Errorf("stale credentials: error = %v, want ErrCredentialsExpired", err)
	}

	// A zero delay disables the check.
	if err := creds.Verify(later, 0, user, password); err != nil {
		t.Errorf("with the check disabled: %v", err)
	}
}

func TestCredentialsChoice(t *testing.T) {
	// Credentials ::= CHOICE { unused [0] NULL, used [1] OCTET STRING }
	unused, err := sle.AppendCredentialsChoice(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, err := sle.NewDecoder(unused).Next()
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsContext(0) {
		t.Errorf("unused credentials took tag [%d], want [0]", e.Tag)
	}
	got, err := sle.DecodeCredentialsChoice(e)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("the unused alternative should decode to nil")
	}

	creds, err := sle.GenerateCredentials(testEpoch, 7, "USER", []byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	used, err := sle.AppendCredentialsChoice(nil, creds)
	if err != nil {
		t.Fatal(err)
	}
	e, err = sle.NewDecoder(used).Next()
	if err != nil {
		t.Fatal(err)
	}
	if !e.IsContext(1) {
		t.Errorf("used credentials took tag [%d], want [1]", e.Tag)
	}
}

func TestBindInvocationRoundTrip(t *testing.T) {
	b := &sle.BindInvocation{
		InitiatorIdentifier:     "CTRL-CENTRE",
		ResponderPortIdentifier: "GS-PORT-1",
		ServiceType:             sle.AppReturnAllFrames,
		VersionNumber:           5,
		ServiceInstanceIdentifier: sle.ServiceInstanceIdentifier{
			{Identifier: "sagr", Value: "MISSION"},
			{Identifier: "spack", Value: "PASS1"},
			{Identifier: "rsl-fg", Value: "1"},
			{Identifier: "raf", Value: "onlc1"},
		},
	}

	encoded, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sle.DecodeBindInvocation(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.InitiatorIdentifier != b.InitiatorIdentifier {
		t.Errorf("initiator = %q", got.InitiatorIdentifier)
	}
	if got.ServiceType != sle.AppReturnAllFrames {
		t.Errorf("service type = %s", got.ServiceType)
	}
	if got.VersionNumber != 5 {
		t.Errorf("version = %d, want 5", got.VersionNumber)
	}
	if len(got.ServiceInstanceIdentifier) != 4 {
		t.Fatalf("got %d service instance attributes, want 4", len(got.ServiceInstanceIdentifier))
	}
	if got.ServiceInstanceIdentifier.String() != b.ServiceInstanceIdentifier.String() {
		t.Errorf("service instance = %s, want %s",
			got.ServiceInstanceIdentifier, b.ServiceInstanceIdentifier)
	}
}

func TestAuthorityIdentifierBounds(t *testing.T) {
	// AuthorityIdentifier ::= IdentifierString (SIZE (3 .. 16)), and
	// IdentifierString excludes the space.
	tests := []struct {
		id    string
		valid bool
	}{
		{"ABC", true},
		{"SIXTEEN-CHARS-16", true},
		{"AB", false},
		{"SEVENTEEN-CHARS-1", false},
		{"HAS SPACE", false},
	}
	for _, tt := range tests {
		b := &sle.BindInvocation{
			InitiatorIdentifier:     tt.id,
			ResponderPortIdentifier: "PORT",
			VersionNumber:           1,
		}
		err := b.Validate()
		if tt.valid && err != nil {
			t.Errorf("%q should be valid: %v", tt.id, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("%q should be rejected", tt.id)
		}
	}
}

func TestBindReturnBothOutcomes(t *testing.T) {
	// result CHOICE { positive [0] VersionNumber, negative [1] BindDiagnostic }
	positive := &sle.BindReturn{ResponderIdentifier: "GROUND-STN", Positive: true, VersionNumber: 5}
	encoded, err := positive.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sle.DecodeBindReturn(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Positive || got.VersionNumber != 5 {
		t.Errorf("positive return decoded as %+v", got)
	}

	negative := &sle.BindReturn{ResponderIdentifier: "GROUND-STN", Diagnostic: sle.BindAccessDenied}
	encoded, err = negative.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err = sle.DecodeBindReturn(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Positive {
		t.Error("negative return decoded as positive")
	}
	if got.Diagnostic != sle.BindAccessDenied {
		t.Errorf("diagnostic = %s, want access denied", got.Diagnostic)
	}
}

func TestUnbindAndPeerAbortRoundTrip(t *testing.T) {
	u := &sle.UnbindInvocation{Reason: sle.UnbindSuspend}
	encoded, err := u.Encode()
	if err != nil {
		t.Fatal(err)
	}
	gotU, err := sle.DecodeUnbindInvocation(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if gotU.Reason != sle.UnbindSuspend {
		t.Errorf("reason = %s, want suspend", gotU.Reason)
	}

	ur := &sle.UnbindReturn{}
	encoded, err = ur.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sle.DecodeUnbindReturn(encoded); err != nil {
		t.Fatal(err)
	}

	p := &sle.PeerAbort{Diagnostic: sle.AbortProtocolError}
	gotP, err := sle.DecodePeerAbort(p.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if gotP.Diagnostic != sle.AbortProtocolError {
		t.Errorf("diagnostic = %s, want protocol error", gotP.Diagnostic)
	}
}

func TestPeerAbortTechnologySpecificRange(t *testing.T) {
	// Annex A2.2: 128 to 255 is reserved for the communications technology.
	p := sle.PeerAbortDiagnostic(200)
	if got := p.String(); got == "" {
		t.Error("a technology-specific diagnostic has no description")
	}
}

// association pairs a user and a provider for handshake tests.
func association(t *testing.T, authenticated bool) (user, provider *sle.Association) {
	t.Helper()

	userConfig := sle.AssociationConfig{
		Role:              sle.RoleUser,
		LocalIdentifier:   "CTRL-CENTRE",
		PeerIdentifier:    "GROUND-STN",
		HeartbeatInterval: 30,
		DeadFactor:        3,
	}
	providerConfig := sle.AssociationConfig{
		Role:              sle.RoleProvider,
		LocalIdentifier:   "GROUND-STN",
		PeerIdentifier:    "CTRL-CENTRE",
		HeartbeatInterval: 30,
		DeadFactor:        3,
	}

	if authenticated {
		userConfig.UserName = "CTRL-CENTRE"
		userConfig.Password = []byte("user-secret")
		userConfig.PeerPassword = []byte("provider-secret")
		userConfig.AcceptableDelay = time.Minute

		providerConfig.UserName = "GROUND-STN"
		providerConfig.Password = []byte("provider-secret")
		providerConfig.PeerPassword = []byte("user-secret")
		providerConfig.AcceptableDelay = time.Minute
	}

	var err error
	if user, err = sle.NewAssociation(userConfig); err != nil {
		t.Fatal(err)
	}
	if provider, err = sle.NewAssociation(providerConfig); err != nil {
		t.Fatal(err)
	}
	return user, provider
}

func TestAssociationHandshake(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		name := "unauthenticated"
		if authenticated {
			name = "authenticated"
		}
		t.Run(name, func(t *testing.T) {
			user, provider := association(t, authenticated)
			now := testEpoch

			// The context message opens the connection, before any PDU.
			ctx := user.ContextMessage(now)
			if err := provider.HandleContextMessage(ctx.Body, now); err != nil {
				t.Fatal(err)
			}

			invocation, err := user.Bind(now, 111, sle.AppReturnAllFrames, 5, "GS-PORT",
				sle.ServiceInstanceIdentifier{{Identifier: "raf", Value: "onlc1"}})
			if err != nil {
				t.Fatal(err)
			}
			if user.State() != sle.StateBindPending {
				t.Errorf("user state = %s, want bind pending", user.State())
			}

			// Through the wire, so the codecs are exercised too.
			encoded, err := invocation.Encode()
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := sle.DecodeBindInvocation(encoded)
			if err != nil {
				t.Fatal(err)
			}

			reply, err := provider.HandleBindInvocation(decoded, now, 222)
			if err != nil {
				t.Fatal(err)
			}
			if !reply.Positive {
				t.Fatalf("bind refused: %s", reply.Diagnostic)
			}
			if !provider.Bound() {
				t.Error("the provider is not bound after accepting")
			}

			replyEncoded, err := reply.Encode()
			if err != nil {
				t.Fatal(err)
			}
			replyDecoded, err := sle.DecodeBindReturn(replyEncoded)
			if err != nil {
				t.Fatal(err)
			}
			if err := user.HandleBindReturn(replyDecoded, now); err != nil {
				t.Fatal(err)
			}
			if !user.Bound() {
				t.Error("the user is not bound after a positive return")
			}

			// And unbind.
			unbind, err := user.Unbind(now, 333, sle.UnbindEnd)
			if err != nil {
				t.Fatal(err)
			}
			unbindReturn, err := provider.HandleUnbindInvocation(unbind, now, 444)
			if err != nil {
				t.Fatal(err)
			}
			if err := user.HandleUnbindReturn(unbindReturn, now); err != nil {
				t.Fatal(err)
			}
			if user.State() != sle.StateClosed || provider.State() != sle.StateClosed {
				t.Errorf("states after unbind: user %s, provider %s", user.State(), provider.State())
			}
		})
	}
}

func TestBindRejectedOnBadCredentials(t *testing.T) {
	user, provider := association(t, true)
	now := testEpoch

	// The user has the wrong password for itself, so the provider's check fails.
	badUser, err := sle.NewAssociation(sle.AssociationConfig{
		Role:            sle.RoleUser,
		LocalIdentifier: "CTRL-CENTRE",
		PeerIdentifier:  "GROUND-STN",
		UserName:        "CTRL-CENTRE",
		Password:        []byte("wrong-secret"),
		PeerPassword:    []byte("provider-secret"),
		AcceptableDelay: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	invocation, err := badUser.Bind(now, 1, sle.AppReturnAllFrames, 5, "PORT", nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := provider.HandleBindInvocation(invocation, now, 2)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Positive {
		t.Fatal("the provider accepted a bind with wrong credentials")
	}
	if reply.Diagnostic != sle.BindAccessDenied {
		t.Errorf("diagnostic = %s, want access denied", reply.Diagnostic)
	}
	if provider.Bound() {
		t.Error("the provider bound despite refusing")
	}
	_ = user
}

func TestBindRejectedOnUnknownInitiator(t *testing.T) {
	_, provider := association(t, false)

	stranger, err := sle.NewAssociation(sle.AssociationConfig{
		Role:            sle.RoleUser,
		LocalIdentifier: "SOMEONE-ELSE",
	})
	if err != nil {
		t.Fatal(err)
	}

	invocation, err := stranger.Bind(testEpoch, 1, sle.AppReturnAllFrames, 5, "PORT", nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := provider.HandleBindInvocation(invocation, testEpoch, 2)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Positive {
		t.Fatal("the provider accepted a bind from an unexpected initiator")
	}
	if reply.Diagnostic != sle.BindNotAccessibleToThisInitiator {
		t.Errorf("diagnostic = %s", reply.Diagnostic)
	}
}

func TestSecondBindRefused(t *testing.T) {
	user, provider := association(t, false)
	now := testEpoch

	invocation, err := user.Bind(now, 1, sle.AppReturnAllFrames, 5, "PORT", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.HandleBindInvocation(invocation, now, 2); err != nil {
		t.Fatal(err)
	}

	reply, err := provider.HandleBindInvocation(invocation, now, 3)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Positive {
		t.Fatal("the provider accepted a second bind")
	}
	if reply.Diagnostic != sle.BindAlreadyBound {
		t.Errorf("diagnostic = %s, want already bound", reply.Diagnostic)
	}
}

func TestUnbindRequiresBound(t *testing.T) {
	user, _ := association(t, false)
	if _, err := user.Unbind(testEpoch, 1, sle.UnbindEnd); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("error = %v, want ErrNotBound", err)
	}
}

func TestPeerAbortClosesTheAssociation(t *testing.T) {
	user, _ := association(t, false)

	abort := user.Abort(sle.AbortOperationalRequirement, testEpoch)
	if abort.Diagnostic != sle.AbortOperationalRequirement {
		t.Errorf("diagnostic = %s", abort.Diagnostic)
	}
	if user.State() != sle.StateClosed {
		t.Errorf("state = %s, want closed", user.State())
	}
	if d := user.AbortDiagnostic(); d == nil || *d != sle.AbortOperationalRequirement {
		t.Error("the abort diagnostic was not recorded")
	}
}

func TestHeartbeatTiming(t *testing.T) {
	// This package runs no clock: it answers questions about a time you give it.
	user, _ := association(t, false)
	now := testEpoch

	user.ContextMessage(now) // records lastSent

	if user.HeartbeatDue(now.Add(10 * time.Second)) {
		t.Error("a heartbeat came due before the 30 second interval")
	}
	if !user.HeartbeatDue(now.Add(30 * time.Second)) {
		t.Error("no heartbeat due after the full interval")
	}

	// Any traffic postpones it.
	user.RecordSent(now.Add(25 * time.Second))
	if user.HeartbeatDue(now.Add(30 * time.Second)) {
		t.Error("traffic did not reset the heartbeat timer")
	}
}

func TestPeerDeadDetection(t *testing.T) {
	// §3.3.3: silence longer than the interval times the dead factor means
	// the peer has gone.
	user, _ := association(t, false)
	now := testEpoch

	user.RecordReceived(now)

	if user.PeerDead(now.Add(60 * time.Second)) {
		t.Error("the peer was declared dead within the window")
	}
	// 30 second interval times a dead factor of 3 is 90 seconds.
	if !user.PeerDead(now.Add(91 * time.Second)) {
		t.Error("the peer was not declared dead after the window")
	}
}

func TestHeartbeatDisabledByZeroInterval(t *testing.T) {
	// §3.3.3: a zero interval turns the heartbeat off.
	a, err := sle.NewAssociation(sle.AssociationConfig{
		Role:              sle.RoleUser,
		LocalIdentifier:   "CTRL-CENTRE",
		HeartbeatInterval: 0,
		DeadFactor:        3,
	})
	if err != nil {
		t.Fatal(err)
	}
	a.RecordSent(testEpoch)
	a.RecordReceived(testEpoch)

	far := testEpoch.Add(24 * time.Hour)
	if a.HeartbeatDue(far) {
		t.Error("a heartbeat came due with the interval disabled")
	}
	if a.PeerDead(far) {
		t.Error("the peer was declared dead with the heartbeat disabled")
	}
}

func TestPeerContextOverridesLocalTiming(t *testing.T) {
	// The peer's context message says how often it expects to hear from us.
	user, _ := association(t, false)
	now := testEpoch

	peerContext := (&sle.ContextMessage{HeartbeatInterval: 5, DeadFactor: 2}).Encode()
	if err := user.HandleContextMessage(peerContext, now); err != nil {
		t.Fatal(err)
	}
	user.RecordSent(now)

	// The configured interval was 30 seconds; the peer asked for 5.
	if !user.HeartbeatDue(now.Add(6 * time.Second)) {
		t.Error("the peer's shorter heartbeat interval was not adopted")
	}
}
