package tcdl

import "github.com/ravisuhag/astro/pkg/sdl"

// PhysicalChannel represents a single TC uplink physical communication link.
// It handles MC-level multiplexing (send path) and demultiplexing (receive path)
// per CCSDS 232.0-B-4. For sync-layer operations (CLTU, BCH), use a
// separate tcsc package.
type PhysicalChannel struct {
	Name     string
	channels *sdl.MasterChannelSet[*TCTransferFrame, *MasterChannel]
}

// NewPhysicalChannel creates a TC physical channel.
func NewPhysicalChannel(name string) *PhysicalChannel {
	return &PhysicalChannel{
		Name:     name,
		channels: sdl.NewMasterChannelSet[*TCTransferFrame, *MasterChannel](),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	pc.channels.Add(mc, priority)
}

// GetNextFrame returns the next frame to transmit across all Master Channels.
func (pc *PhysicalChannel) GetNextFrame() (*TCTransferFrame, error) {
	return pc.channels.Next()
}

// AddFrame routes a frame to the Master Channel for its Spacecraft ID.
func (pc *PhysicalChannel) AddFrame(frame *TCTransferFrame) error {
	return pc.channels.Route(frame.Header.SpacecraftID, frame)
}

// HasPendingFrames reports whether any Master Channel has a frame ready.
func (pc *PhysicalChannel) HasPendingFrames() bool {
	return pc.channels.HasPending()
}

// Len returns the number of registered Master Channels.
func (pc *PhysicalChannel) Len() int {
	return pc.channels.Len()
}
