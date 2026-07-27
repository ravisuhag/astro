package sle

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"time"
)

// ISP1 credentials, per CCSDS 913.1-B-2 §3.1.2.
//
// SLE authenticates with a scheme the standard calls "Protected 1". Neither
// end sends its password. Instead the sender hashes a DER-encoded structure
// holding the current time, a random number, its user name and its password,
// and transmits the time, the random number and the digest. The receiver, who
// knows the peer's password, recomputes the digest and compares.
//
// The time is what stops a replay: §3.1.2.2.1 has the receiver reject
// credentials whose time is further from now than an acceptable delay.
//
// # SHA-256, not SHA-1
//
// §3.1.2.3 requires SHA-256. SHA-1 was the previous issue of the standard, and
// §3.2.3's note keeps a 20-octet digest acceptable only so a new implementation
// can talk to an old one. This package generates SHA-256 and accepts either
// length on receive.

// Digest sizes, per §3.2.3 note 2.
const (
	// DigestSizeSHA256 is what this package generates.
	DigestSizeSHA256 = 32
	// DigestSizeSHA1 is the legacy size, accepted on receive only.
	DigestSizeSHA1 = 20
)

// MaxRandomNumber is the upper bound of the randomNumber field, per the
// HashInput type of figure 3-1: INTEGER (0 .. 2147483647).
const MaxRandomNumber = 2147483647

// Credentials are the ISP1Credentials of figure 3-2: the time and random
// number that went into a digest, and the digest itself.
type Credentials struct {
	Time         Time
	RandomNumber int32
	// Protected is the message digest, 32 octets from SHA-256 or 20 from the
	// legacy SHA-1.
	Protected []byte
}

// hashInput builds and DER-encodes the HashInput of figure 3-1:
//
//	HashInput ::= SEQUENCE
//	{ time          OCTET STRING (SIZE(8))
//	, randomNumber  INTEGER (0 .. 2147483647)
//	, userName      VisibleString
//	, passWord      OCTET STRING
//	}
//
// §3.1.2.1.1 specifies DER here even though the PDUs themselves use BER. For
// this structure the two coincide: every field is definite-length and
// minimally encoded either way.
func hashInput(t Time, randomNumber int32, userName string, password []byte) []byte {
	var content []byte
	content = AppendOctetString(content, t.Encode())
	content = AppendInteger(content, int64(randomNumber))
	content = AppendVisibleString(content, userName)
	content = AppendOctetString(content, password)
	return AppendSequence(nil, content)
}

// GenerateCredentials builds credentials for an outgoing PDU, per §3.1.2.1.
//
// The caller supplies the random number rather than this package choosing one.
// A library has no business picking a mission's randomness source, and the
// value has to be reproducible in tests.
func GenerateCredentials(t time.Time, randomNumber int32, userName string, password []byte) (*Credentials, error) {
	if randomNumber < 0 {
		return nil, ErrInvalidCredentials
	}
	sleTime, err := NewTime(t)
	if err != nil {
		return nil, err
	}

	digest := sha256.Sum256(hashInput(sleTime, randomNumber, userName, password))

	return &Credentials{
		Time:         sleTime,
		RandomNumber: randomNumber,
		Protected:    digest[:],
	}, nil
}

// Verify checks credentials received from a peer, per §3.1.2.2.
//
// now is the current time and acceptableDelay is how far the credential time
// may be from it. userName and password are the peer's, which the receiver
// already knows.
//
// The digest comparison is constant time. A timing oracle on a MAC comparison
// is a real attack, and it costs nothing to avoid.
func (c *Credentials) Verify(now time.Time, acceptableDelay time.Duration, userName string, password []byte) error {
	if c == nil {
		return ErrInvalidCredentials
	}
	if len(c.Protected) != DigestSizeSHA256 && len(c.Protected) != DigestSizeSHA1 {
		return ErrInvalidCredentials
	}

	// §3.1.2.2.1: reject credentials too far from now, in either direction.
	skew := now.Sub(c.Time.Time())
	if skew < 0 {
		skew = -skew
	}
	if acceptableDelay > 0 && skew > acceptableDelay {
		return ErrCredentialsExpired
	}

	// A 20-octet digest means the peer is running the previous issue of the
	// standard. This package cannot recompute a SHA-1 digest, because it does
	// not implement the superseded scheme.
	if len(c.Protected) == DigestSizeSHA1 {
		return ErrInvalidCredentials
	}

	want := sha256.Sum256(hashInput(c.Time, c.RandomNumber, userName, password))
	if subtle.ConstantTimeCompare(c.Protected, want[:]) != 1 {
		return ErrAuthenticationFailed
	}
	return nil
}

// Encode serializes the ISP1Credentials SEQUENCE of figure 3-2.
func (c *Credentials) Encode() ([]byte, error) {
	if c == nil {
		return nil, ErrInvalidCredentials
	}
	if len(c.Protected) < DigestSizeSHA1 || len(c.Protected) > DigestSizeSHA256 {
		return nil, ErrInvalidCredentials
	}

	var content []byte
	content = AppendOctetString(content, c.Time.Encode())
	content = AppendInteger(content, int64(c.RandomNumber))
	content = AppendOctetString(content, c.Protected)
	return AppendSequence(nil, content), nil
}

// DecodeCredentials parses an ISP1Credentials SEQUENCE.
func DecodeCredentials(data []byte) (*Credentials, error) {
	d := NewDecoder(data)
	seq, err := d.Next()
	if err != nil {
		return nil, err
	}
	if !seq.IsUniversal(TagSequence) || !seq.Constructed {
		return nil, ErrInvalidCredentials
	}

	inner := d.Nested(seq)

	timeElem, err := inner.Next()
	if err != nil {
		return nil, err
	}
	t, err := DecodeTime(timeElem.Bytes)
	if err != nil {
		return nil, err
	}

	randElem, err := inner.Next()
	if err != nil {
		return nil, err
	}
	random, err := randElem.Int64()
	if err != nil {
		return nil, err
	}
	if random < 0 || random > MaxRandomNumber {
		return nil, ErrInvalidCredentials
	}

	protectedElem, err := inner.Next()
	if err != nil {
		return nil, err
	}

	return &Credentials{
		Time:         t,
		RandomNumber: int32(random),
		Protected:    protectedElem.Copy(),
	}, nil
}

// Humanize returns a human-readable summary. It shows the digest length rather
// than the digest.
func (c *Credentials) Humanize() string {
	if c == nil {
		return "SLE Credentials (unused)"
	}
	algorithm := "SHA-256"
	if len(c.Protected) == DigestSizeSHA1 {
		algorithm = "SHA-1 (legacy)"
	}
	return fmt.Sprintf("SLE Credentials\n  Time ....... %s\n  Random ..... %d\n  Digest ..... %d octets, %s",
		c.Time.Humanize(), c.RandomNumber, len(c.Protected), algorithm)
}

// AppendCredentialsChoice writes the Credentials CHOICE of the common types
// module:
//
//	Credentials ::= CHOICE
//	{ unused [0] NULL
//	, used   [1] OCTET STRING (SIZE (8 .. 256))
//	}
//
// A nil Credentials takes the unused alternative, which is what an
// unauthenticated association sends.
func AppendCredentialsChoice(dst []byte, c *Credentials) ([]byte, error) {
	if c == nil {
		return AppendElement(dst, ClassContext, false, 0, nil), nil
	}
	encoded, err := c.Encode()
	if err != nil {
		return nil, err
	}
	return AppendElement(dst, ClassContext, false, 1, encoded), nil
}

// DecodeCredentialsChoice reads a Credentials CHOICE, returning nil for the
// unused alternative.
func DecodeCredentialsChoice(e *Element) (*Credentials, error) {
	switch {
	case e.IsContext(0):
		return nil, nil
	case e.IsContext(1):
		return DecodeCredentials(e.Bytes)
	default:
		return nil, ErrInvalidTag
	}
}
