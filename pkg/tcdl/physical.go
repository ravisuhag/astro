package tcdl

import "slices"

// PhysicalChannel represents a single TC uplink physical communication link.
// It handles MC-level multiplexing (send path) and demultiplexing (receive path)
// per CCSDS 232.0-B-4. For sync-layer operations (CLTU, BCH), use a
// separate tcsc package when available.
type PhysicalChannel struct {
	Name            string
	masterChannels  map[uint16]*MasterChannel
	priority        map[uint16]int
	sortedSCIDs     []uint16
	currentIndex    int
	remainingWeight int
}

// NewPhysicalChannel creates a TC physical channel.
func NewPhysicalChannel(name string) *PhysicalChannel {
	return &PhysicalChannel{
		Name:           name,
		masterChannels: make(map[uint16]*MasterChannel),
		priority:       make(map[uint16]int),
	}
}

// AddMasterChannel registers a Master Channel with a priority weight.
func (pc *PhysicalChannel) AddMasterChannel(mc *MasterChannel, priority int) {
	if priority < 1 {
		priority = 1
	}
	scid := mc.SCID()
	pc.masterChannels[scid] = mc
	pc.priority[scid] = priority

	pc.sortedSCIDs = make([]uint16, 0, len(pc.masterChannels))
	for s := range pc.masterChannels {
		pc.sortedSCIDs = append(pc.sortedSCIDs, s)
	}
	slices.Sort(pc.sortedSCIDs)

	pc.currentIndex = 0
	if len(pc.sortedSCIDs) > 0 {
		pc.remainingWeight = pc.priority[pc.sortedSCIDs[0]]
	}
}

// GetNextFrame selects the next frame for transmission using weighted
// round-robin MC multiplexing.
func (pc *PhysicalChannel) GetNextFrame() (*TCTransferFrame, error) {
	if len(pc.sortedSCIDs) == 0 {
		return nil, ErrNoMasterChannels
	}

	for range len(pc.sortedSCIDs) {
		scid := pc.sortedSCIDs[pc.currentIndex]
		mc := pc.masterChannels[scid]

		if mc.HasPendingFrames() {
			frame, err := mc.GetNextFrame()
			if err != nil {
				return nil, err
			}
			pc.remainingWeight--
			if pc.remainingWeight <= 0 {
				pc.advanceToNext()
			}
			return frame, nil
		}

		pc.advanceToNext()
	}

	return nil, ErrNoFramesAvailable
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
	for _, mc := range pc.masterChannels {
		if mc.HasPendingFrames() {
			return true
		}
	}
	return false
}

// Len returns the number of registered Master Channels.
func (pc *PhysicalChannel) Len() int {
	return len(pc.masterChannels)
}

func (pc *PhysicalChannel) advanceToNext() {
	pc.currentIndex = (pc.currentIndex + 1) % len(pc.sortedSCIDs)
	scid := pc.sortedSCIDs[pc.currentIndex]
	pc.remainingWeight = pc.priority[scid]
}
