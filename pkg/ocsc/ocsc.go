// Package ocsc implements the data-conditioning chain of the CCSDS optical
// communications coding and synchronization standard, CCSDS 142.0-B-1
// (August 2019).
//
// This is deep-space laser communication in the High Photon Efficiency regime.
// The full standard specifies SCPPM — serially concatenated convolutional
// coding with pulse-position modulation — and everything that surrounds it.
// This package implements the deterministic front half of that chain, the part
// that is pure bit manipulation:
//
//	transfer frames
//	  → attach sync marker      (§3.3)  ASM 1ACFFC1D
//	  → slice into blocks       (§3.4)  k bits, zero-filled
//	  → pseudo-randomize        (§3.5)  g(D) = D^8+D^7+D^5+D^3+1
//	  → attach CRC-32           (§3.6)  h(X) = X^32+X^29+X^18+X^14+X^3+1
//	  → attach termination bits (§3.7)  two zeros
//	  → SCPPM encoder input block
//
// What follows — the SCPPM encoder proper, the channel interleaver, the
// codeword sync marker, the slot mapper — is coupled to the modulation and is
// not here. Nor is anything on the receive side: iterative SCPPM decoding is a
// research-grade job and does not belong in a wire-format library.
//
// # Everything is bits, not octets
//
// The block lengths of table 3-1 are 5006, 7526 and 10046 binary digits. None
// is a multiple of eight. So this package works in BitString throughout, and
// converting to octets is something you do at the very end, if at all.
package ocsc

// CodeRate selects the SCPPM code rate, a managed parameter per §3.4.1.
type CodeRate uint8

const (
	// RateOneThird is code rate 1/3.
	RateOneThird CodeRate = iota
	// RateOneHalf is code rate 1/2.
	RateOneHalf
	// RateTwoThirds is code rate 2/3.
	RateTwoThirds
)

// String names the rate.
func (r CodeRate) String() string {
	switch r {
	case RateOneThird:
		return "1/3"
	case RateOneHalf:
		return "1/2"
	case RateTwoThirds:
		return "2/3"
	default:
		return "invalid"
	}
}

// Valid reports whether the rate is one of the three of table 3-1.
func (r CodeRate) Valid() bool { return r <= RateTwoThirds }

// Information block sizes in binary digits, per table 3-1.
//
// k is the block the slicer produces; k-hat is what the SCPPM encoder receives,
// after 32 CRC digits and 2 termination digits are added. The arithmetic is
// worth seeing: 5006 + 32 + 2 = 5040, and likewise for the others.
const (
	// KOneThird is the information block size at code rate 1/3.
	KOneThird = 5006
	// KOneHalf is the information block size at code rate 1/2.
	KOneHalf = 7526
	// KTwoThirds is the information block size at code rate 2/3.
	KTwoThirds = 10046

	// CRCBits is the width of the attached CRC (§3.6.1.1).
	CRCBits = 32
	// TerminationBits is how many zeros §3.7 appends.
	TerminationBits = 2
)

// InformationBlockSize returns k, the slicer's output length in binary digits,
// for a code rate (table 3-1).
func (r CodeRate) InformationBlockSize() int {
	switch r {
	case RateOneThird:
		return KOneThird
	case RateOneHalf:
		return KOneHalf
	case RateTwoThirds:
		return KTwoThirds
	default:
		return 0
	}
}

// EncoderInputSize returns k-hat, the SCPPM encoder input block length in
// binary digits: k plus the CRC and termination digits (table 3-1, §3.7).
func (r CodeRate) EncoderInputSize() int {
	k := r.InformationBlockSize()
	if k == 0 {
		return 0
	}
	return k + CRCBits + TerminationBits
}

// ASM is the Attached Synchronization Marker of §3.3.2: the 32-bit sequence
// 1ACFFC1D.
//
// It is the same marker TM uses for a CADU, which is convenient: a ground
// system that already hunts for 1ACFFC1D needs no new pattern.
var ASM = [4]byte{0x1A, 0xCF, 0xFC, 0x1D}

// ASMBits is the width of the sync marker in binary digits (§3.3.1).
const ASMBits = 32

// DefaultASM returns the sync marker as octets.
func DefaultASM() []byte {
	out := make([]byte, len(ASM))
	copy(out, ASM[:])
	return out
}

// AttachASM builds a Sync-Marked Transfer Frame, per §3.3.1: the marker
// followed by the transfer frame.
func AttachASM(frame []byte) (*BitString, error) {
	if len(frame) == 0 {
		return nil, ErrEmptyFrame
	}
	out := NewBitString(0)
	out.AppendBytes(ASM[:])
	out.AppendBytes(frame)
	return out, nil
}

// StripASM recovers the transfer frame from a Sync-Marked Transfer Frame.
func StripASM(smtf *BitString) ([]byte, error) {
	if smtf == nil || smtf.Len() < ASMBits+8 {
		return nil, ErrDataTooShort
	}
	for i := 0; i < ASMBits; i++ {
		want := ASM[i/8] >> uint(7-i%8) & 1
		if smtf.Bit(i) != want {
			return nil, ErrInvalidASM
		}
	}

	body := smtf.Slice(ASMBits, smtf.Len())
	// A transfer frame is a whole number of octets, so anything left over is
	// a malformed SMTF.
	if body.Len()%8 != 0 {
		return nil, ErrDataTooShort
	}
	out := make([]byte, body.Len()/8)
	copy(out, body.Bytes())
	return out, nil
}
