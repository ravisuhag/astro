package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// PhysicalChannel represents a single TC uplink physical communication link.
// It handles MC-level multiplexing (send path) and demultiplexing (receive path)
// per CCSDS 232.0-B-4. For sync-layer operations (CLTU, BCH), use a
// separate tcsc package.
type PhysicalChannel struct {
	Name           string
	mux            *sdl.MCMultiplexer[*TCTransferFrame]
	masterChannels map[uint16]*MasterChannel
}

// NewPhysicalChannel creates a TC physical channel.
func NewPhysicalChannel(name string) *PhysicalChannel {
	return &PhysicalChannel{
		Name:           name,
		mux:            sdl.NewMCMultiplexer[*TCTransferFrame](),
		masterChannels: make(map[uint16]*MasterChannel),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	pc.masterChannels[mc.SCID()] = mc
	pc.mux.Add(mc, priority)
}

// GetNextFrame selects the next frame for transmission using weighted
// round-robin MC multiplexing.
func (pc *PhysicalChannel) GetNextFrame() (*TCTransferFrame, error) {
	return pc.mux.Next()
}

// AddFrame demultiplexes an inbound frame to the appropriate Master Channel.
func (pc *PhysicalChannel) AddFrame(frame *TCTransferFrame) error {
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
