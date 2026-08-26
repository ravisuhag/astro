package spp

import (
	"encoding/binary"
	"io"
	"sync"
)

// Service provides both the Packet Service (CCSDS 3.3) and the Octet String
// Service (CCSDS 3.4) over a shared transport.
//
// A Service is safe for concurrent use. Sends are serialized against each
// other and receives against each other, so a sequence count is allocated and
// its packet written as one step and a packet's header and body are read as
// one step. Sending and receiving proceed independently, which is what a
// full-duplex transport wants.
//
// Serializing sends is not a convenience: 4.1.3.4.3.4 requires the Packet
// Sequence Count to be continuous modulo 16384, and allocating counts under a
// lock while writing outside it would let two senders put their packets on the
// wire in the opposite order to the counts they were given.
type Service struct {
	rw           io.ReadWriter
	packetType   uint8
	maxPacketLen int
	newSH        func() SecondaryHeader // optional decoder factory for inbound packets
	errorControl bool                   // expect error control field on received packets
	discardIdle  bool                   // drop received idle packets instead of delivering them

	// sendMu covers a whole send: count allocation, encoding, and the write.
	// recvMu covers a whole receive: the header read, the body read, and the
	// decode. Both may be held while taking mu; mu is never held while taking
	// either, so there is no cycle.
	sendMu sync.Mutex
	recvMu sync.Mutex

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
	// up showing the newest packet's header. It also has to be a factory
	// rather than a type the service copies for itself: the width of a
	// mission's secondary header usually lives in the value (a PUS header
	// reads it from its mission profile), so only the caller can build one
	// that is configured correctly.
	NewSecondaryHeader func() SecondaryHeader

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

	return &Service{
		rw:           rw,
		packetType:   cfg.PacketType,
		maxPacketLen: maxLen,
		newSH:        cfg.NewSecondaryHeader,
		errorControl: cfg.ErrorControl,
		discardIdle:  cfg.DiscardIdle,
		counters:     make(map[uint16]uint16),
		expected:     make(map[uint16]uint16),
		seen:         make(map[uint16]bool),
	}
}

// --- Packet Service (CCSDS 3.3) ---

// SendPacket writes a pre-built space packet to the transport. It is the
// PACKET.request primitive of 3.3.3.2.
//
// A packet whose count the caller owns is sent with that count untouched, and
// the service resynchronizes its own counter for the APID to one past it.
// 4.1.3.4.3.4 requires the count to be continuous modulo 16384; if the counter
// kept its old value, an APID that sent one such packet would emit a jump out
// and a jump back — 0, 1, 2, 1234, 3, 4 — and a receiver would read that as
// two losses. Two kinds of packet own their count:
//
//   - one built with WithSequenceCount, where the caller said which count to
//     use;
//   - one returned by Decode, which already carries the count the originating
//     application assigned it (4.1.3.4.3.3). 3.3.1 requires Packet Service
//     SDUs to be transferred "without further formatting" and the Packet
//     Transfer Function of 4.2.3 does not renumber, so a relay forwards what
//     it received rather than stamping over it.
//
// Any other packet is stamped with the next count for its APID (4.1.3.4.3),
// which mutates the caller's packet in place.
func (s *Service) SendPacket(packet *SpacePacket) error {
	if packet == nil {
		return ErrNilPacket
	}

	// One send at a time. The count and the octets that carry it have to reach
	// the transport together, or concurrent senders would interleave and break
	// the continuity 4.1.3.4.3.4 requires.
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	apid := packet.PrimaryHeader.APID
	packetCountBefore := packet.PrimaryHeader.SequenceCount

	s.mu.Lock()
	counterBefore, counterExisted := s.counters[apid]
	if packet.seqCountAuthoritative {
		s.counters[apid] = (packet.PrimaryHeader.SequenceCount + 1) & 0x3FFF
	} else {
		packet.PrimaryHeader.SequenceCount = s.counters[apid]
		s.counters[apid] = (s.counters[apid] + 1) & 0x3FFF
	}
	s.mu.Unlock()

	data, err := packet.Encode()
	if err == nil && len(data) > s.maxPacketLen {
		err = ErrPacketTooLarge
	}
	if err != nil {
		// Nothing reached the transport, so the count this packet was handed
		// was never spent. Put the counter and the packet back as they were,
		// or a rejected send would leave a hole in the APID's sequence.
		s.mu.Lock()
		if counterExisted {
			s.counters[apid] = counterBefore
		} else {
			delete(s.counters, apid)
		}
		s.mu.Unlock()
		packet.PrimaryHeader.SequenceCount = packetCountBefore
		return err
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
	packet, _, _, err := s.receive()
	return packet, err
}

// receive reads the next deliverable packet, the raw octets it arrived as, and
// the gap that preceded it.
func (s *Service) receive() (*SpacePacket, []byte, int, error) {
	// One receive at a time. A packet's header and body are two reads off the
	// transport, and a second reader landing between them would splice two
	// packets together.
	s.recvMu.Lock()
	defer s.recvMu.Unlock()

	for {
		packet, raw, err := s.readPacket()
		if err != nil {
			return nil, nil, 0, err
		}
		gap := s.trackContinuity(packet.PrimaryHeader.APID, packet.PrimaryHeader.SequenceCount)
		if s.discardIdle && packet.IsIdle() {
			continue
		}
		return packet, raw, gap, nil
	}
}

// readPacket reads exactly one packet off the transport and returns it along
// with the octets it was read from. The caller must hold recvMu.
func (s *Service) readPacket() (*SpacePacket, []byte, error) {
	header := make([]byte, PrimaryHeaderSize)
	if _, err := io.ReadFull(s.rw, header); err != nil {
		return nil, nil, err
	}

	totalPacketSize, err := calculatePacketSize(header)
	if err != nil {
		return nil, nil, err
	}

	if totalPacketSize > s.maxPacketLen {
		// The header is already off the transport. Leaving its body behind
		// would resynchronize the reader onto the middle of this packet, where
		// it would read fill or payload as a primary header and deliver
		// packets that were never sent. Skip the body so the next read starts
		// on a real packet boundary. The length is bounded by the 16-bit
		// length field, so this cannot be made to skip more than 65536 octets.
		if _, derr := io.CopyN(io.Discard, s.rw, int64(totalPacketSize-PrimaryHeaderSize)); derr != nil {
			return nil, nil, derr
		}
		return nil, nil, ErrPacketTooLarge
	}

	buffer := make([]byte, totalPacketSize)
	copy(buffer[:PrimaryHeaderSize], header)
	if _, err := io.ReadFull(s.rw, buffer[PrimaryHeaderSize:]); err != nil {
		return nil, nil, err
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
	packet, err := Decode(buffer, opts...)
	if err != nil {
		return nil, nil, err
	}
	return packet, buffer, nil
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
	shIndicator  bool
	errorControl bool
	packetType   *uint8
	seqCount     *uint16
}

// WithSendSecondaryHeader builds the outgoing packet's secondary header from a
// SecondaryHeader implementation, which is encoded ahead of the octet string.
//
// It sets the Secondary Header Indicator of 3.4.2.3 as a side effect. Use
// WithSendSecondaryHeaderIndicator when the octet string you are passing to
// SendBytes already begins with the header octets.
func WithSendSecondaryHeader(sh SecondaryHeader) SendOption {
	return func(cfg *sendConfig) { cfg.sh = sh }
}

// WithSendSecondaryHeaderIndicator is the Secondary Header Indicator parameter
// of the OCTET_STRING.request primitive (3.4.2.3, 3.4.3.2.2).
//
// Per 3.4.2.3.2 the parameter is a signal, not a header: the service user
// says whether a Packet Secondary Header is contained at the start of the
// octet string it is handing over, and the Packet Assembly Function sets the
// Secondary Header Flag to match (3.4.2.3.3, 4.2.2.3). Nothing in this layer
// has to interpret the octets, so no SecondaryHeader implementation is needed
// — which is what lets a user who holds a pre-formatted data field send it.
func WithSendSecondaryHeaderIndicator(present bool) SendOption {
	return func(cfg *sendConfig) { cfg.shIndicator = present }
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
	if cfg.shIndicator {
		// Rejected as ErrSecondaryHeaderTwice if a header was also supplied:
		// counting it twice would declare a data field longer than the packet
		// carries.
		pktOpts = append(pktOpts, WithSecondaryHeaderIndicator(true))
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
	// Data is the Octet String: the Packet Data Field, which is what is left
	// once the Packet Extraction Function removes the Packet Primary Header
	// (4.3.2.2).
	//
	// When the packet carried a Packet Secondary Header its octets lead Data,
	// whether or not a decoder was configured, because 4.3.2.2 defines
	// SecondaryHeaderIndicator as reporting a secondary header "at the start
	// of the Octet String" and the two have to agree. A configured decoder
	// does not remove them from here; it fills SecondaryHeader in as well.
	//
	// The one thing Data omits is the error control field, when the service
	// was configured to expect one. Those two octets are consumed and checked
	// by this layer and are not part of the octet string the user sent.
	Data []byte

	// APID identifies the managed data path the octet string arrived on
	// (3.4.2.2).
	APID uint16

	// SecondaryHeaderIndicator reports whether a Packet Secondary Header leads
	// Data (3.4.2.3, 4.3.2.2). It is a mandatory parameter, read straight from
	// the Secondary Header Flag.
	SecondaryHeaderIndicator bool

	// SecondaryHeader is the parsed secondary header, when the service had a
	// ServiceConfig.NewSecondaryHeader factory and the flag was set. It is nil
	// otherwise. This is a convenience beyond the primitive of 3.4.3.3.2 —
	// the octets themselves are always at the front of Data.
	SecondaryHeader SecondaryHeader

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
// The Octet String is the Packet Data Field, per the Packet Extraction
// Function of 4.3.2.2: "extract Octet Strings by removing the Packet Primary
// Header". Any secondary header octets stay at the front of it, which is what
// the Secondary Header Indicator is there to announce.
//
// When ServiceConfig.DiscardIdle is set, idle packets are dropped and the
// next real packet is delivered instead.
func (s *Service) ReceiveBytes() (Indication, error) {
	packet, raw, lost, err := s.receive()
	if err != nil {
		return Indication{}, err
	}

	// raw is a buffer readPacket allocated for this packet alone and holds the
	// whole packet, so the data field is everything past the primary header.
	// Decode has already checked that the buffer is at least as long as the
	// declared packet and, when error control is expected, that the data field
	// has room for it.
	field := raw[PrimaryHeaderSize:]
	if s.errorControl {
		field = field[:len(field)-2]
	}

	return Indication{
		Data:                     field,
		APID:                     packet.PrimaryHeader.APID,
		SecondaryHeaderIndicator: packet.PrimaryHeader.SecondaryHeaderFlag == 1,
		SecondaryHeader:          packet.SecondaryHeader,
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
