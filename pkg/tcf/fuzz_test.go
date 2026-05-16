package tcf

import (
	"testing"
	"time"
)

// fuzzEpoch is the CCSDS 1958 epoch used for CUC and CDS seeds.
var fuzzEpoch = time.Date(1958, 1, 1, 0, 0, 0, 0, time.UTC)

func FuzzDecodeCUC(f *testing.F) {
	if c, err := NewCUC(time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)); err == nil {
		if encoded, err := c.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 4))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = DecodeCUC(data, fuzzEpoch)
	})
}

func FuzzDecodeCDS(f *testing.F) {
	if c, err := NewCDS(time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)); err == nil {
		if encoded, err := c.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 6))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = DecodeCDS(data, fuzzEpoch)
	})
}

func FuzzDecodeCCS(f *testing.F) {
	if c, err := NewCCS(time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)); err == nil {
		if encoded, err := c.Encode(); err == nil {
			f.Add(encoded)
		}
	}
	f.Add([]byte{})
	f.Add(make([]byte, 7))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Property: never panic. Errors are fine.
		_, _ = DecodeCCS(data)
	})
}
