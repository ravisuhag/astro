package ocsc

// CRC-32 attachment, per CCSDS 142.0-B-1 clause 3.6.
//
// Thirty-two check digits are appended to each pseudo-randomized information
// block, so the receiver can tell a correctly decoded codeword from one the
// SCPPM decoder got wrong.
//
// The generator (clause 3.6.2.2) is:
//
//	h(X) = X^32 + X^29 + X^18 + X^14 + X^3 + 1
//
// which is 0x20044009. That is yet another CRC-32, the fourth polynomial in
// this library, after IEEE, Castagnoli, and the Proximity-1 one in pkg/pxsc.
// They are all different and none is interchangeable.
//
// The formula in clause 3.6.2.2 adds Σ X^(k+j) for j = 0 to 31 before the modulo.
// That term is the standard way of writing "initialize the register to all
// ones", which is what the implementation below does.

// CRC32Polynomial is the generator of clause 3.6.2.2, MSB-first, with the implicit
// X^32 term dropped.
const CRC32Polynomial uint32 = 0x20044009

// crc32Table is the byte-at-a-time lookup table for the optical generator.
var crc32Table = buildCRC32Table()

// buildCRC32Table precomputes the MSB-first table for CRC32Polynomial.
func buildCRC32Table() [256]uint32 {
	var table [256]uint32
	for i := range table {
		reg := uint32(i) << 24
		for bit := 0; bit < 8; bit++ {
			if reg&0x80000000 != 0 {
				reg = reg<<1 ^ CRC32Polynomial
			} else {
				reg <<= 1
			}
		}
		table[i] = reg
	}
	return table
}

// ComputeCRC32 returns the optical CRC-32 over a bit string.
//
// The register starts at all ones, matching the Σ X^(k+j) term of clause 3.6.2.2.
// Because a block length need not be a multiple of eight, this walks bits
// rather than octets whenever the tail is partial.
func ComputeCRC32(block *BitString) uint32 {
	reg := ^uint32(0)
	if block == nil {
		return reg
	}

	n := block.Len()
	whole := n / 8

	// Whole octets go through the table.
	data := block.Bytes()
	for i := 0; i < whole; i++ {
		reg = reg<<8 ^ crc32Table[byte(reg>>24)^data[i]]
	}

	// Any remaining bits go one at a time.
	for i := whole * 8; i < n; i++ {
		top := reg >> 31 & 1
		reg <<= 1
		if top^uint32(block.Bit(i)) != 0 {
			reg ^= CRC32Polynomial
		}
	}
	return reg
}

// AttachCRC appends the 32 check digits to a block, per clause 3.6.1.1, returning a
// block of k + 32 digits.
func AttachCRC(block *BitString) *BitString {
	sum := ComputeCRC32(block)

	out := NewBitString(0)
	out.AppendBits(block, block.Len())
	for i := 31; i >= 0; i-- {
		out.Append(uint8(sum >> uint(i) & 1))
	}
	return out
}

// VerifyCRC reports whether a block ending in its own 32 check digits is
// intact, and returns the block without them.
func VerifyCRC(block *BitString) (*BitString, bool) {
	if block == nil || block.Len() < CRCBits {
		return nil, false
	}

	body := block.Slice(0, block.Len()-CRCBits)
	want := ComputeCRC32(body)

	var got uint32
	for i := block.Len() - CRCBits; i < block.Len(); i++ {
		got = got<<1 | uint32(block.Bit(i))
	}
	return body, got == want
}
