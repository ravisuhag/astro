package tmdl

import "errors"

// VirtualChannelMultiplexer handles frame scheduling from multiple Virtual Channels.
type VirtualChannelMultiplexer struct {
	VChannels map[uint8]*VirtualChannel // Map of VCID to VirtualChannel
	Priority  map[uint8]int             // Scheduling weight per VC
	lastUsed  uint8                     // Tracks last scheduled VC
}

// NewMultiplexer initializes a Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return &VirtualChannelMultiplexer{
		VChannels: make(map[uint8]*VirtualChannel),
		Priority:  make(map[uint8]int),
		lastUsed:  0,
	}
}

// AddVirtualChannel registers a Virtual Channel with a priority weight.
func (mux *VirtualChannelMultiplexer) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mux.VChannels[vc.VCID] = vc
	mux.Priority[vc.VCID] = priority
}

// GetNextFrame selects the next frame for transmission based on priority.
func (mux *VirtualChannelMultiplexer) GetNextFrame() (*TMTransferFrame, error) {
	if len(mux.VChannels) == 0 {
		return nil, errors.New("no virtual channels available")
	}

	// Collect all VCIDs in a slice for round-robin selection
	vcids := make([]uint8, 0, len(mux.VChannels))
	for vcid := range mux.VChannels {
		vcids = append(vcids, vcid)
	}

	// Find the next eligible VC with available frames
	for i := 0; i < len(vcids); i++ {
		mux.lastUsed = (mux.lastUsed + 1) % uint8(len(vcids)) // Round-robin selection
		vcid := vcids[mux.lastUsed]
		vc := mux.VChannels[vcid]

		// Check if VC has frames
		if vc.HasFrames() {
			return vc.GetNextFrame()
		}
	}
	return nil, errors.New("no frames available in any virtual channel")
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mux *VirtualChannelMultiplexer) HasPendingFrames() bool {
	for _, vc := range mux.VChannels {
		if vc.HasFrames() {
			return true
		}
	}
	return false
}
