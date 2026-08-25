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

	// Receive-side reassembly buffer for the segmented packet in progress.
	recvBuf []byte
	// True while a First segment has been seen and the Last is pending.
	reassembling bool
	// Delimited but not yet delivered packet bytes. A frame data field may
	// carry several packets back to back; the PacketSizer slices them out.
	pktBuf []byte
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
		// Type-B frames carry N(S) = 0 (CCSDS 232.0-B-4 4.1.2.7); the
		// COP-1 frame counter applies to Type-A frames only.
		opts = append(opts, WithBypass())
	} else if s.counter != nil {
		opts = append(opts, WithSequenceNumber(s.counter.Next(s.vcid)))
	}

	frame, err := NewTCTransferFrame(s.scid, s.vcid, data, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet. Segmented packets are
// reassembled from First/Continuation/Last frames; a frame data field
// carrying several packets back to back is delimited with the configured
// PacketSizer, and the extra packets are buffered for later calls.
//
// A gap in a segment sequence — a First or Unsegmented frame arriving
// while a reassembly is in progress, a Continuation or Last without a
// First, or a MAP ID change mid-packet — discards the partial packet and
// returns ErrIncompleteSegment. The interrupting frame is preserved and
// delivered by the next call.
func (s *MAPPacketService) Receive() ([]byte, error) {
	if s.sizer == nil {
		return nil, ErrNoPacketSizer
	}

	for {
		// Serve a packet already delimited from earlier frames first.
		if pkt, ok := s.popPacket(); ok {
			return pkt, nil
		}

		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}

		// Determine segment flags and payload
		segFlags := SegUnsegmented
		payload := frame.DataField
		if frame.SegmentHeader != nil {
			segFlags = frame.SegmentHeader.SequenceFlags
			// A segment for another MAP interrupts any reassembly in
			// progress on this one (TCDL routes one MAP per service).
			if frame.SegmentHeader.MAPID != s.mapID {
				if s.dropPartial() {
					return nil, ErrIncompleteSegment
				}
				continue
			}
		}

		switch segFlags {
		case SegUnsegmented:
			s.pktBuf = append(s.pktBuf, payload...)
			if s.dropPartial() {
				return nil, ErrIncompleteSegment
			}

		case SegFirst:
			interrupted := s.dropPartial()
			s.recvBuf = append([]byte(nil), payload...)
			s.reassembling = true
			if interrupted {
				return nil, ErrIncompleteSegment
			}

		case SegContinuation:
			if !s.reassembling {
				return nil, ErrIncompleteSegment
			}
			s.recvBuf = append(s.recvBuf, payload...)

		case SegLast:
			if !s.reassembling {
				return nil, ErrIncompleteSegment
			}
			s.recvBuf = append(s.recvBuf, payload...)
			s.pktBuf = append(s.pktBuf, s.recvBuf...)
			s.recvBuf = nil
			s.reassembling = false
		}
	}
}

// dropPartial discards a reassembly in progress. It reports whether one
// was actually dropped.
func (s *MAPPacketService) dropPartial() bool {
	if !s.reassembling {
		return false
	}
	s.recvBuf = nil
	s.reassembling = false
	return true
}

// popPacket slices the next complete packet off the front of the delimited
// buffer using the configured PacketSizer.
func (s *MAPPacketService) popPacket() ([]byte, bool) {
	if len(s.pktBuf) == 0 {
		return nil, false
	}
	n := s.sizer(s.pktBuf)
	if n <= 0 || n > len(s.pktBuf) {
		// Too short to delimit yet; wait for more frame data.
		return nil, false
	}
	pkt := append([]byte(nil), s.pktBuf[:n]...)
	s.pktBuf = s.pktBuf[n:]
	if len(s.pktBuf) == 0 {
		s.pktBuf = nil
	}
	return pkt, true
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
		// Type-B frames carry N(S) = 0 (CCSDS 232.0-B-4 4.1.2.7); the
		// COP-1 frame counter applies to Type-A frames only.
		opts = append(opts, WithBypass())
	} else if s.counter != nil {
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

// Ensure services implement the Service interface.
var (
	_ Service = (*MAPPacketService)(nil)
	_ Service = (*MAPAccessService)(nil)
	_ Service = (*VCFrameService)(nil)
)
