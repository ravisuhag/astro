// Example: Pulling telemetry from a ground station with SLE
//
// Every other example here is one end of a space link. This one is the link
// between two ground systems: a mission control centre opening a TCP
// connection to a ground station and asking for the frames it recovered.
//
//	Mission control (the SLE user):
//	  1. Send the TML context message, agreeing heartbeats
//	  2. BIND, authenticating with SHA-256 credentials
//	  3. START, asking for all frame qualities from now on
//	  4. Take transfer buffers as they arrive
//	  5. STOP, then UNBIND
//
//	Ground station (the SLE provider):
//	  Answers each of those, and once started delivers a real CADU that
//	  pkg/tmdl framed and pkg/tmsc wrapped.
//
// The two halves talk over a real TCP connection on localhost. Nothing in
// pkg/sle opens a socket or starts a goroutine; both are the caller's, which
// is why the provider here runs in one this file started.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/ravisuhag/astro/pkg/sle"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

const (
	spacecraftID = 42
	vcid         = 1
	frameLength  = 256
)

// The service instance identifier names one configured service at the
// provider. These four attributes are the usual RAF form: agreement, pass,
// functional group, and the instance itself.
func serviceInstance() sle.ServiceInstanceIdentifier {
	return sle.ServiceInstanceIdentifier{
		{Identifier: "sagr", Value: "DEMOSAT"},
		{Identifier: "spack", Value: "PASS-0417"},
		{Identifier: "rsl-fg", Value: "1"},
		{Identifier: "raf", Value: "onlc1"},
	}
}

func main() {
	// A fixed clock, so credentials and the delivered Earth receive time are
	// the same on every run. Nothing in pkg/sle reads the clock itself.
	now := time.Date(2026, 4, 17, 8, 30, 0, 0, time.UTC)

	// The frame the ground station recovered off the antenna. This is the
	// output of the downlink guide, arriving at the other end of the chain.
	cadu := recoveredCADU()
	fmt.Printf("--- The ground station has a %d octet CADU ---\n", len(cadu))
	fmt.Println()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listening: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// The ground station side, in its own goroutine.
	done := make(chan error, 1)
	go func() { done <- groundStation(listener, cadu, now) }()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		log.Fatalf("connecting to the ground station: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := missionControl(conn, cadu, now); err != nil {
		log.Fatalf("mission control: %v", err)
	}

	_ = conn.Close()
	if err := <-done; err != nil && !closed(err) {
		log.Fatalf("ground station: %v", err)
	}
}

// missionControl is the SLE user: it drives the session and reads the frames.
func missionControl(conn net.Conn, expected []byte, now time.Time) error {
	association, err := sle.NewAssociation(sle.AssociationConfig{
		Role:              sle.RoleUser,
		LocalIdentifier:   "CTRL-CENTRE",
		PeerIdentifier:    "GROUND-STN",
		HeartbeatInterval: 30, // seconds
		DeadFactor:        3,  // three missed intervals means the peer is gone
		UserName:          "CTRL-CENTRE",
		Password:          []byte("user-secret"),
		PeerPassword:      []byte("provider-secret"),
		AcceptableDelay:   time.Minute,
	})
	if err != nil {
		return fmt.Errorf("building the association: %w", err)
	}

	user, err := sle.NewRAFUser(sle.ServiceConfig{
		Association:   association,
		DeliveryMode:  sle.DeliveryReturnCompleteOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      serviceInstance(),
	})
	if err != nil {
		return fmt.Errorf("building the RAF user: %w", err)
	}

	fmt.Println("--- Mission control: opening the session ---")
	fmt.Println()

	// The context message comes first and is the user's to send. It settles
	// the heartbeat interval and dead factor for the connection, and the
	// provider's copy of them replaces whatever it was configured with.
	if err := sle.WriteMessage(conn, association.ContextMessage(now)); err != nil {
		return fmt.Errorf("sending the context message: %w", err)
	}
	fmt.Printf("  context ..... heartbeat every 30s, dead after 3 missed\n")

	// BIND. The random number is the credential nonce; use a real random
	// source in anything that matters.
	if err := user.Bind(now, 1); err != nil {
		return fmt.Errorf("BIND: %w", err)
	}
	if err := send(conn, user); err != nil {
		return err
	}
	if _, err := receive(conn, user, now); err != nil {
		return fmt.Errorf("BIND return: %w", err)
	}
	fmt.Printf("  BIND ........ %s\n", user.State())

	// START. Two undefined ConditionalTimes mean "from now until I say stop",
	// which is what an online service wants. A pair of known times asks an
	// offline service for a stretch of the archive instead.
	if _, err := user.Start(now, 2,
		sle.ConditionalTime{}, sle.ConditionalTime{}, sle.FrameQualityAll); err != nil {
		return fmt.Errorf("START: %w", err)
	}
	if err := send(conn, user); err != nil {
		return err
	}
	if _, err := receive(conn, user, now); err != nil {
		return fmt.Errorf("START return: %w", err)
	}
	fmt.Printf("  START ....... %s, all frame qualities\n", user.State())
	fmt.Println()

	// Frames arrive unprompted now, in transfer buffers.
	fmt.Println("--- Mission control: taking delivery ---")
	fmt.Println()

	event, err := receive(conn, user, now)
	if err != nil {
		return fmt.Errorf("transfer buffer: %w", err)
	}

	frames := event.TransferBuffer.Frames()
	fmt.Printf("  transfer buffer with %d frame(s)\n", len(frames))
	for _, frame := range frames {
		fmt.Printf("    antenna ........... %s\n", frame.AntennaId.Local)
		fmt.Printf("    Earth receive time  %s\n",
			frame.EarthReceiveTime.Humanize())
		fmt.Printf("    quality ........... %s\n", frame.DeliveredFrameQuality)
		fmt.Printf("    continuity ........ %d\n", frame.DataLinkContinuity)
		fmt.Printf("    data .............. %d octets\n", len(frame.Data))

		// What arrived is the CADU, so the frame comes out of it the same way
		// it would at the ground station.
		recovered, err := tmsc.UnwrapCADU(frame.Data, tmsc.DefaultASM(), true)
		if err != nil {
			return fmt.Errorf("unwrapping the delivered CADU: %w", err)
		}
		decoded, err := tmdl.DecodeTransferFrame(recovered)
		if err != nil {
			return fmt.Errorf("decoding the delivered frame: %w", err)
		}
		fmt.Printf("    spacecraft ........ %d, VC %d, frame %d\n",
			decoded.Header.SpacecraftID,
			decoded.Header.VirtualChannelID,
			decoded.Header.MCFrameCount)
		fmt.Printf("    survived intact ... %t\n", bytes.Equal(frame.Data, expected))
	}
	fmt.Println()

	fmt.Println("--- Mission control: closing the session ---")
	fmt.Println()

	if _, err := user.Stop(now, 3); err != nil {
		return fmt.Errorf("STOP: %w", err)
	}
	if err := send(conn, user); err != nil {
		return err
	}
	if _, err := receive(conn, user, now); err != nil {
		return fmt.Errorf("STOP return: %w", err)
	}
	fmt.Printf("  STOP ........ %s\n", user.State())

	if err := user.Unbind(now, 4, sle.UnbindEnd); err != nil {
		return fmt.Errorf("UNBIND: %w", err)
	}
	if err := send(conn, user); err != nil {
		return err
	}
	if _, err := receive(conn, user, now); err != nil {
		return fmt.Errorf("UNBIND return: %w", err)
	}
	fmt.Printf("  UNBIND ...... %s\n", user.State())

	return nil
}

// groundStation is the SLE provider: it answers whatever the user asks and
// delivers one frame once started.
func groundStation(listener net.Listener, cadu []byte, now time.Time) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	association, err := sle.NewAssociation(sle.AssociationConfig{
		Role:              sle.RoleProvider,
		LocalIdentifier:   "GROUND-STN",
		PeerIdentifier:    "CTRL-CENTRE",
		HeartbeatInterval: 30,
		DeadFactor:        3,
		UserName:          "GROUND-STN",
		Password:          []byte("provider-secret"),
		PeerPassword:      []byte("user-secret"),
		AcceptableDelay:   time.Minute,
	})
	if err != nil {
		return err
	}

	provider, err := sle.NewRAFProvider(sle.ServiceConfig{
		Association:   association,
		DeliveryMode:  sle.DeliveryReturnCompleteOnline,
		Version:       5,
		ResponderPort: "GROUND-PORT",
		Instance:      serviceInstance(),
	})
	if err != nil {
		return err
	}

	random := int32(100)
	for {
		message, err := sle.ReadMessage(conn, sle.DefaultMaxMessageSize)
		if err != nil {
			return err
		}

		switch message.Type {
		case sle.MessageContext:
			// The provider takes the user's heartbeat parameters, it does not
			// propose its own.
			if err := association.HandleContextMessage(message.Body, now); err != nil {
				return err
			}
			continue
		case sle.MessageHeartbeat:
			association.RecordReceived(now)
			continue
		}

		event, err := provider.HandlePDU(message.Body, now)
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
			err := provider.HandleStartInvocation(event.StartInvocation,
				&sle.RAFStartReturn{Positive: true}, now, random)
			if err != nil {
				return err
			}
			// Answer the START first: a transfer buffer before the return
			// would arrive while the user is still in the ready state.
			if err := sendProvider(conn, provider); err != nil {
				return err
			}

			receiveTime, err := sle.NewTime(now)
			if err != nil {
				return err
			}
			buffer := sle.RAFTransferBuffer{{Frame: &sle.RAFTransferDataInvocation{
				EarthReceiveTime:      receiveTime,
				AntennaId:             sle.AntennaId{Local: []byte("DSS-25")},
				DataLinkContinuity:    -1, // this provider cannot tell
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

		if err := sendProvider(conn, provider); err != nil {
			return err
		}
	}
}

// recoveredCADU builds what a ground station would have off the antenna: a TM
// transfer frame wrapped as a CADU, randomized, with a sync marker in front.
func recoveredCADU() []byte {
	frame, err := tmdl.NewTransferFrame(spacecraftID, vcid,
		bytes.Repeat([]byte{0x42}, frameLength-6), nil, nil)
	if err != nil {
		log.Fatalf("building the frame: %v", err)
	}
	encoded, err := frame.Encode()
	if err != nil {
		log.Fatalf("encoding the frame: %v", err)
	}
	return tmsc.WrapCADU(encoded, tmsc.DefaultASM(), true)
}

// send writes every PDU the user machine has queued, each in its own TML
// message.
func send(conn net.Conn, user *sle.RAFUser) error {
	for {
		pdu, ok := user.NextPDU()
		if !ok {
			return nil
		}
		if err := sle.WriteMessage(conn,
			&sle.Message{Type: sle.MessageSLEPDU, Body: pdu}); err != nil {
			return fmt.Errorf("writing a PDU: %w", err)
		}
	}
}

// sendProvider is the same thing for the provider machine.
func sendProvider(conn net.Conn, provider *sle.RAFProvider) error {
	for {
		pdu, ok := provider.NextPDU()
		if !ok {
			return nil
		}
		if err := sle.WriteMessage(conn,
			&sle.Message{Type: sle.MessageSLEPDU, Body: pdu}); err != nil {
			return err
		}
	}
}

// receive reads one TML message and feeds the PDU inside it to the user
// machine. Heartbeats are answered here and never reach the state machine.
func receive(conn net.Conn, user *sle.RAFUser, now time.Time) (*sle.RAFUserEvent, error) {
	for {
		message, err := sle.ReadMessage(conn, sle.DefaultMaxMessageSize)
		if err != nil {
			return nil, err
		}
		if message.Type == sle.MessageHeartbeat {
			user.Association().RecordReceived(now)
			continue
		}
		return user.HandlePDU(message.Body, now)
	}
}

// closed reports whether an error is just the far end hanging up.
func closed(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed)
}
