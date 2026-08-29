package usdl

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/epp"
	"github.com/ravisuhag/astro/pkg/sdl"
)

// Service is the interface for all USLP Data Link services.
type Service = sdl.Service

// PacketSizer returns the total length in bytes of the packet starting
// at data[0], or -1 if the data is too short to determine length.
type PacketSizer = sdl.PacketSizer

// ServiceType identifies the USLP service carried on a MAP channel.
type ServiceType int

const (
	MAPP ServiceType = iota // MAP Packet Service
	MAPA                    // MAP Access Service
	MAPO                    // MAP Octet Stream Service
)

// counterKey identifies one VCF Count: per CCSDS 732.1-B-3
// §4.1.2.12.4-12.5 the sequence-controlled and expedited counts of a VC
// are independent, so the key carries the Bypass/Sequence Control Flag
// alongside the VCID.
type counterKey struct {
	vcid      uint8
	expedited bool
}

// FrameCounter manages Virtual Channel Frame Counts per CCSDS 732.1-B-3
// §4.1.2.12, keyed by VC and quality of service (§4.1.2.12.4-12.5: one
// sequence-controlled and one expedited count per VC). The count is
// carried in the primary header's VCF Count field, whose width (0-7
// octets) is a managed parameter (§4.1.2.11).
type FrameCounter struct {
	mu       sync.Mutex
	vcCounts map[counterKey]uint64
}

// NewFrameCounter creates a new FrameCounter.
func NewFrameCounter() *FrameCounter {
	return &FrameCounter{vcCounts: make(map[counterKey]uint64)}
}

// Next returns the current frame count for the given VCID and quality of
// service (expedited = Bypass/Sequence Control Flag set), then increments
// the counter. Callers mask the value to the managed VCF Count field
// width.
func (fc *FrameCounter) Next(vcid uint8, expedited bool) uint64 {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	key := counterKey{vcid: vcid, expedited: expedited}
	count := fc.vcCounts[key]
	fc.vcCounts[key] = count + 1
	return count
}

// stampFrame applies the next VCF Count to the frame and refreshes the
// FECF. The count field width must already match countLen (frames built
// by the services always reserve it up front, so the frame length does
// not change).
func stampFrame(frame *TransferFrame, counter *FrameCounter, vcid uint8, countLen uint8) error {
	if counter != nil && countLen > 0 {
		frame.Header.VCFCountLen = countLen
		frame.Header.VCFCount = counter.Next(vcid, frame.Header.BypassSeqCtrl) & maxVCFCount(countLen)
	}
	return recomputeFECF(frame)
}

// vcfCountOpt returns the frame option carrying the next VCF Count, or
// nothing when the channel carries no count. The services emit sequence-
// controlled frames (Bypass/Sequence Control Flag '0'), so the sequence-
// controlled counter of the VC is used.
func vcfCountOpt(config ChannelConfig, counter *FrameCounter, vcid uint8) []FrameOption {
	if counter == nil || config.VCFCountLen == 0 {
		return nil
	}
	count := counter.Next(vcid, false) & maxVCFCount(config.VCFCountLen)
	return []FrameOption{WithVCFCount(config.VCFCountLen, count)}
}

// makeOCF builds the Operational Control Field for a frame: nil when the
// channel carries no OCF, and the supplier's 4 octets otherwise. A channel
// configured with HasOCF requires a supplier — the OCF content comes from
// the OCF service user (§4.1.5); emitting zeros would fabricate an empty
// Type-1 report.
func makeOCF(config ChannelConfig, supplier func() []byte) ([]byte, error) {
	if !config.HasOCF {
		return nil, nil
	}
	if supplier == nil {
		return nil, ErrNoOCFSupplier
	}
	ocf := supplier()
	if len(ocf) != OCFSize {
		return nil, ErrInvalidOCFLength
	}
	out := make([]byte, OCFSize)
	copy(out, ocf)
	return out, nil
}

// channelOpts returns the frame options that derive directly from the
// channel configuration and the supplied OCF: insert zone, OCF, and FECF
// presence.
func channelOpts(config ChannelConfig, ocf []byte) []FrameOption {
	var opts []FrameOption
	if config.InsertZoneLen > 0 {
		opts = append(opts, WithInsertZone(make([]byte, config.InsertZoneLen)))
	}
	if ocf != nil {
		opts = append(opts, WithOCF(ocf))
	}
	if !config.HasFECF {
		opts = append(opts, WithoutFECF())
	}
	return opts
}

// isIdleEncap reports whether data begins with an Encapsulation Idle
// Packet header: packet version '111' with protocol ID '000'
// (CCSDS 133.1-B-3), the fill mandated for partially completed
// fixed-length TFDZs by CCSDS 732.1-B-3 §4.1.4.3.4.
func isIdleEncap(data []byte) bool {
	return len(data) > 0 && data[0]&0xFC == 0xE0
}

// stripIdleEncap removes leading Encapsulation Idle Packets from buf.
// A trailing truncated or unparseable idle packet clears the buffer: what
// remains after fill starts is fill.
func stripIdleEncap(buf []byte) []byte {
	for isIdleEncap(buf) {
		n := epp.PacketSizer(buf)
		if n < 1 || n > len(buf) {
			return nil
		}
		buf = buf[n:]
	}
	return buf
}

// MAPPacketService implements the MAP Packet service (MAPP) for USLP.
//
// On fixed-length channels, packets are concatenated into fixed-length
// TFDZs under construction rule '000' with the First Header Pointer for
// boundary recovery; a partially filled final TFDZ is completed with an
// Encapsulation Idle Packet (§4.1.4.3.4). On variable-length channels,
// each Send emits one frame under rule '111' (No Segmentation).
type MAPPacketService struct {
	scid        uint16
	vcid        uint8
	mapid       uint8
	config      ChannelConfig
	counter     *FrameCounter
	vc          *VirtualChannel
	ocfSupplier func() []byte

	// Send-side buffer for multi-packet packing
	sendBuf       []byte
	packetOffsets []int

	// Receive-side state for FHP-based extraction
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

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for every frame emitted on a channel
// configured with HasOCF. Without a supplier such a channel refuses to
// emit frames (ErrNoOCFSupplier) rather than fabricating an all-zero
// Type-1 report (§4.1.5).
func (s *MAPPacketService) SetOCFSupplier(supplier func() []byte) {
	s.ocfSupplier = supplier
}

// dfhSize is the TFDF header size for pointer-carrying rules.
const dfhSizeWithPointer = 3

// dfhSizeNoPointer is the TFDF header size for rules without a pointer.
const dfhSizeNoPointer = 1

// Send accepts one packet. On variable-length channels it emits one frame
// per packet; on fixed-length channels it buffers and emits full frames,
// with Flush() emitting the final partial frame.
func (s *MAPPacketService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}

	if s.config.FrameLength == 0 {
		return s.sendVariableLength(data)
	}

	s.packetOffsets = append(s.packetOffsets, len(s.sendBuf))
	s.sendBuf = append(s.sendBuf, data...)

	return s.emitFullFrames()
}

func (s *MAPPacketService) sendVariableLength(data []byte) error {
	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	opts := []FrameOption{
		WithConstructionRule(RuleNoSegmentation),
		WithUPID(UPIDSpacePackets),
	}
	opts = append(opts, channelOpts(s.config, ocf)...)
	opts = append(opts, vcfCountOpt(s.config, s.counter, s.vcid)...)
	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Flush completes any remaining buffered packet data with an
// Encapsulation Idle Packet and emits the final frame (§4.1.4.3.4).
func (s *MAPPacketService) Flush() error {
	if s.config.FrameLength == 0 || len(s.sendBuf) == 0 {
		return nil
	}

	capacity := s.config.DataFieldCapacity(dfhSizeWithPointer)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	fhp := FHPNoPacketStart
	for _, off := range s.packetOffsets {
		if off < len(s.sendBuf) {
			fhp = uint16(off)
			break
		}
	}
	if fhp == FHPNoPacketStart {
		// The buffer holds only the tail of a packet that started in an
		// earlier frame, and the Encapsulation Idle Packet appended below
		// starts right behind it. §4.1.4.2.4.3-4.4.4: the FHP points at
		// the first packet header starting in the TFDZ — 'all ones' is
		// reserved for TFDZs in which no packet starts at all, and the
		// idle packet is a packet. Pointing at it lets a receiver that
		// lost the preceding frame resynchronize on this one.
		fhp = uint16(len(s.sendBuf))
	}

	fill := byte(DefaultIdleFill)
	if len(s.config.IdlePattern) > 0 {
		fill = s.config.IdlePattern[0]
	}
	idle, err := epp.NewIdleFillPacket(capacity-len(s.sendBuf), fill)
	if err != nil {
		return err
	}
	idleBytes, err := idle.Encode()
	if err != nil {
		return err
	}

	chunk := make([]byte, 0, capacity)
	chunk = append(chunk, s.sendBuf...)
	chunk = append(chunk, idleBytes...)

	s.sendBuf = nil
	s.packetOffsets = nil

	return s.emitFrame(chunk, fhp)
}

// emitFullFrames generates frames from sendBuf while it has >= capacity bytes.
func (s *MAPPacketService) emitFullFrames() error {
	capacity := s.config.DataFieldCapacity(dfhSizeWithPointer)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	for len(s.sendBuf) >= capacity {
		chunk := make([]byte, capacity)
		copy(chunk, s.sendBuf[:capacity])

		// Find first packet start in this chunk
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

func (s *MAPPacketService) emitFrame(dataField []byte, fhp uint16) error {
	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	opts := []FrameOption{
		WithConstructionRule(RulePacketsSpanning),
		WithUPID(UPIDSpacePackets),
		WithPointer(fhp),
	}
	opts = append(opts, channelOpts(s.config, ocf)...)
	opts = append(opts, vcfCountOpt(s.config, s.counter, s.vcid)...)

	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, dataField, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive extracts the next complete packet from frame data.
//
// Rule '000' fill is exactly delimited: spare TFDZ space carries
// Encapsulation Idle Packets (§4.1.4.3.4), which stripIdleEncap removes.
// No pattern heuristic is applied to user data, so payloads that happen to
// look like an idle pattern are delivered intact.
func (s *MAPPacketService) Receive() ([]byte, error) {
	if s.config.FrameLength == 0 {
		frame, err := s.vc.NextForMAP(s.mapid)
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
		s.gapDetector = NewFrameGapDetector(s.config.VCFCountLen)
	}

	for {
		// Drop any idle fill packets before sizing user packets.
		s.recvBuf = stripIdleEncap(s.recvBuf)

		if s.synced && len(s.recvBuf) > 0 {
			pktLen := sizer(s.recvBuf)
			if pktLen > 0 && pktLen <= len(s.recvBuf) {
				pkt := make([]byte, pktLen)
				copy(pkt, s.recvBuf[:pktLen])
				s.recvBuf = stripIdleEncap(s.recvBuf[pktLen:])
				return pkt, nil
			}
		}

		frame, err := s.vc.NextForMAP(s.mapid)
		if err != nil {
			return nil, err
		}

		// Frame loss detection via the VCF Count (§4.1.2.12).
		vcGap := s.gapDetector.Track(frame)
		if vcGap > 0 {
			s.recvBuf = nil
			s.synced = false
		}

		if frame.DataFieldHeader.ConstructionRule != RulePacketsSpanning {
			// Not a packet frame for this rule set; drop and resync.
			s.recvBuf = nil
			s.synced = false
			continue
		}

		fhp := frame.DataFieldHeader.Pointer
		data := frame.DataField

		switch fhp {
		case FHPNoPacketStart:
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
			var completed []byte
			if s.synced && int(fhp) > 0 && len(s.recvBuf) > 0 {
				s.recvBuf = append(s.recvBuf, data[:fhp]...)
				pktLen := sizer(s.recvBuf)
				if pktLen > 0 && pktLen <= len(s.recvBuf) {
					completed = make([]byte, pktLen)
					copy(completed, s.recvBuf[:pktLen])
				}
			}
			// Sync/resync from FHP
			s.recvBuf = make([]byte, len(data)-int(fhp))
			copy(s.recvBuf, data[fhp:])
			s.synced = true
			if completed != nil && !isIdleEncap(completed) {
				return completed, nil
			}
		}
	}
}

// MAPAccessService implements the MAP Access service (MAPA) for USLP:
// transfer of fixed-length MAPA_SDUs.
//
// On fixed-length channels an SDU is carried under construction rule
// '001' (start) and, when it spans frames, rule '010' (continuation),
// delimited by the Last Valid Octet Pointer. On variable-length channels
// each SDU rides alone in a rule '111' frame.
type MAPAccessService struct {
	scid        uint16
	vcid        uint8
	mapid       uint8
	sduSize     int
	config      ChannelConfig
	counter     *FrameCounter
	vc          *VirtualChannel
	ocfSupplier func() []byte

	// Receive-side reassembly state
	recvBuf    []byte
	inProgress bool
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

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for every frame emitted on a channel
// configured with HasOCF. Without a supplier such a channel refuses to
// emit frames (ErrNoOCFSupplier) rather than fabricating an all-zero
// Type-1 report (§4.1.5).
func (s *MAPAccessService) SetOCFSupplier(supplier func() []byte) {
	s.ocfSupplier = supplier
}

// Send transfers one MAPA_SDU of the configured constant length.
func (s *MAPAccessService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if len(data) != s.sduSize {
		return ErrSizeMismatch
	}

	if s.config.FrameLength == 0 {
		ocf, err := makeOCF(s.config, s.ocfSupplier)
		if err != nil {
			return err
		}
		opts := []FrameOption{
			WithConstructionRule(RuleNoSegmentation),
			WithUPID(UPIDMissionSpecific1),
		}
		opts = append(opts, channelOpts(s.config, ocf)...)
		opts = append(opts, vcfCountOpt(s.config, s.counter, s.vcid)...)
		frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
		if err != nil {
			return err
		}
		return s.vc.Add(frame)
	}

	capacity := s.config.DataFieldCapacity(dfhSizeWithPointer)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	// The SDU always begins in the first octet of a rule '001' TFDZ
	// (§4.1.4.2.2.1.4) and continues in rule '010' TFDZs.
	rule := RuleStartOfSDU
	for len(data) > 0 {
		n := len(data)
		lvop := LVOPIncomplete
		if n <= capacity {
			lvop = uint16(n - 1)
		} else {
			n = capacity
		}
		chunk := padDataField(data[:n], capacity, s.config.IdlePattern)
		data = data[n:]

		if err := s.emitFrame(chunk, rule, lvop); err != nil {
			return err
		}
		rule = RuleContinuingSDU
	}
	return nil
}

func (s *MAPAccessService) emitFrame(dataField []byte, rule uint8, lvop uint16) error {
	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	opts := []FrameOption{
		WithConstructionRule(rule),
		WithUPID(UPIDMissionSpecific1),
		WithPointer(lvop),
	}
	opts = append(opts, channelOpts(s.config, ocf)...)
	opts = append(opts, vcfCountOpt(s.config, s.counter, s.vcid)...)
	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, dataField, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive returns the next complete MAPA_SDU, reassembling SDUs that span
// frames via the construction rules and Last Valid Octet Pointer.
func (s *MAPAccessService) Receive() ([]byte, error) {
	if s.config.FrameLength == 0 {
		frame, err := s.vc.NextForMAP(s.mapid)
		if err != nil {
			return nil, err
		}
		return frame.DataField, nil
	}

	for {
		frame, err := s.vc.NextForMAP(s.mapid)
		if err != nil {
			return nil, err
		}

		zone := frame.DataField
		dfh := frame.DataFieldHeader

		switch dfh.ConstructionRule {
		case RuleStartOfSDU:
			s.recvBuf = nil
			s.inProgress = true
		case RuleContinuingSDU:
			if !s.inProgress {
				// Continuation without a start (lost frame): skip.
				continue
			}
		default:
			s.recvBuf = nil
			s.inProgress = false
			continue
		}

		if dfh.Pointer == LVOPIncomplete {
			s.recvBuf = append(s.recvBuf, zone...)
			continue
		}
		if int(dfh.Pointer) >= len(zone) {
			// Corrupted pointer: drop the SDU in progress.
			s.recvBuf = nil
			s.inProgress = false
			continue
		}
		sdu := append(s.recvBuf, zone[:dfh.Pointer+1]...)
		s.recvBuf = nil
		s.inProgress = false
		return sdu, nil
	}
}

// Flush is a no-op for MAPA service.
func (s *MAPAccessService) Flush() error { return nil }

// MAPOctetStreamService implements the MAP Octet Stream service (MAPO)
// for USLP: a continuous octet-aligned stream under construction rule
// '011'. Per CCSDS 732.1-B-3 §4.2.4.1 an octet stream is carried only in
// variable-length Transfer Frames.
type MAPOctetStreamService struct {
	scid        uint16
	vcid        uint8
	mapid       uint8
	config      ChannelConfig
	counter     *FrameCounter
	vc          *VirtualChannel
	ocfSupplier func() []byte
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

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for every frame emitted on a channel
// configured with HasOCF. Without a supplier such a channel refuses to
// emit frames (ErrNoOCFSupplier) rather than fabricating an all-zero
// Type-1 report (§4.1.5).
func (s *MAPOctetStreamService) SetOCFSupplier(supplier func() []byte) {
	s.ocfSupplier = supplier
}

// Send emits the supplied octets in one rule '011' frame.
func (s *MAPOctetStreamService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if s.config.FrameLength != 0 {
		// §4.2.4.1 note 1: one cannot transfer a MAP Octet Stream over
		// fixed-length Transfer Frames.
		return ErrOctetStreamFixedLength
	}

	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	opts := []FrameOption{
		WithConstructionRule(RuleOctetStream),
		WithUPID(UPIDUserOctetStream),
	}
	opts = append(opts, channelOpts(s.config, ocf)...)
	opts = append(opts, vcfCountOpt(s.config, s.counter, s.vcid)...)
	frame, err := NewTransferFrame(s.scid, s.vcid, s.mapid, data, opts...)
	if err != nil {
		return err
	}
	return s.vc.Add(frame)
}

// Receive retrieves the next frame's data field.
func (s *MAPOctetStreamService) Receive() ([]byte, error) {
	frame, err := s.vc.NextForMAP(s.mapid)
	if err != nil {
		return nil, err
	}
	return frame.DataField, nil
}

// Flush is a no-op for MAPO service.
func (s *MAPOctetStreamService) Flush() error { return nil }
