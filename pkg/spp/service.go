package spp

import (
	"io"
	"sync"
)

// Service provides both the Packet Service (CCSDS 3.3) and the Octet String
// Service (CCSDS 3.4) over a shared transport.
type Service struct {
	rw           io.ReadWriter
	packetType   uint8
	maxPacketLen int
	sh           SecondaryHeader // optional decoder for inbound packets
	errorControl bool            // expect error control field on received packets
	mu           sync.Mutex
	counters     map[uint16]uint16 // per-APID sequence counters
}

// ServiceConfig holds configuration for a Service.
type ServiceConfig struct {
	PacketType      uint8           // PacketTypeTM or PacketTypeTC
	MaxPacketLength int             // maximum total packet size in octets; default 65542
	SecondaryHeader SecondaryHeader // optional decoder for received secondary headers
	ErrorControl    bool            // if true, received packets are expected to contain a trailing CRC
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
		sh:           cfg.SecondaryHeader,
		errorControl: cfg.ErrorControl,
		counters:     make(map[uint16]uint16),
	}
}

// --- Packet Service (CCSDS 3.3) ---

// SendPacket writes a pre-built space packet to the transport.
// It stamps the packet with the next sequence count for its APID
// per CCSDS 133.0-B-2 Section 4.1.3.5.
func (s *Service) SendPacket(packet *SpacePacket) error {
	if packet == nil {
		return ErrNilPacket
	}

	s.mu.Lock()
	apid := packet.PrimaryHeader.APID
	packet.PrimaryHeader.SequenceCount = s.counters[apid]
	s.counters[apid] = (s.counters[apid] + 1) & 0x3FFF
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
func (s *Service) ReceivePacket() (*SpacePacket, error) {
	header := make([]byte, PrimaryHeaderSize)
	if _, err := io.ReadFull(s.rw, header); err != nil {
		return nil, err
	}

	totalPacketSize, err := CalculatePacketSize(header)
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
	if s.sh != nil {
		opts = append(opts, WithDecodeSecondaryHeader(s.sh))
	}
	if s.errorControl {
		opts = append(opts, WithDecodeErrorControl())
	}
	return Decode(buffer, opts...)
}

// --- Octet String Service (CCSDS 3.4) ---

// SendOption configures optional parameters for SendBytes.
type SendOption func(*sendConfig)

type sendConfig struct {
	sh           SecondaryHeader
	errorControl bool
}

// WithSendSecondaryHeader attaches a secondary header to the outgoing packet.
func WithSendSecondaryHeader(sh SecondaryHeader) SendOption {
	return func(cfg *sendConfig) { cfg.sh = sh }
}

// WithSendErrorControl enables CRC-16-CCITT error control on the outgoing packet.
// The checksum is computed automatically during encoding.
func WithSendErrorControl() SendOption {
	return func(cfg *sendConfig) { cfg.errorControl = true }
}

// SendBytes wraps the given data in a space packet and writes it to the transport.
// The caller provides raw bytes and service parameters; SPP handles packet construction.
func (s *Service) SendBytes(apid uint16, data []byte, opts ...SendOption) error {
	var cfg sendConfig
	for _, o := range opts {
		o(&cfg)
	}

	var pktOpts []PacketOption
	if cfg.sh != nil {
		pktOpts = append(pktOpts, WithSecondaryHeader(cfg.sh))
	}
	if cfg.errorControl {
		pktOpts = append(pktOpts, WithErrorControl())
	}

	packet, err := NewSpacePacket(apid, s.packetType, data, pktOpts...)
	if err != nil {
		return err
	}

	return s.SendPacket(packet)
}

// ReceiveBytes reads a space packet from the transport and returns the APID
// and user data, stripping away the packet structure.
func (s *Service) ReceiveBytes() (apid uint16, data []byte, err error) {
	packet, err := s.ReceivePacket()
	if err != nil {
		return 0, nil, err
	}
	return packet.PrimaryHeader.APID, packet.UserData, nil
}

