// Package sdls implements the Space Data Link Security Protocol
// per CCSDS 355.0-B-2 (July 2022).
//
// SDLS protects the data field of a TM, TC, AOS, or USLP Transfer Frame. It
// inserts a Security Header before the frame data and a Security Trailer
// carrying a Message Authentication Code after it. The carrier frame packages
// need no changes: the caller builds the protected data field with this
// package and hands the result to the frame constructor.
//
//	                  ┌──────────── carrier frame data field ────────────┐
//	frame header ...  │ Security Header │ data (or ciphertext) │ Trailer │
//	                  └──────────────────────────────────────────────────┘
//
// The wire layout of the Security Header is not self-describing. Field widths
// come from the Security Association named by the Security Parameter Index,
// which both ends agree on before the link opens (§2.3.1.4).
//
// This package ships the annex baselines: the AES-256-GCM authenticated
// encryption of §E1 (TM), §E3 (AOS) and §E4 (USLP), and the AES-CMAC
// authentication of §E2 (telecommand). It also offers GMAC — not an annex
// baseline itself, but the authentication-only companion of the GCM
// baselines. Pick between the two MAC algorithms with
// SecurityAssociation.AuthAlgorithm.
package sdls

import "encoding/binary"

// MaxSecurityHeaderSize is the largest permitted Security Header, per
// CCSDS 355.0-B-2 §4.1.1.1.4.
const MaxSecurityHeaderSize = 64

// SPISize is the width of the Security Parameter Index field in bytes
// (16 bits, per §4.1.1.2.1).
const SPISize = 2

// Reserved Security Parameter Index values, per §4.1.1.2.3.
const (
	// SPIReservedZero is reserved by CCSDS for future use.
	SPIReservedZero uint16 = 0
	// SPIReservedAllOnes is reserved by CCSDS for future use.
	SPIReservedAllOnes uint16 = 65535
)

// FieldLengths gives the octet widths of the managed Security Header and
// Security Trailer fields for one Security Association. Every width is fixed
// for the lifetime of the SA (§2.3.1.4, §4.2.2.5).
//
// A width of zero means the field is absent: §4.1.1.3.4 for the IV,
// §4.1.1.4.4 for the Sequence Number, §4.1.1.5.3 for Pad Length.
type FieldLengths struct {
	IV     int // Initialization Vector, §4.1.1.3
	SeqNum int // anti-replay Sequence Number, §4.1.1.4
	PadLen int // Pad Length, §4.1.1.5
	MAC    int // Message Authentication Code in the trailer, §4.1.2.3
}

// HeaderSize returns the encoded width of a Security Header with these field
// lengths, including the mandatory 2-octet SPI.
func (fl FieldLengths) HeaderSize() int {
	return SPISize + fl.IV + fl.SeqNum + fl.PadLen
}

// Validate reports whether the field widths are usable.
func (fl FieldLengths) Validate() error {
	if fl.IV < 0 || fl.SeqNum < 0 || fl.PadLen < 0 || fl.MAC < 0 {
		return ErrInvalidFieldLengths
	}
	if fl.HeaderSize() > MaxSecurityHeaderSize {
		return ErrHeaderTooLong
	}
	return nil
}

// SecurityHeader is the Security Header of §4.1.1: a mandatory SPI followed by
// three optional fields, contiguous and in this order.
type SecurityHeader struct {
	SPI       uint16
	IV        []byte // §4.1.1.3
	SeqNum    []byte // §4.1.1.4
	PadLength []byte // §4.1.1.5
}

// Encode serializes the Security Header. Field widths come from the values
// already stored on the struct, so build it with the SA's FieldLengths.
func (h *SecurityHeader) Encode() ([]byte, error) {
	size := SPISize + len(h.IV) + len(h.SeqNum) + len(h.PadLength)
	if size > MaxSecurityHeaderSize {
		return nil, ErrHeaderTooLong
	}

	out := make([]byte, 0, size)
	spi := make([]byte, SPISize)
	binary.BigEndian.PutUint16(spi, h.SPI)
	out = append(out, spi...)
	out = append(out, h.IV...)
	out = append(out, h.SeqNum...)
	out = append(out, h.PadLength...)
	return out, nil
}

// PadCount returns the Pad Length field as an integer. An absent field means
// no padding.
func (h *SecurityHeader) PadCount() int {
	n := 0
	for _, b := range h.PadLength {
		n = n<<8 | int(b)
	}
	return n
}

// DecodeSecurityHeader parses a Security Header from the front of data using
// the field widths of the Security Association that the SPI names. It returns
// the header and the number of octets consumed.
//
// The caller reads the SPI first (it is always the leading 2 octets), looks up
// the SA, and passes that SA's FieldLengths here.
func DecodeSecurityHeader(data []byte, fl FieldLengths) (*SecurityHeader, int, error) {
	if err := fl.Validate(); err != nil {
		return nil, 0, err
	}
	size := fl.HeaderSize()
	if len(data) < size {
		return nil, 0, ErrDataTooShort
	}

	h := &SecurityHeader{SPI: binary.BigEndian.Uint16(data[:SPISize])}
	offset := SPISize

	// Each field is copied out so the header never aliases the caller's buffer.
	h.IV = copySlice(data[offset : offset+fl.IV])
	offset += fl.IV
	h.SeqNum = copySlice(data[offset : offset+fl.SeqNum])
	offset += fl.SeqNum
	h.PadLength = copySlice(data[offset : offset+fl.PadLen])
	offset += fl.PadLen

	return h, offset, nil
}

// copySlice returns a copy of b, or nil when b is empty, so absent optional
// fields stay nil rather than becoming empty non-nil slices.
func copySlice(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Humanize returns a human-readable summary of the Security Header.
func (h *SecurityHeader) Humanize() string {
	return "SDLS Security Header" +
		"\n  SPI ......... " + itoa(int(h.SPI)) +
		"\n  IV .......... " + hexOrNone(h.IV) +
		"\n  Sequence .... " + hexOrNone(h.SeqNum) +
		"\n  Pad Length .. " + hexOrNone(h.PadLength)
}
