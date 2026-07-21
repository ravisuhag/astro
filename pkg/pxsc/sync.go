package pxsc

import "bytes"

// Synchronizer finds PLTUs in a byte stream, per CCSDS 211.2-B-3 §3.6.
//
// A Proximity-1 stream is not a tidy sequence of units. PLTUs of different
// lengths are separated by runs of idle data, and the receiver has to hunt for
// each sync marker in turn. Worse, the marker is only 24 bits, so a random
// match happens roughly once every 16 million octets — the CRC is what
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
// alone, per CCSDS 211.0-B-6 §3.2.2.10.2.
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
func (s *Synchronizer) tryAt(data []byte, start, minLen, maxLen int) (PLTU, bool) {
	available := len(data) - start - PLTUOverhead
	if available < minLen {
		return PLTU{}, false
	}
	if maxLen > available {
		maxLen = available
	}

	for frameLen := minLen; frameLen <= maxLen; frameLen++ {
		end := start + ASMSize + frameLen + CRC32Size
		body := data[start+ASMSize : end]

		if !VerifyCRC32(body) {
			continue
		}

		frame := make([]byte, frameLen)
		copy(frame, body[:frameLen])

		crcBytes := body[frameLen:]
		crc := uint32(crcBytes[0])<<24 | uint32(crcBytes[1])<<16 |
			uint32(crcBytes[2])<<8 | uint32(crcBytes[3])

		return PLTU{Frame: frame, CRC: crc, Offset: start}, true
	}
	return PLTU{}, false
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
