package tcf

// PField represents the preamble field of a CCSDS time code.
// The P-field identifies the time code format, precision, and epoch level.
//
// First octet layout:
// +---+-------+-------------------+
// | E | ID(3) | Format-specific(4)|
// +---+-------+-------------------+
//
// Second octet (if extension flag is set):
// +---+-------------------------------+
// | 0 | Extension-specific (7 bits)   |
// +---+-------------------------------+
type PField struct {
	Extension  bool  // Extension flag: true if a second octet follows
	TimeCodeID uint8 // Time code identification (3 bits)
	Detail     uint8 // Format-specific detail bits (4 bits from first octet)
	ExtDetail  uint8 // Extension detail bits (7 bits from second octet)
}

// Encode serializes the PField into 1 or 2 bytes.
func (p *PField) Encode() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var ext uint8
	if p.Extension {
		ext = 1
	}
	first := (ext << 7) | (p.TimeCodeID << 4) | (p.Detail & 0x0F)

	if !p.Extension {
		return []byte{first}, nil
	}

	second := p.ExtDetail & 0x7F
	return []byte{first, second}, nil
}

// Decode deserializes 1 or 2 bytes into a PField.
func (p *PField) Decode(data []byte) error {
	if len(data) < 1 {
		return ErrDataTooShort
	}

	p.Extension = (data[0] >> 7) == 1
	p.TimeCodeID = (data[0] >> 4) & 0x07
	p.Detail = data[0] & 0x0F

	if p.Extension {
		if len(data) < 2 {
			return ErrDataTooShort
		}
		p.ExtDetail = data[1] & 0x7F
	}

	return p.Validate()
}

// Size returns the encoded size of the PField in bytes.
func (p *PField) Size() int {
	if p.Extension {
		return 2
	}
	return 1
}

// Validate checks that the PField conforms to CCSDS 301.0-B-4.
func (p *PField) Validate() error {
	if p.TimeCodeID > 7 {
		return ErrInvalidPField
	}
	if p.Detail > 0x0F {
		return ErrInvalidPField
	}
	if p.Extension && p.ExtDetail > 0x7F {
		return ErrInvalidPField
	}
	return nil
}
