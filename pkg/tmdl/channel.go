package tmdl

import (
	"errors"
)

// VirtualChannel represents a logical data stream within a spacecraft.
type VirtualChannel struct {
	VCID          uint8
	FrameBuffer   []*TMTransferFrame // Stores received frames
	MaxBufferSize int                // Max frames that can be stored
}

// NewVirtualChannel initializes a new Virtual Channel.
func NewVirtualChannel(vcid uint8, bufferSize int) *VirtualChannel {
	return &VirtualChannel{
		VCID:          vcid,
		FrameBuffer:   make([]*TMTransferFrame, 0, bufferSize),
		MaxBufferSize: bufferSize,
	}
}

// AddFrame stores a new frame in the Virtual Channel buffer.
func (vc *VirtualChannel) AddFrame(f *TMTransferFrame) error {
	if len(vc.FrameBuffer) >= vc.MaxBufferSize {
		return errors.New("virtual channel buffer full, dropping frame")
	}
	vc.FrameBuffer = append(vc.FrameBuffer, f)
	return nil
}

// GetNextFrame retrieves and removes the oldest frame from the buffer.
func (vc *VirtualChannel) GetNextFrame() (*TMTransferFrame, error) {
	if len(vc.FrameBuffer) == 0 {
		return nil, errors.New("no frames in buffer")
	}
	f := vc.FrameBuffer[0]
	vc.FrameBuffer = vc.FrameBuffer[1:]
	return f, nil
}

// HasFrames checks if there are frames available in the Virtual Channel.
func (vc *VirtualChannel) HasFrames() bool {
	return len(vc.FrameBuffer) > 0
}
