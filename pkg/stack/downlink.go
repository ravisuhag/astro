package stack

import (
	"fmt"

	"github.com/ravisuhag/astro/pkg/spp"
	"github.com/ravisuhag/astro/pkg/tmdl"
	"github.com/ravisuhag/astro/pkg/tmsc"
)

// MaxVCID is the largest virtual channel identifier a TM frame can carry.
// The field is three bits (CCSDS 132.0-B-3 clause 4.1.2.3).
const MaxVCID = 7

// VC is one virtual channel on the downlink.
type VC struct {
	// ID is the virtual channel identifier, 0 to MaxVCID.
	ID uint8

	// Priority orders the channel against the others when several have a
	// frame ready. Higher wins. Channels left at zero are all equal, which
	// is fine when only one carries traffic at a time.
	Priority int

	// Buffer is how many frames the channel holds before Send blocks the
	// caller with an error. Zero takes DefaultBuffer.
	Buffer int
}

// DefaultBuffer is the per-channel frame buffer when a VC does not set one.
const DefaultBuffer = 32

// Downlink is the configuration of one telemetry downlink.
//
// The same value configures both ends. That is the point: a sender and a
// receiver built from one Downlink cannot disagree about the frame length or
// the error control field, which is the failure this package exists to
// prevent.
type Downlink struct {
	// SpacecraftID is the 10-bit identifier both ends expect
	// (CCSDS 132.0-B-3 clause 4.1.2.2).
	SpacecraftID uint16

	// FrameLength is the total octets in a frame, including the header and
	// any error control field. It is fixed for the whole physical channel
	// (clause 2.1.3), which is why it lives here and not on a frame.
	FrameLength int

	// FECF appends the two-octet frame error control field (clause 4.1.6).
	FECF bool

	// OCF carries the four-octet operational control field, which is where
	// the CLCW travels on a mission that uses COP-1 (clause 4.1.5).
	OCF bool

	// Randomize applies the CCSDS pseudo-randomizer to the frame before the
	// sync marker (131.0-B-5 clause 7). Both ends must agree, and they do here.
	Randomize bool

	// ASM overrides the attached sync marker. Nil takes the standard
	// 0x1ACFFC1D of 131.0-B-5 clause 9.
	ASM []byte

	// Channels are the virtual channels, in any order. At least one is
	// required, and their IDs must be distinct.
	Channels []VC
}

// Validate reports whether the configuration can be built.
//
// It is called by NewSender and NewReceiver, so a caller does not have to,
// but it is exported because catching a bad configuration at startup beats
// catching it when the first frame fails to decode.
func (d Downlink) Validate() error {
	if d.FrameLength <= 0 {
		return fmt.Errorf("%w: FrameLength must be positive", ErrInvalidConfig)
	}
	if len(d.Channels) == 0 {
		return fmt.Errorf("%w: at least one virtual channel is required", ErrInvalidConfig)
	}

	seen := make(map[uint8]bool, len(d.Channels))
	for _, channel := range d.Channels {
		if channel.ID > MaxVCID {
			return fmt.Errorf("%w: virtual channel %d is above the 3-bit maximum of %d",
				ErrInvalidConfig, channel.ID, MaxVCID)
		}
		if seen[channel.ID] {
			return fmt.Errorf("%w: virtual channel %d is configured twice",
				ErrInvalidConfig, channel.ID)
		}
		seen[channel.ID] = true

		if channel.Buffer < 0 {
			return fmt.Errorf("%w: virtual channel %d has a negative buffer",
				ErrInvalidConfig, channel.ID)
		}
	}

	// The underlying channel configuration has its own profile limits, and a
	// frame too short to hold its own header is caught there rather than
	// duplicated here.
	if err := d.channelConfig().Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	return nil
}

// channelConfig is the configuration the tmdl layer wants.
func (d Downlink) channelConfig() tmdl.ChannelConfig {
	return tmdl.ChannelConfig{
		FrameLength: d.FrameLength,
		HasOCF:      d.OCF,
		HasFEC:      d.FECF,
	}
}

// buffer returns a channel's frame buffer, applying the default.
func (v VC) buffer() int {
	if v.Buffer == 0 {
		return DefaultBuffer
	}
	return v.Buffer
}

// endpoint is the machinery both ends share.
//
// A sender and a receiver are the same set of objects wired the same way;
// they differ only in which direction data moves through them. Building them
// from one function is what guarantees the two ends match.
type endpoint struct {
	config   Downlink
	physical *tmdl.PhysicalChannel
	services map[uint8]*tmdl.VirtualChannelPacketService
}

// newEndpoint builds the channel tree for one end of the link.
func newEndpoint(config Downlink, name string) (*endpoint, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	channelConfig := config.channelConfig()

	physical := tmdl.NewPhysicalChannel(name, channelConfig)
	master := tmdl.NewMasterChannel(config.SpacecraftID, channelConfig)
	physical.AddMasterChannel(master, 1)

	// One counter for the whole master channel. The master channel frame
	// count is per channel, not per virtual channel (CCSDS 132.0-B-3
	// Clause 4.1.2.4), so the services have to share it or the count would restart
	// on every stream.
	counter := tmdl.NewFrameCounter()

	services := make(map[uint8]*tmdl.VirtualChannelPacketService, len(config.Channels))
	for _, channel := range config.Channels {
		virtual := tmdl.NewVirtualChannel(channel.ID, channel.buffer())
		master.AddVirtualChannel(virtual, channel.Priority)

		service := tmdl.NewVirtualChannelPacketService(
			config.SpacecraftID, channel.ID, virtual, channelConfig, counter)

		// The service has to know where one packet ends to pack several into
		// a frame and to split one across frames. Space Packets carry their
		// own length, so the sizer reads it.
		service.SetPacketSizer(spp.PacketSizer)

		services[channel.ID] = service
	}

	return &endpoint{config: config, physical: physical, services: services}, nil
}

// service returns the packet service for a virtual channel.
func (e *endpoint) service(vcid uint8) (*tmdl.VirtualChannelPacketService, error) {
	service, ok := e.services[vcid]
	if !ok {
		return nil, fmt.Errorf("%w: virtual channel %d is not configured", ErrUnknownChannel, vcid)
	}
	return service, nil
}

// Sender turns Space Packets into CADUs, the spacecraft side of a downlink.
//
// It is not safe for concurrent use. A downlink is one ordered stream of
// frames, and the frame counters say so; serialising access is the caller's
// job because only the caller knows what order the frames should go in.
type Sender struct {
	*endpoint
}

// NewSender builds the spacecraft side of a downlink.
func NewSender(config Downlink) (*Sender, error) {
	end, err := newEndpoint(config, "downlink-sender")
	if err != nil {
		return nil, err
	}
	return &Sender{endpoint: end}, nil
}

// Send hands one encoded Space Packet to a virtual channel.
//
// The packet is buffered and packed into frames with whatever else is on that
// channel; it does not become a CADU until a frame fills up or Flush is
// called. That is the point of the packet service: a frame carries as many
// packets as fit, and a packet longer than a frame is split across several.
func (s *Sender) Send(vcid uint8, packet []byte) error {
	service, err := s.service(vcid)
	if err != nil {
		return err
	}
	return service.Send(packet)
}

// SendPacket encodes a Space Packet and sends it.
//
// It is the common case of Send: most callers have a packet rather than
// octets, and encoding it themselves is a step that can only go one way.
func (s *Sender) SendPacket(vcid uint8, packet *spp.SpacePacket) error {
	encoded, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("encoding packet: %w", err)
	}
	return s.Send(vcid, encoded)
}

// Flush pushes the partly filled frame on every channel out.
//
// Without it the last packets of a pass sit in a buffer waiting for traffic
// that is not coming. The frames it releases are padded to the channel's
// frame length, as a fixed-length channel requires.
func (s *Sender) Flush() error {
	// In channel order rather than map order, so a flush produces the same
	// frames every time.
	for _, channel := range s.config.Channels {
		if err := s.services[channel.ID].Flush(); err != nil {
			return fmt.Errorf("flushing virtual channel %d: %w", channel.ID, err)
		}
	}
	return nil
}

// NextCADU returns the next CADU, or false when there is none waiting.
//
// Frames come out in the priority order the configuration gave the channels.
func (s *Sender) NextCADU() ([]byte, bool, error) {
	if !s.physical.HasPendingFrames() {
		return nil, false, nil
	}

	frame, err := s.physical.GetNextFrame()
	if err != nil {
		return nil, false, fmt.Errorf("taking the next frame: %w", err)
	}

	encoded, err := frame.EncodeWithConfig(s.config.channelConfig())
	if err != nil {
		return nil, false, fmt.Errorf("encoding frame: %w", err)
	}

	return tmsc.WrapCADU(encoded, s.config.ASM, s.config.Randomize), true, nil
}

// CADUs iterates the CADUs waiting to be transmitted.
//
// It stops at the first error, handing it to the caller, and ends when
// nothing is left pending. It does not wait for more: a sender is driven by
// Send, so an empty queue means the caller has more sending to do.
func (s *Sender) CADUs() func(yield func([]byte, error) bool) {
	return func(yield func([]byte, error) bool) {
		for {
			cadu, ok, err := s.NextCADU()
			if err != nil {
				yield(nil, err)
				return
			}
			if !ok {
				return
			}
			if !yield(cadu, nil) {
				return
			}
		}
	}
}

// HasPending reports whether any frame is waiting to go out.
//
// It is a boolean rather than a count because the layer underneath does not
// offer one: PhysicalChannel.Len reports how many master channels are
// registered, not how many frames are queued.
func (s *Sender) HasPending() bool { return s.physical.HasPendingFrames() }

// Receiver turns CADUs back into Space Packets, the ground side of a
// downlink.
//
// Like Sender it is not safe for concurrent use, and for the same reason: the
// frames arrive in an order that means something.
type Receiver struct {
	*endpoint
}

// NewReceiver builds the ground side of a downlink.
//
// Give it the same Downlink value the sender was built from. That is what
// makes the two ends agree.
func NewReceiver(config Downlink) (*Receiver, error) {
	end, err := newEndpoint(config, "downlink-receiver")
	if err != nil {
		return nil, err
	}
	return &Receiver{endpoint: end}, nil
}

// Accept takes one received CADU: it strips the sync marker, decodes the
// frame, and routes it to its virtual channel.
//
// A CADU that does not decode is an error rather than something to swallow,
// because the caller is the only one who can tell a corrupt frame from a
// misconfigured channel. A station that would rather keep going should log
// it and carry on to the next CADU.
func (r *Receiver) Accept(cadu []byte) error {
	frameData, err := tmsc.UnwrapCADU(cadu, r.config.ASM, r.config.Randomize)
	if err != nil {
		return fmt.Errorf("unwrapping CADU: %w", err)
	}

	// The config-aware decoder is the one that enforces the agreement: it
	// rejects a frame that is not the configured length and only looks for an
	// error control field when the channel carries one. The plain decoder
	// assumes both, so a channel without a FECF would fail its CRC check
	// against two octets of data field.
	frame, err := tmdl.DecodeTMTransferFrameWithConfig(frameData, r.config.channelConfig())
	if err != nil {
		return fmt.Errorf("decoding frame: %w", err)
	}

	if err := r.physical.AddFrame(frame); err != nil {
		return fmt.Errorf("routing frame: %w", err)
	}
	return nil
}

// Next returns the next whole Space Packet from a virtual channel, or false
// when the channel has none ready.
//
// A packet split across frames does not appear until its last frame has been
// accepted, which is why this returns false rather than a partial packet.
func (r *Receiver) Next(vcid uint8) ([]byte, bool, error) {
	service, err := r.service(vcid)
	if err != nil {
		return nil, false, err
	}

	// The service reports "nothing more" as an error, which is how it
	// distinguishes an empty buffer from a broken one. Here it is a normal
	// end of stream, so it becomes false.
	packet, err := service.Receive()
	if err != nil {
		return nil, false, nil
	}
	return packet, true, nil
}

// Packets iterates the whole Space Packets waiting on a virtual channel.
//
// It ends when the channel has nothing more ready, so a caller feeding CADUs
// in as they arrive should call it again after each batch.
func (r *Receiver) Packets(vcid uint8) func(yield func([]byte, error) bool) {
	return func(yield func([]byte, error) bool) {
		for {
			packet, ok, err := r.Next(vcid)
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

// NextPacket returns the next Space Packet from a channel, decoded.
//
// It is Next plus spp.Decode, which is what a caller wanting the packet's
// fields rather than its octets would write anyway.
func (r *Receiver) NextPacket(vcid uint8, options ...spp.DecodeOption) (*spp.SpacePacket, bool, error) {
	data, ok, err := r.Next(vcid)
	if err != nil || !ok {
		return nil, false, err
	}

	packet, err := spp.Decode(data, options...)
	if err != nil {
		return nil, false, fmt.Errorf("decoding packet: %w", err)
	}
	return packet, true, nil
}
