package spp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

// PrimaryHeader represents the mandatory header section of a space packet.
type PrimaryHeader struct {
	Version             uint8  // Protocol version (3 bits)
	Type                uint8  // Packet type (1 bit)
	SecondaryHeaderFlag uint8  // Indicates if a secondary header is present (1 bit)
	APID                uint16 // Application Process Identifier (11 bits)
	SequenceFlags       uint8  // Packet sequence control flags (2 bits)
	SequenceCount       uint16 // Packet sequence number (14 bits)
	PacketLength        uint16 // Total packet length minus the primary header (16 bits)
}

// Encode serializes the PrimaryHeader into a 6-byte array.
func (ph *PrimaryHeader) Encode() ([]byte, error) {
	if err := ph.Validate(); err != nil {
		return nil, err
	}

	buf := make([]byte, 6)

	// Encode the first byte (Version, Type, SecondaryHeaderFlag, first 3 bits of APID)
	buf[0] = (ph.Version << 5) | (ph.Type << 4) | (ph.SecondaryHeaderFlag << 3) | uint8((ph.APID>>8)&0x07)

	// Encode the next byte (remaining 8 bits of APID)
	buf[1] = uint8(ph.APID & 0xFF)

	// Encode the third byte (SequenceFlags and first 6 bits of SequenceCount)
	buf[2] = (ph.SequenceFlags << 6) | uint8((ph.SequenceCount>>8)&0x3F)

	// Encode the fourth byte (remaining 8 bits of SequenceCount)
	buf[3] = uint8(ph.SequenceCount & 0xFF)

	// Encode the PacketLength (2 bytes)
	buf[4] = uint8(ph.PacketLength >> 8)   // Most significant byte
	buf[5] = uint8(ph.PacketLength & 0xFF) // Least significant byte
	return buf, nil
}

// Decode deserializes a 6-byte array into a PrimaryHeader.
func (ph *PrimaryHeader) Decode(data []byte) error {
	if len(data) < 6 {
		return errors.New("data too short to decode primary header")
	}

	// Decode the first byte (Version, Type, SecondaryHeaderFlag, first 3 bits of APID)
	ph.Version = data[0] >> 5
	ph.Type = (data[0] >> 4) & 0x01
	ph.SecondaryHeaderFlag = (data[0] >> 3) & 0x01
	ph.APID = uint16(data[0]&0x07)<<8 | uint16(data[1])

	// Decode the third byte (SequenceFlags and first 6 bits of SequenceCount)
	ph.SequenceFlags = data[2] >> 6
	ph.SequenceCount = uint16(data[2]&0x3F)<<8 | uint16(data[3])

	// Decode the PacketLength (2 bytes)
	ph.PacketLength = uint16(data[4])<<8 | uint16(data[5])

	return ph.Validate()
}

// Validate method for PrimaryHeader
func (ph *PrimaryHeader) Validate() error {
	if ph.Version > 7 {
		return errors.New("invalid Version: must be in range 0-7 (3 bits)")
	}
	if ph.Type > 1 {
		return errors.New("invalid Type: must be 0 or 1 (1 bit)")
	}
	if ph.SecondaryHeaderFlag > 1 {
		return errors.New("invalid SecondaryHeaderFlag: must be 0 or 1 (1 bit)")
	}
	if ph.APID > 2047 {
		return errors.New("invalid APID: must be in range 0-2047 (11 bits)")
	}
	if ph.SequenceFlags > 3 {
		return errors.New("invalid SequenceFlags: must be in range 0-3 (2 bits)")
	}
	if ph.SequenceCount > 16383 {
		return errors.New("invalid SequenceCount: must be in range 0-16383 (14 bits)")
	}
	// PacketLength is already a uint16, no need for further validation
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

// SecondaryHeader represents the optional customizable secondary header of a space packet.
type SecondaryHeader struct {
	Timestamp   uint64                 // Optional timestamp for mission-specific data
	OtherFields map[string]interface{} // Additional mission-specific fields
}

// Encode serializes the SecondaryHeader into a byte slice.
func (sh *SecondaryHeader) Encode() ([]byte, error) {
	if err := sh.Validate(); err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)

	// Encode Timestamp
	if err := binary.Write(buf, binary.BigEndian, sh.Timestamp); err != nil {
		return nil, err
	}

	// Encode the OtherFields
	for key, value := range sh.OtherFields {
		keyLen := uint16(len(key))
		if err := binary.Write(buf, binary.BigEndian, keyLen); err != nil {
			return nil, errors.New("failed to encode field key length")
		}
		if _, err := buf.WriteString(key); err != nil {
			return nil, errors.New("failed to encode field key")
		}

		switch v := value.(type) {
		case string:
			valueType := uint8(1) // Type 1 = string
			if err := binary.Write(buf, binary.BigEndian, valueType); err != nil {
				return nil, errors.New("failed to encode value type")
			}
			valueLen := uint16(len(v))
			if err := binary.Write(buf, binary.BigEndian, valueLen); err != nil {
				return nil, errors.New("failed to encode value length")
			}
			if _, err := buf.WriteString(v); err != nil {
				return nil, errors.New("failed to encode string value")
			}
		default:
			return nil, errors.New("unsupported field value type")
		}
	}

	return buf.Bytes(), nil
}

// Decode deserializes a byte slice into a SecondaryHeader.
func (sh *SecondaryHeader) Decode(data []byte) error {
	if len(data) < 8 {
		return errors.New("data too short to decode secondary header")
	}

	buf := bytes.NewReader(data)

	// Decode the timestamp
	if err := binary.Read(buf, binary.BigEndian, &sh.Timestamp); err != nil {
		return errors.New("failed to decode timestamp")
	}

	sh.OtherFields = make(map[string]interface{})

	// Decode the OtherFields
	for buf.Len() > 0 {
		var keyLen uint16
		if err := binary.Read(buf, binary.BigEndian, &keyLen); err != nil {
			return errors.New("failed to decode field key length")
		}
		key := make([]byte, keyLen)
		if _, err := buf.Read(key); err != nil {
			return errors.New("failed to decode field key")
		}

		var valueType uint8
		if err := binary.Read(buf, binary.BigEndian, &valueType); err != nil {
			return errors.New("failed to decode value type")
		}

		switch valueType {
		case 1: // String
			var valueLen uint16
			if err := binary.Read(buf, binary.BigEndian, &valueLen); err != nil {
				return errors.New("failed to decode value length")
			}
			value := make([]byte, valueLen)
			if _, err := buf.Read(value); err != nil {
				return errors.New("failed to decode string value")
			}
			sh.OtherFields[string(key)] = string(value)
		default:
			return errors.New("unsupported field value type")
		}
	}
	return sh.Validate()
}

// Validate method for SecondaryHeader
func (sh *SecondaryHeader) Validate() error {
	// Example validation: ensure timestamp is not zero
	if sh.Timestamp == 0 {
		return errors.New("invalid Timestamp: cannot be zero")
	}
	// Add more secondary header validation logic if needed
	return nil
}

// Humanize generates a human-readable representation of the SecondaryHeader.
func (sh *SecondaryHeader) Humanize() string {
	return strings.Join([]string{
		"  Timestamp: " + strconv.FormatUint(sh.Timestamp, 10),
		"  Other Fields: " + mapToString(sh.OtherFields),
	}, "\n")
}
