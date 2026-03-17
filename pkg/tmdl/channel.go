package tmdl

// VirtualChannel represents a logical data stream within a spacecraft.
type VirtualChannel struct {
	VCID        uint8
	frameBuffer []*TMTransferFrame
	maxSize     int
}

// NewVirtualChannel initializes a new Virtual Channel.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return &VirtualChannel{
		VCID:        vcid,
		frameBuffer: make([]*TMTransferFrame, 0, bufferSize),
		maxSize:     bufferSize,
	}
}

// AddFrame stores a new frame in the Virtual Channel buffer.
func (vc *VirtualChannel) AddFrame(f *TMTransferFrame) error {
	if len(vc.frameBuffer) >= vc.maxSize {
		return ErrBufferFull
	}
	vc.frameBuffer = append(vc.frameBuffer, f)
	return nil
}

// GetNextFrame retrieves and removes the oldest frame from the buffer.
func (vc *VirtualChannel) GetNextFrame() (*TMTransferFrame, error) {
	if len(vc.frameBuffer) == 0 {
		return nil, ErrNoFramesAvailable
	}
	f := vc.frameBuffer[0]
	vc.frameBuffer[0] = nil // Allow GC of consumed frame
	vc.frameBuffer = vc.frameBuffer[1:]
	return f, nil
}

// HasFrames checks if there are frames available in the Virtual Channel.
func (vc *VirtualChannel) HasFrames() bool {
	return len(vc.frameBuffer) > 0
}

// Len returns the number of frames currently buffered.
func (vc *VirtualChannel) Len() int {
	return len(vc.frameBuffer)
}
