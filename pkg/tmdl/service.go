package tmdl

import (
	"errors"
)

// Service defines the interface for all TM Data Link services.
type Service interface {
	Send(data []byte) error
	Receive() ([]byte, error)
}

// VirtualChannelPacketService implements the VCP service.
type VirtualChannelPacketService struct {
	ChannelID uint8
	Frames    []*TMTransferFrame
}

// NewVirtualChannelPacketService creates a new VCP service instance.
func NewVirtualChannelPacketService(channelID uint8) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		ChannelID: channelID,
		Frames:    make([]*TMTransferFrame, 0),
	}
}

// Send adds a telemetry packet to the Virtual Channel Packet Service.
func (s *VirtualChannelPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return errors.New("data cannot be empty")
	}

	// Create a new TM Transfer Frame with the provided data
	newFrame, err := NewTMTransferFrame(933, s.ChannelID, data, nil, nil)
	if err != nil {
		return err
	}

	s.Frames = append(s.Frames, newFrame)
	return nil
}

// Receive retrieves the next packet from the Virtual Channel Packet Service.
func (s *VirtualChannelPacketService) Receive() ([]byte, error) {
	if len(s.Frames) == 0 {
		return nil, errors.New("no frames available")
	}

	// Retrieve and remove the oldest frame
	nextFrame := s.Frames[0]
	s.Frames = s.Frames[1:]

	return nextFrame.DataField, nil
}

// VirtualChannelFrameService implements the VCF service.
type VirtualChannelFrameService struct {
	ChannelID uint8
	Frames    []*TMTransferFrame
}

// NewVirtualChannelFrameService creates a new VCF service instance.
func NewVirtualChannelFrameService(channelID uint8) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{
		ChannelID: channelID,
		Frames:    make([]*TMTransferFrame, 0),
	}
}

// Send adds a full TM Transfer Frame to the Virtual Channel Frame Service.
func (s *VirtualChannelFrameService) Send(frame *TMTransferFrame) error {
	if frame == nil {
		return errors.New("frame cannot be nil")
	}

	s.Frames = append(s.Frames, frame)
	return nil
}

// Receive retrieves the next full TM Transfer Frame from the Virtual Channel Frame Service.
func (s *VirtualChannelFrameService) Receive() (*TMTransferFrame, error) {
	if len(s.Frames) == 0 {
		return nil, errors.New("no frames available")
	}

	// Retrieve and remove the oldest frame
	nextFrame := s.Frames[0]
	s.Frames = s.Frames[1:]

	return nextFrame, nil
}

// VirtualChannelAccessService implements the VCA service.
type VirtualChannelAccessService struct {
	ChannelID uint8
	VCASize   int // Fixed size of each data unit
	Frames    []*TMTransferFrame
}

// NewVirtualChannelAccessService creates a new VCA service instance.
func NewVirtualChannelAccessService(channelID uint8, vcaSize int) *VirtualChannelAccessService {
	return &VirtualChannelAccessService{
		ChannelID: channelID,
		VCASize:   vcaSize,
		Frames:    make([]*TMTransferFrame, 0),
	}
}

// Send adds a fixed-length service data unit to the Virtual Channel Access Service.
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if len(data) != s.VCASize {
		return errors.New("data size mismatch: expected fixed size")
	}

	// Create a new TM Transfer Frame with the fixed-size data
	newFrame, err := NewTMTransferFrame(933, s.ChannelID, data, nil, nil)
	if err != nil {
		return err
	}

	s.Frames = append(s.Frames, newFrame)
	return nil
}

// Receive retrieves the next fixed-length service data unit from the VCA service.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	if len(s.Frames) == 0 {
		return nil, errors.New("no frames available")
	}

	// Retrieve and remove the oldest frame
	nextFrame := s.Frames[0]
	s.Frames = s.Frames[1:]

	return nextFrame.DataField, nil
}

// MasterChannelService manages full TM Transfer Frames for a Master Channel.
type MasterChannelService struct {
	SCID   uint16
	Frames []*TMTransferFrame
}

// NewMasterChannelService creates a new Master Channel service instance.
func NewMasterChannelService(scid uint16) *MasterChannelService {
	return &MasterChannelService{
		SCID:   scid,
		Frames: make([]*TMTransferFrame, 0),
	}
}

// AddFrame adds a full TM Transfer Frame to the Master Channel.
func (s *MasterChannelService) AddFrame(frame *TMTransferFrame) error {
	if frame.Header.SpacecraftID != s.SCID {
		return errors.New("frame SCID does not match Master Channel SCID")
	}

	s.Frames = append(s.Frames, frame)
	return nil
}

// GetNextFrame retrieves the next TM Transfer Frame from the Master Channel.
func (s *MasterChannelService) GetNextFrame() (*TMTransferFrame, error) {
	if len(s.Frames) == 0 {
		return nil, errors.New("no frames available in Master Channel")
	}

	// Retrieve and remove the oldest frame
	nextFrame := s.Frames[0]
	s.Frames = s.Frames[1:]

	return nextFrame, nil
}

// HasFrames checks if there are any pending frames in the Master Channel.
func (s *MasterChannelService) HasFrames() bool {
	return len(s.Frames) > 0
}
