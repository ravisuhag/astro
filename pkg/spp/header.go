package spp

import (
	"strconv"
	"strings"
)

const PrimaryHeaderSize = 6 // CCSDS primary header is always 6 bytes

// Packet types per CCSDS 133.0-B-2.
const (
	PacketTypeTM uint8 = 0 // Telemetry
	PacketTypeTC uint8 = 1 // Telecommand
)

// Sequence flags per CCSDS 133.0-B-2.
const (
	SeqFlagContinuation uint8 = 0 // Continuation segment
	SeqFlagFirstSegment uint8 = 1 // First segment
	SeqFlagLastSegment  uint8 = 2 // Last segment
	SeqFlagUnsegmented  uint8 = 3 // Unsegmented (standalone)
)

// SecondaryHeader is implemented by mission-specific secondary headers.
// The CCSDS standard defines the existence and size constraints of the
// secondary header (1-63 octets), but its format is mission-defined.
type SecondaryHeader interface {
	// Encode serializes the secondary header into bytes.
	Encode() ([]byte, error)
	// Decode deserializes bytes into the secondary header.
	Decode([]byte) error
	// Size returns the fixed size in bytes of the encoded secondary header.
	Size() int
}

// PrimaryHeader represents the mandatory 6-byte header of a CCSDS space packet.
type PrimaryHeader struct {
	Version             uint8  // Packet version number (3 bits, must be 0 for CCSDS v1)
	Type                uint8  // Packet type: 0 = TM, 1 = TC (1 bit)
	SecondaryHeaderFlag uint8  // Indicates if a secondary header is present (1 bit)
	APID                uint16 // Application Process Identifier (11 bits, 0-2047)
	SequenceFlags       uint8  // Sequence flags (2 bits)
	SequenceCount       uint16 // Packet sequence count (14 bits, 0-16383)
	PacketLength        uint16 // Packet data field length minus 1 (16 bits)
}

// Encode serializes the PrimaryHeader into a 6-byte array.
func (ph *PrimaryHeader) Encode() ([]byte, error) {
	if err := ph.Validate(); err != nil {
		return nil, err
	}

	buf := make([]byte, 6)
	buf[0] = (ph.Version << 5) | (ph.Type << 4) | (ph.SecondaryHeaderFlag << 3) | uint8((ph.APID>>8)&0x07)
	buf[1] = uint8(ph.APID & 0xFF)
	buf[2] = (ph.SequenceFlags << 6) | uint8((ph.SequenceCount>>8)&0x3F)
	buf[3] = uint8(ph.SequenceCount & 0xFF)
	buf[4] = uint8(ph.PacketLength >> 8)
	buf[5] = uint8(ph.PacketLength & 0xFF)
	return buf, nil
}

// Decode deserializes a 6-byte array into a PrimaryHeader.
func (ph *PrimaryHeader) Decode(data []byte) error {
	if len(data) < 6 {
		return ErrDataTooShort
	}

	ph.Version = data[0] >> 5
	ph.Type = (data[0] >> 4) & 0x01
	ph.SecondaryHeaderFlag = (data[0] >> 3) & 0x01
	ph.APID = uint16(data[0]&0x07)<<8 | uint16(data[1])
	ph.SequenceFlags = data[2] >> 6
	ph.SequenceCount = uint16(data[2]&0x3F)<<8 | uint16(data[3])
	ph.PacketLength = uint16(data[4])<<8 | uint16(data[5])

	return ph.Validate()
}

// Validate checks that all fields conform to CCSDS 133.0-B-2.
func (ph *PrimaryHeader) Validate() error {
	if ph.Version != 0 {
		return ErrInvalidVersion
	}
	if ph.Type > 1 {
		return ErrInvalidType
	}
	if ph.SecondaryHeaderFlag > 1 {
		return ErrInvalidHeader
	}
	if ph.APID > 2047 {
		return ErrInvalidAPID
	}
	if ph.SequenceFlags > 3 {
		return ErrInvalidSequenceFlags
	}
	if ph.SequenceCount > 16383 {
		return ErrInvalidSequenceCount
	}
	return nil
}

// Humanize generates a human-readable representation of the PrimaryHeader.
func (ph *PrimaryHeader) Humanize() string {
	return strings.Join([]string{
		"  Version: " + strconv.Itoa(int(ph.Version)),
		"  Type: " + strconv.Itoa(int(ph.Type)),
		"  Secondary Header Flag: " + strconv.Itoa(int(ph.SecondaryHeaderFlag)),
		"  APID: " + strconv.Itoa(int(ph.APID)),
		"  Sequence Flags: " + strconv.Itoa(int(ph.SequenceFlags)),
		"  Sequence Count: " + strconv.Itoa(int(ph.SequenceCount)),
		"  Packet Length: " + strconv.Itoa(int(ph.PacketLength)),
	}, "\n")
}

// validateSecondaryHeader checks the CCSDS structural constraints on a secondary header.
func validateSecondaryHeader(sh SecondaryHeader) error {
	size := sh.Size()
	if size < 1 {
		return ErrSecondaryHeaderTooSmall
	}
	if size > 63 {
		return ErrSecondaryHeaderTooLarge
	}
	return nil
}
