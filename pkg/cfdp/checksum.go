package cfdp

import "hash/crc32"

// Checksum types from the SANA Checksum Identifiers registry, as referenced by
// CCSDS 727.0-B-5 clause 4.2.2.2.
const (
	// ChecksumModular is the legacy modular checksum. Clause 4.2.2.3 requires every
	// implementation to provide it.
	ChecksumModular uint8 = 0
	// ChecksumCRC32C is CRC-32 Castagnoli, registry entry 2.
	ChecksumCRC32C uint8 = 2
	// ChecksumCRC32 is the standard CRC-32, registry entry 3.
	ChecksumCRC32 uint8 = 3
	// ChecksumNull always yields zero. Clause 4.2.2.4 requires it, and warns that it
	// protects nothing.
	ChecksumNull uint8 = 15
)

// Checksum accumulates a CFDP file checksum. Every checksum is 32 bits
// (clause 4.2.1.2).
//
// Segments may arrive in any order and at any offset: Update takes the file
// offset with the data, so an out-of-order stream produces the same result as
// a sequential one.
type Checksum interface {
	// Update folds a run of file octets at the given file offset into the sum.
	Update(offset uint64, data []byte)
	// Sum returns the checksum computed so far.
	Sum() uint32
	// Type returns the SANA registry identifier for this algorithm.
	Type() uint8
}

// NewChecksum returns an accumulator for a checksum type, or
// ErrUnsupportedChecksumType if this package does not implement it.
func NewChecksum(checksumType uint8) (Checksum, error) {
	switch checksumType {
	case ChecksumModular:
		return &modularChecksum{}, nil
	case ChecksumCRC32C:
		return newTableChecksum(ChecksumCRC32C, crc32.MakeTable(crc32.Castagnoli)), nil
	case ChecksumCRC32:
		return newTableChecksum(ChecksumCRC32, crc32.IEEETable), nil
	case ChecksumNull:
		return &nullChecksum{}, nil
	default:
		return nil, ErrUnsupportedChecksumType
	}
}

// modularChecksum implements the legacy modular checksum of clause 4.2.2.3, with the
// worked example in Annex F.
//
// The file is cut into 4-octet words aligned to file offsets that are
// multiples of 4, each word read big-endian, and the words summed with carries
// out of the high-order octet discarded.
//
// Rather than buffer words, this folds one octet at a time: an octet at file
// offset q contributes its value shifted left by 8*(3 - q mod 4) bits. That is
// the same arithmetic, and it makes the alignment rule of Annex F note 1 and
// the padding rule of note 2 fall out for free. A partial word simply has
// fewer contributions, which is identical to padding it with zeros.
type modularChecksum struct {
	sum uint32
}

func (c *modularChecksum) Update(offset uint64, data []byte) {
	for i, b := range data {
		shift := 8 * (3 - (offset+uint64(i))%4)
		c.sum += uint32(b) << shift // uint32 wraps, discarding the carry
	}
}

func (c *modularChecksum) Sum() uint32 { return c.sum }
func (c *modularChecksum) Type() uint8 { return ChecksumModular }

// nullChecksum is the null algorithm of clause 4.2.2.4: always zero.
type nullChecksum struct{}

func (c *nullChecksum) Update(uint64, []byte) {}
func (c *nullChecksum) Sum() uint32           { return 0 }
func (c *nullChecksum) Type() uint8           { return ChecksumNull }

// tableChecksum implements the CRC-32 family. A CRC is order-dependent, so
// unlike the modular checksum it needs the file assembled in order. It buffers
// segments by offset and folds them once they are contiguous.
type tableChecksum struct {
	kind    uint8
	table   *crc32.Table
	sum     uint32
	next    uint64            // file offset the CRC has consumed up to
	pending map[uint64][]byte // out-of-order segments awaiting their turn
}

func newTableChecksum(kind uint8, table *crc32.Table) *tableChecksum {
	return &tableChecksum{kind: kind, table: table, pending: make(map[uint64][]byte)}
}

func (c *tableChecksum) Update(offset uint64, data []byte) {
	if len(data) == 0 {
		return
	}
	buf := make([]byte, len(data))
	copy(buf, data)
	c.pending[offset] = buf
	c.drain()
}

// drain folds every buffered segment that is now contiguous with what the CRC
// has already consumed.
func (c *tableChecksum) drain() {
	for {
		seg, ok := c.pending[c.next]
		if !ok {
			return
		}
		c.sum = crc32.Update(c.sum, c.table, seg)
		delete(c.pending, c.next)
		c.next += uint64(len(seg))
	}
}

func (c *tableChecksum) Sum() uint32 { return c.sum }
func (c *tableChecksum) Type() uint8 { return c.kind }
