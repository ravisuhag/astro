package cfdp

// LV is a Length-Value object per CCSDS 727.0-B-5 table 5-2: an 8-bit length
// followed by that many octets. Used for filenames, whose position in the PDU
// is fixed even though their length is not.
//
// A zero length means the value is absent, an empty filename, for instance,
// marks a transaction with no associated file (clause 5.2.5).
type LV struct {
	Value []byte
}

// Encode serializes the LV object.
func (l LV) Encode() ([]byte, error) {
	if len(l.Value) > 255 {
		return nil, ErrValueTooLong
	}
	out := make([]byte, 1, 1+len(l.Value))
	out[0] = byte(len(l.Value))
	return append(out, l.Value...), nil
}

// IsEmpty reports whether the LV carries no value.
func (l LV) IsEmpty() bool { return len(l.Value) == 0 }

// String returns the value as text, which is how filenames are carried.
func (l LV) String() string { return string(l.Value) }

// DecodeLV reads one LV object from the front of data, returning it and the
// number of octets consumed.
func DecodeLV(data []byte) (LV, int, error) {
	if len(data) < 1 {
		return LV{}, 0, ErrDataTooShort
	}
	n := int(data[0])
	if len(data) < 1+n {
		return LV{}, 0, ErrDataTooShort
	}
	if n == 0 {
		return LV{}, 1, nil
	}
	v := make([]byte, n)
	copy(v, data[1:1+n])
	return LV{Value: v}, 1 + n, nil
}

// TLVType identifies the kind of a TLV object, per clause 5.4.
type TLVType uint8

const (
	// TLVFilestoreRequest carries a filestore action to perform (clause 5.4.1).
	TLVFilestoreRequest TLVType = 0x00
	// TLVFilestoreResponse reports the outcome of one request (clause 5.4.2).
	TLVFilestoreResponse TLVType = 0x01
	// TLVMessageToUser carries an opaque application message (clause 5.4.3).
	TLVMessageToUser TLVType = 0x02
	// TLVFaultHandlerOverride changes the handler for one condition (clause 5.4.4).
	TLVFaultHandlerOverride TLVType = 0x04
	// TLVFlowLabel carries a mission-defined flow label (clause 5.4.5).
	TLVFlowLabel TLVType = 0x05
	// TLVEntityID carries an entity ID, used for fault location (clause 5.4.6).
	TLVEntityID TLVType = 0x06
)

// String names the TLV type.
func (t TLVType) String() string {
	switch t {
	case TLVFilestoreRequest:
		return "filestore request"
	case TLVFilestoreResponse:
		return "filestore response"
	case TLVMessageToUser:
		return "message to user"
	case TLVFaultHandlerOverride:
		return "fault handler override"
	case TLVFlowLabel:
		return "flow label"
	case TLVEntityID:
		return "entity ID"
	default:
		return "unknown"
	}
}

// TLV is a Type-Length-Value object per table 5-3. Unlike an LV, a TLV can sit
// anywhere in the PDU, because its type field says what it is.
type TLV struct {
	Type  TLVType
	Value []byte
}

// Encode serializes the TLV object.
func (t TLV) Encode() ([]byte, error) {
	if len(t.Value) > 255 {
		return nil, ErrValueTooLong
	}
	out := make([]byte, 2, 2+len(t.Value))
	out[0] = byte(t.Type)
	out[1] = byte(len(t.Value))
	return append(out, t.Value...), nil
}

// DecodeTLV reads one TLV object from the front of data, returning it and the
// number of octets consumed.
func DecodeTLV(data []byte) (TLV, int, error) {
	if len(data) < 2 {
		return TLV{}, 0, ErrDataTooShort
	}
	n := int(data[1])
	if len(data) < 2+n {
		return TLV{}, 0, ErrDataTooShort
	}
	t := TLV{Type: TLVType(data[0])}
	if n > 0 {
		t.Value = make([]byte, n)
		copy(t.Value, data[2:2+n])
	}
	return t, 2 + n, nil
}

// DecodeTLVs reads TLV objects until data runs out. A trailing partial object
// is an error, not a silent truncation.
func DecodeTLVs(data []byte) ([]TLV, error) {
	var out []TLV
	for offset := 0; offset < len(data); {
		t, n, err := DecodeTLV(data[offset:])
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		offset += n
	}
	return out, nil
}

// encodeTLVs serializes a run of TLV objects back to back.
func encodeTLVs(tlvs []TLV) ([]byte, error) {
	var out []byte
	for _, t := range tlvs {
		b, err := t.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// EntityIDTLV builds the entity ID TLV that carries a fault location (clause 5.4.6).
func EntityIDTLV(id EntityID) (TLV, error) {
	b, err := id.Encode()
	if err != nil {
		return TLV{}, err
	}
	return TLV{Type: TLVEntityID, Value: b}, nil
}

// AsEntityID reads an entity ID out of a TLV of type 06.
func (t TLV) AsEntityID() (EntityID, error) {
	if t.Type != TLVEntityID {
		return EntityID{}, ErrInvalidDirectiveCode
	}
	return decodeEntityID(t.Value, len(t.Value))
}
