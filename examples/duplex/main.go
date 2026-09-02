// Example: A full-duplex link, with the CLCW riding home
//
// The downlink example sends telemetry. The uplink example sends commands.
// Neither is a whole link, because the two are joined: FOP-1 on the ground
// will not send past its sliding window until a CLCW comes back saying what
// the spacecraft accepted, and that CLCW travels in the Operational Control
// Field of a telemetry frame.
//
// So this example closes the loop:
//
//	Ground                                       Spacecraft
//	  commands ──► FOP-1 ──► CLTU ──────────────► FARM-1 ──► commands
//	                                                 │
//	                                                 ▼
//	  FOP-1 ◄──── CLCW ◄──── OCF of a TM frame ◄── CLCW generated
//	     │
//	     └─ the window moves, so more commands can go
//
// The whole point is the third arrow. Without it the ground sends its window's
// worth of commands and then stops, and nothing is wrong with the link.
package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/stack"
	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/ravisuhag/astro/pkg/tcsc"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

const (
	spacecraftID = 42
	vcidUplink   = 0
	vcidDownlink = 0
	mapID        = 1

	apidCommand = 100
	apidTelem   = 200

	// A small window, so it fills within the example. A real mission uses
	// something like 10.
	copWindow = 2
)

// The downlink carries the CLCW, so HasOCF is true. That is the one setting
// the separate downlink example turns off, and the reason the two examples do
// not compose.
var downlinkConfig = tmdl.ChannelConfig{
	FrameLength: 256,
	HasOCF:      true,
	HasFEC:      true,
}

func main() {
	link := newLink()

	fmt.Println("--- Round 1: fill the window ---")
	fmt.Println()

	// The window is 2, so the third command has nowhere to go until a CLCW
	// comes home.
	for i, command := range []string{"SET MODE 3", "POINT 12.5 -3.1", "START SCAN"} {
		link.queueCommand(uint16(i), command)
	}
	link.uplinkPass()
	fmt.Println()

	fmt.Println("--- The spacecraft's view ---")
	fmt.Println()
	link.showOnboard()
	fmt.Println()

	fmt.Println("--- Downlink pass: telemetry with the CLCW in the OCF ---")
	fmt.Println()
	link.downlinkPass()
	fmt.Println()

	fmt.Println("--- Round 2: the window has moved ---")
	fmt.Println()
	link.uplinkPass()
	fmt.Println()

	fmt.Println("--- What happens with no downlink ---")
	fmt.Println()
	noDownlink()
	fmt.Println()

	fmt.Println("--- The same loop through pkg/stack ---")
	fmt.Println()
	throughTheComposer()
}

// throughTheComposer is the same closed loop built by pkg/stack instead of by
// hand. The composer takes the operational control field supplier as a
// construction option, so the whole join is one closure.
func throughTheComposer() {
	uplinkConfig := stack.Uplink{
		SpacecraftID: spacecraftID,
		Channels:     []stack.UplinkVC{{ID: vcidUplink, Window: copWindow}},
	}

	commander, err := stack.NewCommander(uplinkConfig)
	if err != nil {
		log.Fatalf("building the commander: %v", err)
	}
	onboard, err := stack.NewOnboard(uplinkConfig)
	if err != nil {
		log.Fatalf("building the onboard side: %v", err)
	}

	downlinkConfig := stack.Downlink{
		SpacecraftID: spacecraftID,
		FrameLength:  256,
		FECF:         true,
		OCF:          true, // reserve the field...
		Channels:     []stack.VC{{ID: vcidDownlink, Priority: 1}},
	}

	// ...and say what goes in it. NewSender refuses the configuration
	// without this, because four zero octets decode as a valid CLCW
	// reporting V(R)=0 and the ground would believe it.
	sender, err := stack.NewSender(downlinkConfig, stack.WithOCF(func() []byte {
		field, err := onboard.CLCW(vcidUplink)
		if err != nil {
			return nil // a nil field fails the frame rather than faking one
		}
		return field
	}))
	if err != nil {
		log.Fatalf("building the sender: %v", err)
	}
	receiver, err := stack.NewReceiver(downlinkConfig)
	if err != nil {
		log.Fatalf("building the receiver: %v", err)
	}

	// What the composer refuses, for the same reason.
	if _, err := stack.NewSender(downlinkConfig); err != nil {
		fmt.Printf("  without a supplier: %v\n", err)
	}
	fmt.Println()

	commands := []string{"SET MODE 3", "POINT 12.5 -3.1", "START SCAN"}
	uplinked := 0

	for i, text := range commands {
		packet, err := spp.NewTCPacket(apidCommand, []byte(text),
			spp.WithSequenceCount(uint16(i)))
		if err != nil {
			log.Fatalf("building a telecommand: %v", err)
		}
		if err := commander.SendPacket(vcidUplink, packet); err != nil {
			log.Fatalf("queueing a telecommand: %v", err)
		}

		// Whatever FOP-1 will release, which stops at the window.
		for cltu, err := range commander.CLTUs() {
			if err != nil {
				log.Fatalf("building a CLTU: %v", err)
			}
			accepted, err := onboard.Accept(cltu)
			if err != nil {
				log.Fatalf("accepting a CLTU: %v", err)
			}
			if accepted {
				uplinked++
			}
		}

		// One telemetry frame down, carrying the CLCW in the OCF.
		telemetry, err := spp.NewTMPacket(apidTelem, []byte("BATT 28.1V"))
		if err != nil {
			log.Fatalf("building telemetry: %v", err)
		}
		if err := sender.SendPacket(vcidDownlink, telemetry); err != nil {
			log.Fatalf("sending telemetry: %v", err)
		}
		if err := sender.Flush(); err != nil {
			log.Fatalf("flushing: %v", err)
		}
		for cadu, err := range sender.CADUs() {
			if err != nil {
				log.Fatalf("building a CADU: %v", err)
			}
			if err := receiver.Accept(cadu); err != nil {
				log.Fatalf("accepting a CADU: %v", err)
			}
		}

		// Feed every CLCW that came home back to FOP-1. This is what moves
		// the window, and without it the loop stops after copWindow commands.
		fields := 0
		for field := range receiver.OCFs() {
			if err := commander.AcceptCLCW(field); err != nil {
				log.Fatalf("accepting a CLCW: %v", err)
			}
			fields++
		}

		pending, err := commander.Pending(vcidUplink)
		if err != nil {
			log.Fatalf("reading the pending count: %v", err)
		}
		fmt.Printf("  round %d: %q uplinked, %d CLCW(s) home, %d outstanding\n",
			i, text, fields, pending)
	}

	fmt.Println()
	fmt.Printf("  commands accepted ... %d of %d\n", uplinked, len(commands))

	var recovered []string
	for encoded, err := range onboard.Packets(vcidUplink) {
		if err != nil {
			log.Fatalf("reading a command: %v", err)
		}
		decoded, err := spp.Decode(encoded)
		if err != nil {
			log.Fatalf("decoding a command: %v", err)
		}
		recovered = append(recovered, string(decoded.UserData))
	}
	fmt.Printf("  recovered ........... %q\n", recovered)
}

// link holds both ends of both directions. In reality these are two programs
// in two places; putting them in one struct is what makes the example
// readable.
type link struct {
	// Ground, uplink side.
	fop      *cop.FOP
	uplinkVC *tcdl.VirtualChannel
	mapSvc   *tcdl.MAPPacketService

	// Spacecraft, uplink side.
	farm       *cop.FARM
	onboardVC  *tcdl.VirtualChannel
	onboardMAP *tcdl.MAPPacketService

	// Spacecraft, downlink side.
	downlinkVC  *tmdl.VirtualChannel
	downlinkSvc *tmdl.VirtualChannelPacketService

	// Ground, downlink side.
	groundVC  *tmdl.VirtualChannel
	groundSvc *tmdl.VirtualChannelPacketService

	commandsAccepted []string
	pendingCommands  []string
}

func newLink() *link {
	l := &link{}

	// Ground uplink: a MAP packet service feeding FOP-1.
	l.uplinkVC = tcdl.NewVirtualChannel(vcidUplink, 32)
	l.mapSvc = tcdl.NewMAPPacketService(
		spacecraftID, vcidUplink, mapID, false, // bypass=false: Type-A, reliable
		l.uplinkVC, tcdl.NewFrameCounter())
	l.mapSvc.SetPacketSizer(spp.PacketSizer)

	l.fop = cop.NewFOP(spacecraftID, vcidUplink, copWindow)
	l.fop.Initialize(0)

	// Spacecraft uplink: FARM-1 decides what to accept, and a MAP service
	// pulls the packets back out of the frames it accepted. Its window must
	// match the ground's.
	l.farm = cop.NewFARM(vcidUplink, copWindow)
	l.onboardVC = tcdl.NewVirtualChannel(vcidUplink, 32)
	l.onboardMAP = tcdl.NewMAPPacketService(
		spacecraftID, vcidUplink, mapID, false, l.onboardVC, nil)
	l.onboardMAP.SetPacketSizer(spp.PacketSizer)

	// Spacecraft downlink. The OCF supplier is the join between the two
	// directions: every frame this service emits asks FARM-1 for the current
	// CLCW and puts it in the frame.
	l.downlinkVC = tmdl.NewVirtualChannel(vcidDownlink, 32)
	l.downlinkSvc = tmdl.NewVirtualChannelPacketService(
		spacecraftID, vcidDownlink, l.downlinkVC, downlinkConfig, tmdl.NewFrameCounter())
	l.downlinkSvc.SetPacketSizer(spp.PacketSizer)
	l.downlinkSvc.SetOCFSupplier(func() []byte {
		clcw := l.farm.GenerateCLCW()
		encoded, err := clcw.Encode()
		if err != nil {
			log.Fatalf("encoding the CLCW: %v", err)
		}
		return encoded
	})

	// Ground downlink.
	l.groundVC = tmdl.NewVirtualChannel(vcidDownlink, 32)
	l.groundSvc = tmdl.NewVirtualChannelPacketService(
		spacecraftID, vcidDownlink, l.groundVC, downlinkConfig, nil)
	l.groundSvc.SetPacketSizer(spp.PacketSizer)

	return l
}

// queueCommand builds a telecommand and hands it to the MAP service, which
// packs it into a TC frame. Nothing has been transmitted yet.
func (l *link) queueCommand(seq uint16, command string) {
	packet, err := spp.NewTCPacket(apidCommand, []byte(command),
		spp.WithSequenceCount(seq), spp.WithErrorControl())
	if err != nil {
		log.Fatalf("building the telecommand: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		log.Fatalf("encoding the telecommand: %v", err)
	}
	if err := l.mapSvc.Send(encoded); err != nil {
		log.Fatalf("queueing the telecommand: %v", err)
	}
	l.pendingCommands = append(l.pendingCommands, command)
}

// uplinkPass offers every queued frame to FOP-1 and transmits what it
// releases. FOP-1 refusing a frame is the sliding window doing its job.
func (l *link) uplinkPass() {
	for l.uplinkVC.Len() > 0 {
		frame, err := l.uplinkVC.Next()
		if err != nil {
			break
		}
		encoded, err := frame.Encode()
		if err != nil {
			log.Fatalf("encoding the frame: %v", err)
		}

		// FOP-1 assigns N(S) and remembers the frame until a CLCW
		// acknowledges it.
		if err := l.fop.TransmitFrame(encoded); err != nil {
			fmt.Printf("  held back: %v\n", err)
			fmt.Printf("  %d frame(s) outstanding, the window is %d\n",
				l.fop.PendingCount(), copWindow)
			// Put it back. The caller holds the backlog, because someone
			// offering a command wants it queued, not dropped: the window is
			// a transmission constraint, not a limit on the pass.
			if err := l.uplinkVC.Add(frame); err != nil {
				log.Fatalf("re-queueing the frame: %v", err)
			}
			break
		}
		fmt.Printf("  queued in FOP-1, %d outstanding\n", l.fop.PendingCount())
	}

	// Everything FOP-1 will release, wrapped as CLTUs and flown up.
	for {
		frame, ns, ok := l.fop.GetNextFrame()
		if !ok {
			break
		}
		cltu, err := tcsc.WrapCLTU(frame, nil, nil, true)
		if err != nil {
			log.Fatalf("wrapping the CLTU: %v", err)
		}
		fmt.Printf("  transmitted N(S)=%d as a %d octet CLTU\n", ns, len(cltu))
		l.onboardReceive(cltu)
	}
}

// onboardReceive is the spacecraft: unwrap, decode, let FARM-1 decide.
func (l *link) onboardReceive(cltu []byte) {
	frameBytes, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, true)
	if err != nil {
		log.Fatalf("unwrapping the CLTU: %v", err)
	}

	frame, err := tcdl.DecodeTCTransferFrameWithSegmentHeader(frameBytes)
	if err != nil {
		log.Fatalf("decoding the TC frame: %v", err)
	}

	accepted, err := l.farm.ProcessFrame(
		frame.Header.BypassFlag,
		frame.Header.ControlCommandFlag,
		frame.Header.FrameSequenceNum,
		frame.DataField)
	if err != nil {
		log.Fatalf("FARM-1: %v", err)
	}
	if !accepted {
		fmt.Printf("    FARM-1 rejected N(S)=%d\n", frame.Header.FrameSequenceNum)
		return
	}

	// FARM-1 said yes, so the frame goes to the MAP service, which
	// reassembles the packet and hands it up.
	if err := l.onboardVC.Add(frame); err != nil {
		log.Fatalf("queueing the accepted frame: %v", err)
	}
	encoded, err := l.onboardMAP.Receive()
	if err != nil {
		log.Fatalf("reassembling the telecommand: %v", err)
	}
	packet, err := spp.Decode(encoded, spp.WithDecodeErrorControl())
	if err != nil {
		log.Fatalf("decoding the telecommand: %v", err)
	}

	command := string(packet.UserData)
	l.commandsAccepted = append(l.commandsAccepted, command)
	fmt.Printf("    FARM-1 accepted N(S)=%d: %q\n",
		frame.Header.FrameSequenceNum, command)
}

func (l *link) showOnboard() {
	fmt.Printf("  FARM-1 state ........ %s\n", l.farm.State())
	fmt.Printf("  commands accepted ... %d of %d queued\n",
		len(l.commandsAccepted), len(l.pendingCommands))
	for _, command := range l.commandsAccepted {
		fmt.Printf("    %q\n", command)
	}

	clcw := l.farm.GenerateCLCW()
	fmt.Printf("  CLCW it will report .. V(R)=%d, retransmit=%t, lockout=%t\n",
		clcw.ReportValue, clcw.RetransmitFlag, clcw.LockoutFlag)
}

// downlinkPass sends one telemetry packet down. The CLCW rides in the OCF of
// whatever frame that produces, whether or not the spacecraft had anything
// else to say.
func (l *link) downlinkPass() {
	packet, err := spp.NewTMPacket(apidTelem, []byte("BATT 28.1V MODE 3"),
		spp.WithErrorControl())
	if err != nil {
		log.Fatalf("building the telemetry packet: %v", err)
	}
	encoded, err := packet.Encode()
	if err != nil {
		log.Fatalf("encoding the telemetry packet: %v", err)
	}

	if err := l.downlinkSvc.Send(encoded); err != nil {
		log.Fatalf("sending telemetry: %v", err)
	}
	if err := l.downlinkSvc.Flush(); err != nil {
		log.Fatalf("flushing the downlink: %v", err)
	}

	for l.downlinkVC.Len() > 0 {
		frame, err := l.downlinkVC.Next()
		if err != nil {
			break
		}
		frameBytes, err := frame.EncodeWithConfig(downlinkConfig)
		if err != nil {
			log.Fatalf("encoding the TM frame: %v", err)
		}
		cadu := tmsc.WrapCADU(frameBytes, tmsc.DefaultASM(), true)
		fmt.Printf("  transmitted a %d octet CADU\n", len(cadu))

		l.groundReceive(cadu)
	}
}

// groundReceive is the whole point of the example: pull the CLCW out of the
// OCF and give it to FOP-1.
func (l *link) groundReceive(cadu []byte) {
	frameBytes, err := tmsc.UnwrapCADU(cadu, tmsc.DefaultASM(), true)
	if err != nil {
		log.Fatalf("unwrapping the CADU: %v", err)
	}

	frame, err := tmdl.DecodeTMTransferFrameWithConfig(frameBytes, downlinkConfig)
	if err != nil {
		log.Fatalf("decoding the TM frame: %v", err)
	}

	if err := l.groundVC.Add(frame); err != nil {
		log.Fatalf("queueing the frame: %v", err)
	}
	if telemetry, err := l.groundSvc.Receive(); err == nil {
		fmt.Printf("  telemetry ..... %q\n", packetPayload(telemetry))
	}

	if len(frame.OperationalControl) != 4 {
		fmt.Println("  no OCF in the frame: the downlink config has HasOCF false")
		return
	}

	var clcw cop.CLCW
	if err := clcw.Decode(frame.OperationalControl); err != nil {
		log.Fatalf("decoding the CLCW: %v", err)
	}
	fmt.Printf("  CLCW .......... V(R)=%d, retransmit=%t, lockout=%t\n",
		clcw.ReportValue, clcw.RetransmitFlag, clcw.LockoutFlag)

	before := l.fop.PendingCount()
	if err := l.fop.ProcessCLCW(&clcw); err != nil {
		log.Fatalf("FOP-1 processing the CLCW: %v", err)
	}
	fmt.Printf("  FOP-1 ......... %d outstanding, was %d\n",
		l.fop.PendingCount(), before)
}

// noDownlink is the failure the T1 timer exists to catch. FOP-1 fills its
// window, no CLCW comes back, and nothing at all is reported as wrong.
func noDownlink() {
	fop := cop.NewFOP(spacecraftID, vcidUplink, copWindow)
	fop.Initialize(0)

	uplinkVC := tcdl.NewVirtualChannel(vcidUplink, 32)
	mapSvc := tcdl.NewMAPPacketService(
		spacecraftID, vcidUplink, mapID, false, uplinkVC, tcdl.NewFrameCounter())
	mapSvc.SetPacketSizer(spp.PacketSizer)

	sent, refused := 0, 0
	for i := range 6 {
		packet, err := spp.NewTCPacket(apidCommand, []byte("NOOP"),
			spp.WithSequenceCount(uint16(i)))
		if err != nil {
			log.Fatalf("building a telecommand: %v", err)
		}
		encoded, err := packet.Encode()
		if err != nil {
			log.Fatalf("encoding a telecommand: %v", err)
		}
		if err := mapSvc.Send(encoded); err != nil {
			log.Fatalf("queueing a telecommand: %v", err)
		}

		frame, err := uplinkVC.Next()
		if err != nil {
			continue
		}
		frameBytes, err := frame.Encode()
		if err != nil {
			log.Fatalf("encoding a frame: %v", err)
		}

		if err := fop.TransmitFrame(frameBytes); err != nil {
			refused++
			continue
		}
		sent++
	}

	fmt.Printf("  6 commands offered, no CLCW ever arrives\n")
	fmt.Printf("  accepted by FOP-1 ... %d (the window is %d)\n", sent, copWindow)
	fmt.Printf("  refused ............. %d\n", refused)
	fmt.Printf("  FOP-1 state ......... %s\n", fop.State())
	fmt.Println()
	fmt.Println("  The link is fine. The spacecraft is fine. Commanding has")
	fmt.Println("  simply stopped, and the only thing that will notice is the")
	fmt.Println("  T1 timer, which does not run itself: call Tick on your own")
	fmt.Println("  schedule or a single lost CLCW stalls the uplink for good.")
}

// packetPayload decodes a Space Packet and returns its user data.
func packetPayload(encoded []byte) string {
	packet, err := spp.Decode(encoded, spp.WithDecodeErrorControl())
	if err != nil {
		return ""
	}
	return string(bytes.TrimRight(packet.UserData, "\x00"))
}
