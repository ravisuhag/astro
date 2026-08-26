package spp

import (
	"encoding/binary"
	"io"
	"reflect"
	"sync"
)

// Service provides both the Packet Service (CCSDS 3.3) and the Octet String
// Service (CCSDS 3.4) over a shared transport.
type Service struct {
	rw           io.ReadWriter
	packetType   uint8
	maxPacketLen int
	newSH        func() SecondaryHeader // optional decoder factory for inbound packets
	errorControl bool                   // expect error control field on received packets
	discardIdle  bool                   // drop received idle packets instead of delivering them

	mu       sync.Mutex
	counters map[uint16]uint16 // per-APID send sequence counters
	expected map[uint16]uint16 // per-APID next expected receive count
	seen     map[uint16]bool   // per-APID: has anything been received yet
	lastGap  int               // packets missing before the most recent receive
}

// ServiceConfig holds configuration for a Service.
type ServiceConfig struct {
	PacketType      uint8 // PacketTypeTM or PacketTypeTC
	MaxPacketLength int   // maximum total packet size in octets; default 65542
	ErrorControl    bool  // if true, received packets are expected to contain a trailing CRC

	// NewSecondaryHeader builds a fresh secondary header for each received
	// packet whose Secondary Header Flag is set. Leave it nil to have the
	// header octets delivered at the front of UserData instead.
	//
	// It is a factory rather than a single value because a decoded header
	// belongs to the packet it came from. One shared instance would be
	// overwritten by every later packet, so every delivered packet would end
	// up showing the newest packet's header.
	NewSecondaryHeader func() SecondaryHeader

	// SecondaryHeader is the old form of NewSecondaryHeader: one instance the
	// service decoded every packet into.
	//
	// Deprecated: use NewSecondaryHeader. When only this field is set the
	// service clones the value's type for each packet, so the aliasing bug
	// does not come back, but the factory states the intent directly.
	SecondaryHeader SecondaryHeader

	// DiscardIdle drops received idle packets (APID 0x7FF) instead of
	// delivering them. They are link fill with no application meaning
	// (4.1.3.3.4.4), so a receiving application normally wants them gone.
	DiscardIdle bool
}

// NewService creates a new SPP service over the given transport.
func NewService(rw io.ReadWriter, cfg ServiceConfig) *Service {
	maxLen := cfg.MaxPacketLength
	if maxLen <= 0 || maxLen > 65542 {
		maxLen = 65542
	}

	newSH := cfg.NewSecondaryHeader
	if newSH == nil && cfg.SecondaryHeader != nil {
		newSH = cloneFactory(cfg.SecondaryHeader)
	}

	return &Service{
		rw:           rw,
		packetType:   cfg.PacketType,
		maxPacketLen: maxLen,
		newSH:        newSH,
		errorControl: cfg.ErrorControl,
		discardIdle:  cfg.DiscardIdle,
		counters:     make(map[uint16]uint16),
		expected:     make(map[uint16]uint16),
		seen:         make(map[uint16]bool),
	}
}

// cloneFactory turns a single SecondaryHeader value into a factory that
// returns a fresh zero value of the same type for every call, so decoded
// packets never share one header instance.
//
// A SecondaryHeader must have pointer methods to be decodable into, so a
// non-pointer value cannot be usefully cloned; such a value is handed back as
// it is, which is what the old single-instance configuration did.
func cloneFactory(sh SecondaryHeader) func() SecondaryHeader {
	t := reflect.TypeOf(sh)
	if t == nil || t.Kind() != reflect.Pointer {
		return func() SecondaryHeader { return sh }
	}
	elem := t.Elem()
	return func() SecondaryHeader {
		fresh, ok := reflect.New(elem).Interface().(SecondaryHeader)
		if !ok {
			return sh
		}
		return fresh
	}
}

// --- Packet Service (CCSDS 3.3) ---

// SendPacket writes a pre-built space packet to the transport.
//
// It stamps the packet with the next sequence count for its APID per CCSDS
// 133.0-B-2 4.1.3.4.3, mutating the caller's packet in place.
//
// A packet whose count was pinned with WithSequenceCount keeps that count,
// and the service resynchronizes its own counter for that APID to one past
// the pinned value. 4.1.3.4.3.4 requires the count to be continuous modulo
// 16384; if the counter kept its old value, an APID that sent one pinned
// packet would emit a jump out and a jump back — 0, 1, 2, 1234, 3, 4 — and a
// receiver would read that as two losses.
func (s *Service) SendPacket(packet *SpacePacket) error {
	if packet == nil {
		return ErrNilPacket
	}

	s.mu.Lock()
	apid := packet.PrimaryHeader.APID
	if packet.seqCountSet {
		s.counters[apid] = (packet.PrimaryHeader.SequenceCount + 1) & 0x3FFF
	} else {
		packet.PrimaryHeader.SequenceCount = s.counters[apid]
		s.counters[apid] = (s.counters[apid] + 1) & 0x3FFF
	}
	s.mu.Unlock()

	data, err := packet.Encode()
	if err != nil {
		return err
	}
	if len(data) > s.maxPacketLen {
		return ErrPacketTooLarge
	}
	_, err = s.rw.Write(data)
	return err
}

// ReceivePacket reads and decodes a complete space packet from the transport.
//
// It also runs the sequence count continuity check of 4.3.2.2 for the
// packet's APID; the result is available from LastDataLoss and is carried on
// the Indication that ReceiveBytes returns.
//
// When ServiceConfig.DiscardIdle is set, idle packets are read and dropped
// and the next real packet is returned.
func (s *Service) ReceivePacket() (*SpacePacket, error) {
	packet, _, err := s.receive()
	return packet, err
}

// receive reads the next deliverable packet and the gap that preceded it.
func (s *Service) receive() (*SpacePacket, int, error) {
	for {
		packet, err := s.readPacket()
		if err != nil {
			return nil, 0, err
		}
		gap := s.trackContinuity(packet.PrimaryHeader.APID, packet.PrimaryHeader.SequenceCount)
		if s.discardIdle && packet.IsIdle() {
			continue
		}
		return packet, gap, nil
	}
}

// readPacket reads exactly one packet off the transport.
func (s *Service) readPacket() (*SpacePacket, error) {
	header := make([]byte, PrimaryHeaderSize)
	if _, err := io.ReadFull(s.rw, header); err != nil {
		return nil, err
	}

	totalPacketSize, err := calculatePacketSize(header)
	if err != nil {
		return nil, err
	}

	if totalPacketSize > s.maxPacketLen {
		return nil, ErrPacketTooLarge
	}

	buffer := make([]byte, totalPacketSize)
	copy(buffer[:PrimaryHeaderSize], header)
	if _, err := io.ReadFull(s.rw, buffer[PrimaryHeaderSize:]); err != nil {
		return nil, err
	}

	var opts []DecodeOption
	// A fresh header per packet: the decoded values belong to this packet and
	// must not be overwritten when the next one arrives.
	if s.newSH != nil {
		if sh := s.newSH(); sh != nil {
			opts = append(opts, WithDecodeSecondaryHeader(sh))
		}
	}
	if s.errorControl {
		opts = append(opts, WithDecodeErrorControl())
	}
	return Decode(buffer, opts...)
}

// --- Packet Sequence Count continuity (CCSDS 4.3.2.2) ---

// trackContinuity records a received sequence count for an APID and returns
// how many packets went missing before it, modulo 16384.
//
// The arithmetic mirrors sdl.GapCounter, which does the same job for frame
// counts on a virtual channel; it is written out here because the counts are
// keyed by an 11-bit APID rather than an 8-bit channel number.
//
// The first packet on an APID reports no loss. There is nothing to compare it
// against, and measuring it from an assumed zero would invent a gap for every
// receiver that joins a pass already in progress.
func (s *Service) trackContinuity(apid, count uint16) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	gap := 0
	if s.seen[apid] {
		gap = int((count - s.expected[apid]) & 0x3FFF)
	} else {
		s.seen[apid] = true
	}
	s.expected[apid] = (count + 1) & 0x3FFF
	s.lastGap = gap
	return gap
}

// LastDataLoss returns how many packets were missing before the most recently
// received packet, across all APIDs. Zero means the count was continuous.
//
// This is the Data Loss Indicator of 3.4.2.4, an optional service parameter;
// the continuity check that produces it is not optional (4.3.2.2).
func (s *Service) LastDataLoss() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastGap
}

// ResetContinuity forgets every APID's received count, so the next packet on
// each is treated as a first packet and reports no loss. Use it after a link
// outage, where the gap across the break carries no information.
func (s *Service) ResetContinuity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.expected)
	clear(s.seen)
	s.lastGap = 0
}

// --- Octet String Service (CCSDS 3.4) ---

// SendOption configures optional parameters for SendBytes.
type SendOption func(*sendConfig)

type sendConfig struct {
	sh           SecondaryHeader
	errorControl bool
	packetType   *uint8
	seqCount     *uint16
}

// WithSendSecondaryHeader attaches a secondary header to the outgoing packet.
// This is the Secondary Header Indicator parameter of 3.4.2.3.
func WithSendSecondaryHeader(sh SecondaryHeader) SendOption {
	return func(cfg *sendConfig) { cfg.sh = sh }
}

// WithSendErrorControl enables CRC-16-CCITT error control on the outgoing packet.
// The checksum is computed automatically during encoding.
func WithSendErrorControl() SendOption {
	return func(cfg *sendConfig) { cfg.errorControl = true }
}

// WithSendPacketType overrides the Packet Type of the outgoing packet,
// which otherwise comes from ServiceConfig.PacketType.
//
// Packet Type is a parameter of the OCTET_STRING.request primitive
// (3.4.3.2.2), so one service can send both telemetry and telecommand octet
// strings.
func WithSendPacketType(packetType uint8) SendOption {
	return func(cfg *sendConfig) { cfg.packetType = &packetType }
}

// WithSendSequenceCount pins the Packet Sequence Count of the outgoing
// packet instead of taking the service's next count for the APID.
//
// It is the Packet Sequence Count parameter of the OCTET_STRING.request
// primitive (3.4.3.2.2). As with WithSequenceCount, the service then carries
// on from one past the pinned value so the APID's count stays continuous.
func WithSendSequenceCount(n uint16) SendOption {
	return func(cfg *sendConfig) { cfg.seqCount = &n }
}

// WithSendPacketName pins the same 14 bits as WithSendSequenceCount under the
// other name the standard gives them.
//
// For a telecommand packet (Packet Type '1') bits 18-31 of the primary header
// carry either the Packet Sequence Count or a Packet Name (4.1.3.4.3.2), so
// the two options do the same thing; this one says which meaning is intended.
func WithSendPacketName(name uint16) SendOption {
	return WithSendSequenceCount(name)
}

// SendBytes wraps the given data in a space packet and writes it to the transport.
// The caller provides raw bytes and service parameters; SPP handles packet construction.
func (s *Service) SendBytes(apid uint16, data []byte, opts ...SendOption) error {
	var cfg sendConfig
	for _, o := range opts {
		o(&cfg)
	}

	packetType := s.packetType
	if cfg.packetType != nil {
		packetType = *cfg.packetType
	}

	var pktOpts []PacketOption
	if cfg.sh != nil {
		pktOpts = append(pktOpts, WithSecondaryHeader(cfg.sh))
	}
	if cfg.errorControl {
		pktOpts = append(pktOpts, WithErrorControl())
	}
	if cfg.seqCount != nil {
		pktOpts = append(pktOpts, WithSequenceCount(*cfg.seqCount))
	}

	packet, err := NewSpacePacket(apid, packetType, data, pktOpts...)
	if err != nil {
		return err
	}

	return s.SendPacket(packet)
}

// Indication carries the parameters the OCTET_STRING.indication primitive
// delivers to the Octet String Service user (CCSDS 133.0-B-2 3.4.3.3.2).
type Indication struct {
	// Data is the Octet String: the packet's user data.
	Data []byte

	// APID identifies the managed data path the octet string arrived on
	// (3.4.2.2).
	APID uint16

	// SecondaryHeaderIndicator reports whether the received packet carried a
	// Packet Secondary Header (3.4.2.3). It is a mandatory parameter, read
	// straight from the Secondary Header Flag. When the service has no
	// secondary header decoder configured, the header octets are at the front
	// of Data; with a decoder they have been stripped.
	SecondaryHeaderIndicator bool

	// DataLoss is the Data Loss Indicator (3.4.2.4): true when the Packet
	// Sequence Count for this APID skipped ahead, so packets were lost in
	// transmission. The parameter is optional; the continuity check behind it
	// is mandatory (4.3.2.2).
	DataLoss bool

	// PacketsLost is how many packets the count skipped, modulo 16384. It is
	// zero unless DataLoss is true.
	PacketsLost int
}

// ReceiveBytes reads a space packet from the transport and delivers it as an
// Octet String with the indication parameters of 3.4.3.3.2: the APID, the
// mandatory Secondary Header Indicator, and the optional Data Loss Indicator.
//
// When ServiceConfig.DiscardIdle is set, idle packets are dropped and the
// next real packet is delivered instead.
func (s *Service) ReceiveBytes() (Indication, error) {
	packet, lost, err := s.receive()
	if err != nil {
		return Indication{}, err
	}

	return Indication{
		Data:                     packet.UserData,
		APID:                     packet.PrimaryHeader.APID,
		SecondaryHeaderIndicator: packet.PrimaryHeader.SecondaryHeaderFlag == 1,
		DataLoss:                 lost > 0,
		PacketsLost:              lost,
	}, nil
}

// calculatePacketSize computes the total size of a space packet from a raw header dump.
func calculatePacketSize(header []byte) (int, error) {
	if len(header) < PrimaryHeaderSize {
		return 0, ErrDataTooShort
	}
	packetLength := binary.BigEndian.Uint16(header[4:6])
	return PrimaryHeaderSize + int(packetLength) + 1, nil
}
