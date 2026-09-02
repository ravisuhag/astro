package tmdl

import (
	"sync"

	"github.com/ravisuhag/astro/pkg/sdl"
	"github.com/ravisuhag/astro/pkg/spp"
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

// makeOCF builds the Operational Control Field for a frame on the given
// channel: nil when the channel has no OCF, the supplier's 4 octets when one
// is installed, and all zeros otherwise.
func makeOCF(config ChannelConfig, supplier func() []byte) ([]byte, error) {
	if !config.HasOCF {
		return nil, nil
	}
	if supplier != nil {
		ocf := supplier()
		if len(ocf) != 4 {
			return nil, ErrInvalidOCFLength
		}
		out := make([]byte, 4)
		copy(out, ocf)
		return out, nil
	}
	return make([]byte, 4), nil
}

// makeFSH builds the Transfer Frame Secondary Header data field for a frame
// on the given channel: nil when the channel carries no secondary header, the
// supplier's octets when one is installed, and zeros otherwise.
//
// The length is the channel's, not the supplier's choice: CCSDS 132.0-B-3
// Clause 4.1.3.1.6 fixes the secondary header length for the associated channel
// throughout a Mission Phase, and clause 4.1.3.1.5 requires the field to occur in
// every frame of that channel, so a frame is emitted with a zero-filled
// header rather than none when the user has nothing to say.
func makeFSH(config ChannelConfig, supplier func() []byte) ([]byte, error) {
	if config.FSHDataLength <= 0 {
		return nil, nil
	}
	out := make([]byte, config.FSHDataLength)
	if supplier != nil {
		sdu := supplier()
		if len(sdu) != config.FSHDataLength {
			return nil, ErrFSHSizeMismatch
		}
		copy(out, sdu)
	}
	return out, nil
}

// isIdleFill checks if all bytes are 0xFF (raw idle fill pattern).
//
// Conformant streams built by this package no longer produce raw 0xFF fill,
// spare data field space carries real SPP idle packets (see idleFillPacket).
// The check is kept as decode-side leniency for frames produced by older
// versions of this package, which padded with bare 0xFF.
func isIdleFill(data []byte) bool {
	for _, b := range data {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// minPacketSize is the shortest possible Space Packet: six octets of primary
// header plus at least one octet of data (CCSDS 133.0-B-2 clause 4.1.1.2).
const minPacketSize = spp.PrimaryHeaderSize + 1

// idleFillPacket returns an encoded SPP idle packet (APID 0x7FF) of exactly
// n octets. While n is below the seven-octet minimum packet size, it is grown
// by whole data fields so the packet ends exactly on a later frame boundary,
// per CCSDS 132.0-B-3 clause 4.2.2.4: fill that cannot hold a packet header spans
// into the next frame.
func idleFillPacket(n, capacity int) ([]byte, error) {
	for n < minPacketSize {
		n += capacity
	}
	fill := make([]byte, n-spp.PrimaryHeaderSize)
	for i := range fill {
		fill[i] = 0xFF
	}
	pkt, err := spp.NewTMPacket(spp.APIDIdle, fill)
	if err != nil {
		return nil, err
	}
	return pkt.Encode()
}

// isIdlePacket reports whether an encoded packet carries the SPP idle APID
// (all ones). ECSS-E-ST-50-03C 5.4.3.5d and CCSDS 132.0-B-3 clause 4.3.2 require
// the packet extraction function to discard such packets.
func isIdlePacket(pkt []byte) bool {
	return spp.IsIdleBytes(pkt)
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
	gapResync   bool // when true, discard partial packets on frame gaps

	// ocfSupplier, when set, provides the 4-octet Operational Control Field
	// for each emitted frame on a channel with HasOCF.
	ocfSupplier func() []byte

	// fshSupplier, when set, provides the Transfer Frame Secondary Header
	// data field for each emitted frame on a channel with FSHDataLength > 0.
	fshSupplier func() []byte

	// lastFSH is the FSH_SDU from the most recently received frame.
	lastFSH []byte
}

// NewVirtualChannelPacketService creates a new VCP service instance.
// Gap-based resync is enabled automatically when a FrameCounter is provided.
// For pure receivers (counter=nil) that consume externally-stamped frames,
// call SetGapResync(true) to enable resync on frame loss.
func NewVirtualChannelPacketService(scid uint16, vcid uint8, vc *VirtualChannel, config ChannelConfig, counter *FrameCounter) *VirtualChannelPacketService {
	return &VirtualChannelPacketService{
		scid:      scid,
		vcid:      vcid,
		config:    config,
		counter:   counter,
		vc:        vc,
		gapResync: counter != nil,
	}
}

// SetGapResync enables or disables gap-based resync on the receive side.
// When enabled, the receiver discards partially-assembled packets whenever
// a frame gap is detected. Enable this for pure receivers that consume
// externally-stamped frames without a local FrameCounter.
func (s *VirtualChannelPacketService) SetGapResync(enabled bool) {
	s.gapResync = enabled
}

// SetPacketSizer configures the PacketSizer used by Receive() to detect
// packet boundaries. Must be set before calling Receive() when
// ChannelConfig is set (e.g., pass spp.PacketSizer for Space Packets).
func (s *VirtualChannelPacketService) SetPacketSizer(sizer PacketSizer) {
	s.sizer = sizer
}

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for every frame emitted on a channel
// configured with HasOCF. Without a supplier the field is all zeros, which a
// receiver reads as an empty Type-1-Report; per CCSDS 132.0-B-3 clause 4.1.5 the
// field content should come from the OCF service user.
func (s *VirtualChannelPacketService) SetOCFSupplier(supplier func() []byte) {
	s.ocfSupplier = supplier
}

// SetFSHSupplier installs the VC_FSH service user (CCSDS 132.0-B-3 clause 3.5): a
// callback whose FSH_SDU fills the Transfer Frame Secondary Header of every
// frame this service emits. The SDU must be exactly
// ChannelConfig.FSHDataLength octets. Without a supplier the header is
// zero-filled, since clause 4.1.3.1.5 requires it in every frame of the channel
// once the channel carries one.
func (s *VirtualChannelPacketService) SetFSHSupplier(supplier func() []byte) {
	s.fshSupplier = supplier
}

// LastFSH returns the FSH_SDU carried by the most recently received frame, or
// nil when none carried one. It is the VC_FSH.indication of clause 3.5.3.3; pair it
// with the VC frame gap for the optional FSH_SDU Loss Flag.
func (s *VirtualChannelPacketService) LastFSH() []byte { return s.lastFSH }

// Send appends packet data to the send buffer and generates full frames.
// When ChannelConfig is set, packs packets into fixed-length frames with
// proper FirstHeaderPtr. Call Flush() after the last Send() to emit any
// remaining partial frame, padded with SPP idle packets.
//
// When ChannelConfig is not set, Send creates one variable-length frame per
// packet. That legacy path violates the fixed-frame-length rule of CCSDS
// 132.0-B-3 clause 2.1.3 and exists only for in-process loopback and tests; set
// ChannelConfig.FrameLength for anything that leaves the process.
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

// Flush fills any remaining buffered data up to a frame boundary with an SPP
// idle packet (APID 0x7FF) and emits the resulting frame(s). Only meaningful
// when ChannelConfig is set.
//
// CCSDS 132.0-B-3 clause 4.2.2 and ECSS-E-ST-50-03C 5.4.3.4g require spare data
// field space to carry idle packets a conformant receiver can parse and
// discard, not raw fill it would misread as a packet header. When the spare
// space is under the seven-octet minimum packet size, the idle packet spans
// into one or more following frames, so Flush may emit more than one frame.
func (s *VirtualChannelPacketService) Flush() error {
	if s.config.FrameLength == 0 || len(s.sendBuf) == 0 {
		return nil
	}

	capacity := s.config.DataFieldCapacity(s.config.FSHDataLength)
	if capacity <= 0 {
		return ErrDataFieldTooSmall
	}

	if remainder := capacity - len(s.sendBuf); remainder > 0 {
		fill, err := idleFillPacket(remainder, capacity)
		if err != nil {
			return err
		}
		// The idle packet is a real packet: record its start so the
		// First Header Pointer points at its header.
		s.packetOffsets = append(s.packetOffsets, len(s.sendBuf))
		s.sendBuf = append(s.sendBuf, fill...)
	}

	return s.emitFullFrames()
}

// emitFullFrames generates frames from sendBuf while it has >= capacity bytes.
func (s *VirtualChannelPacketService) emitFullFrames() error {
	capacity := s.config.DataFieldCapacity(s.config.FSHDataLength)
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

func (s *VirtualChannelPacketService) emitFrame(dataField []byte, fhp uint16) error {
	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	fsh, err := makeFSH(s.config, s.fshSupplier)
	if err != nil {
		return err
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, dataField, fsh, ocf)
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
				// ECSS-E-ST-50-03C 5.4.3.5d: extracted idle packets
				// are fill and never reach the service user.
				if isIdlePacket(pkt) {
					continue
				}
				return pkt, nil
			}
		}

		// Pull next frame
		frame, err := s.vc.Next()
		if err != nil {
			return nil, err
		}

		// The Virtual Channel Reception Function decommutates every frame of
		// the channel (clause 4.3.3.2), so the FSH_SDU of an OID frame counts too:
		// only its data field is idle, the secondary header can carry valid
		// data (clause 4.1.4.6.3 note 1).
		if frame.Header.FSHFlag {
			s.lastFSH = append([]byte(nil), frame.SecondaryHeader.DataField...)
		}

		if IsIdleFrame(frame) {
			continue
		}

		// VC gap detection: resync when frames are lost to avoid
		// assembling corrupt packets from non-contiguous data.
		_, vcGap := s.gapDetector.Track(frame)
		if s.gapResync && vcGap > 0 {
			s.recvBuf = nil
			s.synced = false
		}

		fhp := frame.Header.FirstHeaderPtr
		data := frame.DataField

		switch fhp {
		case FHPOnlyIdleData:
			// The data field is nothing but fill; there is no payload here.
			continue

		case FHPNoPacketStart:
			// No packet *starts* here, but the field continues one that began
			// in an earlier frame, so it is payload and must be appended.
			if s.synced {
				s.recvBuf = append(s.recvBuf, data...)
			}

		default:
			if int(fhp) >= len(data) {
				// Corrupted FHP, discard and resync
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
					// Idle fill packets are discarded, not delivered.
					if isIdlePacket(pkt) {
						continue
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
	// config carries HasFEC so pass-through frames are decoded and re-encoded
	// the way the channel actually frames them. It defaults to a channel with
	// an error control field, which is the mandatory case under clause 5.6.1b.
	config ChannelConfig

	vcid uint8
	vc   *VirtualChannel
}

// NewVirtualChannelFrameService creates a new VCF service instance.
func NewVirtualChannelFrameService(vcid uint8, vc *VirtualChannel) *VirtualChannelFrameService {
	return &VirtualChannelFrameService{vcid: vcid, vc: vc, config: ChannelConfig{HasFEC: true}}
}

// SetChannelConfig tells the service how its channel is framed, which matters
// only for HasFEC: a channel carrying no Frame Error Control Field needs the
// pass-through decode and re-encode to agree with it.
func (s *VirtualChannelFrameService) SetChannelConfig(config ChannelConfig) {
	s.config = config
}

// Send decodes the provided bytes as a TM Transfer Frame and pushes it into the Virtual Channel.
func (s *VirtualChannelFrameService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	frame, err := DecodeTMTransferFrameWithConfig(data, s.config)
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
	return frame.EncodeWithConfig(s.config)
}

// Flush is a no-op for VCF service.
func (s *VirtualChannelFrameService) Flush() error { return nil }

// VCAStatus contains the VCA Status Fields of CCSDS 132.0-B-3 clause 3.4.2.3: the
// Packet Order Flag, the Segment Length Identifier, and the First Header
// Pointer of the Transfer Frame Data Field Status. With the Synchronization
// Flag set these bits are undefined by CCSDS and belong to the VCA service
// user, who gives them whatever meaning the mission needs, validity,
// sequence, or other status of the VCA_SDU. Providing the field is mandatory;
// the semantics are user-optional.
//
// SyncFlag is reported on receive for completeness. It is not a status field
// the user sets: Clause 4.1.2.7.3.2 fixes it at '1' for a frame carrying a VCA_SDU,
// and VirtualChannelAccessService.Send always sets it.
type VCAStatus struct {
	SyncFlag        bool
	PacketOrderFlag bool
	SegmentLengthID uint8
	FirstHeaderPtr  uint16
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

	// ocfSupplier, when set, provides the 4-octet Operational Control Field
	// for each emitted frame on a channel with HasOCF.
	ocfSupplier func() []byte

	// fshSupplier, when set, provides the Transfer Frame Secondary Header
	// data field for each emitted frame on a channel with FSHDataLength > 0.
	fshSupplier func() []byte

	// sendStatus is the VCA Status Fields the next Send writes into the
	// Transfer Frame Data Field Status.
	sendStatus VCAStatus

	// lastFSH is the FSH_SDU from the most recently received frame.
	lastFSH []byte
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
		// A user who sets no status fields gets all ones in the First Header
		// Pointer, the value clause 4.1.2.7.6.4 uses for 'no packet starts here'.
		// It is the least surprising thing to put in a field CCSDS leaves
		// undefined for VCA frames, and matches what a receiver that ignores
		// the VCA status fields expects to see.
		sendStatus: VCAStatus{FirstHeaderPtr: FHPNoPacketStart},
	}
}

// SetOCFSupplier installs a callback that supplies the 4-octet Operational
// Control Field (typically a CLCW) for every frame emitted on a channel
// configured with HasOCF. Without a supplier the field is all zeros.
func (s *VirtualChannelAccessService) SetOCFSupplier(supplier func() []byte) {
	s.ocfSupplier = supplier
}

// SetFSHSupplier installs the VC_FSH service user (CCSDS 132.0-B-3 clause 3.5) for
// this virtual channel; see VirtualChannelPacketService.SetFSHSupplier.
func (s *VirtualChannelAccessService) SetFSHSupplier(supplier func() []byte) {
	s.fshSupplier = supplier
}

// LastFSH returns the FSH_SDU carried by the most recently received frame, or
// nil when none carried one (the VC_FSH.indication of clause 3.5.3.3).
func (s *VirtualChannelAccessService) LastFSH() []byte { return s.lastFSH }

// SetSendStatus sets the VCA Status Fields carried by frames from subsequent
// Send calls. It is the VCA Status Fields parameter of the VCA.request
// primitive (CCSDS 132.0-B-3 clause 3.4.3.2.2), a mandatory parameter whose
// semantics belong to the service user.
//
// The Synchronization Flag field of the argument is ignored: Clause 4.1.2.7.3.2
// fixes it at '1' for a frame carrying a VCA_SDU, and Send always sets it.
func (s *VirtualChannelAccessService) SetSendStatus(status VCAStatus) {
	s.sendStatus = status
}

// Send wraps a fixed-length SDU into a TM Transfer Frame.
//
// CCSDS 132.0-B-3 clause 3.4.2.2 fixes the VCA_SDU length per virtual channel, so
// data must be exactly vcaSize octets. On a fixed-length channel the SDU must
// also fit the data field: a vcaSize past DataFieldCapacity returns
// ErrDataTooLarge, since the padding a larger SDU would force could not be
// told apart from SDU content by any receiver.
func (s *VirtualChannelAccessService) Send(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyData
	}
	if len(data) != s.vcaSize {
		return ErrSizeMismatch
	}
	if s.config.FrameLength > 0 {
		capacity := s.config.DataFieldCapacity(s.config.FSHDataLength)
		if s.vcaSize > capacity {
			return ErrDataTooLarge
		}
		data = padDataField(data, capacity)
	}

	ocf, err := makeOCF(s.config, s.ocfSupplier)
	if err != nil {
		return err
	}
	fsh, err := makeFSH(s.config, s.fshSupplier)
	if err != nil {
		return err
	}

	frame, err := NewTMTransferFrame(s.scid, s.vcid, data, fsh, ocf)
	if err != nil {
		return err
	}

	// Clause 4.1.2.7.3.2: the Synchronization Flag is '1' when a VCA_SDU is
	// inserted into the data field.
	frame.Header.SyncFlag = true

	// The remaining Transfer Frame Data Field Status bits are the VCA Status
	// Fields of clause 3.4.2.3, undefined by CCSDS with the Synchronization Flag
	// set and carried on behalf of the service user.
	frame.Header.PacketOrderFlag = s.sendStatus.PacketOrderFlag
	frame.Header.SegmentLengthID = s.sendStatus.SegmentLengthID & 0x03
	frame.Header.FirstHeaderPtr = s.sendStatus.FirstHeaderPtr & 0x07FF

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
		FirstHeaderPtr:  frame.Header.FirstHeaderPtr,
	}
	if frame.Header.FSHFlag {
		s.lastFSH = append([]byte(nil), frame.SecondaryHeader.DataField...)
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
