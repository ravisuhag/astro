package pxsc

import "bytes"

// Synchronizer finds PLTUs in a byte stream, per CCSDS 211.2-B-3 clause 3.6.
//
// A Proximity-1 stream is not a tidy sequence of units. PLTUs of different
// lengths are separated by runs of idle data, and the receiver has to hunt for
// each sync marker in turn. Worse, the marker is only 24 bits, so a random
// match happens roughly once every 16 million octets. The CRC is what
// separates a real PLTU from a coincidence.
//
// That is why this scans rather than parses: find a marker, try the frame
// lengths that could follow, and accept the first one whose CRC verifies.
//
// A Synchronizer is not safe for concurrent use.
type Synchronizer struct {
	// MinFrameLength is the shortest transfer frame to consider. For
	// Version-3 frames this is 5, the header size.
	MinFrameLength int
	// MaxFrameLength is the longest transfer frame to consider. Zero selects
	// DefaultMaxFrameLength.
	MaxFrameLength int
}

// DefaultMinFrameLength is the shortest Version-3 Transfer Frame: the header
// alone, per CCSDS 211.0-B-6 clause 3.2.2.10.2.
const DefaultMinFrameLength = 5

// NewSynchronizer returns a synchronizer with the Version-3 frame bounds.
func NewSynchronizer() *Synchronizer {
	return &Synchronizer{
		MinFrameLength: DefaultMinFrameLength,
		MaxFrameLength: DefaultMaxFrameLength,
	}
}

// bounds returns the effective frame length limits.
func (s *Synchronizer) bounds() (minLen, maxLen int) {
	minLen = s.MinFrameLength
	if minLen <= 0 {
		minLen = DefaultMinFrameLength
	}
	maxLen = s.MaxFrameLength
	if maxLen <= 0 {
		maxLen = DefaultMaxFrameLength
	}
	return minLen, maxLen
}

// Scan finds every PLTU in data.
//
// It walks the stream looking for sync markers. At each one it tries frame
// lengths from the minimum upward and takes the first whose CRC-32 verifies,
// then resumes after that PLTU. A marker with no verifying length is skipped
// as a false match.
func (s *Synchronizer) Scan(data []byte) []PLTU {
	minLen, maxLen := s.bounds()

	var out []PLTU
	offset := 0

	for offset < len(data) {
		idx := bytes.Index(data[offset:], ASM[:])
		if idx < 0 {
			break
		}
		start := offset + idx

		pltu, ok := s.tryAt(data, start, minLen, maxLen)
		if !ok {
			// A false marker match. Step one octet past it and keep hunting,
			// so a marker hiding inside frame data is still found.
			offset = start + 1
			continue
		}

		out = append(out, pltu)
		offset = start + pltu.Length()
	}
	return out
}

// tryAt attempts to read a PLTU beginning at start.
//
// Clause 3.6 describes a receiver that reads the frame's own Length field to find
// the end of the PLTU, so the length the header implies is checked first.
// Only when that fails (a corrupted header, or a frame this synchronizer
// cannot parse) does it fall back to scanning every length in bounds.
func (s *Synchronizer) tryAt(data []byte, start, minLen, maxLen int) (PLTU, bool) {
	available := len(data) - start - PLTUOverhead
	if available < minLen {
		return PLTU{}, false
	}
	if maxLen > available {
		maxLen = available
	}

	// The length the frame header claims, tried before any scanning.
	implied := impliedFrameLength(data[start+ASMSize:])
	if implied >= minLen && implied <= maxLen {
		if pltu, ok := s.checkAt(data, start, implied); ok {
			return pltu, true
		}
	}

	return s.scanLengths(data, start, minLen, maxLen, implied)
}

// scanLengths is tryAt's fallback: the header's self-declared length (if
// any) already failed, so every candidate frame length from minLen to
// maxLen is tried in turn.
//
// Each candidate's body extends the previous one by a single octet, so the
// CRC-32 over the growing prefix is carried forward with updateCRC32
// instead of being recomputed in full for every candidate — same lengths
// tried, same CRCs compared, same frames found, just in O(maxLen) rather
// than O(maxLen²).
func (s *Synchronizer) scanLengths(data []byte, start, minLen, maxLen, implied int) (PLTU, bool) {
	if maxLen < minLen {
		return PLTU{}, false
	}

	// body[:maxLen] is every octet any candidate frame could claim; the four
	// octets after that are the trailing CRC for the longest candidate, and
	// a prefix of them is the trailing CRC for every shorter one too.
	body := data[start+ASMSize : start+ASMSize+maxLen+CRC32Size]

	// crc is the running CRC-32 over body[:frameLen], advanced one octet at
	// a time as frameLen grows. Prime it for the first candidate.
	var crc uint32
	for i := 0; i < minLen; i++ {
		crc = updateCRC32(crc, body[i])
	}

	for frameLen := minLen; frameLen <= maxLen; frameLen++ {
		if frameLen != implied { // implied was already tried via checkAt
			// The expected CRC sits at body[frameLen:frameLen+4] and moves
			// with frameLen; a codeword verifies when running the running
			// CRC through those four octets zeroes the register, the same
			// test VerifyCRC32 makes over the whole candidate body.
			end := crc
			for _, b := range body[frameLen : frameLen+CRC32Size] {
				end = updateCRC32(end, b)
			}
			if end == 0 {
				frame := make([]byte, frameLen)
				copy(frame, body[:frameLen])

				crcBytes := body[frameLen : frameLen+CRC32Size]
				value := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 |
					uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

				return PLTU{Frame: frame, CRC: value, Offset: start}, true
			}
		}
		if frameLen < maxLen {
			crc = updateCRC32(crc, body[frameLen])
		}
	}
	return PLTU{}, false
}

// checkAt verifies a candidate PLTU of exactly frameLen frame octets at start.
func (s *Synchronizer) checkAt(data []byte, start, frameLen int) (PLTU, bool) {
	end := start + ASMSize + frameLen + CRC32Size
	body := data[start+ASMSize : end]

	if !VerifyCRC32(body) {
		return PLTU{}, false
	}

	frame := make([]byte, frameLen)
	copy(frame, body[:frameLen])

	crcBytes := body[frameLen:]
	crc := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 |
		uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

	return PLTU{Frame: frame, CRC: crc, Offset: start}, true
}

// impliedFrameLength reads the frame length a Version-3 Transfer Frame header
// claims for itself, or -1 when the octets do not look like one.
//
// Per CCSDS 211.0-B-6 clause 3.2.2.10, the 11-bit Frame Length field spans the low
// three bits of header octet 2 and all of octet 3, carrying a count one less
// than the total frame length.
func impliedFrameLength(body []byte) int {
	if len(body) < 4 {
		return -1
	}
	// Transfer Frame Version Number '10' in the top two bits marks Version-3.
	if body[0]>>6 != 0b10 {
		return -1
	}
	return (int(body[2]&0x07)<<8 | int(body[3])) + 1
}

// ScanFrames finds every PLTU and returns just the transfer frames.
func (s *Synchronizer) ScanFrames(data []byte) [][]byte {
	units := s.Scan(data)
	out := make([][]byte, 0, len(units))
	for i := range units {
		out = append(out, units[i].Frame)
	}
	return out
}

// FindASM returns the offset of the first sync marker at or after start, or
// -1 when there is none.
func FindASM(data []byte, start int) int {
	if start < 0 || start >= len(data) {
		return -1
	}
	idx := bytes.Index(data[start:], ASM[:])
	if idx < 0 {
		return -1
	}
	return start + idx
}
