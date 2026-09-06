package stack

import (
	"errors"
	"fmt"

	"github.com/ravisuhag/astro/pkg/cop"
	"github.com/ravisuhag/astro/pkg/sdl"
	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tcdl"
	"github.com/ravisuhag/astro/pkg/tcsc"
)

// The uplink, and why it is not the downlink backwards.
//
// A downlink is a stream: the spacecraft sends, the ground receives, and
// nothing the ground does changes what arrives. One configuration builds both
// ends and each end runs on its own.
//
// Commanding is a conversation. COP-1 (CCSDS 232.1-B-2) gives the ground a
// sequence-controlled service that guarantees delivery in order, and it pays
// for that with feedback: FOP-1 on the ground will not send past its sliding
// window until FARM-1 on the spacecraft says, in a CLCW carried back on the
// telemetry link, what it has accepted. A commander that cannot take CLCWs in
// is not doing sequence control, whatever else it is doing.
//
// So the two ends here are asymmetric in a way the downlink's are not. The
// ground side sends packets and accepts CLCWs; the spacecraft side accepts
// frames and produces CLCWs. Both are still built from one Uplink value, for
// the same reason as before: the frame layout and the randomisation have to
// agree, and nothing else checks that they do.
//
// The two services are both here because a mission needs both. Sequence
// controlled (type AD) is for commands that must arrive exactly once and in
// order. Expedited (type BD) bypasses the sequence check entirely, which is
// what you use when the sequence machinery is what is broken. An unlock
// after a lockout, say.

// MaxUplinkVCID is the largest virtual channel identifier a TC frame can
// carry. The field is six bits (CCSDS 232.0-B-4 clause 4.1.2.4).
const MaxUplinkVCID = 63

// UplinkVC is one virtual channel on the uplink.
type UplinkVC struct {
	// ID is the virtual channel identifier, 0 to MaxUplinkVCID.
	ID uint8

	// MAPID identifies the Multiplexer Access Point within the channel, 0 to
	// 63. A mission that does not use MAP multiplexing leaves it zero.
	MAPID uint8

	// Window is the COP-1 sliding window width: how many frames may be
	// outstanding before the ground has to wait for a CLCW. Zero takes
	// DefaultWindow.
	Window uint8

	// Buffer is how many frames the channel holds. Zero takes DefaultBuffer.
	Buffer int
}

// DefaultWindow is the COP-1 sliding window width when a channel does not set
// one.
//
// CCSDS 232.1-B-2 makes the width a managed parameter with no default, so this
// is a working value rather than a standard one. It is well under the 127 the
// eight-bit sequence number allows, which is what keeps a lost CLCW from
// looking like a wrap.
const DefaultWindow = 10

// Uplink is the configuration of one command uplink.
//
// As with Downlink, the same value configures both ends.
type Uplink struct {
	// SpacecraftID is the 10-bit identifier both ends expect
	// (CCSDS 232.0-B-4 clause 4.1.2.3).
	SpacecraftID uint16

	// Randomize applies the CCSDS pseudo-randomiser to each codeblock before
	// the CLTU is assembled. Both ends must agree, and they do here.
	Randomize bool

	// Channels are the virtual channels, in any order. At least one is
	// required and their IDs must be distinct.
	Channels []UplinkVC
}

// Validate reports whether the configuration can be built.
func (u Uplink) Validate() error {
	if len(u.Channels) == 0 {
		return fmt.Errorf("%w: at least one virtual channel is required", ErrInvalidConfig)
	}

	seen := make(map[uint8]bool, len(u.Channels))
	for _, channel := range u.Channels {
		if channel.ID > MaxUplinkVCID {
			return fmt.Errorf("%w: virtual channel %d is above the 6-bit maximum of %d",
				ErrInvalidConfig, channel.ID, MaxUplinkVCID)
		}
		if seen[channel.ID] {
			return fmt.Errorf("%w: virtual channel %d is configured twice",
				ErrInvalidConfig, channel.ID)
		}
		seen[channel.ID] = true

		if channel.MAPID > 63 {
			return fmt.Errorf("%w: virtual channel %d has MAP ID %d, above the 6-bit maximum",
				ErrInvalidConfig, channel.ID, channel.MAPID)
		}
		if channel.Buffer < 0 {
			return fmt.Errorf("%w: virtual channel %d has a negative buffer",
				ErrInvalidConfig, channel.ID)
		}
		if channel.Window > cop.MaxWindow {
			return fmt.Errorf("%w: virtual channel %d has window %d, above the COP-1 maximum of %d",
				ErrInvalidConfig, channel.ID, channel.Window, cop.MaxWindow)
		}
	}
	return nil
}

// window returns a channel's sliding window width, applying the default.
func (v UplinkVC) window() uint8 {
	if v.Window == 0 {
		return DefaultWindow
	}
	return v.Window
}

// buffer returns a channel's frame buffer, applying the default.
func (v UplinkVC) buffer() int {
	if v.Buffer == 0 {
		return DefaultBuffer
	}
	return v.Buffer
}

// Commander is the ground side of an uplink: packets in, CLTUs out, CLCWs
// back in.
//
// It is not safe for concurrent use. COP-1 is a state machine per virtual
// channel, and the order commands are offered in is the order they will be
// delivered, so serialising access is the caller's job.
type Commander struct {
	config Uplink

	// Each channel gets two packet services, because the bypass flag is
	// stamped into the frame header at construction and not chosen at
	// transmission. A frame built by the sequenced service says type AD and
	// one built by the expedited service says type BD, and FARM-1 reads the
	// header, so offering an AD-shaped frame through FOP-1's BD path gets it
	// rejected on arrival, which is what an earlier draft of this did.
	sequenced map[uint8]*channelService
	expedited map[uint8]*channelService

	fops map[uint8]*cop.FOP

	// backlog holds encoded frames FOP-1 has not taken yet.
	//
	// FOP-1 refuses a frame once its sliding window is full, because that is
	// what sequence control means. But a caller offering a command wants it
	// queued, not refused. The window is a transmission constraint, not a
	// limit on how much a pass may hold. So the frames wait here and are
	// offered again whenever the window might have moved.
	backlog map[uint8][]pendingFrame
}

// pendingFrame is one encoded frame waiting for FOP-1 to have room, and
// whether it bypasses the sequence check.
type pendingFrame struct {
	data      []byte
	expedited bool
}

// channelService pairs a packet service with the frame buffer it fills.
type channelService struct {
	service *tcdl.MAPPacketService
	frames  *tcdl.VirtualChannel
}

// newChannelService builds a packet service and its buffer for one delivery
// mode.
func newChannelService(config Uplink, channel UplinkVC, bypass bool) *channelService {
	frames := tcdl.NewVirtualChannel(channel.ID, channel.buffer())

	service := tcdl.NewMAPPacketService(
		config.SpacecraftID, channel.ID, channel.MAPID, bypass, frames, tcdl.NewFrameCounter())
	service.SetPacketSizer(spp.PacketSizer)

	return &channelService{service: service, frames: frames}
}

// NewCommander builds the ground side of an uplink.
//
// Every channel's FOP-1 starts initialised at sequence number zero, which is
// the state after a successful unlock. A mission resuming a pass mid-sequence
// should set it with SetV(S).
func NewCommander(config Uplink) (*Commander, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	commander := &Commander{
		config:    config,
		sequenced: make(map[uint8]*channelService, len(config.Channels)),
		expedited: make(map[uint8]*channelService, len(config.Channels)),
		fops:      make(map[uint8]*cop.FOP, len(config.Channels)),
		backlog:   make(map[uint8][]pendingFrame, len(config.Channels)),
	}

	for _, channel := range config.Channels {
		commander.sequenced[channel.ID] = newChannelService(config, channel, false)
		commander.expedited[channel.ID] = newChannelService(config, channel, true)

		fop := cop.NewFOP(config.SpacecraftID, channel.ID, channel.window())
		// Sequence number zero, which is where an unlocked channel starts.
		fop.Initialize(0)
		commander.fops[channel.ID] = fop
	}

	return commander, nil
}

// channel returns the delivery-mode service and the FOP for a virtual
// channel.
func (c *Commander) channel(vcid uint8, expedited bool) (*channelService, *cop.FOP, error) {
	services := c.sequenced
	if expedited {
		services = c.expedited
	}

	service, ok := services[vcid]
	if !ok {
		return nil, nil, fmt.Errorf(
			"%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}
	return service, c.fops[vcid], nil
}

// Send offers one encoded Space Packet for sequence-controlled delivery.
//
// The packet is segmented into TC frames and each frame handed to FOP-1,
// which will not release it past the sliding window until a CLCW says the
// spacecraft has room. So Send succeeding means the command is queued, not
// that it is on its way.
func (c *Commander) Send(vcid uint8, packet []byte) error {
	service, fop, err := c.channel(vcid, false)
	if err != nil {
		return err
	}

	if err := service.service.Send(packet); err != nil {
		return fmt.Errorf("segmenting the packet: %w", err)
	}
	if err := c.enqueue(vcid, service.frames, false); err != nil {
		return err
	}
	return c.pump(vcid, fop)
}

// SendExpedited offers a packet for expedited delivery, bypassing the
// sequence check.
//
// Type BD frames are not counted, not retransmitted and not acknowledged.
// They are what you use when the sequence machinery is the thing that is
// broken, and they arrive whatever state FOP-1 is in.
func (c *Commander) SendExpedited(vcid uint8, packet []byte) error {
	service, fop, err := c.channel(vcid, true)
	if err != nil {
		return err
	}

	if err := service.service.Send(packet); err != nil {
		return fmt.Errorf("segmenting the packet: %w", err)
	}
	if err := c.enqueue(vcid, service.frames, true); err != nil {
		return err
	}
	return c.pump(vcid, fop)
}

// SendPacket encodes a Space Packet and sends it for sequence-controlled
// delivery.
func (c *Commander) SendPacket(vcid uint8, packet *spp.SpacePacket) error {
	encoded, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("encoding packet: %w", err)
	}
	return c.Send(vcid, encoded)
}

// enqueue takes the frames the packet service just built and puts them on the
// channel's backlog.
func (c *Commander) enqueue(vcid uint8, virtual *tcdl.VirtualChannel, expedited bool) error {
	for virtual.HasFrames() {
		frame, err := virtual.Next()
		if err != nil {
			return fmt.Errorf("taking a built frame: %w", err)
		}

		encoded, err := frame.Encode()
		if err != nil {
			return fmt.Errorf("encoding frame: %w", err)
		}
		c.backlog[vcid] = append(c.backlog[vcid], pendingFrame{data: encoded, expedited: expedited})
	}
	return nil
}

// pump offers as much of a channel's backlog to FOP-1 as it will take.
//
// It stops at the first frame FOP-1 refuses for want of window, leaving that
// frame and everything after it queued in order. Any other refusal is a real
// error: the channel is in a state that needs the operator, not more frames.
func (c *Commander) pump(vcid uint8, fop *cop.FOP) error {
	queue := c.backlog[vcid]

	for len(queue) > 0 {
		next := queue[0]

		var err error
		if next.expedited {
			err = fop.TransmitBDFrame(next.data)
		} else {
			err = fop.TransmitFrame(next.data)
		}

		if errors.Is(err, cop.ErrFOPWindowFull) {
			break
		}
		if err != nil {
			c.backlog[vcid] = queue
			return fmt.Errorf("offering a frame on channel %d: %w", vcid, err)
		}
		queue = queue[1:]
	}

	c.backlog[vcid] = queue
	return nil
}

// NextCLTU returns the next CLTU to transmit, or false when nothing is ready.
//
// Nothing ready does not mean nothing queued: FOP-1 holds frames back when
// the sliding window is full or the channel is waiting, and only a CLCW will
// release them. That is sequence control working, not a fault.
func (c *Commander) NextCLTU() ([]byte, bool, error) {
	// In configuration order, so a drain is repeatable.
	for _, channel := range c.config.Channels {
		// Offer anything the backlog is holding first: a CLCW since the last
		// call may have moved the window.
		if err := c.pump(channel.ID, c.fops[channel.ID]); err != nil {
			return nil, false, err
		}

		frame, _, ok := c.fops[channel.ID].GetNextFrame()
		if !ok {
			continue
		}

		cltu, err := tcsc.WrapCLTU(frame, nil, nil, c.config.Randomize)
		if err != nil {
			return nil, false, fmt.Errorf("wrapping the CLTU: %w", err)
		}
		return cltu, true, nil
	}
	return nil, false, nil
}

// CLTUs iterates the CLTUs ready to transmit.
func (c *Commander) CLTUs() func(yield func([]byte, error) bool) {
	return func(yield func([]byte, error) bool) {
		for {
			cltu, ok, err := c.NextCLTU()
			if err != nil {
				yield(nil, err)
				return
			}
			if !ok {
				return
			}
			if !yield(cltu, nil) {
				return
			}
		}
	}
}

// AcceptCLCW feeds back one Communications Link Control Word from the
// telemetry link.
//
// This is what closes the loop. Without it a sequence-controlled channel
// stops at its sliding window and stays there.
func (c *Commander) AcceptCLCW(encoded []byte) error {
	var clcw cop.CLCW
	if err := clcw.Decode(encoded); err != nil {
		return fmt.Errorf("decoding the CLCW: %w", err)
	}

	_, fop, err := c.channel(clcw.VirtualChannelID, false)
	if err != nil {
		// A CLCW for a channel this commander does not drive is not an
		// error in itself (a spacecraft reports on channels the ground may
		// not be commanding) but it cannot be acted on either.
		return err
	}
	if err := fop.ProcessCLCW(&clcw); err != nil {
		return fmt.Errorf("processing the CLCW for channel %d: %w", clcw.VirtualChannelID, err)
	}

	// The window may have moved, so offer what was waiting on it.
	return c.pump(clcw.VirtualChannelID, fop)
}

// State reports what COP-1 is doing on a channel, which is the thing to look
// at when commands stop going out.
func (c *Commander) State(vcid uint8) (cop.FOPState, error) {
	_, fop, err := c.channel(vcid, false)
	if err != nil {
		return 0, err
	}
	return fop.State(), nil
}

// Pending reports how many frames a channel is holding: those FOP-1 has sent
// and is waiting to have acknowledged, plus those still queued behind the
// sliding window.
func (c *Commander) Pending(vcid uint8) (int, error) {
	_, fop, err := c.channel(vcid, false)
	if err != nil {
		return 0, err
	}
	return fop.PendingCount() + len(c.backlog[vcid]), nil
}

// Onboard is the spacecraft side of an uplink: CLTUs in, packets out, CLCWs
// to send back.
//
// Not safe for concurrent use, for the same reason as Commander.
type Onboard struct {
	config Uplink

	// master routes an accepted frame to its virtual channel, which is where
	// the packet service reads it from.
	master   *tcdl.MasterChannel
	services map[uint8]*tcdl.MAPPacketService
	farms    map[uint8]*cop.FARM

	// channels gives Next a way to see how many frames a virtual channel is
	// holding, so it can tell how many FARM buffers a call to Receive just
	// freed. Both services and channels are keyed by VCID and built from
	// the same *tcdl.VirtualChannel; this just keeps a handle to it.
	channels map[uint8]*tcdl.VirtualChannel
}

// NewOnboard builds the spacecraft side of an uplink.
//
// Give it the same Uplink value the commander was built from.
func NewOnboard(config Uplink) (*Onboard, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	onboard := &Onboard{
		config:   config,
		master:   tcdl.NewMasterChannel(config.SpacecraftID),
		services: make(map[uint8]*tcdl.MAPPacketService, len(config.Channels)),
		farms:    make(map[uint8]*cop.FARM, len(config.Channels)),
		channels: make(map[uint8]*tcdl.VirtualChannel, len(config.Channels)),
	}

	for _, channel := range config.Channels {
		virtual := tcdl.NewVirtualChannel(channel.ID, channel.buffer())
		onboard.master.AddVirtualChannel(virtual, 1)
		onboard.channels[channel.ID] = virtual
		counter := tcdl.NewFrameCounter()

		service := tcdl.NewMAPPacketService(
			config.SpacecraftID, channel.ID, channel.MAPID, false, virtual, counter)
		service.SetPacketSizer(spp.PacketSizer)
		onboard.services[channel.ID] = service

		farm := cop.NewFARM(channel.ID, channel.window())
		// The FARM's buffer count mirrors the virtual channel's frame
		// buffer: a frame FARM-1 accepts must have somewhere to go, or
		// V(R) advances past data that was dropped and the loss is
		// invisible to the ground (the Wait flag exists for exactly this).
		farm.SetBuffers(channel.buffer())
		onboard.farms[channel.ID] = farm
	}

	return onboard, nil
}

// Accept takes one received CLTU: it unwraps the codeblocks, decodes the
// frame, runs it past FARM-1, and hands the accepted data to the channel.
//
// A frame FARM-1 rejects is not an error. A retransmission of something
// already accepted, or a frame outside the window, is exactly what the
// procedure exists to filter, and reporting it as a failure would make
// ordinary operation look broken. accepted says which happened.
func (o *Onboard) Accept(cltu []byte) (accepted bool, err error) {
	frameData, _, err := tcsc.UnwrapCLTU(cltu, nil, nil, o.config.Randomize)
	if err != nil {
		return false, fmt.Errorf("unwrapping the CLTU: %w", err)
	}

	frame, err := tcdl.DecodeTCTransferFrameWithSegmentHeader(frameData)
	if err != nil {
		return false, fmt.Errorf("decoding the frame: %w", err)
	}

	vcid := frame.Header.VirtualChannelID
	farm, ok := o.farms[vcid]
	if !ok {
		return false, fmt.Errorf("%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}

	// FARM-1 decides whether this frame counts. A rejection carries an error
	// explaining which rule it broke, which is information rather than a
	// failure, so it is not propagated.
	accepted, _ = farm.ProcessFrame(
		frame.Header.BypassFlag,
		frame.Header.ControlCommandFlag,
		frame.Header.FrameSequenceNum,
		frame.DataField,
	)
	if !accepted {
		return false, nil
	}

	if err := o.master.AddFrame(frame); err != nil {
		return false, fmt.Errorf("routing the frame on channel %d: %w", vcid, err)
	}
	return true, nil
}

// Next returns the next whole Space Packet from a virtual channel.
func (o *Onboard) Next(vcid uint8) ([]byte, bool, error) {
	service, ok := o.services[vcid]
	if !ok {
		return nil, false, fmt.Errorf("%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}

	// A packet may span several frames (MAP packet service reassembly), so
	// one buffer per packet would under-release and drift the FARM into
	// permanent Wait. Release one buffer per frame Receive actually took
	// off the channel instead, measured by the drop in queue length: that
	// covers the error path too, where a frame can be consumed and the
	// call still fail on an incomplete segment.
	before := o.channelLen(vcid)
	packet, err := service.Receive()
	consumed := before - o.channelLen(vcid)
	if farm := o.farms[vcid]; farm != nil {
		for range consumed {
			farm.ReleaseBuffer()
		}
	}
	if err != nil {
		// The service reports an empty buffer the same way it reports a
		// real reassembly failure: as an error. sdl.ErrNoFramesAvailable is
		// the only one of those that means "nothing ready yet"; anything
		// else (a misconfigured packet sizer, an incomplete segment) is a
		// fault the caller needs to see rather than a silent empty read.
		if errors.Is(err, sdl.ErrNoFramesAvailable) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return packet, true, nil
}

// channelLen reports how many frames a virtual channel is currently
// holding, or 0 if the channel is not configured.
func (o *Onboard) channelLen(vcid uint8) int {
	virtual, ok := o.channels[vcid]
	if !ok {
		return 0
	}
	return virtual.Len()
}

// Packets iterates the whole Space Packets waiting on a virtual channel.
func (o *Onboard) Packets(vcid uint8) func(yield func([]byte, error) bool) {
	return func(yield func([]byte, error) bool) {
		for {
			packet, ok, err := o.Next(vcid)
			if err != nil {
				yield(nil, err)
				return
			}
			if !ok {
				return
			}
			if !yield(packet, nil) {
				return
			}
		}
	}
}

// CLCW returns the control word to send back on the telemetry link, encoded
// and ready for a frame's Operational Control Field.
//
// This is the other half of the loop: without it reaching the commander, a
// sequence-controlled channel stops at its sliding window.
func (o *Onboard) CLCW(vcid uint8) ([]byte, error) {
	farm, ok := o.farms[vcid]
	if !ok {
		return nil, fmt.Errorf("%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}

	encoded, err := farm.GenerateCLCW().Encode()
	if err != nil {
		return nil, fmt.Errorf("encoding the CLCW for channel %d: %w", vcid, err)
	}
	return encoded, nil
}

// State reports what FARM-1 is doing on a channel.
func (o *Onboard) State(vcid uint8) (cop.FARMState, error) {
	farm, ok := o.farms[vcid]
	if !ok {
		return 0, fmt.Errorf("%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}
	return farm.State(), nil
}
