package pxdl

import (
	"encoding/binary"
	"fmt"
)

// Supervisory PDUs, per CCSDS 211.0-B-6 §3.2.4.
//
// SPDUs are the protocol talking to itself: link control words reporting what
// arrived, and directives changing how the link runs. They travel in P-frames,
// and only on the Expedited service.
//
// Two shapes, told apart by the leading bit:
//
//	fixed:     format ID '1' │ type ID (1 bit) │ data (14 bits)
//	variable:  format ID '0' │ type ID (3 bits) │ length (4 bits) │ data

// SPDU format identifiers, per §3.2.4.2.
const (
	// SPDUFormatVariable marks a variable-length SPDU ('0').
	SPDUFormatVariable uint8 = 0
	// SPDUFormatFixed marks a 16-bit fixed-length SPDU ('1').
	SPDUFormatFixed uint8 = 1
)

// FixedSPDUSize is the width of a fixed-length SPDU in octets (§3.2.4.2.1).
const FixedSPDUSize = 2

// Fixed-length SPDU type identifiers, per table 3-5.
const (
	// FixedTypePLCW identifies a Proximity Link Control Word ('0').
	FixedTypePLCW uint8 = 0
	// FixedTypeReserved is reserved for future CCSDS specification ('1').
	FixedTypeReserved uint8 = 1
)

// MaxVariableSPDUData is the largest variable-length SPDU data field, bounded
// by its 4-bit length field (§3.2.4.2.2).
const MaxVariableSPDUData = 15

// PLCW is the Proximity Link Control Word, the Type F1 fixed-length SPDU of
// §3.2.4.3.2.
//
// It is Proximity-1's acknowledgement: the receiver reports which frame it
// expects next, so the sender knows what got through. The same job COP-1's
// CLCW does for TC links.
//
// Sixteen bits, described in figure 3-5 from bit 0:
//
//	format ID(1) │ type ID(1) │ retransmit(1) │ PCID(1) │ spare(1) │
//	expedited frame counter(3) │ report value(8)
type PLCW struct {
	// RetransmitFlag says the receiver is missing frames and wants them again.
	RetransmitFlag bool
	// PCID names the physical channel this report covers.
	PCID uint8
	// ExpeditedFrameCounter counts frames received on the Expedited service,
	// 3 bits.
	ExpeditedFrameCounter uint8
	// ReportValue is V(R): the sequence number the receiver expects next
	// (§3.2.4.3.2.2.2).
	ReportValue uint8
}

// Validate checks the PLCW's field widths.
func (p *PLCW) Validate() error {
	if p.PCID > 1 {
		return ErrInvalidPCID
	}
	if p.ExpeditedFrameCounter > 0x07 {
		return ErrInvalidSPDU
	}
	return nil
}

// Encode serializes the PLCW into two octets.
func (p *PLCW) Encode() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var v uint16
	v |= uint16(SPDUFormatFixed) << 15
	v |= uint16(FixedTypePLCW) << 14
	if p.RetransmitFlag {
		v |= 1 << 13
	}
	v |= uint16(p.PCID&0x01) << 12
	// Bit 11 is the reserved spare, left zero.
	v |= uint16(p.ExpeditedFrameCounter&0x07) << 8
	v |= uint16(p.ReportValue)

	return appendUint16(nil, v), nil
}

// DecodePLCW parses a Proximity Link Control Word.
func DecodePLCW(data []byte) (*PLCW, error) {
	if len(data) < FixedSPDUSize {
		return nil, ErrDataTooShort
	}
	v := binary.BigEndian.Uint16(data[:FixedSPDUSize])

	if format := uint8(v >> 15 & 0x01); format != SPDUFormatFixed {
		return nil, ErrInvalidSPDU
	}
	if typeID := uint8(v >> 14 & 0x01); typeID != FixedTypePLCW {
		return nil, ErrInvalidSPDU
	}

	return &PLCW{
		RetransmitFlag:        v&(1<<13) != 0,
		PCID:                  uint8(v >> 12 & 0x01),
		ExpeditedFrameCounter: uint8(v >> 8 & 0x07),
		ReportValue:           uint8(v),
	}, nil
}

// Humanize returns a human-readable summary.
func (p *PLCW) Humanize() string {
	return fmt.Sprintf("Proximity Link Control Word\n"+
		"  Report value (V(R)) .... %d\n"+
		"  Retransmit ............. %t\n"+
		"  PCID ................... %d\n"+
		"  Expedited frame count .. %d",
		p.ReportValue, p.RetransmitFlag, p.PCID, p.ExpeditedFrameCounter)
}

// VariableSPDU is a variable-length supervisory PDU, per §3.2.4.2.2.
//
// One octet of header — a zero format bit, a 3-bit type, and a 4-bit length —
// then up to 15 octets of directives or status reports, all of the same type.
//
// Note the length field is the actual octet count, not a count-less-one. The
// standard calls that out explicitly, presumably because everything else in
// CCSDS goes the other way.
type VariableSPDU struct {
	// TypeID selects the kind of directive or report, 3 bits. Annex B
	// specifies the types; this package carries the payload without
	// interpreting it.
	TypeID uint8
	// Data holds one or more directives or reports of that type.
	Data []byte
}

// Validate checks the SPDU's field widths.
func (s *VariableSPDU) Validate() error {
	if s.TypeID > 0x07 {
		return ErrInvalidSPDU
	}
	if len(s.Data) > MaxVariableSPDUData {
		return ErrSPDUDataTooLarge
	}
	return nil
}

// Encode serializes the variable-length SPDU.
func (s *VariableSPDU) Encode() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	// Format ID '0' in the top bit, then type, then the real length.
	head := s.TypeID & 0x07 << 4
	head |= byte(len(s.Data)) & 0x0F
	return append([]byte{head}, s.Data...), nil
}

// DecodeVariableSPDU parses a variable-length SPDU from the front of data,
// returning it and the octets consumed.
func DecodeVariableSPDU(data []byte) (*VariableSPDU, int, error) {
	if len(data) < 1 {
		return nil, 0, ErrDataTooShort
	}
	if format := data[0] >> 7 & 0x01; format != SPDUFormatVariable {
		return nil, 0, ErrInvalidSPDU
	}

	s := &VariableSPDU{TypeID: data[0] >> 4 & 0x07}
	length := int(data[0] & 0x0F)

	if len(data) < 1+length {
		return nil, 0, ErrDataTooShort
	}
	if length > 0 {
		s.Data = make([]byte, length)
		copy(s.Data, data[1:1+length])
	}
	return s, 1 + length, nil
}

// Humanize returns a human-readable summary.
func (s *VariableSPDU) Humanize() string {
	return fmt.Sprintf("Variable-Length SPDU\n  Type ..... %d\n  Length ... %d octets",
		s.TypeID, len(s.Data))
}

// SPDU is one supervisory PDU of either shape. Exactly one field is set.
type SPDU struct {
	PLCW     *PLCW
	Variable *VariableSPDU
}

// Encode serializes whichever kind this is.
func (s *SPDU) Encode() ([]byte, error) {
	switch {
	case s.PLCW != nil:
		return s.PLCW.Encode()
	case s.Variable != nil:
		return s.Variable.Encode()
	default:
		return nil, ErrInvalidSPDU
	}
}

// DecodeSPDUs parses the run of supervisory PDUs in a P-frame's data field.
//
// §3.2.4.1: SPDUs are self-identifying and self-delimiting, so a decoder can
// walk them without being told how many there are.
func DecodeSPDUs(data []byte) ([]SPDU, error) {
	var out []SPDU

	for offset := 0; offset < len(data); {
		format := data[offset] >> 7 & 0x01

		if format == SPDUFormatFixed {
			plcw, err := DecodePLCW(data[offset:])
			if err != nil {
				return nil, err
			}
			out = append(out, SPDU{PLCW: plcw})
			offset += FixedSPDUSize
			continue
		}

		variable, n, err := DecodeVariableSPDU(data[offset:])
		if err != nil {
			return nil, err
		}
		out = append(out, SPDU{Variable: variable})
		offset += n
	}
	return out, nil
}

// EncodeSPDUs serializes a run of supervisory PDUs into a P-frame data field.
func EncodeSPDUs(spdus []SPDU) ([]byte, error) {
	var out []byte
	for _, s := range spdus {
		encoded, err := s.Encode()
		if err != nil {
			return nil, err
		}
		out = append(out, encoded...)
	}
	return out, nil
}

// SPDUs parses the supervisory PDUs a P-frame carries.
func (f *TransferFrame) SPDUs() ([]SPDU, error) {
	if !f.IsSupervisoryFrame() {
		return nil, ErrNotSupervisoryFrame
	}
	return DecodeSPDUs(f.DataField)
}
