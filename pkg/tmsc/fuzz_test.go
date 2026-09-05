package tmsc_test

import (
	"bytes"
	"testing"

	"github.com/ravisuhag/astro/pkg/tmsc"
)

// FuzzRSDecode fuzzes the Reed-Solomon decoder for CCSDS TM Synchronization
// and Channel Coding (CCSDS 131.0-B-5 section 4). A codeword arrives at this
// decoder straight off the downlink, symbol errors included by design, so
// nothing about its content can be assumed correct.
func FuzzRSDecode(f *testing.F) {
	rs223 := tmsc.NewRS255_223()
	if cw, err := rs223.Encode(bytes.Repeat([]byte{0x5A}, rs223.DataLen())); err == nil {
		f.Add(cw)
	}
	rs239 := tmsc.NewRS255_239()
	if cw, err := rs239.Encode(bytes.Repeat([]byte{0xA5}, rs239.DataLen())); err == nil {
		f.Add(cw)
	}
	f.Add([]byte{})
	f.Add(make([]byte, 255))
	f.Add(make([]byte, 254))
	f.Add(make([]byte, 256))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. A wrong length or an uncorrectable
		// codeword is an error, not a crash.
		_, _, _ = rs223.Decode(data)
		_, _, _ = rs239.Decode(data)
	})
}

// FuzzRSDecodeShortened fuzzes the shortened-codeblock path (CCSDS
// 131.0-B-5 clauses 4.3.7-4.3.8): interleaving depth and virtual fill are
// channel configuration rather than wire content, but a decoder must still
// refuse a data length that does not match them rather than run past the
// buffer, which is exactly what the fuzzed depth/virtualFill pair against
// fuzzed data exercises.
func FuzzRSDecodeShortened(f *testing.F) {
	rs := tmsc.NewRS255_239()

	seedData := bytes.Repeat([]byte{0x11}, rs.DataLen()*2-8)
	if cw, err := rs.EncodeShortened(seedData, 2, 8); err == nil {
		f.Add(cw, 2, 8)
	}
	f.Add([]byte{}, 1, 0)
	f.Add(make([]byte, 255), 1, 0)
	f.Add(make([]byte, 10), 3, 0)

	f.Fuzz(func(t *testing.T, data []byte, depth, virtualFill int) {
		_, _, _ = rs.DecodeShortened(data, depth, virtualFill)
	})
}
