package tmdl

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// Service is the interface for all TM Data Link services.
type Service = sdl.Service

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
type PacketSizer = sdl.PacketSizer

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
func stampFrame(frame *TMTransferFrame, counter *FrameCounter, vcid uint8) error {
	if counter != nil {
		mc, vc := counter.Next(vcid)
		frame.Header.MCFrameCount = mc
		frame.Header.VCFrameCount = vc
	}
	return recomputeCRC(frame)
}

// isIdleFill checks if all bytes are 0xFF (idle fill pattern).
func isIdleFill(data []byte) bool {
	for _, b := range data {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// VirtualChannelPacketService implements the VCP service.
// When ChannelConfig is set, packets are packed into fixed-length frames
// using native CCSDS FirstHeaderPtr for boundary detection, with FHP-based
// resync on frame loss. A PacketSizer must be set via SetPacketSizer
// before calling Receive (e.g., spp.PacketSizer for CCSDS Space Packets).
type VirtualChannelPacketService struct {
	scid    uint16
	vcid    uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel

	// Send-side buffer for multi-packet packing
	sendBuf       []byte
	packetOffsets []int

	// Receive-side state for FHP-based extraction
	recvBuf     []byte
	synced      bool
	sizer       PacketSizer
	gapDetector *FrameGapDetector
}

// NewVirtualChannelPacketService creates a new VCP service instance.
func NewVirtualChannelPacketService(scid uint16, vcid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		scid:    scid,
		vcid:    vcid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// SetPacketSizer configures the PacketSizer used by Receive() to detect
// packet boundaries. Must be set before calling Receive() when
// ChannelConfig is set (e.g., pass spp.PacketSizer for Space Packets).
func (s *VirtualChannelPacketService) SetPacketSizer(sizer PacketSizer) {
	s.sizer = sizer
}

// Send appends packet data to the send buffer and generates full frames.
// When ChannelConfig is not set, creates one frame per packet (legacy).
// When ChannelConfig is set, packs packets into fixed-length frames with
// proper FirstHeaderPtr. Call Flush() after the last Send() to emit any
// remaining partial frame.
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
		return s.vc.Add(frame)
	}

	// Record packet boundary and buffer data
	s.packetOffsets = append(s.packetOffsets, len(s.sendBuf))
	s.sendBuf = append(s.sendBuf, data...)

	return s.emitFullFrames()
}

// Flush pads and emits any remaining buffered data as a final frame.
// Only meaningful when ChannelConfig is set.
func (s *VirtualChannelPacketService) Flush() error {
	if s.config.FrameLength == 0 || len(s.sendBuf) == 0 {
		return nil
	}

	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}
	chunk := padDataField(s.sendBuf, capacity)

	fhp := uint16(0x07FE)
	for _, off := range s.packetOffsets {
		if off < len(s.sendBuf) {
			fhp = uint16(off)
			break
		}
	}

	s.sendBuf = nil
	s.packetOffsets = nil

	return s.emitFrame(chunk, fhp)
}

// emitFullFrames generates frames from sendBuf while it has >= capacity bytes.
func (s *VirtualChannelPacketService) emitFullFrames() error {
	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])

		// Find first packet start in this chunk
		fhp := uint16(0x07FE)
		var remaining []int
		for _, off := range s.packetOffsets {
			if off < capacity {
				if fhp == 0x07FE {
					fhp = uint16(off)
				}
			} else {
				remaining = append(remaining, off-capacity)
			}
		}
		s.packetOffsets = remaining
		s.sendBuf = s.sendBuf[capacity:]

		if err := s.emitFrame(chunk, fhp); err != nil {
			return err
		}
	}

	return nil
}

func (s *VirtualChannelPacketService) emitFrame(dataField []byte, fhp uint16) error {
	var ocf []byte
	if s.config.HasOCF {
		ocf = make([]byte, 4)
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, dataField, nil, ocf)
	if err != nil {
		return err
	}
	frame.Header.FirstHeaderPtr = fhp

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet from frame data.
// When ChannelConfig is not set, returns the data field of one frame (legacy).
// When ChannelConfig is set, uses FHP to find packet boundaries and
// PacketSizer to determine packet length. Resyncs after frame loss.
func (s *VirtualChannelPacketService) Receive() ([]byte, error) {
	if s.config.FrameLength == 0 {
		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}
		return frame.DataField, nil
	}

	if s.sizer == nil {
		return nil, ErrNoPacketSizer
	}
	sizer := s.sizer
	if s.gapDetector == nil {
		s.gapDetector = NewFrameGapDetector()
	}

	for {
		// Try to extract a complete packet from buffer
		if s.synced && len(s.recvBuf) > 0 && !isIdleFill(s.recvBuf) {
			pktLen := sizer(s.recvBuf)
			if pktLen > 0 && pktLen <= len(s.recvBuf) {
				pkt := make([]byte, pktLen)
				copy(pkt, s.recvBuf[:pktLen])
				s.recvBuf = s.recvBuf[pktLen:]
				if isIdleFill(s.recvBuf) {
					s.recvBuf = nil
				}
				return pkt, nil
			}
		}

		// Pull next frame
		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}

		if IsIdleFrame(frame) {
			continue
		}

		// VC gap detection (only meaningful when frames are counter-stamped)
		_, vcGap := s.gapDetector.Track(frame)
		if s.counter != nil && vcGap > 0 {
			s.recvBuf = nil
			s.synced = false
		}

		fhp := frame.Header.FirstHeaderPtr
		data := frame.DataField

		switch fhp {
		case 0x07FF:
			continue // idle

		case 0x07FE:
			// Continuation only
			if s.synced {
				s.recvBuf = append(s.recvBuf, data...)
			}

		default:
			if int(fhp) >= len(data) {
				// Corrupted FHP — discard and resync
				s.recvBuf = nil
				s.synced = false
				continue
			}
			// New packet starts at offset fhp
			if s.synced && int(fhp) > 0 && len(s.recvBuf) > 0 {
				// Append tail of previous packet
				s.recvBuf = append(s.recvBuf, data[:fhp]...)

				// Try to extract completed previous packet
				pktLen := sizer(s.recvBuf)
				if pktLen > 0 && pktLen <= len(s.recvBuf) {
					pkt := make([]byte, pktLen)
					copy(pkt, s.recvBuf[:pktLen])
					// Start new accumulation from FHP
					s.recvBuf = make([]byte, len(data)-int(fhp))
					copy(s.recvBuf, data[fhp:])
					if isIdleFill(s.recvBuf) {
						s.recvBuf = nil
					}
					return pkt, nil
				}
			}
			// Sync/resync from FHP
			s.recvBuf = make([]byte, len(data)-int(fhp))
			copy(s.recvBuf, data[fhp:])
			s.synced = true
			if isIdleFill(s.recvBuf) {
				s.recvBuf = nil
			}
		}
	}
}

// VirtualChannelFrameService implements the VCF service.
type VirtualChannelFrameService struct {
	vcid uint8
	vc   *VirtualChannel
}

// NewVirtualChannelFrameService creates a new VCF service instance.
func NewVirtualChannelFrameService(vcid uint8, vc *VirtualChannel) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{vcid: vcid, vc: vc}
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
	return s.vc.Add(frame)
}

// Receive retrieves the next frame from the Virtual Channel and returns it as encoded bytes.
func (s *VirtualChannelFrameService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.Encode()
}

// Flush is a no-op for VCF service.
func (s *VirtualChannelFrameService) Flush() error { return nil }

// VCAStatus contains the Transfer Frame Data Field Status fields
// delivered alongside a VCA SDU per CCSDS 132.0-B-3 §3.4.2.3.
type VCAStatus struct {
	SyncFlag        bool
	PacketOrderFlag bool
	SegmentLengthID uint8
}

// VirtualChannelAccessService implements the VCA service.
type VirtualChannelAccessService struct {
	scid       uint16
	vcid       uint8
	vcaSize    int
	config     ChannelConfig
	counter    *FrameCounter
	vc         *VirtualChannel
	lastStatus VCAStatus
}

// NewVirtualChannelAccessService creates a new VCA service instance.
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

// Send wraps a fixed-length SDU into a TM Transfer Frame.
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
	return s.vc.Add(frame)
}

// Receive retrieves the next frame and returns its data field.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	s.lastStatus = VCAStatus{
		SyncFlag:        frame.Header.SyncFlag,
		PacketOrderFlag: frame.Header.PacketOrderFlag,
		SegmentLengthID: frame.Header.SegmentLengthID,
	}
	if s.config.FrameLength > 0 {
		if len(frame.DataField) < s.vcaSize {
			return nil, ErrDataTooShort
		}
		return frame.DataField[:s.vcaSize], nil
	}
	return frame.DataField, nil
}

// LastStatus returns the status fields from the most recent Receive.
func (s *VirtualChannelAccessService) LastStatus() VCAStatus {
	return s.lastStatus
}

// Flush is a no-op for VCA service.
func (s *VirtualChannelAccessService) Flush() error { return nil }
