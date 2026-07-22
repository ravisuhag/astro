package ocsc

// Bit-level helpers.
//
// CCSDS 142.0-B-1 works in binary digits, not octets, and the block lengths
// of table 3-1 are not multiples of eight: an information block at code rate
// 1/3 is 5006 bits, which is 625 octets and six bits over. So the whole
// conditioning chain is bit-addressable, and these helpers carry it.
//
// A BitString stores bits most significant first within each octet, matching
// the transmission order of §1.6.

// BitString is a run of binary digits with a length that need not be a
// multiple of eight.
type BitString struct {
	// data holds the bits, most significant first in each octet. Any bits
	// past Len in the final octet are zero.
	data []byte
	// length is the number of meaningful bits.
	length int
}

// NewBitString returns an all-zero bit string of the given length.
func NewBitString(length int) *BitString {
	if length < 0 {
		length = 0
	}
	return &BitString{data: make([]byte, (length+7)/8), length: length}
}

// BitStringFromBytes wraps octets as a bit string of exactly 8*len(data) bits.
func BitStringFromBytes(data []byte) *BitString {
	out := &BitString{data: make([]byte, len(data)), length: len(data) * 8}
	copy(out.data, data)
	return out
}

// BitStringFromBits wraps octets as a bit string of the given bit length.
// Bits past length in the final octet are cleared.
func BitStringFromBits(data []byte, length int) *BitString {
	if length < 0 {
		length = 0
	}
	need := (length + 7) / 8
	buf := make([]byte, need)
	copy(buf, data)

	out := &BitString{data: buf, length: length}
	out.clearTail()
	return out
}

// clearTail zeroes any bits past the length in the final octet, so two bit
// strings of equal length always compare equal as octets.
func (b *BitString) clearTail() {
	if b.length%8 == 0 || len(b.data) == 0 {
		return
	}
	keep := uint(b.length % 8)
	mask := byte(0xFF) << (8 - keep)
	b.data[len(b.data)-1] &= mask
}

// Len returns the number of bits.
func (b *BitString) Len() int { return b.length }

// Bytes returns the underlying octets. The caller must not modify them.
func (b *BitString) Bytes() []byte { return b.data }

// Bit returns bit i, counting from the most significant bit of octet zero.
func (b *BitString) Bit(i int) uint8 {
	if i < 0 || i >= b.length {
		return 0
	}
	return b.data[i/8] >> uint(7-i%8) & 1
}

// SetBit sets bit i to v.
func (b *BitString) SetBit(i int, v uint8) {
	if i < 0 || i >= b.length {
		return
	}
	mask := byte(1) << uint(7-i%8)
	if v&1 != 0 {
		b.data[i/8] |= mask
	} else {
		b.data[i/8] &^= mask
	}
}

// Append adds one bit to the end.
func (b *BitString) Append(v uint8) {
	if b.length%8 == 0 {
		b.data = append(b.data, 0)
	}
	b.length++
	b.SetBit(b.length-1, v)
}

// AppendBits adds the leading count bits of other.
func (b *BitString) AppendBits(other *BitString, count int) {
	if count > other.Len() {
		count = other.Len()
	}
	for i := 0; i < count; i++ {
		b.Append(other.Bit(i))
	}
}

// AppendBytes adds every bit of data.
func (b *BitString) AppendBytes(data []byte) {
	for _, octet := range data {
		for i := 7; i >= 0; i-- {
			b.Append(octet >> uint(i) & 1)
		}
	}
}

// Slice returns bits [start, end) as a new bit string.
func (b *BitString) Slice(start, end int) *BitString {
	if start < 0 {
		start = 0
	}
	if end > b.length {
		end = b.length
	}
	if end <= start {
		return NewBitString(0)
	}

	out := NewBitString(end - start)
	for i := start; i < end; i++ {
		out.SetBit(i-start, b.Bit(i))
	}
	return out
}

// Equal reports whether two bit strings hold the same bits.
func (b *BitString) Equal(other *BitString) bool {
	if other == nil || b.length != other.length {
		return false
	}
	for i := 0; i < b.length; i++ {
		if b.Bit(i) != other.Bit(i) {
			return false
		}
	}
	return true
}

// XorBit flips bit i when v is 1.
func (b *BitString) XorBit(i int, v uint8) {
	if v&1 != 0 {
		b.SetBit(i, b.Bit(i)^1)
	}
}
