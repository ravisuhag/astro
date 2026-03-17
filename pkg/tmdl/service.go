package tmdl

import "sync"

// Service defines the interface for all TM Data Link services.
type Service interface {
	Send(data []byte) error
	Receive() ([]byte, error)
}

// ServiceType defines the types of TM services available.
type ServiceType int

const (
	VCP ServiceType = iota // Virtual Channel Packet Service
	VCA                    // Virtual Channel Access Service
	VCF                    // Virtual Channel Frame Service
)

// FrameCounter manages 8-bit MC and VC frame counts per CCSDS 132.0-B-3.
// Share a single FrameCounter across all services for the same spacecraft
// so the Master Channel count increments correctly.
type FrameCounter struct {
	mu       sync.Mutex
	mcCount  uint8
	vcCounts map[uint8]uint8
}

// NewFrameCounter creates a new FrameCounter.
func NewFrameCounter() *FrameCounter {
	return &FrameCounter{vcCounts: make(map[uint8]uint8)}
}

// Next returns the current MC and VC frame counts for the given VCID,
// then increments both counters.
func (fc *FrameCounter) Next(vcid uint8) (mc, vc uint8) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	mc = fc.mcCount
	vc = fc.vcCounts[vcid]
	fc.mcCount++
	fc.vcCounts[vcid] = vc + 1
	return mc, vc
}

// stampFrame applies optional frame counters and recomputes CRC.
// It always recomputes CRC to ensure it reflects the final header state,
// which is important when headers are modified after NewTMTransferFrame
// (e.g., VCA setting SyncFlag).
func stampFrame(frame *TMTransferFrame, counter *FrameCounter, vcid uint8) error {
	if counter != nil {
		mc, vc := counter.Next(vcid)
		frame.Header.MCFrameCount = mc
		frame.Header.VCFrameCount = vc
	}
	encoded, err := frame.EncodeWithoutFEC()
	if err != nil {
		return err
	}
	frame.FrameErrorControl = ComputeCRC(encoded)
	return nil
}

// VirtualChannelPacketService implements the VCP service.
// It wraps variable-length telemetry packets into frames and pushes
// them into the associated VirtualChannel.
type VirtualChannelPacketService struct {
	scid    uint16
	vcid    uint8
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewVirtualChannelPacketService creates a new VCP service instance.
// Frames are buffered in vc. If counter is non-nil, frame counts and
// CRC are auto-stamped on each Send.
func NewVirtualChannelPacketService(scid uint16, vcid uint8, vc *VirtualChannel, counter *FrameCounter) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		scid:    scid,
		vcid:    vcid,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps data into a TM Transfer Frame and pushes it into the Virtual Channel.
func (s *VirtualChannelPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, data, nil, nil)
	if err != nil {
		return err
	}

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}

	return s.vc.AddFrame(frame)
}

// Receive retrieves the next frame from the Virtual Channel and returns its data field.
func (s *VirtualChannelPacketService) Receive() ([]byte, error) {
	frame, err := s.vc.GetNextFrame()
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// VirtualChannelFrameService implements the VCF service.
// Send accepts encoded frame bytes; Receive returns encoded frame bytes.
type VirtualChannelFrameService struct {
	vcid uint8
	vc   *VirtualChannel
}

// NewVirtualChannelFrameService creates a new VCF service instance.
// Frames are buffered in vc.
func NewVirtualChannelFrameService(vcid uint8, vc *VirtualChannel) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{
		vcid: vcid,
		vc:   vc,
	}
}

// Send decodes the provided bytes as a TM Transfer Frame and pushes it into the Virtual Channel.
func (s *VirtualChannelFrameService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	frame, err := DecodeTMTransferFrame(data)
	if err != nil {
		return err
	}

	return s.vc.AddFrame(frame)
}

// Receive retrieves the next frame from the Virtual Channel and returns it as encoded bytes.
func (s *VirtualChannelFrameService) Receive() ([]byte, error) {
	frame, err := s.vc.GetNextFrame()
	if err != nil {
		return nil, err
	}
	return frame.Encode()
}

// VirtualChannelAccessService implements the VCA service.
// It accepts fixed-length service data units and pushes frames into
// the associated VirtualChannel.
type VirtualChannelAccessService struct {
	scid    uint16
	vcid    uint8
	vcaSize int
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewVirtualChannelAccessService creates a new VCA service instance.
// Frames are buffered in vc. If counter is non-nil, frame counts and
// CRC are auto-stamped on each Send.
func NewVirtualChannelAccessService(scid uint16, vcid uint8, vcaSize int, vc *VirtualChannel, counter *FrameCounter) *VirtualChannelAccessService {
	return &VirtualChannelAccessService{
		scid:    scid,
		vcid:    vcid,
		vcaSize: vcaSize,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps a fixed-length SDU into a TM Transfer Frame and pushes it into the Virtual Channel.
// Per CCSDS 132.0-B-3 §4.1.2.7, VCA frames use synchronous mode (SyncFlag=1)
// with FirstHeaderPtr set to 0x07FF (undefined in sync mode).
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if len(data) != s.vcaSize {
		return ErrSizeMismatch
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, data, nil, nil)
	if err != nil {
		return err
	}

	frame.Header.SyncFlag = true
	frame.Header.FirstHeaderPtr = 0x07FF

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}

	return s.vc.AddFrame(frame)
}

// Receive retrieves the next frame from the Virtual Channel and returns its data field.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.GetNextFrame()
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// MasterChannel manages TM Transfer Frames for a Master Channel identified by SCID.
// It holds a multiplexer for the send path and routes inbound frames to
// Virtual Channels for the receive path.
type MasterChannel struct {
	scid     uint16
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
		mux:      NewMultiplexer(),
		channels: make(map[uint8]*VirtualChannel),
	}
}

// AddVirtualChannel registers a Virtual Channel with this Master Channel
// and adds it to the multiplexer with the given priority weight.
func (mc *MasterChannel) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mc.channels[vc.VCID] = vc
	mc.mux.AddVirtualChannel(vc, priority)
}

// AddFrame routes an inbound frame to the appropriate Virtual Channel based on VCID.
func (mc *MasterChannel) AddFrame(frame *TMTransferFrame) error {
	if frame.Header.SpacecraftID != mc.scid {
		return ErrSCIDMismatch
	}
	vc, ok := mc.channels[frame.Header.VirtualChannelID]
	if !ok {
		return ErrVirtualChannelNotFound
	}
	return vc.AddFrame(frame)
}

// GetNextFrame retrieves the next frame from the multiplexer (send path).
func (mc *MasterChannel) GetNextFrame() (*TMTransferFrame, error) {
	return mc.mux.GetNextFrame()
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPendingFrames()
}
