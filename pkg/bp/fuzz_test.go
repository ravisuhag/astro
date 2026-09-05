package bp_test

import (
	"encoding/hex"
	"testing"

	"github.com/ravisuhag/astro/pkg/bp"
)

// FuzzDecodeBundle asserts one property: no input makes the decoder panic.
// Errors are the expected outcome for almost everything.
func FuzzDecodeBundle(f *testing.F) {
	// The published bundle of RFC 9173 appendix A.1.1.3, so the fuzzer starts
	// from real structure rather than random noise.
	if seed, err := hex.DecodeString(
		"9f88070000820282010282028202018202820201820018281a000f42408501010000" +
			"5823526561647920746f2067656e657261746520612033322d62797465207061796c" +
			"6f6164ff"); err == nil {
		f.Add(seed)
	}

	primary := &bp.PrimaryBlock{
		CRCType:     bp.CRC32C,
		Destination: bp.IPN(1, 2),
		Source:      bp.IPN(2, 1),
		ReportTo:    bp.IPN(2, 1),
		Timestamp:   bp.CreationTimestamp{Time: 757382400000, Sequence: 1},
		Lifetime:    3600000,
	}
	if b, err := bp.NewBundle(primary, []byte("seed payload")); err == nil {
		if encoded, err := b.Encode(); err == nil {
			f.Add(encoded)
		}
	}

	f.Add([]byte{})
	f.Add([]byte{0x9F})
	f.Add([]byte{0x9F, 0xFF})
	f.Add([]byte{0x88, 0x07, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		b, err := bp.Decode(data)
		if err != nil {
			return
		}

		// Anything that decoded must survive the rest of the API without
		// panicking either.
		_ = b.Payload()
		_ = b.Validate()
		_, _ = b.Encode()
		_, _ = b.StatusReport()
		_, _ = b.Fragment(8)
		for _, blk := range b.Blocks {
			_, _ = blk.BundleAge()
			_, _ = blk.PreviousNode()
			_, _, _ = blk.HopCount()
		}
	})
}

// FuzzReassemble asserts one property: no pair of decodable bundles makes
// Reassemble panic, however their fragment offsets and total lengths
// disagree with each other or with reality. Errors are the expected outcome
// for almost everything; only a well-formed matching pair reassembles.
func FuzzReassemble(f *testing.F) {
	// Two real fragments of the same bundle, so the fuzzer starts from a
	// pair that reassembles cleanly rather than from random noise.
	for _, seed := range []string{
		"9f8b070102820282010282028202018202820201821b000000b057824800011a0036ee8000182b44907f714185010100005474686520717569636b2062726f776e20666f7820ff",
		"9f8b070102820282010282028202018202820201821b000000b057824800011a0036ee8014182b443514fa988501010000546a756d7073206f76657220746865206c617a7920ff",
	} {
		if b, err := hex.DecodeString(seed); err == nil {
			f.Add(b, b)
		}
	}

	f.Fuzz(func(t *testing.T, a, b []byte) {
		first, err := bp.Decode(a)
		if err != nil {
			return
		}
		second, err := bp.Decode(b)
		if err != nil {
			return
		}

		// Anything that decoded must survive Reassemble without panicking,
		// no matter how its fragment fields disagree with each other.
		_, _ = bp.Reassemble([]*bp.Bundle{first, second})
	})
}

// FuzzDecodeStatusReport covers the administrative record path, which Decode
// only reaches through a bundle carrying the right flag.
func FuzzDecodeStatusReport(f *testing.F) {
	report := &bp.StatusReport{
		Delivered:        bp.StatusItem{Asserted: true},
		SubjectSource:    bp.IPN(2, 1),
		SubjectTimestamp: bp.CreationTimestamp{Time: 757382400000, Sequence: 40},
	}
	if encoded, err := report.Encode(); err == nil {
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte{0x82, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		if r, err := bp.DecodeStatusReport(data); err == nil {
			_, _ = r.Encode()
		}
	})
}
