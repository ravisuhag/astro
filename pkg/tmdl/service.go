package tmdl

// Service defines the interface for all TM Data Link services.
type Service interface {
	Send(data []byte) error
	Receive() ([]byte, error)
}

// VirtualChannelPacketService implements the VCP service.
type VirtualChannelPacketService struct {
	scid      uint16
	channelID uint8
	frames    []*TMTransferFrame
}

// NewVirtualChannelPacketService creates a new VCP service instance.
func NewVirtualChannelPacketService(scid uint16, channelID uint8) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		scid:      scid,
		channelID: channelID,
		frames:    make([]*TMTransferFrame, 0),
	}
}

// Send adds a telemetry packet to the Virtual Channel Packet Service.
func (s *VirtualChannelPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	newFrame, err := NewTMTransferFrame(s.scid, s.channelID, data, nil, nil)
	if err != nil {
		return err
	}

	s.frames = append(s.frames, newFrame)
	return nil
}

// Receive retrieves the next packet from the Virtual Channel Packet Service.
func (s *VirtualChannelPacketService) Receive() ([]byte, error) {
	if len(s.frames) == 0 {
		return nil, ErrNoFramesAvailable
	}

	nextFrame := s.frames[0]
	s.frames[0] = nil // Allow GC of consumed frame
	s.frames = s.frames[1:]

	return nextFrame.DataField, nil
}

// VirtualChannelFrameService implements the VCF service.
// Send accepts encoded frame bytes; Receive returns encoded frame bytes.
type VirtualChannelFrameService struct {
	channelID uint8
	frames    []*TMTransferFrame
}

// NewVirtualChannelFrameService creates a new VCF service instance.
func NewVirtualChannelFrameService(channelID uint8) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{
		channelID: channelID,
		frames:    make([]*TMTransferFrame, 0),
	}
}

// Send decodes the provided bytes as a TM Transfer Frame and stores it.
func (s *VirtualChannelFrameService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	frame, err := DecodeTMTransferFrame(data)
	if err != nil {
		return err
	}

	s.frames = append(s.frames, frame)
	return nil
}

// Receive retrieves the next full TM Transfer Frame as encoded bytes.
func (s *VirtualChannelFrameService) Receive() ([]byte, error) {
	if len(s.frames) == 0 {
		return nil, ErrNoFramesAvailable
	}

	nextFrame := s.frames[0]
	s.frames[0] = nil // Allow GC of consumed frame
	s.frames = s.frames[1:]

	return nextFrame.Encode()
}

// VirtualChannelAccessService implements the VCA service.
type VirtualChannelAccessService struct {
	scid      uint16
	channelID uint8
	vcaSize   int
	frames    []*TMTransferFrame
}

// NewVirtualChannelAccessService creates a new VCA service instance.
func NewVirtualChannelAccessService(scid uint16, channelID uint8, vcaSize int) *VirtualChannelAccessService {
	return &VirtualChannelAccessService{
		scid:      scid,
		channelID: channelID,
		vcaSize:   vcaSize,
		frames:    make([]*TMTransferFrame, 0),
	}
}

// Send adds a fixed-length service data unit to the Virtual Channel Access Service.
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if len(data) != s.vcaSize {
		return ErrSizeMismatch
	}

	newFrame, err := NewTMTransferFrame(s.scid, s.channelID, data, nil, nil)
	if err != nil {
		return err
	}

	s.frames = append(s.frames, newFrame)
	return nil
}

// Receive retrieves the next fixed-length service data unit from the VCA service.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	if len(s.frames) == 0 {
		return nil, ErrNoFramesAvailable
	}

	nextFrame := s.frames[0]
	s.frames[0] = nil // Allow GC of consumed frame
	s.frames = s.frames[1:]

	return nextFrame.DataField, nil
}

// MasterChannelService manages full TM Transfer Frames for a Master Channel.
type MasterChannelService struct {
	scid   uint16
	frames []*TMTransferFrame
}

// NewMasterChannelService creates a new Master Channel service instance.
func NewMasterChannelService(scid uint16) *MasterChannelService {
	return &MasterChannelService{
		scid:   scid,
		frames: make([]*TMTransferFrame, 0),
	}
}

// AddFrame adds a full TM Transfer Frame to the Master Channel.
func (s *MasterChannelService) AddFrame(frame *TMTransferFrame) error {
	if frame.Header.SpacecraftID != s.scid {
		return ErrSCIDMismatch
	}

	s.frames = append(s.frames, frame)
	return nil
}

// GetNextFrame retrieves the next TM Transfer Frame from the Master Channel.
func (s *MasterChannelService) GetNextFrame() (*TMTransferFrame, error) {
	if len(s.frames) == 0 {
		return nil, ErrNoFramesAvailable
	}

	nextFrame := s.frames[0]
	s.frames[0] = nil // Allow GC of consumed frame
	s.frames = s.frames[1:]

	return nextFrame, nil
}

// HasFrames checks if there are any pending frames in the Master Channel.
func (s *MasterChannelService) HasFrames() bool {
	return len(s.frames) > 0
}
