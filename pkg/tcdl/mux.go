package tcdl

import "slices"

// VirtualChannelMultiplexer handles frame scheduling from multiple Virtual Channels
// using weighted round-robin.
type VirtualChannelMultiplexer struct {
	channels        map[uint8]*VirtualChannel
	priority        map[uint8]int
	sortedVCIDs     []uint8
	currentIndex    int
	remainingWeight int
}

// NewMultiplexer initializes a Virtual Channel multiplexer.
func NewMultiplexer() *VirtualChannelMultiplexer {
	return &VirtualChannelMultiplexer{
		channels: make(map[uint8]*VirtualChannel),
		priority: make(map[uint8]int),
	}
}

// AddVirtualChannel registers a Virtual Channel with a priority weight.
func (mux *VirtualChannelMultiplexer) AddVirtualChannel(vc *VirtualChannel, priority int) {
	if priority < 1 {
		priority = 1
	}
	mux.channels[vc.VCID] = vc
	mux.priority[vc.VCID] = priority

	mux.sortedVCIDs = make([]uint8, 0, len(mux.channels))
	for vcid := range mux.channels {
		mux.sortedVCIDs = append(mux.sortedVCIDs, vcid)
	}
	slices.Sort(mux.sortedVCIDs)

	mux.currentIndex = 0
	if len(mux.sortedVCIDs) > 0 {
		mux.remainingWeight = mux.priority[mux.sortedVCIDs[0]]
	}
}

// GetNextFrame selects the next frame for transmission based on weighted round-robin.
func (mux *VirtualChannelMultiplexer) GetNextFrame() (*TCTransferFrame, error) {
	if len(mux.sortedVCIDs) == 0 {
		return nil, ErrNoVirtualChannels
	}

	for range len(mux.sortedVCIDs) {
		vcid := mux.sortedVCIDs[mux.currentIndex]
		vc := mux.channels[vcid]

		if vc.HasFrames() {
			frame, err := vc.GetNextFrame()
			if err != nil {
				return nil, err
			}
			mux.remainingWeight--
			if mux.remainingWeight <= 0 {
				mux.advanceToNext()
			}
			return frame, nil
		}

		mux.advanceToNext()
	}

	return nil, ErrNoFramesAvailable
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mux *VirtualChannelMultiplexer) HasPendingFrames() bool {
	for _, vc := range mux.channels {
		if vc.HasFrames() {
			return true
		}
	}
	return false
}

// Len returns the number of registered Virtual Channels.
func (mux *VirtualChannelMultiplexer) Len() int {
	return len(mux.channels)
}

func (mux *VirtualChannelMultiplexer) advanceToNext() {
	mux.currentIndex = (mux.currentIndex + 1) % len(mux.sortedVCIDs)
	vcid := mux.sortedVCIDs[mux.currentIndex]
	mux.remainingWeight = mux.priority[vcid]
}
