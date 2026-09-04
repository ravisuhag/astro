package bp

import "hash/crc32"

// The two checksums RFC 9171 clause 4.2.1 allows on a block.
//
// Neither is the CRC that pkg/crc computes. That package carries the CCSDS
// checksums, and the CCSDS CRC-16 differs from the one here in three ways at
// once: it does not reflect its input or output, and it applies no final XOR.
// Feeding a bundle to it produces a plausible-looking wrong answer, so this
// package keeps its own rather than reaching for the shared one.

// CRCType names the checksum on a block (RFC 9171 clause 4.2.1).
type CRCType uint64

const (
	// CRCNone means no checksum is present. The primary block may use it only
	// when a BPSec integrity block covers that block instead
	// (RFC 9171 clause 4.3.1).
	CRCNone CRCType = 0
	// CRC16X25 is the standard X-25 CRC-16, two octets.
	CRC16X25 CRCType = 1
	// CRC32C is the Castagnoli CRC-32, four octets.
	CRC32C CRCType = 2
)

// size returns the octet width of the checksum, or zero for CRCNone.
func (c CRCType) size() int {
	switch c {
	case CRC16X25:
		return 2
	case CRC32C:
		return 4
	}
	return 0
}

// valid reports whether the type code is one RFC 9171 clause 4.2.1 defines.
// The clause says these values "and no others" are valid.
func (c CRCType) valid() bool {
	return c == CRCNone || c == CRC16X25 || c == CRC32C
}

// crc16X25Poly is the X-25 generator 0x1021 in reflected form, which is how a
// table driven by the low bit needs it.
const crc16X25Poly = 0x8408

// crc16X25Table is built once at startup rather than per call.
var crc16X25Table = func() [256]uint16 {
	var t [256]uint16
	for i := range t {
		crc := uint16(i)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ crc16X25Poly
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
	return t
}()

// crc16X25 computes the X-25 CRC-16: reflected input and output, initial value
// all ones, and a final complement. Its check value over the ASCII digits
// "123456789" is 0x906E, which the test pins.
func crc16X25(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc = crc>>8 ^ crc16X25Table[byte(crc)^b]
	}
	return ^crc
}

// crc32Castagnoli computes CRC-32C. The polynomial and the table come from the
// standard library, so there is nothing here to get wrong.
var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

func crc32Castagnoli(data []byte) uint32 {
	return crc32.Checksum(data, castagnoliTable)
}

// appendZeroCRC writes the checksum field as a byte string of the right width
// filled with zeros.
//
// RFC 9171 clauses 4.3.1 and 4.3.2 both compute the checksum over every octet
// of the block including the checksum field itself, with that field
// "temporarily populated with all bytes set to zero". So the field is present
// while the checksum is computed, not absent — a block checksummed without it
// comes out different and no round trip inside one implementation would show it.
func appendZeroCRC(dst []byte, t CRCType) []byte {
	n := t.size()
	if n == 0 {
		return dst
	}
	dst = appendHead(dst, majorByteStr, uint64(n))
	for i := 0; i < n; i++ {
		dst = append(dst, 0)
	}
	return dst
}

// fillCRC computes the checksum over a block already ending in a zero-filled
// checksum field, and writes it into that field. block must be the whole
// encoded block, starting at its array head.
func fillCRC(block []byte, t CRCType) {
	n := t.size()
	if n == 0 {
		return
	}
	field := block[len(block)-n:]
	switch t {
	case CRC16X25:
		v := crc16X25(block)
		field[0], field[1] = byte(v>>8), byte(v)
	case CRC32C:
		v := crc32Castagnoli(block)
		field[0], field[1] = byte(v>>24), byte(v>>16)
		field[2], field[3] = byte(v>>8), byte(v)
	}
}

// checkCRC verifies the checksum on a decoded block. block is the whole
// encoded block and got is the checksum read from its final field.
func checkCRC(block []byte, t CRCType, got []byte) error {
	n := t.size()
	if n == 0 {
		return nil
	}
	if len(got) != n {
		return ErrWrongCRCWidth
	}

	// Recompute over the block with the checksum field zeroed, exactly as the
	// sender did. The field is the last n octets of the block.
	scratch := make([]byte, len(block))
	copy(scratch, block)
	for i := len(scratch) - n; i < len(scratch); i++ {
		scratch[i] = 0
	}

	var want [4]byte
	switch t {
	case CRC16X25:
		v := crc16X25(scratch)
		want[0], want[1] = byte(v>>8), byte(v)
	case CRC32C:
		v := crc32Castagnoli(scratch)
		want[0], want[1] = byte(v>>24), byte(v>>16)
		want[2], want[3] = byte(v>>8), byte(v)
	}

	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return ErrCRCMismatch
		}
	}
	return nil
}
