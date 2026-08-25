package epp

import "io"

// Service provides packet send/receive operations over a shared transport
// for Encapsulation Packets per CCSDS 133.1-B-3.
type Service struct {
	rw           io.ReadWriter
	maxPacketLen int64
}

// ServiceConfig holds configuration for a Service.
type ServiceConfig struct {
	// MaxPacketLength is the maximum total packet size in octets.
	// It defaults to MaxPacketLength8 (4,294,967,295), the largest packet
	// the protocol can represent, so that spec-valid packets are never
	// rejected unless the mission configures a lower limit.
	MaxPacketLength int
}

// NewService creates a new EPP service over the given transport.
func NewService(rw io.ReadWriter, cfg ServiceConfig) *Service {
	maxLen := int64(cfg.MaxPacketLength)
	if maxLen <= 0 || maxLen > MaxPacketLength8 {
		maxLen = MaxPacketLength8
	}
	return &Service{
		rw:           rw,
		maxPacketLen: maxLen,
	}
}

// SendPacket writes a pre-built encapsulation packet to the transport.
func (s *Service) SendPacket(packet *EncapsulationPacket) error {
	if packet == nil {
		return ErrNilPacket
	}

	data, err := packet.Encode()
	if err != nil {
		return err
	}
	if int64(len(data)) > s.maxPacketLen {
		return ErrPacketTooLarge
	}
	_, err = s.rw.Write(data)
	return err
}

// ReceivePacket reads and decodes a complete encapsulation packet from the transport.
func (s *Service) ReceivePacket() (*EncapsulationPacket, error) {
	// Read the first byte to determine the header size
	first := make([]byte, 1)
	if _, err := io.ReadFull(s.rw, first); err != nil {
		return nil, err
	}

	hdrSize := HeaderSize(first)
	if hdrSize < 0 {
		return nil, ErrInvalidPVN
	}

	// Read remaining header bytes if needed
	headerBuf := make([]byte, hdrSize)
	headerBuf[0] = first[0]
	if hdrSize > 1 {
		if _, err := io.ReadFull(s.rw, headerBuf[1:]); err != nil {
			return nil, err
		}
	}

	// Decode header to get total packet length
	var header Header
	if err := header.Decode(headerBuf); err != nil {
		return nil, err
	}

	// 1-octet idle packet: no data to read
	if header.LengthOfLength == LoLNone {
		return &EncapsulationPacket{Header: header}, nil
	}

	totalSize := int64(header.PacketLength)
	if totalSize > s.maxPacketLen {
		return nil, ErrPacketTooLarge
	}

	// Read the data zone
	dataSize := totalSize - int64(hdrSize)
	if dataSize < 0 {
		return nil, ErrPacketLengthMismatch
	}

	dataBuf := make([]byte, dataSize)
	if dataSize > 0 {
		if _, err := io.ReadFull(s.rw, dataBuf); err != nil {
			return nil, err
		}
	}

	ep := &EncapsulationPacket{
		Header: header,
		Data:   dataBuf,
	}

	if err := ep.Validate(); err != nil {
		return nil, err
	}

	return ep, nil
}

// SendBytes wraps the given data in an encapsulation packet and writes it
// to the transport. The caller provides raw bytes and protocol ID; EPP
// handles packet construction.
func (s *Service) SendBytes(protocolID uint8, data []byte, opts ...PacketOption) error {
	packet, err := NewPacket(protocolID, data, opts...)
	if err != nil {
		return err
	}
	return s.SendPacket(packet)
}

// ReceiveBytes reads an encapsulation packet from the transport and returns
// the Protocol ID and data zone, stripping away the packet structure.
func (s *Service) ReceiveBytes() (protocolID uint8, data []byte, err error) {
	packet, err := s.ReceivePacket()
	if err != nil {
		return 0, nil, err
	}
	return packet.Header.ProtocolID, packet.Data, nil
}
