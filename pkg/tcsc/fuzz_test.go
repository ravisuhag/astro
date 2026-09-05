package tcsc_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/tcsc"
)

// FuzzUnwrapCLTU fuzzes the CLTU unwrapper (CCSDS 231.0-B-4): the start
// sequence, the BCH(63,56) codeblocks and the tail sequence all arrive
// together as one octet string off the uplink receiver, so this is the
// first thing that sees a forward-link transmission.
func FuzzUnwrapCLTU(f *testing.F) {
	if cltu, err := tcsc.WrapCLTU(bytes.Repeat([]byte{0x33}, 40), nil, nil, false); err == nil {
		f.Add(cltu, false, false)
	}
	if cltu, err := tcsc.WrapCLTU(bytes.Repeat([]byte{0xAA}, 56), nil, nil, true); err == nil {
		f.Add(cltu, true, false)
	}
	f.Add([]byte{}, false, false)
	f.Add(tcsc.DefaultStartSequence(), false, false)
	f.Add(append(tcsc.DefaultStartSequence(), tcsc.DefaultTailSequence()...), false, false)

	f.Fuzz(func(t *testing.T, cltu []byte, randomize, ted bool) {
		// Property: never panic. Malformed input is an error, not a crash.
		mode := tcsc.ModeSEC
		if ted {
			mode = tcsc.ModeTED
		}
		_, _, _ = tcsc.UnwrapCLTUWithMode(cltu, nil, nil, randomize, mode)
	})
}

// FuzzBCHDecode fuzzes the BCH(63,56) codeblock decoder (CCSDS 231.0-B-4
// section 3): one codeblock is 8 octets, and UnwrapCLTUWithMode calls this
// once per codeblock found between the CLTU's start and tail sequences, so
// its input is exactly as untrusted as the CLTU that carries it.
func FuzzBCHDecode(f *testing.F) {
	f.Add(make([]byte, tcsc.CodeblockBytes), false)
	f.Add(bytes.Repeat([]byte{0xFF}, tcsc.CodeblockBytes), false)
	f.Add([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, true)

	f.Fuzz(func(t *testing.T, data []byte, ted bool) {
		var cb [tcsc.CodeblockBytes]byte
		copy(cb[:], data)

		mode := tcsc.ModeSEC
		if ted {
			mode = tcsc.ModeTED
		}
		_, _, _ = tcsc.BCHDecodeWithMode(cb, mode)
	})
}
