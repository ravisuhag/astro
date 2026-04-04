package usdl

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
)

// Service is the interface for all USLP Data Link services.
type Service = sdl.Service

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
type PacketSizer = sdl.PacketSizer

// ServiceType defines the types of USLP services available.
type ServiceType int

const (
	MAPP ServiceType = iota // MAP Packet Service
	MAPA                    // MAP Access Service
	MAPO                    // MAP Octet Stream Service
)

// FrameCounter manages 16-bit per-VC frame sequence counts per CCSDS 732.1-B-2.
type FrameCounter struct {
	mu       sync.Mutex
	vcCounts map[uint8]uint16
}

// NewFrameCounter creates a new FrameCounter.
func NewFrameCounter() *FrameCounter {
	return &FrameCounter{vcCounts: make(map[uint8]uint16)}
}

// Next returns the current VC frame count for the given VCID,
// then increments the counter.
func (fc *FrameCounter) Next(vcid uint8) uint16 {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	seq := fc.vcCounts[vcid]
	fc.vcCounts[vcid] = seq + 1
	return seq
}

// stampFrame applies the frame sequence number and recomputes FECF.
func stampFrame(frame *TransferFrame, counter *FrameCounter, vcid uint8) error {
	if counter != nil {
		seq := counter.Next(vcid)
		frame.DataFieldHeader.SequenceNumber = seq
	}
	return recomputeFECF(frame)
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

// MAPPacketService implements the MAPP service for USLP.
// Packets are multiplexed into USLP frames using FirstHeaderOffset
// for boundary detection, with FHO-based resync on frame loss.
type MAPPacketService struct {
	scid    uint16
	vcid    uint8
	mapid   uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel

	// Send-side buffer for multi-packet packing
	sendBuf       []byte
	packetOffsets []int

	// Receive-side state for FHO-based extraction
	recvBuf     []byte
	synced      bool
	sizer       PacketSizer
	gapDetector *FrameGapDetector
}

// NewMAPPacketService creates a new MAPP service instance.
func NewMAPPacketService(scid uint16, vcid, mapid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *MAPPacketService {
	return &MAPPacketService{
		scid:    scid,
		vcid:    vcid,
		mapid:   mapid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// SetPacketSizer configures the PacketSizer used by Receive() to detect
// packet boundaries.
func (s *MAPPacketService) SetPacketSizer(sizer PacketSizer) {
	s.sizer = sizer
}

// Send appends packet data to the send buffer and generates full frames.
// When ChannelConfig.FrameLength is 0, creates one frame per packet.
// When set, packs packets into fixed-length frames with proper FirstHeaderOffset.
// Call Flush() after the last Send() to emit any remaining partial frame.
func (s *MAPPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	if s.config.FrameLength == 0 {
		return s.sendVariableLength(data)
	}

	// Record packet boundary and buffer data
	s.packetOffsets = append(s.packetOffsets, len(s.sendBuf))
	s.sendBuf = append(s.sendBuf, data...)

	return s.emitFullFrames()
}

func (s *MAPPacketService) sendVariableLength(data []byte) error {
	opts := s.frameOpts()
	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
	if err != nil {
		return err
	}
	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

func (s *MAPPacketService) frameOpts() []FrameOption {
	opts := []FrameOption{
		WithConstructionRule(RulePacketSpanning),
	}
	if s.config.FrameLength > 0 {
		opts = append(opts, WithEndOfFPH())
	}
	if s.config.HasOCF {
		opts = append(opts, WithOCF(make([]byte, 4)))
	}
	if s.config.UseCRC32 {
		opts = append(opts, WithCRC32())
	}
	if s.config.InsertZoneLen > 0 {
		opts = append(opts, WithInsertZone(make([]byte, s.config.InsertZoneLen)))
	}
	return opts
}

// Flush pads and emits any remaining buffered data as a final frame.
func (s *MAPPacketService) Flush() error {
	if s.config.FrameLength == 0 || len(s.sendBuf) == 0 {
		return nil
	}

	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}
	chunk := padDataField(s.sendBuf, capacity)

	fho := FHONoPacketStart
	for _, off := range s.packetOffsets {
		if off < len(s.sendBuf) {
			fho = uint16(off)
			break
		}
	}

	s.sendBuf = nil
	s.packetOffsets = nil

	return s.emitFrame(chunk, fho)
}

// emitFullFrames generates frames from sendBuf while it has >= capacity bytes.
func (s *MAPPacketService) emitFullFrames() error {
	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])

		// Find first packet start in this chunk
		fho := FHONoPacketStart
		var remaining []int
		for _, off := range s.packetOffsets {
			if off < capacity {
				if fho == FHONoPacketStart {
					fho = uint16(off)
				}
			} else {
				remaining = append(remaining, off-capacity)
			}
		}
		s.packetOffsets = remaining
		s.sendBuf = s.sendBuf[capacity:]

		if err := s.emitFrame(chunk, fho); err != nil {
			return err
		}
	}

	return nil
}

func (s *MAPPacketService) emitFrame(dataField []byte, fho uint16) error {
	opts := s.frameOpts()
	opts = append(opts, WithFirstHeaderOffset(fho))

	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, dataField, opts...)
	if err != nil {
		return err
	}

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet from frame data.
func (s *MAPPacketService) Receive() ([]byte, error) {
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

		// VC gap detection
		vcGap := s.gapDetector.Track(frame)
		if s.counter != nil && vcGap > 0 {
			s.recvBuf = nil
			s.synced = false
		}

		fho := frame.DataFieldHeader.FirstHeaderOffset
		data := frame.DataField

		switch fho {
		case FHOAllIdle:
			continue // idle

		case FHONoPacketStart:
			// Continuation only
			if s.synced {
				s.recvBuf = append(s.recvBuf, data...)
			}

		default:
			if int(fho) >= len(data) {
				// Corrupted FHO — discard and resync
				s.recvBuf = nil
				s.synced = false
				continue
			}
			// New packet starts at offset fho
			if s.synced && int(fho) > 0 && len(s.recvBuf) > 0 {
				// Append tail of previous packet
				s.recvBuf = append(s.recvBuf, data[:fho]...)

				// Try to extract completed previous packet
				pktLen := sizer(s.recvBuf)
				if pktLen > 0 && pktLen <= len(s.recvBuf) {
					pkt := make([]byte, pktLen)
					copy(pkt, s.recvBuf[:pktLen])
					s.recvBuf = make([]byte, len(data)-int(fho))
					copy(s.recvBuf, data[fho:])
					if isIdleFill(s.recvBuf) {
						s.recvBuf = nil
					}
					return pkt, nil
				}
			}
			// Sync/resync from FHO
			s.recvBuf = make([]byte, len(data)-int(fho))
			copy(s.recvBuf, data[fho:])
			s.synced = true
			if isIdleFill(s.recvBuf) {
				s.recvBuf = nil
			}
		}
	}
}

// MAPAccessService implements the MAPA service for USLP.
// Provides fixed-length SDU transfer.
type MAPAccessService struct {
	scid    uint16
	vcid    uint8
	mapid   uint8
	sduSize int
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewMAPAccessService creates a new MAPA service instance.
func NewMAPAccessService(scid uint16, vcid, mapid uint8, sduSize int, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *MAPAccessService {
	return &MAPAccessService{
		scid:    scid,
		vcid:    vcid,
		mapid:   mapid,
		sduSize: sduSize,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps a fixed-length SDU into a USLP Transfer Frame.
func (s *MAPAccessService) Send(data []byte) error {
	if s.config.FrameLength == 0 {
		if len(data) != s.sduSize {
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

	opts := []FrameOption{
		WithConstructionRule(RuleVCASDU),
		WithFirstHeaderOffset(FHOAllIdle),
	}
	if s.config.FrameLength > 0 {
		opts = append(opts, WithEndOfFPH())
	}
	if s.config.HasOCF {
		opts = append(opts, WithOCF(make([]byte, 4)))
	}
	if s.config.UseCRC32 {
		opts = append(opts, WithCRC32())
	}

	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
	if err != nil {
		return err
	}

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame and returns its data field.
func (s *MAPAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	if s.config.FrameLength > 0 && len(frame.DataField) >= s.sduSize {
		return frame.DataField[:s.sduSize], nil
	}
	return frame.DataField, nil
}

// Flush is a no-op for MAPA service.
func (s *MAPAccessService) Flush() error { return nil }

// MAPOctetStreamService implements the MAPO service for USLP.
// Provides unreliable octet stream transfer without packet boundaries.
type MAPOctetStreamService struct {
	scid    uint16
	vcid    uint8
	mapid   uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel

	sendBuf []byte
}

// NewMAPOctetStreamService creates a new MAPO service instance.
func NewMAPOctetStreamService(scid uint16, vcid, mapid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *MAPOctetStreamService {
	return &MAPOctetStreamService{
		scid:    scid,
		vcid:    vcid,
		mapid:   mapid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// Send buffers octet data and emits full frames.
func (s *MAPOctetStreamService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	if s.config.FrameLength == 0 {
		opts := []FrameOption{
			WithConstructionRule(RuleOctetStream),
			WithFirstHeaderOffset(FHONoPacketStart),
		}
		if s.config.UseCRC32 {
			opts = append(opts, WithCRC32())
		}
		frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
		if err != nil {
			return err
		}
		if err := stampFrame(frame, s.counter, s.vcid); err != nil {
			return err
		}
		return s.vc.Add(frame)
	}

	s.sendBuf = append(s.sendBuf, data...)
	return s.emitFullFrames()
}

func (s *MAPOctetStreamService) emitFullFrames() error {
	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])
		s.sendBuf = s.sendBuf[capacity:]

		if err := s.emitFrame(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *MAPOctetStreamService) emitFrame(dataField []byte) error {
	opts := []FrameOption{
		WithConstructionRule(RuleOctetStream),
		WithFirstHeaderOffset(FHONoPacketStart),
	}
	if s.config.FrameLength > 0 {
		opts = append(opts, WithEndOfFPH())
	}
	if s.config.HasOCF {
		opts = append(opts, WithOCF(make([]byte, 4)))
	}
	if s.config.UseCRC32 {
		opts = append(opts, WithCRC32())
	}

	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, dataField, opts...)
	if err != nil {
		return err
	}

	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame's data field.
func (s *MAPOctetStreamService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// Flush pads and emits any remaining buffered data.
func (s *MAPOctetStreamService) Flush() error {
	if s.config.FrameLength == 0 || len(s.sendBuf) == 0 {
		return nil
	}
	capacity := s.config.DataFieldCapacity(0)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}
	chunk := padDataField(s.sendBuf, capacity)
	s.sendBuf = nil
	return s.emitFrame(chunk)
}
