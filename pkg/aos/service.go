package aos

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
	"github.com/ravisuhag/astro/pkg/spp"
)

// Service is the interface for all AOS Data Link services.
type Service = sdl.Service

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
type PacketSizer = sdl.PacketSizer

// ServiceType identifies the AOS service type carried on a Virtual Channel.
type ServiceType int

const (
	MPDU ServiceType = iota // Multiplexing PDU (variable-length packets)
	BPDU                    // Bitstream PDU
	VCA                     // Virtual Channel Access
	VCF                     // Virtual Channel Frame
)

// FrameCounter manages 24-bit per-VC frame counts per CCSDS 732.0-B-4.
// AOS does not specify a Master Channel Frame Count.
type FrameCounter struct {
	mu       sync.Mutex
	vcCounts map[uint8]uint32
}

// NewFrameCounter creates a new FrameCounter.
func NewFrameCounter() *FrameCounter {
	return &FrameCounter{vcCounts: make(map[uint8]uint32)}
}

// Next returns the current VC frame count for the given VCID, then
// increments the counter (with 24-bit wrap).
func (fc *FrameCounter) Next(vcid uint8) uint32 {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	count := fc.vcCounts[vcid]
	fc.vcCounts[vcid] = (count + 1) & MaxVCFrameCount
	return count
}

// stampFrame applies the VC frame count and recomputes FECF.
func stampFrame(frame *TransferFrame, counter *FrameCounter, vcid uint8) error {
	if counter != nil {
		frame.Header.VCFrameCount = counter.Next(vcid)
	}
	return recomputeFECF(frame)
}

// isIdleFill reports whether all bytes match the repeating idle fill pattern.
func isIdleFill(data []byte, pattern []byte) bool {
	if len(pattern) == 0 {
		pattern = []byte{DefaultIdleFill}
	}
	for i, b := range data {
		if b != pattern[i%len(pattern)] {
			return false
		}
	}
	return true
}

// minIdlePacketLen is the smallest encodable SPP idle packet: a 6-byte
// primary header plus at least one data byte.
const minIdlePacketLen = spp.PrimaryHeaderSize + 1

// newIdlePacket returns an encoded SPP idle packet with the given total
// length, its data field filled with the channel's idle pattern.
//
// A total below minIdlePacketLen is refused rather than passed on: the data
// field length would be negative and the allocation would panic.
func newIdlePacket(total int, pattern []byte) ([]byte, error) {
	if total < minIdlePacketLen {
		return nil, ErrDataFieldTooSmall
	}
	payload := make([]byte, total-spp.PrimaryHeaderSize)
	fillIdle(payload, pattern)
	pkt, err := spp.NewIdlePacket(payload)
	if err != nil {
		return nil, err
	}
	return pkt.Encode()
}

// isIdlePacket reports whether the packet's APID marks it as an idle packet.
func isIdlePacket(pkt []byte) bool {
	return spp.IsIdleBytes(pkt)
}

// MultiplexingService implements the M_PDU service for AOS.
//
// Packets are multiplexed into the data field of fixed-length frames
// using the M_PDU First Header Pointer for boundary detection. The
// receive side resyncs from the FHP after frame loss.
type MultiplexingService struct {
	scid    uint8
	vcid    uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel

	sendBuf       []byte
	packetOffsets []int

	recvBuf     []byte
	synced      bool
	sizer       PacketSizer
	gapDetector *FrameGapDetector
}

// NewMultiplexingService creates a new M_PDU service instance.
func NewMultiplexingService(scid, vcid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *MultiplexingService {
	return &MultiplexingService{
		scid:    scid,
		vcid:    vcid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// SetPacketSizer configures the packet sizer used by Receive() to detect
// packet boundaries within the M_PDU packet zone.
func (s *MultiplexingService) SetPacketSizer(sizer PacketSizer) { s.sizer = sizer }

// packetZoneCapacity returns the bytes available for packet data after
// reserving the M_PDU header.
func (s *MultiplexingService) packetZoneCapacity() int {
	return s.config.DataFieldCapacity() - MPDUHeaderSize
}

// Send buffers a packet and emits frames whenever the packet zone fills.
// Call Flush after the last Send to emit any partial frame as idle-padded.
func (s *MultiplexingService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if s.config.FrameLength == 0 {
		return ErrDataFieldTooSmall
	}

	s.packetOffsets = append(s.packetOffsets, len(s.sendBuf))
	s.sendBuf = append(s.sendBuf, data...)

	return s.emitFullFrames()
}

// Flush completes any remaining buffered packet data with an SPP idle
// packet (APID 0x7FF) and emits the final frame(s), per CCSDS 732.0-B-4
// §4.1.4.2.2: a partially filled packet zone is completed with an idle
// packet, not raw fill. When the leftover space is too small to hold a
// whole idle packet, the idle packet spans into further frames.
func (s *MultiplexingService) Flush() error {
	if len(s.sendBuf) == 0 {
		return nil
	}

	capacity := s.packetZoneCapacity()
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	if remaining := capacity - len(s.sendBuf); remaining > 0 {
		// The idle packet has to be a whole packet: at least a 6-octet
		// primary header and one octet of fill. While the leftover space is
		// smaller than that, grow the packet by another packet zone so it
		// ends exactly on a later frame boundary. One extra zone is not
		// always enough — a small zone may need several — so this loops.
		idleLen := remaining
		for idleLen < minIdlePacketLen {
			idleLen += capacity
		}

		idlePkt, err := newIdlePacket(idleLen, s.config.IdlePattern)
		if err != nil {
			return err
		}
		// The idle packet's own start is deliberately not recorded as a
		// packet offset: the First Header Pointer keeps pointing at the first
		// user packet in the zone, and the spanning tail frames report no
		// packet start.
		s.sendBuf = append(s.sendBuf, idlePkt...)
	}
	// Otherwise the buffered data fills the zone exactly, so there is nothing
	// to pad and no extra idle frame to emit.

	return s.emitFullFrames()
}

func (s *MultiplexingService) emitFullFrames() error {
	capacity := s.packetZoneCapacity()
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])

		fhp := FHPNoPacketStart
		var remaining []int
		for _, off := range s.packetOffsets {
			if off < capacity {
				if fhp == FHPNoPacketStart {
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

func (s *MultiplexingService) emitFrame(packetZone []byte, fhp uint16) error {
	dataField, err := PackMPDUDataField(fhp, packetZone)
	if err != nil {
		return err
	}

	opts := frameOpts(s.config)
	frame, err := NewTransferFrame(s.scid, s.vcid, dataField, opts...)
	if err != nil {
		return err
	}
	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet from frame data using the
// M_PDU First Header Pointer to find packet boundaries. A PacketSizer
// must be configured via SetPacketSizer before calling Receive.
func (s *MultiplexingService) Receive() ([]byte, error) {
	if s.sizer == nil {
		return nil, ErrNoPacketSizer
	}
	sizer := s.sizer
	if s.gapDetector == nil {
		s.gapDetector = NewFrameGapDetector()
	}

	pattern := s.config.IdlePattern
	for {
		if s.synced && len(s.recvBuf) > 0 && !isIdleFill(s.recvBuf, pattern) {
			pktLen := sizer(s.recvBuf)
			if pktLen > 0 && pktLen <= len(s.recvBuf) {
				pkt := make([]byte, pktLen)
				copy(pkt, s.recvBuf[:pktLen])
				s.recvBuf = s.recvBuf[pktLen:]
				if isIdleFill(s.recvBuf, pattern) {
					s.recvBuf = nil
				}
				if isIdlePacket(pkt) {
					// Idle fill packets are discarded, not delivered.
					continue
				}
				return pkt, nil
			}
		}

		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}

		if IsIdleFrame(frame) {
			continue
		}

		vcGap := s.gapDetector.Track(frame)
		if s.counter != nil && vcGap > 0 {
			s.recvBuf = nil
			s.synced = false
		}

		var hdr MPDUHeader
		if err := hdr.Decode(frame.DataField); err != nil {
			s.recvBuf = nil
			s.synced = false
			continue
		}
		packetZone := frame.DataField[MPDUHeaderSize:]
		fhp := hdr.FirstHeaderPointer

		switch fhp {
		case FHPAllIdle:
			continue

		case FHPNoPacketStart:
			if s.synced {
				s.recvBuf = append(s.recvBuf, packetZone...)
			}

		default:
			if int(fhp) >= len(packetZone) {
				s.recvBuf = nil
				s.synced = false
				continue
			}
			var completed []byte
			if s.synced && int(fhp) > 0 && len(s.recvBuf) > 0 {
				s.recvBuf = append(s.recvBuf, packetZone[:fhp]...)
				pktLen := sizer(s.recvBuf)
				if pktLen > 0 && pktLen <= len(s.recvBuf) {
					completed = make([]byte, pktLen)
					copy(completed, s.recvBuf[:pktLen])
				}
			}
			s.recvBuf = make([]byte, len(packetZone)-int(fhp))
			copy(s.recvBuf, packetZone[fhp:])
			s.synced = true
			if isIdleFill(s.recvBuf, pattern) {
				s.recvBuf = nil
			}
			if completed != nil && !isIdlePacket(completed) {
				return completed, nil
			}
		}
	}
}

// BitstreamService implements the B_PDU service for AOS.
//
// Octet-aligned bitstream data is packed into the bitstream zone of
// fixed-length frames. The Bitstream Data Pointer marks the position
// where data ends within the frame on the final partial frame.
type BitstreamService struct {
	scid    uint8
	vcid    uint8
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel

	sendBuf []byte
}

// NewBitstreamService creates a new B_PDU service instance.
func NewBitstreamService(scid, vcid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *BitstreamService {
	return &BitstreamService{
		scid:    scid,
		vcid:    vcid,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

func (s *BitstreamService) zoneCapacity() int {
	return s.config.DataFieldCapacity() - BPDUHeaderSize
}

// Send buffers bitstream data and emits full frames.
func (s *BitstreamService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if s.config.FrameLength == 0 {
		return ErrDataFieldTooSmall
	}
	s.sendBuf = append(s.sendBuf, data...)
	return s.emitFullFrames()
}

func (s *BitstreamService) emitFullFrames() error {
	capacity := s.zoneCapacity()
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}
	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])
		s.sendBuf = s.sendBuf[capacity:]
		// Full frame of valid bitstream data.
		if err := s.emitFrame(chunk, BDPAllValid); err != nil {
			return err
		}
	}
	return nil
}

// maxBPDUPartialBytes is the largest number of valid octets a partial
// B_PDU frame can signal. The 14-bit Bitstream Data Pointer carries the
// bit index of the last valid bit, and the two top values (0x3FFE all
// idle, 0x3FFF all valid) are reserved, so n*8-1 <= 0x3FFD gives n <= 2047.
const maxBPDUPartialBytes = 2047

// Flush pads and emits any remaining buffered bitstream data with a
// Bitstream Data Pointer marking the last valid byte. When the partial
// payload holds more valid octets than the 14-bit pointer can express
// (possible only on zones larger than 2047 bytes), it is split across
// additional partial frames instead of being mislabeled.
func (s *BitstreamService) Flush() error {
	if len(s.sendBuf) == 0 {
		return nil
	}
	capacity := s.zoneCapacity()
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}
	for len(s.sendBuf) > 0 {
		n := len(s.sendBuf)
		if n > maxBPDUPartialBytes {
			n = maxBPDUPartialBytes
		}
		// BDP is the bit index of the last valid bit; for octet-aligned
		// data with n valid bytes that is n*8 - 1, which fits the pointer
		// by construction of maxBPDUPartialBytes.
		bdp := uint16(n*8 - 1)
		chunk := padDataField(s.sendBuf[:n], capacity, s.config.IdlePattern)
		s.sendBuf = s.sendBuf[n:]
		if err := s.emitFrame(chunk, bdp); err != nil {
			return err
		}
	}
	s.sendBuf = nil
	return nil
}

func (s *BitstreamService) emitFrame(zone []byte, bdp uint16) error {
	dataField, err := PackBPDUDataField(bdp, zone)
	if err != nil {
		return err
	}
	opts := frameOpts(s.config)
	frame, err := NewTransferFrame(s.scid, s.vcid, dataField, opts...)
	if err != nil {
		return err
	}
	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive returns the bitstream zone of the next frame, trimmed by the
// Bitstream Data Pointer when present.
func (s *BitstreamService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	if len(frame.DataField) < BPDUHeaderSize {
		return nil, ErrDataTooShort
	}
	var hdr BPDUHeader
	if err := hdr.Decode(frame.DataField); err != nil {
		return nil, err
	}
	zone := frame.DataField[BPDUHeaderSize:]
	switch hdr.BitstreamDataPointer {
	case BDPAllIdle:
		return nil, nil
	case BDPAllValid:
		return zone, nil
	default:
		// BDP is bit index of last valid bit; convert to byte length.
		validBits := int(hdr.BitstreamDataPointer) + 1
		validBytes := (validBits + 7) / 8
		if validBytes > len(zone) {
			validBytes = len(zone)
		}
		return zone[:validBytes], nil
	}
}

// VirtualChannelAccessService implements the VCA service for AOS.
//
// VCA delivers an opaque, fixed-length SDU per frame. The data field
// has no protocol header — the entire data field is the SDU.
type VirtualChannelAccessService struct {
	scid    uint8
	vcid    uint8
	sduSize int
	config  ChannelConfig
	counter *FrameCounter
	vc      *VirtualChannel
}

// NewVirtualChannelAccessService creates a new VCA service instance.
func NewVirtualChannelAccessService(scid, vcid uint8, sduSize int, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *VirtualChannelAccessService {
	return &VirtualChannelAccessService{
		scid:    scid,
		vcid:    vcid,
		sduSize: sduSize,
		config:  config,
		counter: counter,
		vc:      vc,
	}
}

// Send wraps a fixed-length SDU into an AOS Transfer Frame. On a
// fixed-length physical channel the VCA_SDU must exactly fill the data
// field (CCSDS 732.0-B-4 §3.3.4: VCA_SDUs are of constant, managed
// length); short or long SDUs are rejected rather than padded, since a
// receiver has no in-band way to recover the SDU boundary.
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if s.config.FrameLength == 0 {
		if len(data) != s.sduSize {
			return ErrSizeMismatch
		}
	} else {
		capacity := s.config.DataFieldCapacity()
		if len(data) > capacity {
			return ErrDataTooLarge
		}
		if len(data) != capacity {
			return ErrSizeMismatch
		}
	}

	opts := frameOpts(s.config)
	frame, err := NewTransferFrame(s.scid, s.vcid, data, opts...)
	if err != nil {
		return err
	}
	if err := stampFrame(frame, s.counter, s.vcid); err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame and returns its data field, which is
// the whole VCA_SDU: on a fixed-length physical channel the SDU exactly
// fills the data field, so no out-of-band trimming is needed.
func (s *VirtualChannelAccessService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// Flush is a no-op for VCA service.
func (s *VirtualChannelAccessService) Flush() error { return nil }

// VirtualChannelFrameService implements the VCF service for AOS.
type VirtualChannelFrameService struct {
	vcid   uint8
	vc     *VirtualChannel
	config ChannelConfig
}

// NewVirtualChannelFrameService creates a new VCF service instance.
func NewVirtualChannelFrameService(vcid uint8, vc *VirtualChannel, config ChannelConfig) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{
		vcid:   vcid,
		vc:     vc,
		config: config,
	}
}

// Send decodes the provided bytes as an AOS Transfer Frame and pushes it.
func (s *VirtualChannelFrameService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	frame, err := DecodeFrame(data, s.config)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame from the Virtual Channel as bytes.
func (s *VirtualChannelFrameService) Receive() ([]byte, error) {
	frame, err := s.vc.Next()
	if err != nil {
		return nil, err
	}
	return frame.Encode()
}

// Flush is a no-op for VCF service.
func (s *VirtualChannelFrameService) Flush() error { return nil }

// frameOpts builds the frame options that derive directly from a channel
// configuration: insert zone reservation, OCF presence, and FECF.
func frameOpts(config ChannelConfig) []FrameOption {
	var opts []FrameOption
	if config.HasFHEC {
		opts = append(opts, WithFHEC())
	}
	if config.InsertZoneLen > 0 {
		opts = append(opts, WithInsertZone(make([]byte, config.InsertZoneLen)))
	}
	if config.HasOCF {
		opts = append(opts, WithOCF(make([]byte, OCFSize)))
	}
	if config.HasFECF {
		opts = append(opts, WithFECF())
	}
	return opts
}
