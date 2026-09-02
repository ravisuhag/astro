package tmdl

import (
	"errors"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// MaxFrameLength is the longest TM Transfer Frame ECSS-E-ST-50-03C 5.1b
// allows, in octets.
//
// CCSDS 132.0-B-3 sets no such ceiling; this is the European profile
// constraining it. A mission following CCSDS alone may exceed it, so the limit
// is checked by Validate rather than enforced silently.
const MaxFrameLength = 2048

// ChannelConfig defines the fixed parameters of a physical channel
// per CCSDS 132.0-B-3. All frames on a physical channel share the
// same fixed length and optional field configuration.
type ChannelConfig struct {
	FrameLength int  // Total frame length in octets (fixed per physical channel)
	HasOCF      bool // Whether Operational Control Field (4 bytes) is present
	HasFEC      bool // Whether Frame Error Control (2-byte CRC) is present

	// FSHDataLength is the length in octets of the Transfer Frame Secondary
	// Header Data Field carried by every frame on the channel, or 0 when the
	// channel carries no secondary header. CCSDS 132.0-B-3 clause 4.1.3.1.6 fixes
	// the secondary header length for the associated channel throughout a
	// Mission Phase, which is why it is channel configuration rather than a
	// per-frame choice. The encoded header adds one identification octet, so
	// a value of N costs N+1 octets of frame space. Range 1 to 63.
	//
	// Services fill the header from their FSH supplier (the VC_FSH service of
	// Clause 3.5) or with zeros when none is installed; MasterChannel's supplier
	// (the MC_FSH service of clause 3.8) overwrites it at frame release.
	FSHDataLength int
}

// Validate checks the configuration against the profile limits.
//
// It is not called automatically: a CCSDS-only mission may legitimately run
// frames longer than the ECSS ceiling. Call it when conformance to
// ECSS-E-ST-50-03C matters.
func (c ChannelConfig) Validate() error {
	if c.FrameLength < 8 {
		return ErrDataFieldTooSmall
	}
	if c.FrameLength > MaxFrameLength {
		return ErrFrameTooLong
	}
	if c.FSHDataLength < 0 || c.FSHDataLength > MaxSecondaryHeaderSize-1 {
		return ErrInvalidHeaderLength
	}
	return nil
}

// DataFieldCapacity returns the maximum data field size available
// in frames on this physical channel. secondaryHeaderLen is the
// length of the secondary header data field (0 if not present);
// when present, the encoded secondary header adds 1 prefix byte
// plus secondaryHeaderLen data bytes.
func (c ChannelConfig) DataFieldCapacity(secondaryHeaderLen int) int {
	capacity := c.FrameLength - 6 // primary header is always 6 bytes
	if secondaryHeaderLen > 0 {
		capacity -= 1 + secondaryHeaderLen // 1 prefix byte + data
	}
	if c.HasOCF {
		capacity -= 4
	}
	if c.HasFEC {
		capacity -= 2
	}
	return capacity
}

// PhysicalChannel represents a single physical communication link
// that carries one or more Master Channels. It handles MC-level
// multiplexing (send path) and demultiplexing (receive path)
// per CCSDS 132.0-B-3. For sync-layer operations (ASM, randomization,
// CADU wrapping), use the tmsc package (CCSDS 131.0-B-5).
type PhysicalChannel struct {
	Name           string // Channel identifier (e.g., "X-band")
	config         ChannelConfig
	mux            *sdl.MCMultiplexer[*TMTransferFrame]
	masterChannels map[uint16]*MasterChannel

	// oidFill is the fallback PN generator for idle frames produced with no
	// Master Channel registered; a registered Master Channel uses its own.
	oidFill *OIDSequence
}

// NewPhysicalChannel creates a physical channel with the given configuration.
func NewPhysicalChannel(name string, config ChannelConfig) *PhysicalChannel {
	return &PhysicalChannel{
		Name:           name,
		config:         config,
		mux:            sdl.NewMCMultiplexer[*TMTransferFrame](),
		masterChannels: make(map[uint16]*MasterChannel),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight
// for the MC multiplexing scheme. Priority must be at least 1.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	pc.masterChannels[mc.SCID()] = mc
	pc.mux.Add(mc, priority)
}

// GetNextFrame selects the next frame for transmission using weighted
// round-robin MC multiplexing across registered Master Channels.
func (pc *PhysicalChannel) GetNextFrame() (*TMTransferFrame, error) {
	return pc.mux.Next()
}

// GetNextFrameOrIdle returns the next frame from MC multiplexing,
// or an idle frame if no Master Channel has pending data.
//
// The idle frame comes from a deterministically chosen Master Channel (the
// lowest registered SCID) which builds it per CCSDS 132.0-B-3 clause 4.2.6.4: on a
// packet-carrying VCID, counted by the channel's FrameCounter, filled from
// its persistent PN generator, and carrying its MC_FSH/MC_OCF SDUs when
// suppliers are installed. With no Master Channel registered at all, a bare
// idle frame is produced with SCID 0 and the fallback IdleFrameVCID, drawing
// on the physical channel's own PN generator.
func (pc *PhysicalChannel) GetNextFrameOrIdle() (*TMTransferFrame, error) {
	frame, err := pc.GetNextFrame()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, sdl.ErrNoFramesAvailable) && !errors.Is(err, sdl.ErrNoMasterChannels) {
		return nil, err
	}
	if pc.config.FrameLength == 0 {
		return nil, sdl.ErrNoFramesAvailable
	}
	var chosen *MasterChannel
	for _, mc := range pc.masterChannels {
		if chosen == nil || mc.scid < chosen.scid {
			chosen = mc
		}
	}
	if chosen != nil {
		return chosen.GetNextFrameOrIdle()
	}
	if pc.oidFill == nil {
		pc.oidFill = NewOIDSequence()
	}
	return NewIdleFrameWithCounter(0, IdleFrameVCID, pc.config, nil, pc.oidFill)
}

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel
// based on the Spacecraft ID in the frame header.
func (pc *PhysicalChannel) AddFrame(frame *TMTransferFrame) error {
	mc, ok := pc.masterChannels[frame.Header.SpacecraftID]
	if !ok {
		return ErrMasterChannelNotFound
	}
	return mc.AddFrame(frame)
}

// HasPendingFrames checks if any Master Channel has pending frames.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	return pc.mux.HasPending()
}

// Len returns the number of registered Master Channels.
func (pc *PhysicalChannel) Len() int {
	return pc.mux.Len()
}
