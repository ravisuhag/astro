// Package keywrap implements the AES Key Wrap algorithm of RFC 3394.
//
// Key wrap encrypts one key with another. BPSec uses it to carry the symmetric
// key inside a security block: RFC 9173 clauses 3.3.2 and 4.3.3 both define a
// "wrapped key" security context parameter holding exactly this ciphertext,
// and a receiver unwraps it with a key encryption key it already holds.
//
// The algorithm is the index-based form of RFC 3394 clause 2.2.1, which the
// document says produces identical results to the register-shifting form it
// states first. Wrapping n 64-bit blocks costs 6n AES block operations and
// produces n+1 blocks: the extra one carries the integrity check.
//
// This is not authenticated encryption in the AEAD sense and takes no nonce.
// The initial value is a constant, which is what lets Unwrap detect a wrong
// key or altered ciphertext (clause 2.2.3.1). RFC 9173 clause 4.6 calls out
// that the constant here is deliberately unlike the per-invocation IV its
// AES-GCM context needs, so the two must not be confused.
//
// It lives in internal because it carries no API commitment of its own. Go's
// standard library has no key wrap.
package keywrap

import (
	"crypto/aes"
	"crypto/subtle"
	"encoding/binary"
	"errors"
)

// blockSize is the width of one key data block: 64 bits (RFC 3394 clause 2).
const blockSize = 8

// defaultIV is the initial value A[0] of RFC 3394 clause 2.2.3.1, the
// hexadecimal constant A6A6A6A6A6A6A6A6. Unwrap recovering this value is the
// integrity check on the key data.
var defaultIV = [blockSize]byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}

var (
	// ErrKeyDataLength indicates key data that is not a whole number of
	// 64-bit blocks, or is fewer than two of them. RFC 3394 clause 2 fixes
	// both bounds: "the only restriction the key wrap algorithm places on n is
	// that n be at least two."
	ErrKeyDataLength = errors.New("keywrap: key data must be at least two whole 64-bit blocks")

	// ErrCiphertextLength indicates wrapped key data that is not a whole
	// number of 64-bit blocks, or holds fewer than three of them. Wrapping n
	// blocks yields n+1, and n is at least two.
	ErrCiphertextLength = errors.New("keywrap: wrapped key data must be at least three whole 64-bit blocks")

	// ErrIntegrityCheck indicates an unwrap whose recovered initial value is
	// not the constant of clause 2.2.3.1. The key encryption key is wrong, or
	// the ciphertext was altered. Clause 2.2.2 requires returning an error and
	// no key data at all.
	ErrIntegrityCheck = errors.New("keywrap: unwrapped key data failed its integrity check")
)

// Wrap encrypts keyData under the key encryption key kek, returning n+1 blocks
// where keyData held n (RFC 3394 clause 2.2.1).
//
// kek must be a valid AES key: 16, 24 or 32 octets. keyData must be a whole
// number of 8-octet blocks, at least two.
func Wrap(kek, keyData []byte) ([]byte, error) {
	if len(keyData) < 2*blockSize || len(keyData)%blockSize != 0 {
		return nil, ErrKeyDataLength
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	n := len(keyData) / blockSize

	// out[0:8] holds A; out[8:] holds R[1..n], wrapped in place.
	out := make([]byte, (n+1)*blockSize)
	copy(out, defaultIV[:])
	copy(out[blockSize:], keyData)

	var buf [2 * blockSize]byte
	for j := 0; j < 6; j++ {
		for i := 1; i <= n; i++ {
			copy(buf[:blockSize], out[:blockSize])
			copy(buf[blockSize:], out[i*blockSize:(i+1)*blockSize])
			block.Encrypt(buf[:], buf[:])

			// A = MSB(64, B) ^ t, where t = (n*j)+i.
			t := uint64(n*j + i)
			xorCounter(buf[:blockSize], t)

			copy(out[:blockSize], buf[:blockSize])
			copy(out[i*blockSize:(i+1)*blockSize], buf[blockSize:])
		}
	}
	return out, nil
}

// Unwrap reverses Wrap (RFC 3394 clause 2.2.2). It returns an error rather
// than key data when the integrity check fails, which is what tells a caller
// the key encryption key was wrong.
func Unwrap(kek, wrapped []byte) ([]byte, error) {
	if len(wrapped) < 3*blockSize || len(wrapped)%blockSize != 0 {
		return nil, ErrCiphertextLength
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}

	n := len(wrapped)/blockSize - 1

	work := make([]byte, len(wrapped))
	copy(work, wrapped)

	var buf [2 * blockSize]byte
	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			copy(buf[:blockSize], work[:blockSize])

			// B = AES-1(K, (A ^ t) | R[i]), where t = (n*j)+i.
			t := uint64(n*j + i)
			xorCounter(buf[:blockSize], t)

			copy(buf[blockSize:], work[i*blockSize:(i+1)*blockSize])
			block.Decrypt(buf[:], buf[:])

			copy(work[:blockSize], buf[:blockSize])
			copy(work[i*blockSize:(i+1)*blockSize], buf[blockSize:])
		}
	}

	// Clause 2.2.2 step 3: check A against the initial value, and return no
	// key data at all if it does not match. The comparison is constant time so
	// that a caller cannot learn how much of the check passed from timing.
	if subtle.ConstantTimeCompare(work[:blockSize], defaultIV[:]) != 1 {
		return nil, ErrIntegrityCheck
	}
	return work[blockSize:], nil
}

// xorCounter exclusive-ors the big-endian counter t into an 8-octet block.
func xorCounter(b []byte, t uint64) {
	var enc [blockSize]byte
	binary.BigEndian.PutUint64(enc[:], t)
	for i := range b {
		b[i] ^= enc[i]
	}
}
