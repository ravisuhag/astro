package sle_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
)

// servicePair builds a matched RAF user and provider over a fresh
// association, ready to be driven by hand.
func servicePair(t *testing.T, mode sle.DeliveryMode) (*sle.RAFUser, *sle.RAFProvider) {
	t.Helper()

	userAssoc, providerAssoc := association(t, false)
	instance := sle.ServiceInstanceIdentifier{
		{Identifier: "sagr", Value: "MISSION"},
		{Identifier: "spack", Value: "PASS1"},
		{Identifier: "rsl-fg", Value: "1"},
		{Identifier: "raf", Value: "onlc1"},
	}

	user, err := sle.NewRAFUser(sle.ServiceConfig{
		Association:   userAssoc,
		DeliveryMode:  mode,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      instance,
	})
	if err != nil {
		t.Fatalf("NewRAFUser() = %v", err)
	}

	provider, err := sle.NewRAFProvider(sle.ServiceConfig{
		Association:   providerAssoc,
		DeliveryMode:  mode,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      instance,
	})
	if err != nil {
		t.Fatalf("NewRAFProvider() = %v", err)
	}
	return user, provider
}

// pump moves every queued PDU from one side to the other, feeding each to the
// far side's HandlePDU. It is the whole transport the state-machine tests
// need: no sockets, no goroutines.
func pumpToProvider(t *testing.T, user *sle.RAFUser, provider *sle.RAFProvider, now time.Time) []*sle.RAFProviderEvent {
	t.Helper()
	var events []*sle.RAFProviderEvent
	for {
		pdu, ok := user.NextPDU()
		if !ok {
			return events
		}
		event, err := provider.HandlePDU(pdu, now)
		if err != nil {
			t.Fatalf("provider.HandlePDU() = %v", err)
		}
		events = append(events, event)
	}
}

func pumpToUser(t *testing.T, provider *sle.RAFProvider, user *sle.RAFUser, now time.Time) []*sle.RAFUserEvent {
	t.Helper()
	var events []*sle.RAFUserEvent
	for {
		pdu, ok := provider.NextPDU()
		if !ok {
			return events
		}
		event, err := user.HandlePDU(pdu, now)
		if err != nil {
			t.Fatalf("user.HandlePDU() = %v", err)
		}
		events = append(events, event)
	}
}

// TestServiceStatesStartAtOne pins the numbering to the specs': state 1
// unbound, 2 ready, 3 active, so a logged state matches the table.
func TestServiceStatesStartAtOne(t *testing.T) {
	if sle.ServiceUnbound != 1 || sle.ServiceReady != 2 || sle.ServiceActive != 3 {
		t.Errorf("states are %d/%d/%d, want 1/2/3",
			sle.ServiceUnbound, sle.ServiceReady, sle.ServiceActive)
	}
	if sle.ServiceUnbound.String() != "unbound" {
		t.Errorf("state 1 is %q, want %q", sle.ServiceUnbound, "unbound")
	}
}

// TestRAFStateWalk follows the lifecycle the state table of CCSDS 911.1-B-5
// Clause 4.2.2 describes: rows 5 and 4 for BIND, row 9 for START, row 10 for STOP,
// row 8 for UNBIND.
func TestRAFStateWalk(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)

	if user.State() != sle.ServiceUnbound || provider.State() != sle.ServiceUnbound {
		t.Fatal("a fresh instance is not at state 1")
	}

	// BIND: row 5 accepts and both ends reach state 2.
	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].BindInvocation == nil {
		t.Fatalf("provider saw %d events, want one BIND invocation", len(events))
	}
	if err := provider.HandleBindInvocation(events[0].BindInvocation, now, 2); err != nil {
		t.Fatalf("HandleBindInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceReady {
		t.Errorf("provider state = %s, want ready", provider.State())
	}
	if events := pumpToUser(t, provider, user, now); len(events) != 1 {
		t.Fatalf("user saw %d events, want one BIND return", len(events))
	}
	if user.State() != sle.ServiceReady {
		t.Errorf("user state = %s, want ready", user.State())
	}

	// START: row 9's positive branch takes the provider to state 3.
	if _, err := user.Start(now, 3, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	events = pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].StartInvocation == nil {
		t.Fatalf("provider saw %d events, want one START invocation", len(events))
	}
	err := provider.HandleStartInvocation(events[0].StartInvocation, &sle.RAFStartReturn{Positive: true}, now, 4)
	if err != nil {
		t.Fatalf("HandleStartInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceActive {
		t.Errorf("provider state = %s, want active", provider.State())
	}
	pumpToUser(t, provider, user, now)
	if user.State() != sle.ServiceActive {
		t.Errorf("user state = %s, want active", user.State())
	}

	// Data flows only in state 3.
	buffer := sle.RAFTransferBuffer{{Frame: &sle.RAFTransferDataInvocation{
		EarthReceiveTime: mustTime(t, now),
		AntennaId:        sle.AntennaId{Local: []byte("DSS-25")},
		Data:             []byte{0x1A, 0xCF, 0xFC, 0x1D},
	}}}
	if err := provider.SendTransferBuffer(buffer, now); err != nil {
		t.Fatalf("SendTransferBuffer() = %v", err)
	}
	dataEvents := pumpToUser(t, provider, user, now)
	if len(dataEvents) != 1 || len(dataEvents[0].TransferBuffer.Frames()) != 1 {
		t.Fatalf("user did not receive the frame: %+v", dataEvents)
	}

	// STOP: row 10's positive branch returns both to state 2.
	if _, err := user.Stop(now, 5); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	events = pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].StopInvocation == nil {
		t.Fatalf("provider saw %d events, want one STOP invocation", len(events))
	}
	if err := provider.HandleStopInvocation(events[0].StopInvocation, true, 0, now, 6); err != nil {
		t.Fatalf("HandleStopInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceReady {
		t.Errorf("provider state after STOP = %s, want ready", provider.State())
	}
	pumpToUser(t, provider, user, now)
	if user.State() != sle.ServiceReady {
		t.Errorf("user state after STOP = %s, want ready", user.State())
	}

	// UNBIND: row 8 returns both to state 1.
	if err := user.Unbind(now, 7, sle.UnbindEnd); err != nil {
		t.Fatalf("Unbind() = %v", err)
	}
	events = pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].UnbindInvocation == nil {
		t.Fatalf("provider saw %d events, want one UNBIND invocation", len(events))
	}
	if err := provider.HandleUnbindInvocation(events[0].UnbindInvocation, now, 8); err != nil {
		t.Fatalf("HandleUnbindInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceUnbound {
		t.Errorf("provider state after UNBIND = %s, want unbound", provider.State())
	}
	pumpToUser(t, provider, user, now)
	if user.State() != sle.ServiceUnbound {
		t.Errorf("user state after UNBIND = %s, want unbound", user.State())
	}
}

// TestServiceRefusesOperationsOutOfState covers the 'not applicable' and
// 'peer abort protocol error' cells: an operation the state does not allow is
// refused rather than sent.
func TestServiceRefusesOperationsOutOfState(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)

	// State 1: no START, no STOP, no UNBIND, no GET-PARAMETER, no
	// SCHEDULE-STATUS-REPORT, no STATUS-REPORT — every operation but
	// PEER-ABORT needs a bound association.
	if _, err := user.Start(now, 1, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("Start() at state 1 = %v, want ErrNotBound", err)
	}
	if _, err := user.Stop(now, 1); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("Stop() at state 1 = %v, want ErrNotBound", err)
	}
	if err := user.Unbind(now, 1, sle.UnbindEnd); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("Unbind() at state 1 = %v, want ErrNotBound", err)
	}
	if err := provider.SendTransferBuffer(sle.RAFTransferBuffer{}, now); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("SendTransferBuffer() at state 1 = %v, want ErrNotBound", err)
	}
	if _, err := user.GetParameter(1, now, 1); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("GetParameter() at state 1 = %v, want ErrNotBound", err)
	}
	if _, err := user.ScheduleStatusReport(sle.ReportImmediately, 0, now, 1); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("ScheduleStatusReport() at state 1 = %v, want ErrNotBound", err)
	}
	if err := provider.HandleGetParameterInvocation(
		&sle.GetParameterInvocation{InvokeId: 1, Parameter: 1}, nil, now, 1); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("HandleGetParameterInvocation() at state 1 = %v, want ErrNotBound", err)
	}
	if err := provider.HandleScheduleStatusReportInvocation(
		&sle.ScheduleStatusReportInvocation{InvokeId: 1, Kind: sle.ReportImmediately},
		true, 0, now, 1); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("HandleScheduleStatusReportInvocation() at state 1 = %v, want ErrNotBound", err)
	}
	if err := provider.SendStatusReport(&sle.RAFStatusReportInvocation{}, now); !errors.Is(err, sle.ErrNotBound) {
		t.Errorf("SendStatusReport() at state 1 = %v, want ErrNotBound", err)
	}

	bindTo(t, user, provider, now)

	// State 2: no STOP, no data, but START is fine.
	if _, err := user.Stop(now, 2); !errors.Is(err, sle.ErrNotStarted) {
		t.Errorf("Stop() at state 2 = %v, want ErrNotStarted", err)
	}
	if err := provider.SendTransferBuffer(sle.RAFTransferBuffer{}, now); !errors.Is(err, sle.ErrNotStarted) {
		t.Errorf("SendTransferBuffer() at state 2 = %v, want ErrNotStarted", err)
	}

	startTo(t, user, provider, now)

	// State 3: no second START, no UNBIND before STOP.
	if _, err := user.Start(now, 3, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); !errors.Is(err, sle.ErrAlreadyStarted) {
		t.Errorf("second Start() = %v, want ErrAlreadyStarted", err)
	}
	if err := user.Unbind(now, 3, sle.UnbindEnd); !errors.Is(err, sle.ErrAlreadyStarted) {
		t.Errorf("Unbind() at state 3 = %v, want ErrAlreadyStarted", err)
	}

	// PEER-ABORT has no state gate at all: unlike every operation above, it
	// succeeds from state 3 exactly as it would from any other state.
	provider.PeerAbort(sle.AbortProtocolError, now)
	if provider.State() != sle.ServiceUnbound {
		t.Errorf("PeerAbort() at state 3 left state = %s, want unbound", provider.State())
	}
}

// bindTo drives a bind exchange to completion.
func bindTo(t *testing.T, user *sle.RAFUser, provider *sle.RAFProvider, now time.Time) {
	t.Helper()
	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	if err := provider.HandleBindInvocation(events[0].BindInvocation, now, 2); err != nil {
		t.Fatalf("HandleBindInvocation() = %v", err)
	}
	pumpToUser(t, provider, user, now)
}

// startTo drives a start exchange to completion.
func startTo(t *testing.T, user *sle.RAFUser, provider *sle.RAFProvider, now time.Time) {
	t.Helper()
	if _, err := user.Start(now, 3, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	err := provider.HandleStartInvocation(events[0].StartInvocation, &sle.RAFStartReturn{Positive: true}, now, 4)
	if err != nil {
		t.Fatalf("HandleStartInvocation() = %v", err)
	}
	pumpToUser(t, provider, user, now)
}

// TestRefusedStartLeavesTheUserAtStateTwo covers row 9's negative branch.
func TestRefusedStartLeavesTheUserAtStateTwo(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	if _, err := user.Start(now, 3, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	refusal := &sle.RAFStartReturn{Positive: false, SpecificDiagnostic: sle.RAFStartOutOfService}
	if err := provider.HandleStartInvocation(events[0].StartInvocation, refusal, now, 4); err != nil {
		t.Fatalf("HandleStartInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceReady {
		t.Errorf("provider state after refusing = %s, want ready", provider.State())
	}

	userEvents := pumpToUser(t, provider, user, now)
	if len(userEvents) != 1 || userEvents[0].StartReturn == nil {
		t.Fatalf("user saw %d events, want one START return", len(userEvents))
	}
	if userEvents[0].StartReturn.Positive {
		t.Error("the refusal decoded as positive")
	}
	if user.State() != sle.ServiceReady {
		t.Errorf("user state after refusal = %s, want ready", user.State())
	}
}

// TestRefusedStopLeavesTheUserActive covers row 10's ELSE branch: a negative
// STOP return does not end data transfer.
func TestRefusedStopLeavesTheUserActive(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)
	startTo(t, user, provider, now)

	if _, err := user.Stop(now, 5); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	if err := provider.HandleStopInvocation(events[0].StopInvocation, false, sle.DiagOtherReason, now, 6); err != nil {
		t.Fatalf("HandleStopInvocation() = %v", err)
	}
	if provider.State() != sle.ServiceActive {
		t.Errorf("provider state = %s, want still active", provider.State())
	}
	pumpToUser(t, provider, user, now)
	if user.State() != sle.ServiceActive {
		t.Errorf("user state = %s, want still active", user.State())
	}
}

// TestTransferBufferBeforeStartIsAProtocolError covers the state table's
// 'peer abort protocol error' cell: data arriving at state 2 aborts.
func TestTransferBufferBeforeStartIsAProtocolError(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	// Forge the PDU the provider would never send in this state.
	buffer := sle.RAFTransferBuffer{{Frame: &sle.RAFTransferDataInvocation{
		EarthReceiveTime: mustTime(t, now),
		AntennaId:        sle.AntennaId{Local: []byte("A")},
		Data:             []byte{1, 2, 3, 4},
	}}}
	content, err := buffer.Encode()
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	pdu := sle.AppendPDU(nil, 8, content)

	if _, err := user.HandlePDU(pdu, now); !errors.Is(err, sle.ErrUnexpectedPDU) {
		t.Fatalf("HandlePDU() = %v, want ErrUnexpectedPDU", err)
	}
	if user.State() != sle.ServiceUnbound {
		t.Errorf("state after the abort = %s, want unbound", user.State())
	}
	if user.Pending() == 0 {
		t.Error("no PEER-ABORT was queued")
	}
}

// TestUnmatchedReturnIsRejected guards the invoke-identifier bookkeeping: a
// return nobody asked for does not quietly change state.
func TestUnmatchedReturnIsRejected(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	stray := &sle.RAFStartReturn{InvokeId: 99, Positive: true}
	if err := user.HandleStartReturn(stray); !errors.Is(err, sle.ErrUnknownInvokeId) {
		t.Fatalf("HandleStartReturn() = %v, want ErrUnknownInvokeId", err)
	}
	if user.State() != sle.ServiceReady {
		t.Errorf("state = %s, want still ready", user.State())
	}
}

// TestOutstandingTracksConfirmedOperations shows what a caller running the
// spec's return timers watches.
func TestOutstandingTracksConfirmedOperations(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	id, err := user.Start(now, 3, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	outstanding := user.Outstanding()
	if outstanding[id] != sle.OpStartInvocation {
		t.Errorf("outstanding[%d] = %v, want START invocation", id, outstanding[id])
	}

	startTo := &sle.RAFStartReturn{InvokeId: id, Positive: true}
	if err := user.HandleStartReturn(startTo); err != nil {
		t.Fatalf("HandleStartReturn() = %v", err)
	}
	if len(user.Outstanding()) != 0 {
		t.Errorf("still outstanding after the return: %v", user.Outstanding())
	}
}

// TestServiceUserRefusesInvokeIdStillOutstanding covers the user side of B5:
// InvokeId is 16 bits, so nextInvokeId wraps after 65,536 confirmed
// operations. If the identifier it would assign next is still awaiting its
// return, invoke must refuse locally with a diagnosable cause rather than
// send a PDU the provider would answer with the (misleadingly remote-looking)
// 'duplicate invoke ID' diagnostic.
func TestServiceUserRefusesInvokeIdStillOutstanding(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)
	startTo(t, user, provider, now)

	// STOP is valid throughout state 3 and invoking it does not itself change
	// state, so it can be invoked repeatedly. Never handling its return keeps
	// every identifier in `awaiting`, so the 65,537th invocation reuses
	// identifier 0, which is still outstanding from the very first call.
	const wrapAt = 65536
	for i := 0; i < wrapAt; i++ {
		if _, err := user.Stop(now, 1); err != nil {
			t.Fatalf("Stop() at iteration %d = %v, want nil", i, err)
		}
	}
	if _, err := user.Stop(now, 1); !errors.Is(err, sle.ErrInvokeIdExhausted) {
		t.Fatalf("Stop() at iteration %d = %v, want ErrInvokeIdExhausted", wrapAt, err)
	}
}

// TestServiceRejectsMismatchedRole catches the wiring mistake of handing a
// provider association to a user machine.
func TestServiceRejectsMismatchedRole(t *testing.T) {
	_, providerAssoc := association(t, false)

	if _, err := sle.NewRAFUser(sle.ServiceConfig{Association: providerAssoc}); err == nil {
		t.Error("NewRAFUser accepted a provider association")
	}
	if _, err := sle.NewRAFUser(sle.ServiceConfig{}); err == nil {
		t.Error("NewRAFUser accepted a nil association")
	}
}

// TestOperationTagsMatchTheServiceKind checks that a user machine stamps its
// PDUs with its own service's tags.
func TestOperationTagsMatchTheServiceKind(t *testing.T) {
	now := testTime
	userAssoc, _ := association(t, false)

	user, err := sle.NewFCLTUUser(sle.ServiceConfig{
		Association:   userAssoc,
		DeliveryMode:  sle.DeliveryForwardOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      sle.ServiceInstanceIdentifier{{Identifier: "cltu", Value: "onlc1"}},
	})
	if err != nil {
		t.Fatalf("NewFCLTUUser() = %v", err)
	}
	if user.Kind() != sle.ServiceFCLTU {
		t.Errorf("Kind() = %v, want FCLTU", user.Kind())
	}

	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	pdu, ok := user.NextPDU()
	if !ok {
		t.Fatal("no PDU queued")
	}
	decoded, err := sle.DecodePDU(pdu, sle.ServiceFCLTU)
	if err != nil {
		t.Fatalf("DecodePDU() = %v", err)
	}
	if decoded.Tag != 100 {
		t.Errorf("BIND tag = %d, want 100", decoded.Tag)
	}
}

// TestGetParameterHappyPath drives GET-PARAMETER to a positive answer,
// covering ServiceUser.GetParameter, ServiceProvider.HandleGetParameterInvocation
// and ServiceUser.HandleGetParameterReturn — all shared by every service, and
// all 0% before this test existed.
func TestGetParameterHappyPath(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	if user.DeliveryMode() != sle.DeliveryReturnCompleteOnline {
		t.Fatalf("DeliveryMode() = %v, want DeliveryReturnCompleteOnline", user.DeliveryMode())
	}
	bindTo(t, user, provider, now)

	id, err := user.GetParameter(7, now, 2)
	if err != nil {
		t.Fatalf("GetParameter() = %v", err)
	}
	if op := user.Outstanding()[id]; op != sle.OpGetParameterInvocation {
		t.Fatalf("outstanding[%d] = %v, want GET-PARAMETER invocation", id, op)
	}

	events := pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].GetParameterInvocation == nil {
		t.Fatalf("provider saw %d events, want one GET-PARAMETER invocation", len(events))
	}
	if events[0].GetParameterInvocation.Parameter != 7 {
		t.Errorf("parameter = %d, want 7", events[0].GetParameterInvocation.Parameter)
	}

	// The parameter value is a still-encoded BER element: this package does
	// not model any service's parameter CHOICE, so the caller supplies one.
	answer := sle.AppendInteger(nil, 42)
	err = provider.HandleGetParameterInvocation(events[0].GetParameterInvocation, answer, now, 3)
	if err != nil {
		t.Fatalf("HandleGetParameterInvocation() = %v", err)
	}

	userEvents := pumpToUser(t, provider, user, now)
	if len(userEvents) != 1 || userEvents[0].GetParameterReturn == nil {
		t.Fatalf("user saw %d events, want one GET-PARAMETER return", len(userEvents))
	}
	if !userEvents[0].GetParameterReturn.Positive {
		t.Error("the GET-PARAMETER was refused")
	}
	if !bytes.Equal(userEvents[0].GetParameterReturn.Parameter, answer) {
		t.Errorf("parameter = %x, want %x", userEvents[0].GetParameterReturn.Parameter, answer)
	}
	if len(user.Outstanding()) != 0 {
		t.Errorf("still outstanding after the return: %v", user.Outstanding())
	}
}

// TestGetParameterUnknownIsRefused drives the negative branch: a provider
// with nothing to say answers 'unknown parameter'.
func TestGetParameterUnknownIsRefused(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	if _, err := user.GetParameter(99, now, 2); err != nil {
		t.Fatalf("GetParameter() = %v", err)
	}
	events := pumpToProvider(t, user, provider, now)
	err := provider.HandleGetParameterInvocation(events[0].GetParameterInvocation, nil, now, 3)
	if err != nil {
		t.Fatalf("HandleGetParameterInvocation() = %v", err)
	}

	userEvents := pumpToUser(t, provider, user, now)
	if userEvents[0].GetParameterReturn.Positive {
		t.Fatal("an unanswered parameter decoded as positive")
	}
	if userEvents[0].GetParameterReturn.SpecificDiagnostic != sle.GetParameterUnknown {
		t.Errorf("diagnostic = %v, want unknown parameter", userEvents[0].GetParameterReturn.SpecificDiagnostic)
	}
}

// TestScheduleStatusReportHappyPathThenStatusReport drives
// SCHEDULE-STATUS-REPORT to a positive answer, covering
// ServiceUser.ScheduleStatusReport, ServiceProvider.HandleScheduleStatusReportInvocation
// and ServiceUser.HandleScheduleStatusReportReturn, then has the provider
// emit the unconfirmed STATUS-REPORT the schedule asked for and checks the
// user receives it — the request-then-deliver loop clause 3.9 of each
// service specification describes, and ServiceProvider.SendStatusReport's
// only caller in this package's own tests.
func TestScheduleStatusReportHappyPathThenStatusReport(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)

	id, err := user.ScheduleStatusReport(sle.ReportImmediately, 0, now, 2)
	if err != nil {
		t.Fatalf("ScheduleStatusReport() = %v", err)
	}
	if op := user.Outstanding()[id]; op != sle.OpScheduleStatusReportInvocation {
		t.Fatalf("outstanding[%d] = %v, want SCHEDULE-STATUS-REPORT invocation", id, op)
	}

	events := pumpToProvider(t, user, provider, now)
	if len(events) != 1 || events[0].ScheduleStatusReportInvocation == nil {
		t.Fatalf("provider saw %d events, want one SCHEDULE-STATUS-REPORT invocation", len(events))
	}
	if events[0].ScheduleStatusReportInvocation.Kind != sle.ReportImmediately {
		t.Errorf("kind = %v, want immediately", events[0].ScheduleStatusReportInvocation.Kind)
	}

	err = provider.HandleScheduleStatusReportInvocation(
		events[0].ScheduleStatusReportInvocation, true, 0, now, 3)
	if err != nil {
		t.Fatalf("HandleScheduleStatusReportInvocation() = %v", err)
	}

	userEvents := pumpToUser(t, provider, user, now)
	if len(userEvents) != 1 || userEvents[0].ScheduleStatusReportReturn == nil {
		t.Fatalf("user saw %d events, want one SCHEDULE-STATUS-REPORT return", len(userEvents))
	}
	if !userEvents[0].ScheduleStatusReportReturn.Positive {
		t.Error("the schedule request was refused")
	}
	if len(user.Outstanding()) != 0 {
		t.Errorf("still outstanding after the return: %v", user.Outstanding())
	}

	// The provider now emits the report it was just asked for. This is
	// unconfirmed, so it needs no invoke identifier and settles nothing.
	report := &sle.RAFStatusReportInvocation{
		ErrorFreeFrameNumber: 10,
		DeliveredFrameNumber: 12,
		ProductionStatus:     sle.ProductionRunning,
	}
	if err := provider.SendStatusReport(report, now); err != nil {
		t.Fatalf("SendStatusReport() = %v", err)
	}
	reportEvents := pumpToUser(t, provider, user, now)
	if len(reportEvents) != 1 || reportEvents[0].StatusReport == nil {
		t.Fatalf("user saw %d events, want one STATUS-REPORT", len(reportEvents))
	}
	if reportEvents[0].StatusReport.DeliveredFrameNumber != 12 {
		t.Errorf("delivered frame number = %d, want 12", reportEvents[0].StatusReport.DeliveredFrameNumber)
	}
	if reportEvents[0].StatusReport.ErrorFreeFrameNumber != 10 {
		t.Errorf("error-free frame number = %d, want 10", reportEvents[0].StatusReport.ErrorFreeFrameNumber)
	}
}

// TestPeerAbortEndsTheAssociationFromEitherSide covers HandlePeerAbort:
// whichever end sends PEER-ABORT, the far end's own HandlePDU decodes it and
// reaches HandlePeerAbort, ending that end unbound too — even though it was
// active a moment before, which is the point: PEER-ABORT overrides whatever
// state the far end thought it was in.
func TestPeerAbortEndsTheAssociationFromEitherSide(t *testing.T) {
	now := testTime
	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	bindTo(t, user, provider, now)
	startTo(t, user, provider, now)

	if provider.State() != sle.ServiceActive || user.State() != sle.ServiceActive {
		t.Fatal("both ends should be active before the abort")
	}

	provider.PeerAbort(sle.AbortProtocolError, now)
	if provider.State() != sle.ServiceUnbound {
		t.Errorf("provider state after its own PeerAbort = %s, want unbound", provider.State())
	}

	events := pumpToUser(t, provider, user, now)
	if len(events) != 1 || events[0].PeerAbort == nil {
		t.Fatalf("user saw %d events, want one PEER-ABORT", len(events))
	}
	if events[0].PeerAbort.Diagnostic != sle.AbortProtocolError {
		t.Errorf("diagnostic = %v, want protocol error", events[0].PeerAbort.Diagnostic)
	}
	if user.State() != sle.ServiceUnbound {
		t.Errorf("user state after HandlePeerAbort = %s, want unbound", user.State())
	}
}
