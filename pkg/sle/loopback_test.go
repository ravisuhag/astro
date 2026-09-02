package sle_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
	"github.com/ravisuhag/astro/pkg/tcsc"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// The loopback tests are the only ones here that touch a socket, and they use
// net.Pipe rather than a real one. net.Pipe rendezvouses synchronously (a
// write blocks until someone reads) so each test runs one side in a
// goroutine. That goroutine is the test's, not the library's: nothing under
// pkg/sle starts one.

// tmlPair connects two ends over net.Pipe and returns readers and writers for
// TML messages.
func tmlPair(t *testing.T) (userConn, providerConn net.Conn) {
	t.Helper()
	userConn, providerConn = net.Pipe()
	t.Cleanup(func() {
		_ = userConn.Close()
		_ = providerConn.Close()
	})
	return userConn, providerConn
}

// sendPDU wraps an SLE PDU in a TML data message and writes it.
func sendPDU(conn net.Conn, pdu []byte) error {
	return sle.WriteMessage(conn, &sle.Message{Type: sle.MessageSLEPDU, Body: pdu})
}

// recvPDU reads one TML message and returns the PDU inside it.
func recvPDU(conn net.Conn) ([]byte, error) {
	message, err := sle.ReadMessage(conn, sle.DefaultMaxMessageSize)
	if err != nil {
		return nil, err
	}
	if message.Type != sle.MessageSLEPDU {
		return nil, sle.ErrInvalidMessageType
	}
	return message.Body, nil
}

// TestRAFLoopbackDeliversACADU runs a whole RAF session over net.Pipe and
// checks a real CADU survives it: pkg/tmdl builds the frame, pkg/tmsc wraps
// it, RAF carries it, and pkg/tmsc unwraps it back to the same bytes.
func TestRAFLoopbackDeliversACADU(t *testing.T) {
	now := testTime

	frame, err := tmdl.NewTMTransferFrame(0x2A, 1, bytes.Repeat([]byte{0x42}, 100), nil, nil)
	if err != nil {
		t.Fatalf("NewTMTransferFrame() = %v", err)
	}
	frameBytes, err := frame.Encode()
	if err != nil {
		t.Fatalf("frame.Encode() = %v", err)
	}
	cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)

	user, provider := servicePair(t, sle.DeliveryReturnCompleteOnline)
	userConn, providerConn := tmlPair(t)

	// The provider side runs in the test's goroutine budget, answering
	// whatever the user sends until the connection closes.
	done := make(chan error, 1)
	go func() {
		done <- runRAFProvider(provider, providerConn, cadu, now)
	}()

	// BIND.
	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	if err := drain(user, userConn); err != nil {
		t.Fatalf("sending BIND: %v", err)
	}
	if _, err := readInto(t, user, userConn, now); err != nil {
		t.Fatalf("BIND return: %v", err)
	}
	if user.State() != sle.ServiceReady {
		t.Fatalf("state after BIND = %s, want ready", user.State())
	}

	// START.
	if _, err := user.Start(now, 2, sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	if err := drain(user, userConn); err != nil {
		t.Fatalf("sending START: %v", err)
	}
	if _, err := readInto(t, user, userConn, now); err != nil {
		t.Fatalf("START return: %v", err)
	}
	if user.State() != sle.ServiceActive {
		t.Fatalf("state after START = %s, want active", user.State())
	}

	// The frame arrives unprompted, in a transfer buffer.
	event, err := readInto(t, user, userConn, now)
	if err != nil {
		t.Fatalf("transfer buffer: %v", err)
	}
	frames := event.TransferBuffer.Frames()
	if len(frames) != 1 {
		t.Fatalf("received %d frames, want 1", len(frames))
	}

	recovered, err := tmsc.UnwrapCADU(frames[0].Data, tmsc.DefaultASM(), true)
	if err != nil {
		t.Fatalf("UnwrapCADU() = %v", err)
	}
	if !bytes.Equal(recovered, frameBytes) {
		t.Errorf("the frame did not survive the session: %d octets back, want %d",
			len(recovered), len(frameBytes))
	}

	// STOP, then UNBIND.
	if _, err := user.Stop(now, 3); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	if err := drain(user, userConn); err != nil {
		t.Fatalf("sending STOP: %v", err)
	}
	if _, err := readInto(t, user, userConn, now); err != nil {
		t.Fatalf("STOP return: %v", err)
	}
	if user.State() != sle.ServiceReady {
		t.Fatalf("state after STOP = %s, want ready", user.State())
	}

	if err := user.Unbind(now, 4, sle.UnbindEnd); err != nil {
		t.Fatalf("Unbind() = %v", err)
	}
	if err := drain(user, userConn); err != nil {
		t.Fatalf("sending UNBIND: %v", err)
	}
	if _, err := readInto(t, user, userConn, now); err != nil {
		t.Fatalf("UNBIND return: %v", err)
	}
	if user.State() != sle.ServiceUnbound {
		t.Errorf("state after UNBIND = %s, want unbound", user.State())
	}

	_ = userConn.Close()
	if err := <-done; err != nil && !isClosed(err) {
		t.Errorf("provider side: %v", err)
	}
}

// drain writes every PDU the machine has queued.
func drain(user *sle.RAFUser, conn net.Conn) error {
	for {
		pdu, ok := user.NextPDU()
		if !ok {
			return nil
		}
		if err := sendPDU(conn, pdu); err != nil {
			return err
		}
	}
}

// readInto reads one PDU and feeds it to the user machine.
func readInto(t *testing.T, user *sle.RAFUser, conn net.Conn, now time.Time) (*sle.RAFUserEvent, error) {
	t.Helper()
	pdu, err := recvPDU(conn)
	if err != nil {
		return nil, err
	}
	return user.HandlePDU(pdu, now)
}

// runRAFProvider answers a user's session, sending one CADU once started.
func runRAFProvider(provider *sle.RAFProvider, conn net.Conn, cadu []byte, now time.Time) error {
	random := int32(100)
	for {
		pdu, err := recvPDU(conn)
		if err != nil {
			return err
		}
		event, err := provider.HandlePDU(pdu, now)
		if err != nil {
			return err
		}
		random++

		switch event.Operation {
		case sle.OpBindInvocation:
			if err := provider.HandleBindInvocation(event.BindInvocation, now, random); err != nil {
				return err
			}
		case sle.OpStartInvocation:
			err := provider.HandleStartInvocation(
				event.StartInvocation, &sle.RAFStartReturn{Positive: true}, now, random)
			if err != nil {
				return err
			}
			// Answer the START, then deliver.
			if err := drainProvider(provider, conn); err != nil {
				return err
			}
			buffer := sle.RAFTransferBuffer{{Frame: &sle.RAFTransferDataInvocation{
				EarthReceiveTime:      mustTimeOrZero(now),
				AntennaId:             sle.AntennaId{Local: []byte("DSS-25")},
				DataLinkContinuity:    -1,
				DeliveredFrameQuality: sle.FrameGood,
				Data:                  cadu,
			}}}
			if err := provider.SendTransferBuffer(buffer, now); err != nil {
				return err
			}
		case sle.OpStopInvocation:
			if err := provider.HandleStopInvocation(event.StopInvocation, true, 0, now, random); err != nil {
				return err
			}
		case sle.OpUnbindInvocation:
			if err := provider.HandleUnbindInvocation(event.UnbindInvocation, now, random); err != nil {
				return err
			}
		}

		if err := drainProvider(provider, conn); err != nil {
			return err
		}
	}
}

// drainProvider writes every PDU the provider has queued.
func drainProvider(provider *sle.RAFProvider, conn net.Conn) error {
	for {
		pdu, ok := provider.NextPDU()
		if !ok {
			return nil
		}
		if err := sendPDU(conn, pdu); err != nil {
			return err
		}
	}
}

// mustTimeOrZero converts a time, falling back to the zero value. The
// provider goroutine cannot fail a test, so it cannot use mustTime.
func mustTimeOrZero(at time.Time) sle.Time {
	converted, err := sle.NewTime(at)
	if err != nil {
		return sle.Time{}
	}
	return converted
}

// isClosed reports whether an error is the expected end of a closed pipe.
func isClosed(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, net.ErrClosed)
}

// TestFCLTULoopbackRadiatesACLTU runs the forward service the other way: the
// user sends a CLTU that pkg/tcsc built, the provider accepts it and then
// reports it radiated.
func TestFCLTULoopbackRadiatesACLTU(t *testing.T) {
	now := testTime

	cltu, err := tcsc.WrapCLTU(bytes.Repeat([]byte{0x33}, 40), nil, nil, false)
	if err != nil {
		t.Fatalf("WrapCLTU() = %v", err)
	}

	userAssoc, providerAssoc := association(t, false)
	instance := sle.ServiceInstanceIdentifier{{Identifier: "cltu", Value: "onlc1"}}

	user, err := sle.NewFCLTUUser(sle.ServiceConfig{
		Association:   userAssoc,
		DeliveryMode:  sle.DeliveryForwardOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      instance,
	})
	if err != nil {
		t.Fatalf("NewFCLTUUser() = %v", err)
	}
	provider, err := sle.NewFCLTUProvider(sle.ServiceConfig{
		Association:   providerAssoc,
		DeliveryMode:  sle.DeliveryForwardOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      instance,
	})
	if err != nil {
		t.Fatalf("NewFCLTUProvider() = %v", err)
	}

	// BIND.
	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	bindEvents := moveFCLTU(t, user, provider, now)
	if len(bindEvents) != 1 || bindEvents[0].BindInvocation == nil {
		t.Fatalf("provider saw %d events, want one BIND invocation", len(bindEvents))
	}
	if err := provider.HandleBindInvocation(bindEvents[0].BindInvocation, now, 2); err != nil {
		t.Fatalf("HandleBindInvocation() = %v", err)
	}
	moveFCLTUBack(t, provider, user, now)
	if user.State() != sle.ServiceReady {
		t.Fatalf("user state = %s, want ready", user.State())
	}

	// START at CLTU number 500.
	if _, err := user.Start(now, 3, 500); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	startEvent := nextProviderEvent(t, user, provider, now)
	err = provider.HandleStartInvocation(
		startEvent.StartInvocation,
		&sle.FCLTUStartReturn{Positive: true, StartRadiationTime: mustTime(t, now)},
		now, 4)
	if err != nil {
		t.Fatalf("HandleStartInvocation() = %v", err)
	}
	moveFCLTUBack(t, provider, user, now)
	if user.State() != sle.ServiceActive {
		t.Fatalf("user state = %s, want active", user.State())
	}

	expected, known := provider.ExpectedCltuIdentification()
	if !known || expected != 500 {
		t.Fatalf("provider expects %d (known %v), want 500", expected, known)
	}

	// TRANSFER-DATA: the CLTU goes up, the provider takes it.
	_, cltuID, err := user.TransferData(now, 5, cltu, sle.ConditionalTime{}, sle.ConditionalTime{},
		0, sle.ProduceNotification)
	if err != nil {
		t.Fatalf("TransferData() = %v", err)
	}
	if cltuID != 500 {
		t.Errorf("first CLTU numbered %d, want 500", cltuID)
	}

	dataEvent := nextProviderEvent(t, user, provider, now)
	if dataEvent.TransferDataInvocation == nil {
		t.Fatal("provider did not receive a CLTU")
	}
	if !bytes.Equal(dataEvent.TransferDataInvocation.Data, cltu) {
		t.Error("the CLTU did not survive the session")
	}
	recovered, _, err := tcsc.UnwrapCLTU(dataEvent.TransferDataInvocation.Data, nil, nil, false)
	if err != nil {
		t.Fatalf("UnwrapCLTU() = %v", err)
	}
	// WrapCLTU pads the last codeblock and the padding is not
	// self-describing, so only the leading 40 octets are the frame.
	if len(recovered) < 40 || !bytes.Equal(recovered[:40], bytes.Repeat([]byte{0x33}, 40)) {
		t.Error("the CLTU did not unwrap back to the frame it carried")
	}

	err = provider.HandleTransferDataInvocation(dataEvent.TransferDataInvocation, true, 0, 65536, now, 6)
	if err != nil {
		t.Fatalf("HandleTransferDataInvocation() = %v", err)
	}
	moveFCLTUBack(t, provider, user, now)

	next, known := user.NextCltuIdentification()
	if !known || next != 501 {
		t.Errorf("next CLTU number = %d (known %v), want 501", next, known)
	}

	// The provider reports the radiation afterwards, which is what makes the
	// service asynchronous.
	notify := &sle.FCLTUAsyncNotifyInvocation{
		Kind: sle.NotifyCltuRadiated,
		LastProcessed: sle.CltuLastProcessed{
			Processed:          true,
			CltuIdentification: 500,
			RadiationStartTime: sle.ConditionalTime{Known: true, Time: mustTime(t, now)},
			Status:             sle.CltuRadiated,
		},
		LastOk: sle.CltuLastOk{
			Ok:                 true,
			CltuIdentification: 500,
			RadiationStopTime:  mustTime(t, now),
		},
		ProductionStatus: sle.FCLTUProductionOperational,
		UplinkStatus:     sle.UplinkNominal,
	}
	if err := provider.SendAsyncNotify(notify, now); err != nil {
		t.Fatalf("SendAsyncNotify() = %v", err)
	}
	events := moveFCLTUBack(t, provider, user, now)
	if len(events) != 1 || events[0].AsyncNotify == nil {
		t.Fatalf("user saw %d events, want one ASYNC-NOTIFY", len(events))
	}
	if events[0].AsyncNotify.LastOk.CltuIdentification != 500 {
		t.Errorf("radiation reported for CLTU %d, want 500",
			events[0].AsyncNotify.LastOk.CltuIdentification)
	}
}

// TestFCLTURefusesAnOutOfSequenceCLTU checks the rule of clause 3.6.2.5: a CLTU
// whose number is not the expected one is refused, and the refusal carries
// the number the provider still wants.
func TestFCLTURefusesAnOutOfSequenceCLTU(t *testing.T) {
	now := testTime
	userAssoc, providerAssoc := association(t, false)
	instance := sle.ServiceInstanceIdentifier{{Identifier: "cltu", Value: "onlc1"}}

	config := sle.ServiceConfig{
		DeliveryMode:  sle.DeliveryForwardOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      instance,
	}
	userConfig, providerConfig := config, config
	userConfig.Association = userAssoc
	providerConfig.Association = providerAssoc

	user, err := sle.NewFCLTUUser(userConfig)
	if err != nil {
		t.Fatalf("NewFCLTUUser() = %v", err)
	}
	provider, err := sle.NewFCLTUProvider(providerConfig)
	if err != nil {
		t.Fatalf("NewFCLTUProvider() = %v", err)
	}

	if err := user.Bind(now, 1); err != nil {
		t.Fatalf("Bind() = %v", err)
	}
	bindEvents := moveFCLTU(t, user, provider, now)
	if len(bindEvents) != 1 || bindEvents[0].BindInvocation == nil {
		t.Fatalf("provider saw %d events, want one BIND invocation", len(bindEvents))
	}
	if err := provider.HandleBindInvocation(bindEvents[0].BindInvocation, now, 2); err != nil {
		t.Fatalf("HandleBindInvocation() = %v", err)
	}
	moveFCLTUBack(t, provider, user, now)

	if _, err := user.Start(now, 3, 10); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	startEvent := nextProviderEvent(t, user, provider, now)
	err = provider.HandleStartInvocation(startEvent.StartInvocation,
		&sle.FCLTUStartReturn{Positive: true, StartRadiationTime: mustTime(t, now)}, now, 4)
	if err != nil {
		t.Fatalf("HandleStartInvocation() = %v", err)
	}
	moveFCLTUBack(t, provider, user, now)

	// Hand the provider a CLTU numbered 99 when it wants 10.
	forged := &sle.FCLTUTransferDataInvocation{
		InvokeId:           7,
		CltuIdentification: 99,
		Data:               []byte{0xEB, 0x90, 0x00, 0x01},
	}
	err = provider.HandleTransferDataInvocation(forged, true, 0, 65536, now, 8)
	if !errors.Is(err, sle.ErrCltuOutOfSequence) {
		t.Fatalf("HandleTransferDataInvocation() = %v, want ErrCltuOutOfSequence", err)
	}

	pdu, ok := provider.NextPDU()
	if !ok {
		t.Fatal("no refusal was queued")
	}
	decoded, err := sle.DecodePDU(pdu, sle.ServiceFCLTU)
	if err != nil {
		t.Fatalf("DecodePDU() = %v", err)
	}
	answer, err := sle.DecodeFCLTUTransferDataReturn(decoded.Content)
	if err != nil {
		t.Fatalf("DecodeFCLTUTransferDataReturn() = %v", err)
	}
	if answer.Positive {
		t.Fatal("the out-of-sequence CLTU was accepted")
	}
	if answer.SpecificDiagnostic != sle.FCLTUDataOutOfSequence {
		t.Errorf("diagnostic = %v, want out of sequence", answer.SpecificDiagnostic)
	}
	if answer.CltuIdentification != 10 {
		t.Errorf("the refusal names %d as expected next, want 10", answer.CltuIdentification)
	}

	// The provider still wants 10, so nothing was consumed.
	expected, _ := provider.ExpectedCltuIdentification()
	if expected != 10 {
		t.Errorf("provider now expects %d, want 10", expected)
	}
}

// moveFCLTU passes every queued user PDU to the provider.
func moveFCLTU(t *testing.T, user *sle.FCLTUUser, provider *sle.FCLTUProvider, now time.Time) []*sle.FCLTUProviderEvent {
	t.Helper()
	var events []*sle.FCLTUProviderEvent
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

// moveFCLTUBack passes every queued provider PDU to the user.
func moveFCLTUBack(t *testing.T, provider *sle.FCLTUProvider, user *sle.FCLTUUser, now time.Time) []*sle.FCLTUUserEvent {
	t.Helper()
	var events []*sle.FCLTUUserEvent
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

// nextProviderEvent moves the user's queued PDUs across and returns the last
// event the provider saw.
func nextProviderEvent(t *testing.T, user *sle.FCLTUUser, provider *sle.FCLTUProvider, now time.Time) *sle.FCLTUProviderEvent {
	t.Helper()
	events := moveFCLTU(t, user, provider, now)
	if len(events) == 0 {
		t.Fatal("the provider saw no PDU")
	}
	return events[len(events)-1]
}
