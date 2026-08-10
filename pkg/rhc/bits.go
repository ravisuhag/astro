package rhc

// Bit-level packing.
//
// Every output of CCSDS 124.0-B-1 is a variable-length binary vector, and
// nothing in it is octet aligned. §1.6.1 fixes the order: "The first bit in
// the vector to be transmitted (i.e., the most left justified when drawing a
// figure) is defined to be 'bit N-1'", and for an unsigned value the most
// significant bit is bit N-1. So writing is MSB first.
//
// This reader and writer stay inside pkg/rhc. pkg/ldc has its own, and the two
// are kept apart on purpose: sharing them would tie two compression packages
// together for the sake of forty lines.

// BitWriter packs bits MSB first into a growing octet slice.
//
// The zero value is ready to use.
type BitWriter struct {
	data    []byte
	pending uint8
	used    uint8
}

// WriteBits appends the low n bits of v, most significant first.
func (w *BitWriter) WriteBits(v uint64, n int) {
	if n <= 0 {
		return
	}
	if n > 64 {
		n = 64
	}
	if n < 64 {
		v &= (1 << uint(n)) - 1
	}

	for n > 0 {
		room := int(8 - w.used)
		take := n
		if take > room {
			take = room
		}
		chunk := uint8((v >> uint(n-take)) & ((1 << uint(take)) - 1))

		w.pending = w.pending<<uint(take) | chunk
		w.used += uint8(take)
		n -= take

		if w.used == 8 {
			w.data = append(w.data, w.pending)
			w.pending = 0
			w.used = 0
		}
	}
}

// WriteBit appends one bit.
func (w *BitWriter) WriteBit(set bool) {
	if set {
		w.WriteBits(1, 1)
		return
	}
	w.WriteBits(0, 1)
}

// WriteString appends a literal bit string such as "110", which is how the
// spec writes its fixed codewords.
func (w *BitWriter) WriteString(bits string) {
	for _, c := range bits {
		w.WriteBit(c == '1')
	}
}

// BitLen reports how many bits have been written.
func (w *BitWriter) BitLen() int { return len(w.data)*8 + int(w.used) }

// Bytes returns the written bits, padding the last octet with zeros.
//
// The padding is this package's, not the standard's: §2.2 says framing an
// output vector is mission specific, and an octet slice is the framing this
// API offers. The true length is BitLen.
func (w *BitWriter) Bytes() []byte {
	out := make([]byte, len(w.data), len(w.data)+1)
	copy(out, w.data)
	if w.used > 0 {
		out = append(out, w.pending<<(8-w.used))
	}
	return out
}

// BitReader reads MSB first and reports exhaustion rather than panicking.
type BitReader struct {
	data []byte
	pos  int
	// limit bounds reading, in bits, so a reader over an octet slice does not
	// wander into the padding of the last octet.
	limit int
}

// NewBitReader prepares a reader over every bit of data.
func NewBitReader(data []byte) *BitReader {
	return &BitReader{data: data, limit: len(data) * 8}
}

// NewBitReaderN prepares a reader over the first n bits of data.
func NewBitReaderN(data []byte, n int) *BitReader {
	if n > len(data)*8 {
		n = len(data) * 8
	}
	if n < 0 {
		n = 0
	}
	return &BitReader{data: data, limit: n}
}

// BitsLeft reports how many bits remain.
func (r *BitReader) BitsLeft() int { return r.limit - r.pos }

// Pos reports how many bits have been consumed.
func (r *BitReader) Pos() int { return r.pos }

// ReadBits reads the next n bits as an unsigned value, most significant first.
func (r *BitReader) ReadBits(n int) (uint64, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 64 || r.BitsLeft() < n {
		return 0, ErrDataTooShort
	}

	var v uint64
	for n > 0 {
		octet := r.pos / 8
		offset := r.pos % 8
		available := 8 - offset
		take := n
		if take > available {
			take = available
		}
		chunk := (r.data[octet] >> uint(available-take)) & ((1 << uint(take)) - 1)

		v = v<<uint(take) | uint64(chunk)
		r.pos += take
		n -= take
	}
	return v, nil
}

// ReadBit reads one bit.
func (r *BitReader) ReadBit() (bool, error) {
	v, err := r.ReadBits(1)
	return v == 1, err
}
