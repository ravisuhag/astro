package tcdl

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// Service is the interface for all TC Data Link services.
type Service = sdl.Service

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
type PacketSizer = sdl.PacketSizer

// ServiceType defines the types of TC services available.
type ServiceType int

const (
	MAPPacket ServiceType = iota // MAP Packet Service
	MAPAccess                    // MAP Access Service
	VCFrame                      // VC Frame Service
)

// FrameCounter manages per-VC 8-bit frame sequence numbers N(S) for COP-1.
type FrameCounter struct {
	mu       sync.Mutex
	vcCounts map[uint8]uint8
}

// NewFrameCounter creates a new FrameCounter.
func NewFrameCounter() *FrameCounter {
	return &FrameCounter{vcCounts: make(map[uint8]uint8)}
}

// Next returns the current sequence number for the given VCID,
// then increments it.
func (fc *FrameCounter) Next(vcid uint8) uint8 {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	n := fc.vcCounts[vcid]
	fc.vcCounts[vcid] = n + 1
	return n
}

// maxDataCapacity returns the max data field size for a TC frame.
func maxDataCapacity(hasSegmentHeader bool) int {
	capacity := MaxFrameLength - PrimaryHeaderSize - FECSize
	if hasSegmentHeader {
		capacity--
	}
	return capacity
}

// MAPPacketService implements the MAP Packet Service.
// Supports segmentation: packets larger than one frame are split across
// multiple frames using the segment header sequence flags.
type MAPPacketService struct {
	scid    uint16
	vcid    uint8
	mapID   uint8
	bypass  bool
	counter *FrameCounter
	vc      *VirtualChannel
	sizer   PacketSizer

	// Receive-side reassembly buffer
	recvBuf []byte
}

// NewMAPPacketService creates a new MAP Packet Service instance.
func NewMAPPacketService(scid uint16, vcid uint8, mapID uint8, bypass bool, vc *VirtualChannel, counter *FrameCounter) *MAPPacketService {
	return &MAPPacketService{
		scid:    scid,
		vcid:    vcid,
		mapID:   mapID,
		bypass:  bypass,
		counter: counter,
		vc:      vc,
	}
}

// SetPacketSizer configures the PacketSizer used by Receive() to detect
// packet boundaries.
func (s *MAPPacketService) SetPacketSizer(sizer PacketSizer) {
	s.sizer = sizer
}

// Send encodes and segments a packet into one or more TC frames.
// Small packets produce a single unsegmented frame. Large packets are
// split using first/continuation/last segment flags.
func (s *MAPPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	capacity := maxDataCapacity(true)
	if len(data) <= capacity {
		return s.emitFrame(data, SegUnsegmented)
	}

	// Segment across multiple frames
	offset := 0
	for offset < len(data) {
		end := offset + capacity
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]

		var flags uint8
		switch {
		case offset == 0:
			flags = SegFirst
		case end == len(data):
			flags = SegLast
		default:
			flags = SegContinuation
		}

		if err := s.emitFrame(chunk, flags); err != nil {
			return err
		}
		offset = end
	}
	return nil
}

func (s *MAPPacketService) emitFrame(data []byte, segFlags uint8) error {
	sh := SegmentHeader{SequenceFlags: segFlags, MAPID: s.mapID}
	opts := []FrameOption{WithSegmentHeader(sh)}
	if s.bypass {
		opts = append(opts, WithBypass())
	}
	if s.counter != nil {
		opts = append(opts, WithSequenceNumber(s.counter.Next(s.vcid)))
	}

	frame, err := NewTCTransferFrame(s.scid, s.vcid, data, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet by reassembling segments.
func (s *MAPPacketService) Receive() ([]byte, error) {
	if s.sizer == nil {
		return nil, ErrNoPacketSizer
	}

	for {
		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}

		// Determine segment flags and payload
		segFlags := SegUnsegmented
		payload := frame.DataField
		if frame.SegmentHeader != nil {
			segFlags = frame.SegmentHeader.SequenceFlags
		}

		switch segFlags {
		case SegUnsegmented:
			s.recvBuf = nil
			return payload, nil

		case SegFirst:
			s.recvBuf = make([]byte, len(payload))
			copy(s.recvBuf, payload)

		case SegContinuation:
			if s.recvBuf == nil {
				continue
			}
			s.recvBuf = append(s.recvBuf, payload...)

		case SegLast:
			if s.recvBuf == nil {
				continue
			}
			s.recvBuf = append(s.recvBuf, payload...)
			result := s.recvBuf
			s.recvBuf = nil
			return result, nil
		}
	}
}

// Flush is a no-op for MAP Packet Service.
func (s *MAPPacketService) Flush() error { return nil }

// MAPAccessService implements the MAP Access Service.
// Sends raw data units without packet boundaries.
type MAPAccessService struct {
	scid    uint16
	vcid    uint8
	mapID   uint8
	bypass  bool
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewMAPAccessService creates a new MAP Access Service instance.
func NewMAPAccessService(scid uint16, vcid uint8, mapID uint8, bypass bool, vc *VirtualChannel, counter *FrameCounter) *MAPAccessService {
	return &MAPAccessService{
		scid:    scid,
		vcid:    vcid,
		mapID:   mapID,
		bypass:  bypass,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps data into a TC frame with an unsegmented segment header.
func (s *MAPAccessService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	sh := SegmentHeader{SequenceFlags: SegUnsegmented, MAPID: s.mapID}
	opts := []FrameOption{WithSegmentHeader(sh)}
	if s.bypass {
		opts = append(opts, WithBypass())
	}
	if s.counter != nil {
		opts = append(opts, WithSequenceNumber(s.counter.Next(s.vcid)))
	}
	frame, err := NewTCTransferFrame(s.scid, s.vcid, data, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive returns the data field of the next frame.
func (s *MAPAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// Flush is a no-op for MAP Access Service.
func (s *MAPAccessService) Flush() error { return nil }

// VCFrameService implements the VC Frame Service.
// Pass-through: sends and receives pre-encoded TC frames.
type VCFrameService struct {
	vcid uint8
	vc   *VirtualChannel
}

// NewVCFrameService creates a new VC Frame Service instance.
func NewVCFrameService(vcid uint8, vc *VirtualChannel) *VCFrameService {
	return &VCFrameService{vcid: vcid, vc: vc}
}

// Send decodes bytes as a TC Transfer Frame and pushes into the VC.
func (s *VCFrameService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	frame, err := DecodeTCTransferFrame(data)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame and returns it as encoded bytes.
func (s *VCFrameService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.Encode()
}

// Flush is a no-op for VC Frame Service.
func (s *VCFrameService) Flush() error { return nil }

// MasterChannel manages TC Transfer Frames for a Master Channel identified by SCID.
type MasterChannel struct {
	scid     uint16
	mux      *VirtualChannelMultiplexer
	channels map[uint8]*VirtualChannel
	detector *FrameGapDetector
}

// NewMasterChannel creates a new Master Channel for the given spacecraft ID.
func NewMasterChannel(scid uint16) *MasterChannel {
	return &MasterChannel{
		scid:     scid,
		mux:      NewMultiplexer(),
		channels: make(map[uint8]*VirtualChannel),
		detector: NewFrameGapDetector(),
	}
}

// SCID returns the Spacecraft Identifier for this Master Channel.
func (mc *MasterChannel) SCID() uint16 { return mc.scid }

// AddVirtualChannel registers a Virtual Channel with this Master Channel.
func (mc *MasterChannel) AddVirtualChannel(vc *VirtualChannel, priority int) {
	mc.channels[vc.ID] = vc
	mc.mux.AddChannel(vc, priority)
}

// AddFrame routes an inbound frame to the appropriate Virtual Channel.
func (mc *MasterChannel) AddFrame(frame *TCTransferFrame) error {
	if frame.Header.SpacecraftID != mc.scid {
		return ErrSCIDMismatch
	}
	mc.detector.Track(frame)
	vc, ok := mc.channels[frame.Header.VirtualChannelID]
	if !ok {
		return ErrVirtualChannelNotFound
	}
	return vc.Add(frame)
}

// GetNextFrame retrieves the next frame from the multiplexer.
func (mc *MasterChannel) GetNextFrame() (*TCTransferFrame, error) {
	return mc.mux.Next()
}

// HasPendingFrames checks if any Virtual Channel has pending frames.
func (mc *MasterChannel) HasPendingFrames() bool {
	return mc.mux.HasPending()
}

// VCFrameGap returns the VC gap from the last AddFrame call.
func (mc *MasterChannel) VCFrameGap() int {
	return mc.detector.VCFrameGap()
}

// Ensure services implement the Service interface.
var (
	_ Service = (*MAPPacketService)(nil)
	_ Service = (*MAPAccessService)(nil)
	_ Service = (*VCFrameService)(nil)
)

