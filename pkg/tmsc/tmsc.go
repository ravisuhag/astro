// Package tmsc implements the TM Synchronization and Channel Coding sublayer
// per CCSDS 131.0-B-4 (TM Synchronization and Channel Coding).
//
// This sublayer sits between the TM Data Link Protocol (CCSDS 132.0-B-3)
// and the physical layer, providing:
//   - Attached Sync Marker (ASM) for frame synchronization
//   - CCSDS pseudo-randomization for bit transition density assurance
//   - Channel Access Data Unit (CADU) wrapping and unwrapping
package tmsc

import "bytes"

// DefaultASM returns the standard CCSDS Attached Sync Marker (0x1ACFFC1D)
// used to identify the start of each Transfer Frame in the bitstream.
// A fresh copy is returned each call to prevent accidental mutation.
func DefaultASM() []byte {
	return []byte{0x1A, 0xCF, 0xFC, 0x1D}
}

// Randomize applies CCSDS pseudo-randomization by XOR-ing data with
// the standard PN (pseudo-noise) sequence. The same operation is used
// for both randomization and de-randomization since XOR is self-inverse.
// Returns a new slice; the input is not modified.
func Randomize(data []byte) []byte {
	out := make([]byte, len(data))
	copy(out, data)
	pn := GeneratePNSequence(len(data))
	for i := range out {
		out[i] ^= pn[i]
	}
	return out
}

// WrapCADU produces a Channel Access Data Unit from encoded frame data.
// It optionally applies CCSDS pseudo-randomization and prepends the
// Attached Sync Marker per CCSDS 131.0-B-4. If asm is nil, DefaultASM
// is used.
func WrapCADU(frameData, asm []byte, randomize bool) []byte {
	if asm == nil {
		asm = DefaultASM()
	}
	data := frameData
	if randomize {
		data = Randomize(frameData)
	}
	cadu := make([]byte, len(asm)+len(data))
	copy(cadu, asm)
	copy(cadu[len(asm):], data)
	return cadu
}

// UnwrapCADU extracts encoded frame data from a Channel Access Data Unit.
// It validates and strips the ASM, and optionally de-randomizes the data.
// If asm is nil, DefaultASM is used.
func UnwrapCADU(cadu, asm []byte, randomize bool) ([]byte, error) {
	if asm == nil {
		asm = DefaultASM()
	}
	if len(cadu) < len(asm) {
		return nil, ErrDataTooShort
	}
	if !bytes.Equal(cadu[:len(asm)], asm) {
		return nil, ErrSyncMarkerMismatch
	}
	data := make([]byte, len(cadu)-len(asm))
	copy(data, cadu[len(asm):])
	if randomize {
		data = Randomize(data)
	}
	return data, nil
}

// GeneratePNSequence produces the CCSDS pseudo-random sequence using an
// 8-bit LFSR with polynomial h(x) = x^8 + x^7 + x^5 + x^3 + 1,
// initialized to all 1s per CCSDS 131.0-B-4.
func GeneratePNSequence(length int) []byte {
	seq := make([]byte, length)
	reg := uint8(0xFF)
	for i := range length {
		var b uint8
		for bit := 7; bit >= 0; bit-- {
			output := (reg >> 7) & 1
			b |= output << uint(bit)
			// Taps: x^8(bit7), x^7(bit6), x^5(bit4), x^3(bit2)
			feedback := ((reg >> 7) ^ (reg >> 6) ^ (reg >> 4) ^ (reg >> 2)) & 1
			reg = ((reg << 1) | feedback) & 0xFF
		}
		seq[i] = b
	}
	return seq
}
