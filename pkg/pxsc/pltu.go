// Package pxsc implements the Proximity-1 Coding and Synchronization Sublayer
// per CCSDS 211.2-B-3 (October 2019).
//
// This is the layer beneath pkg/pxdl. It wraps each transfer frame in a
// Proximity Link Transmission Unit — a sync marker, the frame, and a CRC-32 —
// and fills the gaps between them with idle data so the receiver keeps bit
// lock.
//
//	PLTU:  ASM (FAF320) │ transfer frame │ CRC-32
//	        3 octets       variable         4 octets
//
// It plays the same role for Proximity-1 that pkg/tmsc plays for TM and
// pkg/tcsc for TC, and the shape of the API follows those two.
//
// Three things are deliberately absent. The LDPC code of §3.4.4 and its
// pseudo-randomizer are not implemented. Neither is Viterbi decoding of the
// convolutional code: this library ships no soft-decision or trellis decoders
// anywhere, and adding one here would be out of step. The convolutional
// *encoder* is present, because it is a deterministic bit transform.
package pxsc

import "fmt"

// ASM is the Attached Synchronization Marker of CCSDS 211.2-B-3 §3.2.3.2:
// the 24-bit pattern FAF320.
//
// Note it is three octets, not the four that TM and AOS use. A Proximity-1
// link is short and re-acquires per PLTU, so the marker is cheaper.
var ASM = [3]byte{0xFA, 0xF3, 0x20}

// ASMSize is the width of the sync marker in octets (§3.2.3.1).
const ASMSize = 3

// PLTUOverhead is what a PLTU costs on top of the frame it carries.
const PLTUOverhead = ASMSize + CRC32Size

// DefaultMaxFrameLength bounds a frame recovered from a PLTU when no limit is
// given. It matches the largest Version-3 Transfer Frame, per
// CCSDS 211.0-B-6 §3.2.2.10.2.
//
// The coding sublayer itself sets no ceiling: §3.2.2 note 1 says the maximum
// comes from the mission's Maximum_Frame_Length parameter. This is a safe
// default for a decoder that has not been told one.
const DefaultMaxFrameLength = 2048

// DefaultASM returns the sync marker as a slice, matching the shape of
// pkg/tmsc.DefaultASM.
func DefaultASM() []byte {
	out := make([]byte, ASMSize)
	copy(out, ASM[:])
	return out
}

// WrapPLTU builds a Proximity Link Transmission Unit around a transfer frame,
// per §3.2.2: the sync marker, the frame, then a CRC-32 over the frame.
//
// Annex C, C1.2 note 2 is explicit that the ASM is not covered by the CRC.
func WrapPLTU(frame []byte) ([]byte, error) {
	if len(frame) == 0 {
		return nil, ErrEmptyFrame
	}

	out := make([]byte, 0, ASMSize+len(frame)+CRC32Size)
	out = append(out, ASM[:]...)
	out = append(out, frame...)

	// The CRC covers the frame alone.
	sum := ComputeCRC32(frame)
	return append(out, byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum)), nil
}

// UnwrapPLTU recovers the transfer frame from a PLTU, checking the sync marker
// and the CRC-32.
//
// Like pkg/tmsc.UnwrapCADU, this expects the marker at offset zero. Use a
// Synchronizer to find PLTUs in a byte stream.
func UnwrapPLTU(pltu []byte) ([]byte, error) {
	return UnwrapPLTUWithLimit(pltu, DefaultMaxFrameLength)
}

// UnwrapPLTUWithLimit recovers the transfer frame, rejecting one longer than
// maxFrameLength octets.
func UnwrapPLTUWithLimit(pltu []byte, maxFrameLength int) ([]byte, error) {
	if len(pltu) < PLTUOverhead+1 {
		return nil, ErrDataTooShort
	}
	if pltu[0] != ASM[0] || pltu[1] != ASM[1] || pltu[2] != ASM[2] {
		return nil, ErrInvalidASM
	}

	frameLen := len(pltu) - PLTUOverhead
	if maxFrameLength > 0 && frameLen > maxFrameLength {
		return nil, ErrFrameTooLarge
	}

	frame := pltu[ASMSize : ASMSize+frameLen]

	// §3.6: a PLTU whose CRC fails is discarded.
	if !VerifyCRC32(pltu[ASMSize:]) {
		return nil, ErrCRCMismatch
	}

	out := make([]byte, frameLen)
	copy(out, frame)
	return out, nil
}

// PLTU is a decoded Proximity Link Transmission Unit.
type PLTU struct {
	// Frame is the transfer frame the PLTU carried.
	Frame []byte
	// CRC is the attached check value.
	CRC uint32
	// Offset is where the PLTU started in the stream it came from.
	Offset int
}

// Length returns the PLTU's total width in octets.
func (p *PLTU) Length() int { return PLTUOverhead + len(p.Frame) }

// Humanize returns a human-readable summary.
func (p *PLTU) Humanize() string {
	return fmt.Sprintf("Proximity Link Transmission Unit\n"+
		"  ASM ......... FAF320\n"+
		"  Frame ....... %d octets\n"+
		"  CRC-32 ...... %#08x\n"+
		"  Offset ...... %d",
		len(p.Frame), p.CRC, p.Offset)
}
