package ldc

// Bit-level packing.
//
// Everything in CCSDS 121.0-B-3 happens below the octet: an FS codeword is as
// many bits as the sample it encodes, split bits are k wide, and an option
// identifier is three to six bits. Nothing lands on an octet boundary until
// the fill at the very end.
//
// Bit order is MSB first, per clause 1.5.2: "The first bit in the word to be
// transmitted ... is defined to be 'bit 0'", and for an unsigned value the
// most significant bit corresponds to the highest power of two. So a value
// written with WriteBits appears in the output most significant bit first,
// and the first octet's bit 7 is the first bit of the stream.
//
// This reader and writer stay inside pkg/ldc. Nothing else in the repository
// works below the octet, and a shared utility with one consumer is a shared
// utility waiting to grow the wrong shape.

// BitWriter packs values MSB first into a growing octet slice.
//
// The zero value is ready to use.
type BitWriter struct {
	data []byte
	// pending holds the bits of a partly filled final octet, left aligned in
	// the low bits.
	pending uint8
	// used counts how many bits of pending are filled, 0 to 7.
	used uint8
}

// WriteBits appends the low n bits of v, most significant first.
//
// n must be 0 to 64. Bits above the low n of v are ignored.
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
		// How many more bits fit in the octet being filled.
		room := int(8 - w.used)
		take := n
		if take > room {
			take = room
		}

		// The top `take` bits of what is left of v.
		shift := uint(n - take)
		chunk := uint8((v >> shift) & ((1 << uint(take)) - 1))

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

// WriteZeros appends n zero bits. It is what an FS codeword is mostly made of,
// and writing them a run at a time avoids looping per bit at the caller.
func (w *BitWriter) WriteZeros(n uint64) {
	for n >= 64 {
		w.WriteBits(0, 64)
		n -= 64
	}
	w.WriteBits(0, int(n))
}

// WriteOne appends a single one bit.
func (w *BitWriter) WriteOne() { w.WriteBits(1, 1) }

// BitLen reports how many bits have been written.
func (w *BitWriter) BitLen() int { return len(w.data)*8 + int(w.used) }

// Bytes returns the written bits, padding the last octet with zero fill.
//
// Clause 7.2.3.2 requires fill bits to be zeros. This pads only to the next octet;
// padding to the output word size is the file writer's job, because only it
// knows B.
func (w *BitWriter) Bytes() []byte {
	if w.used == 0 {
		out := make([]byte, len(w.data))
		copy(out, w.data)
		return out
	}
	out := make([]byte, len(w.data), len(w.data)+1)
	copy(out, w.data)
	return append(out, w.pending<<(8-w.used))
}

// BitReader reads MSB first and reports exhaustion rather than panicking.
type BitReader struct {
	data []byte
	// pos is the next bit to read, counted from the start of data.
	pos int
}

// NewBitReader prepares a reader over data.
func NewBitReader(data []byte) *BitReader {
	return &BitReader{data: data}
}

// BitsLeft reports how many bits remain unread.
func (r *BitReader) BitsLeft() int { return len(r.data)*8 - r.pos }

// Pos reports how many bits have been consumed.
func (r *BitReader) Pos() int { return r.pos }

// Align advances to the next octet boundary, discarding fill bits.
func (r *BitReader) Align() {
	if rem := r.pos % 8; rem != 0 {
		r.pos += 8 - rem
	}
}

// ReadBits reads the next n bits as an unsigned value, most significant first.
//
// n must be 0 to 64. Running out of input is ErrDataTooShort, never a panic:
// this reader is the only thing standing between a hostile compressed stream
// and the decoder.
func (r *BitReader) ReadBits(n int) (uint64, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 64 {
		return 0, ErrDataTooShort
	}
	if r.BitsLeft() < n {
		return 0, ErrDataTooShort
	}

	var v uint64
	for n > 0 {
		octet := r.pos / 8
		offset := r.pos % 8

		// How many bits are left in this octet.
		available := 8 - offset
		take := n
		if take > available {
			take = available
		}

		// The bits sit `available-take` places above the octet's low end.
		shift := uint(available - take)
		chunk := (r.data[octet] >> shift) & ((1 << uint(take)) - 1)

		v = v<<uint(take) | uint64(chunk)
		r.pos += take
		n -= take
	}
	return v, nil
}

// ReadFS reads one fundamental-sequence codeword and returns the value it
// encodes: the number of zeros before the terminating one.
//
// Table 3-1 makes an FS codeword m zeros followed by a one, so decoding is
// just counting zeros. limit caps how many zeros are tolerated before the
// stream is called malformed. Without it, a run of zero octets would be read
// as an enormous sample value and the caller would allocate on it.
func (r *BitReader) ReadFS(limit uint64) (uint64, error) {
	var count uint64
	for {
		bit, err := r.ReadBits(1)
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			return count, nil
		}
		count++
		if count > limit {
			return 0, ErrDataTooShort
		}
	}
}
