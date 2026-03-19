package tcdl

import "sync"

// VirtualChannel represents a logical data stream within the TC link.
// VCID is 6 bits (0-63) per CCSDS 232.0-B-4.
type VirtualChannel struct {
	VCID        uint8
	mu          sync.Mutex
	frameBuffer []*TCTransferFrame
	maxSize     int
}

// NewVirtualChannel initializes a new Virtual Channel.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return &VirtualChannel{
		VCID:        vcid,
		frameBuffer: make([]*TCTransferFrame, 0, bufferSize),
		maxSize:     bufferSize,
	}
}

// AddFrame stores a new frame in the Virtual Channel buffer.
func (vc *VirtualChannel) AddFrame(f *TCTransferFrame) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	if len(vc.frameBuffer) >= vc.maxSize {
		return ErrBufferFull
	}
	vc.frameBuffer = append(vc.frameBuffer, f)
	return nil
}

// GetNextFrame retrieves and removes the oldest frame from the buffer.
func (vc *VirtualChannel) GetNextFrame() (*TCTransferFrame, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	if len(vc.frameBuffer) == 0 {
		return nil, ErrNoFramesAvailable
	}
	f := vc.frameBuffer[0]
	vc.frameBuffer[0] = nil
	vc.frameBuffer = vc.frameBuffer[1:]
	if len(vc.frameBuffer) == 0 {
		vc.frameBuffer = make([]*TCTransferFrame, 0, vc.maxSize)
	}
	return f, nil
}

// HasFrames checks if there are frames available in the Virtual Channel.
func (vc *VirtualChannel) HasFrames() bool {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return len(vc.frameBuffer) > 0
}

// Len returns the number of frames currently buffered.
func (vc *VirtualChannel) Len() int {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return len(vc.frameBuffer)
}
