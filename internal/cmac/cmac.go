// Package cmac implements the AES-CMAC message authentication code of
// NIST SP 800-38B, also published as RFC 4493.
//
// CCSDS 355.0-B-2 clause E2 names it as the baseline authentication algorithm for
// telecommand: AES-CMAC with a 256-bit key and a 128-bit tag. The Go standard
// library has AES but no CMAC, so it is implemented here rather than pulled in
// as a dependency, pkg/ takes none.
//
// # How it works
//
// CMAC is CBC-MAC with the flaw fixed. Plain CBC-MAC is forgeable across
// messages of different lengths, so CMAC derives two subkeys from the block
// cipher and mixes one of them into the final block: K1 when the message is a
// whole number of blocks, K2 when it had to be padded. Which subkey was used
// is therefore bound into the tag, and the length ambiguity disappears.
package cmac

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/subtle"
	"errors"
)

// BlockSize is the AES block size in octets, and the size of a CMAC tag.
const BlockSize = aes.BlockSize

// ErrInvalidTagLength indicates a truncation length outside 1 to 16 octets.
var ErrInvalidTagLength = errors.New("invalid CMAC tag length: must be 1 to 16 octets")

// rb is the constant of SP 800-38B clause 5.3 for a 128-bit block: the low octet of
// the polynomial x^128 + x^7 + x^2 + x + 1.
const rb = 0x87

// CMAC holds the subkeys derived from a key, so a caller authenticating many
// messages under one key derives them once.
type CMAC struct {
	block cipher.Block
	k1    [BlockSize]byte
	k2    [BlockSize]byte
}

// New returns a CMAC for the given AES key. The key must be 16, 24 or 32
// octets; CCSDS 355.0-B-2 clause E2a requires 32.
func New(key []byte) (*CMAC, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if block.BlockSize() != BlockSize {
		return nil, errors.New("cmac: cipher block size is not 128 bits")
	}

	c := &CMAC{block: block}
	c.deriveSubkeys()
	return c, nil
}

// deriveSubkeys implements the subkey generation of SP 800-38B clause 6.1.
//
//	L  = CIPH(0^128)
//	K1 = L<<1        if MSB(L) = 0, else (L<<1) XOR Rb
//	K2 = K1<<1       if MSB(K1) = 0, else (K1<<1) XOR Rb
func (c *CMAC) deriveSubkeys() {
	var l [BlockSize]byte
	c.block.Encrypt(l[:], l[:])

	c.k1 = shiftLeftOne(l)
	c.k2 = shiftLeftOne(c.k1)
}

// shiftLeftOne doubles a block in GF(2^128): a one-bit left shift, with the
// reduction polynomial folded back in when the shifted-out bit was set.
//
// It is written to run in constant time. The branch it would otherwise take
// depends on a bit of a value derived from the key, and a timing signal there
// would leak information about the key.
func shiftLeftOne(in [BlockSize]byte) [BlockSize]byte {
	var out [BlockSize]byte

	// The top bit decides whether Rb is folded in. Spread it to a full mask
	// rather than branching on it.
	msb := in[0] >> 7
	mask := -msb // 0x00 when msb is 0, 0xFF when it is 1

	var carry byte
	for i := BlockSize - 1; i >= 0; i-- {
		out[i] = in[i]<<1 | carry
		carry = in[i] >> 7
	}
	out[BlockSize-1] ^= rb & mask
	return out
}

// Sum returns the 128-bit CMAC tag of message.
func (c *CMAC) Sum(message []byte) []byte {
	var last [BlockSize]byte

	// SP 800-38B clause 6.2: a message that is a positive whole number of blocks
	// uses K1 on its final block; anything else (including the empty
	// message) is padded with a one bit and zeros, and uses K2.
	complete := len(message) > 0 && len(message)%BlockSize == 0

	var n int
	if complete {
		n = len(message)/BlockSize - 1
		copy(last[:], message[n*BlockSize:])
		xorInto(&last, &c.k1)
	} else {
		n = len(message) / BlockSize
		remainder := message[n*BlockSize:]
		copy(last[:], remainder)
		last[len(remainder)] = 0x80
		xorInto(&last, &c.k2)
	}

	// CBC over the leading blocks, then the prepared final block.
	var x [BlockSize]byte
	for i := range n {
		block := message[i*BlockSize : (i+1)*BlockSize]
		for j := range x {
			x[j] ^= block[j]
		}
		c.block.Encrypt(x[:], x[:])
	}

	for j := range x {
		x[j] ^= last[j]
	}
	c.block.Encrypt(x[:], x[:])

	tag := make([]byte, BlockSize)
	copy(tag, x[:])
	return tag
}

// SumTruncated returns the leading length octets of the tag.
//
// SP 800-38B clause 6.4 permits truncation and warns that a shorter tag weakens the
// forgery bound. CCSDS 355.0-B-2 clause E2c specifies the full 128 bits, so a caller
// following the baseline has no reason to truncate.
func (c *CMAC) SumTruncated(message []byte, length int) ([]byte, error) {
	if length < 1 || length > BlockSize {
		return nil, ErrInvalidTagLength
	}
	return c.Sum(message)[:length], nil
}

// Verify reports whether tag authenticates message.
//
// The comparison is constant time. Comparing tags with bytes.Equal would leak
// how many leading octets a forgery got right, which is enough to find the
// rest one octet at a time.
func (c *CMAC) Verify(message, tag []byte) bool {
	if len(tag) < 1 || len(tag) > BlockSize {
		return false
	}
	return subtle.ConstantTimeCompare(c.Sum(message)[:len(tag)], tag) == 1
}

// xorInto XORs src into dst.
func xorInto(dst, src *[BlockSize]byte) {
	for i := range dst {
		dst[i] ^= src[i]
	}
}
