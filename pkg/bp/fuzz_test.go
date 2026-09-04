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
