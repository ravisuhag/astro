package tmdl

import (
	"encoding/binary"
	"errors"
	"sync"
)

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
// them into the associated VirtualChannel. When ChannelConfig is set,
// packets are segmented across fixed-length frames with a 2-byte
// length prefix for reassembly.
type VirtualChannelPacketService struct {
	scid    uint16
	vcid    uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewVirtualChannelPacketService creates a new VCP service instance.
// Frames are buffered in vc. If counter is non-nil, frame counts and
// CRC are auto-stamped on each Send. When config.FrameLength > 0,
// packets are segmented into fixed-length frames.
func NewVirtualChannelPacketService(scid uint16, vcid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		scid:    scid,
		vcid:    vcid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps data into TM Transfer Frame(s) and pushes them into the Virtual Channel.
// When ChannelConfig is set, the packet is prepended with a 2-byte big-endian length
// and segmented across fixed-length frames. FirstHeaderPtr is 0 for the first frame
// and 0x07FE for continuation frames.
func (s *VirtualChannelPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	if s.config.FrameLength == 0 {
		frame, err := NewTMTransferFrame(s.scid, s.vcid, data, nil, nil)
		if err != nil {
			return err
		}
		if err := stampFrame(frame, s.counter, s.vcid); err != nil {
			return err
		}
		return s.vc.AddFrame(frame)
	}

	capacity := s.config.DataFieldCapacity(0)
	if capacity < 3 {
		return ErrDataFieldTooSmall
	}
	if len(data) > 65535 {
		return ErrPacketTooLarge
	}

	// Prepend 2-byte big-endian packet length for reassembly
	prefixed := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(prefixed[:2], uint16(len(data)))
	copy(prefixed[2:], data)

	var ocf []byte
	if s.config.HasOCF {
		ocf = make([]byte, 4)
	}

	for offset := 0; offset < len(prefixed); offset += capacity {
		end := offset + capacity
		if end > len(prefixed) {
			end = len(prefixed)
		}
		chunk := padDataField(prefixed[offset:end], capacity)

		frame, err := NewTMTransferFrame(s.scid, s.vcid, chunk, nil, ocf)
		if err != nil {
			return err
		}

		if offset == 0 {
			frame.Header.FirstHeaderPtr = 0
		} else {
			frame.Header.FirstHeaderPtr = 0x07FE
		}

		if err := stampFrame(frame, s.counter, s.vcid); err != nil {
			return err
		}
		if err := s.vc.AddFrame(frame); err != nil {
			return err
		}
	}

	return nil
}

// Receive retrieves the next packet from the Virtual Channel.
// When ChannelConfig is set, it reassembles segmented packets using
// the 2-byte length prefix, skipping idle frames.
func (s *VirtualChannelPacketService) Receive() ([]byte, error) {
	if s.config.FrameLength == 0 {
		frame, err := s.vc.GetNextFrame()
		if err != nil {
			return nil, err
		}
		return frame.DataField, nil
	}

	// Skip idle frames, find first frame of packet
	frame, err := s.nextNonIdleFrame()
	if err != nil {
		return nil, err
	}
	if frame.Header.FirstHeaderPtr != 0 {
		return nil, ErrIncompletePacket
	}
	if len(frame.DataField) < 2 {
		return nil, ErrDataTooShort
	}

	packetLen := int(binary.BigEndian.Uint16(frame.DataField[:2]))
	buf := make([]byte, 0, packetLen)
	buf = append(buf, frame.DataField[2:]...)

	for len(buf) < packetLen {
		frame, err = s.nextNonIdleFrame()
		if err != nil {
			return nil, err
		}
		if frame.Header.FirstHeaderPtr != 0x07FE {
			return nil, ErrIncompletePacket
		}
		buf = append(buf, frame.DataField...)
	}

	return buf[:packetLen], nil
}

// nextNonIdleFrame reads frames from the VirtualChannel, skipping idle frames.
func (s *VirtualChannelPacketService) nextNonIdleFrame() (*TMTransferFrame, error) {
	for {
		frame, err := s.vc.GetNextFrame()
		if err != nil {
			return nil, err
		}
		if !IsIdleFrame(frame) {
			return frame, nil
		}
	}
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
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewVirtualChannelAccessService creates a new VCA service instance.
// Frames are buffered in vc. If counter is non-nil, frame counts and
// CRC are auto-stamped on each Send. When config.FrameLength > 0,
// the data field is padded to the frame's data field capacity.
func NewVirtualChannelAccessService(scid uint16, vcid uint8, vcaSize int, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *VirtualChannelAccessService {
	return &VirtualChannelAccessService{
		scid:    scid,
		vcid:    vcid,
		vcaSize: vcaSize,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps a fixed-length SDU into a TM Transfer Frame and pushes it into the Virtual Channel.
// Per CCSDS 132.0-B-3 §4.1.2.7, VCA frames use synchronous mode (SyncFlag=1)
// with FirstHeaderPtr set to 0x07FF (undefined in sync mode).
// When ChannelConfig is set, the data field is padded to the frame capacity.
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if s.config.FrameLength == 0 {
		if len(data) != s.vcaSize {
			return ErrSizeMismatch
		}
	} else {
		capacity := s.config.DataFieldCapacity(0)
		if len(data) == 0 {
			return ErrEmptyData
		}
		if len(data) > capacity {
			return ErrDataTooLarge
		}
		data = padDataField(data, capacity)
	}

	var ocf []byte
	if s.config.HasOCF {
		ocf = make([]byte, 4)
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, data, nil, ocf)
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
// When ChannelConfig is set, the data field is trimmed to vcaSize.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.GetNextFrame()
	if err != nil {
		return nil, err
	}
	if s.config.FrameLength > 0 {
		return frame.DataField[:s.vcaSize], nil
	}
	return frame.DataField, nil
}

// MasterChannel manages TM Transfer Frames for a Master Channel identified by SCID.
// It holds a multiplexer for the send path and routes inbound frames to
// Virtual Channels for the receive path.
type MasterChannel struct {
	scid     uint16
	config   ChannelConfig
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
// When config.FrameLength > 0, GetNextFrameOrIdle can generate idle frames.
func NewMasterChannel(scid uint16, config ChannelConfig) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
		config:   config,
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

// GetNextFrameOrIdle retrieves the next frame from the multiplexer,
// or generates an idle frame if no Virtual Channel has pending data.
// Requires ChannelConfig to be set (FrameLength > 0).
func (mc *MasterChannel) GetNextFrameOrIdle() (*TMTransferFrame, error) {
	frame, err := mc.mux.GetNextFrame()
	if err == nil {
		return frame, nil
	}
	if !errors.Is(err, ErrNoFramesAvailable) {
		return nil, err
	}
	if mc.config.FrameLength == 0 {
		return nil, ErrNoFramesAvailable
	}
	return NewIdleFrame(mc.scid, 7, mc.config)
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPendingFrames()
}
