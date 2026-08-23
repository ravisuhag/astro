package sdls

import (
	"crypto/aes"
	"crypto/cipher"
	"math/big"

	"github.com/ravisuhag/astro/internal/cmac"
)

// Mode is the Security Association service type of CCSDS 355.0-B-2 §4.2.2.4.
// Every SA performs one and only one of these.
type Mode uint8

const (
	// Authentication proves integrity and origin without hiding the data.
	// The algorithm is chosen by AuthAlgorithm: GMAC for the §E1 TM and AOS
	// baselines, AES-CMAC for the §E2 telecommand baseline.
	Authentication Mode = iota + 1

	// Encryption hides the data without authenticating it. §2.3.3 warns that
	// encryption without authentication can give a false sense of security,
	// and this package does not implement it: ApplySecurity returns
	// ErrUnsupportedMode.
	Encryption

	// AuthenticatedEncryption is the §E1/§E3/§E4 baseline: AES-256-GCM with a
	// 96-bit IV and a 128-bit MAC.
	AuthenticatedEncryption
)

// AuthAlgorithm selects the MAC algorithm used when Mode is Authentication.
//
// CCSDS 355.0-B-2 keeps the service type and the algorithm apart, and so does
// this: §4.2.2.4 defines the three modes, while the annexes name a different
// algorithm per link. §E1 gives GMAC for TM and §E3/§E4 for AOS and USLP;
// §E2 gives AES-CMAC for telecommand.
type AuthAlgorithm uint8

const (
	// AuthGMAC is AES-GCM over an empty plaintext with the whole
	// Authentication Payload as associated data — the §E1, §E3 and §E4
	// baseline. It is the zero value, so an SA that does not choose keeps the
	// behaviour it had before CMAC existed.
	AuthGMAC AuthAlgorithm = iota

	// AuthCMAC is AES-CMAC, the §E2 telecommand baseline: a 256-bit key and a
	// 128-bit MAC.
	//
	// §E2.2 note: CMAC performs no encryption and needs no initialization
	// vector, so the §E2 Security Header is six octets — a 16-bit SPI and a
	// 32-bit Sequence Number, with the IV and Pad Length fields zero octets
	// wide. An SA using CMAC must set FieldLengths.IV to 0.
	AuthCMAC
)

// String names the algorithm.
func (a AuthAlgorithm) String() string {
	if a == AuthCMAC {
		return "AES-CMAC"
	}
	return "GMAC"
}

// String names the mode.
func (m Mode) String() string {
	switch m {
	case Authentication:
		return "authentication"
	case Encryption:
		return "encryption"
	case AuthenticatedEncryption:
		return "authenticated encryption"
	default:
		return "unknown"
	}
}

// GCM tag widths this package accepts, in octets. The §E1 baseline is 16.
// §4.2.3.4 f) allows truncating a MAC by dropping its least significant bits,
// which is the same most-significant-bits-first truncation that
// crypto/cipher.NewGCMWithTagSize performs. Go refuses tags below 12 octets.
const (
	MinMACSize = 12
	MaxMACSize = 16
)

// GCMIVSize is the initialization vector width GCM requires, in octets
// (96 bits, per §E1.1 b).
const GCMIVSize = 12

// AESKeySize is the key width this package accepts, in octets (256 bits,
// per §E1.1 a).
const AESKeySize = 32

// SecurityAssociation holds the agreed parameters for one secured Virtual
// Channel or MAP (§4.2.2). Both ends configure a matching SA before the link
// opens; the SPI travels on the wire so the receiver can find it.
//
// A SecurityAssociation is NOT safe for concurrent use. It carries the sender's
// IV counter and the receiver's anti-replay state, both of which mutate on
// every frame. Callers must serialize access, typically by giving each
// direction of each channel its own SA value.
type SecurityAssociation struct {
	// SPI identifies this SA on the wire (§4.2.2.3). 0 and 65535 are reserved.
	SPI uint16

	// Mode is the single service type this SA performs (§4.2.2.4).
	Mode Mode

	// Key is the caller-supplied AES-256 key: exactly 32 octets. The SA does
	// not copy it, load it, or store it anywhere; key management is out of
	// scope for this package.
	Key []byte

	// AuthAlgorithm selects the MAC algorithm when Mode is Authentication. It
	// is ignored for AuthenticatedEncryption, which is always AES-GCM.
	AuthAlgorithm AuthAlgorithm

	// FieldLengths is the wire layout of this SA's header and trailer fields.
	FieldLengths FieldLengths

	// AuthMask is the authentication bit mask of §4.2.2.6.2, applied with a
	// bitwise AND to the frame header and security header before the MAC is
	// computed. It must be at least as long as the frame header plus the
	// security header.
	//
	// Building it correctly is the caller's job and depends on the frame type;
	// §4.2.2.6.2 sets the rules. Broadly: ones over the Virtual Channel ID,
	// the Security Header, and the frame data field; zeros over the TM Master
	// Channel Frame Count, the AOS Frame Header Error Control, the Insert
	// Zone, and every other header field a mission does not choose to cover.
	//
	// A nil mask means "authenticate every octet of the frame header",
	// which is stricter than the §E1 baseline. The Initialization Vector is
	// excluded from the MAC either way: §4.2.2.6.2 h) makes that mandatory,
	// so this package enforces it regardless of the mask supplied.
	AuthMask []byte

	// SeqWindow is the sequence number window of §2.3.2.3.3: a positive delta
	// beyond which a received counter is discarded. Zero disables the
	// anti-replay check entirely, which is only appropriate for testing.
	SeqWindow uint64

	// ivCounter is the sender's IV, held as a big-endian integer across the
	// full IV width and incremented once per protected frame.
	ivCounter []byte

	// seqCounter is the sender's managed anti-replay sequence number
	// (§4.2.3.4 a), used when the SA carries an explicit Sequence Number field.
	seqCounter []byte

	// lastSeq is the highest counter value the receiver has accepted, and
	// seqSeen records whether any frame has been accepted yet. Both advance
	// only after a MAC verifies (§4.2.4.4).
	lastSeq []byte
	seqSeen bool
}

// usesIVAsSequence reports whether the IV doubles as the anti-replay counter.
// §4.1.1.4.2 allows this for counter-mode algorithms, and the §E1 baseline
// takes it: the Sequence Number field is zero octets wide.
func (sa *SecurityAssociation) usesIVAsSequence() bool {
	return sa.FieldLengths.SeqNum == 0 && sa.FieldLengths.IV > 0
}

// needsMAC reports whether this SA's service type produces a Security Trailer.
func (sa *SecurityAssociation) needsMAC() bool {
	return sa.Mode == Authentication || sa.Mode == AuthenticatedEncryption
}

// Validate checks the SA against the constraints of §4.1.1 and §4.2.2.
func (sa *SecurityAssociation) Validate() error {
	// §4.1.1.2.3 reserves both extremes of the SPI range.
	if sa.SPI == SPIReservedZero || sa.SPI == SPIReservedAllOnes {
		return ErrInvalidSPI
	}

	switch sa.Mode {
	case Authentication, Encryption, AuthenticatedEncryption:
	default:
		return ErrInvalidMode
	}

	if len(sa.Key) != AESKeySize {
		return ErrInvalidKey
	}

	if err := sa.FieldLengths.Validate(); err != nil {
		return err
	}

	if sa.needsMAC() {
		if sa.usesCMAC() {
			// §E2.2 note: CMAC performs no encryption and needs no
			// initialization vector, so the field is zero octets wide. A
			// non-zero IV here means the SA was configured for the wrong
			// baseline.
			if sa.FieldLengths.IV != 0 {
				return ErrInvalidFieldLengths
			}
		} else if sa.FieldLengths.IV != GCMIVSize {
			// GCM and GMAC both need a 96-bit nonce.
			return ErrInvalidFieldLengths
		}
		if sa.FieldLengths.MAC < MinMACSize || sa.FieldLengths.MAC > MaxMACSize {
			return ErrInvalidFieldLengths
		}
	}

	return nil
}

// usesCMAC reports whether this SA authenticates with AES-CMAC rather than
// GMAC. It is only meaningful for Mode == Authentication; authenticated
// encryption is always AES-GCM.
func (sa *SecurityAssociation) usesCMAC() bool {
	return sa.Mode == Authentication && sa.AuthAlgorithm == AuthCMAC
}

// newCMAC builds the CMAC for this SA.
func (sa *SecurityAssociation) newCMAC() (*cmac.CMAC, error) {
	return cmac.New(sa.Key)
}

// newGCM builds the AEAD for this SA at its configured tag width.
func (sa *SecurityAssociation) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(sa.Key)
	if err != nil {
		return nil, ErrInvalidKey
	}
	return cipher.NewGCMWithTagSize(block, sa.FieldLengths.MAC)
}

// nextIV advances the sender's IV counter and returns the value to transmit.
// The counter spans the full IV width as a big-endian integer. It refuses to
// wrap: reusing an IV under one GCM key is catastrophic, so the SA returns
// ErrIVExhausted instead (§E1.1 b, and the GCM requirement behind it).
func (sa *SecurityAssociation) nextIV() ([]byte, error) {
	width := sa.FieldLengths.IV
	if width == 0 {
		return nil, nil
	}
	if sa.ivCounter == nil {
		// The first frame transmits counter value 1, leaving 0 unused so a
		// freshly initialized receiver never accepts a replayed zero.
		sa.ivCounter = make([]byte, width)
	}
	if err := incrementBE(sa.ivCounter); err != nil {
		return nil, err
	}
	return copySlice(sa.ivCounter), nil
}

// nextSeqNum advances the sender's explicit sequence number, per §4.2.3.4 a).
// It returns nil when the SA has no Sequence Number field.
func (sa *SecurityAssociation) nextSeqNum() ([]byte, error) {
	width := sa.FieldLengths.SeqNum
	if width == 0 {
		return nil, nil
	}
	if sa.seqCounter == nil {
		sa.seqCounter = make([]byte, width)
	}
	if err := incrementBE(sa.seqCounter); err != nil {
		return nil, err
	}
	return copySlice(sa.seqCounter), nil
}

// incrementBE adds one to a big-endian integer held in b, returning
// ErrIVExhausted rather than wrapping past all-ones.
func incrementBE(b []byte) error {
	for i := len(b) - 1; i >= 0; i-- {
		b[i]++
		if b[i] != 0 {
			return nil
		}
	}
	// Every octet wrapped to zero, so the counter space is used up. Restore
	// the all-ones value so the SA cannot be coaxed into reuse by retrying.
	for i := range b {
		b[i] = 0xFF
	}
	return ErrIVExhausted
}

// checkAndAdvanceSequence applies the anti-replay rules of §2.3.2.3.
//
// The received counter must be strictly greater than the highest one accepted
// so far, and no further ahead than SeqWindow. It is the caller's job to run
// this only after the MAC has verified, so a forged frame cannot advance the
// window (§4.2.4.4).
func (sa *SecurityAssociation) checkAndAdvanceSequence(received []byte) error {
	if sa.SeqWindow == 0 || len(received) == 0 {
		return nil
	}

	got := new(big.Int).SetBytes(received)

	if sa.seqSeen {
		last := new(big.Int).SetBytes(sa.lastSeq)

		// §2.3.2.3.2: accept only a value higher than the stored one.
		if got.Cmp(last) <= 0 {
			return ErrReplayDetected
		}

		// §2.3.2.3.3: and no more than the window ahead of it.
		delta := new(big.Int).Sub(got, last)
		if delta.Cmp(new(big.Int).SetUint64(sa.SeqWindow)) > 0 {
			return ErrReplayDetected
		}
	}

	sa.lastSeq = copySlice(received)
	sa.seqSeen = true
	return nil
}

// Humanize returns a human-readable summary of the Security Association.
func (sa *SecurityAssociation) Humanize() string {
	return "SDLS Security Association" +
		"\n  SPI .......... " + itoa(int(sa.SPI)) +
		"\n  Mode ......... " + sa.Mode.String() +
		"\n  IV bytes ..... " + itoa(sa.FieldLengths.IV) +
		"\n  SeqNum bytes . " + itoa(sa.FieldLengths.SeqNum) +
		"\n  PadLen bytes . " + itoa(sa.FieldLengths.PadLen) +
		"\n  MAC bytes .... " + itoa(sa.FieldLengths.MAC) +
		"\n  Seq window ... " + itoa(int(sa.SeqWindow))
}
