package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Incremental input for the streaming commands.
//
// readInput reads the whole input before anything is decoded, which is fine
// for a command that inspects one frame but wrong for the ones documented as
// streaming: a live pipe never reaches EOF, and a multi-gigabyte capture does
// not want to be resident. The readers here consume as they go and hand each
// unit to a callback the moment it is complete.

// maxStreamUnit caps how large one unit may be, and is also the read buffer
// size.
//
// A unit's length comes from the data itself — a length field a corrupt stream
// controls — so it needs a ceiling. A Space Packet stops at 65542 octets and
// every frame here is smaller, so 128 KiB accepts anything legal with room to
// spare. Buffering that much up front means a unit never fails to fit, which
// is what lets the reader peek at a whole one without growing.
const maxStreamUnit = 128 * 1024

// maxUnitHeader bounds how far the reader will look ahead while trying to
// learn a unit's length. Every header here is well under this; the bound stops
// a stream of octets that never yield a length from being read forever.
const maxUnitHeader = 64

// openInput returns a reader over the file named by args, or stdin, decoding
// hex on the fly when the format asks for it.
//
// The returned closer is always non-nil.
func openInput(args []string, inputFmt string) (io.Reader, io.Closer, error) {
	var source io.Reader = os.Stdin
	var closer io.Closer = noopCloser{}

	if len(args) > 0 && args[0] != "-" {
		file, err := os.Open(args[0])
		if err != nil {
			return nil, nil, fmt.Errorf("reading input: %w", err)
		}
		source, closer = file, file
	}

	switch inputFmt {
	case "bin":
		return source, closer, nil
	case "hex":
		return hex.NewDecoder(newHexFilter(source)), closer, nil
	default:
		_ = closer.Close()
		return nil, nil, fmt.Errorf("unknown input format: %s (use 'hex' or 'bin')", inputFmt)
	}
}

// noopCloser stands in when the input is stdin, which this command does not
// own and must not close.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// hexFilter drops the whitespace that hex input is usually laid out with, so
// encoding/hex sees an unbroken run of digits.
//
// It exists because hex.Decoder rejects whitespace outright, and a capture
// written as lines of octets is the normal shape of hex input.
type hexFilter struct {
	source io.Reader
	buf    []byte
}

func newHexFilter(source io.Reader) *hexFilter {
	return &hexFilter{source: source, buf: make([]byte, 4096)}
}

func (h *hexFilter) Read(p []byte) (int, error) {
	for {
		limit := len(p)
		if limit > len(h.buf) {
			limit = len(h.buf)
		}

		n, err := h.source.Read(h.buf[:limit])
		written := 0
		for _, c := range h.buf[:n] {
			switch c {
			case ' ', '\n', '\r', '\t':
				// Layout, not data.
			default:
				p[written] = c
				written++
			}
		}
		if written > 0 || err != nil {
			return written, err
		}
		// The whole read was whitespace; go round again rather than returning
		// (0, nil), which a caller may treat as EOF.
	}
}

// UnitSizer reports the length of the unit starting at the front of data, or a
// value below one when the data so far is too short to tell.
//
// It mirrors the package-level sizers the protocol packages expose, such as
// spp.PacketSizer.
type UnitSizer func(data []byte) int

// streamUnits reads variable-length units from source and calls handle with
// each complete one, in order.
//
// The slice handed to handle is only valid for the duration of the call; it
// aliases the reader's buffer. A handler that keeps it must copy.
//
// Trailing octets that do not form a whole unit are reported through trailing,
// which is called once at the end when there are any. That is a warning rather
// than an error: a capture cut mid-packet is a normal thing to be handed.
func streamUnits(source io.Reader, sizer UnitSizer, minimum int, handle func(unit []byte) error, trailing func(n int)) error {
	reader := bufio.NewReaderSize(source, maxStreamUnit)

	for {
		// Read enough of the front to learn how long this unit is. A sizer
		// reports "not yet" by returning a value below one, which happens
		// where the header itself is variable-length — an Encapsulation
		// Packet's length field is one, two or four octets, and which it is
		// only becomes clear from the first. So peek wider until the sizer
		// can answer.
		size := 0
		var header []byte
		var err error

		for want := minimum; want <= maxUnitHeader; want++ {
			header, err = peekAtLeast(reader, want)
			if err == io.EOF {
				if len(header) > 0 && trailing != nil {
					trailing(len(header))
				}
				return nil
			}
			if err != nil {
				return err
			}
			if size = sizer(header); size >= 1 {
				break
			}
		}

		if size < 1 {
			return fmt.Errorf("cannot determine unit length at this position")
		}
		if size > maxStreamUnit {
			return fmt.Errorf("unit length %d exceeds the %d-octet maximum", size, maxStreamUnit)
		}

		unit, err := peekAtLeast(reader, size)
		if err == io.EOF {
			// The stream ended part way through a unit.
			if trailing != nil {
				trailing(len(unit))
			}
			return nil
		}
		if err != nil {
			return err
		}

		if err := handle(unit[:size]); err != nil {
			return err
		}
		if _, err := reader.Discard(size); err != nil {
			return err
		}
	}
}

// peekAtLeast returns at least n buffered octets without consuming them,
// returning io.EOF along with whatever it has when the stream ends first.
//
// The reader's buffer is maxStreamUnit and the caller has already refused
// anything larger, so ErrBufferFull cannot happen here; it is still reported
// rather than ignored.
func peekAtLeast(reader *bufio.Reader, n int) ([]byte, error) {
	data, err := reader.Peek(n)
	if err == bufio.ErrBufferFull {
		return data, fmt.Errorf("unit of %d octets does not fit the read buffer", n)
	}
	return data, err
}
